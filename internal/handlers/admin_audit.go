package handlers

import (
	"net/http"
	"strconv"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/services"
)

// AuditLogPage shows the audit log listing (admin only)
func (h *Handler) AuditLogPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	filter := services.AuditFilter{
		Limit: 50,
	}

	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = action
	}
	if resource := r.URL.Query().Get("resource"); resource != "" {
		filter.Resource = resource
	}
	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 1 {
			filter.Offset = (p - 1) * filter.Limit
		}
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse("2006-01-02", since); err == nil {
			filter.Since = &t
		}
	}
	if until := r.URL.Query().Get("until"); until != "" {
		if t, err := time.Parse("2006-01-02", until); err == nil {
			end := t.Add(24 * time.Hour) // include the full day
			filter.Until = &end
		}
	}

	logs, total, err := h.auditService.List(r.Context(), filter)
	if err != nil {
		http.Error(w, "Failed to load audit logs", http.StatusInternalServerError)
		return
	}

	currentPage := (filter.Offset / filter.Limit) + 1
	totalPages := int(total) / filter.Limit
	if int(total)%filter.Limit > 0 {
		totalPages++
	}

	h.renderAdmin(w, r, "audit_log", map[string]interface{}{
		"Logs":        logs,
		"Total":       total,
		"CurrentPage": currentPage,
		"TotalPages":  totalPages,
		"Filter":      filter,
	})
}
