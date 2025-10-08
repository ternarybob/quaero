// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:42:54 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

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
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
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
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
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
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
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
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
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
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
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

// TestEmbeddingAPI_MultipleSequentialEmbeddings tests generating embeddings in sequence
// This verifies the llama-server can handle multiple consecutive requests
func TestEmbeddingAPI_MultipleSequentialEmbeddings(t *testing.T) {
	t.Log("=== Testing Embedding API - Multiple Sequential Embeddings ===")

	// Step 1: Initialize application
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application.EmbeddingHandler, "EmbeddingHandler should be initialized")
	t.Log("✓ Application initialized")

	// Step 2: Generate multiple embeddings sequentially
	testTexts := []string{
		"First test document for embedding",
		"Second test document with different content",
		"Third test document to verify consistency",
		"Fourth test document for load testing",
		"Fifth and final test document",
	}

	embeddings := make([][]float32, 0, len(testTexts))

	for i, text := range testTexts {
		requestBody := handlers.EmbedRequest{
			Text: text,
		}
		bodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err, "Failed to marshal request %d", i+1)

		req := httptest.NewRequest(http.MethodPost, "/api/embeddings/generate", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		application.EmbeddingHandler.GenerateEmbeddingHandler(w, req)
		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Request %d should succeed", i+1)

		var response handlers.EmbedResponse
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err, "Failed to decode response %d", i+1)

		assert.True(t, response.Success, "Request %d should indicate success", i+1)
		assert.NotNil(t, response.Embedding, "Embedding %d should not be nil", i+1)
		assert.Greater(t, response.Dimension, 0, "Dimension %d should be greater than 0", i+1)

		embeddings = append(embeddings, response.Embedding)
		t.Logf("✓ Generated embedding %d/%d: dimension=%d", i+1, len(testTexts), response.Dimension)
	}

	// Step 3: Verify all embeddings have consistent dimensions
	firstDimension := len(embeddings[0])
	for i, emb := range embeddings {
		assert.Equal(t, firstDimension, len(emb), "Embedding %d should have same dimension as others", i+1)
	}

	// Step 4: Verify different texts produce different embeddings
	for i := 0; i < len(embeddings)-1; i++ {
		isSame := true
		for j := 0; j < len(embeddings[i]); j++ {
			if embeddings[i][j] != embeddings[i+1][j] {
				isSame = false
				break
			}
		}
		assert.False(t, isSame, "Embedding %d should differ from embedding %d", i+1, i+2)
	}

	t.Logf("✓ All %d embeddings generated with consistent dimensions", len(embeddings))
	t.Log("✓ Each embedding is unique (different texts produce different vectors)")
	t.Log("✅ SUCCESS: Multiple sequential embeddings work correctly")
}
