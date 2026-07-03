package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"
)

func TestImportService_RunRSSImport(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	rss := `<?xml version="1.0"?>
<rss version="2.0"><channel>
<item><title>Imported Post</title><link>https://ex.com/p1</link><description>Body content</description><guid>g1</guid></item>
</channel></rss>`
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer feed.Close()

	svc := NewImportService(db, NewContentService(db))
	ctx := context.Background()
	seedNamedTemplate(t, db, "RSSTmpl")

	src := &models.ImportSource{
		Name: "Feed", URL: feed.URL, TemplateName: "RSSTmpl",
		Schedule: "daily", Active: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := svc.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	job, err := svc.RunRSSImport(ctx, src.ID, "tester")
	if err != nil {
		t.Fatalf("RunRSSImport: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job")
	}

	// The import may finish synchronously or shortly after; poll briefly.
	for i := 0; i < 30; i++ {
		j, err := svc.GetJob(ctx, job.ID)
		if err == nil && j.Status != "running" && j.Status != "" && j.Status != "pending" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}
