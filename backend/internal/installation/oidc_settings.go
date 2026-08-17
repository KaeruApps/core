package installation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var ErrOIDCVerificationRequired = errors.New("OIDC settings changes require verification")

type OIDCSettings struct {
	Name                  string   `json:"name"`
	PublicURL             string   `json:"public_url"`
	AccessURLs            []string `json:"access_urls"`
	IssuerURL             string   `json:"issuer_url"`
	ClientID              string   `json:"client_id"`
	ClientSecretSet       bool     `json:"client_secret_set"`
	AdditionalScopes      []string `json:"additional_scopes"`
	UsernameClaim         string   `json:"username_claim"`
	DisplayNameClaim      string   `json:"display_name_claim"`
	AvatarClaim           string   `json:"avatar_claim"`
	GroupsClaim           string   `json:"groups_claim"`
	AdminGroups           []string `json:"admin_groups"`
	ButtonText            string   `json:"button_text"`
	ButtonImageConfigured bool     `json:"button_image_configured"`
	CallbackURL           string   `json:"callback_url"`
}

type OIDCButtonImage struct {
	Content     []byte
	ContentType string
}

type OIDCBranding struct {
	Name                  string `json:"name"`
	ButtonText            string `json:"button_text"`
	ButtonImageConfigured bool   `json:"button_image_configured"`
}

type OIDCSettingsStore interface {
	LoadOIDCSettings(context.Context) (OIDCSettings, error)
	LoadOIDCBranding(context.Context) (OIDCBranding, error)
	LoadOIDCButtonImage(context.Context) (OIDCButtonImage, error)
	LoadOIDCConfiguration(context.Context) (OIDCConfiguration, error)
	UpdateOIDCConfiguration(context.Context, OIDCConfiguration) error
	SavePendingOIDCConfiguration(context.Context, OIDCConfiguration, OIDCLoginAttempt) error
}

type OIDCSettingsManager struct {
	store      OIDCSettingsStore
	discoverer OIDCProviderDiscoverer
	now        func() time.Time
}

func NewOIDCSettingsManager(store OIDCSettingsStore, timeout time.Duration) *OIDCSettingsManager {
	return &OIDCSettingsManager{
		store:      store,
		discoverer: remoteOIDCProviderDiscoverer{client: &http.Client{Timeout: timeout}},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (manager *OIDCSettingsManager) Get(ctx context.Context) (OIDCSettings, error) {
	return manager.store.LoadOIDCSettings(ctx)
}

func (manager *OIDCSettingsManager) GetBranding(ctx context.Context) (OIDCBranding, error) {
	return manager.store.LoadOIDCBranding(ctx)
}

func (manager *OIDCSettingsManager) GetButtonImage(ctx context.Context) (OIDCButtonImage, error) {
	return manager.store.LoadOIDCButtonImage(ctx)
}

func (manager *OIDCSettingsManager) Update(ctx context.Context, input OIDCSetupInput) (OIDCSettings, error) {
	secretChanged := input.ClientSecret != ""
	current, err := manager.store.LoadOIDCConfiguration(ctx)
	if err != nil {
		return OIDCSettings{}, err
	}
	if input.ClientSecret == "" {
		input.ClientSecret = current.ClientSecret
	}
	currentSettings, err := manager.store.LoadOIDCSettings(ctx)
	if err != nil {
		return OIDCSettings{}, err
	}
	input.PublicURL = currentSettings.PublicURL
	input.AccessURLs = append([]string{currentSettings.PublicURL}, input.AccessURLs...)
	input = normalizeOIDCSetupInput(input)
	if oidcAuthenticationChanged(currentSettings, input, secretChanged) {
		return OIDCSettings{}, ErrOIDCVerificationRequired
	}
	if err := ValidateOIDCSetup(input); err != nil {
		return OIDCSettings{}, err
	}
	endpoint, err := manager.discoverer.Discover(ctx, input.IssuerURL)
	if err != nil || endpoint.AuthURL == "" || endpoint.TokenURL == "" {
		return OIDCSettings{}, fmt.Errorf("%w: provider endpoints are unavailable", ErrOIDCDiscovery)
	}
	if err := manager.store.UpdateOIDCConfiguration(ctx, OIDCConfiguration{
		OIDCSetupInput: input,
		UpdatedAt:      manager.now(),
	}); err != nil {
		return OIDCSettings{}, err
	}
	return manager.store.LoadOIDCSettings(ctx)
}

func (manager *OIDCSettingsManager) StartVerification(ctx context.Context, input OIDCSetupInput, requestOrigin string) (OIDCAuthorization, error) {
	current, err := manager.store.LoadOIDCConfiguration(ctx)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	if input.ClientSecret == "" {
		input.ClientSecret = current.ClientSecret
	}
	currentSettings, err := manager.store.LoadOIDCSettings(ctx)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	input.PublicURL = currentSettings.PublicURL
	input.AccessURLs = append([]string{currentSettings.PublicURL}, input.AccessURLs...)
	input = normalizeOIDCSetupInput(input)
	if err := ValidateOIDCSetup(input); err != nil {
		return OIDCAuthorization{}, err
	}
	origin := strings.TrimRight(strings.TrimSpace(requestOrigin), "/")
	if !slices.Contains(input.AccessURLs, origin) {
		return OIDCAuthorization{}, ErrOIDCAccessURL
	}
	input.RedirectURI = origin + "/api/v1/auth/oidc/callback"
	endpoint, err := manager.discoverer.Discover(ctx, input.IssuerURL)
	if err != nil || endpoint.AuthURL == "" || endpoint.TokenURL == "" {
		return OIDCAuthorization{}, fmt.Errorf("%w: provider endpoints are unavailable", ErrOIDCDiscovery)
	}
	state, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	verifier, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	nonce, err := randomURLSafeValue(32)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	now := manager.now()
	attempt := OIDCLoginAttempt{
		StateHash: sha256.Sum256([]byte(state)), CodeVerifier: verifier, Nonce: nonce,
		RedirectURI: input.RedirectURI, ExpiresAt: now.Add(loginAttemptLifetime), CreatedAt: now,
		Purpose: OIDCLoginPurposeSettingsVerification,
	}
	if err := manager.store.SavePendingOIDCConfiguration(ctx, OIDCConfiguration{OIDCSetupInput: input, UpdatedAt: now}, attempt); err != nil {
		return OIDCAuthorization{}, err
	}
	configuration := oauth2.Config{
		ClientID: input.ClientID, ClientSecret: input.ClientSecret, Endpoint: endpoint,
		RedirectURL: input.RedirectURI,
		Scopes:      append([]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}, input.AdditionalScopes...),
	}
	return OIDCAuthorization{AuthorizationURL: configuration.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), ExpiresAt: attempt.ExpiresAt}, nil
}

func oidcAuthenticationChanged(current OIDCSettings, input OIDCSetupInput, secretChanged bool) bool {
	return secretChanged || current.IssuerURL != input.IssuerURL || current.ClientID != input.ClientID ||
		!slices.Equal(current.AdditionalScopes, input.AdditionalScopes) || current.UsernameClaim != input.UsernameClaim ||
		current.GroupsClaim != input.GroupsClaim || !slices.Equal(current.AdminGroups, input.AdminGroups) ||
		!slices.Equal(current.AccessURLs, input.AccessURLs)
}
