package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// PlacesSearchManager orchestrates Google Places API search workflows and document creation
type PlacesSearchManager struct {
	placesService   interfaces.PlacesService
	documentService interfaces.DocumentService
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	authStorage     interfaces.AuthStorage
	logger          arbor.ILogger
}

// Compile-time assertion: PlacesSearchManager implements StepManager interface
var _ interfaces.StepManager = (*PlacesSearchManager)(nil)

// NewPlacesSearchManager creates a new places search manager for orchestrating Google Places API searches
func NewPlacesSearchManager(
	placesService interfaces.PlacesService,
	documentService interfaces.DocumentService,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	authStorage interfaces.AuthStorage,
	logger arbor.ILogger,
) *PlacesSearchManager {
	return &PlacesSearchManager{
		placesService:   placesService,
		documentService: documentService,
		eventService:    eventService,
		kvStorage:       kvStorage,
		authStorage:     authStorage,
		logger:          logger,
	}
}

// CreateParentJob executes a places search operation using the Google Places API.
// Searches for places matching the query and creates documents for each result.
// Returns a placeholder job ID since places search doesn't create async jobs.
func (m *PlacesSearchManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return "", fmt.Errorf("step config is required for places_search")
	}

	// Extract search_query (required)
	searchQuery, ok := stepConfig["search_query"].(string)
	if !ok || searchQuery == "" {
		return "", fmt.Errorf("search_query is required in step config")
	}

	// Extract search_type (required, default to "text_search")
	searchType, ok := stepConfig["search_type"].(string)
	if !ok || searchType == "" {
		searchType = "text_search"
	}

	// Validate search_type
	if searchType != "text_search" && searchType != "nearby_search" {
		return "", fmt.Errorf("search_type must be one of: text_search, nearby_search")
	}

	// Check for API key in step config and resolve it from KV store
	if apiKeyName, ok := stepConfig["api_key"].(string); ok && apiKeyName != "" {
		// Strip curly braces if present (e.g., "{google_places_api_key}" → "google_places_api_key")
		// This handles cases where variable substitution didn't happen during job definition loading
		cleanAPIKeyName := strings.Trim(apiKeyName, "{}")

		resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.kvStorage, cleanAPIKeyName, "")
		if err != nil {
			return "", fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
		}
		m.logger.Info().
			Str("step_name", step.Name).
			Str("api_key_name", cleanAPIKeyName).
			Msg("Resolved API key from storage for places search execution")
		stepConfig["resolved_api_key"] = resolvedAPIKey
	}

	// Build search request
	req := &models.PlacesSearchRequest{
		SearchQuery: searchQuery,
		SearchType:  searchType,
	}

	// Extract optional max_results
	if maxResults, ok := stepConfig["max_results"].(float64); ok {
		req.MaxResults = int(maxResults)
	} else if maxResults, ok := stepConfig["max_results"].(int); ok {
		req.MaxResults = maxResults
	}

	// Extract optional list_name
	if listName, ok := stepConfig["list_name"].(string); ok {
		req.ListName = listName
	}

	// Extract location for nearby_search
	if searchType == "nearby_search" {
		locationMap, ok := stepConfig["location"].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("location is required for nearby_search")
		}

		latitude, ok := locationMap["latitude"].(float64)
		if !ok {
			return "", fmt.Errorf("location.latitude is required for nearby_search")
		}

		longitude, ok := locationMap["longitude"].(float64)
		if !ok {
			return "", fmt.Errorf("location.longitude is required for nearby_search")
		}

		req.Location = &models.Location{
			Latitude:  latitude,
			Longitude: longitude,
		}

		// Optional radius
		if radius, ok := locationMap["radius"].(float64); ok {
			req.Location.Radius = int(radius)
		} else if radius, ok := locationMap["radius"].(int); ok {
			req.Location.Radius = radius
		}
	}

	// Extract optional filters
	if filters, ok := stepConfig["filters"].(map[string]interface{}); ok {
		req.Filters = filters
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("search_query", req.SearchQuery).
		Str("search_type", req.SearchType).
		Int("max_results", req.MaxResults).
		Msg("Orchestrating places search")

	// Execute search
	result, err := m.placesService.SearchPlaces(ctx, parentJobID, req)
	if err != nil {
		return "", fmt.Errorf("failed to search places: %w", err)
	}

	// Marshal result to JSON for logging purposes
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal places result: %w", err)
	}

	m.logger.Debug().
		Str("step_name", step.Name).
		Str("result_json", string(resultJSON)).
		Msg("Places search result marshaled")

	m.logger.Info().
		Str("step_name", step.Name).
		Int("total_results", result.TotalResults).
		Str("parent_job_id", parentJobID).
		Msg("Places search orchestration completed successfully")

	// Create individual documents for each place
	docs, err := m.createPlaceDocuments(result, parentJobID, jobDef.Tags)
	if err != nil {
		return "", fmt.Errorf("failed to create place documents: %w", err)
	}

	// Save each document to database and publish events
	savedCount := 0
	for _, doc := range docs {
		if err := m.documentService.SaveDocument(ctx, doc); err != nil {
			m.logger.Warn().
				Err(err).
				Str("document_id", doc.ID).
				Str("place_name", doc.Title).
				Msg("Failed to save place document")
			continue // Continue with other documents even if one fails
		}

		savedCount++

		m.logger.Debug().
			Str("document_id", doc.ID).
			Str("place_name", doc.Title).
			Msg("Place document saved successfully")

		// Publish document_saved event for each document
		// This is a generic, job-type agnostic event that ANY manager can publish
		if m.eventService != nil && parentJobID != "" {
			docID := doc.ID // Capture for goroutine
			payload := map[string]interface{}{
				"job_id":        parentJobID, // For places jobs, the parent job is the job itself
				"parent_job_id": parentJobID,
				"document_id":   docID,
				"source_type":   "places",
				"timestamp":     time.Now().Format(time.RFC3339),
			}
			event := interfaces.Event{
				Type:    interfaces.EventDocumentSaved,
				Payload: payload,
			}
			// Publish synchronously to ensure document count is updated before job completes
			if err := m.eventService.PublishSync(context.Background(), event); err != nil {
				m.logger.Warn().
					Err(err).
					Str("document_id", docID).
					Str("parent_job_id", parentJobID).
					Msg("Failed to publish document_saved event")
			} else {
				m.logger.Debug().
					Str("document_id", docID).
					Str("parent_job_id", parentJobID).
					Msg("Published document_saved event for parent job document count")
			}
		}
	}

	m.logger.Info().
		Int("documents_created", savedCount).
		Int("total_results", result.TotalResults).
		Msg("Places search results saved as individual documents")

	// Return parent job ID as placeholder since this is a synchronous operation
	return parentJobID, nil
}

