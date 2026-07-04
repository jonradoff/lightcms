package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AgentService is the CMS Agent: a built-in agent that monitors the site
// and reports to humans. v1 sends configurable email digests of the
// analyses the system already performs internally (maintenance scans,
// analytics, agent activity, pending reviews).
type AgentService struct {
	db          *database.DB
	email       *EmailService
	maintenance *MaintenanceService
	analytics   *AnalyticsService
	forks       *ForkService
	approvals   *ApprovalService

	baseURL         string
	anthropicAPIKey string
	anthropicURL    string // overridable in tests

	ticker *time.Ticker
	done   chan struct{}
}

// AgentConfig is the CMS Agent configuration, stored in the settings
// collection as type "agent_config".
type AgentConfig struct {
	Enabled   bool   `bson:"enabled" json:"enabled"`
	Email     string `bson:"email" json:"email"`
	Frequency string `bson:"frequency" json:"frequency"` // "daily" | "weekdays" | "weekly"
	SendHour  int    `bson:"send_hour" json:"send_hour"` // 0-23 UTC

	// Digest sections
	IncludeSiteHealth   bool `bson:"include_site_health" json:"include_site_health"`
	IncludeTraffic      bool `bson:"include_traffic" json:"include_traffic"`
	IncludePending      bool `bson:"include_pending" json:"include_pending"`
	IncludeBrokenLinks  bool `bson:"include_broken_links" json:"include_broken_links"`
	IncludeAgentWork    bool `bson:"include_agent_work" json:"include_agent_work"`
	IncludeAICommentary bool `bson:"include_ai_commentary" json:"include_ai_commentary"`

	LastDigestAt *time.Time `bson:"last_digest_at,omitempty" json:"last_digest_at,omitempty"`
	LastError    string     `bson:"last_error,omitempty" json:"last_error,omitempty"`
}

// DefaultAgentConfig returns the config used before any is saved.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		Frequency:           "daily",
		SendHour:            13, // 13:00 UTC ≈ morning in the Americas
		IncludeSiteHealth:   true,
		IncludeTraffic:      true,
		IncludePending:      true,
		IncludeAgentWork:    true,
		IncludeBrokenLinks:  false,
		IncludeAICommentary: false,
	}
}

func NewAgentService(db *database.DB, email *EmailService, maintenance *MaintenanceService,
	analytics *AnalyticsService, forks *ForkService, approvals *ApprovalService,
	baseURL, anthropicAPIKey string) *AgentService {
	return &AgentService{
		db: db, email: email, maintenance: maintenance, analytics: analytics,
		forks: forks, approvals: approvals,
		baseURL:         strings.TrimRight(baseURL, "/"),
		anthropicAPIKey: anthropicAPIKey,
		anthropicURL:    "https://api.anthropic.com/v1/messages",
		done:            make(chan struct{}),
	}
}

// SetAnthropicURL overrides the Anthropic endpoint (tests only).
func (s *AgentService) SetAnthropicURL(u string) { s.anthropicURL = u }

// EmailConfigured reports whether outbound email is available.
func (s *AgentService) EmailConfigured() bool { return s.email != nil && s.email.Configured() }

// GetConfig loads the agent configuration (defaults when unset).
func (s *AgentService) GetConfig(ctx context.Context) AgentConfig {
	var doc struct {
		Config AgentConfig `bson:"config"`
	}
	err := s.db.Settings().FindOne(ctx, bson.M{"type": "agent_config"}).Decode(&doc)
	if err != nil {
		return DefaultAgentConfig()
	}
	return doc.Config
}

// SaveConfig persists the agent configuration (preserving send-state fields).
func (s *AgentService) SaveConfig(ctx context.Context, cfg AgentConfig) error {
	current := s.GetConfig(ctx)
	cfg.LastDigestAt = current.LastDigestAt
	cfg.LastError = current.LastError
	if cfg.Frequency != "daily" && cfg.Frequency != "weekdays" && cfg.Frequency != "weekly" {
		return fmt.Errorf("frequency must be daily, weekdays, or weekly")
	}
	if cfg.SendHour < 0 || cfg.SendHour > 23 {
		return fmt.Errorf("send hour must be 0-23")
	}
	if cfg.Enabled && cfg.Email == "" {
		return fmt.Errorf("recipient email is required when the digest is enabled")
	}
	_, err := s.db.Settings().UpdateOne(ctx,
		bson.M{"type": "agent_config"},
		bson.M{"$set": bson.M{"config": cfg}, "$setOnInsert": bson.M{"type": "agent_config"}},
		options.Update().SetUpsert(true))
	return err
}

