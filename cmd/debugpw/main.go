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
	testPassword := "admin123" // Default password for testing

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")

	// Check what's in admin_settings
	fmt.Println("--- Checking 'admin_settings' collection ---")
	var settings bson.M
	err = db.Collection("admin_settings").FindOne(ctx, bson.M{}).Decode(&settings)
	if err != nil {
		fmt.Println("No admin_settings found:", err)
	} else {
		fmt.Printf("Admin settings: %+v\n", settings)

		if hash, ok := settings["password_hash"].(string); ok {
			fmt.Printf("Hash length: %d\n", len(hash))
			fmt.Printf("Hash: %s\n", hash)

			// Try to verify the password
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(testPassword))
			if err != nil {
				fmt.Printf("Password verification FAILED: %v\n", err)
			} else {
				fmt.Println("Password verification SUCCEEDED!")
			}
		}
	}

	// Check settings collection with type: "admin" (this is what the app actually uses)
	fmt.Println("\n--- Checking 'settings' collection with type='admin' ---")
	var settings2 bson.M
	err = db.Collection("settings").FindOne(ctx, bson.M{"type": "admin"}).Decode(&settings2)
	if err != nil {
		fmt.Println("No admin settings found:", err)
	} else {
		fmt.Printf("Admin settings: %+v\n", settings2)
		if hash, ok := settings2["password_hash"].(string); ok {
			fmt.Printf("Hash: %s\n", hash)
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(testPassword))
			if err != nil {
				fmt.Printf("Password verification FAILED: %v\n", err)
			} else {
				fmt.Println("Password verification SUCCEEDED!")
			}
		}
	}
}
