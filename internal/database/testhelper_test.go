package database

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	sharedTestOnce sync.Once
	sharedTestDB   *DB
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

// lookupEnv is a simple wrapper so tests read env vars consistently.
func lookupEnv(key string) string {
	return os.Getenv(key)
}
