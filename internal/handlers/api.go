package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"lightcms/internal/auth"
	"lightcms/internal/models"
	"lightcms/internal/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// APIHandler handles REST API endpoints (JSON-only, no sessions/templates)
type APIHandler struct {
	contentService  *services.ContentService
	templateService *services.TemplateService
	assetService    *services.AssetService
	settingsService *services.SettingsService
	apiKeyService   *services.APIKeyService
	searchService   *services.SearchService
	auditService    *services.AuditService
	snippetService  *services.SnippetService
	forkService     *services.ForkService
	importService   *services.ImportService
	webhookService     *services.WebhookService
	lockService        *services.LockService
	linkCheckerService *services.LinkCheckerService
	commentService     *services.CommentService
	approvalService    *services.ApprovalService
	userService        *services.UserService
}

// SetCommentService wires in the comment service.
func (a *APIHandler) SetCommentService(cs *services.CommentService) { a.commentService = cs }

// SetApprovalService wires in the approval service.
func (a *APIHandler) SetApprovalService(as *services.ApprovalService) { a.approvalService = as }

// SetUserService wires in the user service (used for approver ID validation).
func (a *APIHandler) SetUserService(us *services.UserService) { a.userService = us }

// NewAPIHandler creates a new API handler
func NewAPIHandler(
	contentService *services.ContentService,
	templateService *services.TemplateService,
	assetService *services.AssetService,
	settingsService *services.SettingsService,
	apiKeyService *services.APIKeyService,
	auditService *services.AuditService,
	snippetService *services.SnippetService,
) *APIHandler {
	return &APIHandler{
		contentService:  contentService,
		templateService: templateService,
		assetService:    assetService,
		settingsService: settingsService,
		apiKeyService:   apiKeyService,
		auditService:    auditService,
		snippetService:  snippetService,
	}
}

// getAPIUser extracts the authenticated user from an API request context.
// Returns nil if no user is set (e.g., legacy/system key without user association).
func (a *APIHandler) getAPIUser(r *http.Request) *auth.SessionUser {
	user, _ := auth.UserFromAPIContext(r.Context())
	return user
}

// requirePermission checks if the authenticated API user has the required permission.
// Returns true if allowed, false if denied (and writes a 403 response).
func (a *APIHandler) requirePermission(w http.ResponseWriter, r *http.Request, perm string) bool {
	user := a.getAPIUser(r)
	if user == nil {
		// No user context — allow for backward compatibility (system keys without user)
		return true
	}
	if !auth.HasPermission(user.Role, perm) {
		a.jsonError(w, http.StatusForbidden, "insufficient permissions")
		return false
	}
	return true
}

// auditLog fires an async audit log entry for the current API user
func (a *APIHandler) auditLog(r *http.Request, action, resource, resourceID string, details map[string]interface{}) {
	user := a.getAPIUser(r)
	entry := models.AuditLog{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
	}
	if user != nil {
		if oid, err := primitive.ObjectIDFromHex(user.ID); err == nil {
			entry.UserID = oid
		}
		entry.UserEmail = user.Email
		entry.ViaAPI = user.ViaAPIKey
	}
	a.auditService.LogAsync(entry)
}

// jsonResponse writes a JSON response with the given status code
func (a *APIHandler) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response
func (a *APIHandler) jsonError(w http.ResponseWriter, statusCode int, message string) {
	a.jsonResponse(w, statusCode, map[string]interface{}{
		"error": message,
	})
}

// decodeJSON reads and decodes the request body into the given target
func (a *APIHandler) decodeJSON(r *http.Request, target interface{}) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
