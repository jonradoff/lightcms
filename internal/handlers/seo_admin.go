package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SetSEOService wires the SEO settings service into the API handler.
func (a *APIHandler) SetSEOService(s *services.SEOService) { a.seoService = s }

// SetAnalyticsService wires analytics into the API handler (AI traffic report).
func (a *APIHandler) SetAnalyticsService(s *services.AnalyticsService) { a.analyticsService = s }

type crawlerRow struct {
	Token, Vendor, Purpose, PurposeLabel, Override string
	Allowed                                        bool
}

func crawlerRows(cfg services.SEOConfig) []crawlerRow {
	rows := make([]crawlerRow, 0, len(services.KnownCrawlers))
	for i := range services.KnownCrawlers {
		k := &services.KnownCrawlers[i]
		rows = append(rows, crawlerRow{
			Token: k.Token, Vendor: k.Vendor, Purpose: k.Purpose,
			PurposeLabel: services.CrawlerPurposeLabels[k.Purpose],
			Override:     cfg.CrawlerOverrides[k.Token],
			Allowed:      cfg.CrawlerAllowed(k),
		})
	}
	return rows
}

func (h *Handler) templateNames(r *http.Request) []string {
	cursor, err := h.db.FindMany(r.Context(), "templates", bson.M{}, options.Find().SetProjection(bson.M{"name": 1}))
	if err != nil {
		return nil
	}
	var ts []models.Template
	cursor.All(r.Context(), &ts) //nolint:errcheck
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// SEOToolPage renders Tools → SEO & AI (admin only).
func (h *Handler) SEOToolPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	h.renderSEOTool(w, r, h.seoConfig(r.Context()), r.URL.Query().Get("error"), r.URL.Query().Get("saved") == "1")
}

func (h *Handler) renderSEOTool(w http.ResponseWriter, r *http.Request, cfg services.SEOConfig, errMsg string, saved bool) {
	base := h.resolveBaseURL(r)
	selected := map[string]bool{}
	for _, t := range cfg.FeedTemplates {
		selected[t] = true
	}
	type tmplOpt struct {
		Name     string
		Selected bool
	}
	var tmpls []tmplOpt
	for _, n := range h.templateNames(r) {
		tmpls = append(tmpls, tmplOpt{n, selected[n]})
	}
	var indexNowActive bool
	if h.indexNowService != nil {
		if st, err := h.indexNowService.Status(r.Context()); err == nil {
			indexNowActive = st.Active
		}
	}
	h.renderAdmin(w, r, "seo_tool", map[string]interface{}{
		"Title":           "SEO & AI",
		"Cfg":             cfg,
		"Crawlers":        crawlerRows(cfg),
		"Templates":       tmpls,
		"FeedCategories":  strings.Join(cfg.FeedCategories, ", "),
		"AuthorSameAs":    strings.Join(cfg.AuthorSameAs, "\n"),
		"PublisherSameAs": strings.Join(cfg.PublisherSameAs, "\n"),
		"RobotsPreview":   cfg.RobotsTxt(base, nil),
		"BaseURL":         base,
		"IndexNowActive":  indexNowActive,
		"Error":           errMsg,
		"Saved":           saved,
	})
}

func splitList(s string, seps string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(seps, r) })
}

