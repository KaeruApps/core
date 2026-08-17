package installation

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCLoginStore interface {
	LoadOIDCConfiguration(context.Context) (OIDCConfiguration, error)
	SaveOIDCLoginAttempt(context.Context, OIDCLoginAttempt) error
}

type OIDCLoginManager struct {
	store      OIDCLoginStore
	discoverer OIDCProviderDiscoverer
	now        func() time.Time
}

func NewOIDCLoginManager(store OIDCLoginStore, timeout time.Duration) *OIDCLoginManager {
	return &OIDCLoginManager{
		store:      store,
		discoverer: remoteOIDCProviderDiscoverer{client: &http.Client{Timeout: timeout}},
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (manager *OIDCLoginManager) Start(ctx context.Context, requestOrigin string) (OIDCAuthorization, error) {
	configuration, err := manager.store.LoadOIDCConfiguration(ctx)
	if err != nil {
		return OIDCAuthorization{}, err
	}
	accessURL := strings.TrimRight(strings.TrimSpace(requestOrigin), "/")
	if accessURL == "" && len(configuration.AccessURLs) > 0 {
		accessURL = configuration.AccessURLs[0]
	}
	allowed := false
	for _, configuredURL := range configuration.AccessURLs {
		if accessURL == configuredURL {
			allowed = true
			break
		}
	}
	if !allowed {
		return OIDCAuthorization{}, ErrOIDCAccessURL
	}
	redirectURI := accessURL + "/api/v1/auth/oidc/callback"
	endpoint, err := manager.discoverer.Discover(ctx, configuration.IssuerURL)
	if err != nil || endpoint.AuthURL == "" || endpoint.TokenURL == "" {
		return OIDCAuthorization{}, fmt.Errorf("%w: provider endpoints are unavailable", ErrOIDCDiscovery)
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
		StateHash: sha256.Sum256([]byte(state)), CodeVerifier: verifier, Nonce: nonce,
		RedirectURI: redirectURI, ExpiresAt: now.Add(loginAttemptLifetime), CreatedAt: now,
		Purpose: OIDCLoginPurposeLogin,
	}
	if err := manager.store.SaveOIDCLoginAttempt(ctx, attempt); err != nil {
		return OIDCAuthorization{}, err
	}
	oauthConfiguration := oauth2.Config{
		ClientID: configuration.ClientID, ClientSecret: configuration.ClientSecret,
		Endpoint: endpoint, RedirectURL: redirectURI,
		Scopes: append([]string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}, configuration.AdditionalScopes...),
	}
	return OIDCAuthorization{
		AuthorizationURL: oauthConfiguration.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)),
		ExpiresAt:        attempt.ExpiresAt,
	}, nil
}
