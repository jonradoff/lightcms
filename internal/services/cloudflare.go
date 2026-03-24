package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CloudflareService purges cached content via the Cloudflare API.
// It reads credentials from a getter function on every call so that
// changes made through the admin settings UI take effect immediately
// without requiring a server restart.
type CloudflareService struct {
	getConfig func() (zoneID, apiToken string, enabled bool)
	baseURL   string
	client    *http.Client
}

// NewCloudflareService creates a CloudflareService that reads zone ID,
// API token, and enabled state from getConfig on each operation.
func NewCloudflareService(getConfig func() (zoneID, apiToken string, enabled bool), baseURL string) *CloudflareService {
	return &CloudflareService{
		getConfig: getConfig,
		baseURL:   baseURL,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

type cfResponse struct {
	Success bool `json:"success"`
}

// PurgeByURLs purges specific paths from the Cloudflare cache.
func (s *CloudflareService) PurgeByURLs(ctx context.Context, paths []string) error {
	zoneID, apiToken, enabled := s.getConfig()
	if !enabled || zoneID == "" || apiToken == "" {
		return nil
	}
	urls := make([]string, len(paths))
	for i, p := range paths {
		urls[i] = s.baseURL + p
	}
	body, err := json.Marshal(map[string]interface{}{"files": urls})
	if err != nil {
		return fmt.Errorf("cloudflare: marshal: %w", err)
	}
	return s.postPurge(ctx, zoneID, apiToken, body)
}

// PurgeEverything purges the entire Cloudflare cache for the zone.
func (s *CloudflareService) PurgeEverything(ctx context.Context) error {
	zoneID, apiToken, enabled := s.getConfig()
	if !enabled || zoneID == "" || apiToken == "" {
		return nil
	}
	body, err := json.Marshal(map[string]interface{}{"purge_everything": true})
	if err != nil {
		return fmt.Errorf("cloudflare: marshal: %w", err)
	}
	return s.postPurge(ctx, zoneID, apiToken, body)
}

// TestConnection verifies both the API token and Zone ID by calling the
// zone-specific details endpoint. This confirms the token has access to
// the configured zone, not just that the token itself is valid.
func (s *CloudflareService) TestConnection(ctx context.Context) error {
	zoneID, apiToken, _ := s.getConfig()
	if zoneID == "" || apiToken == "" {
		return fmt.Errorf("zone ID and API token must both be set")
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("cloudflare: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	switch resp.StatusCode {
	case http.StatusOK:
		if !result.Success {
			return fmt.Errorf("cloudflare: API returned success=false for zone lookup")
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("invalid or insufficient API token (status %d) — ensure the token has Cache Purge permission for this zone", resp.StatusCode)
	case http.StatusNotFound:
		return fmt.Errorf("zone ID not found (status 404) — verify the Zone ID is correct")
	default:
		return fmt.Errorf("unexpected status %d from Cloudflare", resp.StatusCode)
	}
}

// postPurge is the shared helper that POSTs to the purge_cache endpoint.
func (s *CloudflareService) postPurge(ctx context.Context, zoneID, apiToken string, body []byte) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cloudflare: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		return fmt.Errorf("cloudflare: purge returned status %d", resp.StatusCode)
	}

	var result cfResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("cloudflare: decode response: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("cloudflare: purge reported failure")
	}
	return nil
}
