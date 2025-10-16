package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// TestDataSetup creates test authentication and sources for development/testing
type TestDataSetup struct {
	baseURL string
	client  *http.Client
}

func NewTestDataSetup(baseURL string) *TestDataSetup {
	return &TestDataSetup{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
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

	log.Println("✓ Created authentication for bobmcallan.atlassian.net")

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
				log.Printf("  Auth ID: %s", authID)
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
		log.Printf("✓ Created %s source: %s (ID: %s)", sourceType, sourceName, sourceID)
		return sourceID, nil
	}

	return "", fmt.Errorf("could not extract source ID")
}

// SetupTestData creates all test data
func (t *TestDataSetup) SetupTestData() error {
	log.Println("Setting up test data...")
	log.Println("====================================================")

	// Step 1: Create authentication
	authID, err := t.SetupAuthentication()
	if err != nil {
		return fmt.Errorf("failed to setup authentication: %w", err)
	}

	log.Println()
	log.Println("Creating sources with authentication...")

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

	log.Println()
	log.Println("====================================================")
	log.Println("✅ Test data setup complete!")
	log.Println()
	log.Println("Summary:")
	log.Printf("  • Authentication ID: %s", authID)
	log.Printf("  • Jira Source ID: %s", jiraID)
	log.Printf("  • Confluence Source ID: %s", confluenceID)
	log.Println()
	log.Println("You can now:")
	log.Println("  1. Visit http://localhost:8085/sources to see the sources")
	log.Println("  2. Visit http://localhost:8085/auth to see the authentication")
	log.Println("  3. Use the Chrome extension to capture real authentication")
	log.Println()

	return nil
}

// CleanupTestData removes all test data
func (t *TestDataSetup) CleanupTestData() error {
	log.Println("Cleaning up test data...")
	log.Println("====================================================")

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
			log.Printf("  ⚠ Failed to delete source %s: %v", sourceName, err)
		} else {
			delResp.Body.Close()
			log.Printf("  ✓ Deleted source: %s", sourceName)
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
			log.Printf("  ⚠ Failed to delete auth for %s: %v", siteDomain, err)
		} else {
			delResp.Body.Close()
			log.Printf("  ✓ Deleted authentication: %s", siteDomain)
		}
	}

	log.Println()
	log.Println("✅ Cleanup complete!")
	return nil
}

func main() {
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

	setup := NewTestDataSetup(serverURL)

	if cleanup {
		if err := setup.CleanupTestData(); err != nil {
			log.Fatalf("Cleanup failed: %v", err)
		}
	} else {
		// Check if server is running
		resp, err := http.Get(serverURL + "/api/status")
		if err != nil {
			log.Fatalf("❌ Server is not running at %s\n   Please start the server first: cd bin && ./quaero.exe -c quaero.toml", serverURL)
		}
		resp.Body.Close()

		if err := setup.SetupTestData(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
	}
}
