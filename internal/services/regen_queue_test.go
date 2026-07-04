package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRegenQueue_EnqueueAndList(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	q := NewRegenQueue(db, NewContentService(db))
	ctx := context.Background()
	tmplID := primitive.NewObjectID()

	// Enqueue creates a pending job doc (worker not started; channel is buffered).
	q.Enqueue(ctx, tmplID, "Blog Post")

	jobs, err := q.ListRecentJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecentJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].TemplateName != "Blog Post" || jobs[0].Status != "pending" {
		t.Errorf("unexpected job: %+v", jobs[0])
	}

	// Enqueuing the same template again is deduped while pending.
	q.Enqueue(ctx, tmplID, "Blog Post")
	jobs, _ = q.ListRecentJobs(ctx, 10)
	if len(jobs) != 1 {
		t.Errorf("expected dedup to keep 1 job, got %d", len(jobs))
	}
}
