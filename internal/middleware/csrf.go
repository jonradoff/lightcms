package middleware

import (
	"log"
	"net/http"

	"github.com/gorilla/csrf"
)

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
	protect := csrf.Protect(
		authKey,
		csrf.Secure(secureCookies),
		csrf.Path("/cm"),
		csrf.SameSite(csrf.SameSiteStrictMode),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("CSRF validation failed for %s %s: %v", r.Method, r.URL.Path, csrf.FailureReason(r))
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
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
