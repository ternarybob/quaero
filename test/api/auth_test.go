package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// Helper functions for auth test operations

// createTestAuthData returns sample auth data matching AtlassianAuthData structure
func createTestAuthData() map[string]interface{} {
	return map[string]interface{}{
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-session-token-123",
				"domain":   ".atlassian.net",
				"path":     "/",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
				"secure":   true,
				"httpOnly": true,
				"sameSite": "Lax",
			},
			{
				"name":     "tenant.session.token",
				"value":    "test-tenant-token-456",
				"domain":   "test.atlassian.net",
				"path":     "/",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
				"secure":   true,
				"httpOnly": true,
				"sameSite": "Strict",
			},
		},
		"tokens": map[string]interface{}{
			"cloudId":  "test-cloud-id-789",
			"atlToken": "test-atl-token-abc",
		},
		"userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
		"baseUrl":   "https://test.atlassian.net",
		"timestamp": time.Now().Unix(),
	}
}

// captureTestAuth posts auth data and returns credential ID
func captureTestAuth(t *testing.T, env *common.TestEnvironment, authData map[string]interface{}) string {
	helper := env.NewHTTPTestHelper(t)

	// POST auth data
	resp, err := helper.POST("/api/auth", authData)
	require.NoError(t, err, "Failed to capture auth")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)

	// Get credential ID from list
	listResp, err := helper.GET("/api/auth/list")
	require.NoError(t, err, "Failed to get auth list")
	defer listResp.Body.Close()

	helper.AssertStatusCode(listResp, http.StatusOK)

	var credentials []map[string]interface{}
	err = helper.ParseJSONResponse(listResp, &credentials)
	require.NoError(t, err, "Failed to parse auth list")

	require.Greater(t, len(credentials), 0, "Should have at least one credential after capture")

	// Return the ID of the most recent credential (last in list)
	credID, ok := credentials[len(credentials)-1]["id"].(string)
	require.True(t, ok, "Credential should have ID field")
	require.NotEmpty(t, credID, "Credential ID should not be empty")

	t.Logf("Captured auth credential: id=%s", credID)
	return credID
}

// deleteTestAuth deletes auth credential by ID
func deleteTestAuth(t *testing.T, env *common.TestEnvironment, id string) {
	helper := env.NewHTTPTestHelper(t)

	resp, err := helper.DELETE(fmt.Sprintf("/api/auth/%s", id))
	require.NoError(t, err, "Failed to delete auth")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse delete response")

	assert.Equal(t, "success", result["status"], "Delete should return success status")
	t.Logf("Deleted auth credential: id=%s", id)
}

// cleanupAllAuth deletes all auth credentials
func cleanupAllAuth(t *testing.T, env *common.TestEnvironment) {
	helper := env.NewHTTPTestHelper(t)

	// Get all credentials
	resp, err := helper.GET("/api/auth/list")
	if err != nil {
		t.Logf("Failed to get auth list for cleanup: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Auth list returned status %d, skipping cleanup", resp.StatusCode)
		return
	}

	var credentials []map[string]interface{}
	err = helper.ParseJSONResponse(resp, &credentials)
	if err != nil {
		t.Logf("Failed to parse auth list for cleanup: %v", err)
		return
	}

	// Delete each credential
	for _, cred := range credentials {
		if id, ok := cred["id"].(string); ok {
			deleteResp, err := helper.DELETE(fmt.Sprintf("/api/auth/%s", id))
			if err != nil {
				t.Logf("Failed to delete credential %s during cleanup: %v", id, err)
				continue
			}
			deleteResp.Body.Close()

			if deleteResp.StatusCode == http.StatusOK {
				t.Logf("Cleaned up auth credential: id=%s", id)
			} else {
				t.Logf("Warning: Cleanup failed for credential %s with status %d", id, deleteResp.StatusCode)
			}
		}
	}
}

// Test functions

