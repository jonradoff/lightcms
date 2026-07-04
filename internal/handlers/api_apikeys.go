package handlers

import (
	"net/http"

	"github.com/jonradoff/lightcms/v7/internal/auth"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Key management endpoints

func (a *APIHandler) APIListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAPIKeyManage) {
		return
	}

	user := a.getAPIUser(r)
	// Admins see all keys, others see only their own
	if user != nil && auth.HasPermission(user.Role, auth.PermAPIKeyManageAll) {
		keys, err := a.apiKeyService.ListAPIKeys(r.Context())
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.jsonResponse(w, http.StatusOK, keys)
	} else if user != nil {
		userID, err := primitive.ObjectIDFromHex(user.ID)
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, "invalid user ID")
			return
		}
		keys, err := a.apiKeyService.ListAPIKeysForUser(r.Context(), userID)
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.jsonResponse(w, http.StatusOK, keys)
	} else {
		// No user context (system key) — return all
		keys, err := a.apiKeyService.ListAPIKeys(r.Context())
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.jsonResponse(w, http.StatusOK, keys)
	}
}

func (a *APIHandler) APICreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAPIKeyManage) {
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Scopes      []string `json:"scopes"`
		SandboxOnly bool     `json:"sandbox_only"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	for _, s := range req.Scopes {
		if !auth.IsKnownPermission(s) {
			a.jsonError(w, http.StatusBadRequest, "unknown scope: "+s)
			return
		}
	}

	// Create key owned by the current user
	var userID *primitive.ObjectID
	if user := a.getAPIUser(r); user != nil {
		uid, err := primitive.ObjectIDFromHex(user.ID)
		if err == nil {
			userID = &uid
		}
	}

	rawKey, apiKey, err := a.apiKeyService.CreateScopedAPIKey(r.Context(), req.Name, req.Description, userID, req.Scopes, req.SandboxOnly)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.auditLog(r, "apikey.create", "apikey", apiKey.ID.Hex(), map[string]interface{}{"name": req.Name, "scopes": req.Scopes, "sandbox_only": req.SandboxOnly})
	a.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"key":         rawKey,
		"id":          apiKey.ID.Hex(),
		"name":        apiKey.Name,
		"description": apiKey.Description,
		"prefix":      apiKey.Prefix,
		"created_at":  apiKey.CreatedAt,
	})
}

func (a *APIHandler) APIDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAPIKeyManage) {
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid API key ID")
		return
	}

	user := a.getAPIUser(r)
	isAdmin := user != nil && auth.HasPermission(user.Role, auth.PermAPIKeyManageAll)

	if isAdmin {
		// Admins can delete any key
		if err := a.apiKeyService.DeleteAPIKey(r.Context(), id); err != nil {
			a.jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if user != nil {
		// Non-admins can only delete their own keys
		ownerID, err := primitive.ObjectIDFromHex(user.ID)
		if err != nil {
			a.jsonError(w, http.StatusInternalServerError, "invalid user ID")
			return
		}
		if err := a.apiKeyService.DeleteAPIKeyForUser(r.Context(), id, ownerID); err != nil {
			// DeleteOne returns an error or 0 matched — surface as 403
			a.jsonError(w, http.StatusForbidden, "API key not found or not owned by you")
			return
		}
	} else {
		a.jsonError(w, http.StatusForbidden, "cannot determine key ownership")
		return
	}

	a.auditLog(r, "apikey.delete", "apikey", id.Hex(), nil)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}
