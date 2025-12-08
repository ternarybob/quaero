package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// MockSearchService is a mock implementation of SearchService for testing
type MockSearchService struct {
	documents []*models.Document
	err       error
}

func NewMockSearchService(docs []*models.Document, err error) *MockSearchService {
	return &MockSearchService{
		documents: docs,
		err:       err,
	}
}

func (m *MockSearchService) Search(ctx context.Context, query string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.documents, nil
}

func (m *MockSearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	for _, doc := range m.documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *MockSearchService) SearchByReference(ctx context.Context, reference string, opts interfaces.SearchOptions) ([]*models.Document, error) {
	return m.documents, nil
}

func TestAggregateDevOpsSummaryAction_AggregateFromDocuments(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAggregateDevOpsSummaryAction(nil, nil, nil, nil, logger)

	t.Run("Aggregate from multiple documents", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"platforms":      []interface{}{"linux", "windows"},
						"component":      "core",
						"file_role":      "source",
						"build_targets":  []interface{}{"app", "tests"},
						"test_framework": "gtest",
						"test_type":      "unit",
						"test_requires":  []interface{}{"network"},
						"external_deps":  []interface{}{"libcurl"},
					},
				},
			},
			{
				ID: "doc2",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"platforms":      []interface{}{"linux"},
						"component":      "database",
						"file_role":      "source",
						"build_targets":  []interface{}{"dblib"},
						"test_framework": "catch",
						"test_type":      "integration",
						"external_deps":  []interface{}{"postgres"},
					},
				},
			},
		}

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		// Check platforms
		if len(aggregated.Platforms) < 2 {
			t.Errorf("Expected at least 2 platforms, got %d: %v", len(aggregated.Platforms), aggregated.Platforms)
		}

		// Check components
		if len(aggregated.Components) != 2 {
			t.Errorf("Expected 2 components, got %d", len(aggregated.Components))
		}

		// Check build targets
		if len(aggregated.BuildInfo.Targets) < 2 {
			t.Errorf("Expected at least 2 build targets, got %d", len(aggregated.BuildInfo.Targets))
		}

		// Check test frameworks
		if len(aggregated.TestInfo.Frameworks) != 2 {
			t.Errorf("Expected 2 test frameworks, got %d: %v", len(aggregated.TestInfo.Frameworks), aggregated.TestInfo.Frameworks)
		}

		// Check external deps
		if len(aggregated.Dependencies) < 2 {
			t.Errorf("Expected at least 2 dependencies, got %d", len(aggregated.Dependencies))
		}
	})

	t.Run("Deduplicate platforms", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"platforms": []interface{}{"linux", "windows"},
						"component": "core",
					},
				},
			},
			{
				ID: "doc2",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"platforms": []interface{}{"linux", "macos"},
						"component": "utils",
					},
				},
			},
		}

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		// Should have 3 unique platforms: linux, windows, macos
		if len(aggregated.Platforms) != 3 {
			t.Errorf("Expected 3 unique platforms, got %d: %v", len(aggregated.Platforms), aggregated.Platforms)
		}
	})

	t.Run("Component file count", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
						"file_role": "source",
					},
				},
			},
			{
				ID: "doc2",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
						"file_role": "header",
					},
				},
			},
			{
				ID: "doc3",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "utils",
						"file_role": "source",
					},
				},
			},
		}

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		// Find core component
		var coreComponent *ComponentInfo
		for i := range aggregated.Components {
			if aggregated.Components[i].Name == "core" {
				coreComponent = &aggregated.Components[i]
				break
			}
		}

		if coreComponent == nil {
			t.Fatal("Expected to find core component")
		}

		if coreComponent.FileCount != 2 {
			t.Errorf("Expected core component to have 2 files, got %d", coreComponent.FileCount)
		}
	})

	t.Run("Handle documents without devops metadata", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID:       "doc1",
				Metadata: map[string]interface{}{},
			},
			{
				ID: "doc2",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
					},
				},
			},
		}

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		// Should only process doc2
		if len(aggregated.Components) != 1 {
			t.Errorf("Expected 1 component, got %d", len(aggregated.Components))
		}
	})

	t.Run("Empty document list", func(t *testing.T) {
		aggregated, err := action.AggregateFromDocuments([]*models.Document{})
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		if len(aggregated.Platforms) != 0 {
			t.Errorf("Expected 0 platforms, got %d", len(aggregated.Platforms))
		}
		if len(aggregated.Components) != 0 {
			t.Errorf("Expected 0 components, got %d", len(aggregated.Components))
		}
	})

	t.Run("Infer build systems from file paths", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID:         "doc1",
				SourceType: "local_dir",
				Metadata: map[string]interface{}{
					"local_dir": map[string]interface{}{
						"file_path": "/project/CMakeLists.txt",
					},
					"devops": map[string]interface{}{
						"component": "build",
					},
				},
			},
			{
				ID:         "doc2",
				SourceType: "local_dir",
				Metadata: map[string]interface{}{
					"local_dir": map[string]interface{}{
						"file_path": "/project/Makefile",
					},
					"devops": map[string]interface{}{
						"component": "build",
					},
				},
			},
		}

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments failed: %v", err)
		}

		// Should detect cmake and make
		if len(aggregated.BuildInfo.Systems) < 2 {
			t.Errorf("Expected at least 2 build systems, got %d: %v",
				len(aggregated.BuildInfo.Systems), aggregated.BuildInfo.Systems)
		}
	})
}

