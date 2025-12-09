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
// Supports:
//   - Flat keys: "category" matches metadata["category"]
//   - Nested keys: "rule_classifier.category" matches metadata["rule_classifier"]["category"]
//   - Multi-value: "build,config,docs" matches if value equals any of these
//
// Used by filterByMetadata helper
func matchesMetadata(metadata map[string]interface{}, filters map[string]string) bool {
	for key, filterValue := range filters {
		// Get the metadata value (supports nested keys via dot notation)
		metaValue := getNestedValue(metadata, key)
		if metaValue == nil {
			return false
		}

		// Parse filter value - support comma-separated multi-value filters
		allowedValues := strings.Split(filterValue, ",")
		for i := range allowedValues {
			allowedValues[i] = strings.TrimSpace(allowedValues[i])
		}

		// Check if metadata value matches any allowed value
		if !matchesAnyValue(metaValue, allowedValues) {
			return false
		}
	}
	return true
}

// getNestedValue retrieves a value from nested map using dot notation
// e.g., "rule_classifier.category" retrieves metadata["rule_classifier"]["category"]
func getNestedValue(data map[string]interface{}, key string) interface{} {
	parts := strings.Split(key, ".")

	var current interface{} = data
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, exists := v[part]
			if !exists {
				return nil
			}
			current = val
		default:
			return nil
		}
	}
	return current
}

// matchesAnyValue checks if metaValue matches any of the allowed values
func matchesAnyValue(metaValue interface{}, allowedValues []string) bool {
	// Convert metadata value to string(s) for comparison
	switch v := metaValue.(type) {
	case string:
		for _, allowed := range allowedValues {
			if v == allowed {
				return true
			}
		}
		return false
	case []string:
		// Check if any item in array matches any allowed value
		for _, item := range v {
			for _, allowed := range allowedValues {
				if item == allowed {
					return true
				}
			}
		}
		return false
	case []interface{}:
		// Check if any item in array matches any allowed value
		for _, item := range v {
			itemStr := fmt.Sprintf("%v", item)
			for _, allowed := range allowedValues {
				if itemStr == allowed {
					return true
				}
			}
		}
		return false
	default:
		metaStr := fmt.Sprintf("%v", v)
		for _, allowed := range allowedValues {
			if metaStr == allowed {
				return true
			}
		}
		return false
	}
}
