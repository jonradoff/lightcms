package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newErrorCLIServer returns a server that replies 500 to everything, driving
// each command's client-error return branch.
func newErrorCLIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
}

// newRichCLIServer returns canned responses with non-empty lists so that the
// table-rendering loops in the human (non-JSON) output mode execute.
func newRichCLIServer() *httptest.Server {
	mux := http.NewServeMux()
	ts := "2025-01-01T00:00:00Z"

	mux.HandleFunc("/api/v1/content", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "c1", "title": "Live", "slug": "live", "full_path": "/live", "published": true, "created_at": ts, "updated_at": ts},
			{"id": "c2", "title": "Draft", "slug": "draft", "full_path": "/draft", "published": false, "created_at": ts, "updated_at": ts},
			{"id": "c3", "title": "Gone", "slug": "gone", "full_path": "/gone", "deleted": true, "created_at": ts, "updated_at": ts},
		})
	})
	mux.HandleFunc("/api/v1/content/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/versions") {
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"version": 1, "title": "V1", "comment": "first", "created_at": ts},
				{"version": 2, "title": "V2", "comment": "second", "created_at": ts},
			})
			return
		}
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "c1", "title": "Live", "slug": "live", "full_path": "/live",
				"data":       map[string]interface{}{"body": "old", "keep": "yes"},
				"created_at": ts, "updated_at": ts,
			})
		case http.MethodPut:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "c1", "title": "Updated", "slug": "live", "full_path": "/live",
				"created_at": ts, "updated_at": ts,
			})
		}
	})
	mux.HandleFunc("/api/v1/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "t1", "name": "Sys", "slug": "sys", "category": "core", "is_system": true,
					"fields": []map[string]interface{}{{"name": "body"}}, "created_at": ts, "updated_at": ts},
				{"id": "t2", "name": "Custom", "slug": "custom", "is_system": false,
					"fields": []interface{}{}, "created_at": ts, "updated_at": ts},
			})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "t3", "name": "New", "slug": "new", "fields": []interface{}{},
				"created_at": ts, "updated_at": ts,
			})
		}
	})
	mux.HandleFunc("/api/v1/templates/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "t1", "name": "Sys", "slug": "sys", "fields": []interface{}{},
			"created_at": ts, "updated_at": ts,
		})
	})
	mux.HandleFunc("/api/v1/theme", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"site_name": "Rich Site"})
	})
	mux.HandleFunc("/api/v1/theme/versions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"version": 1, "site_name": "Old", "comment": "initial", "created_at": ts},
			{"version": 2, "site_name": "New", "comment": "tweak", "created_at": ts},
		})
	})
	mux.HandleFunc("/api/v1/redirects", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "r1", "from_path": "/a", "to_path": "/b", "status_code": 301, "description": "d",
				"created_at": ts, "updated_at": ts},
		})
	})
	mux.HandleFunc("/api/v1/redirects/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "r1", "from_path": "/a", "to_path": "/c",
			"created_at": ts, "updated_at": ts})
	})
	mux.HandleFunc("/api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "f1", "name": "Blog", "slug": "blog", "path": "/blog", "created_at": ts, "updated_at": ts},
		})
	})
	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "col1", "name": "Articles", "slug": "articles", "category": "blog",
					"created_at": ts, "updated_at": ts},
			})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "col2", "name": "New Col", "slug": "new-col", "created_at": ts, "updated_at": ts,
			})
		}
	})
	mux.HandleFunc("/api/v1/collections/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "col1", "name": "Articles", "slug": "articles", "created_at": ts, "updated_at": ts,
		})
	})
	mux.HandleFunc("/api/v1/assets", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "a1", "filename": "x.png", "serve_path": "/images/x.png", "mime_type": "image/png", "size": 5},
			})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "a2", "filename": "up.txt", "serve_path": "/docs/up.txt", "mime_type": "text/plain", "size": 5,
			})
		}
	})
	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query": r.URL.Query().Get("q"), "search_type": "fulltext", "total": 2,
			"matches": []map[string]interface{}{
				{"id": "c1", "title": "Live", "full_path": "/live", "published": true, "matched_in": []string{"title", "body"}},
				{"id": "c2", "title": "Draft", "full_path": "/draft", "published": false, "matched_in": []string{"body"}},
			},
		})
	})
	mux.HandleFunc("/api/v1/search-replace/preview", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new", "total_matches": 3, "affected_pages": 1,
			"matches": []map[string]interface{}{
				{"id": "c1", "title": "Live", "full_path": "/live", "match_count": 3},
			},
		})
	})
	mux.HandleFunc("/api/v1/search-replace/execute", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"search": "old", "replace": "new", "total_replacements": 3, "pages_updated": 1,
		})
	})
	mux.HandleFunc("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "k1", "name": "used", "prefix": "lc_a", "description": "d",
					"last_used_at": ts, "created_at": ts},
				{"id": "k2", "name": "unused", "prefix": "lc_b", "created_at": ts},
			})
		case http.MethodPost:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "k3", "name": "fresh", "key": "lc_secret", "prefix": "lc_s", "created_at": ts,
			})
		}
	})
	// Fallback for the routes that only need a 200.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	})
	return httptest.NewServer(mux)
}

