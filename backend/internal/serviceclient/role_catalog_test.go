package serviceclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testClient(handler roundTripFunc) *RoleCatalogClient {
	return &RoleCatalogClient{httpClient: &http.Client{Timeout: time.Second, Transport: handler}}
}

func TestRoleCatalogClientFetchesRoles(t *testing.T) {
	client := testClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/core/v1/system/roles" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"roles":[{"key":"viewer","name":"Viewer","priority":10}]}`)),
		}, nil
	})

	roles, err := client.Fetch(context.Background(), "http://mock-service:3101")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(roles) != 1 || roles[0].Key != "viewer" {
		t.Fatalf("unexpected roles: %#v", roles)
	}
}

func TestRoleCatalogClientRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "status", status: http.StatusServiceUnavailable, body: `{}`},
		{name: "unknown field", status: http.StatusOK, body: `{"roles":[],"unexpected":true}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := testClient(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: test.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(test.body)),
				}, nil
			})

			if _, err := client.Fetch(context.Background(), "http://mock-service:3101"); err == nil {
				t.Fatal("Fetch() expected an error")
			}
		})
	}
}
