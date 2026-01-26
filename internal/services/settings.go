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

// GetTheme retrieves theme settings
func (s *SettingsService) GetTheme(ctx context.Context) (*database.ThemeSettings, error) {
	return s.db.GetThemeSettings(ctx)
}

// UpdateTheme updates theme settings and regenerates content if header/footer changed
func (s *SettingsService) UpdateTheme(ctx context.Context, theme *database.ThemeSettings) error {
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
