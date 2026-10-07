package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/middleware"
	"github.com/jonradoff/lightcms/v7/internal/models"
)

// Tests for the 7.4.1 admin follow-ups: the admin search-and-replace and
// broken-link fix endpoints check a permission and sit behind CSRF, admin
// search-and-replace leaves fork copies alone, and no admin editor action on
// a fork copy touches the live page (its static file above all).

// roleReq is sessionReq for any role: a request carrying a valid session
// cookie for a user with that role.
func roleReq(role, method, target string, body io.Reader, vars map[string]string) *http.Request {
	store := sessions.NewCookieStore([]byte(testSessionSecret))
	req := httptest.NewRequest(method, target, body)
	rec := httptest.NewRecorder()
	sess, _ := store.New(req, "lightcms-session")
	sess.Values["authenticated"] = true
	sess.Values["user_id"] = "000000000000000000000001"
	sess.Values["user_email"] = role + "@localhost"
	sess.Values["user_role"] = role
	_ = sess.Save(req, rec)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	return req
}

// roleCall runs a handler as a user with the given role ("" = no session).
func roleCall(h http.HandlerFunc, role, method, target, body, contentType string, vars map[string]string) *httptest.ResponseRecorder {
	var req *http.Request
	if role == "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		if vars != nil {
			req = mux.SetURLVars(req, vars)
		}
	} else {
		req = roleReq(role, method, target, strings.NewReader(body), vars)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// staticFile is where the handlers and services write a page's generated
// HTML, relative to the package directory the tests run in.
func staticFile(fullPath string) string {
	return filepath.Join("content/generated", strings.TrimPrefix(fullPath, "/")+".html")
}

// writeStatic puts a known static file in place for a live page and removes
// it when the test ends.
func writeStatic(t *testing.T, fullPath, html string) {
	t.Helper()
	p := staticFile(fullPath)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(html), 0644); err != nil {
		t.Fatalf("write static: %v", err)
	}
	t.Cleanup(func() { os.Remove(p) })
}

func assertStatic(t *testing.T, when, fullPath, want string) {
	t.Helper()
	got, err := os.ReadFile(staticFile(fullPath))
	if err != nil {
		t.Errorf("%s: live static file %s is gone: %v", when, staticFile(fullPath), err)
		return
	}
	if string(got) != want {
		t.Errorf("%s: live static file changed: %q, want %q", when, got, want)
	}
}

func pageExists(t *testing.T, db *database.DB, id primitive.ObjectID) bool {
	t.Helper()
	n, err := db.Count(context.Background(), "content", bson.M{"_id": id})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return n == 1
}

