package handlers

import (
	"net/http"
	"strconv"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SetAgentService wires the CMS Agent service into the handler.
func (h *Handler) SetAgentService(a *services.AgentService) { h.agentService = a }

// AgentToolPage renders the CMS Agent configuration screen (admin only).
func (h *Handler) AgentToolPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	cfg := h.agentService.GetConfig(r.Context())
	q := r.URL.Query()
	h.renderAdmin(w, r, "agent_tool", map[string]interface{}{
		"Title":           "CMS Agent",
		"Config":          cfg,
		"EmailConfigured": h.agentService.EmailConfigured(),
		"AIAvailable":     h.anthropicAPIKey != "",
		"Sent":            q.Get("sent") == "1",
		"Saved":           q.Get("saved") == "1",
		"SendFailed":      q.Get("error") != "",
	})
}

// AgentToolSaveConfig handles the configuration form POST.
func (h *Handler) AgentToolSaveConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	hour, _ := strconv.Atoi(r.FormValue("send_hour"))
	cfg := services.AgentConfig{
		Enabled:             r.FormValue("enabled") == "on",
		Email:               r.FormValue("email"),
		Frequency:           r.FormValue("frequency"),
		SendHour:            hour,
		IncludeSiteHealth:   r.FormValue("include_site_health") == "on",
		IncludeTraffic:      r.FormValue("include_traffic") == "on",
		IncludePending:      r.FormValue("include_pending") == "on",
		IncludeBrokenLinks:  r.FormValue("include_broken_links") == "on",
		IncludeAgentWork:    r.FormValue("include_agent_work") == "on",
		IncludeAICommentary: r.FormValue("include_ai_commentary") == "on",
	}
	if err := h.agentService.SaveConfig(r.Context(), cfg); err != nil {
		h.renderAdmin(w, r, "agent_tool", map[string]interface{}{
			"Title": "CMS Agent", "Config": cfg,
			"EmailConfigured": h.agentService.EmailConfigured(),
			"AIAvailable":     h.anthropicAPIKey != "",
			"Error":           err.Error(),
		})
		return
	}
	if h.auditService != nil {
		uid, _ := primitive.ObjectIDFromHex(user.ID)
		h.auditService.LogAsync(models.AuditLog{
			UserID: uid, UserEmail: user.Email,
			Action: "agent.config_update", Resource: "agent",
			Details: map[string]interface{}{"enabled": cfg.Enabled, "frequency": cfg.Frequency, "email": cfg.Email},
		})
	}
	http.Redirect(w, r, "/cm/tools/agent?saved=1", http.StatusSeeOther)
}

// AgentToolSendTest sends a digest immediately to the configured (or posted) address.
func (h *Handler) AgentToolSendTest(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	cfg := h.agentService.GetConfig(r.Context())
	if v := r.FormValue("email"); v != "" {
		cfg.Email = v
	}
	if cfg.Email == "" {
		http.Redirect(w, r, "/cm/tools/agent?error=no-email", http.StatusSeeOther)
		return
	}
	if _, err := h.agentService.SendDigest(r.Context(), cfg); err != nil {
		http.Redirect(w, r, "/cm/tools/agent?error=send", http.StatusSeeOther)
		return
	}
	if h.auditService != nil {
		uid, _ := primitive.ObjectIDFromHex(user.ID)
		h.auditService.LogAsync(models.AuditLog{
			UserID: uid, UserEmail: user.Email,
			Action: "agent.test_digest", Resource: "agent",
			Details: map[string]interface{}{"to": cfg.Email},
		})
	}
	http.Redirect(w, r, "/cm/tools/agent?sent=1", http.StatusSeeOther)
}
