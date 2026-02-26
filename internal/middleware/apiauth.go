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

// APIAuth is middleware that authenticates requests via API key
type APIAuth struct {
	validate APIKeyValidateFunc
}

// NewAPIAuth creates a new API auth middleware
func NewAPIAuth(validate APIKeyValidateFunc) *APIAuth {
	return &APIAuth{validate: validate}
}

// Middleware returns a gorilla/mux compatible middleware function
func (m *APIAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			apiJsonError(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		// Expect "Bearer lc_xxx..."
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			apiJsonError(w, http.StatusUnauthorized, "Invalid Authorization header format (expected: Bearer <api-key>)")
			return
		}

		apiKey := parts[1]
		if err := m.validate(r.Context(), apiKey); err != nil {
			apiJsonError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func apiJsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}
