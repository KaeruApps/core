package serviceclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHealthClientChecksAvailability(t *testing.T) {
	client := &HealthClient{httpClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/core/v1/health" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"available":true,"initialized":false}`)),
		}, nil
	})}}

	available, err := client.Check(context.Background(), "http://mock-service:3101")
	if err != nil || !available {
		t.Fatalf("Check() available = %v, error = %v", available, err)
	}
}

func TestHealthClientRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "status", status: http.StatusServiceUnavailable, body: `{"available":false}`},
		{name: "missing available", status: http.StatusOK, body: `{}`},
		{name: "multiple objects", status: http.StatusOK, body: `{"available":true}{"available":true}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &HealthClient{httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: test.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(test.body)),
				}, nil
			})}}
			if _, err := client.Check(context.Background(), "http://mock-service:3101"); err == nil {
				t.Fatal("Check() expected an error")
			}
		})
	}
}
