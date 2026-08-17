package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/KaeruApps/core/internal/installation"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const sessionLifetime = 30 * 24 * time.Hour

var (
	ErrLoginAttemptInvalid = errors.New("OIDC login attempt is invalid or expired")
	ErrOIDCClaimsInvalid   = errors.New("OIDC claims are invalid")
	ErrAdminGroupRequired  = errors.New("user does not belong to a configured administrator group")
	ErrUserDisabled        = errors.New("user is disabled")
)

type OIDCClaimError struct {
	Claim string
	Kind  string
}

func (err *OIDCClaimError) Error() string {
	return fmt.Sprintf("%s claim %q is missing or invalid", err.Kind, err.Claim)
}

func (err *OIDCClaimError) Unwrap() error { return ErrOIDCClaimsInvalid }

func (err *OIDCClaimError) UserMessage() string {
	return fmt.Sprintf("The identity provider did not return a valid %s claim %q.", err.Kind, err.Claim)
}

type OIDCIdentity struct {
	ID            string
	Issuer        string
	Subject       string
	Username      string
	DisplayName   string
	Email         *string
	EmailVerified *bool
	AvatarURL     *string
	Groups        []string
}

type BrowserSession struct {
	ID        string
	TokenHash [sha256.Size]byte
	CreatedAt time.Time
	ExpiresAt time.Time
	UserAgent string
	IPAddress string
}

type OIDCCallbackResult struct {
	SessionToken string
	ExpiresAt    time.Time
	SecureCookie bool
	Purpose      string
}

type OIDCCallbackStore interface {
	ConsumeOIDCLoginAttempt(context.Context, [sha256.Size]byte, time.Time) (installation.OIDCConfiguration, installation.OIDCLoginAttempt, error)
	CoreAdministratorAllowed(context.Context, []string) (bool, error)
	PendingAdministratorAllowed(context.Context, []string) (bool, error)
	BootstrapAdministrator(context.Context, OIDCIdentity, BrowserSession, time.Time) error
	CompleteUserLogin(context.Context, OIDCIdentity, BrowserSession, time.Time) error
	CompleteOIDCSettingsVerification(context.Context, OIDCIdentity, BrowserSession, time.Time) error
}

type OIDCTokenAuthenticator interface {
	Authenticate(context.Context, installation.OIDCConfiguration, installation.OIDCLoginAttempt, string) (OIDCIdentity, error)
}

type remoteOIDCTokenAuthenticator struct {
	client *http.Client
}

func (authenticator remoteOIDCTokenAuthenticator) Authenticate(
	ctx context.Context,
	configuration installation.OIDCConfiguration,
	attempt installation.OIDCLoginAttempt,
	code string,
) (OIDCIdentity, error) {
	ctx = oidc.ClientContext(ctx, authenticator.client)
	provider, err := oidc.NewProvider(ctx, configuration.IssuerURL)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("discover OIDC provider: %w", err)
	}
	oauthConfiguration := oauth2.Config{
		ClientID:     configuration.ClientID,
		ClientSecret: configuration.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  attempt.RedirectURI,
		Scopes:       append([]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}, configuration.AdditionalScopes...),
	}
	token, err := oauthConfiguration.Exchange(ctx, code, oauth2.VerifierOption(attempt.CodeVerifier))
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("exchange OIDC authorization code: %w", err)
	}
	rawIDToken, exists := token.Extra("id_token").(string)
	if !exists || rawIDToken == "" {
		return OIDCIdentity{}, errors.New("OIDC token response does not contain an ID token")
	}
	verifiedToken, err := provider.Verifier(&oidc.Config{ClientID: configuration.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return OIDCIdentity{}, fmt.Errorf("verify OIDC ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(verifiedToken.Nonce), []byte(attempt.Nonce)) != 1 {
		return OIDCIdentity{}, errors.New("OIDC nonce does not match")
	}

	claims := map[string]json.RawMessage{}
	if err := verifiedToken.Claims(&claims); err != nil {
		return OIDCIdentity{}, fmt.Errorf("decode OIDC ID token claims: %w", err)
	}
	requiredClaimsMissing := claimMissing(claims, configuration.UsernameClaim) || claimMissing(claims, configuration.GroupsClaim)
	optionalClaimsMissing := claimMissing(claims, "email") ||
		claimMissing(claims, configuration.DisplayNameClaim) ||
		claimMissing(claims, configuration.AvatarClaim)
	if requiredClaimsMissing || optionalClaimsMissing {
		userInfo, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			if requiredClaimsMissing {
				return OIDCIdentity{}, fmt.Errorf("load OIDC UserInfo claims: %w", err)
			}
		} else {
			if userInfo.Subject != verifiedToken.Subject {
				return OIDCIdentity{}, errors.New("OIDC UserInfo subject does not match ID token")
			}
			userInfoClaims := map[string]json.RawMessage{}
			if err := userInfo.Claims(&userInfoClaims); err != nil {
				return OIDCIdentity{}, fmt.Errorf("decode OIDC UserInfo claims: %w", err)
			}
			for name, value := range userInfoClaims {
				if claimMissing(claims, name) {
					claims[name] = value
				}
			}
		}
	}

	identity, err := identityFromClaims(configuration, verifiedToken.Subject, claims)
	if err != nil {
		return OIDCIdentity{}, err
	}
	return identity, nil
}

