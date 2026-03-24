package handlers

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lightcms/internal/auth"
	"lightcms/internal/database"
	"lightcms/internal/services"

	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

var (
	sharedTestOnce sync.Once
	sharedTestDB   *database.DB
	sharedTestErr  error
)

// loadTestEnv walks up from the working directory looking for .env.test and
// loads it into the process environment.
func loadTestEnv(t *testing.T) {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		data, err := os.ReadFile(filepath.Join(dir, ".env.test"))
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if idx := strings.IndexByte(line, '='); idx > 0 {
					os.Setenv(strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]))
				}
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// testDB returns a shared test database connection, skipping if MONGODB_URI
// is not set. The connection is created once per package binary using sync.Once.
func testDB(t *testing.T) *database.DB {
	t.Helper()
	sharedTestOnce.Do(func() {
		loadTestEnv(t)
		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			return
		}
		dbName := os.Getenv("DATABASE_NAME")
		if dbName == "" {
			dbName = "lightcms-test"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		wc := writeconcern.New(writeconcern.WMajority())
		wcOpts := options.Client().SetWriteConcern(wc)
		db, err := database.Connect(ctx, uri, dbName, wcOpts)
		if err != nil {
			sharedTestErr = err
			return
		}
		sharedTestDB = db
	})
	if sharedTestErr != nil {
		t.Fatalf("testDB: %v", sharedTestErr)
	}
	if sharedTestDB == nil {
		t.Skip("skipping: MONGODB_URI not set")
	}
	return sharedTestDB
}

// cleanupCollections drops test collections for isolation.
// Dropping (rather than just deleting documents) ensures indexes are also cleared,
// preventing duplicate key errors on re-creation and avoiding Atlas collection
// accumulation when tests use unique indexes (e.g. users.email).
// Drops are issued in parallel to minimise Atlas round-trip overhead.
func cleanupCollections(t *testing.T, db *database.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collections := []string{
		"content", "content_versions", "templates", "folders", "collections",
		"settings", "theme_versions", "assets", "api_keys", "redirects",
		"snippets", "users", "audit_logs", "contact_messages", "login_attempts",
		"oauth_clients", "user_activity",
	}
	var wg sync.WaitGroup
	for _, name := range collections {
		wg.Add(1)
		go func(col string) {
			defer wg.Done()
			db.Collection(col).Drop(ctx) //nolint:errcheck
		}(name)
	}
	wg.Wait()
}

// newTestHandler returns a Handler wired to a real test DB and a cleanup function.
func newTestHandler(t *testing.T) (*Handler, func()) {
	t.Helper()
	db := testDB(t)
	cleanupCollections(t, db)

	// Services
	userService := services.NewUserService(db)
	auditService := services.NewAuditService(db)
	snippetService := services.NewSnippetService(db)
	contentService := services.NewContentService(db)
	searchService := services.NewSearchService(db, "") // no Voyage key in tests
	apiKeyService := services.NewAPIKeyService(db)
	_ = apiKeyService // used internally by New()

	// Session store
	store := sessions.NewCookieStore([]byte("test-session-secret"))

	// Auth manager
	authManager := auth.NewManager(store, db, userService)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := authManager.MigrateToMultiUser(ctx); err != nil {
		t.Fatalf("MigrateToMultiUser: %v", err)
	}

	// Handler
	h := New(db, authManager, "http://localhost:8082", "test", userService, auditService, snippetService)
	h.SetSearchService(searchService)
	h.SetContentService(contentService)
	h.SetProxyConfig(nil)

	cleanup := func() {
		cleanupCollections(t, db)
	}
	return h, cleanup
}

// newTestAPIHandler returns an APIHandler wired to a real test DB, the DB itself
// (for seeding data), and a cleanup function.
func newTestAPIHandler(t *testing.T) (*APIHandler, *database.DB, func()) {
	t.Helper()
	db := testDB(t)
	cleanupCollections(t, db)

	// Services
	contentService := services.NewContentService(db)
	templateService := services.NewTemplateService(db, contentService)
	assetService := services.NewAssetService(db)
	settingsService := services.NewSettingsService(db, contentService)
	apiKeyService := services.NewAPIKeyService(db)
	auditService := services.NewAuditService(db)
	snippetService := services.NewSnippetService(db)
	searchService := services.NewSearchService(db, "")

	ah := NewAPIHandler(contentService, templateService, assetService, settingsService, apiKeyService, auditService, snippetService)
	ah.SetSearchService(searchService)

	cleanup := func() {
		cleanupCollections(t, db)
	}
	return ah, db, cleanup
}

// seedTemplate inserts a minimal test template and returns its ID.
func seedTemplate(t *testing.T, db *database.DB, name, slug string) primitive.ObjectID {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id":         id,
		"name":        name,
		"slug":        slug,
		"html_layout": "<html><body>{{.Body}}</body></html>",
		"fields":      bson.A{},
		"created_at":  now,
		"updated_at":  now,
	}
	_, err := db.Collection("templates").InsertOne(ctx, doc)
	if err != nil {
		t.Fatalf("seedTemplate: %v", err)
	}
	return id
}

// seedContent inserts a minimal test content item and returns its ID.
func seedContent(t *testing.T, db *database.DB, templateID primitive.ObjectID, title, slug, fullPath string) primitive.ObjectID {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	id := primitive.NewObjectID()
	doc := bson.M{
		"_id":         id,
		"template_id": templateID,
		"title":       title,
		"slug":        slug,
		"full_path":   fullPath,
		"status":      "draft",
		"fields":      bson.M{},
		"created_at":  now,
		"updated_at":  now,
		"deleted":     false,
	}
	_, err := db.Collection("content").InsertOne(ctx, doc)
	if err != nil {
		t.Fatalf("seedContent: %v", err)
	}
	return id
}
