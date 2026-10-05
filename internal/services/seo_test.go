package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
)

func TestIdentifyCrawler(t *testing.T) {
	cases := map[string]string{ // UA → expected token ("" = none)
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)":                               "GPTBot",
		"Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)":                                                                    "ClaudeBot",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Claude-SearchBot/1.0)":                                                 "Claude-SearchBot",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ChatGPT-User/1.0; +https://openai.com/bot)":                            "ChatGPT-User",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; Perplexity-User/1.0; +https://perplexity.ai/perplexity-user)":          "Perplexity-User",
		"Mozilla/5.0 (compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)":                                                           "OAI-SearchBot",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15 (Applebot/0.1)": "Applebot",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)":                                                             "Googlebot",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0 Safari/537.36":                       "",
		"": "",
	}
	for ua, want := range cases {
		got := ""
		if c := IdentifyCrawler(ua); c != nil {
			got = c.Token
		}
		if got != want {
			t.Errorf("IdentifyCrawler(%.50q) = %q, want %q", ua, got, want)
		}
	}
	// Known crawlers without a generic "bot" marker still count as bots.
	if classifyUserAgent("Mozilla/5.0 (compatible; Perplexity-User/1.0; +https://perplexity.ai/perplexity-user)") != "Bot" {
		t.Error("Perplexity-User should classify as Bot")
	}
	// Robots-only tokens resolve by token but never match a UA.
	if CrawlerByToken("Google-Extended") == nil || IdentifyCrawler("Google-Extended") != nil {
		t.Error("Google-Extended should be a robots-only token")
	}
}

func TestAIAssistantDetection(t *testing.T) {
	for host, want := range map[string]string{
		"chatgpt.com": "ChatGPT", "www.perplexity.ai": "Perplexity", "claude.ai": "Claude",
		"gemini.google.com": "Gemini", "copilot.microsoft.com": "Copilot", "google.com": "", "example.com": "",
	} {
		if got := AIAssistantForHost(host); got != want {
			t.Errorf("AIAssistantForHost(%s) = %q, want %q", host, got, want)
		}
	}
	for q, want := range map[string]string{
		"utm_source=chatgpt.com":       "https://chatgpt.com/",
		"a=1&utm_source=perplexity":    "https://perplexity.ai/",
		"utm_source=www.perplexity.ai": "https://perplexity.ai/",
		"utm_source=newsletter":        "",
		"":                             "",
		"utm_source=%zz":               "",
	} {
		if got := AIReferrerFromQuery(q); got != want {
			t.Errorf("AIReferrerFromQuery(%q) = %q, want %q", q, got, want)
		}
	}
}

func TestSEOConfigDefaultsAndValidate(t *testing.T) {
	d := SEOConfig{}.withDefaults()
	if d.TrainingPolicy != CrawlerAllow || d.AISearchPolicy != CrawlerAllow || d.UserFetchPolicy != CrawlerAllow {
		t.Error("policies should default to allow")
	}
	if d.FeedLimit != DefaultFeedLimit || len(d.FeedTemplates) == 0 || len(d.FeedCategories) == 0 {
		t.Errorf("feed defaults wrong: %+v", d)
	}
	if all := (SEOConfig{FeedAllPages: true}).withDefaults(); len(all.FeedTemplates) != 0 {
		t.Error("FeedAllPages should not get default templates")
	}

	bad := []SEOConfig{
		{TrainingPolicy: "maybe"},
		{CrawlerOverrides: map[string]string{"NotABot": "disallow"}},
		{CrawlerOverrides: map[string]string{"GPTBot": "sometimes"}},
		{AuthorType: "Robot"},
		{AuthorURL: "not a url"},
		{AuthorSameAs: []string{"ftp://x.y"}},
		{FeedLimit: 9999},
	}
	for i, c := range bad {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d should fail validation: %+v", i, c)
		}
	}
	ok := SEOConfig{TrainingPolicy: " Disallow ", CrawlerOverrides: map[string]string{"GPTBot": "default", "CCBot": "ALLOW"},
		AuthorSameAs: []string{" https://a.b/x ", "", "https://a.b/x"}, RobotsExtra: "a\r\nb\n"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if ok.TrainingPolicy != "disallow" || ok.CrawlerOverrides["CCBot"] != "allow" || len(ok.CrawlerOverrides) != 1 ||
		len(ok.AuthorSameAs) != 1 || ok.RobotsExtra != "a\nb" {
		t.Errorf("normalization wrong: %+v", ok)
	}
}

