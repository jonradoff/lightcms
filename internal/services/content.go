package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	db            *database.DB
	searchService *SearchService
}

// NewContentService creates a new content service
func NewContentService(db *database.DB) *ContentService {
	return &ContentService{db: db}
}

// SetSearchService sets the search service for automatic embedding generation
func (s *ContentService) SetSearchService(ss *SearchService) {
	s.searchService = ss
}

// triggerEmbedding asynchronously generates an embedding for the given content
func (s *ContentService) triggerEmbedding(contentID primitive.ObjectID) {
	if s.searchService == nil || !s.searchService.HasVoyageKey() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.searchService.UpdateContentEmbedding(ctx, contentID); err != nil {
			fmt.Printf("Warning: failed to update embedding for %s: %v\n", contentID.Hex(), err)
		}
	}()
}

// triggerKeywordRebuild asynchronously rebuilds the search keyword cache
func (s *ContentService) triggerKeywordRebuild() {
	if s.searchService == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.searchService.RebuildKeywords(ctx); err != nil {
			fmt.Printf("Warning: failed to rebuild search keywords: %v\n", err)
		}
	}()
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
		s.triggerEmbedding(content.ID)
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
			"tags":             content.Tags,
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
		s.triggerEmbedding(content.ID)
	} else {
		// Remove static page if unpublished
		s.removeStaticPage(content.FullPath)
	}

	// Rebuild search keyword cache when content changes
	s.triggerKeywordRebuild()

	// Regenerate index pages that may reference this content via lc:query
	go func() {
		rCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		s.RegenerateIndexPages(rCtx)
	}()

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

	// Rebuild search keyword cache
	s.triggerKeywordRebuild()

	// Regenerate index pages that referenced this content
	go func() {
		rCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		s.RegenerateIndexPages(rCtx)
	}()

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

	// Rebuild search keyword cache
	s.triggerKeywordRebuild()

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
	content.Tags = v.Tags
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
			Tags:            original.Tags,
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
		Tags:            content.Tags,
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

	// Render content (template fields interpolated)
	html, err := s.renderContent(content, &tmpl)
	if err != nil {
		return fmt.Errorf("failed to render content: %w", err)
	}

	// Process any lc:query directives in the rendered HTML
	if strings.Contains(html, "lc:query") {
		html, err = s.processQueryDirectives(ctx, html)
		if err != nil {
			fmt.Printf("Warning: lc:query processing error for %s: %v\n", content.FullPath, err)
		}
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

// QueryContentForDirective fetches published content matching the given filter criteria, sorted as specified.
// filter: map of field->value (supported keys: tag, category, template, folder)
// sortField: "title", "created_at", "published_at" — sortDir: "asc" or "desc"
func (s *ContentService) QueryContentForDirective(ctx context.Context, filter map[string]string, sortField, sortDir string) ([]models.Content, error) {
	q := bson.M{"published": true, "deleted": bson.M{"$ne": true}}

	if v, ok := filter["tag"]; ok && v != "" {
		q["tags"] = v
	}
	if v, ok := filter["category"]; ok && v != "" {
		q["category"] = v
	}
	if v, ok := filter["template"]; ok && v != "" {
		q["template_name"] = v
	}
	if v, ok := filter["folder"]; ok && v != "" {
		q["folder_path"] = bson.M{"$regex": "^" + regexp.QuoteMeta(v)}
	}

	var contents []models.Content
	if err := s.db.FindAll(ctx, "content", q, &contents); err != nil {
		return nil, fmt.Errorf("lc:query failed: %w", err)
	}

	// Sort in Go (avoids mongo index requirements for arbitrary fields)
	dir := 1
	if sortDir == "desc" {
		dir = -1
	}
	switch sortField {
	case "title", "":
		sort.Slice(contents, func(i, j int) bool {
			if dir == 1 {
				return strings.ToLower(contents[i].Title) < strings.ToLower(contents[j].Title)
			}
			return strings.ToLower(contents[i].Title) > strings.ToLower(contents[j].Title)
		})
	case "created_at":
		sort.Slice(contents, func(i, j int) bool {
			if dir == 1 {
				return contents[i].CreatedAt.Before(contents[j].CreatedAt)
			}
			return contents[i].CreatedAt.After(contents[j].CreatedAt)
		})
	case "published_at":
		sort.Slice(contents, func(i, j int) bool {
			ai := contents[i].PublishedAt
			aj := contents[j].PublishedAt
			if ai == nil {
				return dir == -1
			}
			if aj == nil {
				return dir == 1
			}
			if dir == 1 {
				return ai.Before(*aj)
			}
			return ai.After(*aj)
		})
	}
	return contents, nil
}

// lcQueryDirectiveRE matches <!-- lc:query ... --> comment blocks (single-line or across multiple lines).
var lcQueryDirectiveRE = regexp.MustCompile(`(?s)<!--\s*lc:query\s+(.*?)-->`)

// parseDirectiveAttrs parses key="value" pairs from a directive attribute string.
func parseDirectiveAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		attrs[m[1]] = m[2]
	}
	return attrs
}

