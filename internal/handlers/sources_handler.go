package handlers

import (
	"encoding/json"
	"fmt"
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

	// Validate filters before processing
	if err := validateSourceFilters(&source); err != nil {
		h.logger.Warn().Err(err).Str("type", string(source.Type)).Msg("Filter validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Filter validation failed: %s", err.Error()))
		return
	}

	// Log filter configuration for debugging (without sensitive data)
	if source.Filters != nil && len(source.Filters) > 0 {
		filterKeys := make([]string, 0, len(source.Filters))
		for key := range source.Filters {
			filterKeys = append(filterKeys, key)
		}
		h.logger.Debug().Str("type", string(source.Type)).Strs("filter_keys", filterKeys).Msg("Filter validation passed")
	}

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

	// Validate filters before processing
	if err := validateSourceFilters(&source); err != nil {
		h.logger.Warn().Err(err).Str("id", id).Str("type", string(source.Type)).Msg("Filter validation failed")
		WriteError(w, http.StatusBadRequest, fmt.Sprintf("Filter validation failed: %s", err.Error()))
		return
	}

	// Log filter configuration for debugging (without sensitive data)
	if source.Filters != nil && len(source.Filters) > 0 {
		filterKeys := make([]string, 0, len(source.Filters))
		for key := range source.Filters {
			filterKeys = append(filterKeys, key)
		}
		h.logger.Debug().Str("id", id).Str("type", string(source.Type)).Strs("filter_keys", filterKeys).Msg("Filter validation passed")
	}

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

// validateSourceFilters validates URL pattern filter format
func validateSourceFilters(source *models.SourceConfig) error {
	if source.Filters == nil {
		return nil // No filters is valid
	}

	// Validate include patterns
	if includePatterns, exists := source.Filters["include_patterns"]; exists {
		if err := validateURLPatterns(includePatterns, "include patterns"); err != nil {
			return err
		}
	}

	// Validate exclude patterns
	if excludePatterns, exists := source.Filters["exclude_patterns"]; exists {
		if err := validateURLPatterns(excludePatterns, "exclude patterns"); err != nil {
			return err
		}
	}

	// Check for unsupported filter keys
	for key := range source.Filters {
		if key != "include_patterns" && key != "exclude_patterns" {
			return fmt.Errorf("unsupported filter key: %s (supported: include_patterns, exclude_patterns)", key)
		}
	}

	return nil
}

// validateURLPatterns validates that patterns are properly formatted
func validateURLPatterns(patterns interface{}, patternType string) error {
	switch v := patterns.(type) {
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("%s cannot be empty array", patternType)
		}
		for i, pattern := range v {
			if str, ok := pattern.(string); !ok || strings.TrimSpace(str) == "" {
				return fmt.Errorf("%s item %d must be non-empty string", patternType, i)
			}
		}
	case []string:
		if len(v) == 0 {
			return fmt.Errorf("%s cannot be empty array", patternType)
		}
		for i, pattern := range v {
			if strings.TrimSpace(pattern) == "" {
				return fmt.Errorf("%s item %d cannot be empty", patternType, i)
			}
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s cannot be empty string", patternType)
		}
	default:
		return fmt.Errorf("%s must be array of strings or single string", patternType)
	}
	return nil
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
