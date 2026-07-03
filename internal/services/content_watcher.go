package services

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// WatchForChanges starts a MongoDB change stream to watch for content updates
// and automatically regenerates static pages when content is modified.
// This enables real-time sync when the database is modified externally (e.g., via MCP).
// The function blocks until the context is cancelled or an error occurs.
func (s *ContentService) WatchForChanges(ctx context.Context) {
	// Watch for update and replace operations on the content collection
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "operationType", Value: bson.M{"$in": []string{"update", "replace"}}},
		}}},
	}

	for {
		// Check if context is cancelled before starting/restarting stream
		select {
		case <-ctx.Done():
			log.Println("Content change watcher stopped")
			return
		default:
		}

		stream, err := s.db.WatchCollection(ctx, "content", pipeline)
		if err != nil {
			log.Printf("Failed to start content change stream: %v", err)
			// Wait before retrying to avoid tight loop on persistent errors
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}

		log.Println("Content change stream started - watching for database updates")

		// Process change events
		for stream.Next(ctx) {
			var event bson.M
			if err := stream.Decode(&event); err != nil {
				log.Printf("Failed to decode change event: %v", err)
				continue
			}

			// Extract the document ID from the change event
			docKey, ok := event["documentKey"].(bson.M)
			if !ok {
				continue
			}
			id, ok := docKey["_id"].(primitive.ObjectID)
			if !ok {
				continue
			}

			// Fetch the updated content and regenerate if published
			content, err := s.GetContent(ctx, id)
			if err != nil {
				log.Printf("Failed to get content %s for regeneration: %v", id.Hex(), err)
				continue
			}

			if content.Published && !content.Deleted {
				if err := s.GenerateStaticPage(ctx, content); err != nil {
					log.Printf("Failed to regenerate %s: %v", content.FullPath, err)
				} else {
					log.Printf("Regenerated static page: %s", content.FullPath)
				}
			} else if content.Deleted || !content.Published {
				// Remove static page if content was unpublished or deleted
				s.removeStaticPage(content.FullPath)
				log.Printf("Removed static page: %s", content.FullPath)
			}
		}

		// Stream ended - check for errors
		if err := stream.Err(); err != nil {
			log.Printf("Content change stream error: %v", err)
		}
		stream.Close(ctx)

		// Brief pause before reconnecting
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}
