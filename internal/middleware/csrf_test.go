package middleware

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/csrf"
)

var csrfTestKey = []byte("32-byte-long-test-csrf-key!!1234")

// csrfTestHandler serves the token on GET and 200 on anything that passes.
func csrfTestHandler(secureCookies bool) http.Handler {
	return CSRFProtect(csrfTestKey, secureCookies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(csrf.Token(r)))
	}))
}

// csrfTokenFor does the GET that issues the CSRF cookie and masked token.
func csrfTokenFor(t *testing.T, h http.Handler, base string) (*http.Cookie, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, base+"/cm/login", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET token: status %d", rr.Code)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("GET token: got %d cookies, want 1", len(cookies))
	}
	return cookies[0], rr.Body.String()
}

type csrfPost struct {
	name    string
	token   string // "" = no token sent
	origin  string
	referer string
	header  [2]string // extra header a client might forge
	tls     bool      // request arrived over TLS on this connection
	want    int
}

func runCSRFPosts(t *testing.T, h http.Handler, base string, cookie *http.Cookie, posts []csrfPost) {
	t.Helper()
	for _, p := range posts {
		req := httptest.NewRequest(http.MethodPost, base+"/cm/content", strings.NewReader(""))
		req.AddCookie(cookie)
		if p.token != "" {
			req.Header.Set("X-CSRF-Token", p.token)
		}
		if p.origin != "" {
			req.Header.Set("Origin", p.origin)
		}
		if p.referer != "" {
			req.Header.Set("Referer", p.referer)
		}
		if p.header[0] != "" {
			req.Header.Set(p.header[0], p.header[1])
		}
		if p.tls {
			req.TLS = &tls.ConnectionState{}
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != p.want {
			t.Errorf("%s: status %d, want %d", p.name, rr.Code, p.want)
		}
	}
}

// secure_cookies=false (local development over plain HTTP): a same-origin
// http POST with a valid token passes; everything else is still rejected.
func TestCSRFProtect_PlaintextMode(t *testing.T) {
	const base = "http://localhost:8082"
	h := csrfTestHandler(false)
	cookie, token := csrfTokenFor(t, h, base)
	if cookie.Secure {
		t.Error("CSRF cookie is Secure with secure_cookies=false")
	}

	runCSRFPosts(t, h, base, cookie, []csrfPost{
		{name: "same-origin http, valid token", token: token, origin: base, want: http.StatusOK},
		{name: "no Origin, same-origin Referer, valid token", token: token, referer: base + "/cm/content/new", want: http.StatusOK},
		{name: "missing token", origin: base, want: http.StatusForbidden},
		{name: "invalid token", token: "bm90LWEtcmVhbC10b2tlbg==", origin: base, want: http.StatusForbidden},
		{name: "cross-origin", token: token, origin: "http://evil.example", want: http.StatusForbidden},
		{name: "same host, other port", token: token, origin: "http://localhost:9999", want: http.StatusForbidden},
		{name: "same host over https", token: token, origin: "https://localhost:8082", want: http.StatusForbidden},
		// A request that really arrived over TLS is not marked plaintext
		{name: "TLS connection, http Origin", token: token, origin: base, tls: true, want: http.StatusForbidden},
		{name: "TLS connection, https Origin", token: token, origin: "https://localhost:8082", tls: true, want: http.StatusOK},
	})
}

// secure_cookies=true (production behind a TLS-terminating proxy): the
// strict HTTPS origin/referer checks are unchanged, and no request header
// can switch them off.
func TestCSRFProtect_SecureMode(t *testing.T) {
	const base = "https://example.com"
	h := csrfTestHandler(true)
	cookie, token := csrfTokenFor(t, h, "http://example.com") // proxy forwards plain HTTP
	if !cookie.Secure {
		t.Error("CSRF cookie is not Secure with secure_cookies=true")
	}
	// Site-wide and under its own name, so the browser also sends it to
	// /api/v1 and a pre-7.4.2 /cm-scoped cookie cannot shadow it.
	if cookie.Path != "/" || cookie.Name != csrfCookieName || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("CSRF cookie name=%q path=%q samesite=%v, want %s, / and Strict", cookie.Name, cookie.Path, cookie.SameSite, csrfCookieName)
	}

	runCSRFPosts(t, h, "http://example.com", cookie, []csrfPost{
		{name: "https Origin, valid token", token: token, origin: base, want: http.StatusOK},
		{name: "no Origin, https Referer, valid token", token: token, referer: base + "/cm/content/new", want: http.StatusOK},
		{name: "http Origin, valid token", token: token, origin: "http://example.com", want: http.StatusForbidden},
		{name: "http Origin with forged X-Forwarded-Proto", token: token, origin: "http://example.com", header: [2]string{"X-Forwarded-Proto", "http"}, want: http.StatusForbidden},
		{name: "no Origin, http Referer", token: token, referer: "http://example.com/cm", want: http.StatusForbidden},
		{name: "no Origin, no Referer", token: token, want: http.StatusForbidden},
		{name: "cross-origin", token: token, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "missing token", origin: base, want: http.StatusForbidden},
		{name: "invalid token", token: "bm90LWEtcmVhbC10b2tlbg==", origin: base, want: http.StatusForbidden},
	})
}

