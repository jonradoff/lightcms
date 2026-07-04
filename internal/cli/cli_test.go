package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/apiclient"
)

// newTestServer creates an httptest.Server that returns canned JSON responses
// for the LightCMS API routes used by the CLI.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// Content
	mux.HandleFunc("/api/v1/content", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "c1", "title": "Test", "slug": "test", "full_path": "/test",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/content/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/publish"), strings.HasSuffix(path, "/unpublish"),
			strings.HasSuffix(path, "/restore"), strings.HasSuffix(path, "/revert"):
			w.WriteHeader(http.StatusOK)
		case strings.Contains(path, "/versions"):
			if r.Method == http.MethodGet {
				json.NewEncoder(w).Encode([]map[string]interface{}{})
			} else {
				w.WriteHeader(http.StatusOK)
			}
		default:
			switch r.Method {
			case http.MethodGet:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "c1", "title": "Test", "slug": "test", "full_path": "/test",
					"data":       map[string]interface{}{},
					"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
				})
			case http.MethodPut:
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": "c1", "title": "Updated", "slug": "test", "full_path": "/test",
					"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
				})
			case http.MethodDelete:
				w.WriteHeader(http.StatusOK)
			}
		}
	})

	// Templates
	mux.HandleFunc("/api/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "t1", "name": "Blog", "slug": "blog",
				"fields":     []interface{}{},
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/templates/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "t1", "name": "Blog", "slug": "blog",
				"fields":     []interface{}{},
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodPut:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "t1", "name": "Updated", "slug": "blog",
				"fields":     []interface{}{},
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	// Theme
	mux.HandleFunc("/api/v1/theme", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"site_name": "Test Site", "primary_color": "#000",
			})
		case http.MethodPut:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"site_name": "Updated Site", "primary_color": "#fff",
			})
		}
	})
	mux.HandleFunc("/api/v1/theme/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})
	mux.HandleFunc("/api/v1/theme/versions/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Config
	mux.HandleFunc("/api/v1/config", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title_template": "{{.Title}} | Site",
		})
	})

	// Redirects
	mux.HandleFunc("/api/v1/redirects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "r1", "from_path": "/old", "to_path": "/new", "status_code": 301,
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/redirects/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "r1", "from_path": "/old", "to_path": "/new2",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	// Folders
	mux.HandleFunc("/api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "f1", "name": "Blog", "slug": "blog", "path": "/blog",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/folders/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "f1", "name": "Blog", "slug": "blog", "path": "/blog",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	// Collections
	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "col1", "name": "Articles", "slug": "articles",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/collections/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "col1", "name": "Articles", "slug": "articles",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodPut:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "col1", "name": "Updated", "slug": "articles",
				"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	// Assets
	mux.HandleFunc("/api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "a1", "filename": "logo.png", "serve_path": "/images/logo.png",
				"mime_type": "image/png", "size": 1024,
			})
		}
	})
	mux.HandleFunc("/api/v1/assets/folders", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]string{"/images", "/docs"})
	})
	mux.HandleFunc("/api/v1/assets/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "a1", "filename": "logo.png", "serve_path": "/images/logo.png",
				"mime_type": "image/png", "size": 1024,
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	// Search
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": r.URL.Query().Get("q"), "search_type": "fulltext",
			"total": 0, "matches": []interface{}{},
		})
	})

	// Search-Replace
	mux.HandleFunc("/api/v1/search-replace/preview", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new",
			"total_matches": 0, "affected_pages": 0, "matches": []interface{}{},
		})
	})
	mux.HandleFunc("/api/v1/search-replace/execute", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new",
			"total_replacements": 0, "pages_updated": 0,
		})
	})

	// API Keys
	mux.HandleFunc("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "k1", "name": "test-key", "key": "lc_abc123",
				"prefix":     "lc_abc",
				"created_at": "2025-01-01T00:00:00Z",
			})
		}
	})
	mux.HandleFunc("/api/v1/api-keys/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Regenerate
	mux.HandleFunc("/api/v1/regenerate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(mux)
}

func newTestApp(serverURL string, jsonMode bool) *App {
	client := apiclient.New(serverURL, "test-api-key")
	return New(client, jsonMode)
}

// --- Dispatch tests ---

func TestRun_NoArgs(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	err := app.Run(nil)
	if err == nil {
		t.Fatal("expected error for nil args")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_EmptyArgs(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	err := app.Run([]string{})
	if err == nil {
		t.Fatal("expected error for empty args")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	err := app.Run([]string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Content tests ---

func TestContentList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"content", "list"}); err != nil {
			t.Fatalf("content list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestContentGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "get", "c1"}); err != nil {
		t.Fatalf("content get failed: %v", err)
	}
}

func TestContentGet_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "get"})
	if err == nil {
		t.Fatal("expected error when no ID or path provided")
	}
}

func TestContentCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"content", "create",
			"--template", "t1", "--title", "Hello", "--slug", "hello"})
		if err != nil {
			t.Fatalf("content create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestContentCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "create", "--title", "Hello"})
	if err == nil {
		t.Fatal("expected error when missing required flags")
	}
}

func TestContentUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"content", "update", "--id", "c1", "--title", "Updated"})
		if err != nil {
			t.Fatalf("content update (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestContentUpdate_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "update"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestContentDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "delete", "c1"}); err != nil {
		t.Fatalf("content delete failed: %v", err)
	}
}

func TestContentDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestContentPublish(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "publish", "c1"}); err != nil {
		t.Fatalf("content publish failed: %v", err)
	}
}

func TestContentUnpublish(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "unpublish", "c1"}); err != nil {
		t.Fatalf("content unpublish failed: %v", err)
	}
}

func TestContentRestore(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "restore", "c1"}); err != nil {
		t.Fatalf("content restore failed: %v", err)
	}
}

func TestContentVersions(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"content", "versions", "c1"}); err != nil {
			t.Fatalf("content versions (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestContentRevert(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "revert", "--id", "c1", "--version", "1"}); err != nil {
		t.Fatalf("content revert failed: %v", err)
	}
}

func TestContentRevert_MissingFlags(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "revert"})
	if err == nil {
		t.Fatal("expected error when missing --id and --version")
	}
}

func TestContentNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestContentUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"content", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Template tests ---

func TestTemplateList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"template", "list"}); err != nil {
			t.Fatalf("template list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestTemplateGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"template", "get", "t1"}); err != nil {
		t.Fatalf("template get failed: %v", err)
	}
}

func TestTemplateGet_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template", "get"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestTemplateCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"template", "create", "--name", "Blog", "--slug", "blog"})
		if err != nil {
			t.Fatalf("template create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestTemplateCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template", "create", "--name", "Blog"})
	if err == nil {
		t.Fatal("expected error when missing --slug")
	}
}

func TestTemplateUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"template", "update", "--id", "t1", "--name", "Updated"})
		if err != nil {
			t.Fatalf("template update (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestTemplateUpdate_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template", "update"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestTemplateDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"template", "delete", "t1"}); err != nil {
		t.Fatalf("template delete failed: %v", err)
	}
}

func TestTemplateDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestTemplateNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestTemplateUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"template", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Theme tests ---

func TestThemeGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"theme", "get"}); err != nil {
		t.Fatalf("theme get failed: %v", err)
	}
}

func TestThemeUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"theme", "update", "--site-name", "New Name"})
		if err != nil {
			t.Fatalf("theme update (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestThemeUpdate_NoFields(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"theme", "update"})
	if err == nil {
		t.Fatal("expected error when no fields to update")
	}
}

func TestThemeVersions(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"theme", "versions"}); err != nil {
			t.Fatalf("theme versions (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestThemeRevert(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"theme", "revert", "--version", "1"}); err != nil {
		t.Fatalf("theme revert failed: %v", err)
	}
}

func TestThemeRevert_NoVersion(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"theme", "revert"})
	if err == nil {
		t.Fatal("expected error when no version")
	}
}

func TestThemeNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"theme"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestThemeUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"theme", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Config tests ---

func TestConfigGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"config", "get"}); err != nil {
		t.Fatalf("config get failed: %v", err)
	}
}

func TestConfigUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"config", "update", "--title-template", "{{.Title}} | Site"})
		if err != nil {
			t.Fatalf("config update (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestConfigNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"config"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestConfigUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"config", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Redirect tests ---

func TestRedirectList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"redirect", "list"}); err != nil {
			t.Fatalf("redirect list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestRedirectCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"redirect", "create", "--from", "/old", "--to", "/new"})
		if err != nil {
			t.Fatalf("redirect create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestRedirectCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"redirect", "create", "--from", "/old"})
	if err == nil {
		t.Fatal("expected error when missing --to")
	}
}

func TestRedirectUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"redirect", "update", "--id", "r1", "--to", "/new2"}); err != nil {
		t.Fatalf("redirect update failed: %v", err)
	}
}

func TestRedirectUpdate_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"redirect", "update"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestRedirectDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"redirect", "delete", "r1"}); err != nil {
		t.Fatalf("redirect delete failed: %v", err)
	}
}

func TestRedirectDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"redirect", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestRedirectNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"redirect"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestRedirectUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"redirect", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Folder tests ---

func TestFolderList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"folder", "list"}); err != nil {
			t.Fatalf("folder list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestFolderCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"folder", "create", "--name", "Blog", "--slug", "blog"})
		if err != nil {
			t.Fatalf("folder create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestFolderCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"folder", "create", "--name", "Blog"})
	if err == nil {
		t.Fatal("expected error when missing --slug")
	}
}

func TestFolderDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"folder", "delete", "f1"}); err != nil {
		t.Fatalf("folder delete failed: %v", err)
	}
}

func TestFolderDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"folder", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestFolderNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"folder"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestFolderUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"folder", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Collection tests ---

func TestCollectionList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"collection", "list"}); err != nil {
			t.Fatalf("collection list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestCollectionGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"collection", "get", "col1"}); err != nil {
		t.Fatalf("collection get failed: %v", err)
	}
}

func TestCollectionGet_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection", "get"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestCollectionCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"collection", "create", "--name", "Articles", "--slug", "articles"})
		if err != nil {
			t.Fatalf("collection create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestCollectionCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection", "create", "--name", "Articles"})
	if err == nil {
		t.Fatal("expected error when missing --slug")
	}
}

func TestCollectionUpdate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"collection", "update", "--id", "col1", "--name", "Updated"}); err != nil {
		t.Fatalf("collection update failed: %v", err)
	}
}

func TestCollectionUpdate_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection", "update"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestCollectionDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"collection", "delete", "col1"}); err != nil {
		t.Fatalf("collection delete failed: %v", err)
	}
}

func TestCollectionDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestCollectionNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestCollectionUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"collection", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Asset tests ---

func TestAssetList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"asset", "list"}); err != nil {
			t.Fatalf("asset list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestAssetGet(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"asset", "get", "a1"}); err != nil {
		t.Fatalf("asset get failed: %v", err)
	}
}

func TestAssetGet_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"asset", "get"})
	if err == nil {
		t.Fatal("expected error when no ID or path")
	}
}

func TestAssetDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"asset", "delete", "a1"}); err != nil {
		t.Fatalf("asset delete failed: %v", err)
	}
}

func TestAssetDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"asset", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestAssetFolders(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"asset", "folders"}); err != nil {
			t.Fatalf("asset folders (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestAssetUpload_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"asset", "upload"})
	if err == nil {
		t.Fatal("expected error when missing --file and --path")
	}
}

func TestAssetNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"asset"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestAssetUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"asset", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Search tests ---

func TestSearch(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"search", "hello world"}); err != nil {
			t.Fatalf("search (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestSearch_NoQuery(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"search"})
	if err == nil {
		t.Fatal("expected error when no query")
	}
}

// --- Search-Replace tests ---

func TestSearchReplacePreview(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"search-replace", "preview", "--search", "old", "--replace", "new"})
		if err != nil {
			t.Fatalf("search-replace preview (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestSearchReplacePreview_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"search-replace", "preview", "--search", "old"})
	if err == nil {
		t.Fatal("expected error when missing --replace")
	}
}

func TestSearchReplaceExecute(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"search-replace", "execute", "--search", "old", "--replace", "new"})
		if err != nil {
			t.Fatalf("search-replace execute (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestSearchReplaceExecute_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"search-replace", "execute", "--search", "old"})
	if err == nil {
		t.Fatal("expected error when missing --replace")
	}
}

func TestSearchReplaceNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"search-replace"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestSearchReplaceUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"search-replace", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- API Key tests ---

func TestAPIKeyList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		if err := app.Run([]string{"api-key", "list"}); err != nil {
			t.Fatalf("api-key list (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestAPIKeyCreate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	for _, jsonMode := range []bool{false, true} {
		app := newTestApp(ts.URL, jsonMode)
		err := app.Run([]string{"api-key", "create", "--name", "test-key"})
		if err != nil {
			t.Fatalf("api-key create (json=%v) failed: %v", jsonMode, err)
		}
	}
}

func TestAPIKeyCreate_MissingRequired(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"api-key", "create"})
	if err == nil {
		t.Fatal("expected error when missing --name")
	}
}

func TestAPIKeyDelete(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"api-key", "delete", "k1"}); err != nil {
		t.Fatalf("api-key delete failed: %v", err)
	}
}

func TestAPIKeyDelete_NoID(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"api-key", "delete"})
	if err == nil {
		t.Fatal("expected error when no ID")
	}
}

func TestAPIKeyNoSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"api-key"})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestAPIKeyUnknownSubcommand(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	err := app.Run([]string{"api-key", "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- Regenerate tests ---

func TestRegenerate(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"regenerate"}); err != nil {
		t.Fatalf("regenerate failed: %v", err)
	}
}

// --- Server error tests ---

func TestServerError(t *testing.T) {
	// Server that always returns 500
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer errServer.Close()

	app := newTestApp(errServer.URL, false)

	commands := [][]string{
		{"content", "list"},
		{"template", "list"},
		{"theme", "get"},
		{"config", "get"},
		{"redirect", "list"},
		{"folder", "list"},
		{"collection", "list"},
		{"asset", "list"},
		{"search", "test"},
		{"api-key", "list"},
		{"regenerate"},
	}

	for _, args := range commands {
		err := app.Run(args)
		if err == nil {
			t.Errorf("expected error for %v with 500 server", args)
		}
	}
}

// --- Helper function tests ---

func TestGetIDArg(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
	}{
		{[]string{"abc123"}, "abc123"},
		{[]string{"--flag", "value", "abc123"}, "value"},
		{[]string{"--flag"}, ""},
		{nil, ""},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		got := getIDArg(tt.args)
		if got != tt.expected {
			t.Errorf("getIDArg(%v) = %q, want %q", tt.args, got, tt.expected)
		}
	}
}

// --- Content list with filters ---

func TestContentListWithFilters(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "list", "-deleted", "-category", "blog"}); err != nil {
		t.Fatalf("content list with filters failed: %v", err)
	}
}

// --- Content get by path ---

func TestContentGetByPath(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	// The /api/v1/content/by-path route needs a handler
	// Our test server handles /api/v1/content/ prefix which covers by-path
	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"content", "get", "-path", "/test"}); err != nil {
		t.Fatalf("content get by path failed: %v", err)
	}
}

// --- Asset list with folder filter ---

func TestAssetListWithFolder(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"asset", "list", "-folder", "/images"}); err != nil {
		t.Fatalf("asset list with folder failed: %v", err)
	}
}

// --- Search with type flag ---

func TestSearchWithTypeFlag(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	app := newTestApp(ts.URL, false)
	if err := app.Run([]string{"search", "-type", "name", "query"}); err != nil {
		t.Fatalf("search with type flag failed: %v", err)
	}
}
