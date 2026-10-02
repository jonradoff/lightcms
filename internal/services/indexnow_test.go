package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestIndexNowHostIneligible(t *testing.T) {
	cases := map[string]bool{ // host → eligible
		"metavert.io":       true,
		"www.example.co.uk": true,
		"myapp.fly.dev":     true,
		"localhost":         false,
		"127.0.0.1":         false,
		"10.0.0.5":          false,
		"intranet":          false,
		"site.local":        false,
		"cms.test":          false,
		"box.internal":      false,
		"example.com":       false,
		"blog.example.org":  false,
		"printer.home.arpa": false,
		"METAVERT.IO.":      true,
	}
	for host, want := range cases {
		if got := indexNowHostIneligible(host) == ""; got != want {
			t.Errorf("%s: eligible=%v, want %v (%s)", host, got, want, indexNowHostIneligible(host))
		}
	}
}

func TestIndexNowIneligible(t *testing.T) {
	if r := NewIndexNowService(nil, "https://metavert.io", true).Ineligible(); !strings.Contains(r, "development") {
		t.Errorf("dev mode should be ineligible, got %q", r)
	}
	if r := NewIndexNowService(nil, "", false).Ineligible(); r == "" {
		t.Error("empty BASE_URL should be ineligible")
	}
	if r := NewIndexNowService(nil, "ftp://metavert.io", false).Ineligible(); r == "" {
		t.Error("non-http scheme should be ineligible")
	}
	if r := NewIndexNowService(nil, "http://localhost:8082", false).Ineligible(); r == "" {
		t.Error("localhost should be ineligible")
	}
	if r := NewIndexNowService(nil, "https://metavert.io/", false).Ineligible(); r != "" {
		t.Errorf("public prod site should be eligible, got %q", r)
	}
	// Ineligible installs never queue anything.
	s := NewIndexNowService(nil, "http://localhost:8082", false)
	s.Notify("/a")
	if len(s.pending) != 0 {
		t.Error("ineligible service queued a path")
	}
}

func TestIndexNowPageURL(t *testing.T) {
	s := NewIndexNowService(nil, "https://metavert.io", false)
	if got := s.pageURL("/blog/hello world"); got != "https://metavert.io/blog/hello%20world" {
		t.Errorf("pageURL escaping: %s", got)
	}
	if got := s.pageURL("/"); got != "https://metavert.io/" {
		t.Errorf("homepage: %s", got)
	}
	s2 := NewIndexNowService(nil, "https://example.net/site/", false)
	if got := s2.pageURL("/a"); got != "https://example.net/site/a" {
		t.Errorf("base path: %s", got)
	}
}

// indexNowHarness is a fake site (serving the key file) plus a fake IndexNow endpoint.
type indexNowHarness struct {
	body     string // engine response body
	svc      *IndexNowService
	site     *httptest.Server
	engine   *httptest.Server
	mu       sync.Mutex
	requests []map[string]interface{}
	status   int    // engine response status
	siteKey  string // overrides the key the site serves ("" = real key)
}

func newIndexNowHarness(t *testing.T) (*indexNowHarness, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	h := &indexNowHarness{status: http.StatusOK}
	h.site = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := h.svc.KeyForFile(r.Context())
		h.mu.Lock()
		if h.siteKey != "" {
			key = h.siteKey
		}
		h.mu.Unlock()
		if r.URL.Path == "/"+key+".txt" {
			w.Write([]byte(key))
			return
		}
		http.NotFound(w, r)
	}))
	h.engine = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		h.mu.Lock()
		h.requests = append(h.requests, body)
		status, respBody := h.status, h.body
		h.mu.Unlock()
		w.WriteHeader(status)
		w.Write([]byte(respBody))
	}))
	h.svc = NewIndexNowService(db, h.site.URL, false)
	h.svc.allowPrivateHosts = true
	h.svc.SetEndpoint(h.engine.URL)
	return h, func() {
		h.site.Close()
		h.engine.Close()
		cleanup()
	}
}

func (h *indexNowHarness) reqs() []map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]map[string]interface{}(nil), h.requests...)
}

func urlList(req map[string]interface{}) []string {
	var out []string
	for _, u := range req["urlList"].([]interface{}) {
		out = append(out, u.(string))
	}
	return out
}

