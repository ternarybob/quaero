package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/server"
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

// setupTestApp creates a test application instance
func setupTestApp(t *testing.T) (*app.App, func()) {
	// Create test configuration
	config := common.NewDefaultConfig()
	config.Storage.SQLite.Path = ":memory:" // Use in-memory database for tests
	config.LLM.Mode = "offline"
	config.LLM.Offline.MockMode = true // Enable mock mode for testing
	config.Processing.Enabled = false  // Disable background processing

	// Create logger
	logger := arbor.NewLogger()

	// Initialize application
	application, err := app.New(config, logger)
	require.NoError(t, err, "Failed to initialize test application")

	// Return cleanup function
	cleanup := func() {
		application.Close()
	}

	return application, cleanup
}

// TestChatAPI_Health tests the chat health endpoint
func TestChatAPI_Health(t *testing.T) {
	t.Log("=== Testing Chat API - Health Endpoint ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)
	req := httptest.NewRequest(http.MethodGet, "/api/chat/health", nil)
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code, "Status should be 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, true, response["healthy"], "Service should be healthy")
	assert.Equal(t, "offline", response["mode"], "Mode should be offline")

	t.Log("✅ SUCCESS: Health endpoint returns correct status")
}

// TestChatAPI_HealthMethodNotAllowed tests invalid HTTP method for health endpoint
func TestChatAPI_HealthMethodNotAllowed(t *testing.T) {
	t.Log("=== Testing Chat API - Health Method Not Allowed ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/health", nil)
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "Status should be 405 Method Not Allowed")

	t.Log("✅ SUCCESS: Health endpoint rejects invalid methods")
}

// TestChatAPI_Chat_Success tests successful chat request
func TestChatAPI_Chat_Success(t *testing.T) {
	t.Log("=== Testing Chat API - Successful Chat Request ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create request body
	reqBody := map[string]interface{}{
		"message": "Hello, how are you?",
		"rag_config": map[string]interface{}{
			"enabled": false,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code, "Status should be 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, true, response["success"], "Request should be successful")
	assert.NotEmpty(t, response["message"], "Response should have a message")
	assert.Equal(t, "offline", response["mode"], "Mode should be offline")
	assert.Equal(t, "offline", response["model"], "Model should be offline")

	t.Log("✅ SUCCESS: Chat request completed successfully")
	t.Logf("Response message: %v", response["message"])
}

// TestChatAPI_Chat_WithRAG tests chat request with RAG enabled
func TestChatAPI_Chat_WithRAG(t *testing.T) {
	t.Log("=== Testing Chat API - Chat with RAG ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create request body with RAG enabled
	reqBody := map[string]interface{}{
		"message": "What is the system about?",
		"rag_config": map[string]interface{}{
			"enabled":        true,
			"max_documents":  5,
			"min_similarity": 0.7,
			"search_mode":    "vector",
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code, "Status should be 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, true, response["success"], "Request should be successful")
	assert.NotEmpty(t, response["message"], "Response should have a message")

	t.Log("✅ SUCCESS: Chat with RAG completed successfully")
}

// TestChatAPI_Chat_WithHistory tests chat request with conversation history
func TestChatAPI_Chat_WithHistory(t *testing.T) {
	t.Log("=== Testing Chat API - Chat with History ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create request body with history
	reqBody := map[string]interface{}{
		"message": "What about now?",
		"history": []map[string]string{
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi, how can I help?"},
		},
		"rag_config": map[string]interface{}{
			"enabled": false,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code, "Status should be 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, true, response["success"], "Request should be successful")
	assert.NotEmpty(t, response["message"], "Response should have a message")

	t.Log("✅ SUCCESS: Chat with history completed successfully")
}

// TestChatAPI_Chat_EmptyMessage tests error handling for empty message
func TestChatAPI_Chat_EmptyMessage(t *testing.T) {
	t.Log("=== Testing Chat API - Empty Message Error ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create request body with empty message
	reqBody := map[string]interface{}{
		"message": "",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status should be 400 Bad Request")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, false, response["success"], "Request should fail")
	assert.Contains(t, response["error"], "Message field is required", "Error should mention required field")

	t.Log("✅ SUCCESS: Empty message error handled correctly")
}

// TestChatAPI_Chat_InvalidJSON tests error handling for invalid JSON
func TestChatAPI_Chat_InvalidJSON(t *testing.T) {
	t.Log("=== Testing Chat API - Invalid JSON Error ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create invalid JSON
	invalidJSON := []byte(`{"message": "Hello", invalid}`)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusBadRequest, w.Code, "Status should be 400 Bad Request")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, false, response["success"], "Request should fail")
	assert.Contains(t, response["error"], "Invalid request body", "Error should mention invalid body")

	t.Log("✅ SUCCESS: Invalid JSON error handled correctly")
}

// TestChatAPI_Chat_MethodNotAllowed tests invalid HTTP method
func TestChatAPI_Chat_MethodNotAllowed(t *testing.T) {
	t.Log("=== Testing Chat API - Method Not Allowed ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "Status should be 405 Method Not Allowed")

	t.Log("✅ SUCCESS: Invalid method rejected correctly")
}

// TestChatAPI_Chat_CustomSystemPrompt tests chat with custom system prompt
func TestChatAPI_Chat_CustomSystemPrompt(t *testing.T) {
	t.Log("=== Testing Chat API - Custom System Prompt ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Create request body with custom system prompt
	reqBody := map[string]interface{}{
		"message":       "Write a function",
		"system_prompt": "You are a helpful coding assistant specialized in Go programming.",
		"rag_config": map[string]interface{}{
			"enabled": false,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	srv.Handler().ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code, "Status should be 200 OK")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err, "Response should be valid JSON")

	assert.Equal(t, true, response["success"], "Request should be successful")
	assert.NotEmpty(t, response["message"], "Response should have a message")

	t.Log("✅ SUCCESS: Chat with custom system prompt completed successfully")
}

// TestChatAPI_Integration_MultipleRequests tests multiple sequential chat requests
func TestChatAPI_Integration_MultipleRequests(t *testing.T) {
	t.Log("=== Testing Chat API - Multiple Sequential Requests ===")

	// Setup
	application, cleanup := setupTestApp(t)
	defer cleanup()

	srv := server.New(application)

	// Make multiple requests
	messages := []string{
		"Hello",
		"How are you?",
		"Tell me a joke",
		"Thank you",
	}

	for i, msg := range messages {
		t.Logf("Request %d: %s", i+1, msg)

		reqBody := map[string]interface{}{
			"message": msg,
			"rag_config": map[string]interface{}{
				"enabled": false,
			},
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.Handler().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, fmt.Sprintf("Request %d should succeed", i+1))

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Response should be valid JSON")
		assert.Equal(t, true, response["success"], fmt.Sprintf("Request %d should be successful", i+1))

		t.Logf("Response %d: %v", i+1, response["message"])
	}

	t.Log("✅ SUCCESS: Multiple sequential requests handled correctly")
}

// Benchmark tests
func BenchmarkChatAPI_SimpleRequest(b *testing.B) {
	// Setup
	config := common.NewDefaultConfig()
	config.Storage.SQLite.Path = ":memory:"
	config.LLM.Mode = "offline"
	config.LLM.Offline.MockMode = true
	config.Processing.Enabled = false

	logger := arbor.NewLogger()
	application, _ := app.New(config, logger)
	defer application.Close()

	srv := server.New(application)

	reqBody := map[string]interface{}{
		"message": "Hello",
		"rag_config": map[string]interface{}{
			"enabled": false,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/chat", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
	}
}
