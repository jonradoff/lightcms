package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// APIKeyValidateFunc is a function that validates an API key.
// Used to avoid import cycle between middleware and services packages.
type APIKeyValidateFunc func(ctx context.Context, rawKey string) error

// OAuthValidateFunc validates an OAuth access token.
type OAuthValidateFunc func(ctx context.Context, rawToken string) error

// APIAuth is middleware that authenticates requests via API key or OAuth token
type APIAuth struct {
	validate            APIKeyValidateFunc
	validateOAuth       OAuthValidateFunc
	systemAPIKey        string // internal API key substituted for OAuth-authenticated requests
	resourceMetadataURL string // for WWW-Authenticate header (OAuth discovery)
}

// NewAPIAuth creates a new API auth middleware
func NewAPIAuth(validate APIKeyValidateFunc) *APIAuth {
	return &APIAuth{validate: validate}
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
			if err := m.validate(r.Context(), token); err != nil {
				m.setWWWAuthenticate(w)
				apiJsonError(w, http.StatusUnauthorized, "Invalid API key")
				return
			}
			next.ServeHTTP(w, r)
		} else if m.validateOAuth != nil {
			if err := m.validateOAuth(r.Context(), token); err != nil {
				m.setWWWAuthenticate(w)
				apiJsonError(w, http.StatusUnauthorized, "Invalid access token")
				return
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