// recordSendState updates last-digest bookkeeping after a send attempt.
func (s *AgentService) recordSendState(ctx context.Context, sentAt time.Time, sendErr error) {
	set := bson.M{"config.last_digest_at": sentAt}
	if sendErr != nil {
		set["config.last_error"] = sendErr.Error()
	} else {
		set["config.last_error"] = ""
	}
	_, _ = s.db.Settings().UpdateOne(ctx, bson.M{"type": "agent_config"},
		bson.M{"$set": set, "$setOnInsert": bson.M{"type": "agent_config"}},
		options.Update().SetUpsert(true))
}

// Start launches the digest scheduler (checks every 10 minutes).
func (s *AgentService) Start(ctx context.Context) {
	s.ticker = time.NewTicker(10 * time.Minute)
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-ctx.Done():
				return
			case <-s.ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

// Stop terminates the scheduler.
func (s *AgentService) Stop() {
	close(s.done)
	if s.ticker != nil {
		s.ticker.Stop()
	}
}

func (s *AgentService) tick(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cfg := s.GetConfig(runCtx)
	if !cfg.Enabled || !s.EmailConfigured() {
		return
	}
	if !digestDue(cfg, time.Now().UTC()) {
		return
	}
	if _, err := s.SendDigest(runCtx, cfg); err != nil {
		log.Printf("[cms-agent] digest send failed: %v", err)
	}
}

// digestDue reports whether a digest should be sent at now (UTC), given the
// configured frequency, send hour, and last send time.
func digestDue(cfg AgentConfig, now time.Time) bool {
	if now.Hour() < cfg.SendHour {
		return false
	}
	switch cfg.Frequency {
	case "weekdays":
		if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			return false
		}
	case "weekly":
		if now.Weekday() != time.Monday {
			return false
		}
	}
	if cfg.LastDigestAt == nil {
		return true
	}
	last := cfg.LastDigestAt.UTC()
	// Already sent today?
	return !(last.Year() == now.Year() && last.YearDay() == now.YearDay())
}

// DigestData carries everything a digest email is built from.
type DigestData struct {
	GeneratedAt time.Time
	Since       time.Time
	SiteName    string
	BaseURL     string

	Health   *MaintenanceReport
	Traffic  *trafficSummary
	Pending  *pendingSummary
	AgentLog []agentLogEntry

	Commentary string
}

type trafficSummary struct {
	UptimePct     float64
	Visitors      int
	HumanVisitors int
	DAU, MAU      int64
	TopPages      []PageStat
	TopReferrers  []ReferrerStat
}

type pendingSummary struct {
	Forks     []forkSummary
	Approvals []approvalSummary
	Scheduled []scheduledSummary
}

type forkSummary struct {
	Name  string
	Pages int64
	URL   string
}

type approvalSummary struct {
	Title, Path, By string
}

type scheduledSummary struct {
	Title, Path string
	At          time.Time
}

type agentLogEntry struct {
	Session string
	Action  string
	Path    string
	At      time.Time
}

