package handlers

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"

	"github.com/gorilla/mux"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedPublishedContent inserts a published live content item.
func seedPublishedContent(t *testing.T, h *Handler, templateID primitive.ObjectID, title, fullPath, metaDesc, plainText string) primitive.ObjectID {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id":              id,
		"template_id":      templateID,
		"title":            title,
		"slug":             strings.TrimPrefix(fullPath, "/"),
		"full_path":        fullPath,
		"meta_description": metaDesc,
		"plain_text":       plainText,
		"published":        true,
		"published_at":     now,
		"created_at":       now,
		"updated_at":       now,
	}
	if _, err := h.db.Collection("content").InsertOne(ctx, doc); err != nil {
		t.Fatalf("seedPublishedContent: %v", err)
	}
	return id
}

func TestServeLlmsTxt(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedPublishedContent(t, h, tmplID, "Alpha Page", "/alpha", "About alpha.", "Alpha body text.")
	seedPublishedContent(t, h, tmplID, "Beta Page", "/beta", "", "")
	seedContent(t, h.db, tmplID, "Draft Page", "draft", "/draft") // unpublished

	// A forked copy must not appear.
	forkID := primitive.NewObjectID()
	ctx := context.Background()
	_, _ = h.db.Collection("content").InsertOne(ctx, bson.M{
		"_id": primitive.NewObjectID(), "template_id": tmplID,
		"title": "Forked Page", "full_path": "/forked", "published": true,
		"fork_id": forkID, "created_at": time.Now(), "updated_at": time.Now(),
	})

	req := httptest.NewRequest("GET", "/llms.txt", nil)
	rr := httptest.NewRecorder()
	h.ServeLlmsTxt(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"- [Alpha Page](http://localhost:8082/alpha): About alpha.",
		"- [Beta Page](http://localhost:8082/beta)",
		"llms-full.txt",
		"sitemap.xml",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("llms.txt missing %q\nbody:\n%s", want, body)
		}
	}
	for _, absent := range []string{"Draft Page", "Forked Page"} {
		if strings.Contains(body, absent) {
			t.Errorf("llms.txt should not contain %q", absent)
		}
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}

func TestServeLlmsFullTxt(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedPublishedContent(t, h, tmplID, "Gamma Page", "/gamma", "Gamma desc.", "Full gamma body text here.")

	req := httptest.NewRequest("GET", "/llms-full.txt", nil)
	rr := httptest.NewRecorder()
	h.ServeLlmsFullTxt(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"## Gamma Page",
		"URL: http://localhost:8082/gamma",
		"Description: Gamma desc.",
		"Full gamma body text here.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("llms-full.txt missing %q\nbody:\n%s", want, body)
		}
	}
}

func TestBuildJSONLD(t *testing.T) {
	if got := buildJSONLD(nil, nil, "Site", "http://x", ""); got != "" {
		t.Errorf("nil content: got %q, want empty", got)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	content := &models.Content{
		Title:           "My Post",
		FullPath:        "/blog/my-post",
		MetaDescription: "A post about </script> escaping.",
		PublishedAt:     &now,
		UpdatedAt:       now,
	}

	tests := []struct {
		name     string
		tmpl     *models.Template
		wantType string
	}{
		{"blog template", &models.Template{Name: "Blog Post"}, `"@type":"BlogPosting"`},
		{"press template", &models.Template{Name: "Press Release"}, `"@type":"NewsArticle"`},
		{"plain template", &models.Template{Name: "Standard Page"}, `"@type":"WebPage"`},
		{"nil template", nil, `"@type":"WebPage"`},
	}
	for _, tc := range tests {
		got := buildJSONLD(content, tc.tmpl, "My Site", "https://example.com/", "https://example.com/img.png")
		if !strings.Contains(got, tc.wantType) {
			t.Errorf("%s: missing %s in %s", tc.name, tc.wantType, got)
		}
		if !strings.Contains(got, `<script type="application/ld+json">`) {
			t.Errorf("%s: missing script wrapper", tc.name)
		}
		if strings.Contains(got, "</script> escaping") {
			t.Errorf("%s: unescaped </script> inside JSON payload", tc.name)
		}
		for _, want := range []string{
			`"headline":"My Post"`,
			`"url":"https://example.com/blog/my-post"`,
			`"datePublished":"2026-07-01T12:00:00Z"`,
			`"publisher":{"@type":"Organization","name":"My Site"}`,
			`"image":"https://example.com/img.png"`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: missing %s\ngot: %s", tc.name, want, got)
			}
		}
	}
}

