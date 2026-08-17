package installation

import (
	"context"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type stubOIDCSettingsStore struct {
	configuration OIDCConfiguration
	settings      OIDCSettings
	updated       OIDCConfiguration
}

func (store *stubOIDCSettingsStore) LoadOIDCSettings(context.Context) (OIDCSettings, error) {
	return store.settings, nil
}

func (store *stubOIDCSettingsStore) LoadOIDCBranding(context.Context) (OIDCBranding, error) {
	return OIDCBranding{}, nil
}

func (store *stubOIDCSettingsStore) LoadOIDCButtonImage(context.Context) (OIDCButtonImage, error) {
	return OIDCButtonImage{}, nil
}

func (store *stubOIDCSettingsStore) LoadOIDCConfiguration(context.Context) (OIDCConfiguration, error) {
	return store.configuration, nil
}

func (store *stubOIDCSettingsStore) UpdateOIDCConfiguration(_ context.Context, configuration OIDCConfiguration) error {
	store.updated = configuration
	return nil
}

func (store *stubOIDCSettingsStore) SavePendingOIDCConfiguration(_ context.Context, configuration OIDCConfiguration, _ OIDCLoginAttempt) error {
	store.updated = configuration
	return nil
}

func TestOIDCSettingsManagerKeepsCorePublicURLFirst(t *testing.T) {
	current := validOIDCSetupInput()
	current.ClientSecret = "existing-secret"
	store := &stubOIDCSettingsStore{
		configuration: OIDCConfiguration{OIDCSetupInput: current},
		settings: OIDCSettings{
			PublicURL: "https://core.example.com", AccessURLs: current.AccessURLs,
			IssuerURL: current.IssuerURL, ClientID: current.ClientID,
			AdditionalScopes: current.AdditionalScopes, UsernameClaim: current.UsernameClaim,
			GroupsClaim: current.GroupsClaim, AdminGroups: current.AdminGroups,
		},
	}
	manager := &OIDCSettingsManager{
		store: store,
		discoverer: stubOIDCDiscoverer{endpoint: oauth2.Endpoint{
			AuthURL: "https://identity.example.com/authorize", TokenURL: "https://identity.example.com/token",
		}},
		now: func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) },
	}
	input := validOIDCSetupInput()
	input.ClientSecret = ""
	input.AccessURLs = []string{"https://lan.example.com", "https://core.example.com"}

	if _, err := manager.StartVerification(context.Background(), input, "https://core.example.com"); err != nil {
		t.Fatalf("StartVerification() error = %v", err)
	}
	if len(store.updated.AccessURLs) != 2 || store.updated.AccessURLs[0] != "https://core.example.com" || store.updated.AccessURLs[1] != "https://lan.example.com" {
		t.Fatalf("updated access URLs = %#v", store.updated.AccessURLs)
	}
	if store.updated.ClientSecret != "existing-secret" {
		t.Fatalf("updated client secret was not preserved")
	}
}
