package services

import (
	"context"
	"testing"
	"time"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// submitOne creates a workflow (if first time) and submits a fresh content item,
// returning the resulting request and the approver user ID.
func TestApprovalService_SubmitApproveRejectCancel(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewApprovalService(db, NewContentService(db), NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	approverID := primitive.NewObjectID()
	submitterID := primitive.NewObjectID()
	wf := models.ApprovalWorkflow{
		Name: "WF", Trigger: "all_contributor", Mode: "concurrent",
		Approvers: []models.WorkflowApprover{{UserID: approverID, UserEmail: "approver@x.com", Order: 0}},
	}
	if _, err := svc.CreateWorkflow(ctx, wf); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	newContent := func(path string) *models.Content {
		return &models.Content{ID: primitive.NewObjectID(), Title: "T", FullPath: path, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}

	// Submit + approve.
	req, err := svc.SubmitContentForApproval(ctx, newContent("/a"), submitterID, "sub@x.com")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if req == nil || req.ID.IsZero() {
		t.Fatal("expected a request")
	}
	got, err := svc.GetRequest(ctx, req.ID)
	if err != nil || got.Status != models.ApprovalStatusPending {
		t.Fatalf("GetRequest: %v / %+v", err, got)
	}
	if pending, err := svc.ListPending(ctx); err != nil || len(pending) == 0 {
		t.Fatalf("ListPending: %v / %d", err, len(pending))
	}
	if err := svc.Approve(ctx, req.ID, approverID, "approver@x.com", "looks good"); err != nil {
		t.Errorf("Approve: %v", err)
	}

	// Submit + reject (separate content).
	req2, err := svc.SubmitContentForApproval(ctx, newContent("/b"), submitterID, "sub@x.com")
	if err != nil {
		t.Fatalf("Submit2: %v", err)
	}
	if err := svc.Reject(ctx, req2.ID, approverID, "approver@x.com", "Approver", "needs work"); err != nil {
		t.Errorf("Reject: %v", err)
	}

	// Submit + cancel (separate content).
	req3, err := svc.SubmitContentForApproval(ctx, newContent("/c"), submitterID, "sub@x.com")
	if err != nil {
		t.Fatalf("Submit3: %v", err)
	}
	if err := svc.Cancel(ctx, req3.ID); err != nil {
		t.Errorf("Cancel: %v", err)
	}

	// ListMyQueue for the approver.
	if _, err := svc.ListMyQueue(ctx, approverID); err != nil {
		t.Errorf("ListMyQueue: %v", err)
	}
}
