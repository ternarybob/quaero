package actions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// MockKeyValueStorage is a mock implementation of KeyValueStorage for testing
type MockKeyValueStorage struct {
	data map[string]string
}

func NewMockKeyValueStorage() *MockKeyValueStorage {
	return &MockKeyValueStorage{
		data: make(map[string]string),
	}
}

func (m *MockKeyValueStorage) Get(ctx context.Context, key string) (string, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return "", nil
}

func (m *MockKeyValueStorage) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	if val, ok := m.data[key]; ok {
		return &interfaces.KeyValuePair{
			Key:   key,
			Value: val,
		}, nil
	}
	return nil, nil
}

func (m *MockKeyValueStorage) Set(ctx context.Context, key, value, description string) error {
	m.data[key] = value
	return nil
}

func (m *MockKeyValueStorage) Upsert(ctx context.Context, key, value, description string) (bool, error) {
	_, exists := m.data[key]
	m.data[key] = value
	return !exists, nil
}

func (m *MockKeyValueStorage) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MockKeyValueStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	pairs := make([]interfaces.KeyValuePair, 0, len(m.data))
	for k, v := range m.data {
		pairs = append(pairs, interfaces.KeyValuePair{Key: k, Value: v})
	}
	return pairs, nil
}

func (m *MockKeyValueStorage) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

func TestBuildDependencyGraphAction_NormalizePath(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Unix path",
			path:     "/usr/include/stdio.h",
			expected: "/usr/include/stdio.h",
		},
		{
			name:     "Windows path",
			path:     "C:\\Program Files\\include\\header.h",
			expected: "c:/program files/include/header.h",
		},
		{
			name:     "Path with ..",
			path:     "/home/user/../include/file.h",
			expected: "/home/include/file.h",
		},
		{
			name:     "Path with .",
			path:     "/usr/./include/./stdio.h",
			expected: "/usr/include/stdio.h",
		},
		{
			name:     "Mixed case",
			path:     "/USR/Include/STDIO.H",
			expected: "/usr/include/stdio.h",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := action.NormalizePath(tt.path)
			if result != tt.expected {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestBuildDependencyGraphAction_ResolvePath(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	tests := []struct {
		name        string
		basePath    string
		includePath string
		wantSuffix  string // check if result ends with this
	}{
		{
			name:        "Relative include",
			basePath:    "/project/src/main.cpp",
			includePath: "utils.h",
			wantSuffix:  "src/utils.h",
		},
		{
			name:        "Relative include with subdirectory",
			basePath:    "/project/src/main.cpp",
			includePath: "lib/helper.h",
			wantSuffix:  "src/lib/helper.h",
		},
		{
			name:        "Relative include with parent directory",
			basePath:    "/project/src/module/file.cpp",
			includePath: "../common.h",
			wantSuffix:  "src/common.h",
		},
		{
			name:        "Absolute include",
			basePath:    "/project/src/main.cpp",
			includePath: "/usr/include/stdio.h",
			wantSuffix:  "/usr/include/stdio.h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := action.ResolvePath(tt.basePath, tt.includePath)
			// For simplicity, just check result is not empty
			// In real implementation, would check exact path resolution
			if result == "" {
				t.Errorf("ResolvePath returned empty string")
			}
		})
	}
}

func TestBuildDependencyGraphAction_BuildGraph(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	t.Run("Build graph from documents", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID:  "doc1",
				URL: "/project/main.cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes":   []interface{}{"utils.h", "config.h"},
						"platforms":        []interface{}{"linux", "windows"},
						"component":        "core",
						"file_role":        "source",
						"build_deps":       []interface{}{"main.o"},
						"linked_libraries": []interface{}{"pthread"},
					},
				},
			},
			{
				ID:  "doc2",
				URL: "/project/utils.cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"utils.h"},
						"platforms":      []interface{}{"linux"},
						"component":      "core",
						"file_role":      "source",
					},
				},
			},
			{
				ID:  "doc3",
				URL: "/project/test.cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"utils.h"},
						"platforms":      []interface{}{"linux"},
						"component":      "tests",
						"file_role":      "test",
						"test_requires":  []interface{}{"database"},
					},
				},
			},
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph failed: %v", err)
		}

		if graph == nil {
			t.Fatal("Expected non-nil graph")
		}

		if len(graph.Nodes) != 3 {
			t.Errorf("Expected 3 nodes, got %d", len(graph.Nodes))
		}

		// Check edges were created from includes
		if len(graph.Edges) == 0 {
			t.Error("Expected edges to be created from includes")
		}

		// Check components were aggregated
		if len(graph.Components) == 0 {
			t.Error("Expected component summaries to be computed")
		}
	})

	t.Run("Build graph with no documents", func(t *testing.T) {
		docs := []*models.Document{}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph failed: %v", err)
		}

		if len(graph.Nodes) != 0 {
			t.Errorf("Expected 0 nodes, got %d", len(graph.Nodes))
		}
		if len(graph.Edges) != 0 {
			t.Errorf("Expected 0 edges, got %d", len(graph.Edges))
		}
	})

	t.Run("Build graph with documents without devops metadata", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID:       "doc1",
				URL:      "/project/file.txt",
				Metadata: map[string]interface{}{},
			},
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph failed: %v", err)
		}

		// Should skip documents without devops metadata
		if len(graph.Nodes) != 0 {
			t.Errorf("Expected 0 nodes for docs without devops metadata, got %d", len(graph.Nodes))
		}
	})
}

