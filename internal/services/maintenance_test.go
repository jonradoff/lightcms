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
	// Background goroutines from earlier tests can insert extra pages, so
	// assert membership by path rather than exact counts.
	if report.PageCount < 4 {
		t.Errorf("page count = %d, want >= 4", report.PageCount)
	}
	stale := map[string]int{}
	for _, s := range report.StalePages {
		stale[s.Path] = s.AgeDays
	}
	if age, found := stale["/stale"]; !found || age < 199 {
		t.Errorf("stale = %+v, want /stale with age >= 199", report.StalePages)
	}
	if _, found := stale["/fresh"]; found {
		t.Errorf("fresh page marked stale: %+v", report.StalePages)
	}
	if stale["/forked"] != 0 {
		t.Errorf("fork content leaked into stale report")
	}
	paths := func(refs []PageRef) map[string]bool {
		m := map[string]bool{}
		for _, r := range refs {
			m[r.Path] = true
		}
		return m
	}
	if mm := paths(report.MissingMeta); !mm["/no-meta"] || mm["/fresh"] {
		t.Errorf("missing meta = %+v", report.MissingMeta)
	}
	if d := paths(report.Drafts); !d["/draft"] {
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

func TestMaintenanceService_StartStopAndScanAndLog(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewMaintenanceService(db, nil)

	// scanAndLog runs a scan and stores a report.
	svc.scanAndLog(context.Background())
	if _, err := svc.LatestReport(context.Background()); err != nil {
		t.Errorf("scanAndLog should have stored a report: %v", err)
	}

	// Start/Stop lifecycle does not panic or leak.
	ctx, cancel := context.WithCancel(context.Background())
	svc2 := NewMaintenanceService(db, nil)
	svc2.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	svc2.Stop()
	cancel()
}
