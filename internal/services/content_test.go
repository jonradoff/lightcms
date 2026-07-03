package services

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestContentService(t *testing.T) (*ContentService, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	svc := NewContentService(db)
	return svc, cleanup
}

func createTestTemplate(t *testing.T, svc *ContentService) primitive.ObjectID {
	t.Helper()
	ctx := context.Background()
	tmpl := models.Template{
		Name:       "Test Template",
		Slug:       "test-template",
		Fields:     []models.TemplateField{{Name: "content", Label: "Content", Type: "richtext", Required: true}},
		HTMLLayout: "<div>{{.content}}</div>",
	}
	id, err := svc.db.InsertOne(ctx, "templates", tmpl)
	if err != nil {
		t.Fatalf("failed to create test template: %v", err)
	}
	return id
}

func TestCreateContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Test Page",
		Slug:         "test-page",
		Data:         map[string]interface{}{"content": "<p>Hello</p>"},
	}

	err := svc.CreateContent(ctx, content)
	if err != nil {
		t.Fatalf("CreateContent failed: %v", err)
	}

	if content.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
	if content.FullPath != "/test-page" {
		t.Errorf("expected full_path /test-page, got %s", content.FullPath)
	}
}

func TestCreateContent_WithFolder(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Blog Post",
		Slug:         "my-post",
		FolderPath:   "/blog",
		Data:         map[string]interface{}{"content": "<p>Post</p>"},
	}

	err := svc.CreateContent(ctx, content)
	if err != nil {
		t.Fatalf("CreateContent failed: %v", err)
	}

	if content.FullPath != "/blog/my-post" {
		t.Errorf("expected /blog/my-post, got %s", content.FullPath)
	}
}

func TestGetContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Get Test", Slug: "get-test",
		Data: map[string]interface{}{"content": "hello"},
	}
	svc.CreateContent(ctx, content)

	got, err := svc.GetContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if got.Title != "Get Test" {
		t.Errorf("expected title 'Get Test', got %q", got.Title)
	}
}

func TestGetContentByPath(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Path Test", Slug: "path-test",
		Data: map[string]interface{}{"content": "hello"},
	}
	svc.CreateContent(ctx, content)

	got, err := svc.GetContentByPath(ctx, "/path-test")
	if err != nil {
		t.Fatalf("GetContentByPath failed: %v", err)
	}
	if got.Title != "Path Test" {
		t.Errorf("expected title 'Path Test', got %q", got.Title)
	}
}

func TestUpdateContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Original", Slug: "update-test",
		Data: map[string]interface{}{"content": "original"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "Updated"
	content.Data["content"] = "updated content"
	err := svc.UpdateContent(ctx, content, "Updated title and content")
	if err != nil {
		t.Fatalf("UpdateContent failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", got.Title)
	}
}

func TestListContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Post 1", Slug: "post-1",
		Category: "blog", Data: map[string]interface{}{"content": "1"},
	})
	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Post 2", Slug: "post-2",
		Category: "news", Data: map[string]interface{}{"content": "2"},
	})

	// List all
	all, err := svc.ListContent(ctx, false, "", nil)
	if err != nil {
		t.Fatalf("ListContent failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 items, got %d", len(all))
	}

	// Filter by category
	blogs, _ := svc.ListContent(ctx, false, "blog", nil)
	if len(blogs) != 1 {
		t.Errorf("expected 1 blog item, got %d", len(blogs))
	}
}

func TestDeleteContent_SoftDelete(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "To Delete", Slug: "delete-test",
		Data: map[string]interface{}{"content": "deleteme"},
	}
	svc.CreateContent(ctx, content)

	err := svc.DeleteContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("DeleteContent failed: %v", err)
	}

	// Should not appear in normal listing
	items, _ := svc.ListContent(ctx, false, "", nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items after soft delete, got %d", len(items))
	}

	// Should appear with includeDeleted
	items, _ = svc.ListContent(ctx, true, "", nil)
	if len(items) != 1 {
		t.Errorf("expected 1 item with includeDeleted, got %d", len(items))
	}
}

func TestRestoreContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Restore Me", Slug: "restore-test",
		Data: map[string]interface{}{"content": "restoreme"},
	}
	svc.CreateContent(ctx, content)
	svc.DeleteContent(ctx, content.ID)

	err := svc.RestoreContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("RestoreContent failed: %v", err)
	}

	items, _ := svc.ListContent(ctx, false, "", nil)
	if len(items) != 1 {
		t.Errorf("expected 1 item after restore, got %d", len(items))
	}
}

