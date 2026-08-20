package serviceclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestIconClientFetchesSVG(t *testing.T) {
	client := &IconClient{httpClient: &http.Client{
		Timeout: time.Second,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/api/core/v1/system/icon" {
				t.Fatalf("path = %q", request.URL.Path)
			}
			headers := make(http.Header)
			headers.Set("Content-Type", "image/svg+xml")
			headers.Set("ETag", `"icon-v1"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     headers,
				Body:       io.NopCloser(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)),
			}, nil
		}),
	}}

	icon, err := client.Fetch(context.Background(), "http://mock-service:3101")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if icon.ContentType != "image/svg+xml" || icon.SourceETag != `"icon-v1"` {
		t.Fatalf("unexpected icon: %#v", icon)
	}
}

func TestIconClientRejectsInvalidContent(t *testing.T) {
	client := &IconClient{httpClient: &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/svg+xml"}},
				Body:       io.NopCloser(strings.NewReader(`<html></html>`)),
			}, nil
		}),
	}}

	if _, err := client.Fetch(context.Background(), "http://mock-service:3101"); err == nil {
		t.Fatal("Fetch() expected an error")
	}
}
