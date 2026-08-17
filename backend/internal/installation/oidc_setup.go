package installation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	maxButtonImageBytes                  = 1024 * 1024
	maxAdminGroups                       = 100
	loginAttemptLifetime                 = 10 * time.Minute
	OIDCLoginPurposeSetup                = "setup"
	OIDCLoginPurposeLogin                = "login"
	OIDCLoginPurposeSettingsVerification = "settings_verification"
)

var (
	ErrAlreadyInitialized = errors.New("installation is already initialized")
	ErrOIDCDiscovery      = errors.New("OIDC provider discovery failed")
	ErrOIDCAccessURL      = errors.New("request origin is not a configured OIDC access URL")
	claimNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.:-]{0,127}$`)
	scopePattern          = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)
)

type ValidationError struct {
	Field   string
	Message string
}

func (validationError *ValidationError) Error() string {
	return validationError.Field + ": " + validationError.Message
}

type OIDCSetupInput struct {
	Name                   string
	PublicURL              string
	AccessURLs             []string
	IssuerURL              string
	ClientID               string
	ClientSecret           string
	AdditionalScopes       []string
	UsernameClaim          string
	DisplayNameClaim       string
	AvatarClaim            string
	GroupsClaim            string
	AdminGroups            []string
	ButtonText             string
	ButtonImage            []byte
	ButtonImageContentType string
	RedirectURI            string
}

type OIDCConfiguration struct {
	OIDCSetupInput
	UpdatedAt time.Time
}

type OIDCLoginAttempt struct {
	StateHash    [sha256.Size]byte
	CodeVerifier string
	Nonce        string
	RedirectURI  string
	ExpiresAt    time.Time
	CreatedAt    time.Time
	Purpose      string
}

type OIDCAuthorization struct {
	AuthorizationURL string    `json:"authorization_url"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type OIDCSetupStore interface {
	SaveOIDCSetup(context.Context, OIDCConfiguration, OIDCLoginAttempt) error
}

type OIDCProviderDiscoverer interface {
	Discover(context.Context, string) (oauth2.Endpoint, error)
}

type remoteOIDCProviderDiscoverer struct {
	client *http.Client
}

func (discoverer remoteOIDCProviderDiscoverer) Discover(ctx context.Context, issuerURL string) (oauth2.Endpoint, error) {
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, discoverer.client), issuerURL)
	if err != nil {
		return oauth2.Endpoint{}, err
	}
	return provider.Endpoint(), nil
}

type OIDCSetupManager struct {
	store      OIDCSetupStore
	discoverer OIDCProviderDiscoverer
	now        func() time.Time
}

func NewOIDCSetupManager(store OIDCSetupStore, timeout time.Duration) *OIDCSetupManager {
	return &OIDCSetupManager{
		store:      store,
		discoverer: remoteOIDCProviderDiscoverer{client: &http.Client{Timeout: timeout}},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (manager *OIDCSetupManager) Start(ctx context.Context, input OIDCSetupInput) (OIDCAuthorization, error) {
	input = normalizeOIDCSetupInput(input)
	if err := ValidateOIDCSetup(input); err != nil {
		return OIDCAuthorization{}, err
	}

	endpoint, err := manager.discoverer.Discover(ctx, input.IssuerURL)
	if err != nil {
		return OIDCAuthorization{}, fmt.Errorf("%w: %v", ErrOIDCDiscovery, err)
	}
	if endpoint.AuthURL == "" || endpoint.TokenURL == "" {
		return OIDCAuthorization{}, fmt.Errorf("%w: authorization or token endpoint is missing", ErrOIDCDiscovery)
	}

	state, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, fmt.Errorf("generate OIDC state: %w", err)
	}
	verifier, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	nonce, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, fmt.Errorf("generate OIDC nonce: %w", err)
	}

	now := manager.now()
	attempt := OIDCLoginAttempt{
		StateHash:    sha256.Sum256([]byte(state)),
		CodeVerifier: verifier,
		Nonce:        nonce,
		RedirectURI:  input.RedirectURI,
		ExpiresAt:    now.Add(loginAttemptLifetime),
		CreatedAt:    now,
		Purpose:      OIDCLoginPurposeSetup,
	}
	if err := manager.store.SaveOIDCSetup(ctx, OIDCConfiguration{OIDCSetupInput: input, UpdatedAt: now}, attempt); err != nil {
		return OIDCAuthorization{}, err
	}

	configuration := oauth2.Config{
		ClientID:     input.ClientID,
		ClientSecret: input.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  input.RedirectURI,
		Scopes:       append([]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}, input.AdditionalScopes...),
	}
	return OIDCAuthorization{
		AuthorizationURL: configuration.AuthCodeURL(
			state,
			oidc.Nonce(nonce),
			oauth2.S256ChallengeOption(verifier),
		),
		ExpiresAt: attempt.ExpiresAt,
	}, nil
}