func TestVersioning(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "V1", Slug: "version-test",
		Data: map[string]interface{}{"content": "version 1"},
	}
	svc.CreateContent(ctx, content, "Initial version")

	// Update to create v2
	content.Title = "V2"
	content.Data["content"] = "version 2"
	svc.UpdateContent(ctx, content, "Second version")

	// Get versions
	versions, err := svc.GetVersions(ctx, content.ID)
	if err != nil {
		t.Fatalf("GetVersions failed: %v", err)
	}

	// Should have version 1 (initial) and version 2 (after update creates v1 from original + v2)
	if len(versions) < 2 {
		t.Fatalf("expected at least 2 versions, got %d", len(versions))
	}

	// Get specific version
	v1, err := svc.GetVersion(ctx, content.ID, 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if v1.Title != "V1" {
		t.Errorf("expected v1 title 'V1', got %q", v1.Title)
	}
}

func TestRevertToVersion(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Original Title", Slug: "revert-test",
		Data: map[string]interface{}{"content": "original"},
	}
	svc.CreateContent(ctx, content)

	// Update
	content.Title = "Changed Title"
	svc.UpdateContent(ctx, content)

	// Revert to version 1
	err := svc.RevertToVersion(ctx, content.ID, 1, "Reverting to original")
	if err != nil {
		t.Fatalf("RevertToVersion failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.Title != "Original Title" {
		t.Errorf("expected reverted title 'Original Title', got %q", got.Title)
	}
}

func TestExtractInternalLinks(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Data: map[string]interface{}{
			"content": `<p>Visit <a href="/about">About</a> and <a href="/blog/post-1">Post 1</a> and <a href="https://external.com">External</a></p>`,
		},
	}

	links := svc.extractInternalLinks(content)
	if len(links) != 2 {
		t.Fatalf("expected 2 internal links, got %d: %v", len(links), links)
	}

	linkSet := make(map[string]bool)
	for _, l := range links {
		linkSet[l] = true
	}
	if !linkSet["/about"] {
		t.Error("expected /about in internal links")
	}
	if !linkSet["/blog/post-1"] {
		t.Error("expected /blog/post-1 in internal links")
	}
}

func TestGetContent_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	_, err := svc.GetContent(context.Background(), primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestGetContentByPath_Deleted(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Deleted", Slug: "deleted-path",
		Data: map[string]interface{}{"content": "x"},
	}
	svc.CreateContent(ctx, content)
	svc.DeleteContent(ctx, content.ID)

	_, err := svc.GetContentByPath(ctx, "/deleted-path")
	if err == nil {
		t.Error("expected error for deleted content by path")
	}
}

func TestPublishUnpublish(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Pub Test", Slug: "pub-test",
		Data: map[string]interface{}{"content": "test"},
	}
	svc.CreateContent(ctx, content)

	// Publish
	err := svc.PublishContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("PublishContent failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if !got.Published {
		t.Error("expected published to be true")
	}
	if got.PublishedAt == nil {
		t.Error("expected published_at to be set")
	}

	// Unpublish
	err = svc.UnpublishContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("UnpublishContent failed: %v", err)
	}

	got, _ = svc.GetContent(ctx, content.ID)
	if got.Published {
		t.Error("expected published to be false")
	}
}

func TestGenerateStaticPage(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Static Test", Slug: "static-test",
		Data: map[string]interface{}{"content": "<p>Hello World</p>"},
	}
	svc.CreateContent(ctx, content)

	err := svc.GenerateStaticPage(ctx, content)
	if err != nil {
		t.Fatalf("GenerateStaticPage failed: %v", err)
	}

	filePath := "content/generated/static-test.html"
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("static file not found: %v", err)
	}
	if !strings.Contains(string(data), "Hello World") {
		t.Error("expected rendered content in static file")
	}
	os.Remove(filePath)
}

func TestGenerateStaticPage_RootPath(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Home", Slug: "",
		FullPath: "/",
		Data:     map[string]interface{}{"content": "Home page"},
	}
	svc.CreateContent(ctx, content)

	err := svc.GenerateStaticPage(ctx, content)
	if err != nil {
		t.Fatalf("GenerateStaticPage failed: %v", err)
	}

	filePath := "content/generated/index.html"
	_, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("index.html not found: %v", err)
	}
	os.Remove(filePath)
}

