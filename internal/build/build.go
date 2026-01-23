package build

import (
	"encoding/json"
	"os"
)

// Config holds build-time configuration
type Config struct {
	Version string `json:"version"`
}

var config *Config

// Load loads the build configuration from build.json
func Load() (*Config, error) {
	if config != nil {
		return config, nil
	}

	data, err := os.ReadFile("build.json")
	if err != nil {
		// Default if file not found
		config = &Config{Version: "1.0"}
		return config, nil
	}

	config = &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetVersion returns the current software version
func GetVersion() string {
	if config == nil {
		Load()
	}
	return config.Version
}
