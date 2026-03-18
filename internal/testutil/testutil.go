package testutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lightcms/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

const testDBName = "lightcms-test"

// sharedConn holds a single MongoDB connection shared across all tests in a
// package. Creating a new Atlas connection + running createIndexes (~30 round-
// trips) per test function was causing the services package (215 tests) to
// exceed the 900s CI timeout. With a shared connection, createIndexes runs once
// per package binary invocation and CleanupCollections (DeleteMany) is fast.
var (
	sharedOnce sync.Once
	sharedDB   *database.DB
	sharedErr  error
)

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

// MustConnectTestDB returns a shared test database connection. The connection is
// created once per package binary using sync.Once; subsequent calls reuse it.
// Each call wipes all test collections via DeleteMany for isolation.
// The returned cleanup function wipes collections again at test teardown.
func MustConnectTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	// Initialise the shared connection exactly once per test binary.
	sharedOnce.Do(func() {
		loadEnvTest()

		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			return // sharedDB stays nil; caller will Skip
		}

		dbName := os.Getenv("DATABASE_NAME")
		if dbName == "" {
			dbName = testDBName
		}

		if !strings.Contains(strings.ToLower(dbName), "test") {
			sharedErr = fmt.Errorf("REFUSING to run tests — database name %q does not contain 'test'. "+
				"This safety guard prevents accidental use of production databases.", dbName)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		wc := writeconcern.New(writeconcern.WMajority())
		wcOpts := options.Client().SetWriteConcern(wc)
		db, err := database.Connect(ctx, uri, dbName, wcOpts)
		if err != nil {
			sharedErr = fmt.Errorf("failed to connect to test database: %w", err)
			return
		}
		sharedDB = db
	})

	if sharedErr != nil {
		t.Fatalf("testutil: %v", sharedErr)
	}
	if sharedDB == nil {
		t.Skip("skipping: MONGODB_URI not set (no .env.test file)")
	}

	// Wipe all collections before the test runs.
	CleanupCollections(t, sharedDB)

	// Return a no-op cleanup — the next test's MustConnectTestDB call will
	// wipe collections before it runs. Cleaning at both start and end doubled
	// the number of Atlas round-trips and caused the services package (118+
	// tests × 20 collections × 2) to exceed the CI timeout.
	cleanup := func() {}

	return sharedDB, cleanup
}

// CleanupCollections removes all documents from test collections for isolation
// between tests. Uses DeleteMany instead of Drop so that indexes are preserved —
// Drop + recreate requires createIndexes to do real round-trips on every test.
func CleanupCollections(t *testing.T, db *database.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