func TestRobotsTxt(t *testing.T) {
	// Default: unchanged behavior — allow everything, sitemap, no AI section.
	r := SEOConfig{}.RobotsTxt("https://ex.net/", []string{"https://ex.net/feed.xml"})
	for _, want := range []string{"User-agent: *\nAllow: /", "Sitemap: https://ex.net/sitemap.xml", "# Feed: https://ex.net/feed.xml"} {
		if !strings.Contains(r, want) {
			t.Errorf("default robots missing %q:\n%s", want, r)
		}
	}
	if strings.Contains(r, "Disallow") || strings.Contains(r, "Content-Signal") {
		t.Errorf("default robots should not restrict anything:\n%s", r)
	}

	// Block training, override: allow GPTBot, block PerplexityBot and Googlebot.
	c := SEOConfig{TrainingPolicy: CrawlerDisallow, ContentSignals: true, RobotsExtra: "User-agent: *\nDisallow: /private/",
		CrawlerOverrides: map[string]string{"GPTBot": CrawlerAllow, "PerplexityBot": CrawlerDisallow, "Googlebot": CrawlerDisallow}}
	r = c.RobotsTxt("https://ex.net", nil)
	for _, want := range []string{
		"User-agent: ClaudeBot\n", "User-agent: Google-Extended\n", "User-agent: CCBot\n",
		"User-agent: PerplexityBot\n", "User-agent: Googlebot\n", "Disallow: /\n",
		"Content-Signal: search=yes, ai-input=yes, ai-train=no",
		"Disallow: /private/",
	} {
		if !strings.Contains(r, want) {
			t.Errorf("robots missing %q:\n%s", want, r)
		}
	}
	for _, absent := range []string{"User-agent: GPTBot", "User-agent: OAI-SearchBot", "User-agent: Bingbot", "User-agent: ChatGPT-User"} {
		if strings.Contains(r, absent) {
			t.Errorf("robots should not contain %q:\n%s", absent, r)
		}
	}
	// Everything AI blocked → ai-input=no.
	r = SEOConfig{TrainingPolicy: CrawlerDisallow, AISearchPolicy: CrawlerDisallow, UserFetchPolicy: CrawlerDisallow, ContentSignals: true}.RobotsTxt("https://ex.net", nil)
	if !strings.Contains(r, "ai-input=no, ai-train=no") {
		t.Errorf("content signal wrong:\n%s", r)
	}
}

func TestSEOServiceSaveGet(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()
	s := NewSEOService(db)

	if got := s.Get(ctx); got.TrainingPolicy != CrawlerAllow || got.FeedLimit != DefaultFeedLimit {
		t.Errorf("unset config should return defaults: %+v", got)
	}
	if err := s.Save(ctx, SEOConfig{TrainingPolicy: "maybe"}); err == nil {
		t.Error("invalid config saved")
	}
	if err := s.Save(ctx, SEOConfig{TrainingPolicy: CrawlerDisallow, AuthorName: "Jane Doe", FeedLimit: 10}); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(ctx); got.TrainingPolicy != CrawlerDisallow || got.AuthorName != "Jane Doe" || got.FeedLimit != 10 {
		t.Errorf("Get after Save: %+v", got)
	}
	// A fresh service (other machine) reads it from the DB.
	if got := NewSEOService(db).Get(ctx); got.AuthorName != "Jane Doe" {
		t.Errorf("fresh service read: %+v", got)
	}
	if raw := s.GetRaw(ctx); raw.AISearchPolicy != "" {
		t.Error("GetRaw should not apply defaults")
	}
	// Doesn't disturb SiteConfig.
	n, _ := db.Settings().CountDocuments(ctx, bson.M{"type": seoConfigType})
	if n != 1 {
		t.Errorf("expected one seo_config doc, got %d", n)
	}
	var nilSvc *SEOService
	if nilSvc.Get(ctx).TrainingPolicy != CrawlerAllow {
		t.Error("nil service should return defaults")
	}
}

func TestExtractFAQ(t *testing.T) {
	html := `<h2>Intro</h2><p>Not a question?</p>
<details><summary>What is LightCMS?</summary><p>A CMS for agents.</p></details>
<details><summary>Show more</summary><p>Collapsible, not a question.</p></details>
<h2>Frequently Asked Questions</h2>
<div><h3>Is it free?</h3><p>Yes, MIT licensed.</p>
<h3>Does it need MongoDB?</h3><p>Yes.</p><p>Atlas free tier works.</p></div>
<h2>Next section</h2><h3>Unrelated?</h3><p>Outside the FAQ.</p>`
	items := ExtractFAQ(html)
	got := map[string]string{}
	for _, it := range items {
		got[it.Question] = it.Answer
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 FAQ items, got %d: %+v", len(items), items)
	}
	if got["What is LightCMS?"] != "A CMS for agents." || got["Is it free?"] != "Yes, MIT licensed." || got["Does it need MongoDB?"] != "Yes. Atlas free tier works." {
		t.Errorf("FAQ items wrong: %+v", got)
	}
	if len(ExtractFAQ("<p>No questions here.</p>")) != 0 {
		t.Error("false positive FAQ")
	}
}

