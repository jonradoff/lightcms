package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/netguard"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 7.4.3 service-level fixes: a fork copy written through the service layer
// has no effect outside its fork; lc:query and the wikilink index are
// live-only; webhook delivery goes through the SSRF guard.

// hooks743 is hooks742 with the webhook subscribed to every content event.
func hooks743(t *testing.T) (cs *ContentService, hooks, purges *int32, ctx context.Context) {
	t.Helper()
	cs, hooks, purges, ctx = hooks742(t)
	if err := cs.db.UpdateOne(ctx, "webhooks", bson.M{"name": "t"}, bson.M{"$set": bson.M{
		"events": []string{"content.create", "content.update", "content.publish", "content.unpublish", "content.delete"},
	}}); err != nil {
		t.Fatalf("widen webhook: %v", err)
	}
	return cs, hooks, purges, ctx
}

func seedTemplate743(t *testing.T, cs *ContentService, layout string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	if _, err := cs.db.InsertOne(context.Background(), "templates", bson.M{"_id": id, "name": "T743", "slug": "t743", "html_layout": layout, "fields": bson.A{}}); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	return id
}

// Creating, saving, renaming and reverting a fork copy through the service
// changes nothing outside the fork: no webhook, no Cloudflare purge, no
// rewrite of [[wikilinks]] in live pages, no static file.
func TestForkCopy_ServiceWritesHaveNoSideEffectsOutsideTheFork(t *testing.T) {
	cs, hooks, purges, ctx := hooks743(t)
	tmpl := seedTemplate743(t, cs, "<p>{{.title}}</p>")
	forkID := primitive.NewObjectID()

	// A live page, and a live page that links to it by title and by path
	seed742(t, cs, models.Content{TemplateID: tmpl, Title: "Target Page", Slug: "f1-target", FullPath: "/f1-target", Published: true})
	linker := seed742(t, cs, models.Content{TemplateID: tmpl, Title: "Linker", Slug: "f1-linker", FullPath: "/f1-linker",
		Data: map[string]interface{}{"body": "See [[Target Page]] and [[/f1-target|here]]."}})
	t.Cleanup(func() { os.Remove(staticPagePath("/f1-target")); os.Remove(staticPagePath("/f1-renamed")) })

	// 1. Create a copy through the service
	fork := &models.Content{TemplateID: tmpl, Title: "Target Page", Slug: "f1-target", ForkID: &forkID,
		Data: map[string]interface{}{"body": "copy"}}
	if err := cs.CreateContent(ctx, fork); err != nil {
		t.Fatalf("create fork copy: %v", err)
	}

	// 2. Rename it (title and path) through the service
	fork.Title = "Renamed In Fork"
	fork.Slug = "f1-renamed"
	if err := cs.UpdateContent(ctx, fork, "rename in fork"); err != nil {
		t.Fatalf("update fork copy: %v", err)
	}

	// 3. Revert it to its first version (title and path change back)
	if err := cs.RevertToVersion(ctx, fork.ID, 1); err != nil {
		t.Fatalf("revert fork copy: %v", err)
	}
	fork.Title, fork.Slug = "Renamed Again", "f1-renamed"
	if err := cs.UpdateContent(ctx, fork); err != nil {
		t.Fatalf("second update: %v", err)
	}

	time.Sleep(2500 * time.Millisecond) // the wikilink rewrite and webhooks are asynchronous
	if n := atomic.LoadInt32(hooks); n != 0 {
		t.Errorf("writing a fork copy delivered %d webhook(s); want none", n)
	}
	if n := atomic.LoadInt32(purges); n != 0 {
		t.Errorf("writing a fork copy attempted %d Cloudflare purge(s); want none", n)
	}
	got, _ := cs.GetContent(ctx, linker)
	if body, _ := got.Data["body"].(string); body != "See [[Target Page]] and [[/f1-target|here]]." {
		t.Errorf("renaming a fork copy rewrote wikilinks in a live page: %q", body)
	}
	for _, p := range []string{"/f1-target", "/f1-renamed"} {
		if _, err := os.Stat(staticPagePath(p)); err == nil {
			t.Errorf("a fork copy write produced the static file for %s", p)
		}
	}

	// Bulk create of a copy fires nothing either
	res := cs.BulkCreateContent(ctx, []*models.Content{{TemplateID: tmpl, Title: "Bulk copy", Slug: "f1-bulk", ForkID: &forkID, Published: true}}, "")
	if len(res) != 1 || !res[0].Success {
		t.Fatalf("bulk create: %+v", res)
	}
	time.Sleep(1500 * time.Millisecond)
	if n := atomic.LoadInt32(hooks); n != 0 {
		t.Errorf("bulk-creating a fork copy delivered %d webhook(s); want none", n)
	}
	if _, err := os.Stat(staticPagePath("/f1-bulk")); err == nil {
		os.Remove(staticPagePath("/f1-bulk"))
		t.Error("bulk-creating a published-flagged fork copy wrote a static file")
	}

	// The harness does see the same operations on a live page
	live := &models.Content{TemplateID: tmpl, Title: "Live One", Slug: "f1-live"}
	if err := cs.CreateContent(ctx, live); err != nil {
		t.Fatalf("create live: %v", err)
	}
	if n := settle(hooks, 1); n != 1 {
		t.Errorf("creating a live page delivered %d webhooks, want 1", n)
	}
	live.Title = "Live One Edited"
	if err := cs.UpdateContent(ctx, live); err != nil {
		t.Fatalf("update live: %v", err)
	}
	if n := settle(hooks, 2); n != 2 {
		t.Errorf("updating a live page delivered %d webhooks in total, want 2", n)
	}
}

