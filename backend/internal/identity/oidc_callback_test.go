package identity

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/KaeruApps/core/internal/installation"
)

type stubOIDCCallbackStore struct {
	configuration installation.OIDCConfiguration
	attempt       installation.OIDCLoginAttempt
	user          OIDCIdentity
	session       BrowserSession
	bootstrapped  bool
	loggedIn      bool
	adminAllowed  bool
	err           error
}

func (store *stubOIDCCallbackStore) CoreAdministratorAllowed(context.Context, []string) (bool, error) {
	return store.adminAllowed, store.err
}

func (store *stubOIDCCallbackStore) PendingAdministratorAllowed(context.Context, []string) (bool, error) {
	return store.adminAllowed, store.err
}

func (store *stubOIDCCallbackStore) ConsumeOIDCLoginAttempt(context.Context, [sha256.Size]byte, time.Time) (installation.OIDCConfiguration, installation.OIDCLoginAttempt, error) {
	return store.configuration, store.attempt, store.err
}

func (store *stubOIDCCallbackStore) BootstrapAdministrator(_ context.Context, user OIDCIdentity, session BrowserSession, _ time.Time) error {
	store.user = user
	store.session = session
	store.bootstrapped = true
	return store.err
}

func (store *stubOIDCCallbackStore) CompleteUserLogin(_ context.Context, user OIDCIdentity, session BrowserSession, _ time.Time) error {
	store.user = user
	store.session = session
	store.loggedIn = true
	return store.err
}

func (store *stubOIDCCallbackStore) CompleteOIDCSettingsVerification(_ context.Context, user OIDCIdentity, session BrowserSession, _ time.Time) error {
	store.user = user
	store.session = session
	store.loggedIn = true
	return store.err
}

func TestOIDCCallbackCompletesNormalUserLogin(t *testing.T) {
	store := &stubOIDCCallbackStore{
		adminAllowed: true,
		attempt: installation.OIDCLoginAttempt{
			RedirectURI: "https://core.example.com/api/v1/auth/oidc/callback",
			Purpose:     installation.OIDCLoginPurposeLogin,
		},
	}
	manager := &OIDCCallbackManager{
		store: store,
		authenticator: stubOIDCTokenAuthenticator{identity: OIDCIdentity{
			Issuer: "https://identity.example.com", Subject: "subject", Username: "frog", Groups: []string{"kaeru-admins"},
		}},
		now: time.Now,
	}

	if _, err := manager.Complete(context.Background(), "state", "code", "", ""); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !store.loggedIn || store.bootstrapped {
		t.Fatalf("normal callback used wrong persistence path: loggedIn=%v bootstrapped=%v", store.loggedIn, store.bootstrapped)
	}
}

func TestOIDCCallbackCompletesSettingsVerification(t *testing.T) {
	store := &stubOIDCCallbackStore{
		adminAllowed: true,
		attempt: installation.OIDCLoginAttempt{
			RedirectURI: "https://core.example.com/api/v1/auth/oidc/callback",
			Purpose:     installation.OIDCLoginPurposeSettingsVerification,
		},
	}
	manager := &OIDCCallbackManager{
		store: store,
		authenticator: stubOIDCTokenAuthenticator{identity: OIDCIdentity{
			Issuer: "https://new-identity.example.com", Subject: "subject", Username: "frog", Groups: []string{"new-admins"},
		}},
		now: time.Now,
	}

	result, err := manager.Complete(context.Background(), "state", "code", "", "")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !store.loggedIn || result.Purpose != installation.OIDCLoginPurposeSettingsVerification {
		t.Fatalf("verification callback was not completed: store=%#v result=%#v", store, result)
	}
}

type stubOIDCTokenAuthenticator struct {
	identity OIDCIdentity
	err      error
}

func (authenticator stubOIDCTokenAuthenticator) Authenticate(context.Context, installation.OIDCConfiguration, installation.OIDCLoginAttempt, string) (OIDCIdentity, error) {
	return authenticator.identity, authenticator.err
}

