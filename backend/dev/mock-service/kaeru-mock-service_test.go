package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoleCatalog(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/core/v1/system/roles", nil)
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
	request := httptest.NewRequest(http.MethodGet, "/api/core/v1/health", nil)
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

	healthRequest := httptest.NewRequest(http.MethodGet, "/api/core/v1/health", nil)
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
	request := httptest.NewRequest(http.MethodGet, "/api/core/v1/system/icon", nil)
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

func TestBackupOptionsArePublished(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/core/v1/backup/options", nil)
	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var catalog backupOptionCatalog
	if err := json.Unmarshal(recorder.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode backup options: %v", err)
	}
	if len(catalog.Options) != len(mockBackupOptions) {
		t.Fatalf("published %d options, want %d", len(catalog.Options), len(mockBackupOptions))
	}
	defaults := 0
	for _, option := range catalog.Options {
		if option.Default {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("published %d default options, want exactly 1", defaults)
	}
}

func TestBackupRequiresAKnownOption(t *testing.T) {
	handler := newHandler()
	for _, target := range []string{
		"/api/core/v1/backup",
		"/api/core/v1/backup?option=",
		"/api/core/v1/backup?option=abc",
		"/api/core/v1/backup?option=999",
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/core/v1/backup?option=2", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "Database Only") {
		t.Errorf("backup body does not name the requested option: %q", recorder.Body.String())
	}
}

func TestBackupCanBeStarted(t *testing.T) {
	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/core/v1/backup?option=1", nil))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}
