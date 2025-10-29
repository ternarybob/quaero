// -----------------------------------------------------------------------
// Last Modified: Monday, 27th January 2025 6:30:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/search"
)

// SearchResult represents a single search result with complete document data
type SearchResult struct {
	ID              string                 `json:"id"`
	SourceType      string                 `json:"source_type"`
	SourceID        string                 `json:"source_id"`
	Title           string                 `json:"title"`
	ContentMarkdown string                 `json:"content_markdown"`
	URL             string                 `json:"url"`
	DetailLevel     string                 `json:"detail_level"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	Brief           string                 `json:"brief"` // Kept for backward compatibility
}

// SearchHandler handles search-related HTTP requests
type SearchHandler struct {
	searchService interfaces.SearchService
	logger        arbor.ILogger
}

// NewSearchHandler creates a new search handler with dependencies
func NewSearchHandler(searchService interfaces.SearchService, logger arbor.ILogger) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		logger:        logger,
	}
}

// SearchHandler handles GET /api/search?q=query requests
func (h *SearchHandler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	// Method validation
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	// Parse query parameters
	query := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Set defaults
	limit := 50
	offset := 0

	// Parse limit with default and max enforcement
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	// Clamp limit to valid range
	if limit <= 0 {
		limit = 50 // Default when invalid
	}
	if limit > 100 {
		limit = 100 // Enforce maximum limit
	}

	// Parse offset
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsed
		}
	}

	// Clamp offset to non-negative
	if offset < 0 {
		offset = 0
	}

	// Log the search request
	if h.logger != nil {
		h.logger.Info().
			Str("query", query).
			Int("limit", limit).
			Int("offset", offset).
			Msg("Search request received")
	}

	// Build SearchOptions
	opts := interfaces.SearchOptions{
		Limit:  limit,
		Offset: offset,
	}

	// Execute search
	ctx := r.Context()
	documents, err := h.searchService.Search(ctx, query, opts)
	if err != nil {
		// Check if search service is disabled (FTS5 not enabled)
		if errors.Is(err, search.ErrSearchDisabled) {
			if h.logger != nil {
				h.logger.Warn().
					Str("query", query).
					Msg("Search unavailable: FTS5 is not enabled")
			}
			WriteError(w, http.StatusServiceUnavailable, "Search functionality is unavailable: FTS5 is required but not enabled in configuration")
			return
		}

		// Other errors
		if h.logger != nil {
			h.logger.Error().
				Err(err).
				Str("query", query).
				Msg("Failed to execute search")
		}
		WriteError(w, http.StatusInternalServerError, "Failed to execute search")
		return
	}

	// Transform results
	results := make([]SearchResult, 0, len(documents))
	for _, doc := range documents {
		// Truncate ContentMarkdown to 200 characters for brief
		brief := doc.ContentMarkdown
		if len(brief) > 200 {
			brief = brief[:200] + "..."
		}

		results = append(results, SearchResult{
			ID:              doc.ID,
			SourceType:      doc.SourceType,
			SourceID:        doc.SourceID,
			Title:           doc.Title,
			ContentMarkdown: doc.ContentMarkdown,
			URL:             doc.URL,
			DetailLevel:     doc.DetailLevel,
			Metadata:        doc.Metadata,
			CreatedAt:       doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:       doc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Brief:           brief,
		})
	}

	// Log search completion
	if h.logger != nil {
		h.logger.Debug().
			Str("query", query).
			Int("results", len(results)).
			Msg("Search completed")
	}

	// Build response
	response := map[string]interface{}{
		"results": results,
		"count":   len(results),
		"query":   query,
		"limit":   limit,
		"offset":  offset,
	}

	// Return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