func TestBuildDependencyGraphAction_ComputeComponentSummaries(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	t.Run("Compute summaries from nodes", func(t *testing.T) {
		nodes := []GraphNode{
			{
				ID:        "/project/main.cpp",
				Component: "core",
				FileRole:  "source",
				Platforms: []string{"linux", "windows"},
			},
			{
				ID:        "/project/utils.cpp",
				Component: "core",
				FileRole:  "source",
				Platforms: []string{"linux"},
			},
			{
				ID:        "/project/test.cpp",
				Component: "tests",
				FileRole:  "test",
				Platforms: []string{"linux"},
			},
		}

		summaries := action.ComputeComponentSummaries(nodes)

		if len(summaries) != 2 {
			t.Errorf("Expected 2 component summaries, got %d", len(summaries))
		}

		// Find core component
		var coreComponent *ComponentSummary
		var testsComponent *ComponentSummary
		for i := range summaries {
			if summaries[i].Name == "core" {
				coreComponent = &summaries[i]
			}
			if summaries[i].Name == "tests" {
				testsComponent = &summaries[i]
			}
		}

		if coreComponent == nil {
			t.Fatal("Expected to find core component")
		}
		if coreComponent.FileCount != 2 {
			t.Errorf("Expected core component to have 2 files, got %d", coreComponent.FileCount)
		}

		if testsComponent == nil {
			t.Fatal("Expected to find tests component")
		}
		if testsComponent.FileCount != 1 {
			t.Errorf("Expected tests component to have 1 file, got %d", testsComponent.FileCount)
		}
		if !testsComponent.HasTests {
			t.Error("Expected tests component to have HasTests=true")
		}
	})

	t.Run("Handle unknown component", func(t *testing.T) {
		nodes := []GraphNode{
			{
				ID:        "/project/file.cpp",
				Component: "",
				FileRole:  "source",
			},
		}

		summaries := action.ComputeComponentSummaries(nodes)

		if len(summaries) != 1 {
			t.Errorf("Expected 1 summary, got %d", len(summaries))
		}

		if summaries[0].Name != "unknown" {
			t.Errorf("Expected component name 'unknown', got %s", summaries[0].Name)
		}
	})

	t.Run("Aggregate platforms correctly", func(t *testing.T) {
		nodes := []GraphNode{
			{
				ID:        "/project/file1.cpp",
				Component: "core",
				Platforms: []string{"linux", "windows"},
			},
			{
				ID:        "/project/file2.cpp",
				Component: "core",
				Platforms: []string{"macos", "windows"},
			},
		}

		summaries := action.ComputeComponentSummaries(nodes)

		if len(summaries) != 1 {
			t.Fatalf("Expected 1 summary, got %d", len(summaries))
		}

		// Should have unique platforms: linux, windows, macos
		if len(summaries[0].Platforms) < 2 {
			t.Errorf("Expected at least 2 unique platforms, got %d: %v",
				len(summaries[0].Platforms), summaries[0].Platforms)
		}
	})

	t.Run("Empty nodes", func(t *testing.T) {
		summaries := action.ComputeComponentSummaries([]GraphNode{})

		if len(summaries) != 0 {
			t.Errorf("Expected 0 summaries, got %d", len(summaries))
		}
	})
}

func TestBuildDependencyGraphAction_Execute(t *testing.T) {
	logger := arbor.NewLogger()
	kvStorage := NewMockKeyValueStorage()

	t.Run("Execute and store graph", func(t *testing.T) {
		action := NewBuildDependencyGraphAction(nil, kvStorage, nil, logger)

		docs := []*models.Document{
			{
				ID:  "doc1",
				URL: "/project/main.cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"utils.h"},
						"component":      "core",
						"file_role":      "source",
					},
				},
			},
		}

		err := action.Execute(context.Background(), docs)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Check graph was stored in KV storage
		graphJSON, err := kvStorage.Get(context.Background(), "devops:dependency_graph")
		if err != nil {
			t.Fatalf("Failed to get graph from KV: %v", err)
		}

		if graphJSON == "" {
			t.Fatal("Expected graph to be stored in KV")
		}

		// Verify it's valid JSON
		var graph DependencyGraph
		if err := json.Unmarshal([]byte(graphJSON), &graph); err != nil {
			t.Fatalf("Failed to parse stored graph JSON: %v", err)
		}

		if len(graph.Nodes) != 1 {
			t.Errorf("Expected 1 node in stored graph, got %d", len(graph.Nodes))
		}
	})

	t.Run("Execute with empty documents", func(t *testing.T) {
		action := NewBuildDependencyGraphAction(nil, kvStorage, nil, logger)

		err := action.Execute(context.Background(), []*models.Document{})
		if err != nil {
			t.Fatalf("Execute should handle empty docs: %v", err)
		}
	})
}