func TestAggregateDevOpsSummaryAction_FormatForPrompt(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAggregateDevOpsSummaryAction(nil, nil, nil, nil, logger)

	t.Run("Format complete data", func(t *testing.T) {
		data := &AggregatedData{
			BuildInfo: BuildInfo{
				Systems:   []string{"cmake", "make"},
				Targets:   []string{"app", "tests"},
				Toolchain: "gcc",
			},
			Platforms: []string{"linux", "windows"},
			Components: []ComponentInfo{
				{Name: "core", FileCount: 10, Role: "source"},
				{Name: "tests", FileCount: 5, Role: "test"},
			},
			Dependencies: []string{"libcurl", "pthread"},
			TestInfo: TestInfo{
				Frameworks: []string{"gtest"},
				Types:      []string{"unit", "integration"},
				Requires:   []string{"database"},
			},
		}

		prompt := action.FormatForPrompt(data)

		if prompt == "" {
			t.Error("Expected non-empty prompt")
		}

		// Check key information is included
		if !contains(prompt, "cmake") {
			t.Error("Expected cmake in prompt")
		}
		if !contains(prompt, "linux") {
			t.Error("Expected linux in prompt")
		}
		if !contains(prompt, "core") {
			t.Error("Expected core component in prompt")
		}
	})

	t.Run("Format minimal data", func(t *testing.T) {
		data := &AggregatedData{
			BuildInfo:    BuildInfo{},
			Platforms:    []string{},
			Components:   []ComponentInfo{},
			Dependencies: []string{},
			TestInfo:     TestInfo{},
		}

		prompt := action.FormatForPrompt(data)

		if prompt == "" {
			t.Error("Expected non-empty prompt even with minimal data")
		}
	})
}

