package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonradoff/lightcms/v7/internal/database"
	"github.com/jonradoff/lightcms/v7/internal/models"
	"github.com/jonradoff/lightcms/v7/internal/testutil"

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

// --- ExtractPlainText ---

func TestExtractPlainText_Basic(t *testing.T) {
	c := &models.Content{
		Title:           "My Page",
		MetaDescription: "A description",
		Data: map[string]interface{}{
			"body": "<p>Hello <b>world</b></p>",
		},
	}
	got := ExtractPlainText(c)
	if !strings.Contains(got, "My Page") {
		t.Error("expected title in plain text")
	}
	if !strings.Contains(got, "A description") {
		t.Error("expected meta description in plain text")
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Error("expected stripped HTML content")
	}
	if strings.Contains(got, "<p>") || strings.Contains(got, "<b>") {
		t.Error("HTML tags should be stripped")
	}
}

func TestExtractPlainText_Empty(t *testing.T) {
	c := &models.Content{}
	got := ExtractPlainText(c)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractPlainText_NonStringData(t *testing.T) {
	c := &models.Content{
		Title: "Test",
		Data: map[string]interface{}{
			"count": 42,
			"body":  "real text",
		},
	}
	got := ExtractPlainText(c)
	if !strings.Contains(got, "real text") {
		t.Error("expected string data fields")
	}
	if strings.Contains(got, "42") {
		t.Error("non-string fields should be skipped")
	}
}

// --- extractSnippet ---

func TestExtractSnippet_Match(t *testing.T) {
	text := strings.Repeat("word ", 50) + "TARGET" + strings.Repeat(" word", 50)
	got := extractSnippet(text, "target")
	if !strings.Contains(got, "TARGET") {
		t.Error("expected snippet to contain match")
	}
}

func TestExtractSnippet_NoMatch(t *testing.T) {
	text := "This is some text without the search term"
	got := extractSnippet(text, "missing")
	if got == "" {
		t.Error("expected truncated text as fallback")
	}
}

func TestExtractSnippet_Empty(t *testing.T) {
	got := extractSnippet("", "query")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- truncateText ---

func TestTruncateText_Short(t *testing.T) {
	got := truncateText("hello", 100)
	if got != "hello" {
		t.Errorf("expected no truncation, got %q", got)
	}
}

func TestTruncateText_Long(t *testing.T) {
	text := strings.Repeat("word ", 100)
	got := truncateText(text, 30)
	if len(got) > 40 { // some slack for "..."
		t.Errorf("expected truncated text, got length %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected ... suffix")
	}
}

// --- extractPhrases ---

func TestExtractPhrases_Basic(t *testing.T) {
	freq := make(map[string]int)
	extractPhrases("The quick brown fox jumps over the lazy dog", freq)

	// "quick", "brown", "fox", "jumps", "lazy", "dog" should be extracted
	// (stop words like "the", "over" are excluded)
	if freq["quick"] == 0 {
		t.Error("expected 'quick' in freq")
	}
	if freq["fox"] == 0 {
		t.Error("expected 'fox' in freq")
	}
	// 2-word phrases should exist
	if freq["quick brown"] == 0 {
		t.Error("expected 'quick brown' phrase")
	}
}

func TestExtractPhrases_Empty(t *testing.T) {
	freq := make(map[string]int)
	extractPhrases("", freq)
	if len(freq) != 0 {
		t.Errorf("expected empty freq map, got %d entries", len(freq))
	}
}

// --- SearchService DB methods ---

func TestSearchService_GetNavPaths(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	paths := svc.getNavPaths(ctx)
	// With no theme header, should return nil or empty
	if paths != nil && len(paths) > 0 {
		t.Errorf("expected nil/empty nav paths, got %v", paths)
	}
}

func TestSearchService_GetSearchConfig(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	cfg := svc.getSearchConfig(ctx)
	if cfg == nil {
		t.Fatal("expected non-nil search config")
	}
	if cfg.NavBoost != 0.15 {
		t.Errorf("expected default NavBoost 0.15, got %f", cfg.NavBoost)
	}
}

func TestSearchService_SearchFullText_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	results, err := svc.SearchFullText(ctx, "nonexistent query", 10)
	if err != nil {
		t.Fatalf("SearchFullText: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchService_Suggest_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	result, err := svc.Suggest(ctx, "xyz", 5)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil SuggestResult")
	}
}

func TestSearchService_EmbeddingStats(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	total, withEmb, err := svc.EmbeddingStats(ctx)
	if err != nil {
		t.Fatalf("EmbeddingStats: %v", err)
	}
	if total < 0 || withEmb < 0 {
		t.Errorf("unexpected negative stats: total=%d, withEmbedding=%d", total, withEmb)
	}
}

func TestSearchService_Search_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	results, err := svc.Search(ctx, "nothing", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchService_RebuildKeywords_Empty(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	if err := svc.RebuildKeywords(ctx); err != nil {
		t.Fatalf("RebuildKeywords: %v", err)
	}
}

func TestSearchService_RebuildKeywords_WithContent(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	ctx := context.Background()

	// Seed some published content
	for i, title := range []string{"Go Programming Language", "Go Programming Tips", "Advanced Go Programming"} {
		db.Collection("content").InsertOne(ctx, map[string]interface{}{
			"title":            title,
			"full_path":        "/go-" + string(rune('a'+i)),
			"published":        true,
			"meta_description": "Learn " + title,
		})
	}

	if err := svc.RebuildKeywords(ctx); err != nil {
		t.Fatalf("RebuildKeywords with content: %v", err)
	}
}

func TestSearchService_InvalidateSearchConfigCache(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "")
	// Should not panic
	svc.InvalidateSearchConfigCache()
}

func TestSearchService_SearchSemantic_NoVoyageKey(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	svc := NewSearchService(db, "") // no voyage key
	ctx := context.Background()

	results, err := svc.SearchSemantic(ctx, "hello", 10)
	// Should return empty or error gracefully (no voyage key)
	_ = results
	_ = err
}

func TestOllamaEmbeddingProvider(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	// Fake Ollama server.
	var gotModel, gotPrompt string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		gotModel, gotPrompt = req.Model, req.Prompt
		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": []float32{0.1, 0.2, 0.3},
		})
	}))
	defer fake.Close()

	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "ollama")
	t.Setenv("OLLAMA_URL", fake.URL)
	t.Setenv("OLLAMA_EMBED_MODEL", "test-embed-model")

	s := NewSearchService(db, "") // no Voyage key
	if !s.EmbeddingsEnabled() {
		t.Fatal("EmbeddingsEnabled should be true with ollama provider")
	}
	if s.HasVoyageKey() {
		t.Fatal("HasVoyageKey should be false")
	}

	emb, err := s.generateEmbedding(context.Background(), "hello world", "document")
	if err != nil {
		t.Fatalf("generateEmbedding: %v", err)
	}
	if len(emb) != 3 || emb[1] != 0.2 {
		t.Errorf("embedding = %v", emb)
	}
	if gotModel != "test-embed-model" || gotPrompt != "hello world" {
		t.Errorf("request: model=%q prompt=%q", gotModel, gotPrompt)
	}

	// Error paths: non-200 and empty embedding.
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", 404)
	}))
	defer fail.Close()
	t.Setenv("OLLAMA_URL", fail.URL)
	s2 := NewSearchService(db, "")
	if _, err := s2.generateEmbedding(context.Background(), "x", "document"); err == nil {
		t.Error("expected error from failing ollama server")
	}
}

func TestEmbeddingsDisabledWithoutProvider(t *testing.T) {
	db, cleanup := testutil.MustConnectTestDB(t)
	defer cleanup()

	t.Setenv("LIGHTCMS_EMBEDDINGS_PROVIDER", "")
	s := NewSearchService(db, "")
	if s.EmbeddingsEnabled() {
		t.Error("EmbeddingsEnabled should be false with no provider")
	}
	if _, _, err := s.BatchGenerateEmbeddings(context.Background()); err == nil {
		t.Error("BatchGenerateEmbeddings should error without a provider")
	}
}
