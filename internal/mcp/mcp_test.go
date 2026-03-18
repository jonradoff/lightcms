package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lightcms/internal/apiclient"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// testAPI sets up an httptest.Server that simulates the LightCMS REST API,
// an apiclient.Client pointed at it, and an MCP Server wrapping that client.
// The returned cleanup function must be called with defer.
func testAPI(t *testing.T) (*Server, *httptest.Server, func()) {
	t.Helper()

	now := time.Now().UTC().Truncate(time.Second)
	nowStr := now.Format(time.RFC3339)
	pubAt := now.Add(-time.Hour)
	pubAtStr := pubAt.Format(time.RFC3339)

	mux := http.NewServeMux()

	// ---------- Content ----------

	mux.HandleFunc("GET /api/v1/content", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id": "c1", "title": "Page One", "slug": "page-one",
				"full_path": "/page-one", "category": "", "published": true,
				"deleted": false, "updated_at": nowStr, "published_at": pubAtStr,
				"data": map[string]interface{}{"body": "<p>hello</p>"},
			},
			{
				"id": "c2", "title": "Page Two", "slug": "page-two",
				"full_path": "/page-two", "category": "blog", "published": false,
				"deleted": false, "updated_at": nowStr,
				"data": map[string]interface{}{"body": "<p>draft</p>"},
			},
		})
	})

	mux.HandleFunc("GET /api/v1/content/by-path", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "/page-one" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "c1", "title": "Page One", "slug": "page-one",
				"full_path": "/page-one", "published": true,
				"updated_at": nowStr, "created_at": nowStr,
				"data": map[string]interface{}{"body": "<p>hello</p>"},
			})
		} else {
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})

	mux.HandleFunc("GET /api/v1/content/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "c1" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "c1", "title": "Page One", "slug": "page-one",
				"full_path": "/page-one", "published": true,
				"updated_at": nowStr, "created_at": nowStr,
				"data": map[string]interface{}{"body": "<p>hello</p>"},
			})
		} else {
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})

	mux.HandleFunc("POST /api/v1/content", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "c3", "title": req["title"], "slug": req["slug"],
			"full_path": "/" + fmt.Sprintf("%v", req["slug"]),
			"published": false, "updated_at": nowStr, "created_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/content/by-path", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "c1", "title": "Updated Title", "slug": "page-one",
			"full_path": "/page-one", "published": true,
			"updated_at": nowStr, "created_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/content/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": id, "title": "Updated", "slug": "page-one",
			"full_path": "/page-one", "published": true,
			"updated_at": nowStr, "created_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/content/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/content/{id}/publish", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/content/{id}/unpublish", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/content/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("GET /api/v1/content/{id}/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"version": 1, "title": "Page One v1", "slug": "page-one", "full_path": "/page-one", "published": false, "created_at": nowStr},
			{"version": 2, "title": "Page One v2", "slug": "page-one", "full_path": "/page-one", "published": true, "created_at": nowStr},
		})
	})

	mux.HandleFunc("GET /api/v1/content/{id}/versions/{ver}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "title": "Page One v1", "slug": "page-one",
			"full_path": "/page-one", "published": false, "created_at": nowStr,
			"data": map[string]interface{}{"body": "<p>old</p>"},
		})
	})

	mux.HandleFunc("POST /api/v1/content/{id}/versions/{ver}/revert", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/content/batch-publish", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"published": []string{"c1", "c2"}, "failed": []interface{}{},
		})
	})

	mux.HandleFunc("POST /api/v1/content/{id}/preview", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content_id": r.PathValue("id"), "rendered_html": "<html>preview</html>", "warnings": []string{},
		})
	})

	mux.HandleFunc("POST /api/v1/content/bulk-update", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 2, "succeeded": 2, "failed": 0,
		})
	})

	mux.HandleFunc("POST /api/v1/content/bulk-field-op", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1, "succeeded": 1, "failed": 0,
		})
	})

	mux.HandleFunc("POST /api/v1/content/export", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 1, "items": []interface{}{},
		})
	})

	mux.HandleFunc("GET /api/v1/content/backlinks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "c2", "title": "Page Two", "full_path": "/page-two"},
		})
	})

	// ---------- Templates ----------

	mux.HandleFunc("GET /api/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"id": "t1", "name": "Blog Post", "slug": "blog-post",
				"description": "Blog template", "category": "blog",
				"is_system": true, "fields": []map[string]interface{}{{"name": "body"}},
				"created_at": nowStr, "updated_at": nowStr,
			},
		})
	})

	mux.HandleFunc("GET /api/v1/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "t1", "name": "Blog Post", "slug": "blog-post",
			"description": "Blog template", "category": "blog",
			"is_system": true, "html_layout": "<div>{{.body}}</div>",
			"fields": []map[string]interface{}{
				{"name": "body", "label": "Body", "type": "richtext", "required": true},
			},
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("POST /api/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "t2", "name": req["name"], "slug": req["slug"],
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "Updated Template", "slug": "updated",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/templates/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Assets ----------

	mux.HandleFunc("GET /api/v1/assets/folders", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]string{"/images", "/docs"})
	})

	mux.HandleFunc("GET /api/v1/assets/by-path", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "a1", "filename": "logo.png", "serve_path": "/images/logo.png",
			"mime_type": "image/png", "size": 1024,
		})
	})

	mux.HandleFunc("GET /api/v1/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "filename": "logo.png",
			"serve_path": "/images/logo.png", "mime_type": "image/png", "size": 1024,
		})
	})

	mux.HandleFunc("GET /api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "a1", "filename": "logo.png", "serve_path": "/images/logo.png", "mime_type": "image/png", "size": 1024},
		})
	})

	mux.HandleFunc("POST /api/v1/assets/from-url", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "a2", "filename": "photo.jpg", "serve_path": "/images/photo.jpg",
			"mime_type": "image/jpeg", "size": 2048,
		})
	})

	mux.HandleFunc("POST /api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "a2", "filename": "test.png", "serve_path": "/images/test.png",
			"mime_type": "image/png", "size": 512,
		})
	})

	mux.HandleFunc("DELETE /api/v1/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Theme ----------

	mux.HandleFunc("GET /api/v1/theme/versions/{ver}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version": 1, "site_name": "Test Site", "primary_color": "#000",
			"created_at": nowStr,
		})
	})

	mux.HandleFunc("POST /api/v1/theme/versions/{ver}/revert", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/theme/versions/{ver}/pin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/v1/theme/versions/{ver}/unpin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.HandleFunc("GET /api/v1/theme/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"version": 1, "site_name": "Test v1", "created_at": nowStr},
			{"version": 2, "site_name": "Test v2", "created_at": nowStr},
		})
	})

	mux.HandleFunc("GET /api/v1/theme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"primary_color": "#123456", "site_name": "Test Site",
			"font_family": "Inter", "heading_font": "Georgia",
		})
	})

	mux.HandleFunc("PUT /api/v1/theme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"primary_color": "#654321", "site_name": "Updated Site",
		})
	})

	// ---------- Site Config ----------

	mux.HandleFunc("GET /api/v1/config", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title_template": "{{title}} | {{site_name}}",
			"title_template_no_title": "{{site_name}}",
		})
	})

	mux.HandleFunc("PUT /api/v1/config", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title_template": "{{title}} - {{site_name}}",
		})
	})

	// ---------- Redirects ----------

	mux.HandleFunc("GET /api/v1/redirects", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "r1", "from_path": "/old", "to_path": "/new", "status_code": 301, "created_at": nowStr, "updated_at": nowStr},
		})
	})

	mux.HandleFunc("POST /api/v1/redirects", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "r2", "from_path": "/src", "to_path": "/dst", "status_code": 301,
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/redirects/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "from_path": "/updated", "to_path": "/dst",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/redirects/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Folders ----------

	mux.HandleFunc("GET /api/v1/folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "Blog", "slug": "blog", "path": "/blog",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("GET /api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "f1", "name": "Blog", "slug": "blog", "path": "/blog", "created_at": nowStr, "updated_at": nowStr},
		})
	})

	mux.HandleFunc("POST /api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "f2", "name": "Docs", "slug": "docs", "path": "/docs",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Collections ----------

	mux.HandleFunc("GET /api/v1/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "Blog Posts", "slug": "blog-posts",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("GET /api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "col1", "name": "Blog Posts", "slug": "blog-posts", "created_at": nowStr, "updated_at": nowStr},
		})
	})

	mux.HandleFunc("POST /api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "col2", "name": "New Collection", "slug": "new-coll",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "Updated", "slug": "updated",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Search ----------

	mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": r.URL.Query().Get("q"), "search_type": "fulltext",
			"total": 1, "matches": []map[string]interface{}{
				{"id": "c1", "title": "Page One", "full_path": "/page-one", "published": true, "matched_in": []string{"title"}},
			},
		})
	})

	mux.HandleFunc("POST /api/v1/search-replace/scoped/preview", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new", "total_matches": 3, "affected_pages": 1,
		})
	})

	mux.HandleFunc("POST /api/v1/search-replace/scoped/execute", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "search": "old", "replace": "new", "total_replacements": 3, "pages_updated": 1,
		})
	})

	mux.HandleFunc("POST /api/v1/search-replace/preview", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new", "total_matches": 5, "affected_pages": 2,
		})
	})

	mux.HandleFunc("POST /api/v1/search-replace/execute", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "search": "old", "replace": "new", "total_replacements": 5, "pages_updated": 2,
		})
	})

	mux.HandleFunc("GET /api/v1/end-user-search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": r.URL.Query().Get("q"), "mode": "hybrid", "total": 1,
			"results": []map[string]interface{}{
				{"title": "Page One", "path": "/page-one", "snippet": "hello world"},
			},
		})
	})

	mux.HandleFunc("POST /api/v1/reindex-embeddings", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"indexed": 5, "status": "ok",
		})
	})

	// ---------- Snippets ----------

	mux.HandleFunc("GET /api/v1/snippets/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "cta-box", "html": "<div>CTA</div>",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("GET /api/v1/snippets", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "s1", "name": "cta-box", "html": "<div>CTA</div>", "created_at": nowStr, "updated_at": nowStr},
		})
	})

	mux.HandleFunc("POST /api/v1/snippets", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "s2", "name": req["name"], "html": req["html"],
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("PUT /api/v1/snippets/{id}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": r.PathValue("id"), "name": "updated-snippet", "html": "<div>Updated</div>",
			"created_at": nowStr, "updated_at": nowStr,
		})
	})

	mux.HandleFunc("DELETE /api/v1/snippets/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// ---------- Regenerate ----------

	mux.HandleFunc("POST /api/v1/regenerate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	ts := httptest.NewServer(mux)
	client := apiclient.New(ts.URL, "test-api-key")
	server := NewServer(client)

	return server, ts, func() { ts.Close() }
}

// callTool is a test helper that invokes an MCP tool via the in-memory
// transport and returns the result.
func callTool(t *testing.T, s *Server, toolName string, args interface{}) *mcpsdk.CallToolResult {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	go s.MCPServer().Run(ctx, serverTransport)

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name: "test-client", Version: "0.0.1",
	}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	// Marshal args into map for the request
	argsBytes, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	var argsMap map[string]interface{}
	if err := json.Unmarshal(argsBytes, &argsMap); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: argsMap,
	})
	if err != nil {
		t.Fatalf("call tool %s: %v", toolName, err)
	}

	return result
}