// SEOToolSave handles the SEO & AI settings form.
func (h *Handler) SEOToolSave(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermSettingsEdit) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	limit, _ := strconv.Atoi(r.FormValue("feed_limit"))
	cfg := services.SEOConfig{
		TrainingPolicy:   r.FormValue("training_policy"),
		AISearchPolicy:   r.FormValue("ai_search_policy"),
		UserFetchPolicy:  r.FormValue("user_fetch_policy"),
		CrawlerOverrides: map[string]string{},
		ContentSignals:   r.FormValue("content_signals") == "on",
		RobotsExtra:      r.FormValue("robots_extra"),
		MarkdownDisabled: r.FormValue("markdown_enabled") != "on",
		AuthorType:       r.FormValue("author_type"),
		AuthorName:       strings.TrimSpace(r.FormValue("author_name")),
		AuthorURL:        strings.TrimSpace(r.FormValue("author_url")),
		AuthorSameAs:     splitList(r.FormValue("author_same_as"), "\n\r ,"),
		PublisherSameAs:  splitList(r.FormValue("publisher_same_as"), "\n\r ,"),
		FeedDisabled:     r.FormValue("feed_enabled") != "on",
		FeedAllPages:     r.FormValue("feed_all_pages") == "on",
		FeedTemplates:    r.Form["feed_templates"],
		FeedCategories:   splitList(r.FormValue("feed_categories"), ","),
		FeedLimit:        limit,
	}
	for i := range services.KnownCrawlers {
		tok := services.KnownCrawlers[i].Token
		if v := r.FormValue("override_" + tok); v == services.CrawlerAllow || v == services.CrawlerDisallow {
			cfg.CrawlerOverrides[tok] = v
		}
	}
	if err := h.seoService.Save(r.Context(), cfg); err != nil {
		h.renderSEOTool(w, r, cfg, err.Error(), false)
		return
	}
	if h.auditService != nil {
		uid, _ := primitive.ObjectIDFromHex(user.ID)
		h.auditService.LogAsync(models.AuditLog{
			UserID: uid, UserEmail: user.Email, Action: "seo.config_update", Resource: "seo",
			Details: map[string]interface{}{
				"training": cfg.TrainingPolicy, "ai_search": cfg.AISearchPolicy, "user_fetch": cfg.UserFetchPolicy,
				"overrides": len(cfg.CrawlerOverrides), "markdown": !cfg.MarkdownDisabled, "feeds": !cfg.FeedDisabled,
			},
		})
	}
	http.Redirect(w, r, "/cm/tools/seo?saved=1", http.StatusSeeOther)
}

// AnalyticsAIPage renders Analytics → AI traffic.
func (h *Handler) AnalyticsAIPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.auth.GetCurrentUser(r)
	if !ok || !auth.HasPermission(user.Role, auth.PermAuditView) {
		http.Redirect(w, r, "/cm", http.StatusSeeOther)
		return
	}
	since, until, rangeParam, rangeStart, rangeEnd := parseAnalyticsRange(r)
	// Hour buckets use an exclusive upper bound; include the current hour.
	rep, err := h.analyticsService.GetAITraffic(r.Context(), since, until.Add(time.Hour), 25)
	if err != nil {
		rep = &services.AITrafficReport{}
	}
	// Day bars: scale each day's stacked purposes to the busiest day.
	maxDay := 0
	for _, d := range rep.Daily {
		t := 0
		for _, n := range d.ByPurpose {
			t += n
		}
		if t > maxDay {
			maxDay = t
		}
	}
	type bar struct {
		Day                                          string
		Training, AISearch, UserFetch, Search        int
		Total                                        int
		PctTraining, PctAISearch, PctUser, PctSearch float64
	}
	var bars []bar
	for _, d := range rep.Daily {
		b := bar{Day: d.Day, Training: d.ByPurpose[services.CrawlerPurposeTraining], AISearch: d.ByPurpose[services.CrawlerPurposeAISearch],
			UserFetch: d.ByPurpose[services.CrawlerPurposeUserFetch], Search: d.ByPurpose[services.CrawlerPurposeSearch]}
		b.Total = b.Training + b.AISearch + b.UserFetch + b.Search
		if maxDay > 0 {
			f := 100.0 / float64(maxDay)
			b.PctTraining, b.PctAISearch, b.PctUser, b.PctSearch = float64(b.Training)*f, float64(b.AISearch)*f, float64(b.UserFetch)*f, float64(b.Search)*f
		}
		bars = append(bars, b)
	}
	cfEnabled := false
	if cfg, err := h.db.GetSiteConfig(r.Context()); err == nil && cfg != nil {
		cfEnabled = cfg.CFCacheEnabled
	}
	h.renderAdmin(w, r, "analytics_ai", map[string]interface{}{
		"Title": "AI Traffic", "Report": rep, "Bars": bars,
		"Range": rangeParam, "RangeStart": rangeStart, "RangeEnd": rangeEnd,
		"CloudflareCache": cfEnabled,
	})
}