func TestAdminReplace_RoleMatrix(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	body := func(s string) map[string]interface{} { return map[string]interface{}{"body": s} }
	page := seedPage(t, h.db, models.Content{Title: "Role page", FullPath: "/role-page", Data: body("a wombat lives here")})
	const exec = `{"search":"wombat","replace":"quoll"}`

	for _, role := range []string{"viewer", "contributor", "editor"} {
		rr := roleCall(h.ReplacePreview, role, "GET", "/cm/replace/preview?search=wombat&replace=quoll", "", "", nil)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s preview: %d, want 403 (%s)", role, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), page.Hex()) || strings.Contains(rr.Body.String(), "role-page") {
			t.Errorf("%s preview leaked matches: %s", role, rr.Body.String())
		}
		rr = roleCall(h.ReplaceExecute, role, "POST", "/cm/replace/execute", exec, "application/json", nil)
		var out map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		if rr.Code != http.StatusForbidden || out["error"] == nil || out["success"] != nil {
			t.Errorf("%s execute: %d %s, want 403 with a JSON error", role, rr.Code, rr.Body.String())
		}
		if c := loadPage(t, h.db, page); c.Data["body"] != "a wombat lives here" {
			t.Fatalf("%s execute changed the page: %v", role, c.Data["body"])
		}
	}

	// No session: 401, as before.
	if rr := roleCall(h.ReplacePreview, "", "GET", "/cm/replace/preview?search=wombat", "", "", nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("anonymous preview: %d, want 401", rr.Code)
	}
	if rr := roleCall(h.ReplaceExecute, "", "POST", "/cm/replace/execute", exec, "application/json", nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("anonymous execute: %d, want 401", rr.Code)
	}
	if c := loadPage(t, h.db, page); c.Data["body"] != "a wombat lives here" {
		t.Fatalf("anonymous execute changed the page: %v", c.Data["body"])
	}

	// Admin: same response shape as before.
	rr := roleCall(h.ReplacePreview, "admin", "GET", "/cm/replace/preview?search=wombat&replace=quoll", "", "", nil)
	var prev map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &prev)
	m := idSet(prev["matches"])
	if rr.Code != 200 || len(m) != 1 || m[page.Hex()] == nil || prev["search"] != "wombat" || prev["replace"] != "quoll" {
		t.Fatalf("admin preview: %d %s", rr.Code, rr.Body.String())
	}
	for _, f := range []string{"id", "title", "full_path", "match_count", "excerpts"} {
		if _, ok := m[page.Hex()][f]; !ok {
			t.Errorf("admin preview match lacks %q: %v", f, m[page.Hex()])
		}
	}
	rr = roleCall(h.ReplaceExecute, "admin", "POST", "/cm/replace/execute", exec, "application/json", nil)
	var out map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	if rr.Code != 200 || out["success"] != true || out["updated_count"] != float64(1) || len(out) != 2 {
		t.Fatalf("admin execute: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, h.db, page); c.Data["body"] != "a quoll lives here" {
		t.Errorf("admin execute did not rewrite the page: %v", c.Data["body"])
	}
}

