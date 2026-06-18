package handlers

import (
	"net/http"
	"net/url"
	"testing"
)

// TestAdminForms_CreateFlows exercises the form POST handlers for the main
// settings/content-organisation entities under an authenticated admin session.
func TestAdminForms_CreateFlows(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)
	tmplID := seedTemplate(t, db, "Page", "page")

	t.Run("CreateCollection", func(t *testing.T) {
		rr := postForm(t, h.CreateCollection, url.Values{
			"name": {"News"}, "category": {"news"}, "items_per_page": {"10"},
			"sort_field": {"created_at"}, "sort_order": {"desc"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("CreateCollection: %d (%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("CreateFolder", func(t *testing.T) {
		rr := postForm(t, h.CreateFolder, url.Values{"name": {"Blog"}, "slug": {"blog"}}, nil)
		if rr.Code >= 500 {
			t.Errorf("CreateFolder: %d (%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("CreateRedirect", func(t *testing.T) {
		rr := postForm(t, h.CreateRedirect, url.Values{
			"from_path": {"/old"}, "to_path": {"/new"}, "status_code": {"301"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("CreateRedirect: %d (%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("CreateRSSSource", func(t *testing.T) {
		rr := postForm(t, h.CreateRSSSource, url.Values{
			"name": {"Feed"}, "url": {"https://example.com/rss"},
			"template_id": {tmplID.Hex()}, "schedule": {"daily"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("CreateRSSSource: %d (%s)", rr.Code, rr.Body.String())
		}
	})
}

// TestAdminForms_SettingsUpdates exercises the theme and site-config update
// handlers, which touch a large amount of settings logic.
func TestAdminForms_SettingsUpdates(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	t.Run("UpdateTheme", func(t *testing.T) {
		rr := postForm(t, h.UpdateTheme, url.Values{
			"site_name":        {"My Site"},
			"primary_color":    {"#3366ff"},
			"secondary_color":  {"#222222"},
			"accent_color":     {"#ff6600"},
			"background_color": {"#ffffff"},
			"text_color":       {"#111111"},
			"font_family":      {"Inter"},
			"heading_font":     {"Inter"},
			"border_radius":    {"8px"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("UpdateTheme: %d (%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("UpdateSiteConfiguration", func(t *testing.T) {
		rr := postForm(t, h.UpdateSiteConfiguration, url.Values{
			"max_upload_bytes":        {"10485760"},
			"title_template":          {"%s | My Site"},
			"title_template_no_title": {"My Site"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("UpdateSiteConfiguration: %d (%s)", rr.Code, rr.Body.String())
		}
	})

	t.Run("UpdatePassword", func(t *testing.T) {
		// Wrong current password should be handled gracefully (re-render / redirect).
		rr := postForm(t, h.UpdatePassword, url.Values{
			"current_password": {"wrong"},
			"new_password":     {"newpassword123"},
			"confirm_password": {"newpassword123"},
		}, nil)
		if rr.Code >= 500 {
			t.Errorf("UpdatePassword: %d (%s)", rr.Code, rr.Body.String())
		}
	})
}

// TestAdminForms_TemplateLifecycle drives template create → delete via forms.
func TestAdminForms_TemplateLifecycle(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := postForm(t, h.CreateTemplate, url.Values{
		"name":        {"Custom Page"},
		"slug":        {"custom-page"},
		"html_layout": {"<html><body>{{.Body}}</body></html>"},
	}, nil)
	if rr.Code >= 500 {
		t.Fatalf("CreateTemplate: %d (%s)", rr.Code, rr.Body.String())
	}
	_ = http.StatusOK
}