func TestAggregateDevOpsSummaryAction_Execute(t *testing.T) {
	logger := arbor.NewLogger()
	storage := NewMockDocumentStorage()
	kvStorage := NewMockKeyValueStorage()

	t.Run("Successful execution", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
						"platforms": []interface{}{"linux"},
					},
				},
			},
		}

		searchService := NewMockSearchService(docs, nil)
		llmResponse := `# DevOps Guide

## Build System Overview
This project uses make and cmake.

## Toolchain Requirements
- GCC compiler
- CMake 3.10+

## Test Strategy
Unit tests using gtest.`

		llmService := NewMockLLMService(llmResponse, nil)
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Verify summary was stored in KV
		summary, err := kvStorage.Get(context.Background(), "devops:summary")
		if err != nil {
			t.Fatalf("Failed to get summary from KV: %v", err)
		}

		if summary == "" {
			t.Error("Expected summary to be stored in KV")
		}

		// Verify document was created
		doc, err := storage.GetDocument("devops-summary")
		if err != nil {
			t.Fatalf("Failed to get summary document: %v", err)
		}

		if doc == nil {
			t.Error("Expected summary document to be created")
		}

		// Verify completion timestamp was set
		timestamp, err := kvStorage.Get(context.Background(), "devops:enrichment:aggregate_completed")
		if err != nil {
			t.Fatalf("Failed to get completion timestamp: %v", err)
		}

		if timestamp == "" {
			t.Error("Expected completion timestamp to be set")
		}
	})

	t.Run("Handle search error", func(t *testing.T) {
		searchService := NewMockSearchService(nil, errors.New("search failed"))
		llmService := NewMockLLMService("", nil)
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err == nil {
			t.Error("Expected error when search fails")
		}
	})

	t.Run("Handle LLM error", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
					},
				},
			},
		}

		searchService := NewMockSearchService(docs, nil)
		llmService := NewMockLLMService("", errors.New("LLM failed"))
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err == nil {
			t.Error("Expected error when LLM fails")
		}
	})

	t.Run("Handle empty LLM response", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
					},
				},
			},
		}

		searchService := NewMockSearchService(docs, nil)
		llmService := NewMockLLMService("", nil) // empty response
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute should handle empty LLM response: %v", err)
		}

		// Should generate minimal summary
		summary, err := kvStorage.Get(context.Background(), "devops:summary")
		if err != nil {
			t.Fatalf("Failed to get summary: %v", err)
		}

		if summary == "" {
			t.Error("Expected minimal summary to be generated")
		}
	})

	t.Run("Handle no enriched documents", func(t *testing.T) {
		searchService := NewMockSearchService([]*models.Document{}, nil)
		llmService := NewMockLLMService("# DevOps Guide\n\nNo data available.", nil)
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute should handle no documents: %v", err)
		}
	})
}

func TestAggregateDevOpsSummaryAction_CreateSummaryDocument(t *testing.T) {
	logger := arbor.NewLogger()
	storage := NewMockDocumentStorage()
	action := NewAggregateDevOpsSummaryAction(storage, nil, nil, nil, logger)

	t.Run("Create document successfully", func(t *testing.T) {
		markdown := "# DevOps Summary\n\nTest content"

		err := action.CreateSummaryDocument(context.Background(), markdown)
		if err != nil {
			t.Fatalf("CreateSummaryDocument failed: %v", err)
		}

		doc, err := storage.GetDocument("devops-summary")
		if err != nil {
			t.Fatalf("Failed to get document: %v", err)
		}

		if doc == nil {
			t.Fatal("Expected document to be created")
		}

		if doc.ID != "devops-summary" {
			t.Errorf("Expected document ID 'devops-summary', got %s", doc.ID)
		}

		if doc.Title != "DevOps Summary - C/C++ Codebase Analysis" {
			t.Errorf("Unexpected document title: %s", doc.Title)
		}

		if doc.ContentMarkdown != markdown {
			t.Errorf("Expected markdown to be stored, got: %s", doc.ContentMarkdown)
		}

		// Check tags
		hasDevOpsTag := false
		hasSummaryTag := false
		for _, tag := range doc.Tags {
			if tag == "devops" {
				hasDevOpsTag = true
			}
			if tag == "summary" {
				hasSummaryTag = true
			}
		}

		if !hasDevOpsTag {
			t.Error("Expected 'devops' tag")
		}
		if !hasSummaryTag {
			t.Error("Expected 'summary' tag")
		}
	})

	t.Run("Create document with empty markdown", func(t *testing.T) {
		err := action.CreateSummaryDocument(context.Background(), "")
		if err != nil {
			t.Fatalf("CreateSummaryDocument should handle empty markdown: %v", err)
		}
	})
}

