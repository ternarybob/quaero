// -----------------------------------------------------------------------
// Package identifiers provides services for extracting and managing
// cross-source identifiers (e.g., JIRA-123, BUG-456) from documents.
// -----------------------------------------------------------------------

package identifiers

import (
	"regexp"
	"strings"

	"github.com/ternarybob/quaero/internal/models"
)

// Extractor extracts cross-source identifiers from text and documents
type Extractor struct {
	patterns map[string]*regexp.Regexp
}

// NewExtractor creates a new identifier extractor with predefined patterns
func NewExtractor() *Extractor {
	return &Extractor{
		patterns: map[string]*regexp.Regexp{
			// Jira-style issue keys: PROJECT-123, BUG-456, STORY-789
			// Pattern: 1+ uppercase letters, hyphen, 1+ digits
			"jira_issue": regexp.MustCompile(`\b([A-Z]+\-\d+)\b`),

			// GitHub-style patterns (for future use)
			// PR numbers: #123, #456
			"github_pr": regexp.MustCompile(`\B#(\d+)\b`),

			// Commit SHA (short form): abc123d, def456a (7+ hex chars)
			// Pattern: word boundary, 7-40 hex chars, word boundary
			"git_commit": regexp.MustCompile(`\b([a-f0-9]{7,40})\b`),
		},
	}
}

// ExtractFromText finds all identifiers in text content
func (e *Extractor) ExtractFromText(content string) []string {
	var identifiers []string

	for _, pattern := range e.patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				identifiers = append(identifiers, match[1])
			}
		}
	}

	return unique(identifiers)
}

// ExtractFromDocuments extracts identifiers from a list of documents
// This includes both metadata-based identifiers and content-based extraction
func (e *Extractor) ExtractFromDocuments(docs []*models.Document) []string {
	var allIdentifiers []string

	for _, doc := range docs {
		// 1. Extract from metadata (highest priority)
		// Try to get issue_key from metadata map
		if issueKey, ok := doc.Metadata["issue_key"].(string); ok && issueKey != "" {
			allIdentifiers = append(allIdentifiers, issueKey)
		}

		// Try to get referenced_issues from metadata map
		if referencedIssues, ok := doc.Metadata["referenced_issues"].([]interface{}); ok {
			for _, ref := range referencedIssues {
				if refStr, ok := ref.(string); ok {
					allIdentifiers = append(allIdentifiers, refStr)
				}
			}
		}

		// Handle referenced_issues as []string (alternative serialization)
		if referencedIssues, ok := doc.Metadata["referenced_issues"].([]string); ok {
			allIdentifiers = append(allIdentifiers, referencedIssues...)
		}

		// 2. Extract from title
		if doc.Title != "" {
			allIdentifiers = append(allIdentifiers, e.ExtractFromText(doc.Title)...)
		}

		// 3. Extract from content markdown
		allIdentifiers = append(allIdentifiers, e.ExtractFromText(doc.ContentMarkdown)...)
	}

	return unique(allIdentifiers)
}

// ExtractJiraIssues extracts only Jira-style issue keys from text
func (e *Extractor) ExtractJiraIssues(content string) []string {
	pattern := e.patterns["jira_issue"]
	var issues []string

	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			issues = append(issues, match[1])
		}
	}

	return unique(issues)
}

// FilterByType filters identifiers by their type (jira_issue, github_pr, git_commit)
func (e *Extractor) FilterByType(identifiers []string, idType string) []string {
	pattern, ok := e.patterns[idType]
	if !ok {
		return []string{}
	}

	var filtered []string
	for _, id := range identifiers {
		// For git_commit pattern, check lowercase version since hex must be lowercase [a-f0-9]
		testString := id
		if idType == "git_commit" {
			testString = strings.ToLower(id)
		}

		if pattern.MatchString(testString) {
			filtered = append(filtered, id)
		}
	}

	return unique(filtered)
}

// unique removes duplicate strings from a slice while preserving order
func unique(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}

	seen := make(map[string]bool)
	result := []string{}

	for _, item := range items {
		// Normalize to uppercase for comparison (Jira issues are case-insensitive)
		normalized := strings.ToUpper(item)

		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}

	return result
}

// IsJiraIssueKey checks if a string matches the Jira issue key pattern
func (e *Extractor) IsJiraIssueKey(text string) bool {
	pattern := e.patterns["jira_issue"]
	return pattern.MatchString(text)
}

// IsGitCommitSHA checks if a string matches the Git commit SHA pattern
func (e *Extractor) IsGitCommitSHA(text string) bool {
	pattern := e.patterns["git_commit"]
	return pattern.MatchString(text)
}
