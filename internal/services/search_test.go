package services

import (
	"context"
	"testing"

	"lightcms/internal/database"
	"lightcms/internal/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- pure function tests (no DB required) ---

func TestPathBoost_NavLinked(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{"/about": true, "/blog": true}

	got := pathBoost("/about", "Standard Page", navPaths, cfg)
	if got != cfg.NavBoost {
		t.Errorf("expected NavBoost %f for nav-linked path, got %f", cfg.NavBoost, got)
	}
}

func TestPathBoost_DemotedPath(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{}

	got := pathBoost("/videos/my-clip", "Blog Post", navPaths, cfg)
	if got != cfg.DemoteScore {
		t.Errorf("expected DemoteScore %f for demoted path, got %f", cfg.DemoteScore, got)
	}
}

func TestPathBoost_DemotedPath_CaseInsensitive(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{}

	got := pathBoost("/VIDEOS/clip", "Blog Post", navPaths, cfg)
	if got != cfg.DemoteScore {
		t.Errorf("expected DemoteScore for uppercase demoted path, got %f", got)
	}
}

func TestPathBoost_BoostedTemplate(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{}

	got := pathBoost("/some/page", "Concept Page", navPaths, cfg)
	if got != cfg.BoostTemplateScore {
		t.Errorf("expected BoostTemplateScore %f for boosted template, got %f", cfg.BoostTemplateScore, got)
	}
}

func TestPathBoost_BoostedTemplate_CaseInsensitive(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{}

	got := pathBoost("/page", "CONCEPT Guide", navPaths, cfg)
	if got != cfg.BoostTemplateScore {
		t.Errorf("expected BoostTemplateScore for uppercase template, got %f", got)
	}
}

func TestPathBoost_Neutral(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{}

	got := pathBoost("/ordinary/page", "Blog Post", navPaths, cfg)
	if got != 0 {
		t.Errorf("expected 0 for neutral path, got %f", got)
	}
}

func TestPathBoost_NavTakesPriority(t *testing.T) {
	// If a path is both nav-linked and boosted template, nav wins (checked first)
	cfg := database.DefaultSearchConfig()
	navPaths := map[string]bool{"/concepts/home": true}

	got := pathBoost("/concepts/home", "Concept Page", navPaths, cfg)
	if got != cfg.NavBoost {
		t.Errorf("expected NavBoost to take priority, got %f", got)
	}
}

func TestIsDemotedPath_True(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if !isDemotedPath("/videos/my-video", cfg) {
		t.Error("expected /videos/... to be demoted")
	}
	if !isDemotedPath("/video/clip", cfg) {
		t.Error("expected /video/... to be demoted")
	}
}

func TestIsDemotedPath_False(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if isDemotedPath("/blog/post", cfg) {
		t.Error("expected /blog/... not to be demoted")
	}
	if isDemotedPath("/", cfg) {
		t.Error("expected root not to be demoted")
	}
}

func TestIsDemotedPath_CaseInsensitive(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if !isDemotedPath("/VIDEOS/clip", cfg) {
		t.Error("expected case-insensitive demote match")
	}
}

func TestIsDemotedPath_EmptyPrefixes(t *testing.T) {
	cfg := &database.SearchConfig{}
	if isDemotedPath("/anything", cfg) {
		t.Error("expected false with no demote prefixes")
	}
}

func TestIsBoostedTemplate_True(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if !isBoostedTemplate("Concept Page", cfg) {
		t.Error("expected 'Concept Page' to be boosted")
	}
	if !isBoostedTemplate("my-concept-guide", cfg) {
		t.Error("expected 'my-concept-guide' to be boosted (contains 'concept')")
	}
}

func TestIsBoostedTemplate_False(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if isBoostedTemplate("Blog Post", cfg) {
		t.Error("expected 'Blog Post' not to be boosted")
	}
	if isBoostedTemplate("", cfg) {
		t.Error("expected empty string not to be boosted")
	}
}

func TestIsBoostedTemplate_CaseInsensitive(t *testing.T) {
	cfg := database.DefaultSearchConfig()
	if !isBoostedTemplate("CONCEPT GUIDE", cfg) {
		t.Error("expected case-insensitive boost match")
	}
}

func TestIsBoostedTemplate_EmptyTemplates(t *testing.T) {
	cfg := &database.SearchConfig{}
	if isBoostedTemplate("Concept Page", cfg) {
		t.Error("expected false with no boost templates")
	}
}

// --- SearchService constructor & simple methods ---

func TestNewSearchService(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "voyage_key")
	if svc == nil {
		t.Fatal("expected non-nil SearchService")
	}
}

