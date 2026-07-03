package handlers

import (
	"net/http"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SetLockServiceAPI attaches a LockService to the APIHandler.
func (a *APIHandler) SetLockServiceAPI(ls *services.LockService) {
	a.lockService = ls
}

// GET /api/v1/content/{id}/lock
func (a *APIHandler) APIGetContentLock(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentView) {
		return
	}
	if a.lockService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "lock service unavailable")
		return
	}

	contentID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content id")
		return
	}

	lock, err := a.lockService.GetLock(r.Context(), contentID)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if lock == nil {
		a.jsonResponse(w, http.StatusOK, map[string]interface{}{"locked": false})
		return
	}
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"locked":      true,
		"user_email":  lock.UserEmail,
		"acquired_at": lock.AcquiredAt.Format("2006-01-02T15:04:05Z"),
		"expires_at":  lock.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// POST /api/v1/content/{id}/lock
func (a *APIHandler) APIAcquireContentLock(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}
	if a.lockService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "lock service unavailable")
		return
	}

	contentID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content id")
		return
	}

	user := a.getAPIUser(r)
	if user == nil {
		a.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	userID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	existing, err := a.lockService.AcquireLock(r.Context(), contentID, userID, user.Email)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if existing != nil {
		// Another user holds the lock
		a.jsonResponse(w, http.StatusConflict, map[string]interface{}{
			"locked":      true,
			"user_email":  existing.UserEmail,
			"acquired_at": existing.AcquiredAt.Format("2006-01-02T15:04:05Z"),
			"expires_at":  existing.ExpiresAt.Format("2006-01-02T15:04:05Z"),
			"message":     "content is locked by another user",
		})
		return
	}
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"locked":  true,
		"message": "lock acquired",
	})
}

// DELETE /api/v1/content/{id}/lock
func (a *APIHandler) APIReleaseContentLock(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}
	if a.lockService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "lock service unavailable")
		return
	}

	contentID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content id")
		return
	}

	user := a.getAPIUser(r)
	if user == nil {
		a.jsonError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	userID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := a.lockService.ReleaseLock(r.Context(), contentID, userID); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/content/{id}/lock/force — admin only
func (a *APIHandler) APIForceUnlockContent(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermUserManage) {
		return
	}
	if a.lockService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "lock service unavailable")
		return
	}

	contentID, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid content id")
		return
	}

	if err := a.lockService.ForceUnlock(r.Context(), contentID); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
