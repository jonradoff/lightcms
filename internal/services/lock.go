package services

import (
	"context"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// LockDuration is how long a content lock remains valid before it expires.
const LockDuration = 30 * time.Minute

// ContentLockDoc represents an advisory lock on a content item.
type ContentLockDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ContentID  primitive.ObjectID `bson:"content_id" json:"content_id"`
	UserID     primitive.ObjectID `bson:"user_id" json:"user_id"`
	UserEmail  string             `bson:"user_email" json:"user_email"`
	AcquiredAt time.Time          `bson:"acquired_at" json:"acquired_at"`
	ExpiresAt  time.Time          `bson:"expires_at" json:"expires_at"`
}

// LockService manages advisory content locks.
type LockService struct {
	db *database.DB
}

// NewLockService creates a new LockService.
func NewLockService(db *database.DB) *LockService {
	return &LockService{db: db}
}

// AcquireLock attempts to acquire a lock on contentID for the given user.
//
// Returns (nil, nil) if the lock was successfully acquired.
// Returns (existingLock, nil) if another user holds a non-expired lock.
// Returns (nil, err) on database errors.
func (s *LockService) AcquireLock(ctx context.Context, contentID, userID primitive.ObjectID, email string) (*ContentLockDoc, error) {
	now := time.Now()

	// Check for an existing non-expired lock held by a different user.
	var existing ContentLockDoc
	err := s.db.FindOne(ctx, "content_locks", bson.M{
		"content_id": contentID,
		"expires_at": bson.M{"$gt": now},
		"user_id":    bson.M{"$ne": userID},
	}, &existing)
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}
	if err == nil {
		// Lock is held by someone else.
		return &existing, nil
	}

	// No blocking lock found — delete any stale/expired lock for this content first.
	_ = s.db.DeleteOne(ctx, "content_locks", bson.M{"content_id": contentID})

	// Insert a fresh lock.
	doc := ContentLockDoc{
		ContentID:  contentID,
		UserID:     userID,
		UserEmail:  email,
		AcquiredAt: now,
		ExpiresAt:  now.Add(LockDuration),
	}
	if _, err := s.db.InsertOne(ctx, "content_locks", doc); err != nil {
		return nil, err
	}
	return nil, nil
}

// ReleaseLock removes the lock for contentID held by userID.
func (s *LockService) ReleaseLock(ctx context.Context, contentID, userID primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "content_locks", bson.M{
		"content_id": contentID,
		"user_id":    userID,
	})
}

// GetLock returns the active (non-expired) lock for contentID, or nil if none.
func (s *LockService) GetLock(ctx context.Context, contentID primitive.ObjectID) (*ContentLockDoc, error) {
	var doc ContentLockDoc
	err := s.db.FindOne(ctx, "content_locks", bson.M{
		"content_id": contentID,
		"expires_at": bson.M{"$gt": time.Now()},
	}, &doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// RefreshLock extends the expiry of an existing lock by LockDuration for the same user.
func (s *LockService) RefreshLock(ctx context.Context, contentID, userID primitive.ObjectID) error {
	return s.db.UpdateOne(ctx, "content_locks",
		bson.M{"content_id": contentID, "user_id": userID},
		bson.M{"$set": bson.M{"expires_at": time.Now().Add(LockDuration)}},
	)
}

// ForceUnlock removes any lock for contentID regardless of owner.
// The caller is responsible for verifying admin permissions.
func (s *LockService) ForceUnlock(ctx context.Context, contentID primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "content_locks", bson.M{"content_id": contentID})
}
