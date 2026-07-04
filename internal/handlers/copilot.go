package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The admin copilot: a chat panel in /cm that drives real content
// operations through an Anthropic tool-use loop. Tools execute directly
// against the service layer with per-tool RBAC checks, and every mutation
// is audit-logged under a copilot agent session.

const copilotMaxTurns = 8
const copilotMaxTokens = 2000

func copilotModel() string {
	if m := os.Getenv("LIGHTCMS_COPILOT_MODEL"); m != "" {
		return m
	}
	return "claude-sonnet-4-6"
}

const copilotSystemPrompt = `You are the LightCMS admin copilot. You help content editors manage their site through natural language, using the provided tools.

Rules:
- Use search_content / get_content / list_recent_content to find pages before editing them.
- Always include a concise version_comment when creating or updating content.
- Content "data" fields depend on the page's template — call get_content or list_templates to learn field names before writing them.
- Confirm destructive intents: if the user asks for something sweeping (many pages), summarize what you would do and ask before doing it.
- get_maintenance_report lists stale pages, missing meta descriptions, and drafts — use it for "what needs attention" questions.
- Site analytics are available via get_analytics (top pages, referrers, traffic summary) — use it for questions about popularity or traffic.
- Be concise. After acting, state plainly what changed and give the page path(s).`

// copilotToolDefs returns the Anthropic tool definitions for the copilot.
func copilotToolDefs() []map[string]interface{} {
	obj := func(props map[string]interface{}, required ...string) map[string]interface{} {
		s := map[string]interface{}{"type": "object", "properties": props}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	str := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	return []map[string]interface{}{
		{"name": "search_content", "description": "Full-text search across site content. Returns matching pages with paths and snippets.",
			"input_schema": obj(map[string]interface{}{"query": str("Search terms")}, "query")},
		{"name": "get_content", "description": "Fetch one page by URL path (e.g. /about) or content ID, including its template name and data fields.",
			"input_schema": obj(map[string]interface{}{"path": str("URL path"), "id": str("Content ID (alternative to path)")})},
		{"name": "list_recent_content", "description": "List the most recently updated pages (title, path, published state).",
			"input_schema": obj(map[string]interface{}{})},
		{"name": "list_templates", "description": "List content templates with their field names and types.",
			"input_schema": obj(map[string]interface{}{})},
		{"name": "update_content", "description": "Update a page. Only provided fields change; data merges per-key into existing data.",
			"input_schema": obj(map[string]interface{}{
				"id":               str("Content ID"),
				"title":            str("New title (optional)"),
				"meta_description": str("New meta description (optional)"),
				"data":             map[string]interface{}{"type": "object", "description": "Template data fields to merge"},
				"version_comment":  str("Required: short description of the change"),
			}, "id", "version_comment")},
		{"name": "create_content", "description": "Create a new page from a template (as a draft unless published=true).",
			"input_schema": obj(map[string]interface{}{
				"template_name":   str("Template name, e.g. 'Blog Post'"),
				"title":           str("Page title"),
				"slug":            str("URL slug"),
				"folder_path":     str("Folder path like /blog (optional)"),
				"data":            map[string]interface{}{"type": "object", "description": "Template data fields"},
				"version_comment": str("Required: short description"),
			}, "template_name", "title", "slug", "version_comment")},
		{"name": "get_maintenance_report", "description": "Latest site-health scan: stale pages (>180 days unmodified), published pages missing meta descriptions, and lingering drafts. Use as a work queue for site upkeep.",
			"input_schema": obj(map[string]interface{}{})},
		{"name": "get_analytics", "description": "Site analytics: most popular pages (by views), top referrers, or a traffic summary (DAU/MAU/uptime), over the last N days. Human traffic only unless include_bots is true.",
			"input_schema": obj(map[string]interface{}{
				"metric":       str("One of: top_pages, top_referrers, summary"),
				"days":         map[string]interface{}{"type": "integer", "description": "Lookback window in days (default 7, max 90)"},
				"include_bots": map[string]interface{}{"type": "boolean", "description": "Include bot traffic (default false)"},
			}, "metric")},
		{"name": "publish_content", "description": "Publish a page, making it live.",
			"input_schema": obj(map[string]interface{}{"id": str("Content ID")}, "id")},
		{"name": "unpublish_content", "description": "Unpublish a page, removing it from the live site.",
			"input_schema": obj(map[string]interface{}{"id": str("Content ID")}, "id")},
	}
}

// copilotAction records one tool execution for the frontend action log.
type copilotAction struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
	IsWrite bool   `json:"is_write"`
}

