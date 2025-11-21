package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/kv"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

// setupTestDB creates a test database for KV testing
func setupTestDB(t *testing.T) (*sqlite.SQLiteDB, func()) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	config := &common.SQLiteConfig{
		Path:          dbPath,
		EnableFTS5:    false,
		CacheSizeMB:   10,
		WALMode:       false,
		BusyTimeoutMS: 5000,
	}

	logger := arbor.NewLogger()
	db, err := sqlite.NewSQLiteDB(logger, config)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// TestKVCaseInsensitiveStorage tests that keys are case-insensitive at the storage layer
func TestKVCaseInsensitiveStorage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)

	// Test 1: Set with uppercase, get with lowercase
	err := kvStorage.Set(ctx, "GOOGLE_API_KEY", "test-value-123", "Test API key")
	require.NoError(t, err)

	value, err := kvStorage.Get(ctx, "google_api_key")
	require.NoError(t, err)
	assert.Equal(t, "test-value-123", value)

	// Test 2: Set with lowercase, get with uppercase
	err = kvStorage.Set(ctx, "github_token", "token-456", "GitHub token")
	require.NoError(t, err)

	value, err = kvStorage.Get(ctx, "GITHUB_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "token-456", value)

	// Test 3: Update with different case should update same record
	err = kvStorage.Set(ctx, "GitHub_Token", "token-789", "Updated token")
	require.NoError(t, err)

	value, err = kvStorage.Get(ctx, "github_token")
	require.NoError(t, err)
	assert.Equal(t, "token-789", value)

	// Test 4: Verify only one record exists (not three)
	pairs, err := kvStorage.List(ctx)
	require.NoError(t, err)
	assert.Len(t, pairs, 2, "Should have 2 keys (google_api_key and github_token)")
}

// TestKVUpsertBehavior tests the explicit upsert method
func TestKVUpsertBehavior(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)

	// Test 1: Upsert new key returns true
	isNew, err := kvStorage.Upsert(ctx, "NEW_KEY", "value-1", "New key")
	require.NoError(t, err)
	assert.True(t, isNew, "Expected isNew=true for new key")

	// Test 2: Upsert existing key returns false
	isNew, err = kvStorage.Upsert(ctx, "new_key", "value-2", "Updated key")
	require.NoError(t, err)
	assert.False(t, isNew, "Expected isNew=false for existing key")

	// Test 3: Verify value was updated
	value, err := kvStorage.Get(ctx, "NEW_KEY")
	require.NoError(t, err)
	assert.Equal(t, "value-2", value)
}

// TestKVDeleteCaseInsensitive tests that delete works with different cases
func TestKVDeleteCaseInsensitive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)

	// Create key with uppercase
	err := kvStorage.Set(ctx, "DELETE_ME", "value", "Test delete")
	require.NoError(t, err)

	// Delete with lowercase
	err = kvStorage.Delete(ctx, "delete_me")
	require.NoError(t, err)

	// Verify key is gone
	_, err = kvStorage.Get(ctx, "DELETE_ME")
	assert.ErrorIs(t, err, interfaces.ErrKeyNotFound)
}

// TestKVAPIEndpointCaseInsensitive tests the HTTP API endpoints with case variations
func TestKVAPIEndpointCaseInsensitive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)
	kvService := kv.NewService(kvStorage, nil, logger) // nil event service for test
	handler := handlers.NewKVHandler(kvService, logger)

	// Test 1: Create key with POST (uppercase)
	createReq := map[string]string{
		"key":         "API_KEY",
		"value":       "secret-123",
		"description": "Test API key",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/kv", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateKVHandler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "Response: %s", w.Body.String())

	// Test 2: GET key with lowercase
	req = httptest.NewRequest("GET", "/api/kv/api_key", nil)
	w = httptest.NewRecorder()

	handler.GetKVHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response: %s", w.Body.String())

	var getResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&getResp)
	require.NoError(t, err)
	assert.Equal(t, "secret-123", getResp["value"])

	// Test 3: PUT update with mixed case
	updateReq := map[string]string{
		"value":       "secret-456",
		"description": "Updated key",
	}
	body, _ = json.Marshal(updateReq)
	req = httptest.NewRequest("PUT", "/api/kv/Api_Key", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.UpdateKVHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Should be 200 for update. Response: %s", w.Body.String())

	var updateResp map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&updateResp)
	require.NoError(t, err)
	created, ok := updateResp["created"].(bool)
	assert.True(t, ok, "created field should exist")
	assert.False(t, created, "Expected created=false for update")

	// Test 4: Verify updated value
	req = httptest.NewRequest("GET", "/api/kv/API_KEY", nil)
	w = httptest.NewRecorder()

	handler.GetKVHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.NewDecoder(w.Body).Decode(&getResp)
	require.NoError(t, err)
	assert.Equal(t, "secret-456", getResp["value"])

	// Test 5: DELETE with different case
	req = httptest.NewRequest("DELETE", "/api/kv/api_key", nil)
	w = httptest.NewRecorder()

	handler.DeleteKVHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test 6: Verify key is deleted
	req = httptest.NewRequest("GET", "/api/kv/API_KEY", nil)
	w = httptest.NewRecorder()

	handler.GetKVHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestKVUpsertEndpoint tests the PUT endpoint upsert behavior
