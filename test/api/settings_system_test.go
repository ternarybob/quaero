package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// Helper functions for test operations

// createKVPair creates a KV pair and returns the key
func createKVPair(t *testing.T, helper *common.HTTPTestHelper, key, value, description string) string {
	body := map[string]string{
		"key":         key,
		"value":       value,
		"description": description,
	}

	resp, err := helper.POST("/api/kv", body)
	require.NoError(t, err, "Failed to create KV pair")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusCreated)
	t.Logf("Created KV pair: key=%s", key)

	return key
}

// deleteKVPair deletes a KV pair
func deleteKVPair(t *testing.T, helper *common.HTTPTestHelper, key string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/kv/%s", key))
	require.NoError(t, err, "Failed to delete KV pair")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)
	t.Logf("Deleted KV pair: key=%s", key)
}

// cleanupKVPair deletes a KV pair if it exists (ignores 404)
func cleanupKVPair(t *testing.T, helper *common.HTTPTestHelper, key string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/kv/%s", key))
	if err != nil {
		t.Logf("Failed to cleanup KV pair %s: %v", key, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Logf("Warning: Cleanup failed for key %s with status %d", key, resp.StatusCode)
	} else {
		t.Logf("Cleaned up KV pair: key=%s", key)
	}
}

// createConnector creates a connector and returns its ID
func createConnector(t *testing.T, helper *common.HTTPTestHelper, name, connectorType string, config map[string]interface{}) string {
	body := map[string]interface{}{
		"name":   name,
		"type":   connectorType,
		"config": config,
	}

	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err, "Failed to create connector")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse create connector response")

	connector, ok := result["connector"].(map[string]interface{})
	require.True(t, ok, "Response should contain connector object")

	id, ok := connector["id"].(string)
	require.True(t, ok, "Connector should have ID field")

	t.Logf("Created connector: id=%s, name=%s, type=%s", id, name, connectorType)
	return id
}

// deleteConnector deletes a connector
func deleteConnector(t *testing.T, helper *common.HTTPTestHelper, id string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/connectors/%s", id))
	require.NoError(t, err, "Failed to delete connector")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusNoContent)
	t.Logf("Deleted connector: id=%s", id)
}

// TestKVStore_CRUD tests complete KV store lifecycle
func TestKVStore_CRUD(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Cleanup from previous runs
	cleanupKVPair(t, helper, "test_key")

	// 1. POST /api/kv - Create new KV pair
	t.Log("Step 1: Creating KV pair TEST_KEY")
	body := map[string]string{
		"key":         "TEST_KEY",
		"value":       "test-value-123",
		"description": "Test key",
	}
	resp, err := helper.POST("/api/kv", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var createResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &createResult)
	require.NoError(t, err)
	assert.Equal(t, "Key/value pair created successfully", createResult["message"])

	// 2. GET /api/kv - List KV pairs (value should be masked)
	t.Log("Step 2: Listing KV pairs (checking masking)")
	resp, err = helper.GET("/api/kv")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var pairs []interface{}
	err = helper.ParseJSONResponse(resp, &pairs)
	require.NoError(t, err)

	require.Greater(t, len(pairs), 0, "Should have at least one KV pair")

	// Find our TEST_KEY and verify masking
	foundKey := false
	for _, p := range pairs {
		pair := p.(map[string]interface{})
		if pair["key"] == "test_key" { // Keys are normalized to lowercase
			foundKey = true
			// Value should be masked: "test...123"
			assert.Equal(t, "test...-123", pair["value"], "Value should be masked in list")
			break
		}
	}
	assert.True(t, foundKey, "TEST_KEY should be in the list")

	// 3. GET /api/kv/test_key - Get specific KV pair (lowercase, full value)
	t.Log("Step 3: Getting specific KV pair (full unmasked value)")
	resp, err = helper.GET("/api/kv/test_key")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var getResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &getResult)
	require.NoError(t, err)
	assert.Equal(t, "test_key", getResult["key"], "Key should be lowercase")
	assert.Equal(t, "test-value-123", getResult["value"], "Should return full unmasked value")

	// 4. PUT /api/kv/Test_Key - Update (mixed case, should upsert existing)
	t.Log("Step 4: Updating KV pair with mixed case key")
	updateBody := map[string]string{
		"value":       "updated-value-456",
		"description": "Updated description",
	}
	resp, err = helper.PUT("/api/kv/Test_Key", updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var updateResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &updateResult)
	require.NoError(t, err)
	assert.Equal(t, false, updateResult["created"], "Should be update, not create")

	// 5. GET /api/kv/TEST_KEY - Verify update (uppercase)
	t.Log("Step 5: Verifying updated value")
	resp, err = helper.GET("/api/kv/TEST_KEY")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var verifyResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &verifyResult)
	require.NoError(t, err)
	assert.Equal(t, "updated-value-456", verifyResult["value"], "Value should be updated")

	// 6. DELETE /api/kv/test_key
	t.Log("Step 6: Deleting KV pair")
	resp, err = helper.DELETE("/api/kv/test_key")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// 7. GET /api/kv/TEST_KEY - Verify deletion (should 404)
	t.Log("Step 7: Verifying deletion")
	resp, err = helper.GET("/api/kv/TEST_KEY")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("✓ KV CRUD test completed successfully")
}

