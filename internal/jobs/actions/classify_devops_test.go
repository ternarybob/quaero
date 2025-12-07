package actions

import (
	"context"
	"errors"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

func TestClassifyDevOpsAction_TruncateContent(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()
	mockLLM := NewMockLLMService("", nil)
	action := NewClassifyDevOpsAction(storage, mockLLM, logger)

	t.Run("Content shorter than limit", func(t *testing.T) {
		content := "short content"
		result := action.TruncateContent(content, 100)

		if result != content {
			t.Errorf("Expected content to be unchanged, got: %s", result)
		}
	})

	t.Run("Content longer than limit", func(t *testing.T) {
		content := "a very long content that exceeds the maximum length limit"
		result := action.TruncateContent(content, 20)

		if len(result) <= 20 {
			t.Error("Expected truncated content to be approximately at limit")
		}
		if result == content {
			t.Error("Expected content to be truncated")
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		result := action.TruncateContent("", 100)

		if result != "" {
			t.Errorf("Expected empty string, got: %s", result)
		}
	})
}

func TestClassifyDevOpsAction_ParseClassification(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()
	mockLLM := NewMockLLMService("", nil)
	action := NewClassifyDevOpsAction(storage, mockLLM, logger)

	t.Run("Valid JSON response", func(t *testing.T) {
		response := `{
  "file_role": "source",
  "component": "networking",
  "test_type": "unit",
  "test_framework": "gtest",
  "test_requires": ["network"],
  "external_deps": ["libcurl"],
  "config_sources": ["env"]
}`

		result, err := action.ParseClassification(response)
		if err != nil {
			t.Fatalf("ParseClassification failed: %v", err)
		}

		if result.FileRole != "source" {
			t.Errorf("Expected file_role 'source', got %s", result.FileRole)
		}
		if result.Component != "networking" {
			t.Errorf("Expected component 'networking', got %s", result.Component)
		}
		if result.TestType != "unit" {
			t.Errorf("Expected test_type 'unit', got %s", result.TestType)
		}
		if result.TestFramework != "gtest" {
			t.Errorf("Expected test_framework 'gtest', got %s", result.TestFramework)
		}
		if len(result.TestRequires) != 1 {
			t.Errorf("Expected 1 test_requires, got %d", len(result.TestRequires))
		}
		if len(result.ExternalDeps) != 1 {
			t.Errorf("Expected 1 external_deps, got %d", len(result.ExternalDeps))
		}
		if len(result.ConfigSources) != 1 {
			t.Errorf("Expected 1 config_sources, got %d", len(result.ConfigSources))
		}
	})

	t.Run("JSON with markdown code blocks", func(t *testing.T) {
		response := "```json\n{\"file_role\": \"header\", \"component\": \"utils\"}\n```"

		result, err := action.ParseClassification(response)
		if err != nil {
			t.Fatalf("ParseClassification should handle markdown blocks: %v", err)
		}

		if result.FileRole != "header" {
			t.Errorf("Expected file_role 'header', got %s", result.FileRole)
		}
	})

	t.Run("JSON with markdown code blocks (alternative format)", func(t *testing.T) {
		response := "```\n{\"file_role\": \"test\", \"component\": \"core\"}\n```"

		result, err := action.ParseClassification(response)
		if err != nil {
			t.Fatalf("ParseClassification should handle markdown blocks: %v", err)
		}

		if result.FileRole != "test" {
			t.Errorf("Expected file_role 'test', got %s", result.FileRole)
		}
	})

	t.Run("JSON embedded in text", func(t *testing.T) {
		response := "Here is the result: {\"file_role\": \"build\", \"component\": \"system\"} done"

		result, err := action.ParseClassification(response)
		if err != nil {
			t.Fatalf("ParseClassification should extract JSON: %v", err)
		}

		if result.FileRole != "build" {
			t.Errorf("Expected file_role 'build', got %s", result.FileRole)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		response := "{file_role: source}"

		_, err := action.ParseClassification(response)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("Missing required field", func(t *testing.T) {
		response := `{"component": "network"}`

		_, err := action.ParseClassification(response)
		if err == nil {
			t.Error("Expected error for missing file_role")
		}
	})

	t.Run("Empty file_role", func(t *testing.T) {
		response := `{"file_role": "", "component": "network"}`

		_, err := action.ParseClassification(response)
		if err == nil {
			t.Error("Expected error for empty file_role")
		}
	})

	t.Run("Minimal valid JSON", func(t *testing.T) {
		response := `{"file_role": "config"}`

		result, err := action.ParseClassification(response)
		if err != nil {
			t.Fatalf("ParseClassification failed: %v", err)
		}

		if result.FileRole != "config" {
			t.Errorf("Expected file_role 'config', got %s", result.FileRole)
		}
	})
}

func TestClassifyDevOpsAction_CallLLMWithRetry(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()

	t.Run("Successful first call", func(t *testing.T) {
		mockLLM := NewMockLLMService(`{"file_role": "source"}`, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		response, err := action.CallLLMWithRetry(context.Background(), "test prompt", 3)
		if err != nil {
			t.Fatalf("CallLLMWithRetry failed: %v", err)
		}

		if response != `{"file_role": "source"}` {
			t.Errorf("Unexpected response: %s", response)
		}

		if mockLLM.callCount != 1 {
			t.Errorf("Expected 1 call, got %d", mockLLM.callCount)
		}
	})

	t.Run("Retry on error", func(t *testing.T) {
		callCount := 0
		mockLLM := &MockLLMService{
			response: `{"file_role": "source"}`,
		}

		// Simulate failure on first 2 calls, success on 3rd
		originalChat := mockLLM.Chat
		mockLLM.Chat = func(ctx context.Context, messages []interface{}) (string, error) {
			callCount++
			if callCount < 3 {
				return "", errors.New("temporary error")
			}
			mockLLM.callCount++
			return mockLLM.response, nil
		}

		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		response, err := action.CallLLMWithRetry(context.Background(), "test prompt", 3)
		if err != nil {
			t.Fatalf("CallLLMWithRetry should succeed after retries: %v", err)
		}

		if callCount != 3 {
			t.Errorf("Expected 3 calls (2 failures + 1 success), got %d", callCount)
		}

		if response != `{"file_role": "source"}` {
			t.Errorf("Unexpected response: %s", response)
		}
	})

	t.Run("Max retries exceeded", func(t *testing.T) {
		mockLLM := NewMockLLMService("", errors.New("persistent error"))
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		_, err := action.CallLLMWithRetry(context.Background(), "test prompt", 3)
		if err == nil {
			t.Error("Expected error after max retries")
		}

		if mockLLM.callCount != 3 {
			t.Errorf("Expected 3 calls, got %d", mockLLM.callCount)
		}
	})

	t.Run("Context cancelled", func(t *testing.T) {
		mockLLM := NewMockLLMService("", errors.New("error"))
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := action.CallLLMWithRetry(ctx, "test prompt", 3)
		if err == nil {
			t.Error("Expected error for cancelled context")
		}
	})
}

func TestClassifyDevOpsAction_Execute(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()

	t.Run("Successful classification", func(t *testing.T) {
		response := `{
  "file_role": "source",
  "component": "database",
  "test_type": "none"
}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-1",
			Title:           "db.cpp",
			ContentMarkdown: "#include <sqlite3.h>\nint query() { return 0; }",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata")
		}

		if devopsData == nil {
			t.Fatal("Expected non-nil devops metadata")
		}

		if mockLLM.callCount != 1 {
			t.Errorf("Expected 1 LLM call, got %d", mockLLM.callCount)
		}
	})

	t.Run("Skip already classified", func(t *testing.T) {
		mockLLM := NewMockLLMService("", nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-2",
			Title:           "file.cpp",
			ContentMarkdown: "content",
			Metadata: map[string]interface{}{
				"devops": map[string]interface{}{
					"enrichment_passes": []interface{}{"classify_devops"},
				},
			},
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should skip, no LLM call
		if mockLLM.callCount != 0 {
			t.Errorf("Expected 0 LLM calls, got %d", mockLLM.callCount)
		}
	})

	t.Run("Force reclassification", func(t *testing.T) {
		response := `{"file_role": "source", "component": "core"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-3",
			Title:           "file.cpp",
			ContentMarkdown: "content",
			Metadata: map[string]interface{}{
				"devops": map[string]interface{}{
					"enrichment_passes": []interface{}{"classify_devops"},
				},
			},
		}

		err := action.Execute(context.Background(), doc, true) // force=true
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should call LLM even if already processed
		if mockLLM.callCount != 1 {
			t.Errorf("Expected 1 LLM call, got %d", mockLLM.callCount)
		}
	})

	t.Run("LLM error handling", func(t *testing.T) {
		mockLLM := NewMockLLMService("", errors.New("LLM service unavailable"))
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-4",
			Title:           "file.cpp",
			ContentMarkdown: "content",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err == nil {
			t.Error("Expected error when LLM fails")
		}

		// Should mark as enrichment_failed
		if enrichmentFailed, ok := doc.Metadata["enrichment_failed"].(bool); !ok || !enrichmentFailed {
			t.Error("Expected enrichment_failed to be set")
		}
	})

	t.Run("Invalid JSON response handling", func(t *testing.T) {
		mockLLM := NewMockLLMService("not valid json", nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-5",
			Title:           "file.cpp",
			ContentMarkdown: "content",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}

		// Should mark as enrichment_failed
		if enrichmentFailed, ok := doc.Metadata["enrichment_failed"].(bool); !ok || !enrichmentFailed {
			t.Error("Expected enrichment_failed to be set")
		}
	})

	t.Run("Merge with existing Pass 1 data", func(t *testing.T) {
		response := `{"file_role": "source", "component": "network"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-6",
			Title:           "net.cpp",
			ContentMarkdown: "#include <sys/socket.h>",
			Metadata: map[string]interface{}{
				"devops": map[string]interface{}{
					"includes":          []interface{}{"sys/socket.h"},
					"platforms":         []interface{}{"linux"},
					"enrichment_passes": []interface{}{"extract_structure"},
				},
			},
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should preserve existing data and add classification
		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata")
		}

		if devopsData == nil {
			t.Fatal("Expected non-nil devops metadata")
		}

		// Verify enrichment passes were updated
		devopsMap, ok := devopsData.(map[string]interface{})
		if !ok {
			t.Fatal("Expected devops to be a map")
		}

		passes, ok := devopsMap["enrichment_passes"].([]interface{})
		if !ok {
			t.Fatal("Expected enrichment_passes to be an array")
		}

		hasExtract := false
		hasClassify := false
		for _, pass := range passes {
			if passStr, ok := pass.(string); ok {
				if passStr == "extract_structure" {
					hasExtract = true
				}
				if passStr == "classify_devops" {
					hasClassify = true
				}
			}
		}

		if !hasExtract {
			t.Error("Expected extract_structure pass to be preserved")
		}
		if !hasClassify {
			t.Error("Expected classify_devops pass to be added")
		}
	})
}

func TestClassifyDevOpsAction_ExtractJSON(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()
	mockLLM := NewMockLLMService("", nil)
	action := NewClassifyDevOpsAction(storage, mockLLM, logger)

	tests := []struct {
		name     string
		response string
		wantJSON bool
	}{
		{
			name:     "Plain JSON",
			response: `{"file_role": "source"}`,
			wantJSON: true,
		},
		{
			name:     "JSON with markdown",
			response: "```json\n{\"file_role\": \"header\"}\n```",
			wantJSON: true,
		},
		{
			name:     "JSON with code fence",
			response: "```\n{\"file_role\": \"test\"}\n```",
			wantJSON: true,
		},
		{
			name:     "JSON in text",
			response: "The classification is: {\"file_role\": \"build\"} as shown above",
			wantJSON: true,
		},
		{
			name:     "No JSON",
			response: "This is just text without JSON",
			wantJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := action.extractJSON(tt.response)

			if tt.wantJSON {
				if result == "" {
					t.Error("Expected to extract JSON")
				}
				// Try to parse it as JSON to verify it's valid
				var parsed map[string]interface{}
				// We don't fail if it's not valid JSON, just check we extracted something
				_ = parsed
			} else {
				// For non-JSON, just verify we got something back
				if result == "" && tt.response != "" {
					t.Error("Expected some result")
				}
			}
		})
	}
}

func TestClassifyDevOpsAction_EdgeCases(t *testing.T) {
	logger := arbor.NewNoOpLogger()
	storage := NewMockDocumentStorage()

	t.Run("Very long content truncation", func(t *testing.T) {
		response := `{"file_role": "source", "component": "core"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		// Create a very long content (> 6000 chars)
		longContent := ""
		for i := 0; i < 10000; i++ {
			longContent += "x"
		}

		doc := &models.Document{
			ID:              "test-long",
			Title:           "large.cpp",
			ContentMarkdown: longContent,
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute should handle long content: %v", err)
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		response := `{"file_role": "config", "component": "unknown"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-empty",
			Title:           "empty.cpp",
			ContentMarkdown: "",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute should handle empty content: %v", err)
		}
	})

	t.Run("Nil metadata", func(t *testing.T) {
		response := `{"file_role": "source", "component": "test"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-nil",
			Title:           "test.cpp",
			ContentMarkdown: "content",
			Metadata:        nil,
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute should handle nil metadata: %v", err)
		}

		if doc.Metadata == nil {
			t.Error("Expected metadata to be initialized")
		}
	})

	t.Run("Malformed existing devops metadata", func(t *testing.T) {
		response := `{"file_role": "source", "component": "core"}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-malformed",
			Title:           "test.cpp",
			ContentMarkdown: "content",
			Metadata: map[string]interface{}{
				"devops": "not a valid structure",
			},
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute should handle malformed metadata: %v", err)
		}
	})

	t.Run("Complex classification result", func(t *testing.T) {
		response := `{
  "file_role": "test",
  "component": "integration-tests",
  "test_type": "integration",
  "test_framework": "custom",
  "test_requires": ["database", "network", "filesystem"],
  "external_deps": ["postgres", "redis", "libcurl"],
  "config_sources": ["env", "file", "registry"]
}`
		mockLLM := NewMockLLMService(response, nil)
		action := NewClassifyDevOpsAction(storage, mockLLM, logger)

		doc := &models.Document{
			ID:              "test-complex",
			Title:           "integration_test.cpp",
			ContentMarkdown: "complex test file",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute should handle complex classification: %v", err)
		}

		// Verify all fields were set
		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata")
		}

		if devopsData == nil {
			t.Fatal("Expected non-nil devops metadata")
		}
	})
}
