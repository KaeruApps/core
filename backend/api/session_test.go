package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/identity"
)

func TestDevelopmentSessionUsesSyntheticAdministrator(t *testing.T) {
	principal := identity.DevelopmentPrincipal()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{DevelopmentPrincipal: &principal}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET session status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Authenticated bool               `json:"authenticated"`
		User          identity.Principal `json:"user"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Authenticated || body.User.ServiceRoles["core"] != "admin" {
		t.Fatalf("unexpected development session: %#v", body)
	}
}

func TestSessionRequiresAuthenticationOutsideDevelopmentMode(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("GET session status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
