package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Tests for the 7.4.1 follow-ups: path lookups, the search-and-replace
// streams and the embedding batch all work on live pages only.

// seedForkCopy inserts a fork copy at fullPath and returns its ID.
func seedForkCopy(t *testing.T, cs *ContentService, forkID primitive.ObjectID, title, fullPath string) primitive.ObjectID {
	t.Helper()
	now := time.Now()
	c := &models.Content{
		ID: primitive.NewObjectID(), Title: title, Slug: fullPath[1:], FullPath: fullPath,
		ForkID: &forkID, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := cs.db.InsertOne(context.Background(), "content", c); err != nil {
		t.Fatalf("seedForkCopy %s: %v", fullPath, err)
	}
	return c.ID
}

// streamIDs drains a content cursor into a set of IDs.
func streamIDs(t *testing.T, cur *mongo.Cursor, err error) map[primitive.ObjectID]bool {
	t.Helper()
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	ctx := context.Background()
	defer cur.Close(ctx)
	ids := map[primitive.ObjectID]bool{}
	for cur.Next(ctx) {
		var c models.Content
		if err := cur.Decode(&c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		ids[c.ID] = true
	}
	return ids
}

func TestGetContentByPath_LiveOnly(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	fs := NewForkService(db, cs)
	ctx := context.Background()
	forkID := primitive.NewObjectID()

	// The fork copy is inserted first so an unfiltered lookup would be free
	// to return it.
	copyID := seedForkCopy(t, cs, forkID, "Copy", "/gp-both")
	liveID := seedLiveContent(t, db, "Live", "/gp-both")
	forkOnlyID := seedForkCopy(t, cs, forkID, "Fork Only", "/gp-fork-only")

	// Live page wins over a copy at the same path.
	got, err := cs.GetContentByPath(ctx, "/gp-both")
	if err != nil {
		t.Fatalf("GetContentByPath live: %v", err)
	}
	if got.ID != liveID || got.ForkID != nil {
		t.Errorf("GetContentByPath = %s (fork_id %v), want the live page %s (copy is %s)", got.ID.Hex(), got.ForkID, liveID.Hex(), copyID.Hex())
	}
	// The case-insensitive fallback is live-only too.
	if got, err := cs.GetContentByPath(ctx, "/GP-Both"); err != nil || got.ID != liveID {
		t.Errorf("case-insensitive lookup = %v, %v; want the live page", got, err)
	}

	// A path that exists only as a fork copy is not found, in any casing.
	for _, p := range []string{"/gp-fork-only", "/GP-FORK-ONLY"} {
		if got, err := cs.GetContentByPath(ctx, p); err == nil {
			t.Errorf("GetContentByPath(%s) resolved to fork copy %s, want not found", p, got.ID.Hex())
		}
	}

	// Fork-aware lookups still reach the copies.
	if got, err := fs.GetForkPageByPath(ctx, forkID, "/gp-fork-only"); err != nil || got.ID != forkOnlyID {
		t.Errorf("GetForkPageByPath fork-only = %v, %v", got, err)
	}
	if got, err := fs.GetForkPageByPath(ctx, forkID, "/gp-both"); err != nil || got.ID != copyID {
		t.Errorf("GetForkPageByPath both = %v, %v", got, err)
	}
}

func TestUpsertContent_DoesNotUpdateForkCopy(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)
	forkID := primitive.NewObjectID()
	copyID := seedForkCopy(t, svc, forkID, "Fork Only", "/up-fork-only")

	created, err := svc.UpsertContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Upserted", Slug: "up-fork-only",
		Data: map[string]interface{}{"content": "x"},
	}, "upsert")
	if err != nil {
		t.Fatalf("UpsertContent: %v", err)
	}
	if !created {
		t.Error("upsert at a fork-only path updated the fork copy instead of creating a live page")
	}
	cp, err := svc.GetContent(ctx, copyID)
	if err != nil {
		t.Fatalf("reload copy: %v", err)
	}
	if cp.Title != "Fork Only" {
		t.Errorf("fork copy title = %q, want it untouched", cp.Title)
	}
	live, err := svc.GetContentByPath(ctx, "/up-fork-only")
	if err != nil || live.ForkID != nil || live.Title != "Upserted" {
		t.Errorf("live page after upsert = %+v, %v", live, err)
	}
}

