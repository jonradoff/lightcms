package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedSEOPage inserts a published page whose template renders {{.Body}}.
func seedSEOPage(t *testing.T, h *Handler, tmplID primitive.ObjectID, tmplName, title, fullPath, body string, extra bson.M) primitive.ObjectID {
	t.Helper()
	now := time.Now()
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id": id, "template_id": tmplID, "template_name": tmplName, "title": title,
		"slug": strings.TrimPrefix(fullPath[strings.LastIndex(fullPath, "/"):], "/"), "full_path": fullPath,
		"meta_description": title + " description", "data": bson.M{"Body": body},
		"published": true, "published_at": now.Add(-time.Hour), "use_theme": true, "use_header": true, "use_footer": true,
		"created_at": now, "updated_at": now,
	}
	for k, v := range extra {
		doc[k] = v
	}
	if _, err := h.db.Collection("content").InsertOne(context.Background(), doc); err != nil {
		t.Fatal(err)
	}
	return id
}

func servePath(h *Handler, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := sessionReq("GET", path, nil, map[string]string{"slug": strings.TrimPrefix(path, "/")})
	// Public requests: drop the admin session cookie.
	req.Header.Del("Cookie")
	h.ServePage(rr, req)
	return rr
}

func newSEOHandler(t *testing.T) (*Handler, *services.SEOService, func()) {
	h, cleanup := newTestHandler(t)
	seo := services.NewSEOService(h.db)
	h.SetSEOService(seo)
	return h, seo, cleanup
}