// processQueryDirectives scans html for <!-- lc:query ... --> directives, evaluates each
// against the database, renders each result through the named snippet, and returns the
// final HTML with directives replaced.
func (s *ContentService) processQueryDirectives(ctx context.Context, html string) (string, error) {
	var firstErr error
	result := lcQueryDirectiveRE.ReplaceAllStringFunc(html, func(match string) string {
		sub := lcQueryDirectiveRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		attrs := parseDirectiveAttrs(sub[1])

		// Resolve filter — supports both direct keys (tag="X") and compound filter="key:value"
		filterMap := make(map[string]string)
		for _, key := range []string{"tag", "category", "template", "folder"} {
			if v, ok := attrs[key]; ok {
				filterMap[key] = v
			}
		}
		if fv, ok := attrs["filter"]; ok {
			parts := strings.SplitN(fv, ":", 2)
			if len(parts) == 2 {
				filterMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Resolve sort
		sortField, sortDir := "title", "asc"
		if sv, ok := attrs["sort"]; ok {
			parts := strings.SplitN(sv, ":", 2)
			sortField = parts[0]
			if len(parts) == 2 {
				sortDir = parts[1]
			}
		}

		// Query
		items, err := s.QueryContentForDirective(ctx, filterMap, sortField, sortDir)
		if err != nil {
			firstErr = err
			return "<!-- lc:query error: " + err.Error() + " -->"
		}

		// Get snippet HTML
		snippetName := attrs["snippet"]
		snippetHTML := ""
		if snippetName != "" {
			var snip models.Snippet
			if err := s.db.FindOne(ctx, "snippets", bson.M{"name": snippetName}, &snip); err == nil {
				snippetHTML = snip.HTML
			}
		}

		// Render items
		var rendered strings.Builder
		for _, item := range items {
			if snippetHTML != "" {
				itemHTML, err := renderSnippet(snippetHTML, item)
				if err != nil {
					rendered.WriteString("<!-- snippet render error: " + err.Error() + " -->")
				} else {
					rendered.WriteString(itemHTML)
				}
			} else {
				// Default: simple link
				rendered.WriteString(`<a href="` + item.FullPath + `">` + item.Title + `</a>`)
			}
		}
		return rendered.String()
	})
	return result, firstErr
}

// renderSnippet renders a snippet HTML template with the given content item.
func renderSnippet(snippetHTML string, item models.Content) (string, error) {
	t, err := template.New("snippet").Parse(snippetHTML)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, item); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RegenerateIndexPages regenerates all published pages whose templates contain lc:query directives.
// Called after any content mutation so index pages stay in sync.
func (s *ContentService) RegenerateIndexPages(ctx context.Context) {
	// Find all templates with lc:query directives
	var templates []models.Template
	if err := s.db.FindAll(ctx, "templates", bson.M{}, &templates); err != nil {
		return
	}

	var indexTemplateIDs []primitive.ObjectID
	for _, tmpl := range templates {
		if strings.Contains(tmpl.HTMLLayout, "lc:query") {
			indexTemplateIDs = append(indexTemplateIDs, tmpl.ID)
		}
	}
	if len(indexTemplateIDs) == 0 {
		return
	}

	// Find all published pages using index templates
	var pages []models.Content
	if err := s.db.FindAll(ctx, "content", bson.M{
		"template_id": bson.M{"$in": indexTemplateIDs},
		"published":   true,
		"deleted":     bson.M{"$ne": true},
	}, &pages); err != nil {
		return
	}

	for i := range pages {
		if err := s.GenerateStaticPage(ctx, &pages[i]); err != nil {
			fmt.Printf("Warning: failed to regenerate index page %s: %v\n", pages[i].FullPath, err)
		}
	}
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

