package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	// Panic recovery to prevent service crash
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("PANIC RECOVERED in scheduled task")
		}
	}()

	s.logger.Debug().Msg(">>> SCHEDULER: Step 1 - Acquiring mutex lock")
	s.mu.Lock()
	if s.isProcessing {
		s.logger.Debug().Msg(">>> SCHEDULER: Mutex already locked, skipping this cycle")
		s.mu.Unlock()
		return
	}
	s.isProcessing = true
	s.mu.Unlock()
	s.logger.Debug().Msg(">>> SCHEDULER: Step 2 - Mutex acquired, processing started")

	defer func() {
		s.mu.Lock()
		s.isProcessing = false
		s.mu.Unlock()
		s.logger.Debug().Msg(">>> SCHEDULER: Processing flag cleared")
	}()

	s.logger.Info().Msg(">>> SCHEDULER: Starting scheduled collection and embedding cycle")

	ctx := context.Background()
	s.logger.Debug().Msg(">>> SCHEDULER: Step 3 - Context created")

	// Step 1: Publish collection event
	s.logger.Debug().Msg(">>> SCHEDULER: Step 4 - Creating collection event")
	collectionEvent := interfaces.Event{
		Type:    interfaces.EventCollectionTriggered,
		Payload: nil,
	}
	s.logger.Debug().
		Str("event_type", string(collectionEvent.Type)).
		Msg(">>> SCHEDULER: Step 5 - Collection event created")

	s.logger.Debug().Msg(">>> SCHEDULER: Step 6 - Publishing collection event synchronously")
	if err := s.eventService.PublishSync(ctx, collectionEvent); err != nil {
		s.logger.Error().
			Err(err).
			Msg(">>> SCHEDULER: FAILED - Collection event publish error")
		return
	}
	s.logger.Debug().Msg(">>> SCHEDULER: Step 7 - Collection event published successfully")

	s.logger.Info().Msg(">>> SCHEDULER: Collection completed, waiting before embedding")

	// Wait 10 seconds to allow collection writes to complete and reduce database contention
	time.Sleep(10 * time.Second)
	s.logger.Debug().Msg(">>> SCHEDULER: Step 7.5 - Delay complete, starting embedding")

	// Step 3: Publish embedding event
	s.logger.Debug().Msg(">>> SCHEDULER: Step 8 - Creating embedding event")
	embeddingEvent := interfaces.Event{
		Type:    interfaces.EventEmbeddingTriggered,
		Payload: nil,
	}
	s.logger.Debug().
		Str("event_type", string(embeddingEvent.Type)).
		Msg(">>> SCHEDULER: Step 9 - Embedding event created")

	s.logger.Debug().Msg(">>> SCHEDULER: Step 10 - Publishing embedding event synchronously")
	if err := s.eventService.PublishSync(ctx, embeddingEvent); err != nil {
		s.logger.Error().
			Err(err).
			Msg(">>> SCHEDULER: FAILED - Embedding event publish error")
		return
	}
	s.logger.Debug().Msg(">>> SCHEDULER: Step 11 - Embedding event published successfully")

	s.logger.Info().Msg(">>> SCHEDULER: Scheduled cycle completed successfully")
}
