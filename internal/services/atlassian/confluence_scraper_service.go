// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:12:24 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package atlassian

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// ConfluenceScraperService scrapes Confluence spaces and pages
type ConfluenceScraperService struct {
	authService       interfaces.AtlassianAuthService
	confluenceStorage interfaces.ConfluenceStorage
	documentService   interfaces.DocumentService
	eventService      interfaces.EventService
	crawlerService    *crawler.Service
	logger            arbor.ILogger
	uiLogger          interface{}
}

// NewConfluenceScraperService creates a new Confluence scraper service
func NewConfluenceScraperService(
	confluenceStorage interfaces.ConfluenceStorage,
	documentService interfaces.DocumentService,
	authService interfaces.AtlassianAuthService,
	eventService interfaces.EventService,
	crawlerService *crawler.Service,
	logger arbor.ILogger,
) *ConfluenceScraperService {
	service := &ConfluenceScraperService{
		confluenceStorage: confluenceStorage,
		documentService:   documentService,
		authService:       authService,
		eventService:      eventService,
		crawlerService:    crawlerService,
		logger:            logger,
	}

	// Subscribe to collection events
	if eventService != nil {
		handler := func(ctx context.Context, event interfaces.Event) error {
			return service.handleCollectionEvent(ctx, event)
		}
		if err := eventService.Subscribe(interfaces.EventCollectionTriggered, handler); err != nil {
			logger.Error().Err(err).Msg("Failed to subscribe Confluence service to collection events")
		}
	}

	return service
}

// Close closes the scraper and releases resources
func (s *ConfluenceScraperService) Close() error {
	return nil
}

// SetUILogger sets a UI logger for real-time updates
func (s *ConfluenceScraperService) SetUILogger(logger interface{}) {
	s.uiLogger = logger
}

// ScrapeConfluence is an alias for ScrapeSpaces for interface compatibility
func (s *ConfluenceScraperService) ScrapeConfluence() error {
	// Call the actual implementation in confluence_spaces.go
	return s.ScrapeSpaces()
}

