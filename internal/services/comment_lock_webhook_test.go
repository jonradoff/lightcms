package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- CommentService ---

func TestCommentService_CreateListDelete(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewCommentService(db)
	ctx := context.Background()
	contentID := primitive.NewObjectID()
	userID := primitive.NewObjectID()

	c, err := svc.Create(ctx, contentID, userID, "u@x.com", "User", "hello", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID.IsZero() {
		t.Fatal("expected non-zero comment ID")
	}
	if c.Text != "hello" {
		t.Errorf("text = %q, want hello", c.Text)
	}

	list, err := svc.ListForContent(ctx, contentID)
	if err != nil {
		t.Fatalf("ListForContent: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(list))
	}

	n, err := svc.CountForContent(ctx, contentID)
	if err != nil {
		t.Fatalf("CountForContent: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}

	if err := svc.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	n, _ = svc.CountForContent(ctx, contentID)
	if n != 0 {
		t.Errorf("count after delete = %d, want 0", n)
	}
}

func TestCommentService_CreateEmptyText(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewCommentService(db)
	if _, err := svc.Create(context.Background(), primitive.NewObjectID(), primitive.NewObjectID(), "u@x.com", "U", "", nil); err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestCommentService_ListRecent(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewCommentService(db)
	ctx := context.Background()
	contentID := primitive.NewObjectID()
	userID := primitive.NewObjectID()
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, contentID, userID, "u@x.com", "U", "msg", nil); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// ListRecent joins against content; with no content doc it should still not error.
	if _, err := svc.ListRecent(ctx, 10); err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
}

// --- LockService ---

func TestLockService_AcquireGetRefreshRelease(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewLockService(db)
	ctx := context.Background()
	contentID := primitive.NewObjectID()
	userID := primitive.NewObjectID()

	// Fresh acquire returns (nil, nil) — no blocking lock existed.
	if blocking, err := svc.AcquireLock(ctx, contentID, userID, "u@x.com"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	} else if blocking != nil {
		t.Fatalf("expected no blocking lock on fresh acquire, got %+v", blocking)
	}

	got, err := svc.GetLock(ctx, contentID)
	if err != nil {
		t.Fatalf("GetLock: %v", err)
	}
	if got == nil || got.ContentID != contentID {
		t.Fatalf("GetLock returned %+v", got)
	}

	if err := svc.RefreshLock(ctx, contentID, userID); err != nil {
		t.Fatalf("RefreshLock: %v", err)
	}

	// Same user re-acquiring should succeed (idempotent).
	if _, err := svc.AcquireLock(ctx, contentID, userID, "u@x.com"); err != nil {
		t.Fatalf("re-AcquireLock by same user: %v", err)
	}

	if err := svc.ReleaseLock(ctx, contentID, userID); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}
	got, _ = svc.GetLock(ctx, contentID)
	if got != nil {
		t.Errorf("expected no lock after release, got %+v", got)
	}
}

func TestLockService_AcquireConflictAndForceUnlock(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewLockService(db)
	ctx := context.Background()
	contentID := primitive.NewObjectID()
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()

	if _, err := svc.AcquireLock(ctx, contentID, owner, "owner@x.com"); err != nil {
		t.Fatalf("AcquireLock owner: %v", err)
	}
	// A different user is blocked: AcquireLock returns the existing (owner's) lock.
	blocking, err := svc.AcquireLock(ctx, contentID, other, "other@x.com")
	if err != nil {
		t.Fatalf("AcquireLock other: %v", err)
	}
	if blocking == nil || blocking.UserID != owner {
		t.Fatalf("expected blocking lock owned by %v, got %+v", owner, blocking)
	}
	// Force unlock clears it regardless of owner.
	if err := svc.ForceUnlock(ctx, contentID); err != nil {
		t.Fatalf("ForceUnlock: %v", err)
	}
	if blocking, err := svc.AcquireLock(ctx, contentID, other, "other@x.com"); err != nil {
		t.Fatalf("AcquireLock after force unlock: %v", err)
	} else if blocking != nil {
		t.Fatalf("expected clean acquire after force unlock, got %+v", blocking)
	}
}

// --- WebhookService ---

func TestWebhookService_CRUD(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewWebhookService(db)
	ctx := context.Background()

	wh, err := svc.Create(ctx, "hook", "https://example.com/hook", "sek", []string{"content.published"}, true)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wh.ID.IsZero() {
		t.Fatal("expected non-zero webhook ID")
	}

	got, err := svc.Get(ctx, wh.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "hook" || got.URL != "https://example.com/hook" {
		t.Errorf("unexpected webhook: %+v", got)
	}

	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(list))
	}

	if err := svc.Update(ctx, wh.ID, "hook2", "https://example.com/h2", "sek2", []string{"content.deleted"}, false); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = svc.Get(ctx, wh.ID)
	if got.Name != "hook2" || got.Active {
		t.Errorf("update not applied: %+v", got)
	}

	if err := svc.RegenerateSecret(ctx, wh.ID, "newsecret"); err != nil {
		t.Fatalf("RegenerateSecret: %v", err)
	}
	got, _ = svc.Get(ctx, wh.ID)
	if got.Secret != "newsecret" {
		t.Errorf("secret = %q, want newsecret", got.Secret)
	}

	if _, err := svc.ListDeliveries(ctx, wh.ID, 10); err != nil {
		t.Fatalf("ListDeliveries: %v", err)
	}

	if err := svc.Delete(ctx, wh.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// Get returns (nil, nil) for a missing webhook.
	if got, err := svc.Get(ctx, wh.ID); err != nil {
		t.Errorf("Get after delete: unexpected error %v", err)
	} else if got != nil {
		t.Errorf("expected nil doc after delete, got %+v", got)
	}
}

func TestWebhookService_GetNotFound(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewWebhookService(db)
	got, err := svc.Get(context.Background(), primitive.NewObjectID())
	if err != nil {
		t.Errorf("Get missing: unexpected error %v", err)
	}
	if got != nil {
		t.Errorf("expected nil doc for missing webhook, got %+v", got)
	}
}
