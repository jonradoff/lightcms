package auth

import (
	"context"

	"lightcms/internal/middleware"
)

// UserFromAPIContext extracts the authenticated SessionUser from an API request context.
// The middleware stores the user as interface{}; this provides the typed accessor.
func UserFromAPIContext(ctx context.Context) (*SessionUser, bool) {
	val, ok := middleware.APIUserFromContext(ctx)
	if !ok {
		return nil, false
	}
	user, ok := val.(*SessionUser)
	return user, ok
}

// Permission constants for RBAC
const (
	// Content
	PermContentView    = "content.view"
	PermContentCreate  = "content.create"
	PermContentEdit    = "content.edit"
	PermContentDelete  = "content.delete"
	PermContentPublish = "content.publish"

	// Templates
	PermTemplateView   = "template.view"
	PermTemplateCreate = "template.create"
	PermTemplateEdit   = "template.edit"
	PermTemplateDelete = "template.delete"

	// Assets
	PermAssetView   = "asset.view"
	PermAssetUpload = "asset.upload"
	PermAssetDelete = "asset.delete"

	// Settings (theme, config, redirects, folders, collections)
	PermSettingsView = "settings.view"
	PermSettingsEdit = "settings.edit"

	// Search & replace
	PermSearchReplace = "search_replace.execute"

	// API keys
	PermAPIKeyManage    = "apikey.manage"     // manage own keys
	PermAPIKeyManageAll = "apikey.manage_all" // manage all keys

	// User management
	PermUserManage = "user.manage"

	// Audit logs
	PermAuditView = "audit.view"

	// Content forks
	PermForkCreate = "fork.create" // create + edit fork pages (editor+)
	PermForkMerge  = "fork.merge"  // merge fork into live (admin only)

	// Approvals & discussions
	PermContentSubmitApproval   = "content.submit_approval"    // submit content for approval (contributor+)
	PermApprovalView            = "approval.view"              // view approval requests
	PermApprovalDecide          = "approval.decide"            // approve or reject requests (editor+)
	PermApprovalManageWorkflows = "approval.manage_workflows"  // create/edit/delete workflows (editor+)
	PermDiscussionPost          = "discussion.post"            // post comments (contributor+)
	PermCommentDelete           = "comment.delete"             // delete any comment (admin only)
)

// RolePermissions maps each role to its allowed permissions
var RolePermissions = map[string][]string{
	"viewer": {
		PermContentView,
		PermTemplateView,
		PermAssetView,
		PermSettingsView,
	},
	"contributor": {
		PermContentView, PermContentCreate,
		PermContentSubmitApproval,
		PermApprovalView,
		PermDiscussionPost,
		PermTemplateView,
		PermAssetView, PermAssetUpload, // uploads flagged pending_review
		PermSettingsView,
		PermAPIKeyManage,
	},
	"editor": {
		PermContentView, PermContentCreate, PermContentEdit, PermContentDelete, PermContentPublish,
		PermContentSubmitApproval,
		PermApprovalView, PermApprovalDecide, PermApprovalManageWorkflows,
		PermDiscussionPost,
		PermTemplateView,
		PermAssetView, PermAssetUpload, PermAssetDelete,
		PermSettingsView,
		PermAPIKeyManage,
		PermForkCreate,
	},
	"admin": {
		PermContentView, PermContentCreate, PermContentEdit, PermContentDelete, PermContentPublish,
		PermContentSubmitApproval,
		PermApprovalView, PermApprovalDecide, PermApprovalManageWorkflows,
		PermDiscussionPost, PermCommentDelete,
		PermTemplateView, PermTemplateCreate, PermTemplateEdit, PermTemplateDelete,
		PermAssetView, PermAssetUpload, PermAssetDelete,
		PermSettingsView, PermSettingsEdit,
		PermSearchReplace,
		PermAPIKeyManage, PermAPIKeyManageAll,
		PermUserManage,
		PermAuditView,
		PermForkCreate, PermForkMerge,
	},
}

// HasPermission checks if a role has the specified permission
func HasPermission(role, perm string) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// SessionUser represents the authenticated user extracted from a session or API context
type SessionUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
