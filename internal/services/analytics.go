package services

import (
	"context"
	"sync"
	"time"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const activityCollection = "user_activity"

// AnalyticsService tracks user activity for DAU/MAU metrics.
//
// Deduplication strategy:
//   - visited is an in-process cache keyed by "userID:date".
//     sync.Map.LoadOrStore is atomic — at most one goroutine per (userID, day)
//     will reach the database write, eliminating redundant upserts.
//   - The unique compound index on (user_id, date) is a safety net for
//     multi-instance deployments or post-restart cold caches.
//   - A background goroutine sweeps the cache at midnight UTC so it never
//     accumulates more than one day's worth of entries.
type AnalyticsService struct {
	db      *database.DB
	visited sync.Map // key: "userID:YYYY-MM-DD" → struct{}{}
	stop    chan struct{}
}

// NewAnalyticsService creates a new AnalyticsService, ensures required indexes
// exist, and starts the midnight cache-cleanup goroutine.
func NewAnalyticsService(ctx context.Context, db *database.DB) *AnalyticsService {
	svc := &AnalyticsService{db: db, stop: make(chan struct{})}

	col := db.Collection(activityCollection)

	// Unique compound index on (user_id, date) — DB-level dedup safety net.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "date", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("user_id_date_unique"),
	})

	// Index for MAU range queries on date.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "date", Value: 1}},
		Options: options.Index().SetName("date_1"),
	})

	// TTL index on created_at — MongoDB auto-deletes documents after 90 days.
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(90 * 24 * 3600).SetName("created_at_ttl_90d"),
	})

	go svc.runMidnightCleanup()
	return svc
}

// Stop shuts down the background cleanup goroutine.
func (s *AnalyticsService) Stop() {
	close(s.stop)
}

// runMidnightCleanup wakes at each UTC midnight and flushes the in-memory
// visited cache so it never holds more than one day's worth of keys.
func (s *AnalyticsService) runMidnightCleanup() {
	for {
		now := time.Now().UTC()
		nextMidnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
		t := time.NewTimer(time.Until(nextMidnight))
		select {
		case <-t.C:
			s.visited.Range(func(k, _ any) bool {
				s.visited.Delete(k)
				return true
			})
		case <-s.stop:
			t.Stop()
			return
		}
	}
}

// RecordActivity records that userID was active today. Safe to call on every
// request — the in-memory cache ensures at most one DB write per (userID, day).
func (s *AnalyticsService) RecordActivity(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	today := time.Now().UTC().Format("2006-01-02")
	cacheKey := userID + ":" + today

	// LoadOrStore is atomic: only the first goroutine for this (userID, day)
	// gets loaded=false and proceeds to the DB write.
	if _, loaded := s.visited.LoadOrStore(cacheKey, struct{}{}); loaded {
		return
	}

	col := s.db.Collection(activityCollection)
	col.UpdateOne(
		ctx,
		bson.M{"user_id": userID, "date": today},
		bson.M{
			"$set":         bson.M{"last_seen": time.Now().UTC()},
			"$setOnInsert": bson.M{"created_at": time.Now().UTC()},
		},
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