// ==================== REST API ====================

type seoResponse struct {
	services.SEOConfig
	Crawlers []crawlerRow `json:"crawlers"`
	Robots   string       `json:"robots_txt_preview"`
}

// APIGetSEO returns the SEO & AI settings (with defaults) and the crawler registry.
func (a *APIHandler) APIGetSEO(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsView) {
		return
	}
	cfg := a.seoService.Get(r.Context())
	a.jsonResponse(w, http.StatusOK, seoResponse{SEOConfig: cfg, Crawlers: crawlerRows(cfg), Robots: cfg.RobotsTxt(a.publicBaseURL(r), nil)})
}

// APIUpdateSEO applies a partial update: only fields present in the body change.
func (a *APIHandler) APIUpdateSEO(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermSettingsEdit) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "could not read body")
		return
	}
	cfg := a.seoService.GetRaw(r.Context())
	if err := json.Unmarshal(body, &cfg); err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := a.seoService.Save(r.Context(), cfg); err != nil {
		a.jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(body, &fields) //nolint:errcheck
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	a.auditLog(r, "seo.config_update", "seo", "", map[string]interface{}{"fields": keys})
	a.APIGetSEO(w, r)
}

// APIAITraffic returns the AI traffic report: ?days=N (default 30, max 90).
func (a *APIHandler) APIAITraffic(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermAuditView) {
		return
	}
	if a.analyticsService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "analytics not available")
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	until := time.Now().UTC().Add(time.Hour) // exclusive bound: include the current hour
	rep, err := a.analyticsService.GetAITraffic(r.Context(), until.Add(-time.Duration(days)*24*time.Hour), until, 25)
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.jsonResponse(w, http.StatusOK, rep)
}

func (a *APIHandler) publicBaseURL(r *http.Request) string {
	if a.baseURL != "" {
		return strings.TrimRight(a.baseURL, "/")
	}
	return "https://" + r.Host
}

func init() {
	adminTemplates["seo_tool"] = seoToolTemplate
	adminTemplates["analytics_ai"] = analyticsAITemplate
}

const seoInput = `width:100%; padding:8px 10px; border:1px solid var(--border); border-radius:8px; background:var(--bg); color:var(--text); font:inherit;`

