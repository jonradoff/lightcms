package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/auth"
	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/middleware"
	"github.com/jonradoff/lightcms/v6/internal/models"
)

// ---------------------------------------------------------------------------
// Chat-specific rate limiters (separate from search — Haiku calls cost money)
// ---------------------------------------------------------------------------

var chatPerIPLimiter = struct {
	sync.Mutex
	requests map[string][]time.Time
}{requests: make(map[string][]time.Time)}

var chatGlobalLimiter = struct {
	sync.Mutex
	requests []time.Time
}{requests: make([]time.Time, 0)}

func checkChatRateLimit(r *http.Request, proxyConfig *middleware.TrustedProxyConfig, perIPLimit, globalLimit int) bool {
	ip := middleware.GetClientIP(r, proxyConfig)
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	chatPerIPLimiter.Lock()
	times := chatPerIPLimiter.requests[ip]
	start := 0
	for start < len(times) && times[start].Before(cutoff) {
		start++
	}
	times = times[start:]
	if len(times) >= perIPLimit {
		chatPerIPLimiter.requests[ip] = times
		chatPerIPLimiter.Unlock()
		return true
	}
	chatPerIPLimiter.requests[ip] = append(times, now)
	chatPerIPLimiter.Unlock()

	chatGlobalLimiter.Lock()
	gs := chatGlobalLimiter.requests
	gs2 := gs[:0]
	for _, t := range gs {
		if !t.Before(cutoff) {
			gs2 = append(gs2, t)
		}
	}
	if len(gs2) >= globalLimit {
		chatGlobalLimiter.requests = gs2
		chatGlobalLimiter.Unlock()
		return true
	}
	chatGlobalLimiter.requests = append(gs2, now)
	chatGlobalLimiter.Unlock()
	return false
}

// ---------------------------------------------------------------------------
// SSE helpers
// ---------------------------------------------------------------------------

func sseEvent(w http.ResponseWriter, flusher http.Flusher, v interface{}) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// ---------------------------------------------------------------------------
// Anthropic streaming
// ---------------------------------------------------------------------------

const anthropicURL = "https://api.anthropic.com/v1/messages"
const anthropicModel = "claude-haiku-4-5-20251001"
const chatMaxTokens = 300
const snippetLen = 400 // chars per excerpt sent to Haiku

// streamHaiku calls the Anthropic streaming API and writes token events to w.
// Returns false if the call failed entirely (caller should send a fallback).
func (h *Handler) streamHaiku(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	systemPrompt, userPrompt string,
) bool {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":      anthropicModel,
		"max_tokens": chatMaxTokens,
		"stream":     true,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicURL, bytes.NewReader(reqBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", h.anthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Parse Anthropic SSE stream and forward text delta events
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			break
		}
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			sseEvent(w, flusher, map[string]string{"type": "token", "text": event.Delta.Text})
		}
	}

	return scanner.Err() == nil
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// ChatWidgetPage renders the admin chat widget configuration and test page
func (h *Handler) ChatWidgetPage(w http.ResponseWriter, r *http.Request) {
	if !h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/cm/login", http.StatusSeeOther)
		return
	}

	cfg, _ := h.db.GetChatWidgetConfig(r.Context())
	if cfg == nil {
		cfg = database.DefaultChatWidgetConfig()
	}
	// Ensure prompt fields are populated for display even if DB has empty strings
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = database.DefaultChatSystemPrompt
	}
	if cfg.UserPromptTemplate == "" {
		cfg.UserPromptTemplate = database.DefaultChatUserPromptTemplate
	}

	data := map[string]interface{}{
		"ChatConfig":      cfg,
		"BaseURL":         h.baseURL,
		"SemanticEnabled": h.searchService.HasVoyageKey(),
		"HaikuEnabled":    h.anthropicAPIKey != "",
		"Saved":           r.URL.Query().Get("saved") == "1",
	}

	h.renderAdmin(w, r, "chat_widget_tool", data)
}