func TestIndexNowConfigKeyStable(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()

	cfg, err := h.svc.GetConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Key) != 32 {
		t.Fatalf("expected 32-char key, got %q", cfg.Key)
	}
	if cfg.Disabled {
		t.Error("IndexNow should be on by default")
	}
	again, _ := h.svc.GetConfig(ctx)
	if again.Key != cfg.Key {
		t.Error("key changed between reads")
	}
	n, _ := h.svc.db.Settings().CountDocuments(ctx, bson.M{"type": indexNowConfigType})
	if n != 1 {
		t.Errorf("expected 1 config doc, got %d", n)
	}

	newKey, err := h.svc.RegenerateKey(ctx)
	if err != nil || newKey == cfg.Key || len(newKey) != 32 {
		t.Fatalf("RegenerateKey: %q %v", newKey, err)
	}
	if got := h.svc.KeyForFile(ctx); got != newKey {
		t.Errorf("KeyForFile = %q, want new key", got)
	}

	if err := h.svc.SetEnabled(ctx, false); err != nil {
		t.Fatal(err)
	}
	st, _ := h.svc.Status(ctx)
	if st.Enabled || st.Active {
		t.Error("expected disabled status")
	}
	if st.Key != newKey || st.KeyURL != h.site.URL+"/"+newKey+".txt" {
		t.Errorf("KeyURL = %s", st.KeyURL)
	}
}

func TestIndexNowFlushBatchesAndCooldown(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	cfg, _ := h.svc.GetConfig(ctx)

	h.svc.Notify("/a", "b", "/a") // duplicate + missing slash
	h.svc.flush(ctx, time.Now())  // inside debounce window → nothing sent
	if len(h.reqs()) != 0 {
		t.Fatal("flushed before debounce elapsed")
	}

	h.svc.flush(ctx, time.Now().Add(indexNowDebounce+time.Second))
	reqs := h.reqs()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	req := reqs[0]
	if req["key"] != cfg.Key || req["keyLocation"] != h.site.URL+"/"+cfg.Key+".txt" {
		t.Errorf("bad key fields: %v", req)
	}
	if !strings.HasPrefix(h.site.URL, "http://"+req["host"].(string)) {
		t.Errorf("host = %v", req["host"])
	}
	urls := urlList(req)
	if len(urls) != 2 || urls[0] != h.site.URL+"/a" || urls[1] != h.site.URL+"/b" {
		t.Errorf("urlList = %v", urls)
	}
	if len(h.svc.pending) != 0 {
		t.Error("pending not cleared after success")
	}

	// Re-notified within cooldown: held back, then sent once cooldown passes.
	h.svc.Notify("/a")
	h.svc.flush(ctx, time.Now().Add(time.Minute))
	if len(h.reqs()) != 1 {
		t.Fatal("resubmitted during cooldown")
	}
	h.svc.flush(ctx, time.Now().Add(indexNowCooldown+time.Minute))
	if len(h.reqs()) != 2 {
		t.Fatal("not resubmitted after cooldown")
	}

	st, _ := h.svc.Status(ctx)
	if st.TotalSubmitted != 3 || len(st.History) != 2 || st.LastStatus != 200 || !st.Verified {
		t.Errorf("status bookkeeping wrong: total=%d history=%d last=%d verified=%v",
			st.TotalSubmitted, len(st.History), st.LastStatus, st.Verified)
	}
}

func TestIndexNowKeyMismatchBlocksSubmission(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	h.siteKey = "someoneelseskey0000000000000000" // BASE_URL served by a different instance

	h.svc.Notify("/a")
	h.svc.flush(ctx, time.Now().Add(time.Minute))
	if len(h.reqs()) != 0 {
		t.Fatal("submitted despite key-file mismatch")
	}
	st, _ := h.svc.Status(ctx)
	if st.Verified || st.VerifyError == "" || st.LastError == "" {
		t.Errorf("expected verify error recorded, got %+v", st)
	}
}

