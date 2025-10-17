package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
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
	db           *sql.DB // Database for persisting job settings
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

// NewServiceWithDB creates a new scheduler service with database persistence
func NewServiceWithDB(eventService interfaces.EventService, logger arbor.ILogger, db *sql.DB) interfaces.SchedulerService {
	return &Service{
		eventService: eventService,
		cron:         cron.New(),
		logger:       logger,
		db:           db,
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

	// Only add legacy scheduled task if no default jobs are registered
	// This prevents duplicate collection when using the new job system
	s.jobMu.Lock()
	hasDefaultJobs := len(s.jobs) > 0
	s.jobMu.Unlock()

	if !hasDefaultJobs {
		_, err := s.cron.AddFunc(cronExpr, s.runScheduledTask)
		if err != nil {
			return fmt.Errorf("failed to add cron job: %w", err)
		}
		s.logger.Info().
			Str("cron_expr", cronExpr).
			Msg("Legacy scheduled task enabled (no default jobs registered)")
	} else {
		s.logger.Info().
			Msg("Legacy scheduled task disabled (default jobs are registered)")
	}

	s.cron.Start()
	s.running = true

	s.logger.Info().Msg("Scheduler started")

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
func (s *Service) RegisterJob(name string, schedule string, description string, handler func() error) error {
	// Validate schedule before attempting to register
	if err := common.ValidateJobSchedule(schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("job %s already registered", name)
	}

	// Create job entry
	entry := &jobEntry{
		name:        name,
		schedule:    schedule,
		description: description,
		handler:     handler,
		enabled:     true,
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

	// Persist to database
	if err := s.saveJobSettings(name, entry.schedule, true); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist job enabled status")
	}

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

	// Persist to database
	if err := s.saveJobSettings(name, entry.schedule, false); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist job disabled status")
	}

	return nil
}

// UpdateJobSchedule updates the schedule of an existing job
func (s *Service) UpdateJobSchedule(name string, schedule string) error {
	// Validate schedule before attempting to update
	if err := common.ValidateJobSchedule(schedule); err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	// If job is enabled, remove from cron and re-add with new schedule
	if entry.enabled {
		// Remove old cron entry
		s.cron.Remove(entry.cronID)

		// Add with new schedule
		cronID, err := s.cron.AddFunc(schedule, func() {
			s.executeJob(name)
		})
		if err != nil {
			// Restore old schedule if new one fails
			oldCronID, restoreErr := s.cron.AddFunc(entry.schedule, func() {
				s.executeJob(name)
			})
			if restoreErr != nil {
				s.logger.Error().
					Str("job_name", name).
					Err(restoreErr).
					Msg("Failed to restore old schedule after update failure")
				entry.enabled = false
			} else {
				entry.cronID = oldCronID
			}
			return fmt.Errorf("failed to update job schedule: %w", err)
		}

		entry.cronID = cronID
	}

	// Update schedule in job entry
	entry.schedule = schedule

	// Note: nextRun will be computed on-demand by GetJobStatus()
	// No need to iterate cron.Entries() here to avoid holding jobMu during iteration

	s.logger.Info().
		Str("job_name", name).
		Str("new_schedule", schedule).
		Msg("Job schedule updated")

	// Persist to database
	if err := s.saveJobSettings(name, schedule, entry.enabled); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist job schedule update")
	}

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
	// Copy job names while holding lock to avoid concurrent map iteration
	s.jobMu.Lock()
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	s.jobMu.Unlock()

	// Build statuses without holding lock (GetJobStatus has its own locking)
	statuses := make(map[string]*interfaces.JobStatus)
	for _, name := range names {
		status, err := s.GetJobStatus(name)
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

	// Acquire global mutex to prevent concurrent execution with other jobs
	s.globalMu.Lock()
	defer s.globalMu.Unlock()

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

// saveJobSettings persists job schedule and enabled status to database
func (s *Service) saveJobSettings(name string, schedule string, enabled bool) error {
	if s.db == nil {
		return nil // No database available, skip persistence
	}

	query := `
		INSERT INTO job_settings (job_name, schedule, enabled, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(job_name) DO UPDATE SET
			schedule = excluded.schedule,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`

	_, err := s.db.Exec(query, name, schedule, enabled, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to save job settings: %w", err)
	}

	s.logger.Debug().
		Str("job_name", name).
		Str("schedule", schedule).
		Str("enabled", fmt.Sprintf("%t", enabled)).
		Msg("Job settings persisted to database")

	return nil
}

// LoadJobSettings loads job settings from database and applies them
// Should be called after jobs are registered but before scheduler starts
func (s *Service) LoadJobSettings() error {
	if s.db == nil {
		s.logger.Debug().Msg("No database available, skipping job settings load")
		return nil
	}

	query := `SELECT job_name, schedule, enabled FROM job_settings`
	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to load job settings: %w", err)
	}
	defer rows.Close()

	settingsLoaded := 0
	for rows.Next() {
		var name, schedule string
		var enabled bool

		if err := rows.Scan(&name, &schedule, &enabled); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to scan job setting")
			continue
		}

		// Check if job exists
		s.jobMu.Lock()
		entry, exists := s.jobs[name]
		s.jobMu.Unlock()

		if !exists {
			s.logger.Warn().Str("job_name", name).Msg("Job setting found but job not registered, skipping")
			continue
		}

		// Update schedule if different
		if entry.schedule != schedule {
			if err := s.UpdateJobSchedule(name, schedule); err != nil {
				s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to update job schedule from database")
			} else {
				settingsLoaded++
			}
		}

		// Update enabled status if different
		if entry.enabled != enabled {
			if enabled {
				if err := s.EnableJob(name); err != nil {
					s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to enable job from database")
				}
			} else {
				if err := s.DisableJob(name); err != nil {
					s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to disable job from database")
				}
			}
			settingsLoaded++
		}
	}

	if settingsLoaded > 0 {
		s.logger.Info().Int("count", settingsLoaded).Msg("Loaded job settings from database")
	}

	return nil
}
