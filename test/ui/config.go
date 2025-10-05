package ui

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// TestConfig holds configuration for UI tests
type TestConfig struct {
	ServerURL string `toml:"server_url"`
}

// LoadTestConfig loads test configuration from TOML file or environment
func LoadTestConfig() (*TestConfig, error) {
	config := &TestConfig{
		ServerURL: "http://localhost:8080", // Default
	}

	// Try to load from file
	configPath := os.Getenv("TEST_CONFIG_PATH")
	if configPath == "" {
		configPath = "test_config.toml"
	}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := toml.Unmarshal(data, config); err != nil {
			return nil, err
		}
	}

	// Environment variable overrides file config
	if serverURL := os.Getenv("TEST_SERVER_URL"); serverURL != "" {
		config.ServerURL = serverURL
	}

	return config, nil
}
