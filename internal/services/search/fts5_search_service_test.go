package search

import (
	"context"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// mockDocumentStorage implements interfaces.DocumentStorage for testing
type mockDocumentStorage struct {
	documents []*models.Document
}

func (m *mockDocumentStorage) SaveDocument(doc *models.Document) error {
	m.documents = append(m.documents, doc)
	return nil
}

func (m *mockDocumentStorage) GetDocument(id string) (*models.Document, error) {
	for _, doc := range m.documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *mockDocumentStorage) UpdateDocument(doc *models.Document) error {
	for i, d := range m.documents {
		if d.ID == doc.ID {
			m.documents[i] = doc
			return nil
		}
	}
	return nil
}

func (m *mockDocumentStorage) DeleteDocument(id string) error {
	for i, doc := range m.documents {
		if doc.ID == id {
			m.documents = append(m.documents[:i], m.documents[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockDocumentStorage) Close() error {
	return nil
}

// Stub methods for DocumentStorage interface (not used in these tests)
func (m *mockDocumentStorage) SaveDocuments(docs []*models.Document) error { return nil }
func (m *mockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	// Simple mock implementation - return all documents if query matches content
	// Strip quotes from query if present (FTS5 quoted phrases)
	searchQuery := query
	if len(query) >= 2 && query[0] == '"' && query[len(query)-1] == '"' {
		searchQuery = query[1 : len(query)-1]
	}

	var results []*models.Document
	for _, doc := range m.documents {
		if query == "" || containsSubstring(doc.ContentMarkdown, searchQuery) || containsSubstring(doc.Title, searchQuery) {
			results = append(results, doc)
		}
	}

	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
func (m *mockDocumentStorage) VectorSearch(embedding []float32, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) HybridSearch(query string, embedding []float32, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	return m.documents, nil
}
func (m *mockDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) CountDocuments() (int, error)                          { return len(m.documents), nil }
func (m *mockDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) { return 0, nil }
func (m *mockDocumentStorage) CountVectorized() (int, error)                         { return 0, nil }
func (m *mockDocumentStorage) GetStats() (*models.DocumentStats, error) {
	return &models.DocumentStats{
		TotalDocuments: len(m.documents),
	}, nil
}
func (m *mockDocumentStorage) SetForceSyncPending(id string, pending bool) error  { return nil }
func (m *mockDocumentStorage) SetForceEmbedPending(id string, pending bool) error { return nil }
func (m *mockDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) { return nil, nil }
func (m *mockDocumentStorage) GetDocumentsForceEmbed(limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) GetUnvectorizedDocuments(limit int) ([]*models.Document, error) {
	return nil, nil
}
func (m *mockDocumentStorage) ClearAllEmbeddings() (int, error) { return 0, nil }
func (m *mockDocumentStorage) ClearAll() error                  { return nil }
func (m *mockDocumentStorage) RebuildFTS5Index() error          { return nil }
func (m *mockDocumentStorage) GetAllTags() ([]string, error)   { return []string{}, nil }

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && contains(s, substr)))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFTS5SearchService_Search(t *testing.T) {
	ctx := context.Background()

	// Create mock storage with sample documents
	storage := &mockDocumentStorage{
		documents: []*models.Document{
			{
				ID:              "doc_1",
				SourceType:      "jira",
				SourceID:        "PROJ-123",
				Title:           "Bug in authentication",
				ContentMarkdown: "Users cannot log in due to authentication service failure",
				Metadata: map[string]interface{}{
					"project": "PROJ",
					"type":    "bug",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:              "doc_2",
				SourceType:      "confluence",
				SourceID:        "page_456",
				Title:           "API Documentation",
				ContentMarkdown: "Complete guide to authentication API endpoints",
				Metadata: map[string]interface{}{
					"space": "DOCS",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:              "doc_3",
				SourceType:      "jira",
				SourceID:        "PROJ-124",
				Title:           "Feature request for dashboard",
				ContentMarkdown: "Add new widgets to dashboard interface",
				Metadata: map[string]interface{}{
					"project": "PROJ",
					"type":    "feature",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	// Create search service
	searchService := NewFTS5SearchService(storage, nil)

	t.Run("Search by keyword", func(t *testing.T) {
		results, err := searchService.Search(ctx, "authentication", interfaces.SearchOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("Search with limit", func(t *testing.T) {
		results, err := searchService.Search(ctx, "authentication", interfaces.SearchOptions{
			Limit: 1,
		})

		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result due to limit, got %d", len(results))
		}
	})

	t.Run("Search with source type filter", func(t *testing.T) {
		results, err := searchService.Search(ctx, "", interfaces.SearchOptions{
			Limit:       10,
			SourceTypes: []string{"jira"},
		})

		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 Jira results, got %d", len(results))
		}

		for _, doc := range results {
			if doc.SourceType != "jira" {
				t.Errorf("Expected jira source type, got %s", doc.SourceType)
			}
		}
	})

	t.Run("Search with metadata filter", func(t *testing.T) {
		results, err := searchService.Search(ctx, "", interfaces.SearchOptions{
			Limit: 10,
			MetadataFilters: map[string]string{
				"type": "bug",
			},
		})

		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 bug result, got %d", len(results))
		}

		if len(results) > 0 && results[0].ID != "doc_1" {
			t.Errorf("Expected doc_1, got %s", results[0].ID)
		}
	})
}

func TestFTS5SearchService_GetByID(t *testing.T) {
	ctx := context.Background()

	storage := &mockDocumentStorage{
		documents: []*models.Document{
			{
				ID:              "doc_1",
				SourceType:      "jira",
				Title:           "Test Document",
				ContentMarkdown: "Test content",
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			},
		},
	}

	searchService := NewFTS5SearchService(storage, nil)

	t.Run("Get existing document", func(t *testing.T) {
		doc, err := searchService.GetByID(ctx, "doc_1")

		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if doc == nil {
			t.Fatal("Expected document, got nil")
		}

		if doc.ID != "doc_1" {
			t.Errorf("Expected doc_1, got %s", doc.ID)
		}
	})

	t.Run("Get non-existent document", func(t *testing.T) {
		doc, err := searchService.GetByID(ctx, "doc_nonexistent")

		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if doc != nil {
			t.Errorf("Expected nil, got document %s", doc.ID)
		}
	})
}

func TestFTS5SearchService_SearchByReference(t *testing.T) {
	ctx := context.Background()

	storage := &mockDocumentStorage{
		documents: []*models.Document{
			{
				ID:              "doc_1",
				SourceType:      "jira",
				Title:           "PROJ-123: Bug fix",
				ContentMarkdown: "Fixed issue PROJ-123 as reported by @alice",
				Metadata: map[string]interface{}{
					"issue_keys": []string{"PROJ-123"},
					"mentions":   []string{"@alice"},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:              "doc_2",
				SourceType:      "confluence",
				Title:           "Meeting notes",
				ContentMarkdown: "Discussed PROJ-123 and PROJ-124 with @bob",
				Metadata: map[string]interface{}{
					"issue_keys": []string{"PROJ-123", "PROJ-124"},
					"mentions":   []string{"@bob"},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	searchService := NewFTS5SearchService(storage, nil)

	t.Run("Search by Jira issue reference", func(t *testing.T) {
		results, err := searchService.SearchByReference(ctx, "PROJ-123", interfaces.SearchOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("Search by user mention", func(t *testing.T) {
		results, err := searchService.SearchByReference(ctx, "@alice", interfaces.SearchOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})
}
