package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"

	"lightcms/internal/auth"
	"lightcms/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SetWebhookServiceAPI attaches a WebhookService to the APIHandler.
func (a *APIHandler) SetWebhookServiceAPI(ws *services.WebhookService) {
	a.webhookService = ws
}

// generateAPIWebhookSecret generates a cryptographically random 32-byte hex secret.
func generateAPIWebhookSecret() string {
	b := make([]byte, 32)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

// GET /api/v1/webhooks
func (a *APIHandler) APIListWebhooks(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}
	hooks, err := a.webhookService.List(r.Context(), 200)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Mask secrets in list response
	type webhookResponse struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		URL       string   `json:"url"`
		Events    []string `json:"events"`
		Active    bool     `json:"active"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
	}
	result := make([]webhookResponse, len(hooks))
	for i, wh := range hooks {
		result[i] = webhookResponse{
			ID:        wh.ID.Hex(),
			Name:      wh.Name,
			URL:       wh.URL,
			Events:    wh.Events,
			Active:    wh.Active,
			CreatedAt: wh.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: wh.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	a.jsonResponse(w, http.StatusOK, result)
}

// POST /api/v1/webhooks
func (a *APIHandler) APICreateWebhook(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}

	var req struct {
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Active bool     `json:"active"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.URL == "" {
		a.jsonError(w, http.StatusBadRequest, "name and url are required")
		return
	}

	secret := generateAPIWebhookSecret()
	wh, err := a.webhookService.Create(r.Context(), req.Name, req.URL, secret, req.Events, req.Active)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return secret ONCE in create response
	a.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"id":         wh.ID.Hex(),
		"name":       wh.Name,
		"url":        wh.URL,
		"events":     wh.Events,
		"active":     wh.Active,
		"secret":     secret,
		"created_at": wh.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// PUT /api/v1/webhooks/{id}
func (a *APIHandler) APIUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	existing, err := a.webhookService.Get(r.Context(), id)
	if err != nil || existing == nil {
		a.jsonError(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req struct {
		Name   *string  `json:"name"`
		URL    *string  `json:"url"`
		Events []string `json:"events"`
		Active *bool    `json:"active"`
	}
	if err := a.decodeJSON(r, &req); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	url := existing.URL
	if req.URL != nil {
		url = *req.URL
	}
	events := existing.Events
	if req.Events != nil {
		events = req.Events
	}
	active := existing.Active
	if req.Active != nil {
		active = *req.Active
	}

	if err := a.webhookService.Update(r.Context(), id, name, url, existing.Secret, events, active); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := a.webhookService.Get(r.Context(), id)
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id":         updated.ID.Hex(),
		"name":       updated.Name,
		"url":        updated.URL,
		"events":     updated.Events,
		"active":     updated.Active,
		"updated_at": updated.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// DELETE /api/v1/webhooks/{id}
func (a *APIHandler) APIDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	if err := a.webhookService.Delete(r.Context(), id); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/webhooks/{id}/regenerate-secret
func (a *APIHandler) APIRegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	existing, err := a.webhookService.Get(r.Context(), id)
	if err != nil || existing == nil {
		a.jsonError(w, http.StatusNotFound, "webhook not found")
		return
	}

	newSecret := generateAPIWebhookSecret()
	if err := a.webhookService.RegenerateSecret(r.Context(), id, newSecret); err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return new secret ONCE
	a.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"secret": newSecret,
	})
}

// GET /api/v1/webhooks/{id}/deliveries
func (a *APIHandler) APIListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	if a.webhookService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "webhook service unavailable")
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	deliveries, err := a.webhookService.ListDeliveries(r.Context(), id, limit)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deliveries == nil {
		deliveries = []services.WebhookDeliveryDoc{}
	}
	a.jsonResponse(w, http.StatusOK, deliveries)
}
