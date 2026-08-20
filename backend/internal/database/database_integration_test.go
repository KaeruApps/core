package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
	"github.com/KaeruApps/core/internal/registry"
	"github.com/KaeruApps/core/internal/serviceclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

// integrationDatabaseURL returns the database these tests may destroy.
//
// The tests delete every row in the OIDC, user, and session tables, so they
// refuse to run against a database whose name does not mark it as disposable.
// Pointing them at a development database would silently wipe its setup.
func integrationDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := os.Getenv("KAERU_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KAERU_TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatalf("KAERU_TEST_DATABASE_URL is not a valid URL: %v", err)
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if !strings.Contains(name, "test") {
		t.Fatalf(
			"refusing to run destructive tests against database %q: "+
				"KAERU_TEST_DATABASE_URL must name a dedicated test database "+
				"(one whose name contains \"test\"). Run 'npm run test:integration'.",
			name,
		)
	}
	return databaseURL
}

func TestPostgresOIDCBootstrap(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	reset := func(resetContext context.Context) {
		_, _ = pool.Exec(resetContext, `
			DELETE FROM user_sessions;
			DELETE FROM user_oidc_groups;
			DELETE FROM users;
			DELETE FROM oidc_login_attempts;
			DELETE FROM oidc_settings;
			UPDATE installation_settings SET setup_state = 'required' WHERE singleton = TRUE;
		`)
	}
	reset(ctx)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		reset(cleanupContext)
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	// OIDC setup attaches administrator mappings to the Kaeru Core service, which
	// the server registers at startup. A freshly created test database has no
	// such row, so register it here rather than depending on a database that
	// happens to have been used by a running server.
	registryStore := NewRegistryStore(pool)
	if err := registryStore.EnsureCoreService(ctx, registry.NewCoreService("test", "http://kaeru-core:8080", now), now); err != nil {
		t.Fatalf("EnsureCoreService() error = %v", err)
	}
	state := "integration-state"
	stateHash := sha256.Sum256([]byte(state))
	setupStore := NewOIDCSetupStore(pool)
	configuration := installation.OIDCConfiguration{
		OIDCSetupInput: installation.OIDCSetupInput{
			Name:       "KyleAuth",
			PublicURL:  "https://core.example.com",
			AccessURLs: []string{"https://core.example.com"},
			IssuerURL:  "https://identity.example.com", ClientID: "kaeru-core", ClientSecret: "secret",
			UsernameClaim: "preferred_username", AvatarClaim: "picture", GroupsClaim: "groups",
			AdminGroups: []string{"kaeru-admins", "platform-admins"}, ButtonText: "Sign in", RedirectURI: "https://core.example.com/api/v1/auth/oidc/callback",
		},
		UpdatedAt: now,
	}
	attempt := installation.OIDCLoginAttempt{
		StateHash: stateHash, CodeVerifier: "verifier", Nonce: "nonce",
		RedirectURI: configuration.RedirectURI, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
		Purpose: installation.OIDCLoginPurposeSetup,
	}
	if err := setupStore.SaveOIDCSetup(ctx, configuration, attempt); err != nil {
		t.Fatalf("SaveOIDCSetup() error = %v", err)
	}
	loadedConfiguration, loadedAttempt, err := setupStore.ConsumeOIDCLoginAttempt(ctx, stateHash, now)
	if err != nil {
		t.Fatalf("ConsumeOIDCLoginAttempt() error = %v", err)
	}
	if loadedAttempt.Nonce != "nonce" {
		t.Fatalf("unexpected callback state: %#v, %#v", loadedConfiguration, loadedAttempt)
	}
	allowed, err := setupStore.CoreAdministratorAllowed(ctx, []string{"users", "kaeru-admins"})
	if err != nil || !allowed {
		t.Fatalf("CoreAdministratorAllowed() allowed = %v, error = %v", allowed, err)
	}

	tokenHash := sha256.Sum256([]byte("session-token"))
	user := identity.OIDCIdentity{
		ID: "019c2a46-7f5d-7ca2-9f4a-ae191ca84322", Issuer: configuration.IssuerURL,
		Subject: "admin-subject", Username: "Admin", DisplayName: "Admin", Groups: []string{"kaeru-admins", "users"},
	}
	session := identity.BrowserSession{
		ID: "019c2a46-7f5d-7ca2-9f4a-ae191ca84323", TokenHash: tokenHash,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour), UserAgent: "integration test", IPAddress: "127.0.0.1",
	}
	if err := setupStore.BootstrapAdministrator(ctx, user, session, now); err != nil {
		t.Fatalf("BootstrapAdministrator() error = %v", err)
	}
	principal, authenticated, err := NewIdentityStore(pool).PrincipalBySessionHash(ctx, tokenHash, now.Add(time.Minute))
	if err != nil || !authenticated {
		t.Fatalf("PrincipalBySessionHash() authenticated = %v, error = %v", authenticated, err)
	}
	if principal.Name != "Admin" || principal.ServiceRoles["core"] != "admin" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	users, err := NewUserDirectoryStore(pool).ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].Username != "Admin" || len(users[0].OIDCGroups) != 2 {
		t.Fatalf("unexpected user directory: %#v", users)
	}
}

