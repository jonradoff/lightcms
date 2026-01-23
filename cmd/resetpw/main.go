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

	// Generate bcrypt hash for admin123
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		log.Fatal(err)
	}

	// Set password_hash to the bcrypt hash of admin123
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"type": "admin"},
		bson.M{"$set": bson.M{"password_hash": string(hash), "is_default_password": true}},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Modified %d document(s)\n", result.ModifiedCount)
	fmt.Printf("Password hash set to: %s\n", string(hash))
	fmt.Println("You can now login with 'admin123'")
}