// Safe methods never need a token in either mode.
func TestCSRFProtect_SafeMethods(t *testing.T) {
	for _, secure := range []bool{false, true} {
		h := csrfTestHandler(secure)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "http://localhost:8082/cm", nil))
		if rr.Code != http.StatusOK {
			t.Errorf("secure=%v GET: status %d, want 200", secure, rr.Code)
		}
	}
}

// ---- Session-authenticated /api/v1 requests (APIAuth.SetSessionAuth) ----

// apiSessionStack is the server's wiring in miniature: the /cm pages behind
// CSRFProtect issue the cookie and token, and /api/v1 sits behind APIAuth
// with the session fallback and CSRFProtectAPI. A request is "signed in"
// when it carries the session cookie; "lc_good" is the one valid API key.
func apiSessionStack(secureCookies bool) (cm http.Handler, api http.Handler, reached *int) {
	cm = CSRFProtect(csrfTestKey, secureCookies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(csrf.Token(r)))
	}))
	auth := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) {
		if rawKey == "lc_good" {
			return "key-user", nil
		}
		return nil, errors.New("bad key")
	})
	auth.SetSessionAuth(func(r *http.Request) interface{} {
		if _, err := r.Cookie("session"); err == nil {
			return "session-user"
		}
		return nil
	}, CSRFProtectAPI(csrfTestKey, secureCookies))
	n := 0
	api = auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusOK)
	}))
	return cm, api, &n
}

type apiCall struct {
	name    string
	method  string
	session bool   // carries the session cookie
	csrf    bool   // carries the CSRF cookie the /cm page set
	token   string // X-CSRF-Token
	bearer  string // Authorization: Bearer ...
	origin  string
	referer string
	header  [2]string
	want    int
}

func runAPICalls(t *testing.T, api http.Handler, reached *int, base string, csrfCookie *http.Cookie, calls []apiCall) {
	t.Helper()
	for _, c := range calls {
		req := httptest.NewRequest(c.method, base+"/api/v1/content/abc/comments", strings.NewReader(`{"text":"hi"}`))
		req.Header.Set("Content-Type", "application/json")
		if c.session {
			req.AddCookie(&http.Cookie{Name: "session", Value: "s"})
		}
		if c.csrf {
			req.AddCookie(csrfCookie)
		}
		if c.token != "" {
			req.Header.Set("X-CSRF-Token", c.token)
		}
		if c.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+c.bearer)
		}
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		if c.referer != "" {
			req.Header.Set("Referer", c.referer)
		}
		if c.header[0] != "" {
			req.Header.Set(c.header[0], c.header[1])
		}
		before := *reached
		rr := httptest.NewRecorder()
		api.ServeHTTP(rr, req)
		if rr.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rr.Code, c.want, strings.TrimSpace(rr.Body.String()))
		}
		if got := *reached > before; got != (c.want == http.StatusOK) {
			t.Errorf("%s: handler reached = %v with status %d", c.name, got, rr.Code)
		}
		if rr.Code == http.StatusForbidden && !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
			t.Errorf("%s: 403 is not JSON (%q)", c.name, rr.Header().Get("Content-Type"))
		}
	}
}

