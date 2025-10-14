package api

import (
	"context"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/search"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

// setupTestDB creates a test database for integration testing
func setupTestDB(t *testing.T) (interfaces.DocumentStorage, func()) {
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	config := &common.SQLiteConfig{
		Path:               dbPath,
		EnableFTS5:         true,
		EnableVector:       false,
		EmbeddingDimension: 768,
		CacheSizeMB:        50,
		WALMode:            false,
		BusyTimeoutMS:      5000,
	}

	logger := arbor.NewLogger()

	db, err := sqlite.NewSQLiteDB(logger, config)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	storage := sqlite.NewDocumentStorage(db, logger)

	cleanup := func() {
		db.Close()
	}

	return storage, cleanup
}

// Helper function to convert []interface{} to []string
func interfaceSliceToStringSlice(slice interface{}) ([]string, bool) {
	interfaceSlice, ok := slice.([]interface{})
	if !ok {
		// Try direct []string cast
		stringSlice, ok := slice.([]string)
		return stringSlice, ok
	}

	result := make([]string, len(interfaceSlice))
	for i, v := range interfaceSlice {
		str, ok := v.(string)
		if !ok {
			return nil, false
		}
		result[i] = str
	}
	return result, true
}

// TestMetadataExtraction_Integration tests that metadata extraction works
// when documents are saved through DocumentService
func TestMetadataExtraction_Integration(t *testing.T) {
	// Setup: Create storage and services
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()

	// Create DocumentService (which has MetadataExtractor integrated)
	docService := documents.NewService(storage, logger)

	ctx := context.Background()

	t.Run("Extract Jira keys from document content", func(t *testing.T) {
		doc := &models.Document{
			Title:      "Fix bug in PROJ-123",
			Content:    "This addresses PROJ-123 and is related to PROJ-456",
			SourceType: "jira",
			SourceID:   "PROJ-123",
			Metadata:   map[string]interface{}{"project": "PROJ"},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Save document (should automatically extract metadata)
		err := docService.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		// Retrieve document from storage
		saved, err := storage.GetDocument(doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document: %v", err)
		}

		// Verify metadata was extracted and merged
		if saved.Metadata == nil {
			t.Fatal("Expected metadata to be populated")
		}

		// Check that original metadata is preserved
		if saved.Metadata["project"] != "PROJ" {
			t.Errorf("Expected project=PROJ, got %v", saved.Metadata["project"])
		}

		// Check that issue_keys were extracted
		issueKeys, ok := saved.Metadata["issue_keys"]
		if !ok {
			t.Fatal("Expected issue_keys to be extracted")
		}

		keys, ok := interfaceSliceToStringSlice(issueKeys)
		if !ok {
			t.Fatalf("Expected issue_keys to be convertible to []string, got %T", issueKeys)
		}

		if len(keys) != 2 {
			t.Errorf("Expected 2 issue keys, got %d: %v", len(keys), keys)
		}

		// Verify both keys are present
		foundPROJ123 := false
		foundPROJ456 := false
		for _, key := range keys {
			if key == "PROJ-123" {
				foundPROJ123 = true
			}
			if key == "PROJ-456" {
				foundPROJ456 = true
			}
		}

		if !foundPROJ123 || !foundPROJ456 {
			t.Errorf("Expected PROJ-123 and PROJ-456, got %v", keys)
		}
	})

	t.Run("Extract user mentions from document content", func(t *testing.T) {
		doc := &models.Document{
			Title:      "Code review requested",
			Content:    "Please review @alice and cc @bob for approval",
			SourceType: "confluence",
			SourceID:   "page_123",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		err := docService.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		saved, err := storage.GetDocument(doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document: %v", err)
		}

		mentions, ok := saved.Metadata["mentions"]
		if !ok {
			t.Fatal("Expected mentions to be extracted")
		}

		mentionsList, ok := interfaceSliceToStringSlice(mentions)
		if !ok {
			t.Fatalf("Expected mentions to be convertible to []string, got %T", mentions)
		}

		if len(mentionsList) != 2 {
			t.Errorf("Expected 2 mentions, got %d: %v", len(mentionsList), mentionsList)
		}
	})

	t.Run("Extract PR references from document content", func(t *testing.T) {
		doc := &models.Document{
			Title:      "Bug fixes",
			Content:    "Fixes #123 and implements #456",
			SourceType: "github",
			SourceID:   "commit_abc",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		err := docService.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		saved, err := storage.GetDocument(doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document: %v", err)
		}

		prRefs, ok := saved.Metadata["pr_refs"]
		if !ok {
			t.Fatal("Expected pr_refs to be extracted")
		}

		prList, ok := interfaceSliceToStringSlice(prRefs)
		if !ok {
			t.Fatalf("Expected pr_refs to be convertible to []string, got %T", prRefs)
		}

		if len(prList) != 2 {
			t.Errorf("Expected 2 PR refs, got %d: %v", len(prList), prList)
		}
	})

	t.Run("Batch save with metadata extraction", func(t *testing.T) {
		docs := []*models.Document{
			{
				Title:      "PROJ-100: First issue",
				Content:    "Content mentioning PROJ-100",
				SourceType: "jira",
				SourceID:   "PROJ-100",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			{
				Title:      "PROJ-200: Second issue",
				Content:    "Content mentioning PROJ-200 and @charlie",
				SourceType: "jira",
				SourceID:   "PROJ-200",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
		}

		err := docService.SaveDocuments(ctx, docs)
		if err != nil {
			t.Fatalf("Failed to save documents: %v", err)
		}

		// Verify first document
		saved1, err := storage.GetDocument(docs[0].ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document 1: %v", err)
		}

		if saved1.Metadata["issue_keys"] == nil {
			t.Error("Expected issue_keys to be extracted from doc 1")
		}

		// Verify second document
		saved2, err := storage.GetDocument(docs[1].ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document 2: %v", err)
		}

		if saved2.Metadata["issue_keys"] == nil {
			t.Error("Expected issue_keys to be extracted from doc 2")
		}

		if saved2.Metadata["mentions"] == nil {
			t.Error("Expected mentions to be extracted from doc 2")
		}
	})
}

// TestSearchService_Integration tests FTS5SearchService with actual storage
func TestSearchService_Integration(t *testing.T) {
	// Setup: Create storage and populate with test data
	storage, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()

	// Create and populate test documents
	docService := documents.NewService(storage, logger)
	ctx := context.Background()

	testDocs := []*models.Document{
		{
			Title:      "Bug in authentication",
			Content:    "Users cannot log in due to authentication service failure",
			SourceType: "jira",
			SourceID:   "PROJ-123",
			Metadata: map[string]interface{}{
				"project": "PROJ",
				"type":    "bug",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Title:      "API Documentation",
			Content:    "Complete guide to authentication API endpoints",
			SourceType: "confluence",
			SourceID:   "page_456",
			Metadata: map[string]interface{}{
				"space": "DOCS",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Title:      "Feature request for dashboard",
			Content:    "Add new widgets to dashboard interface",
			SourceType: "jira",
			SourceID:   "PROJ-124",
			Metadata: map[string]interface{}{
				"project": "PROJ",
				"type":    "feature",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	err := docService.SaveDocuments(ctx, testDocs)
	if err != nil {
		t.Fatalf("Failed to save test documents: %v", err)
	}

	// Create SearchService
	searchService := search.NewFTS5SearchService(storage, nil)

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

		if len(results) > 0 && results[0].SourceID != "PROJ-123" {
			t.Errorf("Expected PROJ-123, got %s", results[0].SourceID)
		}
	})

	t.Run("Get document by ID", func(t *testing.T) {
		docID := testDocs[0].ID

		doc, err := searchService.GetByID(ctx, docID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if doc == nil {
			t.Fatal("Expected document, got nil")
		}

		if doc.ID != docID {
			t.Errorf("Expected doc ID %s, got %s", docID, doc.ID)
		}
	})

	t.Run("Search by reference with extracted metadata", func(t *testing.T) {
		// Save a document with reference that will be extracted
		doc := &models.Document{
			Title:      "Related work",
			Content:    "This is related to PROJ-123 reported by @alice",
			SourceType: "confluence",
			SourceID:   "page_789",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		err := docService.SaveDocument(ctx, doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		// Search by the Jira key that was extracted
		results, err := searchService.SearchByReference(ctx, "PROJ-123", interfaces.SearchOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		// Should find the document we just saved and the original Jira doc from test setup
		if len(results) < 1 {
			t.Errorf("Expected at least 1 result for PROJ-123, got %d", len(results))
		}

		// Verify the document we just saved is in the results
		foundNew := false
		for _, r := range results {
			if r.SourceID == "page_789" {
				foundNew = true
				break
			}
		}
		if !foundNew {
			t.Error("Expected to find the newly saved document in search results")
		}

		// Search by user mention
		results, err = searchService.SearchByReference(ctx, "@alice", interfaces.SearchOptions{
			Limit: 10,
		})

		if err != nil {
			t.Fatalf("SearchByReference failed: %v", err)
		}

		if len(results) < 1 {
			t.Errorf("Expected at least 1 result for @alice, got %d", len(results))
		}
	})
}
