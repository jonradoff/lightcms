package services

import (
	"context"
	"testing"
	"time"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestFaultInjection_Services exercises the database-error branches that occur
// *after* an initial read succeeds (e.g. read content, then the write fails).
// These multi-step branches are unreachable with a fully-dead database, so we
// seed with the hook cleared, then inject a failure for one specific operation.
func TestFaultInjection_Services(t *testing.T) {
	db := testutil.MustConnectFaultDB(t)
	defer db.SetFaultHook(nil)
	testutil.CleanupCollections(t, db)

	cs := NewContentService(db)
	ctx := context.Background()
	tmplID := seedTmpl(t, cs, "Page", "page")

	c := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Hello",
		Slug: "hello", FullPath: "/hello", Published: true,
		Data: map[string]interface{}{"body": "x"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := cs.CreateContent(ctx, c); err != nil {
		t.Fatalf("seed CreateContent: %v", err)
	}

	// Reads succeed, UpdateOne fails → publish/unpublish/update/delete error paths.
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	if err := cs.PublishContent(ctx, c.ID); err == nil {
		t.Error("PublishContent should fail when UpdateOne fails")
	}
	if err := cs.UnpublishContent(ctx, c.ID); err == nil {
		t.Error("UnpublishContent should fail")
	}
	if err := cs.UpdateContent(ctx, c); err == nil {
		t.Error("UpdateContent should fail")
	}
	if err := cs.DeleteContent(ctx, c.ID); err == nil {
		t.Error("DeleteContent should fail")
	}
	db.SetFaultHook(nil)

	// Template read succeeds, content InsertOne fails → CreateContent insert error.
	db.SetFaultHook(testutil.FailOp("InsertOne"))
	if err := cs.CreateContent(ctx, &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "X", Slug: "x", FullPath: "/x",
		Data: map[string]interface{}{"body": "y"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err == nil {
		t.Error("CreateContent should fail when InsertOne fails")
	}
	db.SetFaultHook(nil)

	// SettingsService: save error branches (read theme ok, save fails).
	ss := NewSettingsService(db, cs)
	theme, _ := ss.GetTheme(ctx)
	if theme != nil {
		db.SetFaultHook(testutil.FailOp("UpdateOne"))
		_ = ss.UpdateTheme(ctx, theme)
		db.SetFaultHook(nil)
	}

	// WebhookService: create one, then fail update/delete.
	ws := NewWebhookService(db)
	wh, err := ws.Create(ctx, "h", "https://x", "s", []string{"e"}, true)
	if err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	if err := ws.Update(ctx, wh.ID, "h2", "https://y", "s", []string{"e"}, false); err == nil {
		t.Error("webhook Update should fail")
	}
	db.SetFaultHook(testutil.FailOp("DeleteOne"))
	if err := ws.Delete(ctx, wh.ID); err == nil {
		t.Error("webhook Delete should fail")
	}
	db.SetFaultHook(nil)

	// ForkService: create fork ok, then archive/delete write fails.
	fs := NewForkService(db, cs)
	fork, err := fs.Create(ctx, "F", "", primitive.NewObjectID(), "e@x.com")
	if err != nil {
		t.Fatalf("seed fork: %v", err)
	}
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	if err := fs.Archive(ctx, fork.ID); err == nil {
		t.Error("fork Archive should fail")
	}
	db.SetFaultHook(testutil.FailOp("DeleteMany"))
	_ = fs.Delete(ctx, fork.ID)
	db.SetFaultHook(nil)

	// LockService: acquire ok, then release/refresh write fails.
	ls := NewLockService(db)
	uid := primitive.NewObjectID()
	cid := primitive.NewObjectID()
	_, _ = ls.AcquireLock(ctx, cid, uid, "e@x.com")
	db.SetFaultHook(testutil.FailOp("DeleteOne"))
	if err := ls.ReleaseLock(ctx, cid, uid); err == nil {
		t.Error("ReleaseLock should fail")
	}
	db.SetFaultHook(testutil.FailOp("UpdateOne"))
	if err := ls.RefreshLock(ctx, cid, uid); err == nil {
		t.Error("RefreshLock should fail")
	}
	db.SetFaultHook(nil)

	testutil.CleanupCollections(t, db)
}
