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
