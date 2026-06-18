package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// getPageQ invokes a session-authed GET handler with a raw query string.
func getPageQ(t *testing.T, h http.HandlerFunc, rawQuery string, vars map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := sessionReq(http.MethodGet, "/cm/x?"+rawQuery, nil, vars)
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// TestAdminCorePages_Render exercises the main /cm dashboard, list, and "new"
// page handlers with an authenticated admin session — covering the real render
// paths (existing tests largely hit these unauthenticated).
func TestAdminCorePages_Render(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	pages := map[string]http.HandlerFunc{
		"AdminDashboard":          h.AdminDashboard,
		"ListTemplates":           h.ListTemplates,
		"NewTemplate":             h.NewTemplate,
		"ListContent":             h.ListContent,
		"NewContent":              h.NewContent,
		"ListCollections":         h.ListCollections,
		"NewCollection":           h.NewCollection,
		"ListFolders":             h.ListFolders,
		"NewFolder":               h.NewFolder,
		"ThemeSettings":           h.ThemeSettings,
		"ThemeVersions":           h.ThemeVersions,
		"SecuritySettings":        h.SecuritySettings,
		"ForceChangePasswordPage": h.ForceChangePasswordPage,
		"APIKeysPage":             h.APIKeysPage,
		"NewAPIKeyPage":           h.NewAPIKeyPage,
		"SiteConfiguration":       h.SiteConfiguration,
		"ListRedirects":           h.ListRedirects,
		"NewRedirect":             h.NewRedirect,
		"ListContactMessages":     h.ListContactMessages,
		"GetAllSlugs":             h.GetAllSlugs,
		"GetAllFoldersAPI":        h.GetAllFoldersAPI,
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

// TestAdminEditPages_Render exercises the ID-based edit/detail pages against
// real seeded entities so the happy render path is covered.
func TestAdminEditPages_Render(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	db := testDB(t)

	tmplID := seedTemplate(t, db, "Page", "page")
	contentID := seedContent(t, db, tmplID, "Doc", "doc", "/doc")

	cases := []struct {
		name    string
		handler http.HandlerFunc
		query   string
		vars    map[string]string
	}{
		{"EditTemplate", h.EditTemplate, "", map[string]string{"id": tmplID.Hex()}},
		{"EditContent", h.EditContent, "", map[string]string{"id": contentID.Hex()}},
		{"NewContentWithTemplate", h.NewContentWithTemplate, "template_id=" + tmplID.Hex(), nil},
		{"ListContentVersions", h.ListContentVersions, "", map[string]string{"id": contentID.Hex()}},
		{"GetTemplateFields", h.GetTemplateFields, "template_id=" + tmplID.Hex(), nil},
		{"ChangeTemplatePreview", h.ChangeTemplatePreview, "template_id=" + tmplID.Hex(), map[string]string{"id": contentID.Hex()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := getPageQ(t, tc.handler, tc.query, tc.vars)
			if rr.Code >= 500 {
				t.Errorf("%s: server error %d: %s", tc.name, rr.Code, rr.Body.String())
			}
		})
	}
}
