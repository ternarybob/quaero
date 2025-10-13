package identifiers

import (
	"testing"

	"github.com/ternarybob/quaero/internal/models"
)

func TestExtractFromText(t *testing.T) {
	extractor := NewExtractor()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Single Jira issue",
			content:  "This fixes BUG-123 in the authentication module",
			expected: []string{"BUG-123"},
		},
		{
			name:     "Multiple Jira issues",
			content:  "Relates to BUG-123 and STORY-456, resolves TASK-789",
			expected: []string{"BUG-123", "STORY-456", "TASK-789"},
		},
		{
			name:     "Duplicate issues (case variations)",
			content:  "BUG-123 relates to Bug-123 and bug-123",
			expected: []string{"BUG-123"}, // Deduplication with uppercase normalization
		},
		{
			name:     "No identifiers",
			content:  "Just some regular text without any issue keys",
			expected: []string{},
		},
		{
			name:     "Mixed with GitHub PR",
			content:  "Fixed in BUG-123, see PR #456 for details",
			expected: []string{"BUG-123", "456"}, // Captures both Jira and GitHub PR
		},
		{
			name:     "Git commit SHA",
			content:  "Commit abc123def456 fixes the issue",
			expected: []string{"ABC123DEF456"}, // 12-char commit SHA
		},
		{
			name:     "Invalid patterns",
			content:  "Not-A-Match, 123-456 (no prefix)",
			expected: []string{}, // None of these match valid patterns (Note: TOOLONG-123456789 actually matches Jira pattern)
		},
		{
			name:     "Common Jira project keys",
			content:  "PROJ-1, TEST-999, DEV-42, INFRA-100",
			expected: []string{"PROJ-1", "TEST-999", "DEV-42", "INFRA-100"},
		},
		{
			name:     "Edge case: hyphen in text",
			content:  "The fix-me command handles BUG-456 correctly",
			expected: []string{"BUG-456"},
		},
		{
			name:     "URLs containing issue keys",
			content:  "See https://jira.example.com/browse/BUG-789",
			expected: []string{"BUG-789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ExtractFromText(tt.content)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d identifiers, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check each expected identifier is present
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected identifier %q not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestExtractFromDocuments(t *testing.T) {
	extractor := NewExtractor()

	tests := []struct {
		name     string
		docs     []*models.Document
		expected []string
	}{
		{
			name: "Extract from metadata",
			docs: []*models.Document{
				{
					ID:      "1",
					Title:   "Authentication bug",
					Content: "Session timeout issue",
					Metadata: map[string]interface{}{
						"issue_key": "BUG-123",
					},
				},
			},
			expected: []string{"BUG-123"},
		},
		{
			name: "Extract from referenced issues",
			docs: []*models.Document{
				{
					ID:      "1",
					Title:   "Design document",
					Content: "Architecture overview",
					Metadata: map[string]interface{}{
						"referenced_issues": []string{"BUG-123", "STORY-456"},
					},
				},
			},
			expected: []string{"BUG-123", "STORY-456"},
		},
		{
			name: "Extract from title and content",
			docs: []*models.Document{
				{
					ID:      "1",
					Title:   "Fix for BUG-123",
					Content: "This commit resolves STORY-456 and BUG-789",
				},
			},
			expected: []string{"BUG-123", "STORY-456", "BUG-789"},
		},
		{
			name: "Multiple documents with deduplication",
			docs: []*models.Document{
				{
					ID:      "1",
					Title:   "Jira ticket",
					Content: "BUG-123 reported",
					Metadata: map[string]interface{}{
						"issue_key": "BUG-123",
					},
				},
				{
					ID:       "2",
					Title:    "Confluence page",
					Content:  "Documented in BUG-123",
					Metadata: map[string]interface{}{},
				},
				{
					ID:       "3",
					Title:    "GitHub commit",
					Content:  "Fixed BUG-123 and added STORY-456",
					Metadata: map[string]interface{}{},
				},
			},
			expected: []string{"BUG-123", "STORY-456"}, // BUG-123 deduplicated
		},
		{
			name:     "Empty documents",
			docs:     []*models.Document{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ExtractFromDocuments(tt.docs)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d identifiers, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check each expected identifier is present
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected identifier %q not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestExtractJiraIssues(t *testing.T) {
	extractor := NewExtractor()

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "Only Jira issues",
			content:  "BUG-123, STORY-456, TASK-789",
			expected: []string{"BUG-123", "STORY-456", "TASK-789"},
		},
		{
			name:     "Mixed content - filter Jira only",
			content:  "BUG-123 fixed in commit abc123def and PR #456",
			expected: []string{"BUG-123"}, // Only Jira issue, not commit or PR
		},
		{
			name:     "No Jira issues",
			content:  "Commit abc123def456 merged via PR #789",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ExtractJiraIssues(tt.content)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d Jira issues, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check each expected issue is present
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected Jira issue %q not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	extractor := NewExtractor()

	// Note: GitHub PR pattern requires # prefix (e.g., "#123")
	// The ExtractFromText method captures just the digits without #
	identifiers := []string{"BUG-123", "STORY-456", "abc123def456"}

	tests := []struct {
		name     string
		idType   string
		expected []string
	}{
		{
			name:     "Filter Jira issues",
			idType:   "jira_issue",
			expected: []string{"BUG-123", "STORY-456"},
		},
		{
			name:     "Filter Git commits (lowercase hex)",
			idType:   "git_commit",
			expected: []string{"ABC123DEF456"}, // Uppercased by unique()
		},
		{
			name:     "Invalid type",
			idType:   "invalid_type",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.FilterByType(identifiers, tt.idType)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d identifiers, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check each expected identifier is present
			for _, expected := range tt.expected {
				found := false
				for _, actual := range result {
					if actual == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected identifier %q not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestIsJiraIssueKey(t *testing.T) {
	extractor := NewExtractor()

	tests := []struct {
		text     string
		expected bool
	}{
		{"BUG-123", true},
		{"STORY-456", true},
		{"PROJ-1", true},
		{"Invalid", false},
		{"123-456", false},
		{"BUG123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := extractor.IsJiraIssueKey(tt.text)
			if result != tt.expected {
				t.Errorf("IsJiraIssueKey(%q) = %v, expected %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestIsGitCommitSHA(t *testing.T) {
	extractor := NewExtractor()

	tests := []struct {
		text     string
		expected bool
	}{
		{"abc123def456", true},                         // 12 char
		{"abc123d", true},                              // 7 char (short)
		{"abc123def456789012345678901234567890", true}, // 40 char (full)
		{"abc123", false},                              // Too short (6 char)
		{"BUG-123", false},                             // Not a commit
		{"invalid", false},                             // Not hex
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := extractor.IsGitCommitSHA(tt.text)
			if result != tt.expected {
				t.Errorf("IsGitCommitSHA(%q) = %v, expected %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No duplicates",
			input:    []string{"A", "B", "C"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "With duplicates",
			input:    []string{"A", "B", "A", "C", "B"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "Case variations (normalized to uppercase)",
			input:    []string{"bug-123", "BUG-123", "Bug-123"},
			expected: []string{"BUG-123"},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "Single item",
			input:    []string{"A"},
			expected: []string{"A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unique(tt.input)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check each expected item is present (order matters)
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("At index %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}
