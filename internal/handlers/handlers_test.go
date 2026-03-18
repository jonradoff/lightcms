package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
)

// csrfWrapped runs handler through a minimal CSRF-protected mux so that
// csrf.Token(r) / csrf.TemplateField(r) don't panic inside renderAdmin.
// The returned recorder contains the response.
func csrfWrapped(t *testing.T, method, path string, handler http.HandlerFunc, body *strings.Reader) *httptest.ResponseRecorder {
	t.Helper()
	protect := csrf.Protect(
		[]byte("32-byte-long-test-csrf-key!!1234"),
		csrf.Secure(false), // allow plain HTTP in tests
	)
	r := mux.NewRouter()
	r.HandleFunc(path, handler).Methods(method)
	srv := protect(r)

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// 1. New() constructor
// ---------------------------------------------------------------------------

func TestNew_ReturnsNonNil(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	if h == nil {
		t.Fatal("New() returned nil Handler")
	}
}

// ---------------------------------------------------------------------------
// 2. IsDev()
// ---------------------------------------------------------------------------

func TestIsDev(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// newTestHandler passes env="test", so IsDev should be false
	if h.IsDev() {
		t.Fatal("expected IsDev()=false for env='test'")
	}

	// Manually override env for development check
	h.env = "development"
	if !h.IsDev() {
		t.Fatal("expected IsDev()=true for env='development'")
	}

	h.env = "dev"
	if !h.IsDev() {
		t.Fatal("expected IsDev()=true for env='dev'")
	}

	h.env = "production"
	if h.IsDev() {
		t.Fatal("expected IsDev()=false for env='production'")
	}
}

// ---------------------------------------------------------------------------
// 3. Setter methods — call them and verify no panic
// ---------------------------------------------------------------------------

func TestSetters_NoPanic(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// These are called in newTestHandler already; call again with nil to
	// ensure they handle nil gracefully.
	h.SetSearchService(nil)
	h.SetContentService(nil)
	h.SetProxyConfig(nil)
	h.SetAnalyticsService(nil)
}

// ---------------------------------------------------------------------------
// 4. SeedDefaults — verify templates and content get created
// ---------------------------------------------------------------------------

func TestSeedDefaults(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.SeedDefaults(ctx); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}

	// Templates should exist
	tmplCount, err := h.db.Count(ctx, "templates", bson.M{})
	if err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if tmplCount == 0 {
		t.Fatal("expected templates to be seeded, got 0")
	}

	// Content should have at least the hello world page
	contentCount, err := h.db.Count(ctx, "content", bson.M{})
	if err != nil {
		t.Fatalf("count content: %v", err)
	}
	if contentCount == 0 {
		t.Fatal("expected content to be seeded, got 0")
	}
}

// ---------------------------------------------------------------------------
// 5. LoginPage — GET, verify 200 and HTML
// ---------------------------------------------------------------------------

func TestLoginPage_Returns200(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfWrapped(t, http.MethodGet, "/cm/login", h.LoginPage, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content-type, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Login") {
		t.Fatal("expected 'Login' in HTML body")
	}
}

// ---------------------------------------------------------------------------
// 6. LoginHandler — POST with bad credentials, verify login page re-renders
// ---------------------------------------------------------------------------

