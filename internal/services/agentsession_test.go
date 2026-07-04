package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func auditEntry(session, action, contentID string, at time.Time, details map[string]interface{}) models.AuditLog {
	return models.AuditLog{
		UserID: primitive.NewObjectID(), UserEmail: "agent@x.com", ViaAPI: true,
		Action: action, Resource: "content", ResourceID: contentID,
		AgentSession: session, Details: details, CreatedAt: at,
	}
}

func TestAgentSession_ChangesAndRollback(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	audit := NewAuditService(db)
	svc := NewAgentSessionService(audit, cs)
	ctx := context.Background()

	// --- Updated item: created before the session, modified during it. ---
	updated := &models.Content{
		Title: "Before", Slug: "sess-upd", FullPath: "/sess-upd",
		Data: map[string]interface{}{"body": "original"},
	}
	if err := cs.CreateContent(ctx, updated, "initial"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// Ensure the pre-session version strictly predates the session window.
	time.Sleep(1100 * time.Millisecond)
	sessionStart := time.Now()

	updated.Title = "After Agent"
	updated.Data = map[string]interface{}{"body": "agent-modified"}
	if err := cs.UpdateContent(ctx, updated, "agent change"); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	// --- Created item: made during the session. ---
	created := &models.Content{
		Title: "Agent Made", Slug: "sess-new", FullPath: "/sess-new",
		Data: map[string]interface{}{},
	}
	if err := cs.CreateContent(ctx, created, "agent create"); err != nil {
		t.Fatalf("CreateContent(created): %v", err)
	}

	// --- Deleted item: existed, session deleted it. ---
	deleted := &models.Content{
		Title: "Doomed", Slug: "sess-del", FullPath: "/sess-del",
		Data: map[string]interface{}{},
	}
	if err := cs.CreateContent(ctx, deleted, "pre-existing"); err != nil {
		t.Fatalf("CreateContent(deleted): %v", err)
	}
	if err := cs.DeleteContent(ctx, deleted.ID); err != nil {
		t.Fatalf("DeleteContent: %v", err)
	}

	// Audit trail for session "sess1" (written synchronously for determinism).
	sess := "agent-test-sess1"
	entries := []models.AuditLog{
		auditEntry(sess, "content.update", updated.ID.Hex(), sessionStart.Add(50*time.Millisecond),
			map[string]interface{}{"title": "After Agent", "path": "/sess-upd"}),
		auditEntry(sess, "content.create", created.ID.Hex(), sessionStart.Add(100*time.Millisecond),
			map[string]interface{}{"title": "Agent Made", "path": "/sess-new"}),
		auditEntry(sess, "content.delete", deleted.ID.Hex(), sessionStart.Add(150*time.Millisecond), nil),
	}
	for _, e := range entries {
		audit.Log(ctx, e)
	}

	// --- Changes ledger ---
	summary, err := svc.Changes(ctx, sess)
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	if summary.Entries != 3 || len(summary.ContentItems) != 3 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.ContentItems[0].Path != "/sess-upd" || summary.ContentItems[0].Actions[0] != "content.update" {
		t.Errorf("first item: %+v", summary.ContentItems[0])
	}

	// Unknown session errors on rollback.
	if _, err := svc.Rollback(ctx, "no-such-session"); err == nil {
		t.Error("rollback of unknown session should error")
	}

	// --- Rollback ---
	result, err := svc.Rollback(ctx, sess)
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if len(result.Reverted) != 1 || result.Reverted[0] != updated.ID.Hex() {
		t.Errorf("reverted = %v (skipped: %+v)", result.Reverted, result.Skipped)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != created.ID.Hex() {
		t.Errorf("deleted = %v", result.Deleted)
	}
	if len(result.Restored) != 1 || result.Restored[0] != deleted.ID.Hex() {
		t.Errorf("restored = %v", result.Restored)
	}

	// Verify database state.
	var u models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": updated.ID}, &u); err != nil {
		t.Fatalf("reload updated: %v", err)
	}
	if u.Title != "Before" {
		t.Errorf("updated.Title = %q, want pre-session %q", u.Title, "Before")
	}
	if body, _ := u.Data["body"].(string); body != "original" {
		t.Errorf("updated.body = %q, want original", body)
	}

	var c models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": created.ID}, &c); err != nil {
		t.Fatalf("reload created: %v", err)
	}
	if !c.Deleted {
		t.Error("created item should be soft-deleted after rollback")
	}

	var d models.Content
	if err := db.FindOne(ctx, "content", bson.M{"_id": deleted.ID}, &d); err != nil {
		t.Fatalf("reload deleted: %v", err)
	}
	if d.Deleted {
		t.Error("deleted item should be restored after rollback")
	}
}

func TestVersionProvenance(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	cs := NewContentService(db)
	ctx := context.Background()

	// No provenance: defaults to human.
	c := &models.Content{Title: "Prov", Slug: "prov", FullPath: "/prov", Data: map[string]interface{}{}}
	if err := cs.CreateContent(ctx, c, "init"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// Agent provenance via context.
	agentCtx := WithProvenance(WithEditorEmail(ctx, "bot@x.com"), Provenance{
		Actor: "agent", Via: "api", AgentSession: "agent-prov-1",
	})
	c.Title = "Prov v2"
	if err := cs.UpdateContent(agentCtx, c, "agent change"); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	// Copilot provenance.
	copilotCtx := WithProvenance(ctx, Provenance{Actor: "agent", Via: "copilot", AgentSession: "copilot-u-1"})
	c.Title = "Prov v3"
	if err := cs.UpdateContent(copilotCtx, c, "copilot change"); err != nil {
		t.Fatalf("UpdateContent copilot: %v", err)
	}

	versions, err := cs.GetVersions(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetVersions: %v", err)
	}
	byVersion := map[int]models.ContentVersion{}
	for _, v := range versions {
		byVersion[v.Version] = v
	}

	if v := byVersion[1]; v.Actor != "human" || v.AgentSession != "" {
		t.Errorf("v1 provenance = %q/%q/%q, want human default", v.Actor, v.Via, v.AgentSession)
	}
	// The agent update produced the next version.
	var agentV, copilotV *models.ContentVersion
	for i := range versions {
		switch versions[i].AgentSession {
		case "agent-prov-1":
			agentV = &versions[i]
		case "copilot-u-1":
			copilotV = &versions[i]
		}
	}
	if agentV == nil || agentV.Actor != "agent" || agentV.Via != "api" || agentV.ModifiedByEmail != "bot@x.com" {
		t.Errorf("agent version provenance wrong: %+v", agentV)
	}
	if copilotV == nil || copilotV.Actor != "agent" || copilotV.Via != "copilot" {
		t.Errorf("copilot version provenance wrong: %+v", copilotV)
	}
}
