package handlers

import (
	"net/http"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ==================== Users (for mention/approver search) ====================

// APIListUsers returns a minimal user list for UI features like @mention and approver selection.
// GET /api/v1/users
func (a *APIHandler) APIListUsers(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentView) {
		return
	}
	if a.userService == nil {
		a.jsonResponse(w, http.StatusOK, map[string]interface{}{"users": []interface{}{}})
		return
	}
	users, err := a.userService.ListUsers(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type userSummary struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	result := make([]userSummary, 0, len(users))
	for _, u := range users {
		if !u.Disabled {
			result = append(result, userSummary{
				ID:          u.ID.Hex(),
				Email:       u.Email,
				DisplayName: u.DisplayName,
				Role:        u.Role,
			})
		}
	}
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"users": result})
}

// ==================== Approval Workflows ====================

// APIListApprovalWorkflows returns all configured approval workflows.
// GET /api/v1/approval-workflows
func (a *APIHandler) APIListApprovalWorkflows(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalManageWorkflows) {
		return
	}
	wfs, err := a.approvalService.ListWorkflows(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, wfs)
}

// APIGetApprovalWorkflow returns one workflow.
// GET /api/v1/approval-workflows/{id}
func (a *APIHandler) APIGetApprovalWorkflow(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalManageWorkflows) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid workflow ID")
		return
	}
	wf, err := a.approvalService.GetWorkflow(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "workflow not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, wf)
}

// APICreateApprovalWorkflow creates a new workflow.
// POST /api/v1/approval-workflows
func (a *APIHandler) APICreateApprovalWorkflow(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalManageWorkflows) {
		return
	}
	var req struct {
		Name         string                    `json:"name"`
		Description  string                    `json:"description"`
		Trigger      string                    `json:"trigger"`
		TriggerValue string                    `json:"trigger_value"`
		Approvers    []models.WorkflowApprover `json:"approvers"`
		Mode         string                    `json:"mode"`
	}
	if err := a.decodeJSON(r, &req); err != nil || req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !validWorkflowTrigger(req.Trigger) {
		a.jsonError(w, http.StatusBadRequest, "trigger must be one of: all_contributor, folder_path, template_id, tag")
		return
	}
	if req.Mode != models.WorkflowModeSequential && req.Mode != models.WorkflowModeConcurrent {
		a.jsonError(w, http.StatusBadRequest, "mode must be sequential or concurrent")
		return
	}

	// Validate that all approver user IDs refer to real users
	if a.userService != nil {
		for _, approver := range req.Approvers {
			if _, err := a.userService.GetByID(r.Context(), approver.UserID); err != nil {
				a.jsonError(w, http.StatusBadRequest, "approver user ID not found: "+approver.UserID.Hex())
				return
			}
		}
	}

	user := a.getAPIUser(r)
	var createdBy primitive.ObjectID
	if user != nil {
		createdBy, _ = primitive.ObjectIDFromHex(user.ID)
	}

	wf := models.ApprovalWorkflow{
		Name:         req.Name,
		Description:  req.Description,
		Trigger:      req.Trigger,
		TriggerValue: req.TriggerValue,
		Approvers:    req.Approvers,
		Mode:         req.Mode,
		CreatedBy:    createdBy,
	}
	created, err := a.approvalService.CreateWorkflow(r.Context(), wf)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "approval_workflow.create", "approval_workflow", created.ID.Hex(), map[string]interface{}{"name": created.Name})
	a.jsonResponse(w, http.StatusCreated, created)
}

// APIUpdateApprovalWorkflow updates a workflow.
// PUT /api/v1/approval-workflows/{id}
func (a *APIHandler) APIUpdateApprovalWorkflow(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalManageWorkflows) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid workflow ID")
		return
	}
	var req struct {
		Name         string                    `json:"name"`
		Description  string                    `json:"description"`
		Trigger      string                    `json:"trigger"`
		TriggerValue string                    `json:"trigger_value"`
		Approvers    []models.WorkflowApprover `json:"approvers"`
		Mode         string                    `json:"mode"`
	}
	if err := a.decodeJSON(r, &req); err != nil || req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Validate that all approver user IDs refer to real users
	if a.userService != nil {
		for _, approver := range req.Approvers {
			if _, err := a.userService.GetByID(r.Context(), approver.UserID); err != nil {
				a.jsonError(w, http.StatusBadRequest, "approver user ID not found: "+approver.UserID.Hex())
				return
			}
		}
	}

	wf := models.ApprovalWorkflow{
		Name: req.Name, Description: req.Description,
		Trigger: req.Trigger, TriggerValue: req.TriggerValue,
		Approvers: req.Approvers, Mode: req.Mode,
	}
	if err := a.approvalService.UpdateWorkflow(r.Context(), id, wf); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "approval_workflow.update", "approval_workflow", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIDeleteApprovalWorkflow deletes a workflow.
