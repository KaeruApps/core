package serviceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const maxHealthResponseBytes = 16 * 1024

type HealthClient struct {
	httpClient *http.Client
}

func NewHealthClient(timeout time.Duration) *HealthClient {
	return &HealthClient{httpClient: &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (client *HealthClient) Check(ctx context.Context, internalURL string) (bool, error) {
	endpoint, err := url.JoinPath(internalURL, "/api/core/v1/health")
	if err != nil {
		return false, fmt.Errorf("build service health URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("create service health request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("request service health: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("service health returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxHealthResponseBytes+1))
	if err != nil {
		return false, fmt.Errorf("read service health: %w", err)
	}
	if len(body) > maxHealthResponseBytes {
		return false, fmt.Errorf("service health response exceeds %d bytes", maxHealthResponseBytes)
	}

	var health struct {
		Available *bool `json:"available"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&health); err != nil {
		return false, fmt.Errorf("decode service health: %w", err)
	}
	if health.Available == nil {
		return false, fmt.Errorf("decode service health: available is required")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("decode service health: response must contain one JSON object")
	}

	return *health.Available, nil
}
