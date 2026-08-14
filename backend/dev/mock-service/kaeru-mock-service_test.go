package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoleCatalog(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/roles", nil)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var catalog roleCatalog
	if err := json.NewDecoder(response.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(catalog.Roles) != 3 || catalog.Roles[2].Key != "admin" || catalog.Roles[2].Priority != 100 {
		t.Fatalf("unexpected role catalog: %#v", catalog)
	}
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestAvailabilityCanBeToggled(t *testing.T) {
	handler := newHandler()
	toggleRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/dev/availability",
		bytes.NewBufferString(`{"available":false}`),
	)
	toggleResponse := httptest.NewRecorder()
	handler.ServeHTTP(toggleResponse, toggleRequest)
	if toggleResponse.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, body = %s", toggleResponse.Code, toggleResponse.Body.String())
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, healthRequest)
	var health struct {
		Available bool `json:"available"`
	}
	if err := json.NewDecoder(healthResponse.Body).Decode(&health); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if health.Available {
		t.Fatal("expected mock service to report unavailable")
	}
}

func TestIcon(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system/icon", nil)
	response := httptest.NewRecorder()

	newHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", contentType)
	}
	if response.Body.Len() < 1000 {
		t.Fatalf("icon response is unexpectedly small: %d bytes", response.Body.Len())
	}
}
