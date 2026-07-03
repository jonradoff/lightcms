package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// TestInsertMany_SuccessAndFault covers the success path and the injected
// fault branch of InsertMany (the empty-docs branch is covered elsewhere).
func TestInsertMany_SuccessAndFault(t *testing.T) {
	db := testDB(t)
	defer db.SetFaultHook(nil)
	ctx := context.Background()

	docs := []interface{}{
		bson.M{"name": "one", "created_at": time.Now()},
		bson.M{"name": "two", "created_at": time.Now()},
	}
	if err := db.InsertMany(ctx, "contact_messages", docs); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	count, err := db.Count(ctx, "contact_messages", bson.M{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 inserted docs, got %d", count)
	}

	db.SetFaultHook(func(op, _ string) error {
		if op == "InsertMany" {
			return errors.New("injected")
		}
		return nil
	})
	if err := db.InsertMany(ctx, "contact_messages", docs); err == nil {
		t.Error("expected injected InsertMany error")
	}
	db.SetFaultHook(nil)
}

// TestDisconnect covers DB.Disconnect on a dedicated connection (the shared
// test connection must stay alive for other tests).
func TestDisconnect(t *testing.T) {
	loadTestEnv(t)
	uri := lookupEnv("MONGODB_URI")
	if uri == "" {
		t.Skip("skipping: MONGODB_URI not set")
	}
	dbName := lookupEnv("DATABASE_NAME")
	if dbName == "" {
		dbName = "lightcms-test"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := Connect(ctx, uri, dbName)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := db.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	// A second disconnect on an already-closed client returns an error.
	if err := db.Disconnect(ctx); err == nil {
		t.Error("expected error on double Disconnect")
	}
}