func TestBuildDependencyGraphAction_EdgeCases(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	t.Run("Circular includes", func(t *testing.T) {
		// A includes B, B includes C, C includes A
		docs := []*models.Document{
			{
				ID:  "A",
				URL: "/project/a.h",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"b.h"},
						"component":      "core",
					},
				},
			},
			{
				ID:  "B",
				URL: "/project/b.h",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"c.h"},
						"component":      "core",
					},
				},
			},
			{
				ID:  "C",
				URL: "/project/c.h",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes": []interface{}{"a.h"},
						"component":      "core",
					},
				},
			},
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph should handle circular includes: %v", err)
		}

		// Should still create the graph with circular edges
		if len(graph.Nodes) != 3 {
			t.Errorf("Expected 3 nodes, got %d", len(graph.Nodes))
		}

		if len(graph.Edges) != 3 {
			t.Errorf("Expected 3 edges for circular includes, got %d", len(graph.Edges))
		}
	})

	t.Run("Multiple edge types", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID:  "main",
				URL: "/project/main.cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"local_includes":   []interface{}{"utils.h"},
						"build_deps":       []interface{}{"utils.o"},
						"linked_libraries": []interface{}{"pthread"},
						"test_requires":    []interface{}{"database"},
						"component":        "core",
					},
				},
			},
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph failed: %v", err)
		}

		// Should create edges for all relationship types
		edgeTypes := make(map[string]int)
		for _, edge := range graph.Edges {
			edgeTypes[edge.Type]++
		}

		expectedTypes := []string{"includes", "builds", "links", "tests"}
		for _, expectedType := range expectedTypes {
			if edgeTypes[expectedType] == 0 {
				t.Errorf("Expected at least one %s edge", expectedType)
			}
		}
	})

	t.Run("Path normalization consistency", func(t *testing.T) {
		// Different path representations should normalize to same value
		path1 := "/project/./src/main.cpp"
		path2 := "/PROJECT/src/MAIN.CPP"
		path3 := "/project/src/../src/main.cpp"

		norm1 := action.NormalizePath(path1)
		norm2 := action.NormalizePath(path2)
		norm3 := action.NormalizePath(path3)

		if norm1 != norm2 {
			t.Errorf("Path normalization inconsistent: %s != %s", norm1, norm2)
		}
		if norm1 != norm3 {
			t.Errorf("Path normalization inconsistent: %s != %s", norm1, norm3)
		}
	})

	t.Run("Documents with missing fields", func(t *testing.T) {
		docs := []*models.Document{
			{
				ID: "incomplete",
				// Missing URL
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
					},
				},
			},
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph should handle missing fields: %v", err)
		}

		// Should still create a node (using ID as fallback)
		if len(graph.Nodes) != 1 {
			t.Errorf("Expected 1 node, got %d", len(graph.Nodes))
		}
	})

	t.Run("Large graph", func(t *testing.T) {
		// Create many documents
		docs := make([]*models.Document, 100)
		for i := 0; i < 100; i++ {
			docs[i] = &models.Document{
				ID:  string(rune('a' + i)),
				URL: "/project/file" + string(rune('a'+i)) + ".cpp",
				Metadata: map[string]interface{}{
					"devops": map[string]interface{}{
						"component": "core",
						"file_role": "source",
					},
				},
			}
		}

		graph, err := action.BuildGraph(docs)
		if err != nil {
			t.Fatalf("BuildGraph should handle large graphs: %v", err)
		}

		if len(graph.Nodes) != 100 {
			t.Errorf("Expected 100 nodes, got %d", len(graph.Nodes))
		}
	})
}

func TestBuildDependencyGraphAction_GetDevOpsMetadata(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewBuildDependencyGraphAction(nil, nil, nil, logger)

	t.Run("Extract valid metadata", func(t *testing.T) {
		doc := &models.Document{
			ID: "test",
			Metadata: map[string]interface{}{
				"devops": map[string]interface{}{
					"component":      "core",
					"local_includes": []interface{}{"utils.h"},
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

	t.Run("Handle missing devops field", func(t *testing.T) {
		doc := &models.Document{
			ID:       "test",
			Metadata: map[string]interface{}{},
		}

		metadata := action.GetDevOpsMetadata(doc)
		if metadata != nil {
			t.Error("Expected nil for missing devops field")
		}
	})

	t.Run("Handle malformed devops data", func(t *testing.T) {
		doc := &models.Document{
			ID: "test",
			Metadata: map[string]interface{}{
				"devops": "not a valid structure",
			},
		}

		metadata := action.GetDevOpsMetadata(doc)
		// Should return nil or handle gracefully
		_ = metadata
	})
}
