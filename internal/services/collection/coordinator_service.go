package collection

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/workers"
)

// CoordinatorService coordinates collection from multiple sources
type CoordinatorService struct {
	jiraScraper       interfaces.JiraScraperService
	confluenceScraper interfaces.ConfluenceScraperService
	documentStorage   interfaces.DocumentStorage
	eventService      interfaces.EventService
	logger            arbor.ILogger
}

// NewCoordinatorService creates a new collection coordinator
func NewCoordinatorService(
	jiraScraper interfaces.JiraScraperService,
	confluenceScraper interfaces.ConfluenceScraperService,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CoordinatorService {
	return &CoordinatorService{
		jiraScraper:       jiraScraper,
		confluenceScraper: confluenceScraper,
		documentStorage:   documentStorage,
		eventService:      eventService,
		logger:            logger,
	}
}

// Start subscribes to collection events
func (s *CoordinatorService) Start() error {
	handler := func(ctx context.Context, event interfaces.Event) error {
		return s.handleCollectionEvent(ctx, event)
	}

	return s.eventService.Subscribe(interfaces.EventCollectionTriggered, handler)
}

// handleCollectionEvent processes collection triggered events
func (s *CoordinatorService) handleCollectionEvent(ctx context.Context, event interfaces.Event) error {
	// Panic recovery to prevent service crash
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("PANIC RECOVERED in collection event handler")
		}
	}()

	s.logger.Info().Msg("=== COLLECTION COORDINATOR: Handling force sync documents only")

	s.logger.Debug().Msg("Validating dependencies")
	if s.documentStorage == nil {
		s.logger.Error().Msg("documentStorage is nil")
		return fmt.Errorf("document storage is nil")
	}
	if s.jiraScraper == nil {
		s.logger.Error().Msg("jiraScraper is nil")
		return fmt.Errorf("jira scraper is nil")
	}
	if s.confluenceScraper == nil {
		s.logger.Error().Msg("confluenceScraper is nil")
		return fmt.Errorf("confluence scraper is nil")
	}

	s.logger.Debug().Msg("Fetching force sync documents from storage")
	forceSyncDocs, err := s.documentStorage.GetDocumentsForceSync()
	if err != nil {
		s.logger.Error().Err(err).Msg("Error querying force sync documents")
		return fmt.Errorf("failed to get force sync documents: %w", err)
	}

	if len(forceSyncDocs) == 0 {
		s.logger.Info().Msg("No force sync documents found")
		return nil
	}

	s.logger.Info().
		Int("count", len(forceSyncDocs)).
		Msg("Processing force sync documents")

	// Create worker pool for parallel processing
	pool := workers.NewPool(10, s.logger)
	pool.Start()
	defer func() {
		pool.Shutdown()
		s.logger.Debug().Msg("Worker pool shutdown complete")
	}()

	// Submit force sync jobs
	for i, doc := range forceSyncDocs {
		doc := doc
		s.logger.Debug().
			Int("job_index", i).
			Str("doc_id", doc.ID).
			Str("source_type", doc.SourceType).
			Str("source_id", doc.SourceID).
			Msg("Submitting sync job")

		job := func(ctx context.Context) error {
			return s.syncDocument(ctx, doc)
		}
		if err := pool.Submit(job); err != nil {
			s.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Int("job_index", i).
				Msg("Error submitting sync job to pool")
		}
	}

	pool.Wait()
	errors := pool.Errors()

	if len(errors) > 0 {
		s.logger.Warn().
			Int("error_count", len(errors)).
			Msg("Some sync jobs failed")
		for i, err := range errors {
			s.logger.Error().
				Int("error_index", i).
				Err(err).
				Msg("Job error detail")
		}
		return fmt.Errorf("collection completed with %d errors", len(errors))
	}

	s.logger.Info().Msg("=== COLLECTION COORDINATOR: Force sync completed successfully")
	return nil
}

// syncDocument syncs a single document from its source
func (s *CoordinatorService) syncDocument(ctx context.Context, doc *models.Document) error {
	s.logger.Info().
		Str("doc_id", doc.ID).
		Str("source_type", doc.SourceType).
		Str("source_id", doc.SourceID).
		Msg("Syncing document")

	var err error
	switch doc.SourceType {
	case "jira":
		err = s.syncJiraDocument(ctx, doc)
	case "confluence":
		err = s.syncConfluenceDocument(ctx, doc)
	default:
		return fmt.Errorf("unknown source type: %s", doc.SourceType)
	}

	if err != nil {
		return fmt.Errorf("failed to sync %s document: %w", doc.SourceType, err)
	}

	now := time.Now()
	doc.LastSynced = &now
	doc.ForceSyncPending = false

	if err := s.documentStorage.UpdateDocument(doc); err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	s.logger.Info().
		Str("doc_id", doc.ID).
		Msg("Document synced successfully")

	return nil
}

// syncJiraDocument syncs a Jira document by scraping its project issues
func (s *CoordinatorService) syncJiraDocument(ctx context.Context, doc *models.Document) error {
	projectKey := doc.SourceID
	s.logger.Info().Str("project", projectKey).Msg("Syncing Jira project issues")

	if err := s.jiraScraper.GetProjectIssues(projectKey); err != nil {
		return fmt.Errorf("failed to scrape Jira project %s: %w", projectKey, err)
	}

	return nil
}

// syncConfluenceDocument syncs a Confluence document by scraping its space pages
func (s *CoordinatorService) syncConfluenceDocument(ctx context.Context, doc *models.Document) error {
	spaceKey := doc.SourceID
	s.logger.Info().Str("space", spaceKey).Msg("Syncing Confluence space pages")

	if err := s.confluenceScraper.GetSpacePages(spaceKey); err != nil {
		return fmt.Errorf("failed to scrape Confluence space %s: %w", spaceKey, err)
	}

	return nil
}
