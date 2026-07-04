package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestForkService_Merge_CreateUpdateConflict(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	fs := NewForkService(db, cs)
	ctx := context.Background()
	uid := primitive.NewObjectID()

	liveID := seedLiveContent(t, db, "Live Original", "/merge-live")

	fork, err := fs.Create(ctx, "merge-fork", "", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fork the live page and edit the copy.
	copyPage, err := fs.ForkPage(ctx, fork.ID, liveID)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	if err := db.UpdateOne(ctx, "content", bson.M{"_id": copyPage.ID},
		bson.M{"$set": bson.M{"title": "Fork Edit", "updated_at": time.Now()}}); err != nil {
		t.Fatalf("edit fork copy: %v", err)
	}

	// Simulate a live edit after the fork point → conflict on merge.
	if err := db.UpdateOne(ctx, "content", bson.M{"_id": liveID},
		bson.M{"$set": bson.M{"title": "Live Edited Meanwhile", "updated_at": time.Now().Add(time.Minute)}}); err != nil {
		t.Fatalf("edit live: %v", err)
	}

	// A page that exists only in the fork → created on merge. Published so the
	// static-generation branch runs too.
	forkOnly := &models.Content{
		ID: primitive.NewObjectID(), Title: "Brand New", Slug: "merge-new",
		FullPath: "/merge-new", Published: true, ForkID: &fork.ID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := db.InsertOne(ctx, "content", forkOnly); err != nil {
		t.Fatalf("insert fork-only page: %v", err)
	}

	result, err := fs.Merge(ctx, fork.ID, uid, "merger@x.com")
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if result.Created != 1 || result.Updated != 1 {
		t.Errorf("created=%d updated=%d, want 1/1", result.Created, result.Updated)
	}
	if len(result.Conflicts) != 1 {
		t.Errorf("conflicts=%d, want 1", len(result.Conflicts))
	} else if result.Conflicts[0].LiveTitle != "Live Edited Meanwhile" {
		t.Errorf("conflict live title = %q", result.Conflicts[0].LiveTitle)
	}

	// Fork wins: live page updated with fork title.
	var live models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": liveID}, &live); err != nil {
		t.Fatalf("reload live: %v", err)
	}
	if live.Title != "Fork Edit" {
		t.Errorf("live title after merge = %q, want Fork Edit", live.Title)
	}

	// The fork-only page now exists as a live page.
	var created models.Content
	if err := db.FindOne(ctx, "content", bson.M{
		"full_path": "/merge-new", "fork_id": bson.M{"$exists": false},
	}, &created); err != nil {
		t.Fatalf("created live page not found: %v", err)
	}

	// Fork is marked merged.
	merged, err := fs.GetByID(ctx, fork.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if merged.Status != "merged" {
		t.Errorf("fork status = %q, want merged", merged.Status)
	}

	// Merging a non-active fork fails.
	if _, err := fs.Merge(ctx, fork.ID, uid, "merger@x.com"); err == nil {
		t.Error("expected error merging an already-merged fork")
	}

	// Deleting a merged fork fails.
	if err := fs.Delete(ctx, fork.ID); err == nil {
		t.Error("expected error deleting a merged fork")
	}
}

func TestForkService_Merge_Errors(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	fs := NewForkService(db, NewContentService(db))
	ctx := context.Background()
	uid := primitive.NewObjectID()

	// Unknown fork.
	if _, err := fs.Merge(ctx, primitive.NewObjectID(), uid, "x@x.com"); err == nil {
		t.Error("expected error for unknown fork")
	}

	// InsertOne failure while creating a new live page.
	fork, err := fs.Create(ctx, "err-fork", "", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	forkOnly := &models.Content{
		ID: primitive.NewObjectID(), Title: "Only Fork", Slug: "err-new",
		FullPath: "/err-new", ForkID: &fork.ID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := db.InsertOne(ctx, "content", forkOnly); err != nil {
		t.Fatalf("insert fork-only page: %v", err)
	}
	db.SetFaultHook(testutil.FailOp("InsertOne"))
	_, mergeErr := fs.Merge(ctx, fork.ID, uid, "x@x.com")
	db.SetFaultHook(nil)
	if mergeErr == nil {
		t.Error("expected merge error when live-page insert fails")
	}

	// UpdateOne failure while updating an existing live page.
	liveID := seedLiveContent(t, db, "Live Err", "/err-live")
	fork2, err := fs.Create(ctx, "err-fork-2", "", uid, "ed@x.com")
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}
	if _, err := fs.ForkPage(ctx, fork2.ID, liveID); err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	_, mergeErr = fs.Merge(ctx, fork2.ID, uid, "x@x.com")
	db.SetFaultHook(nil)
	if mergeErr == nil {
		t.Error("expected merge error when live-page update fails")
	}
}

func TestForkService_DeleteErrors(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	fs := NewForkService(db, NewContentService(db))
	ctx := context.Background()

	// Unknown fork.
	if err := fs.Delete(ctx, primitive.NewObjectID()); err == nil {
		t.Error("expected error deleting unknown fork")
	}
}