func TestKVUpsertEndpoint(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)
	kvService := kv.NewService(kvStorage, nil, logger)
	handler := handlers.NewKVHandler(kvService, logger)

	// Test 1: PUT new key returns 201 Created
	createReq := map[string]string{
		"value":       "new-value",
		"description": "New key via PUT",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("PUT", "/api/kv/NEW_KEY", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateKVHandler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "Expected 201 for new key. Response: %s", w.Body.String())

	var createResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&createResp)
	require.NoError(t, err)
	created, ok := createResp["created"].(bool)
	assert.True(t, ok, "created field should exist")
	assert.True(t, created, "Expected created=true for new key")

	// Test 2: PUT existing key returns 200 OK
	updateReq := map[string]string{
		"value":       "updated-value",
		"description": "Updated via PUT",
	}
	body, _ = json.Marshal(updateReq)
	req = httptest.NewRequest("PUT", "/api/kv/new_key", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.UpdateKVHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 for existing key. Response: %s", w.Body.String())

	var updateResp map[string]interface{}
	err2 := json.NewDecoder(w.Body).Decode(&updateResp)
	require.NoError(t, err2)
	updated, ok2 := updateResp["created"].(bool)
	assert.True(t, ok2, "created field should exist")
	assert.False(t, updated, "Expected created=false for existing key")
}

// TestKVDuplicateKeyValidation tests that POST /api/kv returns 409 for duplicate keys
func TestKVDuplicateKeyValidation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()

	kvStorage := sqlite.NewKVStorage(db, logger)
	kvService := kv.NewService(kvStorage, nil, logger)
	handler := handlers.NewKVHandler(kvService, logger)

	// Test 1: Create initial key with POST
	createReq := map[string]string{
		"key":         "TEST_KEY",
		"value":       "test-value-123",
		"description": "Original test key",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest("POST", "/api/kv", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateKVHandler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "Expected 201 for first key. Response: %s", w.Body.String())

	// Test 2: Try to create duplicate with same case - should return 409
	duplicateReq := map[string]string{
		"key":         "TEST_KEY",
		"value":       "different-value",
		"description": "Duplicate key attempt",
	}
	body, _ = json.Marshal(duplicateReq)
	req = httptest.NewRequest("POST", "/api/kv", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.CreateKVHandler(w, req)

	assert.Equal(t, http.StatusConflict, w.Code, "Expected 409 Conflict for duplicate key. Response: %s", w.Body.String())

	var errorResp map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&errorResp)
	require.NoError(t, err)
	errorMsg, ok := errorResp["error"].(string)
	assert.True(t, ok, "Expected error field in response")
	assert.Contains(t, errorMsg, "already exists", "Error message should mention key already exists")
	// Note: Current implementation normalizes keys to lowercase, so error shows "test_key" not "TEST_KEY"
	assert.Contains(t, errorMsg, "test_key", "Error message should show existing key name")

	// Test 3: Try to create duplicate with different case - should return 409
	caseInsensitiveDuplicateReq := map[string]string{
		"key":         "test_key", // lowercase version
		"value":       "another-value",
		"description": "Case-insensitive duplicate attempt",
	}
	body, _ = json.Marshal(caseInsensitiveDuplicateReq)
	req = httptest.NewRequest("POST", "/api/kv", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.CreateKVHandler(w, req)

	assert.Equal(t, http.StatusConflict, w.Code, "Expected 409 Conflict for case-insensitive duplicate. Response: %s", w.Body.String())

	var errorResp2 map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&errorResp2)
	require.NoError(t, err)
	errorMsg2, ok := errorResp2["error"].(string)
	assert.True(t, ok, "Expected error field in response")
	assert.Contains(t, errorMsg2, "already exists", "Error message should mention key already exists")
	// Note: Current implementation normalizes keys to lowercase
	assert.Contains(t, errorMsg2, "test_key", "Error message should show existing key name")

	// Test 4: Verify only one key exists in storage
	pairs, err := kvStorage.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, pairs, 1, "Should only have 1 key")
	// Note: Current implementation normalizes keys to lowercase during storage
	assert.Equal(t, "test_key", pairs[0].Key, "Key should be normalized to lowercase")
	assert.Equal(t, "test-value-123", pairs[0].Value, "Value should be original value")
}
