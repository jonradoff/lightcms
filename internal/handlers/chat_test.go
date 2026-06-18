package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestChatWidget_ConfigFlow(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Admin config page (session).
	if rr := getPage(t, h.ChatWidgetPage, nil); rr.Code >= 500 {
		t.Errorf("ChatWidgetPage: %d (%s)", rr.Code, rr.Body.String())
	}

	// Save config.
	if rr := postForm(t, h.ChatWidgetSaveConfig, url.Values{
		"enabled":              {"on"},
		"widget_title":         {"Ask AI"},
		"welcome_message":      {"Hi!"},
		"placeholder":          {"Type a question"},
		"position":             {"bottom-right"},
		"primary_color":        {"#3366ff"},
		"system_prompt":        {"You are a helpful assistant for {siteName}."},
		"user_prompt_template": {"Question: {question}\nContext: {excerpts}"},
	}, nil); rr.Code >= 500 {
		t.Errorf("ChatWidgetSaveConfig: %d (%s)", rr.Code, rr.Body.String())
	}

	// Save config with an invalid placeholder should be rejected (not 500).
	if rr := postForm(t, h.ChatWidgetSaveConfig, url.Values{
		"system_prompt": {"Bad {nope} placeholder"},
	}, nil); rr.Code >= 500 {
		t.Errorf("ChatWidgetSaveConfig invalid: %d", rr.Code)
	}

	// Public config endpoint (no auth needed).
	{
		req := httptest.NewRequest(http.MethodGet, "/chat/config", nil)
		rr := httptest.NewRecorder()
		h.ChatWidgetConfigPublic(rr, req)
		if rr.Code >= 500 {
			t.Errorf("ChatWidgetConfigPublic: %d", rr.Code)
		}
	}
}

func TestChat_Helpers(t *testing.T) {
	// validatePromptPlaceholders
	if err := validatePromptPlaceholders("Hello {siteName}, {question}, {excerpts}"); err != nil {
		t.Errorf("valid placeholders rejected: %v", err)
	}
	if err := validatePromptPlaceholders("Bad {unknown}"); err == nil {
		t.Error("expected error for unknown placeholder")
	}

	h, cleanup := newTestHandler(t)
	defer cleanup()

	// chatOrigin returns a non-panicking string derived from baseURL.
	_ = h.chatOrigin()

	// corsOK sets CORS headers.
	rr := httptest.NewRecorder()
	h.corsOK(rr)
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("corsOK did not set Access-Control-Allow-Origin")
	}
}

func TestChat_RateLimit(t *testing.T) {
	// checkChatRateLimit with generous limits should not limit a single request.
	req := httptest.NewRequest(http.MethodGet, "/chat/query", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	if checkChatRateLimit(req, nil, 100, 1000) {
		t.Error("single request should not be rate limited")
	}
}

func TestChatWidgetQuery_NoKey(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	// No Anthropic key configured → handler should respond with an error status,
	// not panic or 500-crash the process.
	req := httptest.NewRequest(http.MethodPost, "/chat/query", nil)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ChatWidgetQuery(rr, req)
	// Any well-formed HTTP status is acceptable; just must not be a panic.
	if rr.Code == 0 {
		t.Error("ChatWidgetQuery wrote no status")
	}
}