func normalizeOIDCSetupInput(input OIDCSetupInput) OIDCSetupInput {
	input.Name = strings.TrimSpace(input.Name)
	input.PublicURL = strings.TrimRight(strings.TrimSpace(input.PublicURL), "/")
	input.IssuerURL = strings.TrimSpace(input.IssuerURL)
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.UsernameClaim = strings.TrimSpace(input.UsernameClaim)
	input.DisplayNameClaim = strings.TrimSpace(input.DisplayNameClaim)
	input.AvatarClaim = strings.TrimSpace(input.AvatarClaim)
	input.GroupsClaim = strings.TrimSpace(input.GroupsClaim)
	input.ButtonText = strings.TrimSpace(input.ButtonText)
	if len(input.ButtonImage) > 0 {
		input.ButtonImageContentType = http.DetectContentType(input.ButtonImage)
	}

	seenScopes := map[string]struct{}{"openid": {}, "profile": {}, "email": {}}
	normalizedScopes := make([]string, 0, len(input.AdditionalScopes))
	for _, value := range input.AdditionalScopes {
		scope := strings.TrimSpace(value)
		if scope == "" {
			continue
		}
		if _, exists := seenScopes[scope]; exists {
			continue
		}
		seenScopes[scope] = struct{}{}
		normalizedScopes = append(normalizedScopes, scope)
	}
	input.AdditionalScopes = normalizedScopes

	seenAccessURLs := make(map[string]struct{}, len(input.AccessURLs)+1)
	normalizedAccessURLs := make([]string, 0, len(input.AccessURLs)+1)
	if len(input.AccessURLs) == 0 && input.PublicURL != "" {
		input.AccessURLs = []string{input.PublicURL}
	}
	for _, value := range input.AccessURLs {
		accessURL := strings.TrimRight(strings.TrimSpace(value), "/")
		if accessURL == "" {
			continue
		}
		if _, exists := seenAccessURLs[accessURL]; exists {
			continue
		}
		seenAccessURLs[accessURL] = struct{}{}
		normalizedAccessURLs = append(normalizedAccessURLs, accessURL)
	}
	input.AccessURLs = normalizedAccessURLs
	if len(input.AccessURLs) > 0 {
		input.RedirectURI = input.AccessURLs[0] + "/api/v1/auth/oidc/callback"
	}

	seenGroups := make(map[string]struct{}, len(input.AdminGroups))
	normalizedGroups := make([]string, 0, len(input.AdminGroups))
	for _, value := range input.AdminGroups {
		group := strings.TrimSpace(value)
		if group == "" {
			continue
		}
		if _, exists := seenGroups[group]; exists {
			continue
		}
		seenGroups[group] = struct{}{}
		normalizedGroups = append(normalizedGroups, group)
	}
	input.AdminGroups = normalizedGroups
	return input
}

