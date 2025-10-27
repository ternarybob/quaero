package search

import (
	"strings"

	"github.com/ternarybob/quaero/internal/models"
)

// containsReference checks if a document contains a specific reference
// Searches in title, content, source ID, and metadata fields
// Used by both FTS5SearchService and AdvancedSearchService
func containsReference(doc *models.Document, reference string) bool {
	// Check in title
	if strings.Contains(doc.Title, reference) {
		return true
	}

	// Check in content markdown
	if strings.Contains(doc.ContentMarkdown, reference) {
		return true
	}

	// Check in source ID (for references like issue keys)
	if strings.Contains(doc.SourceID, reference) {
		return true
	}

	// Check in metadata
	for _, value := range doc.Metadata {
		switch v := value.(type) {
		case string:
			if strings.Contains(v, reference) {
				return true
			}
		case []string:
			for _, item := range v {
				if strings.Contains(item, reference) {
					return true
				}
			}
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					if strings.Contains(str, reference) {
						return true
					}
				}
			}
		}
	}

	return false
}
