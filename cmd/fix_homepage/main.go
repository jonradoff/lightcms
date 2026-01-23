package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"lightcms/internal/dbutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	mongoURI := dbutil.GetMongoURI()
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set. Set it via environment variable or config file.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")

	// Find homepage template ID
	var template struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err = db.Collection("templates").FindOne(ctx, bson.M{"slug": "homepage"}).Decode(&template)
	if err != nil {
		log.Fatal("Homepage template not found:", err)
	}
	fmt.Printf("Homepage template ID: %s\n", template.ID.Hex())

	// Find all content at "/"
	cursor, err := db.Collection("content").Find(ctx, bson.M{"full_path": "/"})
	if err != nil {
		log.Fatal(err)
	}
	var contents []bson.M
	cursor.All(ctx, &contents)

	fmt.Printf("Found %d content items at /\n", len(contents))
	for _, c := range contents {
		fmt.Printf("  ID: %v, Title: %v, Template: %v\n", c["_id"], c["title"], c["template_name"])
	}

	// Delete all content at "/" that isn't the homepage template
	for _, c := range contents {
		templateName, _ := c["template_name"].(string)
		if templateName != "homepage" {
			id := c["_id"].(primitive.ObjectID)
			fmt.Printf("Deleting non-homepage content: %s (%s)\n", id.Hex(), templateName)
			db.Collection("content").DeleteOne(ctx, bson.M{"_id": id})
		}
	}

	// Check if we need to create the homepage
	count, _ := db.Collection("content").CountDocuments(ctx, bson.M{"full_path": "/", "template_name": "homepage"})
	if count == 0 {
		fmt.Println("Creating homepage content...")
		now := time.Now()
		content := bson.M{
			"template_id":      template.ID,
			"template_name":    "homepage",
			"title":            "Metavert",
			"slug":             "",
			"full_path":        "/",
			"category":         "",
			"meta_description": "Metavert is a venture studio building companies that will create the metaverse: the embodied internet of the future.",
			"data": bson.M{
				"hero_tagline": `<h1><strong>metavert</strong> <em>(noun)</em></h1>
<p>1. One that transforms or converts to the metaverse</p>
<p>2. A venture studio building companies that will create the metaverse: the embodied internet of the future</p>`,
				"intro_content": `<p><strong>Metavert</strong> is a venture studio building the metaverse: the embodied internet of the future. Founded by <a href="https://linkedin.com/in/jonradoff" target="_blank">Jon Radoff</a>, we invest in and help build companies across the metaverse value chain.</p>
<p><a href="/concepts-glossary" class="cta-button">Explore Concepts</a></p>`,
				"sections": `<div class="feature-grid">
	<div class="feature-card">
		<h3>Creator Economy</h3>
		<p>Software and marketplaces that enable creative people to add content to the metaverse, from individual assets to complete virtual worlds.</p>
		<p><a href="/creator-economy">Learn more →</a></p>
	</div>
	<div class="feature-card">
		<h3>Decentralization</h3>
		<p>Technologies and design patterns shifting power away from centralized authorities toward individual creators through blockchain and open standards.</p>
		<p><a href="/decentralization">Learn more →</a></p>
	</div>
	<div class="feature-card">
		<h3>Real-Time Technology</h3>
		<p>The GameTech stack: 3D engines, live services, and infrastructure that power immersive experiences and persistent virtual worlds.</p>
		<p><a href="/gametech">Learn more →</a></p>
	</div>
	<div class="feature-card">
		<h3>Games</h3>
		<p>Games have moved technology forward in innovative ways, advancing real-time networking and graphics to form the foundation of the metaverse.</p>
		<p><a href="/games">Learn more →</a></p>
	</div>
	<div class="feature-card">
		<h3>Spatial Computing</h3>
		<p>Technology integrating humans into computing environments through AR, VR, and extended reality systems.</p>
		<p><a href="/spatial-computing">Learn more →</a></p>
	</div>
	<div class="feature-card">
		<h3>Machine Intelligence</h3>
		<p>AI enabling sophisticated interfaces, physical world simulation, and interactive virtual beings within the metaverse.</p>
		<p><a href="/artificial-intelligence">Learn more →</a></p>
	</div>
</div>`,
				"quote_text":   "The future will be about the dematerialization of things into experiences. The metaverse is the continuation of that transformation.",
				"quote_author": "Jon Radoff",
			},
			"published":    true,
			"published_at": &now,
			"use_header":   true,
			"use_footer":   true,
			"use_theme":    true,
			"raw_mode":     false,
			"created_at":   now,
			"updated_at":   now,
		}
		result, err := db.Collection("content").InsertOne(ctx, content)
		if err != nil {
			log.Fatal("Error creating homepage:", err)
		}
		fmt.Printf("Created homepage: %v\n", result.InsertedID)
	} else {
		fmt.Println("Homepage already exists with correct template")
	}

	fmt.Println("\nDone!")
}