func TestRegenerateAllContent(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c1 := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Regen 1", Slug: "regen-1",
		Published: true, Data: map[string]interface{}{"content": "one"},
	}
	svc.CreateContent(ctx, c1)
	c2 := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Regen 2", Slug: "regen-2",
		Published: true, Data: map[string]interface{}{"content": "two"},
	}
	svc.CreateContent(ctx, c2)

	err := svc.RegenerateAllContent(ctx)
	if err != nil {
		t.Fatalf("RegenerateAllContent failed: %v", err)
	}

	for _, slug := range []string{"regen-1", "regen-2"} {
		filePath := "content/generated/" + slug + ".html"
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("expected static file for %s", slug)
		}
		os.Remove(filePath)
	}
}

func TestRemoveStaticPage(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	os.MkdirAll("content/generated", 0755)
	os.WriteFile("content/generated/remove-test.html", []byte("test"), 0644)

	svc.removeStaticPage("/remove-test")

	if _, err := os.Stat("content/generated/remove-test.html"); err == nil {
		t.Error("expected static file to be removed")
		os.Remove("content/generated/remove-test.html")
	}
}

func TestRemoveStaticPage_RootPath(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	os.MkdirAll("content/generated", 0755)
	os.WriteFile("content/generated/index.html", []byte("test"), 0644)

	svc.removeStaticPage("/")

	if _, err := os.Stat("content/generated/index.html"); err == nil {
		t.Error("expected index.html to be removed")
		os.Remove("content/generated/index.html")
	}
}

func TestExtractInternalLinks_NoLinks(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Data: map[string]interface{}{"content": "<p>No links here</p>"},
	}

	links := svc.extractInternalLinks(content)
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

func TestExtractInternalLinks_DedupesLinks(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Data: map[string]interface{}{
			"body1": `<a href="/about">About</a>`,
			"body2": `<a href="/about">About Again</a>`,
		},
	}

	links := svc.extractInternalLinks(content)
	if len(links) != 1 {
		t.Errorf("expected 1 deduplicated link, got %d: %v", len(links), links)
	}
}

func TestCreateContent_EmptySlug(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "No Slug", Slug: "",
		Data: map[string]interface{}{"content": "test"},
	}
	svc.CreateContent(ctx, content)

	if content.FullPath != "/" {
		t.Errorf("expected full_path '/', got %q", content.FullPath)
	}
}

func TestListContent_WithFolderFilter(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	folderID := primitive.NewObjectID()

	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "In Folder", Slug: "in-folder",
		FolderID: &folderID, Data: map[string]interface{}{"content": "1"},
	})
	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "No Folder", Slug: "no-folder",
		Data: map[string]interface{}{"content": "2"},
	})

	items, _ := svc.ListContent(ctx, false, "", &folderID)
	if len(items) != 1 {
		t.Errorf("expected 1 item in folder, got %d", len(items))
	}
}