// executeCopilotTool runs one tool call against the service layer.
func (h *Handler) executeCopilotTool(ctx context.Context, role, sessionID string, name string, input map[string]interface{}) (string, *copilotAction) {
	getStr := func(k string) string { v, _ := input[k].(string); return v }
	deny := func(perm string) (string, *copilotAction) {
		return fmt.Sprintf(`{"error":"your role (%s) lacks permission %s"}`, role, perm), nil
	}
	fail := func(err error) (string, *copilotAction) {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), nil
	}
	ok := func(v interface{}, action *copilotAction) (string, *copilotAction) {
		b, _ := json.Marshal(v)
		return string(b), action
	}

	ctx = servicesWithCopilotProvenance(ctx, sessionID)

	switch name {
	case "search_content":
		if !auth.HasPermission(role, auth.PermContentView) {
			return deny(auth.PermContentView)
		}
		results, err := h.searchService.Search(ctx, getStr("query"), "", 10)
		if err != nil {
			return fail(err)
		}
		return ok(results, nil)

	case "get_content":
		if !auth.HasPermission(role, auth.PermContentView) {
			return deny(auth.PermContentView)
		}
		var c *models.Content
		var err error
		if p := getStr("path"); p != "" {
			c, err = h.contentService.GetContentByPath(ctx, p)
		} else if id, idErr := primitive.ObjectIDFromHex(getStr("id")); idErr == nil {
			c, err = h.contentService.GetContent(ctx, id)
		} else {
			return fail(fmt.Errorf("provide path or a valid id"))
		}
		if err != nil {
			return fail(err)
		}
		return ok(map[string]interface{}{
			"id": c.ID.Hex(), "title": c.Title, "path": c.FullPath,
			"template": c.TemplateName, "published": c.Published,
			"meta_description": c.MetaDescription, "tags": c.Tags, "data": c.Data,
		}, nil)

	case "list_recent_content":
		if !auth.HasPermission(role, auth.PermContentView) {
			return deny(auth.PermContentView)
		}
		cursor, err := h.db.FindMany(ctx, "content",
			bson.M{"deleted": bson.M{"$ne": true}, "fork_id": bson.M{"$exists": false}},
			options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetLimit(20))
		if err != nil {
			return fail(err)
		}
		var items []models.Content
		if err := cursor.All(ctx, &items); err != nil {
			return fail(err)
		}
		out := make([]map[string]interface{}, 0, len(items))
		for _, c := range items {
			out = append(out, map[string]interface{}{
				"id": c.ID.Hex(), "title": c.Title, "path": c.FullPath,
				"published": c.Published, "updated_at": c.UpdatedAt.Format(time.RFC3339),
			})
		}
		return ok(out, nil)

	case "list_templates":
		if !auth.HasPermission(role, auth.PermTemplateView) {
			return deny(auth.PermTemplateView)
		}
		cursor, err := h.db.FindMany(ctx, "templates", bson.M{})
		if err != nil {
			return fail(err)
		}
		var tmpls []models.Template
		if err := cursor.All(ctx, &tmpls); err != nil {
			return fail(err)
		}
		out := make([]map[string]interface{}, 0, len(tmpls))
		for _, t := range tmpls {
			fields := make([]map[string]string, 0, len(t.Fields))
			for _, f := range t.Fields {
				fields = append(fields, map[string]string{"name": f.Name, "type": f.Type, "label": f.Label})
			}
			out = append(out, map[string]interface{}{"id": t.ID.Hex(), "name": t.Name, "fields": fields})
		}
		return ok(out, nil)

	case "update_content":
		if !auth.HasPermission(role, auth.PermContentEdit) {
			return deny(auth.PermContentEdit)
		}
		id, err := primitive.ObjectIDFromHex(getStr("id"))
		if err != nil {
			return fail(fmt.Errorf("invalid id"))
		}
		c, err := h.contentService.GetContent(ctx, id)
		if err != nil {
			return fail(err)
		}
		if t := getStr("title"); t != "" {
			c.Title = t
		}
		if m := getStr("meta_description"); m != "" {
			c.MetaDescription = m
		}
		if data, okd := input["data"].(map[string]interface{}); okd {
			if c.Data == nil {
				c.Data = map[string]interface{}{}
			}
			for k, v := range data {
				c.Data[k] = v
			}
		}
		comment := getStr("version_comment")
		if comment == "" {
			comment = "Copilot edit"
		}
		if err := h.contentService.UpdateContent(ctx, c, "Copilot: "+comment); err != nil {
			return fail(err)
		}
		h.copilotAudit(sessionID, "content.update", c.ID.Hex(), map[string]interface{}{"title": c.Title, "path": c.FullPath, "via": "copilot"})
		return ok(map[string]interface{}{"success": true, "id": c.ID.Hex(), "path": c.FullPath},
			&copilotAction{Tool: name, Summary: "Updated " + c.FullPath, IsWrite: true})

	case "create_content":
		if !auth.HasPermission(role, auth.PermContentCreate) {
			return deny(auth.PermContentCreate)
		}
		var tmpl models.Template
		if err := h.db.FindOne(ctx, "templates", bson.M{"name": getStr("template_name")}, &tmpl); err != nil {
			return fail(fmt.Errorf("template %q not found", getStr("template_name")))
		}
		data, _ := input["data"].(map[string]interface{})
		if data == nil {
			data = map[string]interface{}{}
		}
		c := &models.Content{
			TemplateID: tmpl.ID, TemplateName: tmpl.Name,
			Title: getStr("title"), Slug: getStr("slug"),
			FolderPath: getStr("folder_path"), Data: data,
			UseHeader: true, UseFooter: true, UseTheme: true,
		}
		comment := getStr("version_comment")
		if comment == "" {
			comment = "Copilot create"
		}
		if err := h.contentService.CreateContent(ctx, c, "Copilot: "+comment); err != nil {
			return fail(err)
		}
		h.copilotAudit(sessionID, "content.create", c.ID.Hex(), map[string]interface{}{"title": c.Title, "path": c.FullPath, "via": "copilot"})
		return ok(map[string]interface{}{"success": true, "id": c.ID.Hex(), "path": c.FullPath, "published": false},
			&copilotAction{Tool: name, Summary: "Created draft " + c.FullPath, IsWrite: true})

	case "get_maintenance_report":
		if !auth.HasPermission(role, auth.PermContentView) {
			return deny(auth.PermContentView)
		}
		if h.maintenanceService == nil {
			return fail(fmt.Errorf("maintenance service unavailable"))
		}
		report, err := h.maintenanceService.LatestReport(ctx)
		if err != nil {
			// No stored report yet — run a scan on demand.
			report, err = h.maintenanceService.RunScan(ctx, false)
			if err != nil {
				return fail(err)
			}
		}
		return ok(report, nil)

	case "get_analytics":
		if !auth.HasPermission(role, auth.PermSettingsView) {
			return deny(auth.PermSettingsView)
		}
		if h.analyticsService == nil {
			return fail(fmt.Errorf("analytics service unavailable"))
		}
		days := 7
		if d, okd := input["days"].(float64); okd && d > 0 {
			days = int(d)
			if days > 90 {
				days = 90
			}
		}
		filter := services.BotFilterHuman
		if b, okb := input["include_bots"].(bool); okb && b {
			filter = services.BotFilterAll
		}
		since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		// The hour-bucket query's upper bound is exclusive — extend past the
		// current hour so today's traffic is included.
		until := time.Now().Add(time.Hour)
		switch getStr("metric") {
		case "top_pages":
			pages, err := h.analyticsService.GetTopPages(ctx, since, until, 15, filter)
			if err != nil {
				return fail(err)
			}
			return ok(map[string]interface{}{"days": days, "top_pages": pages}, nil)
		case "top_referrers":
			refs, err := h.analyticsService.GetTopReferrers(ctx, since, until, 15, filter)
			if err != nil {
				return fail(err)
			}
			return ok(map[string]interface{}{"days": days, "top_referrers": refs}, nil)
		case "summary":
			uptime, total, human := h.analyticsService.GetUptimeSummary(ctx, since)
			return ok(map[string]interface{}{
				"days": days, "uptime_pct": uptime,
				"visitors_total": total, "visitors_human": human,
				"dau": h.analyticsService.GetDAU(ctx), "mau": h.analyticsService.GetMAU(ctx),
			}, nil)
		default:
			return fail(fmt.Errorf("metric must be top_pages, top_referrers, or summary"))
		}

	case "publish_content", "unpublish_content":
		if !auth.HasPermission(role, auth.PermContentPublish) {
			return deny(auth.PermContentPublish)
		}
		id, err := primitive.ObjectIDFromHex(getStr("id"))
		if err != nil {
			return fail(fmt.Errorf("invalid id"))
		}
		verb := "Published"
		if name == "publish_content" {
			err = h.contentService.PublishContent(ctx, id)
		} else {
			err = h.contentService.UnpublishContent(ctx, id)
			verb = "Unpublished"
		}
		if err != nil {
			return fail(err)
		}
		h.copilotAudit(sessionID, "content."+strings.TrimSuffix(name, "_content"), id.Hex(), map[string]interface{}{"via": "copilot"})
		return ok(map[string]interface{}{"success": true},
			&copilotAction{Tool: name, Summary: verb + " " + id.Hex(), IsWrite: true})
	}
	return fmt.Sprintf(`{"error":"unknown tool %s"}`, name), nil
}

