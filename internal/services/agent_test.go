package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// fakeResend captures emails sent to a fake Resend endpoint.
type fakeResend struct {
	mu    sync.Mutex
	sent  []map[string]interface{}
	fail  bool
	srv   *httptest.Server
	email *EmailService
}

func newFakeResend(t *testing.T) *fakeResend {
	f := &fakeResend{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, `{"message":"invalid key"}`, 401)
			return
		}
		if f.fail {
			http.Error(w, `{"message":"boom"}`, 500)
			return
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.sent = append(f.sent, body)
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"id": "msg-123"})
	}))
	t.Cleanup(f.srv.Close)
	f.email = NewEmailService("test-key", "Agent <agent@test.com>")
	f.email.SetEndpoint(f.srv.URL)
	return f
}

func TestEmailService(t *testing.T) {
	f := newFakeResend(t)
	ctx := context.Background()

	id, err := f.email.Send(ctx, "to@x.com", "Subj", "<p>hi</p>", "hi")
	if err != nil || id != "msg-123" {
		t.Fatalf("Send: id=%q err=%v", id, err)
	}
	f.mu.Lock()
	sent := f.sent[0]
	f.mu.Unlock()
	if sent["subject"] != "Subj" || sent["from"] != "Agent <agent@test.com>" {
		t.Errorf("payload: %+v", sent)
	}

	// API failure surfaces the message.
	f.fail = true
	if _, err := f.email.Send(ctx, "to@x.com", "S", "h", "t"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected API error, got %v", err)
	}

	// Unconfigured service refuses.
	empty := NewEmailService("", "")
	if empty.Configured() {
		t.Error("empty service should not be configured")
	}
	if _, err := empty.Send(ctx, "to@x.com", "S", "h", "t"); err == nil {
		t.Error("unconfigured send should error")
	}
}

func TestDigestDue(t *testing.T) {
	at := func(day time.Weekday, hour int) time.Time {
		// 2026-07-06 is a Monday
		base := time.Date(2026, 7, 6, hour, 30, 0, 0, time.UTC)
		return base.AddDate(0, 0, int(day-time.Monday))
	}
	past := time.Date(2026, 7, 1, 14, 0, 0, 0, time.UTC)

	cfg := AgentConfig{Frequency: "daily", SendHour: 13}
	if digestDue(cfg, at(time.Monday, 8)) {
		t.Error("before send hour should not be due")
	}
	if !digestDue(cfg, at(time.Monday, 13)) {
		t.Error("at send hour with no prior send should be due")
	}
	cfg.LastDigestAt = &past
	if !digestDue(cfg, at(time.Monday, 14)) {
		t.Error("last sent days ago should be due")
	}
	today := at(time.Monday, 13)
	cfg.LastDigestAt = &today
	if digestDue(cfg, at(time.Monday, 15)) {
		t.Error("already sent today should not be due")
	}

	cfg = AgentConfig{Frequency: "weekdays", SendHour: 13}
	if digestDue(cfg, at(time.Saturday, 15)) || digestDue(cfg, at(time.Sunday, 15)) {
		t.Error("weekends should not be due for weekdays frequency")
	}
	if !digestDue(cfg, at(time.Tuesday, 15)) {
		t.Error("Tuesday should be due for weekdays frequency")
	}

	cfg = AgentConfig{Frequency: "weekly", SendHour: 13}
	if digestDue(cfg, at(time.Wednesday, 15)) {
		t.Error("Wednesday should not be due for weekly frequency")
	}
	if !digestDue(cfg, at(time.Monday, 15)) {
		t.Error("Monday should be due for weekly frequency")
	}
}

func TestAgentConfigSaveLoad(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewAgentService(db, nil, nil, nil, nil, nil, "http://x", "")
	ctx := context.Background()

	// Defaults before save.
	cfg := svc.GetConfig(ctx)
	if !cfg.IncludeSiteHealth || cfg.Frequency != "daily" || cfg.SendHour != 13 {
		t.Errorf("defaults: %+v", cfg)
	}

	cfg.Enabled = true
	cfg.Email = "jon@example.com"
	cfg.Frequency = "weekly"
	cfg.IncludeBrokenLinks = true
	if err := svc.SaveConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := svc.GetConfig(ctx)
	if got.Email != "jon@example.com" || got.Frequency != "weekly" || !got.IncludeBrokenLinks {
		t.Errorf("round trip: %+v", got)
	}

	// Validation errors.
	bad := got
	bad.Frequency = "hourly"
	if err := svc.SaveConfig(ctx, bad); err == nil {
		t.Error("invalid frequency should be rejected")
	}
	bad = got
	bad.SendHour = 99
	if err := svc.SaveConfig(ctx, bad); err == nil {
		t.Error("invalid hour should be rejected")
	}
	bad = got
	bad.Email = ""
	if err := svc.SaveConfig(ctx, bad); err == nil {
		t.Error("enabled without email should be rejected")
	}
}