// TestAdminReplace_CSRFRequired mounts the execute handler the way
// cmd/server does (under /cm, behind middleware.CSRFProtect) and checks that
// a POST without the token is refused before the handler runs.
func TestAdminReplace_CSRFRequired(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	page := seedPage(t, h.db, models.Content{Title: "CSRF page", FullPath: "/csrf-page", Data: map[string]interface{}{"body": "a wombat lives here"}})

	r := mux.NewRouter()
	admin := r.PathPrefix("/cm").Subrouter()
	admin.Use(middleware.CSRFProtect([]byte("32-byte-long-test-csrf-key!!1234"), false))
	admin.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, csrf.Token(r)) }).Methods("GET")
	admin.HandleFunc("/replace/execute", h.ReplaceExecute).Methods("POST")

	post := func(token string, cookies []*http.Cookie, origin string) *httptest.ResponseRecorder {
		req := roleReq("admin", "POST", "http://localhost/cm/replace/execute", strings.NewReader(`{"search":"wombat","replace":"quoll"}`), nil)
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		return rr
	}
	unchanged := func(when string) {
		t.Helper()
		if c := loadPage(t, h.db, page); c.Data["body"] != "a wombat lives here" {
			t.Fatalf("%s: page was rewritten: %v", when, c.Data["body"])
		}
	}

	// An admin session, no token: what a cross-site POST carries at best.
	rr := post("", nil, "http://evil.example")
	if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "CSRF") {
		t.Fatalf("no token: %d %s, want 403 from the CSRF middleware", rr.Code, rr.Body.String())
	}
	unchanged("no token")

	// Fetch a real token and its cookie.
	get := roleReq("admin", "GET", "http://localhost/cm/token", nil, nil)
	grr := httptest.NewRecorder()
	r.ServeHTTP(grr, get)
	token, cookies := grr.Body.String(), grr.Result().Cookies()
	if grr.Code != 200 || token == "" || len(cookies) == 0 {
		t.Fatalf("token fetch: %d token=%q cookies=%d", grr.Code, token, len(cookies))
	}

	// A valid token sent from another origin is still refused.
	if rr := post(token, cookies, "http://evil.example"); rr.Code != http.StatusForbidden {
		t.Fatalf("token from a foreign origin: %d, want 403", rr.Code)
	}
	unchanged("foreign origin")

	// A token without its cookie is refused.
	if rr := post(token, nil, "http://localhost"); rr.Code != http.StatusForbidden {
		t.Fatalf("token without cookie: %d, want 403", rr.Code)
	}
	unchanged("token without cookie")

	// The admin UI's request: same origin, cookie and X-CSRF-Token header.
	rr = post(token, cookies, "http://localhost")
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"updated_count":1`) {
		t.Fatalf("with token: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, h.db, page); c.Data["body"] != "a quoll lives here" {
		t.Errorf("with token: page not rewritten: %v", c.Data["body"])
	}
}

// TestServerRoutes_AdminStateChangesAreUnderCSRF reads the route table in
// cmd/server/main.go. The /api subrouter has no CSRF middleware, so the only
// non-GET route allowed on it is the public contact form; the admin
// search-and-replace and broken-link fix routes must be on the /cm subrouter.
func TestServerRoutes_AdminStateChangesAreUnderCSRF(t *testing.T) {
	src, err := os.ReadFile("../../cmd/server/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	main := string(src)

	if !regexp.MustCompile(`admin := r\.PathPrefix\("/cm"\)\.Subrouter\(\)\s*\n\s*admin\.Use\(csrfMiddleware\)`).MatchString(main) {
		t.Fatal("the /cm subrouter no longer installs csrfMiddleware first")
	}

	apiRoute := regexp.MustCompile(`(?m)^\s*api\.Handle(?:Func)?\("([^"]+)".*\.Methods\(([^)]*)\)`)
	routes := apiRoute.FindAllStringSubmatch(main, -1)
	if len(routes) < 5 {
		t.Fatalf("found only %d /api routes; the pattern no longer matches main.go", len(routes))
	}
	publicWrites := map[string]bool{"/contact": true}
	for _, m := range routes {
		path, methods := m[1], m[2]
		if regexp.MustCompile(`"(POST|PUT|PATCH|DELETE)"`).MatchString(methods) && !publicWrites[path] {
			t.Errorf("/api%s accepts %s outside the CSRF-protected /cm subrouter", path, methods)
		}
	}
	if n := len(regexp.MustCompile(`(?m)^\s*api\.Handle(?:Func)?\(`).FindAllString(main, -1)); n != len(routes) {
		t.Errorf("%d /api routes registered but %d have a Methods(...) restriction on the same line", n, len(routes))
	}

	for _, want := range []string{
		`admin.HandleFunc("/replace/preview", h.ReplacePreview).Methods("GET")`,
		`admin.HandleFunc("/replace/execute", h.ReplaceExecute).Methods("POST")`,
		`admin.HandleFunc("/tools/broken-links/fix", h.FixBrokenLink).Methods("POST")`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("main.go lacks %s", want)
		}
	}

	// The admin UI calls the /cm routes and sends the token on the writes.
	initAdminTemplateCache()
	ui := adminTemplates["content_list"] + adminTemplates["broken_links"]
	for _, gone := range []string{"/api/content/replace-", "/api/tools/fix-link"} {
		if strings.Contains(ui, gone) {
			t.Errorf("admin UI still calls %s", gone)
		}
	}
	for _, re := range []string{
		`fetch\('/cm/replace/execute', \{\s*method: 'POST',\s*headers: \{[^}]*'X-CSRF-Token'`,
		`fetch\('/cm/tools/broken-links/fix', \{\s*method: 'POST',\s*headers: \{[^}]*'X-CSRF-Token'`,
		`fetch\('/cm/replace/preview\?`,
	} {
		if !regexp.MustCompile(re).MatchString(ui) {
			t.Errorf("admin UI does not match %s", re)
		}
	}
}

