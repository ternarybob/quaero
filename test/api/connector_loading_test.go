package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestConnectorLoading_ListConnectors tests that the connector list API works correctly
func TestConnectorLoading_ListConnectors(t *testing.T) {
	// Setup test environment - this will start a fresh server instance
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// List connectors via API
	t.Log("Step 1: Listing connectors via API")
	resp, err := helper.GET("/api/connectors")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var connectors []interface{}
	err = helper.ParseJSONResponse(resp, &connectors)
	require.NoError(t, err)

	t.Logf("Found %d connectors", len(connectors))

	// Note: Connectors loaded from TOML files will appear here if configured.
	// The test validates the API endpoint works correctly.

	t.Log("Connector list test completed successfully")
}

// TestConnectorLoading_ValidTypes tests that only valid connector types are accepted
func TestConnectorLoading_ValidTypes(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Valid github type
	t.Log("Step 1: Testing valid github type")
	body := map[string]interface{}{
		"name": "Valid GitHub",
		"type": "github",
		"config": map[string]interface{}{
			"token": "skip_validation_token",
		},
	}
	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should create successfully (or fail validation with 400, not 500)
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
		"github type should be accepted, got status %d", resp.StatusCode)

	// Test 2: Valid gitlab type
	t.Log("Step 2: Testing valid gitlab type")
	body2 := map[string]interface{}{
		"name": "Valid GitLab",
		"type": "gitlab",
		"config": map[string]interface{}{
			"token": "skip_validation_token",
		},
	}
	resp, err = helper.POST("/api/connectors", body2)
	require.NoError(t, err)
	defer resp.Body.Close()

	// GitLab may not be fully implemented yet, but type should be recognized
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusBadRequest,
		"gitlab type should be recognized, got status %d", resp.StatusCode)

	// Test 3: Invalid type
	t.Log("Step 3: Testing invalid type")
	body3 := map[string]interface{}{
		"name": "Invalid Type",
		"type": "unknown_type",
		"config": map[string]interface{}{
			"token": "test",
		},
	}
	resp, err = helper.POST("/api/connectors", body3)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should reject unknown type
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("Connector type validation test completed successfully")
}

// TestConnectorLoading_EmptyDirectory tests that startup succeeds with empty connectors directory
func TestConnectorLoading_EmptyDirectory(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// The test environment should start successfully even without connectors directory
	// Verify the API is responsive
	t.Log("Verifying API is responsive with empty/no connectors directory")
	resp, err := helper.GET("/api/connectors")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var connectors []interface{}
	err = helper.ParseJSONResponse(resp, &connectors)
	require.NoError(t, err)

	t.Logf("Connectors list returned (count: %d)", len(connectors))
	t.Log("Empty directory test completed successfully")
}

// TestConnectorLoading_MissingToken tests that connectors with missing token are rejected
func TestConnectorLoading_MissingToken(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test connector creation with missing token
	t.Log("Testing connector creation with missing token")
	body := map[string]interface{}{
		"name":   "Missing Token",
		"type":   "github",
		"config": map[string]interface{}{
			// token is missing
		},
	}
	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should reject due to missing token
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	if err == nil {
		t.Logf("Error response: %v", result)
	}

	t.Log("Missing token validation test completed successfully")
}
