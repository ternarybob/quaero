package places

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service implements the PlacesService interface
type Service struct {
	config       *common.PlacesAPIConfig
	eventService interfaces.EventService
	logger       arbor.ILogger
	apiKey       string
	httpClient   *http.Client
	lastRequest  time.Time // For rate limiting
}

// NewService creates a new Places service instance
func NewService(
	config *common.PlacesAPIConfig,
	storageManager interfaces.StorageManager,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) interfaces.PlacesService {
	// Resolve API key with KV-first resolution order: KV store → config fallback
	ctx := context.Background()
	apiKey, err := common.ResolveAPIKey(ctx, storageManager.KeyValueStorage(), "google-places", config.APIKey)
	if err != nil {
		// If resolution fails, fall back to config value (for backward compatibility)
		apiKey = config.APIKey
		logger.Warn().Err(err).Msg("Failed to resolve API key from KV store, using config value")
	}

	return &Service{
		config:       config,
		eventService: eventService,
		logger:       logger,
		apiKey:       apiKey,
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		lastRequest: time.Time{},
	}
}

// SearchPlaces performs a Google Places API search and returns results
func (s *Service) SearchPlaces(ctx context.Context, jobID string, req *models.PlacesSearchRequest) (*models.PlacesSearchResult, error) {
	s.logger.Info().
		Str("job_id", jobID).
		Str("search_query", req.SearchQuery).
		Str("search_type", req.SearchType).
		Msg("Starting place search")

	// Publish started event
	s.publishEvent("places_search_started", map[string]interface{}{
		"job_id":       jobID,
		"search_query": req.SearchQuery,
		"search_type":  req.SearchType,
	})

	// Call Google Places API based on search type
	var places []PlaceResult
	var err error

	switch req.SearchType {
	case "text_search":
		places, err = s.textSearch(ctx, req)
	case "nearby_search":
		if req.Location == nil {
			return nil, fmt.Errorf("location is required for nearby_search")
		}
		places, err = s.nearbySearch(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported search_type: %s", req.SearchType)
	}

	if err != nil {
		s.publishEvent("places_search_failed", map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		})
		return nil, fmt.Errorf("failed to search places: %w", err)
	}

	// Convert to PlaceItem models
	items := make([]models.PlaceItem, len(places))
	for i, place := range places {
		items[i] = s.convertToPlaceItem(place)
	}

	result := &models.PlacesSearchResult{
		SearchQuery:  req.SearchQuery,
		SearchType:   req.SearchType,
		TotalResults: len(items),
		Places:       items,
	}

	// Publish completed event
	s.publishEvent("places_search_completed", map[string]interface{}{
		"job_id":        jobID,
		"total_results": result.TotalResults,
		"search_query":  req.SearchQuery,
	})

	s.logger.Info().
		Int("total_results", result.TotalResults).
		Msg("Place search completed")

	return result, nil
}

// textSearch performs a Google Places Text Search
func (s *Service) textSearch(ctx context.Context, req *models.PlacesSearchRequest) ([]PlaceResult, error) {
	// Rate limiting
	if err := s.waitForRateLimit(); err != nil {
		return nil, err
	}

	maxResults := req.MaxResults
	if maxResults == 0 || maxResults > s.config.MaxResultsPerSearch {
		maxResults = s.config.MaxResultsPerSearch
	}

	apiURL := "https://maps.googleapis.com/maps/api/place/textsearch/json"
	params := url.Values{}
	params.Set("query", req.SearchQuery)
	params.Set("key", s.apiKey)

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// Redact API key in logs
	logURL := fmt.Sprintf("%s?query=%s&key=***REDACTED***", apiURL, url.QueryEscape(req.SearchQuery))
	s.logger.Debug().Str("url", logURL).Msg("Calling Google Places Text Search API")

	resp, err := s.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google Places API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google Places API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp PlacesTextSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if apiResp.Status != "OK" && apiResp.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("API error: %s - %s", apiResp.Status, apiResp.ErrorMessage)
	}

	// Limit results
	if len(apiResp.Results) > maxResults {
		apiResp.Results = apiResp.Results[:maxResults]
	}

	// Log sample place names for debugging search relevance
	samplePlaces := []string{}
	for i, place := range apiResp.Results {
		if i < 3 { // Log first 3 places
			samplePlaces = append(samplePlaces, place.Name)
		}
	}

	s.logger.Info().
		Str("search_query", req.SearchQuery).
		Int("results_count", len(apiResp.Results)).
		Str("status", apiResp.Status).
		Strs("sample_places", samplePlaces).
		Msg("Google Places Text Search completed - verify relevance")

	return apiResp.Results, nil
}