func TestGetVersion(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "V1", Slug: "get-ver",
		Data: map[string]interface{}{"content": "v1"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "V2"
	svc.UpdateContent(ctx, content)

	v, err := svc.GetVersion(ctx, content.ID, 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if v.Title != "V1" {
		t.Errorf("expected v1 title, got %q", v.Title)
	}
}

func TestPublishContent_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	err := svc.PublishContent(context.Background(), primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestUnpublishContent_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	err := svc.UnpublishContent(context.Background(), primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestDeleteContent_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	err := svc.DeleteContent(context.Background(), primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestUpdateContent_Published(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-update"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Published Page",
		Slug:         "published-page",
		Published:    true,
		Data:         map[string]interface{}{"content": "<p>Published</p>"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "Updated Published"
	content.Data["content"] = "<p>Updated published content</p>"
	err := svc.UpdateContent(ctx, content, "Updated while published")
	if err != nil {
		t.Fatalf("UpdateContent on published content failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.Title != "Updated Published" {
		t.Errorf("expected title 'Updated Published', got %q", got.Title)
	}
}

func TestUpdateContent_Unpublish(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-unpub"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Was Published",
		Slug:         "was-published",
		Published:    true,
		Data:         map[string]interface{}{"content": "<p>Was published</p>"},
	}
	svc.CreateContent(ctx, content)

	// Unpublish it
	content.Published = false
	err := svc.UpdateContent(ctx, content, "Unpublished")
	if err != nil {
		t.Fatalf("UpdateContent to unpublish failed: %v", err)
	}
}

func TestUpdateContent_WithFolderPath(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Folder Update", Slug: "folder-update",
		FolderPath: "/blog",
		Data:       map[string]interface{}{"content": "hello"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "Updated with Folder"
	err := svc.UpdateContent(ctx, content, "folder update")
	if err != nil {
		t.Fatalf("UpdateContent with folder failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.FullPath != "/blog/folder-update" {
		t.Errorf("expected full path /blog/folder-update, got %q", got.FullPath)
	}
}

func TestRestoreContent_Published(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-restore"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Restore Published",
		Slug:         "restore-published",
		Published:    true,
		Data:         map[string]interface{}{"content": "<p>Restore me</p>"},
	}
	svc.CreateContent(ctx, content)
	svc.DeleteContent(ctx, content.ID)

	err := svc.RestoreContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("RestoreContent failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got == nil {
		t.Fatal("expected restored content")
	}
	if got.Title != "Restore Published" {
		t.Errorf("expected title 'Restore Published', got %q", got.Title)
	}
}

func TestGetVersions(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Versioned", Slug: "versioned",
		Data: map[string]interface{}{"content": "v1"},
	}
	svc.CreateContent(ctx, content)

	// Update to create v2
	content.Title = "Versioned V2"
	svc.UpdateContent(ctx, content, "second version")

	versions, err := svc.GetVersions(ctx, content.ID)
	if err != nil {
		t.Fatalf("GetVersions failed: %v", err)
	}
	if len(versions) < 2 {
		t.Errorf("expected at least 2 versions, got %d", len(versions))
	}
}

func TestListContent_IncludeDeleted(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Delete Me", Slug: "delete-me",
		Data: map[string]interface{}{"content": "delete"},
	}
	svc.CreateContent(ctx, content)
	svc.DeleteContent(ctx, content.ID)

	// Without includeDeleted
	results, _ := svc.ListContent(ctx, false, "", nil)
	for _, r := range results {
		if r.ID == content.ID {
			t.Error("deleted content should not appear in normal list")
		}
	}

	// With includeDeleted
	results, _ = svc.ListContent(ctx, true, "", nil)
	found := false
	for _, r := range results {
		if r.ID == content.ID {
			found = true
		}
	}
	if !found {
		t.Error("deleted content should appear when includeDeleted is true")
	}
}

// TestUpdateContent_TriggersV1Creation exercises the saveVersion branch
// where count == 0 && original != nil (versions were cleared before update)
func TestUpdateContent_TriggersV1Creation(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "V1 Test", Slug: "v1-test",
		Data: map[string]interface{}{"content": "original"},
	}
	svc.CreateContent(ctx, content)

	// Delete all versions to simulate a fresh state
	svc.db.Collection("content_versions").Drop(ctx)

	// Now update — saveVersion should create v1 from original, then v2 for the update
	content.Title = "V1 Test Updated"
	err := svc.UpdateContent(ctx, content, "triggers v1")
	if err != nil {
		t.Fatalf("UpdateContent failed: %v", err)
	}

	// Should have 2 versions: v1 (original) and v2 (updated)
	versions, _ := svc.GetVersions(ctx, content.ID)
	if len(versions) < 2 {
		t.Errorf("expected at least 2 versions, got %d", len(versions))
	}
}

func TestListContent_ByCategory(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Blog 1", Slug: "blog-1",
		Category: "blog", Data: map[string]interface{}{"content": "b1"},
	})
	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "News 1", Slug: "news-1",
		Category: "news", Data: map[string]interface{}{"content": "n1"},
	})

	results, err := svc.ListContent(ctx, false, "blog", nil)
	if err != nil {
		t.Fatalf("ListContent by category failed: %v", err)
	}
	for _, r := range results {
		if r.Category != "blog" {
			t.Errorf("expected only blog category, got %q", r.Category)
		}
	}
}

func TestRevertToVersion_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	err := svc.RevertToVersion(ctx, primitive.NewObjectID(), 1, "")
	if err == nil {
		t.Error("expected error reverting non-existent content")
	}
}

func TestGenerateStaticPage_BadTemplate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-bad-tmpl"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Create a template with bad syntax
	badTmpl := models.Template{
		Name:       "Bad Template",
		Slug:       "bad-template",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "text", Required: true}},
		HTMLLayout: "{{.body | nonexistentFunc}}",
	}
	svc.db.InsertOne(ctx, "templates", &badTmpl)

	content := &models.Content{
		TemplateID: badTmpl.ID,
		Title:      "Bad Content",
		Slug:       "bad-content",
		Data:       map[string]interface{}{"body": "test"},
	}
	svc.CreateContent(ctx, content)

	err := svc.GenerateStaticPage(ctx, content)
	if err == nil {
		t.Error("expected error for bad template syntax")
	}
}

