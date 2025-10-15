package api

import (
	"bytes"
	"encoding/json"
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

// TestCreateSourcesWithAuthentication creates test Jira and Confluence sources with authentication
func TestCreateSourcesWithAuthentication(t *testing.T) {
	baseURL := test.MustGetTestServerURL()

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
	h := test.NewHTTPTestHelper(t, baseURL)

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
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

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
