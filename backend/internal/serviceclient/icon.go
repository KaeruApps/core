package serviceclient

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"

	"github.com/KaeruApps/core/internal/registry"
)

const maxServiceIconBytes = 256 * 1024

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

type IconClient struct {
	httpClient *http.Client
}

func NewIconClient(timeout time.Duration) *IconClient {
	return &IconClient{httpClient: &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (client *IconClient) Fetch(ctx context.Context, internalURL string) (registry.FetchedServiceIcon, error) {
	endpoint, err := url.JoinPath(internalURL, "/api/v1/system/icon")
	if err != nil {
		return registry.FetchedServiceIcon{}, fmt.Errorf("build service icon URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return registry.FetchedServiceIcon{}, fmt.Errorf("create service icon request: %w", err)
	}
	request.Header.Set("Accept", "image/png, image/svg+xml")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return registry.FetchedServiceIcon{}, fmt.Errorf("request service icon: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return registry.FetchedServiceIcon{}, fmt.Errorf("service icon returned HTTP %d", response.StatusCode)
	}

	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (contentType != "image/png" && contentType != "image/svg+xml") {
		return registry.FetchedServiceIcon{}, fmt.Errorf("service icon has unsupported content type %q", response.Header.Get("Content-Type"))
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxServiceIconBytes+1))
	if err != nil {
		return registry.FetchedServiceIcon{}, fmt.Errorf("read service icon: %w", err)
	}
	if len(content) == 0 || len(content) > maxServiceIconBytes {
		return registry.FetchedServiceIcon{}, fmt.Errorf("service icon must contain between 1 and %d bytes", maxServiceIconBytes)
	}
	if err := validateIconContent(contentType, content); err != nil {
		return registry.FetchedServiceIcon{}, err
	}

	return registry.FetchedServiceIcon{
		Content:     content,
		ContentType: contentType,
		SourceETag:  response.Header.Get("ETag"),
	}, nil
}

func validateIconContent(contentType string, content []byte) error {
	switch contentType {
	case "image/png":
		if !bytes.HasPrefix(content, pngSignature) {
			return fmt.Errorf("service icon is not a valid PNG")
		}
	case "image/svg+xml":
		decoder := xml.NewDecoder(bytes.NewReader(content))
		for {
			token, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("service icon is not valid SVG: %w", err)
			}
			if start, ok := token.(xml.StartElement); ok {
				if start.Name.Local != "svg" {
					return fmt.Errorf("service icon root element must be svg")
				}
				return nil
			}
		}
	}
	return nil
}
