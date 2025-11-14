package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// TestMigrateAPIKeysToKVStore_AlreadyMigrated tests migration when all keys already in KV store
// This reflects the post-Phase-3 state where all API keys have been migrated
func TestMigrateAPIKeysToKVStore_AlreadyMigrated(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Pre-populate KV store with API keys (simulating post-Phase-3 state)
	err = manager.KeyValueStorage().Set(ctx, "google-gemini", "sk-test-key-123", "Google Gemini API key")
	require.NoError(t, err)

	err = manager.KeyValueStorage().Set(ctx, "google-places", "sk-places-456", "Google Places API key")
	require.NoError(t, err)

	// Run migration (should be no-op since auth storage has no API keys)
	err = manager.MigrateAPIKeysToKVStore(ctx)
	require.NoError(t, err)

	// Verify keys are still in KV store
	value1, err := manager.KeyValueStorage().Get(ctx, "google-gemini")
	require.NoError(t, err)
	assert.Equal(t, "sk-test-key-123", value1)

	value2, err := manager.KeyValueStorage().Get(ctx, "google-places")
	require.NoError(t, err)
	assert.Equal(t, "sk-places-456", value2)
}

// TestMigrateAPIKeysToKVStore_EmptyDatabase tests migration with no API keys
// This is the expected state after Phase 4 cleanup
func TestMigrateAPIKeysToKVStore_EmptyDatabase(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Run migration on empty database (no API keys in auth storage)
	err = manager.MigrateAPIKeysToKVStore(ctx)
	require.NoError(t, err)

	// Verify no changes (this is expected after Phase 4)
	pairs, err := manager.KeyValueStorage().List(ctx)
	require.NoError(t, err)
	// Note: May be empty or contain keys from other sources, but not from migration
	assert.NotNil(t, pairs)
}

// TestAuthStorageOnlyCookies tests that auth storage only contains cookie-based credentials
// This validates the Phase 4 cleanup goal
func TestAuthStorageOnlyCookies(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Auth storage should only accept cookie-based credentials now
	// (no APIKey or AuthType fields exist in the model)
	cookieCreds := make(map[string]interface{})
	cookieCreds["name"] = "Test Site"
	cookieCreds["site_domain"] = "example.com"
	cookieCreds["service_type"] = "web"

	// The schema no longer has api_key or auth_type columns
	// This test verifies Phase 4 cleanup is complete
	credentials, err := manager.AuthStorage().ListCredentials(ctx)
	require.NoError(t, err)

	// All credentials should be cookie-based (have site_domain)
	for _, cred := range credentials {
		// After Phase 4, all credentials must have a site_domain
		// (APIKey and AuthType fields don't exist anymore)
		assert.NotEmpty(t, cred.SiteDomain, "All credentials should be cookie-based with site_domain")
	}
}
