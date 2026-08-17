package api

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/registry"
)

type stubServiceIconManager struct {
	icon      registry.ServiceIcon
	serviceID string
	err       error
}

func (manager *stubServiceIconManager) Get(_ context.Context, serviceID string) (registry.ServiceIcon, error) {
	manager.serviceID = serviceID
	return manager.icon, manager.err
}

func TestGetServiceIcon(t *testing.T) {
	content := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	manager := &stubServiceIconManager{icon: registry.ServiceIcon{
		Content: content, ContentType: "image/svg+xml", ContentHash: sha256.Sum256(content),
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/service-id/icon", nil)
	response := httptest.NewRecorder()

	NewRouter(authenticatedTestDependencies(Dependencies{ServiceIconManager: manager, Initialized: true})).ServeHTTP(response, request)

	if response.Code != http.StatusOK || manager.serviceID != "service-id" {
		t.Fatalf("GET icon status = %d, service ID = %q", response.Code, manager.serviceID)
	}
	if response.Header().Get("Content-Type") != "image/svg+xml" || response.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("unexpected icon headers: %#v", response.Header())
	}
	if response.Body.String() != string(content) {
		t.Fatalf("unexpected icon body: %q", response.Body.String())
	}
}

func TestGetMissingServiceIcon(t *testing.T) {
	manager := &stubServiceIconManager{err: registry.ErrServiceIconNotFound}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/service-id/icon", nil)
	response := httptest.NewRecorder()

	NewRouter(authenticatedTestDependencies(Dependencies{ServiceIconManager: manager, Initialized: true})).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("GET icon status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
