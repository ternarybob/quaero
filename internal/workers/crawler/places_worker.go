// -----------------------------------------------------------------------
// PlacesWorker - Worker for Google Places API search operations
// -----------------------------------------------------------------------

package crawler

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
	"github.com/ternarybob/quaero/internal/queue"
)

// PlacesWorker handles Google Places API search operations.
// This worker executes places search jobs synchronously (no child jobs).
type PlacesWorker struct {
	placesService   interfaces.PlacesService
	documentService interfaces.DocumentService
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager // For unified job logging
}

// Compile-time assertion: PlacesWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*PlacesWorker)(nil)

// NewPlacesWorker creates a new places search worker
func NewPlacesWorker(
	placesService interfaces.PlacesService,
	documentService interfaces.DocumentService,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *PlacesWorker {
	return &PlacesWorker{
		placesService:   placesService,
		documentService: documentService,
		eventService:    eventService,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypePlacesSearch for the DefinitionWorker interface
func (w *PlacesWorker) GetType() models.WorkerType {
	return models.WorkerTypePlacesSearch
}

// Init performs the initialization/setup phase for a places search step.
// This is where we validate configuration and prepare search parameters.
func (w *PlacesWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for places_search")
	}

	// Extract search_query (required)
	searchQuery, ok := stepConfig["search_query"].(string)
	if !ok || searchQuery == "" {
		return nil, fmt.Errorf("search_query is required in step config")
	}

	// Extract search_type (required, default to "text_search")
	searchType, _ := stepConfig["search_type"].(string)
	if searchType == "" {
		searchType = "text_search"
	}

	// Validate search_type
	if searchType != "text_search" && searchType != "nearby_search" {
		return nil, fmt.Errorf("search_type must be one of: text_search, nearby_search")
	}

	// Resolve API key if present
	resolvedAPIKey := ""
	if apiKeyValue, ok := stepConfig["api_key"].(string); ok && apiKeyValue != "" {
		if strings.HasPrefix(apiKeyValue, "{") && strings.HasSuffix(apiKeyValue, "}") {
			cleanAPIKeyName := strings.Trim(apiKeyValue, "{}")
			var err error
			resolvedAPIKey, err = common.ResolveAPIKey(ctx, w.kvStorage, cleanAPIKeyName, "")
			if err != nil {
				return nil, fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
			}
			w.logger.Info().
				Str("phase", "init").
				Str("step_name", step.Name).
				Str("api_key_name", cleanAPIKeyName).
				Msg("Resolved API key from storage")
		} else {
			resolvedAPIKey = apiKeyValue
		}
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Str("search_query", searchQuery).
		Str("search_type", searchType).
		Msg("Places search worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   searchQuery,
				Name: fmt.Sprintf("Places: %s", searchQuery),
				Type: "places_search",
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"search_query":     searchQuery,
			"search_type":      searchType,
			"resolved_api_key": resolvedAPIKey,
			"step_config":      stepConfig,
		},
	}, nil
}

// CreateJobs executes a places search operation using the Google Places API.
// Searches for places matching the query and creates documents for each result.
// Returns the step job ID since places search executes synchronously.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
func (w *PlacesWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize places_search worker: %w", err)
		}
	}

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})
	if stepConfig == nil {
		stepConfig = step.Config
	}

	// Apply resolved API key
	if resolvedAPIKey, ok := initResult.Metadata["resolved_api_key"].(string); ok && resolvedAPIKey != "" {
		stepConfig["resolved_api_key"] = resolvedAPIKey
	}

	// Get manager_id from step job's parent_id for event aggregation
	managerID := ""
	if stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID); err == nil && stepJobInterface != nil {
		if stepJob, ok := stepJobInterface.(*models.QueueJobState); ok && stepJob != nil && stepJob.ParentID != nil {
			managerID = *stepJob.ParentID
		}
	}
	_ = managerID // Used for logging context

	// Extract search parameters from init result
	searchQuery, _ := initResult.Metadata["search_query"].(string)
	searchType, _ := initResult.Metadata["search_type"].(string)

	w.logger.Info().
		Str("phase", "run").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Str("search_query", searchQuery).
		Str("search_type", searchType).
		Msg("Starting places search from init result")

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
		var latitude, longitude float64
		var radius int
		var hasLocation bool

		// Try flat fields first (new format)
		if lat, ok := stepConfig["location_latitude"].(float64); ok {
			latitude = lat
			hasLocation = true
		} else if lat, ok := stepConfig["location_latitude"].(int64); ok {
			latitude = float64(lat)
			hasLocation = true
		}

		if lon, ok := stepConfig["location_longitude"].(float64); ok {
			longitude = lon
		} else if lon, ok := stepConfig["location_longitude"].(int64); ok {
			longitude = float64(lon)
		}

		if rad, ok := stepConfig["location_radius"].(float64); ok {
			radius = int(rad)
		} else if rad, ok := stepConfig["location_radius"].(int64); ok {
			radius = int(rad)
		}

		// Fallback to nested location map (legacy format)
		if !hasLocation {
			if locationMap, ok := stepConfig["location"].(map[string]interface{}); ok {
				if lat, ok := locationMap["latitude"].(float64); ok {
					latitude = lat
					hasLocation = true
				}
				if lon, ok := locationMap["longitude"].(float64); ok {
					longitude = lon
				}
				if rad, ok := locationMap["radius"].(float64); ok {
					radius = int(rad)
				} else if rad, ok := locationMap["radius"].(int); ok {
					radius = rad
				}
			}
		}

		if !hasLocation {
			return "", fmt.Errorf("location is required for nearby_search (use location_latitude/location_longitude or location map)")
		}

		req.Location = &models.Location{
			Latitude:  latitude,
			Longitude: longitude,
			Radius:    radius,
		}
	}

	// Extract optional filters
	if filters, ok := stepConfig["filters"].(map[string]interface{}); ok {
		req.Filters = filters
	}

	w.logger.Info().
		Str("step_name", step.Name).
		Str("search_query", req.SearchQuery).
		Str("search_type", req.SearchType).
		Int("max_results", req.MaxResults).
		Msg("Orchestrating places search")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, managerID, step.Name, "info",
		fmt.Sprintf("Starting places search: %s", req.SearchQuery),
		map[string]interface{}{
			"search_type": req.SearchType,
			"max_results": req.MaxResults,
		})

	// Execute search
	result, err := w.placesService.SearchPlaces(ctx, stepID, req)
	if err != nil {
		w.logJobEvent(ctx, stepID, managerID, step.Name, "error",
			fmt.Sprintf("Places search failed: %v", err), nil)
		return "", fmt.Errorf("failed to search places: %w", err)
	}

	// Marshal result to JSON for logging purposes
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal places result: %w", err)
	}

	w.logger.Debug().
		Str("step_name", step.Name).
		Str("result_json", string(resultJSON)).
		Msg("Places search result marshaled")

	w.logger.Info().
		Str("step_name", step.Name).
		Int("total_results", result.TotalResults).
		Str("step_id", stepID).
		Msg("Places search orchestration completed successfully")

	// Create individual documents for each place
	docs, err := w.createPlaceDocuments(result, stepID, jobDef.Tags)
	if err != nil {
		return "", fmt.Errorf("failed to create place documents: %w", err)
	}

	// Save each document to database and publish events
	savedCount := 0
	for _, doc := range docs {
		if err := w.documentService.SaveDocument(ctx, doc); err != nil {
			w.logger.Warn().
				Err(err).
				Str("document_id", doc.ID).
				Str("place_name", doc.Title).
				Msg("Failed to save place document")
			continue // Continue with other documents even if one fails
		}

		savedCount++

		w.logger.Debug().
			Str("document_id", doc.ID).
			Str("place_name", doc.Title).
			Msg("Place document saved successfully")

		// Increment document count on manager job for UI display
		if w.jobMgr != nil && managerID != "" {
			if err := w.jobMgr.IncrementDocumentCount(ctx, managerID); err != nil {
				w.logger.Warn().Err(err).Str("manager_id", managerID).Msg("Failed to increment document count")
			}
		}

		// Log document saved via Job Manager's unified logging
		if w.jobMgr != nil && stepID != "" {
			message := fmt.Sprintf("Document saved: %s (ID: %s)", doc.Title, doc.ID[:8])
			w.jobMgr.AddJobLog(context.Background(), stepID, "info", message)
		}
	}

	w.logger.Info().
		Int("documents_created", savedCount).
		Int("total_results", result.TotalResults).
		Msg("Places search results saved as individual documents")

	// Log completion for UI
	w.logJobEvent(ctx, stepID, managerID, step.Name, "info",
		fmt.Sprintf("Places search completed: created %d documents", savedCount),
		map[string]interface{}{
			"documents_created": savedCount,
			"total_results":     result.TotalResults,
		})

	// Return step job ID as placeholder since this is a synchronous operation
	return stepID, nil
}

