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

// AdvancedSearchService implements SearchService with Google-style query parsing
// Supports:
// - OR (default): "cat dog mat" → finds documents with any term
// - AND (+ prefix): "+cat +dog" → finds documents with all terms
// - Phrases (quotes): "cat on mat" → finds exact phrase
// - Qualifiers: "document_type:jira case:match"
// - Mixed queries: "+cat dog \"on mat\" document_type:jira"
//
// Examples:
//
//	cat dog                    → cat OR dog
//	+cat +dog                  → cat AND dog
//	+cat dog mat               → cat AND (dog OR mat)
//	"cat on mat"               → exact phrase
//	document_type:jira cat     → filter by Jira, search "cat"
//	case:match Cat             → case-sensitive search for "Cat"
type AdvancedSearchService struct {
	storage interfaces.DocumentStorage
	logger  arbor.ILogger
	parser  *QueryParser
	config  *common.Config
}

// ParsedQuery represents a parsed Google-style query
type ParsedQuery struct {
	// FTS5Query is the converted query for full-text search
	FTS5Query string

	// ID is the document ID for direct lookup (extracted from id: qualifier)
	ID string

	// DocumentType filters results by source type (extracted from document_type: qualifier)
	DocumentType string

	// CaseSensitive enables case-sensitive post-filtering (from case:match qualifier)
	CaseSensitive bool

	// Tokens stores all parsed tokens for case-sensitive filtering that respects AND/OR semantics
	Tokens []Token
}

// NewAdvancedSearchService creates a new advanced search service with Google-style parsing
func NewAdvancedSearchService(
	storage interfaces.DocumentStorage,
	logger arbor.ILogger,
	config *common.Config,
) *AdvancedSearchService {
	if logger == nil {
		logger = common.GetLogger()
	}
	return &AdvancedSearchService{
		storage: storage,
		logger:  logger,
		parser:  NewQueryParser(),
		config:  config,
	}
}

// Search performs a full-text search with Google-style query parsing
// Converts Google-style queries to FTS5 syntax and applies application-level filters
func (s *AdvancedSearchService) Search(
	ctx context.Context,
	query string,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	// Parse the query
	parsedQuery := s.parseQuery(query)

	s.logParsedQuery(query, parsedQuery)

	// Execute search (ListDocuments or FullTextSearch)
	results, err := s.executeSearch(query, parsedQuery, opts)
	if err != nil {
		return nil, err
	}

	// Apply all post-search filters
	results = s.applyFilters(results, parsedQuery, opts)

	s.logSearchCompletion(query, len(results))

	return results, nil
}

// logParsedQuery logs the parsed query details for debugging
func (s *AdvancedSearchService) logParsedQuery(query string, parsedQuery ParsedQuery) {
	s.logger.Debug().
		Str("original_query", query).
		Str("fts5_query", parsedQuery.FTS5Query).
		Str("id", parsedQuery.ID).
		Str("document_type", parsedQuery.DocumentType).
		Bool("case_sensitive", parsedQuery.CaseSensitive).
		Msg("Parsed query")
}

// executeSearch performs the appropriate search operation based on query type
// Uses GetByID for ID-based queries, ListDocuments for empty queries, or FullTextSearch for FTS5 queries
func (s *AdvancedSearchService) executeSearch(
	query string,
	parsedQuery ParsedQuery,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	// If ID is specified, do direct lookup
	if parsedQuery.ID != "" {
		doc, err := s.storage.GetDocument(parsedQuery.ID)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("id", parsedQuery.ID).
				Msg("Failed to get document by ID")
			// Return empty results instead of error for ID not found
			return []*models.Document{}, nil
		}
		return []*models.Document{doc}, nil
	}

	// If FTS5 query is empty, list all documents (for filter-only queries)
	if parsedQuery.FTS5Query == "" {
		return s.executeListDocuments(parsedQuery, opts)
	}

	// Execute FTS5 search
	return s.executeFullTextSearch(query, parsedQuery, opts)
}

