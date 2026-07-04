package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestClassifyUserAgent_AllBranches table-tests every browser/device branch.
func TestClassifyUserAgent_AllBranches(t *testing.T) {
	tests := []struct {
		ua   string
		want string
	}{
		{"", "Unknown"},
		{"Googlebot/2.1 (+http://www.google.com/bot.html)", "Bot"},
		{"my-crawler/1.0", "Bot"},
		{"spider-thing", "Bot"},
		{"Yahoo! Slurp", "Bot"},
		{"Wget/1.21", "Bot"},
		{"curl/8.0", "Bot"},
		{"python-requests/2.31", "Bot"},
		{"Go-http-client/1.1", "Bot"},
		{"HeadlessChrome/120", "Bot"},
		{"Mozilla/5.0 (Windows NT 10.0) Chrome/120 Edg/120.0", "Edge"},
		{"Mozilla/5.0 (Android; Mobile) Chrome/120 Edge/120.0", "Edge Mobile"},
		{"Mozilla/5.0 (Windows NT 10.0) Chrome/120 Safari/537.36", "Chrome"},
		{"Mozilla/5.0 (Linux; Android 14; Mobile) Chrome/120", "Chrome Mobile"},
		{"Mozilla/5.0 (X11; Linux) Gecko/20100101 Firefox/121.0", "Firefox"},
		{"Mozilla/5.0 (Android 14; Mobile; rv:121.0) Gecko Firefox/121.0", "Firefox Mobile"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit Version/17 Safari/605", "Safari"},
		{"Mozilla/5.0 (iPad; CPU OS 17) AppleWebKit Version/17 Safari/605", "Safari iPad"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17) AppleWebKit Version/17 Mobile/15E148 Safari/604", "Safari Mobile"},
		{"SomeRandomBrowser/1.0", "Other"},
		{"RandomThing Mobile/1.0", "Other Mobile"},
		{"RandomThing Tablet/1.0", "Other Tablet"},
	}
	for _, tt := range tests {
		if got := classifyUserAgent(tt.ua); got != tt.want {
			t.Errorf("classifyUserAgent(%q) = %q, want %q", tt.ua, got, tt.want)
		}
	}
}

// TestContentService_SettersAndTriggers exercises the trivial wiring setters
// and Cloudflare purge helpers (with Cloudflare disabled so no network I/O).
func TestContentService_SettersAndTriggers(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	ws := NewWebhookService(db)
	cs.SetWebhookService(ws)
	if cs.webhookService != ws {
		t.Error("SetWebhookService did not set the webhook service")
	}

	cf := NewCloudflareService(func() (string, string, bool) { return "", "", false }, "http://localhost")
	cs.SetCloudflareService(cf)
	if cs.cfService != cf {
		t.Error("SetCloudflareService did not set the Cloudflare service")
	}

	// Disabled Cloudflare — purge helpers spawn no-op goroutines.
	cs.PurgeCloudflareURLs([]string{"/a", "/b"})
	cs.PurgeCloudflareURLs(nil) // empty-path branch
	cs.PurgeCloudflareAll()

	// nil cfService branches
	cs2 := NewContentService(db)
	cs2.PurgeCloudflareURLs([]string{"/a"})
	cs2.PurgeCloudflareAll()

	// TriggerIndexRegen signals the coalescing worker; second call hits the
	// "already pending" default branch.
	cs.TriggerIndexRegen()
	cs.TriggerIndexRegen()

	cms := NewCommentService(db)
	cms.SetWebhookService(ws)
	if cms.webhookService != ws {
		t.Error("CommentService.SetWebhookService did not set the webhook service")
	}

	ts := NewTemplateService(db, cs)
	rq := NewRegenQueue(db, cs)
	ts.SetRegenQueue(rq)
	if ts.regenQueue != rq {
		t.Error("SetRegenQueue did not set the queue")
	}
}

// TestContentService_GetAuthorRole covers the zero-ID, missing-user, and
// found-user branches of getAuthorRole plus getContentAuthorRole fallback.
func TestContentService_GetAuthorRole(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()

	if got := cs.getAuthorRole(ctx, primitive.NilObjectID); got != "admin" {
		t.Errorf("zero ID: got %q, want admin", got)
	}
	if got := cs.getAuthorRole(ctx, primitive.NewObjectID()); got != "editor" {
		t.Errorf("unknown user: got %q, want editor", got)
	}

	userID, err := db.InsertOne(ctx, "users", bson.M{"email": "v@x.com", "role": "viewer"})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if got := cs.getAuthorRole(ctx, userID); got != "viewer" {
		t.Errorf("known user: got %q, want viewer", got)
	}

	// Empty-role user falls back to editor.
	emptyID, err := db.InsertOne(ctx, "users", bson.M{"email": "e@x.com"})
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if got := cs.getAuthorRole(ctx, emptyID); got != "editor" {
		t.Errorf("empty role: got %q, want editor", got)
	}

	// Content with no recorded versions → legacy/admin.
	c := &models.Content{ID: primitive.NewObjectID()}
	if got := cs.getContentAuthorRole(ctx, c); got != "admin" {
		t.Errorf("no versions: got %q, want admin", got)
	}
}

// TestQueryContentForDirective covers all filter keys and sort branches.
func TestQueryContentForDirective(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	docs := []interface{}{
		bson.M{"title": "Beta", "full_path": "/blog/beta", "folder_path": "/blog",
			"template_name": "Blog Post", "category": "news", "tags": []string{"go"},
			"published": true, "created_at": t1, "published_at": t1},
		bson.M{"title": "Alpha", "full_path": "/blog/alpha", "folder_path": "/blog",
			"template_name": "Blog Post", "category": "news", "tags": []string{"go"},
			"published": true, "created_at": t2, "published_at": t2},
		bson.M{"title": "Gamma", "full_path": "/docs/gamma", "folder_path": "/docs",
			"template_name": "Standard Page", "category": "docs",
			"published": true, "created_at": t2}, // no published_at → nil branch
		bson.M{"title": "Draft", "full_path": "/draft", "published": false},
	}
	if err := db.InsertMany(ctx, "content", docs); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	// Tag filter, default (title) sort ascending.
	got, err := cs.QueryContentForDirective(ctx, map[string]string{"tag": "go"}, "", "asc")
	if err != nil {
		t.Fatalf("tag query: %v", err)
	}
	if len(got) != 2 || got[0].Title != "Alpha" {
		t.Errorf("tag query: got %d items (first %q), want 2 with Alpha first", len(got), first(got))
	}

	// Title descending.
	got, err = cs.QueryContentForDirective(ctx, map[string]string{"category": "news"}, "title", "desc")
	if err != nil {
		t.Fatalf("category query: %v", err)
	}
	if len(got) != 2 || got[0].Title != "Beta" {
		t.Errorf("title desc: got first %q, want Beta", first(got))
	}

	// Template filter, created_at asc/desc.
	got, err = cs.QueryContentForDirective(ctx, map[string]string{"template": "Blog Post"}, "created_at", "asc")
	if err != nil || len(got) != 2 || got[0].Title != "Beta" {
		t.Errorf("created_at asc: err=%v first=%q", err, first(got))
	}
	got, err = cs.QueryContentForDirective(ctx, map[string]string{"template": "Blog Post"}, "created_at", "desc")
	if err != nil || len(got) != 2 || got[0].Title != "Alpha" {
		t.Errorf("created_at desc: err=%v first=%q", err, first(got))
	}

	// Folder filter (regex prefix) — includes the nil published_at item when /docs.
	got, err = cs.QueryContentForDirective(ctx, map[string]string{"folder": "/blog"}, "published_at", "asc")
	if err != nil || len(got) != 2 || got[0].Title != "Beta" {
		t.Errorf("published_at asc: err=%v first=%q", err, first(got))
	}
	got, err = cs.QueryContentForDirective(ctx, nil, "published_at", "desc")
	if err != nil || len(got) != 3 {
		t.Fatalf("published_at desc all: err=%v len=%d", err, len(got))
	}
	// nil PublishedAt sorts first on desc (comparator returns dir == -1 for nil).
	if got[0].Title != "Gamma" {
		t.Errorf("published_at desc: nil-date item should sort first, got %q", got[0].Title)
	}
	// published_at asc with a nil item covers the ai==nil / aj==nil branches.
	if _, err := cs.QueryContentForDirective(ctx, nil, "published_at", "asc"); err != nil {
		t.Errorf("published_at asc all: %v", err)
	}
}

func first(items []models.Content) string {
	if len(items) == 0 {
		return ""
	}
	return items[0].Title
}

// TestRegenerateIndexPages covers the full regeneration path: template with
// an lc:query directive, a published page using it, and the Cloudflare purge
// branch (disabled config, so no network I/O).
func TestRegenerateIndexPages(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	cs.SetCloudflareService(NewCloudflareService(func() (string, string, bool) { return "", "", false }, "http://localhost"))
	ctx := context.Background()

	// No templates at all → early return.
	cs.RegenerateIndexPages(ctx)

	// Template without lc:query → indexTemplateIDs empty → early return.
	if _, err := db.InsertOne(ctx, "templates", models.Template{
		Name: "Plain", Slug: "plain", HTMLLayout: "<div>{{.content}}</div>",
	}); err != nil {
		t.Fatalf("insert plain template: %v", err)
	}
	cs.RegenerateIndexPages(ctx)

	// Index template + published page.
	tmplID, err := db.InsertOne(ctx, "templates", models.Template{
		Name: "Index", Slug: "index-tmpl",
		HTMLLayout: `<div><!-- lc:query tag="go" sort="title" dir="asc" --><p>{{.content}}</p></div>`,
	})
	if err != nil {
		t.Fatalf("insert index template: %v", err)
	}
	if _, err := db.InsertOne(ctx, "content", bson.M{
		"template_id": tmplID, "template_name": "Index",
		"title": "Idx", "slug": "idx-page", "full_path": "/idx-page",
		"published": true, "use_theme": true,
		"data": bson.M{"content": "hello"},
	}); err != nil {
		t.Fatalf("insert content: %v", err)
	}

	cs.RegenerateIndexPages(ctx)
}

// TestTemplateService_RegenerateContentByTemplate covers the happy path and
// the FindMany error branch of the unexported regeneration helper.
func TestTemplateService_RegenerateContentByTemplate(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	defer db.SetFaultHook(nil)
	cs := NewContentService(db)
	ts := NewTemplateService(db, cs)
	ctx := context.Background()

	tmplID, err := db.InsertOne(ctx, "templates", models.Template{
		Name: "Regen", Slug: "regen-tmpl", HTMLLayout: "<div>{{.content}}</div>",
	})
	if err != nil {
		t.Fatalf("insert template: %v", err)
	}
	if _, err := db.InsertOne(ctx, "content", bson.M{
		"template_id": tmplID, "template_name": "Regen",
		"title": "Page", "slug": "regen-page", "full_path": "/regen-page",
		"published": true, "use_theme": true,
		"data": bson.M{"content": "hi"},
	}); err != nil {
		t.Fatalf("insert content: %v", err)
	}

	ts.regenerateContentByTemplate(ctx, tmplID)

	// Error branch: FindMany failure.
	db.SetFaultHook(func(op, _ string) error {
		if op == "FindMany" {
			return errors.New("injected")
		}
		return nil
	})
	ts.regenerateContentByTemplate(ctx, tmplID)
	db.SetFaultHook(nil)
}

// TestScheduler_RunOnce covers the due-content publish path and the query
// error branch of SchedulerService.runOnce.
func TestScheduler_RunOnce(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	defer db.SetFaultHook(nil)
	cs := NewContentService(db)
	sched := NewSchedulerService(db, cs)
	ctx := context.Background()

	tmplID := createTestTemplate(t, cs)
	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Scheduled", Slug: "scheduled-page",
		Data: map[string]interface{}{"content": "<p>soon</p>"},
	}
	if err := cs.CreateContent(ctx, content); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	past := time.Now().Add(-time.Minute)
	if err := db.UpdateOne(ctx, "content", bson.M{"_id": content.ID},
		bson.M{"$set": bson.M{"publish_at": past, "published": false}}); err != nil {
		t.Fatalf("set publish_at: %v", err)
	}

	sched.runOnce(ctx)

	got, err := cs.GetContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if !got.Published {
		t.Error("expected scheduler to publish due content")
	}

	// Error branch: FindMany failure.
	db.SetFaultHook(func(op, _ string) error {
		if op == "FindMany" {
			return errors.New("injected")
		}
		return nil
	})
	sched.runOnce(ctx)
	db.SetFaultHook(nil)

	// Start/Stop lifecycle for good measure.
	sched2 := NewSchedulerService(db, cs)
	startCtx, cancel := context.WithCancel(ctx)
	sched2.Start(startCtx)
	cancel()
	sched2.Stop()
}

