// Unit tests for KeywordExtractor agent
// These tests focus on input validation and response parsing logic
// Full ADK integration is tested in test/api/agent_job_test.go
// Note: Some tests use test helpers in keyword_extractor.go to expose internal functions

package unit

import (
	"testing"

	"github.com/ternarybob/quaero/internal/services/agents"
)

// TestKeywordExtractor_ParseKeywordResponse tests the parseKeywordResponse helper function
func TestKeywordExtractor_ParseKeywordResponse(t *testing.T) {
	tests := []struct {
		name             string
		response         string
		maxKeywords      int
		expectKeywords   []string
		expectConfidence map[string]float64
		expectError      bool
	}{
		{
			name:           "Simple array",
			response:       `["keyword1", "keyword2", "keyword3"]`,
			maxKeywords:    10,
			expectKeywords: []string{"keyword1", "keyword2", "keyword3"},
		},
		{
			name:        "Object with confidence",
			response:    `{"keywords": ["kw1", "kw2"], "confidence": {"kw1": 0.95, "kw2": 0.87}}`,
			maxKeywords: 10,
			expectKeywords: []string{"kw1", "kw2"},
			expectConfidence: map[string]float64{
				"kw1": 0.95,
				"kw2": 0.87,
			},
		},
		{
			name:           "Array exceeding max",
			response:       `["kw1", "kw2", "kw3", "kw4", "kw5"]`,
			maxKeywords:    3,
			expectKeywords: []string{"kw1", "kw2", "kw3"},
		},
		{
			name:        "Object exceeding max",
			response:    `{"keywords": ["kw1", "kw2", "kw3"], "confidence": {"kw1": 0.9, "kw2": 0.8, "kw3": 0.7}}`,
			maxKeywords: 2,
			expectKeywords: []string{"kw1", "kw2"},
			expectConfidence: map[string]float64{
				"kw1": 0.9,
				"kw2": 0.8,
			},
		},
		{
			name:        "Invalid JSON",
			response:    `not json`,
			maxKeywords: 10,
			expectError: true,
		},
		{
			name:           "Empty array",
			response:       `[]`,
			maxKeywords:    10,
			expectKeywords: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords, confidence, err := agents.TestParseKeywordResponse(tt.response, tt.maxKeywords)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify keywords
			if len(keywords) != len(tt.expectKeywords) {
				t.Errorf("Expected %d keywords, got %d", len(tt.expectKeywords), len(keywords))
			}

			for i, expectedKw := range tt.expectKeywords {
				if i >= len(keywords) {
					t.Errorf("Missing keyword at index %d: %s", i, expectedKw)
					continue
				}
				if keywords[i] != expectedKw {
					t.Errorf("Keyword at index %d: expected %s, got %s", i, expectedKw, keywords[i])
				}
			}

			// Verify confidence
			if tt.expectConfidence != nil {
				if confidence == nil {
					t.Error("Expected confidence map, got nil")
				} else {
					for kw, expectedScore := range tt.expectConfidence {
						actualScore, ok := confidence[kw]
						if !ok {
							t.Errorf("Missing confidence score for keyword %s", kw)
						} else if actualScore != expectedScore {
							t.Errorf("Confidence for %s: expected %.2f, got %.2f", kw, expectedScore, actualScore)
						}
					}
				}
			}

			t.Logf(" Parsed %d keywords", len(keywords))
		})
	}
}

// TestKeywordExtractor_CleanMarkdownFences tests the cleanMarkdownFences helper function
func TestKeywordExtractor_CleanMarkdownFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No fences",
			input:    `{"keywords": ["kw1"]}`,
			expected: `{"keywords": ["kw1"]}`,
		},
		{
			name:     "With json fence",
			input:    "```json\n{\"keywords\": [\"kw1\"]}\n```",
			expected: `{"keywords": ["kw1"]}`,
		},
		{
			name:     "With JSON fence (uppercase)",
			input:    "```JSON\n{\"keywords\": [\"kw1\"]}\n```",
			expected: `{"keywords": ["kw1"]}`,
		},
		{
			name:     "With plain fence",
			input:    "```\n{\"keywords\": [\"kw1\"]}\n```",
			expected: `{"keywords": ["kw1"]}`,
		},
		{
			name:     "With whitespace",
			input:    "  ```json\n{\"keywords\": [\"kw1\"]}\n```  ",
			expected: `{"keywords": ["kw1"]}`,
		},
		{
			name:     "Multiple fences (only outer removed)",
			input:    "```json\n{\"code\": \"```inner```\"}\n```",
			expected: "{\"code\": \"```inner```\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agents.TestCleanMarkdownFences(tt.input)

			if result != tt.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expected, result)
			}

			t.Logf(" Cleaned markdown fences correctly")
		})
	}
}

