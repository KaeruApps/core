package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAbout(t *testing.T) {
	application := ApplicationInfo{
		Name:        "Kaeru Core",
		Version:     "0.1.0-test",
		Description: "The shared foundation for the self-hosted Kaeru Platform.",
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/about", nil)
	response := httptest.NewRecorder()

	NewRouter(Dependencies{ApplicationInfo: application}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("GET about status = %d", response.Code)
	}
	var body ApplicationInfo
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body != application {
		t.Fatalf("unexpected application information: %#v", body)
	}
}
