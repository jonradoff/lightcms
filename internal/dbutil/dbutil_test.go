package dbutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMongoURI_NoConfig(t *testing.T) {
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
