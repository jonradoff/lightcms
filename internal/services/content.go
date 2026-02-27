package services

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ContentService centralizes all content operations with automatic versioning
type ContentService struct {
	db *database.DB
}

// NewContentService creates a new content service
func NewContentService(db *database.DB) *ContentService {
	return &ContentService{db: db}
}

// CreateContent creates new content and saves the initial version
func (s *ContentService) CreateContent(ctx context.Context, content *models.Content, versionComment ...string) error {
	now := time.Now()
	content.CreatedAt = now
	content.UpdatedAt = now

	// Build full path
	if content.FolderPath != "" && content.FolderPath != "/" {
		content.FullPath = content.FolderPath + "/" + content.Slug
	} else if content.Slug != "" {
		content.FullPath = "/" + content.Slug
	} else {
		content.FullPath = "/"
	}

	// Extract internal links
	content.InternalLinks = s.extractInternalLinks(content)

	// Insert content
	id, err := s.db.InsertOne(ctx, "content", content)
	if err != nil {
		return fmt.Errorf("failed to insert content: %w", err)
	}
	content.ID = id

	// Save initial version with optional comment
	comment := ""
	if len(versionComment) > 0 {
		comment = versionComment[0]
	}
	if err := s.saveVersion(ctx, content, nil, comment); err != nil {
		return fmt.Errorf("failed to save initial version: %w", err)
	}

	// Generate static page if published
	if content.Published {
		if err := s.GenerateStaticPage(ctx, content); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to generate static page: %v\n", err)
		}
	}

	return nil
}

// UpdateContent updates content and saves a new version with an optional comment
func (s *ContentService) UpdateContent(ctx context.Context, content *models.Content, versionComment ...string) error {
	// Get the original content for versioning
	var original models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": content.ID}, &original); err != nil {
		return fmt.Errorf("failed to get original content: %w", err)
	}

	content.UpdatedAt = time.Now()

	// Build full path
	if content.FolderPath != "" && content.FolderPath != "/" {
		content.FullPath = content.FolderPath + "/" + content.Slug
	} else if content.Slug != "" {
		content.FullPath = "/" + content.Slug
	} else {
		content.FullPath = "/"
	}

	// Extract internal links
	content.InternalLinks = s.extractInternalLinks(content)

	// Update content
	update := bson.M{
		"$set": bson.M{
			"template_id":      content.TemplateID,
			"template_name":    content.TemplateName,
			"title":            content.Title,
			"slug":             content.Slug,
			"folder_id":        content.FolderID,
			"folder_path":      content.FolderPath,
			"full_path":        content.FullPath,
			"category":         content.Category,
			"meta_description": content.MetaDescription,
			"og_image":         content.OGImage,
			"data":             content.Data,
			"published":        content.Published,
			"published_at":     content.PublishedAt,
			"use_header":       content.UseHeader,
			"use_footer":       content.UseFooter,
			"use_theme":        content.UseTheme,
			"raw_mode":         content.RawMode,
			"internal_links":   content.InternalLinks,
			"updated_at":       content.UpdatedAt,
		},
	}

	if err := s.db.UpdateOne(ctx, "content", bson.M{"_id": content.ID}, update); err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}

	// Save version with original for first-time versioning
	comment := ""
	if len(versionComment) > 0 {
		comment = versionComment[0]
	}
	if err := s.saveVersion(ctx, content, &original, comment); err != nil {
		return fmt.Errorf("failed to save version: %w", err)
	}

	// Generate or remove static page based on publish status
	if content.Published {
		if err := s.GenerateStaticPage(ctx, content); err != nil {
			fmt.Printf("Warning: failed to generate static page: %v\n", err)
		}
	} else {
		// Remove static page if unpublished
		s.removeStaticPage(content.FullPath)
	}

	return nil
}

// PublishContent publishes content and generates static page
func (s *ContentService) PublishContent(ctx context.Context, id primitive.ObjectID) error {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	now := time.Now()
	content.Published = true
	content.PublishedAt = &now

	return s.UpdateContent(ctx, &content)
}

// UnpublishContent unpublishes content and removes static page
func (s *ContentService) UnpublishContent(ctx context.Context, id primitive.ObjectID) error {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	content.Published = false
	content.PublishedAt = nil

	return s.UpdateContent(ctx, &content)
}

// DeleteContent soft-deletes content
func (s *ContentService) DeleteContent(ctx context.Context, id primitive.ObjectID) error {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"deleted_at": now,
			"updated_at": now,
		},
	}

	if err := s.db.UpdateOne(ctx, "content", bson.M{"_id": id}, update); err != nil {
		return fmt.Errorf("failed to delete content: %w", err)
	}

	// Remove static page
	s.removeStaticPage(content.FullPath)

	return nil
}

