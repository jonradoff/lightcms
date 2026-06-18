package database

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestDBHelpers_CRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	col := "test_helpers_scratch"
	_ = db.Collection(col).Drop(ctx)
	defer db.Collection(col).Drop(ctx)

	// InsertManyUnordered
	docs := []interface{}{
		bson.M{"k": "a", "n": 1},
		bson.M{"k": "b", "n": 2},
		bson.M{"k": "c", "n": 3},
	}
	if _, err := db.InsertManyUnordered(ctx, col, docs); err != nil {
		t.Fatalf("InsertManyUnordered: %v", err)
	}

	// UpdateOne
	if err := db.UpdateOne(ctx, col, bson.M{"k": "a"}, bson.M{"$set": bson.M{"n": 10}}); err != nil {
		t.Fatalf("UpdateOne: %v", err)
	}

	// Aggregate (sum of n)
	var results []bson.M
	pipeline := []bson.M{{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$n"}}}}
	if err := db.Aggregate(ctx, col, pipeline, &results); err != nil {
		t.Fatalf("Aggregate: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 aggregate result, got %d", len(results))
	}

	// DeleteOne
	if err := db.DeleteOne(ctx, col, bson.M{"k": "b"}); err != nil {
		t.Fatalf("DeleteOne: %v", err)
	}

	// Collection handle is usable.
	n, err := db.Collection(col).CountDocuments(ctx, bson.M{})
	if err != nil {
		t.Fatalf("CountDocuments: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 docs after delete, got %d", n)
	}
}

func TestDBHelpers_ChatWidgetConfig(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_ = db.DeleteOne(ctx, "settings", bson.M{"_id": "chat_widget"})

	// Default returned when none stored.
	def := DefaultChatWidgetConfig()
	if def == nil {
		t.Fatal("DefaultChatWidgetConfig nil")
	}
	cfg, err := db.GetChatWidgetConfig(ctx)
	if err != nil {
		t.Fatalf("GetChatWidgetConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected a config (default)")
	}

	// Save then read back.
	cfg.Enabled = true
	cfg.WidgetTitle = "Ask"
	if err := db.SaveChatWidgetConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveChatWidgetConfig: %v", err)
	}
	got, err := db.GetChatWidgetConfig(ctx)
	if err != nil {
		t.Fatalf("GetChatWidgetConfig 2: %v", err)
	}
	if !got.Enabled || got.WidgetTitle != "Ask" {
		t.Errorf("config not persisted: %+v", got)
	}
}

func TestDBHelpers_LoginAttempts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_ = db.Collection("login_attempts").Drop(ctx)

	if _, err := db.GetAllLoginAttempts(ctx); err != nil {
		t.Fatalf("GetAllLoginAttempts: %v", err)
	}
	if err := db.ClearLoginAttemptsByIP(ctx, "203.0.113.1"); err != nil {
		t.Fatalf("ClearLoginAttemptsByIP: %v", err)
	}
}