func TestGenerateStaticPage_MissingTemplate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()

	content := &models.Content{
		TemplateID: primitive.NewObjectID(), // nonexistent
		Title:      "Orphaned Content",
		Slug:       "orphaned",
		Data:       map[string]interface{}{"body": "test"},
	}
	svc.CreateContent(ctx, content)

	err := svc.GenerateStaticPage(ctx, content)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestRenderContent_InvalidSyntax(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Title: "Test",
		Data:  map[string]interface{}{"body": "hello"},
	}
	tmpl := &models.Template{
		HTMLLayout: "{{.body | ",
	}

	_, err := svc.renderContent(content, tmpl)
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestRenderContent_NonStringData(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Title: "Test",
		Data: map[string]interface{}{
			"body":  "hello",
			"count": 42,
		},
	}
	tmpl := &models.Template{
		HTMLLayout: "<div>{{.body}}</div>",
	}

	html, err := svc.renderContent(content, tmpl)
	if err != nil {
		t.Fatalf("renderContent failed: %v", err)
	}
	if !strings.Contains(html, "hello") {
		t.Error("expected rendered content")
	}
}

func TestExtractInternalLinks_MultipleFields(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Data: map[string]interface{}{
			"body":    `<a href="/about">About</a> and <a href="/contact">Contact</a>`,
			"sidebar": `<a href="/faq">FAQ</a>`,
			"count":   42, // non-string, should be skipped
		},
	}

	links := svc.extractInternalLinks(content)
	if len(links) != 3 {
		t.Errorf("expected 3 links, got %d: %v", len(links), links)
	}
}

func TestExtractInternalLinks_TrailingSlash(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	content := &models.Content{
		Data: map[string]interface{}{
			"body": `<a href="/">Home</a>`,
		},
	}

	links := svc.extractInternalLinks(content)
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "/" {
		t.Errorf("expected '/', got %q", links[0])
	}
}

// RestoreContent with nonexistent ID succeeds silently (UpdateOne matches 0 docs)
// but the FindOne path returns nil — exercises the "can't regenerate" branch
func TestRestoreContent_NonexistentID(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	err := svc.RestoreContent(context.Background(), primitive.NewObjectID())
	// RestoreContent doesn't fail for nonexistent — UpdateOne matches 0, FindOne returns nil early
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetContentByPath_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	_, err := svc.GetContentByPath(context.Background(), "/nonexistent-path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestGetVersion_NotFound(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	_, err := svc.GetVersion(context.Background(), primitive.NewObjectID(), 999)
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestRevertToVersion_Success(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-revert"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Revert Me",
		Slug:         "revert-me",
		Published:    true,
		Data:         map[string]interface{}{"content": "v1 data"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "Revert Me V2"
	content.Data["content"] = "v2 data"
	svc.UpdateContent(ctx, content, "second version")

	err := svc.RevertToVersion(ctx, content.ID, 1, "reverting to v1")
	if err != nil {
		t.Fatalf("RevertToVersion failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.Title != "Revert Me" {
		t.Errorf("expected reverted title 'Revert Me', got %q", got.Title)
	}
}

func TestUpdateContent_EmptySlug(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Home", Slug: "",
		Data: map[string]interface{}{"content": "home"},
	}
	svc.CreateContent(ctx, content)

	content.Title = "Updated Home"
	err := svc.UpdateContent(ctx, content, "update home")
	if err != nil {
		t.Fatalf("UpdateContent empty slug failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.FullPath != "/" {
		t.Errorf("expected full path /, got %q", got.FullPath)
	}
}

func TestUpdateContent_NoVersionComment(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "No Comment", Slug: "no-comment",
		Data: map[string]interface{}{"content": "original"},
	}
	svc.CreateContent(ctx, content)

	// Update without version comment
	content.Title = "Updated No Comment"
	err := svc.UpdateContent(ctx, content)
	if err != nil {
		t.Fatalf("UpdateContent without comment failed: %v", err)
	}
}

func TestRemoveStaticPage_EmptyPath(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-remove-empty"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	os.WriteFile(tmpDir+"/content/generated/index.html", []byte("test"), 0644)

	// Empty string should be treated as root ("/index")
	svc.removeStaticPage("")

	if _, err := os.Stat(tmpDir + "/content/generated/index.html"); err == nil {
		t.Error("expected index.html to be removed for empty path")
	}
}

func TestPublishContent_Success(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-publish"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Publish Me", Slug: "publish-me",
		Data: map[string]interface{}{"content": "<p>publish</p>"},
	}
	svc.CreateContent(ctx, content)

	err := svc.PublishContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("PublishContent failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if !got.Published {
		t.Error("expected content to be published")
	}
	if got.PublishedAt == nil {
		t.Error("expected published_at to be set")
	}
}

func TestUnpublishContent_Success(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-unpublish-s"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Unpublish Me", Slug: "unpublish-me",
		Published: true, Data: map[string]interface{}{"content": "<p>remove</p>"},
	}
	svc.CreateContent(ctx, content)

	err := svc.UnpublishContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("UnpublishContent failed: %v", err)
	}

	got, _ := svc.GetContent(ctx, content.ID)
	if got.Published {
		t.Error("expected content to be unpublished")
	}
}