func TestCLI_ErrorReturns(t *testing.T) {
	ts := newErrorCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	tmp := t.TempDir()
	upload := filepath.Join(tmp, "up.txt")
	if err := os.WriteFile(upload, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"content", "list"},
		{"content", "get", "c1"},
		{"content", "create", "--template", "t1", "--title", "T", "--slug", "s"},
		{"content", "update", "--id", "c1", "--title", "T"},
		{"content", "update", "--id", "c1", "--data", `{"k":"v"}`}, // GetContent fails during merge
		{"content", "delete", "c1"},
		{"content", "publish", "c1"},
		{"content", "unpublish", "c1"},
		{"content", "restore", "c1"},
		{"content", "versions", "c1"},
		{"content", "revert", "--id", "c1", "--version", "1"},
		{"template", "get", "t1"},
		{"template", "create", "--name", "N", "--slug", "n"},
		{"template", "update", "--id", "t1", "--name", "N"},
		{"template", "delete", "t1"},
		{"theme", "get"},
		{"theme", "update", "--site-name", "S"},
		{"theme", "versions"},
		{"theme", "revert", "--version", "1"},
		{"config", "get"},
		{"config", "update", "--title-template", "T"},
		{"redirect", "list"},
		{"redirect", "create", "--from", "/a", "--to", "/b"},
		{"redirect", "update", "--id", "r1", "--from", "/a"},
		{"redirect", "delete", "r1"},
		{"folder", "list"},
		{"folder", "create", "--name", "N", "--slug", "n"},
		{"folder", "delete", "f1"},
		{"collection", "list"},
		{"collection", "get", "col1"},
		{"collection", "create", "--name", "N", "--slug", "n"},
		{"collection", "update", "--id", "col1", "--name", "N"},
		{"collection", "delete", "col1"},
		{"asset", "list"},
		{"asset", "get", "a1"},
		{"asset", "upload", "--file", upload, "--path", "/docs/up.txt"},
		{"asset", "delete", "a1"},
		{"asset", "folders"},
		{"search", "query"},
		{"search-replace", "preview", "--search", "a", "--replace", "b"},
		{"search-replace", "execute", "--search", "a", "--replace", "b"},
		{"api-key", "list"},
		{"api-key", "create", "--name", "k"},
		{"api-key", "delete", "k1"},
		{"regenerate"},
	}
	for _, cmd := range cmds {
		if err := app.Run(cmd); err == nil {
			t.Errorf("%v: expected error from failing backend", cmd)
		}
	}
}

func TestCLI_TableOutputWithRows(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false) // human output mode

	cmds := [][]string{
		{"content", "list"},
		{"content", "versions", "c1"},
		{"template", "list"},
		{"theme", "versions"},
		{"redirect", "list"},
		{"folder", "list"},
		{"collection", "list"},
		{"asset", "list"},
		{"api-key", "list"},
		{"search", "live"},
		{"search-replace", "preview", "--search", "old", "--replace", "new"},
		{"search-replace", "execute", "--search", "old", "--replace", "new", "--comment", "c"},
		{"api-key", "create", "--name", "fresh", "--description", "d"},
	}
	for _, cmd := range cmds {
		if err := app.Run(cmd); err != nil {
			t.Errorf("%v: %v", cmd, err)
		}
	}
}

func TestCLI_ContentUpdateDataMerge(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	err := app.Run([]string{"content", "update", "--id", "c1",
		"--slug", "live2", "--template", "t9", "--folder", "/f",
		"--category", "cat", "--comment", "vc", "--data", `{"body":"new"}`})
	if err != nil {
		t.Fatalf("content update with data merge: %v", err)
	}

	// Invalid JSON branches.
	if err := app.Run([]string{"content", "update", "--id", "c1", "--data", `{bad`}); err == nil {
		t.Error("expected invalid --data JSON error (update)")
	}
	if err := app.Run([]string{"content", "create", "--template", "t1", "--title", "T",
		"--slug", "s", "--data", `{bad`}); err == nil {
		t.Error("expected invalid --data JSON error (create)")
	}
}

