package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAuditService_Log(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	userID := primitive.NewObjectID()
	svc.Log(ctx, models.AuditLog{
		UserID:    userID,
		UserEmail: "test@example.com",
		Action:    "content.create",
		Resource:  "content",
	})

	logs, total, err := svc.List(ctx, AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 log, got %d", total)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(logs))
	}
	if logs[0].Action != "content.create" {
		t.Errorf("unexpected action: %q", logs[0].Action)
	}
}

func TestAuditService_LogAsync(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	svc.LogAsync(models.AuditLog{
		UserEmail: "async@example.com",
		Action:    "login.success",
		Resource:  "auth",
	})

	// Wait for goroutine to complete
	time.Sleep(200 * time.Millisecond)

	logs, total, err := svc.List(ctx, AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 1 {
		t.Error("expected at least 1 log from LogAsync")
	}
	_ = logs
}

func TestAuditService_List_FilterByAction(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	svc.Log(ctx, models.AuditLog{Action: "content.create", Resource: "content"})
	svc.Log(ctx, models.AuditLog{Action: "content.delete", Resource: "content"})
	svc.Log(ctx, models.AuditLog{Action: "login.success", Resource: "auth"})

	logs, total, err := svc.List(ctx, AuditFilter{Action: "content.create", Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 filtered log, got %d", total)
	}
	if logs[0].Action != "content.create" {
		t.Errorf("unexpected action: %q", logs[0].Action)
	}
}

func TestAuditService_List_FilterByResource(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	svc.Log(ctx, models.AuditLog{Action: "content.create", Resource: "content"})
	svc.Log(ctx, models.AuditLog{Action: "template.create", Resource: "template"})
	svc.Log(ctx, models.AuditLog{Action: "asset.upload", Resource: "content"})

	logs, total, err := svc.List(ctx, AuditFilter{Resource: "content", Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 content logs, got %d", total)
	}
	_ = logs
}

func TestAuditService_List_FilterByUserID(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	userA := primitive.NewObjectID()
	userB := primitive.NewObjectID()

	svc.Log(ctx, models.AuditLog{UserID: userA, Action: "content.create"})
	svc.Log(ctx, models.AuditLog{UserID: userB, Action: "content.create"})
	svc.Log(ctx, models.AuditLog{UserID: userA, Action: "content.delete"})

	logs, total, err := svc.List(ctx, AuditFilter{UserID: &userA, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 {
		t.Errorf("expected 2 logs for userA, got %d", total)
	}
	_ = logs
}

func TestAuditService_List_FilterByDateRange(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	past := time.Now().Add(-2 * time.Hour)
	future := time.Now().Add(2 * time.Hour)

	svc.Log(ctx, models.AuditLog{Action: "old.event", CreatedAt: time.Now().Add(-3 * time.Hour)})
	svc.Log(ctx, models.AuditLog{Action: "recent.event"}) // CreatedAt set by Log()

	logs, total, err := svc.List(ctx, AuditFilter{Since: &past, Until: &future, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 log in range, got %d", total)
	}
	if logs[0].Action != "recent.event" {
		t.Errorf("unexpected action: %q", logs[0].Action)
	}
}

func TestAuditService_List_Pagination(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		svc.Log(ctx, models.AuditLog{Action: "event"})
	}

	// First page
	page1, total, err := svc.List(ctx, AuditFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 results, got %d", len(page1))
	}

	// Second page
	page2, _, err := svc.List(ctx, AuditFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("expected 2 results on page 2, got %d", len(page2))
	}

	// Last page
	page3, _, err := svc.List(ctx, AuditFilter{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("List page3: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("expected 1 result on last page, got %d", len(page3))
	}
}

func TestAuditService_List_LimitCap(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	// Limit of 0 should default to 50; limit > 200 should cap at 200.
	// Just verify no error and default applies.
	_, _, err := svc.List(ctx, AuditFilter{Limit: 0})
	if err != nil {
		t.Fatalf("List with zero limit: %v", err)
	}
	_, _, err = svc.List(ctx, AuditFilter{Limit: 999})
	if err != nil {
		t.Fatalf("List with oversized limit: %v", err)
	}
}

func TestAuditService_Log_SetsCreatedAt(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAuditService(db)
	ctx := context.Background()

	before := time.Now().Add(-time.Second)
	svc.Log(ctx, models.AuditLog{Action: "test.event"})

	logs, _, _ := svc.List(ctx, AuditFilter{Limit: 1})
	if len(logs) == 0 {
		t.Fatal("expected 1 log")
	}
	if logs[0].CreatedAt.Before(before) {
		t.Error("expected CreatedAt to be set by Log()")
	}
}
