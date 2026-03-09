package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SettingsService handles theme, config, and redirect operations
type SettingsService struct {
	db             *database.DB
	contentService *ContentService
}

// NewSettingsService creates a new settings service
func NewSettingsService(db *database.DB, contentService *ContentService) *SettingsService {
	return &SettingsService{db: db, contentService: contentService}
}

// EnsureThemeCSS regenerates theme-vars.css from the database on startup.
// This ensures the CSS file is present after a fresh container start or deploy.
func (s *SettingsService) EnsureThemeCSS(ctx context.Context) error {
	theme, err := s.db.GetThemeSettings(ctx)
	if err != nil {
		return err
	}
	return s.generateThemeCSS(theme)
}

// GetTheme retrieves theme settings
func (s *SettingsService) GetTheme(ctx context.Context) (*database.ThemeSettings, error) {
	return s.db.GetThemeSettings(ctx)
}

// UpdateTheme updates theme settings and regenerates content if header/footer changed
func (s *SettingsService) UpdateTheme(ctx context.Context, theme *database.ThemeSettings, versionComment ...string) error {
	// Get current theme to check for header/footer changes
	current, err := s.db.GetThemeSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current theme: %w", err)
	}

	headerFooterChanged := current.HeaderHTML != theme.HeaderHTML ||
		current.FooterHTML != theme.FooterHTML ||
		current.HeadHTML != theme.HeadHTML

	theme.UpdatedAt = time.Now()

	if err := s.db.SaveThemeSettings(ctx, theme); err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}

	// Save theme version
	comment := ""
	if len(versionComment) > 0 {
		comment = versionComment[0]
	}
	if err := s.saveThemeVersion(ctx, theme, current, comment); err != nil {
		fmt.Printf("Warning: failed to save theme version: %v\n", err)
	}

	// Generate theme CSS
	if err := s.generateThemeCSS(theme); err != nil {
		fmt.Printf("Warning: failed to generate theme CSS: %v\n", err)
	}

	// Regenerate all content if header/footer changed
	if headerFooterChanged {
		go s.contentService.RegenerateAllContent(context.Background())
	}

	return nil
}

// saveThemeVersion saves a new version of the theme settings
func (s *SettingsService) saveThemeVersion(ctx context.Context, theme *database.ThemeSettings, original *database.ThemeSettings, comment string) error {
	// Get the current version count
	count, err := s.db.GetThemeVersionCount(ctx)
	if err != nil {
		return err
	}

	// If no versions exist and we have original theme, save it as v1 first
	if count == 0 && original != nil {
		v1 := database.ThemeVersion{
			Version:         1,
			PrimaryColor:    original.PrimaryColor,
			SecondaryColor:  original.SecondaryColor,
			AccentColor:     original.AccentColor,
			BackgroundColor: original.BackgroundColor,
			TextColor:       original.TextColor,
			FontFamily:      original.FontFamily,
			HeadingFont:     original.HeadingFont,
			BorderRadius:    original.BorderRadius,
			CustomCSS:       original.CustomCSS,
			SiteName:        original.SiteName,
			SiteTagline:     original.SiteTagline,
			LogoURL:         original.LogoURL,
			HeadHTML:        original.HeadHTML,
			HeaderHTML:      original.HeaderHTML,
			FooterHTML:      original.FooterHTML,
		}
		if err := s.db.SaveThemeVersion(ctx, &v1); err != nil {
			return err
		}
		count = 1
	}

	version := int(count) + 1

	themeVersion := database.ThemeVersion{
		Version:         version,
		Comment:         comment,
		PrimaryColor:    theme.PrimaryColor,
		SecondaryColor:  theme.SecondaryColor,
		AccentColor:     theme.AccentColor,
		BackgroundColor: theme.BackgroundColor,
		TextColor:       theme.TextColor,
		FontFamily:      theme.FontFamily,
		HeadingFont:     theme.HeadingFont,
		BorderRadius:    theme.BorderRadius,
		CustomCSS:       theme.CustomCSS,
		SiteName:        theme.SiteName,
		SiteTagline:     theme.SiteTagline,
		LogoURL:         theme.LogoURL,
		HeadHTML:        theme.HeadHTML,
		HeaderHTML:      theme.HeaderHTML,
		FooterHTML:      theme.FooterHTML,
	}

	return s.db.SaveThemeVersion(ctx, &themeVersion)
}