// A live rename still rewrites wikilinks (the guard is for copies only).
func TestLivePage_RenameStillRewritesWikilinks(t *testing.T) {
	cs, _, _, ctx := hooks743(t)
	tmpl := seedTemplate743(t, cs, "<p>{{.title}}</p>")
	target := &models.Content{TemplateID: tmpl, Title: "Old Name", Slug: "f1b-target"}
	if err := cs.CreateContent(ctx, target); err != nil {
		t.Fatalf("create: %v", err)
	}
	linker := seed742(t, cs, models.Content{TemplateID: tmpl, Title: "Linker", Slug: "f1b-linker", FullPath: "/f1b-linker",
		Data: map[string]interface{}{"body": "See [[Old Name]]."}})
	target.Title = "New Name"
	if err := cs.UpdateContent(ctx, target); err != nil {
		t.Fatalf("update: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, _ := cs.GetContent(ctx, linker)
		if body, _ := got.Data["body"].(string); body == "See [[New Name]]." {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("live rename did not rewrite the wikilink: %q", body)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// lc:query results and the wikilink index feed public pages: a fork copy
// flagged published (pre-7.4.1 damage) and a soft-deleted page are in neither.
func TestDirectiveQueryAndWikilinkIndex_LiveOnly(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()
	forkID := primitive.NewObjectID()

	seed742(t, cs, models.Content{Title: "F3 Live", Slug: "f3-live", FullPath: "/f3-live", Category: "f3cat", Tags: []string{"f3tag"}, Published: true})
	seed742(t, cs, models.Content{Title: "F3 Copy", Slug: "f3-copy", FullPath: "/f3-copy", Category: "f3cat", Tags: []string{"f3tag"}, Published: true, ForkID: &forkID})
	seed742(t, cs, models.Content{Title: "F3 Deleted", Slug: "f3-deleted", FullPath: "/f3-deleted", Category: "f3cat", Tags: []string{"f3tag"}, Published: true, Deleted: true})
	seed742(t, cs, models.Content{Title: "F3 Draft", Slug: "f3-draft", FullPath: "/f3-draft", Category: "f3cat", Tags: []string{"f3tag"}})

	for _, filter := range []map[string]string{{"category": "f3cat"}, {"tag": "f3tag"}} {
		items, err := cs.QueryContentForDirective(ctx, filter, "title", "asc")
		if err != nil {
			t.Fatalf("query %v: %v", filter, err)
		}
		var titles []string
		for _, it := range items {
			titles = append(titles, it.Title)
		}
		if strings.Join(titles, ",") != "F3 Live" {
			t.Errorf("lc:query %v returned %v, want only the live published page", filter, titles)
		}
	}

	out, err := cs.processQueryDirectives(ctx, `<ul><!-- lc:query category="f3cat" --></ul>`)
	if err != nil {
		t.Fatalf("directive: %v", err)
	}
	if !strings.Contains(out, "/f3-live") || strings.Contains(out, "f3-copy") || strings.Contains(out, "f3-deleted") || strings.Contains(out, "f3-draft") {
		t.Errorf("rendered lc:query = %q", out)
	}

	idx := cs.buildWikilinkIndex(ctx)
	if _, ok := idx.titleToPath["f3 live"]; !ok {
		t.Error("wikilink index lacks the live page")
	}
	for _, title := range []string{"f3 copy", "f3 deleted", "f3 draft"} {
		if p, ok := idx.titleToPath[title]; ok {
			t.Errorf("wikilink index resolves %q to %s", title, p)
		}
	}
}

// useGuardedTransport puts the SSRF guard back for one test (TestMain swaps
// it out so the package's webhook receivers on 127.0.0.1 are reachable).
func useGuardedTransport(t *testing.T) {
	t.Helper()
	prev := outboundTransport
	outboundTransport = netguard.Transport
	t.Cleanup(func() { outboundTransport = prev })
}

// A webhook pointed at the server's own network is never contacted, the
// refusal is in the delivery log, and it is not retried.
func TestWebhookDelivery_BlocksPrivateDestinations(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	useGuardedTransport(t)
	ctx := context.Background()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":"):]

	ws := NewWebhookService(db)
	ws.retryDelays = []time.Duration{50 * time.Millisecond, 50 * time.Millisecond}
	targets := map[string]string{
		"loopback":       srv.URL, // http://127.0.0.1:port
		"localhost":      "http://localhost" + port,
		"ipv6-loopback":  "http://[::1]" + port,
		"mapped":         "http://[::ffff:127.0.0.1]" + port,
		"link-local":     "http://169.254.169.254/latest/meta-data/",
		"private":        "http://10.0.0.1/hook",
		"decimal":        "http://2130706433" + port, // 127.0.0.1 as an integer
		"rebind-wrapper": "http://127.0.0.1.nip.invalid" + port,
	}
	ids := map[string]primitive.ObjectID{}
	for name, u := range targets {
		wh, err := ws.Create(ctx, name, u, "secret", []string{"content.create"}, true)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		ids[name] = wh.ID
	}
	ws.FireEvent(ctx, "content.create", map[string]string{"id": "x"})

	deadline := time.Now().Add(20 * time.Second)
	for {
		n, _ := db.Count(ctx, "webhook_deliveries", bson.M{})
		if n >= int64(len(targets)) || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond) // a retry, if one were scheduled, would land here
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("the loopback receiver was contacted %d time(s)", n)
	}
	for name, id := range ids {
		ds, err := ws.ListDeliveries(ctx, id, 10)
		if err != nil || len(ds) == 0 {
			t.Errorf("%s: no delivery log entry (err=%v)", name, err)
			continue
		}
		d := ds[len(ds)-1]
		if d.Success || d.Error == "" {
			t.Errorf("%s: delivery logged as success=%v error=%q", name, d.Success, d.Error)
		}
		switch name {
		case "decimal", "rebind-wrapper":
			// not resolvable (or resolved and blocked): failing is what matters
		default:
			if !strings.Contains(d.Error, "blocked") {
				t.Errorf("%s: delivery error %q does not say the destination was blocked", name, d.Error)
			}
			if len(ds) != 1 {
				t.Errorf("%s: %d delivery attempts; a blocked destination is not retried", name, len(ds))
			}
		}
	}
}

// The guard is what production services are built with.
func TestOutboundClients_UseTheGuard(t *testing.T) {
	useGuardedTransport(t)
	if ws := NewWebhookService(nil); ws.httpClient.Transport != netguard.Transport {
		t.Error("NewWebhookService does not dial through netguard.Transport")
	}
	if lc := NewLinkCheckerService(nil); lc.httpClient.Transport != netguard.Transport {
		t.Error("NewLinkCheckerService does not dial through netguard.Transport")
	}
}

// A webhook fired from an HTTP handler is delivered after the handler has
// returned and its request context has been cancelled.
func TestWebhookFireEvent_SurvivesCallerCancellation(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	ws := NewWebhookService(db)
	wh, err := ws.Create(context.Background(), "after-request", srv.URL, "secret", []string{"content.create"}, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ws.FireEvent(ctx, "content.create", map[string]string{"id": "x"})
	cancel() // the handler returns

	deadline := time.Now().Add(20 * time.Second)
	for atomic.LoadInt32(&hits) == 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("receiver was contacted %d time(s), want 1", atomic.LoadInt32(&hits))
	}
	time.Sleep(300 * time.Millisecond)
	ds, err := ws.ListDeliveries(context.Background(), wh.ID, 10)
	if err != nil || len(ds) != 1 || !ds[0].Success {
		t.Fatalf("delivery log: %+v (err=%v), want one successful delivery", ds, err)
	}
}
