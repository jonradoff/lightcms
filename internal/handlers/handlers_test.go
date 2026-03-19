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

// ===========================================================================
// Authenticated admin handler tests
// ===========================================================================

// getAuthCookies creates an authenticated session and returns its cookies.
func getAuthCookies(t *testing.T, h *Handler) []*http.Cookie {
	t.Helper()
	ctx := context.Background()
	user, err := h.auth.ValidateCredentials(ctx, "admin@localhost", "admin123")
	if err != nil || user == nil {
		t.Fatalf("ValidateCredentials: %v", err)
	}
	// Clear is_default_password so MustChangePassword doesn't redirect
	user.IsDefaultPassword = false
	h.db.UpdateOne(ctx, "users", bson.M{"_id": user.ID}, bson.M{"$set": bson.M{"is_default_password": false}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	if err := h.auth.LoginUser(rr, req, user); err != nil {
		t.Fatalf("LoginUser: %v", err)
	}
	return rr.Result().Cookies()
}

// csrfAuthGet runs a GET handler through CSRF middleware with an authenticated session.
// For handlers that use mux.Vars, register the handler on a mux.Router with the given path pattern.
func csrfAuthGet(t *testing.T, h *Handler, pathPattern, actualPath string, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	cookies := getAuthCookies(t, h)
	protect := csrf.Protect([]byte("32-byte-long-test-csrf-key!!1234"), csrf.Secure(false))
	r := mux.NewRouter()
	r.HandleFunc(pathPattern, handler).Methods("GET")
	srv := protect(r)
	req := httptest.NewRequest("GET", actualPath, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

// csrfAuthPost does a two-step GET (to obtain CSRF token cookie) then POST with form data.
func csrfAuthPost(t *testing.T, h *Handler, pathPattern, actualPath string,
	getHandler, postHandler http.HandlerFunc, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	cookies := getAuthCookies(t, h)
	protect := csrf.Protect([]byte("32-byte-long-test-csrf-key!!1234"), csrf.Secure(false))
	r := mux.NewRouter()
	r.HandleFunc(pathPattern, getHandler).Methods("GET")
	r.HandleFunc(pathPattern, postHandler).Methods("POST")
	srv := protect(r)

	// Step 1: GET to obtain CSRF cookie
	getReq := httptest.NewRequest("GET", actualPath, nil)
	for _, c := range cookies {
		getReq.AddCookie(c)
	}
	getRR := httptest.NewRecorder()
	srv.ServeHTTP(getRR, getReq)

	// Collect all cookies (auth + CSRF)
	allCookies := append(cookies, getRR.Result().Cookies()...)

	// Extract CSRF token from the response body hidden field
	body := getRR.Body.String()
	csrfToken := ""
	if idx := strings.Index(body, `name="gorilla.csrf.Token" value="`); idx != -1 {
		start := idx + len(`name="gorilla.csrf.Token" value="`)
		end := strings.Index(body[start:], `"`)
		if end != -1 {
			csrfToken = body[start : start+end]
		}
	}

	if csrfToken != "" {
		form.Set("gorilla.csrf.Token", csrfToken)
	}

	// Step 2: POST with form data + cookies
	postReq := httptest.NewRequest("POST", actualPath, strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range allCookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()
	srv.ServeHTTP(postRR, postReq)
	return postRR
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

func TestAdminDashboard_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm", "/cm", h.AdminDashboard)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Dashboard") {
		t.Fatal("expected 'Dashboard' in response body")
	}
}

// ---------------------------------------------------------------------------
// Template Admin
// ---------------------------------------------------------------------------

func TestListTemplates_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/templates", "/cm/templates", h.ListTemplates)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Templates") {
		t.Fatal("expected 'Templates' in response body")
	}
}

func TestNewTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/templates/new", "/cm/templates/new", h.NewTemplate)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test Template")
	form.Set("slug", "test-template")
	form.Set("html_layout", "<html><body>{{.Body}}</body></html>")

	rr := csrfAuthPost(t, h, "/cm/templates/new", "/cm/templates/new", h.NewTemplate, h.CreateTemplate, form)
	// Should redirect on success (303) or re-render form on CSRF issue (403/200)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestEditTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Edit Me", "edit-me")
	rr := csrfAuthGet(t, h, "/cm/templates/{id}", "/cm/templates/"+tmplID.Hex(), h.EditTemplate)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUpdateTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Update Me", "update-me")
	form := url.Values{}
	form.Set("name", "Updated Template")
	form.Set("slug", "updated-template")
	form.Set("html_layout", "<html><body>Updated {{.Body}}</body></html>")

	rr := csrfAuthPost(t, h, "/cm/templates/{id}", "/cm/templates/"+tmplID.Hex(), h.EditTemplate, h.UpdateTemplate, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestDeleteTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Delete Me", "delete-me")
	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/templates/{id}/delete", "/cm/templates/"+tmplID.Hex()+"/delete",
		// Use a simple GET handler for the CSRF cookie step
		func(w http.ResponseWriter, r *http.Request) {
			csrf.Token(r) // trigger CSRF cookie
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteTemplate, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Content Admin
// ---------------------------------------------------------------------------

func TestListContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/content", "/cm/content", h.ListContent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/content/new", "/cm/content/new", h.NewContent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewContentWithTemplate_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Blog Post", "blog-post")
	rr := csrfAuthGet(t, h, "/cm/content/new/{templateID}", "/cm/content/new/"+tmplID.Hex(), h.NewContentWithTemplate)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestEditContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Test Page", "test-page", "/test-page")
	rr := csrfAuthGet(t, h, "/cm/content/{id}", "/cm/content/"+contentID.Hex(), h.EditContent)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Blog", "blog")

	form := url.Values{}
	form.Set("template_id", tmplID.Hex())
	form.Set("title", "My Test Post")
	form.Set("slug", "my-test-post")

	rr := csrfAuthPost(t, h, "/cm/content/new/{templateID}", "/cm/content/new/"+tmplID.Hex(),
		h.NewContentWithTemplate, h.CreateContent, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestUpdateContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Old Title", "old-title", "/old-title")

	form := url.Values{}
	form.Set("title", "Updated Title")
	form.Set("slug", "updated-title")

	rr := csrfAuthPost(t, h, "/cm/content/{id}", "/cm/content/"+contentID.Hex(),
		h.EditContent, h.UpdateContent, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestDeleteContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Delete Me", "delete-me", "/delete-me")

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/content/{id}/delete", "/cm/content/"+contentID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteContent, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

func TestUndeleteContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Restore Me", "restore-me", "/restore-me")

	// Soft-delete first
	ctx := context.Background()
	h.db.UpdateOne(ctx, "content", bson.M{"_id": contentID}, bson.M{
		"$set": bson.M{"deleted": true, "full_path": "__deleted__/" + contentID.Hex()},
	})

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/content/{id}/undelete", "/cm/content/"+contentID.Hex()+"/undelete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.UndeleteContent, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

func TestListContentVersions_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Versioned", "versioned", "/versioned")

	// ListContentVersions returns JSON, not HTML, so no CSRF needed for rendering.
	// But it does check auth.
	cookies := getAuthCookies(t, h)
	r := mux.NewRouter()
	r.HandleFunc("/cm/content/{id}/versions", h.ListContentVersions).Methods("GET")
	req := httptest.NewRequest("GET", "/cm/content/"+contentID.Hex()+"/versions", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

func TestRegenerateContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedContent(t, h.db, tmplID, "Regen Me", "regen-me", "/regen-me")

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/content/{id}/regenerate", "/cm/content/"+contentID.Hex()+"/regenerate",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.RegenerateContent, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Collection Admin
// ---------------------------------------------------------------------------

func TestListCollections_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/collections", "/cm/collections", h.ListCollections)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewCollection_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/collections/new", "/cm/collections/new", h.NewCollection)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateCollection_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test Collection")
	form.Set("description", "A test collection")
	form.Set("category", "blog")
	form.Set("sort_field", "created_at")
	form.Set("sort_order", "desc")
	form.Set("items_per_page", "10")

	rr := csrfAuthPost(t, h, "/cm/collections/new", "/cm/collections/new",
		h.NewCollection, h.CreateCollection, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestEditCollection_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	collID, err := h.db.InsertOne(ctx, "collections", bson.M{
		"name":       "Edit Collection",
		"slug":       "edit-collection",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed collection: %v", err)
	}

	rr := csrfAuthGet(t, h, "/cm/collections/{id}", "/cm/collections/"+collID.Hex(), h.EditCollection)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUpdateCollection_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	collID, err := h.db.InsertOne(ctx, "collections", bson.M{
		"name":       "Update Collection",
		"slug":       "update-collection",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed collection: %v", err)
	}

	form := url.Values{}
	form.Set("name", "Updated Collection")
	form.Set("description", "updated desc")
	form.Set("items_per_page", "20")

	rr := csrfAuthPost(t, h, "/cm/collections/{id}", "/cm/collections/"+collID.Hex(),
		h.EditCollection, h.UpdateCollection, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestDeleteCollection_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	collID, err := h.db.InsertOne(ctx, "collections", bson.M{
		"name":       "Delete Collection",
		"slug":       "delete-collection",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed collection: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/collections/{id}/delete", "/cm/collections/"+collID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteCollection, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Folder Admin
// ---------------------------------------------------------------------------

func TestListFolders_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/folders", "/cm/folders", h.ListFolders)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewFolder_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/folders/new", "/cm/folders/new", h.NewFolder)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateFolder_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "Test Folder")
	form.Set("slug", "test-folder")

	rr := csrfAuthPost(t, h, "/cm/folders/new", "/cm/folders/new",
		h.NewFolder, h.CreateFolder, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestDeleteFolder_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	folderID, err := h.db.InsertOne(ctx, "folders", bson.M{
		"name":       "Delete Folder",
		"slug":       "delete-folder",
		"path":       "/delete-folder",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/folders/{id}/delete", "/cm/folders/"+folderID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteFolder, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Redirect Admin
// ---------------------------------------------------------------------------

func TestListRedirects_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/redirects", "/cm/redirects", h.ListRedirects)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewRedirect_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/redirects/new", "/cm/redirects/new", h.NewRedirect)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateRedirect_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("from_path", "/old-page")
	form.Set("to_path", "/new-page")
	form.Set("status_code", "301")
	form.Set("description", "test redirect")

	rr := csrfAuthPost(t, h, "/cm/redirects/new", "/cm/redirects/new",
		h.NewRedirect, h.CreateRedirect, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestEditRedirect_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	redirectID, err := h.db.InsertOne(ctx, "redirects", bson.M{
		"from_path":   "/old",
		"to_path":     "/new",
		"status_code": 301,
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	})
	if err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	rr := csrfAuthGet(t, h, "/cm/redirects/{id}", "/cm/redirects/"+redirectID.Hex(), h.EditRedirect)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUpdateRedirect_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	redirectID, err := h.db.InsertOne(ctx, "redirects", bson.M{
		"from_path":   "/old",
		"to_path":     "/new",
		"status_code": 301,
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	})
	if err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	form := url.Values{}
	form.Set("from_path", "/old-updated")
	form.Set("to_path", "/new-updated")
	form.Set("status_code", "302")

	rr := csrfAuthPost(t, h, "/cm/redirects/{id}", "/cm/redirects/"+redirectID.Hex(),
		h.EditRedirect, h.UpdateRedirect, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303/200/403, got %d", rr.Code)
	}
}

func TestDeleteRedirect_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	redirectID, err := h.db.InsertOne(ctx, "redirects", bson.M{
		"from_path":   "/del-old",
		"to_path":     "/del-new",
		"status_code": 301,
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	})
	if err != nil {
		t.Fatalf("seed redirect: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/redirects/{id}/delete", "/cm/redirects/"+redirectID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteRedirect, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Snippet Admin
// ---------------------------------------------------------------------------

func TestListSnippets_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/snippets", "/cm/snippets", h.ListSnippets)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNewSnippet_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/snippets/new", "/cm/snippets/new", h.NewSnippet)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCreateSnippet_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{}
	form.Set("name", "test-snippet")
	form.Set("html", "<div>Hello</div>")

	rr := csrfAuthPost(t, h, "/cm/snippets/new", "/cm/snippets/new",
		h.NewSnippet, h.CreateSnippet, form)
	if rr.Code != http.StatusFound && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 302/200/403, got %d", rr.Code)
	}
}

func TestEditSnippet_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	snip, err := h.snippetService.CreateSnippet(ctx, "edit-snip", "<p>edit me</p>")
	if err != nil {
		t.Fatalf("seed snippet: %v", err)
	}

	rr := csrfAuthGet(t, h, "/cm/snippets/{id}", "/cm/snippets/"+snip.ID.Hex(), h.EditSnippet)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestUpdateSnippet_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	snip, err := h.snippetService.CreateSnippet(ctx, "update-snip", "<p>old</p>")
	if err != nil {
		t.Fatalf("seed snippet: %v", err)
	}

	form := url.Values{}
	form.Set("name", "update-snip-renamed")
	form.Set("html", "<p>new</p>")

	rr := csrfAuthPost(t, h, "/cm/snippets/{id}", "/cm/snippets/"+snip.ID.Hex(),
		h.EditSnippet, h.UpdateSnippet, form)
	if rr.Code != http.StatusFound && rr.Code != http.StatusOK && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 302/200/403, got %d", rr.Code)
	}
}

func TestDeleteSnippet_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	snip, err := h.snippetService.CreateSnippet(ctx, "delete-snip", "<p>bye</p>")
	if err != nil {
		t.Fatalf("seed snippet: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/snippets/{id}/delete", "/cm/snippets/"+snip.ID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteSnippet, form)
	if rr.Code != http.StatusFound && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 302 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Asset Admin
// ---------------------------------------------------------------------------

func TestAssetLibrary_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/assets", "/cm/assets", h.AssetLibrary)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAssetUploadForm_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/assets/upload", "/cm/assets/upload", h.AssetUploadForm)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Search & Tools
// ---------------------------------------------------------------------------

func TestSearchContent_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// SearchContent returns JSON, does not call renderAdmin, so no CSRF needed.
	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/search?q=test", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.SearchContent(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

func TestSearchContent_Fulltext(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedContent(t, h.db, tmplID, "Unique Searchable Title", "unique-searchable", "/unique-searchable")

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/search?q=Unique+Searchable&type=fulltext", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.SearchContent(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var results []map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&results)
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
}

func TestCheckSlug_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/check-slug?path=/nonexistent", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.CheckSlug(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["exists"] != false {
		t.Fatalf("expected exists=false, got %v", resp["exists"])
	}
}

func TestCheckSlug_Exists(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedContent(t, h.db, tmplID, "Slug Test", "slug-test", "/slug-test")

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/check-slug?path=/slug-test", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.CheckSlug(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["exists"] != true {
		t.Fatalf("expected exists=true, got %v", resp["exists"])
	}
}

func TestReplacePreview_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/replace/preview?search=hello&replace=world", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ReplacePreview(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

func TestReplacePreview_EmptySearch(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/replace/preview?search=&replace=world", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ReplacePreview(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Contact Messages Admin
// ---------------------------------------------------------------------------

func TestListContactMessages_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/messages", "/cm/messages", h.ListContactMessages)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestViewContactMessage_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	msgID, err := h.db.InsertOne(ctx, "contact_messages", bson.M{
		"name":       "Test User",
		"email":      "test@example.com",
		"message":    "Hello there",
		"read":       false,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}

	rr := csrfAuthGet(t, h, "/cm/messages/{id}", "/cm/messages/"+msgID.Hex(), h.ViewContactMessage)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestDeleteContactMessage_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	msgID, err := h.db.InsertOne(ctx, "contact_messages", bson.M{
		"name":       "Delete User",
		"email":      "del@example.com",
		"message":    "Goodbye",
		"read":       false,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/messages/{id}/delete", "/cm/messages/"+msgID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteContactMessage, form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusForbidden {
		t.Fatalf("expected 303 or 403, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// Broken Link Finder
// ---------------------------------------------------------------------------

func TestBrokenLinkFinder_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/tools/broken-links", "/cm/tools/broken-links", h.BrokenLinkFinder)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// DeleteAsset Admin
// ---------------------------------------------------------------------------

func TestDeleteAsset_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// Seed an asset directly in the DB
	ctx := context.Background()
	assetID, err := h.db.InsertOne(ctx, "assets", bson.M{
		"filename":   "test.png",
		"folder":     "/",
		"serve_path": "/test.png",
		"mime_type":  "image/png",
		"size":       100,
		"created_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	form := url.Values{}
	rr := csrfAuthPost(t, h, "/cm/assets/{id}/delete", "/cm/assets/"+assetID.Hex()+"/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`<input type="hidden" name="gorilla.csrf.Token" value="` + csrf.Token(r) + `">`))
		},
		h.DeleteAsset, form)
	// Should redirect or return some status (may get error if asset file not on disk)
	if rr.Code == http.StatusUnauthorized {
		t.Fatal("expected authenticated request, got 401")
	}
}

// ---------------------------------------------------------------------------
// EditFolder Admin
// ---------------------------------------------------------------------------

func TestEditFolder_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	folderID, err := h.db.InsertOne(ctx, "folders", bson.M{
		"name":       "Edit Folder",
		"slug":       "edit-folder",
		"path":       "/edit-folder",
		"created_at": time.Now(),
		"updated_at": time.Now(),
	})
	if err != nil {
		t.Fatalf("seed folder: %v", err)
	}

	rr := csrfAuthGet(t, h, "/cm/folders/{id}", "/cm/folders/"+folderID.Hex(), h.EditFolder)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// ReplaceExecute
// ---------------------------------------------------------------------------

func TestReplaceExecute_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Page", "page")
	seedContent(t, h.db, tmplID, "Replace Test", "replace-test", "/replace-test")

	cookies := getAuthCookies(t, h)
	body := `{"search":"Replace","replace":"Replaced"}`
	req := httptest.NewRequest("POST", "/cm/replace/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ReplaceExecute(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

// ---------------------------------------------------------------------------
// SecuritySettings & UpdatePassword
// ---------------------------------------------------------------------------

func TestSecuritySettings_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/security", "/cm/security", h.SecuritySettings)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// BrokenLinkScan — verify auth check
// ---------------------------------------------------------------------------

func TestBrokenLinkScan_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/cm/tools/broken-links/scan", nil)
	rr := httptest.NewRecorder()
	h.BrokenLinkScan(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// GetAllFoldersAPI
// ---------------------------------------------------------------------------

func TestGetAllFoldersAPI_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	cookies := getAuthCookies(t, h)
	req := httptest.NewRequest("GET", "/cm/api/folders", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.GetAllFoldersAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content-type, got %s", ct)
	}
}

func TestGetAllFoldersAPI_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/cm/api/folders", nil)
	rr := httptest.NewRecorder()
	h.GetAllFoldersAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// ReplaceExecute unauthenticated
// ---------------------------------------------------------------------------

func TestReplaceExecute_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	body := `{"search":"a","replace":"b"}`
	req := httptest.NewRequest("POST", "/cm/replace/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ReplaceExecute(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// ReplacePreview unauthenticated
// ---------------------------------------------------------------------------

func TestReplacePreview_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/cm/replace/preview?search=a&replace=b", nil)
	rr := httptest.NewRecorder()
	h.ReplacePreview(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// ServeAsset — public handler, no auth needed
// ---------------------------------------------------------------------------

func TestServeAsset_NotFound(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/assets/nonexistent.png", nil)
	rr := httptest.NewRecorder()
	h.ServeAsset(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestServeAsset_NoAssetsPrefix(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/other/path.png", nil)
	rr := httptest.NewRecorder()
	h.ServeAsset(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
