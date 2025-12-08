package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// FTS5SearchService implements SearchService using Badger full-text search
type FTS5SearchService struct {
	storage interfaces.DocumentStorage
	logger  arbor.ILogger
}

// NewFTS5SearchService creates a new FTS5-based search service
func NewFTS5SearchService(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
) *FTS5SearchService {
	if logger == nil {
		logger = common.GetLogger()
	}
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
			Offset:   opts.Offset,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}

		results, err = s.storage.ListDocuments(listOpts)
		if err != nil {
			s.logger.Error().
				Err(err).
				Msg("Failed to list documents")
			return nil, fmt.Errorf("search failed: %w", err)
		}

		// Debug: Log document count and tag info before filtering
		s.logger.Debug().
			Int("docs_before_filter", len(results)).
			Strs("filter_tags", opts.Tags).
			Msg("Listed documents before tag filtering")

		// Debug: Sample document tags
		for i, doc := range results {
			if i < 3 { // Log first 3 docs
				s.logger.Debug().
					Str("doc_id", doc.ID).
					Strs("doc_tags", doc.Tags).
					Msg("Sample document tags")
			}
		}
	} else {
		// Use FullTextSearch from storage layer
		limit := opts.Limit
		if limit == 0 {
			limit = 100 // Default limit
		}

		results, err = s.storage.FullTextSearch(query, limit)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("query", query).
				Msg("Failed to search documents")
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

	// Apply tags filter if specified (documents must have ALL tags)
	if len(opts.Tags) > 0 {
		beforeTagFilter := len(results)
		results = filterByTags(results, opts.Tags)
		s.logger.Debug().
			Int("before_tag_filter", beforeTagFilter).
			Int("after_tag_filter", len(results)).
			Strs("tags", opts.Tags).
			Msg("Tag filtering results")
	}

	// Apply limit after filters
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	s.logger.Debug().
		Str("query", query).
		Int("results", len(results)).
		Msg("Search completed")

	return results, nil
}

// GetByID retrieves a single document by its ID
func (s *FTS5SearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
	doc, err := s.storage.GetDocument(id)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("id", id).
			Msg("Failed to get document by ID")
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
		s.logger.Error().
			Err(err).
			Str("reference", reference).
			Msg("Failed to search by reference")
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

	s.logger.Debug().
		Str("reference", reference).
		Int("results", len(filtered)).
		Msg("Search by reference completed")

	return filtered, nil
}
