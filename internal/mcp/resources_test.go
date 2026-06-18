package mcp

import (
	"context"
	"net/http"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExtractBearerToken(t *testing.T) {
	cases := map[string]string{
		"Bearer abc123": "abc123",
		"bearer xyz":    "xyz",
		"Basic foo":     "",
		"":              "",
		"Bearer":        "",
	}
	for header, want := range cases {
		r, _ := http.NewRequest("GET", "/", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		if got := extractBearerToken(r); got != want {
			t.Errorf("extractBearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestNewHTTPHandler(t *testing.T) {
	if NewHTTPHandler("8082") == nil {
		t.Error("NewHTTPHandler returned nil")
	}
}

func TestServerCard(t *testing.T) {
	b, err := ServerCard("http://localhost:8082")
	if err != nil {
		t.Fatalf("ServerCard: %v", err)
	}
	if len(b) == 0 {
		t.Error("ServerCard returned empty bytes")
	}
}

func TestResourceHandlers(t *testing.T) {
	s := permissiveServer(t)
	ctx := context.Background()
	req := &mcpsdk.ReadResourceRequest{Params: &mcpsdk.ReadResourceParams{}}

	if _, err := s.handleSiteStructure(ctx, req); err != nil {
		t.Errorf("handleSiteStructure: %v", err)
	}
	if _, err := s.handleRecentContent(ctx, req); err != nil {
		t.Errorf("handleRecentContent: %v", err)
	}
	if _, err := s.handleThemeConfig(ctx, req); err != nil {
		t.Errorf("handleThemeConfig: %v", err)
	}
}
