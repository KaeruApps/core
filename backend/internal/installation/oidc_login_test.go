package installation

import (
	"context"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type stubOIDCLoginStore struct {
	configuration OIDCConfiguration
	attempt       OIDCLoginAttempt
}

func (store *stubOIDCLoginStore) LoadOIDCConfiguration(context.Context) (OIDCConfiguration, error) {
	return store.configuration, nil
}

func (store *stubOIDCLoginStore) SaveOIDCLoginAttempt(_ context.Context, attempt OIDCLoginAttempt) error {
	store.attempt = attempt
	return nil
}

func TestOIDCLoginManagerCreatesNormalLoginAttempt(t *testing.T) {
	store := &stubOIDCLoginStore{configuration: OIDCConfiguration{OIDCSetupInput: validOIDCSetupInput()}}
	manager := &OIDCLoginManager{
		store: store,
		discoverer: stubOIDCDiscoverer{endpoint: oauth2.Endpoint{
			AuthURL: "https://identity.example.com/authorize", TokenURL: "https://identity.example.com/token",
		}},
		now: func() time.Time { return time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC) },
	}

	authorization, err := manager.Start(context.Background(), "https://core.example.com")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	parsed, err := url.Parse(authorization.AuthorizationURL)
	if err != nil || parsed.Query().Get("state") == "" {
		t.Fatalf("invalid authorization URL: %q, error = %v", authorization.AuthorizationURL, err)
	}
	if store.attempt.Purpose != OIDCLoginPurposeLogin {
		t.Fatalf("attempt purpose = %q, want login", store.attempt.Purpose)
	}
	if parsed.Query().Get("redirect_uri") != "https://core.example.com/api/v1/auth/oidc/callback" {
		t.Fatalf("redirect URI = %q, want stored configuration value", parsed.Query().Get("redirect_uri"))
	}
}

func TestOIDCLoginManagerRejectsUnknownOrigin(t *testing.T) {
	store := &stubOIDCLoginStore{configuration: OIDCConfiguration{OIDCSetupInput: validOIDCSetupInput()}}
	manager := &OIDCLoginManager{store: store}

	_, err := manager.Start(context.Background(), "https://unknown.example.com")
	if err != ErrOIDCAccessURL {
		t.Fatalf("Start() error = %v, want ErrOIDCAccessURL", err)
	}
}
