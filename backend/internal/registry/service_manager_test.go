package registry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubServiceConfigurationStore struct {
	service ServiceDetails
	updated UpdateServiceInput
	err     error
}

func (store *stubServiceConfigurationStore) ListServices(context.Context) ([]Service, error) {
	return []Service{store.service.Service}, store.err
}

func (store *stubServiceConfigurationStore) GetService(context.Context, string) (ServiceDetails, error) {
	return store.service, store.err
}

func (store *stubServiceConfigurationStore) UpdateService(_ context.Context, _ string, input UpdateServiceInput) (ServiceDetails, error) {
	store.updated = input
	return store.service, store.err
}

func (store *stubServiceConfigurationStore) SyncServiceRoles(_ context.Context, serviceID string, roles []RoleDefinition, refreshedAt time.Time) error {
	store.service.Roles = make([]ServiceRole, len(roles))
	for index, role := range roles {
		store.service.Roles[index] = ServiceRole{
			ServiceID: serviceID,
			Key:       role.Key,
			Name:      role.Name,
			Priority:  role.Priority,
			Active:    true,
		}
	}
	store.service.RoleCatalog = RoleCatalogState{Status: "fresh", RefreshedAt: &refreshedAt}
	return store.err
}

func (store *stubServiceConfigurationStore) RecordRoleCatalogFailure(_ context.Context, _ string, _ string) error {
	store.service.RoleCatalog.Stale = true
	if store.service.RoleCatalog.RefreshedAt == nil {
		store.service.RoleCatalog.Status = "unavailable"
	} else {
		store.service.RoleCatalog.Status = "cached"
	}
	return store.err
}

func (store *stubServiceConfigurationStore) UnregisterService(context.Context, string) error {
	if store.err != nil {
		return store.err
	}
	store.service.RegistrationStatus = "unregistered"
	return nil
}

type stubServiceDatabaseLifecycle struct {
	suspendCalls int
	resumeCalls  int
	err          error
}

func (lifecycle *stubServiceDatabaseLifecycle) Suspend(context.Context, DatabaseCredentials) error {
	lifecycle.suspendCalls++
	return lifecycle.err
}

func (lifecycle *stubServiceDatabaseLifecycle) Resume(context.Context, DatabaseCredentials) error {
	lifecycle.resumeCalls++
	return lifecycle.err
}

type stubRoleCatalogClient struct {
	roles []RoleDefinition
	err   error
	url   string
}

func (client *stubRoleCatalogClient) Fetch(_ context.Context, internalURL string) ([]RoleDefinition, error) {
	client.url = internalURL
	return client.roles, client.err
}

func TestServiceManagerUpdatesConfiguration(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{Roles: []ServiceRole{
		{Key: "viewer", Active: true},
		{Key: "admin", Active: true},
	}}}
	manager := NewServiceManager(store, nil, nil)
	defaultRole := "viewer"
	input := UpdateServiceInput{
		PublicURL:      "https://upload.example.com",
		DefaultRoleKey: &defaultRole,
		RoleMappings: []ServiceRoleMapping{
			{RoleKey: "admin", OIDCGroups: []string{"administrators", "infrastructure"}},
		},
	}

	if _, err := manager.Update(context.Background(), "service-id", input); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if store.updated.DefaultRoleKey == nil || *store.updated.DefaultRoleKey != defaultRole {
		t.Fatalf("unexpected stored update: %#v", store.updated)
	}
}

func TestServiceManagerListsServices(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{Service: Service{ID: "service-id"}}}
	services, err := NewServiceManager(store, nil, nil).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(services) != 1 || services[0].ID != "service-id" {
		t.Fatalf("unexpected services: %#v", services)
	}
}

func TestServiceManagerRefreshesRolesWhenGettingService(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{Service: Service{
		ID: "service-id", InternalURL: "http://mock-service:3101", RegistrationStatus: "registered",
	}}}
	client := &stubRoleCatalogClient{roles: []RoleDefinition{
		{Key: "viewer", Name: "Viewer", Priority: 10},
		{Key: "admin", Name: "Administrator", Priority: 100},
	}}
	manager := NewServiceManager(store, client, nil)
	refreshedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return refreshedAt }

	service, err := manager.Get(context.Background(), "service-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if client.url != store.service.InternalURL || len(service.Roles) != 2 || service.Roles[0].Key != "viewer" {
		t.Fatalf("unexpected refreshed service: %#v", service)
	}
	if service.RoleCatalog.Status != "fresh" || service.RoleCatalog.RefreshedAt == nil || !service.RoleCatalog.RefreshedAt.Equal(refreshedAt) {
		t.Fatalf("unexpected role catalog state: %#v", service.RoleCatalog)
	}
}

