package middleware

import (
	"log"
	"net/http"

	"github.com/gorilla/csrf"
)

// csrfCookieName is the cookie holding the real CSRF token. It is not
// gorilla's default name: until 7.4.2 the cookie was scoped to /cm, and a
// browser holding that older cookie would keep sending it first, so the
// site-wide cookie the API check needs would never be issued.
const csrfCookieName = "lightcms_csrf"

// CSRFProtect returns the CSRF middleware for the admin routes (/cm).
//
// gorilla/csrf assumes every request arrived over HTTPS when it checks the
// Origin/Referer of an unsafe request, so over plain HTTP each admin POST is
// rejected as cross-origin. When the server is configured without secure
// cookies (secure_cookies=false: local development, no HTTPS) requests are
// marked as plaintext so the origin is compared as http://. With
// secureCookies=true (production, TLS terminated at the proxy) nothing is
// marked and the strict HTTPS checks apply unchanged.
//
// The decision comes from server configuration and the connection itself,
// never from a request header a client could send.
func CSRFProtect(authKey []byte, secureCookies bool) func(http.Handler) http.Handler {
	return csrfProtect(authKey, secureCookies, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
	})
}

// CSRFProtectAPI returns the same check for session-authenticated /api/v1
// and /mcp requests (see APIAuth.SetSessionAuth): the token an admin page was
// rendered with, sent as X-CSRF-Token, validated against the same cookie
// with the same key, plus gorilla's Origin/Referer check. Failures are JSON,
// as every API error is. Build it with the key and secureCookies value given
// to CSRFProtect.
func CSRFProtectAPI(authKey []byte, secureCookies bool) func(http.Handler) http.Handler {
	return csrfProtect(authKey, secureCookies, func(w http.ResponseWriter, r *http.Request) {
		apiJsonError(w, http.StatusForbidden, "Invalid or missing CSRF token: session-authenticated API requests must send the admin page's token as X-CSRF-Token")
	})
}

func csrfProtect(authKey []byte, secureCookies bool, onError http.HandlerFunc) func(http.Handler) http.Handler {
	protect := csrf.Protect(
		authKey,
		csrf.Secure(secureCookies),
		csrf.CookieName(csrfCookieName),
		// Site-wide, so the browser also sends it to /api/v1 and /mcp
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("CSRF validation failed for %s %s: %v", r.Method, r.URL.Path, csrf.FailureReason(r))
			onError(w, r)
		})),
	)

	return func(next http.Handler) http.Handler {
		protected := protect(next)
		if secureCookies {
			return protected
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// A request that really did arrive over TLS keeps the strict checks
			if r.TLS == nil {
				r = csrf.PlaintextHTTPRequest(r)
			}
			protected.ServeHTTP(w, r)
		})
	}
}
