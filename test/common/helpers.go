// -----------------------------------------------------------------------
// Test helpers for both API and UI tests
// Shared across test/api and test/ui packages
// -----------------------------------------------------------------------

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/internal/common"
)

// =============================================================================
// Test Server URL Helpers
// =============================================================================

// GetTestServerURL returns the test server URL from environment variable or bin/quaero.toml
func GetTestServerURL() (string, error) {
	// Check if running in mock mode
	if IsMockMode() {
		return "http://localhost:9999", nil
	}

	// Check environment variable first (highest priority)
	if url := os.Getenv("TEST_SERVER_URL"); url != "" {
		return url, nil
	}

	// Read from bin/quaero.toml
	configPath := filepath.Join("..", "bin", "quaero.toml")

	// Try to read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config file doesn't exist, use default
		return "http://localhost:8085", nil
	}

	var config common.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		// If config is invalid, use default
		return "http://localhost:8085", nil
	}

	// Construct URL from config
	host := config.Server.Host
	if host == "" {
		host = "localhost"
	}
	port := config.Server.Port
	if port == 0 {
		port = 8085
	}

	return fmt.Sprintf("http://%s:%d", host, port), nil
}

// MustGetTestServerURL returns the test server URL or panics on error
func MustGetTestServerURL() string {
	url, err := GetTestServerURL()
	if err != nil {
		panic(fmt.Sprintf("Failed to get test server URL: %v", err))
	}
	return url
}

// GetExpectedPort returns the expected port from config or default
func GetExpectedPort() int {
	// Check environment variable first
	if url := os.Getenv("TEST_SERVER_URL"); url != "" {
		// Extract port from URL
		parts := strings.Split(url, ":")
		if len(parts) >= 3 {
			portStr := parts[2]
			if port, err := strconv.Atoi(portStr); err == nil {
				return port
			}
		}
	}

	// Read from bin/quaero.toml
	configPath := filepath.Join("..", "bin", "quaero.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 8085 // Default
	}

	var config common.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return 8085 // Default
	}

	if config.Server.Port == 0 {
		return 8085
	}

	return config.Server.Port
}

// GetTestMode returns the test mode from environment variable
// Returns "mock" or "integration" (default: "integration")
// - mock: Uses in-memory mock server, no real database, fast, isolated
// - integration: Uses real Quaero service, tests full stack, requires service running
func GetTestMode() string {
	mode := os.Getenv("TEST_MODE")
	if mode == "" {
		return "integration" // Default for backward compatibility
	}
	return mode
}

// IsMockMode returns true if tests should run in mock mode
func IsMockMode() bool {
	return GetTestMode() == "mock"
}

// =============================================================================
// File Assertion Helpers
// =============================================================================

// AssertFileExistsAndNotEmpty asserts that a file exists and has content
func AssertFileExistsAndNotEmpty(t *testing.T, path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return false
	}

	if info.Size() == 0 {
		t.Errorf("File is empty: %s", path)
		return false
	}

	t.Logf("PASS: File exists and is not empty: %s (%d bytes)", path, info.Size())
	return true
}

// AssertFileExists asserts that a file exists (can be empty)
func AssertFileExists(t *testing.T, path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return false
	}
	return true
}

// RequireFileExistsAndNotEmpty requires that a file exists and has content, failing immediately if not
func RequireFileExistsAndNotEmpty(t *testing.T, path string) {
	info, err := os.Stat(path)
	require.NoError(t, err, "File must exist: %s", path)
	require.Greater(t, info.Size(), int64(0), "File must not be empty: %s", path)
	t.Logf("PASS: File exists and is not empty: %s (%d bytes)", path, info.Size())
}

// =============================================================================
// Retry Helpers
// =============================================================================

// Retry retries a function until it succeeds or max attempts reached
func Retry(fn func() error, maxAttempts int, delay int) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("retry failed after %d attempts: %w", maxAttempts, lastErr)
}
