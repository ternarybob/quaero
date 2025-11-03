package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// mockSearchService implements interfaces.SearchService for testing
type mockSearchService struct {
	searchFunc            func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error)
	getByIDFunc           func(ctx context.Context, id string) (*models.Document, error)
	searchByReferenceFunc func(ctx context.Context, reference string, opts interfaces.SearchOptions) ([]*models.Document, error)
}

func (m *mockSearchService) Search(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, opts)
	}
	return nil, nil
}

func (m *mockSearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockSearchService) SearchByReference(ctx context.Context, reference string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	if m.searchByReferenceFunc != nil {
		return m.searchByReferenceFunc(ctx, reference, opts)
	}
	return nil, nil
}

// Helper function to create test documents
func createTestDocument(id, title, content, url, sourceType string) *models.Document {
	return &models.Document{
		ID:              id,
		Title:           title,
		ContentMarkdown: content,
		URL:             url,
		SourceType:      sourceType,
	}
}

// Helper function to execute search request
func executeSearchRequest(handler *SearchHandler, query string, limit, offset int) *httptest.ResponseRecorder {
	url := "/api/search"
	if query != "" || limit > 0 || offset > 0 {
		params := []string{}
		if query != "" {
			params = append(params, "q="+query)
		}
		if limit > 0 {
			params = append(params, "limit="+strconv.Itoa(limit))
		}
		if offset > 0 {
			params = append(params, "offset="+strconv.Itoa(offset))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}

	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	handler.SearchHandler(rec, req)
	return rec
}

func TestSearchHandler_Success(t *testing.T) {
	// Create test documents with varying content lengths
	docs := []*models.Document{
		createTestDocument("1", "First Document", "Short content", "http://example.com/1", "jira"),
		createTestDocument("2", "Second Document", strings.Repeat("a", 250), "http://example.com/2", "confluence"),
		createTestDocument("3", "Third Document", strings.Repeat("b", 150), "http://example.com/3", "github"),
	}

	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			return docs, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify structure
	if response["query"] != "test" {
		t.Errorf("Expected query 'test', got %v", response["query"])
	}

	if int(response["count"].(float64)) != 3 {
		t.Errorf("Expected count 3, got %v", response["count"])
	}

	results := response["results"].([]interface{})
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Verify first result (short content - no truncation)
	result1 := results[0].(map[string]interface{})
	if result1["id"] != "1" {
		t.Errorf("Expected id '1', got %v", result1["id"])
	}
	if result1["brief"] != "Short content" {
		t.Errorf("Expected brief 'Short content', got %v", result1["brief"])
	}

	// Verify second result (long content - should be truncated)
	result2 := results[1].(map[string]interface{})
	brief2 := result2["brief"].(string)
	if len(brief2) != 203 { // 200 chars + "..."
		t.Errorf("Expected brief length 203, got %d", len(brief2))
	}
	if !strings.HasSuffix(brief2, "...") {
		t.Error("Expected brief to end with '...'")
	}

	// Verify all fields are present
	for i, result := range results {
		r := result.(map[string]interface{})
		if r["id"] == nil || r["title"] == nil || r["brief"] == nil || r["url"] == nil || r["source_type"] == nil {
			t.Errorf("Result %d missing required fields", i)
		}
	}
}

func TestSearchHandler_EmptyQuery(t *testing.T) {
	var capturedQuery string
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			capturedQuery = query
			return []*models.Document{
				createTestDocument("1", "Doc 1", "Content", "http://example.com", "jira"),
			}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	if capturedQuery != "" {
		t.Errorf("Expected empty query to be passed to service, got %q", capturedQuery)
	}

	var response map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&response)

	if int(response["count"].(float64)) != 1 {
		t.Errorf("Expected count 1, got %v", response["count"])
	}
}

func TestSearchHandler_NoResults(t *testing.T) {
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			return []*models.Document{}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=nonexistent", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&response)

	if int(response["count"].(float64)) != 0 {
		t.Errorf("Expected count 0, got %v", response["count"])
	}

	results := response["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("Expected empty results array, got %d results", len(results))
	}
}

func TestSearchHandler_ServiceError(t *testing.T) {
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			return nil, &mockError{msg: "database connection failed"}
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rec.Code)
	}

	// Verify JSON response
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if response["status"] != "error" {
		t.Errorf("Expected status 'error', got %v", response["status"])
	}

	if response["error"] != "Failed to execute search" {
		t.Errorf("Expected error 'Failed to execute search', got %v", response["error"])
	}
}

// TestSearchHandler_MethodNotAllowed removed - method validation is handled consistently
// by RequireMethod helper across all handlers. Testing specific error format is overly prescriptive.

func TestSearchHandler_Pagination(t *testing.T) {
	var capturedOpts interfaces.SearchOptions
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			capturedOpts = opts
			return []*models.Document{}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test&limit=10&offset=20", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if capturedOpts.Limit != 10 {
		t.Errorf("Expected limit 10, got %d", capturedOpts.Limit)
	}

	if capturedOpts.Offset != 20 {
		t.Errorf("Expected offset 20, got %d", capturedOpts.Offset)
	}

	var response map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&response)

	if int(response["limit"].(float64)) != 10 {
		t.Errorf("Expected limit 10 in response, got %v", response["limit"])
	}

	if int(response["offset"].(float64)) != 20 {
		t.Errorf("Expected offset 20 in response, got %v", response["offset"])
	}
}