func TestServePage_IncludesJSONLD(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Blog Post", "blog-post")
	seedPublishedContent(t, h, tmplID, "LD Page", "/ld-page", "LD test page.", "")

	req := httptest.NewRequest("GET", "/ld-page", nil)
	req = mux.SetURLVars(req, map[string]string{"slug": "ld-page"})
	rr := httptest.NewRecorder()
	h.ServePage(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `application/ld+json`) {
		t.Errorf("served page missing JSON-LD script tag")
	}
	if !strings.Contains(body, `"@type":"BlogPosting"`) {
		t.Errorf("served page missing BlogPosting type\nbody head: %.500s", body)
	}
}

func TestBuildWebsiteJSONLD(t *testing.T) {
	got := buildWebsiteJSONLD("My Site", "A tagline.", "https://example.com/")
	for _, want := range []string{
		`"@type":"WebSite"`,
		`"name":"My Site"`,
		`"description":"A tagline."`,
		`"url":"https://example.com"`,
		`"@type":"SearchAction"`,
		`"target":"https://example.com/api/search?q={search_term_string}"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	// No tagline → no description key.
	got = buildWebsiteJSONLD("X", "", "https://x.com")
	if strings.Contains(got, "description") {
		t.Errorf("empty tagline should omit description: %s", got)
	}
}

func TestHomepage_WebsiteJSONLD(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServePage(rr, req)

	if rr.Code != 200 {
		t.Skipf("homepage render returned %d (no homepage content in test DB)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"@type":"WebSite"`) {
		t.Errorf("homepage missing WebSite JSON-LD")
	}
}

func TestRawHomepage_WebsiteJSONLDInjection(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Blank Page", "blank-page")
	ctx := context.Background()
	now := time.Now()
	rawHTML := "<html><head><title>Custom</title></head><body>home</body></html>"
	_, _ = h.db.Collection("content").InsertOne(ctx, bson.M{
		"_id": primitive.NewObjectID(), "template_id": tmplID, "template_name": "Blank Page",
		"title": "Home", "slug": "", "full_path": "/", "published": true,
		"use_theme": false, "raw_mode": true,
		"data":       bson.M{"content": rawHTML},
		"created_at": now, "updated_at": now, "published_at": now,
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServePage(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"@type":"WebSite"`) {
		t.Errorf("raw homepage missing injected WebSite JSON-LD:\n%.400s", body)
	}
	if !strings.Contains(body, `<script type="application/ld+json">`) ||
		strings.Index(body, "application/ld+json") > strings.Index(body, "</head>") {
		t.Errorf("JSON-LD not injected inside <head>")
	}
	if !strings.Contains(body, "<body>home</body>") {
		t.Errorf("authored content altered")
	}
}

// TestSlugWithPeriod verifies that slugs containing periods work end to end:
// creation, static generation, and public serving (the dot-in-path asset
// shortcut must fall through to content when no asset file exists).
func TestSlugWithPeriod(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	ctx := context.Background()

	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Page",
		Title: "Release Two Point Oh", Slug: "release-2.0",
		Data: map[string]interface{}{"Body": "dotted slug body"}, Published: true,
	}
	if err := h.contentService.CreateContent(ctx, c, "dotted slug test"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	if c.FullPath != "/release-2.0" {
		t.Fatalf("full_path = %q, want /release-2.0", c.FullPath)
	}

	// Static file generated with .html suffix appended (no collision with
	// the asset-serving path, which has no suffix).
	staticPath := filepath.Join("content", "generated", "release-2.0.html")
	defer os.Remove(staticPath)
	if _, err := os.Stat(staticPath); err != nil {
		t.Errorf("static file not generated at %s: %v", staticPath, err)
	}

	// Public request: the "contains a dot → asset?" branch must fall
	// through and serve the page.
	req := httptest.NewRequest("GET", "/release-2.0", nil)
	req = mux.SetURLVars(req, map[string]string{"slug": "release-2.0"})
	rr := httptest.NewRecorder()
	h.ServePage(rr, req)
	if rr.Code != 200 {
		t.Fatalf("ServePage status = %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html (not asset MIME)", ct)
	}
	if !strings.Contains(rr.Body.String(), "Release Two Point Oh") {
		t.Errorf("page body missing content")
	}

	// It also appears in llms.txt like any other page.
	rr = httptest.NewRecorder()
	h.ServeLlmsTxt(rr, httptest.NewRequest("GET", "/llms.txt", nil))
	if !strings.Contains(rr.Body.String(), "/release-2.0") {
		t.Errorf("llms.txt missing dotted-slug page")
	}
}
