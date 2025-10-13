package chat

import (
	"strings"

	"github.com/ternarybob/quaero/internal/models"
)

// deduplicateDocuments removes duplicate documents by ID
// Preserves the first occurrence of each document
func deduplicateDocuments(docs []*models.Document) []*models.Document {
	if len(docs) == 0 {
		return []*models.Document{}
	}

	seen := make(map[string]bool)
	result := make([]*models.Document, 0, len(docs))

	for _, doc := range docs {
		if doc == nil {
			continue
		}

		if !seen[doc.ID] {
			seen[doc.ID] = true
			result = append(result, doc)
		}
	}

	return result
}

// rankByCrossSourceConnections scores documents based on cross-source connections
// Documents that reference more identifiers get higher scores
// Returns documents sorted by score (highest first)
func rankByCrossSourceConnections(docs []*models.Document, identifiers []string) []*models.Document {
	if len(docs) == 0 {
		return []*models.Document{}
	}

	// Create a case-insensitive identifier lookup map
	identifierMap := make(map[string]bool)
	for _, id := range identifiers {
		identifierMap[strings.ToUpper(id)] = true
	}

	// Score each document
	type scoredDoc struct {
		doc   *models.Document
		score int
	}

	scored := make([]scoredDoc, 0, len(docs))

	for _, doc := range docs {
		score := 0

		// Check metadata.issue_key
		if issueKey, ok := doc.Metadata["issue_key"].(string); ok {
			if identifierMap[strings.ToUpper(issueKey)] {
				score += 10 // High weight for direct issue_key match
			}
		}

		// Check metadata.referenced_issues
		if referencedIssues, ok := doc.Metadata["referenced_issues"].([]interface{}); ok {
			for _, ref := range referencedIssues {
				if refStr, ok := ref.(string); ok {
					if identifierMap[strings.ToUpper(refStr)] {
						score += 5 // Medium weight for referenced issue
					}
				}
			}
		}

		// Check title for identifier mentions
		titleUpper := strings.ToUpper(doc.Title)
		for identifier := range identifierMap {
			if strings.Contains(titleUpper, identifier) {
				score += 3 // Lower weight for title mention
			}
		}

		// Check content for identifier mentions (sample first 1000 chars for performance)
		contentSample := doc.Content
		if len(contentSample) > 1000 {
			contentSample = contentSample[:1000]
		}
		contentUpper := strings.ToUpper(contentSample)
		for identifier := range identifierMap {
			if strings.Contains(contentUpper, identifier) {
				score += 1 // Lowest weight for content mention
			}
		}

		// Bonus for cross-source documents
		// If document is from different source than the identifier's typical source
		// (e.g., Confluence doc referencing Jira issue)
		if score > 0 {
			sourceType := strings.ToLower(doc.SourceType)
			isCrossSource := false

			// Jira issue referenced in non-Jira document
			if issueKey, ok := doc.Metadata["issue_key"].(string); ok {
				if sourceType != "jira" && issueKey != "" {
					isCrossSource = true
				}
			}

			// Check if document references issues from other sources
			if referencedIssues, ok := doc.Metadata["referenced_issues"].([]interface{}); ok {
				if len(referencedIssues) > 0 {
					isCrossSource = true
				}
			}

			if isCrossSource {
				score += 20 // High bonus for true cross-source linking
			}
		}

		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score (descending)
	// Simple bubble sort for small arrays (fine for typical RAG context sizes)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Extract sorted documents
	result := make([]*models.Document, len(scored))
	for i, sd := range scored {
		result[i] = sd.doc
	}

	return result
}
