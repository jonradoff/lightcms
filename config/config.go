package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	Port          string `json:"port"`
	MongoURI      string `json:"mongo_uri"`
	Env           string `json:"env"` // "development" or "production"
	SessionSecret string `json:"session_secret"`
	BaseURL       string `json:"base_url"`       // Public URL of the site (e.g., "https://example.com")
	SecureCookies bool   `json:"secure_cookies"` // Set to true in production (requires HTTPS)
}

// DefaultDev returns default development configuration
func DefaultDev() *Config {
	return &Config{
		Port:          "8082",
		MongoURI:      "", // Must be set in config file
		Env:           "development",
		SessionSecret: "dev-session-secret-change-in-prod",
		BaseURL:       "http://localhost:8082",
		SecureCookies: false,
	}
}

// DefaultProd returns default production configuration
func DefaultProd() *Config {
	return &Config{
		Port:          "80",
		MongoURI:      "", // Must be set in config file
		Env:           "production",
		SessionSecret: "", // Must be set in config file
		BaseURL:       "", // Must be set in config file
		SecureCookies: true,
	}
}

// Load loads configuration from JSON config file or environment variables
// Priority: environment variables > config.prod.json > config.dev.json
func Load() (*Config, error) {
	// Check if running with environment variables (e.g., Fly.io)
	if mongoURI := os.Getenv("MONGO_URI"); mongoURI != "" {
		return loadFromEnv()
	}

	// Fall back to config files
	var configPath string
	var cfg *Config

	// Check for production config first
	if _, err := os.Stat("config.prod.json"); err == nil {
		configPath = "config.prod.json"
		cfg = DefaultProd()
	} else if _, err := os.Stat("config.dev.json"); err == nil {
		configPath = "config.dev.json"
		cfg = DefaultDev()
	} else {
		return nil, fmt.Errorf("no config file found (expected config.dev.json or config.prod.json)")
	}

	if err := loadFromFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables (for Fly.io deployment)
func loadFromEnv() (*Config, error) {
	cfg := DefaultProd()

	cfg.MongoURI = os.Getenv("MONGO_URI")
	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI environment variable is required")
	}

	cfg.SessionSecret = os.Getenv("SESSION_SECRET")
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable is required")
	}

	cfg.BaseURL = os.Getenv("BASE_URL")
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://lightcms.fly.dev" // Default Fly.io URL
	}

	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}

	if env := os.Getenv("ENV"); env != "" {
		cfg.Env = env
	}

	// SecureCookies defaults to true in production
	if secure := os.Getenv("SECURE_COOKIES"); secure == "false" {
		cfg.SecureCookies = false
	}

	return cfg, nil
}

func loadFromFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, cfg)
}

// Save saves configuration to a file
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsDev returns true if running in development mode
func (c *Config) IsDev() bool {
	return c.Env == "development" || c.Env == "dev"
}

// IsProd returns true if running in production mode
func (c *Config) IsProd() bool {
	return c.Env == "production" || c.Env == "prod"
}
