// Package api contains API integration tests for the search endpoint.
//
// These tests verify the AdvancedSearchService integration with the HTTP API,
// testing Google-style query parsing (OR default, +AND, "phrases", qualifiers).
package api

import (
	"net/http"
	"testing"
)

// TestSearchBasicQuery tests basic search with single term
func TestSearchBasicQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchBasicQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Test basic search query
	resp, err := h.GET("/api/search?q=authentication&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify response structure
	if _, ok := result["results"]; !ok {
		t.Error("Search response missing 'results' field")
	}

	if _, ok := result["count"]; !ok {
		t.Error("Search response missing 'count' field")
	}

	if query, ok := result["query"].(string); !ok || query != "authentication" {
		t.Errorf("Expected query 'authentication', got: %v", result["query"])
	}

	if limit, ok := result["limit"].(float64); !ok || int(limit) != 10 {
		t.Errorf("Expected limit 10, got: %v", result["limit"])
	}

	if offset, ok := result["offset"].(float64); !ok || int(offset) != 0 {
		t.Errorf("Expected offset 0, got: %v", result["offset"])
	}

	t.Logf("✓ Basic search query successful (query='authentication')")
}

// TestSearchORQuery tests OR search (default behavior)
func TestSearchORQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchORQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// OR search: documents containing "authentication" OR "security"
	resp, err := h.GET("/api/search?q=authentication security&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved
	if query, ok := result["query"].(string); !ok || query != "authentication security" {
		t.Errorf("Expected query 'authentication security', got: %v", result["query"])
	}

	// Results array should exist (count may be 0 if no documents match)
	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	t.Logf("✓ OR search successful (query='authentication security', results=%d)", len(results))
}

