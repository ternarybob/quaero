package processing

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ternarybob/arbor"
)

// Scheduler handles periodic document processing
type Scheduler struct {
	service *Service
	cron    *cron.Cron
	logger  arbor.ILogger
}

// NewScheduler creates a new processing scheduler
func NewScheduler(service *Service, logger arbor.ILogger) *Scheduler {
	return &Scheduler{
		service: service,
		cron:    cron.New(cron.WithSeconds()),
		logger:  logger,
	}
}

// Start begins the scheduled processing
func (s *Scheduler) Start(schedule string) error {
	if schedule == "" {
		// Default: every 6 hours
		schedule = "0 0 */6 * * *"
	}

	_, err := s.cron.AddFunc(schedule, func() {
		s.runProcessing()
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	s.logger.Info().
		Str("schedule", schedule).
		Msg("Document processing scheduler started")

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cron.Stop()
	s.logger.Info().Msg("Document processing scheduler stopped")
}

// RunNow triggers an immediate processing run
func (s *Scheduler) RunNow() {
	s.logger.Info().Msg("Triggering immediate processing run")
	go s.runProcessing()
}

func (s *Scheduler) runProcessing() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.logger.Info().Msg("Starting scheduled processing")

	stats, err := s.service.ProcessAll(ctx)
	if err != nil {
		s.logger.Error().
			Err(err).
			Msg("Scheduled processing failed")
		return
	}

	s.logger.Info().
		Int("processed", stats.TotalProcessed).
		Int("new", stats.NewDocuments).
		Int("updated", stats.UpdatedDocuments).
		Int("errors", stats.TotalErrors).
		Dur("duration", stats.Duration).
		Msg("Scheduled processing completed")
}
