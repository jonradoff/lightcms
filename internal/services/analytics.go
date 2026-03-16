package services

import (
	"context"
	"time"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const activityCollection = "user_activity"

// AnalyticsService tracks user activity for DAU/MAU metrics.
type AnalyticsService struct {
	db *database.DB
}

// NewAnalyticsService creates a new AnalyticsService and ensures the required index exists.
func NewAnalyticsService(ctx context.Context, db *database.DB) *AnalyticsService {
	svc := &AnalyticsService{db: db}
	// Unique compound index on (user_id, date) — enforces at-most-one record per user per day.
	col := db.Collection(activityCollection)
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "date", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("user_id_date_unique"),
	})
	// Index for MAU range queries on date.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "date", Value: 1}},
		Options: options.Index().SetName("date_1"),
	})
	return svc
}

// RecordActivity records that userID was active today. Safe to call on every request —
// uses upsert so it is idempotent within a calendar day.
func (s *AnalyticsService) RecordActivity(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	col := s.db.Collection(activityCollection)
	col.UpdateOne(
		ctx,
		bson.M{"user_id": userID, "date": today},
		bson.M{"$set": bson.M{"last_seen": time.Now().UTC()}},
		options.Update().SetUpsert(true),
	)
}

// GetDAU returns the number of distinct users active today (UTC).
func (s *AnalyticsService) GetDAU(ctx context.Context) int64 {
	today := time.Now().UTC().Format("2006-01-02")
	n, _ := s.db.Count(ctx, activityCollection, bson.M{"date": today})
	return n
}

// GetMAU returns the number of distinct users active in the last 30 calendar days.
func (s *AnalyticsService) GetMAU(ctx context.Context) int64 {
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02")
	var result []struct {
		Count int64 `bson:"count"`
	}
	pipeline := bson.A{
		bson.M{"$match": bson.M{"date": bson.M{"$gte": cutoff}}},
		bson.M{"$group": bson.M{"_id": "$user_id"}},
		bson.M{"$count": "count"},
	}
	s.db.Aggregate(ctx, activityCollection, pipeline, &result)
	if len(result) > 0 {
		return result[0].Count
	}
	return 0
}

// GetContentCreatedToday returns the number of content items created since midnight UTC.
func (s *AnalyticsService) GetContentCreatedToday(ctx context.Context) int64 {
	midnight := time.Now().UTC().Truncate(24 * time.Hour)
	n, _ := s.db.Count(ctx, "content", bson.M{
		"created_at": bson.M{"$gte": midnight},
		"deleted":    bson.M{"$ne": true},
	})
	return n
}