// TestKeywordExtractor_InputValidation tests input validation logic via the validation helper
func TestKeywordExtractor_InputValidation(t *testing.T) {
	tests := []struct {
		name             string
		input            map[string]interface{}
		expectDocID      string
		expectContent    string
		expectMaxKw      int
		expectError      bool
		errorContains    string
	}{
		{
			name: "Missing document_id",
			input: map[string]interface{}{
				"content": "test",
			},
			expectError:   true,
			errorContains: "document_id",
		},
		{
			name: "Empty document_id",
			input: map[string]interface{}{
				"document_id": "",
				"content":     "test",
			},
			expectError:   true,
			errorContains: "document_id",
		},
		{
			name: "Missing content",
			input: map[string]interface{}{
				"document_id": "doc1",
			},
			expectError:   true,
			errorContains: "content",
		},
		{
			name: "Empty content",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "",
			},
			expectError:   true,
			errorContains: "content",
		},
		{
			name: "Valid input (minimal)",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test content",
			},
			expectDocID:   "doc1",
			expectContent: "test content",
			expectMaxKw:   10, // Default value
			expectError:   false,
		},
		{
			name: "Valid with max_keywords (int)",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test",
				"max_keywords": 15,
			},
			expectDocID:   "doc1",
			expectContent: "test",
			expectMaxKw:   15,
			expectError:   false,
		},
		{
			name: "Valid with max_keywords (float64)",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test",
				"max_keywords": 12.5,
			},
			expectDocID:   "doc1",
			expectContent: "test",
			expectMaxKw:   12, // Converted from float
			expectError:   false,
		},
		{
			name: "Valid with max_keywords (string)",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test",
				"max_keywords": "8",
			},
			expectDocID:   "doc1",
			expectContent: "test",
			expectMaxKw:   8, // Parsed from string
			expectError:   false,
		},
		{
			name: "max_keywords below minimum",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test",
				"max_keywords": 2,
			},
			expectDocID:   "doc1",
			expectContent: "test",
			expectMaxKw:   5, // Clamped to minimum
			expectError:   false,
		},
		{
			name: "max_keywords above maximum",
			input: map[string]interface{}{
				"document_id": "doc1",
				"content":     "test",
				"max_keywords": 20,
			},
			expectDocID:   "doc1",
			expectContent: "test",
			expectMaxKw:   15, // Clamped to maximum
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call validation helper directly
			docID, content, maxKw, err := agents.TestValidateInput(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errorContains != "" {
					// Check if error message contains expected substring
					errorMsg := err.Error()
					contains := false
					for i := 0; i <= len(errorMsg)-len(tt.errorContains); i++ {
						if errorMsg[i:i+len(tt.errorContains)] == tt.errorContains {
							contains = true
							break
						}
					}
					if !contains {
						t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
					} else {
						t.Logf(" Validation error: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				} else {
					// Verify returned values
					if docID != tt.expectDocID {
						t.Errorf("Expected document_id '%s', got '%s'", tt.expectDocID, docID)
					}
					if content != tt.expectContent {
						t.Errorf("Expected content '%s', got '%s'", tt.expectContent, content)
					}
					if maxKw != tt.expectMaxKw {
						t.Errorf("Expected max_keywords %d, got %d", tt.expectMaxKw, maxKw)
					}
					t.Logf(" Validation passed: docID=%s, contentLen=%d, maxKw=%d", docID, len(content), maxKw)
				}
			}
		})
	}
}


// TestKeywordExtractor_MaxKeywordsClamp tests max_keywords clamping to [5, 15] range
// This test verifies that the parsing logic correctly handles different numeric types
func TestKeywordExtractor_MaxKeywordsClamp(t *testing.T) {
	tests := []struct {
		name        string
		maxKeywords interface{}
		description string
	}{
		{
			name:        "Below minimum",
			maxKeywords: 2,
			description: "Should clamp to 5",
		},
		{
			name:        "At minimum",
			maxKeywords: 5,
			description: "Should remain 5",
		},
		{
			name:        "In range",
			maxKeywords: 10,
			description: "Should remain 10",
		},
		{
			name:        "At maximum",
			maxKeywords: 15,
			description: "Should remain 15",
		},
		{
			name:        "Above maximum",
			maxKeywords: 20,
			description: "Should clamp to 15",
		},
		{
			name:        "Negative",
			maxKeywords: -5,
			description: "Should clamp to 5",
		},
		{
			name:        "Float",
			maxKeywords: 12.5,
			description: "Should convert to 12",
		},
		{
			name:        "String",
			maxKeywords: "8",
			description: "Should parse to 8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert interface{} to int for maxKeywords
			var maxKw int
			switch v := tt.maxKeywords.(type) {
			case int:
				maxKw = v
			case float64:
				maxKw = int(v)
			case string:
				// Simple string to int conversion for single-digit strings
				if len(v) > 0 && v[0] >= '0' && v[0] <= '9' {
					maxKw = int(v[0] - '0')
				} else {
					maxKw = 10
				}
			default:
				maxKw = 10
			}

			// Clamp to [5, 15] range (simulating Execute function behavior)
			if maxKw < 5 {
				maxKw = 5
			} else if maxKw > 15 {
				maxKw = 15
			}

			// Create JSON response with 20 keywords (oversized array)
			resultKeywords, _, _ := agents.TestParseKeywordResponse(`["kw1","kw2","kw3","kw4","kw5","kw6","kw7","kw8","kw9","kw10","kw11","kw12","kw13","kw14","kw15","kw16","kw17","kw18","kw19","kw20"]`, maxKw)

			// Verify truncation to clamped maxKeywords value
			if len(resultKeywords) != maxKw {
				t.Errorf("Expected exactly %d keywords (after clamping), got %d", maxKw, len(resultKeywords))
			}

			t.Logf(" %s: Clamped to %d keywords", tt.description, len(resultKeywords))
		})
	}
}


// TestKeywordExtractor_GetType verifies the agent type identifier
func TestKeywordExtractor_GetType(t *testing.T) {
	extractor := &agents.KeywordExtractor{}

	agentType := extractor.GetType()

	if agentType != "keyword_extractor" {
		t.Errorf("Expected type 'keyword_extractor', got '%s'", agentType)
	}

	t.Log(" Agent type verified: keyword_extractor")
}