// ReturnsChildJobs returns false since places search executes synchronously
func (w *PlacesWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration for places search type
func (w *PlacesWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("places_search step requires config")
	}

	// Validate required search_query field
	searchQuery, ok := step.Config["search_query"].(string)
	if !ok || searchQuery == "" {
		return fmt.Errorf("places_search step requires 'search_query' in config")
	}

	// Validate optional search_type field
	if searchType, ok := step.Config["search_type"].(string); ok {
		if searchType != "text_search" && searchType != "nearby_search" {
			return fmt.Errorf("places_search step search_type must be 'text_search' or 'nearby_search', got: %s", searchType)
		}

		// If nearby_search, validate location fields
		if searchType == "nearby_search" {
			hasLocation := false

			// Check for flat location fields (new format)
			if _, ok := step.Config["location_latitude"].(float64); ok {
				hasLocation = true
			}

			// Fallback to nested location map (legacy format)
			if !hasLocation {
				if locationMap, ok := step.Config["location"].(map[string]interface{}); ok {
					if _, ok := locationMap["latitude"].(float64); ok {
						hasLocation = true
					}
				}
			}

			if !hasLocation {
				return fmt.Errorf("places_search step with search_type 'nearby_search' requires location fields (location_latitude/location_longitude or location map)")
			}
		}
	}

	return nil
}

// createPlaceDocuments creates individual documents for each place in the search results
func (w *PlacesWorker) createPlaceDocuments(result *models.PlacesSearchResult, jobID string, tags []string) ([]*models.Document, error) {
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

// logJobEvent logs a job event for real-time UI display using the unified logging system
func (w *PlacesWorker) logJobEvent(ctx context.Context, parentJobID, _, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
