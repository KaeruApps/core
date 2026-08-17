package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/installation"
)

type stubInstallationState struct {
	state installation.State
	err   error
}

func (stub stubInstallationState) State(context.Context) (installation.State, error) {
	return stub.state, stub.err
}

func TestSetupStatus(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{
		InstallationState: stubInstallationState{state: installation.StateConfiguring},
	}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var body struct {
		State       installation.State `json:"state"`
		Initialized bool               `json:"initialized"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != installation.StateConfiguring || body.Initialized {
		t.Fatalf("unexpected setup status: %#v", body)
	}
}

func TestSetupStatusDevelopmentOverride(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, DevelopmentMode: true}).ServeHTTP(response, request)

	var body struct {
		State           installation.State `json:"state"`
		Initialized     bool               `json:"initialized"`
		DevelopmentMode bool               `json:"development_mode"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != installation.StateReady || !body.Initialized || !body.DevelopmentMode {
		t.Fatalf("unexpected development setup status: %#v", body)
	}
}

func TestUninitializedInstallationRejectsAdministrativeAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
	var responseError apiError
	if err := json.NewDecoder(response.Body).Decode(&responseError); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseError.Error.Code != "core_not_initialized" {
		t.Fatalf("expected core_not_initialized, got %q", responseError.Error.Code)
	}
}
