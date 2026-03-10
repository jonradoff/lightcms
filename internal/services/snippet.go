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

// SnippetService manages reusable HTML snippet templates used in lc:query directives.
type SnippetService struct {
	db *database.DB
}

func NewSnippetService(db *database.DB) *SnippetService {
	return &SnippetService{db: db}
}

func (s *SnippetService) ListSnippets(ctx context.Context) ([]models.Snippet, error) {
	var snippets []models.Snippet
	cursor, err := s.db.FindMany(ctx, "snippets", bson.M{}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list snippets: %w", err)
	}
	if err := cursor.All(ctx, &snippets); err != nil {
		return nil, fmt.Errorf("failed to decode snippets: %w", err)
	}
	return snippets, nil
}

func (s *SnippetService) GetSnippet(ctx context.Context, id primitive.ObjectID) (*models.Snippet, error) {
	var snip models.Snippet
	if err := s.db.FindOne(ctx, "snippets", bson.M{"_id": id}, &snip); err != nil {
		return nil, fmt.Errorf("snippet not found: %w", err)
	}
	return &snip, nil
}

func (s *SnippetService) GetSnippetByName(ctx context.Context, name string) (*models.Snippet, error) {
	var snip models.Snippet
	if err := s.db.FindOne(ctx, "snippets", bson.M{"name": name}, &snip); err != nil {
		return nil, fmt.Errorf("snippet not found: %w", err)
	}
	return &snip, nil
}

func (s *SnippetService) CreateSnippet(ctx context.Context, name, html string) (*models.Snippet, error) {
	now := time.Now()
	snip := models.Snippet{
		Name:      name,
		HTML:      html,
		CreatedAt: now,
		UpdatedAt: now,
	}
	id, err := s.db.InsertOne(ctx, "snippets", snip)
	if err != nil {
		return nil, fmt.Errorf("failed to create snippet: %w", err)
	}
	snip.ID = id
	return &snip, nil
}

func (s *SnippetService) UpdateSnippet(ctx context.Context, id primitive.ObjectID, name, html string) (*models.Snippet, error) {
	now := time.Now()
	update := bson.M{"$set": bson.M{
		"name":       name,
		"html":       html,
		"updated_at": now,
	}}
	if err := s.db.UpdateOne(ctx, "snippets", bson.M{"_id": id}, update); err != nil {
		return nil, fmt.Errorf("failed to update snippet: %w", err)
	}
	return s.GetSnippet(ctx, id)
}

func (s *SnippetService) DeleteSnippet(ctx context.Context, id primitive.ObjectID) error {
	if err := s.db.DeleteOne(ctx, "snippets", bson.M{"_id": id}); err != nil {
		return fmt.Errorf("failed to delete snippet: %w", err)
	}
	return nil
}