// DELETE /api/v1/approval-workflows/{id}
func (a *APIHandler) APIDeleteApprovalWorkflow(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalManageWorkflows) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid workflow ID")
		return
	}
	if err := a.approvalService.DeleteWorkflow(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "approval_workflow.delete", "approval_workflow", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// ==================== Approval Requests ====================

// APIListApprovalRequests lists pending approval requests.
// GET /api/v1/approval-requests?filter=mine|all
func (a *APIHandler) APIListApprovalRequests(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalView) {
		return
	}
	filter := r.URL.Query().Get("filter")
	user := a.getAPIUser(r)

	var reqs []models.ApprovalRequest
	var err error

	if filter == "mine" && user != nil {
		userID, _ := primitive.ObjectIDFromHex(user.ID)
		reqs, err = a.approvalService.ListMyQueue(r.Context(), userID)
	} else {
		reqs, err = a.approvalService.ListPending(r.Context())
	}
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, reqs)
}

// APIGetApprovalRequest returns a single approval request.
// GET /api/v1/approval-requests/{id}
func (a *APIHandler) APIGetApprovalRequest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalView) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	req, err := a.approvalService.GetRequest(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "approval request not found")
		return
	}
	a.jsonResponse(w, http.StatusOK, req)
}

// APISubmitForApproval explicitly submits content for approval.
// POST /api/v1/content/{id}/submit-approval
func (a *APIHandler) APISubmitForApproval(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentSubmitApproval) {
		return
	}
	contentID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content ID")
		return
	}
	content, err := a.contentService.GetContent(r.Context(), contentID)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "content not found")
		return
	}
	user := a.getAPIUser(r)
	var submitterID primitive.ObjectID
	var submitterEmail string
	if user != nil {
		submitterID, _ = primitive.ObjectIDFromHex(user.ID)
		submitterEmail = user.Email
	}
	req, err := a.approvalService.SubmitContentForApproval(r.Context(), content, submitterID, submitterEmail)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusCreated, req)
}

// APIApproveRequest approves an approval request.
// POST /api/v1/approval-requests/{id}/approve
func (a *APIHandler) APIApproveRequest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalDecide) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	var body struct {
		Comment string `json:"comment"`
	}
	a.decodeJSON(r, &body) //nolint:errcheck

	user := a.getAPIUser(r)
	var approverID primitive.ObjectID
	var approverEmail string
	if user != nil {
		approverID, _ = primitive.ObjectIDFromHex(user.ID)
		approverEmail = user.Email
	}
	if err := a.approvalService.Approve(r.Context(), id, approverID, approverEmail, body.Comment); err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.auditLog(r, "approval.approve", "approval_request", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APIRejectRequest rejects an approval request. A comment is required.
// POST /api/v1/approval-requests/{id}/reject
func (a *APIHandler) APIRejectRequest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalDecide) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	var body struct {
		Comment string `json:"comment"`
	}
	if err := a.decodeJSON(r, &body); err != nil || strings.TrimSpace(body.Comment) == "" {
		a.jsonError(w, http.StatusBadRequest, "a rejection comment is required")
		return
	}

	user := a.getAPIUser(r)
	var approverID primitive.ObjectID
	var approverEmail, displayName string
	if user != nil {
		approverID, _ = primitive.ObjectIDFromHex(user.ID)
		approverEmail = user.Email
		displayName = user.Email
	}
	if err := a.approvalService.Reject(r.Context(), id, approverID, approverEmail, displayName, body.Comment); err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.auditLog(r, "approval.reject", "approval_request", id.Hex(), map[string]interface{}{"comment": body.Comment})
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// APICancelRequest cancels an approval request.
// POST /api/v1/approval-requests/{id}/cancel
func (a *APIHandler) APICancelRequest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermApprovalDecide) {
		return
	}
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request ID")
		return
	}
	if err := a.approvalService.Cancel(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}

// validWorkflowTrigger checks the trigger type value.
func validWorkflowTrigger(t string) bool {
	switch t {
	case models.WorkflowTriggerAllContributor,
		models.WorkflowTriggerFolderPath,
		models.WorkflowTriggerTemplateID,
		models.WorkflowTriggerTag:
		return true
	}
	return false
}
