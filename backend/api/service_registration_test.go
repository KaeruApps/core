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

type stubServiceRegistrar struct {
	result registry.RegistrationResult
	err    error
	input  registry.RegistrationInput
}

func (registrar *stubServiceRegistrar) Register(_ context.Context, input registry.RegistrationInput) (registry.RegistrationResult, error) {
	registrar.input = input
	return registrar.result, registrar.err
}

func TestRegisterService(t *testing.T) {
	registrar := &stubServiceRegistrar{result: registry.RegistrationResult{
		Status: "created", ServiceID: "service-id", ServiceToken: "kaeru_secret",
		Database: registry.DatabaseCredentials{Host: "kaeru-postgres", Port: 5432},
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/internal/services/register", strings.NewReader(`{
		"service_type":"upload",
		"instance_id":"019c2a46-7f5d-7ca2-9f4a-ae191ca84322",
		"name":"Upload Archiver",
		"version":"0.1.0",
		"internal_url":"http://kaeru-upload:8080"
	}`))
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceRegistrar: registrar, Initialized: true}).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if registrar.input.ServiceType != "upload" {
		t.Fatalf("unexpected registration input: %#v", registrar.input)
	}
	var result registry.RegistrationResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.ServiceToken != registrar.result.ServiceToken {
		t.Fatalf("unexpected response: %#v", result)
	}
}

func TestRegisterServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "validation", err: &registry.ValidationError{Field: "name", Message: "is required"}, wantStatus: http.StatusBadRequest, wantCode: "validation_failed"},
		{name: "duplicate", err: registry.ErrAlreadyRegistered, wantStatus: http.StatusConflict, wantCode: "service_already_registered"},
		{name: "duplicate service type", err: registry.ErrServiceTypeRegistered, wantStatus: http.StatusConflict, wantCode: "service_type_already_registered"},
		{name: "provisioning", err: registry.ErrProvisioningUnavailable, wantStatus: http.StatusServiceUnavailable, wantCode: "database_provisioning_unavailable"},
		{name: "internal", err: errors.New("unexpected"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registrar := &stubServiceRegistrar{err: test.err}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/internal/services/register", strings.NewReader(`{}`))
			response := httptest.NewRecorder()

			NewRouter(Dependencies{ServiceRegistrar: registrar, Initialized: true}).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, response.Code)
			}
			var responseError apiError
			if err := json.NewDecoder(response.Body).Decode(&responseError); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if responseError.Error.Code != test.wantCode {
				t.Fatalf("expected error code %q, got %q", test.wantCode, responseError.Error.Code)
			}
		})
	}
}

func TestRegisterServiceRejectsRoles(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/internal/services/register", strings.NewReader(`{"roles":[]}`))
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceRegistrar: &stubServiceRegistrar{}, Initialized: true}).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestRegisterServiceWaitsForInitialization(t *testing.T) {
	registrar := &stubServiceRegistrar{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/internal/services/register", strings.NewReader(`{}`))
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ServiceRegistrar: registrar}).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
	if retryAfter := response.Header().Get("Retry-After"); retryAfter != "15" {
		t.Fatalf("expected Retry-After 15, got %q", retryAfter)
	}
	var responseError apiError
	if err := json.NewDecoder(response.Body).Decode(&responseError); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseError.Error.Code != "core_not_initialized" {
		t.Fatalf("expected core_not_initialized, got %q", responseError.Error.Code)
	}
	if registrar.input.ServiceType != "" {
		t.Fatal("registrar should not be called before Core is initialized")
	}
}
