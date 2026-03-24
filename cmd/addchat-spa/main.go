// cmd/addchat-spa/main.go — injects the chat widget script tag into the
// SPA homepage content item (full_path="/", use_theme=false, Blank Page).
// The theme head_html is not used by this page, so we must inject directly.
// Run from the lightcms directory:
//
//	go run cmd/addchat-spa/main.go
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

	// Fetch the homepage content item
	var item struct {
		ID       interface{}            `bson:"_id"`
		UseTheme bool                   `bson:"use_theme"`
		Data     map[string]interface{} `bson:"data"`
	}
	err = db.Content().FindOne(ctx, bson.M{"full_path": "/"}).Decode(&item)
	if err != nil {
		log.Fatalf("fetch homepage: %v", err)
	}

	content, _ := item.Data["content"].(string)

	if strings.Contains(content, "chat-widget.js") {
		fmt.Println("chat-widget.js already present in homepage content — nothing to do.")
		return
	}

	// Inject before </head> if present, otherwise prepend
	var newContent string
	if idx := strings.Index(content, "</head>"); idx >= 0 {
		newContent = content[:idx] + scriptTag + "\n" + content[idx:]
	} else {
		newContent = scriptTag + "\n" + content
	}

	_, err = db.Content().UpdateOne(
		ctx,
		bson.M{"_id": item.ID},
		bson.M{"$set": bson.M{"data.content": newContent, "updated_at": time.Now()}},
		options.Update().SetUpsert(false),
	)
	if err != nil {
		log.Fatalf("update homepage: %v", err)
	}

	fmt.Println("✓ Added chat-widget.js to homepage content.")
	fmt.Println("  Publish the homepage from the admin UI to regenerate the static file.")
}
