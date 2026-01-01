package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestKVDefaults verifies that all default KV values are seeded on startup.
// It checks that GET /api/kv/defaults returns the expected defaults,
// and that these defaults are present in the actual KV store.
func TestKVDefaults(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Get expected defaults from /api/kv/defaults
	resp, err := helper.GET("/api/kv/defaults")
	require.NoError(t, err, "Failed to call /api/kv/defaults endpoint")
	helper.AssertStatusCode(resp, http.StatusOK)

	var defaults []struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Description string `json:"description"`
	}
	err = helper.ParseJSONResponse(resp, &defaults)
	require.NoError(t, err, "Failed to parse defaults response")
	require.NotEmpty(t, defaults, "Defaults should not be empty")

	t.Logf("Found %d default KV values", len(defaults))

	// Verify each default exists in the KV store with correct value
	for _, d := range defaults {
		t.Run("Default_"+d.Key, func(t *testing.T) {
			// Get the actual value from KV store
			resp, err := helper.GET("/api/kv/" + d.Key)
			require.NoError(t, err, "Failed to get KV value for key: %s", d.Key)
			helper.AssertStatusCode(resp, http.StatusOK)

			var kvValue struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				Description string `json:"description"`
			}
			err = helper.ParseJSONResponse(resp, &kvValue)
			require.NoError(t, err, "Failed to parse KV value for key: %s", d.Key)

			// Assert the value matches the expected default
			assert.Equal(t, d.Key, kvValue.Key, "Key should match")
			assert.Equal(t, d.Value, kvValue.Value, "Value should match default for key: %s", d.Key)
			assert.Equal(t, d.Description, kvValue.Description, "Description should match for key: %s", d.Key)

			t.Logf("Verified default KV: %s = %s", d.Key, d.Value)
		})
	}
}
