package handlers

import (
	"net/http"
	"strconv"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/services"
)

// GET /api/v1/audit
func (a *APIHandler) APIListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := a.getAPIUser(r)
	if user == nil || !auth.HasPermission(user.Role, auth.PermAuditView) {
		a.jsonError(w, http.StatusForbidden, "admin role required")
		return
	}
	if a.auditService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "audit service unavailable")
		return
	}

	filter := services.AuditFilter{}

	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}

	if v := r.URL.Query().Get("action"); v != "" {
		filter.Action = v
	}
	if v := r.URL.Query().Get("resource"); v != "" {
		filter.Resource = v
	}

	logs, total, err := a.auditService.List(r.Context(), filter)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = nil
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"logs":  logs,
	})
}
