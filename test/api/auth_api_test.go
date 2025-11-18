// Package api contains API integration tests.
package api

import (
	"encoding/json"
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
)

// TestAuthListEndpoint tests the GET /api/auth/list endpoint
func TestAuthListEndpoint(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthListEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Test listing authentications
	resp, err := h.GET("/api/auth/list")
	if err != nil {
		t.Fatalf("Failed to get auth list: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// Parse response
	var auths []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &auths); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be an array (possibly empty)
	env.LogTest(t, "Found %d authentication credentials", len(auths))
}

// TestAuthCaptureEndpoint tests capturing auth from Chrome extension
func TestAuthCaptureEndpoint(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthCaptureEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create test auth data (simulating Chrome extension)
	authData := map[string]interface{}{
		"baseUrl":   "https://test.atlassian.net",
		"userAgent": "Mozilla/5.0 Test",
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-token",
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

	// Send POST request
	resp, err := h.POST("/api/auth", authData)
	if err != nil {
		t.Fatalf("Failed to post auth data: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// Verify response
	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, ok := result["status"].(string); !ok || status != "success" {
		t.Errorf("Expected success status, got %v", result["status"])
	}

	env.LogTest(t, "Authentication capture endpoint works")
}

// TestAuthStatusEndpoint tests the GET /api/auth/status endpoint
func TestAuthStatusEndpoint(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthStatusEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	resp, err := h.GET("/api/auth/status")
	if err != nil {
		t.Fatalf("Failed to get auth status: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// Parse response
	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	resp.Body.Close()

	// Should have authenticated field
	if _, ok := status["authenticated"]; !ok {
		t.Error("Response missing 'authenticated' field")
	}

	env.LogTest(t, "Auth status: authenticated=%v", status["authenticated"])
}
