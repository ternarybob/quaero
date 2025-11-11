package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// PlacesSearchManager orchestrates Google Places API search workflows and document creation
type PlacesSearchManager struct {
	placesService   interfaces.PlacesService
	documentService interfaces.DocumentService
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// Compile-time assertion: PlacesSearchManager implements StepManager interface
var _ interfaces.StepManager = (*PlacesSearchManager)(nil)

// NewPlacesSearchManager creates a new places search manager for orchestrating Google Places API searches
func NewPlacesSearchManager(
	placesService interfaces.PlacesService,
	documentService interfaces.DocumentService,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *PlacesSearchManager {
	return &PlacesSearchManager{
		placesService:   placesService,
		documentService: documentService,
		eventService:    eventService,
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

	// Convert search result to document for storage
	doc, err := m.convertPlacesResultToDocument(result, parentJobID, jobDef.Tags)
	if err != nil {
		return "", fmt.Errorf("failed to convert places result to document: %w", err)
	}

	// Save document to database
	if err := m.documentService.SaveDocument(ctx, doc); err != nil {
		return "", fmt.Errorf("failed to save places document: %w", err)
	}

	m.logger.Info().
		Str("document_id", doc.ID).
		Str("document_title", doc.Title).
		Int("places_count", result.TotalResults).
		Msg("Places search result saved as document")

	// Publish document_saved event for parent job document count tracking
	// This is a generic, job-type agnostic event that ANY manager can publish
	if m.eventService != nil && parentJobID != "" {
		payload := map[string]interface{}{
			"job_id":        parentJobID, // For places jobs, the parent job is the job itself
			"parent_job_id": parentJobID,
			"document_id":   doc.ID,
			"source_type":   "places",
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		event := interfaces.Event{
			Type:    interfaces.EventDocumentSaved,
			Payload: payload,
		}
		// Publish asynchronously to not block document save
		go func() {
			if err := m.eventService.Publish(context.Background(), event); err != nil {
				m.logger.Warn().
					Err(err).
					Str("document_id", doc.ID).
					Str("parent_job_id", parentJobID).
					Msg("Failed to publish document_saved event")
			} else {
				m.logger.Debug().
					Str("document_id", doc.ID).
					Str("parent_job_id", parentJobID).
					Msg("Published document_saved event for parent job document count")
			}
		}()
	}

	// Return parent job ID as placeholder since this is a synchronous operation
	return parentJobID, nil
}

// GetManagerType returns "places_search" - the action type this manager handles
func (m *PlacesSearchManager) GetManagerType() string {
	return "places_search"
}

// convertPlacesResultToDocument converts a PlacesSearchResult to a Document for storage
func (m *PlacesSearchManager) convertPlacesResultToDocument(result *models.PlacesSearchResult, jobID string, tags []string) (*models.Document, error) {
	// Generate document ID from job ID and timestamp
	docID := fmt.Sprintf("doc_places_%s", jobID)

	// Build markdown content with formatted places list
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# Places Search Results: %s\n\n", result.SearchQuery))
	contentBuilder.WriteString(fmt.Sprintf("**Search Type:** %s\n", result.SearchType))
	contentBuilder.WriteString(fmt.Sprintf("**Total Results:** %d\n\n", result.TotalResults))
	contentBuilder.WriteString("## Places\n\n")

	for i, place := range result.Places {
		contentBuilder.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, place.Name))

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
		contentBuilder.WriteString("---\n\n")
	}

	// Convert result to metadata map
	metadataBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal places result to metadata: %w", err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal places metadata: %w", err)
	}

	now := time.Now()
	doc := &models.Document{
		ID:              docID,
		SourceType:      "places",
		SourceID:        jobID,
		Title:           fmt.Sprintf("Places Search: %s", result.SearchQuery),
		ContentMarkdown: contentBuilder.String(),
		DetailLevel:     models.DetailLevelFull,
		Metadata:        metadata,
		URL:             "",
		Tags:            tags,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	return doc, nil
}