// executeListDocuments handles filter-only queries with no search terms
func (s *AdvancedSearchService) executeListDocuments(
	parsedQuery ParsedQuery,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 1000 // Higher default for list operations
	}

	// Use OrderBy/OrderDir from SearchOptions if specified, otherwise default to updated_at desc
	orderBy := opts.OrderBy
	if orderBy == "" {
		orderBy = "updated_at"
	}
	orderDir := opts.OrderDir
	if orderDir == "" {
		orderDir = "desc"
	}

	listOpts := &interfaces.ListOptions{
		Limit:    limit,
		Offset:   opts.Offset,
		OrderBy:  orderBy,
		OrderDir: orderDir,
		Tags:     opts.Tags, // Push tags filter to DB level for efficiency
	}

	// Push document_type filter to DB level for efficiency
	if parsedQuery.DocumentType != "" {
		listOpts.SourceType = parsedQuery.DocumentType
	}

	results, err := s.storage.ListDocuments(listOpts)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Failed to list documents")
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

// executeFullTextSearch handles FTS5 search queries
func (s *AdvancedSearchService) executeFullTextSearch(
	query string,
	parsedQuery ParsedQuery,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	limit := s.calculateSearchLimit(opts.Limit, parsedQuery.CaseSensitive)

	results, err := s.storage.FullTextSearch(parsedQuery.FTS5Query, limit)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("query", query).
			Str("fts5_query", parsedQuery.FTS5Query).
			Msg("Failed to search documents")
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

// calculateSearchLimit determines the appropriate limit for FTS5 queries
// Increases limit for case-sensitive searches to account for post-filtering
func (s *AdvancedSearchService) calculateSearchLimit(requestedLimit int, caseSensitive bool) int {
	limit := requestedLimit
	if limit == 0 {
		limit = 100 // Default limit
	}

	// If case-sensitive filtering is enabled, fetch more results to account for post-filtering
	// Case-sensitive filtering may eliminate some results, so we fetch extra to avoid under-delivery
	if caseSensitive && s.config != nil {
		// Use configured multiplier and max cap
		multiplier := s.config.Search.CaseSensitiveMultiplier
		maxCap := s.config.Search.CaseSensitiveMaxCap

		limit = limit * multiplier
		if limit > maxCap {
			limit = maxCap
		}
	}

	return limit
}

// applyFilters applies all post-search filters to the result set
// Handles document type, source type, case sensitivity, metadata, and limit
func (s *AdvancedSearchService) applyFilters(
	results []*models.Document,
	parsedQuery ParsedQuery,
	opts interfaces.SearchOptions,
) []*models.Document {
	// Apply document type filter from qualifier (only for FTS5 queries, not empty queries)
	// Empty queries already filtered at DB level via ListOptions.SourceType
	if parsedQuery.DocumentType != "" && parsedQuery.FTS5Query != "" {
		results = s.applyDocumentTypeFilter(results, parsedQuery.DocumentType)
	}

	// Apply document type filter from options
	if len(opts.SourceTypes) > 0 {
		results = filterBySourceType(results, opts.SourceTypes)
	}

	// Apply case sensitivity filter
	if parsedQuery.CaseSensitive {
		results = s.applyCaseSensitivity(results, parsedQuery.Tokens)
	}

	// Apply metadata filters if specified
	if len(opts.MetadataFilters) > 0 {
		results = filterByMetadata(results, opts.MetadataFilters)
	}

	// Apply tags filter if specified (documents must have ALL tags)
	if len(opts.Tags) > 0 {
		results = filterByTags(results, opts.Tags)
	}

	// Apply limit after filters
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results
}

// logSearchCompletion logs the final search results count
func (s *AdvancedSearchService) logSearchCompletion(query string, resultCount int) {
	s.logger.Debug().
		Str("query", query).
		Int("results", resultCount).
		Msg("Advanced search completed")
}

// GetByID retrieves a single document by its ID
func (s *AdvancedSearchService) GetByID(ctx context.Context, id string) (*models.Document, error) {
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
func (s *AdvancedSearchService) SearchByReference(
	ctx context.Context,
	reference string,
	opts interfaces.SearchOptions,
) ([]*models.Document, error) {
	// Quote the reference for FTS5 to treat it as a literal phrase
	// This prevents special characters (like dashes) from being interpreted as operators
	// Double quotes are escaped by doubling them for FTS5
	quotedReference := `"` + strings.ReplaceAll(reference, `"`, `""`) + `"`

	// Use FullTextSearch from storage layer directly to bypass query parsing
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

// parseQuery converts a Google-style query into ParsedQuery struct
// Handles tokenization, qualifier extraction, and FTS5 conversion
func (s *AdvancedSearchService) parseQuery(query string) ParsedQuery {
	// Tokenize the query
	tokens := s.parser.Tokenize(query)

	// Extract qualifiers
	qualifiers := s.parser.ExtractQualifiers(tokens)

	// Build FTS5 query from tokens
	fts5Query := s.parser.BuildFTS5Query(tokens)

	// Parse qualifiers
	id := qualifiers["id"]
	documentType := qualifiers["document_type"]
	caseSensitive := qualifiers["case"] == "match"

	return ParsedQuery{
		FTS5Query:     fts5Query,
		ID:            id,
		DocumentType:  documentType,
		CaseSensitive: caseSensitive,
		Tokens:        tokens,
	}
}

// applyDocumentTypeFilter filters results by document source type
// Matches the SourceType field against the document_type qualifier
func (s *AdvancedSearchService) applyDocumentTypeFilter(docs []*models.Document, docType string) []*models.Document {
	filtered := make([]*models.Document, 0, len(docs))
	for _, doc := range docs {
		// Case-insensitive comparison
		if strings.EqualFold(doc.SourceType, docType) {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// applyCaseSensitivity performs case-sensitive post-filtering on search results
// FTS5 is case-insensitive by default, so we filter results after retrieval
// This respects AND/OR semantics:
// - All required terms/phrases (+ prefix) must be present with exact case
// - At least one optional term/phrase must be present with exact case (if any optionals exist)
//
// Limitation: This approach requires fetching more results from FTS5 than needed,
// as some may be filtered out. Consider increasing the FTS5 limit if case-sensitive
// searches return too few results.
func (s *AdvancedSearchService) applyCaseSensitivity(docs []*models.Document, tokens []Token) []*models.Document {
	if len(tokens) == 0 {
		return docs
	}

	// Separate required and optional tokens (excluding qualifiers)
	var requiredTokens []Token
	var optionalTokens []Token

	for _, token := range tokens {
		if token.Type == TokenTypeQualifier {
			continue // Skip qualifiers
		}

		if token.Required {
			requiredTokens = append(requiredTokens, token)
		} else {
			optionalTokens = append(optionalTokens, token)
		}
	}

	filtered := make([]*models.Document, 0, len(docs))

	for _, doc := range docs {
		content := doc.Title + " " + doc.ContentMarkdown

		// Check all required terms/phrases are present with exact case
		allRequiredMatch := true
		for _, token := range requiredTokens {
			if !strings.Contains(content, token.Value) {
				allRequiredMatch = false
				break
			}
		}

		if !allRequiredMatch {
			continue
		}

		// If there are optional terms/phrases, at least one must match with exact case
		if len(optionalTokens) > 0 {
			atLeastOneOptionalMatch := false
			for _, token := range optionalTokens {
				if strings.Contains(content, token.Value) {
					atLeastOneOptionalMatch = true
					break
				}
			}

			if !atLeastOneOptionalMatch {
				continue
			}
		}

		// All conditions met
		filtered = append(filtered, doc)
	}

	return filtered
}
