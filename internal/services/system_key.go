package services

import (
	"context"
	"fmt"
	"log"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureSystemAPIKey creates or retrieves the system-internal API key used for
// OAuth-authenticated MCP sessions. The key is associated with the given admin
// user so that OAuth-authenticated requests are authorized with admin-level
// permissions (rather than bypassing permission checks via a nil user).
// The raw key is stored in the settings collection so it persists across restarts.
func EnsureSystemAPIKey(ctx context.Context, db *database.DB, apiKeyService *APIKeyService, adminUserID *primitive.ObjectID) (string, error) {
	// Check if system key already exists
	var setting struct {
		RawKey string `bson:"raw_key"`
	}
	err := db.FindOne(ctx, "settings", bson.M{"type": "system_api_key"}, &setting)
	if err == nil && setting.RawKey != "" {
		// Verify the key is still valid in the api_keys collection
		if existingKey, err := apiKeyService.ValidateAPIKey(ctx, setting.RawKey); err == nil {
			// If the key exists but has no user, update it to use the admin user.
			if existingKey.UserID == nil && adminUserID != nil {
				db.UpdateOne(ctx, "api_keys", bson.M{"_id": existingKey.ID}, //nolint:errcheck
					bson.M{"$set": bson.M{"user_id": adminUserID}})
				log.Printf("Updated system API key to associate with admin user")
			}
			return setting.RawKey, nil
		}
		// Key was deleted or invalid, fall through to create a new one
		log.Printf("System API key invalid, creating new one")
	}

	// Create new system API key associated with the admin user
	rawKey, _, err := apiKeyService.CreateAPIKeyForUser(ctx, "System (OAuth)", "Internal key for OAuth-authenticated MCP sessions", adminUserID)
	if err != nil {
		return "", fmt.Errorf("failed to create system API key: %w", err)
	}

	// Store raw key in settings for persistence
	_, upsertErr := db.Settings().UpdateOne(ctx,
		bson.M{"type": "system_api_key"},
		bson.M{"$set": bson.M{"type": "system_api_key", "raw_key": rawKey}},
		options.Update().SetUpsert(true),
	)
	if upsertErr != nil {
		return "", fmt.Errorf("failed to store system API key: %w", upsertErr)
	}

	log.Printf("Created system API key for OAuth MCP sessions (associated with admin user)")
	return rawKey, nil
}
