package services

import (
	"context"
	"fmt"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TemplateService handles template operations
type TemplateService struct {
	db             *database.DB
	contentService *ContentService
	regenQueue     *RegenQueue
}

// NewTemplateService creates a new template service
func NewTemplateService(db *database.DB, contentService *ContentService) *TemplateService {
	return &TemplateService{db: db, contentService: contentService}
}

// SetRegenQueue sets the incremental regeneration queue.
func (s *TemplateService) SetRegenQueue(rq *RegenQueue) {
	s.regenQueue = rq
}

// CreateTemplate creates a new template
func (s *TemplateService) CreateTemplate(ctx context.Context, tmpl *models.Template) error {
	now := time.Now()
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now

	id, err := s.db.InsertOne(ctx, "templates", tmpl)
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}
	tmpl.ID = id

	return nil
}

// UpdateTemplate updates a template and regenerates content if layout changed
func (s *TemplateService) UpdateTemplate(ctx context.Context, tmpl *models.Template) error {
	// Get original to check if layout changed
	var original models.Template
	if err := s.db.FindOne(ctx, "templates", bson.M{"_id": tmpl.ID}, &original); err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	tmpl.UpdatedAt = time.Now()

	update := bson.M{
		"$set": bson.M{
			"name":        tmpl.Name,
			"slug":        tmpl.Slug,
			"description": tmpl.Description,
			"fields":      tmpl.Fields,
			"html_layout": tmpl.HTMLLayout,
			"category":    tmpl.Category,
			"updated_at":  tmpl.UpdatedAt,
		},
	}

	if err := s.db.UpdateOne(ctx, "templates", bson.M{"_id": tmpl.ID}, update); err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}

	// If layout changed, regenerate all content using this template
	if original.HTMLLayout != tmpl.HTMLLayout {
		if s.regenQueue != nil {
			s.regenQueue.Enqueue(context.Background(), tmpl.ID, tmpl.Name)
		} else {
			go s.regenerateContentByTemplate(context.Background(), tmpl.ID)
		}
	}

	return nil
}

// DeleteTemplate deletes a template (if not system template)
func (s *TemplateService) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	var tmpl models.Template
	if err := s.db.FindOne(ctx, "templates", bson.M{"_id": id}, &tmpl); err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	if tmpl.IsSystem {
		return fmt.Errorf("cannot delete system template")
	}

	// Check if any content uses this template
	count, err := s.db.Count(ctx, "content", bson.M{"template_id": id})
	if err != nil {
		return fmt.Errorf("failed to check content: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("cannot delete template: %d content items use this template", count)
	}

	if err := s.db.DeleteOne(ctx, "templates", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	return nil
}

// GetTemplate retrieves a template by ID
func (s *TemplateService) GetTemplate(ctx context.Context, id primitive.ObjectID) (*models.Template, error) {
	var tmpl models.Template
	if err := s.db.FindOne(ctx, "templates", bson.M{"_id": id}, &tmpl); err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return &tmpl, nil
}

// GetTemplateBySlug retrieves a template by slug
func (s *TemplateService) GetTemplateBySlug(ctx context.Context, slug string) (*models.Template, error) {
	var tmpl models.Template
	if err := s.db.FindOne(ctx, "templates", bson.M{"slug": slug}, &tmpl); err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	return &tmpl, nil
}

// ListTemplates lists all templates
func (s *TemplateService) ListTemplates(ctx context.Context) ([]models.Template, error) {
	cursor, err := s.db.FindMany(ctx, "templates", bson.M{},
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	var templates []models.Template
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, fmt.Errorf("failed to decode templates: %w", err)
	}

	return templates, nil
}

// regenerateContentByTemplate regenerates all content using a specific template
func (s *TemplateService) regenerateContentByTemplate(ctx context.Context, templateID primitive.ObjectID) {
	cursor, err := s.db.FindMany(ctx, "content",
		bson.M{"template_id": templateID, "published": true, "deleted": bson.M{"$ne": true}}, nil)
	if err != nil {
		fmt.Printf("Warning: failed to find content for template: %v\n", err)
		return
	}

	var contents []models.Content
	if err := cursor.All(ctx, &contents); err != nil {
		fmt.Printf("Warning: failed to decode content: %v\n", err)
		return
	}

	var purgedPaths []string
	for _, content := range contents {
		if err := s.contentService.GenerateStaticPage(ctx, &content); err != nil {
			fmt.Printf("Warning: failed to regenerate %s: %v\n", content.FullPath, err)
		} else {
			purgedPaths = append(purgedPaths, content.FullPath)
		}
	}
	s.contentService.PurgeCloudflareURLs(purgedPaths)
}
