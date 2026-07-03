package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/dbutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Backfills visitors_human/visitors_bot arrays AND page_views_human/page_views_bot
// maps for existing hourly documents, using the user_agents page-view ratio to
// proportionally split the data.

func main() {
	mongoURI := dbutil.GetMongoURI()
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set. Set it via environment variable or config file.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")
	col := db.Collection("user_activity")

	// Find all hourly documents that have page_views but no page_views_human
	filter := bson.M{
		"user_id":    "__hourly__",
		"page_views": bson.M{"$exists": true},
		"$or": bson.A{
			bson.M{"page_views_human": bson.M{"$exists": false}},
		},
	}

	cursor, err := col.Find(ctx, filter)
	if err != nil {
		log.Fatalf("Find error: %v", err)
	}
	defer cursor.Close(ctx)

	var migrated, skipped int
	for cursor.Next(ctx) {
		var doc struct {
			ID         interface{}    `bson:"_id"`
			Date       string         `bson:"date"`
			PageViews  map[string]int `bson:"page_views"`
			UserAgents map[string]int `bson:"user_agents"`
		}
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Decode error: %v", err)
			continue
		}

		if len(doc.PageViews) == 0 {
			skipped++
			continue
		}

		// Calculate bot ratio from user_agents page view counts
		botViews := 0
		totalViews := 0
		for cat, hits := range doc.UserAgents {
			totalViews += hits
			if cat == "Bot" {
				botViews += hits
			}
		}

		botRatio := 0.0
		if totalViews > 0 {
			botRatio = float64(botViews) / float64(totalViews)
		}

		// Split each page's views proportionally
		pvHuman := make(map[string]int)
		pvBot := make(map[string]int)
		for path, count := range doc.PageViews {
			botCount := int(math.Round(float64(count) * botRatio))
			if botCount > count {
				botCount = count
			}
			humanCount := count - botCount
			if humanCount > 0 {
				pvHuman[path] = humanCount
			}
			if botCount > 0 {
				pvBot[path] = botCount
			}
		}

		setFields := bson.M{
			"page_views_human": pvHuman,
			"page_views_bot":   pvBot,
		}

		_, err := col.UpdateOne(ctx,
			bson.M{"_id": doc.ID},
			bson.M{"$set": setFields},
		)
		if err != nil {
			log.Printf("Update error for %s: %v", doc.Date, err)
			continue
		}
		migrated++

		totalPV := 0
		for _, v := range doc.PageViews {
			totalPV += v
		}
		totalHuman := 0
		for _, v := range pvHuman {
			totalHuman += v
		}
		totalBot := 0
		for _, v := range pvBot {
			totalBot += v
		}
		fmt.Printf("  %s: %d page views -> %d human, %d bot across %d pages (bot ratio: %.0f%%)\n",
			doc.Date, totalPV, totalHuman, totalBot, len(doc.PageViews), botRatio*100)
	}

	fmt.Printf("\nDone. Migrated: %d, Skipped: %d\n", migrated, skipped)
}
