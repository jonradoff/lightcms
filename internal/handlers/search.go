package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// searchRateLimiter tracks per-IP request counts for the public search endpoint
var searchRateLimiter = struct {
	sync.Mutex
	requests map[string][]time.Time
}{
	requests: make(map[string][]time.Time),
}

// globalSearchRateLimiter tracks total request count across all IPs for DDoS protection
var globalSearchRateLimiter = struct {
	sync.Mutex
	requests []time.Time
}{
	requests: make([]time.Time, 0),
}

const (
	searchRateWindow      = time.Minute
	searchRateLimit       = 30  // max requests per IP per minute
	globalSearchRateLimit = 300 // max total requests per minute across all IPs
)

// checkGlobalSearchRateLimit returns true if total search volume exceeds the global limit
func checkGlobalSearchRateLimit() bool {
	globalSearchRateLimiter.Lock()
	defer globalSearchRateLimiter.Unlock()

	now := time.Now()
	cutoff := now.Add(-searchRateWindow)

	// Prune old entries
	start := 0
	for start < len(globalSearchRateLimiter.requests) && globalSearchRateLimiter.requests[start].Before(cutoff) {
		start++
	}
	globalSearchRateLimiter.requests = globalSearchRateLimiter.requests[start:]

	if len(globalSearchRateLimiter.requests) >= globalSearchRateLimit {
		return true
	}

	globalSearchRateLimiter.requests = append(globalSearchRateLimiter.requests, now)
	return false
}

// checkSearchRateLimit returns true if the request should be rate-limited (per-IP)
func checkSearchRateLimit(r *http.Request) bool {
	ip := extractIP(r)

	searchRateLimiter.Lock()
	defer searchRateLimiter.Unlock()

	now := time.Now()
	cutoff := now.Add(-searchRateWindow)

	// Prune old entries
	times := searchRateLimiter.requests[ip]
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]

	if len(times) >= searchRateLimit {
		searchRateLimiter.requests[ip] = times
		return true
	}

	searchRateLimiter.requests[ip] = append(times, now)
	return false
}

// extractIP gets the client IP from X-Forwarded-For or RemoteAddr
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// EndUserSearch handles the public search API endpoint (no auth required)
func (h *Handler) EndUserSearch(w http.ResponseWriter, r *http.Request) {
	// Global rate limit (DDoS protection)
	if checkGlobalSearchRateLimit() {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "service temporarily overloaded, try again later"})
		return
	}

	// Per-IP rate limit
	if checkSearchRateLimit(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded, try again later"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "q parameter is required"})
		return
	}

	// Limit query length to prevent abuse
	if len(query) > 500 {
		query = query[:500]
	}

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "hybrid"
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	results, err := h.searchService.Search(r.Context(), query, mode, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"mode":    mode,
		"results": results,
		"total":   len(results),
	})
}

// SearchToolPage renders the admin search tool page
func (h *Handler) SearchToolPage(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"SearchEnabled":    true,
		"SemanticEnabled":  h.searchService.HasVoyageKey(),
		"BaseURL":          h.baseURL,
	}

	total, withEmbedding, err := h.searchService.EmbeddingStats(r.Context())
	if err == nil {
		data["TotalContent"] = total
		data["WithEmbedding"] = withEmbedding
	}

	h.renderAdmin(w, r, "search_tool", data)
}

// SearchToolTest handles the admin AJAX search test endpoint
func (h *Handler) SearchToolTest(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "hybrid"
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results, err := h.searchService.Search(r.Context(), query, mode, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"mode":    mode,
		"results": results,
		"total":   len(results),
	})
}

// SearchToolReindex triggers batch embedding generation
func (h *Handler) SearchToolReindex(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.searchService.HasVoyageKey() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "semantic search not configured — set VOYAGE_API_KEY to enable embeddings"})
		return
	}

	processed, errCount, err := h.searchService.BatchGenerateEmbeddings(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed": processed,
		"errors":    errCount,
		"message":   "Reindex complete",
	})
}
