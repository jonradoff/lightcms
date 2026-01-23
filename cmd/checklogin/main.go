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

	// Check login_attempts collection for rate limiting
	fmt.Println("--- Checking login_attempts ---")
	cursor, err := db.Collection("login_attempts").Find(ctx, bson.M{})
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		var attempts []bson.M
		cursor.All(ctx, &attempts)
		for _, a := range attempts {
			fmt.Printf("Login attempt: %+v\n", a)
		}
		if len(attempts) == 0 {
			fmt.Println("No login attempts recorded")
		}
	}

	// Clear rate limits
	fmt.Println("\n--- Clearing all login attempts ---")
	result, _ := db.Collection("login_attempts").DeleteMany(ctx, bson.M{})
	fmt.Printf("Deleted %d login attempt records\n", result.DeletedCount)

	// Double check the admin password
	fmt.Println("\n--- Verifying password in settings collection ---")
	var settings bson.M
	err = db.Collection("settings").FindOne(ctx, bson.M{"type": "admin"}).Decode(&settings)
	if err != nil {
		fmt.Println("No admin settings found:", err)
	} else {
		if hash, ok := settings["password_hash"].(string); ok {
			fmt.Printf("Hash found: %s\n", hash[:20]+"...")
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(testPassword))
			if err != nil {
				fmt.Printf("Password verification FAILED: %v\n", err)
			} else {
				fmt.Println("Password verification SUCCEEDED!")
			}
		} else {
			fmt.Println("No password_hash field found in settings!")
			fmt.Printf("Settings content: %+v\n", settings)
		}
	}
}