// nearbySearch performs a Google Places Nearby Search
func (s *Service) nearbySearch(ctx context.Context, req *models.PlacesSearchRequest) ([]PlaceResult, error) {
	// Rate limiting
	if err := s.waitForRateLimit(); err != nil {
		return nil, err
	}

	maxResults := req.MaxResults
	if maxResults == 0 || maxResults > s.config.MaxResultsPerSearch {
		maxResults = s.config.MaxResultsPerSearch
	}

	apiURL := "https://maps.googleapis.com/maps/api/place/nearbysearch/json"
	params := url.Values{}
	params.Set("location", fmt.Sprintf("%f,%f", req.Location.Latitude, req.Location.Longitude))
	if req.Location.Radius > 0 {
		params.Set("radius", fmt.Sprintf("%d", req.Location.Radius))
	} else {
		params.Set("radius", "5000") // Default 5km radius
	}

	// Add type filter if specified (critical for filtering results)
	if req.Filters != nil {
		if placeType, ok := req.Filters["type"].(string); ok && placeType != "" {
			params.Set("type", placeType)
			s.logger.Debug().Str("type_filter", placeType).Msg("Applied type filter to nearby search")
		}
	}

	params.Set("key", s.apiKey)

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// Redact API key in logs
	logURL := fmt.Sprintf("%s?location=%f,%f&radius=%d", apiURL, req.Location.Latitude, req.Location.Longitude, req.Location.Radius)
	if req.Filters != nil {
		if placeType, ok := req.Filters["type"].(string); ok && placeType != "" {
			logURL += fmt.Sprintf("&type=%s", placeType)
		}
	}
	logURL += "&key=***REDACTED***"
	s.logger.Debug().Str("url", logURL).Msg("Calling Google Places Nearby Search API")

	resp, err := s.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google Places API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Google Places API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp PlacesNearbySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}

	if apiResp.Status != "OK" && apiResp.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("API error: %s - %s", apiResp.Status, apiResp.ErrorMessage)
	}

	// Limit results
	if len(apiResp.Results) > maxResults {
		apiResp.Results = apiResp.Results[:maxResults]
	}

	// Log sample place names for debugging search relevance
	samplePlaces := []string{}
	for i, place := range apiResp.Results {
		if i < 3 { // Log first 3 places
			samplePlaces = append(samplePlaces, place.Name)
		}
	}

	logEvent := s.logger.Info().
		Str("search_query", req.SearchQuery).
		Float64("latitude", req.Location.Latitude).
		Float64("longitude", req.Location.Longitude).
		Int("radius", req.Location.Radius).
		Int("results_count", len(apiResp.Results)).
		Str("status", apiResp.Status).
		Strs("sample_places", samplePlaces)

	// Add type filter to log if specified
	if req.Filters != nil {
		if placeType, ok := req.Filters["type"].(string); ok && placeType != "" {
			logEvent = logEvent.Str("type_filter", placeType)
		}
	}

	logEvent.Msg("Google Places Nearby Search completed - verify relevance")

	return apiResp.Results, nil
}

// waitForRateLimit enforces rate limiting between API requests
func (s *Service) waitForRateLimit() error {
	if !s.lastRequest.IsZero() {
		elapsed := time.Since(s.lastRequest)
		if elapsed < s.config.RateLimit {
			waitTime := s.config.RateLimit - elapsed
			s.logger.Debug().
				Dur("wait_time", waitTime).
				Msg("Rate limiting: waiting before next API call")
			time.Sleep(waitTime)
		}
	}
	s.lastRequest = time.Now()
	return nil
}

// convertToPlaceItem converts a Google Places API result to a PlaceItem model
func (s *Service) convertToPlaceItem(place PlaceResult) models.PlaceItem {
	item := models.PlaceItem{
		PlaceID:          place.PlaceID,
		Name:             place.Name,
		FormattedAddress: place.FormattedAddress,
		Rating:           place.Rating,
		UserRatingsTotal: place.UserRatingsTotal,
		PriceLevel:       place.PriceLevel,
		Types:            place.Types,
	}

	// Geometry/Location
	if place.Geometry != nil && place.Geometry.Location != nil {
		item.Latitude = place.Geometry.Location.Lat
		item.Longitude = place.Geometry.Location.Lng
	}

	// Opening hours
	if place.OpeningHours != nil {
		item.OpeningHours = place.OpeningHours
	}

	// Photos - convert []Photo to []interface{}
	if len(place.Photos) > 0 {
		item.Photos = make([]interface{}, len(place.Photos))
		for i, photo := range place.Photos {
			item.Photos[i] = photo
		}
	}

	return item
}

// publishEvent publishes an event via the event service
func (s *Service) publishEvent(eventType string, data map[string]interface{}) {
	if s.eventService == nil {
		return
	}

	data["timestamp"] = time.Now().Format(time.RFC3339)
	event := interfaces.Event{
		Type:    interfaces.EventType(eventType),
		Payload: data,
	}
	if err := s.eventService.Publish(context.Background(), event); err != nil {
		s.logger.Warn().
			Err(err).
			Str("event_type", eventType).
			Msg("Failed to publish event")
	}
}
