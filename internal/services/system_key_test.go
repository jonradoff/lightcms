package services

import (
	"context"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/testutil"
)

func TestEnsureSystemAPIKey_CreatesNewKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	apiKeySvc := NewAPIKeyService(db)
	ctx := context.Background()

	rawKey, err := EnsureSystemAPIKey(ctx, db, apiKeySvc, nil)
	if err != nil {
		t.Fatalf("EnsureSystemAPIKey failed: %v", err)
	}

	if rawKey == "" {
		t.Error("expected non-empty raw key")
	}

	// Key should be valid
	_, err = apiKeySvc.ValidateAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("system key should be valid: %v", err)
	}
}

func TestEnsureSystemAPIKey_ReturnsCachedKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	apiKeySvc := NewAPIKeyService(db)
	ctx := context.Background()

	// First call creates
	key1, err := EnsureSystemAPIKey(ctx, db, apiKeySvc, nil)
	if err != nil {
		t.Fatalf("first EnsureSystemAPIKey failed: %v", err)
	}

	// Second call returns same key
	key2, err := EnsureSystemAPIKey(ctx, db, apiKeySvc, nil)
	if err != nil {
		t.Fatalf("second EnsureSystemAPIKey failed: %v", err)
	}

	if key1 != key2 {
		t.Error("expected same key on second call")
	}
}

func TestEnsureSystemAPIKey_RecreatesDeletedKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	apiKeySvc := NewAPIKeyService(db)
	ctx := context.Background()

	// Create system key
	key1, _ := EnsureSystemAPIKey(ctx, db, apiKeySvc, nil)

	// Delete all API keys (simulating key deletion)
	keys, _ := apiKeySvc.ListAPIKeys(ctx)
	for _, k := range keys {
		apiKeySvc.DeleteAPIKey(ctx, k.ID)
	}

	// Should create a new key since the old one is invalid
	key2, err := EnsureSystemAPIKey(ctx, db, apiKeySvc, nil)
	if err != nil {
		t.Fatalf("EnsureSystemAPIKey after deletion failed: %v", err)
	}

	if key1 == key2 {
		t.Error("expected different key after deletion")
	}

	// New key should be valid
	_, err = apiKeySvc.ValidateAPIKey(ctx, key2)
	if err != nil {
		t.Fatal("new system key should be valid")
	}
}