func TestCLI_TemplateFileAndFieldsFlags(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	tmp := t.TempDir()
	layout := filepath.Join(tmp, "layout.html")
	if err := os.WriteFile(layout, []byte("<div>{{.body}}</div>"), 0o644); err != nil {
		t.Fatal(err)
	}

	fields := `[{"name":"body","label":"Body","type":"richtext","required":true}]`

	if err := app.Run([]string{"template", "create", "--name", "N", "--slug", "n",
		"--description", "d", "--category", "c", "--fields", fields,
		"--layout-file", layout}); err != nil {
		t.Errorf("template create with layout file: %v", err)
	}
	if err := app.Run([]string{"template", "update", "--id", "t1",
		"--description", "d2", "--category", "c2", "--fields", fields,
		"--layout-file", layout}); err != nil {
		t.Errorf("template update with layout file: %v", err)
	}
	if err := app.Run([]string{"template", "update", "--id", "t1", "--layout", "<b>inline</b>"}); err != nil {
		t.Errorf("template update with inline layout: %v", err)
	}

	// Invalid fields JSON and unreadable layout file.
	if err := app.Run([]string{"template", "create", "--name", "N", "--slug", "n", "--fields", `[bad`}); err == nil {
		t.Error("expected invalid --fields JSON error (create)")
	}
	if err := app.Run([]string{"template", "update", "--id", "t1", "--fields", `[bad`}); err == nil {
		t.Error("expected invalid --fields JSON error (update)")
	}
	if err := app.Run([]string{"template", "create", "--name", "N", "--slug", "n",
		"--layout-file", filepath.Join(tmp, "missing.html")}); err == nil {
		t.Error("expected missing layout file error (create)")
	}
	if err := app.Run([]string{"template", "update", "--id", "t1",
		"--layout-file", filepath.Join(tmp, "missing.html")}); err == nil {
		t.Error("expected missing layout file error (update)")
	}
}

func TestCLI_ThemeUpdateAllFlags(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	tmp := t.TempDir()
	mk := func(name, content string) string {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cssFile := mk("style.css", "body{}")
	headFile := mk("head.html", "<meta>")
	headerFile := mk("header.html", "<header>")
	footerFile := mk("footer.html", "<footer>")

	if err := app.Run([]string{"theme", "update",
		"--site-name", "S", "--tagline", "T", "--logo", "/l.png",
		"--primary-color", "#111", "--secondary-color", "#222", "--accent-color", "#333",
		"--bg-color", "#444", "--text-color", "#555", "--font", "Inter",
		"--heading-font", "Lora", "--border-radius", "4px",
		"--css-file", cssFile, "--head-file", headFile,
		"--header-file", headerFile, "--footer-file", footerFile}); err != nil {
		t.Errorf("theme update all flags: %v", err)
	}

	// Inline string variants of the file-based fields.
	if err := app.Run([]string{"theme", "update",
		"--css", "p{}", "--head-html", "<meta x>", "--header-html", "<h>", "--footer-html", "<f>"}); err != nil {
		t.Errorf("theme update inline html: %v", err)
	}

	// Missing file error branches.
	missing := filepath.Join(tmp, "nope.css")
	for _, flagName := range []string{"--css-file", "--head-file", "--header-file", "--footer-file"} {
		if err := app.Run([]string{"theme", "update", flagName, missing}); err == nil {
			t.Errorf("expected missing file error for %s", flagName)
		}
	}
}

func TestCLI_SettingsOptionalFlags(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	cmds := [][]string{
		{"config", "update", "--title-template", "T", "--title-template-no-title", "N"},
		{"redirect", "update", "--id", "r1", "--from", "/a", "--to", "/b", "--status", "302", "--description", "d"},
		{"collection", "create", "--name", "N", "--slug", "n", "--description", "d",
			"--category", "c", "--sort-field", "title", "--sort-order", "asc", "--per-page", "20"},
		{"collection", "update", "--id", "col1", "--name", "N", "--slug", "n", "--description", "d",
			"--category", "c", "--sort-field", "title", "--sort-order", "desc", "--per-page", "10"},
	}
	for _, cmd := range cmds {
		if err := app.Run(cmd); err != nil {
			t.Errorf("%v: %v", cmd, err)
		}
	}
}

func TestCLI_AssetUploadSuccessAndMissingFile(t *testing.T) {
	ts := newRichCLIServer()
	defer ts.Close()
	app := newTestApp(ts.URL, false)

	tmp := t.TempDir()
	upload := filepath.Join(tmp, "up.txt")
	if err := os.WriteFile(upload, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := app.Run([]string{"asset", "upload", "--file", upload, "--path", "/docs/up.txt",
		"--description", "d"}); err != nil {
		t.Errorf("asset upload: %v", err)
	}
	if err := app.Run([]string{"asset", "upload", "--file", filepath.Join(tmp, "missing.bin"),
		"--path", "/docs/x"}); err == nil {
		t.Error("expected error for missing upload file")
	}
	// asset get by path
	if err := app.Run([]string{"asset", "get", "--path", "/images/x.png"}); err != nil {
		t.Errorf("asset get by path: %v", err)
	}
}
