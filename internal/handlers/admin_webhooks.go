package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/jonradoff/lightcms/v6/internal/auth"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebhooksPage lists all registered webhooks.
func (h *Handler) WebhooksPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	webhooks, err := h.webhookService.List(r.Context(), 200)
	if err != nil {
		http.Error(w, "Failed to load webhooks", http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "webhooks_list", map[string]interface{}{
		"Webhooks": webhooks,
	})
}

// NewWebhookPage renders the webhook creation form.
func (h *Handler) NewWebhookPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
		"IsNew": true,
	})
}

// CreateWebhook handles the POST to create a new webhook.
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	url := strings.TrimSpace(r.FormValue("url"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "true"
	events := r.Form["events"]

	if name == "" || url == "" {
		h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
			"IsNew": true,
			"Error": "Name and URL are required",
		})
		return
	}

	// Auto-generate secret server-side; never accept it from the form
	secret := generateWebhookSecret()

	wh, err := h.webhookService.Create(r.Context(), name, url, secret, events, active)
	if err != nil {
		h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
			"IsNew": true,
			"Error": err.Error(),
		})
		return
	}

	h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
		"IsNew":         false,
		"Webhook":       wh,
		"CreatedSecret": secret,
		"Success":       "Webhook created. Save the secret below — it will not be shown again.",
	})
}

// EditWebhookPage renders the webhook edit form.
func (h *Handler) EditWebhookPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	wh, err := h.webhookService.Get(r.Context(), id)
	if err != nil || wh == nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
		"IsNew":   false,
		"Webhook": wh,
	})
}

// UpdateWebhook handles the POST to update an existing webhook.
func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	url := strings.TrimSpace(r.FormValue("url"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "true"
	events := r.Form["events"]

	// Fetch current webhook to preserve its secret (never update from form)
	existing, err := h.webhookService.Get(r.Context(), id)
	if err != nil || existing == nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}
	secret := existing.Secret

	if err := h.webhookService.Update(r.Context(), id, name, url, secret, events, active); err != nil {
		wh, _ := h.webhookService.Get(r.Context(), id)
		h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
			"IsNew":   false,
			"Webhook": wh,
			"Error":   err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/cm/webhooks", http.StatusSeeOther)
}

// DeleteWebhook handles the POST to delete a webhook.
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	if err := h.webhookService.Delete(r.Context(), id); err != nil {
		http.Error(w, "Failed to delete webhook", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/webhooks", http.StatusSeeOther)
}

// WebhookDeliveriesPage shows delivery history for a webhook.
func (h *Handler) WebhookDeliveriesPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	wh, err := h.webhookService.Get(r.Context(), id)
	if err != nil || wh == nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	deliveries, err := h.webhookService.ListDeliveries(r.Context(), id, 100)
	if err != nil {
		http.Error(w, "Failed to load deliveries", http.StatusInternalServerError)
		return
	}

	h.renderAdmin(w, r, "webhook_deliveries", map[string]interface{}{
		"Webhook":    wh,
		"Deliveries": deliveries,
	})
}

// WebhookDocsPage renders the webhook documentation page.
func (h *Handler) WebhookDocsPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}

	h.renderAdmin(w, r, "webhook_docs", nil)
}

// RegenerateWebhookSecret generates a new secret for a webhook and shows it once.
func (h *Handler) RegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	if h.webhookService == nil {
		http.Error(w, "Webhook service unavailable", http.StatusServiceUnavailable)
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	wh, err := h.webhookService.Get(r.Context(), id)
	if err != nil || wh == nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	newSecret := generateWebhookSecret()
	if err := h.webhookService.Update(r.Context(), id, wh.Name, wh.URL, newSecret, wh.Events, wh.Active); err != nil {
		http.Error(w, "Failed to regenerate secret", http.StatusInternalServerError)
		return
	}

	// Re-fetch to get fresh doc
	wh, _ = h.webhookService.Get(r.Context(), id)
	h.renderAdmin(w, r, "webhook_form", map[string]interface{}{
		"IsNew":             false,
		"Webhook":           wh,
		"RegeneratedSecret": newSecret,
		"Success":           "Secret regenerated. Save the new secret below — it will not be shown again.",
	})
}

// generateWebhookSecret generates a cryptographically random 32-byte hex secret.
func generateWebhookSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