func TestServiceManagerFallsBackToCachedRoles(t *testing.T) {
	refreshedAt := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service:     Service{ID: "service-id", InternalURL: "http://offline-service:3101", RegistrationStatus: "registered"},
		Roles:       []ServiceRole{{Key: "viewer", Name: "Viewer", Priority: 10, Active: true}},
		RoleCatalog: RoleCatalogState{Status: "fresh", RefreshedAt: &refreshedAt},
	}}
	manager := NewServiceManager(store, &stubRoleCatalogClient{err: errors.New("offline")}, nil)

	service, err := manager.Get(context.Background(), "service-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(service.Roles) != 1 || service.RoleCatalog.Status != "cached" || !service.RoleCatalog.Stale {
		t.Fatalf("unexpected cached service: %#v", service)
	}
}

func TestServiceManagerUnregistersService(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{Service: Service{
		ID:                 "service-id",
		RegistrationStatus: "registered",
		DatabaseName:       "kaeru_upload",
		DatabaseUsername:   "kaeru_upload",
	}}}
	lifecycle := &stubServiceDatabaseLifecycle{}
	manager := NewServiceManager(store, nil, lifecycle)

	service, err := manager.Unregister(context.Background(), "service-id")
	if err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if service.RegistrationStatus != "unregistered" || lifecycle.suspendCalls != 1 {
		t.Fatalf("unexpected unregistered service: %#v, lifecycle: %#v", service, lifecycle)
	}
}

func TestServiceManagerDoesNotRefreshOrUnregisterCore(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service: Service{
			ID:                 CoreServiceID,
			ServiceType:        CoreServiceType,
			RegistrationStatus: "registered",
			InternalURL:        "http://localhost:3000",
		},
		Roles: []ServiceRole{{Key: CoreAdminRoleKey, Name: "Administrator", Priority: 100, Active: true}},
	}}
	client := &stubRoleCatalogClient{err: errors.New("Core must not fetch its own roles")}
	manager := NewServiceManager(store, client, &stubServiceDatabaseLifecycle{})

	service, err := manager.Get(context.Background(), CoreServiceID)
	if err != nil || len(service.Roles) != 1 || client.url != "" {
		t.Fatalf("Get() service = %#v, client URL = %q, error = %v", service, client.url, err)
	}
	if _, err := manager.Unregister(context.Background(), CoreServiceID); !errors.Is(err, ErrBuiltInService) {
		t.Fatalf("Unregister() error = %v, want ErrBuiltInService", err)
	}
}

func TestServiceManagerKeepsCoreDefaultAtNoAccess(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service: Service{ID: CoreServiceID, ServiceType: CoreServiceType},
		Roles:   []ServiceRole{{Key: CoreAdminRoleKey, Active: true}},
	}}
	manager := NewServiceManager(store, nil, nil)
	admin := CoreAdminRoleKey

	_, err := manager.Update(context.Background(), CoreServiceID, UpdateServiceInput{
		PublicURL:      "https://core.example.com",
		DefaultRoleKey: &admin,
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "default_role_key" {
		t.Fatalf("Update() error = %v, want default_role_key validation error", err)
	}
}

func TestServiceManagerNormalizesCorePublicURL(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service: Service{ID: CoreServiceID, ServiceType: CoreServiceType},
		Roles:   []ServiceRole{{Key: CoreAdminRoleKey, Active: true}},
	}}
	manager := NewServiceManager(store, nil, nil)

	if _, err := manager.Update(context.Background(), CoreServiceID, UpdateServiceInput{
		PublicURL: "https://core.example.com/",
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if store.updated.PublicURL != "https://core.example.com" {
		t.Fatalf("stored public URL = %q", store.updated.PublicURL)
	}
}

func TestServiceManagerRequiresVerificationForCoreAdministratorMappingChanges(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service: Service{ID: CoreServiceID, ServiceType: CoreServiceType},
		Roles:   []ServiceRole{{Key: CoreAdminRoleKey, Active: true}},
		RoleMappings: []ServiceRoleMapping{
			{RoleKey: CoreAdminRoleKey, OIDCGroups: []string{"kaeru-admins"}},
		},
	}}
	manager := NewServiceManager(store, nil, nil)

	_, err := manager.Update(context.Background(), CoreServiceID, UpdateServiceInput{
		PublicURL: "https://core.example.com",
		RoleMappings: []ServiceRoleMapping{
			{RoleKey: CoreAdminRoleKey, OIDCGroups: []string{"new-admins"}},
		},
	})
	if !errors.Is(err, ErrCoreAdminVerificationRequired) {
		t.Fatalf("Update() error = %v, want ErrCoreAdminVerificationRequired", err)
	}
	if store.updated.PublicURL != "" {
		t.Fatalf("UpdateService() was called with %#v", store.updated)
	}
}

