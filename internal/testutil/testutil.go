package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
)

const testDBName = "lightcms-test"

// loadEnvTest loads the .env.test file into the process environment.
func loadEnvTest() {
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		envPath := filepath.Join(dir, ".env.test")
		data, err := os.ReadFile(envPath)
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if idx := strings.IndexByte(line, '='); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					val := strings.TrimSpace(line[idx+1:])
					os.Setenv(key, val)
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

// MustConnectTestDB connects to the test database. It fails the test if the
// database name does not contain "test" to prevent accidental use of production databases.
// Returns the database handle and a cleanup function that drops the database.
func MustConnectTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	loadEnvTest()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("skipping: MONGODB_URI not set (no .env.test file)")
	}

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = testDBName
	}

	if !strings.Contains(strings.ToLower(dbName), "test") {
		t.Fatalf("testutil: REFUSING to run tests — database name %q does not contain 'test'. "+
			"This safety guard prevents accidental use of production databases.", dbName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, uri, dbName)
	if err != nil {
		t.Fatalf("testutil: failed to connect to test database: %v", err)
	}

	// Clean collections at start for isolation (previous test may have left data)
	CleanupCollections(t, db)

	cleanup := func() {
		CleanupCollections(t, db)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		db.Disconnect(ctx)
	}

	return db, cleanup
}

// CleanupCollections removes all documents from test collections for isolation
// between tests. Uses DeleteMany instead of Drop so that indexes are preserved —
// Drop + recreate requires createIndexes to do real round-trips on every test,
// which was causing the services package to exceed the 900s test timeout.
func CleanupCollections(t *testing.T, db *database.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections := []string{
		"content", "content_versions", "templates", "folders", "collections",
		"custom_pages", "settings", "theme_versions", "contact_messages",
		"login_attempts", "assets", "api_keys", "redirects", "snippets",
		"users", "audit_logs", "user_activity",
		"oauth_clients", "oauth_auth_codes", "oauth_access_tokens", "oauth_refresh_tokens",
	}
	empty := bson.M{}
	for _, name := range collections {
		db.Collection(name).DeleteMany(ctx, empty)
	}
}

// ParseJSON decodes a response body into the target struct.
func ParseJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("testutil: failed to parse JSON response: %v", err)
	}
}

// ReadResponseBody reads and returns the full response body as a string.
func ReadResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	return sb.String()
}