// TestKVStore_CaseInsensitive tests case-insensitive key handling
func TestKVStore_CaseInsensitive(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Cleanup from previous runs
	cleanupKVPair(t, helper, "google_api_key")

	// 1. Create with uppercase key
	t.Log("Step 1: Creating KV pair with uppercase GOOGLE_API_KEY")
	createKVPair(t, helper, "GOOGLE_API_KEY", "test-api-key-value", "Google API Key")

	// 2. GET with lowercase key
	t.Log("Step 2: Getting with lowercase google_api_key")
	resp, err := helper.GET("/api/kv/google_api_key")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result1 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result1)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key-value", result1["value"])

	// 3. GET with mixed case key
	t.Log("Step 3: Getting with mixed case Google_Api_Key")
	resp, err = helper.GET("/api/kv/Google_Api_Key")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result2 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result2)
	require.NoError(t, err)
	assert.Equal(t, "test-api-key-value", result2["value"])

	// 4. PUT with mixed case (should update, not create new)
	t.Log("Step 4: Updating with mixed case GOOGLE_api_KEY")
	updateBody := map[string]string{
		"value": "updated-api-key-value",
	}
	resp, err = helper.PUT("/api/kv/GOOGLE_api_KEY", updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var updateResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &updateResult)
	require.NoError(t, err)
	assert.Equal(t, false, updateResult["created"], "Should be update, not create")

	// 5. Verify only 1 key exists (not 3 duplicates)
	t.Log("Step 5: Verifying only one key exists")
	resp, err = helper.GET("/api/kv")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var pairs []interface{}
	err = helper.ParseJSONResponse(resp, &pairs)
	require.NoError(t, err)

	googleKeyCount := 0
	for _, p := range pairs {
		pair := p.(map[string]interface{})
		if pair["key"] == "google_api_key" {
			googleKeyCount++
		}
	}
	assert.Equal(t, 1, googleKeyCount, "Should only have 1 google_api_key, not duplicates")

	// Cleanup
	deleteKVPair(t, helper, "google_api_key")

	t.Log("✓ Case-insensitive test completed successfully")
}

// TestKVStore_Upsert tests PUT upsert behavior
func TestKVStore_Upsert(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Cleanup from previous runs
	cleanupKVPair(t, helper, "new_key")

	// 1. PUT non-existent key (should create with 201)
	t.Log("Step 1: PUT new key NEW_KEY (should create)")
	body1 := map[string]string{
		"value":       "value-1",
		"description": "New key via upsert",
	}
	resp, err := helper.PUT("/api/kv/NEW_KEY", body1)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result1 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result1)
	require.NoError(t, err)
	assert.Equal(t, true, result1["created"], "Should indicate creation")

	// 2. PUT same key (lowercase) with new value (should update with 200)
	t.Log("Step 2: PUT existing key new_key (should update)")
	body2 := map[string]string{
		"value": "value-2",
	}
	resp, err = helper.PUT("/api/kv/new_key", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result2 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result2)
	require.NoError(t, err)
	assert.Equal(t, false, result2["created"], "Should indicate update")

	// 3. GET and verify updated value
	t.Log("Step 3: Verifying updated value")
	resp, err = helper.GET("/api/kv/NEW_KEY")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result3 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result3)
	require.NoError(t, err)
	assert.Equal(t, "value-2", result3["value"], "Value should be updated")

	// Cleanup
	deleteKVPair(t, helper, "new_key")

	t.Log("✓ Upsert test completed successfully")
}

