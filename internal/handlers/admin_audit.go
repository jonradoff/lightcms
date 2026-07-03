package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	// Fetch rate-limit data for the rate limits tab
	loginAttempts, _ := h.db.GetAllLoginAttempts(r.Context())

	h.renderAdmin(w, r, "audit_log", map[string]interface{}{
		"Logs":          logs,
		"Total":         total,
		"CurrentPage":   currentPage,
		"TotalPages":    totalPages,
		"Filter":        filter,
		"LoginAttempts": loginAttempts,
	})
}

// ClearRateLimit clears the rate-limit record for a specific IP (admin only).
func (h *Handler) ClearRateLimit(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ip := mux.Vars(r)["ip"]
	if err := h.db.ClearLoginAttemptsByIP(r.Context(), ip); err != nil {
		http.Error(w, "Failed to clear rate limit", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/audit", http.StatusSeeOther)
}

// ForceUnlock removes a content lock (admin only).
func (h *Handler) ForceUnlock(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.lockService == nil {
		http.Error(w, "Lock service unavailable", http.StatusServiceUnavailable)
		return
	}

	contentIDStr := mux.Vars(r)["id"]
	contentID, err := primitive.ObjectIDFromHex(contentIDStr)
	if err != nil {
		http.Error(w, "Invalid content ID", http.StatusBadRequest)
		return
	}

	if err := h.lockService.ForceUnlock(r.Context(), contentID); err != nil {
		http.Error(w, "Failed to force unlock", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/content/"+contentIDStr, http.StatusSeeOther)
}

// RefreshLock extends the expiry of an existing content lock (heartbeat endpoint).
func (h *Handler) RefreshLock(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if h.lockService == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	contentIDStr := mux.Vars(r)["id"]
	contentID, err := primitive.ObjectIDFromHex(contentIDStr)
	if err != nil {
		http.Error(w, "Invalid content ID", http.StatusBadRequest)
		return
	}

	userID, _ := primitive.ObjectIDFromHex(user.ID)
	if err := h.lockService.RefreshLock(r.Context(), contentID, userID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Non-fatal — just log
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
