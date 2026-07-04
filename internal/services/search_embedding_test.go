package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// newFakeOllama returns an httptest server that answers /api/embeddings with a
// fixed 3-dim vector and counts requests.
func newFakeOllama(t *testing.T) (*httptest.Server, *int64) {
	t.Helper()
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt64(&calls, 1)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": []float32{0.5, 0.25, 0.125},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// seedSearchContent inserts a content doc directly for embedding tests.
func seedSearchContent(t *testing.T, db *database.DB, c *models.Content) primitive.ObjectID {
	t.Helper()
	if c.ID.IsZero() {
		c.ID = primitive.NewObjectID()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().Add(-time.Hour)
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().Add(-time.Hour)
	}
	if _, err := db.InsertOne(context.Background(), "content", c); err != nil {
		t.Fatalf("seedSearchContent: %v", err)
	}
	return c.ID
}

func TestBatchGenerateEmbeddings_Ollama(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake, calls := newFakeOllama(t)
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)

	// Needs an embedding.
	needsID := seedSearchContent(t, db, &models.Content{
		Title: "Needs Embedding", Slug: "needs", FullPath: "/needs",
		Published: true,
		Data:      map[string]interface{}{"body": "<p>hello embedding world</p>"},
	})
	// Empty plain text — skipped.
	seedSearchContent(t, db, &models.Content{
		Slug: "empty", FullPath: "/empty", Published: true,
	})
	// Up-to-date embedding — skipped.
	fresh := &models.Content{
		Title: "Fresh", Slug: "fresh", FullPath: "/fresh", Published: true,
	}
	fresh.PlainText = "Fresh"
	later := time.Now()
	fresh.EmbeddingAt = &later
	seedSearchContent(t, db, fresh)
	// Unpublished — excluded by filter.
	seedSearchContent(t, db, &models.Content{
		Title: "Draft", Slug: "draft-e", FullPath: "/draft-e", Published: false,
	})

	svc := NewSearchService(db, "")
	processed, errCount, err := svc.BatchGenerateEmbeddings(ctx)
	if err != nil {
		t.Fatalf("BatchGenerateEmbeddings: %v", err)
	}
	if processed != 1 || errCount != 0 {
		t.Errorf("processed=%d errCount=%d, want 1/0", processed, errCount)
	}
	if atomic.LoadInt64(calls) != 1 {
		t.Errorf("ollama calls = %d, want 1", atomic.LoadInt64(calls))
	}

	// Embedding stored?
	var stored models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": needsID}, &stored); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.EmbeddingAt == nil || stored.PlainText == "" {
		t.Errorf("embedding not stored: embedding_at=%v plain_text=%q", stored.EmbeddingAt, stored.PlainText)
	}

	// Stats should now report 3 published, 2 with embeddings (needs + fresh).
	total, withEmb, err := svc.EmbeddingStats(ctx)
	if err != nil {
		t.Fatalf("EmbeddingStats: %v", err)
	}
	if total != 3 || withEmb != 2 {
		t.Errorf("stats total=%d withEmb=%d, want 3/2", total, withEmb)
	}

	// Generation failure path: failing Ollama server.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer failSrv.Close()
	t.Setenv("OLLAMA_URL", failSrv.URL)
	// Force the item to look stale again.
	if err := db.UpdateOne(ctx, "content", bson.M{"_id": needsID},
		bson.M{"$set": bson.M{"updated_at": time.Now().Add(time.Hour)}}); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	failSvc := NewSearchService(db, "")
	processed, errCount, err = failSvc.BatchGenerateEmbeddings(ctx)
	if err != nil {
		t.Fatalf("BatchGenerateEmbeddings (failing): %v", err)
	}
	if processed != 0 || errCount != 1 {
		t.Errorf("failing run processed=%d errCount=%d, want 0/1", processed, errCount)
	}

	// Save-failure path: embeddings generate but the DB write is rejected.
	t.Setenv("OLLAMA_URL", fake.URL)
	saveSvc := NewSearchService(db, "")
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	processed, errCount, err = saveSvc.BatchGenerateEmbeddings(ctx)
	db.SetFaultHook(nil)
	if err != nil {
		t.Fatalf("BatchGenerateEmbeddings (save fail): %v", err)
	}
	if processed != 0 || errCount != 1 {
		t.Errorf("save-fail run processed=%d errCount=%d, want 0/1", processed, errCount)
	}
}

