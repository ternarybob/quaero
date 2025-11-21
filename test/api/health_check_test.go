package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestHealthCheckWithBadger tests the health endpoint using Badger storage
// It verifies that the service starts correctly with Badger configuration
// and responds to health checks.
func TestHealthCheckWithBadger(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name(), "../config/test-quaero-badger.toml")
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Test health endpoint
	resp, err := helper.GET("/api/health")
	require.NoError(t, err, "Failed to call health endpoint")
	
	// Verify status code
	helper.AssertStatusCode(resp, http.StatusOK)

	// Verify response body
	var result map[string]string
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse health response")
	
	assert.Equal(t, "ok", result["status"], "Health status should be 'ok'")
	
	// Log success
	t.Logf("Health check passed with Badger storage")
	t.Logf("Service running on port %d", env.Port)
}