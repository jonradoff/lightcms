package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/testutil"
)

func TestAnalyticsService_RecordActivity_DAU(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")
	defer svc.Stop()

	dau := svc.GetDAU(ctx)
	if dau != 0 {
		t.Errorf("expected 0 DAU on empty DB, got %d", dau)
	}

	svc.RecordActivity(ctx, "user-aaa")
	svc.RecordActivity(ctx, "user-bbb")

	dau = svc.GetDAU(ctx)
	if dau != 2 {
		t.Errorf("expected 2 DAU, got %d", dau)
	}
}

func TestAnalyticsService_RecordActivity_Dedup(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")
	defer svc.Stop()

	// Same user recorded multiple times — should only count once
	svc.RecordActivity(ctx, "user-dedup")
	svc.RecordActivity(ctx, "user-dedup")
	svc.RecordActivity(ctx, "user-dedup")

	dau := svc.GetDAU(ctx)
	if dau != 1 {
		t.Errorf("expected 1 DAU after dedup, got %d", dau)
	}
}

func TestAnalyticsService_RecordActivity_EmptyUserID(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")
	defer svc.Stop()

	// Empty userID should be silently ignored
	svc.RecordActivity(ctx, "")

	dau := svc.GetDAU(ctx)
	if dau != 0 {
		t.Errorf("expected 0 DAU for empty userID, got %d", dau)
	}
}

func TestAnalyticsService_GetMAU(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")
	defer svc.Stop()

	mau := svc.GetMAU(ctx)
	if mau != 0 {
		t.Errorf("expected 0 MAU on empty DB, got %d", mau)
	}

	svc.RecordActivity(ctx, "mau-user-1")
	svc.RecordActivity(ctx, "mau-user-2")

	mau = svc.GetMAU(ctx)
	if mau != 2 {
		t.Errorf("expected 2 MAU, got %d", mau)
	}
}

func TestAnalyticsService_GetContentCreatedToday_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")
	defer svc.Stop()

	count := svc.GetContentCreatedToday(ctx)
	if count != 0 {
		t.Errorf("expected 0 content created today, got %d", count)
	}
}

func TestAnalyticsService_Stop(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	svc := NewAnalyticsService(ctx, db, "http://localhost:8082")

	// Should not panic or block
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Stop() did not return in time")
	}
}
