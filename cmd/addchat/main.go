// cmd/addchat/main.go — one-time script to inject the chat widget script tag
// into the theme header_html. Run from the lightcms directory:
//
//	go run cmd/addchat/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"lightcms/config"
	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const scriptTag = `<script src="/static/js/chat-widget.js" async></script>`

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.MongoURI, "lightcms")
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	// Fetch current theme
	var theme struct {
		HeaderHTML string `bson:"header_html"`
		HeadHTML   string `bson:"head_html"`
	}
	err = db.Settings().FindOne(ctx, bson.M{"type": "theme"}).Decode(&theme)
	if err != nil {
		log.Fatalf("fetch theme: %v", err)
	}

	// Check if already present
	if strings.Contains(theme.HeadHTML, "chat-widget.js") || strings.Contains(theme.HeaderHTML, "chat-widget.js") {
		fmt.Println("chat-widget.js script tag already present — nothing to do.")
		return
	}

	// Append to head_html
	newHeadHTML := strings.TrimSpace(theme.HeadHTML) + "\n" + scriptTag

	_, err = db.Settings().UpdateOne(
		ctx,
		bson.M{"type": "theme"},
		bson.M{"$set": bson.M{"head_html": newHeadHTML, "updated_at": time.Now()}},
		options.Update().SetUpsert(false),
	)
	if err != nil {
		log.Fatalf("update theme: %v", err)
	}

	fmt.Println("✓ Added chat-widget.js to theme head_html.")
	fmt.Println("  Regenerate site pages to pick up the change.")
}
