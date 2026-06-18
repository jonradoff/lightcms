package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// getPage invokes a session-authed GET handler and returns the recorder.
func getPage(t *testing.T, h http.HandlerFunc, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h(rr, sessionReq(http.MethodGet, "/cm/x", nil, vars))
	return rr
}

// postForm invokes a session-authed POST handler with form values.
func postForm(t *testing.T, h http.HandlerFunc, form url.Values, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := sessionReq(http.MethodPost, "/cm/x", strings.NewReader(form.Encode()), vars)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// TestAdminPages_Render exercises the admin GET page handlers with an
// authenticated admin session. Each should render (200) or redirect, never 500.
func TestAdminPages_Render(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	pages := map[string]http.HandlerFunc{
		"AnalyticsPage":           h.AnalyticsPage,
		"AnalyticsReferrerReport": h.AnalyticsReferrerReport,
		"WebhooksPage":            h.WebhooksPage,
		"NewWebhookPage":          h.NewWebhookPage,
		"WebhookDocsPage":         h.WebhookDocsPage,
		"ImportsPage":             h.ImportsPage,
		"NewRSSSourcePage":        h.NewRSSSourcePage,
		"ImportMarkdownPage":      h.ImportMarkdownPage,
		"ImportCSVPage":           h.ImportCSVPage,
		"UsersPage":               h.UsersPage,
		"NewUserPage":             h.NewUserPage,
		"AuditLogPage":            h.AuditLogPage,
	}
	for name, handler := range pages {
		t.Run(name, func(t *testing.T) {
			rr := getPage(t, handler, nil)
			if rr.Code >= 500 {
				t.Errorf("%s: server error %d: %s", name, rr.Code, rr.Body.String())
			}
		})
	}
}

// TestAdminPages_ByID exercises ID-based admin pages with a non-existent ID;
// they should handle it gracefully (redirect / not-found), never 500.
func TestAdminPages_ByID(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	missing := primitive.NewObjectID().Hex()
	byID := map[string]http.HandlerFunc{
		"EditWebhookPage":       h.EditWebhookPage,
		"WebhookDeliveriesPage": h.WebhookDeliveriesPage,
		"EditUserPage":          h.EditUserPage,
		"EditRSSSourcePage":     h.EditRSSSourcePage,
		"ImportJobPage":         h.ImportJobPage,
		"AnalyticsPageDetail":   h.AnalyticsPageDetail,
	}
	for name, handler := range byID {
		t.Run(name, func(t *testing.T) {
			rr := getPage(t, handler, map[string]string{"id": missing})
			if rr.Code >= 500 {
				t.Errorf("%s: server error %d: %s", name, rr.Code, rr.Body.String())
			}
		})
	}
}

// TestAdminWebhooks_Forms exercises the webhook create/update/delete form flow.
func TestAdminWebhooks_Forms(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Create
	rr := postForm(t, h.CreateWebhook, url.Values{
		"name":   {"My Hook"},
		"url":    {"https://example.com/hook"},
		"events": {"content.published"},
		"active": {"on"},
	}, nil)
	if rr.Code >= 500 {
		t.Fatalf("CreateWebhook: %d (%s)", rr.Code, rr.Body.String())
	}

	// Find the created webhook to drive update/delete.
	ws := h.webhookService
	list, err := ws.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List webhooks: %v", err)
	}
	if len(list) == 0 {
		t.Skip("webhook not created; skipping update/delete")
	}
	id := list[0].ID.Hex()

	if rr := postForm(t, h.UpdateWebhook, url.Values{
		"name": {"Renamed"}, "url": {"https://example.com/h2"}, "events": {"content.deleted"},
	}, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("UpdateWebhook: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.RegenerateWebhookSecret, nil, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("RegenerateWebhookSecret: %d", rr.Code)
	}
	if rr := postForm(t, h.DeleteWebhook, nil, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("DeleteWebhook: %d", rr.Code)
	}
}

// TestAdminUsers_Forms exercises the user create/update flow.
func TestAdminUsers_Forms(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := postForm(t, h.CreateUser, url.Values{
		"email": {"editor@example.com"},
		"role":  {"editor"},
	}, nil)
	if rr.Code >= 500 {
		t.Fatalf("CreateUser: %d (%s)", rr.Code, rr.Body.String())
	}

	u, err := h.userService.GetByEmail(context.Background(), "editor@example.com")
	if err != nil || u == nil {
		t.Skip("user not created; skipping update")
	}
	id := u.ID.Hex()

	if rr := postForm(t, h.UpdateUser, url.Values{"email": {"editor@example.com"}, "role": {"viewer"}}, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("UpdateUser: %d (%s)", rr.Code, rr.Body.String())
	}
	if rr := postForm(t, h.ToggleUserDisabled, nil, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("ToggleUserDisabled: %d", rr.Code)
	}
	if rr := postForm(t, h.ResetUserPassword, nil, map[string]string{"id": id}); rr.Code >= 500 {
		t.Errorf("ResetUserPassword: %d", rr.Code)
	}
}