// BuildDigest gathers all enabled sections.
func (s *AgentService) BuildDigest(ctx context.Context, cfg AgentConfig) (*DigestData, error) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)
	if cfg.Frequency == "weekly" {
		since = now.Add(-7 * 24 * time.Hour)
	}
	if cfg.LastDigestAt != nil && cfg.LastDigestAt.After(since.Add(-31*24*time.Hour)) && cfg.LastDigestAt.Before(now) {
		since = cfg.LastDigestAt.UTC()
	}

	d := &DigestData{GeneratedAt: now, Since: since, BaseURL: s.baseURL}
	if theme, err := s.db.GetThemeSettings(ctx); err == nil {
		d.SiteName = theme.SiteName
	}

	if cfg.IncludeSiteHealth && s.maintenance != nil {
		report, err := s.maintenance.RunScan(ctx, cfg.IncludeBrokenLinks)
		if err == nil {
			d.Health = report
		}
	}

	if cfg.IncludeTraffic && s.analytics != nil {
		until := now.Add(time.Hour) // hour buckets: exclusive upper bound
		t := &trafficSummary{}
		t.UptimePct, t.Visitors, t.HumanVisitors = s.analytics.GetUptimeSummary(ctx, since)
		t.DAU = s.analytics.GetDAU(ctx)
		t.MAU = s.analytics.GetMAU(ctx)
		t.TopPages, _ = s.analytics.GetTopPages(ctx, since, until, 5, BotFilterHuman)
		t.TopReferrers, _ = s.analytics.GetTopReferrers(ctx, since, until, 5, BotFilterHuman)
		d.Traffic = t
	}

	if cfg.IncludePending {
		p := &pendingSummary{}
		if s.forks != nil {
			if forks, err := s.forks.List(ctx); err == nil {
				for _, f := range forks {
					if f.Status != "active" {
						continue
					}
					count, _ := s.forks.GetPageCount(ctx, f.ID)
					if count == 0 {
						continue
					}
					p.Forks = append(p.Forks, forkSummary{
						Name: f.Name, Pages: count, URL: s.baseURL + "/cm/forks/" + f.ID.Hex(),
					})
				}
			}
		}
		if s.approvals != nil {
			if reqs, err := s.approvals.ListPending(ctx); err == nil {
				for _, r := range reqs {
					p.Approvals = append(p.Approvals, approvalSummary{
						Title: r.ContentTitle, Path: r.ContentPath, By: r.SubmittedByEmail,
					})
				}
			}
		}
		// Upcoming scheduled publishes (next 7 days)
		cursor, err := s.db.FindMany(ctx, "content", bson.M{
			"publish_at": bson.M{"$gt": now, "$lt": now.Add(7 * 24 * time.Hour)},
			"deleted":    bson.M{"$ne": true},
		}, options.Find().SetProjection(bson.M{"title": 1, "full_path": 1, "publish_at": 1}).SetLimit(20))
		if err == nil {
			var items []struct {
				Title     string     `bson:"title"`
				FullPath  string     `bson:"full_path"`
				PublishAt *time.Time `bson:"publish_at"`
			}
			if cursor.All(ctx, &items) == nil {
				for _, it := range items {
					if it.PublishAt != nil {
						p.Scheduled = append(p.Scheduled, scheduledSummary{Title: it.Title, Path: it.FullPath, At: *it.PublishAt})
					}
				}
			}
		}
		d.Pending = p
	}

	if cfg.IncludeAgentWork {
		cursor, err := s.db.FindMany(ctx, "audit_logs", bson.M{
			"agent_session": bson.M{"$exists": true, "$ne": ""},
			"created_at":    bson.M{"$gte": since},
		}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(50))
		if err == nil {
			var logs []struct {
				AgentSession string                 `bson:"agent_session"`
				Action       string                 `bson:"action"`
				Details      map[string]interface{} `bson:"details"`
				CreatedAt    time.Time              `bson:"created_at"`
			}
			if cursor.All(ctx, &logs) == nil {
				for _, l := range logs {
					path, _ := l.Details["path"].(string)
					d.AgentLog = append(d.AgentLog, agentLogEntry{
						Session: l.AgentSession, Action: l.Action, Path: path, At: l.CreatedAt,
					})
				}
			}
		}
	}

	if cfg.IncludeAICommentary && s.anthropicAPIKey != "" {
		if commentary, err := s.generateCommentary(ctx, d); err == nil {
			d.Commentary = commentary
		} else {
			log.Printf("[cms-agent] AI commentary failed (continuing without): %v", err)
		}
	}

	return d, nil
}

// SendDigest builds and emails the digest, recording send state.
// Returns the Resend message ID.
func (s *AgentService) SendDigest(ctx context.Context, cfg AgentConfig) (string, error) {
	data, err := s.BuildDigest(ctx, cfg)
	if err != nil {
		s.recordSendState(ctx, time.Now().UTC(), err)
		return "", err
	}
	htmlBody, textBody := renderDigest(data)
	subject := fmt.Sprintf("CMS Agent digest — %s — %s", data.SiteName, data.GeneratedAt.Format("Jan 2, 2006"))

	msgID, err := s.email.Send(ctx, cfg.Email, subject, htmlBody, textBody)
	s.recordSendState(ctx, time.Now().UTC(), err)

	logDoc := bson.M{
		"sent_at": time.Now().UTC(), "to": cfg.Email, "subject": subject,
		"message_id": msgID, "ok": err == nil,
	}
	if err != nil {
		logDoc["error"] = err.Error()
	}
	_, _ = s.db.InsertOne(ctx, "agent_digests", logDoc)
	return msgID, err
}

