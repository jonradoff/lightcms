package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLinkChecker_StartAndComplete(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewLinkCheckerService(db)
	ctx := context.Background()

	id, err := svc.StartJob(ctx)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	if id.IsZero() {
		t.Fatal("expected non-zero job ID")
	}

	// Poll until the background scan finishes (no content => fast).
	var status string
	for i := 0; i < 100; i++ {
		job, err := svc.GetJob(ctx, id)
		if err != nil {
			t.Fatalf("GetJob: %v", err)
		}
		status = job.Status
		if status != "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != "done" {
		t.Fatalf("expected job status 'done', got %q", status)
	}
}

func TestLinkChecker_GetJobNotFound(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewLinkCheckerService(db)
	if _, err := svc.GetJob(context.Background(), primitive.NewObjectID()); err == nil {
		t.Error("expected error for missing job")
	}
}