// TestSearchANDQuery tests AND search with + prefix
func TestSearchANDQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchANDQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// AND search: documents containing "authentication" AND "security"
	resp, err := h.GET("/api/search?q=%2Bauthentication %2Bsecurity&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved (URL decoded)
	if query, ok := result["query"].(string); !ok || query != "+authentication +security" {
		t.Errorf("Expected query '+authentication +security', got: %v", result["query"])
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	t.Logf("✓ AND search successful (query='+authentication +security', results=%d)", len(results))
}

// TestSearchPhraseQuery tests phrase search with quotes
func TestSearchPhraseQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchPhraseQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Phrase search: exact phrase "security audit"
	resp, err := h.GET("/api/search?q=\"security audit\"&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved
	if query, ok := result["query"].(string); !ok || query != "\"security audit\"" {
		t.Errorf("Expected query '\"security audit\"', got: %v", result["query"])
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	t.Logf("✓ Phrase search successful (query='\"security audit\"', results=%d)", len(results))
}

// TestSearchDocumentTypeQualifier tests document_type qualifier
func TestSearchDocumentTypeQualifier(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchDocumentTypeQualifier")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Search with document_type qualifier
	resp, err := h.GET("/api/search?q=authentication document_type:jira&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved
	if query, ok := result["query"].(string); !ok || query != "authentication document_type:jira" {
		t.Errorf("Expected query 'authentication document_type:jira', got: %v", result["query"])
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	// If we have results, verify they are Jira documents
	for i, r := range results {
		resultMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}

		if sourceType, ok := resultMap["source_type"].(string); ok && sourceType != "" && sourceType != "jira" {
			t.Errorf("Result %d has wrong source_type: expected 'jira', got '%s'", i, sourceType)
		}
	}

	t.Logf("✓ document_type:jira qualifier successful (results=%d)", len(results))
}

// TestSearchCaseQualifier tests case:match qualifier
func TestSearchCaseQualifier(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchCaseQualifier")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Search with case:match qualifier (case-sensitive)
	resp, err := h.GET("/api/search?q=Authentication case:match&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved
	if query, ok := result["query"].(string); !ok || query != "Authentication case:match" {
		t.Errorf("Expected query 'Authentication case:match', got: %v", result["query"])
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	t.Logf("✓ case:match qualifier successful (results=%d)", len(results))
}

// TestSearchMixedQuery tests complex query with multiple features
func TestSearchMixedQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchMixedQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Mixed query: +AND, phrase, qualifier
	resp, err := h.GET("/api/search?q=%2Bauthentication \"security audit\" document_type:jira&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify query is preserved
	if query, ok := result["query"].(string); !ok || query != "+authentication \"security audit\" document_type:jira" {
		t.Errorf("Expected mixed query, got: %v", result["query"])
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	t.Logf("✓ Mixed query successful (results=%d)", len(results))
}

// TestSearchEmptyQuery tests empty query handling
func TestSearchEmptyQuery(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchEmptyQuery")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Empty query should return empty results (not error)
	resp, err := h.GET("/api/search?q=&limit=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Empty query returns empty results
	results, ok := result["results"].([]interface{})
	if !ok {
		t.Error("Results field is not an array")
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty query, got %d", len(results))
	}

	t.Log("✓ Empty query handled correctly")
}

// TestSearchPagination tests pagination parameters
func TestSearchPagination(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchPagination")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Test with custom limit and offset
	resp, err := h.GET("/api/search?q=test&limit=5&offset=10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify pagination parameters
	if limit, ok := result["limit"].(float64); !ok || int(limit) != 5 {
		t.Errorf("Expected limit 5, got: %v", result["limit"])
	}

	if offset, ok := result["offset"].(float64); !ok || int(offset) != 10 {
		t.Errorf("Expected offset 10, got: %v", result["offset"])
	}

	t.Log("✓ Pagination parameters handled correctly")
}

// TestSearchDefaultPagination tests default pagination values
func TestSearchDefaultPagination(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchDefaultPagination")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Test without pagination params (should use defaults)
	resp, err := h.GET("/api/search?q=test")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify default pagination
	if limit, ok := result["limit"].(float64); !ok || int(limit) != 50 {
		t.Errorf("Expected default limit 50, got: %v", result["limit"])
	}

	if offset, ok := result["offset"].(float64); !ok || int(offset) != 0 {
		t.Errorf("Expected default offset 0, got: %v", result["offset"])
	}

	t.Log("✓ Default pagination values correct (limit=50, offset=0)")
}

// TestSearchMaxLimitEnforcement tests that limit is capped at 100
func TestSearchMaxLimitEnforcement(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchMaxLimitEnforcement")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Request limit > 100 (should be capped at 100)
	resp, err := h.GET("/api/search?q=test&limit=500")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify limit is capped at 100
	if limit, ok := result["limit"].(float64); !ok || int(limit) != 100 {
		t.Errorf("Expected max limit 100, got: %v", result["limit"])
	}

	t.Log("✓ Maximum limit enforcement correct (capped at 100)")
}

// TestSearchNegativeOffset tests negative offset handling
func TestSearchNegativeOffset(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchNegativeOffset")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Negative offset should be clamped to 0
	resp, err := h.GET("/api/search?q=test&offset=-10")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	// Verify offset is clamped to 0
	if offset, ok := result["offset"].(float64); !ok || int(offset) != 0 {
		t.Errorf("Expected offset clamped to 0, got: %v", result["offset"])
	}

	t.Log("✓ Negative offset clamped to 0")
}

// TestSearchResultStructure tests that results have correct structure
func TestSearchResultStructure(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchResultStructure")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	resp, err := h.GET("/api/search?q=test&limit=1")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(resp, &result); err != nil {
		t.Fatalf("Failed to parse search response: %v", err)
	}

	results, ok := result["results"].([]interface{})
	if !ok {
		t.Fatal("Results field is not an array")
	}

	// If we have results, verify structure
	if len(results) > 0 {
		firstResult, ok := results[0].(map[string]interface{})
		if !ok {
			t.Fatal("First result is not an object")
		}

		// Verify required fields
		requiredFields := []string{"id", "title", "brief", "url", "source_type"}
		for _, field := range requiredFields {
			if _, ok := firstResult[field]; !ok {
				t.Errorf("Result missing required field: %s", field)
			}
		}

		// Verify brief is truncated to max 203 chars (200 + "...")
		if brief, ok := firstResult["brief"].(string); ok && len(brief) > 203 {
			t.Errorf("Brief should be truncated to max 203 chars, got %d", len(brief))
		}

		t.Logf("✓ Result structure correct: %+v", firstResult)
	} else {
		t.Log("✓ No results to validate structure (database may be empty)")
	}
}

// TestSearchMethodNotAllowed tests that non-GET methods are rejected
func TestSearchMethodNotAllowed(t *testing.T) {
	env, err := SetupTestEnvironment("TestSearchMethodNotAllowed")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// POST should be rejected
	resp, err := h.POST("/api/search", map[string]interface{}{"q": "test"})
	if err != nil {
		t.Fatalf("Failed to make POST request: %v", err)
	}

	// Should return 405 Method Not Allowed
	h.AssertStatusCode(resp, http.StatusMethodNotAllowed)

	t.Log("✓ Non-GET method correctly rejected with 405")
}