func TestIndexNowRetryAndDrop(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()

	h.status = http.StatusTooManyRequests
	h.svc.Notify("/a")
	now := time.Now().Add(time.Minute)
	h.svc.flush(ctx, now)
	if p := h.svc.pending["/a"]; p == nil || p.attempts != 1 || !p.notBefore.After(now) {
		t.Fatalf("429 should requeue with backoff, got %+v", p)
	}

	h.status = http.StatusForbidden
	h.svc.flush(ctx, now.Add(time.Hour))
	if len(h.svc.pending) != 0 {
		t.Error("403 should drop the batch, not retry")
	}
	st, _ := h.svc.Status(ctx)
	if !strings.Contains(st.LastError, "403") || st.TotalSubmitted != 0 {
		t.Errorf("expected recorded 403, got %q total=%d", st.LastError, st.TotalSubmitted)
	}
}

func TestIndexNowDisabledDropsPending(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	h.svc.Notify("/a")
	h.svc.GetConfig(ctx) // ensure doc exists
	// Disabled from another machine: this one still has the path queued.
	h.svc.db.Settings().UpdateOne(ctx, bson.M{"type": indexNowConfigType}, bson.M{"$set": bson.M{"disabled": true}})
	h.svc.flush(ctx, time.Now().Add(time.Minute))
	if len(h.reqs()) != 0 || len(h.svc.pending) != 0 {
		t.Error("disabled IndexNow should drop pending without submitting")
	}
}

func seedIndexNowContent(t *testing.T, h *indexNowHarness) {
	t.Helper()
	ctx := context.Background()
	fork := primitive.NewObjectID()
	docs := []interface{}{
		models.Content{Title: "Home", Slug: "", FullPath: "/", Published: true},
		models.Content{Title: "Live", Slug: "live", FullPath: "/live", Published: true},
		models.Content{Title: "Draft", Slug: "draft", FullPath: "/draft", Published: false},
		models.Content{Title: "Gone", Slug: "gone", FullPath: "/gone", Published: true, Deleted: true},
		models.Content{Title: "Fork", Slug: "live-fork", FullPath: "/live-fork", Published: true, ForkID: &fork},
	}
	if _, err := h.svc.db.Collection("content").InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}
}

func TestIndexNowCatchUpAndSubmitAll(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	seedIndexNowContent(t, h)

	n, err := h.svc.CatchUp(ctx)
	if err != nil || n != 2 {
		t.Fatalf("CatchUp = %d, %v; want 2 live pages", n, err)
	}
	urls := urlList(h.reqs()[0])
	if len(urls) != 2 || urls[0] != h.site.URL+"/" || urls[1] != h.site.URL+"/live" {
		t.Errorf("catch-up urls = %v (drafts, deleted pages and forks must be excluded)", urls)
	}

	// Runs once.
	if n, _ := h.svc.CatchUp(ctx); n != 0 || len(h.reqs()) != 1 {
		t.Error("catch-up ran twice")
	}
	// Manual full submit is rate-limited right after a full submission.
	if _, err := h.svc.SubmitAll(ctx); err == nil || !strings.Contains(err.Error(), "try again") {
		t.Errorf("SubmitAll should be rate-limited, got %v", err)
	}
	h.svc.db.Settings().UpdateOne(ctx, bson.M{"type": indexNowConfigType},
		bson.M{"$set": bson.M{"last_full_at": time.Now().Add(-2 * time.Hour)}})
	if n, err := h.svc.SubmitAll(ctx); err != nil || n != 2 {
		t.Errorf("SubmitAll = %d, %v", n, err)
	}
}

func TestIndexNowCatchUpReleasesClaimOnFailure(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	seedIndexNowContent(t, h)
	h.status = http.StatusInternalServerError

	if _, err := h.svc.CatchUp(ctx); err == nil {
		t.Fatal("expected failure")
	}
	cfg, _ := h.svc.GetConfig(ctx)
	if cfg.InitialSubmitAt != nil {
		t.Fatal("failed catch-up should release its claim so it retries")
	}
	h.status = http.StatusAccepted
	if n, err := h.svc.CatchUp(ctx); err != nil || n != 2 {
		t.Fatalf("retry CatchUp = %d, %v", n, err)
	}
}

func TestIndexNowCatchUpSkippedWhenDisabled(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	seedIndexNowContent(t, h)
	h.svc.SetEnabled(ctx, false)
	if n, err := h.svc.CatchUp(ctx); n != 0 || err != nil || len(h.reqs()) != 0 {
		t.Errorf("disabled catch-up submitted: %d %v", n, err)
	}
	if _, err := h.svc.SubmitAll(ctx); err == nil {
		t.Error("SubmitAll should fail when disabled")
	}
}

