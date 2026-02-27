package services

import (
	"context"
	"os"
	"strings"
	"testing"

	"lightcms/internal/database"
	"lightcms/internal/models"
	"lightcms/internal/testutil"
)

func newTestSettingsService(t *testing.T) (*SettingsService, func()) {
	t.Helper()
	db, cleanup := testutil.MustConnectTestDB(t)
	contentSvc := NewContentService(db)
	settingsSvc := NewSettingsService(db, contentSvc)
	return settingsSvc, cleanup
}

func TestGetTheme_Defaults(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	theme, err := svc.GetTheme(context.Background())
	if err != nil {
		t.Fatalf("GetTheme failed: %v", err)
	}

	if theme.PrimaryColor == "" {
		t.Error("expected default primary color")
	}
	if theme.SiteName == "" {
		t.Error("expected default site name")
	}
	if theme.FontFamily == "" {
		t.Error("expected default font family")
	}
}

func TestUpdateTheme(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	theme := &database.ThemeSettings{
		PrimaryColor:    "#ff0000",
		SecondaryColor:  "#00ff00",
		AccentColor:     "#0000ff",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		FontFamily:      "Arial",
		HeadingFont:     "Georgia",
		BorderRadius:    "8px",
		SiteName:        "Test Site",
		SiteTagline:     "Testing",
	}

	err := svc.UpdateTheme(ctx, theme)
	if err != nil {
		t.Fatalf("UpdateTheme failed: %v", err)
	}

	// Retrieve and verify
	saved, err := svc.GetTheme(ctx)
	if err != nil {
		t.Fatalf("GetTheme after update failed: %v", err)
	}
	if saved.PrimaryColor != "#ff0000" {
		t.Errorf("expected primary color #ff0000, got %s", saved.PrimaryColor)
	}
	if saved.SiteName != "Test Site" {
		t.Errorf("expected site name 'Test Site', got %s", saved.SiteName)
	}
}

