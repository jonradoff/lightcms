package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/testutil"
)

func TestCreateAPIKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	rawKey, apiKey, err := svc.CreateAPIKey(ctx, "Test Key", "A test key")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Raw key should have lc_ prefix
	if !strings.HasPrefix(rawKey, "lc_") {
		t.Errorf("expected rawKey to start with lc_, got %q", rawKey)
	}

	// API key metadata should be set
	if apiKey.Name != "Test Key" {
		t.Errorf("expected name 'Test Key', got %q", apiKey.Name)
	}
	if apiKey.Description != "A test key" {
		t.Errorf("expected description 'A test key', got %q", apiKey.Description)
	}
	if apiKey.Prefix == "" {
		t.Error("expected non-empty prefix")
	}
	if apiKey.KeyHash == "" {
		t.Error("expected non-empty key hash")
	}
	if apiKey.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
}

func TestCreateAPIKey_EmptyName(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	_, _, err := svc.CreateAPIKey(context.Background(), "", "desc")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestListAPIKeys(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	// Create multiple keys
	svc.CreateAPIKey(ctx, "Key 1", "First")
	time.Sleep(10 * time.Millisecond)
	svc.CreateAPIKey(ctx, "Key 2", "Second")

	keys, err := svc.ListAPIKeys(ctx)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// Should be sorted newest first
	if keys[0].Name != "Key 2" {
		t.Errorf("expected newest key first, got %q", keys[0].Name)
	}
}

func TestListAPIKeys_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	keys, err := svc.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestDeleteAPIKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	_, apiKey, _ := svc.CreateAPIKey(ctx, "To Delete", "")

	err := svc.DeleteAPIKey(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("DeleteAPIKey failed: %v", err)
	}

	// Verify it's gone
	keys, _ := svc.ListAPIKeys(ctx)
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after deletion, got %d", len(keys))
	}
}

func TestValidateAPIKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	rawKey, _, err := svc.CreateAPIKey(ctx, "Valid Key", "")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Valid key should work
	apiKey, err := svc.ValidateAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if apiKey.Name != "Valid Key" {
		t.Errorf("expected name 'Valid Key', got %q", apiKey.Name)
	}
	if apiKey.LastUsedAt == nil {
		t.Error("expected last_used_at to be set after validation")
	}
}

func TestValidateAPIKey_Invalid(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	// Wrong key
	_, err := svc.ValidateAPIKey(ctx, "lc_0000000000000000000000000000000000")
	if err == nil {
		t.Error("expected error for invalid API key")
	}

	// Bad format
	_, err = svc.ValidateAPIKey(ctx, "bad")
	if err == nil {
		t.Error("expected error for bad key format")
	}

	// No prefix
	_, err = svc.ValidateAPIKey(ctx, "xx_12345678")
	if err == nil {
		t.Error("expected error for wrong prefix")
	}
}
