package installation

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type stubOIDCSetupStore struct {
	configuration OIDCConfiguration
	attempt       OIDCLoginAttempt
	err           error
}

func (store *stubOIDCSetupStore) SaveOIDCSetup(_ context.Context, configuration OIDCConfiguration, attempt OIDCLoginAttempt) error {
	store.configuration = configuration
	store.attempt = attempt
	return store.err
}

type stubOIDCDiscoverer struct {
	endpoint oauth2.Endpoint
	err      error
}

func (discoverer stubOIDCDiscoverer) Discover(context.Context, string) (oauth2.Endpoint, error) {
	return discoverer.endpoint, discoverer.err
}

func validOIDCSetupInput() OIDCSetupInput {
	return OIDCSetupInput{
		Name:             "KyleAuth",
		PublicURL:        "https://core.example.com",
		AccessURLs:       []string{"https://core.example.com"},
		IssuerURL:        "https://identity.example.com",
		ClientID:         "kaeru-core",
		ClientSecret:     "secret",
		AdditionalScopes: []string{"groups"},
		UsernameClaim:    "preferred_username",
		AvatarClaim:      "picture",
		GroupsClaim:      "groups",
		AdminGroups:      []string{"kaeru-admins", "platform-admins"},
		ButtonText:       "Sign in with Kaeru",
	}
}

func TestOIDCSetupManagerPersistsAttemptAndBuildsAuthorizationURL(t *testing.T) {
	store := &stubOIDCSetupStore{}
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	manager := &OIDCSetupManager{
		store: store,
		discoverer: stubOIDCDiscoverer{endpoint: oauth2.Endpoint{
			AuthURL:  "https://identity.example.com/authorize",
			TokenURL: "https://identity.example.com/token",
		}},
		now: func() time.Time { return now },
	}

	input := validOIDCSetupInput()
	input.AccessURLs = nil
	result, err := manager.Start(context.Background(), input)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	authorizationURL, err := url.Parse(result.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	query := authorizationURL.Query()
	if query.Get("client_id") != "kaeru-core" || query.Get("redirect_uri") != "https://core.example.com/api/v1/auth/oidc/callback" {
		t.Fatalf("unexpected authorization query: %v", query)
	}
	if query.Get("scope") != "openid profile email groups" {
		t.Fatalf("unexpected scopes: %q", query.Get("scope"))
	}
	if query.Get("nonce") == "" || query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL is missing OIDC protections: %v", query)
	}
	state := query.Get("state")
	if state == "" || store.attempt.StateHash != sha256.Sum256([]byte(state)) {
		t.Fatal("stored login attempt does not match the authorization state")
	}
	if store.attempt.ExpiresAt != now.Add(loginAttemptLifetime) || result.ExpiresAt != store.attempt.ExpiresAt {
		t.Fatalf("unexpected expiry: %v", result.ExpiresAt)
	}
	if len(store.configuration.AdminGroups) != 2 || len(store.configuration.AccessURLs) != 1 || store.configuration.AccessURLs[0] != "https://core.example.com" || store.configuration.RedirectURI != "https://core.example.com/api/v1/auth/oidc/callback" {
		t.Fatalf("unexpected normalized setup configuration: %#v", store.configuration)
	}
}

func TestValidateOIDCSetupRejectsInvalidImage(t *testing.T) {
	input := validOIDCSetupInput()
	input.ButtonImage = []byte("<svg></svg>")
	input = normalizeOIDCSetupInput(input)

	err := ValidateOIDCSetup(input)
	var validationError *ValidationError
	if err == nil || !errors.As(err, &validationError) || validationError.Field != "button_image" {
		t.Fatalf("expected button_image validation error, got %v", err)
	}
}