func TestServeRobotsTxtPolicy(t *testing.T) {
	h, seo, cleanup := newSEOHandler(t)
	defer cleanup()
	if err := seo.Save(context.Background(), services.SEOConfig{TrainingPolicy: services.CrawlerDisallow, ContentSignals: true}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.ServeRobotsTxt(rr, httptest.NewRequest("GET", "/robots.txt", nil))
	body := rr.Body.String()
	for _, want := range []string{"User-agent: GPTBot", "User-agent: ClaudeBot", "Disallow: /", "ai-train=no", "Sitemap: http://localhost:8082/sitemap.xml", "# Feed: http://localhost:8082/feed.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "User-agent: OAI-SearchBot") {
		t.Error("AI search crawler blocked though only training was disallowed")
	}
}

func TestMarkdownAlternate(t *testing.T) {
	h, seo, cleanup := newSEOHandler(t)
	defer cleanup()
	tmplID := seedTemplate(t, h.db, "Standard Page", "standard-page")
	seedSEOPage(t, h, tmplID, "Standard Page", "MD Page", "/md-page", `<h1>MD Page</h1><h2>Hello</h2><p>World <a href="/x">link</a></p>`, nil)
	seedSEOPage(t, h, tmplID, "Standard Page", "Real Doc", "/real.md", `<p>I am HTML</p>`, nil)
	seedSEOPage(t, h, tmplID, "Standard Page", "Hidden", "/hidden", `<p>secret</p>`, bson.M{"noindex": true})

	rr := servePath(h, "/md-page.md")
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("md copy: status=%d ct=%s body=%s", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
	md := rr.Body.String()
	for _, want := range []string{"# MD Page\n", "> MD Page description", "Source: http://localhost:8082/md-page", "## Hello", "World [link](http://localhost:8082/x)"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	if strings.Count(md, "# MD Page") != 1 {
		t.Errorf("duplicated title heading:\n%s", md)
	}
	if rr.Header().Get("X-Robots-Tag") != "noindex" || !strings.Contains(rr.Header().Get("Link"), `rel="canonical"`) {
		t.Errorf("md copy headers: %v", rr.Header())
	}

	// A real page whose path ends in .md wins.
	if rr := servePath(h, "/real.md"); !strings.Contains(rr.Body.String(), "I am HTML") || strings.HasPrefix(rr.Header().Get("Content-Type"), "text/markdown") {
		t.Errorf("real .md page not served as HTML: %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	// …and has its own Markdown copy at .md.md.
	if rr := servePath(h, "/real.md.md"); rr.Code != 200 || !strings.Contains(rr.Body.String(), "I am HTML") {
		t.Errorf("real.md.md: %d", rr.Code)
	}
	// Hidden pages and unknown pages have no Markdown copy.
	if rr := servePath(h, "/hidden.md"); rr.Code != 404 {
		t.Errorf("noindex page md copy: %d", rr.Code)
	}
	if rr := servePath(h, "/nope.md"); rr.Code != 404 {
		t.Errorf("unknown md: %d", rr.Code)
	}
	// Disabled: 404.
	seo.Save(context.Background(), services.SEOConfig{MarkdownDisabled: true})
	if rr := servePath(h, "/md-page.md"); rr.Code != 404 {
		t.Errorf("disabled md copy: %d", rr.Code)
	}
}

func TestPageSEOHead(t *testing.T) {
	h, seo, cleanup := newSEOHandler(t)
	defer cleanup()
	ctx := context.Background()
	seo.Save(ctx, services.SEOConfig{AuthorName: "Jane Doe", AuthorURL: "https://jane.example.org", AuthorSameAs: []string{"https://x.com/jane"}})
	tmplID := seedTemplate(t, h.db, "Blog Post", "blog-post")
	h.db.Collection("folders").InsertOne(ctx, bson.M{"_id": primitive.NewObjectID(), "name": "Writing", "slug": "blog", "path": "/blog"})
	seedSEOPage(t, h, tmplID, "Blog Post", "Post One", "/blog/post-one",
		`<p>Body</p><h2>FAQ</h2><h3>Is it good?</h3><p>Very.</p>`, bson.M{"folder_path": "/blog"})
	seedSEOPage(t, h, tmplID, "Blog Post", "Guest Post", "/guest", `<p>x</p>`, bson.M{"author_name": "Guest Writer", "noindex": true})

	rr := servePath(h, "/blog/post-one")
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	page := rr.Body.String()
	for _, want := range []string{
		`<link rel="canonical" href="http://localhost:8082/blog/post-one">`,
		`type="text/markdown"`, `href="http://localhost:8082/blog/post-one.md"`,
		`type="application/rss+xml"`, `href="http://localhost:8082/feed.xml"`,
		`"@type":"BlogPosting"`, `"author":{"@type":"Person","name":"Jane Doe","sameAs":["https://x.com/jane"],"url":"https://jane.example.org"}`,
		`"@type":"BreadcrumbList"`, `"name":"Writing"`,
		`"@type":"FAQPage"`, `"name":"Is it good?"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page head missing %s", want)
		}
	}
	if strings.Contains(page, `name="robots"`) || rr.Header().Get("X-Robots-Tag") != "" {
		t.Error("indexable page marked noindex")
	}

	rr = servePath(h, "/guest")
	page = rr.Body.String()
	if !strings.Contains(page, `<meta name="robots" content="noindex">`) || rr.Header().Get("X-Robots-Tag") != "noindex" {
		t.Error("noindex page missing robots meta/header")
	}
	if !strings.Contains(page, `"author":{"@type":"Person","name":"Guest Writer"}`) {
		t.Error("per-page author override missing")
	}
	if strings.Contains(page, `type="text/markdown"`) {
		t.Error("noindex page should not advertise a Markdown copy")
	}
}

func TestBuildPageJSONLDSkipsExistingTypesAndHomepage(t *testing.T) {
	cfg := services.SEOConfig{PublisherSameAs: []string{"https://github.com/acme"}}
	c := &models.Content{Title: "Home", FullPath: "/", UpdatedAt: time.Now()}
	out := buildPageJSONLD(&pageLD{Content: c, SiteName: "Acme", BaseURL: "https://acme.example", LogoURL: "/logo.png", SEO: &cfg,
		ExistingLD: `<script type="application/ld+json">{"@type": "WebPage"}</script>`})
	if strings.Contains(out, `"@type":"WebPage"`) {
		t.Error("WebPage emitted though the page declares its own")
	}
	for _, want := range []string{`"@type":"WebSite"`, `"@type":"Organization"`, `"url":"https://acme.example/logo.png"`, `"sameAs":["https://github.com/acme"]`} {
		if !strings.Contains(out, want) {
			t.Errorf("homepage JSON-LD missing %s:\n%s", want, out)
		}
	}
	if existingLDTypes("<p>no ld</p>")["WebPage"] {
		t.Error("existingLDTypes false positive")
	}
}

func TestFeeds(t *testing.T) {
	h, seo, cleanup := newSEOHandler(t)
	defer cleanup()
	ctx := context.Background()
	blog := seedTemplate(t, h.db, "Blog Post", "blog-post")
	std := seedTemplate(t, h.db, "Standard Page", "standard-page")
	seedSEOPage(t, h, blog, "Blog Post", "Blog A", "/blog-a", `<p>A</p>`, bson.M{"category": "news"})
	seedSEOPage(t, h, blog, "Blog Post", "Hidden Blog", "/hidden-blog", `<p>H</p>`, bson.M{"noindex": true})
	seedSEOPage(t, h, std, "Standard Page", "Concept", "/concept", `<p>C</p>`, bson.M{"category": "news"})
	h.db.Collection("collections").InsertOne(ctx, bson.M{"_id": primitive.NewObjectID(), "name": "News", "slug": "news", "category": "news"})

	rr := httptest.NewRecorder()
	h.ServeRSSFeed(rr, httptest.NewRequest("GET", "/feed.xml", nil))
	rss := rr.Body.String()
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "application/rss+xml") {
		t.Fatalf("rss: %d %s", rr.Code, rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rss, "<title>Blog A</title>") || strings.Contains(rss, "Concept") || strings.Contains(rss, "Hidden Blog") {
		t.Errorf("site feed selection wrong:\n%s", rss)
	}
	for _, want := range []string{`<rss version="2.0"`, `<link>http://localhost:8082/blog-a</link>`, `rel="self"`} {
		if !strings.Contains(rss, want) {
			t.Errorf("rss missing %s", want)
		}
	}

	rr = httptest.NewRecorder()
	h.ServeAtomFeed(rr, httptest.NewRequest("GET", "/atom.xml", nil))
	if !strings.Contains(rr.Body.String(), `<feed xmlns="http://www.w3.org/2005/Atom">`) || !strings.Contains(rr.Body.String(), "<title>Blog A</title>") {
		t.Errorf("atom wrong:\n%s", rr.Body.String())
	}

	// Collection feed: by category, regardless of template.
	rr = httptest.NewRecorder()
	h.ServeRSSFeed(rr, sessionReq("GET", "/news/feed.xml", nil, map[string]string{"collection": "news"}))
	if body := rr.Body.String(); !strings.Contains(body, "Concept") || !strings.Contains(body, "Blog A") {
		t.Errorf("collection feed wrong:\n%s", body)
	}

	// All pages mode and disabled mode.
	seo.Save(ctx, services.SEOConfig{FeedAllPages: true})
	rr = httptest.NewRecorder()
	h.ServeRSSFeed(rr, httptest.NewRequest("GET", "/feed.xml", nil))
	if !strings.Contains(rr.Body.String(), "Concept") {
		t.Error("feed_all_pages should include every page")
	}
	seo.Save(ctx, services.SEOConfig{FeedDisabled: true})
	rr = httptest.NewRecorder()
	h.ServeRSSFeed(rr, httptest.NewRequest("GET", "/feed.xml", nil))
	if rr.Code != 404 {
		t.Errorf("disabled feed: %d", rr.Code)
	}
}

func TestAbsolutizeHTML(t *testing.T) {
	got := absolutizeHTML(`<a href="/a">x</a><img src="/i.png"><a href="//cdn.x/y">c</a><a href="https://o.org">o</a>`, "https://s.net/")
	want := `<a href="https://s.net/a">x</a><img src="https://s.net/i.png"><a href="//cdn.x/y">c</a><a href="https://o.org">o</a>`
	if got != want {
		t.Errorf("absolutizeHTML:\n got %s\nwant %s", got, want)
	}
}

func TestSEOToolPageAndSave(t *testing.T) {
	h, seo, cleanup := newSEOHandler(t)
	defer cleanup()
	rr := httptest.NewRecorder()
	h.SEOToolPage(rr, sessionReq("GET", "/cm/tools/seo", nil, nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "AI crawler policy") || !strings.Contains(rr.Body.String(), "GPTBot") {
		t.Fatalf("seo page: %d", rr.Code)
	}

	form := url.Values{
		"training_policy": {"disallow"}, "ai_search_policy": {"allow"}, "user_fetch_policy": {"allow"},
		"override_PerplexityBot": {"disallow"}, "content_signals": {"on"}, "markdown_enabled": {"on"},
		"author_type": {"Person"}, "author_name": {"Jane"}, "author_same_as": {"https://a.example\nhttps://b.example"},
		"feed_enabled": {"on"}, "feed_templates": {"Blog Post"}, "feed_categories": {"news, blog"}, "feed_limit": {"25"},
	}
	req := sessionReq("POST", "/cm/tools/seo", strings.NewReader(form.Encode()), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.SEOToolSave(rr, req)
	if rr.Code != 303 {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	cfg := seo.Get(context.Background())
	if cfg.TrainingPolicy != "disallow" || cfg.CrawlerOverrides["PerplexityBot"] != "disallow" || !cfg.ContentSignals ||
		cfg.MarkdownDisabled || cfg.AuthorName != "Jane" || len(cfg.AuthorSameAs) != 2 || cfg.FeedDisabled ||
		cfg.FeedLimit != 25 || len(cfg.FeedCategories) != 2 || cfg.FeedTemplates[0] != "Blog Post" {
		t.Errorf("saved config wrong: %+v", cfg)
	}

	// Invalid input re-renders with the error.
	form.Set("author_url", "nope")
	req = sessionReq("POST", "/cm/tools/seo", strings.NewReader(form.Encode()), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.SEOToolSave(rr, req)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "not an absolute") {
		t.Errorf("invalid save: %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.SEOToolPage(rr, httptest.NewRequest("GET", "/cm/tools/seo", nil))
	if rr.Code != 303 {
		t.Errorf("unauth: %d", rr.Code)
	}
}

func TestAnalyticsAIPage(t *testing.T) {
	h, _, cleanup := newSEOHandler(t)
	defer cleanup()
	h.analyticsService.RecordCrawlerHit("/x", "GPTBot/1.2")
	h.analyticsService.FlushBufferForTest()
	rr := httptest.NewRecorder()
	h.AnalyticsAIPage(rr, sessionReq("GET", "/cm/analytics/ai?range=7d", nil, nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "AI Traffic") || !strings.Contains(rr.Body.String(), "GPTBot") {
		t.Fatalf("ai page: %d", rr.Code)
	}
}

func TestAPISEOSettingsAndAITraffic(t *testing.T) {
	ah, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	ah.SetSEOService(services.NewSEOService(db))
	ah.SetBaseURL("https://site.example")

	// Partial update keeps unspecified fields.
	for _, body := range []string{`{"author_name":"Jane"}`, `{"training_policy":"disallow"}`} {
		rr := httptest.NewRecorder()
		ah.APIUpdateSEO(rr, authReq(http.MethodPut, "/api/v1/seo", strings.NewReader(body)))
		if rr.Code != 200 {
			t.Fatalf("PUT %s: %d %s", body, rr.Code, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	ah.APIGetSEO(rr, authReq(http.MethodGet, "/api/v1/seo", nil))
	var got map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&got)
	if got["author_name"] != "Jane" || got["training_policy"] != "disallow" {
		t.Errorf("partial updates not merged: %v", got)
	}
	if !strings.Contains(got["robots_txt_preview"].(string), "User-agent: GPTBot") || len(got["crawlers"].([]interface{})) == 0 {
		t.Errorf("GET /seo missing preview/crawlers")
	}

	rr = httptest.NewRecorder()
	ah.APIUpdateSEO(rr, authReq(http.MethodPut, "/api/v1/seo", strings.NewReader(`{"training_policy":"sometimes"}`)))
	if rr.Code != 400 {
		t.Errorf("invalid PUT: %d", rr.Code)
	}

	// AI traffic without analytics wired → 503; with → report.
	rr = httptest.NewRecorder()
	ah.APIAITraffic(rr, authReq(http.MethodGet, "/api/v1/analytics/ai", nil))
	if rr.Code != 503 {
		t.Errorf("no analytics: %d", rr.Code)
	}
	ah.SetAnalyticsService(services.NewAnalyticsService(context.Background(), db, "https://site.example"))
	rr = httptest.NewRecorder()
	ah.APIAITraffic(rr, authReq(http.MethodGet, "/api/v1/analytics/ai?days=7", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "by_purpose") {
		t.Errorf("ai traffic: %d %s", rr.Code, rr.Body.String())
	}
}

func TestServeSitemapRegeneratesWhenStale(t *testing.T) {
	h, _, cleanup := newSEOHandler(t)
	defer cleanup()
	tmplID := seedTemplate(t, h.db, "Standard Page", "standard-page")
	seedSEOPage(t, h, tmplID, "Standard Page", "Fresh Page", "/fresh-page", `<p>x</p>`, nil)
	// A stale file that predates the page.
	os.WriteFile("static/sitemap.xml", []byte("<urlset></urlset>"), 0644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes("static/sitemap.xml", old, old)
	defer exec.Command("git", "checkout", "static/sitemap.xml").Run()

	rr := httptest.NewRecorder()
	h.ServeSitemap(rr, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if !strings.Contains(rr.Body.String(), "/fresh-page") {
		t.Errorf("stale sitemap not regenerated:\n%s", rr.Body.String())
	}
}
