package sqlite

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) (*SQLiteDB, func()) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := tempDir + "/test.db"

	// Create config
	config := &common.SQLiteConfig{
		Path:               dbPath,
		EnableFTS5:         true,
		EnableVector:       false,
		EmbeddingDimension: 768,
		CacheSizeMB:        50,
		WALMode:            false, // Disable WAL for simpler test cleanup
		BusyTimeoutMS:      5000,
	}

	// Create logger
	logger := arbor.NewLogger()

	// Create database
	db, err := NewSQLiteDB(logger, config)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

// TestSearchByIdentifier tests the SearchByIdentifier method with various scenarios
func TestSearchByIdentifier(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	storage := NewDocumentStorage(db, arbor.NewLogger())

	// Create test documents
	now := time.Now()

	// Document 1: Jira issue with issue_key in metadata
	jiraDoc := &models.Document{
		ID:         "jira-1",
		SourceType: "jira",
		SourceID:   "BUG-123",
		Title:      "Authentication bug in login flow",
		Content:    "Users cannot log in when using SSO authentication",
		Metadata: map[string]interface{}{
			"issue_key":   "BUG-123",
			"project_key": "BUG",
			"issue_type":  "Bug",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Document 2: Confluence page referencing BUG-123 in metadata.referenced_issues
	confluenceDoc := &models.Document{
		ID:         "confluence-1",
		SourceType: "confluence",
		SourceID:   "12345",
		Title:      "Authentication Architecture",
		Content:    "This page documents the authentication system design.",
		Metadata: map[string]interface{}{
			"space_key":         "TECH",
			"referenced_issues": []interface{}{"BUG-123", "STORY-456"},
		},
		CreatedAt: now.Add(-1 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}

	// Document 3: Another Confluence page with BUG-123 in title
	confluenceDoc2 := &models.Document{
		ID:         "confluence-2",
		SourceType: "confluence",
		SourceID:   "12346",
		Title:      "Fix for bug-123 deployed to production",
		Content:    "The fix has been deployed successfully.",
		Metadata:   map[string]interface{}{},
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-2 * time.Hour),
	}

	// Document 4: GitHub commit with BUG-123 in content
	githubDoc := &models.Document{
		ID:         "github-1",
		SourceType: "github",
		SourceID:   "abc123def456",
		Title:      "Fix authentication timeout",
		Content:    "This commit resolves BUG-123 by increasing the session timeout to 30 minutes.",
		Metadata: map[string]interface{}{
			"commit_sha": "abc123def456",
			"author":     "John Doe",
		},
		CreatedAt: now.Add(-3 * time.Hour),
		UpdatedAt: now.Add(-3 * time.Hour),
	}

	// Document 5: Unrelated document (should not match)
	unrelatedDoc := &models.Document{
		ID:         "jira-2",
		SourceType: "jira",
		SourceID:   "STORY-999",
		Title:      "New feature request",
		Content:    "Implement dark mode for the application.",
		Metadata: map[string]interface{}{
			"issue_key":   "STORY-999",
			"project_key": "STORY",
		},
		CreatedAt: now.Add(-4 * time.Hour),
		UpdatedAt: now.Add(-4 * time.Hour),
	}

	// Save all documents
	docs := []*models.Document{jiraDoc, confluenceDoc, confluenceDoc2, githubDoc, unrelatedDoc}
	if err := storage.SaveDocuments(docs); err != nil {
		t.Fatalf("Failed to save test documents: %v", err)
	}

	tests := []struct {
		name           string
		identifier     string
		excludeSources []string
		limit          int
		expectedIDs    []string // Expected document IDs in any order
	}{
		{
			name:           "Find all documents referencing BUG-123",
			identifier:     "BUG-123",
			excludeSources: []string{},
			limit:          10,
			expectedIDs:    []string{"jira-1", "confluence-1", "confluence-2", "github-1"},
		},
		{
			name:           "Find BUG-123 excluding Jira source",
			identifier:     "BUG-123",
			excludeSources: []string{"jira"},
			limit:          10,
			expectedIDs:    []string{"confluence-1", "confluence-2", "github-1"},
		},
		{
			name:           "Find BUG-123 excluding Confluence source",
			identifier:     "BUG-123",
			excludeSources: []string{"confluence"},
			limit:          10,
			expectedIDs:    []string{"jira-1", "github-1"},
		},
		{
			name:           "Find BUG-123 excluding multiple sources",
			identifier:     "BUG-123",
			excludeSources: []string{"jira", "confluence"},
			limit:          10,
			expectedIDs:    []string{"github-1"},
		},
		{
			name:           "Find BUG-123 with limit 2",
			identifier:     "BUG-123",
			excludeSources: []string{},
			limit:          2,
			expectedIDs:    []string{"jira-1", "confluence-1"}, // Most recent 2
		},
		{
			name:           "Find STORY-456 (in referenced_issues)",
			identifier:     "STORY-456",
			excludeSources: []string{},
			limit:          10,
			expectedIDs:    []string{"confluence-1"},
		},
		{
			name:           "Search for non-existent identifier",
			identifier:     "NOTFOUND-999",
			excludeSources: []string{},
			limit:          10,
			expectedIDs:    []string{},
		},
		{
			name:           "Empty identifier",
			identifier:     "",
			excludeSources: []string{},
			limit:          10,
			expectedIDs:    []string{},
		},
		{
			name:           "Case-insensitive search (bug-123 lowercase)",
			identifier:     "bug-123",
			excludeSources: []string{},
			limit:          10,
			expectedIDs:    []string{"jira-1", "confluence-1", "confluence-2", "github-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := storage.SearchByIdentifier(tt.identifier, tt.excludeSources, tt.limit)
			if err != nil {
				t.Fatalf("SearchByIdentifier failed: %v", err)
			}

			// Check result count
			if len(results) != len(tt.expectedIDs) {
				t.Errorf("Expected %d results, got %d. Results: %v",
					len(tt.expectedIDs), len(results), docIDsToString(results))
				return
			}

			// Verify all expected IDs are present (order not important)
			resultIDs := make(map[string]bool)
			for _, doc := range results {
				resultIDs[doc.ID] = true
			}

			for _, expectedID := range tt.expectedIDs {
				if !resultIDs[expectedID] {
					t.Errorf("Expected document ID %q not found in results: %v",
						expectedID, docIDsToString(results))
				}
			}

			// Verify no unexpected IDs
			for _, doc := range results {
				found := false
				for _, expectedID := range tt.expectedIDs {
					if doc.ID == expectedID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Unexpected document ID %q in results", doc.ID)
				}
			}
		})
	}
}

// TestSearchByIdentifierWithReferencedIssuesAsStringArray tests handling of referenced_issues as []string
func TestSearchByIdentifierWithReferencedIssuesAsStringArray(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	storage := NewDocumentStorage(db, arbor.NewLogger())

	now := time.Now()

	// Create document with referenced_issues as []string (alternative serialization)
	doc := &models.Document{
		ID:         "test-1",
		SourceType: "confluence",
		SourceID:   "99999",
		Title:      "Test Document",
		Content:    "Test content",
		Metadata: map[string]interface{}{
			"referenced_issues": []string{"TASK-100", "TASK-200"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := storage.SaveDocument(doc); err != nil {
		t.Fatalf("Failed to save document: %v", err)
	}

	// Search for TASK-100
	results, err := storage.SearchByIdentifier("TASK-100", []string{}, 10)
	if err != nil {
		t.Fatalf("SearchByIdentifier failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].ID != "test-1" {
		t.Errorf("Expected document test-1, got %s", results[0].ID)
	}
}

// TestSearchByIdentifierMetadataIntegrity verifies metadata is preserved correctly
func TestSearchByIdentifierMetadataIntegrity(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	storage := NewDocumentStorage(db, arbor.NewLogger())

	now := time.Now()

	// Create document with complex metadata
	doc := &models.Document{
		ID:         "meta-test-1",
		SourceType: "jira",
		SourceID:   "TEST-500",
		Title:      "Metadata Test",
		Content:    "Testing metadata preservation",
		Metadata: map[string]interface{}{
			"issue_key":         "TEST-500",
			"project_key":       "TEST",
			"referenced_issues": []interface{}{"BUG-100", "STORY-200"},
			"priority":          "High",
			"labels":            []interface{}{"backend", "security"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := storage.SaveDocument(doc); err != nil {
		t.Fatalf("Failed to save document: %v", err)
	}

	// Search and verify metadata
	results, err := storage.SearchByIdentifier("TEST-500", []string{}, 10)
	if err != nil {
		t.Fatalf("SearchByIdentifier failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]

	// Verify issue_key
	if issueKey, ok := result.Metadata["issue_key"].(string); !ok || issueKey != "TEST-500" {
		t.Errorf("Expected issue_key TEST-500, got %v", result.Metadata["issue_key"])
	}

	// Verify referenced_issues is preserved
	referencedIssues, ok := result.Metadata["referenced_issues"].([]interface{})
	if !ok {
		t.Fatalf("referenced_issues not found or wrong type")
	}
	if len(referencedIssues) != 2 {
		t.Errorf("Expected 2 referenced issues, got %d", len(referencedIssues))
	}

	// Verify priority
	if priority, ok := result.Metadata["priority"].(string); !ok || priority != "High" {
		t.Errorf("Expected priority High, got %v", result.Metadata["priority"])
	}
}

// Helper function to convert document IDs to string for better error messages
func docIDsToString(docs []*models.Document) string {
	ids := make([]string, len(docs))
	for i, doc := range docs {
		ids[i] = doc.ID
	}
	data, _ := json.Marshal(ids)
	return string(data)
}
