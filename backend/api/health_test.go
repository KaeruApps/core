package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}

	var body struct {
		Available       bool `json:"available"`
		Initialized     bool `json:"initialized"`
		DevelopmentMode bool `json:"development_mode"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Available {
		t.Fatal("expected Core to be available")
	}
	if body.Initialized {
		t.Fatal("expected Core to be uninitialized")
	}
	if body.DevelopmentMode {
		t.Fatal("expected development mode to be disabled")
	}
}

func TestHealthReportsDevelopmentModeAsInitialized(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, DevelopmentMode: true}).ServeHTTP(response, request)

	var body struct {
		Initialized     bool `json:"initialized"`
		DevelopmentMode bool `json:"development_mode"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Initialized || !body.DevelopmentMode {
		t.Fatalf("unexpected development health response: %#v", body)
	}
}