func TestAdminFixBrokenLink_RequiresEditPermission(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	const before = `<a href="http://old.example/x">link</a>`
	page := seedPage(t, h.db, models.Content{Title: "Links", FullPath: "/links-page", Data: map[string]interface{}{"body": before}})
	req := `{"contentId":"` + page.Hex() + `","field":"body","oldUrl":"http://old.example/x","newUrl":"http://new.example/y"}`

	for _, role := range []string{"viewer", "contributor"} {
		rr := roleCall(h.FixBrokenLink, role, "POST", "/cm/tools/broken-links/fix", req, "application/json", nil)
		if rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), `"error"`) {
			t.Errorf("%s: %d %s, want 403 with a JSON error", role, rr.Code, rr.Body.String())
		}
		if c := loadPage(t, h.db, page); c.Data["body"] != before {
			t.Fatalf("%s changed the page: %v", role, c.Data["body"])
		}
	}
	if rr := roleCall(h.FixBrokenLink, "", "POST", "/cm/tools/broken-links/fix", req, "application/json", nil); rr.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", rr.Code)
	}
	rr := roleCall(h.FixBrokenLink, "editor", "POST", "/cm/tools/broken-links/fix", req, "application/json", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"success":true`) {
		t.Fatalf("editor: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, h.db, page); c.Data["body"] != `<a href="http://new.example/y">link</a>` {
		t.Errorf("editor fix not applied: %v", c.Data["body"])
	}
}

func TestAdminReplace_LeavesForkCopiesAlone(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	forkID := createTestFork(t, h.db, "adminsr")
	body := func(s string) map[string]interface{} { return map[string]interface{}{"body": s} }
	live := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "B live wombat", FullPath: "/adminsr-page", Published: true, Data: body("a wombat lives here")})
	other := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "A other", FullPath: "/adminsr-other", Data: body("another wombat")})
	forkCopy := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Copy", FullPath: "/adminsr-page", ForkID: &forkID, Data: body("a wombat in the fork")})
	forkOnly := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "wombat draft", FullPath: "/adminsr-new", ForkID: &forkID, Data: body("new wombat page")})
	t.Cleanup(func() { os.Remove(staticFile("/adminsr-page")) })

	rr := roleCall(h.ReplacePreview, "admin", "GET", "/cm/replace/preview?search=wombat&replace=quoll", "", "", nil)
	var prev struct {
		Matches []struct {
			ID         string `json:"id"`
			MatchCount int    `json:"match_count"`
		} `json:"matches"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &prev)
	// Live pages only, ordered by title as before ("A other" before "B live wombat").
	if rr.Code != 200 || len(prev.Matches) != 2 || prev.Matches[0].ID != other.Hex() || prev.Matches[1].ID != live.Hex() || prev.Matches[1].MatchCount != 2 {
		t.Fatalf("preview = %s, want the two live pages in title order", rr.Body.String())
	}

	rr = roleCall(h.ReplaceExecute, "admin", "POST", "/cm/replace/execute", `{"search":"wombat","replace":"quoll"}`, "application/json", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"updated_count":2`) {
		t.Fatalf("execute: %d %s, want updated_count 2 (the live pages)", rr.Code, rr.Body.String())
	}
	c := loadPage(t, h.db, live)
	if c.Data["body"] != "a quoll lives here" || c.Title != "B live quoll" || !c.Published {
		t.Errorf("live page: title=%q body=%v published=%v", c.Title, c.Data["body"], c.Published)
	}
	if c := loadPage(t, h.db, forkCopy); c.Data["body"] != "a wombat in the fork" {
		t.Errorf("fork copy rewritten: %v", c.Data["body"])
	}
	if c := loadPage(t, h.db, forkOnly); c.Data["body"] != "new wombat page" || c.Title != "wombat draft" {
		t.Errorf("fork-only page rewritten: %q %v", c.Title, c.Data["body"])
	}

	// Through the service layer: a version with the editor and a real comment.
	var v models.ContentVersion
	if err := h.db.FindOne(context.Background(), "content_versions", bson.M{"content_id": live, "title": "B live quoll"}, &v); err != nil {
		t.Fatalf("no version saved for the rewritten page: %v", err)
	}
	if v.ModifiedByEmail != "admin@localhost" || !strings.Contains(v.Comment, "wombat") || strings.Contains(v.Comment, "link replacement") {
		t.Errorf("version: by=%q comment=%q", v.ModifiedByEmail, v.Comment)
	}
	if n, _ := h.db.Count(context.Background(), "content_versions", bson.M{"content_id": bson.M{"$in": []primitive.ObjectID{forkCopy, forkOnly}}}); n != 0 {
		t.Errorf("%d versions written for fork copies", n)
	}
	// The published live page is regenerated with the new text.
	if html, err := os.ReadFile(staticFile("/adminsr-page")); err != nil {
		t.Errorf("published live page was not regenerated: %v", err)
	} else if strings.Contains(string(html), "wombat") {
		t.Errorf("regenerated page still has the old text: %s", html)
	}
}

func TestAdminDeleteContent_ForkCopyLeavesLivePageIntact(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	forkID := createTestFork(t, h.db, "admindel")
	const liveHTML = "<p>LIVE HTML</p>"
	live := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Live", FullPath: "/admindel-page", Published: true, Data: map[string]interface{}{"body": "live"}})
	writeStatic(t, "/admindel-page", liveHTML)
	if _, err := h.db.InsertOne(ctx, "redirects", models.Redirect{FromPath: "/admindel-old", ToPath: "/admindel-page", StatusCode: 301}); err != nil {
		t.Fatalf("seed redirect: %v", err)
	}
	copyPage, err := h.forkService.ForkPage(ctx, forkID, live)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	vars := map[string]string{"id": copyPage.ID.Hex()}

	intact := func(when string) {
		t.Helper()
		assertStatic(t, when, "/admindel-page", liveHTML)
		c := loadPage(t, h.db, live)
		if c.Deleted || !c.Published || c.FullPath != "/admindel-page" {
			t.Errorf("%s: live page changed: deleted=%v published=%v path=%q", when, c.Deleted, c.Published, c.FullPath)
		}
		if n, _ := h.db.Count(ctx, "redirects", bson.M{"to_path": "/admindel-page"}); n != 1 {
			t.Errorf("%s: redirect to the live page was removed", when)
		}
	}

	// Roles that cannot work in forks cannot remove the copy.
	for _, role := range []string{"viewer", "contributor"} {
		if rr := roleCall(h.DeleteContent, role, "POST", "/cm/x", "", "", vars); rr.Code != http.StatusForbidden {
			t.Errorf("%s deleting a fork copy: %d, want 403", role, rr.Code)
		}
		if !pageExists(t, h.db, copyPage.ID) {
			t.Fatalf("%s removed the fork copy", role)
		}
	}
	intact("after refused deletes")

	// An editor deleting the copy from its editor: removed from the fork, as
	// "Remove" on the fork page does, and sent back to the fork.
	rr := roleCall(h.DeleteContent, "editor", "POST", "/cm/x", "", "", vars)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/cm/forks/"+forkID.Hex() {
		t.Fatalf("delete fork copy: %d -> %q, want 303 to the fork page", rr.Code, rr.Header().Get("Location"))
	}
	if pageExists(t, h.db, copyPage.ID) {
		t.Error("fork copy still exists")
	}
	if pages, _ := h.forkService.ListPages(ctx, forkID); len(pages) != 0 {
		t.Errorf("fork still lists %d page(s)", len(pages))
	}
	intact("after deleting the fork copy")

	// Deleting a live page needs content.delete.
	liveVars := map[string]string{"id": live.Hex()}
	for _, role := range []string{"viewer", "contributor"} {
		if rr := roleCall(h.DeleteContent, role, "POST", "/cm/x", "", "", liveVars); rr.Code != http.StatusForbidden {
			t.Errorf("%s deleting a live page: %d, want 403", role, rr.Code)
		}
	}
	intact("after refused live deletes")
	if rr := roleCall(h.DeleteContent, "editor", "POST", "/cm/x", "", "", liveVars); rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/cm/content" {
		t.Fatalf("editor deleting a live page: %d -> %q", rr.Code, rr.Header().Get("Location"))
	}
	if c := loadPage(t, h.db, live); !c.Deleted {
		t.Error("live page not deleted by an editor")
	}
	if _, err := os.Stat(staticFile("/admindel-page")); err == nil {
		t.Error("deleting the live page left its static file behind")
	}
}

// TestAdminEditorActions_ForkCopyNeverTouchesLivePage covers the sibling
// editor actions that take a content id: save (including Published ticked
// and a slug change), regenerate, change template and revert version.
func TestAdminEditorActions_ForkCopyNeverTouchesLivePage(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	tmpl2 := seedTemplate(t, h.db, "Other", "other")
	forkID := createTestFork(t, h.db, "adminsib")
	const liveHTML = "<p>LIVE HTML</p>"
	live := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Sibling", Slug: "adminsib-page", FullPath: "/adminsib-page", Published: true, Data: map[string]interface{}{"body": "live"}})
	linker := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Linker", FullPath: "/adminsib-linker", Data: map[string]interface{}{"body": `<a href="/adminsib-page">x</a>`}})
	writeStatic(t, "/adminsib-page", liveHTML)
	t.Cleanup(func() { os.Remove(staticFile("/adminsib-renamed")) })
	copyPage, err := h.forkService.ForkPage(ctx, forkID, live)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}
	vars := map[string]string{"id": copyPage.ID.Hex()}

	intact := func(when string) {
		t.Helper()
		assertStatic(t, when, "/adminsib-page", liveHTML)
		c := loadPage(t, h.db, live)
		if c.Deleted || !c.Published || c.FullPath != "/adminsib-page" || c.Title != "Sibling" || c.TemplateID != tmplID {
			t.Errorf("%s: live page changed: %+v", when, c)
		}
		if c := loadPage(t, h.db, copyPage.ID); c.Published {
			t.Errorf("%s: fork copy is marked published", when)
		}
	}

	// Save with Published ticked.
	rr := postForm(t, h.UpdateContent, url.Values{"title": {"Sibling (fork edit)"}, "slug": {"adminsib-page"}, "published": {"on"}}, vars)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("save fork copy: %d %s", rr.Code, rr.Body.String())
	}
	if c := loadPage(t, h.db, copyPage.ID); c.Title != "Sibling (fork edit)" {
		t.Errorf("fork copy edit not saved: %q", c.Title)
	}
	intact("save with Published ticked")

	// Save as a draft (the pre-fix code removed the live file here).
	postForm(t, h.UpdateContent, url.Values{"title": {"Sibling (fork edit 2)"}, "slug": {"adminsib-page"}}, vars)
	intact("save as draft")

	// Rename the slug inside the fork, asking for a redirect.
	postForm(t, h.UpdateContent, url.Values{"title": {"Sibling (fork edit 2)"}, "slug": {"adminsib-renamed"}, "slug_rename_enabled": {"yes"}, "create_redirect": {"yes"}}, vars)
	if c := loadPage(t, h.db, copyPage.ID); c.FullPath != "/adminsib-renamed" {
		t.Errorf("fork copy rename not saved: %q", c.FullPath)
	}
	intact("slug change in the fork")
	if n, _ := h.db.Count(ctx, "redirects", bson.M{"from_path": "/adminsib-page"}); n != 0 {
		t.Error("renaming a fork copy created a redirect away from the live page")
	}
	if c := loadPage(t, h.db, linker); c.Data["body"] != `<a href="/adminsib-page">x</a>` {
		t.Errorf("renaming a fork copy rewrote links in a live page: %v", c.Data["body"])
	}
	if _, err := os.Stat(staticFile("/adminsib-renamed")); err == nil {
		t.Error("a static file was generated for the fork copy")
	}
	// Put the copy back on the live path for the remaining checks.
	postForm(t, h.UpdateContent, url.Values{"title": {"Sibling (fork edit 2)"}, "slug": {"adminsib-page"}, "slug_rename_enabled": {"yes"}}, vars)
	intact("slug changed back")

	// A copy left marked published by an earlier version must still never
	// reach the live file through regenerate, change template or revert.
	if err := h.db.UpdateOne(ctx, "content", bson.M{"_id": copyPage.ID}, bson.M{"$set": bson.M{"published": true}}); err != nil {
		t.Fatalf("mark copy published: %v", err)
	}
	if rr := postForm(t, h.RegenerateContent, nil, vars); rr.Code >= 400 {
		t.Errorf("regenerate: %d", rr.Code)
	}
	assertStatic(t, "regenerate", "/adminsib-page", liveHTML)
	if rr := postForm(t, h.ConfirmChangeTemplate, nil, map[string]string{"id": copyPage.ID.Hex(), "template_id": tmpl2.Hex()}); rr.Code >= 400 {
		t.Errorf("change template: %d %s", rr.Code, rr.Body.String())
	}
	assertStatic(t, "change template", "/adminsib-page", liveHTML)
	if c := loadPage(t, h.db, live); c.TemplateID != tmplID {
		t.Error("changing the fork copy's template changed the live page's")
	}
	var v models.ContentVersion
	if err := h.db.FindOne(ctx, "content_versions", bson.M{"content_id": copyPage.ID}, &v); err != nil {
		t.Fatalf("fork copy has no versions: %v", err)
	}
	if rr := postForm(t, h.RevertContentVersion, nil, map[string]string{"id": copyPage.ID.Hex(), "version": "1"}); rr.Code >= 400 {
		t.Errorf("revert: %d %s", rr.Code, rr.Body.String())
	}
	assertStatic(t, "revert version", "/adminsib-page", liveHTML)

	// The next save clears the stray published flag.
	postForm(t, h.UpdateContent, url.Values{"title": {"Sibling (fork edit 3)"}, "slug": {"adminsib-page"}, "published": {"on"}}, vars)
	intact("save after stray published flag")
}

// TestAdminContentForm_ForkCopyControls checks the editor of a fork copy:
// Published is disabled and explained, and the delete button says what it does.
func TestAdminContentForm_ForkCopyControls(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	forkID := createTestFork(t, h.db, "adminform")
	live := seedPage(t, h.db, models.Content{TemplateID: tmplID, Title: "Form live", FullPath: "/adminform-page", Published: true})
	copyPage, err := h.forkService.ForkPage(context.Background(), forkID, live)
	if err != nil {
		t.Fatalf("ForkPage: %v", err)
	}

	page := func(id primitive.ObjectID) string {
		rr := csrfAuthGet(t, h, "/cm/content/{id}", "/cm/content/"+id.Hex(), h.EditContent)
		if rr.Code != 200 {
			t.Fatalf("editor: %d", rr.Code)
		}
		return rr.Body.String()
	}
	fork := page(copyPage.ID)
	for _, want := range []string{`name="published" disabled>`, "goes live when the fork is merged", ">Remove from Fork</button>", "Remove this copy from the fork? The live page is not affected."} {
		if !strings.Contains(fork, want) {
			t.Errorf("fork copy editor lacks %q", want)
		}
	}
	if strings.Contains(fork, ">Delete Page</button>") {
		t.Error("fork copy editor still offers Delete Page")
	}
	liveHTML := page(live)
	for _, want := range []string{`name="published" checked>`, ">Delete Page</button>", "Are you sure you want to delete this page? This cannot be undone."} {
		if !strings.Contains(liveHTML, want) {
			t.Errorf("live editor lacks %q", want)
		}
	}
	if strings.Contains(liveHTML, "Remove from Fork") || strings.Contains(liveHTML, `name="published" disabled`) {
		t.Error("live editor shows the fork copy controls")
	}
}