// callToolExpectError is like callTool but expects an error from the SDK (e.g.
// schema validation failure for missing required args). Returns true if error occurred.
func callToolExpectError(t *testing.T, s *Server, toolName string, args interface{}) bool {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	go s.MCPServer().Run(ctx, serverTransport)

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name: "test-client", Version: "0.0.1",
	}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	argsBytes, _ := json.Marshal(args)
	var argsMap map[string]interface{}
	json.Unmarshal(argsBytes, &argsMap)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: argsMap,
	})
	if err != nil {
		return true // SDK validation error — expected
	}
	return result.IsError
}

// resultText extracts the text content from a CallToolResult.
func resultText(t *testing.T, r *mcpsdk.CallToolResult) string {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("result has no content")
	}
	tc, ok := r.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", r.Content[0])
	}
	return tc.Text
}

// resultJSON unmarshals the text content of a CallToolResult into dest.
func resultJSON(t *testing.T, r *mcpsdk.CallToolResult, dest interface{}) {
	t.Helper()
	text := resultText(t, r)
	if err := json.Unmarshal([]byte(text), dest); err != nil {
		t.Fatalf("unmarshal result JSON: %v\nraw: %s", err, text)
	}
}

// ============================================================
// Tests
// ============================================================

func TestNewServer(t *testing.T) {
	client := apiclient.New("http://localhost:0", "test")
	s := NewServer(client)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.mcpServer == nil {
		t.Fatal("mcpServer is nil")
	}
	if s.client == nil {
		t.Fatal("client is nil")
	}
}

