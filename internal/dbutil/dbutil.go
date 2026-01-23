package dbutil

import (
	"encoding/json"
	"os"
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
