package services

import (
	"context"
	"testing"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"
	"lightcms/internal/testutil"

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