// ChatWidgetSaveConfig saves the chat widget configuration
func (h *Handler) ChatWidgetSaveConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	parseInt := func(key string, def, min, max int) int {
		v, err := strconv.Atoi(r.FormValue(key))
		if err != nil || v < min || v > max {
			return def
		}
		return v
	}

	position := r.FormValue("position")
	if position != "bottom-left" {
		position = "bottom-right"
	}

	primaryColor := strings.TrimSpace(r.FormValue("primary_color"))
	if primaryColor == "" || !strings.HasPrefix(primaryColor, "#") {
		primaryColor = "#6366f1"
	}

	systemPrompt := strings.TrimSpace(r.FormValue("system_prompt"))
	userPromptTemplate := strings.TrimSpace(r.FormValue("user_prompt_template"))

	// Validate that prompts only reference known placeholders.
	// Unknown placeholders could accidentally expose config values if new ones are added later.
	if err := validatePromptPlaceholders(systemPrompt, userPromptTemplate); err != nil {
		http.Error(w, "Invalid prompt template: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg := &database.ChatWidgetConfig{
		Enabled:            r.FormValue("enabled") == "1",
		WidgetTitle:        strings.TrimSpace(r.FormValue("widget_title")),
		WelcomeMessage:     strings.TrimSpace(r.FormValue("welcome_message")),
		Placeholder:        strings.TrimSpace(r.FormValue("placeholder")),
		PrimaryColor:       primaryColor,
		Position:           position,
		MaxResults:         parseInt("max_results", 3, 1, 10),
		RateLimitPerIP:     parseInt("rate_limit_per_ip", 5, 1, 60),
		RateLimitGlobal:    parseInt("rate_limit_global", 30, 1, 300),
		SystemPrompt:       systemPrompt,
		UserPromptTemplate: userPromptTemplate,
	}

	if cfg.WidgetTitle == "" {
		cfg.WidgetTitle = "Chat with Site"
	}
	if cfg.WelcomeMessage == "" {
		cfg.WelcomeMessage = "Hi! Ask me anything and I'll find the most relevant pages for you."
	}
	if cfg.Placeholder == "" {
		cfg.Placeholder = "Ask a question..."
	}

	if err := h.db.SaveChatWidgetConfig(r.Context(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/cm/tools/chat?saved=1", http.StatusSeeOther)
}

// ChatWidgetConfigPublic returns the public widget configuration for the JS widget.
func (h *Handler) ChatWidgetConfigPublic(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.db.GetChatWidgetConfig(r.Context())
	if err != nil {
		cfg = database.DefaultChatWidgetConfig()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":         cfg.Enabled,
		"title":           cfg.WidgetTitle,
		"welcome_message": cfg.WelcomeMessage,
		"placeholder":     cfg.Placeholder,
		"primary_color":   cfg.PrimaryColor,
		"position":        cfg.Position,
		"ai_enabled":      h.anthropicAPIKey != "",
	})
}

// allowedPromptPlaceholders are the only {placeholder} tokens permitted in saved
// system/user prompt templates. Unknown tokens could expose config values if
// template substitution is extended in the future.
var allowedPromptPlaceholders = map[string]bool{
	"{siteName}": true,
	"{question}": true,
	"{excerpts}": true,
}

var promptPlaceholderRe = regexp.MustCompile(`\{[^}]+\}`)

// validatePromptPlaceholders returns an error if either prompt contains a
// placeholder that is not in the allowed set.
func validatePromptPlaceholders(prompts ...string) error {
	for _, p := range prompts {
		for _, match := range promptPlaceholderRe.FindAllString(p, -1) {
			if !allowedPromptPlaceholders[match] {
				return fmt.Errorf("unknown placeholder %q — allowed: {siteName}, {question}, {excerpts}", match)
			}
		}
	}
	return nil
}

// chatOrigin returns the allowed CORS origin for chat endpoints: the site's
// base URL if configured, otherwise "*" as a fallback.
func (h *Handler) chatOrigin() string {
	if h.baseURL != "" {
		return h.baseURL
	}
	return "*"
}

// corsOK writes CORS preflight headers. Origin is restricted to the site's own
// base URL to prevent cross-origin abuse; falls back to "*" if baseURL is unset.
func (h *Handler) corsOK(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

// ChatWidgetQuery is the public chat endpoint. Always responds as SSE.
//
// Event types sent to the client:
//
//	{"type":"token",   "text":"..."}          — streamed Haiku tokens (when AI configured)
//	{"type":"sources", "results":[...]}        — retrieved page links (always sent)
//	{"type":"done"}                            — stream complete
//	{"type":"error",   "message":"..."}        — non-fatal error (sources still follow)
func (h *Handler) ChatWidgetQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		h.corsOK(w)
		return
	}

	cfg, err := h.db.GetChatWidgetConfig(r.Context())
	if err != nil {
		cfg = database.DefaultChatWidgetConfig()
	}

	// Apply chat-specific rate limits (stricter than search — Haiku has a cost)
	perIP := cfg.RateLimitPerIP
	if perIP <= 0 {
		perIP = 5
	}
	global := cfg.RateLimitGlobal
	if global <= 0 {
		global = 30
	}

	if checkChatRateLimit(r, h.proxyConfig, perIP, global) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded, try again later"})
		return
	}

	// When disabled, only allow authenticated admins to test
	if !cfg.Enabled && !h.auth.IsAuthenticated(r) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "chat widget is not enabled"})
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "q parameter is required"})
		return
	}
	if len(query) > 500 {
		query = query[:500]
	}

	// Log the chat query to the audit log
	if h.auditService != nil {
		ip := middleware.GetClientIP(r, h.proxyConfig)
		entry := models.AuditLog{
			UserEmail: "public",
			Action:    "chat.query",
			Resource:  "chat",
			Details:   map[string]interface{}{"query": query},
			IPAddress: ip,
		}
		h.auditService.Log(r.Context(), entry)
	}

	// --- Phase 1: hybrid search ---
	limit := cfg.MaxResults
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	results, err := h.searchService.Search(r.Context(), query, "hybrid", limit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "search failed"})
		return
	}

	// Build source list for the SSE sources event
	type chatResult struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Excerpt string `json:"excerpt"`
	}
	sources := make([]chatResult, 0, len(results))
	for _, res := range results {
		sources = append(sources, chatResult{
			Title:   res.Title,
			URL:     res.FullPath,
			Excerpt: res.Snippet,
		})
	}

	// --- Switch to SSE ---
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback: plain JSON if streaming not supported
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
		json.NewEncoder(w).Encode(map[string]interface{}{"query": query, "results": sources})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", h.chatOrigin())
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering if present

	// --- Phase 2: Haiku synthesis (if configured and results exist) ---
	if h.anthropicAPIKey != "" && len(results) > 0 {
		// Get site name for {siteName} substitution
		siteName := "this site"
		if theme, err := h.db.GetThemeSettings(r.Context()); err == nil && theme != nil && theme.SiteName != "" {
			siteName = theme.SiteName
		}

		// Use stored prompts (fall back to defaults if blank)
		systemPromptTpl := cfg.SystemPrompt
		if systemPromptTpl == "" {
			systemPromptTpl = database.DefaultChatSystemPrompt
		}
		userPromptTpl := cfg.UserPromptTemplate
		if userPromptTpl == "" {
			userPromptTpl = database.DefaultChatUserPromptTemplate
		}

		// Build numbered excerpts block
		var excerptsBuilder strings.Builder
		for i, res := range results {
			excerpt := res.Snippet
			if len(excerpt) > snippetLen {
				excerpt = excerpt[:snippetLen]
			}
			fmt.Fprintf(&excerptsBuilder, "[%d] \"%s\" (%s):\n%s\n\n", i+1, res.Title, res.FullPath, excerpt)
		}

		// Substitute placeholders.
		// The user query is wrapped in XML delimiters to reduce prompt injection risk:
		// a user cannot break out of the delimiters to override the system prompt.
		safeQuery := strings.ReplaceAll(query, "</", "<\\/") // prevent closing XML tags in query
		systemPrompt := strings.ReplaceAll(systemPromptTpl, "{siteName}", siteName)
		userPrompt := strings.ReplaceAll(userPromptTpl, "{siteName}", siteName)
		userPrompt = strings.ReplaceAll(userPrompt, "{excerpts}", excerptsBuilder.String())
		userPrompt = strings.ReplaceAll(userPrompt, "{question}", "<user_question>"+safeQuery+"</user_question>")

		// Stream Haiku — if it fails, fall through to just sending sources
		callCtx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		ok := h.streamHaiku(callCtx, w, flusher, systemPrompt, userPrompt)
		cancel()

		if !ok {
			sseEvent(w, flusher, map[string]string{"type": "error", "message": "AI synthesis unavailable"})
		}
	}

	// Always send sources (whether or not Haiku ran)
	sseEvent(w, flusher, map[string]interface{}{"type": "sources", "results": sources})
	sseEvent(w, flusher, map[string]string{"type": "done"})
}