// RestoreContent restores soft-deleted content
func (s *ContentService) RestoreContent(ctx context.Context, id primitive.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"deleted":    false,
			"updated_at": time.Now(),
		},
		"$unset": bson.M{
			"deleted_at": "",
		},
	}

	if err := s.db.UpdateOne(ctx, "content", bson.M{"_id": id}, update); err != nil {
		return fmt.Errorf("failed to restore content: %w", err)
	}

	// Regenerate static page if published
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		return nil // Content restored, just can't regenerate
	}

	if content.Published {
		s.GenerateStaticPage(ctx, &content)
	}

	return nil
}

// GetContent retrieves content by ID
func (s *ContentService) GetContent(ctx context.Context, id primitive.ObjectID) (*models.Content, error) {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": id}, &content); err != nil {
		return nil, fmt.Errorf("content not found: %w", err)
	}
	return &content, nil
}

// GetContentByPath retrieves content by full path
func (s *ContentService) GetContentByPath(ctx context.Context, path string) (*models.Content, error) {
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"full_path": path, "deleted": bson.M{"$ne": true}}, &content); err != nil {
		return nil, fmt.Errorf("content not found: %w", err)
	}
	return &content, nil
}

// ListContent lists all content with optional filters
func (s *ContentService) ListContent(ctx context.Context, includeDeleted bool, category string, folderID *primitive.ObjectID) ([]models.Content, error) {
	filter := bson.M{}
	if !includeDeleted {
		filter["deleted"] = bson.M{"$ne": true}
	}
	if category != "" {
		filter["category"] = category
	}
	if folderID != nil {
		filter["folder_id"] = folderID
	}

	cursor, err := s.db.FindMany(ctx, "content", filter, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list content: %w", err)
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return nil, fmt.Errorf("failed to decode content: %w", err)
	}

	return contents, nil
}