func TestUpdateContentEmbedding_Ollama(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake, _ := newFakeOllama(t)
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)
	svc := NewSearchService(db, "")

	id := seedSearchContent(t, db, &models.Content{
		Title: "Embed Me", Slug: "embed-me", FullPath: "/embed-me",
		Published: true,
		Data:      map[string]interface{}{"body": "<p>content to embed</p>"},
	})
	if err := svc.UpdateContentEmbedding(ctx, id); err != nil {
		t.Fatalf("UpdateContentEmbedding: %v", err)
	}
	var stored models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": id}, &stored); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.EmbeddingAt == nil {
		t.Error("embedding_at not set")
	}
	if stored.PlainText == "" {
		t.Error("plain_text not cached")
	}

	// Missing content → error.
	if err := svc.UpdateContentEmbedding(ctx, primitive.NewObjectID()); err == nil {
		t.Error("expected error for missing content")
	}

	// Empty plain text → nil (nothing to embed).
	emptyID := seedSearchContent(t, db, &models.Content{
		Slug: "blank", FullPath: "/blank", Published: true,
	})
	if err := svc.UpdateContentEmbedding(ctx, emptyID); err != nil {
		t.Errorf("empty content should be a no-op, got %v", err)
	}

	// Provider failure → error.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer failSrv.Close()
	t.Setenv("OLLAMA_URL", failSrv.URL)
	failSvc := NewSearchService(db, "")
	if err := failSvc.UpdateContentEmbedding(ctx, id); err == nil {
		t.Error("expected error when provider is down")
	}
}

func TestSearch_ModeDispatch_Ollama(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake, _ := newFakeOllama(t)
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)

	seedSearchContent(t, db, &models.Content{
		Title: "Gopher Guide", Slug: "gopher", FullPath: "/gopher",
		Published: true, PlainText: "a friendly gopher guide to testing",
	})
	seedSearchContent(t, db, &models.Content{
		Title: "Other Page", Slug: "other", FullPath: "/other",
		Published: true, PlainText: "nothing relevant here",
	})

	svc := NewSearchService(db, "")

	// Exact mode.
	results, err := svc.Search(ctx, "gopher", "exact", 10)
	if err != nil {
		t.Fatalf("Search exact: %v", err)
	}
	if len(results) != 1 || results[0].FullPath != "/gopher" {
		t.Errorf("exact results = %+v", results)
	}

	// Limit clamping (0 and >50 both become 10).
	if _, err := svc.Search(ctx, "gopher", "exact", 0); err != nil {
		t.Errorf("Search limit 0: %v", err)
	}
	if _, err := svc.Search(ctx, "gopher", "exact", 100); err != nil {
		t.Errorf("Search limit 100: %v", err)
	}

	// Semantic mode: query embedding succeeds via fake Ollama; the Atlas
	// $vectorSearch aggregation may fail on the test cluster — either outcome
	// exercises the dispatch and query-embedding path.
	if _, err := svc.Search(ctx, "gopher", "semantic", 5); err != nil {
		t.Logf("semantic search returned error (acceptable on test cluster): %v", err)
	}

	// Hybrid (default) mode: exact leg succeeds even if the semantic leg errors.
	results, err = svc.Search(ctx, "gopher", "", 5)
	if err != nil {
		t.Fatalf("Search hybrid: %v", err)
	}
	found := false
	for _, r := range results {
		if r.FullPath == "/gopher" {
			found = true
		}
	}
	if !found {
		t.Errorf("hybrid results missing /gopher: %+v", results)
	}
}

func TestSearchSemantic_NoProviderAndProviderDown(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// No provider at all.
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "")
	none := NewSearchService(db, "")
	if _, err := none.SearchSemantic(ctx, "q", 5); err == nil {
		t.Error("expected error with no provider")
	}
	// Semantic mode without provider falls back to full-text.
	if _, err := none.Search(ctx, "q", "semantic", 5); err != nil {
		t.Errorf("semantic fallback: %v", err)
	}

	// Provider configured but down → query-embedding error branch.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer failSrv.Close()
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", failSrv.URL)
	down := NewSearchService(db, "")
	if _, err := down.SearchSemantic(ctx, "q", 5); err == nil {
		t.Error("expected query-embedding error when provider is down")
	}
}

func TestTriggerEmbedding_OnPublishedCreate(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()

	fake, calls := newFakeOllama(t)
	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)

	cs := NewContentService(db)
	ss := NewSearchService(db, "")
	cs.SetSearchService(ss)

	content := &models.Content{
		Title: "Auto Embedded", Slug: "auto-embedded",
		Published: true,
		Data:      map[string]interface{}{"body": "<p>auto embedding trigger</p>"},
	}
	if err := cs.CreateContent(ctx, content, "initial"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// triggerEmbedding runs async — poll for the embedding to land.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var stored models.Content
		if err := db.FindOne(ctx, "content", bson.M{"_id": content.ID}, &stored); err == nil && stored.EmbeddingAt != nil {
			if atomic.LoadInt64(calls) == 0 {
				t.Error("embedding set but no ollama call recorded")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("embedding was not generated within 10s of published create")
}
