package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"lightcms/internal/dbutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	mongoURI := dbutil.GetMongoURI()
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set. Set it via environment variable or config file.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	collection := client.Database("lightcms").Collection("settings")

	// Find the admin settings
	var result bson.M
	err = collection.FindOne(ctx, bson.M{"type": "admin"}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		fmt.Println("No admin settings document found!")
		return
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Admin settings: %+v\n", result)

	hash, ok := result["password_hash"].(string)
	if !ok {
		fmt.Println("password_hash is not a string or is missing")
	} else {
		fmt.Printf("password_hash value: '%s' (len=%d)\n", hash, len(hash))
	}

	// Test bcrypt comparison with admin123
	if hash != "" {
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("admin123"))
		if err != nil {
			fmt.Printf("bcrypt compare failed: %v\n", err)
		} else {
			fmt.Println("bcrypt compare SUCCESS - admin123 matches!")
		}
	}
}
