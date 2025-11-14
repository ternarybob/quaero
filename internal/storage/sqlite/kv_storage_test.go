package sqlite

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// setupTestDB creates a test database and returns cleanup function
func setupKVTestDB(t *testing.T) (*SQLiteDB, func()) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	config := &common.SQLiteConfig{
		Path:          dbPath,
		EnableFTS5:    false,
		EnableVector:  false,
		CacheSizeMB:   10,
		WALMode:       false,
		BusyTimeoutMS: 5000,
	}

	logger := arbor.NewLogger()
	db, err := NewSQLiteDB(logger, config)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func TestKVStorage_SetAndGet(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Set a key/value pair with description
	err := storage.Set(ctx, "test-key", "test-value", "A test key")
	require.NoError(t, err)

	// Retrieve it and verify
	value, err := storage.Get(ctx, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "test-value", value)

	// Verify through List to check timestamps
	pairs, err := storage.List(ctx)
	require.NoError(t, err)
	require.Len(t, pairs, 1)

	assert.Equal(t, "test-key", pairs[0].Key)
	assert.Equal(t, "test-value", pairs[0].Value)
	assert.Equal(t, "A test key", pairs[0].Description)
	assert.False(t, pairs[0].CreatedAt.IsZero())
	assert.False(t, pairs[0].UpdatedAt.IsZero())
}

func TestKVStorage_SetUpdate(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Set initial value
	err := storage.Set(ctx, "update-key", "initial-value", "Initial description")
	require.NoError(t, err)

	// Get initial timestamps
	pairs, err := storage.List(ctx)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	initialCreatedAt := pairs[0].CreatedAt
	initialUpdatedAt := pairs[0].UpdatedAt

	// Wait a full second to ensure timestamp difference
	time.Sleep(1100 * time.Millisecond)

	// Update the same key
	err = storage.Set(ctx, "update-key", "updated-value", "Updated description")
	require.NoError(t, err)

	// Verify update
	value, err := storage.Get(ctx, "update-key")
	require.NoError(t, err)
	assert.Equal(t, "updated-value", value)

	// Verify timestamps
	pairs, err = storage.List(ctx)
	require.NoError(t, err)
	require.Len(t, pairs, 1)

	assert.Equal(t, "updated-value", pairs[0].Value)
	assert.Equal(t, "Updated description", pairs[0].Description)
	assert.Equal(t, initialCreatedAt.Unix(), pairs[0].CreatedAt.Unix(), "created_at should not change")
	assert.Greater(t, pairs[0].UpdatedAt.Unix(), initialUpdatedAt.Unix(), "updated_at should be newer")
}

func TestKVStorage_GetNotFound(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Attempt to get non-existent key
	_, err := storage.Get(ctx, "non-existent-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent-key")
	assert.Contains(t, err.Error(), "not found")
}

func TestKVStorage_Delete(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Set a key
	err := storage.Set(ctx, "delete-key", "delete-value", "")
	require.NoError(t, err)

	// Delete it
	err = storage.Delete(ctx, "delete-key")
	require.NoError(t, err)

	// Verify it's gone
	_, err = storage.Get(ctx, "delete-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify Delete on non-existent key returns error
	err = storage.Delete(ctx, "delete-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestKVStorage_List(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Set multiple key/value pairs with delays to ensure different Unix timestamps
	err := storage.Set(ctx, "key1", "value1", "First key")
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond)

	err = storage.Set(ctx, "key2", "value2", "Second key")
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond)

	err = storage.Set(ctx, "key3", "value3", "Third key")
	require.NoError(t, err)

	// List all pairs
	pairs, err := storage.List(ctx)
	require.NoError(t, err)
	require.Len(t, pairs, 3)

	// Verify ordering by updated_at DESC (most recent first)
	// key3 should be first (most recent), then key2, then key1
	assert.Equal(t, "key3", pairs[0].Key, "Most recent key should be first")
	assert.Equal(t, "key2", pairs[1].Key, "Second most recent key should be second")
	assert.Equal(t, "key1", pairs[2].Key, "Oldest key should be last")

	// Verify all fields are populated
	for _, pair := range pairs {
		assert.NotEmpty(t, pair.Key)
		assert.NotEmpty(t, pair.Value)
		assert.NotEmpty(t, pair.Description)
		assert.False(t, pair.CreatedAt.IsZero())
		assert.False(t, pair.UpdatedAt.IsZero())
	}
}

func TestKVStorage_GetAll(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Set multiple key/value pairs
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		err := storage.Set(ctx, key, value, "")
		require.NoError(t, err)
	}

	// GetAll and verify
	kvMap, err := storage.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, kvMap, 3)

	// Verify all keys and values match
	for key, expectedValue := range testData {
		actualValue, exists := kvMap[key]
		assert.True(t, exists, "Key %s should exist", key)
		assert.Equal(t, expectedValue, actualValue)
	}
}

func TestKVStorage_EmptyList(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// List on empty database
	pairs, err := storage.List(ctx)
	require.NoError(t, err)
	assert.NotNil(t, pairs, "Should return empty slice, not nil")
	assert.Len(t, pairs, 0)
}

func TestKVStorage_EmptyGetAll(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// GetAll on empty database
	kvMap, err := storage.GetAll(ctx)
	require.NoError(t, err)
	assert.NotNil(t, kvMap, "Should return empty map, not nil")
	assert.Len(t, kvMap, 0)
}

func TestKVStorage_ConcurrentWrites(t *testing.T) {
	db, cleanup := setupKVTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewKVStorage(db, logger)
	ctx := context.Background()

	// Use goroutines to write multiple keys concurrently
	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			key := "concurrent-key-" + string(rune('0'+index))
			value := "concurrent-value-" + string(rune('0'+index))
			err := storage.Set(ctx, key, value, "")
			assert.NoError(t, err, "Concurrent write should succeed")
		}(i)
	}

	wg.Wait()

	// Verify all keys stored successfully (mutex prevents SQLITE_BUSY)
	kvMap, err := storage.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, kvMap, numGoroutines, "All concurrent writes should succeed")
}