// GetThemeVersions retrieves all theme versions
func (s *SettingsService) GetThemeVersions(ctx context.Context) ([]database.ThemeVersion, error) {
	return s.db.GetThemeVersions(ctx)
}

// GetThemeVersion retrieves a specific theme version
func (s *SettingsService) GetThemeVersion(ctx context.Context, version int) (*database.ThemeVersion, error) {
	return s.db.GetThemeVersion(ctx, version)
}

// RevertThemeToVersion reverts the theme to a previous version
func (s *SettingsService) RevertThemeToVersion(ctx context.Context, version int, versionComment ...string) error {
	// Get the version to revert to
	v, err := s.GetThemeVersion(ctx, version)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Create theme settings from version
	theme := &database.ThemeSettings{
		PrimaryColor:    v.PrimaryColor,
		SecondaryColor:  v.SecondaryColor,
		AccentColor:     v.AccentColor,
		BackgroundColor: v.BackgroundColor,
		TextColor:       v.TextColor,
		FontFamily:      v.FontFamily,
		HeadingFont:     v.HeadingFont,
		BorderRadius:    v.BorderRadius,
		CustomCSS:       v.CustomCSS,
		SiteName:        v.SiteName,
		SiteTagline:     v.SiteTagline,
		LogoURL:         v.LogoURL,
		HeadHTML:        v.HeadHTML,
		HeaderHTML:      v.HeaderHTML,
		FooterHTML:      v.FooterHTML,
	}

	// Pass through the version comment if provided
	return s.UpdateTheme(ctx, theme, versionComment...)
}

// SetThemeVersionLocked pins or unpins a theme version so it cannot be auto-pruned.
func (s *SettingsService) SetThemeVersionLocked(ctx context.Context, version int, locked bool) error {
	return s.db.SetThemeVersionLocked(ctx, version, locked)
}

// EnsureThemeVersion1 creates version 1 from current theme if no versions exist
func (s *SettingsService) EnsureThemeVersion1(ctx context.Context) error {
	count, err := s.db.GetThemeVersionCount(ctx)
	if err != nil {
		return err
	}

	// Already have versions
	if count > 0 {
		return nil
	}

	// Get current theme
	theme, err := s.db.GetThemeSettings(ctx)
	if err != nil {
		return err
	}

	// Save as version 1
	v1 := database.ThemeVersion{
		Version:         1,
		Comment:         "Initial theme version",
		PrimaryColor:    theme.PrimaryColor,
		SecondaryColor:  theme.SecondaryColor,
		AccentColor:     theme.AccentColor,
		BackgroundColor: theme.BackgroundColor,
		TextColor:       theme.TextColor,
		FontFamily:      theme.FontFamily,
		HeadingFont:     theme.HeadingFont,
		BorderRadius:    theme.BorderRadius,
		CustomCSS:       theme.CustomCSS,
		SiteName:        theme.SiteName,
		SiteTagline:     theme.SiteTagline,
		LogoURL:         theme.LogoURL,
		HeadHTML:        theme.HeadHTML,
		HeaderHTML:      theme.HeaderHTML,
		FooterHTML:      theme.FooterHTML,
	}

	return s.db.SaveThemeVersion(ctx, &v1)
}

// generateThemeCSS generates CSS variables file from theme settings
func (s *SettingsService) generateThemeCSS(theme *database.ThemeSettings) error {
	css := fmt.Sprintf(`:root {
	--primary: %s;
	--secondary: %s;
	--accent: %s;
	--background: %s;
	--text: %s;
	--font-family: %s;
	--heading-font: %s;
	--border-radius: %s;
}
`, theme.PrimaryColor, theme.SecondaryColor, theme.AccentColor,
		theme.BackgroundColor, theme.TextColor, theme.FontFamily,
		theme.HeadingFont, theme.BorderRadius)

	// Add custom CSS if provided
	if theme.CustomCSS != "" {
		css += "\n" + theme.CustomCSS
	}

	// Ensure directory exists
	if err := os.MkdirAll("static/css", 0755); err != nil {
		return err
	}

	return os.WriteFile("static/css/theme-vars.css", []byte(css), 0644)
}

