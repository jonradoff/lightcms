package handlers

import (
	"net/http"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/services"

	"github.com/gorilla/mux"
)

// SetAgentSessionService injects the agent-session ledger service.
func (a *APIHandler) SetAgentSessionService(s *services.AgentSessionService) {
	a.agentSessionService = s
}

// APIAgentSessionChanges returns the ledger of everything an agent session
// changed, grouped per content item.
func (a *APIHandler) APIAgentSessionChanges(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAuditView) {
		return
	}
	sessionID := mux.Vars(r)["id"]
	summary, err := a.agentSessionService.Changes(r.Context(), sessionID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, summary)
}

// APIAgentSessionRollback reverts everything an agent session changed:
// created content is soft-deleted, deleted content restored, updated
// content reverted to its pre-session version.
func (a *APIHandler) APIAgentSessionRollback(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}
	if !a.requirePermission(w, r, auth.PermContentDelete) {
		return
	}
	sessionID := mux.Vars(r)["id"]
	result, err := a.agentSessionService.Rollback(r.Context(), sessionID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.auditLog(r, "agent_session.rollback", "agent_session", sessionID, map[string]interface{}{
		"reverted": len(result.Reverted), "deleted": len(result.Deleted),
		"restored": len(result.Restored), "skipped": len(result.Skipped),
	})
	a.jsonResponse(w, http.StatusOK, result)
}

// SetMaintenanceService injects the maintenance scan service.
func (a *APIHandler) SetMaintenanceService(m *services.MaintenanceService) {
	a.maintenanceService = m
}

// APIMaintenanceReport returns the most recent maintenance report.
func (a *APIHandler) APIMaintenanceReport(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentView) {
		return
	}
	report, err := a.maintenanceService.LatestReport(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "no maintenance report yet — run a scan first")
		return
	}
	a.jsonResponse(w, http.StatusOK, report)
}

// APIMaintenanceScan runs a maintenance scan now and returns the report.
func (a *APIHandler) APIMaintenanceScan(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentView) {
		return
	}
	withLinks := r.URL.Query().Get("link_check") == "true"
	report, err := a.maintenanceService.RunScan(r.Context(), withLinks)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "maintenance.scan", "maintenance", report.ID.Hex(), map[string]interface{}{
		"stale": len(report.StalePages), "missing_meta": len(report.MissingMeta), "drafts": len(report.Drafts),
	})
	a.jsonResponse(w, http.StatusOK, report)
}