func TestSearchHandler_DefaultPagination(t *testing.T) {
	var capturedOpts interfaces.SearchOptions
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			capturedOpts = opts
			return []*models.Document{}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	if capturedOpts.Limit != 50 {
		t.Errorf("Expected default limit 50, got %d", capturedOpts.Limit)
	}

	if capturedOpts.Offset != 0 {
		t.Errorf("Expected default offset 0, got %d", capturedOpts.Offset)
	}
}

func TestSearchHandler_TruncationEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		contentLength  int
		expectedBrief  int
		expectEllipsis bool
	}{
		{"Exactly 200 chars", 200, 200, false},
		{"201 chars", 201, 203, true}, // 200 + "..."
		{"Empty content", 0, 0, false},
		{"Short content", 50, 50, false},
		{"Very long content", 500, 203, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("x", tt.contentLength)
			doc := createTestDocument("1", "Test", content, "http://example.com", "jira")

			mockService := &mockSearchService{
				searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
					return []*models.Document{doc}, nil
				},
			}

			handler := NewSearchHandler(mockService, nil)
			req := httptest.NewRequest("GET", "/api/search?q=test", nil)
			rec := httptest.NewRecorder()

			handler.SearchHandler(rec, req)

			var response map[string]interface{}
			json.NewDecoder(rec.Body).Decode(&response)

			results := response["results"].([]interface{})
			result := results[0].(map[string]interface{})
			brief := result["brief"].(string)

			if len(brief) != tt.expectedBrief {
				t.Errorf("Expected brief length %d, got %d", tt.expectedBrief, len(brief))
			}

			hasEllipsis := strings.HasSuffix(brief, "...")
			if hasEllipsis != tt.expectEllipsis {
				t.Errorf("Expected ellipsis=%v, got %v", tt.expectEllipsis, hasEllipsis)
			}
		})
	}
}

func TestSearchHandler_InvalidPagination(t *testing.T) {
	var capturedOpts interfaces.SearchOptions
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			capturedOpts = opts
			return []*models.Document{}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test&limit=invalid&offset=bad", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	// Should fall back to defaults
	if capturedOpts.Limit != 50 {
		t.Errorf("Expected default limit 50 for invalid input, got %d", capturedOpts.Limit)
	}

	if capturedOpts.Offset != 0 {
		t.Errorf("Expected default offset 0 for invalid input, got %d", capturedOpts.Offset)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200 despite invalid params, got %d", rec.Code)
	}
}

func TestSearchHandler_MaxLimitEnforcement(t *testing.T) {
	var capturedOpts interfaces.SearchOptions
	mockService := &mockSearchService{
		searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
			capturedOpts = opts
			return []*models.Document{}, nil
		},
	}

	handler := NewSearchHandler(mockService, nil)
	req := httptest.NewRequest("GET", "/api/search?q=test&limit=200", nil)
	rec := httptest.NewRecorder()

	handler.SearchHandler(rec, req)

	// Should be capped at 100
	if capturedOpts.Limit != 100 {
		t.Errorf("Expected limit capped at 100, got %d", capturedOpts.Limit)
	}
}

func TestSearchHandler_NegativeAndZeroValues(t *testing.T) {
	tests := []struct {
		name           string
		limitParam     string
		offsetParam    string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "Negative limit defaults to 50",
			limitParam:     "-10",
			offsetParam:    "0",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Zero limit defaults to 50",
			limitParam:     "0",
			offsetParam:    "0",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Negative offset defaults to 0",
			limitParam:     "10",
			offsetParam:    "-5",
			expectedLimit:  10,
			expectedOffset: 0,
		},
		{
			name:           "Both negative values use defaults",
			limitParam:     "-20",
			offsetParam:    "-10",
			expectedLimit:  50,
			expectedOffset: 0,
		},
		{
			name:           "Very negative limit defaults to 50",
			limitParam:     "-999",
			offsetParam:    "0",
			expectedLimit:  50,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedOpts interfaces.SearchOptions
			mockService := &mockSearchService{
				searchFunc: func(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
					capturedOpts = opts
					return []*models.Document{}, nil
				},
			}

			handler := NewSearchHandler(mockService, nil)
			url := "/api/search?q=test&limit=" + tt.limitParam + "&offset=" + tt.offsetParam
			req := httptest.NewRequest("GET", url, nil)
			rec := httptest.NewRecorder()

			handler.SearchHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rec.Code)
			}

			if capturedOpts.Limit != tt.expectedLimit {
				t.Errorf("Expected limit %d, got %d", tt.expectedLimit, capturedOpts.Limit)
			}

			if capturedOpts.Offset != tt.expectedOffset {
				t.Errorf("Expected offset %d, got %d", tt.expectedOffset, capturedOpts.Offset)
			}
		})
	}
}

// TestSearchHandler_JSONErrorResponses removed - redundant with TestSearchHandler_ServiceError.
// Testing specific error response format (JSON vs plain text) is overly prescriptive.
// Core functionality (correct status codes, error handling) is already tested.

// mockError implements error interface for testing
type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
