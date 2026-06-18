package services

import (
	"context"
	"testing"
	"time"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSettingsService_RedirectFolderCollection(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ss := NewSettingsService(db, NewContentService(db))
	ctx := context.Background()

	// Redirects
	r := &models.Redirect{ID: primitive.NewObjectID(), FromPath: "/old", ToPath: "/new", StatusCode: 301}
	if err := ss.CreateRedirect(ctx, r); err != nil {
		t.Fatalf("CreateRedirect: %v", err)
	}
	if _, err := ss.GetRedirect(ctx, r.ID); err != nil {
		t.Errorf("GetRedirect: %v", err)
	}
	r.ToPath = "/newer"
	if err := ss.UpdateRedirect(ctx, r); err != nil {
		t.Errorf("UpdateRedirect: %v", err)
	}
	if _, err := ss.ListRedirects(ctx); err != nil {
		t.Errorf("ListRedirects: %v", err)
	}
	if err := ss.DeleteRedirect(ctx, r.ID); err != nil {
		t.Errorf("DeleteRedirect: %v", err)
	}

	// Folders
	f := &models.Folder{ID: primitive.NewObjectID(), Name: "Blog", Slug: "blog"}
	if err := ss.CreateFolder(ctx, f); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if _, err := ss.ListFolders(ctx); err != nil {
		t.Errorf("ListFolders: %v", err)
	}

	// Collections
	col := &models.Collection{ID: primitive.NewObjectID(), Name: "News", Slug: "news", Category: "news"}
	if err := ss.CreateCollection(ctx, col); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	col.Name = "Updates"
	if err := ss.UpdateCollection(ctx, col); err != nil {
		t.Errorf("UpdateCollection: %v", err)
	}
	if _, err := ss.ListCollections(ctx); err != nil {
		t.Errorf("ListCollections: %v", err)
	}
	if err := ss.DeleteCollection(ctx, col.ID); err != nil {
		t.Errorf("DeleteCollection: %v", err)
	}
}

func TestApprovalService_TemplateTrigger(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	aps := NewApprovalService(db, cs, NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	tmplID := primitive.NewObjectID()
	// Workflow that triggers on a specific template id → exercises the
	// template_id branch of matchWorkflow.
	if _, err := aps.CreateWorkflow(ctx, models.ApprovalWorkflow{
		Name: "ByTemplate", Trigger: "template_id", TriggerValue: tmplID.Hex(), Mode: "concurrent",
		Approvers: []models.WorkflowApprover{{UserID: primitive.NewObjectID(), UserEmail: "r@x.com"}},
	}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	content := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "T", Slug: "t", FullPath: "/t",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	req, err := aps.SubmitContentForApproval(ctx, content, primitive.NewObjectID(), "a@x.com")
	if err != nil {
		t.Fatalf("SubmitContentForApproval(template trigger): %v", err)
	}
	if req == nil {
		t.Fatal("expected request for template-matched content")
	}
	if _, err := aps.GetRequest(ctx, req.ID); err != nil {
		t.Errorf("GetRequest: %v", err)
	}
}