// makeRequest makes an authenticated HTTP request to the Confluence API
func (s *ConfluenceScraperService) makeRequest(method, path string) ([]byte, error) {
	if !s.authService.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	baseURL := s.authService.GetBaseURL()
	fullURL := baseURL + path

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Use auth service's HTTP client which has cookies configured
	client := s.authService.GetHTTPClient()
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// transformToDocument converts Confluence page to normalized document
func (s *ConfluenceScraperService) transformToDocument(page *models.ConfluencePage) (*models.Document, error) {
	docID := fmt.Sprintf("doc_%s", uuid.New().String())

	// Extract body content
	bodyContent := ""
	if storage, ok := page.Body["storage"].(map[string]interface{}); ok {
		if value, ok := storage["value"].(string); ok {
			bodyContent = value
		}
	}

	// Build plain text content (strip HTML tags for basic text)
	content := fmt.Sprintf("Page: %s\n\nTitle: %s\n\nContent:\n%s", page.ID, page.Title, bodyContent)

	// Build markdown content
	contentMD := fmt.Sprintf("# %s\n\n%s", page.Title, bodyContent)

	// Build metadata
	// NOTE: ConfluencePage model currently has limited fields (ID, Title, SpaceID, Body)
	// TODO: Enhance Confluence scraper to capture version, author, dates from API
	metadata := models.ConfluenceMetadata{
		PageID:       page.ID,
		PageTitle:    page.Title,
		SpaceKey:     page.SpaceID,
		SpaceName:    "", // TODO: Extract from Confluence API
		Author:       "", // TODO: Extract from Confluence API
		Version:      0,  // TODO: Extract from Confluence API
		ContentType:  "page",
		LastModified: nil, // TODO: Extract from Confluence API
		CreatedDate:  nil, // TODO: Extract from Confluence API
	}

	metadataMap, err := metadata.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata: %w", err)
	}

	// Use current time for timestamps (Confluence pages don't have these in the model yet)
	now := time.Now()

	return &models.Document{
		ID:              docID,
		SourceType:      "confluence",
		SourceID:        page.ID,
		Title:           page.Title,
		Content:         content,
		ContentMarkdown: contentMD,
		Metadata:        metadataMap,
		URL:             fmt.Sprintf("%s/wiki/spaces/%s/pages/%s", s.authService.GetBaseURL(), page.SpaceID, page.ID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// handleCollectionEvent processes collection triggered events
// NOTE: This does NOT scrape/download data - scraping is user-driven
// This event triggers processing of already-scraped data (pages → documents)
func (s *ConfluenceScraperService) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	s.logger.Info().Msg(">>> CONFLUENCE SERVICE: Collection push event received")

	// Run processing synchronously (not in goroutine) to prevent overlap with embedding
	s.logger.Debug().Msg(">>> CONFLUENCE SERVICE: Starting collection push (pages → documents)")

	// Get all spaces from storage
	spaces, err := s.confluenceStorage.GetAllSpaces(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg(">>> CONFLUENCE SERVICE: Failed to get spaces")
		return err
	}

	if len(spaces) == 0 {
		s.logger.Info().Msg(">>> CONFLUENCE SERVICE: No spaces found - nothing to process")
		return nil
	}

	// Process pages for each space
	totalPages := 0
	totalDocuments := 0
	for _, space := range spaces {
		s.logger.Debug().
			Str("space", space.Key).
			Msg(">>> CONFLUENCE SERVICE: Processing space pages")

		err := s.ProcessPagesForSpace(ctx, space.Key)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("space", space.Key).
				Msg(">>> CONFLUENCE SERVICE: Failed to process space")
			continue
		}

		// Get count for logging
		count, _ := s.confluenceStorage.CountPagesBySpace(ctx, space.Key)
		totalPages += count
		totalDocuments += count
	}

	s.logger.Info().
		Int("spaces", len(spaces)).
		Int("pages", totalPages).
		Int("documents", totalDocuments).
		Msg(">>> CONFLUENCE SERVICE: Collection push completed successfully")

	return nil
}

// ProcessPagesForSpace transforms and saves Confluence pages as documents
func (s *ConfluenceScraperService) ProcessPagesForSpace(ctx context.Context, spaceKey string) error {
	// Get pages from storage
	pages, err := s.confluenceStorage.GetPagesBySpace(ctx, spaceKey)
	if err != nil {
		return fmt.Errorf("failed to get pages: %w", err)
	}

	if len(pages) == 0 {
		s.logger.Info().Str("space", spaceKey).Msg("No pages to process")
		return nil
	}

	// Transform to documents
	documents := make([]*models.Document, 0, len(pages))
	for _, page := range pages {
		doc, err := s.transformToDocument(page)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("page_id", page.ID).
				Msg("Failed to transform page")
			continue
		}
		documents = append(documents, doc)
	}

	// Save documents (embedding handled independently by coordinator)
	if err := s.documentService.SaveDocuments(ctx, documents); err != nil {
		return fmt.Errorf("failed to save documents: %w", err)
	}

	s.logger.Info().
		Str("space", spaceKey).
		Int("pages", len(pages)).
		Int("documents", len(documents)).
		Msg("Processed Confluence pages to documents")

	return nil
}

// GetSpaceStatus returns the last updated time and details for Confluence spaces
func (s *ConfluenceScraperService) GetSpaceStatus() (lastUpdated int64, details string, err error) {
	ctx := context.Background()
	space, timestamp, err := s.confluenceStorage.GetMostRecentSpace(ctx)
	if err != nil {
		// No spaces found or error
		return 0, "No spaces found", nil
	}

	details = fmt.Sprintf("Space %s (%s) was scanned and added to the database", space.Key, space.Name)
	return timestamp, details, nil
}

// GetPageStatus returns the last updated time and details for Confluence pages
func (s *ConfluenceScraperService) GetPageStatus() (lastUpdated int64, details string, err error) {
	ctx := context.Background()
	page, timestamp, err := s.confluenceStorage.GetMostRecentPage(ctx)
	if err != nil {
		// No pages found or error
		return 0, "No pages found", nil
	}

	details = fmt.Sprintf("Page '%s' was scanned and added to the database", page.Title)
	return timestamp, details, nil
}
