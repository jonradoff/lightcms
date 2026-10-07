package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// contextKey is an unexported type for context keys in this package
type contextKey int

const (
	// apiUserContextKey stores the authenticated API user in context
	apiUserContextKey contextKey = iota
)

// APIKeyValidateFunc validates an API key and returns the authenticated user (as interface{}).
// The returned value will be stored in the request context and can be retrieved with APIUserFromContext.
type APIKeyValidateFunc func(ctx context.Context, rawKey string) (interface{}, error)

// OAuthValidateFunc validates an OAuth access token and returns the authenticated user (as interface{}).
type OAuthValidateFunc func(ctx context.Context, rawToken string) (interface{}, error)

// APIUserFromContext extracts the authenticated API user from the request context.
// Callers should type-assert the result to the expected user type.
func APIUserFromContext(ctx context.Context) (interface{}, bool) {
	user := ctx.Value(apiUserContextKey)
	return user, user != nil
}

// InjectAPIUser returns a context with the given user injected as the API user.
// Intended for use in tests only.
func InjectAPIUser(ctx context.Context, user interface{}) context.Context {
	return context.WithValue(ctx, apiUserContextKey, user)
}

// SessionValidateFunc validates a session cookie and returns the authenticated user (as interface{}).
// It receives the full HTTP request so it can read cookies.
type SessionValidateFunc func(r *http.Request) interface{}

// APIAuth is middleware that authenticates requests via API key or OAuth token
type APIAuth struct {
	validate            APIKeyValidateFunc
	validateOAuth       OAuthValidateFunc
	validateSession     SessionValidateFunc
	sessionCSRF         func(http.Handler) http.Handler // CSRF check for unsafe session-authenticated requests
	systemAPIKey        string                          // internal API key substituted for OAuth-authenticated requests
	resourceMetadataURL string                          // for WWW-Authenticate header (OAuth discovery)
}

// NewAPIAuth creates a new API auth middleware
func NewAPIAuth(validate APIKeyValidateFunc) *APIAuth {
	return &APIAuth{validate: validate}
}

// SetSessionAuth enables session cookie fallback for browser-based admin UI calls.
// When no Authorization header is present, the session validator is tried as a fallback.
//
// A session cookie is sent by the browser on its own, so a session-authenticated
// request with an unsafe method (anything but GET, HEAD, OPTIONS, TRACE) must
// also pass csrfProtect — middleware.CSRFProtectAPI in the server. With a nil
// csrfProtect those requests are refused outright. Requests authenticated by
// an API key or OAuth bearer token never reach the check: a browser does not
// attach an Authorization header by itself.
func (m *APIAuth) SetSessionAuth(validateSession SessionValidateFunc, csrfProtect func(http.Handler) http.Handler) {
	m.validateSession = validateSession
	m.sessionCSRF = csrfProtect
}

// isSafeMethod reports whether the method is read-only under RFC 7231 (the
// set gorilla/csrf exempts).
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// SetOAuth enables OAuth token validation alongside API key validation.
// systemAPIKey is injected into the Authorization header for OAuth-authenticated
// requests so the downstream MCP handler can use it for internal REST API calls.
func (m *APIAuth) SetOAuth(validateOAuth OAuthValidateFunc, systemAPIKey, resourceMetadataURL string) {
	m.validateOAuth = validateOAuth
	m.systemAPIKey = systemAPIKey
	m.resourceMetadataURL = resourceMetadataURL
}

// Middleware returns a gorilla/mux compatible middleware function
func (m *APIAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No Authorization header — try session cookie fallback for browser-based admin UI calls
			if m.validateSession != nil {
				if user := m.validateSession(r); user != nil {
					r = r.WithContext(context.WithValue(r.Context(), apiUserContextKey, user))
					if isSafeMethod(r.Method) {
						next.ServeHTTP(w, r)
						return
					}
					if m.sessionCSRF == nil {
						apiJsonError(w, http.StatusForbidden, "Session-authenticated API requests that change state are not enabled")
						return
					}
					m.sessionCSRF(next).ServeHTTP(w, r)
					return
				}
			}
			m.setWWWAuthenticate(w)
			apiJsonError(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			m.setWWWAuthenticate(w)
			apiJsonError(w, http.StatusUnauthorized, "Invalid Authorization header format (expected: Bearer <token>)")
			return
		}

		token := parts[1]

		// Route based on token prefix: lc_ = API key, anything else = OAuth token
		if strings.HasPrefix(token, "lc_") {
			user, err := m.validate(r.Context(), token)
			if err != nil {
				m.setWWWAuthenticate(w)
				apiJsonError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}
			// Inject user into request context
			if user != nil {
				r = r.WithContext(context.WithValue(r.Context(), apiUserContextKey, user))
			}
			next.ServeHTTP(w, r)
		} else if m.validateOAuth != nil {
			user, err := m.validateOAuth(r.Context(), token)
			if err != nil {
				m.setWWWAuthenticate(w)
				apiJsonError(w, http.StatusUnauthorized, "Invalid access token")
				return
			}
			// Inject user into request context
			if user != nil {
				r = r.WithContext(context.WithValue(r.Context(), apiUserContextKey, user))
			}
			// Replace Authorization header with system API key so downstream
			// handlers (MCP http_handler, REST API) work correctly
			r.Header.Set("Authorization", "Bearer "+m.systemAPIKey)
			next.ServeHTTP(w, r)
		} else {
			m.setWWWAuthenticate(w)
			apiJsonError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}
	})
}

// setWWWAuthenticate adds the WWW-Authenticate header for OAuth discovery
func (m *APIAuth) setWWWAuthenticate(w http.ResponseWriter) {
	if m.resourceMetadataURL != "" {
		w.Header().Set("WWW-Authenticate",
			`Bearer resource_metadata="`+m.resourceMetadataURL+`"`)
	}
}

func apiJsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}
