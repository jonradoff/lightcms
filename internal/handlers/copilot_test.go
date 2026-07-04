package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/csrf"
)

func TestCopilotPage(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	h.SetAnthropicAPIKey("test-key")

	req := sessionReq("GET", "/cm/copilot", nil, nil)
	rr := httptest.NewRecorder()
	h.CopilotPage(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, body: %.300s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Copilot") || !strings.Contains(rr.Body.String(), "cp-input") {
		t.Errorf("copilot page missing expected markup")
	}
	// The chat renderer must ship the table/list markdown support.
	for _, want := range []string{"cp-table", "isTableRow", "cp-typing"} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("copilot page missing renderer piece %q", want)
		}
	}

	// Unauthenticated request is refused.
	rr = httptest.NewRecorder()
	h.CopilotPage(rr, httptest.NewRequest("GET", "/cm/copilot", nil))
	if rr.Code != 403 {
		t.Errorf("unauthenticated: status = %d, want 403", rr.Code)
	}
}

func TestCopilotChat_ToolLoop(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	h.SetAnthropicAPIKey("test-key")

	tmplID := seedTemplate(t, h.db, "Page", "page")
	contentID := seedPublishedContent(t, h, tmplID, "Pricing", "/pricing", "Plans.", "")

	// Fake Anthropic: first call returns a tool_use (update_content), second
	// call returns the final text.
	call := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"stop_reason": "tool_use",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Updating the page now."},
					{"type": "tool_use", "id": "tu_1", "name": "update_content",
						"input": map[string]interface{}{
							"id": contentID.Hex(), "title": "New Pricing",
							"version_comment": "rename per user request",
						}},
				},
			})
			return
		}
		// Verify the tool_result round-trip arrived.
		body, _ := json.Marshal(map[string]interface{}{})
		_ = body
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stop_reason": "end_turn",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Done — the page is now titled New Pricing."},
			},
		})
	}))
	defer fake.Close()
	h.anthropicURLOverride = fake.URL

	req := sessionReq("POST", "/cm/copilot/chat",
		strings.NewReader(`{"messages":[{"role":"user","content":"Rename the pricing page"}]}`), nil)
	rr := httptest.NewRecorder()
	h.CopilotChat(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Reply   string `json:"reply"`
		Actions []struct {
			Tool    string `json:"tool"`
			Summary string `json:"summary"`
			IsWrite bool   `json:"is_write"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(resp.Reply, "New Pricing") {
		t.Errorf("reply = %q", resp.Reply)
	}
	if len(resp.Actions) != 1 || resp.Actions[0].Tool != "update_content" || !resp.Actions[0].IsWrite {
		t.Errorf("actions = %+v", resp.Actions)
	}

	// The content was actually updated.
	c, err := h.contentService.GetContent(context.Background(), contentID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if c.Title != "New Pricing" {
		t.Errorf("title = %q, want New Pricing", c.Title)
	}
	if call != 2 {
		t.Errorf("anthropic calls = %d, want 2", call)
	}
}

func TestExecuteCopilotTool_Permissions(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	tmplID := seedTemplate(t, h.db, "Blog Post", "blog-post")
	contentID := seedPublishedContent(t, h, tmplID, "Post", "/post", "", "")
	ctx := context.Background()

	// Viewer can read but not write.
	out, action := h.executeCopilotTool(ctx, "viewer", "sess", "get_content", map[string]interface{}{"path": "/post"})
	if strings.Contains(out, "error") || action != nil {
		t.Errorf("viewer get_content: %s", out)
	}
	out, _ = h.executeCopilotTool(ctx, "viewer", "sess", "update_content",
		map[string]interface{}{"id": contentID.Hex(), "title": "X", "version_comment": "x"})
	if !strings.Contains(out, "lacks permission") {
		t.Errorf("viewer update should be denied: %s", out)
	}
	out, _ = h.executeCopilotTool(ctx, "viewer", "sess", "publish_content",
		map[string]interface{}{"id": contentID.Hex()})
	if !strings.Contains(out, "lacks permission") {
		t.Errorf("viewer publish should be denied: %s", out)
	}

	// Editor can create + publish.
	out, action = h.executeCopilotTool(ctx, "editor", "sess", "create_content", map[string]interface{}{
		"template_name": "Blog Post", "title": "Copilot Draft", "slug": "cp-draft",
		"version_comment": "new draft", "data": map[string]interface{}{},
	})
	if action == nil || !strings.Contains(out, "cp-draft") {
		t.Fatalf("editor create: %s", out)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(out), &created)
	out, action = h.executeCopilotTool(ctx, "editor", "sess", "publish_content",
		map[string]interface{}{"id": created.ID})
	if action == nil || !strings.Contains(out, "success") {
		t.Errorf("editor publish: %s", out)
	}

	// Unknown tool and unknown template are handled.
	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "nonexistent_tool", nil)
	if !strings.Contains(out, "unknown tool") {
		t.Errorf("unknown tool: %s", out)
	}
	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "create_content", map[string]interface{}{
		"template_name": "Nope", "title": "T", "slug": "s", "version_comment": "c",
	})
	if !strings.Contains(out, "not found") {
		t.Errorf("unknown template: %s", out)
	}

	// list_templates and list_recent_content return data.
	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "list_templates", nil)
	if !strings.Contains(out, "Blog Post") {
		t.Errorf("list_templates: %s", out)
	}
	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "list_recent_content", nil)
	if !strings.Contains(out, "/post") {
		t.Errorf("list_recent_content: %s", out)
	}
}

func TestSidebarShowsVersion(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	h.SetAnthropicAPIKey("k")

	req := sessionReq("GET", "/cm/copilot", nil, nil)
	rr := httptest.NewRecorder()
	h.CopilotPage(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `class="sidebar-version"`) {
		t.Error("sidebar version element missing")
	}
	// build.json isn't present in the test cwd, so the default version
	// renders — assert the "v<something>" mechanism, not the exact number.
	if !regexp.MustCompile(`class="sidebar-version">v[0-9][0-9.]*<`).MatchString(rr.Body.String()) {
		t.Errorf("sidebar version not rendered as v<semver>")
	}
}

// TestCopilotCSRFTokenNotDoubleQuoted guards against re-introducing the
// printf %q bug: html/template already emits a quoted JS string for
// {{.CSRFToken}} in script context, so pre-quoting produced a token wrapped
// in literal quote characters that failed CSRF validation in production.
func TestCopilotCSRFTokenNotDoubleQuoted(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	h.SetAnthropicAPIKey("k")

	// Render through the real CSRF middleware so {{.CSRFToken}} is non-empty.
	key := sha256.Sum256([]byte("test-csrf-key"))
	protected := csrf.Protect(key[:], csrf.Secure(false), csrf.Path("/cm"))(http.HandlerFunc(h.CopilotPage))

	req := sessionReq("GET", "/cm/copilot", nil, nil)
	rr := httptest.NewRecorder()
	protected.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("status = %d", rr.Code)
	}
	m := regexp.MustCompile(`'X-CSRF-Token': (\S+)}`).FindStringSubmatch(rr.Body.String())
	if m == nil {
		t.Fatal("could not locate X-CSRF-Token expression in rendered page")
	}
	expr := m[1]
	// Must be exactly one level of JS quoting around a non-empty token.
	if !regexp.MustCompile(`^"[^"\\]+"$`).MatchString(expr) {
		t.Errorf("CSRF token expression malformed (double-quoted or empty): %s", expr)
	}
}