func ValidateOIDCSetup(input OIDCSetupInput) error {
	if input.Name == "" || len(input.Name) > 128 {
		return invalidOIDCField("name", "must contain between 1 and 128 characters")
	}
	publicURL, err := url.ParseRequestURI(input.PublicURL)
	if err != nil || (publicURL.Scheme != "http" && publicURL.Scheme != "https") || publicURL.Host == "" || (publicURL.Path != "" && publicURL.Path != "/") || publicURL.RawQuery != "" || publicURL.Fragment != "" {
		return invalidOIDCField("public_url", "must be an absolute HTTP or HTTPS URL without a path, query, or fragment")
	}
	issuer, err := url.ParseRequestURI(input.IssuerURL)
	if err != nil || (issuer.Scheme != "http" && issuer.Scheme != "https") || issuer.Host == "" {
		return invalidOIDCField("issuer_url", "must be an absolute HTTP or HTTPS URL")
	}
	if input.ClientID == "" || len(input.ClientID) > 512 {
		return invalidOIDCField("client_id", "must contain between 1 and 512 characters")
	}
	if input.ClientSecret == "" || len(input.ClientSecret) > 4096 {
		return invalidOIDCField("client_secret", "must contain between 1 and 4096 characters")
	}
	if !claimNamePattern.MatchString(input.UsernameClaim) {
		return invalidOIDCField("username_claim", "must be a valid claim name")
	}
	if input.DisplayNameClaim != "" && !claimNamePattern.MatchString(input.DisplayNameClaim) {
		return invalidOIDCField("display_name_claim", "must be a valid claim name")
	}
	if len(input.AccessURLs) == 0 || len(input.AccessURLs) > 20 {
		return invalidOIDCField("access_urls", "must contain between 1 and 20 URLs")
	}
	for _, accessURL := range input.AccessURLs {
		parsed, parseErr := url.ParseRequestURI(accessURL)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return invalidOIDCField("access_urls", "must contain absolute HTTP or HTTPS origins without paths, queries, or fragments")
		}
	}
	if input.AvatarClaim != "" && !claimNamePattern.MatchString(input.AvatarClaim) {
		return invalidOIDCField("avatar_claim", "must be a valid claim name")
	}
	if !claimNamePattern.MatchString(input.GroupsClaim) {
		return invalidOIDCField("groups_claim", "must be a valid claim name")
	}
	if len(input.AdminGroups) == 0 || len(input.AdminGroups) > maxAdminGroups {
		return invalidOIDCField("admin_groups", "must contain between 1 and 100 comma-separated groups")
	}
	for _, group := range input.AdminGroups {
		if len(group) > 255 {
			return invalidOIDCField("admin_groups", "each group must contain at most 255 characters")
		}
	}
	if input.ButtonText == "" || len(input.ButtonText) > 128 {
		return invalidOIDCField("button_text", "must contain between 1 and 128 characters")
	}
	if len(input.AdditionalScopes) > 20 {
		return invalidOIDCField("additional_scopes", "must contain at most 20 scopes")
	}
	for _, scope := range input.AdditionalScopes {
		if !scopePattern.MatchString(scope) {
			return invalidOIDCField("additional_scopes", "contains an invalid scope")
		}
	}
	if len(input.ButtonImage) > maxButtonImageBytes {
		return invalidOIDCField("button_image", "must be 1 MB or smaller")
	}
	if len(input.ButtonImage) > 0 && input.ButtonImageContentType != "image/jpeg" && input.ButtonImageContentType != "image/png" {
		return invalidOIDCField("button_image", "must be a JPG or PNG file")
	}
	redirectURI, err := url.ParseRequestURI(input.RedirectURI)
	if err != nil || (redirectURI.Scheme != "http" && redirectURI.Scheme != "https") || redirectURI.Host == "" || redirectURI.Path != "/api/v1/auth/oidc/callback" {
		return invalidOIDCField("redirect_uri", "must be an absolute Core OIDC callback URL")
	}
	return nil
}

func invalidOIDCField(field string, message string) error {
	return &ValidationError{Field: field, Message: message}
}

func randomURLSafeValue(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
