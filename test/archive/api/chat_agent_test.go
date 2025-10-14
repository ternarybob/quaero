package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ChatAgentRequest represents the request body for /api/chat with agent mode
type ChatAgentRequest struct {
	Message  string `json:"message"`
	UseAgent bool   `json:"use_agent"`
}

// ChatAgentResponse represents the response from /api/chat in agent mode
type ChatAgentResponse struct {
	Success  bool                   `json:"success"`
	Message  string                 `json:"message"`
	Mode     string                 `json:"mode"`
	Model    string                 `json:"model"`
	Metadata map[string]interface{} `json:"metadata"`
	Error    string                 `json:"error,omitempty"`
}

func TestChatAgentBasic(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Logf("Testing agent mode at %s", baseURL)

	// Create agent request
	reqBody := ChatAgentRequest{
		Message:  "hello",
		UseAgent: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Make request with extended timeout for agent processing
	client := &http.Client{
		Timeout: 3 * time.Minute,
	}

	startTime := time.Now()
	resp, err := client.Post(
		fmt.Sprintf("%s/api/chat", baseURL),
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Request failed after %v: %v", duration, err)
	}
	defer resp.Body.Close()

	// Parse response
	var chatResp ChatAgentResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Log response details
	t.Logf("Agent response received in %v", duration)
	t.Logf("Success: %v", chatResp.Success)
	t.Logf("Model: %s", chatResp.Model)
	t.Logf("Mode: %s", chatResp.Mode)
	if chatResp.Metadata != nil {
		if agentMode, ok := chatResp.Metadata["agent_mode"]; ok {
			t.Logf("Agent mode flag: %v", agentMode)
		}
		if thinkingTime, ok := chatResp.Metadata["thinking_time"]; ok {
			t.Logf("Thinking time: %v", thinkingTime)
		}
	}

	// Assertions
	if !chatResp.Success {
		t.Errorf("Expected success=true, got false. Error: %s", chatResp.Error)
	}

	if chatResp.Model != "agent-mode" {
		t.Errorf("Expected model='agent-mode', got '%s'", chatResp.Model)
	}

	if chatResp.Message == "" {
		t.Errorf("Expected non-empty message response")
	}

	// Verify agent_mode metadata
	if chatResp.Metadata == nil {
		t.Errorf("Expected metadata to be present")
	} else if agentMode, ok := chatResp.Metadata["agent_mode"]; !ok || agentMode != true {
		t.Errorf("Expected metadata.agent_mode=true, got %v", agentMode)
	}

	t.Logf("✓ Agent mode test passed")
	t.Logf("Response message: %s", truncateMessage(chatResp.Message, 200))
}

func TestChatAgentVsRAG(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Logf("Testing agent vs RAG mode routing")

	client := &http.Client{
		Timeout: 3 * time.Minute,
	}

	// Test 1: Agent mode
	t.Run("AgentMode", func(t *testing.T) {
		reqBody := ChatAgentRequest{
			Message:  "hello",
			UseAgent: true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			fmt.Sprintf("%s/api/chat", baseURL),
			"application/json",
			bytes.NewBuffer(jsonBody),
		)
		if err != nil {
			t.Fatalf("Agent mode request failed: %v", err)
		}
		defer resp.Body.Close()

		var chatResp ChatAgentResponse
		json.NewDecoder(resp.Body).Decode(&chatResp)

		if chatResp.Model != "agent-mode" {
			t.Errorf("Expected agent-mode, got %s", chatResp.Model)
		}

		if chatResp.Metadata["agent_mode"] != true {
			t.Errorf("Expected agent_mode=true in metadata")
		}

		t.Logf("✓ Agent mode: model=%s", chatResp.Model)
	})

	// Test 2: RAG mode (default)
	t.Run("RAGMode", func(t *testing.T) {
		reqBody := ChatAgentRequest{
			Message:  "hello",
			UseAgent: false, // Explicitly RAG mode
		}

		jsonBody, _ := json.Marshal(reqBody)
		resp, err := client.Post(
			fmt.Sprintf("%s/api/chat", baseURL),
			"application/json",
			bytes.NewBuffer(jsonBody),
		)
		if err != nil {
			t.Fatalf("RAG mode request failed: %v", err)
		}
		defer resp.Body.Close()

		var chatResp ChatAgentResponse
		json.NewDecoder(resp.Body).Decode(&chatResp)

		if chatResp.Model == "agent-mode" {
			t.Errorf("Expected RAG model, got agent-mode")
		}

		if chatResp.Metadata["agent_mode"] == true {
			t.Errorf("Expected agent_mode=false in metadata")
		}

		t.Logf("✓ RAG mode: model=%s", chatResp.Model)
	})
}

// Helper function to truncate long messages
func truncateMessage(msg string, maxLen int) string {
	// Remove extra whitespace
	msg = strings.TrimSpace(msg)
	msg = strings.ReplaceAll(msg, "\n", " ")

	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}