// GetVersions retrieves all versions of a content item
func (s *ContentService) GetVersions(ctx context.Context, contentID primitive.ObjectID) ([]models.ContentVersion, error) {
	cursor, err := s.db.FindMany(ctx, "content_versions",
		bson.M{"content_id": contentID},
		options.Find().SetSort(bson.D{{Key: "version", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to get versions: %w", err)
	}

	var versions []models.ContentVersion
	if err := cursor.All(ctx, &versions); err != nil {
		return nil, fmt.Errorf("failed to decode versions: %w", err)
	}

	return versions, nil
}

// GetVersion retrieves a specific version
func (s *ContentService) GetVersion(ctx context.Context, contentID primitive.ObjectID, version int) (*models.ContentVersion, error) {
	var v models.ContentVersion
	if err := s.db.FindOne(ctx, "content_versions",
		bson.M{"content_id": contentID, "version": version}, &v); err != nil {
		return nil, fmt.Errorf("version not found: %w", err)
	}
	return &v, nil
}

// RevertToVersion reverts content to a previous version with an optional comment
func (s *ContentService) RevertToVersion(ctx context.Context, contentID primitive.ObjectID, version int, versionComment ...string) error {
	// Get the version to revert to
	v, err := s.GetVersion(ctx, contentID, version)
	if err != nil {
		return err
	}

	// Get current content
	var content models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": contentID}, &content); err != nil {
		return fmt.Errorf("content not found: %w", err)
	}

	// Update content with version data
	content.TemplateID = v.TemplateID
	content.TemplateName = v.TemplateName
	content.Title = v.Title
	content.Slug = v.Slug
	content.FolderID = v.FolderID
	content.FolderPath = v.FolderPath
	content.FullPath = v.FullPath
	content.Category = v.Category
	content.MetaDescription = v.MetaDescription
	content.OGImage = v.OGImage
	content.Data = v.Data
	content.Published = v.Published
	content.PublishedAt = v.PublishedAt
	content.UseHeader = v.UseHeader
	content.UseFooter = v.UseFooter
	content.UseTheme = v.UseTheme
	content.RawMode = v.RawMode

	// Pass through the version comment if provided
	return s.UpdateContent(ctx, &content, versionComment...)
}

// saveVersion saves a new version of the content with an optional comment
func (s *ContentService) saveVersion(ctx context.Context, content *models.Content, original *models.Content, comment string) error {
	// Get the current version count
	count, err := s.db.Count(ctx, "content_versions", bson.M{"content_id": content.ID})
	if err != nil {
		return err
	}

	// If no versions exist and we have the original content, save it as v1 first
	if count == 0 && original != nil {
		v1 := models.ContentVersion{
			ContentID:       original.ID,
			Version:         1,
			TemplateID:      original.TemplateID,
			TemplateName:    original.TemplateName,
			Title:           original.Title,
			Slug:            original.Slug,
			FolderID:        original.FolderID,
			FolderPath:      original.FolderPath,
			FullPath:        original.FullPath,
			Category:        original.Category,
			MetaDescription: original.MetaDescription,
			OGImage:         original.OGImage,
			Data:            original.Data,
			Published:       original.Published,
			PublishedAt:     original.PublishedAt,
			UseHeader:       original.UseHeader,
			UseFooter:       original.UseFooter,
			UseTheme:        original.UseTheme,
			RawMode:         original.RawMode,
			CreatedAt:       original.CreatedAt,
		}
		if _, err := s.db.InsertOne(ctx, "content_versions", v1); err != nil {
			return err
		}
		count = 1
	}

	version := int(count) + 1

	contentVersion := models.ContentVersion{
		ContentID:       content.ID,
		Version:         version,
		Comment:         comment,
		TemplateID:      content.TemplateID,
		TemplateName:    content.TemplateName,
		Title:           content.Title,
		Slug:            content.Slug,
		FolderID:        content.FolderID,
		FolderPath:      content.FolderPath,
		FullPath:        content.FullPath,
		Category:        content.Category,
		MetaDescription: content.MetaDescription,
		OGImage:         content.OGImage,
		Data:            content.Data,
		Published:       content.Published,
		PublishedAt:     content.PublishedAt,
		UseHeader:       content.UseHeader,
		UseFooter:       content.UseFooter,
		UseTheme:        content.UseTheme,
		RawMode:         content.RawMode,
		CreatedAt:       time.Now(),
	}

	_, err = s.db.InsertOne(ctx, "content_versions", contentVersion)
	return err
}

// GenerateStaticPage renders and saves the content as a static HTML file
func (s *ContentService) GenerateStaticPage(ctx context.Context, content *models.Content) error {
	// Get template
	var tmpl models.Template
	if err := s.db.FindOne(ctx, "templates", bson.M{"_id": content.TemplateID}, &tmpl); err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	// Render content
	html, err := s.renderContent(content, &tmpl)
	if err != nil {
		return fmt.Errorf("failed to render content: %w", err)
	}

	// Determine file path
	filePath := content.FullPath
	if filePath == "" || filePath == "/" {
		filePath = "/index"
	}
	filePath = "content/generated" + filePath + ".html"

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// removeStaticPage removes the static HTML file for content
func (s *ContentService) removeStaticPage(fullPath string) {
	if fullPath == "" || fullPath == "/" {
		fullPath = "/index"
	}
	filePath := "content/generated" + fullPath + ".html"
	os.Remove(filePath)
}

// renderContent renders content using its template
func (s *ContentService) renderContent(content *models.Content, tmpl *models.Template) (string, error) {
	// Build data map with template.HTML for string values
	data := make(map[string]interface{})
	for k, v := range content.Data {
		if str, ok := v.(string); ok {
			data[k] = template.HTML(str)
		} else {
			data[k] = v
		}
	}

	// Add title
	data["title"] = content.Title

	// Parse and execute template
	t, err := template.New("content").Parse(tmpl.HTMLLayout)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// extractInternalLinks extracts all internal links from content data
func (s *ContentService) extractInternalLinks(content *models.Content) []string {
	linkSet := make(map[string]bool)
	linkRegex := regexp.MustCompile(`href=["'](/[^"'#?]*)["']`)

	for _, value := range content.Data {
		if strVal, ok := value.(string); ok {
			matches := linkRegex.FindAllStringSubmatch(strVal, -1)
			for _, match := range matches {
				if len(match) > 1 {
					link := strings.TrimSuffix(match[1], "/")
					if link == "" {
						link = "/"
					}
					linkSet[link] = true
				}
			}
		}
	}

	links := make([]string, 0, len(linkSet))
	for link := range linkSet {
		links = append(links, link)
	}
	return links
}

// RegenerateAllContent regenerates all published content
func (s *ContentService) RegenerateAllContent(ctx context.Context) error {
	cursor, err := s.db.FindMany(ctx, "content",
		bson.M{"published": true, "deleted": bson.M{"$ne": true}}, nil)
	if err != nil {
		return fmt.Errorf("failed to list content: %w", err)
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		return fmt.Errorf("failed to decode content: %w", err)
	}

	for _, content := range contents {
		if err := s.GenerateStaticPage(ctx, &content); err != nil {
			fmt.Printf("Warning: failed to regenerate %s: %v\n", content.FullPath, err)
		}
	}

	return nil
}

