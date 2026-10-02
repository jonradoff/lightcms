package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SetIndexNowService wires the IndexNow service into the handler.
func (h *Handler) SetIndexNowService(in *services.IndexNowService) { h.indexNowService = in }

// SetIndexNowService wires the IndexNow service into the API handler.
func (a *APIHandler) SetIndexNowService(in *services.IndexNowService) { a.indexNowService = in }

// ServeIndexNowKey serves /{key}.txt — the IndexNow ownership proof. Any
// other *.txt path falls through to normal page serving.
func (h *Handler) ServeIndexNowKey(w http.ResponseWriter, r *http.Request) {
	requested := mux.Vars(r)["indexnowkey"]
	if h.indexNowService != nil {
		if key := h.indexNowService.KeyForFile(r.Context()); key != "" && requested == key {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Write([]byte(key))
			return
		}
	}
	// ServePage resolves the page from the "slug" route var.
	h.ServePage(w, mux.SetURLVars(r, map[string]string{"slug": strings.TrimPrefix(r.URL.Path, "/")}))
}

// IndexNowToolPage renders the IndexNow status/config screen (admin only).
func (h *Handler) IndexNowToolPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	st, err := h.indexNowService.Status(r.Context())
	q := r.URL.Query()
	data := map[string]interface{}{
		"Title":   "IndexNow",
		"Status":  st,
		"Saved":   q.Get("saved") == "1",
		"Rotated": q.Get("rotated") == "1",
		"Notice":  q.Get("notice"),
		"Error":   q.Get("error"),
	}
	if err != nil {
		data["Error"] = err.Error()
	}
	h.renderAdmin(w, r, "indexnow_tool", data)
}

// IndexNowToolAction handles the IndexNow page's form POSTs.
func (h *Handler) IndexNowToolAction(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	r.ParseForm()
	action := r.FormValue("action")
	details := map[string]interface{}{}
	redirect := "/cm/tools/indexnow"

	switch action {
	case "save":
		enabled := r.FormValue("enabled") == "on"
		details["enabled"] = enabled
		if err := h.indexNowService.SetEnabled(ctx, enabled); err != nil {
			redirect += "?error=" + url.QueryEscape(err.Error())
		} else {
			redirect += "?saved=1"
		}
	case "rotate":
		if _, err := h.indexNowService.RegenerateKey(ctx); err != nil {
			redirect += "?error=" + url.QueryEscape(err.Error())
		} else {
			redirect += "?rotated=1"
		}
	case "submit_all":
		n, err := h.indexNowService.SubmitAll(ctx)
		details["urls"] = n
		if err != nil {
			redirect += "?error=" + url.QueryEscape(err.Error())
		} else {
			redirect += "?notice=" + url.QueryEscape(fmt.Sprintf("Submitted %d URLs to IndexNow.", n))
		}
	default:
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	if h.auditService != nil {
		uid, _ := primitive.ObjectIDFromHex(user.ID)
		h.auditService.LogAsync(models.AuditLog{
			UserID: uid, UserEmail: user.Email,
			Action: "indexnow." + action, Resource: "indexnow", Details: details,
		})
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// APIIndexNowStatus returns IndexNow status, key, and recent submissions.
func (a *APIHandler) APIIndexNowStatus(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsView) {
		return
	}
	st, err := a.indexNowService.Status(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, st)
}

// APIIndexNowUpdate enables or disables IndexNow: {"enabled": bool}.
func (a *APIHandler) APIIndexNowUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		a.jsonError(w, http.StatusBadRequest, `body must be {"enabled": true|false}`)
		return
	}
	if err := a.indexNowService.SetEnabled(r.Context(), *req.Enabled); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.auditLog(r, "indexnow.save", "indexnow", "", map[string]interface{}{"enabled": *req.Enabled})
	a.APIIndexNowStatus(w, r)
}

// APIIndexNowSubmit submits URLs now. {"all": true} submits every published
// page (rate-limited to once an hour); {"paths": [...]} queues specific paths.
func (a *APIHandler) APIIndexNowSubmit(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	var req struct {
		All   bool     `json:"all"`
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (!req.All && len(req.Paths) == 0) {
		a.jsonError(w, http.StatusBadRequest, `body must be {"all": true} or {"paths": ["/page", ...]}`)
		return
	}
	if req.All {
		n, err := a.indexNowService.SubmitAll(r.Context())
		if err != nil {
			a.jsonError(w, http.StatusConflict, err.Error())
			return
		}
		a.auditLog(r, "indexnow.submit_all", "indexnow", "", map[string]interface{}{"urls": n})
		a.jsonResponse(w, http.StatusOK, map[string]interface{}{"submitted": n})
		return
	}
	st, err := a.indexNowService.Status(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !st.Active {
		reason := st.Ineligible
		if reason == "" {
			reason = "disabled for this site"
		}
		a.jsonError(w, http.StatusConflict, "IndexNow is inactive: "+reason)
		return
	}
	var clean []string
	for _, p := range req.Paths {
		if p = strings.TrimSpace(p); p != "" {
			clean = append(clean, p)
		}
	}
	a.indexNowService.Notify(clean...)
	a.auditLog(r, "indexnow.submit_paths", "indexnow", "", map[string]interface{}{"paths": len(clean)})
	a.jsonResponse(w, http.StatusAccepted, map[string]interface{}{
		"queued": len(clean),
		"note":   "queued; submitted within ~15 seconds (URLs sent in the last 10 minutes wait out their cooldown)",
	})
}
