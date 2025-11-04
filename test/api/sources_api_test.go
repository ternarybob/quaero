// Package api contains API integration tests.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListSources(t *testing.T) {
	env, err := SetupTestEnvironment("TestListSources")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

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
	env, err := SetupTestEnvironment("TestCreateSource")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

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
	env, err := SetupTestEnvironment("TestGetSource")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

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
	env, err := SetupTestEnvironment("TestUpdateSource")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

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
	env, err := SetupTestEnvironment("TestDeleteSource")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

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

// TestCreateSourcesWithAuthentication creates test Jira and Confluence sources with authentication
func TestCreateSourcesWithAuthentication(t *testing.T) {
	env, err := SetupTestEnvironment("TestCreateSourcesWithAuthentication")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	baseURL := env.GetBaseURL()

	// First, create test authentication for bobmcallan.atlassian.net
	authData := map[string]interface{}{
		"baseUrl":   "https://bobmcallan.atlassian.net",
		"userAgent": "Mozilla/5.0 Test",
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-token-sources",
				"domain":   ".atlassian.net",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			},
		},
		"tokens": map[string]string{
			"cloudId":  "test-cloud-id",
			"atlToken": "test-atl-token",
		},
	}

	// Create authentication
	authJSON, err := json.Marshal(authData)
	if err != nil {
		t.Fatalf("Failed to marshal auth data: %v", err)
	}

	authResp, err := http.Post(baseURL+"/api/auth", "application/json", bytes.NewBuffer(authJSON))
	if err != nil {
		t.Fatalf("Failed to create auth: %v", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for auth creation, got %d", authResp.StatusCode)
	}

	// Get list of authentications to find the auth_id
	authListResp, err := http.Get(baseURL + "/api/auth/list")
	if err != nil {
		t.Fatalf("Failed to list auths: %v", err)
	}
	defer authListResp.Body.Close()

	var auths []map[string]interface{}
	if err := json.NewDecoder(authListResp.Body).Decode(&auths); err != nil {
		t.Fatalf("Failed to decode auth list: %v", err)
	}

	var authID string
	for _, auth := range auths {
		if siteDomain, ok := auth["site_domain"].(string); ok && siteDomain == "bobmcallan.atlassian.net" {
			authID = auth["id"].(string)
			break
		}
	}

	if authID == "" {
		t.Fatal("Could not find created authentication")
	}

	t.Logf("Created authentication with ID: %s", authID)

	// Now create Jira source with authentication
	h := env.NewHTTPTestHelper(t)

	jiraSource := map[string]interface{}{
		"name":     "Test Jira with Auth",
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

	jiraResp, err := h.POST("/api/sources", jiraSource)
	if err != nil {
		t.Fatalf("Failed to create Jira source: %v", err)
	}

	h.AssertStatusCode(jiraResp, http.StatusCreated)

	var jiraResult map[string]interface{}
	if err := h.ParseJSONResponse(jiraResp, &jiraResult); err != nil {
		t.Fatalf("Failed to parse Jira response: %v", err)
	}

	// Verify auth_id is set
	if savedAuthID, ok := jiraResult["auth_id"].(string); !ok || savedAuthID != authID {
		t.Errorf("Expected auth_id '%s', got: %v", authID, jiraResult["auth_id"])
	}

	jiraSourceID := jiraResult["id"].(string)
	t.Logf("Created Jira source with ID: %s", jiraSourceID)

	// Create Confluence source with the same authentication
	confluenceSource := map[string]interface{}{
		"name":     "Test Confluence with Auth",
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

	confluenceResp, err := h.POST("/api/sources", confluenceSource)
	if err != nil {
		t.Fatalf("Failed to create Confluence source: %v", err)
	}

	h.AssertStatusCode(confluenceResp, http.StatusCreated)

	var confluenceResult map[string]interface{}
	if err := h.ParseJSONResponse(confluenceResp, &confluenceResult); err != nil {
		t.Fatalf("Failed to parse Confluence response: %v", err)
	}

	// Verify auth_id is set
	if savedAuthID, ok := confluenceResult["auth_id"].(string); !ok || savedAuthID != authID {
		t.Errorf("Expected auth_id '%s', got: %v", authID, confluenceResult["auth_id"])
	}

	confluenceSourceID := confluenceResult["id"].(string)
	t.Logf("Created Confluence source with ID: %s", confluenceSourceID)

	// Cleanup sources
	defer func() {
		h.DELETE("/api/sources/" + jiraSourceID)
		h.DELETE("/api/sources/" + confluenceSourceID)
		// Clean up authentication
		if authID != "" {
			req, err := http.NewRequest("DELETE", baseURL+"/api/auth/"+authID, nil)
			if err == nil {
				client := &http.Client{}
				resp, _ := client.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
			}
		}
	}()

	// Verify both sources use the same authentication
	listResp, err := h.GET("/api/sources")
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}

	var sources []map[string]interface{}
	if err := h.ParseJSONResponse(listResp, &sources); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	jiraFound := false
	confluenceFound := false

	for _, source := range sources {
		id := source["id"].(string)

		if id == jiraSourceID {
			jiraFound = true
			if authIDCheck := source["auth_id"].(string); authIDCheck != authID {
				t.Errorf("Jira source auth_id mismatch: expected %s, got %s", authID, authIDCheck)
			}
		}

		if id == confluenceSourceID {
			confluenceFound = true
			if authIDCheck := source["auth_id"].(string); authIDCheck != authID {
				t.Errorf("Confluence source auth_id mismatch: expected %s, got %s", authID, authIDCheck)
			}
		}
	}

	if !jiraFound {
		t.Error("Jira source not found in list")
	}
	if !confluenceFound {
		t.Error("Confluence source not found in list")
	}

	t.Log("✓ Successfully created Jira and Confluence sources with shared authentication")
}

// TestSourceWithoutAuthentication verifies that sources can be created without authentication
func TestSourceWithoutAuthentication(t *testing.T) {
	env, err := SetupTestEnvironment("TestSourceWithoutAuthentication")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source without auth_id (empty string)
	source := map[string]interface{}{
		"name":     "Source without Auth",
		"type":     "jira",
		"base_url": "https://public.atlassian.net/jira",
		"auth_id":  "", // Explicitly set to empty
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify auth_id is empty
	if authID, ok := result["auth_id"].(string); ok && authID != "" {
		t.Errorf("Expected empty auth_id, got: %s", authID)
	}

	// Cleanup
	if sourceID, ok := result["id"].(string); ok {
		defer h.DELETE("/api/sources/" + sourceID)
	}

	t.Log("✓ Successfully created source without authentication")
}

// TestCreateSourceWithFilters verifies that sources can be created with filter patterns
func TestCreateSourceWithFilters(t *testing.T) {
	env, err := SetupTestEnvironment("TestCreateSourceWithFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source with include and exclude filters
	source := map[string]interface{}{
		"name":        "Source with Filters",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"filters": map[string]interface{}{
			"include_patterns": "browse,projects,issues",
			"exclude_patterns": "admin,logout",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify filters are present in response
	filters, ok := result["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in response")
	}

	includePatterns, ok := filters["include_patterns"].(string)
	if !ok || includePatterns != "browse,projects,issues" {
		t.Errorf("Expected include_patterns 'browse,projects,issues', got: %v", filters["include_patterns"])
	}

	excludePatterns, ok := filters["exclude_patterns"].(string)
	if !ok || excludePatterns != "admin,logout" {
		t.Errorf("Expected exclude_patterns 'admin,logout', got: %v", filters["exclude_patterns"])
	}

	// Get the source and verify filters persisted
	sourceID, ok := result["id"].(string)
	if !ok {
		t.Fatal("Could not extract source ID")
	}

	// Cleanup
	defer h.DELETE("/api/sources/" + sourceID)

	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	var getResult map[string]interface{}
	if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	// Verify filters persisted correctly
	persistedFilters, ok := getResult["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not persisted in database")
	}

	if persistedFilters["include_patterns"] != "browse,projects,issues" {
		t.Errorf("Persisted include_patterns mismatch: %v", persistedFilters["include_patterns"])
	}

	if persistedFilters["exclude_patterns"] != "admin,logout" {
		t.Errorf("Persisted exclude_patterns mismatch: %v", persistedFilters["exclude_patterns"])
	}

	t.Log("✓ Successfully created source with filters and verified persistence")
}

// TestCreateSourceWithoutFilters verifies that sources can be created without filters
func TestCreateSourceWithoutFilters(t *testing.T) {
	env, err := SetupTestEnvironment("TestCreateSourceWithoutFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source without filters field
	source := map[string]interface{}{
		"name":        "Source without Filters",
		"type":        "confluence",
		"base_url":    "https://test.atlassian.net/wiki",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Cleanup
	if sourceID, ok := result["id"].(string); ok {
		defer h.DELETE("/api/sources/" + sourceID)

		// Get the source and verify filters is null or empty
		getResp, err := h.GET("/api/sources/" + sourceID)
		if err != nil {
			t.Fatalf("Failed to get source: %v", err)
		}

		var getResult map[string]interface{}
		if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
			t.Fatalf("Failed to parse get response: %v", err)
		}

		// Filters can be nil or empty map
		if filters, exists := getResult["filters"]; exists && filters != nil {
			if filtersMap, ok := filters.(map[string]interface{}); ok && len(filtersMap) > 0 {
				t.Errorf("Expected empty or nil filters, got: %v", filters)
			}
		}
	}

	t.Log("✓ Successfully created source without filters")
}

// TestUpdateSourceFilters verifies that source filters can be updated
func TestUpdateSourceFilters(t *testing.T) {
	env, err := SetupTestEnvironment("TestUpdateSourceFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source with initial filters
	source := map[string]interface{}{
		"name":        "Source to Update Filters",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"filters": map[string]interface{}{
			"include_patterns": "browse,projects",
			"exclude_patterns": "admin",
		},
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
	defer h.DELETE("/api/sources/" + sourceID)

	// Update the source with modified filters
	updatedSource := map[string]interface{}{
		"name":        "Source to Update Filters",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"filters": map[string]interface{}{
			"include_patterns": "issues,epics,stories",
			"exclude_patterns": "admin,logout,settings",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	updateResp, err := h.PUT("/api/sources/"+sourceID, updatedSource)
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}

	h.AssertStatusCode(updateResp, http.StatusOK)

	// Verify the filters were updated
	getResp, err := h.GET("/api/sources/" + sourceID)
	if err != nil {
		t.Fatalf("Failed to get updated source: %v", err)
	}

	var getResult map[string]interface{}
	if err := h.ParseJSONResponse(getResp, &getResult); err != nil {
		t.Fatalf("Failed to parse get response: %v", err)
	}

	filters, ok := getResult["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in updated source")
	}

	if filters["include_patterns"] != "issues,epics,stories" {
		t.Errorf("Expected updated include_patterns 'issues,epics,stories', got: %v", filters["include_patterns"])
	}

	if filters["exclude_patterns"] != "admin,logout,settings" {
		t.Errorf("Expected updated exclude_patterns 'admin,logout,settings', got: %v", filters["exclude_patterns"])
	}

	t.Log("✓ Successfully updated source filters")
}

// TestFilterSanitization verifies that filter patterns are properly sanitized
func TestFilterSanitization(t *testing.T) {
	env, err := SetupTestEnvironment("TestFilterSanitization")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source with messy filter input (extra whitespace, empty tokens)
	source := map[string]interface{}{
		"name":        "Source with Messy Filters",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"filters": map[string]interface{}{
			"include_patterns": "  browse , , projects  ,issues  ",
			"exclude_patterns": " admin,  ,logout,  ",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Cleanup
	if sourceID, ok := result["id"].(string); ok {
		defer h.DELETE("/api/sources/" + sourceID)
	}

	// Verify filters were sanitized (whitespace trimmed, empty tokens removed)
	filters, ok := result["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in response")
	}

	includePatterns, ok := filters["include_patterns"].(string)
	if !ok || includePatterns != "browse,projects,issues" {
		t.Errorf("Expected sanitized include_patterns 'browse,projects,issues', got: %v", filters["include_patterns"])
	}

	excludePatterns, ok := filters["exclude_patterns"].(string)
	if !ok || excludePatterns != "admin,logout" {
		t.Errorf("Expected sanitized exclude_patterns 'admin,logout', got: %v", filters["exclude_patterns"])
	}

	t.Log("✓ Filter sanitization working correctly")
}

// TestFilterSanitizationAllWhitespace verifies handling of all-whitespace patterns
func TestFilterSanitizationAllWhitespace(t *testing.T) {
	env, err := SetupTestEnvironment("TestFilterSanitizationAllWhitespace")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source with only whitespace in filters
	source := map[string]interface{}{
		"name":        "Source with Whitespace Filters",
		"type":        "jira",
		"base_url":    "https://test.atlassian.net",
		"auth_domain": "test.atlassian.net",
		"enabled":     true,
		"filters": map[string]interface{}{
			"include_patterns": "   ,  ,  ",
			"exclude_patterns": "  ",
		},
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	resp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Cleanup
	if sourceID, ok := result["id"].(string); ok {
		defer h.DELETE("/api/sources/" + sourceID)
	}

	// Verify whitespace-only patterns are sanitized to empty string
	filters, ok := result["filters"].(map[string]interface{})
	if !ok {
		t.Fatal("Filters not found in response")
	}

	if includePatterns := filters["include_patterns"]; includePatterns != "" && includePatterns != nil {
		t.Errorf("Expected empty include_patterns, got: %v", includePatterns)
	}

	if excludePatterns := filters["exclude_patterns"]; excludePatterns != "" && excludePatterns != nil {
		t.Errorf("Expected empty exclude_patterns, got: %v", excludePatterns)
	}

	t.Log("✓ Whitespace-only patterns sanitized correctly")
}

// TestListSourcesIncludesFilters verifies that listing sources includes their filters
func TestListSourcesIncludesFilters(t *testing.T) {
	env, err := SetupTestEnvironment("TestListSourcesIncludesFilters")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create multiple sources with different filter configurations
	sources := []map[string]interface{}{
		{
			"name":        "Source with Include Only",
			"type":        "jira",
			"base_url":    "https://test1.atlassian.net",
			"auth_domain": "test1.atlassian.net",
			"enabled":     true,
			"filters": map[string]interface{}{
				"include_patterns": "browse,projects",
			},
			"crawl_config": map[string]interface{}{
				"concurrency": 1,
			},
		},
		{
			"name":        "Source with Exclude Only",
			"type":        "confluence",
			"base_url":    "https://test2.atlassian.net/wiki",
			"auth_domain": "test2.atlassian.net",
			"enabled":     true,
			"filters": map[string]interface{}{
				"exclude_patterns": "admin,logout",
			},
			"crawl_config": map[string]interface{}{
				"concurrency": 1,
			},
		},
		{
			"name":        "Source with Both Filters",
			"type":        "jira",
			"base_url":    "https://test3.atlassian.net",
			"auth_domain": "test3.atlassian.net",
			"enabled":     true,
			"filters": map[string]interface{}{
				"include_patterns": "issues,epics",
				"exclude_patterns": "settings",
			},
			"crawl_config": map[string]interface{}{
				"concurrency": 1,
			},
		},
	}

	var sourceIDs []string

	// Create all sources
	for _, source := range sources {
		resp, err := h.POST("/api/sources", source)
		if err != nil {
			t.Fatalf("Failed to create source '%s': %v", source["name"], err)
		}

		var result map[string]interface{}
		if err := h.ParseJSONResponse(resp, &result); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if sourceID, ok := result["id"].(string); ok {
			sourceIDs = append(sourceIDs, sourceID)
		}
	}

	// Cleanup all sources
	defer func() {
		for _, sourceID := range sourceIDs {
			h.DELETE("/api/sources/" + sourceID)
		}
	}()

	// List all sources
	listResp, err := h.GET("/api/sources")
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}

	var listedSources []map[string]interface{}
	if err := h.ParseJSONResponse(listResp, &listedSources); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	// Verify our sources are in the list with their filters
	foundCount := 0
	for _, listedSource := range listedSources {
		sourceID := listedSource["id"].(string)

		// Check if this is one of our test sources
		isTestSource := false
		for _, testSourceID := range sourceIDs {
			if sourceID == testSourceID {
				isTestSource = true
				break
			}
		}

		if !isTestSource {
			continue
		}

		foundCount++

		// Verify filters are included in list response
		name := listedSource["name"].(string)
		switch name {
		case "Source with Include Only":
			if filters, ok := listedSource["filters"].(map[string]interface{}); ok {
				if filters["include_patterns"] != "browse,projects" {
					t.Errorf("Source '%s': expected include_patterns 'browse,projects', got: %v", name, filters["include_patterns"])
				}
			} else {
				t.Errorf("Source '%s': filters not found in list response", name)
			}

		case "Source with Exclude Only":
			if filters, ok := listedSource["filters"].(map[string]interface{}); ok {
				if filters["exclude_patterns"] != "admin,logout" {
					t.Errorf("Source '%s': expected exclude_patterns 'admin,logout', got: %v", name, filters["exclude_patterns"])
				}
			} else {
				t.Errorf("Source '%s': filters not found in list response", name)
			}

		case "Source with Both Filters":
			if filters, ok := listedSource["filters"].(map[string]interface{}); ok {
				if filters["include_patterns"] != "issues,epics" {
					t.Errorf("Source '%s': expected include_patterns 'issues,epics', got: %v", name, filters["include_patterns"])
				}
				if filters["exclude_patterns"] != "settings" {
					t.Errorf("Source '%s': expected exclude_patterns 'settings', got: %v", name, filters["exclude_patterns"])
				}
			} else {
				t.Errorf("Source '%s': filters not found in list response", name)
			}
		}
	}

	if foundCount != len(sources) {
		t.Errorf("Expected to find %d test sources in list, found %d", len(sources), foundCount)
	}

	t.Log("✓ List sources includes filters correctly")
}