// GetManagerType returns "places_search" - the action type this manager handles
func (m *PlacesSearchManager) GetManagerType() string {
	return "places_search"
}

// ReturnsChildJobs returns false since places search is synchronous
func (m *PlacesSearchManager) ReturnsChildJobs() bool {
	return false
}

// createPlaceDocuments creates individual documents for each place in the search results
func (m *PlacesSearchManager) createPlaceDocuments(result *models.PlacesSearchResult, jobID string, tags []string) ([]*models.Document, error) {
	docs := make([]*models.Document, 0, len(result.Places))
	now := time.Now()

	for _, place := range result.Places {
		// Generate unique document ID using place_id
		docID := fmt.Sprintf("doc_place_%s", place.PlaceID)

		// Build markdown content for this individual place
		var contentBuilder strings.Builder
		contentBuilder.WriteString(fmt.Sprintf("# %s\n\n", place.Name))

		if place.FormattedAddress != "" {
			contentBuilder.WriteString(fmt.Sprintf("**Address:** %s\n\n", place.FormattedAddress))
		}

		if place.Rating > 0 {
			contentBuilder.WriteString(fmt.Sprintf("**Rating:** %.1f/5.0 (%d reviews)\n\n", place.Rating, place.UserRatingsTotal))
		}

		if place.Website != "" {
			contentBuilder.WriteString(fmt.Sprintf("**Website:** %s\n\n", place.Website))
		}

		if place.PhoneNumber != "" {
			contentBuilder.WriteString(fmt.Sprintf("**Phone:** %s\n\n", place.PhoneNumber))
		}

		if len(place.Types) > 0 {
			contentBuilder.WriteString(fmt.Sprintf("**Types:** %s\n\n", strings.Join(place.Types, ", ")))
		}

		if place.Latitude != 0 && place.Longitude != 0 {
			contentBuilder.WriteString(fmt.Sprintf("**Location:** %.6f, %.6f\n\n", place.Latitude, place.Longitude))
		}

		contentBuilder.WriteString(fmt.Sprintf("**Place ID:** %s\n\n", place.PlaceID))

		// Convert place to metadata map
		placeMetadata := map[string]interface{}{
			"place_id":           place.PlaceID,
			"name":               place.Name,
			"formatted_address":  place.FormattedAddress,
			"rating":             place.Rating,
			"user_ratings_total": place.UserRatingsTotal,
			"website":            place.Website,
			"phone_number":       place.PhoneNumber,
			"types":              place.Types,
			"latitude":           place.Latitude,
			"longitude":          place.Longitude,
			"search_query":       result.SearchQuery,
			"search_type":        result.SearchType,
			"job_id":             jobID, // Track which job created this document
		}

		// Create document for this place
		doc := &models.Document{
			ID:              docID,
			SourceType:      "places",
			SourceID:        place.PlaceID, // Use place_id as source_id for uniqueness
			Title:           place.Name,
			ContentMarkdown: contentBuilder.String(),
			DetailLevel:     models.DetailLevelFull,
			Metadata:        placeMetadata,
			URL:             place.Website, // Use website as URL if available
			Tags:            tags,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		docs = append(docs, doc)
	}

	return docs, nil
}
