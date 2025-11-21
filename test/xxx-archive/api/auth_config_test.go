// Package api contains API integration tests for auth config loading.
package api

import (
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test/common"
)

// TestAuthConfigLoading tests that variables are loaded to KV store from test/config/auth directory (Phase 4)
// NOTE: This test requires /api/kv endpoints to be implemented
func TestAuthConfigLoading(t *testing.T) {
	t.Skip("Skipping until /api/kv endpoints are implemented - see Phase 4 cleanup")

	env, err := common.SetupTestEnvironment("TestAuthConfigLoading")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// TODO: Once /api/kv endpoints are implemented, update this test to:
	// 1. GET /api/kv (or /api/kv/list) to retrieve all KV pairs
	// 2. Parse response as []map[string]interface{} with "key", "value", "description" fields
	// 3. Verify "test-google-places-key" exists in KV store
	// 4. Assert description matches "Test Google Places API key for automated testing"
	// 5. Remove auth_type, service_type, api_key checks (KV store doesn't have these)

	// Get list of KV pairs (endpoint not yet implemented)
	resp, err := h.GET("/api/kv")
	if err != nil {
		t.Fatalf("Failed to get KV list: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// Parse response
	var kvPairs []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &kvPairs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	env.LogTest(t, "Found %d KV pairs", len(kvPairs))

	// Verify test API key is loaded to KV store
	foundTestKey := false
	for _, kv := range kvPairs {
		key, _ := kv["key"].(string)

		env.LogTest(t, "KV pair: key=%s", key)

		// Check if our test API key is present
		if key == "test-google-places-key" {
			foundTestKey = true

			// Verify description is present
			description, _ := kv["description"].(string)
			expectedDesc := "Test Google Places API key for automated testing"
			if description != expectedDesc {
				t.Errorf("Expected description '%s', got '%s'", expectedDesc, description)
			}

			env.LogTest(t, "✓ Found test API key in KV store: %s", key)
		}
	}

	if !foundTestKey {
		t.Error("Test API key 'test-google-places-key' not found in KV store - config loading failed")
	}
}

// TestAuthConfigAPIKeyEndpoint tests the KV store CRUD endpoints for variables (Phase 4)
// NOTE: This test requires /api/kv endpoints to be implemented
func TestAuthConfigAPIKeyEndpoint(t *testing.T) {
	t.Skip("Skipping until /api/kv endpoints are implemented - see Phase 4 cleanup")

	env, err := common.SetupTestEnvironment("TestAuthConfigAPIKeyEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// TODO: Once /api/kv endpoints are implemented, update this test to:
	// 1. GET /api/kv/test-google-places-key to retrieve specific key
	// 2. Verify response has "key", "value" (unmasked), "description" fields
	// 3. Remove auth_type, service_type, id checks (KV store doesn't have these)
	// 4. Test CRUD operations: POST /api/kv (create), PUT /api/kv/{key} (update), DELETE /api/kv/{key} (delete)

	// Get specific API key by key name (endpoint not yet implemented)
	resp, err := h.GET("/api/kv/test-google-places-key")
	if err != nil {
		t.Fatalf("Failed to get API key by key: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var kvDetail map[string]interface{}
	if err := h.ParseJSONResponse(resp, &kvDetail); err != nil {
		t.Fatalf("Failed to decode KV detail: %v", err)
	}

	// Verify KV details
	key, _ := kvDetail["key"].(string)
	if key != "test-google-places-key" {
		t.Errorf("Expected key 'test-google-places-key', got '%s'", key)
	}

	// Verify value is present and unmasked
	value, _ := kvDetail["value"].(string)
	if value == "" {
		t.Error("API key value should not be empty in detail response")
	}

	// Verify description
	description, _ := kvDetail["description"].(string)
	expectedDesc := "Test Google Places API key for automated testing"
	if description != expectedDesc {
		t.Errorf("Expected description '%s', got '%s'", expectedDesc, description)
	}

	env.LogTest(t, "✓ API key detail retrieved successfully from KV store")
}
