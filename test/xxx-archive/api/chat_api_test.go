package api

import (
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
)

func TestChatHealth(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestChatHealth")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	resp, err := h.GET("/api/chat/health")
	if err != nil {
		t.Fatalf("Failed to check chat health: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check healthy field
	healthy, ok := result["healthy"].(bool)
	if !ok {
		t.Error("Response missing 'healthy' field")
	}

	// Check mode field
	mode, ok := result["mode"].(string)
	if !ok {
		t.Error("Response missing 'mode' field")
	}

	t.Logf("Chat service health: healthy=%v, mode=%s", healthy, mode)
}

func TestChatMessage(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestChatMessage")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Note: HTTP client in setup uses 30 second timeout
	// May need to increase if chat responses take longer
	h := env.NewHTTPTestHelper(t)

	// Send a simple message
	message := map[string]interface{}{
		"message": "Hello, what can you help me with?",
		"history": []interface{}{},
	}

	resp, err := h.POST("/api/chat", message)
	if err != nil {
		t.Fatalf("Failed to send chat message: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check success field
	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Errorf("Expected success=true, got: %v", result)
	}

	// Check message field
	responseMsg, ok := result["message"].(string)
	if !ok || responseMsg == "" {
		t.Error("Response missing or empty 'message' field")
	}

	t.Logf("Chat response: %s", responseMsg)
}

func TestChatWithHistory(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestChatWithHistory")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Note: HTTP client in setup uses 30 second timeout
	// May need to increase if chat responses take longer
	h := env.NewHTTPTestHelper(t)

	// First message
	message1 := map[string]interface{}{
		"message": "My name is Alice",
		"history": []interface{}{},
	}

	resp1, err := h.POST("/api/chat", message1)
	if err != nil {
		t.Fatalf("Failed to send first message: %v", err)
	}

	var result1 map[string]interface{}
	if err := h.ParseJSONResponse(resp1, &result1); err != nil {
		t.Fatalf("Failed to parse first response: %v", err)
	}

	firstResponse, ok := result1["message"].(string)
	if !ok {
		t.Fatal("First response missing message")
	}

	// Second message with history
	message2 := map[string]interface{}{
		"message": "What is my name?",
		"history": []interface{}{
			map[string]string{"role": "user", "content": "My name is Alice"},
			map[string]string{"role": "assistant", "content": firstResponse},
		},
	}

	resp2, err := h.POST("/api/chat", message2)
	if err != nil {
		t.Fatalf("Failed to send second message: %v", err)
	}

	h.AssertStatusCode(resp2, http.StatusOK)

	var result2 map[string]interface{}
	if err := h.ParseJSONResponse(resp2, &result2); err != nil {
		t.Fatalf("Failed to parse second response: %v", err)
	}

	secondResponse, ok := result2["message"].(string)
	if !ok || secondResponse == "" {
		t.Error("Second response missing or empty message")
	}

	t.Logf("Second response: %s", secondResponse)
}

func TestChatEmptyMessage(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestChatEmptyMessage")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Send empty message (should fail validation)
	message := map[string]interface{}{
		"message": "",
		"history": []interface{}{},
	}

	resp, err := h.POST("/api/chat", message)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Should return 400 Bad Request
	h.AssertStatusCode(resp, http.StatusBadRequest)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Check error field
	success, ok := result["success"].(bool)
	if ok && success {
		t.Error("Expected success=false for empty message")
	}
}
