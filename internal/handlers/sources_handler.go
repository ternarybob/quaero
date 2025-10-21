package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// SourcesHandler handles HTTP requests for source management
type SourcesHandler struct {
	sourceService *sources.Service
	logger        arbor.ILogger
}

// NewSourcesHandler creates a new SourcesHandler
func NewSourcesHandler(sourceService *sources.Service, logger arbor.ILogger) *SourcesHandler {
	return &SourcesHandler{
		sourceService: sourceService,
		logger:        logger,
	}
}

// ListSourcesHandler handles GET /api/sources
func (h *SourcesHandler) ListSourcesHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	sources, err := h.sourceService.ListSources(r.Context())
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list sources")
		WriteError(w, http.StatusInternalServerError, "Failed to list sources")
		return
	}

	// Return sources array directly (not wrapped in object)
	// If sources is nil, return empty array
	if sources == nil {
		sources = []*models.SourceConfig{}
	}

	WriteJSON(w, http.StatusOK, sources)
}

// GetSourceHandler handles GET /api/sources/{id}
func (h *SourcesHandler) GetSourceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "GET") {
		return
	}

	// Extract ID from URL path
	id := extractIDFromPath(r.URL.Path, "/api/sources/")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	source, err := h.sourceService.GetSource(r.Context(), id)
	if err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to get source")
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "Source not found")
		} else {
			WriteError(w, http.StatusInternalServerError, "Failed to get source")
		}
		return
	}

	WriteJSON(w, http.StatusOK, source)
}

// CreateSourceHandler handles POST /api/sources
func (h *SourcesHandler) CreateSourceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "POST") {
		return
	}

	var source models.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Sanitize filters before validation and saving
	sanitizeSourceFilters(&source)

	if err := h.sourceService.CreateSource(r.Context(), &source); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create source")
		if strings.Contains(err.Error(), "validation failed") {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else {
			WriteError(w, http.StatusInternalServerError, "Failed to create source")
		}
		return
	}

	WriteJSON(w, http.StatusCreated, source)
}

// UpdateSourceHandler handles PUT /api/sources/{id}
func (h *SourcesHandler) UpdateSourceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "PUT") {
		return
	}

	// Extract ID from URL path
	id := extractIDFromPath(r.URL.Path, "/api/sources/")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	var source models.SourceConfig
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode request body")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set ID from path to prevent ID mismatch
	source.ID = id

	// Sanitize filters before validation and saving
	sanitizeSourceFilters(&source)

	if err := h.sourceService.UpdateSource(r.Context(), &source); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to update source")
		if strings.Contains(err.Error(), "validation failed") {
			WriteError(w, http.StatusBadRequest, err.Error())
		} else if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "Source not found")
		} else {
			WriteError(w, http.StatusInternalServerError, "Failed to update source")
		}
		return
	}

	WriteJSON(w, http.StatusOK, source)
}

// DeleteSourceHandler handles DELETE /api/sources/{id}
func (h *SourcesHandler) DeleteSourceHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, "DELETE") {
		return
	}

	// Extract ID from URL path
	id := extractIDFromPath(r.URL.Path, "/api/sources/")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "Source ID is required")
		return
	}

	if err := h.sourceService.DeleteSource(r.Context(), id); err != nil {
		h.logger.Error().Err(err).Str("id", id).Msg("Failed to delete source")
		if strings.Contains(err.Error(), "not found") {
			WriteError(w, http.StatusNotFound, "Source not found")
		} else {
			WriteError(w, http.StatusInternalServerError, "Failed to delete source")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractIDFromPath extracts the ID from a URL path
// Example: "/api/sources/abc-123" with prefix "/api/sources/" returns "abc-123"
func extractIDFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	return id
}

// sanitizeSourceFilters normalizes filter pattern strings by:
// - Trimming overall whitespace
// - Converting empty/whitespace-only strings to empty string
// - Trimming whitespace around individual comma-delimited tokens
func sanitizeSourceFilters(source *models.SourceConfig) {
	if source.Filters == nil {
		return
	}

	// Sanitize include_patterns
	if val, ok := source.Filters["include_patterns"]; ok && val != nil {
		if strVal, isString := val.(string); isString {
			source.Filters["include_patterns"] = sanitizePatternString(strVal)
		}
	}

	// Sanitize exclude_patterns
	if val, ok := source.Filters["exclude_patterns"]; ok && val != nil {
		if strVal, isString := val.(string); isString {
			source.Filters["exclude_patterns"] = sanitizePatternString(strVal)
		}
	}
}

// sanitizePatternString trims whitespace and normalizes comma-delimited patterns
func sanitizePatternString(pattern string) string {
	// Trim overall whitespace
	pattern = strings.TrimSpace(pattern)

	// If empty after trim, return empty string
	if pattern == "" {
		return ""
	}

	// Split by comma, trim each token, and rejoin
	tokens := strings.Split(pattern, ",")
	var cleaned []string
	for _, token := range tokens {
		trimmed := strings.TrimSpace(token)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	// Return empty string if no valid tokens
	if len(cleaned) == 0 {
		return ""
	}

	// Join back with comma
	return strings.Join(cleaned, ",")
}
