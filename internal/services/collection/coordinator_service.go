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
	jiraService       interfaces.JiraStorage
	confluenceService interfaces.ConfluenceStorage
	documentStorage   interfaces.DocumentStorage
	eventService      interfaces.EventService
	logger            arbor.ILogger
}

// NewCoordinatorService creates a new collection coordinator
func NewCoordinatorService(
	jiraService interfaces.JiraStorage,
	confluenceService interfaces.ConfluenceStorage,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *CoordinatorService {
	return &CoordinatorService{
		jiraService:       jiraService,
		confluenceService: confluenceService,
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
	s.logger.Info().Msg("Collection event triggered")

	pool := workers.NewPool(10, s.logger)
	pool.Start()
	defer pool.Shutdown()

	forceSyncDocs, err := s.documentStorage.GetDocumentsForceSync()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get force sync documents")
	}

	for _, doc := range forceSyncDocs {
		doc := doc
		job := func(ctx context.Context) error {
			return s.syncDocument(ctx, doc)
		}
		if err := pool.Submit(job); err != nil {
			s.logger.Error().Err(err).Str("doc_id", doc.ID).Msg("Failed to submit sync job")
		}
	}

	pool.Wait()

	errors := pool.Errors()
	if len(errors) > 0 {
		s.logger.Warn().Int("error_count", len(errors)).Msg("Some sync jobs failed")
		return fmt.Errorf("collection completed with %d errors", len(errors))
	}

	s.logger.Info().Msg("Collection event completed successfully")
	return nil
}

// syncDocument syncs a single document from its source
func (s *CoordinatorService) syncDocument(ctx context.Context, doc *models.Document) error {
	s.logger.Info().
		Str("doc_id", doc.ID).
		Str("source_type", doc.SourceType).
		Str("source_id", doc.SourceID).
		Msg("Syncing document")

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
