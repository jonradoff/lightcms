package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const (
	// apiRateWindow is the sliding window for per-key rate limiting.
	apiRateWindow = time.Minute
	// apiRateLimit is the maximum number of API v1 requests per key per minute.
	// This is intentionally generous — it targets runaway scripts, not normal usage.
	apiRateLimit = 300
)

// apiKeyRateLimiter tracks per-token request timestamps.
var apiKeyRateLimiter = struct {
	sync.Mutex
	tokens map[string][]time.Time
}{
	tokens: make(map[string][]time.Time),
}

// APIRateLimit is middleware that enforces a per-bearer-token sliding-window rate limit
// on the /api/v1/ subrouter. It runs after authentication so the token is already
// validated; unauthenticated requests are rejected by the auth middleware first.
func APIRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the raw bearer token to use as the rate-limit key.
		// We use the token directly (already validated by auth middleware).
		token := ""
		if auth := r.Header.Get("Authorization"); len(auth) > 7 {
			token = auth[7:] // strip "Bearer "
		}
		if token == "" {
			// No token — auth middleware would have blocked this, but be safe.
			next.ServeHTTP(w, r)
			return
		}

		apiKeyRateLimiter.Lock()
		now := time.Now()
		cutoff := now.Add(-apiRateWindow)

		times := apiKeyRateLimiter.tokens[token]
		// Prune entries outside the window
		start := 0
		for start < len(times) && times[start].Before(cutoff) {
			start++
		}
		times = times[start:]

		limited := len(times) >= apiRateLimit
		if !limited {
			apiKeyRateLimiter.tokens[token] = append(times, now)
		} else {
			apiKeyRateLimiter.tokens[token] = times
		}
		apiKeyRateLimiter.Unlock()

		if limited {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "API rate limit exceeded (300 requests/minute). Please slow down.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
