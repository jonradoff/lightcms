package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"lightcms/internal/dbutil"

	"go.mongodb.org/mongo-driver/bson"
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

	// Find content at "/"
	var content bson.M
	err = db.Collection("content").FindOne(ctx, bson.M{"full_path": "/"}).Decode(&content)
	if err != nil {
		log.Fatal(err)
	}

	jsonBytes, _ := json.MarshalIndent(content, "", "  ")
	fmt.Println(string(jsonBytes))
}
