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

// jobEntry represents a registered job with metadata
type jobEntry struct {
	name        string
	schedule    string
	description string
	handler     func() error
	enabled     bool
	cronID      cron.EntryID
	lastRun     *time.Time
	nextRun     *time.Time
	isRunning   bool
	lastError   string
}

// Service implements SchedulerService interface
type Service struct {
	eventService interfaces.EventService
	cron         *cron.Cron
	logger       arbor.ILogger
	mu           sync.Mutex // Protects isProcessing
	jobMu        sync.Mutex // Protects jobs map
	globalMu     sync.Mutex // Prevents concurrent job execution
	jobs         map[string]*jobEntry
	isProcessing bool
	running      bool
}

// NewService creates a new scheduler service
func NewService(eventService interfaces.EventService, logger arbor.ILogger) interfaces.SchedulerService {
	return &Service{
		eventService: eventService,
		cron:         cron.New(),
		logger:       logger,
		jobs:         make(map[string]*jobEntry),
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

// IsRunning returns true if scheduler is active
func (s *Service) IsRunning() bool {
	return s.running
}

// RegisterJob registers a new job with the scheduler
func (s *Service) RegisterJob(name string, schedule string, handler func() error) error {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("job %s already registered", name)
	}

	// Create job entry
	entry := &jobEntry{
		name:     name,
		schedule: schedule,
		handler:  handler,
		enabled:  true,
	}

	// Add to cron scheduler with wrapper
	cronID, err := s.cron.AddFunc(schedule, func() {
		s.executeJob(name)
	})
	if err != nil {
		return fmt.Errorf("failed to add job to cron: %w", err)
	}

	entry.cronID = cronID
	s.jobs[name] = entry

	s.logger.Info().
		Str("job_name", name).
		Str("schedule", schedule).
		Msg("Job registered")

	return nil
}

// EnableJob enables a disabled job
func (s *Service) EnableJob(name string) error {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	if entry.enabled {
		return nil // Already enabled
	}

	// Add back to cron scheduler
	cronID, err := s.cron.AddFunc(entry.schedule, func() {
		s.executeJob(name)
	})
	if err != nil {
		return fmt.Errorf("failed to add job to cron: %w", err)
	}

	entry.cronID = cronID
	entry.enabled = true

	s.logger.Info().
		Str("job_name", name).
		Msg("Job enabled")

	return nil
}

// DisableJob disables an enabled job
func (s *Service) DisableJob(name string) error {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	if !entry.enabled {
		return nil // Already disabled
	}

	// Remove from cron scheduler
	s.cron.Remove(entry.cronID)
	entry.enabled = false

	s.logger.Info().
		Str("job_name", name).
		Msg("Job disabled")

	return nil
}

// GetJobStatus returns the status of a specific job
func (s *Service) GetJobStatus(name string) (*interfaces.JobStatus, error) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return nil, fmt.Errorf("job %s not found", name)
	}

	// Get next run time from cron
	var nextRun *time.Time
	if entry.enabled {
		for _, cronEntry := range s.cron.Entries() {
			if cronEntry.ID == entry.cronID {
				next := cronEntry.Next
				nextRun = &next
				break
			}
		}
	}

	return &interfaces.JobStatus{
		Name:        entry.name,
		Enabled:     entry.enabled,
		Schedule:    entry.schedule,
		Description: entry.description,
		LastRun:     entry.lastRun,
		NextRun:     nextRun,
		IsRunning:   entry.isRunning,
		LastError:   entry.lastError,
	}, nil
}

// GetAllJobStatuses returns all job statuses
func (s *Service) GetAllJobStatuses() map[string]*interfaces.JobStatus {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	statuses := make(map[string]*interfaces.JobStatus)
	for name := range s.jobs {
		// Unlock temporarily to call GetJobStatus (which also locks)
		s.jobMu.Unlock()
		status, err := s.GetJobStatus(name)
		s.jobMu.Lock()

		if err == nil {
			statuses[name] = status
		}
	}

	return statuses
}

// executeJob wraps job execution with mutex, panic recovery, and status tracking
func (s *Service) executeJob(name string) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("job_name", name).
				Str("panic", fmt.Sprintf("%v", r)).
				Msg("PANIC RECOVERED in job execution")

			s.jobMu.Lock()
			if entry, exists := s.jobs[name]; exists {
				entry.isRunning = false
				entry.lastError = fmt.Sprintf("panic: %v", r)
			}
			s.jobMu.Unlock()
		}
	}()

	// Acquire global mutex to prevent concurrent execution
	s.globalMu.Lock()
	defer s.globalMu.Unlock()

	s.logger.Debug().
		Str("job_name", name).
		Msg("Job execution started")

	// Get job handler
	s.jobMu.Lock()
	entry, exists := s.jobs[name]
	if !exists {
		s.jobMu.Unlock()
		s.logger.Warn().
			Str("job_name", name).
			Msg("Job not found")
		return
	}

	// Update status
	entry.isRunning = true
	now := time.Now()
	entry.lastRun = &now
	handler := entry.handler
	s.jobMu.Unlock()

	// Execute job handler
	err := handler()

	// Update status after execution
	s.jobMu.Lock()
	entry.isRunning = false
	if err != nil {
		entry.lastError = err.Error()
		s.logger.Error().
			Str("job_name", name).
			Err(err).
			Msg("Job execution failed")
	} else {
		entry.lastError = ""
		s.logger.Info().
			Str("job_name", name).
			Msg("Job execution completed successfully")
	}
	s.jobMu.Unlock()
}

// runScheduledTask executes the scheduled collection and embedding pipeline (legacy)
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

	s.logger.Info().Msg(">>> SCHEDULER: Collection completed successfully")
}
