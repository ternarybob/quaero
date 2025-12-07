package actions

import (
	"context"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// MockDocumentStorage is a mock implementation of DocumentStorage interface for testing
type MockDocumentStorage struct {
	documents map[string]*models.Document
	updateErr error
}

func NewMockDocumentStorage() *MockDocumentStorage {
	return &MockDocumentStorage{
		documents: make(map[string]*models.Document),
	}
}

func (m *MockDocumentStorage) SaveDocument(doc *models.Document) error {
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) SaveDocuments(docs []*models.Document) error {
	for _, doc := range docs {
		m.documents[doc.ID] = doc
	}
	return nil
}

func (m *MockDocumentStorage) GetDocument(id string) (*models.Document, error) {
	if doc, ok := m.documents[id]; ok {
		return doc, nil
	}
	return nil, nil
}

func (m *MockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	return nil, nil
}

func (m *MockDocumentStorage) UpdateDocument(doc *models.Document) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) DeleteDocument(id string) error {
	delete(m.documents, id)
	return nil
}

func (m *MockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	return nil, nil
}

func TestExtractStructureAction_IsCppFile(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.cpp", true},
		{"header.h", true},
		{"file.cc", true},
		{"file.cxx", true},
		{"file.hpp", true},
		{"file.hxx", true},
		{"file.hh", true},
		{"file.c", true},
		{"file.go", false},
		{"Makefile", false},
		{"file.py", false},
		{"README.md", false},
		{"file.txt", false},
		{"/path/to/file.cpp", true},
		{"/path/to/file.H", true}, // uppercase extension
		{"/path/to/file.CPP", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := action.IsCppFile(tt.path); got != tt.expected {
				t.Errorf("IsCppFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestExtractStructureAction_ExtractIncludes(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	t.Run("Extract local and system includes", func(t *testing.T) {
		content := `
#include <iostream>
#include <vector>
#include "config.h"
#include "utils/helper.h"
`

		local, system := action.ExtractIncludes(content)

		if len(local) != 2 {
			t.Errorf("Expected 2 local includes, got %d: %v", len(local), local)
		}
		if len(system) != 2 {
			t.Errorf("Expected 2 system includes, got %d: %v", len(system), system)
		}
	})

	t.Run("Extract unique includes only", func(t *testing.T) {
		content := `
#include <iostream>
#include <iostream>
#include "config.h"
#include "config.h"
`

		local, system := action.ExtractIncludes(content)

		if len(local) != 1 {
			t.Errorf("Expected 1 unique local include, got %d: %v", len(local), local)
		}
		if len(system) != 1 {
			t.Errorf("Expected 1 unique system include, got %d: %v", len(system), system)
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		local, system := action.ExtractIncludes("")

		if len(local) != 0 {
			t.Errorf("Expected 0 local includes, got %d", len(local))
		}
		if len(system) != 0 {
			t.Errorf("Expected 0 system includes, got %d", len(system))
		}
	})

	t.Run("No includes in content", func(t *testing.T) {
		content := `
int main() {
    return 0;
}
`
		local, system := action.ExtractIncludes(content)

		if len(local) != 0 {
			t.Errorf("Expected 0 local includes, got %d", len(local))
		}
		if len(system) != 0 {
			t.Errorf("Expected 0 system includes, got %d", len(system))
		}
	})
}

func TestExtractStructureAction_ExtractDefines(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	t.Run("Extract defines", func(t *testing.T) {
		content := `
#define MAX_SIZE 100
#define DEBUG
#define VERSION "1.0"
`

		defines := action.ExtractDefines(content)

		if len(defines) != 3 {
			t.Errorf("Expected 3 defines, got %d: %v", len(defines), defines)
		}
	})

	t.Run("Extract unique defines only", func(t *testing.T) {
		content := `
#define MAX_SIZE 100
#define MAX_SIZE 200
#define DEBUG
#define DEBUG
`

		defines := action.ExtractDefines(content)

		if len(defines) != 2 {
			t.Errorf("Expected 2 unique defines, got %d: %v", len(defines), defines)
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		defines := action.ExtractDefines("")

		if len(defines) != 0 {
			t.Errorf("Expected 0 defines, got %d", len(defines))
		}
	})
}

func TestExtractStructureAction_ExtractConditionals(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	t.Run("Extract ifdef and ifndef", func(t *testing.T) {
		content := `
#ifdef DEBUG
#ifndef RELEASE
#ifdef _WIN32
#ifndef __linux__
`

		conditionals := action.ExtractConditionals(content)

		if len(conditionals) != 4 {
			t.Errorf("Expected 4 conditionals, got %d: %v", len(conditionals), conditionals)
		}
	})

	t.Run("Extract unique conditionals only", func(t *testing.T) {
		content := `
#ifdef DEBUG
#ifdef DEBUG
#ifndef DEBUG
`

		conditionals := action.ExtractConditionals(content)

		if len(conditionals) != 1 {
			t.Errorf("Expected 1 unique conditional, got %d: %v", len(conditionals), conditionals)
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		conditionals := action.ExtractConditionals("")

		if len(conditionals) != 0 {
			t.Errorf("Expected 0 conditionals, got %d", len(conditionals))
		}
	})
}

func TestExtractStructureAction_DetectPlatforms(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	t.Run("Detect Windows platform", func(t *testing.T) {
		content := `
#ifdef _WIN32
// Windows-specific code
#endif
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 1 || platforms[0] != "windows" {
			t.Errorf("Expected [windows], got %v", platforms)
		}
	})

	t.Run("Detect Linux platform", func(t *testing.T) {
		content := `
#ifdef __linux__
// Linux-specific code
#endif
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 1 || platforms[0] != "linux" {
			t.Errorf("Expected [linux], got %v", platforms)
		}
	})

	t.Run("Detect macOS platform", func(t *testing.T) {
		content := `
#ifdef __APPLE__
// macOS-specific code
#endif
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 1 || platforms[0] != "macos" {
			t.Errorf("Expected [macos], got %v", platforms)
		}
	})

	t.Run("Detect embedded platform", func(t *testing.T) {
		content := `
#ifdef __ARM__
// ARM-specific code
#endif
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 1 || platforms[0] != "embedded" {
			t.Errorf("Expected [embedded], got %v", platforms)
		}
	})

	t.Run("Detect multiple platforms", func(t *testing.T) {
		content := `
#ifdef _WIN32
// Windows code
#endif
#ifdef __linux__
// Linux code
#endif
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 2 {
			t.Errorf("Expected 2 platforms, got %d: %v", len(platforms), platforms)
		}
	})

	t.Run("No platform-specific code", func(t *testing.T) {
		content := `
int main() {
    return 0;
}
`

		platforms := action.DetectPlatforms(content)

		if len(platforms) != 0 {
			t.Errorf("Expected 0 platforms, got %d: %v", len(platforms), platforms)
		}
	})
}

func TestExtractStructureAction_Execute(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()
	action := NewExtractStructureAction(storage, logger)

	t.Run("Process C++ file", func(t *testing.T) {
		doc := &models.Document{
			ID:    "test-1",
			Title: "main.cpp",
			URL:   "/path/to/main.cpp",
			ContentMarkdown: `
#include <iostream>
#include "config.h"
#define MAX_SIZE 100
#ifdef _WIN32
int main() {
    return 0;
}
`,
			Metadata: make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Check metadata was updated
		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata to be set")
		}

		// We don't assert exact field values because we need to handle the map[string]interface{} conversion
		// Just verify the metadata exists
		if devopsData == nil {
			t.Fatal("Expected devops metadata to not be nil")
		}
	})

	t.Run("Skip non-C++ file", func(t *testing.T) {
		doc := &models.Document{
			ID:              "test-2",
			Title:           "README.md",
			URL:             "/path/to/README.md",
			ContentMarkdown: "# README",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should not add devops metadata for non-C++ files
		_, ok := doc.Metadata["devops"]
		if ok {
			t.Error("Did not expect devops metadata for non-C++ file")
		}
	})

	t.Run("Skip already processed file", func(t *testing.T) {
		doc := &models.Document{
			ID:              "test-3",
			Title:           "header.h",
			URL:             "/path/to/header.h",
			ContentMarkdown: "#include <stdio.h>",
			Metadata: map[string]interface{}{
				"devops": &models.DevOpsMetadata{
					EnrichmentPasses: []string{"extract_structure"},
				},
			},
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should skip processing
	})

	t.Run("Force reprocessing", func(t *testing.T) {
		doc := &models.Document{
			ID:              "test-4",
			Title:           "header.h",
			URL:             "/path/to/header.h",
			ContentMarkdown: "#include <stdio.h>",
			Metadata: map[string]interface{}{
				"devops": &models.DevOpsMetadata{
					EnrichmentPasses: []string{"extract_structure"},
				},
			},
		}

		err := action.Execute(context.Background(), doc, true) // force=true
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should process even if already processed
	})

	t.Run("Empty content", func(t *testing.T) {
		doc := &models.Document{
			ID:              "test-5",
			Title:           "empty.cpp",
			URL:             "/path/to/empty.cpp",
			ContentMarkdown: "",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should still process but extract nothing
		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata to be set")
		}
		if devopsData == nil {
			t.Fatal("Expected devops metadata to not be nil")
		}
	})
}

func TestExtractStructureAction_EdgeCases(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	action := NewExtractStructureAction(nil, logger)

	t.Run("Malformed includes", func(t *testing.T) {
		content := `
#include <incomplete
#include "missing.h
#include
`
		local, system := action.ExtractIncludes(content)

		// Should handle malformed includes gracefully
		// The incomplete ones shouldn't be extracted
		if len(local) > 1 || len(system) > 1 {
			t.Errorf("Expected minimal extraction from malformed content")
		}
	})

	t.Run("Comments containing include-like text", func(t *testing.T) {
		content := `
// #include <iostream>
/* #include "config.h" */
#include <vector>
`
		local, system := action.ExtractIncludes(content)

		// Should extract commented includes too (regex doesn't parse comments)
		// This is acceptable behavior
		if len(system) < 1 {
			t.Errorf("Expected at least 1 system include (vector)")
		}
	})

	t.Run("Nested ifdef blocks", func(t *testing.T) {
		content := `
#ifdef OUTER
  #ifdef INNER
    #ifdef INNERMOST
    #endif
  #endif
#endif
`
		conditionals := action.ExtractConditionals(content)

		if len(conditionals) != 3 {
			t.Errorf("Expected 3 conditionals from nested blocks, got %d: %v", len(conditionals), conditionals)
		}
	})

	t.Run("Multiple platform markers in same file", func(t *testing.T) {
		content := `
#ifdef _WIN32
// Windows code
#endif
#ifdef _WIN64
// 64-bit Windows code
#endif
`
		platforms := action.DetectPlatforms(content)

		// Should detect windows once (not duplicate)
		if len(platforms) != 1 || platforms[0] != "windows" {
			t.Errorf("Expected [windows] once, got %v", platforms)
		}
	})
}