// TestKVStore_DuplicateValidation tests duplicate key detection
func TestKVStore_DuplicateValidation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Cleanup from previous runs
	cleanupKVPair(t, helper, "duplicate_key")

	// 1. Create first key
	t.Log("Step 1: Creating DUPLICATE_KEY")
	createKVPair(t, helper, "DUPLICATE_KEY", "value-1", "First key")

	// 2. Attempt duplicate with same case
	t.Log("Step 2: Attempting duplicate with same case")
	body := map[string]string{
		"key":   "DUPLICATE_KEY",
		"value": "value-2",
	}
	resp, err := helper.POST("/api/kv", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusConflict)

	var result1 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result1)
	require.NoError(t, err)
	assert.Contains(t, result1["error"].(string), "already exists", "Error should mention key already exists")

	// 3. Attempt duplicate with different case (case-insensitive duplicate)
	t.Log("Step 3: Attempting case-insensitive duplicate")
	body2 := map[string]string{
		"key":   "duplicate_key",
		"value": "value-3",
	}
	resp, err = helper.POST("/api/kv", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusConflict)

	// Cleanup
	deleteKVPair(t, helper, "duplicate_key")

	t.Log("✓ Duplicate validation test completed successfully")
}

// TestKVStore_ValueMasking tests value masking in list endpoint
func TestKVStore_ValueMasking(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Cleanup from previous runs
	cleanupKVPair(t, helper, "short")
	cleanupKVPair(t, helper, "long")

	// 1. Create short value
	t.Log("Step 1: Creating short value")
	createKVPair(t, helper, "SHORT", "abc", "Short value")

	// 2. Create long value
	t.Log("Step 2: Creating long value")
	createKVPair(t, helper, "LONG", "sk-1234567890abcdef", "Long value")

	// 3. List and verify masking
	t.Log("Step 3: Listing and verifying masking")
	resp, err := helper.GET("/api/kv")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var pairs []interface{}
	err = helper.ParseJSONResponse(resp, &pairs)
	require.NoError(t, err)

	// Check masking for both keys
	for _, p := range pairs {
		pair := p.(map[string]interface{})
		if pair["key"] == "short" {
			// Short values should be fully masked: "••••••••"
			assert.Equal(t, "••••••••", pair["value"], "Short value should be fully masked")
		} else if pair["key"] == "long" {
			// Long values should show first 4 + "..." + last 4: "sk-1...cdef"
			assert.Equal(t, "sk-1...cdef", pair["value"], "Long value should be partially masked")
		}
	}

	// 4. Get SHORT directly (should be unmasked)
	t.Log("Step 4: Getting SHORT directly (unmasked)")
	resp, err = helper.GET("/api/kv/SHORT")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var shortResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &shortResult)
	require.NoError(t, err)
	assert.Equal(t, "abc", shortResult["value"], "Direct GET should return unmasked value")

	// 5. Get LONG directly (should be unmasked)
	t.Log("Step 5: Getting LONG directly (unmasked)")
	resp, err = helper.GET("/api/kv/LONG")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var longResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &longResult)
	require.NoError(t, err)
	assert.Equal(t, "sk-1234567890abcdef", longResult["value"], "Direct GET should return unmasked value")

	// Cleanup
	deleteKVPair(t, helper, "short")
	deleteKVPair(t, helper, "long")

	t.Log("✓ Value masking test completed successfully")
}

