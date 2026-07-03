package services

import (
	"context"
	"os"
	"testing"

	"github.com/jonradoff/lightcms/v6/internal/models"
	"github.com/jonradoff/lightcms/v6/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestTemplateService(t *testing.T) (*TemplateService, *ContentService, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	contentSvc := NewContentService(db)
	tmplSvc := NewTemplateService(db, contentSvc)
	return tmplSvc, contentSvc, cleanup
}

func TestCreateTemplate(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name: "Blog Post",
		Slug: "blog-post",
		Fields: []models.TemplateField{
			{Name: "content", Label: "Content", Type: "richtext", Required: true},
			{Name: "author", Label: "Author", Type: "text", Required: false},
		},
		HTMLLayout: "<article>{{.content}}</article>",
	}

	err := svc.CreateTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	if tmpl.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
}

func TestGetTemplate(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "Get Test",
		Slug:       "get-test",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "richtext", Required: true}},
		HTMLLayout: "<div>{{.body}}</div>",
	}
	svc.CreateTemplate(ctx, tmpl)

	got, err := svc.GetTemplate(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if got.Name != "Get Test" {
		t.Errorf("expected name 'Get Test', got %q", got.Name)
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	_, err := svc.GetTemplate(context.Background(), primitive.NewObjectID())
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestGetTemplateBySlug(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "Slug Test",
		Slug:       "slug-test",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "text", Required: true}},
		HTMLLayout: "{{.body}}",
	}
	svc.CreateTemplate(ctx, tmpl)

	got, err := svc.GetTemplateBySlug(ctx, "slug-test")
	if err != nil {
		t.Fatalf("GetTemplateBySlug failed: %v", err)
	}
	if got.Name != "Slug Test" {
		t.Errorf("expected name 'Slug Test', got %q", got.Name)
	}
}

func TestGetTemplateBySlug_NotFound(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	_, err := svc.GetTemplateBySlug(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent slug")
	}
}

func TestListTemplates(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	svc.CreateTemplate(ctx, &models.Template{Name: "Alpha", Slug: "alpha", Fields: []models.TemplateField{{Name: "x", Label: "X", Type: "text", Required: false}}, HTMLLayout: "{{.x}}"})
	svc.CreateTemplate(ctx, &models.Template{Name: "Beta", Slug: "beta", Fields: []models.TemplateField{{Name: "y", Label: "Y", Type: "text", Required: false}}, HTMLLayout: "{{.y}}"})

	templates, err := svc.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	// Sorted by name
	if templates[0].Name != "Alpha" {
		t.Errorf("expected first template 'Alpha', got %q", templates[0].Name)
	}
}

func TestUpdateTemplate(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "Original",
		Slug:       "original",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "text", Required: true}},
		HTMLLayout: "{{.body}}",
	}
	svc.CreateTemplate(ctx, tmpl)

	tmpl.Name = "Updated"
	tmpl.HTMLLayout = "<p>{{.body}}</p>"
	err := svc.UpdateTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}

	got, _ := svc.GetTemplate(ctx, tmpl.ID)
	if got.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", got.Name)
	}
}

func TestDeleteTemplate(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "To Delete",
		Slug:       "to-delete",
		Fields:     []models.TemplateField{{Name: "x", Label: "X", Type: "text", Required: false}},
		HTMLLayout: "{{.x}}",
	}
	svc.CreateTemplate(ctx, tmpl)

	err := svc.DeleteTemplate(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	// Should be gone
	_, err = svc.GetTemplate(ctx, tmpl.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestDeleteTemplate_SystemTemplate(t *testing.T) {
	svc, _, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "System",
		Slug:       "system",
		IsSystem:   true,
		Fields:     []models.TemplateField{{Name: "x", Label: "X", Type: "text", Required: false}},
		HTMLLayout: "{{.x}}",
	}
	svc.CreateTemplate(ctx, tmpl)

	err := svc.DeleteTemplate(ctx, tmpl.ID)
	if err == nil {
		t.Error("expected error deleting system template")
	}
}

func TestDeleteTemplate_InUse(t *testing.T) {
	svc, contentSvc, cleanup := newTestTemplateService(t)
	defer cleanup()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "In Use",
		Slug:       "in-use",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "text", Required: true}},
		HTMLLayout: "{{.body}}",
	}
	svc.CreateTemplate(ctx, tmpl)

	// Create content using this template
	content := &models.Content{
		TemplateID: tmpl.ID, TemplateName: "In Use", Title: "Test", Slug: "tmpl-test",
		Data: map[string]interface{}{"body": "hello"},
	}
	contentSvc.CreateContent(ctx, content)

	err := svc.DeleteTemplate(ctx, tmpl.ID)
	if err == nil {
		t.Error("expected error deleting template in use")
	}
}

func TestUpdateTemplate_RegeneratesContent(t *testing.T) {
	svc, contentSvc, cleanup := newTestTemplateService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-tmpl-regen"
	os.MkdirAll(tmpDir+"/content/generated", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	tmpl := &models.Template{
		Name:       "Regen Template",
		Slug:       "regen-template",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "richtext", Required: true}},
		HTMLLayout: "<div>{{.body}}</div>",
	}
	svc.CreateTemplate(ctx, tmpl)

	// Create published content using this template
	content := &models.Content{
		TemplateID:   tmpl.ID,
		TemplateName: "Regen Template",
		Title:        "Regen Content",
		Slug:         "regen-content",
		Published:    true,
		Data:         map[string]interface{}{"body": "<p>Hello</p>"},
	}
	contentSvc.CreateContent(ctx, content)

	// Update template layout — should trigger regeneration of published content
	tmpl.HTMLLayout = "<section>{{.body}}</section>"
	err := svc.UpdateTemplate(ctx, tmpl)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}

	// Verify the template was updated
	got, _ := svc.GetTemplate(ctx, tmpl.ID)
	if got.HTMLLayout != "<section>{{.body}}</section>" {
		t.Errorf("expected updated layout, got %q", got.HTMLLayout)
	}
}