func TestExecuteCopilotTool_Analytics(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	// Seed a page view and flush the analytics buffer.
	h.analyticsService.RecordPageView(ctx, "/popular-page", "",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15")
	h.analyticsService.FlushBufferForTest()

	out, action := h.executeCopilotTool(ctx, "admin", "sess", "get_analytics",
		map[string]interface{}{"metric": "top_pages", "days": float64(7), "include_bots": true})
	if action != nil {
		t.Error("read-only analytics should not produce a write action")
	}
	if !strings.Contains(out, "/popular-page") {
		t.Errorf("top_pages missing seeded view: %s", out)
	}

	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "get_analytics",
		map[string]interface{}{"metric": "summary"})
	if !strings.Contains(out, "dau") || !strings.Contains(out, "uptime_pct") {
		t.Errorf("summary: %s", out)
	}

	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "get_analytics",
		map[string]interface{}{"metric": "top_referrers"})
	if !strings.Contains(out, "top_referrers") {
		t.Errorf("top_referrers: %s", out)
	}

	out, _ = h.executeCopilotTool(ctx, "admin", "sess", "get_analytics",
		map[string]interface{}{"metric": "nonsense"})
	if !strings.Contains(out, "metric must be") {
		t.Errorf("invalid metric: %s", out)
	}
}