// TestKVStore_ValidationErrors tests validation error cases
func TestKVStore_ValidationErrors(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// 1. POST with empty key
	t.Log("Step 1: POST with empty key")
	body1 := map[string]string{
		"key":   "",
		"value": "some-value",
	}
	resp, err := helper.POST("/api/kv", body1)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	var result1 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result1)
	require.NoError(t, err)
	assert.Contains(t, result1["error"].(string), "Key is required", "Error should mention key required")

	// 2. POST with empty value
	t.Log("Step 2: POST with empty value")
	body2 := map[string]string{
		"key":   "TEST_KEY",
		"value": "",
	}
	resp, err = helper.POST("/api/kv", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	var result2 map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result2)
	require.NoError(t, err)
	assert.Contains(t, result2["error"].(string), "Value is required", "Error should mention value required")

	// 3. POST with invalid JSON (raw string)
	t.Log("Step 3: POST with invalid JSON")
	resp, err = helper.POSTBody("/api/kv", "application/json", []byte("not valid json"))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// 4. GET with empty key (should 400)
	t.Log("Step 4: GET with empty key")
	resp, err = helper.GET("/api/kv/")
	require.NoError(t, err)
	defer resp.Body.Close()
	// May return 404 for empty path depending on routing, accept either 400 or 404
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
		"Empty key should return 400 or 404")

	// 5. PUT on nonexistent key with only description (should 404 - can't update description on missing key)
	t.Log("Step 5: PUT description-only update on missing key")
	body5 := map[string]string{
		"description": "New description",
	}
	resp, err = helper.PUT("/api/kv/nonexistent_key", body5)
	require.NoError(t, err)
	defer resp.Body.Close()
	// This should fail since you can't update just description on a nonexistent key without value
	// Expected: 400 Bad Request (value required for new key)
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
		"Description-only update on missing key should fail")

	t.Log("✓ Validation errors test completed successfully")
}

// TestConnectors_CRUD tests connector lifecycle
func TestConnectors_CRUD(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Note: GitHub connection test will fail with test token, but we're testing the API structure
	// Skip if you don't have valid GitHub token in test config

	// 1. Create connector (may fail connection test with invalid token, adjust as needed)
	t.Log("Step 1: Creating test connector")
	body := map[string]interface{}{
		"name": "Test GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": "test-invalid-token",
			"owner": "test-owner",
			"repo":  "test-repo",
		},
	}
	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Expect 400 due to invalid token (connection test fails)
	// If you have valid token, this should be 201
	if resp.StatusCode == http.StatusBadRequest {
		// Connection test failed as expected with invalid token
		t.Log("⚠️  Connection test failed (expected with test token)")
		t.Skip("Skipping CRUD test - requires valid GitHub token for connection test")
		return
	}

	// If we got 201, continue with full CRUD test
	helper.AssertStatusCode(resp, http.StatusCreated)

	var createResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &createResult)
	require.NoError(t, err)

	// Response should be the connector object directly (not nested)
	connectorID, ok := createResult["id"].(string)
	require.True(t, ok, "Response should have ID field")
	require.NotEmpty(t, connectorID, "Connector ID should not be empty")

	t.Logf("Created connector with ID: %s", connectorID)

	// 2. List connectors
	t.Log("Step 2: Listing connectors")
	resp, err = helper.GET("/api/connectors")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var listResult []interface{}
	err = helper.ParseJSONResponse(resp, &listResult)
	require.NoError(t, err)
	require.Greater(t, len(listResult), 0, "Should have at least one connector")

	// Find our connector in the list
	foundConnector := false
	for _, c := range listResult {
		connector := c.(map[string]interface{})
		if connector["id"] == connectorID {
			foundConnector = true
			assert.Equal(t, "Test GitHub Connector", connector["name"])
			assert.Equal(t, "github", connector["type"])
			break
		}
	}
	assert.True(t, foundConnector, "Created connector should be in list")

	// 3. Update connector
	t.Log("Step 3: Updating connector")
	updateBody := map[string]interface{}{
		"name": "Updated GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": "test-invalid-token",
			"owner": "test-owner",
			"repo":  "test-repo",
		},
	}
	resp, err = helper.PUT(fmt.Sprintf("/api/connectors/%s", connectorID), updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var updateResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &updateResult)
	require.NoError(t, err)
	assert.Equal(t, "Updated GitHub Connector", updateResult["name"])

	// 4. Delete connector
	t.Log("Step 4: Deleting connector")
	resp, err = helper.DELETE(fmt.Sprintf("/api/connectors/%s", connectorID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNoContent)

	// 5. Verify connector is deleted
	t.Log("Step 5: Verifying deletion")
	resp, err = helper.GET("/api/connectors")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var verifyResult []interface{}
	err = helper.ParseJSONResponse(resp, &verifyResult)
	require.NoError(t, err)

	// Connector should not be in list
	for _, c := range verifyResult {
		connector := c.(map[string]interface{})
		assert.NotEqual(t, connectorID, connector["id"], "Deleted connector should not be in list")
	}

	t.Log("✓ Connector CRUD test completed successfully")
}

