package models

import (
	"testing"
)

// TestIsPlaceholder verifies placeholder syntax detection
func TestIsPlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "valid placeholder",
			value:    "{test-key}",
			expected: true,
		},
		{
			name:     "valid placeholder with underscores",
			value:    "{test_source_type}",
			expected: true,
		},
		{
			name:     "valid placeholder with dashes",
			value:    "{test-api-key}",
			expected: true,
		},
		{
			name:     "not a placeholder - plain text",
			value:    "jira",
			expected: false,
		},
		{
			name:     "not a placeholder - missing closing brace",
			value:    "{test-key",
			expected: false,
		},
		{
			name:     "not a placeholder - missing opening brace",
			value:    "test-key}",
			expected: false,
		},
		{
			name:     "not a placeholder - empty braces",
			value:    "{}",
			expected: false,
		},
		{
			name:     "not a placeholder - single char",
			value:    "{a}",
			expected: true,
		},
		{
			name:     "empty string",
			value:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPlaceholder(tt.value)
			if result != tt.expected {
				t.Errorf("isPlaceholder(%q) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestJobDefinition_ValidateWithPlaceholders verifies validation skips placeholder values
func TestJobDefinition_ValidateWithPlaceholders(t *testing.T) {
	tests := []struct {
		name        string
		jobDef      JobDefinition
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid job with placeholder source_type",
			jobDef: JobDefinition{
				ID:         "test-job-1",
				Name:       "Test Job",
				Type:       JobDefinitionTypeCrawler,
				SourceType: "{test-source-type}", // Should not trigger validation error
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "valid job with placeholder failure_action",
			jobDef: JobDefinition{
				ID:         "test-job-2",
				Name:       "Test Job",
				Type:       JobDefinitionTypeCrawler,
				SourceType: "jira",
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
				ErrorTolerance: &ErrorTolerance{
					MaxChildFailures: 5,
					FailureAction:    "{failure-action-key}", // Should not trigger validation error
				},
			},
			shouldError: false,
		},
		{
			name: "invalid job with invalid source_type (not placeholder)",
			jobDef: JobDefinition{
				ID:         "test-job-3",
				Name:       "Test Job",
				Type:       JobDefinitionTypeCrawler,
				SourceType: "invalid-type", // Should trigger validation error
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: true,
			errorMsg:    "invalid source_type",
		},
		{
			name: "invalid job with invalid failure_action (not placeholder)",
			jobDef: JobDefinition{
				ID:         "test-job-4",
				Name:       "Test Job",
				Type:       JobDefinitionTypeCrawler,
				SourceType: "jira",
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
				ErrorTolerance: &ErrorTolerance{
					MaxChildFailures: 5,
					FailureAction:    "invalid-action", // Should trigger validation error
				},
			},
			shouldError: true,
			errorMsg:    "invalid error_tolerance.failure_action",
		},
		{
			name: "valid job with real source_type",
			jobDef: JobDefinition{
				ID:         "test-job-5",
				Name:       "Test Job",
				Type:       JobDefinitionTypeCrawler,
				SourceType: "jira",
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.jobDef.Validate()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected validation error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && len(err.Error()) > 0 {
					// Check if error message contains expected text
					if !contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestWorkerType_IsValid tests the WorkerType.IsValid() method
func TestWorkerType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		workerType WorkerType
		expected   bool
	}{
		{"agent is valid", WorkerTypeAgent, true},
		{"crawler is valid", WorkerTypeCrawler, true},
		{"places_search is valid", WorkerTypePlacesSearch, true},
		{"web_search is valid", WorkerTypeWebSearch, true},
		{"github_repo is valid", WorkerTypeGitHubRepo, true},
		{"github_actions is valid", WorkerTypeGitHubActions, true},
		{"github_git is valid", WorkerTypeGitHubGit, true},
		{"transform is valid", WorkerTypeTransform, true},
		{"reindex is valid", WorkerTypeReindex, true},
		{"local_dir is valid", WorkerTypeLocalDir, true},
		{"code_map is valid", WorkerTypeCodeMap, true},
		{"summary is valid", WorkerTypeSummary, true},
		{"test_job_generator is valid", WorkerTypeTestJobGenerator, true},
		{"analyze_build is valid", WorkerTypeAnalyzeBuild, true},
		{"classify is valid", WorkerTypeClassify, true},
		{"dependency_graph is valid", WorkerTypeDependencyGraph, true},
		{"aggregate_summary is valid", WorkerTypeAggregateSummary, true},
		{"empty string is invalid", WorkerType(""), false},
		{"unknown type is invalid", WorkerType("unknown"), false},
		{"typo is invalid", WorkerType("crawl"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.workerType.IsValid()
			if result != tt.expected {
				t.Errorf("WorkerType(%q).IsValid() = %v, want %v", tt.workerType, result, tt.expected)
			}
		})
	}
}

// TestWorkerType_String tests the WorkerType.String() method
func TestWorkerType_String(t *testing.T) {
	tests := []struct {
		workerType WorkerType
		expected   string
	}{
		{WorkerTypeAgent, "agent"},
		{WorkerTypeCrawler, "crawler"},
		{WorkerTypePlacesSearch, "places_search"},
		{WorkerTypeWebSearch, "web_search"},
		{WorkerTypeGitHubRepo, "github_repo"},
		{WorkerTypeGitHubActions, "github_actions"},
		{WorkerTypeTransform, "transform"},
		{WorkerTypeReindex, "reindex"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.workerType.String()
			if result != tt.expected {
				t.Errorf("WorkerType.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestJobStep_TypeValidation tests that JobStep validation requires the Type field
func TestJobStep_TypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		jobDef      JobDefinition
		shouldError bool
		errorMsg    string
	}{
		{
			name: "step without type fails validation",
			jobDef: JobDefinition{
				ID:   "test-job-type-1",
				Name: "Test Job",
				Type: JobDefinitionTypeCrawler,
				Steps: []JobStep{
					{
						Name:    "test-step",
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: true,
			errorMsg:    "worker type is required",
		},
		{
			name: "step with invalid type fails validation",
			jobDef: JobDefinition{
				ID:   "test-job-type-2",
				Name: "Test Job",
				Type: JobDefinitionTypeCrawler,
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerType("invalid_type"),
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: true,
			errorMsg:    "invalid worker type",
		},
		{
			name: "step with valid type passes validation",
			jobDef: JobDefinition{
				ID:   "test-job-type-3",
				Name: "Test Job",
				Type: JobDefinitionTypeCrawler,
				Steps: []JobStep{
					{
						Name:    "test-step",
						Type:    WorkerTypeCrawler,
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "step with type and description passes validation",
			jobDef: JobDefinition{
				ID:   "test-job-type-4",
				Name: "Test Job",
				Type: JobDefinitionTypeAgent,
				Steps: []JobStep{
					{
						Name:        "keyword-extraction",
						Type:        WorkerTypeAgent,
						Description: "Extract keywords from document content",
						Config: map[string]interface{}{
							"agent_type": "keyword_extractor",
						},
						OnError: ErrorStrategyContinue,
					},
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.jobDef.Validate()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected validation error containing %q, got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

// TestAllWorkerTypes tests that AllWorkerTypes returns all expected worker types
func TestAllWorkerTypes(t *testing.T) {
	allTypes := AllWorkerTypes()

	expectedCount := 17
	if len(allTypes) != expectedCount {
		t.Errorf("AllWorkerTypes() returned %d types, expected %d", len(allTypes), expectedCount)
	}

	// Verify all types are valid
	for _, st := range allTypes {
		if !st.IsValid() {
			t.Errorf("AllWorkerTypes() contains invalid type: %q", st)
		}
	}

	// Verify expected types are present
	expectedTypes := map[WorkerType]bool{
		WorkerTypeAgent:            true,
		WorkerTypeCrawler:          true,
		WorkerTypePlacesSearch:     true,
		WorkerTypeWebSearch:        true,
		WorkerTypeGitHubRepo:       true,
		WorkerTypeGitHubActions:    true,
		WorkerTypeGitHubGit:        true,
		WorkerTypeTransform:        true,
		WorkerTypeReindex:          true,
		WorkerTypeLocalDir:         true,
		WorkerTypeCodeMap:          true,
		WorkerTypeSummary:          true,
		WorkerTypeTestJobGenerator: true,
		WorkerTypeAnalyzeBuild:     true,
		WorkerTypeClassify:         true,
		WorkerTypeDependencyGraph:  true,
		WorkerTypeAggregateSummary: true,
	}

	for _, st := range allTypes {
		if !expectedTypes[st] {
			t.Errorf("AllWorkerTypes() contains unexpected type: %q", st)
		}
		delete(expectedTypes, st)
	}

	if len(expectedTypes) > 0 {
		for st := range expectedTypes {
			t.Errorf("AllWorkerTypes() missing expected type: %q", st)
		}
	}
}
