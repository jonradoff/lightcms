package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_BasicHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/some-page", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		got := rr.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("header %s: expected %q, got %q", tt.header, tt.expected, got)
		}
	}
}

func TestSecurityHeaders_HSTS_HTTPS(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("expected HSTS header for HTTPS request")
	}
	if hsts != "max-age=31536000; includeSubDomains" {
		t.Errorf("unexpected HSTS value: %q", hsts)
	}
}

func TestSecurityHeaders_NoHSTS_HTTP(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No TLS, no X-Forwarded-Proto
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("expected no HSTS header for HTTP request, got %q", hsts)
	}
}

func TestSecurityHeaders_CSP_AdminPages(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/cm/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected CSP header for admin page")
	}
	// Admin CSP should restrict frame-ancestors
	if !contains(csp, "frame-ancestors 'none'") {
		t.Error("expected frame-ancestors 'none' in admin CSP")
	}
	// Admin CSP should have form-action 'self' only
	if !contains(csp, "form-action 'self'") {
		t.Error("expected form-action 'self' in admin CSP")
	}
}

func TestSecurityHeaders_CSP_PublicPages(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/blog/my-post", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected CSP header for public page")
	}
	// Public pages allow https: for scripts (analytics, widgets)
	if !contains(csp, "script-src 'self' 'unsafe-inline' https:") {
		t.Error("expected permissive script-src in public CSP")
	}
}

func TestSecurityHeaders_NoCSP_StaticAssets(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{"/static/css/main.css", "/assets/images/logo.png"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		csp := rr.Header().Get("Content-Security-Policy")
		if csp != "" {
			t.Errorf("expected no CSP for %s, got %q", path, csp)
		}
	}
}

func TestSecurityHeaders_NextHandlerCalled(t *testing.T) {
	called := false
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("expected next handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// File type validation tests

func TestIsAllowedFileType(t *testing.T) {
	allowed := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".pdf", ".css", ".js", ".json", ".woff2", ".ttf", ".html"}
	for _, ext := range allowed {
		if !IsAllowedFileType(ext) {
			t.Errorf("expected %s to be allowed", ext)
		}
	}

	disallowed := []string{".exe", ".sh", ".bat", ".php", ".py", ".rb", ".go", ".java"}
	for _, ext := range disallowed {
		if IsAllowedFileType(ext) {
			t.Errorf("expected %s to be disallowed", ext)
		}
	}
}

func TestIsAllowedFileType_CaseInsensitive(t *testing.T) {
	if !IsAllowedFileType(".JPG") {
		t.Error("expected .JPG to be allowed (case-insensitive)")
	}
	if !IsAllowedFileType(".Png") {
		t.Error("expected .Png to be allowed (case-insensitive)")
	}
}

func TestValidateMIMEType(t *testing.T) {
	tests := []struct {
		ext      string
		mime     string
		expected bool
	}{
		{".jpg", "image/jpeg", true},
		{".png", "image/png", true},
		{".jpg", "image/png", false},
		{".css", "text/css", true},
		{".js", "application/javascript", true},
		{".js", "text/javascript", true},
		{".woff2", "application/octet-stream", true}, // font fallback
		{".svg", "text/xml", true},                    // SVG special case
		{".svg", "text/plain", true},                  // SVG special case
		{".css", "text/plain", true},                  // text/plain fallback for CSS
		{".js", "text/plain", true},                   // text/plain fallback for JS
		{".json", "text/plain", true},                 // text/plain fallback for JSON
		{".xml", "text/plain", true},                  // text/plain fallback for XML
		{".csv", "text/plain", true},                  // text/plain fallback for CSV
		{".html", "text/plain", true},                 // text/plain fallback for HTML
		{".exe", "application/octet-stream", false},   // not allowed extension
	}

	for _, tt := range tests {
		result := ValidateMIMEType(tt.ext, tt.mime)
		if result != tt.expected {
			t.Errorf("ValidateMIMEType(%s, %s): expected %v, got %v", tt.ext, tt.mime, tt.expected, result)
		}
	}
}

func TestValidateMIMEType_CharsetStripping(t *testing.T) {
	// MIME types with charset should still validate
	if !ValidateMIMEType(".css", "text/css; charset=utf-8") {
		t.Error("expected text/css with charset to be valid for .css")
	}
}

func TestValidateFilePath(t *testing.T) {
	valid := []string{"/images/logo.png", "/css/main.css", "/a/b/c"}
	for _, path := range valid {
		if !ValidateFilePath(path) {
			t.Errorf("expected %q to be valid", path)
		}
	}

	invalid := []struct {
		name string
		path string
	}{
		{"directory traversal", "../../../etc/passwd"},
		{"embedded traversal", "/images/../../secret"},
		{"null byte", "/images/logo\x00.png"},
		{"backslash", "/images\\logo.png"},
		{"double slash", "/images//logo.png"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if ValidateFilePath(tt.path) {
				t.Errorf("expected %q to be invalid", tt.path)
			}
		})
	}
}

// Trusted proxy / GetClientIP tests

func TestGetClientIP_NoConfig(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	ip := GetClientIP(req, nil)
	if ip != "192.168.1.1" {
		t.Errorf("expected RemoteAddr IP, got %q", ip)
	}
}

func TestGetClientIP_TrustAllProxies(t *testing.T) {
	config := &TrustedProxyConfig{TrustAllProxies: true}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 172.16.0.1")

	ip := GetClientIP(req, config)
	if ip != "203.0.113.50" {
		t.Errorf("expected first XFF IP, got %q", ip)
	}
}

func TestGetClientIP_TrustAllProxies_NoXFF(t *testing.T) {
	config := &TrustedProxyConfig{TrustAllProxies: true}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:12345"

	ip := GetClientIP(req, config)
	if ip != "172.16.0.1" {
		t.Errorf("expected RemoteAddr IP when no XFF, got %q", ip)
	}
}

func TestGetClientIP_SpecificProxies(t *testing.T) {
	config := &TrustedProxyConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.2")

	ip := GetClientIP(req, config)
	if ip != "203.0.113.50" {
		t.Errorf("expected first non-trusted IP from right, got %q", ip)
	}
}

func TestGetClientIP_UntrustedRemote(t *testing.T) {
	config := &TrustedProxyConfig{
		TrustedProxies: []string{"10.0.0.0/8"},
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.99:12345" // Not a trusted proxy
	req.Header.Set("X-Forwarded-For", "evil.injected.ip")

	ip := GetClientIP(req, config)
	if ip != "203.0.113.99" {
		t.Errorf("expected RemoteAddr (untrusted proxy), got %q", ip)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