// Content lifecycle → IndexNow queue.
func TestContentLifecycleNotifiesIndexNow(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	svc := NewContentService(h.svc.db)
	svc.SetIndexNowService(h.svc)
	tmplID := createTestTemplate(t, svc)

	pending := func() map[string]bool {
		h.svc.mu.Lock()
		defer h.svc.mu.Unlock()
		out := map[string]bool{}
		for p := range h.svc.pending {
			out[p] = true
		}
		h.svc.pending = make(map[string]*indexNowPending)
		return out
	}

	// Draft create: nothing.
	c := &models.Content{TemplateID: tmplID, Title: "IN Test", Slug: "in-test", Data: map[string]interface{}{"content": "v1"}}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatal(err)
	}
	if p := pending(); len(p) != 0 {
		t.Errorf("draft create queued %v", p)
	}

	// Publish: queued.
	if err := svc.PublishContent(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if p := pending(); !p["/in-test"] {
		t.Errorf("publish did not queue: %v", p)
	}

	// Re-save with identical content: HTML unchanged → nothing.
	got, _ := svc.GetContent(ctx, c.ID)
	svc.UpdateContent(ctx, got)
	if p := pending(); len(p) != 0 {
		t.Errorf("no-op save queued %v", p)
	}

	// Edit: queued.
	got, _ = svc.GetContent(ctx, c.ID)
	got.Data["content"] = "v2"
	svc.UpdateContent(ctx, got)
	if p := pending(); !p["/in-test"] {
		t.Errorf("edit did not queue: %v", p)
	}

	// Site-wide regeneration: suppressed.
	if err := svc.RegenerateAllContent(ctx); err != nil {
		t.Fatal(err)
	}
	if p := pending(); len(p) != 0 {
		t.Errorf("regenerate-all queued %v", p)
	}

	// Rename: old and new paths.
	got, _ = svc.GetContent(ctx, c.ID)
	got.Slug = "in-test-2"
	svc.UpdateContent(ctx, got)
	if p := pending(); !p["/in-test"] || !p["/in-test-2"] {
		t.Errorf("rename should queue old+new: %v", p)
	}

	// Unpublish: queued.
	if err := svc.UnpublishContent(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if p := pending(); !p["/in-test-2"] {
		t.Errorf("unpublish did not queue: %v", p)
	}

	// Delete of a published page: queued.
	svc.PublishContent(ctx, c.ID)
	pending()
	if err := svc.DeleteContent(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if p := pending(); !p["/in-test-2"] {
		t.Errorf("delete did not queue: %v", p)
	}

	// Explicitly suppressed context.
	svc.notifyIndexNow(WithoutIndexNow(ctx), "/x")
	if p := pending(); len(p) != 0 {
		t.Errorf("suppressed ctx queued %v", p)
	}
}

func TestIndexNowVerificationPendingIsRetried(t *testing.T) {
	h, cleanup := newIndexNowHarness(t)
	defer cleanup()
	ctx := context.Background()
	h.status = http.StatusForbidden
	h.body = `{"errorCode":"SiteVerificationNotCompleted","message":"Site Verification is not completed."}`

	h.svc.Notify("/a")
	now := time.Now().Add(time.Minute)
	h.svc.flush(ctx, now)
	p := h.svc.pending["/a"]
	if p == nil || p.notBefore.Sub(now) != indexNowVerifyPendingRetry {
		t.Fatalf("verification-pending 403 should be retried in %s, got %+v", indexNowVerifyPendingRetry, p)
	}

	// Catch-up surfaces the sentinel so Start() can retry sooner, and releases its claim.
	seedIndexNowContent(t, h)
	if _, err := h.svc.CatchUp(ctx); !errors.Is(err, errIndexNowVerificationPending) {
		t.Errorf("CatchUp err = %v, want verification-pending", err)
	}
	if cfg, _ := h.svc.GetConfig(ctx); cfg.InitialSubmitAt != nil {
		t.Error("claim not released")
	}

	// Once verified, the queued path goes through.
	h.status, h.body = http.StatusOK, ""
	h.svc.flush(ctx, now.Add(indexNowVerifyPendingRetry+time.Second))
	if len(h.svc.pending) != 0 {
		t.Error("path not submitted after verification completed")
	}
}
