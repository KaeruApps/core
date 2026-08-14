package database

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/KaeruApps/core/internal/registry"
	"github.com/KaeruApps/core/internal/serviceclient"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRegistration(t *testing.T) {
	databaseURL := os.Getenv("KAERU_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KAERU_TEST_DATABASE_URL is not set")
	}

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
		if request.URL.Path == "/api/v1/health" {
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"available":true}`))
			return
		}
		if request.URL.Path != "/api/v1/system/icon" {
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
	nativeAppsURL := "https://native.upload.example.com"
	defaultRoleKey := "viewer"
	updated, err := manager.Update(ctx, result.ServiceID, registry.UpdateServiceInput{
		PublicURL:      "https://upload.example.com",
		NativeAppsURL:  &nativeAppsURL,
		DefaultRoleKey: &defaultRoleKey,
		RoleMappings: []registry.ServiceRoleMapping{
			{RoleKey: "admin", OIDCGroups: []string{"administrators", "infrastructure"}},
			{RoleKey: "viewer", OIDCGroups: []string{"employees"}},
		},
	})
	if err != nil {
		t.Fatalf("Update() service configuration error = %v", err)
	}
	if updated.PublicURL != "https://upload.example.com" || updated.NativeAppsURL != nativeAppsURL || updated.DefaultRoleKey == nil || *updated.DefaultRoleKey != defaultRoleKey {
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
	if cleared.NativeAppsURL != "" || cleared.DefaultRoleKey != nil || len(cleared.RoleMappings) != 0 {
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
