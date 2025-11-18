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
						Action:  "crawl",
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
						Action:  "crawl",
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
						Action:  "crawl",
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
						Action:  "crawl",
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
						Action:  "crawl",
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
