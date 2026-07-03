package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jonradoff/lightcms/v6/internal/dbutil"

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

	// Usage: resetpw [email]
	// If email is provided, reset that user's password.
	// Otherwise, reset the first admin user's password.
	email := ""
	if len(os.Args) > 1 {
		email = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")

	// Generate bcrypt hash for admin123
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), 12)
	if err != nil {
		log.Fatal(err)
	}

	// Try users collection first (v2.0+ multi-user system)
	usersCol := db.Collection("users")
	filter := bson.M{}
	if email != "" {
		filter["email"] = email
	} else {
		filter["role"] = "admin"
	}

	result, err := usersCol.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": bson.M{"password_hash": string(hash), "is_default_password": true}},
	)
	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	if result.MatchedCount > 0 {
		fmt.Printf("Reset password for user (matched %d, modified %d)\n", result.MatchedCount, result.ModifiedCount)
		fmt.Println("You can now login with 'admin123' (you will be prompted to change it)")
		return
	}

	// Fallback: legacy settings collection (pre-v2.0)
	settingsCol := db.Collection("settings")
	legacyResult, err := settingsCol.UpdateOne(
		ctx,
		bson.M{"type": "admin"},
		bson.M{"$set": bson.M{"password_hash": string(hash), "is_default_password": true}},
	)
	if err != nil {
		log.Fatalf("Failed to update legacy settings: %v", err)
	}

	if legacyResult.MatchedCount > 0 {
		fmt.Printf("Reset legacy admin password (modified %d document(s))\n", legacyResult.ModifiedCount)
		fmt.Println("You can now login with 'admin123'")
		return
	}

	if email != "" {
		fmt.Printf("No user found with email '%s'\n", email)
	} else {
		fmt.Println("No admin user found in users or settings collection")
	}
	os.Exit(1)
}