func TestAgentSendDigest(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()
	f := newFakeResend(t)

	// Seed data across sections.
	now := time.Now()
	old := now.Add(-300 * 24 * time.Hour)
	_, _ = db.InsertOne(ctx, "content", &models.Content{
		ID: primitive.NewObjectID(), Title: "Old Page", FullPath: "/old", Slug: "old",
		Published: true, MetaDescription: "x", CreatedAt: old, UpdatedAt: old,
	})
	audit := NewAuditService(db)
	audit.Log(ctx, models.AuditLog{
		UserID: primitive.NewObjectID(), Action: "content.update", Resource: "content",
		ResourceID: "c1", AgentSession: "agent-x", CreatedAt: now.Add(-time.Hour),
		Details: map[string]interface{}{"path": "/old"},
	})
	cs := NewContentService(db)
	forks := NewForkService(db, cs)
	fork, _ := forks.Create(ctx, "pending-fork", "", primitive.NewObjectID(), "e@x.com")
	liveID := seedLiveContent(t, db, "Live", "/fork-me")
	_, _ = forks.ForkPage(ctx, fork.ID, liveID)

	maint := NewMaintenanceService(db, nil)
	analytics := NewAnalyticsService(context.Background(), db, "http://x")
	approvals := NewApprovalService(db, cs, nil, nil)

	svc := NewAgentService(db, f.email, maint, analytics, forks, approvals, "https://site.example", "")
	cfg := DefaultAgentConfig()
	cfg.Enabled = true
	cfg.Email = "owner@example.com"

	msgID, err := svc.SendDigest(ctx, cfg)
	if err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if msgID != "msg-123" {
		t.Errorf("msgID = %q", msgID)
	}

	f.mu.Lock()
	sent := f.sent[len(f.sent)-1]
	f.mu.Unlock()
	htmlBody, _ := sent["html"].(string)
	for _, want := range []string{"Site health", "Traffic", "Awaiting your review", "Agent activity", "/old", "pending-fork"} {
		if !strings.Contains(htmlBody, want) {
			t.Errorf("digest missing %q", want)
		}
	}
	textBody, _ := sent["text"].(string)
	if !strings.Contains(textBody, "Site health") {
		t.Errorf("plain text body missing sections")
	}

	// Send state recorded.
	got := svc.GetConfig(ctx)
	if got.LastDigestAt == nil {
		t.Error("last_digest_at not recorded")
	}
	if got.LastError != "" {
		t.Errorf("last_error should be empty, got %q", got.LastError)
	}
	count, _ := db.Count(ctx, "agent_digests", bson.M{"ok": true})
	if count != 1 {
		t.Errorf("agent_digests log count = %d", count)
	}

	// Failed send records the error.
	f.fail = true
	if _, err := svc.SendDigest(ctx, cfg); err == nil {
		t.Fatal("expected send failure")
	}
	got = svc.GetConfig(ctx)
	if got.LastError == "" {
		t.Error("last_error not recorded on failure")
	}
}

func TestAgentAICommentary(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ctx := context.Background()
	f := newFakeResend(t)

	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "All quiet. Publish more griffins."}},
		})
	}))
	defer anthropic.Close()

	maint := NewMaintenanceService(db, nil)
	svc := NewAgentService(db, f.email, maint, nil, nil, nil, "https://s.example", "test-anthropic-key")
	svc.SetAnthropicURL(anthropic.URL)

	cfg := DefaultAgentConfig()
	cfg.Email = "o@x.com"
	cfg.IncludeAICommentary = true
	cfg.IncludeTraffic = false
	cfg.IncludePending = false
	cfg.IncludeAgentWork = false

	if _, err := svc.SendDigest(ctx, cfg); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	f.mu.Lock()
	htmlBody, _ := f.sent[len(f.sent)-1]["html"].(string)
	f.mu.Unlock()
	if !strings.Contains(htmlBody, "Publish more griffins") {
		t.Errorf("commentary missing from digest")
	}
}