// TestLinkChecker_MarkFailedAndRedirects covers NewLinkCheckerService's
// redirect policy and the markFailed helper.
func TestLinkChecker_MarkFailedAndRedirects(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewLinkCheckerService(db)
	ctx := context.Background()

	// markFailed flips a job to failed.
	jobID, err := db.InsertOne(ctx, "link_check_jobs", LinkCheckJob{
		Status: "running", BrokenLinks: []BrokenLink{}, StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	svc.markFailed(jobID, fmt.Errorf("boom"))
	job, err := svc.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Status != "failed" || job.FinishedAt == nil {
		t.Errorf("markFailed: status=%q finished=%v, want failed/non-nil", job.Status, job.FinishedAt)
	}

	// The client's CheckRedirect policy returns ErrUseLastResponse — a redirect
	// response is returned as-is, not followed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := svc.httpClient.Get(srv.URL)
	if err != nil {
		t.Fatalf("httpClient.Get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected redirect to not be followed (302), got %d", resp.StatusCode)
	}
}

// TestAnalytics_FlushBufferForTest_AndPrefField covers the exported test
// flush helper and the result-processing loops of queryPrefField.
func TestAnalytics_FlushBufferForTest_AndPrefField(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewAnalyticsService(context.Background(), db, "http://localhost:8082")
	defer svc.Stop()
	ctx := context.Background()

	svc.RecordPageView(ctx, "/flush-me", "https://example.org/", "Mozilla/5.0 Chrome/120 Safari/537")
	svc.FlushBufferForTest()

	since := time.Now().Add(-2 * time.Hour)
	until := time.Now().Add(2 * time.Hour)
	if views := svc.GetPageViews(ctx, since, until, "/flush-me"); views < 1 {
		t.Errorf("expected >=1 page view after FlushBufferForTest, got %d", views)
	}

	// Seed a legacy page_refs doc and query it via queryPrefField directly.
	hk := hourKey(time.Now())
	_, err := db.Collection(activityCollection).UpdateOne(ctx,
		bson.M{"user_id": hourlyUserID, "date": hk},
		bson.M{"$set": bson.M{
			"page_refs." + escapeMongoKey("/legacy||google.com"): 3,
			"page_refs.nopipe": 2,
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		t.Fatalf("seed page_refs: %v", err)
	}

	sinceKey := hourKey(since)
	untilKey := hourKey(until)

	stats := svc.queryPrefField(ctx, sinceKey, untilKey, "page_refs", "/legacy||", 10)
	if len(stats) != 1 || stats[0].Domain != "google.com" || stats[0].Hits != 3 {
		t.Errorf("queryPrefField: got %+v, want google.com/3", stats)
	}
	// Empty prefix matches the no-"||" key too → domain "" branch.
	stats = svc.queryPrefField(ctx, sinceKey, untilKey, "page_refs", "", 10)
	if len(stats) < 2 {
		t.Errorf("queryPrefField all: got %d results, want >=2", len(stats))
	}
}

// TestBulkCreateContent_Branches covers the partial-failure (duplicate key),
// published static-generation, webhook, and total-failure branches.
func TestBulkCreateContent_Branches(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	defer db.SetFaultHook(nil)
	cs := NewContentService(db)
	cs.SetWebhookService(NewWebhookService(db))
	ctx := context.Background()

	tmplID := createTestTemplate(t, cs)

	// CleanupCollections drops the content collection (and its indexes), so
	// recreate the unique (full_path, fork_id) index to exercise the
	// duplicate-key partial-failure branch.
	if _, err := db.Collection("content").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "full_path", Value: 1}, {Key: "fork_id", Value: 1}},
		Options: options.Index().SetUnique(true).SetSparse(true),
	}); err != nil {
		t.Fatalf("create unique index: %v", err)
	}

	// Pre-existing live page to force a duplicate-key write error.
	existing := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Existing", Slug: "bulk-dup",
		Data: map[string]interface{}{"content": "old"},
	}
	if err := cs.CreateContent(ctx, existing); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	items := []*models.Content{
		{TemplateID: tmplID, TemplateName: "Test Template", Title: "Dup", Slug: "bulk-dup",
			Data: map[string]interface{}{"content": "dup"}},
		{TemplateID: tmplID, TemplateName: "Test Template", Title: "Pub", Slug: "bulk-pub",
			Published: true, Data: map[string]interface{}{"content": "pub"}},
		{TemplateID: tmplID, TemplateName: "Test Template", Title: "Nested", Slug: "leaf",
			FolderPath: "/bulk-folder", Data: map[string]interface{}{"content": "n"}},
	}
	results := cs.BulkCreateContent(ctx, items, "bulk test")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Success {
		t.Error("expected duplicate-path item to fail")
	}
	if !results[1].Success || !results[2].Success {
		t.Errorf("expected non-duplicate items to succeed: %+v", results)
	}
	if results[2].FullPath != "/bulk-folder/leaf" {
		t.Errorf("folder path: got %q", results[2].FullPath)
	}

	// Total-failure branch via injected InsertManyUnordered error.
	db.SetFaultHook(func(op, _ string) error {
		if op == "InsertManyUnordered" {
			return errors.New("injected")
		}
		return nil
	})
	results = cs.BulkCreateContent(ctx, []*models.Content{
		{TemplateID: tmplID, Title: "F", Slug: "bulk-fail", Data: map[string]interface{}{"content": "f"}},
	}, "fail")
	db.SetFaultHook(nil)
	if len(results) != 1 || results[0].Success {
		t.Errorf("expected total failure, got %+v", results)
	}
}

// TestCommentService_Create_WebhookBranch covers the webhook-firing branch
// (including the mentions payload) of CommentService.Create.
func TestCommentService_Create_WebhookBranch(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cms := NewCommentService(db)
	cms.SetWebhookService(NewWebhookService(db))
	ctx := context.Background()

	c, err := cms.Create(ctx, primitive.NewObjectID(), primitive.NewObjectID(),
		"a@x.com", "Author", "hello @b", []primitive.ObjectID{primitive.NewObjectID()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID.IsZero() {
		t.Error("expected comment ID to be set")
	}

	// Empty text branch.
	if _, err := cms.Create(ctx, primitive.NewObjectID(), primitive.NewObjectID(),
		"a@x.com", "Author", "", nil); err == nil {
		t.Error("expected error for empty comment text")
	}
}
