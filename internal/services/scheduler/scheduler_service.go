package scheduler

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service implements SchedulerService interface
type Service struct {
	eventService interfaces.EventService
	cron         *cron.Cron
	logger       arbor.ILogger
	mu           sync.Mutex
	isProcessing bool
	running      bool
}

// NewService creates a new scheduler service
func NewService(eventService interfaces.EventService, logger arbor.ILogger) interfaces.SchedulerService {
	return &Service{
		eventService: eventService,
		cron:         cron.New(),
		logger:       logger,
	}
}

// Start begins the scheduler with the given cron expression
func (s *Service) Start(cronExpr string) error {
	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	if cronExpr == "" {
		cronExpr = "*/1 * * * *" // Default: every 1 minute
	}

	_, err := s.cron.AddFunc(cronExpr, s.runScheduledTask)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.cron.Start()
	s.running = true

	s.logger.Info().
		Str("cron_expr", cronExpr).
		Msg("Scheduler started")

	return nil
}

// Stop halts the scheduler
func (s *Service) Stop() error {
	if !s.running {
		return nil
	}

	s.cron.Stop()
	s.running = false

	s.logger.Info().Msg("Scheduler stopped")
	return nil
}

// TriggerCollectionNow manually triggers collection
func (s *Service) TriggerCollectionNow() error {
	s.logger.Info().Msg("Manual collection trigger requested")

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: nil,
	}

	return s.eventService.PublishSync(ctx, event)
}

// TriggerEmbeddingNow manually triggers embedding
func (s *Service) TriggerEmbeddingNow() error {
	s.logger.Info().Msg("Manual embedding trigger requested")

	ctx := context.Background()
	event := interfaces.Event{
		Type:    interfaces.EventEmbeddingTriggered,
		Payload: nil,
	}

	return s.eventService.PublishSync(ctx, event)
}

// IsRunning returns true if scheduler is active
func (s *Service) IsRunning() bool {
	return s.running
}

// runScheduledTask executes the scheduled collection and embedding pipeline
func (s *Service) runScheduledTask() {
	s.mu.Lock()
	if s.isProcessing {
		s.logger.Debug().Msg("Previous task still running, skipping this cycle")
		s.mu.Unlock()
		return
	}
	s.isProcessing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isProcessing = false
		s.mu.Unlock()
	}()

	s.logger.Info().Msg("Starting scheduled collection and embedding cycle")

	ctx := context.Background()

	collectionEvent := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: nil,
	}

	if err := s.eventService.PublishSync(ctx, collectionEvent); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Collection event failed")
		return
	}

	s.logger.Info().Msg("Collection completed, starting embedding")

	embeddingEvent := interfaces.Event{
		Type:    interfaces.EventEmbeddingTriggered,
		Payload: nil,
	}

	if err := s.eventService.PublishSync(ctx, embeddingEvent); err != nil {
		s.logger.Error().
			Err(err).
			Msg("Embedding event failed")
		return
	}

	s.logger.Info().Msg("Scheduled cycle completed successfully")
}
