package services

import (
	"context"
	"fmt"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/database"
	"github.com/jonradoff/lightcms/v6/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ContentCommentWithContent embeds a comment plus the title/path of the content
// it belongs to — used for dashboard "Recent Comments" display.
type ContentCommentWithContent struct {
	models.ContentComment
	ContentTitle string `json:"content_title"`
	ContentPath  string `json:"content_path"`
}

// CommentService manages content discussion threads.
type CommentService struct {
	db             *database.DB
	webhookService *WebhookService
}

// NewCommentService creates a new CommentService.
func NewCommentService(db *database.DB) *CommentService {
	return &CommentService{db: db}
}

// SetWebhookService wires in the webhook service for event firing.
func (s *CommentService) SetWebhookService(ws *WebhookService) {
	s.webhookService = ws
}

// Create inserts a new comment and fires the comment.created webhook.
func (s *CommentService) Create(ctx context.Context,
	contentID, userID primitive.ObjectID,
	userEmail, displayName, text string,
	mentions []primitive.ObjectID,
) (*models.ContentComment, error) {
	if text == "" {
		return nil, fmt.Errorf("comment text is required")
	}
	c := &models.ContentComment{
		ContentID:       contentID,
		UserID:          userID,
		UserEmail:       userEmail,
		UserDisplayName: displayName,
		Text:            text,
		Mentions:        mentions,
		CreatedAt:       time.Now(),
	}
	id, err := s.db.InsertOne(ctx, "content_comments", c)
	if err != nil {
		return nil, fmt.Errorf("inserting comment: %w", err)
	}
	c.ID = id

	// Fire webhook asynchronously
	if s.webhookService != nil {
		payload := map[string]interface{}{
			"comment_id":        c.ID.Hex(),
			"content_id":        contentID.Hex(),
			"user_email":        userEmail,
			"user_display_name": displayName,
			"text":              text,
		}
		if len(mentions) > 0 {
			ids := make([]string, len(mentions))
			for i, m := range mentions {
				ids[i] = m.Hex()
			}
			payload["mentions"] = ids
		}
		go s.webhookService.FireEvent(context.Background(), "comment.created", payload)
	}

	return c, nil
}

// ListForContent returns all comments for a content item, oldest-first.
func (s *CommentService) ListForContent(ctx context.Context, contentID primitive.ObjectID) ([]models.ContentComment, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := s.db.FindMany(ctx, "content_comments", bson.M{"content_id": contentID}, opts)
	if err != nil {
		return nil, fmt.Errorf("listing comments: %w", err)
	}
	var comments []models.ContentComment
	if err := cursor.All(ctx, &comments); err != nil {
		return nil, fmt.Errorf("decoding comments: %w", err)
	}
	if comments == nil {
		comments = []models.ContentComment{}
	}
	return comments, nil
}

// ListRecent returns the N most recent comments across all content, with content
// title and path joined in for dashboard display.
func (s *CommentService) ListRecent(ctx context.Context, limit int) ([]ContentCommentWithContent, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := s.db.FindMany(ctx, "content_comments", bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("listing recent comments: %w", err)
	}
	var comments []models.ContentComment
	if err := cursor.All(ctx, &comments); err != nil {
		return nil, fmt.Errorf("decoding recent comments: %w", err)
	}

	// Collect unique content IDs for a single batch lookup
	idSet := map[primitive.ObjectID]struct{}{}
	for _, c := range comments {
		idSet[c.ContentID] = struct{}{}
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	// Fetch content titles/paths
	type contentMeta struct {
		ID    primitive.ObjectID `bson:"_id"`
		Title string             `bson:"title"`
		Path  string             `bson:"full_path"`
	}
	var contentItems []contentMeta
	if len(ids) > 0 {
		cur2, err := s.db.FindMany(ctx, "content",
			bson.M{"_id": bson.M{"$in": ids}},
			options.Find().SetProjection(bson.M{"title": 1, "full_path": 1}),
		)
		if err == nil {
			cur2.All(ctx, &contentItems) //nolint:errcheck
		}
	}
	metaMap := map[primitive.ObjectID]contentMeta{}
	for _, c := range contentItems {
		metaMap[c.ID] = c
	}

	result := make([]ContentCommentWithContent, len(comments))
	for i, c := range comments {
		result[i] = ContentCommentWithContent{ContentComment: c}
		if meta, ok := metaMap[c.ContentID]; ok {
			result[i].ContentTitle = meta.Title
			result[i].ContentPath = meta.Path
		}
	}
	return result, nil
}

// Delete removes a comment by ID.
func (s *CommentService) Delete(ctx context.Context, commentID primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "content_comments", bson.M{"_id": commentID})
}

// CountForContent returns the number of comments on a content item.
func (s *CommentService) CountForContent(ctx context.Context, contentID primitive.ObjectID) (int64, error) {
	return s.db.Count(ctx, "content_comments", bson.M{"content_id": contentID})
}
