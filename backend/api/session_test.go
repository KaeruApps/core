package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/identity"
)

type stubSessionLogoutManager struct {
	token string
}

func (manager *stubSessionLogoutManager) Logout(_ context.Context, token string) error {
	manager.token = token
	return nil
}

func TestDevelopmentSessionUsesSyntheticAdministrator(t *testing.T) {
	principal := identity.DevelopmentPrincipal()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{DevelopmentPrincipal: &principal, Initialized: true}).ServeHTTP(response, request)

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

	NewRouter(Dependencies{Initialized: true}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("GET session status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	manager := &stubSessionLogoutManager{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, SessionLogoutManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("POST session logout status = %d, body = %s", response.Code, response.Body.String())
	}
	if manager.token != "session-token" {
		t.Fatalf("logout token = %q, want session-token", manager.token)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout cookie = %#v, want expired %s cookie", cookies, sessionCookieName)
	}
}

func TestProtectedAPIRouteRequiresAuthentication(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("GET services status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCoreAdministrationRequiresAdministratorRole(t *testing.T) {
	principal := identity.Principal{
		ID: "user-id", Name: "User", ServiceRoles: map[string]string{"core": "user"},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, DevelopmentPrincipal: &principal}).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("GET services status = %d, want %d", response.Code, http.StatusForbidden)
	}
}
