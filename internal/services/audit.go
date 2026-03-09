package services

import (
	"context"
	"log"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AuditService handles audit log operations
type AuditService struct {
	db *database.DB
}

// NewAuditService creates a new audit service
func NewAuditService(db *database.DB) *AuditService {
	return &AuditService{db: db}
}

// Log synchronously writes an audit log entry
func (s *AuditService) Log(ctx context.Context, entry models.AuditLog) {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if _, err := s.db.InsertOne(ctx, "audit_logs", entry); err != nil {
		log.Printf("[AUDIT] Failed to write audit log: %v (action=%s, user=%s)", err, entry.Action, entry.UserEmail)
	}
}

// LogAsync writes an audit log entry in a fire-and-forget goroutine
func (s *AuditService) LogAsync(entry models.AuditLog) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Log(ctx, entry)
	}()
}

// AuditFilter defines filters for listing audit logs
type AuditFilter struct {
	UserID     *primitive.ObjectID
	Action     string
	Resource   string
	ResourceID string
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// List returns audit log entries matching the filter
func (s *AuditService) List(ctx context.Context, f AuditFilter) ([]models.AuditLog, int64, error) {
	filter := bson.M{}
	if f.UserID != nil {
		filter["user_id"] = *f.UserID
	}
	if f.Action != "" {
		filter["action"] = f.Action
	}
	if f.Resource != "" {
		filter["resource"] = f.Resource
	}
	if f.ResourceID != "" {
		filter["resource_id"] = f.ResourceID
	}
	if f.Since != nil || f.Until != nil {
		dateFilter := bson.M{}
		if f.Since != nil {
			dateFilter["$gte"] = *f.Since
		}
		if f.Until != nil {
			dateFilter["$lte"] = *f.Until
		}
		filter["created_at"] = dateFilter
	}

	total, err := s.db.Count(ctx, "audit_logs", filter)
	if err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(f.Offset))

	cursor, err := s.db.FindMany(ctx, "audit_logs", filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []models.AuditLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
