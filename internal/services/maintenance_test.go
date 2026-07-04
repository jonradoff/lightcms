package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMaintenanceService_ScanAndLatest(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()
	old := now.Add(-200 * 24 * time.Hour)

	pages := []*models.Content{
		{ID: primitive.NewObjectID(), Title: "Fresh", FullPath: "/fresh", Slug: "fresh",
			MetaDescription: "ok", Published: true, CreatedAt: now, UpdatedAt: now},
		{ID: primitive.NewObjectID(), Title: "Stale", FullPath: "/stale", Slug: "stale",
			MetaDescription: "ok", Published: true, CreatedAt: old, UpdatedAt: old},
		{ID: primitive.NewObjectID(), Title: "No Meta", FullPath: "/no-meta", Slug: "no-meta",
			Published: true, CreatedAt: now, UpdatedAt: now},
		{ID: primitive.NewObjectID(), Title: "Draft", FullPath: "/draft", Slug: "draft",
			Published: false, CreatedAt: now, UpdatedAt: now},
	}
	forkID := primitive.NewObjectID()
	forked := &models.Content{ID: primitive.NewObjectID(), Title: "Forked", FullPath: "/forked",
		Slug: "forked", Published: true, ForkID: &forkID, CreatedAt: old, UpdatedAt: old}
	for _, p := range append(pages, forked) {
		if _, err := db.InsertOne(ctx, "content", p); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	svc := NewMaintenanceService(db, nil)

	// No report yet.
	if _, err := svc.LatestReport(ctx); err == nil {
		t.Error("LatestReport should error before any scan")
	}

	report, err := svc.RunScan(ctx, false)
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if report.PageCount != 4 {
		t.Errorf("page count = %d, want 4 (fork content excluded)", report.PageCount)
	}
	if len(report.StalePages) != 1 || report.StalePages[0].Path != "/stale" {
		t.Errorf("stale = %+v", report.StalePages)
	}
	if report.StalePages != nil && report.StalePages[0].AgeDays < 199 {
		t.Errorf("age days = %d", report.StalePages[0].AgeDays)
	}
	if len(report.MissingMeta) != 1 || report.MissingMeta[0].Path != "/no-meta" {
		t.Errorf("missing meta = %+v", report.MissingMeta)
	}
	if len(report.Drafts) != 1 || report.Drafts[0].Path != "/draft" {
		t.Errorf("drafts = %+v", report.Drafts)
	}

	latest, err := svc.LatestReport(ctx)
	if err != nil {
		t.Fatalf("LatestReport: %v", err)
	}
	if latest.ID != report.ID {
		t.Errorf("latest = %s, want %s", latest.ID.Hex(), report.ID.Hex())
	}

	// A second scan becomes the latest.
	second, err := svc.RunScan(ctx, false)
	if err != nil {
		t.Fatalf("second RunScan: %v", err)
	}
	latest, _ = svc.LatestReport(ctx)
	if latest.ID != second.ID {
		t.Errorf("latest after second scan = %s, want %s", latest.ID.Hex(), second.ID.Hex())
	}
}
