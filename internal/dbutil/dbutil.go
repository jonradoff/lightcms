package dbutil

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/jonradoff/lightcms/v7/config"
)

// GetMongoURI returns the MongoDB URI for the command-line tools: the
// MONGO_URI environment variable (the one the server reads) when it is set,
// otherwise mongo_uri from config.prod.json, then config.dev.json.
func GetMongoURI() string {
	if uri := strings.TrimSpace(os.Getenv("MONGO_URI")); uri != "" {
		return uri
	}
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
// "lightcms". When the URI comes from MONGO_URI no config file is consulted,
// as in the server's environment mode.
func GetDatabaseName() string {
	if strings.TrimSpace(os.Getenv("MONGO_URI")) != "" {
		return config.ResolveDatabaseName("")
	}
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
