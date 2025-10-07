package ui

import (
	"fmt"
	"os"
)

// TestConfig holds configuration for UI tests
type TestConfig struct {
	ServerURL string `toml:"server_url"`
}

// LoadTestConfig loads test configuration from environment variable
// Fails if TEST_SERVER_URL is not set - no fallback values
func LoadTestConfig() (*TestConfig, error) {
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		return nil, fmt.Errorf("TEST_SERVER_URL environment variable is required for UI tests")
	}

	config := &TestConfig{
		ServerURL: serverURL,
	}

	return config, nil
}
