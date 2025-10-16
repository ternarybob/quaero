package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// FTS5SearchService implements SearchService using SQLite FTS5 full-text search
type FTS5SearchService struct {
	storage interfaces.DocumentStorage
	logger  arbor.ILogger
}

// NewFTS5SearchService creates a new FTS5-based search service
func NewFTS5SearchService(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
) *FTS5SearchService {
	return &FTS5SearchService{
		storage: storage,
		logger:  logger,
	}
}

// Search performs a full-text search across documents
func (s *FTS5SearchService) Search(
	ctx context.Context,
	query string,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	var results []*models.Document
	var err error

	// If query is empty, list all documents (for filter-only queries)
	if query == "" {
		limit := opts.Limit
		if limit == 0 {
			limit = 1000 // Higher default for list operations
		}

		listOpts := &interfaces.ListOptions{
			Limit:    limit,
			Offset:   0,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		results, err = s.storage.ListDocuments(listOpts)
		if err != nil {
			if s.logger != nil {
				s.logger.Error().
					Err(err).
					Msg("Failed to list documents")
			}
			return nil, fmt.Errorf("search failed: %w", err)
		}
	} else {
		// Use FullTextSearch from storage layer
		limit := opts.Limit
		if limit == 0 {
			limit = 100 // Default limit
		}

		results, err = s.storage.FullTextSearch(query, limit)
		if err != nil {
			if s.logger != nil {
				s.logger.Error().
					Err(err).
					Str("query", query).
					Msg("Failed to search documents")
			}
			return nil, fmt.Errorf("search failed: %w", err)
		}
	}

	// Apply source type filter if specified
	if len(opts.SourceTypes) > 0 {
		results = filterBySourceType(results, opts.SourceTypes)
	}

	// Apply metadata filters if specified
	if len(opts.MetadataFilters) > 0 {
		results = filterByMetadata(results, opts.MetadataFilters)
	}

	// Apply limit after filters
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	if s.logger != nil {
		s.logger.Debug().
			Str("query", query).
			Int("results", len(results)).
			Msg("Search completed")
	}

	return results, nil
}

// GetByID retrieves a single document by its ID
func (s *FTS5SearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	doc, err := s.storage.GetDocument(id)
	if err != nil {
		if s.logger != nil {
			s.logger.Error().
				Err(err).
				Str("id", id).
				Msg("Failed to get document by ID")
		}
		return nil, fmt.Errorf("get document failed: %w", err)
	}

	return doc, nil
}

// SearchByReference finds documents containing a specific reference
// (e.g., issue keys like "PROJ-123" or user mentions like "@alice")
func (s *FTS5SearchService) SearchByReference(
	ctx context.Context,
	reference string,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	// Quote the reference for FTS5 to treat it as a literal phrase
	// This prevents special characters (like dashes) from being interpreted as operators
	quotedReference := `"` + strings.ReplaceAll(reference, `"`, `""`) + `"`

	// Use FullTextSearch from storage layer
	limit := opts.Limit
	if limit == 0 {
		limit = 100 // Default limit
	}

	results, err := s.storage.FullTextSearch(quotedReference, limit)
	if err != nil {
		if s.logger != nil {
			s.logger.Error().
				Err(err).
				Str("reference", reference).
				Msg("Failed to search by reference")
		}
		return nil, fmt.Errorf("search by reference failed: %w", err)
	}

	// Filter results to ensure they actually contain the reference
	// This is important because FTS5 may return partial matches
	filtered := make([]*models.Document, 0, len(results))
	for _, doc := range results {
		if containsReference(doc, reference) {
			filtered = append(filtered, doc)
		}
	}

	// Apply source type filter if specified
	if len(opts.SourceTypes) > 0 {
		filtered = filterBySourceType(filtered, opts.SourceTypes)
	}

	// Apply limit
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}

	if s.logger != nil {
		s.logger.Debug().
			Str("reference", reference).
			Int("results", len(filtered)).
			Msg("Search by reference completed")
	}

	return filtered, nil
}

// filterBySourceType filters documents by source type
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
func filterByMetadata(docs []*models.Document, filters map[string]string) []*models.Document {
	filtered := make([]*models.Document, 0, len(docs))
	for _, doc := range docs {
		if matchesMetadata(doc.Metadata, filters) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// matchesMetadata checks if document metadata matches all filter criteria
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

// containsReference checks if a document contains a specific reference
func containsReference(doc *models.Document, reference string) bool {
	// Check in title
	if strings.Contains(doc.Title, reference) {
		return true
	}

	// Check in content markdown
	if strings.Contains(doc.ContentMarkdown, reference) {
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