// generateCommentary asks Claude for a short executive summary of the digest.
func (s *AgentService) generateCommentary(ctx context.Context, d *DigestData) (string, error) {
	raw, _ := json.Marshal(d)
	model := os.Getenv("LIGHTCMS_COPILOT_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 400,
		"system":     "You are the CMS Agent for a website. Given a JSON site digest, write a 3-4 sentence executive summary for the site owner: overall state, anything needing attention, and at most two concrete recommendations. Plain text, no markdown, no preamble.",
		"messages": []map[string]string{
			{"role": "user", "content": string(raw)},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", s.anthropicURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.anthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API status %d", resp.StatusCode)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil || len(out.Content) == 0 {
		return "", fmt.Errorf("unexpected anthropic response")
	}
	return strings.TrimSpace(out.Content[0].Text), nil
}

// renderDigest produces the HTML and plain-text bodies.
func renderDigest(d *DigestData) (htmlBody, textBody string) {
	var h, t strings.Builder
	esc := html.EscapeString

	h.WriteString(`<div style="font-family:system-ui,-apple-system,sans-serif;max-width:640px;margin:0 auto;color:#1e293b;">`)
	h.WriteString(fmt.Sprintf(`<h1 style="font-size:20px;">🤵 CMS Agent digest — %s</h1>`, esc(d.SiteName)))
	h.WriteString(fmt.Sprintf(`<p style="color:#64748b;font-size:13px;">Covering %s to %s (UTC)</p>`,
		d.Since.Format("Jan 2 15:04"), d.GeneratedAt.Format("Jan 2 15:04")))
	fmt.Fprintf(&t, "CMS Agent digest — %s\nCovering %s to %s (UTC)\n\n", d.SiteName,
		d.Since.Format("Jan 2 15:04"), d.GeneratedAt.Format("Jan 2 15:04"))

	section := func(title string) {
		h.WriteString(fmt.Sprintf(`<h2 style="font-size:15px;border-bottom:1px solid #e2e8f0;padding-bottom:4px;margin-top:24px;">%s</h2>`, esc(title)))
		fmt.Fprintf(&t, "\n== %s ==\n", title)
	}
	line := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		h.WriteString(`<p style="margin:4px 0;font-size:14px;">` + msg + `</p>`)
		t.WriteString(tagStripRE.ReplaceAllString(msg, "") + "\n")
	}

	if d.Commentary != "" {
		h.WriteString(fmt.Sprintf(`<div style="background:#eef2ff;border-left:3px solid #6366f1;padding:10px 14px;margin:16px 0;font-size:14px;line-height:1.6;">%s</div>`, esc(d.Commentary)))
		fmt.Fprintf(&t, "SUMMARY: %s\n", d.Commentary)
	}

	if d.Health != nil {
		section("Site health")
		line("%d pages scanned — <strong>%d stale</strong> (180+ days), <strong>%d missing meta descriptions</strong>, <strong>%d drafts</strong>",
			d.Health.PageCount, len(d.Health.StalePages), len(d.Health.MissingMeta), len(d.Health.Drafts))
		for i, p := range d.Health.StalePages {
			if i >= 5 {
				line("…and %d more stale pages", len(d.Health.StalePages)-5)
				break
			}
			line(`• Stale %d days: <a href="%s%s">%s</a>`, p.AgeDays, d.BaseURL, esc(p.Path), esc(p.Title))
		}
		if d.Health.LinkJobID != "" {
			line("Broken-link check started (job %s) — results in the admin panel", esc(d.Health.LinkJobID))
		}
	}

	if d.Traffic != nil {
		section("Traffic")
		line("Uptime %.1f%% — %d visitors (%d human) — DAU %d / MAU %d",
			d.Traffic.UptimePct, d.Traffic.Visitors, d.Traffic.HumanVisitors, d.Traffic.DAU, d.Traffic.MAU)
		for _, p := range d.Traffic.TopPages {
			line(`• %d views — <a href="%s%s">%s</a>`, p.Views, d.BaseURL, esc(p.Path), esc(p.Path))
		}
		for _, r := range d.Traffic.TopReferrers {
			line("• %d hits from %s", r.Hits, esc(r.Domain))
		}
	}

	if d.Pending != nil {
		section("Awaiting your review")
		if len(d.Pending.Forks) == 0 && len(d.Pending.Approvals) == 0 && len(d.Pending.Scheduled) == 0 {
			line("Nothing pending — inbox zero.")
		}
		for _, f := range d.Pending.Forks {
			line(`• Fork <strong>%s</strong> (%d pages) awaiting merge — <a href="%s">review</a>`, esc(f.Name), f.Pages, f.URL)
		}
		for _, a := range d.Pending.Approvals {
			line("• Approval: <strong>%s</strong> (%s) submitted by %s", esc(a.Title), esc(a.Path), esc(a.By))
		}
		for _, sch := range d.Pending.Scheduled {
			line("• Scheduled: <strong>%s</strong> (%s) publishes %s UTC", esc(sch.Title), esc(sch.Path), sch.At.Format("Jan 2 15:04"))
		}
	}

	if len(d.AgentLog) > 0 {
		section("Agent activity")
		line("%d agent actions since the last digest:", len(d.AgentLog))
		for i, l := range d.AgentLog {
			if i >= 10 {
				line("…and %d more (see the audit log)", len(d.AgentLog)-10)
				break
			}
			line("• %s — %s %s", l.At.Format("Jan 2 15:04"), esc(l.Action), esc(l.Path))
		}
	}

	h.WriteString(fmt.Sprintf(`<p style="color:#94a3b8;font-size:12px;margin-top:28px;">Sent by the LightCMS Agent · <a href="%s/cm/tools/agent">configure</a></p></div>`, d.BaseURL))
	fmt.Fprintf(&t, "\n— Sent by the LightCMS Agent · configure at %s/cm/tools/agent\n", d.BaseURL)
	return h.String(), t.String()
}
