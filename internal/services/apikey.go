package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"lightcms/internal/database"
	"lightcms/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// APIKeyService handles API key management
type APIKeyService struct {
	db *database.DB
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(db *database.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// CreateAPIKey generates a new API key, stores its hash, and returns the raw key (shown once)
func (s *APIKeyService) CreateAPIKey(ctx context.Context, name, description string) (string, *models.APIKey, error) {
	if name == "" {
		return "", nil, fmt.Errorf("name is required")
	}

	// Generate 32 random bytes → 64 hex chars, but we use 32 hex chars for the key
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	rawKey := "lc_" + hex.EncodeToString(randomBytes)

	// Hash the key with SHA-256
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Store prefix for identification (lc_ + first 8 hex chars)
	prefix := rawKey[:11]

	now := time.Now()
	apiKey := &models.APIKey{
		Name:        name,
		Description: description,
		Prefix:      prefix,
		KeyHash:     keyHash,
		CreatedAt:   now,
	}

	id, err := s.db.InsertOne(ctx, "api_keys", apiKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create API key: %w", err)
	}
	apiKey.ID = id

	return rawKey, apiKey, nil
}

// ListAPIKeys returns all API keys (without raw key values)
func (s *APIKeyService) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	cursor, err := s.db.FindMany(ctx, "api_keys", bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer cursor.Close(ctx)

	var keys []models.APIKey
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, fmt.Errorf("failed to decode API keys: %w", err)
	}
	return keys, nil
}

// DeleteAPIKey deletes an API key by ID
func (s *APIKeyService) DeleteAPIKey(ctx context.Context, id primitive.ObjectID) error {
	return s.db.DeleteOne(ctx, "api_keys", bson.M{"_id": id})
}

// ValidateAPIKey checks a raw API key, returns the key record if valid, and updates last_used_at
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, rawKey string) (*models.APIKey, error) {
	if len(rawKey) < 4 || rawKey[:3] != "lc_" {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Hash the provided key
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Look up by hash
	var apiKey models.APIKey
	err := s.db.FindOne(ctx, "api_keys", bson.M{"key_hash": keyHash}, &apiKey)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last_used_at (fire-and-forget)
	now := time.Now()
	_ = s.db.UpdateOne(ctx, "api_keys", bson.M{"_id": apiKey.ID}, bson.M{"$set": bson.M{"last_used_at": now}})
	apiKey.LastUsedAt = &now

	return &apiKey, nil
}
