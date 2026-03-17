package build

import (
	"os"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	// Reset cache so this test starts fresh
	config = nil

	// Run from a temp dir without a build.json
	orig, _ := os.Getwd()
	tmp := t.TempDir()
	os.Chdir(tmp)
	defer os.Chdir(orig)
	defer func() { config = nil }()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	// Falls back to default version
	if cfg.Version == "" {
		t.Error("expected non-empty version from fallback")
	}
}

func TestLoad_WithFile(t *testing.T) {
	config = nil
	defer func() { config = nil }()

	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile("build.json", []byte(`{"version":"9.9.9"}`), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Version != "9.9.9" {
		t.Errorf("expected 9.9.9, got %q", cfg.Version)
	}
}

func TestLoad_Cached(t *testing.T) {
	config = nil
	defer func() { config = nil }()

	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile("build.json", []byte(`{"version":"1.2.3"}`), 0644)

	first, _ := Load()

	// Overwrite the file — second Load should return cached value
	os.WriteFile("build.json", []byte(`{"version":"9.9.9"}`), 0644)

	second, _ := Load()
	if first != second {
		t.Error("expected same pointer on second Load (cache)")
	}
	if second.Version != "1.2.3" {
		t.Errorf("expected cached version 1.2.3, got %q", second.Version)
	}
}

func TestGetVersion(t *testing.T) {
	config = nil
	defer func() { config = nil }()

	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile("build.json", []byte(`{"version":"2.0.0"}`), 0644)

	v := GetVersion()
	if v != "2.0.0" {
		t.Errorf("expected 2.0.0, got %q", v)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	config = nil
	defer func() { config = nil }()

	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	os.WriteFile("build.json", []byte(`not json`), 0644)

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
