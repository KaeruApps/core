package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KaeruApps/core/internal/registry"
)

type stubServiceConfigurationManager struct {
	service   registry.ServiceDetails
	update    registry.UpdateServiceInput
	serviceID string
	err       error
}

func (manager *stubServiceConfigurationManager) Get(_ context.Context, serviceID string) (registry.ServiceDetails, error) {
	manager.serviceID = serviceID
	return manager.service, manager.err
}

func (manager *stubServiceConfigurationManager) List(context.Context) ([]registry.Service, error) {
	return []registry.Service{manager.service.Service}, manager.err
}

func (manager *stubServiceConfigurationManager) Update(_ context.Context, serviceID string, input registry.UpdateServiceInput) (registry.ServiceDetails, error) {
	manager.serviceID = serviceID
	manager.update = input
	return manager.service, manager.err
}

func (manager *stubServiceConfigurationManager) Unregister(_ context.Context, serviceID string) (registry.ServiceDetails, error) {
	manager.serviceID = serviceID
	return manager.service, manager.err
}

func TestGetService(t *testing.T) {
	manager := &stubServiceConfigurationManager{service: registry.ServiceDetails{
		Service: registry.Service{ID: "service-id", Name: "Upload Archiver"},
		Roles:   []registry.ServiceRole{{Key: "admin", Name: "Administrator", Priority: 100, Active: true}},
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/service-id", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusOK || manager.serviceID != "service-id" {
		t.Fatalf("GET service status = %d, service ID = %q", response.Code, manager.serviceID)
	}
	var service registry.ServiceDetails
	if err := json.NewDecoder(response.Body).Decode(&service); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(service.Roles) != 1 || service.Roles[0].Name != "Administrator" {
		t.Fatalf("unexpected response: %#v", service)
	}
}

func TestListServices(t *testing.T) {
	manager := &stubServiceConfigurationManager{service: registry.ServiceDetails{
		Service: registry.Service{ID: "service-id", Name: "Upload Archiver"},
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET services status = %d: %s", response.Code, response.Body.String())
	}
	var services []registry.Service
	if err := json.NewDecoder(response.Body).Decode(&services); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(services) != 1 || services[0].ID != "service-id" {
		t.Fatalf("unexpected response: %#v", services)
	}
}

func TestUpdateService(t *testing.T) {
	manager := &stubServiceConfigurationManager{service: registry.ServiceDetails{Service: registry.Service{ID: "service-id"}}}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/services/service-id", strings.NewReader(`{
		"public_url":"https://upload.example.com",
		"native_apps_url":null,
		"default_role_key":"viewer",
		"role_mappings":[{"role_key":"admin","oidc_groups":["administrators"]}]
	}`))
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("PUT service status = %d: %s", response.Code, response.Body.String())
	}
	if manager.update.DefaultRoleKey == nil || *manager.update.DefaultRoleKey != "viewer" {
		t.Fatalf("unexpected update: %#v", manager.update)
	}
	if len(manager.update.RoleMappings) != 1 || manager.update.RoleMappings[0].OIDCGroups[0] != "administrators" {
		t.Fatalf("unexpected mappings: %#v", manager.update.RoleMappings)
	}
}

func TestUnregisterService(t *testing.T) {
	manager := &stubServiceConfigurationManager{service: registry.ServiceDetails{Service: registry.Service{
		ID: "service-id", RegistrationStatus: "unregistered",
	}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/services/service-id/unregister", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusOK || manager.serviceID != "service-id" {
		t.Fatalf("POST unregister status = %d, service ID = %q", response.Code, manager.serviceID)
	}
	var service registry.ServiceDetails
	if err := json.NewDecoder(response.Body).Decode(&service); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if service.RegistrationStatus != "unregistered" {
		t.Fatalf("unexpected response: %#v", service)
	}
}

func TestUnregisterBuiltInServiceIsRejected(t *testing.T) {
	manager := &stubServiceConfigurationManager{err: registry.ErrBuiltInService}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/services/"+registry.CoreServiceID+"/unregister", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("POST unregister status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestServiceEndpointErrors(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		err        error
		wantStatus int
	}{
		{name: "get missing", method: http.MethodGet, err: registry.ErrServiceNotFound, wantStatus: http.StatusNotFound},
		{name: "update validation", method: http.MethodPut, err: &registry.ValidationError{Field: "public_url", Message: "is required"}, wantStatus: http.StatusBadRequest},
		{name: "update missing", method: http.MethodPut, err: registry.ErrServiceNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", method: http.MethodGet, err: errors.New("unexpected"), wantStatus: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := &stubServiceConfigurationManager{err: test.err}
			request := httptest.NewRequest(test.method, "/api/v1/services/service-id", strings.NewReader(`{}`))
			response := httptest.NewRecorder()
			NewRouter(Dependencies{ServiceConfigurationManager: manager}).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
