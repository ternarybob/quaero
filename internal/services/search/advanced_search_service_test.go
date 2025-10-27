package search

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// mockAdvancedDocumentStorage extends mockDocumentStorage with advanced FTS5 query handling
type mockAdvancedDocumentStorage struct {
	documents []*models.Document
}

func (m *mockAdvancedDocumentStorage) SaveDocument(doc *models.Document) error {
	m.documents = append(m.documents, doc)
	return nil
}

func (m *mockAdvancedDocumentStorage) GetDocument(id string) (*models.Document, error) {
	for _, doc := range m.documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *mockAdvancedDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	// Enhanced mock implementation that handles FTS5 syntax
	// Supports: OR, AND, phrases (quoted strings)
	var results []*models.Document

	if query == "" {
		results = m.documents
	} else {
		// Parse FTS5 query (simplified - handles common cases)
		for _, doc := range m.documents {
			if matchesFTS5Query(doc, query) {
				results = append(results, doc)
			}
		}
	}

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// matchesFTS5Query performs simplified FTS5 query matching
// Handles: AND, OR, quoted phrases
func matchesFTS5Query(doc *models.Document, query string) bool {
	// Search in title, content, and source ID (for references)
	content := strings.ToLower(doc.Title + " " + doc.ContentMarkdown + " " + doc.SourceID)

	// Handle AND queries (e.g., "cat AND dog")
	if strings.Contains(query, " AND ") {
		parts := strings.Split(query, " AND ")
		for _, part := range parts {
			term := strings.TrimSpace(part)
			term = strings.Trim(term, "()")
			term = unquote(term)

			// Check if this is an OR subquery
			if strings.Contains(term, " OR ") {
				if !matchesORQuery(content, term) {
					return false
				}
			} else {
				if !strings.Contains(content, strings.ToLower(term)) {
					return false
				}
			}
		}
		return true
	}

	// Handle OR queries (e.g., "cat OR dog")
	if strings.Contains(query, " OR ") {
		return matchesORQuery(content, query)
	}

	// Simple term match
	term := unquote(query)
	return strings.Contains(content, strings.ToLower(term))
}

// matchesORQuery checks if content matches any term in OR query
func matchesORQuery(content, query string) bool {
	parts := strings.Split(query, " OR ")
	for _, part := range parts {
		term := strings.TrimSpace(part)
		term = unquote(term)
		if strings.Contains(content, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// unquote removes quotes from a string
// Also handles escaped quotes ("")
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		// Remove outer quotes
		inner := s[1 : len(s)-1]
		// Handle escaped quotes ("" -> ")
		inner = strings.ReplaceAll(inner, `""`, `"`)
		return inner
	}
	return s
}

// Stub methods for DocumentStorage interface
func (m *mockAdvancedDocumentStorage) UpdateDocument(doc *models.Document) error   { return nil }
func (m *mockAdvancedDocumentStorage) DeleteDocument(id string) error              { return nil }
func (m *mockAdvancedDocumentStorage) Close() error                                { return nil }
func (m *mockAdvancedDocumentStorage) SaveDocuments(docs []*models.Document) error { return nil }
func (m *mockAdvancedDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) VectorSearch(embedding []float32, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) HybridSearch(query string, embedding []float32, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	return m.documents, nil
}
func (m *mockAdvancedDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) CountDocuments() (int, error) {
	return len(m.documents), nil
}
func (m *mockAdvancedDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
	return 0, nil
}
func (m *mockAdvancedDocumentStorage) CountVectorized() (int, error) { return 0, nil }
func (m *mockAdvancedDocumentStorage) GetStats() (*models.DocumentStats, error) {
	return &models.DocumentStats{TotalDocuments: len(m.documents)}, nil
}
func (m *mockAdvancedDocumentStorage) SetForceSyncPending(id string, pending bool) error {
	return nil
}
func (m *mockAdvancedDocumentStorage) SetForceEmbedPending(id string, pending bool) error {
	return nil
}
func (m *mockAdvancedDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) GetDocumentsForceEmbed(limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) GetUnvectorizedDocuments(limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockAdvancedDocumentStorage) ClearAllEmbeddings() (int, error) { return 0, nil }
func (m *mockAdvancedDocumentStorage) ClearAll() error                  { return nil }

// Test data
func getTestDocuments() []*models.Document {
	return []*models.Document{
		{
			ID:              "doc_1",
			SourceType:      "jira",
			SourceID:        "PROJ-123",
			Title:           "Bug in authentication",
			ContentMarkdown: "Users cannot log in due to authentication service failure",
			Metadata:        map[string]interface{}{"project": "PROJ", "type": "bug"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "doc_2",
			SourceType:      "confluence",
			SourceID:        "page_456",
			Title:           "API Documentation",
			ContentMarkdown: "Complete guide to authentication API endpoints",
			Metadata:        map[string]interface{}{"space": "DOCS"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "doc_3",
			SourceType:      "jira",
			SourceID:        "PROJ-124",
			Title:           "Feature request for dashboard",
			ContentMarkdown: "Add new widgets to dashboard interface",
			Metadata:        map[string]interface{}{"project": "PROJ", "type": "feature"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "doc_4",
			SourceType:      "confluence",
			SourceID:        "page_789",
			Title:           "Cat and Dog Training Guide",
			ContentMarkdown: "How to train your cat and dog to get along. The cat sat on the mat while the dog played.",
			Metadata:        map[string]interface{}{"space": "PETS"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              "doc_5",
			SourceType:      "jira",
			SourceID:        "PROJ-125",
			Title:           "CAT Protocol Implementation",
			ContentMarkdown: "Implement CAT network protocol for internal services",
			Metadata:        map[string]interface{}{"project": "PROJ", "type": "task"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
}

func TestParseQuery_SimpleOR(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name          string
		query         string
		expectedFTS5  string
		expectedTerms int
	}{
		{
			name:          "Two terms OR",
			query:         "cat dog",
			expectedFTS5:  "cat OR dog",
			expectedTerms: 2,
		},
		{
			name:          "Three terms OR",
			query:         "cat dog mat",
			expectedFTS5:  "cat OR dog OR mat",
			expectedTerms: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.FTS5Query != tt.expectedFTS5 {
				t.Errorf("Expected FTS5 query %q, got %q", tt.expectedFTS5, parsed.FTS5Query)
			}

			// Count non-qualifier tokens
			nonQualifierCount := 0
			for _, token := range parsed.Tokens {
				if token.Type != TokenTypeQualifier {
					nonQualifierCount++
				}
			}

			if nonQualifierCount != tt.expectedTerms {
				t.Errorf("Expected %d terms, got %d", tt.expectedTerms, nonQualifierCount)
			}
		})
	}
}

func TestParseQuery_RequiredAND(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name         string
		query        string
		expectedFTS5 string
	}{
		{
			name:         "Two required terms",
			query:        "+cat +dog",
			expectedFTS5: "cat AND dog",
		},
		{
			name:         "Three required terms",
			query:        "+cat +dog +mat",
			expectedFTS5: "cat AND dog AND mat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.FTS5Query != tt.expectedFTS5 {
				t.Errorf("Expected FTS5 query %q, got %q", tt.expectedFTS5, parsed.FTS5Query)
			}
		})
	}
}

func TestParseQuery_MixedANDOR(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name         string
		query        string
		expectedFTS5 string
	}{
		{
			name:         "One required, two optional",
			query:        "+cat dog mat",
			expectedFTS5: "cat AND (dog OR mat)",
		},
		{
			name:         "Two required, one optional",
			query:        "+cat +dog mat",
			expectedFTS5: "cat AND dog AND (mat)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.FTS5Query != tt.expectedFTS5 {
				t.Errorf("Expected FTS5 query %q, got %q", tt.expectedFTS5, parsed.FTS5Query)
			}
		})
	}
}

func TestParseQuery_QuotedPhrase(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name         string
		query        string
		expectedFTS5 string
	}{
		{
			name:         "Simple phrase",
			query:        `"cat on mat"`,
			expectedFTS5: `"cat on mat"`,
		},
		{
			name:         "Phrase with other terms",
			query:        `"cat on mat" dog`,
			expectedFTS5: `"cat on mat" OR dog`,
		},
		{
			name:         "Required phrase",
			query:        `+"cat on mat" dog`,
			expectedFTS5: `"cat on mat" AND (dog)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.FTS5Query != tt.expectedFTS5 {
				t.Errorf("Expected FTS5 query %q, got %q", tt.expectedFTS5, parsed.FTS5Query)
			}
		})
	}
}

func TestParseQuery_Qualifiers(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name            string
		query           string
		expectedDocType string
		expectedFTS5    string
	}{
		{
			name:            "document_type qualifier",
			query:           "document_type:jira cat",
			expectedDocType: "jira",
			expectedFTS5:    "cat",
		},
		{
			name:            "type qualifier (alias)",
			query:           "type:confluence dog",
			expectedDocType: "confluence",
			expectedFTS5:    "dog",
		},
		{
			name:            "Qualifier without search terms",
			query:           "document_type:jira",
			expectedDocType: "jira",
			expectedFTS5:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.DocumentType != tt.expectedDocType {
				t.Errorf("Expected document type %q, got %q", tt.expectedDocType, parsed.DocumentType)
			}

			if parsed.FTS5Query != tt.expectedFTS5 {
				t.Errorf("Expected FTS5 query %q, got %q", tt.expectedFTS5, parsed.FTS5Query)
			}
		})
	}
}

func TestParseQuery_CaseMatch(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	tests := []struct {
		name             string
		query            string
		expectedCaseSens bool
	}{
		{
			name:             "Case match enabled",
			query:            "case:match Cat",
			expectedCaseSens: true,
		},
		{
			name:             "Case match not specified",
			query:            "cat",
			expectedCaseSens: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := service.parseQuery(tt.query)

			if parsed.CaseSensitive != tt.expectedCaseSens {
				t.Errorf("Expected case sensitive %v, got %v", tt.expectedCaseSens, parsed.CaseSensitive)
			}
		})
	}
}

func TestParseQuery_EmptyQuery(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	parsed := service.parseQuery("")

	if parsed.FTS5Query != "" {
		t.Errorf("Expected empty FTS5 query, got %q", parsed.FTS5Query)
	}
}

func TestSearch_SimpleQuery(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	results, err := service.Search(ctx, "authentication", interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestSearch_WithDocumentTypeFilter(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	results, err := service.Search(ctx, "document_type:jira authentication", interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result (filtered by jira), got %d", len(results))
	}

	if len(results) > 0 && results[0].SourceType != "jira" {
		t.Errorf("Expected jira source type, got %s", results[0].SourceType)
	}
}

func TestSearch_WithCaseSensitive(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	// Search for uppercase "CAT" with case:match
	results, err := service.Search(ctx, "case:match CAT", interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only match doc_5 which contains "CAT Protocol"
	if len(results) != 1 {
		t.Errorf("Expected 1 result (case-sensitive CAT), got %d", len(results))
	}

	if len(results) > 0 && results[0].ID != "doc_5" {
		t.Errorf("Expected doc_5 (CAT Protocol), got %s", results[0].ID)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	results, err := service.Search(ctx, "", interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Empty query should return all documents
	if len(results) != 5 {
		t.Errorf("Expected 5 results (all documents), got %d", len(results))
	}
}

func TestSearch_NoResults(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	results, err := service.Search(ctx, "nonexistent", interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestSearch_ComplexQuery(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	// Complex query: +cat dog "on the mat" document_type:confluence
	results, err := service.Search(ctx, `+cat dog document_type:confluence`, interfaces.SearchOptions{Limit: 10})

	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should match doc_4 (confluence, contains cat and dog)
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].ID != "doc_4" {
		t.Errorf("Expected doc_4, got %s", results[0].ID)
	}
}

func TestEdgeCase_UnbalancedQuotes(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	// Unbalanced quote should still parse (treat as phrase)
	parsed := service.parseQuery(`"cat dog`)

	// Should treat unbalanced quote as phrase anyway
	if parsed.FTS5Query == "" {
		t.Error("Expected non-empty FTS5 query for unbalanced quote")
	}
}

func TestEdgeCase_MultipleQualifiers(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	parsed := service.parseQuery("document_type:jira case:match cat")

	if parsed.DocumentType != "jira" {
		t.Errorf("Expected document type jira, got %q", parsed.DocumentType)
	}

	if !parsed.CaseSensitive {
		t.Error("Expected case sensitive to be true")
	}

	if parsed.FTS5Query != "cat" {
		t.Errorf("Expected FTS5 query 'cat', got %q", parsed.FTS5Query)
	}
}

func TestEdgeCase_OnlyPlusSign(t *testing.T) {
	service := NewAdvancedSearchService(&mockAdvancedDocumentStorage{}, nil)

	parsed := service.parseQuery("+")

	if parsed.FTS5Query != "" {
		t.Errorf("Expected empty FTS5 query for lone +, got %q", parsed.FTS5Query)
	}
}

func TestAdvancedSearchService_GetByID(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	doc, err := service.GetByID(ctx, "doc_1")

	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if doc == nil {
		t.Fatal("Expected document, got nil")
	}

	if doc.ID != "doc_1" {
		t.Errorf("Expected doc_1, got %s", doc.ID)
	}
}

func TestAdvancedSearchService_SearchByReference(t *testing.T) {
	ctx := context.Background()
	storage := &mockAdvancedDocumentStorage{documents: getTestDocuments()}
	service := NewAdvancedSearchService(storage, nil)

	t.Run("Simple reference", func(t *testing.T) {
		results, err := service.SearchByReference(ctx, "PROJ-123", interfaces.SearchOptions{Limit: 10})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		// Should find doc_1 which contains PROJ-123
		if len(results) == 0 {
			t.Error("Expected at least 1 result")
		}
	})

	t.Run("Reference with embedded quote", func(t *testing.T) {
		// Add a document with a quote in the reference
		docs := append(getTestDocuments(), &models.Document{
			ID:              "doc_quote",
			SourceType:      "jira",
			SourceID:        "PROJ-126",
			Title:           `Issue with "quotes" in title`,
			ContentMarkdown: `This issue contains "quoted text" in the content`,
			Metadata:        map[string]interface{}{"project": "PROJ"},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		})
		storage := &mockAdvancedDocumentStorage{documents: docs}
		service := NewAdvancedSearchService(storage, nil)

		// Search for reference containing a quote
		results, err := service.SearchByReference(ctx, `"quotes"`, interfaces.SearchOptions{Limit: 10})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		// Should find the document with quotes
		if len(results) == 0 {
			t.Error("Expected at least 1 result for reference with quotes")
		}

		// Verify it found the right document
		foundQuoteDoc := false
		for _, doc := range results {
			if doc.ID == "doc_quote" {
				foundQuoteDoc = true
				break
			}
		}

		if !foundQuoteDoc {
			t.Error("Expected to find doc_quote in results")
		}
	})
}