func TestHasVoyageKey_True(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "some_key")
	if !svc.HasVoyageKey() {
		t.Error("expected HasVoyageKey()=true")
	}
}

func TestHasVoyageKey_False(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	if svc.HasVoyageKey() {
		t.Error("expected HasVoyageKey()=false with empty key")
	}
}

func TestInvalidateSearchConfigCache(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	// Should not panic even when nothing is cached
	svc.InvalidateSearchConfigCache()
	// And again after setting something
	svc.cachedSearchConfig = database.DefaultSearchConfig()
	svc.InvalidateSearchConfigCache()
	if svc.cachedSearchConfig != nil {
		t.Error("expected cachedSearchConfig to be nil after invalidation")
	}
}

// --- ListAPIKeysForUser (requires DB) ---

func TestAPIKeyService_ListAPIKeysForUser(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	userA := primitive.NewObjectID()
	userB := primitive.NewObjectID()

	// Create 2 keys for userA, 1 for userB
	svc.CreateAPIKeyForUser(ctx, "Key A1", "", &userA)
	svc.CreateAPIKeyForUser(ctx, "Key A2", "", &userA)
	svc.CreateAPIKeyForUser(ctx, "Key B1", "", &userB)

	keysA, err := svc.ListAPIKeysForUser(ctx, userA)
	if err != nil {
		t.Fatalf("ListAPIKeysForUser: %v", err)
	}
	if len(keysA) != 2 {
		t.Errorf("expected 2 keys for userA, got %d", len(keysA))
	}

	keysB, err := svc.ListAPIKeysForUser(ctx, userB)
	if err != nil {
		t.Fatalf("ListAPIKeysForUser userB: %v", err)
	}
	if len(keysB) != 1 {
		t.Errorf("expected 1 key for userB, got %d", len(keysB))
	}

	// Unknown user gets zero keys
	keysNone, err := svc.ListAPIKeysForUser(ctx, primitive.NewObjectID())
	if err != nil {
		t.Fatalf("ListAPIKeysForUser unknown: %v", err)
	}
	if len(keysNone) != 0 {
		t.Errorf("expected 0 keys for unknown user, got %d", len(keysNone))
	}
}

func TestAPIKeyService_CreateAPIKeyForUser_NoUserID(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	rawKey, key, err := svc.CreateAPIKey(ctx, "System Key", "no owner")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if key.UserID != nil {
		t.Error("expected nil UserID for system key")
	}
	if !isValidAPIKey(rawKey) {
		t.Errorf("unexpected key format: %q", rawKey)
	}
}

func isValidAPIKey(key string) bool {
	return len(key) > 3 && key[:3] == "lc_"
}

func TestAPIKeyService_ValidateAPIKey_InvalidFormat(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	_, err := svc.ValidateAPIKey(ctx, "bad_key")
	if err == nil {
		t.Error("expected error for invalid key format")
	}

	_, err = svc.ValidateAPIKey(ctx, "")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestAPIKeyService_DeleteAPIKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewAPIKeyService(db)
	ctx := context.Background()

	userID := primitive.NewObjectID()
	_, key, err := svc.CreateAPIKeyForUser(ctx, "Temp Key", "", &userID)
	if err != nil {
		t.Fatalf("CreateAPIKeyForUser: %v", err)
	}

	if err := svc.DeleteAPIKey(ctx, key.ID); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}

	keys, _ := svc.ListAPIKeysForUser(ctx, userID)
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after delete, got %d", len(keys))
	}
}

