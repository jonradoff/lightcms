package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/middleware"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"
)

// Database-backed tests for the 7.4.2 hardening: the /cm access checks
// through the real router and handlers (a refusal is styled and changes
// nothing), the rules the handlers add on top of the route table, and the
// smaller admin fixes.

// adminRouter is the /cm router the server builds, minus the CSRF middleware.
func adminRouter(h *Handler) *mux.Router {
	r := mux.NewRouter()
	h.RegisterAdminRoutes(r.PathPrefix("/cm").Subrouter())
	return r
}

// routed sends a request through the router as the given role.
func routed(r http.Handler, role, method, target string, form url.Values) *httptest.ResponseRecorder {
	req := roleReq(role, method, target, strings.NewReader(form.Encode()), nil)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// assertStyledRefusal checks the 403 an HTML form post gets: the admin page
// with a visible message, not a bare "Forbidden" and never a native dialog.
func assertStyledRefusal(t *testing.T, what string, rr *httptest.ResponseRecorder) {
	t.Helper()
	body := rr.Body.String()
	if rr.Code != http.StatusForbidden {
		t.Errorf("%s: status %d, want 403", what, rr.Code)
		return
	}
	for _, want := range []string{`id="forbidden-message"`, `class="error-message"`, "does not have permission", "Nothing was changed", `class="sidebar"`} {
		if !strings.Contains(body, want) {
			t.Errorf("%s: refusal page lacks %q", what, want)
		}
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "text/html") {
		t.Errorf("%s: refusal is %q, want an HTML page", what, rr.Header().Get("Content-Type"))
	}
}

// injectRole returns ctx carrying an API user with the given role.
func injectRole(ctx context.Context, role string) context.Context {
	return middleware.InjectAPIUser(ctx, &auth.SessionUser{ID: "000000000000000000000001", Email: role + "@localhost", Role: role, ViaAPIKey: true})
}

func count(t *testing.T, db *database.DB, coll string, filter bson.M) int64 {
	t.Helper()
	n, err := db.Count(context.Background(), coll, filter)
	if err != nil {
		t.Fatalf("count %s: %v", coll, err)
	}
	return n
}

// The routes that only checked for a session, through the real router and
// handlers: a role without the permission is refused with the styled page
// (or JSON for a fetch endpoint) and nothing is written; an admin is not
// affected.
func TestAdminRoutes_RefusalIsStyledAndChangesNothing(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	r := adminRouter(h)
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	page := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Kept", Slug: "kept", FullPath: "/kept", Published: true})
	theme, _ := h.db.GetThemeSettings(ctx)
	siteName := theme.SiteName

	type attempt struct {
		role, method, target string
		form                 url.Values
		unchanged            func() bool
	}
	none := func(coll string, filter bson.M) func() bool {
		return func() bool { return count(t, h.db, coll, filter) == 0 }
	}
	attempts := []attempt{
		{"viewer", "POST", "/cm/templates/new", url.Values{"name": {"Viewer tpl"}, "slug": {"viewer-tpl"}, "html_layout": {"<p>x</p>"}}, none("templates", bson.M{"name": "Viewer tpl"})},
		{"editor", "POST", "/cm/templates/new", url.Values{"name": {"Editor tpl"}, "slug": {"editor-tpl"}, "html_layout": {"<p>x</p>"}}, none("templates", bson.M{"name": "Editor tpl"})},
		{"editor", "POST", "/cm/templates/" + tmplID.Hex() + "/delete", nil, func() bool { return count(t, h.db, "templates", bson.M{"_id": tmplID}) == 1 }},
		{"viewer", "POST", "/cm/content/create", url.Values{"template_id": {tmplID.Hex()}, "title": {"Viewer page"}, "slug": {"viewer-page"}}, none("content", bson.M{"slug": "viewer-page"})},
		{"viewer", "POST", "/cm/content/" + page.Hex(), url.Values{"title": {"Hacked"}, "slug": {"kept"}}, func() bool { return loadPage(t, h.db, page).Title == "Kept" }},
		{"contributor", "POST", "/cm/content/" + page.Hex() + "/delete", nil, func() bool { return !loadPage(t, h.db, page).Deleted }},
		{"viewer", "POST", "/cm/content/" + page.Hex() + "/regenerate", nil, nil},
		{"contributor", "POST", "/cm/content/" + page.Hex() + "/versions/1/revert", nil, func() bool { return loadPage(t, h.db, page).Title == "Kept" }},
		{"editor", "POST", "/cm/collections/new", url.Values{"name": {"Editor coll"}, "slug": {"editor-coll"}, "category": {"x"}}, none("collections", bson.M{"name": "Editor coll"})},
		{"viewer", "POST", "/cm/theme", url.Values{"site_name": {"Hacked site"}}, func() bool { th, _ := h.db.GetThemeSettings(ctx); return th.SiteName == siteName }},
		{"editor", "POST", "/cm/theme/versions/1/revert", nil, nil},
		{"editor", "POST", "/cm/config", url.Values{"title_template": {"hacked"}}, func() bool { c, _ := h.db.GetSiteConfig(ctx); return c.TitleTemplate != "hacked" }},
		{"contributor", "POST", "/cm/folders/new", url.Values{"name": {"Contrib folder"}, "slug": {"contrib-folder"}}, none("folders", bson.M{"slug": "contrib-folder"})},
		{"editor", "POST", "/cm/redirects/new", url.Values{"from_path": {"/editor-from"}, "to_path": {"/kept"}, "status_code": {"301"}}, none("redirects", bson.M{"from_path": "/editor-from"})},
		{"viewer", "POST", "/cm/messages/mark-all-read", nil, nil},
		{"viewer", "POST", "/cm/assets/upload", nil, nil},
		{"contributor", "POST", "/cm/assets/000000000000000000000009/delete", nil, nil},
		{"viewer", "POST", "/cm/api-keys/new", url.Values{"name": {"Viewer key"}}, none("api_keys", bson.M{"name": "Viewer key"})},
		{"editor", "POST", "/cm/snippets/new", url.Values{"name": {"editor-snip"}, "html": {"<i>x</i>"}}, none("snippets", bson.M{"name": "editor-snip"})},
		{"editor", "POST", "/cm/content/" + page.Hex() + "/lock/force", nil, nil},
		// Pages that held something a low role should not see
		{"viewer", "GET", "/cm/api-keys", nil, nil},
		{"viewer", "GET", "/cm/approvals", nil, nil},
		{"contributor", "GET", "/cm/users", nil, nil},
		{"editor", "GET", "/cm/audit", nil, nil},
	}
	for _, a := range attempts {
		what := a.role + " " + a.method + " " + a.target
		assertStyledRefusal(t, what, routed(r, a.role, a.method, a.target, a.form))
		if a.unchanged != nil && !a.unchanged() {
			t.Errorf("%s: refused, but it changed something", what)
		}
	}

	// Fetch endpoints answer in JSON
	for _, a := range []attempt{
		{"viewer", "POST", "/cm/upload", nil, nil},
		{"viewer", "POST", "/cm/content/" + page.Hex() + "/lock/refresh", nil, nil},
		{"editor", "POST", "/cm/tools/search/reindex", nil, nil},
		{"contributor", "POST", "/cm/copilot/chat", nil, nil},
	} {
		rr := routed(r, a.role, a.method, a.target, a.form)
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Header().Get("Content-Type"), "application/json") || !strings.Contains(rr.Body.String(), `"error"`) {
			t.Errorf("%s %s %s: %d %q %s, want a JSON 403", a.role, a.method, a.target, rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
		}
	}

	// The same requests from an admin go through
	if rr := routed(r, "admin", "POST", "/cm/redirects/new", url.Values{"from_path": {"/admin-from"}, "to_path": {"/kept"}, "status_code": {"301"}}); rr.Code != http.StatusSeeOther {
		t.Errorf("admin create redirect: %d %s", rr.Code, rr.Body.String())
	}
	if count(t, h.db, "redirects", bson.M{"from_path": "/admin-from"}) != 1 {
		t.Error("admin's redirect was not created")
	}
	if rr := routed(r, "admin", "POST", "/cm/snippets/new", url.Values{"name": {"admin-snip"}, "html": {"<i>x</i>"}}); rr.Code >= 400 {
		t.Errorf("admin create snippet: %d %s", rr.Code, rr.Body.String())
	}
	if count(t, h.db, "snippets", bson.M{"name": "admin-snip"}) != 1 {
		t.Error("admin's snippet was not created")
	}
	for _, target := range []string{"/cm/api-keys", "/cm/approvals", "/cm/users", "/cm/audit", "/cm/config", "/cm/templates/new"} {
		if rr := routed(r, "admin", "GET", target, nil); rr.Code != http.StatusOK {
			t.Errorf("admin GET %s: %d", target, rr.Code)
		}
	}
	// An editor keeps the content work the role is for
	if rr := routed(r, "editor", "POST", "/cm/content/"+page.Hex()+"/delete", nil); rr.Code != http.StatusSeeOther {
		t.Errorf("editor delete page: %d %s", rr.Code, rr.Body.String())
	}
	if !loadPage(t, h.db, page).Deleted {
		t.Error("editor's delete did not happen")
	}
}

// A contributor writes and revises submissions in the editor: saving a
// draft works (ticking Publish queues it for approval instead), but a
// published page, a fork copy or someone's deleted page is out of reach —
// saving a published page would have taken it off the site.
func TestAdminUpdateContent_ContributorSavesDraftsOnly(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	r := adminRouter(h)
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	forkID := createTestFork(t, h.db, "contrib")
	draft := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Draft", Slug: "contrib-draft", FullPath: "/contrib-draft"})
	live := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Live", Slug: "contrib-live", FullPath: "/contrib-live", Published: true})
	copyPage := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Copy", Slug: "contrib-live", FullPath: "/contrib-live", ForkID: &forkID})
	const liveHTML = "<p>LIVE</p>"
	writeStatic(t, "/contrib-live", liveHTML)

	// Create: allowed, and Publish becomes a pending approval
	rr := routed(r, "contributor", "POST", "/cm/content/create", url.Values{"template_id": {tmplID.Hex()}, "title": {"Submitted"}, "slug": {"contrib-new"}, "published": {"on"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("contributor create: %d %s", rr.Code, rr.Body.String())
	}
	var created models.Content
	if err := h.db.FindOne(ctx, "content", bson.M{"slug": "contrib-new"}, &created); err != nil {
		t.Fatalf("contributor's page was not created: %v", err)
	}
	if created.Published || !created.PendingApproval {
		t.Errorf("contributor's page: published=%v pending_approval=%v, want a draft pending approval", created.Published, created.PendingApproval)
	}

	// Save a draft: allowed
	rr = routed(r, "contributor", "POST", "/cm/content/"+draft.Hex(), url.Values{"title": {"Draft, revised"}, "slug": {"contrib-draft"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("contributor save draft: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, h.db, draft); c.Title != "Draft, revised" || c.Published {
		t.Errorf("draft after contributor save: title=%q published=%v", c.Title, c.Published)
	}
	// Save a draft with Publish ticked: stays a draft, pending approval
	rr = routed(r, "contributor", "POST", "/cm/content/"+draft.Hex(), url.Values{"title": {"Draft, submitted"}, "slug": {"contrib-draft"}, "published": {"on"}})
	if c := loadPage(t, h.db, draft); rr.Code != http.StatusSeeOther || c.Published || !c.PendingApproval || c.Title != "Draft, submitted" {
		t.Errorf("contributor submit: %d published=%v pending=%v title=%q", rr.Code, c.Published, c.PendingApproval, c.Title)
	}

	// A published page, with Publish ticked or not: refused, nothing changes
	for _, form := range []url.Values{
		{"title": {"Taken down"}, "slug": {"contrib-live"}},
		{"title": {"Taken down"}, "slug": {"contrib-live"}, "published": {"on"}},
	} {
		rr = routed(r, "contributor", "POST", "/cm/content/"+live.Hex(), form)
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `id="forbidden-message"`) || !strings.Contains(rr.Body.String(), "drafts only") {
			t.Errorf("contributor save published page: %d, want the styled 403", rr.Code)
		}
		if c := loadPage(t, h.db, live); !c.Published || c.Title != "Live" || c.PendingApproval {
			t.Errorf("published page changed by a contributor: %+v", c)
		}
		assertStatic(t, "contributor save published page", "/contrib-live", liveHTML)
	}
	// A fork copy: refused
	rr = routed(r, "contributor", "POST", "/cm/content/"+copyPage.Hex(), url.Values{"title": {"Fork edit"}, "slug": {"contrib-live"}})
	if rr.Code != http.StatusForbidden || loadPage(t, h.db, copyPage).Title != "Copy" {
		t.Errorf("contributor save fork copy: %d title=%q", rr.Code, loadPage(t, h.db, copyPage).Title)
	}
	// The handler's own check holds without the router in front of it
	if rr := roleCall(h.UpdateContent, "viewer", "POST", "/cm/content/"+draft.Hex(), "title=Viewer", "application/x-www-form-urlencoded", map[string]string{"id": draft.Hex()}); rr.Code != http.StatusForbidden || loadPage(t, h.db, draft).Title != "Draft, submitted" {
		t.Errorf("viewer calling UpdateContent directly: %d title=%q", rr.Code, loadPage(t, h.db, draft).Title)
	}

	// An editor still edits and publishes
	rr = routed(r, "editor", "POST", "/cm/content/"+live.Hex(), url.Values{"title": {"Live, edited"}, "slug": {"contrib-live"}, "published": {"on"}})
	if c := loadPage(t, h.db, live); rr.Code != http.StatusSeeOther || !c.Published || c.Title != "Live, edited" {
		t.Errorf("editor save published page: %d published=%v title=%q", rr.Code, c.Published, c.Title)
	}
}

// Revoking an API key: your own, or any with apikey.manage_all. The handler
// used to delete whatever ID any signed-in user posted.
func TestAdminDeleteAPIKey_OwnKeysOnly(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	r := adminRouter(h)
	ctx := context.Background()

	self, _ := primitive.ObjectIDFromHex("000000000000000000000001") // the test session's user
	other := primitive.NewObjectID()
	_, mine, err := h.apiKeyService.CreateAPIKeyForUser(ctx, "mine", "", &self)
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	_, theirs, _ := h.apiKeyService.CreateAPIKeyForUser(ctx, "theirs", "", &other)
	_, theirs2, _ := h.apiKeyService.CreateAPIKeyForUser(ctx, "theirs2", "", &other)
	has := func(id primitive.ObjectID) bool { return count(t, h.db, "api_keys", bson.M{"_id": id}) == 1 }

	assertStyledRefusal(t, "viewer revoke", routed(r, "viewer", "POST", "/cm/api-keys/"+theirs.ID.Hex()+"/delete", nil))
	for _, role := range []string{"contributor", "editor"} {
		rr := routed(r, role, "POST", "/cm/api-keys/"+theirs.ID.Hex()+"/delete", nil)
		if !has(theirs.ID) {
			t.Fatalf("%s revoked another user's API key", role)
		}
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `id="forbidden-message"`) || !strings.Contains(rr.Body.String(), "not yours") {
			t.Errorf("%s revoking another user's key: %d, want the styled 403 saying the key is not theirs", role, rr.Code)
		}
	}
	if rr := routed(r, "editor", "POST", "/cm/api-keys/"+mine.ID.Hex()+"/delete", nil); rr.Code != http.StatusSeeOther || has(mine.ID) {
		t.Errorf("editor revoking their own key: %d, still there=%v", rr.Code, has(mine.ID))
	}
	if rr := routed(r, "admin", "POST", "/cm/api-keys/"+theirs2.ID.Hex()+"/delete", nil); rr.Code != http.StatusSeeOther || has(theirs2.ID) {
		t.Errorf("admin revoking any key: %d, still there=%v", rr.Code, has(theirs2.ID))
	}
	if !has(theirs.ID) {
		t.Error("the other user's first key is gone")
	}
}

// The configuration page carries the Cloudflare API token in its form. Only
// a role that can save the form gets it.
func TestAdminSiteConfig_TokenOnlyForSettingsEdit(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	r := adminRouter(h)
	ctx := context.Background()

	const token = "cf-secret-token-0123456789"
	cfg, _ := h.db.GetSiteConfig(ctx)
	cfg.CloudflareZoneID, cfg.CloudflareAPIToken = "zone123", token
	if err := h.db.SaveSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	for _, role := range []string{"viewer", "contributor", "editor"} {
		rr := routed(r, role, "GET", "/cm/config", nil)
		if rr.Code != http.StatusOK {
			t.Errorf("%s GET /cm/config: %d", role, rr.Code)
		}
		if strings.Contains(rr.Body.String(), token) {
			t.Errorf("%s can read the Cloudflare API token from /cm/config", role)
		}
	}
	if rr := routed(r, "admin", "GET", "/cm/config", nil); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), token) {
		t.Errorf("admin GET /cm/config: %d, token present=%v", rr.Code, strings.Contains(rr.Body.String(), token))
	}
	if c, _ := h.db.GetSiteConfig(ctx); c.CloudflareAPIToken != token {
		t.Errorf("stored token changed: %q", c.CloudflareAPIToken)
	}
}

// The broken-link scan fetches every external link in published content from
// the server. It needs content.edit, like the fix, and it never contacts a
// loopback, private or link-local address a page links to.
func TestBrokenLinkScan_PermissionAndSSRFGuard(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	var hits int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close() // listens on 127.0.0.1
	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Links inward", FullPath: "/ssrf-page", Published: true,
		Data: map[string]interface{}{"body": `<a href="` + internal.URL + `/admin">a</a> <a href="http://169.254.169.254/latest/meta-data/">b</a>`}})

	for _, role := range []string{"viewer", "contributor"} {
		rr := roleCall(h.BrokenLinkScan, role, "GET", "/api/tools/broken-links/scan", "", "", nil)
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"error"`) {
			t.Errorf("%s scan: %d %s, want a JSON 403", role, rr.Code, rr.Body.String())
		}
	}
	if rr := roleCall(h.BrokenLinkScan, "", "GET", "/api/tools/broken-links/scan", "", "", nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("scan without a session: %d, want 401", rr.Code)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("a refused scan made %d request(s)", n)
	}

	rr := roleCall(h.BrokenLinkScan, "editor", "GET", "/api/tools/broken-links/scan", "", "", nil)
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "event: complete") {
		t.Fatalf("editor scan: %d %s", rr.Code, body)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("the scan made %d request(s) to a loopback address taken from page content", n)
	}
	// Both links are reported as broken rather than fetched
	if !strings.Contains(body, internal.URL+"/admin") || !strings.Contains(body, "169.254.169.254") || !strings.Contains(body, `"totalBrokenLinks": 2`) {
		t.Errorf("scan output does not report the two internal links as broken:\n%s", body)
	}
}

// Unpublishing a page in a folder removes that page's generated file — not
// the file of a root page that happens to have the same slug.
func TestAdminUpdateContent_UnpublishRemovesOnlyItsOwnFile(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	folderID := primitive.NewObjectID()
	if _, err := h.db.InsertOne(ctx, "folders", &models.Folder{ID: folderID, Name: "C1 docs", Slug: "c1-docs", Path: "/c1-docs", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	root := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Root about", Slug: "c1-about", FullPath: "/c1-about", Published: true})
	inFolder := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Folder about", Slug: "c1-about", FolderID: &folderID, FolderPath: "/c1-docs", FullPath: "/c1-docs/c1-about", Published: true})
	const rootHTML, folderHTML = "<p>ROOT ABOUT</p>", "<p>FOLDER ABOUT</p>"
	writeStatic(t, "/c1-about", rootHTML)
	writeStatic(t, "/c1-docs/c1-about", folderHTML)
	t.Cleanup(func() { os.Remove(filepath.Dir(staticFile("/c1-docs/c1-about"))) })

	// Unpublish the folder page (Publish unticked)
	rr := postForm(t, h.UpdateContent, url.Values{"title": {"Folder about"}, "slug": {"c1-about"}, "folder_id": {folderID.Hex()}}, map[string]string{"id": inFolder.Hex()})
	if rr.Code != http.StatusSeeOther || strings.Contains(rr.Header().Get("Location"), "error") {
		t.Fatalf("unpublish folder page: %d -> %s", rr.Code, rr.Header().Get("Location"))
	}
	if c := loadPage(t, h.db, inFolder); c.Published || c.FullPath != "/c1-docs/c1-about" {
		t.Fatalf("folder page after save: published=%v path=%q", c.Published, c.FullPath)
	}
	assertStatic(t, "unpublishing /c1-docs/c1-about", "/c1-about", rootHTML)
	if _, err := os.Stat(staticFile("/c1-docs/c1-about")); err == nil {
		t.Error("the unpublished folder page's own static file is still there")
	}
	if c := loadPage(t, h.db, root); !c.Published {
		t.Error("root page was unpublished")
	}

	// A legacy row with no full_path: deleting it must not remove the
	// homepage's file (an empty path used to map to index.html).
	writeStatic(t, "/index", "<p>HOME</p>")
	legacy := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Legacy", Slug: "c1-legacy", FullPath: ""})
	if rr := postForm(t, h.DeleteContent, nil, map[string]string{"id": legacy.Hex()}); rr.Code != http.StatusSeeOther {
		t.Fatalf("delete legacy page: %d %s", rr.Code, rr.Body.String())
	}
	assertStatic(t, "deleting a page with an empty full_path", "/index", "<p>HOME</p>")
}

// Admin search-and-replace is audit-logged like the API's.
func TestAdminReplaceExecute_IsAuditLogged(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "A numbat page", FullPath: "/c2-one", Data: map[string]interface{}{"body": "numbat and numbat"}})
	seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Other", FullPath: "/c2-two", Data: map[string]interface{}{"body": "nothing here"}})

	rr := roleCall(h.ReplaceExecute, "admin", "POST", "/cm/replace/execute", `{"search":"numbat","replace":"quokka"}`, "application/json", nil)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"updated_count":1`) {
		t.Fatalf("execute: %d %s", rr.Code, rr.Body.String())
	}

	var entry models.AuditLog
	deadline := time.Now().Add(10 * time.Second)
	for {
		err := h.db.FindOne(ctx, "audit_logs", bson.M{"action": "content.search_replace"}, &entry)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("admin search-and-replace wrote no content.search_replace audit entry")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if entry.UserEmail != "admin@localhost" || entry.Resource != "content" || entry.ViaAPI {
		t.Errorf("audit entry = %+v", entry)
	}
	num := func(v interface{}) int64 {
		switch n := v.(type) {
		case int32:
			return int64(n)
		case int64:
			return n
		case int:
			return int64(n)
		}
		return -1
	}
	if num(entry.Details["pages_updated"]) != 1 || num(entry.Details["total_replacements"]) != 3 || num(entry.Details["pairs_count"]) != 1 {
		t.Errorf("audit details = %v, want 1 page, 3 replacements (title + two in the body), 1 pair", entry.Details)
	}

	// A refused run is not logged as a replace
	roleCall(h.ReplaceExecute, "editor", "POST", "/cm/replace/execute", `{"search":"quokka","replace":"x"}`, "application/json", nil)
	time.Sleep(500 * time.Millisecond)
	if n := count(t, h.db, "audit_logs", bson.M{"action": "content.search_replace"}); n != 1 {
		t.Errorf("%d content.search_replace entries, want 1", n)
	}
}

// Analytics links a path to the page to edit. A fork copy shares its live
// page's path and must never be what the link opens.
func TestAdminAnalytics_EditLinksResolveLivePagesOnly(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	forkID := createTestFork(t, h.db, "c12")
	// One path with the copy inserted first, one with the live page first:
	// an unfiltered lookup gets one of them wrong either way.
	copyA := seedPage(t, h.db, models.Content{Title: "Copy A", Slug: "c12-a", FullPath: "/c12-a", ForkID: &forkID})
	liveA := seedPage(t, h.db, models.Content{Title: "Live A", Slug: "c12-a", FullPath: "/c12-a", Published: true})
	liveB := seedPage(t, h.db, models.Content{Title: "Live B", Slug: "c12-b", FullPath: "/c12-b", Published: true})
	copyB := seedPage(t, h.db, models.Content{Title: "Copy B", Slug: "c12-b", FullPath: "/c12-b", ForkID: &forkID})
	seedPage(t, h.db, models.Content{Title: "Fork only", Slug: "c12-new", FullPath: "/c12-new", ForkID: &forkID})

	stats := []services.PageStat{{Path: "/c12-a"}, {Path: "/c12-b"}, {Path: "/c12-new"}}
	h.resolveEditIDs(ctx, stats)
	if stats[0].EditID != liveA.Hex() || stats[1].EditID != liveB.Hex() {
		t.Errorf("edit IDs = %q, %q; want the live pages %s, %s (copies are %s, %s)", stats[0].EditID, stats[1].EditID, liveA.Hex(), liveB.Hex(), copyA.Hex(), copyB.Hex())
	}
	if stats[2].EditID != "" {
		t.Errorf("a path that exists only in a fork resolved to %s", stats[2].EditID)
	}

	for path, live := range map[string]primitive.ObjectID{"/c12-a": liveA, "/c12-b": liveB} {
		rr := httptest.NewRecorder()
		h.AnalyticsPageDetail(rr, sessionReq("GET", "/cm/analytics/page?path="+url.QueryEscape(path), nil, nil))
		body := rr.Body.String()
		if rr.Code != http.StatusOK || !strings.Contains(body, "/cm/content/"+live.Hex()) {
			t.Errorf("page detail for %s: %d, edit link to the live page present=%v", path, rr.Code, strings.Contains(body, "/cm/content/"+live.Hex()))
		}
		if strings.Contains(body, copyA.Hex()) || strings.Contains(body, copyB.Hex()) {
			t.Errorf("page detail for %s links to a fork copy", path)
		}
	}
}

// The repair endpoint: admin only, dry_run changes nothing, a real run is
// audit-logged.
func TestAPIRepairForkDamage(t *testing.T) {
	a, db, cleanup := newTestAPIHandler(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := seedTemplate(t, db, "Page", "page")
	forkID := createTestFork(t, db, "c14api")
	lost := seedPage(t, db, models.Content{TemplateID: tmplID, Title: "Lost", Slug: "c14api-lost", FullPath: "/c14api-lost", Published: true})
	flagged := seedPage(t, db, models.Content{TemplateID: tmplID, Title: "Flagged", Slug: "c14api-lost", FullPath: "/c14api-lost", ForkID: &forkID, Published: true})
	os.Remove(staticFile("/c14api-lost"))
	t.Cleanup(func() { os.Remove(staticFile("/c14api-lost")) })

	// Editors (and below) are refused
	for _, role := range []string{"viewer", "contributor", "editor"} {
		req := httptest.NewRequest("POST", "/api/v1/maintenance/repair-fork-damage", nil)
		req = req.WithContext(injectRole(req.Context(), role))
		rr := httptest.NewRecorder()
		a.APIRepairForkDamage(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s repair: %d, want 403", role, rr.Code)
		}
	}
	if c := loadPage(t, db, flagged); !c.Published {
		t.Fatal("a refused repair cleared the flag")
	}

	rr, out := apiCall(t, a.APIRepairForkDamage, "POST", "/api/v1/maintenance/repair-fork-damage?dry_run=true", "", nil)
	if rr.Code != http.StatusOK || out["dry_run"] != true || len(out["fork_copies_published"].([]interface{})) != 1 || len(out["missing_static"].([]interface{})) != 1 {
		t.Fatalf("dry run: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, db, flagged); !c.Published {
		t.Error("dry run cleared the flag")
	}
	if _, err := os.Stat(staticFile("/c14api-lost")); err == nil {
		t.Error("dry run generated the file")
	}
	time.Sleep(300 * time.Millisecond)
	if n := count(t, db, "audit_logs", bson.M{"action": "maintenance.repair_fork_damage"}); n != 0 {
		t.Errorf("dry run wrote %d audit entries", n)
	}

	rr, out = apiCall(t, a.APIRepairForkDamage, "POST", "/api/v1/maintenance/repair-fork-damage", "", nil)
	if rr.Code != http.StatusOK || out["dry_run"] != false || out["fork_copies_cleared"] != float64(1) || out["regenerated"] != float64(1) {
		t.Fatalf("repair: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, db, flagged); c.Published {
		t.Error("repair left the fork copy flagged published")
	}
	if c := loadPage(t, db, lost); !c.Published {
		t.Error("repair unpublished the live page")
	}
	if _, err := os.Stat(staticFile("/c14api-lost")); err != nil {
		t.Errorf("repair did not regenerate the live page's file: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for count(t, db, "audit_logs", bson.M{"action": "maintenance.repair_fork_damage"}) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("the real run was not audit-logged")
		}
		time.Sleep(100 * time.Millisecond)
	}
	var entry models.AuditLog
	if err := db.FindOne(ctx, "audit_logs", bson.M{"action": "maintenance.repair_fork_damage"}, &entry); err != nil || entry.UserEmail != "admin@localhost" {
		t.Errorf("audit entry = %+v (%v)", entry, err)
	}
}
