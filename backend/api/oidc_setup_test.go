package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/KaeruApps/core/internal/installation"
)

type stubOIDCSetupManager struct {
	input  installation.OIDCSetupInput
	result installation.OIDCAuthorization
	err    error
}

func (manager *stubOIDCSetupManager) Start(_ context.Context, input installation.OIDCSetupInput) (installation.OIDCAuthorization, error) {
	manager.input = input
	return manager.result, manager.err
}

func TestConfigureOIDC(t *testing.T) {
	manager := &stubOIDCSetupManager{result: installation.OIDCAuthorization{
		AuthorizationURL: "https://identity.example.com/authorize?state=state",
		ExpiresAt:        time.Date(2026, 8, 14, 12, 10, 0, 0, time.UTC),
	}}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	fields := map[string]string{
		"name":               "KyleAuth",
		"public_url":         "https://core.example.com",
		"issuer_url":         "https://identity.example.com",
		"client_id":          "kaeru-core",
		"client_secret":      "secret",
		"additional_scopes":  "groups custom",
		"username_claim":     "preferred_username",
		"display_name_claim": "name",
		"avatar_claim":       "picture",
		"groups_claim":       "groups",
		"admin_groups":       "kaeru-admins, platform-admins",
		"button_text":        "Sign in",
	}
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close multipart form: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/setup/oidc", body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	response := httptest.NewRecorder()
	NewRouter(Dependencies{OIDCSetupManager: manager}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if manager.input.ClientID != "kaeru-core" || manager.input.DisplayNameClaim != "name" || len(manager.input.AdditionalScopes) != 2 || len(manager.input.AdminGroups) != 2 {
		t.Fatalf("unexpected manager input: %#v", manager.input)
	}
	var result installation.OIDCAuthorization
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.AuthorizationURL != manager.result.AuthorizationURL {
		t.Fatalf("unexpected response: %#v", result)
	}
}

func TestConfigureOIDCRejectsInitializedInstallation(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/setup/oidc", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, OIDCSetupManager: &stubOIDCSetupManager{}}).ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, response.Code)
	}
}