func TestServiceManagerRejectsCorePublicURLPath(t *testing.T) {
	store := &stubServiceConfigurationStore{service: ServiceDetails{
		Service: Service{ID: CoreServiceID, ServiceType: CoreServiceType},
		Roles:   []ServiceRole{{Key: CoreAdminRoleKey, Active: true}},
	}}
	manager := NewServiceManager(store, nil, nil)

	_, err := manager.Update(context.Background(), CoreServiceID, UpdateServiceInput{
		PublicURL: "https://core.example.com/kaeru",
	})
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Field != "public_url" {
		t.Fatalf("Update() error = %v, want public_url validation error", err)
	}
}

func TestValidateServiceUpdateRejectsInvalidConfiguration(t *testing.T) {
	roles := []ServiceRole{{Key: "viewer", Active: true}, {Key: "retired", Active: false}}
	tests := []struct {
		name  string
		input UpdateServiceInput
		field string
	}{
		{name: "public URL", input: UpdateServiceInput{}, field: "public_url"},
		{name: "unknown default", input: updateInputWithDefault("admin"), field: "default_role_key"},
		{name: "inactive mapping", input: updateInputWithMapping("retired", "employees"), field: "role_mappings[0].role_key"},
		{name: "mapping without groups", input: updateInputWithMapping("viewer"), field: "role_mappings[0].oidc_groups"},
		{name: "empty group", input: updateInputWithMapping("viewer", ""), field: "role_mappings[0].oidc_groups[0]"},
		{name: "duplicate group", input: updateInputWithMapping("viewer", "employees", "employees"), field: "role_mappings[0].oidc_groups[1]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateServiceUpdate(test.input, roles)
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != test.field {
				t.Fatalf("ValidateServiceUpdate() error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestValidateRoleCatalogRejectsInvalidRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []RoleDefinition
	}{
		{name: "invalid key", roles: []RoleDefinition{{Key: "Invalid Role", Name: "Viewer", Priority: 10}}},
		{name: "empty name", roles: []RoleDefinition{{Key: "viewer", Name: "", Priority: 10}}},
		{name: "negative priority", roles: []RoleDefinition{{Key: "viewer", Name: "Viewer", Priority: -1}}},
		{name: "duplicate key", roles: []RoleDefinition{{Key: "viewer", Name: "Viewer", Priority: 10}, {Key: "viewer", Name: "Other", Priority: 20}}},
		{name: "duplicate priority", roles: []RoleDefinition{{Key: "viewer", Name: "Viewer", Priority: 10}, {Key: "admin", Name: "Admin", Priority: 10}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateRoleCatalog(test.roles); err == nil {
				t.Fatal("ValidateRoleCatalog() expected an error")
			}
		})
	}
}

func updateInputWithDefault(roleKey string) UpdateServiceInput {
	return UpdateServiceInput{PublicURL: "https://upload.example.com", DefaultRoleKey: &roleKey}
}

func updateInputWithMapping(roleKey string, groups ...string) UpdateServiceInput {
	return UpdateServiceInput{
		PublicURL:    "https://upload.example.com",
		RoleMappings: []ServiceRoleMapping{{RoleKey: roleKey, OIDCGroups: groups}},
	}
}
