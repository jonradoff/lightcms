package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/models"
	"lightcms/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const forkPreviewCookie = "lc_fork_preview"

// SetForkService attaches a ForkService to the Handler.
func (h *Handler) SetForkService(fs *services.ForkService) {
	h.forkService = fs
}

// ---------------------------------------------------------------------------
// Fork list
// ---------------------------------------------------------------------------

func (h *Handler) ListForks(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	forks, err := h.forkService.List(ctx)
	if err != nil {
		forks = []models.ContentFork{}
	}
	// Attach page counts
	type forkWithCount struct {
		models.ContentFork
		PageCount int64
	}
	var enriched []forkWithCount
	for _, f := range forks {
		count, _ := h.forkService.GetPageCount(ctx, f.ID)
		enriched = append(enriched, forkWithCount{f, count})
	}
	// If coming from a content page via "Fork to workspace", carry the content ID forward
	forkPageID := r.URL.Query().Get("fork_page")
	h.renderAdmin(w, r, "forks_list", map[string]interface{}{
		"Forks":      enriched,
		"CanMerge":   auth.HasPermission(user.Role, auth.PermForkMerge),
		"ForkPageID": forkPageID,
	})
}

// ---------------------------------------------------------------------------
// Create fork
// ---------------------------------------------------------------------------

func (h *Handler) NewFork(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	h.renderAdmin(w, r, "fork_form", map[string]interface{}{})
}

func (h *Handler) CreateFork(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		h.renderAdmin(w, r, "fork_form", map[string]interface{}{
			"Error": "Fork name is required",
		})
		return
	}
	ctx := r.Context()
	userID, _ := primitive.ObjectIDFromHex(user.ID)
	fork, err := h.forkService.Create(ctx, name, description, userID, user.Email)
	if err != nil {
		h.renderAdmin(w, r, "fork_form", map[string]interface{}{
			"Error": fmt.Sprintf("Failed to create fork: %v", err),
		})
		return
	}
	http.Redirect(w, r, "/cm/forks/"+fork.ID.Hex(), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// View / manage fork
// ---------------------------------------------------------------------------

func (h *Handler) ViewFork(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	fork, err := h.forkService.GetByID(ctx, forkID)
	if err != nil {
		http.Error(w, "Fork not found", http.StatusNotFound)
		return
	}
	pages, _ := h.forkService.ListPages(ctx, forkID)
	h.renderAdmin(w, r, "fork_detail", map[string]interface{}{
		"Fork":     fork,
		"Pages":    pages,
		"CanMerge": auth.HasPermission(user.Role, auth.PermForkMerge),
	})
}

// ---------------------------------------------------------------------------
// Fork a live page into the workspace
// ---------------------------------------------------------------------------

func (h *Handler) ForkPageHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	contentIDStr := r.FormValue("content_id")
	contentID, err := primitive.ObjectIDFromHex(contentIDStr)
	if err != nil {
		http.Error(w, "Invalid content ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	forkPage, err := h.forkService.ForkPage(ctx, forkID, contentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fork page: %v", err), http.StatusInternalServerError)
		return
	}
	// Redirect to edit the fork copy
	http.Redirect(w, r, "/cm/content/"+forkPage.ID.Hex()+"?fork="+forkID.Hex(), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Remove a page from the fork
// ---------------------------------------------------------------------------

func (h *Handler) RemoveForkPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	pageID, err := primitive.ObjectIDFromHex(vars["pageID"])
	if err != nil {
		http.Error(w, "Invalid page ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := h.forkService.RemovePage(ctx, forkID, pageID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to remove page: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/cm/forks/"+forkID.Hex(), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Preview activation / deactivation
// ---------------------------------------------------------------------------

func (h *Handler) StartForkPreview(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkCreate) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	fork, err := h.forkService.GetByID(ctx, forkID)
	if err != nil {
		http.Error(w, "Fork not found", http.StatusNotFound)
		return
	}
	if fork.Status != "active" {
		http.Error(w, "Fork is not active", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     forkPreviewCookie,
		Value:    fork.PreviewToken,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	// Redirect to home so they can browse the site in preview mode
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ExitForkPreview(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     forkPreviewCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	// Return to where they came from, or forks list
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/cm/forks"
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Merge fork into live (admin only)
// ---------------------------------------------------------------------------

func (h *Handler) MergeFork(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkMerge) {
		http.Error(w, "Forbidden — only admins can merge forks", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	userID, _ := primitive.ObjectIDFromHex(user.ID)
	result, err := h.forkService.Merge(ctx, forkID, userID, user.Email)
	if err != nil {
		h.renderAdmin(w, r, "fork_merge_result", map[string]interface{}{
			"Error":  err.Error(),
			"ForkID": forkID.Hex(),
		})
		return
	}
	if h.auditService != nil {
		h.auditService.LogAsync(models.AuditLog{
			UserID:    userID,
			UserEmail: user.Email,
			Action:    "fork.merge",
			Resource:  "fork",
			ResourceID: forkID.Hex(),
			Details: map[string]interface{}{
				"updated":   result.Updated,
				"created":   result.Created,
				"conflicts": len(result.Conflicts),
			},
		})
	}
	h.renderAdmin(w, r, "fork_merge_result", map[string]interface{}{
		"Result": result,
		"ForkID": forkID.Hex(),
	})
}

// ---------------------------------------------------------------------------
// Archive / delete fork
// ---------------------------------------------------------------------------

func (h *Handler) ArchiveFork(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkMerge) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := h.forkService.Archive(ctx, forkID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to archive fork: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/cm/forks", http.StatusSeeOther)
}

func (h *Handler) DeleteForkHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}
	if !auth.HasPermission(user.Role, auth.PermForkMerge) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	forkID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid fork ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := h.forkService.Delete(ctx, forkID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete fork: %v", err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/cm/forks", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// getActiveForkPreview checks the request for a valid fork preview cookie
// and returns the fork if one is active. Returns nil if no preview is active.
// ---------------------------------------------------------------------------

func (h *Handler) getActiveForkPreview(r *http.Request) *models.ContentFork {
	if h.forkService == nil {
		return nil
	}
	cookie, err := r.Cookie(forkPreviewCookie)
	if err != nil || cookie.Value == "" {
		return nil
	}
	fork, err := h.forkService.GetByPreviewToken(r.Context(), cookie.Value)
	if err != nil {
		return nil
	}
	return fork
}