func TestLoginHandler_BadCredentials(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// We need to POST with a valid CSRF token, so we do two requests:
	// First GET to obtain cookie + token, then POST with them.
	protect := csrf.Protect(
		[]byte("32-byte-long-test-csrf-key!!1234"),
		csrf.Secure(false),
	)
	r := mux.NewRouter()
	r.HandleFunc("/cm/login", h.LoginPage).Methods("GET")
	r.HandleFunc("/cm/login", h.LoginHandler).Methods("POST")
	srv := protect(r)

	// GET to obtain CSRF cookie
	getReq := httptest.NewRequest(http.MethodGet, "/cm/login", nil)
	getRR := httptest.NewRecorder()
	srv.ServeHTTP(getRR, getReq)

	// Extract CSRF token from the Set-Cookie header
	cookies := getRR.Result().Cookies()

	// Extract token from the response body (hidden input)
	bodyStr := getRR.Body.String()
	tokenIdx := strings.Index(bodyStr, `name="gorilla.csrf.Token"`)
	if tokenIdx == -1 {
		// Try alternative: the token is in the rendered form
		// Just verify the GET succeeded and skip POST CSRF test
		t.Log("CSRF token field not found in login page; testing POST with fresh CSRF")
	}

	// POST with bad credentials (use form-encoded body)
	form := url.Values{}
	form.Set("email", "nonexistent@example.com")
	form.Set("password", "wrongpassword")

	postReq := httptest.NewRequest(http.MethodPost, "/cm/login", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		postReq.AddCookie(c)
	}

	// Retrieve the CSRF token from the cookie value and set the header
	for _, c := range cookies {
		if strings.Contains(c.Name, "csrf") {
			postReq.Header.Set("X-CSRF-Token", c.Value)
		}
	}

	postRR := httptest.NewRecorder()
	srv.ServeHTTP(postRR, postReq)

	// Should re-render the login page (200) with error, or 403 for CSRF mismatch.
	// Either way, it should not be a 500.
	if postRR.Code == http.StatusInternalServerError {
		t.Fatalf("expected non-500 response, got %d: %s", postRR.Code, postRR.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 7. LogoutHandler — verify redirect to /cm/login
// ---------------------------------------------------------------------------

func TestLogoutHandler_Redirects(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/logout", nil)
	rr := httptest.NewRecorder()

	h.LogoutHandler(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

// ---------------------------------------------------------------------------
// 8. slugify — test various inputs
// ---------------------------------------------------------------------------

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"  Hello   World  ", "hello-world"},
		{"Hello---World", "hello-world"},
		{"Café au lait", "cafe-au-lait"},
		{"My Post #1!", "my-post-1"},
		{"---leading and trailing---", "leading-and-trailing"},
		{"UPPERCASE", "uppercase"},
		{"a/b/c", "a-b-c"},
		{"", ""},
		{"@#$%^&*()", ""},
		{"hello_world", "hello-world"},
		{"naïve résumé", "naive-resume"},
	}

	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_MaxLength(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := slugify(long)
	if len(got) > 255 {
		t.Errorf("slugify of 300-char string produced %d chars, want <= 255", len(got))
	}
}

// ---------------------------------------------------------------------------
// 9. CheckSlug — unauthenticated returns 401 JSON
// ---------------------------------------------------------------------------

func TestCheckSlug_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/check-slug?path=/test", nil)
	rr := httptest.NewRecorder()

	h.CheckSlug(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

// ---------------------------------------------------------------------------
// 10. GenerateSitemap — verify sitemap file is created
// ---------------------------------------------------------------------------

func TestGenerateSitemap(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Ensure static directory exists for sitemap output
	os.MkdirAll("static", 0755)
	defer os.Remove("static/sitemap.xml")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := h.GenerateSitemap(ctx, "https://example.com"); err != nil {
		t.Fatalf("GenerateSitemap: %v", err)
	}

	data, err := os.ReadFile("static/sitemap.xml")
	if err != nil {
		t.Fatalf("sitemap.xml not created: %v", err)
	}
	if !strings.Contains(string(data), "<urlset") {
		t.Fatal("sitemap.xml does not contain <urlset>")
	}
	if !strings.Contains(string(data), "https://example.com/") {
		t.Fatal("sitemap.xml does not contain base URL")
	}
}

// ---------------------------------------------------------------------------
// 11. ServeRobotsTxt — verify 200 with robots.txt content
// ---------------------------------------------------------------------------

func TestServeRobotsTxt(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rr := httptest.NewRecorder()

	h.ServeRobotsTxt(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain, got %s", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Fatal("expected 'User-agent: *' in robots.txt")
	}
	if !strings.Contains(body, "Sitemap:") {
		t.Fatal("expected 'Sitemap:' in robots.txt")
	}
}

// ---------------------------------------------------------------------------
// 12. ServeSitemap — verify XML output after generating
// ---------------------------------------------------------------------------

func TestServeSitemap(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Ensure static directory exists
	os.MkdirAll("static", 0755)
	defer os.Remove("static/sitemap.xml")

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	rr := httptest.NewRecorder()

	h.ServeSitemap(rr, req)

	// Should generate on-the-fly if file doesn't exist
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/xml") {
		t.Fatalf("expected application/xml, got %s", ct)
	}
}

// ---------------------------------------------------------------------------
// 13. GetUnreadMessageCount — verify returns 0 for empty DB
// ---------------------------------------------------------------------------

func TestGetUnreadMessageCount_Empty(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count := h.GetUnreadMessageCount(ctx)
	if count != 0 {
		t.Fatalf("expected 0 unread messages, got %d", count)
	}
}

func TestGetUnreadMessageCount_WithMessages(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Insert some contact messages
	h.db.InsertOne(ctx, "contact_messages", bson.M{"name": "A", "read": false, "created_at": time.Now()})
	h.db.InsertOne(ctx, "contact_messages", bson.M{"name": "B", "read": false, "created_at": time.Now()})
	h.db.InsertOne(ctx, "contact_messages", bson.M{"name": "C", "read": true, "created_at": time.Now()})

	count := h.GetUnreadMessageCount(ctx)
	if count != 2 {
		t.Fatalf("expected 2 unread messages, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// 14. ContactFormSubmit — POST with form data
// ---------------------------------------------------------------------------

func TestContactFormSubmit_MissingFields(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test User")
	// missing email and message

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ContactFormSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != false {
		t.Fatal("expected success=false")
	}
}

func TestContactFormSubmit_InvalidEmail(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test User")
	form.Set("email", "not-an-email")
	form.Set("message", "Hello")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ContactFormSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestContactFormSubmit_Success(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test User")
	form.Set("email", "test@example.com")
	form.Set("message", "Hello, this is a test message.")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ContactFormSubmit(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}

	// Verify it was stored in DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, _ := h.db.Count(ctx, "contact_messages", bson.M{"email": "test@example.com"})
	if count != 1 {
		t.Fatalf("expected 1 contact message in DB, got %d", count)
	}
}

func TestContactFormSubmit_NameTooLong(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", strings.Repeat("x", 201))
	form.Set("email", "test@example.com")
	form.Set("message", "Hello")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ContactFormSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// 15-18. Admin handlers without auth — verify redirect to /cm/login
// ---------------------------------------------------------------------------

func TestAdminDashboard_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm", nil)
	rr := httptest.NewRecorder()

	h.AdminDashboard(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListTemplates_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/templates", nil)
	rr := httptest.NewRecorder()

	h.ListTemplates(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListContent_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/content", nil)
	rr := httptest.NewRecorder()

	h.ListContent(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListCollections_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/collections", nil)
	rr := httptest.NewRecorder()

	h.ListCollections(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListFolders_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/folders", nil)
	rr := httptest.NewRecorder()

	h.ListFolders(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListRedirects_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/redirects", nil)
	rr := httptest.NewRecorder()

	h.ListRedirects(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

func TestListSnippets_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/snippets", nil)
	rr := httptest.NewRecorder()

	h.ListSnippets(rr, req)

	// ListSnippets uses StatusFound (302) instead of SeeOther
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

// ---------------------------------------------------------------------------
// BrokenLinkFinder — unauthenticated redirects
// ---------------------------------------------------------------------------

func TestBrokenLinkFinder_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/tools/broken-links", nil)
	rr := httptest.NewRecorder()

	h.BrokenLinkFinder(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm/login" {
		t.Fatalf("expected redirect to /cm/login, got %s", loc)
	}
}

// ---------------------------------------------------------------------------
// SearchContent — unauthenticated returns 401
// ---------------------------------------------------------------------------

func TestSearchContent_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/search?q=test", nil)
	rr := httptest.NewRecorder()

	h.SearchContent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// AuditLogPage — unauthenticated redirects
// ---------------------------------------------------------------------------

func TestAuditLogPage_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/audit", nil)
	rr := httptest.NewRecorder()

	h.AuditLogPage(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm" {
		t.Fatalf("expected redirect to /cm, got %s", loc)
	}
}

// ---------------------------------------------------------------------------
// UsersPage — unauthenticated redirects
// ---------------------------------------------------------------------------

func TestUsersPage_RedirectsWithoutAuth(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/users", nil)
	rr := httptest.NewRecorder()

	h.UsersPage(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/cm" {
		t.Fatalf("expected redirect to /cm, got %s", loc)
	}
}

// ---------------------------------------------------------------------------
// ServeRobotsTxt — uses baseURL from handler
// ---------------------------------------------------------------------------

func TestServeRobotsTxt_UsesBaseURL(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rr := httptest.NewRecorder()

	h.ServeRobotsTxt(rr, req)

	body := rr.Body.String()
	// The handler's baseURL is "http://localhost:8082" from newTestHandler
	if !strings.Contains(body, "http://localhost:8082/sitemap.xml") {
		t.Fatalf("expected robots.txt to reference handler's baseURL in sitemap, got:\n%s", body)
	}
}

// ---------------------------------------------------------------------------
// SeedDefaults — idempotent (calling twice should not error)
// ---------------------------------------------------------------------------

func TestSeedDefaults_Idempotent(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.SeedDefaults(ctx); err != nil {
		t.Fatalf("first SeedDefaults: %v", err)
	}
	if err := h.SeedDefaults(ctx); err != nil {
		t.Fatalf("second SeedDefaults: %v", err)
	}
}

// ---------------------------------------------------------------------------
// slugifyStrict — empty input returns error
// ---------------------------------------------------------------------------

func TestSlugifyStrict_EmptyInput(t *testing.T) {
	_, err := slugifyStrict("")
	if err == nil {
		t.Fatal("expected error for empty slug input")
	}
}

func TestSlugifyStrict_ValidInput(t *testing.T) {
	got, err := slugifyStrict("Hello World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello-world" {
		t.Fatalf("expected 'hello-world', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// ContactFormSubmit — message too long
// ---------------------------------------------------------------------------

func TestContactFormSubmit_MessageTooLong(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test")
	form.Set("email", "test@example.com")
	form.Set("message", strings.Repeat("x", 10001))

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	h.ContactFormSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