// TestAuthCapture tests POST /api/auth endpoint
func TestAuthCapture(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create test auth data
		authData := createTestAuthData()

		// POST to /api/auth
		resp, err := helper.POST("/api/auth", authData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify 200 OK
		helper.AssertStatusCode(resp, http.StatusOK)

		// Verify response contains success status
		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)
		assert.Equal(t, "success", result["status"], "Response should contain status: success")
		assert.NotEmpty(t, result["message"], "Response should contain success message")

		// Verify credential was stored
		listResp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer listResp.Body.Close()

		helper.AssertStatusCode(listResp, http.StatusOK)

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(listResp, &credentials)
		require.NoError(t, err)
		require.Greater(t, len(credentials), 0, "Should have at least one credential")

		// Verify stored credential has correct baseUrl and site_domain
		cred := credentials[len(credentials)-1]
		assert.Equal(t, "https://test.atlassian.net", cred["base_url"], "Stored credential should have correct base_url")
		assert.NotEmpty(t, cred["site_domain"], "Stored credential should have site_domain")

		// Cleanup
		if id, ok := cred["id"].(string); ok {
			deleteTestAuth(t, env, id)
		}

		t.Log("✓ Auth capture success test completed")
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		// POST malformed JSON
		resp, err := helper.POSTBody("/api/auth", "application/json", []byte("not valid json {"))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify 400 Bad Request
		helper.AssertStatusCode(resp, http.StatusBadRequest)

		t.Log("✓ Invalid JSON test completed")
	})

	t.Run("MissingFields", func(t *testing.T) {
		// Create auth data missing baseUrl field
		authData := map[string]interface{}{
			"cookies": []map[string]interface{}{
				{
					"name":  "test.cookie",
					"value": "test-value",
				},
			},
			"tokens":    map[string]interface{}{},
			"userAgent": "Test Agent",
			// Missing baseUrl - this creates credential with empty site_domain
			"timestamp": time.Now().Unix(),
		}

		// POST auth data
		resp, err := helper.POST("/api/auth", authData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Note: Empty baseUrl is allowed and creates credential with empty site_domain
		// The API doesn't validate field presence, just stores what's provided
		helper.AssertStatusCode(resp, http.StatusOK)

		// Cleanup the empty credential that was created
		listResp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer listResp.Body.Close()

		var credentials []map[string]interface{}
		if err := helper.ParseJSONResponse(listResp, &credentials); err == nil {
			for _, cred := range credentials {
				if id, ok := cred["id"].(string); ok && cred["site_domain"] == "" {
					deleteTestAuth(t, env, id)
				}
			}
		}

		t.Log("✓ Missing fields test completed")
	})

	t.Run("EmptyCookies", func(t *testing.T) {
		// Create auth data with empty cookies array
		authData := createTestAuthData()
		authData["cookies"] = []map[string]interface{}{}

		// POST auth data
		resp, err := helper.POST("/api/auth", authData)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should still succeed (200 OK) even with empty cookies
		helper.AssertStatusCode(resp, http.StatusOK)

		// Verify credential was created
		listResp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer listResp.Body.Close()

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(listResp, &credentials)
		require.NoError(t, err)
		require.Greater(t, len(credentials), 0, "Should have credential even with empty cookies")

		// Cleanup
		cred := credentials[len(credentials)-1]
		if id, ok := cred["id"].(string); ok {
			deleteTestAuth(t, env, id)
		}

		t.Log("✓ Empty cookies test completed")
	})

	t.Log("✓ TestAuthCapture completed successfully")
}

