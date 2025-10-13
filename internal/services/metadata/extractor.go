package metadata

import (
	"regexp"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// Extractor extracts structured metadata from document content using regex patterns
type Extractor struct {
	logger arbor.ILogger

	// Compiled regex patterns
	jiraIssuePattern      *regexp.Regexp
	userMentionPattern    *regexp.Regexp
	prRefPattern          *regexp.Regexp
	confluencePagePattern *regexp.Regexp
}

// NewExtractor creates a new metadata extractor
func NewExtractor(logger arbor.ILogger) *Extractor {
	return &Extractor{
		logger:                logger,
		jiraIssuePattern:      regexp.MustCompile(`[A-Z]+-\d+`),
		userMentionPattern:    regexp.MustCompile(`@\w+`),
		prRefPattern:          regexp.MustCompile(`#\d+`),
		confluencePagePattern: regexp.MustCompile(`page:\d+`),
	}
}

// ExtractMetadata extracts structured metadata from a document's title and content
func (e *Extractor) ExtractMetadata(doc *models.Document) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	// Combine title and content for extraction
	combinedText := doc.Title + " " + doc.Content

	// Extract Jira issue keys
	if issueKeys := e.extractUniqueMatches(e.jiraIssuePattern, combinedText); len(issueKeys) > 0 {
		metadata["issue_keys"] = issueKeys
	}

	// Extract user mentions
	if mentions := e.extractUniqueMatches(e.userMentionPattern, combinedText); len(mentions) > 0 {
		metadata["mentions"] = mentions
	}

	// Extract PR references
	if prRefs := e.extractUniqueMatches(e.prRefPattern, doc.Content); len(prRefs) > 0 {
		metadata["pr_refs"] = prRefs
	}

	// Extract Confluence page references
	if pageRefs := e.extractUniqueMatches(e.confluencePagePattern, doc.Content); len(pageRefs) > 0 {
		metadata["confluence_pages"] = pageRefs
	}

	return metadata, nil
}

// MergeMetadata merges extracted metadata with existing metadata
// Extracted metadata takes precedence over existing metadata
func (e *Extractor) MergeMetadata(existing, extracted map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = make(map[string]interface{})
	}

	if extracted == nil {
		return existing
	}

	// Create a copy of existing metadata
	merged := make(map[string]interface{})
	for k, v := range existing {
		merged[k] = v
	}

	// Overwrite with extracted metadata
	for k, v := range extracted {
		merged[k] = v
	}

	return merged
}

// extractUniqueMatches extracts all unique matches for a pattern in the given text
func (e *Extractor) extractUniqueMatches(pattern *regexp.Regexp, text string) []string {
	matches := pattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Use map to deduplicate
	seen := make(map[string]bool)
	var unique []string

	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			unique = append(unique, match)
		}
	}

	return unique
}
