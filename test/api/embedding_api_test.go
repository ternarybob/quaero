package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
)

// TestEmbeddingAPI_GenerateEmbedding tests the /api/embeddings/generate endpoint
func TestEmbeddingAPI_GenerateEmbedding(t *testing.T) {
	t.Log("=== Testing Embedding API - Generate Embedding ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")
	t.Logf("✓ Configuration loaded from: %s", configPath)

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")
	t.Log("✓ Application initialized")

	// Step 2: Create test request
	requestBody := handlers.EmbedRequest{
		Text: "This is a test embedding request",
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err, "Failed to marshal request body")

	req := httptest.NewRequest(http.MethodPost, "/api/embeddings/generate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	t.Log("✓ Test request created")

	// Step 3: Call handler
	application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Step 4: Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response handlers.EmbedResponse
	err = json.Unmarshal(responseBody, &response)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.True(t, response.Success, "Response should indicate success")
	assert.Empty(t, response.Error, "Error should be empty")
	assert.NotNil(t, response.Embedding, "Embedding should not be nil")
	assert.Greater(t, response.Dimension, 0, "Dimension should be greater than 0")
	assert.Equal(t, len(response.Embedding), response.Dimension, "Embedding length should match dimension")

	t.Logf("✓ Embedding generated: dimension=%d", response.Dimension)
	t.Log("✅ SUCCESS: Embedding API test passed")
}

// TestEmbeddingAPI_EmptyText tests error handling for empty text
func TestEmbeddingAPI_EmptyText(t *testing.T) {
	t.Log("=== Testing Embedding API - Empty Text ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")

	// Step 2: Create test request with empty text
	requestBody := handlers.EmbedRequest{
		Text: "",
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err, "Failed to marshal request body")

	req := httptest.NewRequest(http.MethodPost, "/api/embeddings/generate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Step 3: Call handler
	application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Step 4: Verify response
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request")

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response handlers.EmbedResponse
	err = json.Unmarshal(responseBody, &response)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.False(t, response.Success, "Response should indicate failure")
	assert.NotEmpty(t, response.Error, "Error message should be present")
	assert.Contains(t, response.Error, "required", "Error should mention required field")

	t.Log("✅ SUCCESS: Empty text validation works correctly")
}

// TestEmbeddingAPI_InvalidMethod tests error handling for wrong HTTP method
func TestEmbeddingAPI_InvalidMethod(t *testing.T) {
	t.Log("=== Testing Embedding API - Invalid Method ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")

	// Step 2: Create GET request (should be POST)
	req := httptest.NewRequest(http.MethodGet, "/api/embeddings/generate", nil)
	w := httptest.NewRecorder()

	// Step 3: Call handler
	application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Step 4: Verify response
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "Should return 405 Method Not Allowed")

	t.Log("✅ SUCCESS: HTTP method validation works correctly")
}

// TestEmbeddingAPI_InvalidJSON tests error handling for invalid JSON
func TestEmbeddingAPI_InvalidJSON(t *testing.T) {
	t.Log("=== Testing Embedding API - Invalid JSON ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")

	// Step 2: Create request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/api/embeddings/generate", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Step 3: Call handler
	application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Step 4: Verify response
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request")

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response handlers.EmbedResponse
	err = json.Unmarshal(responseBody, &response)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.False(t, response.Success, "Response should indicate failure")
	assert.NotEmpty(t, response.Error, "Error message should be present")

	t.Log("✅ SUCCESS: Invalid JSON handling works correctly")
}

// TestEmbeddingAPI_LongText tests embedding generation with longer text
func TestEmbeddingAPI_LongText(t *testing.T) {
	t.Log("=== Testing Embedding API - Long Text ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")

	// Step 2: Create request with long text
	longText := ""
	for i := 0; i < 100; i++ {
		longText += "This is a longer test text to verify embedding generation with substantial content. "
	}

	requestBody := handlers.EmbedRequest{
		Text: longText,
	}
	bodyBytes, err := json.Marshal(requestBody)
	require.NoError(t, err, "Failed to marshal request body")

	req := httptest.NewRequest(http.MethodPost, "/api/embeddings/generate", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Step 3: Call handler
	application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
	resp := w.Result()
	defer resp.Body.Close()

	// Step 4: Verify response
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")

	var response handlers.EmbedResponse
	err = json.Unmarshal(responseBody, &response)
	require.NoError(t, err, "Failed to unmarshal response")

	assert.True(t, response.Success, "Response should indicate success")
	assert.NotNil(t, response.Embedding, "Embedding should not be nil")
	assert.Greater(t, response.Dimension, 0, "Dimension should be greater than 0")

	t.Logf("✓ Long text embedded: text_length=%d, dimension=%d", len(longText), response.Dimension)
	t.Log("✅ SUCCESS: Long text embedding works correctly")
}
