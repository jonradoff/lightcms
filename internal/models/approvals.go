package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ContentComment is a single comment in a content item's discussion thread.
// Collection: "content_comments"
// Indexes: {content_id,created_at}, {created_at desc}, {mentions}
type ContentComment struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	ContentID       primitive.ObjectID   `bson:"content_id" json:"content_id"`
	UserID          primitive.ObjectID   `bson:"user_id" json:"user_id"`
	UserEmail       string               `bson:"user_email" json:"user_email"`
	UserDisplayName string               `bson:"user_display_name" json:"user_display_name"`
	Text            string               `bson:"text" json:"text"`
	Mentions        []primitive.ObjectID `bson:"mentions,omitempty" json:"mentions,omitempty"`
	CreatedAt       time.Time            `bson:"created_at" json:"created_at"`
}

// ApprovalWorkflow configures an approval chain for matching content.
// Collection: "approval_workflows"
// Indexes: {trigger,trigger_value}, {created_at desc}
type ApprovalWorkflow struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	// Trigger controls which content submissions this workflow applies to.
	// "all_contributor" — any contributor submission
	// "folder_path"    — content whose FolderPath starts with TriggerValue
	// "template_id"    — content using the specified template (ObjectID hex)
	// "tag"            — content tagged with TriggerValue
	Trigger      string             `bson:"trigger" json:"trigger"`
	TriggerValue string             `bson:"trigger_value,omitempty" json:"trigger_value,omitempty"`
	Approvers    []WorkflowApprover `bson:"approvers" json:"approvers"`
	// Mode: "sequential" (each approver in order) or "concurrent" (any N of M)
	Mode      string             `bson:"mode" json:"mode"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// WorkflowApprover is one approver slot within a workflow.
type WorkflowApprover struct {
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserEmail string             `bson:"user_email" json:"user_email"`
	Order     int                `bson:"order" json:"order"` // 0-based; used for sequential mode
}

// ApprovalDecision records a single approver's action on a request.
type ApprovalDecision struct {
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserEmail string             `bson:"user_email" json:"user_email"`
	Decision  string             `bson:"decision" json:"decision"` // "approved" | "rejected"
	Comment   string             `bson:"comment,omitempty" json:"comment,omitempty"`
	DecidedAt time.Time          `bson:"decided_at" json:"decided_at"`
}

// ApprovalRequest tracks one content item through its approval workflow.
// Collection: "approval_requests"
// Indexes: {content_id}, {status,created_at desc}, {"approvers.user_id",status}, {workflow_id}
type ApprovalRequest struct {
	ID               primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ContentID        primitive.ObjectID  `bson:"content_id" json:"content_id"`
	ContentTitle     string              `bson:"content_title" json:"content_title"`
	ContentPath      string              `bson:"content_path" json:"content_path"`
	WorkflowID       *primitive.ObjectID `bson:"workflow_id,omitempty" json:"workflow_id,omitempty"` // nil = default (any editor/admin)
	SubmittedByID    primitive.ObjectID  `bson:"submitted_by_id" json:"submitted_by_id"`
	SubmittedByEmail string              `bson:"submitted_by_email" json:"submitted_by_email"`
	// Status: "pending" | "approved" | "rejected" | "cancelled"
	Status    string             `bson:"status" json:"status"`
	Decisions []ApprovalDecision `bson:"decisions,omitempty" json:"decisions,omitempty"`
	// RequiredApprovals: for concurrent = min approvals needed; for sequential = total steps
	RequiredApprovals int `bson:"required_approvals" json:"required_approvals"`
	// CurrentStep: sequential only — which step is currently active (0-based)
	CurrentStep int `bson:"current_step" json:"current_step"`
	// Approvers mirrors the workflow snapshot for "my queue" queries
	Approvers []WorkflowApprover `bson:"approvers,omitempty" json:"approvers,omitempty"`
	// AssetID is set when this request is for an asset upload (not content)
	AssetID   *primitive.ObjectID `bson:"asset_id,omitempty" json:"asset_id,omitempty"`
	CreatedAt time.Time           `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time           `bson:"updated_at" json:"updated_at"`
}

// ApprovalRequestKind distinguishes content vs asset approval requests
const (
	ApprovalKindContent = "content"
	ApprovalKindAsset   = "asset"
)

// Approval status values
const (
	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
	ApprovalStatusCancelled = "cancelled"
)

// Workflow trigger types
const (
	WorkflowTriggerAllContributor = "all_contributor"
	WorkflowTriggerFolderPath     = "folder_path"
	WorkflowTriggerTemplateID     = "template_id"
	WorkflowTriggerTag            = "tag"
)

// Workflow modes
const (
	WorkflowModeSequential = "sequential"
	WorkflowModeConcurrent = "concurrent"
)
