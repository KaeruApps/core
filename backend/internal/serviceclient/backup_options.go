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

const maxBackupOptionsBytes = 64 * 1024

// BackupOptionsClient reads the backup kinds a service offers. Core does not
// know what any service can back up, so each service publishes its own list.
type BackupOptionsClient struct {
	httpClient *http.Client
}

func NewBackupOptionsClient(timeout time.Duration) *BackupOptionsClient {
	return &BackupOptionsClient{httpClient: &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (client *BackupOptionsClient) Fetch(ctx context.Context, internalURL string) ([]registry.BackupOption, error) {
	endpoint, err := url.JoinPath(internalURL, "/api/core/v1/backup/options")
	if err != nil {
		return nil, fmt.Errorf("build service backup options URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create service backup options request: %w", err)
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request service backup options: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service backup options returned HTTP %d", response.StatusCode)
	}

	var options registry.BackupOptionsResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBackupOptionsBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&options); err != nil {
		return nil, fmt.Errorf("decode service backup options: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode service backup options: response must contain one JSON object")
	}

	return options.Options, nil
}
