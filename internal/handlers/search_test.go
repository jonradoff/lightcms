package handlers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Pure helper function tests — no DB needed
// ---------------------------------------------------------------------------

func TestClampFloat_BelowMin(t *testing.T) {
	if got := clampFloat(-2.0, -1.0, 1.0); got != -1.0 {
		t.Fatalf("expected -1.0, got %v", got)
	}
}

func TestClampFloat_AboveMax(t *testing.T) {
	if got := clampFloat(2.0, -1.0, 1.0); got != 1.0 {
		t.Fatalf("expected 1.0, got %v", got)
	}
}

func TestClampFloat_InRange(t *testing.T) {
	if got := clampFloat(0.5, -1.0, 1.0); got != 0.5 {
		t.Fatalf("expected 0.5, got %v", got)
	}
}

func TestCheckGlobalSearchRateLimit_NotExceeded(t *testing.T) {
	// Calling it should not panic.
	_ = checkGlobalSearchRateLimit()
}

func TestCheckSearchRateLimit_NilProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?q=test", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	// Should not panic with nil proxyConfig.
	_ = checkSearchRateLimit(req, nil)
}

// ---------------------------------------------------------------------------
// EndUserSearch (no auth)
// ---------------------------------------------------------------------------

func TestEndUserSearch_MissingQuery(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rr := httptest.NewRecorder()
	h.EndUserSearch(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEndUserSearch_EmptyResults(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/search?q=uniquenonexistentquery12345", nil)
	rr := httptest.NewRecorder()
	h.EndUserSearch(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestEndUserSearch_LongQuery(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	longQ := strings.Repeat("a", 600)
	req := httptest.NewRequest(http.MethodGet, "/search?q="+longQ, nil)
	rr := httptest.NewRecorder()
	h.EndUserSearch(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// EndUserSearchSuggest (no auth)
// ---------------------------------------------------------------------------

func TestEndUserSearchSuggest_EmptyPrefix(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/search/suggest?q=", nil)
	rr := httptest.NewRecorder()
	h.EndUserSearchSuggest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestEndUserSearchSuggest_ShortPrefix(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/search/suggest?q=a", nil)
	rr := httptest.NewRecorder()
	h.EndUserSearchSuggest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestEndUserSearchSuggest_ValidPrefix(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/search/suggest?q=ab", nil)
	rr := httptest.NewRecorder()
	h.EndUserSearchSuggest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// SearchToolPage (admin, authenticated)
// ---------------------------------------------------------------------------

func TestSearchToolPage_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfWrapped(t, http.MethodGet, "/cm/tools/search", h.SearchToolPage, nil)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rr.Code)
	}
}

func TestSearchToolPage_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/tools/search", "/cm/tools/search", h.SearchToolPage)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SearchToolSaveConfig (admin POST)
// ---------------------------------------------------------------------------

func TestSearchToolSaveConfig_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfWrapped(t, http.MethodPost, "/cm/tools/search/config",
		h.SearchToolSaveConfig, strings.NewReader("nav_boost=0.5"))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestSearchToolSaveConfig_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	form := url.Values{
		"nav_boost":   {"0.1"},
		"title_boost": {"0.8"},
	}
	rr := csrfAuthPost(t, h,
		"/cm/tools/search/config", "/cm/tools/search/config",
		h.SearchToolPage, h.SearchToolSaveConfig,
		form)
	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusFound && rr.Code != http.StatusOK {
		t.Fatalf("expected redirect or 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SearchToolTest (admin GET)
// ---------------------------------------------------------------------------

func TestSearchToolTest_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/cm/tools/search/test?q=hello", nil)
	rr := httptest.NewRecorder()
	h.SearchToolTest(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestSearchToolTest_Authenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rr := csrfAuthGet(t, h, "/cm/tools/search/test", "/cm/tools/search/test?q=hello", h.SearchToolTest)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// SearchToolReindex (admin, no voyage key)
// ---------------------------------------------------------------------------

func TestSearchToolReindex_Unauthenticated(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/cm/tools/search/reindex", nil)
	rr := httptest.NewRecorder()
	h.SearchToolReindex(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestSearchToolReindex_NoVoyageKey(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	// POST without voyage key → 400 (not a CSRF-protected route, just auth check)
	rr := csrfAuthGet(t, h, "/cm/tools/search/reindex", "/cm/tools/search/reindex", h.SearchToolReindex)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no voyage key), got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// isPrivateOrReservedIP (pure function)
// ---------------------------------------------------------------------------

func TestIsPrivateOrReservedIP_Loopback(t *testing.T) {
	if !isPrivateOrReservedIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("expected 127.0.0.1 to be private")
	}
}

func TestIsPrivateOrReservedIP_RFC1918(t *testing.T) {
	if !isPrivateOrReservedIP(net.ParseIP("192.168.1.1")) {
		t.Fatal("expected 192.168.1.1 to be private")
	}
	if !isPrivateOrReservedIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("expected 10.0.0.1 to be private")
	}
}

func TestIsPrivateOrReservedIP_Public(t *testing.T) {
	if isPrivateOrReservedIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected 8.8.8.8 to be public")
	}
}

func TestIsPrivateOrReservedIP_IPv6Loopback(t *testing.T) {
	if !isPrivateOrReservedIP(net.ParseIP("::1")) {
		t.Fatal("expected ::1 to be private")
	}
}

// ---------------------------------------------------------------------------
// isValidAssetServePath (pure function)
// ---------------------------------------------------------------------------

func TestIsValidAssetServePath_Valid(t *testing.T) {
	cases := []string{"/assets/img.jpg", "/images/logo.png", "/docs/manual.pdf", "/media/video.mp4", "/files/data.zip"}
	for _, c := range cases {
		if !isValidAssetServePath(c) {
			t.Fatalf("expected %q to be valid", c)
		}
	}
}

func TestIsValidAssetServePath_Invalid(t *testing.T) {
	cases := []string{"/static/img.jpg", "/uploads/file.txt", "/random/path", ""}
	for _, c := range cases {
		if isValidAssetServePath(c) {
			t.Fatalf("expected %q to be invalid", c)
		}
	}
}

// ---------------------------------------------------------------------------
// APIEndUserSearchSuggest (API handler)
// ---------------------------------------------------------------------------

func TestAPIEndUserSearchSuggest_ShortPrefix(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/suggest?q=a", nil)
	rr := httptest.NewRecorder()
	ah.APIEndUserSearchSuggest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAPIEndUserSearchSuggest_ValidPrefix(t *testing.T) {
	ah, _, cleanup := newTestAPIHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search/suggest?q=hello", nil)
	rr := httptest.NewRecorder()
	ah.APIEndUserSearchSuggest(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