func (h *Handler) copilotAudit(sessionID, action, resourceID string, details map[string]interface{}) {
	h.auditService.LogAsync(models.AuditLog{
		Action: action, Resource: "content", ResourceID: resourceID,
		Details: details, AgentSession: sessionID,
	})
}

// anthropicMessage mirrors the Anthropic Messages API content structure the
// copilot loop needs (text and tool blocks).
type anthropicContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []anthropicContentBlock
}

// CopilotPage redirects to the dashboard with the copilot drawer open —
// the copilot is a slide-in panel on every admin page rather than a
// dedicated section.
func (h *Handler) CopilotPage(w http.ResponseWriter, r *http.Request) {
	user, okUser := h.auth.GetCurrentUser(r)
	if !okUser || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Error(w, "Forbidden — copilot requires editor access", http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/cm?copilot=1", http.StatusSeeOther)
}

// CopilotChat runs one copilot turn: it forwards the conversation to the
// Anthropic API with tools, executes any tool calls, and returns the final
// assistant text plus the actions taken.
func (h *Handler) CopilotChat(w http.ResponseWriter, r *http.Request) {
	user, okUser := h.auth.GetCurrentUser(r)
	if !okUser || !auth.HasPermission(user.Role, auth.PermContentEdit) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if h.anthropicAPIKey == "" {
		http.Error(w, `{"error":"ANTHROPIC_API_KEY is not configured on the server"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil || len(req.Messages) == 0 {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	sessionID := "copilot-" + user.ID + "-" + time.Now().UTC().Format("20060102")

	msgs := make([]anthropicMessage, 0, len(req.Messages)+2*copilotMaxTurns)
	for _, m := range req.Messages {
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, anthropicMessage{Role: role, Content: m.Content})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	var actions []copilotAction
	finalText := ""

	for turn := 0; turn < copilotMaxTurns; turn++ {
		respBlocks, stopReason, err := h.copilotCallAnthropic(ctx, msgs)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadGateway)
			return
		}

		var textParts []string
		var toolUses []anthropicContentBlock
		for _, b := range respBlocks {
			switch b.Type {
			case "text":
				textParts = append(textParts, b.Text)
			case "tool_use":
				toolUses = append(toolUses, b)
			}
		}

		if stopReason != "tool_use" || len(toolUses) == 0 {
			finalText = strings.Join(textParts, "\n")
			break
		}

		// Append assistant turn (verbatim blocks), then tool results.
		msgs = append(msgs, anthropicMessage{Role: "assistant", Content: respBlocks})
		var results []anthropicContentBlock
		for _, tu := range toolUses {
			resultJSON, action := h.executeCopilotTool(ctx, user.Role, sessionID, tu.Name, tu.Input)
			if action != nil {
				actions = append(actions, *action)
			}
			results = append(results, anthropicContentBlock{
				Type: "tool_result", ToolUseID: tu.ID, Content: resultJSON,
			})
		}
		msgs = append(msgs, anthropicMessage{Role: "user", Content: results})
		finalText = strings.Join(textParts, "\n") // fallback if we hit the turn cap
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"reply":   finalText,
		"actions": actions,
	})
}

// copilotCallAnthropic makes one non-streaming Messages API call with tools.
func (h *Handler) copilotCallAnthropic(ctx context.Context, msgs []anthropicMessage) ([]anthropicContentBlock, string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model":      copilotModel(),
		"max_tokens": copilotMaxTokens,
		"system":     copilotSystemPrompt,
		"tools":      copilotToolDefs(),
		"messages":   msgs,
	})
	endpoint := anthropicURL
	if h.anthropicURLOverride != "" {
		endpoint = h.anthropicURLOverride
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", h.anthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error.Message != "" {
			return nil, "", fmt.Errorf("anthropic API: %s", apiErr.Error.Message)
		}
		return nil, "", fmt.Errorf("anthropic API returned status %d", resp.StatusCode)
	}

	var out struct {
		Content    []anthropicContentBlock `json:"content"`
		StopReason string                  `json:"stop_reason"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", err
	}
	return out.Content, out.StopReason, nil
}

// servicesWithCopilotProvenance stamps copilot provenance on tool contexts.
func servicesWithCopilotProvenance(ctx context.Context, sessionID string) context.Context {
	return services.WithProvenance(ctx, services.Provenance{
		Actor: "agent", Via: "copilot", AgentSession: sessionID,
	})
}
