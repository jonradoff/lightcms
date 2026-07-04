package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ApprovalService manages approval workflows and requests.
type ApprovalService struct {
	db             *database.DB
	contentService *ContentService
	commentService *CommentService
	webhookService *WebhookService
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(db *database.DB, cs *ContentService, cms *CommentService, ws *WebhookService) *ApprovalService {
	return &ApprovalService{
		db:             db,
		contentService: cs,
		commentService: cms,
		webhookService: ws,
	}
}

// ==================== Workflow CRUD ====================

func (s *ApprovalService) CreateWorkflow(ctx context.Context, wf models.ApprovalWorkflow) (*models.ApprovalWorkflow, error) {
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now
	id, err := s.db.InsertOne(ctx, "approval_workflows", &wf)
	if err != nil {
		return nil, fmt.Errorf("creating workflow: %w", err)
	}
	wf.ID = id
	return &wf, nil
}

func (s *ApprovalService) GetWorkflow(ctx context.Context, id primitive.ObjectID) (*models.ApprovalWorkflow, error) {
	var wf models.ApprovalWorkflow
	if err := s.db.FindOne(ctx, "approval_workflows", bson.M{"_id": id}, &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

func (s *ApprovalService) UpdateWorkflow(ctx context.Context, id primitive.ObjectID, wf models.ApprovalWorkflow) error {
	update := bson.M{"$set": bson.M{
		"name":          wf.Name,
		"description":   wf.Description,
		"trigger":       wf.Trigger,
		"trigger_value": wf.TriggerValue,
		"approvers":     wf.Approvers,
		"mode":          wf.Mode,
		"updated_at":    time.Now(),
	}}
	return s.db.UpdateOne(ctx, "approval_workflows", bson.M{"_id": id}, update)
}

func (s *ApprovalService) DeleteWorkflow(ctx context.Context, id primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "approval_workflows", bson.M{"_id": id})
}

func (s *ApprovalService) ListWorkflows(ctx context.Context) ([]models.ApprovalWorkflow, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := s.db.FindMany(ctx, "approval_workflows", bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var workflows []models.ApprovalWorkflow
	if err := cursor.All(ctx, &workflows); err != nil {
		return nil, err
	}
	if workflows == nil {
		workflows = []models.ApprovalWorkflow{}
	}
	return workflows, nil
}

// ==================== Approval Request Lifecycle ====================

// SubmitContentForApproval creates an ApprovalRequest for a content item and
// marks the content as pending_approval. Called when a Contributor saves/publishes,
// or when an Editor explicitly submits content to a workflow.
func (s *ApprovalService) SubmitContentForApproval(ctx context.Context,
	content *models.Content,
	submitterID primitive.ObjectID, submitterEmail string,
) (*models.ApprovalRequest, error) {
	// Check if there's already a pending request for this content
	existing, _ := s.GetPendingRequestForContent(ctx, content.ID)
	if existing != nil {
		return existing, nil // idempotent
	}

	wf, err := s.matchWorkflow(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("matching workflow: %w", err)
	}

	req := models.ApprovalRequest{
		ContentID:        content.ID,
		ContentTitle:     content.Title,
		ContentPath:      content.FullPath,
		SubmittedByID:    submitterID,
		SubmittedByEmail: submitterEmail,
		Status:           models.ApprovalStatusPending,
		CurrentStep:      0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if wf != nil {
		req.WorkflowID = &wf.ID
		req.Approvers = wf.Approvers
		if wf.Mode == models.WorkflowModeSequential {
			req.RequiredApprovals = len(wf.Approvers)
		} else {
			// Concurrent: require majority by default; workflow could specify otherwise
			req.RequiredApprovals = (len(wf.Approvers) + 1) / 2
		}
	}

	id, err := s.db.InsertOne(ctx, "approval_requests", &req)
	if err != nil {
		return nil, fmt.Errorf("creating approval request: %w", err)
	}
	req.ID = id

	// Mark content as pending approval
	s.db.UpdateOne(ctx, "content", bson.M{"_id": content.ID}, //nolint:errcheck
		bson.M{"$set": bson.M{"pending_approval": true, "updated_at": time.Now()}})

	// Fire webhook
	if s.webhookService != nil {
		payload := map[string]interface{}{
			"request_id":         req.ID.Hex(),
			"content_id":         content.ID.Hex(),
			"content_title":      content.Title,
			"content_path":       content.FullPath,
			"submitted_by_email": submitterEmail,
		}
		if wf != nil {
			payload["workflow_id"] = wf.ID.Hex()
			payload["workflow_name"] = wf.Name
		}
		go s.webhookService.FireEvent(context.Background(), "content.pending_approval", payload)
	}

	return &req, nil
}

// SubmitAssetForReview creates an ApprovalRequest for an asset uploaded by a Contributor.
func (s *ApprovalService) SubmitAssetForReview(ctx context.Context,
	asset *models.Asset,
	submitterID primitive.ObjectID, submitterEmail string,
) (*models.ApprovalRequest, error) {
	req := models.ApprovalRequest{
		ContentTitle:     asset.Filename,
		ContentPath:      asset.FullPath,
		AssetID:          &asset.ID,
		SubmittedByID:    submitterID,
		SubmittedByEmail: submitterEmail,
		Status:           models.ApprovalStatusPending,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	id, err := s.db.InsertOne(ctx, "approval_requests", &req)
	if err != nil {
		return nil, fmt.Errorf("creating asset review request: %w", err)
	}
	req.ID = id

	if s.webhookService != nil {
		go s.webhookService.FireEvent(context.Background(), "asset.pending_review", map[string]interface{}{
			"request_id":         req.ID.Hex(),
			"asset_id":           asset.ID.Hex(),
			"filename":           asset.Filename,
			"submitted_by_email": submitterEmail,
		})
	}

	return &req, nil
}

// GetPendingRequestForContent returns the active (pending) approval request for a content item.
func (s *ApprovalService) GetPendingRequestForContent(ctx context.Context, contentID primitive.ObjectID) (*models.ApprovalRequest, error) {
	var req models.ApprovalRequest
	err := s.db.FindOne(ctx, "approval_requests",
		bson.M{"content_id": contentID, "status": models.ApprovalStatusPending}, &req)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// GetRequest fetches an approval request by ID.
func (s *ApprovalService) GetRequest(ctx context.Context, id primitive.ObjectID) (*models.ApprovalRequest, error) {
	var req models.ApprovalRequest
	if err := s.db.FindOne(ctx, "approval_requests", bson.M{"_id": id}, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ListPending returns all pending approval requests, newest first.
func (s *ApprovalService) ListPending(ctx context.Context) ([]models.ApprovalRequest, error) {
	return s.listRequests(ctx, bson.M{"status": models.ApprovalStatusPending})
}

// ListMyQueue returns pending requests where the given user is the current/next approver.
// For default requests (no workflow), any editor/admin sees them (callers filter by role).
// For workflow requests: sequential → user must be the approver at CurrentStep;
// concurrent → user must be in the approvers list.
func (s *ApprovalService) ListMyQueue(ctx context.Context, userID primitive.ObjectID) ([]models.ApprovalRequest, error) {
	filter := bson.M{
		"status": models.ApprovalStatusPending,
		"$or": []bson.M{
			// No workflow — visible to any editor/admin (caller must check role)
			{"workflow_id": bson.M{"$exists": false}},
			{"workflow_id": nil},
			// User is in the approvers list for this request
			{"approvers.user_id": userID},
		},
	}
	return s.listRequests(ctx, filter)
}

// CountPending returns the total number of pending approval requests.
func (s *ApprovalService) CountPending(ctx context.Context) int64 {
	n, _ := s.db.Count(ctx, "approval_requests", bson.M{"status": models.ApprovalStatusPending})
	return n
}

// Approve records an approval decision. If all required approvals are received
// (or the current step advances to the final step), the content is published.
func (s *ApprovalService) Approve(ctx context.Context,
	requestID, approverID primitive.ObjectID,
	approverEmail, comment string,
) error {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}
	if req.Status != models.ApprovalStatusPending {
		return fmt.Errorf("approval request is not pending")
	}

	// Prevent self-approval: the submitter cannot approve their own request.
	if !approverID.IsZero() && approverID == req.SubmittedByID {
		return fmt.Errorf("you cannot approve your own submission")
	}

	// For sequential workflows, enforce step order: only the approver assigned to
	// the current step may approve.
	if req.WorkflowID != nil && len(req.Approvers) > 0 {
		wf, _ := s.GetWorkflow(ctx, *req.WorkflowID)
		if wf != nil && wf.Mode == models.WorkflowModeSequential {
			if req.CurrentStep < len(req.Approvers) {
				stepApproverID := req.Approvers[req.CurrentStep].UserID
				if !approverID.IsZero() && stepApproverID != approverID {
					return fmt.Errorf("it is not your turn to approve this request (step %d requires a different approver)", req.CurrentStep+1)
				}
			}
		}
	}

	decision := models.ApprovalDecision{
		UserID:    approverID,
		UserEmail: approverEmail,
		Decision:  "approved",
		Comment:   comment,
		DecidedAt: time.Now(),
	}

	update := bson.M{
		"$push": bson.M{"decisions": decision},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	// Count existing approvals + this one
	approvedCount := 1
	for _, d := range req.Decisions {
		if d.Decision == "approved" {
			approvedCount++
		}
	}

	// Sequential: advance step; if last step, approve
	if req.WorkflowID != nil && len(req.Approvers) > 0 {
		wf, _ := s.GetWorkflow(ctx, *req.WorkflowID)
		if wf != nil && wf.Mode == models.WorkflowModeSequential {
			nextStep := req.CurrentStep + 1
			update["$set"].(bson.M)["current_step"] = nextStep
			if nextStep < req.RequiredApprovals {
				// More steps remain
				return s.db.UpdateOne(ctx, "approval_requests", bson.M{"_id": requestID}, update)
			}
			// All steps complete → fall through to publish
		}
	}

	// Concurrent / no-workflow: check threshold
	if approvedCount >= req.RequiredApprovals || req.WorkflowID == nil {
		update["$set"].(bson.M)["status"] = models.ApprovalStatusApproved
		if err := s.db.UpdateOne(ctx, "approval_requests", bson.M{"_id": requestID}, update); err != nil {
			return err
		}
		// Publish the content
		if !req.ContentID.IsZero() {
			s.db.UpdateOne(ctx, "content", bson.M{"_id": req.ContentID}, //nolint:errcheck
				bson.M{"$set": bson.M{
					"published":        true,
					"pending_approval": false,
					"published_at":     time.Now(),
					"updated_at":       time.Now(),
				}})
			// Generate static page
			if s.contentService != nil {
				var content models.Content
				if e := s.db.FindOne(ctx, "content", bson.M{"_id": req.ContentID}, &content); e == nil {
					go s.contentService.GenerateStaticPage(context.Background(), &content)
				}
			}
		}
		// Approve asset: clear pending_review flag
		if req.AssetID != nil {
			s.db.UpdateOne(ctx, "assets", bson.M{"_id": req.AssetID}, //nolint:errcheck
				bson.M{"$set": bson.M{"pending_review": false, "updated_at": time.Now()}})
		}
		return nil
	}

	// Not yet enough approvals
	return s.db.UpdateOne(ctx, "approval_requests", bson.M{"_id": requestID}, update)
}

// Reject records a rejection decision, sets the request status to rejected,
// clears the pending_approval flag on content, and auto-posts a comment.
func (s *ApprovalService) Reject(ctx context.Context,
	requestID, approverID primitive.ObjectID,
	approverEmail, approverDisplayName, rejectComment string,
) error {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("approval request not found: %w", err)
	}
	if req.Status != models.ApprovalStatusPending {
		return fmt.Errorf("approval request is not pending")
	}

	decision := models.ApprovalDecision{
		UserID:    approverID,
		UserEmail: approverEmail,
		Decision:  "rejected",
		Comment:   rejectComment,
		DecidedAt: time.Now(),
	}
	if err := s.db.UpdateOne(ctx, "approval_requests", bson.M{"_id": requestID}, bson.M{
		"$push": bson.M{"decisions": decision},
		"$set":  bson.M{"status": models.ApprovalStatusRejected, "updated_at": time.Now()},
	}); err != nil {
		return err
	}

	// Clear pending_approval on content
	if !req.ContentID.IsZero() {
		s.db.UpdateOne(ctx, "content", bson.M{"_id": req.ContentID}, //nolint:errcheck
			bson.M{"$set": bson.M{"pending_approval": false, "updated_at": time.Now()}})
	}
	// Clear pending_review on asset
	if req.AssetID != nil {
		s.db.UpdateOne(ctx, "assets", bson.M{"_id": req.AssetID}, //nolint:errcheck
			bson.M{"$set": bson.M{"pending_review": false, "updated_at": time.Now()}})
	}

	// Auto-post rejection comment to discussion
	if s.commentService != nil && !req.ContentID.IsZero() && rejectComment != "" {
		text := "❌ Rejected: " + rejectComment
		s.commentService.Create(context.Background(), req.ContentID, approverID, approverEmail, approverDisplayName, text, nil) //nolint:errcheck
	}

	return nil
}

// Cancel marks an approval request as cancelled.
func (s *ApprovalService) Cancel(ctx context.Context, requestID primitive.ObjectID) error {
	return s.db.UpdateOne(ctx, "approval_requests", bson.M{"_id": requestID},
		bson.M{"$set": bson.M{"status": models.ApprovalStatusCancelled, "updated_at": time.Now()}})
}

// ==================== Internals ====================

// matchWorkflow finds the best-matching workflow for a content item.
// Priority: template_id > folder_path > tag > all_contributor
func (s *ApprovalService) matchWorkflow(ctx context.Context, content *models.Content) (*models.ApprovalWorkflow, error) {
	var workflows []models.ApprovalWorkflow
	cursor, err := s.db.FindMany(ctx, "approval_workflows", bson.M{}, nil)
	if err != nil {
		return nil, err
	}
	if err := cursor.All(ctx, &workflows); err != nil {
		return nil, err
	}

	var byTemplateID, byFolderPath, byTag, byAllContributor *models.ApprovalWorkflow

	for i, wf := range workflows {
		switch wf.Trigger {
		case models.WorkflowTriggerTemplateID:
			if wf.TriggerValue == content.TemplateID.Hex() {
				byTemplateID = &workflows[i]
			}
		case models.WorkflowTriggerFolderPath:
			if strings.HasPrefix(content.FolderPath, wf.TriggerValue) {
				byFolderPath = &workflows[i]
			}
		case models.WorkflowTriggerTag:
			for _, tag := range content.Tags {
				if tag == wf.TriggerValue {
					byTag = &workflows[i]
					break
				}
			}
		case models.WorkflowTriggerAllContributor:
			byAllContributor = &workflows[i]
		}
	}

	if byTemplateID != nil {
		return byTemplateID, nil
	}
	if byFolderPath != nil {
		return byFolderPath, nil
	}
	if byTag != nil {
		return byTag, nil
	}
	return byAllContributor, nil // may be nil (use default any-editor behavior)
}

func (s *ApprovalService) listRequests(ctx context.Context, filter bson.M) ([]models.ApprovalRequest, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := s.db.FindMany(ctx, "approval_requests", filter, opts)
	if err != nil {
		return nil, err
	}
	var reqs []models.ApprovalRequest
	if err := cursor.All(ctx, &reqs); err != nil {
		return nil, err
	}
	if reqs == nil {
		reqs = []models.ApprovalRequest{}
	}
	return reqs, nil
}