func TestPostgresUserDirectoryReadOnly(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	users, err := NewUserDirectoryStore(pool).ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	t.Logf("loaded %d users", len(users))
}

// The avatar endpoint is cacheable, so a replaced avatar has to produce a
// different URL. Otherwise clients keep showing the previous image until the
// cache expires, which is what users saw in the directory listing.
func TestPostgresUserDirectoryAvatarURLChangesWithAvatar(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	userID, err := randomUUIDForTest()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, oidc_issuer, oidc_subject, username, created_at, updated_at, last_login_at, last_seen_at)
		VALUES ($1, 'https://identity.example.com', $2, $3, $4, $4, $4, $4)
	`, userID, userID, "avatar-user-"+userID[:8], now); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id = $1`, userID)
	})

	directory := NewUserDirectoryStore(pool)
	avatarStore := NewUserPreferencesStore(pool)

	findAvatarURL := func() string {
		users, err := directory.ListUsers(ctx)
		if err != nil {
			t.Fatalf("ListUsers() error = %v", err)
		}
		for _, user := range users {
			if user.ID == userID {
				if user.AvatarURL == nil {
					return ""
				}
				return *user.AvatarURL
			}
		}
		t.Fatalf("user %s missing from the directory", userID)
		return ""
	}

	if url := findAvatarURL(); url != "" {
		t.Fatalf("a user with no uploaded avatar should have no avatar URL, got %q", url)
	}

	firstAvatar := identity.UserAvatar{
		Content:     []byte("first-avatar-bytes"),
		ContentType: "image/png",
		UpdatedAt:   now.Add(time.Second),
	}
	if err := avatarStore.UpdateUserAvatar(ctx, userID, firstAvatar); err != nil {
		t.Fatalf("UpdateUserAvatar() error = %v", err)
	}
	firstURL := findAvatarURL()
	if !strings.Contains(firstURL, "/api/v1/users/"+userID+"/avatar?v=") {
		t.Fatalf("avatar URL %q does not carry a cache-busting version", firstURL)
	}

	secondAvatar := identity.UserAvatar{
		Content:     []byte("second-avatar-bytes"),
		ContentType: "image/png",
		UpdatedAt:   now.Add(2 * time.Second),
	}
	if err := avatarStore.UpdateUserAvatar(ctx, userID, secondAvatar); err != nil {
		t.Fatalf("UpdateUserAvatar() error = %v", err)
	}
	secondURL := findAvatarURL()
	if secondURL == firstURL {
		t.Errorf("avatar URL did not change after the avatar was replaced: %q", secondURL)
	}
}