// Plain-HTTP local development: the token an admin page was rendered with
// authorises that session's unsafe API calls; nothing else does.
func TestAPISessionCSRF_PlaintextMode(t *testing.T) {
	const base = "http://localhost:8082"
	cm, api, reached := apiSessionStack(false)
	cookie, token := csrfTokenFor(t, cm, base)

	runAPICalls(t, api, reached, base, cookie, []apiCall{
		{name: "session POST without token", method: "POST", session: true, csrf: true, origin: base, want: http.StatusForbidden},
		{name: "session POST with token", method: "POST", session: true, csrf: true, token: token, origin: base, want: http.StatusOK},
		{name: "session PUT with token", method: "PUT", session: true, csrf: true, token: token, origin: base, want: http.StatusOK},
		{name: "session DELETE without token", method: "DELETE", session: true, csrf: true, origin: base, want: http.StatusForbidden},
		{name: "session DELETE with token", method: "DELETE", session: true, csrf: true, token: token, origin: base, want: http.StatusOK},
		{name: "session PATCH without token", method: "PATCH", session: true, csrf: true, origin: base, want: http.StatusForbidden},
		{name: "session POST, token but no CSRF cookie", method: "POST", session: true, token: token, origin: base, want: http.StatusForbidden},
		{name: "session POST, invalid token", method: "POST", session: true, csrf: true, token: "bm90LWEtcmVhbC10b2tlbg==", origin: base, want: http.StatusForbidden},
		{name: "session POST, cross-origin with token", method: "POST", session: true, csrf: true, token: token, origin: "http://evil.example", want: http.StatusForbidden},
		{name: "session POST, same host other port with token", method: "POST", session: true, csrf: true, token: token, origin: "http://localhost:9999", want: http.StatusForbidden},
		{name: "session GET without token", method: "GET", session: true, want: http.StatusOK},
		{name: "session HEAD without token", method: "HEAD", session: true, want: http.StatusOK},
		{name: "API key POST, no token", method: "POST", bearer: "lc_good", want: http.StatusOK},
		{name: "API key POST, no token, foreign Origin", method: "POST", bearer: "lc_good", origin: "http://elsewhere.example", want: http.StatusOK},
		{name: "API key DELETE with a session cookie too", method: "DELETE", bearer: "lc_good", session: true, want: http.StatusOK},
		{name: "bad API key", method: "POST", bearer: "lc_bad", session: true, csrf: true, token: token, origin: base, want: http.StatusUnauthorized},
		{name: "no session, no key", method: "POST", csrf: true, token: token, origin: base, want: http.StatusUnauthorized},
	})
}

// Production (secure cookies, TLS at the proxy): the same rules with
// gorilla's strict https Origin/Referer checks, and no header to turn them off.
func TestAPISessionCSRF_SecureMode(t *testing.T) {
	const base = "https://example.com"
	cm, api, reached := apiSessionStack(true)
	cookie, token := csrfTokenFor(t, cm, "http://example.com") // proxy forwards plain HTTP

	runAPICalls(t, api, reached, "http://example.com", cookie, []apiCall{
		{name: "session POST without token", method: "POST", session: true, csrf: true, origin: base, want: http.StatusForbidden},
		{name: "session POST with token, https Origin", method: "POST", session: true, csrf: true, token: token, origin: base, want: http.StatusOK},
		{name: "session POST with token, https Referer", method: "POST", session: true, csrf: true, token: token, referer: base + "/cm/approvals", want: http.StatusOK},
		{name: "session POST with token, cross-origin", method: "POST", session: true, csrf: true, token: token, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "session POST with token, http Origin", method: "POST", session: true, csrf: true, token: token, origin: "http://example.com", want: http.StatusForbidden},
		{name: "session POST with token, http Origin, forged X-Forwarded-Proto", method: "POST", session: true, csrf: true, token: token, origin: "http://example.com", header: [2]string{"X-Forwarded-Proto", "http"}, want: http.StatusForbidden},
		{name: "session POST with token, no Origin or Referer", method: "POST", session: true, csrf: true, token: token, want: http.StatusForbidden},
		{name: "session GET without token", method: "GET", session: true, want: http.StatusOK},
		{name: "API key POST, no token, no Origin", method: "POST", bearer: "lc_good", want: http.StatusOK},
		{name: "API key PUT with a session cookie too", method: "PUT", bearer: "lc_good", session: true, want: http.StatusOK},
	})
}

// Without a CSRF check configured, the session fallback is read-only.
func TestAPISessionCSRF_NotConfiguredRefusesWrites(t *testing.T) {
	auth := NewAPIAuth(func(ctx context.Context, rawKey string) (interface{}, error) { return "key-user", nil })
	auth.SetSessionAuth(func(r *http.Request) interface{} { return "session-user" }, nil)
	h := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	for method, want := range map[string]int{"GET": http.StatusOK, "POST": http.StatusForbidden, "DELETE": http.StatusForbidden} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(method, "/api/v1/x", nil))
		if rr.Code != want {
			t.Errorf("%s: status %d, want %d", method, rr.Code, want)
		}
	}
	req := httptest.NewRequest("POST", "/api/v1/x", nil)
	req.Header.Set("Authorization", "Bearer lc_anything")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("API key POST: status %d, want 200", rr.Code)
	}
}
