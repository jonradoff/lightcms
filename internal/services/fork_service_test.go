package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedLiveContent inserts a minimal non-forked, non-deleted live content item.
func seedLiveContent(t *testing.T, db *database.DB, title, fullPath string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	now := time.Now()
	live := &models.Content{
		ID:        id,
		Title:     title,
		Slug:      fullPath[1:],
		FullPath:  fullPath,
		Published: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := db.InsertOne(context.Background(), "content", live); err != nil {
		t.Fatalf("seedLiveContent: %v", err)
	}
	return id
}

func TestForkService_CreateGetList(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	fs := NewForkService(db, NewContentService(db))
	ctx := context.Background()
	uid := primitive.NewObjectID()

	fork, err := fs.Create(ctx, "Campaign", "Q3 launch", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if fork.Status != "active" || fork.PreviewToken == "" {
		t.Fatalf("unexpected fork: %+v", fork)
	}

	got, err := fs.GetByID(ctx, fork.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Campaign" {
		t.Errorf("name = %q, want Campaign", got.Name)
	}

	byTok, err := fs.GetByPreviewToken(ctx, fork.PreviewToken)
	if err != nil {
		t.Fatalf("GetByPreviewToken: %v", err)
	}
	if byTok.ID != fork.ID {
		t.Error("preview token lookup returned wrong fork")
	}
	if _, err := fs.GetByPreviewToken(ctx, ""); err == nil {
		t.Error("expected error for empty token")
	}

	list, err := fs.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 fork, got %d", len(list))
	}
}

func TestForkService_PageLifecycle(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	fs := NewForkService(db, NewContentService(db))
	ctx := context.Background()
	uid := primitive.NewObjectID()

	fork, err := fs.Create(ctx, "F", "", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	liveID := seedLiveContent(t, db, "Home", "/home")

	page, err := fs.ForkPage(ctx, fork.ID, liveID)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	if page.ForkID == nil || *page.ForkID != fork.ID {
		t.Fatalf("fork page missing fork id: %+v", page)
	}
	if page.FullPath != "/home" {
		t.Errorf("fork page path = %q, want /home", page.FullPath)
	}

	// Forking the same page again returns the existing copy (idempotent).
	again, err := fs.ForkPage(ctx, fork.ID, liveID)
	if err != nil {
		t.Fatalf("ForkPage idempotent: %v", err)
	}
	if again.ID != page.ID {
		t.Error("expected idempotent fork page to return same copy")
	}

	byPath, err := fs.GetForkPageByPath(ctx, fork.ID, "/home")
	if err != nil {
		t.Fatalf("GetForkPageByPath: %v", err)
	}
	if byPath.ID != page.ID {
		t.Error("GetForkPageByPath returned wrong page")
	}

	pages, err := fs.ListPages(ctx, fork.ID)
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 fork page, got %d", len(pages))
	}

	n, err := fs.GetPageCount(ctx, fork.ID)
	if err != nil {
		t.Fatalf("GetPageCount: %v", err)
	}
	if n != 1 {
		t.Errorf("page count = %d, want 1", n)
	}

	if err := fs.RemovePage(ctx, fork.ID, page.ID); err != nil {
		t.Fatalf("RemovePage: %v", err)
	}
	n, _ = fs.GetPageCount(ctx, fork.ID)
	if n != 0 {
		t.Errorf("page count after remove = %d, want 0", n)
	}
}

func TestForkService_ArchiveAndDelete(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	fs := NewForkService(db, NewContentService(db))
	ctx := context.Background()
	uid := primitive.NewObjectID()

	fork, err := fs.Create(ctx, "ToArchive", "", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := fs.Archive(ctx, fork.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	got, err := fs.GetByID(ctx, fork.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "archived" {
		t.Errorf("status = %q, want archived", got.Status)
	}

	fork2, _ := fs.Create(ctx, "ToDelete", "", uid, "ed@x.com")
	if err := fs.Delete(ctx, fork2.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := fs.GetByID(ctx, fork2.ID); err == nil {
		t.Error("expected error getting deleted fork")
	}
}

func TestForkService_Diff(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	fs := NewForkService(db, cs)
	ctx := context.Background()
	uid := primitive.NewObjectID()

	liveID := seedLiveContent(t, db, "Original Title", "/diff-page")

	fork, err := fs.Create(ctx, "diff-fork", "", uid, "a@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Copy the live page and modify the copy.
	copyPage, err := fs.ForkPage(ctx, fork.ID, liveID)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	copyPage.Title = "Changed Title"
	copyPage.Data = map[string]interface{}{"body": "new body"}
	if err := cs.UpdateContent(ctx, copyPage, "edit in fork"); err != nil {
		t.Fatalf("UpdateContent fork copy: %v", err)
	}

	// A page that exists only in the fork.
	newInFork := &models.Content{
		Title: "Only In Fork", Slug: "fork-only", FullPath: "/fork-only",
		ForkID: &fork.ID, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := db.InsertOne(ctx, "content", newInFork); err != nil {
		t.Fatalf("insert fork-only page: %v", err)
	}

	diffs, err := fs.Diff(ctx, fork.ID)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %+v", len(diffs), diffs)
	}

	byPath := map[string]ForkPageDiff{}
	for _, d := range diffs {
		byPath[d.Path] = d
	}

	mod := byPath["/diff-page"]
	if mod.Status != "modified" {
		t.Errorf("status = %q, want modified", mod.Status)
	}
	foundTitle, foundBody := false, false
	for _, f := range mod.Fields {
		if f.Name == "title" && f.Live == "Original Title" && f.Fork == "Changed Title" {
			foundTitle = true
		}
		if f.Name == "body" && f.Fork == "new body" {
			foundBody = true
		}
	}
	if !foundTitle || !foundBody {
		t.Errorf("missing expected field diffs: %+v", mod.Fields)
	}

	if byPath["/fork-only"].Status != "added" {
		t.Errorf("fork-only page status = %q, want added", byPath["/fork-only"].Status)
	}
}

// TestForkCopy_NoStaticSideEffects verifies that fork copies never generate
// or remove static files at their live path (they share full_path with live).
func TestForkCopy_NoStaticSideEffects(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	fs := NewForkService(db, cs)
	ctx := context.Background()

	liveID := seedLiveContent(t, db, "Static Page", "/static-guard")

	// Generate the live static file baseline via a live update.
	var live models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": liveID}, &live); err != nil {
		t.Fatalf("load live: %v", err)
	}

	fork, _ := fs.Create(ctx, "guard-fork", "", primitive.NewObjectID(), "a@x.com")
	copyPage, err := fs.ForkPage(ctx, fork.ID, liveID)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}

	// GenerateStaticPage on a fork copy must be a no-op, not an overwrite.
	if err := cs.GenerateStaticPage(ctx, copyPage); err != nil {
		t.Fatalf("GenerateStaticPage on fork copy errored: %v", err)
	}

	// Updating an unpublished fork copy must not remove the live static file
	// (regression: the unpublished branch used to call removeStaticPage on
	// the shared full_path).
	staticPath := filepath.Join("content", "generated", "static-guard.html")
	if err := os.MkdirAll(filepath.Dir(staticPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(staticPath, []byte("<html>live</html>"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	defer os.Remove(staticPath)

	copyPage.Title = "Edited in fork"
	if err := cs.UpdateContent(ctx, copyPage, "fork edit"); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	data, err := os.ReadFile(staticPath)
	if err != nil {
		t.Fatalf("live static file was removed by a fork-copy update: %v", err)
	}
	if string(data) != "<html>live</html>" {
		t.Errorf("live static file was rewritten by a fork-copy update")
	}
}