func randomUUIDForTest() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func TestPostgresRegistration(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	store := NewRegistryStore(pool)
	provisioner := NewServiceDatabaseProvisioner(pool, "localhost", 5432)
	iconContent := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	iconServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/core/v1/health" {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"available":true}`))
			return
		}
		if request.URL.Path != "/api/core/v1/system/icon" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "image/svg+xml")
		response.Header().Set("ETag", `"integration-icon"`)
		_, _ = response.Write(iconContent)
	}))
	defer iconServer.Close()
	iconManager := registry.NewServiceIconManager(store, serviceclient.NewIconClient(2*time.Second))
	healthMonitor := registry.NewHealthMonitor(store, serviceclient.NewHealthClient(2*time.Second), nil)
	registrar := registry.NewRegistrar(store, provisioner, iconManager).WithHealthRefresher(healthMonitor)
	input := registry.RegistrationInput{
		ServiceType: "integration",
		InstanceID:  testUUID(t),
		Name:        "Integration Test Service",
		Version:     "0.1.0-test",
		InternalURL: iconServer.URL,
	}

	result, err := registrar.Register(ctx, input)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if err := provisioner.Deprovision(cleanupContext, result.Database); err != nil {
			t.Errorf("Deprovision() error = %v", err)
		}
		if err := store.Delete(cleanupContext, result.ServiceID); err != nil {
			t.Errorf("Delete() error = %v", err)
		}
	})

	service, found, err := store.FindByInstanceID(ctx, input.InstanceID)
	if err != nil || !found {
		t.Fatalf("FindByInstanceID() found = %v, error = %v", found, err)
	}
	if service.ProvisioningStatus != "registered" || service.DatabaseName != result.Database.Database {
		t.Fatalf("unexpected stored service: %#v", service)
	}
	if service.RegistrationStatus != "registered" || service.AvailabilityStatus != "online" || service.HealthCheckedAt == nil || result.Database.Database != "kaeru_integration" {
		t.Fatalf("unexpected registration lifecycle state: service = %#v, database = %#v", service, result.Database)
	}
	cachedIcon, err := store.GetServiceIcon(ctx, result.ServiceID)
	if err != nil {
		t.Fatalf("GetServiceIcon() after registration error = %v", err)
	}
	if string(cachedIcon.Content) != string(iconContent) || cachedIcon.ContentType != "image/svg+xml" || cachedIcon.SourceETag != `"integration-icon"` {
		t.Fatalf("registration did not cache the service icon: %#v", cachedIcon)
	}
	services, err := store.ListServices(ctx)
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	foundRegisteredService := false
	for _, listedService := range services {
		if listedService.ID == result.ServiceID {
			foundRegisteredService = true
			break
		}
	}
	if !foundRegisteredService {
		t.Fatalf("ListServices() did not include service %q", result.ServiceID)
	}
	roles, err := store.ListServiceRoles(ctx, result.ServiceID)
	if err != nil || len(roles) != 0 {
		t.Fatalf("ListServiceRoles() roles = %#v, error = %v", roles, err)
	}
	rolesRefreshedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if err := store.SyncServiceRoles(ctx, result.ServiceID, []registry.RoleDefinition{
		{Key: "viewer", Name: "Viewer", Priority: 10},
		{Key: "admin", Name: "Administrator", Priority: 100},
	}, rolesRefreshedAt); err != nil {
		t.Fatalf("SyncServiceRoles() error = %v", err)
	}

	manager := registry.NewServiceManager(store, nil, provisioner)
	defaultRoleKey := "viewer"
	updated, err := manager.Update(ctx, result.ServiceID, registry.UpdateServiceInput{
		PublicURL:      "https://upload.example.com",
		DefaultRoleKey: &defaultRoleKey,
		RoleMappings: []registry.ServiceRoleMapping{
			{RoleKey: "admin", OIDCGroups: []string{"administrators", "infrastructure"}},
			{RoleKey: "viewer", OIDCGroups: []string{"employees"}},
		},
	})
	if err != nil {
		t.Fatalf("Update() service configuration error = %v", err)
	}
	if updated.PublicURL != "https://upload.example.com" || updated.DefaultRoleKey == nil || *updated.DefaultRoleKey != defaultRoleKey {
		t.Fatalf("unexpected updated service: %#v", updated.Service)
	}
	if len(updated.Roles) != 2 || updated.Roles[0].Key != "admin" {
		t.Fatalf("unexpected updated roles: %#v", updated.Roles)
	}
	if updated.RoleCatalog.Status != "fresh" || updated.RoleCatalog.RefreshedAt == nil || !updated.RoleCatalog.RefreshedAt.Equal(rolesRefreshedAt) {
		t.Fatalf("unexpected updated role catalog: %#v", updated.RoleCatalog)
	}
	if len(updated.RoleMappings) != 2 || len(updated.RoleMappings[0].OIDCGroups) != 2 || updated.RoleMappings[1].OIDCGroups[0] != "employees" {
		t.Fatalf("unexpected updated role mappings: %#v", updated.RoleMappings)
	}
	if err := store.RecordRoleCatalogFailure(ctx, result.ServiceID, "service unavailable"); err != nil {
		t.Fatalf("RecordRoleCatalogFailure() error = %v", err)
	}
	cached, err := store.GetService(ctx, result.ServiceID)
	if err != nil || cached.RoleCatalog.Status != "cached" || !cached.RoleCatalog.Stale || len(cached.Roles) != 2 {
		t.Fatalf("cached role catalog = %#v, error = %v", cached, err)
	}
	cleared, err := manager.Update(ctx, result.ServiceID, registry.UpdateServiceInput{
		PublicURL:    "https://upload-new.example.com",
		RoleMappings: []registry.ServiceRoleMapping{},
	})
	if err != nil {
		t.Fatalf("Update() clearing service configuration error = %v", err)
	}
	if cleared.DefaultRoleKey != nil || len(cleared.RoleMappings) != 0 {
		t.Fatalf("service configuration was not cleared: %#v", cleared)
	}
	if err := store.SyncServiceRoles(ctx, result.ServiceID, []registry.RoleDefinition{
		{Key: "viewer", Name: "Read Only", Priority: 100},
	}, rolesRefreshedAt.Add(time.Hour)); err != nil {
		t.Fatalf("SyncServiceRoles() replacing catalog error = %v", err)
	}
	replaced, err := store.GetService(ctx, result.ServiceID)
	if err != nil {
		t.Fatalf("GetService() after role replacement error = %v", err)
	}
	if len(replaced.Roles) != 2 || replaced.Roles[0].Key != "viewer" || !replaced.Roles[0].Active || replaced.Roles[1].Key != "admin" || replaced.Roles[1].Active {
		t.Fatalf("roles were not synchronized correctly: %#v", replaced.Roles)
	}
	if replaced.RoleCatalog.Status != "fresh" || replaced.RoleCatalog.Stale {
		t.Fatalf("role catalog failure was not cleared: %#v", replaced.RoleCatalog)
	}

	authenticated, err := store.Authenticate(ctx, result.ServiceID, result.ServiceToken)
	if err != nil || !authenticated {
		t.Fatalf("Authenticate() authenticated = %v, error = %v", authenticated, err)
	}
	if authenticated, err := store.Authenticate(ctx, result.ServiceID, "kaeru_wrong"); err != nil || authenticated {
		t.Fatalf("wrong token authenticated = %v, error = %v", authenticated, err)
	}

	serviceURL := fmt.Sprintf(
		"postgresql://%s:%s@localhost:5432/%s?sslmode=disable",
		result.Database.Username,
		result.Database.Password,
		result.Database.Database,
	)
	servicePool, err := pgxpool.New(ctx, serviceURL)
	if err != nil {
		t.Fatalf("open provisioned service database: %v", err)
	}
	if err := servicePool.Ping(ctx); err != nil {
		t.Fatalf("ping provisioned service database: %v", err)
	}
	assertDatabaseConnectionDenied(t, ctx, result.Database, "kaeru")
	assertDatabaseConnectionDenied(t, ctx, result.Database, "postgres")

	if _, err := registrar.Register(ctx, input); !errors.Is(err, registry.ErrAlreadyRegistered) {
		t.Fatalf("duplicate Register() error = %v, want ErrAlreadyRegistered", err)
	}
	differentInstance := input
	differentInstance.InstanceID = testUUID(t)
	if _, err := registrar.Register(ctx, differentInstance); !errors.Is(err, registry.ErrServiceTypeRegistered) {
		t.Fatalf("duplicate service type Register() error = %v, want ErrServiceTypeRegistered", err)
	}

	servicePool.Close()
	unregistered, err := manager.Unregister(ctx, result.ServiceID)
	if err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if unregistered.RegistrationStatus != "unregistered" || unregistered.AvailabilityStatus != "offline" || unregistered.PublicURL != "https://upload-new.example.com" || len(unregistered.Roles) != 2 {
		t.Fatalf("unexpected unregistered service: %#v", unregistered)
	}
	if authenticated, err := store.Authenticate(ctx, result.ServiceID, result.ServiceToken); err != nil || authenticated {
		t.Fatalf("unregistered token authenticated = %v, error = %v", authenticated, err)
	}
	assertDatabaseCredentialsDenied(t, ctx, result.Database)

	reattached, err := registrar.Register(ctx, differentInstance)
	if err != nil {
		t.Fatalf("reattach Register() error = %v", err)
	}
	if reattached.Status != "reattached" || reattached.ServiceID != result.ServiceID {
		t.Fatalf("unexpected reattachment result: %#v", reattached)
	}
	if reattached.Database.Database != result.Database.Database || reattached.Database.Password == result.Database.Password {
		t.Fatalf("database identity or password was not rotated: old = %#v, new = %#v", result.Database, reattached.Database)
	}
	if authenticated, err := store.Authenticate(ctx, reattached.ServiceID, reattached.ServiceToken); err != nil || !authenticated {
		t.Fatalf("reattached token authenticated = %v, error = %v", authenticated, err)
	}
	if authenticated, err := store.Authenticate(ctx, result.ServiceID, result.ServiceToken); err != nil || authenticated {
		t.Fatalf("old token authenticated after reattachment = %v, error = %v", authenticated, err)
	}
	reattachedService, err := store.GetService(ctx, result.ServiceID)
	if err != nil {
		t.Fatalf("GetService() after reattachment error = %v", err)
	}
	if reattachedService.RegistrationStatus != "registered" || reattachedService.InstanceID != differentInstance.InstanceID || reattachedService.PublicURL != "https://upload-new.example.com" || len(reattachedService.Roles) != 2 {
		t.Fatalf("configuration was not retained across reattachment: %#v", reattachedService)
	}
	reattachedURL := fmt.Sprintf(
		"postgresql://%s:%s@localhost:5432/%s?sslmode=disable",
		reattached.Database.Username,
		reattached.Database.Password,
		reattached.Database.Database,
	)
	reattachedPool, err := pgxpool.New(ctx, reattachedURL)
	if err != nil {
		t.Fatalf("open reattached service database: %v", err)
	}
	defer reattachedPool.Close()
	if err := reattachedPool.Ping(ctx); err != nil {
		t.Fatalf("ping reattached service database: %v", err)
	}
}

