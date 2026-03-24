package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/database"
	"lightcms/internal/middleware"
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

// checkSearchRateLimit returns true if the request should be rate-limited (per-IP).
// proxyConfig controls how the client IP is extracted from forwarded headers,
// preventing IP spoofing via forged X-Forwarded-For values.
func checkSearchRateLimit(r *http.Request, proxyConfig *middleware.TrustedProxyConfig) bool {
	ip := middleware.GetClientIP(r, proxyConfig)

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
	if checkSearchRateLimit(r, h.proxyConfig) {
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
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query":   query,
		"mode":    mode,
		"results": results,
		"total":   len(results),
	})
}

// EndUserSearchSuggest handles the public typeahead suggest endpoint (no auth required)
func (h *Handler) EndUserSearchSuggest(w http.ResponseWriter, r *http.Request) {
	// Share rate limiting with search
	if checkGlobalSearchRateLimit() {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "service temporarily overloaded, try again later"})
		return
	}
	if checkSearchRateLimit(r, h.proxyConfig) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded, try again later"})
		return
	}

	prefix := r.URL.Query().Get("q")
	if prefix == "" || len(prefix) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keywords": []string{},
			"pages":    []interface{}{},
		})
		return
	}

	if len(prefix) > 100 {
		prefix = prefix[:100]
	}

	limit := 8
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 20 {
			limit = parsed
		}
	}

	result, err := h.searchService.Suggest(r.Context(), prefix, limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(result)
}

// SearchToolPage renders the admin search tool page
func (h *Handler) SearchToolPage(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	searchCfg, _ := h.db.GetSearchConfig(r.Context())
	if searchCfg == nil {
		searchCfg = database.DefaultSearchConfig()
	}

	data := map[string]interface{}{
		"SearchEnabled":       true,
		"SemanticEnabled":     h.searchService.HasVoyageKey(),
		"BaseURL":             h.baseURL,
		"SearchRankingConfig": searchCfg,
		"SavedConfig":         r.URL.Query().Get("saved") == "1",
	}

	total, withEmbedding, err := h.searchService.EmbeddingStats(r.Context())
	if err == nil {
		data["TotalContent"] = total
		data["WithEmbedding"] = withEmbedding
	}

	h.renderAdmin(w, r, "search_tool", data)
}

// SearchToolSaveConfig saves search ranking configuration (admin only)
func (h *Handler) SearchToolSaveConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	parseFloat := func(key string, fallback float64) float64 {
		v, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue(key)), 64)
		if err != nil {
			return fallback
		}
		return v
	}
	parseLines := func(key string) []string {
		raw := strings.TrimSpace(r.FormValue(key))
		if raw == "" {
			return nil
		}
		var out []string
		for _, line := range strings.Split(raw, "\n") {
			if s := strings.TrimSpace(line); s != "" {
				out = append(out, s)
			}
		}
		return out
	}

	def := database.DefaultSearchConfig()
	cfg := &database.SearchConfig{
		NavBoost:           clampFloat(parseFloat("nav_boost", def.NavBoost), -1, 1),
		TitleBoost:         clampFloat(parseFloat("title_boost", def.TitleBoost), 0, 1),
		BoostTemplates:     parseLines("boost_templates"),
		BoostTemplateScore: clampFloat(parseFloat("boost_template_score", def.BoostTemplateScore), -1, 1),
		BoostPaths:         parseLines("boost_paths"),
		BoostPathScore:     clampFloat(parseFloat("boost_path_score", def.BoostPathScore), 0, 1),
		DemotePaths:        parseLines("demote_paths"),
		DemotePathPrefixes: parseLines("demote_path_prefixes"),
		DemoteScore:        clampFloat(parseFloat("demote_score", def.DemoteScore), -1, 1),
	}

	if err := h.db.SaveSearchConfig(r.Context(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	// Invalidate service cache so changes take effect immediately
	h.searchService.InvalidateSearchConfigCache()

	http.Redirect(w, r, "/cm/tools/search?saved=1", http.StatusSeeOther)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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
