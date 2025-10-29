package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/arbor/models"
)

// TestDataSetup creates test authentication and sources for development/testing
type TestDataSetup struct {
	baseURL string
	client  *http.Client
	logger  arbor.ILogger
}

func NewTestDataSetup(baseURL string, logger arbor.ILogger) *TestDataSetup {
	return &TestDataSetup{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// SetupAuthentication creates test authentication for Atlassian
func (t *TestDataSetup) SetupAuthentication() (string, error) {
	// Create authentication for bobmcallan.atlassian.net
	authData := map[string]interface{}{
		"baseUrl":   "https://bobmcallan.atlassian.net",
		"userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-session-token-" + fmt.Sprintf("%d", time.Now().Unix()),
				"domain":   ".atlassian.net",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			},
		},
		"tokens": map[string]string{
			"cloudId":  "test-cloud-id-" + fmt.Sprintf("%d", time.Now().Unix()),
			"atlToken": "test-atl-token-" + fmt.Sprintf("%d", time.Now().Unix()),
		},
	}

	authJSON, err := json.Marshal(authData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth data: %w", err)
	}

	resp, err := http.Post(t.baseURL+"/api/auth", "application/json", bytes.NewBuffer(authJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create auth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	t.logger.Info().Msg("✓ Created authentication for bobmcallan.atlassian.net")

	// Get the auth ID
	listResp, err := http.Get(t.baseURL + "/api/auth/list")
	if err != nil {
		return "", fmt.Errorf("failed to list auths: %w", err)
	}
	defer listResp.Body.Close()

	var auths []map[string]interface{}
	if err := json.NewDecoder(listResp.Body).Decode(&auths); err != nil {
		return "", fmt.Errorf("failed to decode auth list: %w", err)
	}

	for _, auth := range auths {
		if siteDomain, ok := auth["site_domain"].(string); ok && siteDomain == "bobmcallan.atlassian.net" {
			if authID, ok := auth["id"].(string); ok {
				t.logger.Info().Str("auth_id", authID).Msg("  Auth ID")
				return authID, nil
			}
		}
	}

	return "", fmt.Errorf("could not find created authentication")
}

// CreateSource creates a source with the given configuration
func (t *TestDataSetup) CreateSource(source map[string]interface{}) (string, error) {
	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return "", fmt.Errorf("failed to marshal source: %w", err)
	}

	resp, err := http.Post(t.baseURL+"/api/sources", "application/json", bytes.NewBuffer(sourceJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create source: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("source creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if sourceID, ok := result["id"].(string); ok {
		sourceName := result["name"].(string)
		sourceType := result["type"].(string)
		t.logger.Info().
			Str("source_type", sourceType).
			Str("source_name", sourceName).
			Str("source_id", sourceID).
			Msg("✓ Created source")
		return sourceID, nil
	}

	return "", fmt.Errorf("could not extract source ID")
}

// SetupTestData creates all test data
func (t *TestDataSetup) SetupTestData() error {
	t.logger.Info().Msg("Setting up test data...")
	t.logger.Info().Msg("====================================================")

	// Step 1: Create authentication
	authID, err := t.SetupAuthentication()
	if err != nil {
		return fmt.Errorf("failed to setup authentication: %w", err)
	}

	t.logger.Info().Msg("")
	t.logger.Info().Msg("Creating sources with authentication...")

	// Step 2: Create Jira source
	jiraSource := map[string]interface{}{
		"name":     "Bob's Jira Instance",
		"type":     "jira",
		"base_url": "https://bobmcallan.atlassian.net/jira",
		"auth_id":  authID,
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    3,
			"follow_links": true,
			"concurrency":  2,
			"detail_level": "full",
		},
	}

	jiraID, err := t.CreateSource(jiraSource)
	if err != nil {
		return fmt.Errorf("failed to create Jira source: %w", err)
	}

	// Step 3: Create Confluence source
	confluenceSource := map[string]interface{}{
		"name":     "Bob's Confluence Wiki",
		"type":     "confluence",
		"base_url": "https://bobmcallan.atlassian.net/wiki/home",
		"auth_id":  authID,
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    2,
			"follow_links": false,
			"concurrency":  1,
			"detail_level": "basic",
		},
	}

	confluenceID, err := t.CreateSource(confluenceSource)
	if err != nil {
		return fmt.Errorf("failed to create Confluence source: %w", err)
	}

	t.logger.Info().Msg("")
	t.logger.Info().Msg("====================================================")
	t.logger.Info().Msg("✅ Test data setup complete!")
	t.logger.Info().Msg("")
	t.logger.Info().Msg("Summary:")
	t.logger.Info().Str("auth_id", authID).Msg("  • Authentication ID")
	t.logger.Info().Str("jira_id", jiraID).Msg("  • Jira Source ID")
	t.logger.Info().Str("confluence_id", confluenceID).Msg("  • Confluence Source ID")
	t.logger.Info().Msg("")
	t.logger.Info().Msg("You can now:")
	t.logger.Info().Msg("  1. Visit http://localhost:8085/sources to see the sources")
	t.logger.Info().Msg("  2. Visit http://localhost:8085/auth to see the authentication")
	t.logger.Info().Msg("  3. Use the Chrome extension to capture real authentication")
	t.logger.Info().Msg("")

	return nil
}

// CleanupTestData removes all test data
func (t *TestDataSetup) CleanupTestData() error {
	t.logger.Info().Msg("Cleaning up test data...")
	t.logger.Info().Msg("====================================================")

	// Get all sources
	resp, err := http.Get(t.baseURL + "/api/sources")
	if err != nil {
		return fmt.Errorf("failed to list sources: %w", err)
	}
	defer resp.Body.Close()

	var sources []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return fmt.Errorf("failed to decode sources: %w", err)
	}

	// Delete all sources
	for _, source := range sources {
		sourceID := source["id"].(string)
		sourceName := source["name"].(string)

		req, _ := http.NewRequest("DELETE", t.baseURL+"/api/sources/"+sourceID, nil)
		delResp, err := t.client.Do(req)
		if err != nil {
			t.logger.Warn().Err(err).Str("source_name", sourceName).Msg("  ⚠ Failed to delete source")
		} else {
			delResp.Body.Close()
			t.logger.Info().Str("source_name", sourceName).Msg("  ✓ Deleted source")
		}
	}

	// Get all authentications
	authResp, err := http.Get(t.baseURL + "/api/auth/list")
	if err != nil {
		return fmt.Errorf("failed to list auths: %w", err)
	}
	defer authResp.Body.Close()

	var auths []map[string]interface{}
	if err := json.NewDecoder(authResp.Body).Decode(&auths); err != nil {
		return fmt.Errorf("failed to decode auths: %w", err)
	}

	// Delete all authentications
	for _, auth := range auths {
		authID := auth["id"].(string)
		siteDomain := auth["site_domain"].(string)

		req, _ := http.NewRequest("DELETE", t.baseURL+"/api/auth/"+authID, nil)
		delResp, err := t.client.Do(req)
		if err != nil {
			t.logger.Warn().Err(err).Str("site_domain", siteDomain).Msg("  ⚠ Failed to delete auth")
		} else {
			delResp.Body.Close()
			t.logger.Info().Str("site_domain", siteDomain).Msg("  ✓ Deleted authentication")
		}
	}

	t.logger.Info().Msg("")
	t.logger.Info().Msg("✅ Cleanup complete!")
	return nil
}

func main() {
	// Initialize Arbor logger for console output
	logger := arbor.NewLogger().WithConsoleWriter(models.WriterConfiguration{
		Type:             models.LogWriterTypeConsole,
		TimeFormat:       "15:04:05",
		TextOutput:       true,
		DisableTimestamp: false,
	})

	// Get server URL from environment or use default
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://localhost:8085"
	}

	// Check if cleanup flag is set
	cleanup := false
	for _, arg := range os.Args[1:] {
		if arg == "--cleanup" || arg == "-c" {
			cleanup = true
			break
		}
	}

	setup := NewTestDataSetup(serverURL, logger)

	if cleanup {
		if err := setup.CleanupTestData(); err != nil {
			logger.Fatal().Err(err).Msg("Cleanup failed")
		}
	} else {
		// Check if server is running
		resp, err := http.Get(serverURL + "/api/status")
		if err != nil {
			logger.Fatal().
				Str("server_url", serverURL).
				Msg("❌ Server is not running - Please start the server first: cd bin && ./quaero.exe -c quaero.toml")
		}
		resp.Body.Close()

		if err := setup.SetupTestData(); err != nil {
			logger.Fatal().Err(err).Msg("Setup failed")
		}
	}
}