func TestDeleteContent_RemovesStaticPage(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-del-static"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template", Title: "Delete Static", Slug: "delete-static",
		Published: true, Data: map[string]interface{}{"content": "<p>delete</p>"},
	}
	svc.CreateContent(ctx, content)
	svc.GenerateStaticPage(ctx, content)

	err := svc.DeleteContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("DeleteContent failed: %v", err)
	}

	// Static page should be removed
	if _, err := os.Stat(tmpDir + "/content/generated/delete-static.html"); err == nil {
		t.Error("expected static file to be removed after deletion")
	}
}

func TestRegenerateAllContent_WithBadTemplate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-regen-bad"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	// One good and one with nonexistent template
	svc.CreateContent(ctx, &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "Good", Slug: "regen-good",
		Published: true, Data: map[string]interface{}{"content": "ok"},
	})
	svc.CreateContent(ctx, &models.Content{
		TemplateID: primitive.NewObjectID(), TemplateName: "Missing", Title: "Bad", Slug: "regen-bad",
		Published: true, Data: map[string]interface{}{"content": "orphaned"},
	})

	// Should not fail — logs warnings for bad content, continues for good
	err := svc.RegenerateAllContent(ctx)
	if err != nil {
		t.Fatalf("RegenerateAllContent failed: %v", err)
	}

	// Good content should have been regenerated
	if _, err := os.Stat(tmpDir + "/content/generated/regen-good.html"); err != nil {
		t.Error("expected good content to be regenerated")
	}
}

func TestGetBacklinks(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-backlinks"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(oldDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	// Create a target page and a linking page
	target := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Target Page", Slug: "target-page",
	}
	if err := svc.CreateContent(ctx, target); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := svc.PublishContent(ctx, target.ID); err != nil {
		t.Fatalf("publish target: %v", err)
	}

	linker := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Linking Page", Slug: "linking-page",
		Data: map[string]interface{}{"content": `<a href="/target-page">link</a>`},
	}
	if err := svc.CreateContent(ctx, linker); err != nil {
		t.Fatalf("create linker: %v", err)
	}
	if err := svc.PublishContent(ctx, linker.ID); err != nil {
		t.Fatalf("publish linker: %v", err)
	}

	backlinks, err := svc.GetBacklinks(ctx, "/target-page")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}
	found := false
	for _, b := range backlinks {
		if b.FullPath == "/linking-page" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected linking-page in backlinks, got: %+v", backlinks)
	}
}

func TestGetBacklinks_Empty(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	backlinks, err := svc.GetBacklinks(ctx, "/no-such-page")
	if err != nil {
		t.Fatalf("GetBacklinks: %v", err)
	}
	if len(backlinks) != 0 {
		t.Errorf("expected no backlinks, got: %v", backlinks)
	}
}

func TestUpdateWikilinksOnRename_Title(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	// Create a page whose data contains a wikilink to the old title
	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Other Page", Slug: "other-page",
		Data: map[string]interface{}{"content": "See [[Old Title]] for more."},
	}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	svc.UpdateWikilinksOnRename(ctx, "Old Title", "New Title", "", "")

	// Reload and check
	updated, err := svc.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	body, _ := updated.Data["content"].(string)
	if strings.Contains(body, "[[Old Title]]") {
		t.Errorf("old title should be rewritten, got: %s", body)
	}
	if !strings.Contains(body, "[[New Title]]") {
		t.Errorf("expected new title in content, got: %s", body)
	}
}

func TestUpdateWikilinksOnRename_Path(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Other Page", Slug: "other-page",
		Data: map[string]interface{}{"content": "Link to [[/old/path]] here."},
	}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	svc.UpdateWikilinksOnRename(ctx, "", "", "/old/path", "/new/path")

	updated, err := svc.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	body, _ := updated.Data["content"].(string)
	if strings.Contains(body, "[[/old/path]]") {
		t.Errorf("old path should be rewritten, got: %s", body)
	}
	if !strings.Contains(body, "[[/new/path]]") {
		t.Errorf("expected new path in content, got: %s", body)
	}
}

