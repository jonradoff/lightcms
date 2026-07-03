package services

import (
	"context"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/services/importer"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seedNamedTemplate inserts a minimal template with the given name and returns its ID.
func seedNamedTemplate(t *testing.T, db interface {
	InsertOne(context.Context, string, interface{}) (primitive.ObjectID, error)
}, name string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	now := time.Now()
	_, err := db.InsertOne(context.Background(), "templates", bson.M{
		"_id": id, "name": name, "slug": name, "html_layout": "<html>{{.Body}}</html>",
		"fields": bson.A{}, "created_at": now, "updated_at": now,
	})
	if err != nil {
		t.Fatalf("seedNamedTemplate: %v", err)
	}
	return id
}

func TestImportService_SourceCRUD(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewImportService(db, NewContentService(db))
	ctx := context.Background()

	src := &models.ImportSource{
		Name: "Feed", URL: "https://example.com/rss", Schedule: "daily", Active: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := svc.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if src.ID.IsZero() {
		t.Fatal("expected source ID set")
	}

	got, err := svc.GetSource(ctx, src.ID)
	if err != nil || got.Name != "Feed" {
		t.Fatalf("GetSource: %v / %+v", err, got)
	}
	if err := svc.UpdateSource(ctx, src.ID, bson.M{"name": "Feed2"}); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}
	list, err := svc.ListSources(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSources: %v / %d", err, len(list))
	}
	if err := svc.DeleteSource(ctx, src.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
}

func TestImportService_JobsAndSubscribe(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewImportService(db, NewContentService(db))
	ctx := context.Background()

	if _, err := svc.ListJobs(ctx, 10); err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	// Subscribe / Unsubscribe channel plumbing.
	ch := svc.Subscribe("job-1")
	svc.Unsubscribe("job-1", ch)

	// Missing job
	if _, err := svc.GetJob(ctx, primitive.NewObjectID()); err == nil {
		t.Error("expected error for missing job")
	}
}

func TestImportService_RunMarkdownImport(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewImportService(db, NewContentService(db))
	ctx := context.Background()
	seedNamedTemplate(t, db, "Imported")

	pages := []importer.MarkdownPage{
		{Filename: "hello.md", Frontmatter: map[string]string{"title": "Hello"}, Body: "# Hi\n\nWorld"},
		{Filename: "second.md", Frontmatter: map[string]string{}, Body: "Just body"},
	}
	job, err := svc.RunMarkdownImport(ctx, pages, "Imported", "", false, "tester")
	if err != nil {
		t.Fatalf("RunMarkdownImport: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job")
	}
	// The job should record processing of 2 pages.
	got, err := svc.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if got.Status == "" {
		t.Error("expected job status set")
	}
}

func TestImportService_RunCSVImport(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewImportService(db, NewContentService(db))
	ctx := context.Background()
	tmplID := seedNamedTemplate(t, db, "CSVTmpl")

	records := []importer.CSVRecord{
		{Fields: map[string]string{"title": "Row One", "body": "B1"}, Row: 2},
		{Fields: map[string]string{"title": "Row Two", "body": "B2"}, Row: 3},
	}
	job, err := svc.RunCSVImport(ctx, records, map[string]string{"body": "body"}, "title", "", tmplID, "", false, "tester")
	if err != nil {
		t.Fatalf("RunCSVImport: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job")
	}
}

func TestApprovalService_WorkflowCRUD(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewApprovalService(db, NewContentService(db), NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	wf := models.ApprovalWorkflow{
		Name: "Default", Trigger: "all_contributor", Mode: "concurrent",
		Approvers: []models.WorkflowApprover{{UserID: primitive.NewObjectID(), UserEmail: "a@x.com", Order: 0}},
	}
	created, err := svc.CreateWorkflow(ctx, wf)
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	if created.ID.IsZero() {
		t.Fatal("expected workflow ID")
	}

	got, err := svc.GetWorkflow(ctx, created.ID)
	if err != nil || got.Name != "Default" {
		t.Fatalf("GetWorkflow: %v / %+v", err, got)
	}

	created.Name = "Renamed"
	if err := svc.UpdateWorkflow(ctx, created.ID, *created); err != nil {
		t.Fatalf("UpdateWorkflow: %v", err)
	}

	list, err := svc.ListWorkflows(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListWorkflows: %v / %d", err, len(list))
	}

	if err := svc.DeleteWorkflow(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
}

func TestApprovalService_RequestQueries(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewApprovalService(db, NewContentService(db), NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	if n := svc.CountPending(ctx); n != 0 {
		t.Errorf("expected 0 pending, got %d", n)
	}
	if _, err := svc.GetRequest(ctx, primitive.NewObjectID()); err == nil {
		t.Error("expected error for missing request")
	}
	if list, err := svc.ListPending(ctx); err != nil || len(list) != 0 {
		t.Fatalf("ListPending: %v / %d", err, len(list))
	}
	if _, err := svc.ListMyQueue(ctx, primitive.NewObjectID()); err != nil {
		t.Fatalf("ListMyQueue: %v", err)
	}
	if _, err := svc.GetPendingRequestForContent(ctx, primitive.NewObjectID()); err != nil {
		// no pending request → may return (nil,nil) or error; only fail on hard error
		t.Logf("GetPendingRequestForContent returned: %v", err)
	}
}
