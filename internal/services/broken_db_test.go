package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestBrokenDB_Services exercises the database-error branches of service methods
// by running them against a disconnected database. Each call should return an
// error (not panic).
func TestBrokenDB_Services(t *testing.T) {
	db := testutil.MustConnectBrokenDB(t)
	ctx := context.Background()
	id := primitive.NewObjectID()

	cs := NewContentService(db)
	if _, err := cs.ListContent(ctx, false, "", nil); err == nil {
		t.Error("ListContent: expected error on dead DB")
	}
	_, _ = cs.GetContent(ctx, id)
	_, _ = cs.GetContentByPath(ctx, "/x")

	ts := NewTemplateService(db, cs)
	if _, err := ts.ListTemplates(ctx); err == nil {
		t.Error("ListTemplates: expected error")
	}
	_, _ = ts.GetTemplate(ctx, id)

	as := NewAssetService(db)
	if _, err := as.ListAssets(ctx, ""); err == nil {
		t.Error("ListAssets: expected error")
	}

	ss := NewSnippetService(db)
	if _, err := ss.ListSnippets(ctx); err == nil {
		t.Error("ListSnippets: expected error")
	}
	if _, err := ss.CreateSnippet(ctx, "n", "<b>x</b>"); err == nil {
		t.Error("CreateSnippet: expected error")
	}

	us := NewUserService(db)
	if _, err := us.ListUsers(ctx); err == nil {
		t.Error("ListUsers: expected error")
	}
	_, _ = us.GetByEmail(ctx, "x@y.com")

	aks := NewAPIKeyService(db)
	if _, err := aks.ListAPIKeys(ctx); err == nil {
		t.Error("ListAPIKeys: expected error")
	}

	auds := NewAuditService(db)
	_, _, _ = auds.List(ctx, AuditFilter{})

	ws := NewWebhookService(db)
	if _, err := ws.List(ctx, 0); err == nil {
		t.Error("webhook List: expected error")
	}
	if _, err := ws.Create(ctx, "n", "https://x", "s", []string{"e"}, true); err == nil {
		t.Error("webhook Create: expected error")
	}

	ls := NewLockService(db)
	_, _ = ls.GetLock(ctx, id)
	_, _ = ls.AcquireLock(ctx, id, id, "e")

	fs := NewForkService(db, cs)
	if _, err := fs.List(ctx); err == nil {
		t.Error("fork List: expected error")
	}
	if _, err := fs.Create(ctx, "n", "d", id, "e"); err == nil {
		t.Error("fork Create: expected error")
	}

	is := NewImportService(db, cs)
	if _, err := is.ListSources(ctx); err == nil {
		t.Error("ListSources: expected error")
	}
	_, _ = is.GetSource(ctx, id)
	if _, err := is.ListJobs(ctx, 10); err == nil {
		t.Error("ListJobs: expected error")
	}

	cms := NewCommentService(db)
	if _, err := cms.Create(ctx, id, id, "e", "n", "text", nil); err == nil {
		t.Error("comment Create: expected error")
	}
	_, _ = cms.ListForContent(ctx, id)

	aps := NewApprovalService(db, cs, cms, ws)
	if _, err := aps.ListWorkflows(ctx); err == nil {
		t.Error("ListWorkflows: expected error")
	}
	if _, err := aps.CreateWorkflow(ctx, models.ApprovalWorkflow{Name: "w", Mode: "concurrent"}); err == nil {
		t.Error("CreateWorkflow: expected error")
	}
	_, _ = aps.ListPending(ctx)

	sets := NewSettingsService(db, cs)
	_, _ = sets.GetTheme(ctx)
	_, _ = sets.ListRedirects(ctx)
	_, _ = sets.ListFolders(ctx)
	_, _ = sets.ListCollections(ctx)

	lc := NewLinkCheckerService(db)
	if _, err := lc.StartJob(ctx); err == nil {
		t.Error("StartJob: expected error")
	}

	rq := NewRegenQueue(db, cs)
	if _, err := rq.ListRecentJobs(ctx, 10); err == nil {
		t.Error("ListRecentJobs: expected error")
	}

	_ = bson.M{}
}