// GetSiteConfig retrieves site configuration
func (s *SettingsService) GetSiteConfig(ctx context.Context) (*database.SiteConfig, error) {
	return s.db.GetSiteConfig(ctx)
}

// UpdateSiteConfig updates site configuration
func (s *SettingsService) UpdateSiteConfig(ctx context.Context, config *database.SiteConfig) error {
	config.UpdatedAt = time.Now()
	return s.db.SaveSiteConfig(ctx, config)
}

// ============================================
// Redirect Operations
// ============================================

// CreateRedirect creates a new redirect
func (s *SettingsService) CreateRedirect(ctx context.Context, redirect *models.Redirect) error {
	now := time.Now()
	redirect.CreatedAt = now
	redirect.UpdatedAt = now

	// Validate status code
	if redirect.StatusCode != 301 && redirect.StatusCode != 302 {
		redirect.StatusCode = 301 // Default to permanent
	}

	id, err := s.db.InsertOne(ctx, "redirects", redirect)
	if err != nil {
		return fmt.Errorf("failed to create redirect: %w", err)
	}
	redirect.ID = id

	return nil
}

// UpdateRedirect updates a redirect
func (s *SettingsService) UpdateRedirect(ctx context.Context, redirect *models.Redirect) error {
	redirect.UpdatedAt = time.Now()

	// Validate status code
	if redirect.StatusCode != 301 && redirect.StatusCode != 302 {
		redirect.StatusCode = 301
	}

	update := bson.M{
		"$set": bson.M{
			"from_path":   redirect.FromPath,
			"to_path":     redirect.ToPath,
			"status_code": redirect.StatusCode,
			"description": redirect.Description,
			"updated_at":  redirect.UpdatedAt,
		},
	}

	if err := s.db.UpdateOne(ctx, "redirects", bson.M{"_id": redirect.ID}, update); err != nil {
		return fmt.Errorf("failed to update redirect: %w", err)
	}

	return nil
}

// DeleteRedirect deletes a redirect
func (s *SettingsService) DeleteRedirect(ctx context.Context, id primitive.ObjectID) error {
	if err := s.db.DeleteOne(ctx, "redirects", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("failed to delete redirect: %w", err)
	}
	return nil
}

// GetRedirect retrieves a redirect by ID
func (s *SettingsService) GetRedirect(ctx context.Context, id primitive.ObjectID) (*models.Redirect, error) {
	var redirect models.Redirect
	if err := s.db.FindOne(ctx, "redirects", bson.M{"_id": id}, &redirect); err != nil {
		return nil, fmt.Errorf("redirect not found: %w", err)
	}
	return &redirect, nil
}

