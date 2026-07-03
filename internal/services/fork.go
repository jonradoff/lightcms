package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ForkService manages content forks — isolated workspaces for staging site changes.
type ForkService struct {
	db             *database.DB
	contentService *ContentService
}

// NewForkService creates a ForkService.
func NewForkService(db *database.DB, cs *ContentService) *ForkService {
	return &ForkService{db: db, contentService: cs}
}

// MergeResult summarises what happened when a fork was merged into live.
type MergeResult struct {
	Updated   int
	Created   int
	Conflicts []ForkConflict
}

// ForkConflict records a page that was modified on the live site after the fork was made.
type ForkConflict struct {
	ForkItem  models.Content
	LiveTitle string
	LivePath  string
}

// generatePreviewToken returns a random 32-byte hex string.
func generatePreviewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create creates a new fork workspace.
func (s *ForkService) Create(ctx context.Context, name, description string, userID primitive.ObjectID, userEmail string) (*models.ContentFork, error) {
	token, err := generatePreviewToken()
	if err != nil {
		return nil, fmt.Errorf("generate preview token: %w", err)
	}
	now := time.Now()
	fork := &models.ContentFork{
		ID:             primitive.NewObjectID(),
		Name:           name,
		Description:    description,
		Status:         "active",
		PreviewToken:   token,
		CreatedBy:      userID,
		CreatedByEmail: userEmail,
		CreatedAt:      now,
	}
	if _, err := s.db.InsertOne(ctx, "content_forks", fork); err != nil {
		return nil, fmt.Errorf("insert fork: %w", err)
	}
	return fork, nil
}

// GetByID returns a fork by its ObjectID.
func (s *ForkService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.ContentFork, error) {
	var fork models.ContentFork
	if err := s.db.FindOne(ctx, "content_forks", bson.M{"_id": id}, &fork); err != nil {
		return nil, err
	}
	return &fork, nil
}

// GetByPreviewToken returns an active fork by its preview token.
func (s *ForkService) GetByPreviewToken(ctx context.Context, token string) (*models.ContentFork, error) {
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}
	var fork models.ContentFork
	if err := s.db.FindOne(ctx, "content_forks", bson.M{"preview_token": token, "status": "active"}, &fork); err != nil {
		return nil, err
	}
	return &fork, nil
}

// List returns all forks, newest first.
func (s *ForkService) List(ctx context.Context) ([]models.ContentFork, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := s.db.Collection("content_forks").Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var forks []models.ContentFork
	if err := cursor.All(ctx, &forks); err != nil {
		return nil, err
	}
	return forks, nil
}

// ForkPage copies an existing live content item into a fork workspace.
// If the page is already in this fork it returns the existing copy.
func (s *ForkService) ForkPage(ctx context.Context, forkID primitive.ObjectID, contentID primitive.ObjectID) (*models.Content, error) {
	// Check not already forked
	existing, err := s.GetForkPageByContentID(ctx, forkID, contentID)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Load live content
	var live models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": contentID, "deleted": bson.M{"$ne": true}, "fork_id": bson.M{"$exists": false}}, &live); err != nil {
		return nil, fmt.Errorf("content not found: %w", err)
	}

	// Create fork copy
	now := time.Now()
	copy := live
	copy.ID = primitive.NewObjectID()
	copy.ForkID = &forkID
	copy.BaseUpdatedAt = &live.UpdatedAt
	copy.Published = false // fork pages are never live-published
	copy.CreatedAt = now
	copy.UpdatedAt = now

	if _, err := s.db.InsertOne(ctx, "content", &copy); err != nil {
		return nil, fmt.Errorf("insert fork page: %w", err)
	}
	return &copy, nil
}

// GetForkPageByContentID finds the fork copy of a specific live content item.
func (s *ForkService) GetForkPageByContentID(ctx context.Context, forkID primitive.ObjectID, contentID primitive.ObjectID) (*models.Content, error) {
	// The fork copy was made from the live page — we match by full_path within the fork since we don't store the source ID.
	// Instead, look by _id which was freshly generated. Use full_path + fork_id.
	// Actually we need a reliable way — store the source content ID. Let's query by fork_id + original content_id via full_path.
	// Simpler: just check fork_id + matching original content (_id was re-generated, but full_path is the same).
	// We'll use fork_id + full_path by first fetching the live item's full_path.
	var live models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"_id": contentID}, &live); err != nil {
		return nil, err
	}
	return s.GetForkPageByPath(ctx, forkID, live.FullPath)
}