var seoToolTemplate = adminLayoutStart + `
        <div class="page-header">
            <h1>🧭 SEO &amp; AI</h1>
            <p style="color: var(--text-muted);">How search engines and AI systems may use your site, what they find, and how your content is attributed. See also <a href="/cm/analytics/ai" style="color: var(--primary);">AI traffic</a> and <a href="/cm/tools/indexnow" style="color: var(--primary);">IndexNow</a>{{if .IndexNowActive}} (active){{end}}.</p>
        </div>
        {{if .Error}}<div class="card" style="border-color:#ef4444; margin-bottom:1rem;"><p style="color:#f87171; margin:0;">⚠️ {{.Error}}</p></div>{{end}}
        {{if .Saved}}<div class="card" style="border-color:#22c55e; margin-bottom:1rem;"><p style="color:#4ade80; margin:0;">✅ Saved. robots.txt, feeds and page metadata update immediately (cached copies may take up to an hour).</p></div>{{end}}

        <form method="POST" action="/cm/tools/seo">
            {{.CSRFField}}
            <div class="card" style="margin-bottom:1rem;">
                <h3 style="margin-top:0;">AI crawler policy</h3>
                <p style="color:var(--text-muted); margin-top:0;">Rendered into <a href="/robots.txt" target="_blank" style="color: var(--primary);">/robots.txt</a>. Classic search engines (Google, Bing) are not affected unless you override them below. Note: user-initiated fetchers don't always honor robots.txt.</p>
                <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap:1rem; margin-bottom:1rem;">
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Model training (GPTBot, ClaudeBot, Google-Extended…)</label>
                        <select name="training_policy" style="` + seoInput + `"><option value="allow" {{if eq .Cfg.TrainingPolicy "allow"}}selected{{end}}>Allow</option><option value="disallow" {{if eq .Cfg.TrainingPolicy "disallow"}}selected{{end}}>Block</option></select></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">AI search (OAI-SearchBot, PerplexityBot…)</label>
                        <select name="ai_search_policy" style="` + seoInput + `"><option value="allow" {{if eq .Cfg.AISearchPolicy "allow"}}selected{{end}}>Allow</option><option value="disallow" {{if eq .Cfg.AISearchPolicy "disallow"}}selected{{end}}>Block</option></select></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">User-initiated fetches (ChatGPT-User, Claude-User…)</label>
                        <select name="user_fetch_policy" style="` + seoInput + `"><option value="allow" {{if eq .Cfg.UserFetchPolicy "allow"}}selected{{end}}>Allow</option><option value="disallow" {{if eq .Cfg.UserFetchPolicy "disallow"}}selected{{end}}>Block</option></select></div>
                </div>
                <label style="display:flex; gap:8px; align-items:flex-start; margin-bottom:1rem;"><input type="checkbox" name="content_signals" {{if .Cfg.ContentSignals}}checked{{end}}> <span><strong>Add a Content-Signal line</strong> — states search / AI-input / AI-training permissions in the <a href="https://contentsignals.org" target="_blank" style="color: var(--primary);">Content Signals</a> format, derived from the choices above.</span></label>
                <details style="margin-bottom:1rem;"><summary style="cursor:pointer; font-weight:600;">Per-crawler overrides</summary>
                    <table style="width:100%; border-collapse:collapse; font-size:0.9rem; margin-top:0.5rem;">
                        <thead><tr style="text-align:left; color:var(--text-muted);"><th style="padding:4px 6px;">Crawler</th><th style="padding:4px 6px;">Operator</th><th style="padding:4px 6px;">Purpose</th><th style="padding:4px 6px;">Setting</th><th style="padding:4px 6px;">Result</th></tr></thead>
                        <tbody>{{range .Crawlers}}<tr style="border-top:1px solid var(--border);">
                            <td style="padding:4px 6px;"><code>{{.Token}}</code></td><td style="padding:4px 6px;">{{.Vendor}}</td><td style="padding:4px 6px;">{{.PurposeLabel}}</td>
                            <td style="padding:4px 6px;"><select name="override_{{.Token}}" style="padding:4px 6px; border:1px solid var(--border); border-radius:6px; background:var(--bg); color:var(--text);"><option value="">Use purpose setting</option><option value="allow" {{if eq .Override "allow"}}selected{{end}}>Always allow</option><option value="disallow" {{if eq .Override "disallow"}}selected{{end}}>Always block</option></select></td>
                            <td style="padding:4px 6px;">{{if .Allowed}}<span style="color:#4ade80;">allowed</span>{{else}}<span style="color:#f87171;">blocked</span>{{end}}</td></tr>{{end}}</tbody>
                    </table>
                </details>
                <label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Extra robots.txt lines (appended as-is)</label>
                <textarea name="robots_extra" rows="3" placeholder="User-agent: *&#10;Disallow: /private/" style="` + seoInput + ` font-family:monospace;">{{.Cfg.RobotsExtra}}</textarea>
                <details style="margin-top:0.75rem;"><summary style="cursor:pointer; color:var(--text-muted);">Current robots.txt</summary><pre style="background:var(--bg); padding:0.75rem; border-radius:8px; overflow:auto; font-size:0.8rem;">{{.RobotsPreview}}</pre></details>
            </div>

            <div class="card" style="margin-bottom:1rem;">
                <h3 style="margin-top:0;">Markdown copies</h3>
                <label style="display:flex; gap:8px; align-items:flex-start;"><input type="checkbox" name="markdown_enabled" {{if not .Cfg.MarkdownDisabled}}checked{{end}}> <span><strong>Serve a Markdown copy of every page</strong> at <code>/&lt;page&gt;.md</code> (homepage: <code>/index.md</code>) — clean text for AI agents, linked from each page's &lt;head&gt; and from <a href="/llms.txt" target="_blank" style="color: var(--primary);">llms.txt</a>. Copies are marked noindex so they never compete with your pages in search.</span></label>
            </div>

            <div class="card" style="margin-bottom:1rem;">
                <h3 style="margin-top:0;">Authorship &amp; publisher</h3>
                <p style="color:var(--text-muted); margin-top:0;">Used in structured data (schema.org) and feeds. Pages can override the author individually. User account names are never published automatically.</p>
                <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap:1rem; margin-bottom:1rem;">
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Default author type</label>
                        <select name="author_type" style="` + seoInput + `"><option value="Person" {{if ne .Cfg.AuthorType "Organization"}}selected{{end}}>Person</option><option value="Organization" {{if eq .Cfg.AuthorType "Organization"}}selected{{end}}>Organization</option></select></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Default author name</label><input name="author_name" value="{{.Cfg.AuthorName}}" placeholder="Leave blank to omit" style="` + seoInput + `"></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Author URL</label><input name="author_url" value="{{.Cfg.AuthorURL}}" placeholder="https://…" style="` + seoInput + `"></div>
                </div>
                <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap:1rem;">
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Author profiles (sameAs) — one URL per line</label><textarea name="author_same_as" rows="3" style="` + seoInput + `">{{.AuthorSameAs}}</textarea></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Site / organization profiles (sameAs) — one URL per line</label><textarea name="publisher_same_as" rows="3" style="` + seoInput + `">{{.PublisherSameAs}}</textarea></div>
                </div>
            </div>

            <div class="card" style="margin-bottom:1rem;">
                <h3 style="margin-top:0;">Feeds</h3>
                <label style="display:flex; gap:8px; align-items:flex-start; margin-bottom:0.75rem;"><input type="checkbox" name="feed_enabled" {{if not .Cfg.FeedDisabled}}checked{{end}}> <span><strong>Publish feeds</strong> at <a href="/feed.xml" target="_blank" style="color: var(--primary);">/feed.xml</a> (RSS) and <a href="/atom.xml" target="_blank" style="color: var(--primary);">/atom.xml</a> (Atom), plus <code>/&lt;collection&gt;/feed.xml</code> for each collection.</span></label>
                <label style="display:flex; gap:8px; align-items:flex-start; margin-bottom:0.75rem;"><input type="checkbox" name="feed_all_pages" {{if .Cfg.FeedAllPages}}checked{{end}}> <span>Include every page (otherwise only the templates and categories below)</span></label>
                <div style="margin-bottom:0.75rem;"><div style="font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Templates in the site feed</div>
                    <div style="display:flex; flex-wrap:wrap; gap:0.5rem 1.25rem;">{{range .Templates}}<label style="display:flex; gap:6px; align-items:center;"><input type="checkbox" name="feed_templates" value="{{.Name}}" {{if .Selected}}checked{{end}}> {{.Name}}</label>{{end}}</div></div>
                <div style="display:grid; grid-template-columns: 2fr 1fr; gap:1rem;">
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Categories in the site feed (comma-separated)</label><input name="feed_categories" value="{{.FeedCategories}}" style="` + seoInput + `"></div>
                    <div><label style="display:block; font-size:0.85rem; color:var(--text-muted); margin-bottom:4px;">Items per feed</label><input type="number" name="feed_limit" min="1" max="500" value="{{.Cfg.FeedLimit}}" style="` + seoInput + `"></div>
                </div>
            </div>

            <button type="submit" class="btn btn-primary">Save</button>
        </form>
        <p style="color:var(--text-muted); font-size:0.85rem; margin-top:1rem;">Per-page controls (hide from search &amp; AI, author override) are in each page's SEO settings.</p>
` + adminLayoutEnd