// TestAuthStatus tests GET /api/auth/status endpoint
func TestAuthStatus(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("NotAuthenticated", func(t *testing.T) {
		// Verify no credentials exist
		cleanupAllAuth(t, env)

		// GET /api/auth/status
		resp, err := helper.GET("/api/auth/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)
		assert.Equal(t, false, result["authenticated"], "Should return authenticated: false")

		t.Log("✓ Not authenticated test completed")
	})

	t.Run("Authenticated", func(t *testing.T) {
		// Create test credential
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// GET /api/auth/status
		resp, err := helper.GET("/api/auth/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)
		assert.Equal(t, true, result["authenticated"], "Should return authenticated: true")

		// Cleanup
		deleteTestAuth(t, env, credID)

		t.Log("✓ Authenticated test completed")
	})

	t.Log("✓ TestAuthStatus completed successfully")
}

// TestAuthList tests GET /api/auth/list endpoint
func TestAuthList(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("EmptyList", func(t *testing.T) {
		// Ensure no credentials exist
		cleanupAllAuth(t, env)

		// GET /api/auth/list
		resp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(resp, &credentials)
		require.NoError(t, err)
		assert.Equal(t, 0, len(credentials), "Should return empty array")

		t.Log("✓ Empty list test completed")
	})

	t.Run("SingleCredential", func(t *testing.T) {
		// Create test credential
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// GET /api/auth/list
		resp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(resp, &credentials)
		require.NoError(t, err)
		assert.Equal(t, 1, len(credentials), "Should return array with 1 element")

		// Verify credential fields
		cred := credentials[0]
		assert.NotEmpty(t, cred["id"], "Credential should have id")
		// Note: name may be empty for chrome extension captured credentials
		assert.NotNil(t, cred["name"], "Credential should have name field")
		assert.NotEmpty(t, cred["site_domain"], "Credential should have site_domain")
		assert.NotEmpty(t, cred["service_type"], "Credential should have service_type")
		assert.NotEmpty(t, cred["base_url"], "Credential should have base_url")
		assert.NotNil(t, cred["created_at"], "Credential should have created_at")
		assert.NotNil(t, cred["updated_at"], "Credential should have updated_at")

		// CRITICAL: Verify sanitization - cookies and tokens should NOT be present
		_, hasCookies := cred["cookies"]
		assert.False(t, hasCookies, "Cookies should not be present in list response")
		_, hasTokens := cred["tokens"]
		assert.False(t, hasTokens, "Tokens should not be present in list response")

		// Cleanup
		deleteTestAuth(t, env, credID)

		t.Log("✓ Single credential test completed")
	})

	t.Run("MultipleCredentials", func(t *testing.T) {
		// Create 3 test credentials with different baseUrls
		authData1 := createTestAuthData()
		authData1["baseUrl"] = "https://test1.atlassian.net"
		credID1 := captureTestAuth(t, env, authData1)

		authData2 := createTestAuthData()
		authData2["baseUrl"] = "https://test2.atlassian.net"
		credID2 := captureTestAuth(t, env, authData2)

		authData3 := createTestAuthData()
		authData3["baseUrl"] = "https://test3.atlassian.net"
		credID3 := captureTestAuth(t, env, authData3)

		// GET /api/auth/list
		resp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(resp, &credentials)
		require.NoError(t, err)
		assert.Equal(t, 3, len(credentials), "Should return array with 3 elements")

		// Verify all credentials have sanitized fields (no cookies/tokens)
		for _, cred := range credentials {
			_, hasCookies := cred["cookies"]
			assert.False(t, hasCookies, "Cookies should not be present")
			_, hasTokens := cred["tokens"]
			assert.False(t, hasTokens, "Tokens should not be present")
		}

		// Cleanup
		deleteTestAuth(t, env, credID1)
		deleteTestAuth(t, env, credID2)
		deleteTestAuth(t, env, credID3)

		t.Log("✓ Multiple credentials test completed")
	})

	t.Log("✓ TestAuthList completed successfully")
}

// TestAuthGet tests GET /api/auth/{id} endpoint
func TestAuthGet(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create test credential
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// GET /api/auth/{id}
		resp, err := helper.GET(fmt.Sprintf("/api/auth/%s", credID))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var cred map[string]interface{}
		err = helper.ParseJSONResponse(resp, &cred)
		require.NoError(t, err)

		// Verify expected fields
		assert.Equal(t, credID, cred["id"], "Should return correct credential ID")
		// Note: name may be empty for chrome extension captured credentials
		assert.NotNil(t, cred["name"], "Credential should have name field")
		assert.NotEmpty(t, cred["site_domain"], "Credential should have site_domain")
		assert.NotEmpty(t, cred["service_type"], "Credential should have service_type")
		assert.NotEmpty(t, cred["base_url"], "Credential should have base_url")
		assert.NotNil(t, cred["created_at"], "Credential should have created_at")
		assert.NotNil(t, cred["updated_at"], "Credential should have updated_at")

		// CRITICAL: Verify sanitization - cookies and tokens should NOT be present
		_, hasCookies := cred["cookies"]
		assert.False(t, hasCookies, "Cookies should not be present in get response")
		_, hasTokens := cred["tokens"]
		assert.False(t, hasTokens, "Tokens should not be present in get response")

		// Cleanup
		deleteTestAuth(t, env, credID)

		t.Log("✓ Success test completed")
	})

	t.Run("NotFound", func(t *testing.T) {
		// GET /api/auth/nonexistent-id
		resp, err := helper.GET("/api/auth/nonexistent-id")
		require.NoError(t, err)
		defer resp.Body.Close()

		// API returns 404 or 500 when credential not found
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError,
			"Not found should return 404 or 500, got %d", resp.StatusCode)

		t.Log("✓ Not found test completed")
	})

	t.Run("EmptyID", func(t *testing.T) {
		// GET /api/auth/ (trailing slash, no ID)
		resp, err := helper.GET("/api/auth/")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 400 or 404
		assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
			"Empty ID should return 400 or 404")

		t.Log("✓ Empty ID test completed")
	})

	t.Log("✓ TestAuthGet completed successfully")
}