func TestNewServer_RegistersTools(t *testing.T) {
	client := apiclient.New("http://localhost:0", "test")
	s := NewServer(client)

	ctx := context.Background()
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	go s.MCPServer().Run(ctx, serverTransport)

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	if len(res.Tools) == 0 {
		t.Fatal("no tools registered")
	}

	// Check that key tools are present
	toolNames := make(map[string]bool)
	for _, tool := range res.Tools {
		toolNames[tool.Name] = true
	}

	expected := []string{
		"list_content", "get_content", "create_content", "update_content",
		"delete_content", "publish_content", "unpublish_content",
		"list_templates", "get_template", "create_template",
		"list_assets", "get_asset",
		"get_theme", "update_theme", "get_site_config",
		"search_content", "end_user_search",
		"list_snippets", "create_snippet",
	}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("expected tool %q not found", name)
		}
	}
}

// ---------- Content tools ----------

func TestListContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_content", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)

	if resp["total"].(float64) != 2 {
		t.Errorf("expected total=2, got %v", resp["total"])
	}
	if resp["published"].(float64) != 1 {
		t.Errorf("expected published=1, got %v", resp["published"])
	}
}

func TestListContent_WithCategory(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_content", map[string]interface{}{
		"category": "blog",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	// Just verify no error; the mock returns all content regardless of filter
}

