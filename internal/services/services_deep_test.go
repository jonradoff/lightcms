package services

import (
	"context"
	"testing"
	"time"

	"lightcms/internal/models"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func seedTmpl(t *testing.T, cs *ContentService, name, slug string) primitive.ObjectID {
	t.Helper()
	id := primitive.NewObjectID()
	now := time.Now()
	_, err := cs.DB().InsertOne(context.Background(), "templates", bson.M{
		"_id": id, "name": name, "slug": slug,
		"html_layout": "<html><body><h1>{{.Title}}</h1>{{.body}}</body></html>",
		"fields":      bson.A{bson.M{"name": "body", "label": "Body", "type": "markdown"}},
		"created_at":  now, "updated_at": now,
	})
	if err != nil {
		t.Fatalf("seedTmpl: %v", err)
	}
	return id
}

func TestContentService_FullLifecycle(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	ctx := context.Background()
	tmplID := seedTmpl(t, cs, "Page", "page")

	c := &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Hello",
		Slug: "hello", FullPath: "/hello",
		Data:      map[string]interface{}{"body": "# Hi\n\n[[World]] and #tag"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := cs.CreateContent(ctx, c, "initial"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// Publish → generates static page.
	if err := cs.PublishContent(ctx, c.ID); err != nil {
		t.Errorf("PublishContent: %v", err)
	}
	if err := cs.GenerateStaticPage(ctx, c); err != nil {
		t.Logf("GenerateStaticPage: %v", err)
	}

	// Update (creates version) then list/get versions and revert.
	c.Title = "Hello v2"
	c.Data["body"] = "Updated body"
	if err := cs.UpdateContent(ctx, c, "second"); err != nil {
		t.Errorf("UpdateContent: %v", err)
	}
	versions, err := cs.GetVersions(ctx, c.ID)
	if err != nil {
		t.Errorf("GetVersions: %v", err)
	}
	if len(versions) > 0 {
		v := versions[len(versions)-1].Version
		_, _ = cs.GetVersion(ctx, c.ID, v)
		_ = cs.RevertToVersion(ctx, c.ID, v, "revert")
	}

	// Queries.
	_, _ = cs.GetBacklinks(ctx, "/world")
	_, _ = cs.GetContentByIDs(ctx, []primitive.ObjectID{c.ID})
	_, _ = cs.ListContentScoped(ctx, ContentScope{})
	_, _ = cs.QueryContentForDirective(ctx, map[string]string{}, "created_at", "desc")
	cs.UpdateWikilinksOnRename(ctx, "Hello", "Hello v2", "/hello", "/hello")

	// Unpublish, delete, restore.
	_ = cs.UnpublishContent(ctx, c.ID)
	_ = cs.DeleteContent(ctx, c.ID)
	_ = cs.RestoreContent(ctx, c.ID)

	// Upsert + bulk create.
	if _, err := cs.UpsertContent(ctx, &models.Content{
		ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Up", Slug: "up", FullPath: "/up",
		Data: map[string]interface{}{"body": "x"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, "upsert"); err != nil {
		t.Errorf("UpsertContent: %v", err)
	}
	results := cs.BulkCreateContent(ctx, []*models.Content{
		{ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "B1", Slug: "b1", FullPath: "/b1", Data: map[string]interface{}{"body": "x"}},
		{ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "B2", Slug: "b2", FullPath: "/b2", Data: map[string]interface{}{"body": "y"}},
	}, "bulk")
	if len(results) != 2 {
		t.Errorf("BulkCreateContent: expected 2 results, got %d", len(results))
	}
}

func TestSearchService_Methods(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	cs := NewContentService(db)
	tmplID := seedTmpl(t, cs, "Page", "page")
	ctx := context.Background()

	// Seed some published content to search.
	for i, slug := range []string{"alpha", "beta", "gamma"} {
		c := &models.Content{
			ID: primitive.NewObjectID(), TemplateID: tmplID, Title: "Title " + slug,
			Slug: slug, FullPath: "/" + slug, Published: true,
			Data:      map[string]interface{}{"body": "searchable content about golang " + slug},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		_ = cs.CreateContent(ctx, c)
		_ = i
	}

	ss := NewSearchService(db, "") // no Voyage key
	_ = ss.HasVoyageKey()
	ss.InvalidateSearchConfigCache()

	_, _ = ss.Search(ctx, "golang", "fulltext", 10)
	_, _ = ss.SearchFullText(ctx, "golang", 10)
	if _, err := ss.SearchSemantic(ctx, "golang", 10); err == nil {
		t.Log("SearchSemantic returned nil error without Voyage key")
	}
	_, _ = ss.SearchHybrid(ctx, "golang", 10)
	_, _ = ss.Suggest(ctx, "gol", 5)
	_, _, _ = ss.EmbeddingStats(ctx)
	_ = ss.RebuildKeywords(ctx)
}

func TestWebhookService_FireEvent(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	ws := NewWebhookService(db)
	ctx := context.Background()

	// Register a webhook subscribed to the event, then fire it.
	if _, err := ws.Create(ctx, "hook", "https://127.0.0.1:0/unreachable", "secret", []string{"content.published"}, true); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// FireEvent runs delivery (the unreachable URL exercises the error/delivery-record path).
	ws.FireEvent(ctx, "content.published", map[string]interface{}{"id": "x"})
	time.Sleep(200 * time.Millisecond) // let async delivery attempt run
}
