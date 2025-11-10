package executor

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// PlacesSearchStepExecutor executes "places_search" action steps
type PlacesSearchStepExecutor struct {
	placesService interfaces.PlacesService
	logger        arbor.ILogger
}

// NewPlacesSearchStepExecutor creates a new places search step executor
func NewPlacesSearchStepExecutor(
	placesService interfaces.PlacesService,
	logger arbor.ILogger,
) *PlacesSearchStepExecutor {
	return &PlacesSearchStepExecutor{
		placesService: placesService,
		logger:        logger,
	}
}

// ExecuteStep executes a places search step
func (e *PlacesSearchStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
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

	e.logger.Info().
		Str("step_name", step.Name).
		Str("search_query", req.SearchQuery).
		Str("search_type", req.SearchType).
		Int("max_results", req.MaxResults).
		Msg("Executing places search step")

	// Execute search
	result, err := e.placesService.SearchPlaces(ctx, parentJobID, req)
	if err != nil {
		return "", fmt.Errorf("failed to search places: %w", err)
	}

	// Marshal result to JSON for storage in job progress
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal places result: %w", err)
	}

	e.logger.Info().
		Str("step_name", step.Name).
		Int("total_results", result.TotalResults).
		Str("parent_job_id", parentJobID).
		Msg("Places search step completed successfully")

	// Return JSON string to be stored in job progress
	return string(resultJSON), nil
}

// GetStepType returns "places_search"
func (e *PlacesSearchStepExecutor) GetStepType() string {
	return "places_search"
}
