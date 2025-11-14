package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// TestResolveAPIKey_KVStore tests that API keys are resolved from KV store
func TestResolveAPIKey_KVStore(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Insert API key in KV store
	err = manager.KeyValueStorage().Set(ctx, "test-service", "kv-store-key", "KV store value")
	require.NoError(t, err)

	// Resolve API key - should return KV store value
	apiKey, err := common.ResolveAPIKey(ctx, manager.KeyValueStorage(), "test-service", "")
	require.NoError(t, err)
	assert.Equal(t, "kv-store-key", apiKey, "Should resolve API key from KV store")
}

// TestResolveAPIKey_ConfigFallback tests fallback to config value when KV store doesn't have the key
func TestResolveAPIKey_ConfigFallback(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// KV store does not contain the key

	// Resolve API key with config fallback
	apiKey, err := common.ResolveAPIKey(ctx, manager.KeyValueStorage(), "test-service", "config-fallback-key")
	require.NoError(t, err)
	assert.Equal(t, "config-fallback-key", apiKey, "Should fall back to config value when KV store doesn't have the key")
}

// TestResolveAPIKey_NoValueFound tests error when no value is found in any source
func TestResolveAPIKey_NoValueFound(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Neither KV store nor config have the key

	// Resolve API key - should return error
	_, err = common.ResolveAPIKey(ctx, manager.KeyValueStorage(), "nonexistent-service", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found", "Should return error when no value is found")
}

// TestResolveAPIKey_NilKVStorage tests that nil KV storage falls back to config
func TestResolveAPIKey_NilKVStorage(t *testing.T) {
	ctx := context.Background()

	// Pass nil KV storage with config fallback
	apiKey, err := common.ResolveAPIKey(ctx, nil, "test-service", "config-fallback-key")
	require.NoError(t, err)
	assert.Equal(t, "config-fallback-key", apiKey, "Should fall back to config when KV storage is nil")
}

// TestResolveAPIKey_EmptyConfig tests error when config fallback is also empty
func TestResolveAPIKey_EmptyConfig(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// KV store doesn't have key and config is empty

	// Resolve should fail
	_, err = common.ResolveAPIKey(ctx, manager.KeyValueStorage(), "test-service", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestResolveAPIKey_KVPrecedence tests that KV store takes precedence over config
func TestResolveAPIKey_KVPrecedence(t *testing.T) {
	logger := arbor.NewLogger()
	config := &common.SQLiteConfig{Path: ":memory:"}
	manager, err := NewManager(logger, config)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Insert API key in KV store
	err = manager.KeyValueStorage().Set(ctx, "test-service", "kv-value", "KV store value")
	require.NoError(t, err)

	// Resolve with both KV and config - KV should win
	apiKey, err := common.ResolveAPIKey(ctx, manager.KeyValueStorage(), "test-service", "config-value")
	require.NoError(t, err)
	assert.Equal(t, "kv-value", apiKey, "KV store should take precedence over config")
}
