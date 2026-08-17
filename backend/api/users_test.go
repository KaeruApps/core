package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KaeruApps/core/internal/identity"
)

type stubUserDirectory struct {
	users []identity.UserSummary
	err   error
}

func (directory stubUserDirectory) List(context.Context) ([]identity.UserSummary, error) {
	return directory.users, directory.err
}

func TestListUsers(t *testing.T) {
	directory := stubUserDirectory{users: []identity.UserSummary{{
		ID: "user-id", Username: "frog", OIDCGroups: []string{"admins"},
		Access:            []identity.UserServiceAccess{{ServiceID: "core", ServiceName: "Kaeru Core", RoleKey: "admin", RoleName: "Administrator"}},
		RegisteredDevices: []identity.RegisteredDevice{},
	}}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	response := httptest.NewRecorder()

	NewRouter(authenticatedTestDependencies(Dependencies{
		Initialized:   true,
		UserDirectory: directory,
	})).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET users status = %d, body = %s", response.Code, response.Body.String())
	}
	var users []identity.UserSummary
	if err := json.NewDecoder(response.Body).Decode(&users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) != 1 || users[0].Username != "frog" || users[0].Access[0].RoleKey != "admin" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestListUsersRequiresCoreAdministrator(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{Initialized: true, UserDirectory: stubUserDirectory{}}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("GET users status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
