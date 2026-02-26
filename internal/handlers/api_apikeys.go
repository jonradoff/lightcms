package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// API Key management endpoints

func (a *APIHandler) APIListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.apiKeyService.ListAPIKeys(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, keys)
}

func (a *APIHandler) APICreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		a.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	rawKey, apiKey, err := a.apiKeyService.CreateAPIKey(r.Context(), req.Name, req.Description)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

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
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid API key ID")
		return
	}

	if err := a.apiKeyService.DeleteAPIKey(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusOK, map[string]interface{}{"success": true})
}