// TestConnectors_Validation tests connector validation
func TestConnectors_Validation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// 1. POST with empty name
	t.Log("Step 1: POST with empty name")
	body1 := map[string]interface{}{
		"name": "",
		"type": "github",
		"config": map[string]interface{}{
			"token": "test-token",
		},
	}
	resp, err := helper.POST("/api/connectors", body1)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// 2. POST with empty type
	t.Log("Step 2: POST with empty type")
	body2 := map[string]interface{}{
		"name": "Test Connector",
		"type": "",
		"config": map[string]interface{}{
			"token": "test-token",
		},
	}
	resp, err = helper.POST("/api/connectors", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// 3. POST with invalid JSON
	t.Log("Step 3: POST with invalid JSON")
	resp, err = helper.POSTBody("/api/connectors", "application/json", []byte("invalid json"))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// 4. POST GitHub connector with missing config
	t.Log("Step 4: POST GitHub connector with missing config")
	body4 := map[string]interface{}{
		"name": "Test GitHub",
		"type": "github",
	}
	resp, err = helper.POST("/api/connectors", body4)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("✓ Connector validation test completed successfully")
}

// TestConnectors_GitHubConnectionTest tests GitHub connector connection testing
func TestConnectors_GitHubConnectionTest(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test with invalid GitHub token - should fail connection test
	t.Log("Testing GitHub connector with invalid token")
	body := map[string]interface{}{
		"name": "Test Invalid GitHub",
		"type": "github",
		"config": map[string]interface{}{
			"token": "invalid-token-123",
			"owner": "test-owner",
			"repo":  "test-repo",
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 due to connection test failure
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Read error message
	var errorResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &errorResult)
	// May not be JSON, just plain text error
	if err == nil {
		t.Logf("Error response: %v", errorResult)
	}

	t.Log("✓ GitHub connection test completed - invalid token rejected as expected")
	t.Log("Note: To test with valid token, provide GITHUB_TOKEN in test environment")
}

// TestConfig_Get tests config endpoint
func TestConfig_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/config
	t.Log("Getting application config")
	resp, err := helper.GET("/api/config")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure: {version, build, port, host, config}
	t.Log("Verifying config response structure")
	assert.Contains(t, result, "version", "Response should contain version")
	assert.Contains(t, result, "build", "Response should contain build")
	assert.Contains(t, result, "port", "Response should contain port")
	assert.Contains(t, result, "host", "Response should contain host")
	assert.Contains(t, result, "config", "Response should contain config object")

	// Verify version and build are non-empty strings
	version, ok := result["version"].(string)
	assert.True(t, ok, "Version should be string")
	assert.NotEmpty(t, version, "Version should not be empty")

	build, ok := result["build"].(string)
	assert.True(t, ok, "Build should be string")
	assert.NotEmpty(t, build, "Build should not be empty")

	// Verify port matches test environment
	port, ok := result["port"].(float64) // JSON numbers are float64
	assert.True(t, ok, "Port should be number")
	assert.Equal(t, float64(env.Port), port, "Port should match test environment")

	// Verify config object contains expected sections
	config, ok := result["config"].(map[string]interface{})
	assert.True(t, ok, "Config should be object")
	assert.Contains(t, config, "Server", "Config should contain Server section")

	t.Logf("Config: version=%s, build=%s, port=%d", version, build, int(port))
	t.Log("✓ Config endpoint test completed successfully")
}

// TestStatus_Get tests status endpoint
func TestStatus_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/status
	t.Log("Getting application status")
	resp, err := helper.GET("/api/status")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response contains expected status fields
	// Note: Exact structure depends on StatusService implementation
	t.Logf("Status response: %v", result)
	assert.NotEmpty(t, result, "Status response should not be empty")

	t.Log("✓ Status endpoint test completed successfully")
}

// TestVersion_Get tests version endpoint
func TestVersion_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/version
	t.Log("Getting version information")
	resp, err := helper.GET("/api/version")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response: {version, build, git_commit}
	assert.Contains(t, result, "version", "Response should contain version")
	assert.Contains(t, result, "build", "Response should contain build")
	assert.Contains(t, result, "git_commit", "Response should contain git_commit")

	// Verify all fields are non-empty strings
	version, ok := result["version"].(string)
	assert.True(t, ok, "Version should be string")
	assert.NotEmpty(t, version, "Version should not be empty")

	build, ok := result["build"].(string)
	assert.True(t, ok, "Build should be string")
	assert.NotEmpty(t, build, "Build should not be empty")

	gitCommit, ok := result["git_commit"].(string)
	assert.True(t, ok, "Git commit should be string")
	assert.NotEmpty(t, gitCommit, "Git commit should not be empty")

	t.Logf("Version: %s, Build: %s, Git Commit: %s", version, build, gitCommit)
	t.Log("✓ Version endpoint test completed successfully")
}