func TestAnalyticsAITraffic(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()
	a := NewAnalyticsService(ctx, db, "https://ex.net")
	defer a.Stop()

	a.RecordCrawlerHit("/a", "Mozilla/5.0 (compatible; GPTBot/1.2)")
	a.RecordCrawlerHit("/a", "Mozilla/5.0 (compatible; ChatGPT-User/1.0)")
	a.RecordCrawlerHit("/llms.txt", "Mozilla/5.0 (compatible; ClaudeBot/1.0)")
	a.RecordCrawlerHit("/b", "Mozilla/5.0 (compatible; Googlebot/2.1)")
	a.RecordCrawlerHit("/b", "Mozilla/5.0 Chrome/129") // not a crawler: ignored
	human := "Mozilla/5.0 (Macintosh) Chrome/129.0 Safari/537.36"
	a.RecordPageView(ctx, "/a", "https://chatgpt.com/", human)
	a.RecordPageView(ctx, "/a", "https://chatgpt.com/c/123", human)
	a.RecordPageView(ctx, "/c", "https://www.perplexity.ai/search", human)
	a.RecordPageView(ctx, "/c", "https://news.ycombinator.com/", human)
	a.FlushBufferForTest()

	until := time.Now().Add(time.Hour)
	rep, err := a.GetAITraffic(ctx, until.Add(-48*time.Hour), until, 10)
	if err != nil {
		t.Fatal(err)
	}
	if rep.AICrawlerHits != 3 || rep.SearchEngineHits != 1 {
		t.Errorf("crawler totals: ai=%d search=%d", rep.AICrawlerHits, rep.SearchEngineHits)
	}
	purpose := map[string]int{}
	for _, p := range rep.ByPurpose {
		purpose[p.Purpose] = p.Hits
	}
	if purpose[CrawlerPurposeTraining] != 2 || purpose[CrawlerPurposeUserFetch] != 1 || purpose[CrawlerPurposeSearch] != 1 {
		t.Errorf("by purpose: %+v", purpose)
	}
	if len(rep.TopCrawledPages) != 2 || rep.TopCrawledPages[0].Path != "/a" || rep.TopCrawledPages[0].Hits != 2 {
		t.Errorf("top crawled (search engines excluded): %+v", rep.TopCrawledPages)
	}
	refs := map[string]int{}
	for _, r := range rep.Referrals {
		refs[r.Assistant] = r.Hits
	}
	if refs["ChatGPT"] != 2 || refs["Perplexity"] != 1 || rep.ReferralTotal != 3 {
		t.Errorf("referrals: %+v total=%d", refs, rep.ReferralTotal)
	}
	if len(rep.TopLandingPages) != 2 || rep.TopLandingPages[0].Path != "/a" {
		t.Errorf("landing pages: %+v", rep.TopLandingPages)
	}
	if len(rep.Daily) < 2 {
		t.Errorf("daily series should cover the range: %d days", len(rep.Daily))
	}
	total := 0
	for _, d := range rep.Daily {
		for _, n := range d.ByPurpose {
			total += n
		}
	}
	if total != 4 {
		t.Errorf("daily total = %d, want 4", total)
	}
}