func TestStreamContent_LiveOnlyByDefault(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()
	forkID := primitive.NewObjectID()

	liveID := seedLiveContent(t, db, "Live", "/st-page")
	copyID := seedForkCopy(t, cs, forkID, "Copy", "/st-page")
	forkOnlyID := seedForkCopy(t, cs, forkID, "Fork Only", "/st-new")

	cur, err := cs.StreamContent(ctx, false)
	ids := streamIDs(t, cur, err)
	if !ids[liveID] || ids[copyID] || ids[forkOnlyID] {
		t.Errorf("StreamContent default: live=%v copy=%v forkOnly=%v, want true/false/false", ids[liveID], ids[copyID], ids[forkOnlyID])
	}
	cur, err = cs.StreamContent(ctx, false, true)
	ids = streamIDs(t, cur, err)
	if !ids[liveID] || !ids[copyID] || !ids[forkOnlyID] {
		t.Errorf("StreamContent includeForks: live=%v copy=%v forkOnly=%v, want all true", ids[liveID], ids[copyID], ids[forkOnlyID])
	}

	// Scoped: an explicit ID list or folder never pulls copies in by default.
	all := []primitive.ObjectID{liveID, copyID, forkOnlyID}
	cur, err = cs.StreamContentScoped(ctx, ContentScope{ContentIDs: all})
	ids = streamIDs(t, cur, err)
	if !ids[liveID] || ids[copyID] || ids[forkOnlyID] {
		t.Errorf("StreamContentScoped ids: live=%v copy=%v forkOnly=%v, want true/false/false", ids[liveID], ids[copyID], ids[forkOnlyID])
	}
	cur, err = cs.StreamContentScoped(ctx, ContentScope{FolderPath: "/st-new"})
	if ids = streamIDs(t, cur, err); len(ids) != 0 {
		t.Errorf("StreamContentScoped folder returned fork copies: %v", ids)
	}
	cur, err = cs.StreamContentScoped(ctx, ContentScope{ContentIDs: all, IncludeForks: true})
	ids = streamIDs(t, cur, err)
	if !ids[liveID] || !ids[copyID] || !ids[forkOnlyID] {
		t.Errorf("StreamContentScoped IncludeForks: live=%v copy=%v forkOnly=%v, want all true", ids[liveID], ids[copyID], ids[forkOnlyID])
	}

	// ForkCopyIDs picks out the copies, in request order, once each.
	got, err := cs.ForkCopyIDs(ctx, []primitive.ObjectID{forkOnlyID, liveID, copyID, forkOnlyID, primitive.NewObjectID()})
	if err != nil {
		t.Fatalf("ForkCopyIDs: %v", err)
	}
	if len(got) != 2 || got[0] != forkOnlyID || got[1] != copyID {
		t.Errorf("ForkCopyIDs = %v, want [%s %s]", got, forkOnlyID.Hex(), copyID.Hex())
	}
	if got, err := cs.ForkCopyIDs(ctx, nil); err != nil || got != nil {
		t.Errorf("ForkCopyIDs(nil) = %v, %v", got, err)
	}
}

func TestEmbeddings_IgnoreForkCopies(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake, calls := newFakeOllama(t)
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)

	liveID := seedSearchContent(t, db, &models.Content{
		Title: "Live Page", Slug: "emb-live", FullPath: "/emb-live", Published: true,
		Data: map[string]interface{}{"body": "<p>live text</p>"},
	})
	// Fork copies are normally unpublished; a published flag on one (legacy
	// data, a page created as published inside a fork) must still not count.
	forkID := primitive.NewObjectID()
	copyID := seedSearchContent(t, db, &models.Content{
		Title: "Fork Copy", Slug: "emb-live", FullPath: "/emb-live", Published: true, ForkID: &forkID,
		Data: map[string]interface{}{"body": "<p>fork text</p>"},
	})
	embedded := time.Now()
	seedSearchContent(t, db, &models.Content{
		Title: "Embedded Copy", Slug: "emb-new", FullPath: "/emb-new", Published: true, ForkID: &forkID,
		EmbeddingAt: &embedded,
	})

	svc := NewSearchService(db, "")
	processed, errCount, err := svc.BatchGenerateEmbeddings(ctx)
	if err != nil {
		t.Fatalf("BatchGenerateEmbeddings: %v", err)
	}
	if processed != 1 || errCount != 0 || *calls != 1 {
		t.Errorf("processed=%d errCount=%d calls=%d, want 1/0/1 (live page only)", processed, errCount, *calls)
	}
	cs := NewContentService(db)
	if c, err := cs.GetContent(ctx, copyID); err != nil || c.EmbeddingAt != nil {
		t.Errorf("fork copy got an embedding: %v, %v", c, err)
	}
	if c, err := cs.GetContent(ctx, liveID); err != nil || c.EmbeddingAt == nil {
		t.Errorf("live page has no embedding: %v, %v", c, err)
	}

	total, withEmb, err := svc.EmbeddingStats(ctx)
	if err != nil {
		t.Fatalf("EmbeddingStats: %v", err)
	}
	if total != 1 || withEmb != 1 {
		t.Errorf("stats total=%d withEmb=%d, want 1/1 (fork copies not counted)", total, withEmb)
	}
}