// TestHealth_Get tests health endpoint
func TestHealth_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/health
	t.Log("Getting health status")
	resp, err := helper.GET("/api/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response: {status: "ok"}
	assert.Equal(t, "ok", result["status"], "Health status should be 'ok'")

	t.Log("✓ Health endpoint test completed successfully")
}

// TestLogsRecent_Get tests recent logs endpoint
func TestLogsRecent_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/logs/recent
	t.Log("Getting recent logs")
	resp, err := helper.GET("/api/logs/recent")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	logs, ok := result["logs"].([]interface{})
	require.True(t, ok, "Response should contain logs array")

	// Verify response is array (may be empty if no recent activity)
	t.Logf("Recent logs count: %d", len(logs))

	// If logs exist, verify structure
	if len(logs) > 0 {
		firstLog := logs[0].(map[string]interface{})
		// Verify expected fields exist (exact structure may vary)
		t.Logf("First log entry: %v", firstLog)
	} else {
		t.Log("No recent logs (acceptable if service just started)")
	}

	t.Log("✓ Recent logs endpoint test completed successfully")
}

// TestSystemLogs_ListFiles tests log file listing
func TestSystemLogs_ListFiles(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// GET /api/system/logs/files
	t.Log("Listing log files")
	resp, err := helper.GET("/api/system/logs/files")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result []interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response is array of log file info objects
	t.Logf("Log files count: %d", len(result))

	// If files exist, verify structure
	if len(result) > 0 {
		firstFile := result[0].(map[string]interface{})
		// Verify expected fields: name, size, modified_at
		assert.Contains(t, firstFile, "name", "File should have name field")
		t.Logf("First log file: %v", firstFile)
	} else {
		t.Log("No log files found (acceptable if log directory empty)")
	}

	t.Log("✓ Log files listing test completed successfully")
}

// TestSystemLogs_GetContent tests log content retrieval
func TestSystemLogs_GetContent(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// First, get list of log files to know what to query
	t.Log("Getting log file list")
	resp, err := helper.GET("/api/system/logs/files")
	require.NoError(t, err)
	defer resp.Body.Close()

	var files []interface{}
	err = helper.ParseJSONResponse(resp, &files)
	require.NoError(t, err)

	if len(files) == 0 {
		t.Skip("No log files available to test content retrieval")
		return
	}

	// Get the first log file name
	firstFile := files[0].(map[string]interface{})
	filename, ok := firstFile["name"].(string)
	require.True(t, ok, "File should have name field")

	// Test 1: GET log content with limit
	t.Logf("Step 1: Getting log content for %s with limit=10", filename)
	resp, err = helper.GET(fmt.Sprintf("/api/system/logs/content?filename=%s&limit=10", filename))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Logf("⚠️  Log file %s not found (may have been rotated)", filename)
		t.Skip("Log file not available for content test")
		return
	}

	helper.AssertStatusCode(resp, http.StatusOK)

	var entries []interface{}
	err = helper.ParseJSONResponse(resp, &entries)
	require.NoError(t, err)
	t.Logf("Retrieved %d log entries", len(entries))

	// Verify limit is respected (should have max 10 entries)
	assert.LessOrEqual(t, len(entries), 10, "Should not exceed limit of 10 entries")

	// Test 2: GET log content with level filtering
	t.Logf("Step 2: Getting log content with levels filter")
	resp, err = helper.GET(fmt.Sprintf("/api/system/logs/content?filename=%s&limit=50&levels=ERROR,WARN", filename))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var filteredEntries []interface{}
	err = helper.ParseJSONResponse(resp, &filteredEntries)
	require.NoError(t, err)
	t.Logf("Retrieved %d filtered log entries (ERROR/WARN)", len(filteredEntries))

	// Test 3: GET log content without filename (should 400)
	t.Log("Step 3: Testing missing filename parameter")
	resp, err = helper.GET("/api/system/logs/content")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("✓ Log content retrieval test completed successfully")
}
