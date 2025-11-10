package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test/common"
)

// TestQuickCrawlEndpoint tests the /api/job-definitions/quick-crawl endpoint
func TestQuickCrawlEndpoint(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestQuickCrawlEndpoint")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)
	baseURL := h.BaseURL

	t.Run("CreateAndExecuteQuickCrawl", func(t *testing.T) {
		// Build request payload
		payload := map[string]interface{}{
			"url": "https://example.com",
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal payload: %v", err)
		}

		// Send POST request
		resp, err := http.Post(
			baseURL+"/api/job-definitions/quick-crawl",
			"application/json",
			bytes.NewBuffer(payloadBytes),
		)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusAccepted {
			t.Errorf("Expected status 202 Accepted, got %d", resp.StatusCode)
		}

		// Parse response
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response fields
		if jobID, ok := result["job_id"].(string); !ok || jobID == "" {
			t.Errorf("Expected job_id in response, got: %v", result)
		}

		if status, ok := result["status"].(string); !ok || status != "running" {
			t.Errorf("Expected status='running', got: %v", result)
		}

		if url, ok := result["url"].(string); !ok || url != "https://example.com" {
			t.Errorf("Expected url='https://example.com', got: %v", result)
		}

		// Verify default values are returned
		if maxDepth, ok := result["max_depth"].(float64); !ok || maxDepth != 2 {
			t.Errorf("Expected max_depth=2 (default), got: %v", result)
		}

		if maxPages, ok := result["max_pages"].(float64); !ok || maxPages != 10 {
			t.Errorf("Expected max_pages=10 (default), got: %v", result)
		}

		t.Logf("Quick crawl job created successfully: %s", result["job_id"])
	})

	t.Run("QuickCrawlWithCustomParams", func(t *testing.T) {
		// Build request with custom parameters
		payload := map[string]interface{}{
			"url":       "https://example.com/docs",
			"name":      "Custom Quick Crawl",
			"max_depth": 3,
			"max_pages": 20,
			"include_patterns": []string{".*\\.html$"},
			"exclude_patterns": []string{".*/api/.*"},
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal payload: %v", err)
		}

		// Send POST request
		resp, err := http.Post(
			baseURL+"/api/job-definitions/quick-crawl",
			"application/json",
			bytes.NewBuffer(payloadBytes),
		)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusAccepted {
			t.Errorf("Expected status 202 Accepted, got %d", resp.StatusCode)
		}

		// Parse response
		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify custom parameters are reflected
		if maxDepth, ok := result["max_depth"].(float64); !ok || maxDepth != 3 {
			t.Errorf("Expected max_depth=3, got: %v", result)
		}

		if maxPages, ok := result["max_pages"].(float64); !ok || maxPages != 20 {
			t.Errorf("Expected max_pages=20, got: %v", result)
		}

		t.Logf("Quick crawl with custom params created: %s", result["job_id"])
	})

	t.Run("QuickCrawlMissingURL", func(t *testing.T) {
		// Build request without URL
		payload := map[string]interface{}{
			"name": "Invalid Request",
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Failed to marshal payload: %v", err)
		}

		// Send POST request
		resp, err := http.Post(
			baseURL+"/api/job-definitions/quick-crawl",
			"application/json",
			bytes.NewBuffer(payloadBytes),
		)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		// Check status code - should be 400 Bad Request
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 Bad Request, got %d", resp.StatusCode)
		}

		t.Log("Correctly rejected request without URL")
	})
}
