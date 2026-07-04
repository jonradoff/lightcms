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

const resendAPIURL = "https://api.resend.com/emails"

// EmailService sends outbound email via the Resend API (api.resend.com).
// It is intentionally dependency-free: one HTTPS POST with a Bearer key.
type EmailService struct {
	apiKey     string
	from       string
	endpoint   string // overridable in tests
	httpClient *http.Client
}

// NewEmailService creates an EmailService. apiKey and from come from
// config (resend_api_key / RESEND_API_KEY, email_from / EMAIL_FROM).
func NewEmailService(apiKey, from string) *EmailService {
	return &EmailService{
		apiKey:     apiKey,
		from:       from,
		endpoint:   resendAPIURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SetEndpoint overrides the Resend API URL (tests only).
func (s *EmailService) SetEndpoint(url string) { s.endpoint = url }

// Configured reports whether outbound email can be sent.
func (s *EmailService) Configured() bool {
	return s.apiKey != "" && s.from != ""
}

// From returns the configured sender address.
func (s *EmailService) From() string { return s.from }

// Send delivers one email via Resend. Returns the Resend message ID.
func (s *EmailService) Send(ctx context.Context, to, subject, html, text string) (string, error) {
	if !s.Configured() {
		return "", fmt.Errorf("email is not configured: set RESEND_API_KEY and EMAIL_FROM")
	}
	if to == "" {
		return "", fmt.Errorf("recipient address is required")
	}

	body, err := json.Marshal(map[string]interface{}{
		"from":    s.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
		"text":    text,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Message != "" {
			return "", fmt.Errorf("resend API: %s (status %d)", apiErr.Message, resp.StatusCode)
		}
		return "", fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &out)
	return out.ID, nil
}
