package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// callPublicTool invokes a tool on the public MCP server over an in-memory transport.
func callPublicTool(t *testing.T, ps *PublicServer, tool string, args interface{}) string {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	go ps.MCPServer().Run(ctx, serverTransport)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "t", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	b, _ := json.Marshal(args)
	var argsMap map[string]interface{}
	json.Unmarshal(b, &argsMap)
	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: tool, Arguments: argsMap})
	if err != nil {
		t.Fatalf("call %s: %v", tool, err)
	}
	var out strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			out.WriteString(tc.Text)
		}
	}
	return out.String()
}

func TestPublicMCPServer(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	now := time.Now()
	pub := &models.Content{
		ID: primitive.NewObjectID(), Title: "Public Page", Slug: "pub",
		FullPath: "/pub", MetaDescription: "A public page.", Published: true,
		PublishedAt: &now, PlainText: "Hello from the public page body.",
		CreatedAt: now, UpdatedAt: now,
	}
	draft := &models.Content{
		ID: primitive.NewObjectID(), Title: "Secret Draft", Slug: "draft",
		FullPath: "/draft", Published: false, CreatedAt: now, UpdatedAt: now,
	}
	ctx := context.Background()
	for _, c := range []*models.Content{pub, draft} {
		if _, err := db.InsertOne(ctx, "content", c); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	search := services.NewSearchService(db, "")
	ps := NewPublicServer(db, search, "https://example.com/")

	// get_site_info
	out := callPublicTool(t, ps, "get_site_info", struct{}{})
	if !strings.Contains(out, "llms.txt") || !strings.Contains(out, "https://example.com") {
		t.Errorf("get_site_info: %s", out)
	}

	// list_pages: published only
	out = callPublicTool(t, ps, "list_pages", struct{}{})
	if !strings.Contains(out, "/pub") {
		t.Errorf("list_pages missing published page: %s", out)
	}
	if strings.Contains(out, "Secret Draft") {
		t.Errorf("list_pages leaked a draft: %s", out)
	}

	// get_page: full content for published, refusal for draft
	out = callPublicTool(t, ps, "get_page", map[string]string{"path": "/pub"})
	if !strings.Contains(out, "Hello from the public page body") {
		t.Errorf("get_page: %s", out)
	}
	out = callPublicTool(t, ps, "get_page", map[string]string{"path": "/draft"})
	if !strings.Contains(out, "no published page") {
		t.Errorf("get_page should refuse drafts: %s", out)
	}
	// path without leading slash is normalized
	out = callPublicTool(t, ps, "get_page", map[string]string{"path": "pub"})
	if !strings.Contains(out, "Public Page") {
		t.Errorf("get_page without slash: %s", out)
	}
}

func TestPublicMCPServer_SearchSite(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	now := time.Now()
	ctx := context.Background()
	pages := []*models.Content{
		{
			ID: primitive.NewObjectID(), Title: "Griffin Handbook", Slug: "griffin",
			FullPath: "/griffin", Published: true, PublishedAt: &now,
			PlainText: "All about griffins and their care.",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Title: "Hidden Draft", Slug: "hidden",
			FullPath: "/hidden", Published: false,
			PlainText: "griffin secrets", CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, c := range pages {
		if _, err := db.InsertOne(ctx, "content", c); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	search := services.NewSearchService(db, "")
	ps := NewPublicServer(db, search, "https://example.com")

	// Default limit.
	out := callPublicTool(t, ps, "search_site", map[string]interface{}{"query": "griffin"})
	if !strings.Contains(out, "/griffin") {
		t.Errorf("search_site missing published match: %s", out)
	}
	if strings.Contains(out, "/hidden") {
		t.Errorf("search_site leaked a draft: %s", out)
	}

	// Limit above the cap is clamped (>25) — still returns results.
	out = callPublicTool(t, ps, "search_site", map[string]interface{}{"query": "griffin", "limit": 30})
	if !strings.Contains(out, "/griffin") {
		t.Errorf("search_site with clamped limit: %s", out)
	}

	// No matches → empty array, not an error.
	out = callPublicTool(t, ps, "search_site", map[string]interface{}{"query": "zzz-no-such-term"})
	if !strings.Contains(out, "[]") {
		t.Errorf("search_site no-match should return empty list: %s", out)
	}

	// Streamable HTTP handler is mountable and serves requests.
	h := ps.Handler()
	if h == nil {
		t.Fatal("Handler returned nil")
	}
	hs := httptest.NewServer(h)
	defer hs.Close()
	resp, err := http.Get(hs.URL)
	if err != nil {
		t.Fatalf("GET public MCP handler: %v", err)
	}
	resp.Body.Close()
}