func TestOIDCCallbackBootstrapsAdministratorSession(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	store := &stubOIDCCallbackStore{
		adminAllowed: true,
		attempt:      installation.OIDCLoginAttempt{RedirectURI: "https://core.example.com/api/v1/auth/oidc/callback", Purpose: installation.OIDCLoginPurposeSetup},
	}
	manager := &OIDCCallbackManager{
		store: store,
		authenticator: stubOIDCTokenAuthenticator{identity: OIDCIdentity{
			Issuer: "https://identity.example.com", Subject: "subject", Username: "Kaeru Admin",
			Groups: []string{"kaeru-admins"},
		}},
		now: func() time.Time { return now },
	}

	result, err := manager.Complete(context.Background(), "state", "code", "browser", "127.0.0.1")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.SessionToken == "" || !result.SecureCookie || result.ExpiresAt != now.Add(sessionLifetime) {
		t.Fatalf("unexpected callback result: %#v", result)
	}
	if store.user.ID == "" || store.user.Username != "Kaeru Admin" {
		t.Fatalf("unexpected bootstrapped user: %#v", store.user)
	}
	if store.session.TokenHash != sha256.Sum256([]byte(result.SessionToken)) {
		t.Fatal("stored session hash does not match returned session token")
	}
}

func TestOIDCCallbackRequiresAdminGroup(t *testing.T) {
	store := &stubOIDCCallbackStore{
		adminAllowed: false,
	}
	manager := &OIDCCallbackManager{
		store:         store,
		authenticator: stubOIDCTokenAuthenticator{identity: OIDCIdentity{Groups: []string{"users"}}},
		now:           time.Now,
	}

	_, err := manager.Complete(context.Background(), "state", "code", "", "")
	if !errors.Is(err, ErrAdminGroupRequired) {
		t.Fatalf("expected ErrAdminGroupRequired, got %v", err)
	}
}

func TestIdentityFromClaims(t *testing.T) {
	configuration := installation.OIDCConfiguration{OIDCSetupInput: installation.OIDCSetupInput{
		IssuerURL: "https://identity.example.com", UsernameClaim: "preferred_username",
		DisplayNameClaim: "name", AvatarClaim: "picture", GroupsClaim: "groups",
	}}
	claims := map[string]json.RawMessage{
		"preferred_username": json.RawMessage(`"frog"`),
		"name":               json.RawMessage(`"Friendly Frog"`),
		"groups":             json.RawMessage(`["users","admins","users"]`),
		"email":              json.RawMessage(`"frog@example.com"`),
		"email_verified":     json.RawMessage(`true`),
		"picture":            json.RawMessage(`"https://identity.example.com/avatar.png"`),
	}

	user, err := identityFromClaims(configuration, "subject", claims)
	if err != nil {
		t.Fatalf("identityFromClaims() error = %v", err)
	}
	if user.Username != "frog" || user.DisplayName != "Friendly Frog" || user.Email == nil || user.AvatarURL == nil || len(user.Groups) != 2 {
		t.Fatalf("unexpected identity: %#v", user)
	}
}

func TestIdentityFromClaimsDefaultsDisplayNameToUsername(t *testing.T) {
	configuration := installation.OIDCConfiguration{OIDCSetupInput: installation.OIDCSetupInput{
		IssuerURL: "https://identity.example.com", UsernameClaim: "preferred_username",
		DisplayNameClaim: "name", GroupsClaim: "groups",
	}}
	claims := map[string]json.RawMessage{
		"preferred_username": json.RawMessage(`"frog"`),
		"groups":             json.RawMessage(`"users"`),
	}

	user, err := identityFromClaims(configuration, "subject", claims)
	if err != nil {
		t.Fatalf("identityFromClaims() error = %v", err)
	}
	if user.DisplayName != "frog" {
		t.Fatalf("DisplayName = %q, want %q", user.DisplayName, "frog")
	}
}

func TestIdentityFromClaimsIdentifiesMissingGroupsClaim(t *testing.T) {
	configuration := installation.OIDCConfiguration{OIDCSetupInput: installation.OIDCSetupInput{
		IssuerURL: "https://identity.example.com", UsernameClaim: "preferred_username", GroupsClaim: "custom_groups",
	}}
	claims := map[string]json.RawMessage{
		"preferred_username": json.RawMessage(`"frog"`),
	}

	_, err := identityFromClaims(configuration, "subject", claims)
	var claimError *OIDCClaimError
	if !errors.As(err, &claimError) || claimError.Kind != "groups" || claimError.Claim != "custom_groups" {
		t.Fatalf("identityFromClaims() error = %#v", err)
	}
}
