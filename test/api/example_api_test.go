// -----------------------------------------------------------------------
// Example API test using the new self-contained setup
// Demonstrates proper usage of SetupTestEnvironment
// -----------------------------------------------------------------------

package api

import (
	"net/http"
	"testing"
)

// TestExampleListSources demonstrates the new test setup pattern
func TestExampleListSources(t *testing.T) {
	// Setup test environment (builds & starts service automatically)
	env, err := SetupTestEnvironment("TestExampleListSources")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create HTTP helper using the environment's base URL
	h := env.NewHTTPTestHelper(t)

	// Log test action
	env.LogTest(t, "Making GET request to /api/sources")

	// Make API request
	resp, err := h.GET("/api/sources")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	// Assert response status
	h.AssertStatusCode(resp, http.StatusOK)

	env.LogTest(t, "Successfully retrieved sources list")
}

// TestExampleCreateSource demonstrates creating a resource with cleanup
func TestExampleCreateSource(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestExampleCreateSource")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create HTTP helper
	h := env.NewHTTPTestHelper(t)

	env.LogTest(t, "Creating test source")

	// Create a test source
	source := map[string]interface{}{
		"name":        "Example Test Source",
		"type":        "jira",
		"base_url":    "https://example.atlassian.net",
		"auth_domain": "example.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"max_depth":    3,
			"follow_links": true,
			"concurrency":  2,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// API returns 201 Created for POST requests
	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify the source was created
	if name, ok := result["name"].(string); !ok || name != "Example Test Source" {
		t.Errorf("Expected name 'Example Test Source', got: %v", result["name"])
	}

	env.LogTest(t, "Source created successfully: %v", result["id"])

	// Extract source ID for cleanup
	sourceID, ok := result["id"].(string)
	if !ok {
		t.Log("Warning: Could not extract source ID for cleanup")
		return
	}

	// Cleanup: delete the test source
	defer func() {
		env.LogTest(t, "Cleaning up test source: %s", sourceID)
		deleteResp, err := h.DELETE("/api/sources/" + sourceID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test source: %v", err)
			return
		}
		deleteResp.Body.Close()
		env.LogTest(t, "Test source cleaned up successfully")
	}()

	// Verify we can retrieve the created source
	env.LogTest(t, "Verifying source can be retrieved")
	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	h.AssertStatusCode(getResp, http.StatusOK)

	var getResult map[string]interface{}
	if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	if getResult["name"] != "Example Test Source" {
		t.Errorf("Retrieved source name mismatch: %v", getResult["name"])
	}

	env.LogTest(t, "Source verification complete")
}
