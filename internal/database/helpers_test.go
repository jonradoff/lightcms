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

func TestDBHelpers_FaultHook(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	defer db.SetFaultHook(nil)

	// No hook → operation works.
	if _, err := db.Count(ctx, "settings", bson.M{}); err != nil {
		t.Fatalf("Count without hook: %v", err)
	}

	// Hook fails a specific op only.
	db.SetFaultHook(func(op, _ string) error {
		if op == "InsertOne" {
			return errFaultInjected
		}
		return nil
	})
	if _, err := db.InsertOne(ctx, "test_fault_scratch", bson.M{"x": 1}); err == nil {
		t.Error("InsertOne should fail with hook")
	}
	// A different op is unaffected.
	if _, err := db.Count(ctx, "settings", bson.M{}); err != nil {
		t.Errorf("Count should be unaffected by InsertOne hook: %v", err)
	}

	db.SetFaultHook(nil)
	if _, err := db.InsertOne(ctx, "test_fault_scratch", bson.M{"x": 2}); err != nil {
		t.Errorf("InsertOne after clearing hook: %v", err)
	}
	_ = db.Collection("test_fault_scratch").Drop(ctx)
}

var errFaultInjected = &faultErr{}

type faultErr struct{}

func (*faultErr) Error() string { return "injected" }

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
