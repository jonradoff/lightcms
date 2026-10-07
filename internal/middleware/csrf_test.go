package middleware

import (
	"crypto/tls"
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
	if cookie.Path != "/cm" || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("CSRF cookie path=%q samesite=%v, want /cm and Strict", cookie.Path, cookie.SameSite)
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