var analyticsAITemplate = adminLayoutStart + `
        <div class="content-section">
            <h1>🤖 AI Traffic</h1>
            <p style="color: var(--text-muted);">Which AI systems read your site, what they read, and how many people they send back. <a href="/cm/analytics?range={{.Range}}" style="color: var(--primary);">← All analytics</a> · <a href="/cm/tools/seo" style="color: var(--primary);">Crawler policy</a></p>
            <div style="display: flex; gap: 0.5rem; margin-bottom: 1.5rem; flex-wrap: wrap;">
                <a href="/cm/analytics/ai?range=24h" class="btn {{if eq .Range "24h"}}btn-primary{{else}}btn-secondary{{end}}" style="padding: 0.375rem 0.75rem; font-size: 0.875rem;">24 Hours</a>
                <a href="/cm/analytics/ai?range=7d" class="btn {{if eq .Range "7d"}}btn-primary{{else}}btn-secondary{{end}}" style="padding: 0.375rem 0.75rem; font-size: 0.875rem;">7 Days</a>
                <a href="/cm/analytics/ai?range=30d" class="btn {{if eq .Range "30d"}}btn-primary{{else}}btn-secondary{{end}}" style="padding: 0.375rem 0.75rem; font-size: 0.875rem;">30 Days</a>
                <a href="/cm/analytics/ai?range=90d" class="btn {{if eq .Range "90d"}}btn-primary{{else}}btn-secondary{{end}}" style="padding: 0.375rem 0.75rem; font-size: 0.875rem;">90 Days</a>
            </div>
            {{if .CloudflareCache}}<div class="card" style="border-color:#e0a030; margin-bottom:1rem;"><p style="margin:0;">⚠️ Cloudflare caching is on, so requests answered from Cloudflare's cache never reach this server. Crawler and visitor counts here are a lower bound.</p></div>{{end}}

            <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap:1rem; margin-bottom:1.5rem;">
                {{range .Report.ByPurpose}}<div class="card" style="padding:1rem;"><div style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase;">{{.Label}}</div><div style="font-size:1.75rem; font-weight:700;">{{.Hits}}</div><div style="font-size:0.75rem; color:var(--text-muted);">crawler requests</div></div>{{end}}
                <div class="card" style="padding:1rem;"><div style="font-size:0.75rem; color:var(--text-muted); text-transform:uppercase;">AI referrals</div><div style="font-size:1.75rem; font-weight:700;">{{.Report.ReferralTotal}}</div><div style="font-size:0.75rem; color:var(--text-muted);">visits from AI assistants</div></div>
            </div>

            <div class="card" style="margin-bottom:1.5rem;">
                <h3 style="margin-top:0; font-size:0.9rem; color:var(--text-muted); text-transform:uppercase;">Crawler requests per day</h3>
                <div style="display:flex; align-items:flex-end; gap:2px; height:160px;">
                    {{range .Bars}}<div title="{{.Day}}: {{.Total}} (training {{.Training}}, AI search {{.AISearch}}, user fetch {{.UserFetch}}, search {{.Search}})" style="flex:1; display:flex; flex-direction:column-reverse; height:100%;">
                        <div style="height:{{printf "%.1f" .PctSearch}}%; background:#64748b;"></div>
                        <div style="height:{{printf "%.1f" .PctTraining}}%; background:#f59e0b;"></div>
                        <div style="height:{{printf "%.1f" .PctAISearch}}%; background:#60a5fa;"></div>
                        <div style="height:{{printf "%.1f" .PctUser}}%; background:#4ade80;"></div>
                    </div>{{end}}
                </div>
                <div style="display:flex; gap:1rem; font-size:0.75rem; color:var(--text-muted); margin-top:0.5rem; flex-wrap:wrap;">
                    <span><span style="display:inline-block; width:10px; height:10px; background:#4ade80;"></span> AI user fetch</span>
                    <span><span style="display:inline-block; width:10px; height:10px; background:#60a5fa;"></span> AI search</span>
                    <span><span style="display:inline-block; width:10px; height:10px; background:#f59e0b;"></span> AI training</span>
                    <span><span style="display:inline-block; width:10px; height:10px; background:#64748b;"></span> Search engines</span>
                </div>
            </div>

            <div style="display:grid; grid-template-columns: 1fr 1fr; gap:1.5rem;">
                <div class="card"><h3 style="margin-top:0; font-size:0.9rem; color:var(--text-muted); text-transform:uppercase;">Crawlers</h3>
                    {{if .Report.Crawlers}}<table style="width:100%; font-size:0.875rem; border-collapse:collapse;">{{range .Report.Crawlers}}<tr style="border-top:1px solid var(--border);"><td style="padding:4px 6px;"><code>{{.Token}}</code></td><td style="padding:4px 6px; color:var(--text-muted);">{{.Vendor}} · {{.Label}}</td><td style="padding:4px 6px; text-align:right; font-family:monospace;">{{.Hits}}</td></tr>{{end}}</table>{{else}}<p style="color:var(--text-muted);">No known crawlers recorded in this range yet.</p>{{end}}</div>
                <div class="card"><h3 style="margin-top:0; font-size:0.9rem; color:var(--text-muted); text-transform:uppercase;">Visits from AI assistants</h3>
                    {{if .Report.Referrals}}<table style="width:100%; font-size:0.875rem; border-collapse:collapse;">{{range .Report.Referrals}}<tr style="border-top:1px solid var(--border);"><td style="padding:4px 6px;">{{.Assistant}}</td><td style="padding:4px 6px; text-align:right; font-family:monospace;">{{.Hits}}</td></tr>{{end}}</table>{{else}}<p style="color:var(--text-muted);">No AI referrals recorded in this range.</p>{{end}}</div>
                <div class="card"><h3 style="margin-top:0; font-size:0.9rem; color:var(--text-muted); text-transform:uppercase;">Most-read by AI</h3>
                    {{if .Report.TopCrawledPages}}<table style="width:100%; font-size:0.875rem; border-collapse:collapse;">{{range .Report.TopCrawledPages}}<tr style="border-top:1px solid var(--border);"><td style="padding:4px 6px; max-width:280px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;"><a href="{{.Path}}" target="_blank" style="color:#60a5fa;">{{.Path}}</a></td><td style="padding:4px 6px; text-align:right; font-family:monospace;">{{.Hits}}</td></tr>{{end}}</table>{{else}}<p style="color:var(--text-muted);">Nothing yet.</p>{{end}}</div>
                <div class="card"><h3 style="margin-top:0; font-size:0.9rem; color:var(--text-muted); text-transform:uppercase;">Where AI referrals land</h3>
                    {{if .Report.TopLandingPages}}<table style="width:100%; font-size:0.875rem; border-collapse:collapse;">{{range .Report.TopLandingPages}}<tr style="border-top:1px solid var(--border);"><td style="padding:4px 6px; max-width:280px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;"><a href="{{.Path}}" target="_blank" style="color:#60a5fa;">{{.Path}}</a></td><td style="padding:4px 6px; text-align:right; font-family:monospace;">{{.Views}}</td></tr>{{end}}</table>{{else}}<p style="color:var(--text-muted);">Nothing yet.</p>{{end}}</div>
            </div>
            <p style="color:var(--text-muted); font-size:0.8rem; margin-top:1rem;">Crawler identification began with v7.3.0; earlier periods show no crawler data. AI referrals include visits whose referrer is an AI assistant or that carry its utm_source tag (e.g. ChatGPT's ?utm_source=chatgpt.com).</p>
        </div>
` + adminLayoutEnd