func TestUpdateWikilinksOnRename_NoChange(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Other Page", Slug: "other-page",
		Data: map[string]interface{}{"content": "No wikilinks here."},
	}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Same title — should be a no-op
	svc.UpdateWikilinksOnRename(ctx, "My Page", "My Page", "/same", "/same")

	updated, err := svc.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	body, _ := updated.Data["content"].(string)
	if body != "No wikilinks here." {
		t.Errorf("expected content unchanged, got: %s", body)
	}
}

func TestMergeInlineTags_OnCreate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Tagged Post", Slug: "tagged-post",
		Data: map[string]interface{}{"content": "Post about #golang and #webdev."},
	}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	loaded, err := svc.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if !containsTag(loaded.Tags, "golang") {
		t.Errorf("expected golang tag auto-detected on create, got: %v", loaded.Tags)
	}
	if !containsTag(loaded.Tags, "webdev") {
		t.Errorf("expected webdev tag auto-detected on create, got: %v", loaded.Tags)
	}
}

func TestMergeInlineTags_OnUpdate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c := &models.Content{
		TemplateID: tmplID, TemplateName: "Test Template",
		Title: "Update Tags Test", Slug: "update-tags",
		Data: map[string]interface{}{"content": "Initial content."},
	}
	if err := svc.CreateContent(ctx, c); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	update := &models.Content{
		ID:   c.ID,
		Data: map[string]interface{}{"content": "Now about #testing frameworks."},
	}
	if err := svc.UpdateContent(ctx, update); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	loaded, err := svc.GetContent(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetContent: %v", err)
	}
	if !containsTag(loaded.Tags, "testing") {
		t.Errorf("expected testing tag after update, got: %v", loaded.Tags)
	}
}

