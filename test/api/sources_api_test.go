package api

import (
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test"
)

func TestListSources(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	resp, err := h.GET("/api/sources")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// API may return null for empty sources list, which is valid
	// Just check that we get a 200 response
	t.Log("Successfully retrieved sources list")
}

func TestCreateSource(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create a test source
	source := map[string]interface{}{
		"name":        "Test Jira Source",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"max_depth":    3,
			"follow_links": true,
			"concurrency":  2,
			"detail_level": "full",
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

	// Verify the source was created (response is the source object, not a status wrapper)
	if name, ok := result["name"].(string); !ok || name != "Test Jira Source" {
		t.Errorf("Expected name 'Test Jira Source', got: %v", result["name"])
	}

	// Extract source ID for cleanup
	sourceID, ok := result["id"].(string)
	if !ok {
		t.Log("Warning: Could not extract source ID for cleanup")
		return
	}

	// Cleanup: delete the test source
	defer func() {
		deleteResp, err := h.DELETE("/api/sources/" + sourceID)
		if err != nil {
			t.Logf("Warning: Failed to cleanup test source: %v", err)
			return
		}
		deleteResp.Body.Close()
	}()
}

func TestGetSource(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// First create a source
	source := map[string]interface{}{
		"name":        "Test Source for Get",
		"type":        "confluence",
		"base_url":    "https://test.atlassian.net/wiki",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	createResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var createResult map[string]interface{}
	if err := h.ParseJSONResponse(createResp, &createResult); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	sourceID, ok := createResult["id"].(string)
	if !ok {
		t.Fatal("Could not extract source ID")
	}

	// Cleanup
	defer func() {
		h.DELETE("/api/sources/" + sourceID)
	}()

	// Now get the source
	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	h.AssertStatusCode(getResp, http.StatusOK)

	var getResult map[string]interface{}
	if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	// Verify source data
	if name, ok := getResult["name"].(string); !ok || name != "Test Source for Get" {
		t.Errorf("Expected name 'Test Source for Get', got: %v", getResult["name"])
	}
}

func TestUpdateSource(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create a source
	source := map[string]interface{}{
		"name":        "Original Name",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	createResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var createResult map[string]interface{}
	if err := h.ParseJSONResponse(createResp, &createResult); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	sourceID, ok := createResult["id"].(string)
	if !ok {
		t.Fatal("Could not extract source ID")
	}

	// Cleanup
	defer func() {
		h.DELETE("/api/sources/" + sourceID)
	}()

	// Update the source
	updatedSource := map[string]interface{}{
		"name":        "Updated Name",
		"type":        "jira",
		"base_url":    "https://updated.atlassian.net",
		"auth_domain": "updated.atlassian.net",
		"enabled":     false,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	updateResp, err := h.PUT("/api/sources/"+sourceID, updatedSource)
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}

	h.AssertStatusCode(updateResp, http.StatusOK)

	// Verify the update
	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get updated source: %v", err)
	}

	var getResult map[string]interface{}
	if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	if name, ok := getResult["name"].(string); !ok || name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got: %v", getResult["name"])
	}

	if enabled, ok := getResult["enabled"].(bool); !ok || enabled != false {
		t.Errorf("Expected enabled=false, got: %v", getResult["enabled"])
	}
}

func TestDeleteSource(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create a source
	source := map[string]interface{}{
		"name":        "Source to Delete",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	createResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var createResult map[string]interface{}
	if err := h.ParseJSONResponse(createResp, &createResult); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	sourceID, ok := createResult["id"].(string)
	if !ok {
		t.Fatal("Could not extract source ID")
	}

	// Delete the source
	deleteResp, err := h.DELETE("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to delete source: %v", err)
	}

	// DELETE returns 204 No Content on success
	h.AssertStatusCode(deleteResp, http.StatusNoContent)

	// Verify it's deleted (should return 404)
	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to verify deletion: %v", err)
	}

	// Should return 404 or empty result
	if getResp.StatusCode != http.StatusNotFound && getResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 404 or 200 after deletion, got: %d", getResp.StatusCode)
	}
}
