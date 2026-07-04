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

func TestApprovalService_MatchWorkflowPriority(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewApprovalService(db, NewContentService(db), NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	templateID := primitive.NewObjectID()
	approver := models.WorkflowApprover{UserID: primitive.NewObjectID(), UserEmail: "a@x.com"}

	mk := func(name, trigger, value string) *models.ApprovalWorkflow {
		wf, err := svc.CreateWorkflow(ctx, models.ApprovalWorkflow{
			Name: name, Trigger: trigger, TriggerValue: value,
			Mode: models.WorkflowModeConcurrent, Approvers: []models.WorkflowApprover{approver},
		})
		if err != nil {
			t.Fatalf("CreateWorkflow %s: %v", name, err)
		}
		return wf
	}

	tmplWF := mk("by-template", models.WorkflowTriggerTemplateID, templateID.Hex())
	folderWF := mk("by-folder", models.WorkflowTriggerFolderPath, "/blog")
	tagWF := mk("by-tag", models.WorkflowTriggerTag, "review-me")
	allWF := mk("catch-all", models.WorkflowTriggerAllContributor, "")

	cases := []struct {
		name    string
		content *models.Content
		want    primitive.ObjectID
	}{
		{"template beats all", &models.Content{
			TemplateID: templateID, FolderPath: "/blog", Tags: []string{"review-me"},
		}, tmplWF.ID},
		{"folder beats tag", &models.Content{
			FolderPath: "/blog/posts", Tags: []string{"review-me"},
		}, folderWF.ID},
		{"tag beats catch-all", &models.Content{
			FolderPath: "/docs", Tags: []string{"other", "review-me"},
		}, tagWF.ID},
		{"catch-all fallback", &models.Content{
			FolderPath: "/docs",
		}, allWF.ID},
	}
	for _, tc := range cases {
		got, err := svc.matchWorkflow(ctx, tc.content)
		if err != nil {
			t.Fatalf("%s: matchWorkflow: %v", tc.name, err)
		}
		if got == nil || got.ID != tc.want {
			t.Errorf("%s: matched %+v, want workflow %s", tc.name, got, tc.want.Hex())
		}
	}
}

func TestApprovalService_SequentialApprovalFlow(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	svc := NewApprovalService(db, cs, NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	approver1 := primitive.NewObjectID()
	approver2 := primitive.NewObjectID()
	submitter := primitive.NewObjectID()

	if _, err := svc.CreateWorkflow(ctx, models.ApprovalWorkflow{
		Name: "Seq", Trigger: models.WorkflowTriggerAllContributor,
		Mode: models.WorkflowModeSequential,
		Approvers: []models.WorkflowApprover{
			{UserID: approver1, UserEmail: "one@x.com", Order: 0},
			{UserID: approver2, UserEmail: "two@x.com", Order: 1},
		},
	}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	// Seed the content doc so the final publish update has a target.
	content := &models.Content{
		ID: primitive.NewObjectID(), Title: "Seq Page", Slug: "seq-page",
		FullPath: "/seq-page", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if _, err := db.InsertOne(ctx, "content", content); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	req, err := svc.SubmitContentForApproval(ctx, content, submitter, "sub@x.com")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if req.RequiredApprovals != 2 {
		t.Fatalf("required approvals = %d, want 2", req.RequiredApprovals)
	}

	// Resubmitting is idempotent while pending.
	again, err := svc.SubmitContentForApproval(ctx, content, submitter, "sub@x.com")
	if err != nil || again.ID != req.ID {
		t.Errorf("resubmit not idempotent: %v / %+v", err, again)
	}

	// Self-approval is rejected.
	if err := svc.Approve(ctx, req.ID, submitter, "sub@x.com", ""); err == nil {
		t.Error("expected self-approval to fail")
	}

	// Out-of-turn approver is rejected.
	if err := svc.Approve(ctx, req.ID, approver2, "two@x.com", ""); err == nil {
		t.Error("expected out-of-turn approval to fail")
	}

	// Step 1: first approver — request stays pending, step advances.
	if err := svc.Approve(ctx, req.ID, approver1, "one@x.com", "step 1 ok"); err != nil {
		t.Fatalf("Approve step 1: %v", err)
	}
	mid, err := svc.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if mid.Status != models.ApprovalStatusPending || mid.CurrentStep != 1 {
		t.Fatalf("after step 1: status=%s step=%d, want pending/1", mid.Status, mid.CurrentStep)
	}

	// Step 2: second approver — request approved, content published.
	if err := svc.Approve(ctx, req.ID, approver2, "two@x.com", "step 2 ok"); err != nil {
		t.Fatalf("Approve step 2: %v", err)
	}
	final, err := svc.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetRequest final: %v", err)
	}
	if final.Status != models.ApprovalStatusApproved {
		t.Errorf("final status = %s, want approved", final.Status)
	}

	var published models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": content.ID}, &published); err != nil {
		t.Fatalf("reload content: %v", err)
	}
	if !published.Published || published.PendingApproval {
		t.Errorf("content published=%v pending=%v, want true/false", published.Published, published.PendingApproval)
	}

	// Approving a non-pending request fails.
	if err := svc.Approve(ctx, req.ID, approver1, "one@x.com", ""); err == nil {
		t.Error("expected error approving a non-pending request")
	}
	// Rejecting a non-pending request fails too.
	if err := svc.Reject(ctx, req.ID, approver1, "one@x.com", "One", "nope"); err == nil {
		t.Error("expected error rejecting a non-pending request")
	}
}

func TestApprovalService_AssetReviewApproval(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewApprovalService(db, NewContentService(db), NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	asset := &models.Asset{ID: primitive.NewObjectID(), Filename: "pic.png", FullPath: "/assets/pic.png"}
	if _, err := db.InsertOne(ctx, "assets", bson.M{
		"_id": asset.ID, "filename": asset.Filename, "pending_review": true,
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	req, err := svc.SubmitAssetForReview(ctx, asset, primitive.NewObjectID(), "up@x.com")
	if err != nil {
		t.Fatalf("SubmitAssetForReview: %v", err)
	}

	// No workflow → single approval clears the asset review flag.
	if err := svc.Approve(ctx, req.ID, primitive.NewObjectID(), "rev@x.com", "fine"); err != nil {
		t.Fatalf("Approve asset: %v", err)
	}
	var doc struct {
		PendingReview bool `bson:"pending_review"`
	}
	if err := db.FindOne(ctx, "assets", bson.M{"_id": asset.ID}, &doc); err != nil {
		t.Fatalf("reload asset: %v", err)
	}
	if doc.PendingReview {
		t.Error("asset still pending review after approval")
	}
}
