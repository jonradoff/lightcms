package dbutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMongoURI_NoConfig(t *testing.T) {
	t.Setenv("MONGO_URI", "")
	// Run from a temp dir with no config files
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	uri := GetMongoURI()
	if uri != "" {
		t.Errorf("expected empty URI with no config, got %q", uri)
	}
}

func TestGetMongoURI_DevConfig(t *testing.T) {
	t.Setenv("MONGO_URI", "")
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile(filepath.Join(tmp, "config.dev.json"), []byte(`{"mongo_uri":"mongodb://dev:27017"}`), 0644)

	uri := GetMongoURI()
	if uri != "mongodb://dev:27017" {
		t.Errorf("expected dev URI, got %q", uri)
	}
}

func TestGetMongoURI_ProdTakesPriority(t *testing.T) {
	t.Setenv("MONGO_URI", "")
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile(filepath.Join(tmp, "config.prod.json"), []byte(`{"mongo_uri":"mongodb://prod:27017"}`), 0644)
	os.WriteFile(filepath.Join(tmp, "config.dev.json"), []byte(`{"mongo_uri":"mongodb://dev:27017"}`), 0644)

	uri := GetMongoURI()
	if uri != "mongodb://prod:27017" {
		t.Errorf("expected prod URI to take priority, got %q", uri)
	}
}

func TestLoadURIFromConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	os.WriteFile(path, []byte(`not json`), 0644)

	uri := loadURIFromConfig(path)
	if uri != "" {
		t.Errorf("expected empty URI for invalid JSON, got %q", uri)
	}
}

func TestLoadURIFromConfig_MissingFile(t *testing.T) {
	uri := loadURIFromConfig("/nonexistent/path.json")
	if uri != "" {
		t.Errorf("expected empty URI for missing file, got %q", uri)
	}
}

func TestLoadURIFromConfig_EmptyURI(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "empty.json")
	os.WriteFile(path, []byte(`{"mongo_uri":""}`), 0644)

	uri := loadURIFromConfig(path)
	if uri != "" {
		t.Errorf("expected empty URI, got %q", uri)
	}
}

func TestGetDatabaseName(t *testing.T) {
	t.Setenv("MONGO_URI", "")
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	// No config, no env → default
	t.Setenv("DATABASE_NAME", "")
	if got := GetDatabaseName(); got != "lightcms" {
		t.Errorf("expected default lightcms, got %q", got)
	}

	// Name comes from the same config file GetMongoURI picks (prod first)
	os.WriteFile(filepath.Join(tmp, "config.dev.json"), []byte(`{"mongo_uri":"mongodb://dev:27017","database_name":"devdb"}`), 0644)
	if got := GetDatabaseName(); got != "devdb" {
		t.Errorf("expected devdb, got %q", got)
	}
	os.WriteFile(filepath.Join(tmp, "config.prod.json"), []byte(`{"mongo_uri":"mongodb://prod:27017"}`), 0644)
	if got := GetDatabaseName(); got != "lightcms" {
		t.Errorf("expected lightcms for prod config without database_name, got %q", got)
	}

	// Env overrides the file
	t.Setenv("DATABASE_NAME", "lightcms_test")
	if got := GetDatabaseName(); got != "lightcms_test" {
		t.Errorf("expected env override, got %q", got)
	}
}

// MONGO_URI is the variable the server reads and the one the tools' error
// message names: it wins over both config files, and the database name then
// comes from DATABASE_NAME alone (no config file is consulted).
func TestGetMongoURI_EnvWins(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)
	os.WriteFile(filepath.Join(tmp, "config.prod.json"), []byte(`{"mongo_uri":"mongodb://prod:27017","database_name":"proddb"}`), 0644)
	os.WriteFile(filepath.Join(tmp, "config.dev.json"), []byte(`{"mongo_uri":"mongodb://dev:27017","database_name":"devdb"}`), 0644)

	t.Setenv("MONGO_URI", "mongodb://env:27017")
	t.Setenv("DATABASE_NAME", "")
	if got := GetMongoURI(); got != "mongodb://env:27017" {
		t.Errorf("GetMongoURI = %q, want the MONGO_URI value", got)
	}
	if got := GetDatabaseName(); got != "lightcms" {
		t.Errorf("GetDatabaseName = %q, want the default (config files are not read in env mode)", got)
	}
	t.Setenv("DATABASE_NAME", "lightcms-test")
	if got := GetDatabaseName(); got != "lightcms-test" {
		t.Errorf("GetDatabaseName = %q, want lightcms-test", got)
	}

	// No config files at all: the env var alone is enough
	os.Remove(filepath.Join(tmp, "config.prod.json"))
	os.Remove(filepath.Join(tmp, "config.dev.json"))
	if got := GetMongoURI(); got != "mongodb://env:27017" {
		t.Errorf("GetMongoURI without config files = %q", got)
	}

	// Blank is not set
	t.Setenv("MONGO_URI", "  ")
	if got := GetMongoURI(); got != "" {
		t.Errorf("GetMongoURI with blank MONGO_URI = %q, want empty", got)
	}
}