func TestThemeVersioning(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	// Ensure initial version
	err := svc.EnsureThemeVersion1(ctx)
	if err != nil {
		t.Fatalf("EnsureThemeVersion1 failed: %v", err)
	}

	versions, err := svc.GetThemeVersions(ctx)
	if err != nil {
		t.Fatalf("GetThemeVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	// Calling again should be idempotent
	err = svc.EnsureThemeVersion1(ctx)
	if err != nil {
		t.Fatalf("second EnsureThemeVersion1 failed: %v", err)
	}
	versions, _ = svc.GetThemeVersions(ctx)
	if len(versions) != 1 {
		t.Fatalf("expected still 1 version, got %d", len(versions))
	}
}

func TestGetSiteConfig_Defaults(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	config, err := svc.GetSiteConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSiteConfig failed: %v", err)
	}

	if config.TitleTemplate == "" {
		t.Error("expected default title template")
	}
	if config.TitleTemplateNoTitle == "" {
		t.Error("expected default title template no title")
	}
}

func TestUpdateSiteConfig(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	config := &database.SiteConfig{
		TitleTemplate:        "{{title}} | {{site_name}}",
		TitleTemplateNoTitle: "{{site_name}} - Home",
	}

	err := svc.UpdateSiteConfig(ctx, config)
	if err != nil {
		t.Fatalf("UpdateSiteConfig failed: %v", err)
	}

	saved, _ := svc.GetSiteConfig(ctx)
	if saved.TitleTemplate != "{{title}} | {{site_name}}" {
		t.Errorf("expected updated title template, got %s", saved.TitleTemplate)
	}
}

// Redirect tests

func TestCreateRedirect(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	redirect := &models.Redirect{
		FromPath:   "/old-page",
		ToPath:     "/new-page",
		StatusCode: 301,
	}

	err := svc.CreateRedirect(ctx, redirect)
	if err != nil {
		t.Fatalf("CreateRedirect failed: %v", err)
	}

	if redirect.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
}

func TestCreateRedirect_DefaultStatusCode(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	redirect := &models.Redirect{
		FromPath:   "/old",
		ToPath:     "/new",
		StatusCode: 999, // Invalid
	}

	svc.CreateRedirect(context.Background(), redirect)

	// Should default to 301
	got, _ := svc.GetRedirect(context.Background(), redirect.ID)
	if got.StatusCode != 301 {
		t.Errorf("expected default 301, got %d", got.StatusCode)
	}
}

func TestListRedirects(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	svc.CreateRedirect(ctx, &models.Redirect{FromPath: "/b", ToPath: "/x", StatusCode: 301})
	svc.CreateRedirect(ctx, &models.Redirect{FromPath: "/a", ToPath: "/y", StatusCode: 302})

	redirects, err := svc.ListRedirects(ctx)
	if err != nil {
		t.Fatalf("ListRedirects failed: %v", err)
	}

	if len(redirects) != 2 {
		t.Fatalf("expected 2 redirects, got %d", len(redirects))
	}

	// Should be sorted by from_path
	if redirects[0].FromPath != "/a" {
		t.Errorf("expected sorted by from_path, first is %s", redirects[0].FromPath)
	}
}

func TestUpdateRedirect(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	redirect := &models.Redirect{FromPath: "/old", ToPath: "/new", StatusCode: 301}
	svc.CreateRedirect(ctx, redirect)

	redirect.ToPath = "/updated"
	redirect.StatusCode = 302
	err := svc.UpdateRedirect(ctx, redirect)
	if err != nil {
		t.Fatalf("UpdateRedirect failed: %v", err)
	}

	got, _ := svc.GetRedirect(ctx, redirect.ID)
	if got.ToPath != "/updated" {
		t.Errorf("expected updated to_path, got %s", got.ToPath)
	}
	if got.StatusCode != 302 {
		t.Errorf("expected 302, got %d", got.StatusCode)
	}
}

func TestDeleteRedirect(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	redirect := &models.Redirect{FromPath: "/old", ToPath: "/new", StatusCode: 301}
	svc.CreateRedirect(ctx, redirect)

	err := svc.DeleteRedirect(ctx, redirect.ID)
	if err != nil {
		t.Fatalf("DeleteRedirect failed: %v", err)
	}

	redirects, _ := svc.ListRedirects(ctx)
	if len(redirects) != 0 {
		t.Errorf("expected 0 redirects after deletion, got %d", len(redirects))
	}
}

// Folder tests

func TestCreateFolder(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	folder := &models.Folder{Name: "Blog", Slug: "blog"}
	err := svc.CreateFolder(ctx, folder)
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	if folder.Path != "/blog" {
		t.Errorf("expected path /blog, got %s", folder.Path)
	}
	if folder.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
}

func TestCreateFolder_Nested(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	parent := &models.Folder{Name: "Blog", Slug: "blog"}
	svc.CreateFolder(ctx, parent)

	child := &models.Folder{Name: "2024", Slug: "2024", ParentID: &parent.ID}
	err := svc.CreateFolder(ctx, child)
	if err != nil {
		t.Fatalf("CreateFolder nested failed: %v", err)
	}

	if child.Path != "/blog/2024" {
		t.Errorf("expected path /blog/2024, got %s", child.Path)
	}
}

func TestListFolders(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	svc.CreateFolder(ctx, &models.Folder{Name: "Blog", Slug: "blog"})
	svc.CreateFolder(ctx, &models.Folder{Name: "About", Slug: "about"})

	folders, err := svc.ListFolders(ctx)
	if err != nil {
		t.Fatalf("ListFolders failed: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
}

func TestDeleteFolder(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	folder := &models.Folder{Name: "Empty", Slug: "empty"}
	svc.CreateFolder(ctx, folder)

	err := svc.DeleteFolder(ctx, folder.ID)
	if err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	folders, _ := svc.ListFolders(ctx)
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestDeleteFolder_WithSubfolders(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	parent := &models.Folder{Name: "Parent", Slug: "parent"}
	svc.CreateFolder(ctx, parent)

	child := &models.Folder{Name: "Child", Slug: "child", ParentID: &parent.ID}
	svc.CreateFolder(ctx, child)

	err := svc.DeleteFolder(ctx, parent.ID)
	if err == nil {
		t.Error("expected error when deleting folder with subfolders")
	}
}

// Collection tests

func TestCreateCollection(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	coll := &models.Collection{
		Name:     "Blog Posts",
		Slug:     "blog-posts",
		Category: "blog",
	}

	err := svc.CreateCollection(ctx, coll)
	if err != nil {
		t.Fatalf("CreateCollection failed: %v", err)
	}

	if coll.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
}

func TestListCollections(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	svc.CreateCollection(ctx, &models.Collection{Name: "Blog Posts", Slug: "blog", Category: "blog"})
	svc.CreateCollection(ctx, &models.Collection{Name: "Press", Slug: "press", Category: "press"})

	collections, err := svc.ListCollections(ctx)
	if err != nil {
		t.Fatalf("ListCollections failed: %v", err)
	}

	if len(collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(collections))
	}
}

func TestUpdateCollection(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	coll := &models.Collection{Name: "Blog", Slug: "blog", Category: "blog"}
	svc.CreateCollection(ctx, coll)

	coll.Name = "Updated Blog"
	coll.ItemsPerPage = 10
	err := svc.UpdateCollection(ctx, coll)
	if err != nil {
		t.Fatalf("UpdateCollection failed: %v", err)
	}

	got, _ := svc.GetCollection(ctx, coll.ID)
	if got.Name != "Updated Blog" {
		t.Errorf("expected updated name, got %s", got.Name)
	}
	if got.ItemsPerPage != 10 {
		t.Errorf("expected items_per_page 10, got %d", got.ItemsPerPage)
	}
}

func TestDeleteCollection(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	coll := &models.Collection{Name: "Blog", Slug: "blog", Category: "blog"}
	svc.CreateCollection(ctx, coll)

	err := svc.DeleteCollection(ctx, coll.ID)
	if err != nil {
		t.Fatalf("DeleteCollection failed: %v", err)
	}

	collections, _ := svc.ListCollections(ctx)
	if len(collections) != 0 {
		t.Errorf("expected 0 collections, got %d", len(collections))
	}
}

// Theme version revert and get tests

func TestGetThemeVersion(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial version
	svc.EnsureThemeVersion1(ctx)

	v, err := svc.GetThemeVersion(ctx, 1)
	if err != nil {
		t.Fatalf("GetThemeVersion failed: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("expected version 1, got %d", v.Version)
	}
}

func TestRevertThemeToVersion(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	// Set initial theme
	theme := &database.ThemeSettings{
		PrimaryColor:    "#111111",
		SecondaryColor:  "#222222",
		AccentColor:     "#333333",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		FontFamily:      "Arial",
		HeadingFont:     "Georgia",
		BorderRadius:    "4px",
		SiteName:        "Original Site",
	}
	svc.UpdateTheme(ctx, theme)

	// Change the theme
	theme2 := &database.ThemeSettings{
		PrimaryColor:    "#aaaaaa",
		SecondaryColor:  "#bbbbbb",
		AccentColor:     "#cccccc",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		FontFamily:      "Helvetica",
		HeadingFont:     "Times",
		BorderRadius:    "8px",
		SiteName:        "Changed Site",
	}
	svc.UpdateTheme(ctx, theme2)

	// Revert to version 2 (v1 is auto-saved defaults, v2 is the first custom theme)
	err := svc.RevertThemeToVersion(ctx, 2, "Reverting to original")
	if err != nil {
		t.Fatalf("RevertThemeToVersion failed: %v", err)
	}

	// Verify reverted values match the first custom theme
	current, _ := svc.GetTheme(ctx)
	if current.PrimaryColor != "#111111" {
		t.Errorf("expected reverted primary color #111111, got %s", current.PrimaryColor)
	}
	if current.SiteName != "Original Site" {
		t.Errorf("expected reverted site name 'Original Site', got %s", current.SiteName)
	}
}

func TestGetFolder(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	folder := &models.Folder{Name: "Blog", Slug: "blog"}
	svc.CreateFolder(ctx, folder)

	got, err := svc.GetFolder(ctx, folder.ID)
	if err != nil {
		t.Fatalf("GetFolder failed: %v", err)
	}
	if got.Name != "Blog" {
		t.Errorf("expected name 'Blog', got %q", got.Name)
	}
	if got.Path != "/blog" {
		t.Errorf("expected path /blog, got %q", got.Path)
	}
}

func TestGetCollection(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	coll := &models.Collection{Name: "News", Slug: "news", Category: "news"}
	svc.CreateCollection(ctx, coll)

	got, err := svc.GetCollection(ctx, coll.ID)
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}
	if got.Name != "News" {
		t.Errorf("expected name 'News', got %q", got.Name)
	}
}

func TestGetRedirect(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	redirect := &models.Redirect{
		FromPath:    "/old-page",
		ToPath:      "/new-page",
		StatusCode:  301,
		Description: "Moved",
	}
	svc.CreateRedirect(ctx, redirect)

	got, err := svc.GetRedirect(ctx, redirect.ID)
	if err != nil {
		t.Fatalf("GetRedirect failed: %v", err)
	}
	if got.FromPath != "/old-page" {
		t.Errorf("expected from_path /old-page, got %q", got.FromPath)
	}
	if got.Description != "Moved" {
		t.Errorf("expected description 'Moved', got %q", got.Description)
	}
}

func TestUpdateTheme_WithVersionComment(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	theme := &database.ThemeSettings{
		PrimaryColor:    "#ff0000",
		SecondaryColor:  "#00ff00",
		AccentColor:     "#0000ff",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		FontFamily:      "Arial",
		HeadingFont:     "Georgia",
		BorderRadius:    "4px",
		SiteName:        "Test",
	}

	err := svc.UpdateTheme(ctx, theme, "Initial setup")
	if err != nil {
		t.Fatalf("UpdateTheme with comment failed: %v", err)
	}

	versions, _ := svc.GetThemeVersions(ctx)
	if len(versions) == 0 {
		t.Fatal("expected at least 1 version")
	}
}

func TestDeleteFolder_WithContent(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	ctx := context.Background()

	folder := &models.Folder{Name: "Has Content", Slug: "has-content"}
	svc.CreateFolder(ctx, folder)

	// Create content in this folder
	contentSvc := svc.contentService
	tmpl := models.Template{
		Name:       "Test",
		Slug:       "test-tmpl-del",
		Fields:     []models.TemplateField{{Name: "body", Label: "Body", Type: "text", Required: true}},
		HTMLLayout: "{{.body}}",
	}
	tmplID, _ := svc.db.InsertOne(ctx, "templates", tmpl)

	content := &models.Content{
		TemplateID: tmplID, TemplateName: "Test", Title: "In Folder", Slug: "in-folder",
		FolderID: &folder.ID,
		Data:     map[string]interface{}{"body": "test"},
	}
	contentSvc.CreateContent(ctx, content)

	err := svc.DeleteFolder(ctx, folder.ID)
	if err == nil {
		t.Error("expected error deleting folder with content")
	}
}

func TestUpdateTheme_HeaderFooterChange(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-theme-hf"
	os.MkdirAll(tmpDir+"/static/css", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// First update with custom header/footer
	theme1 := &database.ThemeSettings{
		PrimaryColor: "#111111",
		HeaderHTML:   "<header>Original</header>",
		FooterHTML:   "<footer>Original</footer>",
		FontFamily:   "Arial",
	}
	svc.UpdateTheme(ctx, theme1)

	// Update with different header — should trigger content regeneration
	theme2 := &database.ThemeSettings{
		PrimaryColor: "#111111",
		HeaderHTML:   "<header>Changed</header>",
		FooterHTML:   "<footer>Original</footer>",
		FontFamily:   "Arial",
	}
	err := svc.UpdateTheme(ctx, theme2)
	if err != nil {
		t.Fatalf("UpdateTheme with header change failed: %v", err)
	}

	got, _ := svc.GetTheme(ctx)
	if got.HeaderHTML != "<header>Changed</header>" {
		t.Errorf("expected changed header, got %q", got.HeaderHTML)
	}
}

func TestUpdateTheme_CustomCSS(t *testing.T) {
	svc, cleanup := newTestSettingsService(t)
	defer cleanup()

	tmpDir := os.TempDir() + "/lightcms-test-theme-css"
	os.MkdirAll(tmpDir+"/static/css", 0755)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	theme := &database.ThemeSettings{
		PrimaryColor:    "#333333",
		SecondaryColor:  "#666666",
		AccentColor:     "#999999",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		FontFamily:      "sans-serif",
		HeadingFont:     "serif",
		BorderRadius:    "4px",
		CustomCSS:       ".custom { color: red; }",
	}
	err := svc.UpdateTheme(ctx, theme)
	if err != nil {
		t.Fatalf("UpdateTheme with custom CSS failed: %v", err)
	}

	// Verify CSS file was generated
	data, err := os.ReadFile(tmpDir + "/static/css/theme-vars.css")
	if err != nil {
		t.Fatalf("Expected theme-vars.css to exist: %v", err)
	}
	if !strings.Contains(string(data), ".custom") {
		t.Error("expected custom CSS in generated file")
	}
	if !strings.Contains(string(data), "--primary: #333333") {
		t.Error("expected primary color CSS variable")
	}
}
