package serviceclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/KaeruApps/core/internal/registry"
)

const maxRoleCatalogBytes = 64 * 1024

type RoleCatalogClient struct {
	httpClient *http.Client
}

func NewRoleCatalogClient(timeout time.Duration) *RoleCatalogClient {
	return &RoleCatalogClient{httpClient: &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (client *RoleCatalogClient) Fetch(ctx context.Context, internalURL string) ([]registry.RoleDefinition, error) {
	endpoint, err := url.JoinPath(internalURL, "/api/v1/system/roles")
	if err != nil {
		return nil, fmt.Errorf("build service role catalog URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create service role catalog request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request service role catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service role catalog returned HTTP %d", response.StatusCode)
	}

	var catalog registry.RoleCatalogResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxRoleCatalogBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("decode service role catalog: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode service role catalog: response must contain one JSON object")
	}

	return catalog.Roles, nil
}