// GetForkPageByPath finds the fork version of a page by path.
func (s *ForkService) GetForkPageByPath(ctx context.Context, forkID primitive.ObjectID, fullPath string) (*models.Content, error) {
	var item models.Content
	if err := s.db.FindOne(ctx, "content", bson.M{"fork_id": forkID, "full_path": fullPath, "deleted": bson.M{"$ne": true}}, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// ListPages returns all content items belonging to a fork.
func (s *ForkService) ListPages(ctx context.Context, forkID primitive.ObjectID) ([]models.Content, error) {
	opts := options.Find().SetSort(bson.D{{Key: "full_path", Value: 1}})
	cursor, err := s.db.Collection("content").Find(ctx, bson.M{"fork_id": forkID, "deleted": bson.M{"$ne": true}}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var pages []models.Content
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

// RemovePage removes a page from a fork (deletes the fork copy).
func (s *ForkService) RemovePage(ctx context.Context, forkID primitive.ObjectID, pageID primitive.ObjectID) error {
	filter := bson.M{"_id": pageID, "fork_id": forkID}
	result, err := s.db.Collection("content").DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete fork page: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("page not found in fork")
	}
	return nil
}

// Merge merges all fork pages into live content and marks the fork as merged.
// Returns a MergeResult summarising what was created, updated, and any conflicts.
func (s *ForkService) Merge(ctx context.Context, forkID primitive.ObjectID, mergedByID primitive.ObjectID, mergedByEmail string) (*MergeResult, error) {
	fork, err := s.GetByID(ctx, forkID)
	if err != nil {
		return nil, fmt.Errorf("fork not found: %w", err)
	}
	if fork.Status != "active" {
		return nil, fmt.Errorf("fork is %s, cannot merge", fork.Status)
	}

	pages, err := s.ListPages(ctx, forkID)
	if err != nil {
		return nil, fmt.Errorf("list fork pages: %w", err)
	}

	result := &MergeResult{}

	for _, forkPage := range pages {
		// Look for existing live page with the same full_path
		var livePage models.Content
		liveErr := s.db.FindOne(ctx, "content", bson.M{
			"full_path": forkPage.FullPath,
			"fork_id":   bson.M{"$exists": false},
			"deleted":   bson.M{"$ne": true},
		}, &livePage)

		now := time.Now()

		if liveErr != nil {
			// No live page — create new
			newPage := forkPage
			newPage.ID = primitive.NewObjectID()
			newPage.ForkID = nil
			newPage.BaseUpdatedAt = nil
			newPage.CreatedAt = now
			newPage.UpdatedAt = now
			if _, err := s.db.InsertOne(ctx, "content", &newPage); err != nil {
				return nil, fmt.Errorf("create live page %s: %w", forkPage.FullPath, err)
			}
			// Regenerate static file if published
			if newPage.Published && s.contentService != nil {
				_ = s.contentService.GenerateStaticPage(ctx, &newPage)
			}
			result.Created++
		} else {
			// Live page exists — check for conflict
			if forkPage.BaseUpdatedAt != nil && livePage.UpdatedAt.After(*forkPage.BaseUpdatedAt) {
				result.Conflicts = append(result.Conflicts, ForkConflict{
					ForkItem:  forkPage,
					LiveTitle: livePage.Title,
					LivePath:  livePage.FullPath,
				})
				// Still merge (fork wins), just record the conflict
			}

			// Update live page fields from fork
			update := bson.M{"$set": bson.M{
				"title":            forkPage.Title,
				"slug":             forkPage.Slug,
				"folder_id":        forkPage.FolderID,
				"folder_path":      forkPage.FolderPath,
				"full_path":        forkPage.FullPath,
				"category":         forkPage.Category,
				"tags":             forkPage.Tags,
				"meta_description": forkPage.MetaDescription,
				"og_image":         forkPage.OGImage,
				"data":             forkPage.Data,
				"use_header":       forkPage.UseHeader,
				"use_footer":       forkPage.UseFooter,
				"use_theme":        forkPage.UseTheme,
				"raw_mode":         forkPage.RawMode,
				"template_id":      forkPage.TemplateID,
				"template_name":    forkPage.TemplateName,
				"updated_at":       now,
			}}
			if err := s.db.UpdateOne(ctx, "content", bson.M{"_id": livePage.ID}, update); err != nil {
				return nil, fmt.Errorf("update live page %s: %w", forkPage.FullPath, err)
			}
			// Regenerate static file if published
			if livePage.Published && s.contentService != nil {
				livePage.Title = forkPage.Title
				livePage.Data = forkPage.Data
				livePage.UseHeader = forkPage.UseHeader
				livePage.UseFooter = forkPage.UseFooter
				livePage.UseTheme = forkPage.UseTheme
				livePage.TemplateID = forkPage.TemplateID
				livePage.TemplateName = forkPage.TemplateName
				_ = s.contentService.GenerateStaticPage(ctx, &livePage)
			}
			result.Updated++
		}
	}

	// Mark fork as merged
	now := time.Now()
	_ = s.db.UpdateOne(ctx, "content_forks", bson.M{"_id": forkID}, bson.M{"$set": bson.M{
		"status":          "merged",
		"merged_at":       now,
		"merged_by":       mergedByID,
		"merged_by_email": mergedByEmail,
	}})

	return result, nil
}

// Archive marks a fork as archived without merging.
func (s *ForkService) Archive(ctx context.Context, forkID primitive.ObjectID) error {
	now := time.Now()
	return s.db.UpdateOne(ctx, "content_forks", bson.M{"_id": forkID, "status": "active"}, bson.M{"$set": bson.M{
		"status":      "archived",
		"archived_at": now,
	}})
}

// Delete permanently deletes a fork and all its pages (only for non-merged forks).
func (s *ForkService) Delete(ctx context.Context, forkID primitive.ObjectID) error {
	fork, err := s.GetByID(ctx, forkID)
	if err != nil {
		return err
	}
	if fork.Status == "merged" {
		return fmt.Errorf("cannot delete a merged fork")
	}
	// Delete all fork pages
	if _, err := s.db.Collection("content").DeleteMany(ctx, bson.M{"fork_id": forkID}); err != nil {
		return fmt.Errorf("delete fork pages: %w", err)
	}
	// Delete fork record
	if _, err := s.db.Collection("content_forks").DeleteOne(ctx, bson.M{"_id": forkID}); err != nil {
		return fmt.Errorf("delete fork: %w", err)
	}
	return nil
}

// GetPageCount returns the number of pages in a fork.
func (s *ForkService) GetPageCount(ctx context.Context, forkID primitive.ObjectID) (int64, error) {
	return s.db.Count(ctx, "content", bson.M{"fork_id": forkID, "deleted": bson.M{"$ne": true}})
}