type OIDCCallbackManager struct {
	store         OIDCCallbackStore
	authenticator OIDCTokenAuthenticator
	now           func() time.Time
}

func NewOIDCCallbackManager(store OIDCCallbackStore, timeout time.Duration) *OIDCCallbackManager {
	return &OIDCCallbackManager{
		store:         store,
		authenticator: remoteOIDCTokenAuthenticator{client: &http.Client{Timeout: timeout}},
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (manager *OIDCCallbackManager) Complete(
	ctx context.Context,
	state string,
	code string,
	userAgent string,
	ipAddress string,
) (OIDCCallbackResult, error) {
	if state == "" || code == "" {
		return OIDCCallbackResult{}, ErrLoginAttemptInvalid
	}
	now := manager.now()
	configuration, attempt, err := manager.store.ConsumeOIDCLoginAttempt(ctx, sha256.Sum256([]byte(state)), now)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	identity, err := manager.authenticator.Authenticate(ctx, configuration, attempt, code)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	var allowed bool
	if attempt.Purpose == installation.OIDCLoginPurposeSettingsVerification {
		allowed, err = manager.store.PendingAdministratorAllowed(ctx, identity.Groups)
	} else {
		allowed, err = manager.store.CoreAdministratorAllowed(ctx, identity.Groups)
	}
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	if !allowed {
		return OIDCCallbackResult{}, ErrAdminGroupRequired
	}
	userID, err := randomUUID()
	if err != nil {
		return OIDCCallbackResult{}, fmt.Errorf("generate user ID: %w", err)
	}
	identity.ID = userID

	sessionToken, err := randomToken(32)
	if err != nil {
		return OIDCCallbackResult{}, fmt.Errorf("generate session token: %w", err)
	}
	sessionID, err := randomUUID()
	if err != nil {
		return OIDCCallbackResult{}, fmt.Errorf("generate session ID: %w", err)
	}
	session := BrowserSession{
		ID:        sessionID,
		TokenHash: sha256.Sum256([]byte(sessionToken)),
		CreatedAt: now,
		ExpiresAt: now.Add(sessionLifetime),
		UserAgent: truncate(strings.TrimSpace(userAgent), 1024),
		IPAddress: strings.TrimSpace(ipAddress),
	}
	if attempt.Purpose == installation.OIDCLoginPurposeSetup {
		if err := manager.store.BootstrapAdministrator(ctx, identity, session, now); err != nil {
			return OIDCCallbackResult{}, err
		}
	} else if attempt.Purpose == installation.OIDCLoginPurposeLogin {
		if err := manager.store.CompleteUserLogin(ctx, identity, session, now); err != nil {
			return OIDCCallbackResult{}, err
		}
	} else if attempt.Purpose == installation.OIDCLoginPurposeSettingsVerification {
		if err := manager.store.CompleteOIDCSettingsVerification(ctx, identity, session, now); err != nil {
			return OIDCCallbackResult{}, err
		}
	} else {
		return OIDCCallbackResult{}, ErrLoginAttemptInvalid
	}
	redirectURI, _ := url.Parse(attempt.RedirectURI)
	return OIDCCallbackResult{
		SessionToken: sessionToken,
		ExpiresAt:    session.ExpiresAt,
		SecureCookie: redirectURI != nil && redirectURI.Scheme == "https",
		Purpose:      attempt.Purpose,
	}, nil
}

func identityFromClaims(configuration installation.OIDCConfiguration, subject string, claims map[string]json.RawMessage) (OIDCIdentity, error) {
	username, ok := stringClaim(claims, configuration.UsernameClaim)
	username = strings.TrimSpace(username)
	if !ok || username == "" || len(username) > 255 {
		return OIDCIdentity{}, &OIDCClaimError{Claim: configuration.UsernameClaim, Kind: "username"}
	}
	groups, err := groupsClaim(claims, configuration.GroupsClaim)
	if err != nil {
		return OIDCIdentity{}, &OIDCClaimError{Claim: configuration.GroupsClaim, Kind: "groups"}
	}
	displayName := username
	if configuration.DisplayNameClaim != "" {
		if value, exists := stringClaim(claims, configuration.DisplayNameClaim); exists {
			value = strings.TrimSpace(value)
			if value != "" && len(value) <= 255 {
				displayName = value
			}
		}
	}

	var email *string
	if value, ok := stringClaim(claims, "email"); ok {
		value = strings.TrimSpace(value)
		if value != "" && len(value) <= 320 {
			email = &value
		}
	}
	var emailVerified *bool
	if email != nil {
		if value, ok := boolClaim(claims, "email_verified"); ok {
			emailVerified = &value
		}
	}
	var avatarURL *string
	if configuration.AvatarClaim != "" {
		if value, ok := stringClaim(claims, configuration.AvatarClaim); ok && validAvatarURL(value) {
			value = strings.TrimSpace(value)
			avatarURL = &value
		}
	}

	return OIDCIdentity{
		Issuer:        configuration.IssuerURL,
		Subject:       subject,
		Username:      username,
		DisplayName:   displayName,
		Email:         email,
		EmailVerified: emailVerified,
		AvatarURL:     avatarURL,
		Groups:        groups,
	}, nil
}

func groupsClaim(claims map[string]json.RawMessage, name string) ([]string, error) {
	raw, exists := claims[name]
	if !exists {
		return nil, fmt.Errorf("groups claim %q is missing", name)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		var single string
		if singleErr := json.Unmarshal(raw, &single); singleErr != nil {
			return nil, fmt.Errorf("groups claim %q must be a string or array of strings", name)
		}
		values = []string{single}
	}
	unique := make(map[string]struct{}, len(values))
	groups := make([]string, 0, len(values))
	for _, value := range values {
		group := strings.TrimSpace(value)
		if group == "" || len(group) > 255 {
			return nil, fmt.Errorf("groups claim %q contains an invalid group", name)
		}
		if _, exists := unique[group]; exists {
			continue
		}
		unique[group] = struct{}{}
		groups = append(groups, group)
	}
	if len(groups) > 1000 {
		return nil, fmt.Errorf("groups claim %q contains too many groups", name)
	}
	sort.Strings(groups)
	return groups, nil
}

func stringClaim(claims map[string]json.RawMessage, name string) (string, bool) {
	raw, exists := claims[name]
	if !exists {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func boolClaim(claims map[string]json.RawMessage, name string) (bool, bool) {
	raw, exists := claims[name]
	if !exists {
		return false, false
	}
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return value, true
}

func claimMissing(claims map[string]json.RawMessage, name string) bool {
	_, exists := claims[name]
	return name != "" && !exists
}

func validAvatarURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(value)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32]), nil
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}
