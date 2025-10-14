package test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/server"
)

// Test environment globals
var (
	testApp     *app.App
	testServer  *http.Server
	testConfig  *common.Config
	testLogger  arbor.ILogger
	serverURL   string
	testDataDir string
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	var exitCode int

	// Setup
	if err := setupTestEnvironment(); err != nil {
		fmt.Printf("Failed to set up test environment: %v\n", err)
		exitCode = 1
	} else {
		// Run tests
		exitCode = m.Run()

		// Teardown
		teardownTestEnvironment()
	}

	os.Exit(exitCode)
}

// setupTestEnvironment initializes the test application and server
func setupTestEnvironment() error {
	// Create test data directory
	testDataDir = filepath.Join(".", "testdata")
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create test data directory: %w", err)
	}

	// Load test configuration
	testConfig = common.NewDefaultConfig()
	testConfig.Server.Port = 18085 // Use different port for testing
	testConfig.Server.Host = "127.0.0.1"
	testConfig.Database.Path = filepath.Join(testDataDir, "test_quaero.db")
	testConfig.LLM.Mode = "mock" // Use mock mode for testing

	// Initialize test logger
	testLogger = arbor.NewLogger(
		arbor.WithLevel(arbor.LevelDebug),
		arbor.WithConsole(os.Stdout),
	)

	// Initialize application
	var err error
	testApp, err = app.New(testConfig, testLogger)
	if err != nil {
		return fmt.Errorf("failed to initialize test app: %w", err)
	}

	// Create server
	testServer = server.NewServer(
		testConfig,
		testApp.StatusHandler,
		testApp.SourcesHandler,
		testApp.ChatHandler,
		testApp.DocumentHandler,
		testApp.JobHandler,
		testApp.WebSocketHandler,
		testLogger,
	)

	// Start server in background
	serverURL = fmt.Sprintf("http://%s:%d", testConfig.Server.Host, testConfig.Server.Port)
	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			testLogger.Error().Err(err).Msg("Test server error")
		}
	}()

	// Wait for server to be ready
	if err := waitForServer(serverURL, 10*time.Second); err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	testLogger.Info().
		Str("url", serverURL).
		Msg("Test environment ready")

	return nil
}

// teardownTestEnvironment cleans up the test environment
func teardownTestEnvironment() {
	// Shutdown server
	if testServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := testServer.Shutdown(ctx); err != nil {
			testLogger.Warn().Err(err).Msg("Server shutdown error")
		}
	}

	// Close application resources
	if testApp != nil {
		testApp.Close()
	}

	// Clean up test data directory
	if testDataDir != "" {
		os.RemoveAll(testDataDir)
	}

	testLogger.Info().Msg("Test environment cleaned up")
}

// waitForServer waits for the server to become responsive
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/status")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready within %v", timeout)
}

// Helper functions for tests

// GetTestServerURL returns the base URL of the test server
func GetTestServerURL() string {
	return serverURL
}

// GetTestApp returns the test application instance
func GetTestApp() *app.App {
	return testApp
}

// GetTestLogger returns the test logger
func GetTestLogger() arbor.ILogger {
	return testLogger
}

// MakeRequest makes an HTTP request to the test server
func MakeRequest(method, path string, body []byte) (*http.Response, error) {
	url := serverURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}