func assertDatabaseCredentialsDenied(t *testing.T, ctx context.Context, credentials registry.DatabaseCredentials) {
	t.Helper()
	databaseURL := fmt.Sprintf(
		"postgresql://%s:%s@localhost:5432/%s?sslmode=disable",
		credentials.Username,
		credentials.Password,
		credentials.Database,
	)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("suspended service credentials unexpectedly connected")
	}
}

func assertDatabaseConnectionDenied(t *testing.T, ctx context.Context, credentials registry.DatabaseCredentials, databaseName string) {
	t.Helper()
	databaseURL := fmt.Sprintf(
		"postgresql://%s:%s@localhost:5432/%s?sslmode=disable",
		credentials.Username,
		credentials.Password,
		databaseName,
	)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err == nil {
		t.Fatalf("service role unexpectedly connected to database %q", databaseName)
	}
}

func testUUID(t *testing.T) string {
	t.Helper()
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate test UUID: %v", err)
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", random[0:4], random[4:6], random[6:8], random[8:10], random[10:16])
}

// Exercises the alternate URL rules against PostgreSQL: Kaeru Core owns the
// group list, every other service only supplies its own URL for those groups,
// and removing a group removes it everywhere.
func TestPostgresAlternateURLs(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	store := NewRegistryStore(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.EnsureCoreService(ctx, registry.NewCoreService("test", "http://kaeru-core:8080", now), now); err != nil {
		t.Fatalf("EnsureCoreService() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM alternate_url_groups`)
	})

	manager := registry.NewServiceManager(store, nil, nil)

	// Changing Core's administrator mappings needs OIDC verification, so every
	// update here carries the existing ones through untouched.
	existingCore, err := store.GetService(ctx, registry.CoreServiceID)
	if err != nil {
		t.Fatalf("GetService(core) error = %v", err)
	}
	coreMappings := existingCore.RoleMappings

	// Core defines two groups.
	core, err := manager.Update(ctx, registry.CoreServiceID, registry.UpdateServiceInput{
		PublicURL:    "https://core.example.com",
		RoleMappings: coreMappings,
		AlternateURLs: []registry.AlternateURLInput{
			{Group: "Native", URL: "https://core-native.example.com"},
			{Group: "LAN", URL: "https://core.lan"},
		},
	})
	if err != nil {
		t.Fatalf("Update(core) error = %v", err)
	}
	if len(core.AlternateURLs) != 2 {
		t.Fatalf("Core has %d alternate URLs, want 2", len(core.AlternateURLs))
	}
	groupIDs := map[string]int64{}
	for _, alternate := range core.AlternateURLs {
		if alternate.GroupID == 0 {
			t.Errorf("group %q was stored without an identifier", alternate.Group)
		}
		groupIDs[alternate.Group] = alternate.GroupID
	}

	// A registered service sees those groups with no URL of its own yet.
	serviceID := registerServiceForTest(ctx, t, store)
	service, err := store.GetService(ctx, serviceID)
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(service.AlternateURLs) != 2 {
		t.Fatalf("service sees %d groups, want 2", len(service.AlternateURLs))
	}
	for _, alternate := range service.AlternateURLs {
		if alternate.URL != "" {
			t.Errorf("group %q should start without a URL, got %q", alternate.Group, alternate.URL)
		}
	}

	// It supplies a URL for one group and leaves the other blank.
	updated, err := manager.Update(ctx, serviceID, registry.UpdateServiceInput{
		PublicURL: "https://upload.example.com",
		AlternateURLs: []registry.AlternateURLInput{
			{GroupID: groupIDs["Native"], URL: "https://upload-native.example.com"},
			{GroupID: groupIDs["LAN"]},
		},
	})
	if err != nil {
		t.Fatalf("Update(service) error = %v", err)
	}
	if got := updated.ResolveAlternateURL(groupIDs["Native"]); got != "https://upload-native.example.com" {
		t.Errorf("Native resolves to %q, want the service's own URL", got)
	}
	if got := updated.ResolveAlternateURL(groupIDs["LAN"]); got != "https://upload.example.com" {
		t.Errorf("LAN resolves to %q, want the public URL fallback", got)
	}

	// A service may not invent a group.
	if _, err := manager.Update(ctx, serviceID, registry.UpdateServiceInput{
		PublicURL:     "https://upload.example.com",
		AlternateURLs: []registry.AlternateURLInput{{Group: "Invented", URL: "https://x.example.com"}},
	}); err == nil {
		t.Error("a service other than Core created an alternate URL group")
	}

	// Core renames one group and drops the other.
	renamed, err := manager.Update(ctx, registry.CoreServiceID, registry.UpdateServiceInput{
		PublicURL:    "https://core.example.com",
		RoleMappings: coreMappings,
		AlternateURLs: []registry.AlternateURLInput{
			{GroupID: groupIDs["Native"], Group: "Mobile", URL: "https://core-native.example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Update(core rename) error = %v", err)
	}
	if len(renamed.AlternateURLs) != 1 || renamed.AlternateURLs[0].Group != "Mobile" {
		t.Fatalf("Core alternate URLs = %+v, want a single Mobile group", renamed.AlternateURLs)
	}

	// The rename follows through to the service, and the dropped group is gone
	// along with the URL that service had for it.
	after, err := store.GetService(ctx, serviceID)
	if err != nil {
		t.Fatalf("GetService() error = %v", err)
	}
	if len(after.AlternateURLs) != 1 {
		t.Fatalf("service sees %d groups after removal, want 1", len(after.AlternateURLs))
	}
	if after.AlternateURLs[0].Group != "Mobile" {
		t.Errorf("service sees group %q, want the renamed Mobile", after.AlternateURLs[0].Group)
	}
	if after.AlternateURLs[0].URL != "https://upload-native.example.com" {
		t.Errorf("renaming a group lost the service's URL: %q", after.AlternateURLs[0].URL)
	}

	// Duplicate group names are rejected by the database, not just the API.
	if _, err := manager.Update(ctx, registry.CoreServiceID, registry.UpdateServiceInput{
		PublicURL:    "https://core.example.com",
		RoleMappings: coreMappings,
		AlternateURLs: []registry.AlternateURLInput{
			{Group: "Duplicate"},
			{Group: "duplicate"},
		},
	}); err == nil {
		t.Error("Core created two alternate URL groups with the same name")
	}
}

// registerServiceForTest inserts a registered service directly, avoiding the
// PostgreSQL role and database provisioning the registrar would perform.
func registerServiceForTest(ctx context.Context, t *testing.T, store *RegistryStore) string {
	t.Helper()
	serviceID, err := randomUUIDForTest()
	if err != nil {
		t.Fatal(err)
	}
	instanceID, err := randomUUIDForTest()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.database.Exec(ctx, `
		INSERT INTO services (
			id, service_type, instance_id, name, version, internal_url, public_url,
			provisioning_status, registration_status, availability_status, created_at, last_seen_at
		) VALUES ($1, $2, $3, $4, '0.1.0', 'http://service:8080', 'https://upload.example.com',
			'registered', 'registered', 'online', $5, $5)
	`, serviceID, "alternate-url-test-"+serviceID[:8], instanceID, "Kaeru Upload", now); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = store.database.Exec(cleanupContext, `DELETE FROM services WHERE id = $1`, serviceID)
	})
	return serviceID
}