func TestAggregateDevOpsSummaryAction_GetDevOpsMetadata(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAggregateDevOpsSummaryAction(nil, nil, nil, nil, logger)

	t.Run("Get valid metadata", func(t *testing.T) {
		doc := &models.Document{
			ID: "test",
			Metadata: map[string]interface{}{
				"devops": map[string]interface{}{
					"component": "core",
					"platforms": []interface{}{"linux"},
				},
			},
		}

		metadata := action.GetDevOpsMetadata(doc)
		if metadata == nil {
			t.Fatal("Expected non-nil metadata")
		}
	})

	t.Run("Handle missing metadata", func(t *testing.T) {
		doc := &models.Document{
			ID:       "test",
			Metadata: nil,
		}

		metadata := action.GetDevOpsMetadata(doc)
		if metadata != nil {
			t.Error("Expected nil for missing metadata")
		}
	})

	t.Run("Handle invalid metadata structure", func(t *testing.T) {
		doc := &models.Document{
			ID: "test",
			Metadata: map[string]interface{}{
				"devops": "invalid",
			},
		}

		metadata := action.GetDevOpsMetadata(doc)
		// Should handle gracefully
		_ = metadata
	})
}

func TestAggregateDevOpsSummaryAction_EdgeCases(t *testing.T) {
	logger := arbor.NewLogger()
	storage := NewMockDocumentStorage()
	kvStorage := NewMockKeyValueStorage()

	t.Run("Very large number of documents", func(t *testing.T) {
		// Create 1000 documents
		docs := make([]*models.Document, 1000)
		for i := 0; i < 1000; i++ {
			docs[i] = &models.Document{
				ID: "doc" + string(rune(i)),
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
						"platforms": []interface{}{"linux"},
					},
				},
			}
		}

		searchService := NewMockSearchService(docs, nil)
		llmService := NewMockLLMService("# Summary\n\nTest", nil)
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute should handle large number of documents: %v", err)
		}
	})

	t.Run("Documents with complex nested metadata", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "doc1",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component":      "core",
						"platforms":      []interface{}{"linux", "windows", "macos"},
						"build_targets":  []interface{}{"app1", "app2", "lib1", "lib2"},
						"external_deps":  []interface{}{"dep1", "dep2", "dep3"},
						"test_framework": "gtest",
						"test_type":      "unit",
						"test_requires":  []interface{}{"network", "database", "filesystem"},
					},
				},
			},
		}

		searchService := NewMockSearchService(docs, nil)
		llmService := NewMockLLMService("# Summary\n\nComplex test", nil)
		action := NewAggregateDevOpsSummaryAction(storage, kvStorage, searchService, llmService, logger)

		err := action.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute should handle complex metadata: %v", err)
		}
	})

	t.Run("Mixed valid and invalid documents", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "valid",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
					},
				},
			},
			{
				ID:       "invalid1",
				Metadata: nil,
			},
			{
				ID: "invalid2",
				Metadata: map[string]interface{}{
					"devops": "not a map",
				},
			},
		}

		action := NewAggregateDevOpsSummaryAction(nil, nil, nil, nil, logger)

		aggregated, err := action.AggregateFromDocuments(docs)
		if err != nil {
			t.Fatalf("AggregateFromDocuments should handle mixed documents: %v", err)
		}

		// Should process only valid document
		if len(aggregated.Components) != 1 {
			t.Errorf("Expected 1 component from valid document, got %d", len(aggregated.Components))
		}
	})
}

func TestAggregateDevOpsSummaryAction_GenerateMinimalSummary(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAggregateDevOpsSummaryAction(nil, nil, nil, nil, logger)

	summary := action.generateMinimalSummary()

	if summary == "" {
		t.Error("Expected non-empty minimal summary")
	}

	// Check it contains key sections
	if !contains(summary, "DevOps Guide") {
		t.Error("Expected 'DevOps Guide' in minimal summary")
	}

	if !contains(summary, "Build System") {
		t.Error("Expected 'Build System' in minimal summary")
	}

	if !contains(summary, "Next Steps") {
		t.Error("Expected 'Next Steps' in minimal summary")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
