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
)

// ConfluenceScraperService scrapes Confluence spaces and pages
type ConfluenceScraperService struct {
	authService       interfaces.AtlassianAuthService
	confluenceStorage interfaces.ConfluenceStorage
	documentService   interfaces.DocumentService
	eventService      interfaces.EventService
	logger            arbor.ILogger
	uiLogger          interface{}
}

// NewConfluenceScraperService creates a new Confluence scraper service
func NewConfluenceScraperService(
	confluenceStorage interfaces.ConfluenceStorage,
	documentService interfaces.DocumentService,
	authService interfaces.AtlassianAuthService,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *ConfluenceScraperService {
	service := &ConfluenceScraperService{
		confluenceStorage: confluenceStorage,
		documentService:   documentService,
		authService:       authService,
		eventService:      eventService,
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

// ScrapeConfluence is an alias for ScrapeSpaces for compatibility
func (s *ConfluenceScraperService) ScrapeConfluence() error {
	return s.ScrapeSpaces()
}

func (s *ConfluenceScraperService) makeRequest(method, path string) ([]byte, error) {
	if !s.authService.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated: please authenticate using Chrome extension")
	}

	reqURL := s.authService.GetBaseURL() + path

	req, err := http.NewRequest(method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.authService.GetUserAgent())
	req.Header.Set("Accept", "application/json, text/html")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	httpClient := s.authService.GetHTTPClient()
	if httpClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized: authentication required")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		s.logger.Error().
			Str("url", reqURL).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("HTTP request failed")

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("auth expired (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, readErr
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
	metadata := models.ConfluenceMetadata{
		PageID:      page.ID,
		SpaceKey:    page.SpaceID,
		SpaceName:   "",
		Author:      "",
		Version:     0,
		ContentType: "page",
	}

	metadataMap, err := metadata.ToMap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert metadata: %w", err)
	}

	// Use current time for timestamps (Confluence pages don't have these in the model)
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
func (s *ConfluenceScraperService) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	s.logger.Info().Msg(">>> CONFLUENCE SERVICE: Collection event received")

	// Run scraping in goroutine for async execution
	go func() {
		s.logger.Debug().Msg(">>> CONFLUENCE SERVICE: Starting data scrape")

		err := s.ScrapeSpaces()
		if err != nil {
			s.logger.Error().Err(err).Msg(">>> CONFLUENCE SERVICE: Failed to scrape spaces")
			return
		}

		s.logger.Info().Msg(">>> CONFLUENCE SERVICE: Data scrape completed successfully")
	}()

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

	// Save via DocumentService (which handles embedding)
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
