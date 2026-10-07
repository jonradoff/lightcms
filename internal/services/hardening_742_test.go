package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 7.4.2 service-level fixes: deleting or unpublishing a fork copy says
// nothing about the live page at its path; import lookups never land on a
// fork copy; RepairForkDamage finds and repairs pre-7.4.1 damage.

// hooks742 wires a ContentService to a webhook receiver and a Cloudflare
// service whose configuration lookup counts purge attempts (it reports
// "disabled", so nothing is ever sent to Cloudflare).
func hooks742(t *testing.T) (cs *ContentService, hooks, purges *int32, ctx context.Context) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	t.Cleanup(cleanup)
	ctx = context.Background()

	hooks, purges = new(int32), new(int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hooks, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if _, err := db.InsertOne(ctx, "webhooks", &WebhookDoc{
		Name: "t", URL: srv.URL, Secret: "s", Active: true,
		Events:    []string{"content.delete", "content.unpublish"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("seed webhook: %v", err)
	}

	cs = NewContentService(db)
	cs.SetWebhookService(NewWebhookService(db))
	cs.SetCloudflareService(NewCloudflareService(func() (string, string, bool) {
		atomic.AddInt32(purges, 1)
		return "", "", false
	}, "http://localhost"))
	return cs, hooks, purges, ctx
}

// settle waits for the asynchronous webhook delivery and purge goroutines.
func settle(counter *int32, want int32) int32 {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(counter) >= want && want > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return atomic.LoadInt32(counter)
}

func seed742(t *testing.T, cs *ContentService, c models.Content) primitive.ObjectID {
	t.Helper()
	c.ID = primitive.NewObjectID()
	now := time.Now()
	c.CreatedAt, c.UpdatedAt = now, now
	if _, err := cs.db.InsertOne(context.Background(), "content", &c); err != nil {
		t.Fatalf("seed %s: %v", c.FullPath, err)
	}
	return c.ID
}

func TestDeleteContent_ForkCopyFiresNoWebhookOrPurge(t *testing.T) {
	cs, hooks, purges, ctx := hooks742(t)
	forkID := primitive.NewObjectID()
	live := seed742(t, cs, models.Content{Title: "Live", Slug: "c3-del", FullPath: "/c3-del", Published: true})
	fork := seed742(t, cs, models.Content{Title: "Copy", Slug: "c3-del", FullPath: "/c3-del", ForkID: &forkID})

	if err := cs.DeleteContent(ctx, fork); err != nil {
		t.Fatalf("delete fork copy: %v", err)
	}
	time.Sleep(1500 * time.Millisecond) // nothing should arrive; give it the chance to
	if n := atomic.LoadInt32(hooks); n != 0 {
		t.Errorf("deleting a fork copy delivered %d content.delete webhook(s) for the live path", n)
	}
	if n := atomic.LoadInt32(purges); n != 0 {
		t.Errorf("deleting a fork copy attempted %d Cloudflare purge(s) of the live path", n)
	}
	if c, _ := cs.GetContent(ctx, fork); c == nil || !c.Deleted {
		t.Error("fork copy was not marked deleted")
	}

	// The same call on the live page does both: the harness sees them.
	if err := cs.DeleteContent(ctx, live); err != nil {
		t.Fatalf("delete live page: %v", err)
	}
	if n := settle(hooks, 1); n != 1 {
		t.Errorf("deleting the live page delivered %d webhooks, want 1", n)
	}
	if n := settle(purges, 1); n != 1 {
		t.Errorf("deleting the live page attempted %d purges, want 1", n)
	}
}

func TestUnpublishContent_ForkCopyFiresNoWebhookOrPurge(t *testing.T) {
	cs, hooks, purges, ctx := hooks742(t)
	tmpl := primitive.NewObjectID()
	if _, err := cs.db.InsertOne(ctx, "templates", bson.M{"_id": tmpl, "name": "T", "slug": "t", "html_layout": "<p>{{.title}}</p>", "fields": bson.A{}}); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	forkID := primitive.NewObjectID()
	live := seed742(t, cs, models.Content{TemplateID: tmpl, Title: "Live", Slug: "c3-unpub", FullPath: "/c3-unpub", Published: true})
	// A copy left flagged published (pre-7.4.1 damage)
	fork := seed742(t, cs, models.Content{TemplateID: tmpl, Title: "Copy", Slug: "c3-unpub", FullPath: "/c3-unpub", ForkID: &forkID, Published: true})
	t.Cleanup(func() { os.Remove(staticPagePath("/c3-unpub")) })

	if err := cs.UnpublishContent(ctx, fork); err != nil {
		t.Fatalf("unpublish fork copy: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	if h, p := atomic.LoadInt32(hooks), atomic.LoadInt32(purges); h != 0 || p != 0 {
		t.Errorf("unpublishing a fork copy: %d webhook(s), %d purge(s) for the live path; want none", h, p)
	}
	if c, _ := cs.GetContent(ctx, fork); c == nil || c.Published {
		t.Error("fork copy still flagged published")
	}
	if c, _ := cs.GetContent(ctx, live); c == nil || !c.Published {
		t.Error("live page was unpublished")
	}

	if err := cs.UnpublishContent(ctx, live); err != nil {
		t.Fatalf("unpublish live page: %v", err)
	}
	if n := settle(hooks, 1); n != 1 {
		t.Errorf("unpublishing the live page delivered %d webhooks, want 1", n)
	}
	if n := settle(purges, 1); n != 1 {
		t.Errorf("unpublishing the live page attempted %d purges, want 1", n)
	}
}

// An import looks pages up by path and by source URL to decide what to
// update. A fork copy carries both from its live page and must never be the
// row it finds — whichever was inserted first.
func TestImportLookups_LiveOnly(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	is := NewImportService(db, cs)
	ctx := context.Background()
	forkID := primitive.NewObjectID()

	// Fork copy first, so an unfiltered FindOne returns it
	seed742(t, cs, models.Content{Title: "Copy", Slug: "c12", FullPath: "/c12", SourceURL: "https://feed.example/c12", ForkID: &forkID})
	live := seed742(t, cs, models.Content{Title: "Live", Slug: "c12", FullPath: "/c12", SourceURL: "https://feed.example/c12"})
	// A page that exists only in a fork
	seed742(t, cs, models.Content{Title: "Fork only", Slug: "c12-new", FullPath: "/c12-new", SourceURL: "https://feed.example/c12-new", ForkID: &forkID})

	if got := is.findByPath(ctx, "/c12"); got == nil || got.ID != live {
		t.Errorf("findByPath(/c12) = %+v, want the live page %s", got, live.Hex())
	}
	if got := is.findBySourceURL(ctx, "https://feed.example/c12"); got == nil || got.ID != live {
		t.Errorf("findBySourceURL = %+v, want the live page %s", got, live.Hex())
	}
	if got := is.findByPath(ctx, "/c12-new"); got != nil {
		t.Errorf("findByPath resolved a fork-only page: %s", got.ID.Hex())
	}
	if got := is.findBySourceURL(ctx, "https://feed.example/c12-new"); got != nil {
		t.Errorf("findBySourceURL resolved a fork-only page: %s", got.ID.Hex())
	}
}

func TestRepairForkDamage(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()

	tmpl := primitive.NewObjectID()
	if _, err := db.InsertOne(ctx, "templates", bson.M{"_id": tmpl, "name": "T", "slug": "t", "html_layout": "<article>{{.title}}</article>", "fields": bson.A{}}); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	forkID := primitive.NewObjectID()
	page := func(c models.Content) primitive.ObjectID { c.TemplateID = tmpl; return seed742(t, cs, c) }

	// Live published pages: one with its file, two without (one in a folder),
	// one whose stored hash still matches the HTML it lost.
	intact := page(models.Content{Title: "Intact", Slug: "c14-intact", FullPath: "/c14-intact", Published: true})
	lost := page(models.Content{Title: "Lost", Slug: "c14-lost", FullPath: "/c14-lost", Published: true})
	lostDeep := page(models.Content{Title: "Lost deep", Slug: "page", FolderPath: "/c14-dir", FullPath: "/c14-dir/page", Published: true})
	lostHashed := page(models.Content{Title: "Lost hashed", Slug: "c14-hashed", FullPath: "/c14-hashed", Published: true})
	// Not candidates: a draft, a deleted page, a published page with no path
	draft := page(models.Content{Title: "Draft", Slug: "c14-draft", FullPath: "/c14-draft"})
	page(models.Content{Title: "Deleted", Slug: "c14-deleted", FullPath: "/c14-deleted", Published: true, Deleted: true})
	// Fork copies: flagged with a live page, flagged without one, unflagged
	flagged := page(models.Content{Title: "Flagged copy", Slug: "c14-intact", FullPath: "/c14-intact", ForkID: &forkID, Published: true})
	flaggedNew := page(models.Content{Title: "Flagged new", Slug: "c14-forknew", FullPath: "/c14-forknew", ForkID: &forkID, Published: true})
	clean := page(models.Content{Title: "Clean copy", Slug: "c14-lost", FullPath: "/c14-lost", ForkID: &forkID})

	files := []string{"/c14-intact", "/c14-lost", "/c14-dir/page", "/c14-hashed", "/c14-draft", "/c14-deleted", "/c14-forknew"}
	t.Cleanup(func() {
		for _, f := range files {
			os.Remove(staticPagePath(f))
		}
		os.Remove(filepath.Dir(staticPagePath("/c14-dir/page")))
	})
	for _, f := range files {
		os.Remove(staticPagePath(f))
	}
	// Generate the intact and hashed pages for real, then lose the hashed one's file
	for _, id := range []primitive.ObjectID{intact, lostHashed} {
		c, _ := cs.GetContent(ctx, id)
		if err := cs.GenerateStaticPage(ctx, c); err != nil {
			t.Fatalf("generate: %v", err)
		}
	}
	if c, _ := cs.GetContent(ctx, lostHashed); c.ContentHash == "" {
		t.Fatal("setup: generated page has no content hash")
	}
	hashedBefore, _ := cs.GetContent(ctx, lostHashed)
	if err := os.Remove(staticPagePath("/c14-hashed")); err != nil {
		t.Fatalf("setup: remove hashed page's file: %v", err)
	}
	intactHTML, _ := os.ReadFile(staticPagePath("/c14-intact"))
	updatedBefore := map[primitive.ObjectID]time.Time{}
	for _, id := range []primitive.ObjectID{intact, lost, lostDeep, lostHashed, flagged, flaggedNew} {
		c, _ := cs.GetContent(ctx, id)
		updatedBefore[id] = c.UpdatedAt
	}

	ids := func(pages []ForkDamagePage) map[string]ForkDamagePage {
		m := map[string]ForkDamagePage{}
		for _, p := range pages {
			m[p.ID] = p
		}
		return m
	}
	exists := func(path string) bool { _, err := os.Stat(staticPagePath(path)); return err == nil }

	// ---- dry run: lists everything, changes nothing
	rep, err := cs.RepairForkDamage(ctx, true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !rep.DryRun || rep.ForkCopiesCleared != 0 || rep.Regenerated != 0 {
		t.Errorf("dry run reported changes: %+v", rep)
	}
	fc := ids(rep.ForkCopiesPublished)
	if len(fc) != 2 || fc[flagged.Hex()].ID == "" || fc[flaggedNew.Hex()].ID == "" || fc[clean.Hex()].ID != "" {
		t.Errorf("dry run fork copies = %+v, want the two flagged copies", rep.ForkCopiesPublished)
	}
	if p := fc[flagged.Hex()]; p.HasLivePage == nil || !*p.HasLivePage || p.ForkID != forkID.Hex() || p.Path != "/c14-intact" {
		t.Errorf("flagged copy entry = %+v", p)
	}
	if p := fc[flaggedNew.Hex()]; p.HasLivePage == nil || *p.HasLivePage {
		t.Errorf("fork-only flagged copy should report has_live_page=false: %+v", p)
	}
	ms := ids(rep.MissingStatic)
	if len(ms) != 3 || ms[lost.Hex()].ID == "" || ms[lostDeep.Hex()].ID == "" || ms[lostHashed.Hex()].ID == "" {
		t.Errorf("dry run missing static = %+v, want the three lost pages", rep.MissingStatic)
	}
	if ms[intact.Hex()].ID != "" || ms[draft.Hex()].ID != "" {
		t.Errorf("dry run lists a page that is not missing its file: %+v", rep.MissingStatic)
	}
	if rep.PagesChecked != 4 {
		t.Errorf("pages_checked = %d, want 4 live published pages", rep.PagesChecked)
	}
	if c, _ := cs.GetContent(ctx, flagged); !c.Published {
		t.Error("dry run cleared a published flag")
	}
	if exists("/c14-lost") || exists("/c14-dir/page") || exists("/c14-hashed") {
		t.Error("dry run generated a file")
	}

	// ---- real run
	rep, err = cs.RepairForkDamage(ctx, false)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if rep.DryRun || rep.ForkCopiesCleared != 2 || rep.Regenerated != 3 || len(rep.RegenerateFailed) != 0 || len(rep.MissingStatic) != 3 {
		t.Errorf("repair report = %+v", rep)
	}
	for _, id := range []primitive.ObjectID{flagged, flaggedNew} {
		c, _ := cs.GetContent(ctx, id)
		if c.Published || c.ForkID == nil {
			t.Errorf("fork copy %s: published=%v fork=%v after repair", c.Title, c.Published, c.ForkID)
		}
		if !c.UpdatedAt.Equal(updatedBefore[id]) {
			t.Errorf("fork copy %s: updated_at moved", c.Title)
		}
	}
	for path, title := range map[string]string{"/c14-lost": "Lost", "/c14-dir/page": "Lost deep", "/c14-hashed": "Lost hashed"} {
		got, err := os.ReadFile(staticPagePath(path))
		if err != nil || string(got) != "<article>"+title+"</article>" {
			t.Errorf("%s after repair: %q, %v", path, got, err)
		}
	}
	// Untouched: the intact page's file, drafts, deleted and fork-only pages
	if got, _ := os.ReadFile(staticPagePath("/c14-intact")); string(got) != string(intactHTML) {
		t.Errorf("intact page's file changed: %q", got)
	}
	if exists("/c14-draft") || exists("/c14-deleted") || exists("/c14-forknew") {
		t.Error("repair generated a file for a draft, deleted or fork-only page")
	}
	// A repair is not an edit: no version, no content_modified_at, same hash
	for _, id := range []primitive.ObjectID{lost, lostDeep, lostHashed} {
		c, _ := cs.GetContent(ctx, id)
		if !c.Published || !c.UpdatedAt.Equal(updatedBefore[id]) {
			t.Errorf("%s: published=%v, updated_at moved=%v", c.Title, c.Published, !c.UpdatedAt.Equal(updatedBefore[id]))
		}
		if n, _ := db.Count(ctx, "content_versions", bson.M{"content_id": id}); n != 0 {
			t.Errorf("%s: repair saved %d version(s)", c.Title, n)
		}
	}
	after, _ := cs.GetContent(ctx, lostHashed)
	if after.ContentHash != hashedBefore.ContentHash {
		t.Errorf("hash changed: %q -> %q", hashedBefore.ContentHash, after.ContentHash)
	}
	if (after.ContentModifiedAt == nil) != (hashedBefore.ContentModifiedAt == nil) ||
		(after.ContentModifiedAt != nil && !after.ContentModifiedAt.Equal(*hashedBefore.ContentModifiedAt)) {
		t.Errorf("content_modified_at moved: %v -> %v", hashedBefore.ContentModifiedAt, after.ContentModifiedAt)
	}
	if c, _ := cs.GetContent(ctx, lost); c.ContentModifiedAt != nil {
		t.Errorf("repair stamped content_modified_at on a page whose content did not change: %v", c.ContentModifiedAt)
	}

	// ---- idempotent
	rep, err = cs.RepairForkDamage(ctx, false)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if len(rep.ForkCopiesPublished) != 0 || len(rep.MissingStatic) != 0 || rep.ForkCopiesCleared != 0 || rep.Regenerated != 0 {
		t.Errorf("second repair found more to do: %+v", rep)
	}
}

// A page whose template is gone cannot be rendered: it is reported, and the
// rest of the repair carries on.
func TestRepairForkDamage_ReportsFailures(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()

	id := seed742(t, cs, models.Content{TemplateID: primitive.NewObjectID(), Title: "Orphan", Slug: "c14-orphan", FullPath: "/c14-orphan", Published: true})
	os.Remove(staticPagePath("/c14-orphan"))
	rep, err := cs.RepairForkDamage(ctx, false)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if rep.Regenerated != 0 || len(rep.RegenerateFailed) != 1 || rep.RegenerateFailed[0].ID != id.Hex() || rep.RegenerateFailed[0].Error == "" {
		t.Errorf("report = %+v, want one failure for the orphan page", rep)
	}
}
