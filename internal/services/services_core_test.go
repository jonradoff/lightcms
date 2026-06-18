package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestApprovalService_Flow(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ws := NewWebhookService(db)
	cms := NewCommentService(db)
	aps := NewApprovalService(db, cs, cms, ws)
	ctx := context.Background()

	approver := primitive.NewObjectID()
	wf, err := aps.CreateWorkflow(ctx, models.ApprovalWorkflow{
		Name: "All", Trigger: "all_contributor", Mode: "concurrent",
		Approvers: []models.WorkflowApprover{{UserID: approver, UserEmail: "rev@x.com", Order: 0}},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	content := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: primitive.NewObjectID(),
		Title: "Draft", Slug: "draft", FullPath: "/draft",
	}
	req, err := aps.SubmitContentForApproval(ctx, content, primitive.NewObjectID(), "author@x.com")
	if err != nil {
		t.Fatalf("SubmitContentForApproval: %v", err)
	}
	if req == nil {
		t.Fatal("expected an approval request")
	}

	// Queues / counts.
	_, _ = aps.ListPending(ctx)
	_, _ = aps.ListMyQueue(ctx, approver)
	_ = aps.CountPending(ctx)
	_, _ = aps.GetPendingRequestForContent(ctx, content.ID)

	// Approve it.
	if err := aps.Approve(ctx, req.ID, approver, "rev@x.com", "lgtm"); err != nil {
		t.Errorf("Approve: %v", err)
	}

	// Asset review path + a reject + cancel on fresh requests.
	if _, err := aps.SubmitAssetForReview(ctx, &models.Asset{ID: primitive.NewObjectID(), FullPath: "/a.png"}, primitive.NewObjectID(), "author@x.com"); err != nil {
		t.Errorf("SubmitAssetForReview: %v", err)
	}
	_ = wf
}

func TestApprovalService_RejectCancel(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	aps := NewApprovalService(db, cs, NewCommentService(db), NewWebhookService(db))
	ctx := context.Background()

	approver := primitive.NewObjectID()
	if _, err := aps.CreateWorkflow(ctx, models.ApprovalWorkflow{
		Name: "All", Trigger: "all_contributor", Mode: "concurrent",
		Approvers: []models.WorkflowApprover{{UserID: approver, UserEmail: "rev@x.com"}},
	}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	mk := func(slug string) *models.ApprovalRequest {
		c := &models.Content{ID: primitive.NewObjectID(), TemplateID: primitive.NewObjectID(), Title: slug, Slug: slug, FullPath: "/" + slug}
		req, err := aps.SubmitContentForApproval(ctx, c, primitive.NewObjectID(), "a@x.com")
		if err != nil {
			t.Fatalf("submit %s: %v", slug, err)
		}
		return req
	}

	if err := aps.Reject(ctx, mk("r1").ID, approver, "rev@x.com", "Reviewer", "needs work"); err != nil {
		t.Errorf("Reject: %v", err)
	}
	if err := aps.Cancel(ctx, mk("r2").ID); err != nil {
		t.Errorf("Cancel: %v", err)
	}
}

func TestContentService_UpsertAndQuery(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	tmplID := seedTmpl(t, cs, "Page", "page")
	ctx := context.Background()

	c := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "U", Slug: "u", FullPath: "/u",
		Published: true, Data: map[string]interface{}{"body": "v1"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := cs.CreateContent(ctx, c); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	created, err := cs.UpsertContent(ctx, &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "U2", Slug: "u", FullPath: "/u",
		Data: map[string]interface{}{"body": "v2"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, "upsert-update")
	if err != nil {
		t.Errorf("UpsertContent update: %v", err)
	}
	if created {
		t.Error("expected UpsertContent to update (created=false) for existing path")
	}

	if _, err := cs.QueryContentForDirective(ctx, map[string]string{"status": "published"}, "created_at", "desc"); err != nil {
		t.Errorf("QueryContentForDirective: %v", err)
	}
	_, _, _ = cs.ListContentPaginated(ctx, PaginationOpts{Limit: 1, Offset: 0})
}

func TestTemplateSnippet_Lifecycle(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ts := NewTemplateService(db, cs)
	ctx := context.Background()

	tmpl := &models.Template{
		ID: primitive.NewObjectID(), Name: "Custom", Slug: "custom",
		HTMLLayout: "<html><body>{{.Title}}</body></html>",
		CreatedAt:  time.Now(), UpdatedAt: time.Now(),
	}
	if err := ts.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	tmpl.HTMLLayout = "<html><body><h1>{{.Title}}</h1></body></html>"
	if err := ts.UpdateTemplate(ctx, tmpl); err != nil {
		t.Errorf("UpdateTemplate: %v", err)
	}
	if _, err := ts.GetTemplate(ctx, tmpl.ID); err != nil {
		t.Errorf("GetTemplate: %v", err)
	}
	if err := ts.DeleteTemplate(ctx, tmpl.ID); err != nil {
		t.Errorf("DeleteTemplate: %v", err)
	}

	snip := NewSnippetService(db)
	s, err := snip.CreateSnippet(ctx, "promo", "<b>Promo</b>")
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	if _, err := snip.UpdateSnippet(ctx, s.ID, "promo", "<b>Promo v2</b>"); err != nil {
		t.Errorf("UpdateSnippet: %v", err)
	}
	if _, err := snip.GetSnippet(ctx, s.ID); err != nil {
		t.Errorf("GetSnippet: %v", err)
	}
	if err := snip.DeleteSnippet(ctx, s.ID); err != nil {
		t.Errorf("DeleteSnippet: %v", err)
	}
}

func TestWebhookService_DeliverSuccess(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ws := NewWebhookService(db)
	ctx := context.Background()

	got := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		select {
		case got <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	if _, err := ws.Create(ctx, "hook", srv.URL, "secret", []string{"content.published"}, true); err != nil {
		t.Fatalf("Create: %v", err)
	}
	ws.FireEvent(ctx, "content.published", map[string]interface{}{"id": "x"})

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Error("webhook delivery did not reach the endpoint")
	}
	time.Sleep(150 * time.Millisecond) // let delivery record persist
}

func TestLinkChecker_WithBrokenLinks(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	tmplID := seedTmpl(t, cs, "Page", "page")
	ctx := context.Background()

	// Published page referencing a non-existent wikilink target.
	_ = cs.CreateContent(ctx, &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Home", Slug: "home", FullPath: "/home",
		Published: true, Data: map[string]interface{}{"body": "See [[Totally Missing Page]] for details."},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	lc := NewLinkCheckerService(db)
	id, err := lc.StartJob(ctx)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	var status string
	for i := 0; i < 100; i++ {
		job, err := lc.GetJob(ctx, id)
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
		t.Fatalf("expected done, got %q", status)
	}
}

func TestContentService_SnippetExpansionAndIndex(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	tmplID := seedTmpl(t, cs, "Page", "page")
	ctx := context.Background()

	// A snippet that content can include.
	snip := NewSnippetService(db)
	if _, err := snip.CreateSnippet(ctx, "cta", "<div class=cta>Subscribe</div>"); err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}

	c := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Inc", Slug: "inc", FullPath: "/inc",
		Published: true, Data: map[string]interface{}{"body": "Intro [[include:cta]] outro"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := cs.CreateContent(ctx, c); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}
	if err := cs.GenerateStaticPage(ctx, c); err != nil {
		t.Logf("GenerateStaticPage: %v", err)
	}
	// Index/query helpers over real content.
	cs.RegenerateIndexPages(ctx)
	_, _ = cs.StreamContentScoped(ctx, ContentScope{IncludeDeleted: false})
}

func TestSearchService_WithMatches(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	tmplID := seedTmpl(t, cs, "Page", "page")
	ctx := context.Background()
	for _, s := range []string{"golang-guide", "golang-tips", "python-intro"} {
		_ = cs.CreateContent(ctx, &models.Content{
			ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "About " + s, Slug: s, FullPath: "/" + s,
			Published: true, Data: map[string]interface{}{"body": "an article about " + s + " programming"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})
	}
	ss := NewSearchService(db, "")
	_ = ss.RebuildKeywords(ctx)
	_, _ = ss.SearchFullText(ctx, "golang", 10)
	_, _ = ss.Search(ctx, "golang", "fulltext", 10)
	_, _ = ss.Suggest(ctx, "gola", 5)
}
