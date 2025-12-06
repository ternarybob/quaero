package search

import (
	"fmt"
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

// filterBySourceType filters documents by source type
// Used by both FTS5SearchService and AdvancedSearchService
func filterBySourceType(docs []*models.Document, sourceTypes []string) []*models.Document {
	filtered := make([]*models.Document, 0, len(docs))
	for _, doc := range docs {
		for _, sourceType := range sourceTypes {
			if doc.SourceType == sourceType {
				filtered = append(filtered, doc)
				break
			}
		}
	}
	return filtered
}

// filterByMetadata filters documents by metadata key-value pairs
// Used by both FTS5SearchService and AdvancedSearchService
func filterByMetadata(docs []*models.Document, filters map[string]string) []*models.Document {
	filtered := make([]*models.Document, 0, len(docs))
	for _, doc := range docs {
		if matchesMetadata(doc.Metadata, filters) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// filterByTags filters documents that have ALL specified tags (AND operation)
// Used by both FTS5SearchService and AdvancedSearchService
func filterByTags(docs []*models.Document, tags []string) []*models.Document {
	if len(tags) == 0 {
		return docs
	}

	filtered := make([]*models.Document, 0, len(docs))
	for _, doc := range docs {
		if hasAllTags(doc.Tags, tags) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// hasAllTags checks if docTags contains all required tags
func hasAllTags(docTags, requiredTags []string) bool {
	tagSet := make(map[string]bool, len(docTags))
	for _, tag := range docTags {
		tagSet[tag] = true
	}

	for _, required := range requiredTags {
		if !tagSet[required] {
			return false
		}
	}
	return true
}

// matchesMetadata checks if document metadata matches all filter criteria
// Used by filterByMetadata helper
func matchesMetadata(metadata map[string]interface{}, filters map[string]string) bool {
	for key, value := range filters {
		metaValue, exists := metadata[key]
		if !exists {
			return false
		}

		// Convert metadata value to string for comparison
		var metaStr string
		switch v := metaValue.(type) {
		case string:
			metaStr = v
		case []string:
			// Check if value is in array
			for _, item := range v {
				if item == value {
					goto nextFilter
				}
			}
			return false
		case []interface{}:
			// Check if value is in array
			for _, item := range v {
				if fmt.Sprintf("%v", item) == value {
					goto nextFilter
				}
			}
			return false
		default:
			metaStr = fmt.Sprintf("%v", v)
		}

		if metaStr != value {
			return false
		}

	nextFilter:
	}
	return true
}
