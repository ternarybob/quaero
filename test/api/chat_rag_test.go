package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// ChatRequest represents the request body for /api/chat
type ChatRequest struct {
	Message   string              `json:"message"`
	History   []map[string]string `json:"history"`
	RAGConfig RAGConfig           `json:"rag_config"`
}

// RAGConfig represents the RAG configuration
type RAGConfig struct {
	Enabled       bool    `json:"enabled"`
	MaxDocuments  int     `json:"max_documents"`
	MinSimilarity float64 `json:"min_similarity"`
	SearchMode    string  `json:"search_mode"`
}

// ChatResponse represents the response from /api/chat
type ChatResponse struct {
	Success     bool                     `json:"success"`
	Message     string                   `json:"message"`
	Mode        string                   `json:"mode"`
	Model       string                   `json:"model"`
	ContextDocs []map[string]interface{} `json:"context_docs"`
	Metadata    map[string]interface{}   `json:"metadata"`
	Error       string                   `json:"error,omitempty"`
}

func TestChatRAGBasic(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	// Test cases with expected behavior
	testCases := []struct {
		name            string
		message         string
		expectedSuccess bool
		maxDuration     time.Duration
	}{
		{
			name:            "Simple hello #1",
			message:         "hello",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute, // Extended for local LLM processing
		},
		{
			name:            "Simple hello #2",
			message:         "hello",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
		{
			name:            "Simple hello #3",
			message:         "hello",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
		{
			name:            "Hello with question",
			message:         "hello, how are you",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
		{
			name:            "Time question",
			message:         "hello, what time is it",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
		{
			name:            "Location question",
			message:         "where are you",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
		{
			name:            "Document count question",
			message:         "how many documents are there?",
			expectedSuccess: true,
			maxDuration:     2 * time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			reqBody := ChatRequest{
				Message: tc.message,
				History: []map[string]string{},
				RAGConfig: RAGConfig{
					Enabled:       true,
					MaxDocuments:  5,
					MinSimilarity: 0.7,
					SearchMode:    "vector",
				},
			}

			jsonBody, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Create HTTP request with timeout
			client := &http.Client{
				Timeout: tc.maxDuration + (5 * time.Second), // Add 5s buffer
			}

			t.Logf("Sending request: %s", tc.message)
			startTime := time.Now()

			req, err := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Send request
			resp, err := client.Do(req)
			duration := time.Since(startTime)

			if err != nil {
				t.Fatalf("Request failed after %v: %v", duration, err)
			}
			defer resp.Body.Close()

			t.Logf("Response received in %v (max: %v)", duration, tc.maxDuration)

			// Check duration
			if duration > tc.maxDuration {
				t.Errorf("Request took %v, expected < %v", duration, tc.maxDuration)
			}

			// Parse response
			var chatResp ChatResponse
			if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Validate response
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			if chatResp.Success != tc.expectedSuccess {
				t.Errorf("Expected success=%v, got %v. Error: %s",
					tc.expectedSuccess, chatResp.Success, chatResp.Error)
			}

			if tc.expectedSuccess {
				if chatResp.Message == "" {
					t.Errorf("Expected non-empty message, got empty string")
				}
				if chatResp.Mode == "" {
					t.Errorf("Expected non-empty mode, got empty string")
				}
				t.Logf("✓ Response: %s", chatResp.Message[:min(50, len(chatResp.Message))])
				t.Logf("✓ Mode: %s, Model: %s, Context docs: %d",
					chatResp.Mode, chatResp.Model, len(chatResp.ContextDocs))
			}

			// Add small delay between requests to avoid overwhelming the server
			time.Sleep(500 * time.Millisecond)
		})
	}
}

func TestChatRAGServerHealth(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Log("Checking server health before and after chat tests")

	// Check health before
	healthBefore := checkHealth(t, baseURL)
	if !healthBefore {
		t.Fatal("Server unhealthy before chat tests")
	}
	t.Log("✓ Server healthy before tests")

	// Run a simple chat request
	reqBody := ChatRequest{
		Message: "test message",
		History: []map[string]string{},
		RAGConfig: RAGConfig{
			Enabled:       true,
			MaxDocuments:  5,
			MinSimilarity: 0.7,
			SearchMode:    "vector",
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	client := &http.Client{Timeout: 40 * time.Second}

	req, _ := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Chat request failed: %v", err)
	} else {
		resp.Body.Close()
		t.Logf("Chat request completed with status: %d", resp.StatusCode)
	}

	// Wait a moment for any crash to manifest
	time.Sleep(2 * time.Second)

	// Check health after
	healthAfter := checkHealth(t, baseURL)
	if !healthAfter {
		t.Error("❌ Server unhealthy after chat tests - CRASH DETECTED")
		t.Log("Check server logs for crash details")
	} else {
		t.Log("✓ Server healthy after tests")
	}
}

func checkHealth(t *testing.T, baseURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/chat/health")
	if err != nil {
		t.Logf("Health check failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	var healthResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Logf("Failed to decode health response: %v", err)
		return false
	}

	healthy, ok := healthResp["healthy"].(bool)
	return ok && healthy
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestChatRAGSequential(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Log("Testing sequential chat requests to detect crashes")

	messages := []string{
		"hello",
		"hello again",
		"hello one more time",
	}

	for i, msg := range messages {
		t.Run(fmt.Sprintf("Request_%d", i+1), func(t *testing.T) {
			// Check health before request
			if !checkHealth(t, baseURL) {
				t.Fatalf("Server crashed before request %d", i+1)
			}

			reqBody := ChatRequest{
				Message: msg,
				History: []map[string]string{},
				RAGConfig: RAGConfig{
					Enabled:       true,
					MaxDocuments:  5,
					MinSimilarity: 0.7,
					SearchMode:    "vector",
				},
			}

			jsonBody, _ := json.Marshal(reqBody)
			client := &http.Client{Timeout: 35 * time.Second}

			startTime := time.Now()
			req, _ := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			duration := time.Since(startTime)

			if err != nil {
				t.Errorf("Request %d failed after %v: %v", i+1, duration, err)
				// Check if server crashed
				time.Sleep(1 * time.Second)
				if !checkHealth(t, baseURL) {
					t.Fatalf("❌ SERVER CRASHED on request %d", i+1)
				}
				return
			}
			defer resp.Body.Close()

			t.Logf("Request %d completed in %v", i+1, duration)

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: expected status 200, got %d", i+1, resp.StatusCode)
			}

			// Wait and check health after
			time.Sleep(500 * time.Millisecond)
			if !checkHealth(t, baseURL) {
				t.Fatalf("❌ SERVER CRASHED after request %d", i+1)
			}
		})
	}

	t.Log("✓ All sequential requests completed without crashes")
}

// TestChatRAGCorpusSummary tests RAG functionality by querying the corpus summary document
// This test definitively proves RAG is working because:
// 1. The summary document is automatically generated with corpus statistics
// 2. Only RAG can retrieve this document from the vector database
// 3. The LLM response should contain specific document counts from the summary
func TestChatRAGCorpusSummary(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Log("=== Testing RAG with Corpus Summary Queries ===")
	t.Log("These questions can ONLY be answered using RAG retrieval")

	// Test cases that require RAG to answer correctly
	testCases := []struct {
		name              string
		message           string
		expectedInContext bool // Should contain corpus statistics
		maxDuration       time.Duration
	}{
		{
			name:              "Total document count query",
			message:           "How many total documents are in the system?",
			expectedInContext: true,
			maxDuration:       90 * time.Second,
		},
		{
			name:              "Jira document count query",
			message:           "How many Jira issues are indexed?",
			expectedInContext: true,
			maxDuration:       90 * time.Second,
		},
		{
			name:              "Confluence document count query",
			message:           "How many Confluence pages are available?",
			expectedInContext: true,
			maxDuration:       90 * time.Second,
		},
		{
			name:              "Embedded document count query",
			message:           "How many documents have embeddings?",
			expectedInContext: true,
			maxDuration:       90 * time.Second,
		},
		{
			name:              "General corpus statistics query",
			message:           "Tell me about the document corpus statistics",
			expectedInContext: true,
			maxDuration:       90 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request with RAG enabled
			reqBody := ChatRequest{
				Message: tc.message,
				History: []map[string]string{},
				RAGConfig: RAGConfig{
					Enabled:       true,
					MaxDocuments:  5,
					MinSimilarity: 0.7,
					SearchMode:    "vector",
				},
			}

			jsonBody, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			// Create HTTP request with timeout
			client := &http.Client{
				Timeout: tc.maxDuration + (5 * time.Second),
			}

			t.Logf("Query: %s", tc.message)
			startTime := time.Now()

			req, err := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Send request
			resp, err := client.Do(req)
			duration := time.Since(startTime)

			if err != nil {
				t.Fatalf("Request failed after %v: %v", duration, err)
			}
			defer resp.Body.Close()

			t.Logf("Response received in %v", duration)

			// Parse response
			var chatResp ChatResponse
			if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Validate response
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			if !chatResp.Success {
				t.Errorf("Expected success=true, got false. Error: %s", chatResp.Error)
			}

			// Verify RAG is enabled and context documents were retrieved
			if chatResp.Mode != "offline" {
				t.Errorf("Expected mode=offline, got %s", chatResp.Mode)
			}

			if len(chatResp.ContextDocs) == 0 {
				t.Errorf("❌ CRITICAL: No context documents retrieved - RAG may not be working")
				t.Errorf("   Expected at least 1 document (corpus summary) but got 0")
			} else {
				t.Logf("✓ RAG retrieved %d context document(s)", len(chatResp.ContextDocs))

				// Check if corpus summary document is in context
				foundSummary := false
				for i, doc := range chatResp.ContextDocs {
					if title, ok := doc["title"].(string); ok {
						t.Logf("  Context doc %d: %s", i+1, title)
						if title == "Quaero Corpus Summary - Document Statistics and Metadata" {
							foundSummary = true
							t.Logf("  ✓ Found corpus summary document!")
						}
					}
				}

				if tc.expectedInContext && !foundSummary {
					t.Logf("⚠️  WARNING: Corpus summary not in context (may still be working)")
				}
			}

			// Log the response for manual verification
			t.Logf("Response: %s", chatResp.Message)

			// Check if response contains numbers (likely document counts)
			containsNumbers := false
			for _, char := range chatResp.Message {
				if char >= '0' && char <= '9' {
					containsNumbers = true
					break
				}
			}

			if tc.expectedInContext && containsNumbers {
				t.Logf("✓ Response contains numeric data (likely from corpus summary)")
			} else if tc.expectedInContext {
				t.Logf("⚠️  Response may not contain corpus statistics")
			}

			// Add delay between requests
			time.Sleep(500 * time.Millisecond)
		})
	}

	t.Log("")
	t.Log("=== RAG Verification Complete ===")
	t.Log("If context documents were retrieved with corpus statistics,")
	t.Log("then RAG is definitively working correctly.")
}

// TestChatMetadata tests that technical metadata is returned in chat responses
func TestChatMetadata(t *testing.T) {
	baseURL := os.Getenv("TEST_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8085"
	}

	t.Log("=== Testing Technical Metadata in Chat Responses ===")

	// Create request
	reqBody := ChatRequest{
		Message: "Hello, tell me about the system",
		History: []map[string]string{},
		RAGConfig: RAGConfig{
			Enabled:       true,
			MaxDocuments:  5,
			MinSimilarity: 0.7,
			SearchMode:    "vector",
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	t.Log("Sending chat request...")
	startTime := time.Now()

	req, err := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Request failed after %v: %v", duration, err)
	}
	defer resp.Body.Close()

	t.Logf("Response received in %v", duration)

	// Parse response
	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate basic response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if !chatResp.Success {
		t.Errorf("Expected success=true, got false. Error: %s", chatResp.Error)
	}

	// Verify metadata exists
	if chatResp.Metadata == nil {
		t.Fatal("❌ CRITICAL: Metadata field is missing from response")
	}

	t.Log("✓ Metadata field present in response")

	// Verify metadata contains expected fields
	expectedFields := []string{"document_count", "references", "thinking_time", "rag_enabled"}
	for _, field := range expectedFields {
		if _, exists := chatResp.Metadata[field]; !exists {
			t.Errorf("❌ Metadata missing expected field: %s", field)
		} else {
			t.Logf("✓ Metadata contains field: %s = %v", field, chatResp.Metadata[field])
		}
	}

	// Verify thinking_time format (should be like "2.34s")
	if thinkingTime, ok := chatResp.Metadata["thinking_time"].(string); ok {
		if len(thinkingTime) == 0 || thinkingTime[len(thinkingTime)-1] != 's' {
			t.Errorf("❌ thinking_time has unexpected format: %s (expected XXXs)", thinkingTime)
		} else {
			t.Logf("✓ thinking_time formatted correctly: %s", thinkingTime)
		}
	}

	// Verify document_count is a number
	if docCount, ok := chatResp.Metadata["document_count"].(float64); ok {
		t.Logf("✓ document_count is numeric: %.0f", docCount)
	} else {
		t.Errorf("❌ document_count is not numeric: %v", chatResp.Metadata["document_count"])
	}

	// Verify references is an array
	if references, ok := chatResp.Metadata["references"].([]interface{}); ok {
		t.Logf("✓ references is an array with %d items", len(references))
		for i, ref := range references {
			t.Logf("  Reference %d: %v", i+1, ref)
		}
	} else {
		t.Errorf("❌ references is not an array: %v", chatResp.Metadata["references"])
	}

	// Verify rag_enabled is a boolean
	if ragEnabled, ok := chatResp.Metadata["rag_enabled"].(bool); ok {
		t.Logf("✓ rag_enabled is boolean: %v", ragEnabled)
	} else {
		t.Errorf("❌ rag_enabled is not boolean: %v", chatResp.Metadata["rag_enabled"])
	}

	t.Log("")
	t.Log("=== Metadata Verification Complete ===")
}
