// Package api contains API integration tests for auth config loading.
package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ternarybob/quaero/test/common"
)

// TestAuthConfigLoading tests that auth config files are loaded from test/config/auth directory
func TestAuthConfigLoading(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthConfigLoading")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Get list of all auth credentials
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

	env.LogTest(t, "Found %d authentication credentials", len(auths))

	// Verify test API key is loaded
	foundTestKey := false
	for _, auth := range auths {
		name, _ := auth["name"].(string)
		serviceType, _ := auth["service_type"].(string)
		authType, _ := auth["auth_type"].(string)

		env.LogTest(t, "Auth credential: name=%s, service_type=%s, auth_type=%s", name, serviceType, authType)

		// Check if our test API key is present
		if name == "test-google-places-key" && serviceType == "google-places" && authType == "api_key" {
			foundTestKey = true

			// Verify API key is masked in response
			apiKey, _ := auth["api_key"].(string)
			if strings.Contains(apiKey, "Test") {
				t.Error("API key should be masked in list response")
			}

			// Verify description is present
			data, _ := auth["data"].(map[string]interface{})
			if data != nil {
				description, _ := data["description"].(string)
				expectedDesc := "Test Google Places API key for automated testing"
				if description != expectedDesc {
					t.Errorf("Expected description '%s', got '%s'", expectedDesc, description)
				}
			}

			env.LogTest(t, "✓ Found test API key: %s (service: %s)", name, serviceType)
		}
	}

	if !foundTestKey {
		t.Error("Test API key 'test-google-places-key' not found - auth config loading failed")
	}
}

// TestAuthConfigAPIKeyEndpoint tests the API key CRUD endpoints
func TestAuthConfigAPIKeyEndpoint(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthConfigAPIKeyEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Get list of all auth credentials to find test key ID
	resp, err := h.GET("/api/auth/list")
	if err != nil {
		t.Fatalf("Failed to get auth list: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var auths []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &auths); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Find test API key ID
	var testKeyID string
	for _, auth := range auths {
		name, _ := auth["name"].(string)
		if name == "test-google-places-key" {
			testKeyID, _ = auth["id"].(string)
			break
		}
	}

	if testKeyID == "" {
		t.Fatal("Test API key not found in auth list")
	}

	env.LogTest(t, "Test API key ID: %s", testKeyID)

	// Get specific API key by ID
	resp, err = h.GET("/api/auth/api-key/" + testKeyID)
	if err != nil {
		t.Fatalf("Failed to get API key by ID: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var authDetail map[string]interface{}
	if err := h.ParseJSONResponse(resp, &authDetail); err != nil {
		t.Fatalf("Failed to decode API key detail: %v", err)
	}

	// Verify API key details
	name, _ := authDetail["name"].(string)
	if name != "test-google-places-key" {
		t.Errorf("Expected name 'test-google-places-key', got '%s'", name)
	}

	serviceType, _ := authDetail["service_type"].(string)
	if serviceType != "google-places" {
		t.Errorf("Expected service_type 'google-places', got '%s'", serviceType)
	}

	authType, _ := authDetail["auth_type"].(string)
	if authType != "api_key" {
		t.Errorf("Expected auth_type 'api_key', got '%s'", authType)
	}

	// Verify API key is unmasked in detail response (for authenticated requests)
	apiKey, _ := authDetail["api_key"].(string)
	if apiKey == "" {
		t.Error("API key should not be empty in detail response")
	}

	env.LogTest(t, "✓ API key detail retrieved successfully")
}
