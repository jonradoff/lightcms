package dbutil

import (
	"encoding/json"
	"os"

	"github.com/jonradoff/lightcms/v7/config"
)

// GetMongoURI returns the MongoDB URI from config file
// Checks config.prod.json first (production), then config.dev.json (development)
func GetMongoURI() string {
	// Check production config first, then development
	configPaths := []string{
		"config.prod.json",
		"config.dev.json",
	}

	for _, path := range configPaths {
		if uri := loadURIFromConfig(path); uri != "" {
			return uri
		}
	}

	return ""
}

// GetDatabaseName returns the MongoDB database name for the command-line
// tools that read the config file directly. Same priority as the server:
// DATABASE_NAME env > database_name in the config file GetMongoURI picks >
// "lightcms".
func GetDatabaseName() string {
	for _, path := range []string{"config.prod.json", "config.dev.json"} {
		if loadURIFromConfig(path) == "" {
			continue
		}
		return config.ResolveDatabaseName(loadDatabaseNameFromConfig(path))
	}
	return config.ResolveDatabaseName("")
}

func loadDatabaseNameFromConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var cfg struct {
		DatabaseName string `json:"database_name"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.DatabaseName
}

func loadURIFromConfig(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var cfg struct {
		MongoURI string `json:"mongo_uri"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.MongoURI
}