func TestContentModifiedAtAndNoIndexNotify(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	svc := NewContentService(h.svc.db)
	svc.SetIndexNowService(h.svc)
	tmplID := createTestTemplate(t, svc)
	drain := func() map[string]bool {
		h.svc.mu.Lock()
		defer h.svc.mu.Unlock()
		out := map[string]bool{}
		for p := range h.svc.pending {
			out[p] = true
		}
		h.svc.pending = make(map[string]*indexNowPending)
		return out
	}

	c := &models.Content{TemplateID: tmplID, Title: "Mod", Slug: "mod", Published: true, Data: map[string]interface{}{"content": "v1"}}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetContent(ctx, c.ID)
	if got.ContentModifiedAt == nil {
		t.Fatal("content_modified_at not stamped on first render")
	}
	first := *got.ContentModifiedAt

	// Site-wide re-render: not a content change.
	time.Sleep(10 * time.Millisecond)
	svc.RegenerateAllContent(ctx)
	got, _ = svc.GetContent(ctx, c.ID)
	if !got.ContentModifiedAt.Equal(first) {
		t.Error("regenerate-all moved content_modified_at")
	}
	// Real edit: moves.
	got.Data["content"] = "v2"
	svc.UpdateContent(ctx, got)
	got, _ = svc.GetContent(ctx, c.ID)
	if !got.ContentModifiedAt.After(first) {
		t.Error("edit did not move content_modified_at")
	}
	if got.ModifiedAt() != *got.ContentModifiedAt {
		t.Error("ModifiedAt should prefer content_modified_at")
	}
	drain()

	// Hiding from search notifies (engines must recrawl to see noindex) and
	// excludes the page from full submissions.
	got.NoIndex = true
	svc.UpdateContent(ctx, got)
	if p := drain(); !p["/mod"] {
		t.Errorf("noindex change not notified: %v", p)
	}
	got, _ = svc.GetContent(ctx, c.ID)
	if !got.NoIndex {
		t.Fatal("noindex not persisted")
	}
	paths, _ := h.svc.publishedPaths(ctx)
	for _, p := range paths {
		if p == "/mod" {
			t.Error("noindex page included in full IndexNow submission")
		}
	}

	// Admin-editor style transitions via NotifyLiveChange.
	before := *got
	after := *got
	after.FullPath = "/moved"
	svc.NotifyLiveChange(ctx, &before, &after)
	if p := drain(); !p["/mod"] || !p["/moved"] {
		t.Errorf("move: %v", p)
	}
	svc.NotifyLiveChange(ctx, &before, nil)
	if p := drain(); !p["/mod"] {
		t.Errorf("delete: %v", p)
	}
	draft := before
	draft.Published = false
	svc.NotifyLiveChange(ctx, &draft, &draft)
	if p := drain(); len(p) != 0 {
		t.Errorf("draft edit notified: %v", p)
	}
}

func TestPublishedAtStampAndBackfill(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	// Created already-published: gets a published date.
	c := &models.Content{TemplateID: tmplID, Title: "Pub", Slug: "pub-stamp", Published: true, Data: map[string]interface{}{"content": "x"}}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetContent(ctx, c.ID)
	if got.PublishedAt == nil {
		t.Fatal("published_at not stamped on create")
	}
	// Drafts don't.
	d := &models.Content{TemplateID: tmplID, Title: "Draft", Slug: "draft-stamp", Data: map[string]interface{}{"content": "x"}}
	svc.CreateContent(ctx, d)
	if got, _ := svc.GetContent(ctx, d.ID); got.PublishedAt != nil {
		t.Error("draft got a published_at")
	}

	// Legacy rows without published_at.
	created := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	legacy := []interface{}{
		bson.M{"title": "Old1", "full_path": "/old1", "published": true, "created_at": created},
		bson.M{"title": "Old2", "full_path": "/old2", "published": true, "created_at": created, "published_at": nil},
		bson.M{"title": "OldDraft", "full_path": "/old-draft", "published": false, "created_at": created},
		bson.M{"title": "OldDeleted", "full_path": "/old-del", "published": true, "deleted": true, "created_at": created},
	}
	svc.db.Collection("content").InsertMany(ctx, legacy)

	n, err := svc.BackfillPublishedDates(ctx, true)
	if err != nil || n != 2 {
		t.Fatalf("dry run = %d, %v; want 2", n, err)
	}
	if n, _ := svc.BackfillPublishedDates(ctx, false); n != 2 {
		t.Fatalf("backfill updated %d, want 2", n)
	}
	var old models.Content
	svc.db.FindOne(ctx, "content", bson.M{"full_path": "/old1"}, &old)
	if old.PublishedAt == nil || !old.PublishedAt.Equal(created) {
		t.Errorf("published_at = %v, want created_at %v", old.PublishedAt, created)
	}
	var draft models.Content
	svc.db.FindOne(ctx, "content", bson.M{"full_path": "/old-draft"}, &draft)
	if draft.PublishedAt != nil {
		t.Error("draft backfilled")
	}
	if n, _ := svc.BackfillPublishedDates(ctx, true); n != 0 {
		t.Errorf("not idempotent: %d left", n)
	}
}

func TestRemoveStaticPageEmptyPathKeepsHomepage(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	os.MkdirAll("content/generated", 0755)
	home := "content/generated/index.html"
	if _, err := os.Stat(home); err != nil {
		os.WriteFile(home, []byte("home"), 0644)
		defer os.Remove(home)
	}
	svc.removeStaticPage("")
	if _, err := os.Stat(home); err != nil {
		t.Fatal("empty full_path removed the homepage static file")
	}
}