func TestListContentScoped_Empty(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	results, err := svc.ListContentScoped(context.Background(), ContentScope{})
	if err != nil {
		t.Fatalf("ListContentScoped: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestListContentScoped_WithTemplate(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Scoped Test",
		Slug:         "scoped-test",
		FullPath:     "/scoped-test",
	}
	svc.CreateContent(ctx, content)

	results, err := svc.ListContentScoped(ctx, ContentScope{TemplateName: "Test Template"})
	if err != nil {
		t.Fatalf("ListContentScoped: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetContentByIDs_Empty(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	result, err := svc.GetContentByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetContentByIDs: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestGetContentByIDs_Found(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	ctx := context.Background()
	tmplID := createTestTemplate(t, svc)

	c1 := &models.Content{TemplateID: tmplID, Title: "A", Slug: "a", FullPath: "/a"}
	c2 := &models.Content{TemplateID: tmplID, Title: "B", Slug: "b", FullPath: "/b"}
	svc.CreateContent(ctx, c1)
	svc.CreateContent(ctx, c2)

	result, err := svc.GetContentByIDs(ctx, []primitive.ObjectID{c1.ID, c2.ID})
	if err != nil {
		t.Fatalf("GetContentByIDs: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestWithEditorEmail(t *testing.T) {
	ctx := WithEditorEmail(context.Background(), "editor@test.com")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestSetSearchService(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()

	// Should not panic
	svc.SetSearchService(nil)
}

func TestContentService_QueryContentForDirective_Empty(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	ctx := context.Background()

	results, err := svc.QueryContentForDirective(ctx, map[string]string{}, "title", "asc")
	if err != nil {
		t.Fatalf("QueryContentForDirective: %v", err)
	}
	// No published content, should return empty
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestContentService_QueryContentForDirective_WithFilter(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := createTestTemplate(t, svc)
	// Seed published content
	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Blog Post A",
		Slug:         "blog-post-a",
		FullPath:     "/blog/blog-post-a",
		Published:    true,
		Category:     "blog",
		Data:         map[string]interface{}{"content": "hello"},
	}
	svc.CreateContent(ctx, content, "admin")

	results, err := svc.QueryContentForDirective(ctx, map[string]string{"category": "blog"}, "title", "asc")
	if err != nil {
		t.Fatalf("QueryContentForDirective with filter: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestContentService_QueryContentForDirective_SortByPublishedAt(t *testing.T) {
	svc, cleanup := newTestContentService(t)
	defer cleanup()
	ctx := context.Background()

	tmplID := createTestTemplate(t, svc)
	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Test Template",
		Title:        "Sort Test",
		Slug:         "sort-test-pat",
		FullPath:     "/sort-test-pat",
		Published:    true,
		Data:         map[string]interface{}{"content": "hello"},
	}
	svc.CreateContent(ctx, content, "admin")

	// Sort by published_at desc should not panic
	_, err := svc.QueryContentForDirective(ctx, map[string]string{}, "published_at", "desc")
	if err != nil {
		t.Fatalf("QueryContentForDirective sort by published_at: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GenerateStaticPage with lc:query directive (triggers processQueryDirectives,
// buildWikilinkIndex, and related functions)
// ---------------------------------------------------------------------------

func TestContentService_GenerateStaticPage_WithQueryDirective(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewContentService(db)
	ctx := context.Background()

	// Create a template with an lc:query directive
	tmpl := models.Template{
		Name:       "Index Template",
		Slug:       "index-tmpl",
		Fields:     []models.TemplateField{},
		HTMLLayout: `<div><!-- lc:query category="news" sort="title:asc" --></div>`,
	}
	tmplID, _ := svc.db.InsertOne(ctx, "templates", tmpl)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Index Template",
		Title:        "Query Page",
		Slug:         "query-page",
		FullPath:     "/query-page",
		Published:    true,
		Data:         map[string]interface{}{},
	}
	if err := svc.CreateContent(ctx, content, "admin"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// GenerateStaticPage processes lc:query directives
	if err := svc.GenerateStaticPage(ctx, content); err != nil {
		t.Fatalf("GenerateStaticPage with lc:query: %v", err)
	}
}

func TestContentService_GenerateStaticPage_WithWikilinks(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewContentService(db)
	ctx := context.Background()

	// Create template with a wikilink placeholder
	tmpl := models.Template{
		Name:       "Wiki Template",
		Slug:       "wiki-tmpl",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "richtext"}},
		HTMLLayout: `<div>{{.body}}</div>`,
	}
	tmplID, _ := svc.db.InsertOne(ctx, "templates", tmpl)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Wiki Template",
		Title:        "Wiki Page",
		Slug:         "wiki-page",
		FullPath:     "/wiki-page",
		Published:    true,
		Data:         map[string]interface{}{"body": "See [[Home Page]] for details."},
	}
	if err := svc.CreateContent(ctx, content, "admin"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	if err := svc.GenerateStaticPage(ctx, content); err != nil {
		t.Fatalf("GenerateStaticPage with wikilinks: %v", err)
	}
}

func TestContentService_GenerateStaticPage_AdminOnlyPolicy(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()
	svc := NewContentService(db)
	ctx := context.Background()

	// Set admin_only markdown script policy
	db.Collection("settings").InsertOne(ctx, map[string]interface{}{
		"type":                   "site_config",
		"markdown_script_policy": "admin_only",
	})

	tmpl := models.Template{
		Name:       "Markdown Policy Template",
		Slug:       "mdpolicy-tmpl",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "markdown"}},
		HTMLLayout: `<div>{{.body}}</div>`,
	}
	tmplID, _ := svc.db.InsertOne(ctx, "templates", tmpl)

	content := &models.Content{
		TemplateID:   tmplID,
		TemplateName: "Markdown Policy Template",
		Title:        "MD Policy Page",
		Slug:         "md-policy-page",
		FullPath:     "/md-policy-page",
		Published:    true,
		Data:         map[string]interface{}{"body": "## Hello\n\n<script>alert(1)</script>"},
	}
	if err := svc.CreateContent(ctx, content, "admin"); err != nil {
		t.Fatalf("CreateContent: %v", err)
	}

	// Should succeed — triggers getContentAuthorRole (no version → "admin")
	if err := svc.GenerateStaticPage(ctx, content); err != nil {
		t.Fatalf("GenerateStaticPage admin_only policy: %v", err)
	}
}

func TestRenderSnippet_Valid(t *testing.T) {
	item := models.Content{
		Title:    "My Page",
		FullPath: "/my-page",
	}
	html, err := renderSnippet(`<a href="{{.FullPath}}">{{.Title}}</a>`, item)
	if err != nil {
		t.Fatalf("renderSnippet: %v", err)
	}
	if !strings.Contains(html, "/my-page") {
		t.Fatalf("expected /my-page in output: %s", html)
	}
}

func TestRenderSnippet_InvalidTemplate(t *testing.T) {
	_, err := renderSnippet(`{{.Missing`, models.Content{})
	if err == nil {
		t.Error("expected error for invalid template")
	}
}
