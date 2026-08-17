package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KaeruApps/core/internal/identity"
	"github.com/KaeruApps/core/internal/installation"
)

type stubOIDCCallbackManager struct {
	result identity.OIDCCallbackResult
	err    error
}

func TestOIDCCallbackRedirectsSuccessfulSettingsVerification(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	request.AddCookie(&http.Cookie{Name: oidcSettingsVerificationCookieName, Value: "1"})
	response := httptest.NewRecorder()

	NewRouter(Dependencies{OIDCCallbackManager: stubOIDCCallbackManager{result: identity.OIDCCallbackResult{
		SessionToken: "new-session", ExpiresAt: time.Now().Add(time.Hour),
		Purpose: installation.OIDCLoginPurposeSettingsVerification,
	}}}).ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?oidc_verification=success" {
		t.Fatalf("unexpected verification redirect: status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
}

func TestOIDCCallbackRedirectsFailedSettingsVerification(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	request.AddCookie(&http.Cookie{Name: oidcSettingsVerificationCookieName, Value: "1"})
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, OIDCCallbackManager: stubOIDCCallbackManager{err: identity.ErrAdminGroupRequired}}).ServeHTTP(response, request)

	location, err := response.Result().Location()
	if err != nil || location.Path != "/" || location.Query().Get("oidc_verification") != "failed" {
		t.Fatalf("unexpected verification error redirect: location=%v error=%v", location, err)
	}
}

func TestOIDCCallbackDescribesFailedGroupsClaim(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	request.AddCookie(&http.Cookie{Name: oidcSettingsVerificationCookieName, Value: "1"})
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, OIDCCallbackManager: stubOIDCCallbackManager{
		err: &identity.OIDCClaimError{Claim: "custom_groups", Kind: "groups"},
	}}).ServeHTTP(response, request)

	location, err := response.Result().Location()
	if err != nil || location.Query().Get("error") != `The identity provider did not return a valid groups claim "custom_groups".` {
		t.Fatalf("unexpected claim error redirect: location=%v error=%v", location, err)
	}
}

func (manager stubOIDCCallbackManager) Complete(context.Context, string, string, string, string) (identity.OIDCCallbackResult, error) {
	return manager.result, manager.err
}

func TestOIDCCallbackSetsSessionCookieAndRedirects(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{OIDCCallbackManager: stubOIDCCallbackManager{result: identity.OIDCCallbackResult{
		SessionToken: "session-token",
		ExpiresAt:    expiresAt,
		SecureCookie: true,
	}}}).ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/" {
		t.Fatalf("unexpected callback response: status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].Value != "session-token" {
		t.Fatalf("unexpected callback cookies: %#v", cookies)
	}
	if !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie protections are missing: %#v", cookies[0])
	}
}

func TestOIDCCallbackRedirectsErrorsToSetup(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{OIDCCallbackManager: stubOIDCCallbackManager{err: identity.ErrAdminGroupRequired}}).ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, response.Code)
	}
	location, err := response.Result().Location()
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}
	if location.Path != "/setup/oidc" || location.Query().Get("error") == "" {
		t.Fatalf("unexpected error redirect: %s", location)
	}
}

func TestOIDCCallbackRejectsInvalidAttempt(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{OIDCCallbackManager: stubOIDCCallbackManager{err: errors.New("unexpected")}}).ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, response.Code)
	}
}

func TestOIDCCallbackRedirectsInitializedLoginErrorsToLogin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state=state&code=code", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{
		Initialized:         true,
		OIDCCallbackManager: stubOIDCCallbackManager{err: identity.ErrAdminGroupRequired},
	}).ServeHTTP(response, request)

	location, err := response.Result().Location()
	if err != nil {
		t.Fatalf("parse callback redirect: %v", err)
	}
	if location.Path != "/login" || location.Query().Get("error") == "" {
		t.Fatalf("unexpected initialized error redirect: %s", location)
	}
}