// ListRedirects lists all redirects
func (s *SettingsService) ListRedirects(ctx context.Context) ([]models.Redirect, error) {
	cursor, err := s.db.FindMany(ctx, "redirects", bson.M{},
		options.Find().SetSort(bson.D{{Key: "from_path", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list redirects: %w", err)
	}

	var redirects []models.Redirect
	if err := cursor.All(ctx, &redirects); err != nil {
		return nil, fmt.Errorf("failed to decode redirects: %w", err)
	}

	return redirects, nil
}

// ============================================
// Folder Operations
// ============================================

// CreateFolder creates a new folder
func (s *SettingsService) CreateFolder(ctx context.Context, folder *models.Folder) error {
	now := time.Now()
	folder.CreatedAt = now
	folder.UpdatedAt = now

	// Build path
	if folder.ParentID != nil {
		var parent models.Folder
		if err := s.db.FindOne(ctx, "folders", bson.M{"_id": *folder.ParentID}, &parent); err != nil {
			return fmt.Errorf("parent folder not found: %w", err)
		}
		folder.Path = parent.Path + "/" + folder.Slug
	} else {
		folder.Path = "/" + folder.Slug
	}

	id, err := s.db.InsertOne(ctx, "folders", folder)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}
	folder.ID = id

	return nil
}

// GetFolder retrieves a folder by ID
func (s *SettingsService) GetFolder(ctx context.Context, id primitive.ObjectID) (*models.Folder, error) {
	var folder models.Folder
	if err := s.db.FindOne(ctx, "folders", bson.M{"_id": id}, &folder); err != nil {
		return nil, fmt.Errorf("folder not found: %w", err)
	}
	return &folder, nil
}

// ListFolders lists all folders
func (s *SettingsService) ListFolders(ctx context.Context) ([]models.Folder, error) {
	cursor, err := s.db.FindMany(ctx, "folders", bson.M{},
		options.Find().SetSort(bson.D{{Key: "path", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	var folders []models.Folder
	if err := cursor.All(ctx, &folders); err != nil {
		return nil, fmt.Errorf("failed to decode folders: %w", err)
	}

	return folders, nil
}

// DeleteFolder deletes a folder (if empty)
func (s *SettingsService) DeleteFolder(ctx context.Context, id primitive.ObjectID) error {
	// Check for content in folder
	count, err := s.db.Count(ctx, "content", bson.M{"folder_id": id, "deleted": bson.M{"$ne": true}})
	if err != nil {
		return fmt.Errorf("failed to check folder content: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete folder: contains %d content items", count)
	}

	// Check for child folders
	count, err = s.db.Count(ctx, "folders", bson.M{"parent_id": id})
	if err != nil {
		return fmt.Errorf("failed to check child folders: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete folder: contains %d subfolders", count)
	}

	if err := s.db.DeleteOne(ctx, "folders", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	return nil
}

// ============================================
// Collection Operations
// ============================================

// CreateCollection creates a new collection
func (s *SettingsService) CreateCollection(ctx context.Context, collection *models.Collection) error {
	now := time.Now()
	collection.CreatedAt = now
	collection.UpdatedAt = now

	id, err := s.db.InsertOne(ctx, "collections", collection)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	collection.ID = id

	return nil
}

// UpdateCollection updates a collection
func (s *SettingsService) UpdateCollection(ctx context.Context, collection *models.Collection) error {
	collection.UpdatedAt = time.Now()

	update := bson.M{
		"$set": bson.M{
			"name":           collection.Name,
			"slug":           collection.Slug,
			"description":    collection.Description,
			"category":       collection.Category,
			"sort_field":     collection.SortField,
			"sort_order":     collection.SortOrder,
			"item_template":  collection.ItemTemplate,
			"page_template":  collection.PageTemplate,
			"items_per_page": collection.ItemsPerPage,
			"updated_at":     collection.UpdatedAt,
		},
	}

	if err := s.db.UpdateOne(ctx, "collections", bson.M{"_id": collection.ID}, update); err != nil {
		return fmt.Errorf("failed to update collection: %w", err)
	}

	return nil
}

// GetCollection retrieves a collection by ID
func (s *SettingsService) GetCollection(ctx context.Context, id primitive.ObjectID) (*models.Collection, error) {
	var collection models.Collection
	if err := s.db.FindOne(ctx, "collections", bson.M{"_id": id}, &collection); err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}
	return &collection, nil
}

// ListCollections lists all collections
func (s *SettingsService) ListCollections(ctx context.Context) ([]models.Collection, error) {
	cursor, err := s.db.FindMany(ctx, "collections", bson.M{},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var collections []models.Collection
	if err := cursor.All(ctx, &collections); err != nil {
		return nil, fmt.Errorf("failed to decode collections: %w", err)
	}

	return collections, nil
}

// DeleteCollection deletes a collection
func (s *SettingsService) DeleteCollection(ctx context.Context, id primitive.ObjectID) error {
	if err := s.db.DeleteOne(ctx, "collections", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	return nil
}
