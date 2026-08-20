package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/registry"
)

type stubBackupOptionsDirectory struct {
	entries []registry.ServiceBackupOptions
	err     error
}

func (stub stubBackupOptionsDirectory) List(context.Context) ([]registry.ServiceBackupOptions, error) {
	return stub.entries, stub.err
}

func backupOptionsRequest(t *testing.T, dependencies Dependencies) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/backup/options", nil)
	NewRouter(authenticatedTestDependencies(dependencies)).ServeHTTP(recorder, request)
	return recorder
}

func TestListBackupOptionsReturnsTheDirectory(t *testing.T) {
	directory := stubBackupOptionsDirectory{entries: []registry.ServiceBackupOptions{
		{
			ServiceID: registry.CoreServiceID, ServiceType: registry.CoreServiceType,
			ServiceName: registry.CoreServiceName, Available: true,
			Options: registry.CoreBackupOptions(),
		},
		{
			ServiceID: "upload", ServiceType: "upload", ServiceName: "Kaeru Upload",
			Available: false, Unavailable: "The service is offline.",
			Options: []registry.BackupOption{},
		},
	}}

	recorder := backupOptionsRequest(t, Dependencies{
		Initialized: true, DevelopmentMode: true, BackupOptionsDirectory: directory,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body []registry.ServiceBackupOptions
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("got %d entries, want 2", len(body))
	}
	if body[0].ServiceType != registry.CoreServiceType {
		t.Errorf("first entry = %q, want Kaeru Core", body[0].ServiceName)
	}
	if len(body[0].Options) != 1 || body[0].Options[0].Option != "Full Backup" {
		t.Errorf("Core options = %+v, want a single Full Backup option", body[0].Options)
	}
	if body[1].Available || body[1].Unavailable == "" {
		t.Errorf("offline service should report why it is unavailable: %+v", body[1])
	}
}

func TestListBackupOptionsRequiresAdministrator(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/backup/options", nil)
	NewRouter(Dependencies{
		Initialized:            true,
		BackupOptionsDirectory: stubBackupOptionsDirectory{},
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestListBackupOptionsReportsFailures(t *testing.T) {
	recorder := backupOptionsRequest(t, Dependencies{
		Initialized: true, DevelopmentMode: true,
		BackupOptionsDirectory: stubBackupOptionsDirectory{err: errors.New("boom")},
	})
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	unavailable := backupOptionsRequest(t, Dependencies{Initialized: true, DevelopmentMode: true})
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Errorf("status with no directory = %d, want %d", unavailable.Code, http.StatusServiceUnavailable)
	}
}