func TestGetContent_ByID(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_content", map[string]interface{}{
		"id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["id"] != "c1" {
		t.Errorf("expected id=c1, got %v", resp["id"])
	}
	if resp["title"] != "Page One" {
		t.Errorf("expected title='Page One', got %v", resp["title"])
	}
}

func TestGetContent_ByPath(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_content", map[string]interface{}{
		"path": "/page-one",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["id"] != "c1" {
		t.Errorf("expected id=c1, got %v", resp["id"])
	}
}

func TestGetContent_MissingArgs(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_content", map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error for missing id/path")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "either id or path is required") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestCreateContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_content", map[string]interface{}{
		"template_id":     "t1",
		"title":           "New Page",
		"slug":            "new-page",
		"data":            map[string]interface{}{"body": "<p>Hello</p>"},
		"version_comment": "Initial creation",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
	if resp["id"] != "c3" {
		t.Errorf("expected id=c3, got %v", resp["id"])
	}
}

func TestUpdateContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_content", map[string]interface{}{
		"id":              "c1",
		"title":           "Updated Title",
		"version_comment": "Changed title",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestUpdateContent_WithData(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	// This will first GET the content (to merge data), then PUT
	result := callTool(t, s, "update_content", map[string]interface{}{
		"id":              "c1",
		"data":            map[string]interface{}{"body": "<p>Updated</p>"},
		"version_comment": "Updated body",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestUpdateContent_DryRun(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_content", map[string]interface{}{
		"id":      "c1",
		"title":   "New Title",
		"dry_run": true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["dry_run"] != true {
		t.Errorf("expected dry_run=true")
	}
	if resp["valid"] != true {
		t.Errorf("expected valid=true")
	}
}

func TestDeleteContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_content", map[string]interface{}{
		"id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "deleted successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestPublishContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "publish_content", map[string]interface{}{
		"id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "published successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestUnpublishContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "unpublish_content", map[string]interface{}{
		"id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "unpublished successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestRestoreContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "restore_content", map[string]interface{}{
		"id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "restored successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestGetContentVersions(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_content_versions", map[string]interface{}{
		"content_id": "c1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp []map[string]interface{}
	resultJSON(t, result, &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 versions, got %d", len(resp))
	}
}

func TestGetContentVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_content_version", map[string]interface{}{
		"content_id": "c1",
		"version":    1,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestRevertToVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "revert_to_version", map[string]interface{}{
		"content_id":      "c1",
		"version":         1,
		"version_comment": "Reverting to v1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "reverted to version 1") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestPublishMultiple(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "publish_multiple", map[string]interface{}{
		"ids": []string{"c1", "c2"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestPreviewContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "preview_content", map[string]interface{}{
		"id":   "c1",
		"data": map[string]interface{}{"body": "<p>Preview text</p>"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["rendered_html"] != "<html>preview</html>" {
		t.Errorf("unexpected rendered_html: %v", resp["rendered_html"])
	}
}

func TestUpdateContentByPath(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_content_by_path", map[string]interface{}{
		"path":            "/page-one",
		"title":           "Updated Title",
		"version_comment": "Updated via path",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestGetBacklinks(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_backlinks", map[string]interface{}{
		"path": "/page-one",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["count"].(float64) != 1 {
		t.Errorf("expected count=1, got %v", resp["count"])
	}
}

func TestGetBacklinks_MissingPath(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	if !callToolExpectError(t, s, "get_backlinks", map[string]interface{}{}) {
		t.Fatal("expected error for missing path")
	}
}

// ---------- Template tools ----------

func TestListTemplates(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_templates", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp []map[string]interface{}
	resultJSON(t, result, &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 template, got %d", len(resp))
	}
	if resp[0]["name"] != "Blog Post" {
		t.Errorf("expected name='Blog Post', got %v", resp[0]["name"])
	}
}

func TestGetTemplate_ByID(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_template", map[string]interface{}{
		"id": "t1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["name"] != "Blog Post" {
		t.Errorf("expected name='Blog Post', got %v", resp["name"])
	}
}

func TestGetTemplate_BySlug(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_template", map[string]interface{}{
		"slug": "blog-post",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestGetTemplate_MissingArgs(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_template", map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error for missing id/slug")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "either id or slug is required") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestCreateTemplate(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_template", map[string]interface{}{
		"name": "Custom Template",
		"slug": "custom",
		"fields": []map[string]interface{}{
			{"name": "body", "label": "Body", "type": "richtext", "required": true},
		},
		"html_layout": "<div>{{.body}}</div>",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
	if resp["id"] != "t2" {
		t.Errorf("expected id=t2, got %v", resp["id"])
	}
}

func TestUpdateTemplate(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_template", map[string]interface{}{
		"id":   "t1",
		"name": "Updated Blog",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestDeleteTemplate(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_template", map[string]interface{}{
		"id": "t1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "deleted successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

// ---------- Asset tools ----------

func TestListAssets(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_assets", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestListAssets_WithFolder(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_assets", map[string]interface{}{
		"folder": "/images",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestListAssetFolders(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_asset_folders", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp []string
	resultJSON(t, result, &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 folders, got %d", len(resp))
	}
}

func TestGetAsset_ByID(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_asset", map[string]interface{}{
		"id": "a1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestGetAsset_ByPath(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_asset", map[string]interface{}{
		"path": "/images/logo.png",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestGetAsset_MissingArgs(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_asset", map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error for missing id/path")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "either id or path is required") {
		t.Errorf("unexpected error message: %s", text)
	}
}

func TestUploadAsset(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "upload_asset", map[string]interface{}{
		"filename":    "test.png",
		"serve_path":  "/images/test.png",
		"data_base64": "aVZCT1J3MEtHZ29BQUFBTlN=",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestUploadAssetFromURL(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "upload_asset_from_url", map[string]interface{}{
		"url":        "https://example.com/photo.jpg",
		"serve_path": "/images/photo.jpg",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestDeleteAsset(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_asset", map[string]interface{}{
		"id": "a1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "deleted successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

// ---------- Theme / Settings tools ----------

func TestGetTheme(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_theme", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["primary_color"] != "#123456" {
		t.Errorf("expected primary_color=#123456, got %v", resp["primary_color"])
	}
	if resp["site_name"] != "Test Site" {
		t.Errorf("expected site_name='Test Site', got %v", resp["site_name"])
	}
}

func TestUpdateTheme(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_theme", map[string]interface{}{
		"primary_color": "#654321",
		"site_name":     "Updated Site",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Theme updated successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestGetThemeVersions(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_theme_versions", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp []map[string]interface{}
	resultJSON(t, result, &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 versions, got %d", len(resp))
	}
}

func TestGetThemeVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_theme_version", map[string]interface{}{
		"version": 1,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestRevertThemeToVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "revert_theme_to_version", map[string]interface{}{
		"version": 1,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "reverted to version 1") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestPinThemeVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "pin_theme_version", map[string]interface{}{
		"version": 1,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "pinned") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestUnpinThemeVersion(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "unpin_theme_version", map[string]interface{}{
		"version": 1,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "unpinned") {
		t.Errorf("unexpected message: %s", text)
	}
}

func TestGetSiteConfig(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_site_config", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["title_template"] != "{{title}} | {{site_name}}" {
		t.Errorf("unexpected title_template: %v", resp["title_template"])
	}
}

func TestUpdateSiteConfig(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_site_config", map[string]interface{}{
		"title_template": "{{title}} - {{site_name}}",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Site config updated") {
		t.Errorf("unexpected message: %s", text)
	}
}

// ---------- Redirect tools ----------

func TestListRedirects(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_redirects", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestCreateRedirect(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_redirect", map[string]interface{}{
		"from_path":   "/src",
		"to_path":     "/dst",
		"status_code": 301,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestUpdateRedirect(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_redirect", map[string]interface{}{
		"id":        "r1",
		"from_path": "/updated",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestDeleteRedirect(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_redirect", map[string]interface{}{
		"id": "r1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

// ---------- Folder tools ----------

func TestListFolders(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_folders", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestCreateFolder(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_folder", map[string]interface{}{
		"name": "Docs",
		"slug": "docs",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestGetFolder(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_folder", map[string]interface{}{
		"id": "f1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestDeleteFolder(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_folder", map[string]interface{}{
		"id": "f1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

// ---------- Collection tools ----------

func TestListCollections(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_collections", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestCreateCollection(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_collection", map[string]interface{}{
		"name": "New Collection",
		"slug": "new-coll",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestGetCollection(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_collection", map[string]interface{}{
		"id": "col1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestUpdateCollection(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_collection", map[string]interface{}{
		"id":   "col1",
		"name": "Updated",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestDeleteCollection(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_collection", map[string]interface{}{
		"id": "col1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

// ---------- Search tools ----------

func TestSearchContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "search_content", map[string]interface{}{
		"query": "Page",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
}

func TestSearchContent_MissingQuery(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	if !callToolExpectError(t, s, "search_content", map[string]interface{}{}) {
		t.Fatal("expected error for missing query")
	}
}

func TestSearchReplacePreview(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "search_replace_preview", map[string]interface{}{
		"search":  "old",
		"replace": "new",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["total_matches"].(float64) != 5 {
		t.Errorf("expected total_matches=5, got %v", resp["total_matches"])
	}
}

func TestSearchReplacePreview_MissingSearch(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	if !callToolExpectError(t, s, "search_replace_preview", map[string]interface{}{
		"replace": "new",
	}) {
		t.Fatal("expected error for missing search")
	}
}

func TestSearchReplaceExecute(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "search_replace_execute", map[string]interface{}{
		"search":          "old",
		"replace":         "new",
		"version_comment": "Bulk replace",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestSearchReplaceExecute_MissingSearch(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	if !callToolExpectError(t, s, "search_replace_execute", map[string]interface{}{
		"replace": "new",
	}) {
		t.Fatal("expected error for missing search")
	}
}

func TestScopedSearchReplacePreview(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "scoped_search_replace_preview", map[string]interface{}{
		"search":      "old",
		"replace":     "new",
		"folder_path": "/blog",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestScopedSearchReplaceExecute(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "scoped_search_replace_execute", map[string]interface{}{
		"search":          "old",
		"replace":         "new",
		"folder_path":     "/blog",
		"version_comment": "Scoped replace",
		"auto_republish":  true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestEndUserSearch(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "end_user_search", map[string]interface{}{
		"query": "hello",
		"mode":  "hybrid",
		"limit": 5,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["total"].(float64) != 1 {
		t.Errorf("expected total=1, got %v", resp["total"])
	}
}

func TestEndUserSearch_MissingQuery(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	if !callToolExpectError(t, s, "end_user_search", map[string]interface{}{}) {
		t.Fatal("expected error for missing query")
	}
}

func TestReindexEmbeddings(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "reindex_embeddings", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

// ---------- Snippet tools ----------

func TestListSnippets(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "list_snippets", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestGetSnippet(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "get_snippet", map[string]interface{}{
		"id": "s1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestCreateSnippet(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "create_snippet", map[string]interface{}{
		"name": "footer-cta",
		"html": "<div class=\"cta\">Sign up!</div>",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
	if resp["id"] != "s2" {
		t.Errorf("expected id=s2, got %v", resp["id"])
	}
}

func TestUpdateSnippet(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "update_snippet", map[string]interface{}{
		"id":   "s1",
		"name": "updated-snippet",
		"html": "<div>Updated</div>",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

func TestDeleteSnippet(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "delete_snippet", map[string]interface{}{
		"id": "s1",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true")
	}
}

// ---------- Utility tools ----------

func TestRegenerateAllContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "regenerate_all_content", map[string]interface{}{})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "regenerated successfully") {
		t.Errorf("unexpected message: %s", text)
	}
}

// ---------- Bulk operations ----------

func TestBulkUpdateContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "bulk_update_content", map[string]interface{}{
		"updates": []map[string]interface{}{
			{"id": "c1", "title": "Bulk Updated 1"},
			{"id": "c2", "title": "Bulk Updated 2"},
		},
		"version_comment": "Bulk update",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}

	var resp map[string]interface{}
	resultJSON(t, result, &resp)
	if resp["succeeded"].(float64) != 2 {
		t.Errorf("expected succeeded=2, got %v", resp["succeeded"])
	}
}

func TestBulkFieldOperation(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "bulk_field_operation", map[string]interface{}{
		"operation":     "prepend",
		"field":         "body",
		"value":         "<p>Note: </p>",
		"template_name": "Blog Post",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

func TestExportContent(t *testing.T) {
	s, _, cleanup := testAPI(t)
	defer cleanup()

	result := callTool(t, s, "export_content", map[string]interface{}{
		"template_name": "Blog Post",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
}

// ---------- Error propagation from API ----------

func TestAPIError_Propagation(t *testing.T) {
	// Create a server that always returns 500
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := apiclient.New(ts.URL, "test")
	server := NewServer(client)

	result := callTool(t, server, "list_content", map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error result for 500 response")
	}
	text := resultText(t, result)
	if !strings.Contains(text, "Error") {
		t.Errorf("expected error prefix, got: %s", text)
	}
}

// ---------- Helper function tests ----------

func TestTextResult(t *testing.T) {
	r := textResult("hello")
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(r.Content))
	}
	tc, ok := r.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent")
	}
	if tc.Text != "hello" {
		t.Errorf("expected 'hello', got %q", tc.Text)
	}
	if r.IsError {
		t.Error("textResult should not be error")
	}
}

func TestErrorResult(t *testing.T) {
	r := errorResult(fmt.Errorf("something failed"))
	if !r.IsError {
		t.Error("errorResult should have IsError=true")
	}
	tc, ok := r.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if !strings.Contains(tc.Text, "something failed") {
		t.Errorf("expected error message, got %q", tc.Text)
	}
}

func TestJsonResult(t *testing.T) {
	data := map[string]string{"key": "value"}
	r := jsonResult(data)
	if r.IsError {
		t.Error("jsonResult should not be error")
	}
	tc, ok := r.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Fatalf("failed to parse JSON result: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value, got %v", parsed["key"])
	}
}

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	falseVal := boolPtr(false)
	if *trueVal != true {
		t.Error("boolPtr(true) should return *true")
	}
	if *falseVal != false {
		t.Error("boolPtr(false) should return *false")
	}
}
