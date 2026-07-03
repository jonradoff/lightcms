package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDev(t *testing.T) {
	cfg := DefaultDev()

	if cfg.Port != "8082" {
		t.Errorf("expected port 8082, got %s", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("expected env development, got %s", cfg.Env)
	}
	if cfg.SecureCookies {
		t.Error("expected SecureCookies false in dev")
	}
	if cfg.BaseURL != "http://localhost:8082" {
		t.Errorf("expected default dev BaseURL, got %s", cfg.BaseURL)
	}
	if cfg.SessionSecret == "" {
		t.Error("expected non-empty dev session secret")
	}
}

func TestDefaultProd(t *testing.T) {
	cfg := DefaultProd()

	if cfg.Port != "80" {
		t.Errorf("expected port 80, got %s", cfg.Port)
	}
	if cfg.Env != "production" {
		t.Errorf("expected env production, got %s", cfg.Env)
	}
	if !cfg.SecureCookies {
		t.Error("expected SecureCookies true in prod")
	}
	if cfg.MongoURI != "" {
		t.Error("expected empty MongoURI in prod defaults (must be configured)")
	}
}

func TestIsDev(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"development", true},
		{"dev", true},
		{"production", false},
		{"prod", false},
		{"staging", false},
	}
	for _, tt := range tests {
		cfg := &Config{Env: tt.env}
		if cfg.IsDev() != tt.expected {
			t.Errorf("IsDev() for env %q: expected %v", tt.env, tt.expected)
		}
	}
}

func TestIsProd(t *testing.T) {
	tests := []struct {
		env      string
		expected bool
	}{
		{"production", true},
		{"prod", true},
		{"development", false},
		{"dev", false},
		{"staging", false},
	}
	for _, tt := range tests {
		cfg := &Config{Env: tt.env}
		if cfg.IsProd() != tt.expected {
			t.Errorf("IsProd() for env %q: expected %v", tt.env, tt.expected)
		}
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Set required env vars
	os.Setenv("MONGO_URI", "mongodb://localhost:27017/test")
	os.Setenv("SESSION_SECRET", "test-secret-12345678")
	os.Setenv("BASE_URL", "https://test.example.com")
	os.Setenv("PORT", "9090")
	os.Setenv("ENV", "staging")
	os.Setenv("SECURE_COOKIES", "false")
	defer func() {
		os.Unsetenv("MONGO_URI")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("PORT")
		os.Unsetenv("ENV")
		os.Unsetenv("SECURE_COOKIES")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.MongoURI != "mongodb://localhost:27017/test" {
		t.Errorf("expected MongoURI from env, got %s", cfg.MongoURI)
	}
	if cfg.SessionSecret != "test-secret-12345678" {
		t.Errorf("expected SessionSecret from env, got %s", cfg.SessionSecret)
	}
	if cfg.BaseURL != "https://test.example.com" {
		t.Errorf("expected BaseURL from env, got %s", cfg.BaseURL)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected Port from env, got %s", cfg.Port)
	}
	if cfg.Env != "staging" {
		t.Errorf("expected Env from env, got %s", cfg.Env)
	}
	if cfg.SecureCookies {
		t.Error("expected SecureCookies false from env")
	}
}

func TestLoadFromEnv_MissingSessionSecret(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://localhost:27017/test")
	os.Unsetenv("SESSION_SECRET")
	defer os.Unsetenv("MONGO_URI")

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing SESSION_SECRET")
	}
}

func TestLoadFromEnv_DefaultBaseURL(t *testing.T) {
	os.Setenv("MONGO_URI", "mongodb://localhost:27017/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Unsetenv("BASE_URL")
	defer func() {
		os.Unsetenv("MONGO_URI")
		os.Unsetenv("SESSION_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.BaseURL == "" {
		t.Error("expected non-empty default BaseURL")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Clear env vars that would trigger env-based loading
	os.Unsetenv("MONGO_URI")

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.dev.json")
	os.WriteFile(configFile, []byte(`{
		"port": "7070",
		"mongo_uri": "mongodb://localhost:27017/filetest",
		"env": "development",
		"session_secret": "file-secret",
		"base_url": "http://localhost:7070",
		"secure_cookies": false
	}`), 0644)

	os.Setenv("LIGHTCMS_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("LIGHTCMS_CONFIG_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Port != "7070" {
		t.Errorf("expected port from file, got %s", cfg.Port)
	}
	if cfg.MongoURI != "mongodb://localhost:27017/filetest" {
		t.Errorf("expected MongoURI from file, got %s", cfg.MongoURI)
	}
}

func TestLoadFromFile_ProdPriority(t *testing.T) {
	os.Unsetenv("MONGO_URI")

	tmpDir := t.TempDir()
	// Create both dev and prod configs
	os.WriteFile(filepath.Join(tmpDir, "config.dev.json"), []byte(`{"port": "8082", "env": "development"}`), 0644)
	os.WriteFile(filepath.Join(tmpDir, "config.prod.json"), []byte(`{"port": "80", "env": "production"}`), 0644)

	os.Setenv("LIGHTCMS_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("LIGHTCMS_CONFIG_DIR")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Prod config should take priority
	if cfg.Env != "production" {
		t.Errorf("expected production (prod priority), got %s", cfg.Env)
	}
}

func TestLoadFromFile_NoConfigFound(t *testing.T) {
	os.Unsetenv("MONGO_URI")

	tmpDir := t.TempDir()
	os.Setenv("LIGHTCMS_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("LIGHTCMS_CONFIG_DIR")

	_, err := Load()
	if err == nil {
		t.Error("expected error when no config files found")
	}
}

func TestLoadFromEnv_SecureCookies_Default(t *testing.T) {
	// Not setting SECURE_COOKIES → should stay true (prod default)
	os.Setenv("MONGO_URI", "mongodb://localhost/test")
	os.Setenv("SESSION_SECRET", "secret-long-enough")
	os.Unsetenv("SECURE_COOKIES")
	defer func() {
		os.Unsetenv("MONGO_URI")
		os.Unsetenv("SESSION_SECRET")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.SecureCookies {
		t.Error("expected SecureCookies=true when SECURE_COOKIES env not set")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	os.Unsetenv("MONGO_URI")

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "config.dev.json"), []byte(`not valid json`), 0644)
	os.Setenv("LIGHTCMS_CONFIG_DIR", tmpDir)
	defer os.Unsetenv("LIGHTCMS_CONFIG_DIR")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "config.json")

	cfg := &Config{
		Port:          "9999",
		MongoURI:      "mongodb://test",
		Env:           "test",
		SessionSecret: "secret",
		BaseURL:       "http://test",
		SecureCookies: true,
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	// Verify content can be loaded back
	os.Unsetenv("MONGO_URI")
	os.Setenv("LIGHTCMS_CONFIG_DIR", filepath.Dir(path))
	defer os.Unsetenv("LIGHTCMS_CONFIG_DIR")

	// Rename to something Load() looks for
	prodPath := filepath.Join(filepath.Dir(path), "config.prod.json")
	os.Rename(path, prodPath)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if loaded.Port != "9999" {
		t.Errorf("expected saved port, got %s", loaded.Port)
	}
}