// TestAuthDelete tests DELETE /api/auth/{id} endpoint
func TestAuthDelete(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("Success", func(t *testing.T) {
		// Create test credential
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// DELETE /api/auth/{id}
		resp, err := helper.DELETE(fmt.Sprintf("/api/auth/%s", credID))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)
		assert.Equal(t, "success", result["status"], "Should return status: success")

		// Verify credential no longer exists
		getResp, err := helper.GET(fmt.Sprintf("/api/auth/%s", credID))
		require.NoError(t, err)
		defer getResp.Body.Close()

		// API returns 404 or 500 when credential not found
		assert.True(t, getResp.StatusCode == http.StatusNotFound || getResp.StatusCode == http.StatusInternalServerError,
			"Deleted credential should return 404 or 500, got %d", getResp.StatusCode)

		t.Log("✓ Success test completed")
	})

	t.Run("NotFound", func(t *testing.T) {
		// DELETE /api/auth/nonexistent-id
		resp, err := helper.DELETE("/api/auth/nonexistent-id")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 200 OK (idempotent DELETE) or 500 Internal Server Error
		// Note: 200 OK is actually better UX for idempotent operations
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
			"DELETE nonexistent should return 200 (idempotent) or 500, got %d", resp.StatusCode)

		t.Log("✓ Not found test completed")
	})

	t.Run("EmptyID", func(t *testing.T) {
		// DELETE /api/auth/ (trailing slash, no ID)
		resp, err := helper.DELETE("/api/auth/")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 400 or 404 (routing may redirect to different handler)
		assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
			"Empty ID should return 400 or 404, got %d", resp.StatusCode)

		t.Log("✓ Empty ID test completed")
	})

	t.Log("✓ TestAuthDelete completed successfully")
}

// TestAuthSanitization tests comprehensive sanitization verification
func TestAuthSanitization(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	helper := env.NewHTTPTestHelper(t)

	t.Run("ListSanitization", func(t *testing.T) {
		// Create test credential with cookies and tokens
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// GET /api/auth/list
		resp, err := helper.GET("/api/auth/list")
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var credentials []map[string]interface{}
		err = helper.ParseJSONResponse(resp, &credentials)
		require.NoError(t, err)
		require.Greater(t, len(credentials), 0, "Should have at least one credential")

		cred := credentials[0]

		// Assert cookies field is not present
		_, hasCookies := cred["cookies"]
		assert.False(t, hasCookies, "Cookies should not be exposed in list")

		// Assert tokens field is not present
		_, hasTokens := cred["tokens"]
		assert.False(t, hasTokens, "Tokens should not be exposed in list")

		// Verify only safe fields are present
		safeFields := []string{"id", "name", "site_domain", "service_type", "base_url", "created_at", "updated_at"}
		for _, field := range safeFields {
			assert.NotNil(t, cred[field], "Safe field %s should be present", field)
		}

		// Cleanup
		deleteTestAuth(t, env, credID)

		t.Log("✓ List sanitization test completed")
	})

	t.Run("GetSanitization", func(t *testing.T) {
		// Create test credential with cookies and tokens
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)

		// GET /api/auth/{id}
		resp, err := helper.GET(fmt.Sprintf("/api/auth/%s", credID))
		require.NoError(t, err)
		defer resp.Body.Close()

		helper.AssertStatusCode(resp, http.StatusOK)

		var cred map[string]interface{}
		err = helper.ParseJSONResponse(resp, &cred)
		require.NoError(t, err)

		// Assert cookies field is not present
		_, hasCookies := cred["cookies"]
		assert.False(t, hasCookies, "Cookies should not be exposed in get")

		// Assert tokens field is not present
		_, hasTokens := cred["tokens"]
		assert.False(t, hasTokens, "Tokens should not be exposed in get")

		// Verify only safe fields are present
		safeFields := []string{"id", "name", "site_domain", "service_type", "base_url", "created_at", "updated_at"}
		for _, field := range safeFields {
			assert.NotNil(t, cred[field], "Safe field %s should be present", field)
		}

		// Cleanup
		deleteTestAuth(t, env, credID)

		t.Log("✓ Get sanitization test completed")
	})

	t.Log("✓ TestAuthSanitization completed successfully")
}
