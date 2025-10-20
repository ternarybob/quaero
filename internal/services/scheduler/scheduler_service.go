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
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// jobEntry represents a registered job with metadata
type jobEntry struct {
	name        string
	schedule    string
	description string
	handler     func() error
	enabled     bool
	autoStart   bool
	cronID      cron.EntryID
	lastRun     *time.Time
	nextRun     *time.Time
	isRunning   bool
	lastError   string
}

// Service implements SchedulerService interface
type Service struct {
	eventService   interfaces.EventService
	crawlerService *crawler.Service      // For shutdown coordination
	jobStorage     interfaces.JobStorage // For stale job detection
	cron           *cron.Cron
	logger         arbor.ILogger
	db             *sql.DB    // Database for persisting job settings
	mu             sync.Mutex // Protects isProcessing
	jobMu          sync.Mutex // Protects jobs map
	globalMu       sync.Mutex // Prevents concurrent job execution
	jobs           map[string]*jobEntry
	isProcessing   bool
	running        bool
	staleJobTicker *time.Ticker // For stale job detection cleanup goroutine
	jobDefStorage  interfaces.JobDefinitionStorage
	jobExecutor    *jobs.JobExecutor
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
func NewServiceWithDB(eventService interfaces.EventService, logger arbor.ILogger, db *sql.DB, crawlerService *crawler.Service, jobStorage interfaces.JobStorage, jobDefStorage interfaces.JobDefinitionStorage, jobExecutor *jobs.JobExecutor) interfaces.SchedulerService {
	return &Service{
		eventService:   eventService,
		crawlerService: crawlerService,
		jobStorage:     jobStorage,
		jobDefStorage:  jobDefStorage,
		jobExecutor:    jobExecutor,
		cron:           cron.New(),
		logger:         logger,
		db:             db,
		jobs:           make(map[string]*jobEntry),
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

	// Load job definitions from storage and register them
	if err := s.LoadJobDefinitions(); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to load job definitions from storage")
		// Non-critical error - continue starting scheduler
	}

	s.cron.Start()
	s.running = true

	// Launch stale job detector goroutine (runs every 5 minutes)
	if s.jobStorage != nil {
		s.staleJobTicker = time.NewTicker(5 * time.Minute)
		go s.staleJobDetectorLoop()
		s.logger.Info().Msg("Stale job detector started (5 minute interval)")
	}

	s.logger.Info().Msg("Scheduler started")

	// Execute auto-start jobs in background
	go s.executeAutoStartJobs()

	return nil
}

// Stop halts the scheduler
func (s *Service) Stop() error {
	if !s.running {
		return nil
	}

	// Cancel running crawler jobs before stopping scheduler
	if s.crawlerService != nil {
		runningJobIDs := s.crawlerService.GetRunningJobIDs()
		if len(runningJobIDs) > 0 {
			s.logger.Info().Int("count", len(runningJobIDs)).Msg("Cancelling running crawler jobs")

			// Cancel each job
			for _, jobID := range runningJobIDs {
				if err := s.crawlerService.CancelJob(jobID); err != nil {
					s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to cancel job")
				}
			}

			// Wait up to 30 seconds for jobs to cancel
			timeout := time.After(30 * time.Second)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeout:
					remaining := s.crawlerService.GetRunningJobIDs()
					if len(remaining) > 0 {
						s.logger.Warn().Int("count", len(remaining)).Msg("Some jobs did not cancel within timeout")
					}
					goto cancelComplete
				case <-ticker.C:
					remaining := s.crawlerService.GetRunningJobIDs()
					if len(remaining) == 0 {
						s.logger.Info().Msg("All running jobs cancelled successfully")
						goto cancelComplete
					}
				}
			}
		cancelComplete:
		}
	}

	// Stop stale job ticker if running
	if s.staleJobTicker != nil {
		s.staleJobTicker.Stop()
		s.logger.Info().Msg("Stale job detector stopped")
	}

	s.cron.Stop()
	s.running = false

	s.logger.Info().Msg("Scheduler stopped")
	return nil
}

// executeAutoStartJobs executes all jobs with autoStart=true immediately after scheduler starts
func (s *Service) executeAutoStartJobs() {
	// Wait a brief moment for scheduler to be fully started
	time.Sleep(100 * time.Millisecond)

	// Collect auto-start job names while holding lock
	s.jobMu.Lock()
	autoStartJobs := make([]string, 0)
	for name, entry := range s.jobs {
		if entry.enabled && entry.autoStart {
			autoStartJobs = append(autoStartJobs, name)
		}
	}
	s.jobMu.Unlock()

	// Execute auto-start jobs (without holding lock)
	for _, jobName := range autoStartJobs {
		s.logger.Info().
			Str("job_name", jobName).
			Msg("Executing auto-start job")
		go s.executeJob(jobName)
	}

	if len(autoStartJobs) > 0 {
		s.logger.Info().
			Int("count", len(autoStartJobs)).
			Msg("Auto-start jobs initiated")
	} else {
		s.logger.Debug().Msg("No auto-start jobs configured")
	}
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
func (s *Service) RegisterJob(name string, schedule string, description string, autoStart bool, handler func() error) error {
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
		autoStart:   autoStart,
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
	if err := s.saveJobSettings(name, entry.schedule, entry.description, true, entry.lastRun); err != nil {
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
	if err := s.saveJobSettings(name, entry.schedule, entry.description, false, entry.lastRun); err != nil {
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
	if err := s.saveJobSettings(name, schedule, entry.description, entry.enabled, entry.lastRun); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist job schedule update")
	}

	return nil
}

// UpdateJob updates job settings (description, schedule, enabled status)
// Use nil pointers to skip updating specific fields
func (s *Service) UpdateJob(name string, description, schedule *string, enabled *bool) error {
	s.jobMu.Lock()
	entry, exists := s.jobs[name]
	if !exists {
		s.jobMu.Unlock()
		return fmt.Errorf("job %s not found", name)
	}
	s.jobMu.Unlock()

	// Update description if provided
	if description != nil {
		s.jobMu.Lock()
		entry.description = *description
		s.jobMu.Unlock()
		s.logger.Info().
			Str("job_name", name).
			Str("new_description", *description).
			Msg("Job description updated")
	}

	// Update schedule if provided
	if schedule != nil {
		if err := s.UpdateJobSchedule(name, *schedule); err != nil {
			return fmt.Errorf("failed to update schedule: %w", err)
		}
	}

	// Update enabled status if provided
	if enabled != nil {
		if *enabled {
			if err := s.EnableJob(name); err != nil {
				return fmt.Errorf("failed to enable job: %w", err)
			}
		} else {
			if err := s.DisableJob(name); err != nil {
				return fmt.Errorf("failed to disable job: %w", err)
			}
		}
	}

	// If only description was updated, persist it manually
	// (schedule and enabled changes are already persisted by their respective methods)
	if description != nil && schedule == nil && enabled == nil {
		s.jobMu.Lock()
		if err := s.saveJobSettings(name, entry.schedule, entry.description, entry.enabled, entry.lastRun); err != nil {
			s.jobMu.Unlock()
			s.logger.Warn().Err(err).Msg("Failed to persist job description update")
		} else {
			s.jobMu.Unlock()
		}
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
		AutoStart:   entry.autoStart,
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
	// Variables for panic recovery persistence
	var capturedSchedule string
	var capturedDescription string
	var capturedEnabled bool
	var capturedLastRun *time.Time

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

			// Persist lastRun timestamp even on panic
			if err := s.saveJobSettings(name, capturedSchedule, capturedDescription, capturedEnabled, capturedLastRun); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to persist job lastRun timestamp after panic")
			}
		}
	}()

	// Acquire global mutex to prevent concurrent execution
	s.globalMu.Lock()
	defer s.globalMu.Unlock()

	s.logger.Info().
		Str("job_name", name).
		Msg("🚀 Job execution started")

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
	handler := entry.handler

	// Capture values for persistence before unlocking
	capturedSchedule = entry.schedule
	capturedDescription = entry.description
	capturedEnabled = entry.enabled
	capturedLastRun = entry.lastRun
	s.jobMu.Unlock()

	// Execute job handler
	err := handler()

	// Update status after execution
	completionTime := time.Now()
	s.jobMu.Lock()
	entry.isRunning = false
	entry.lastRun = &completionTime
	if err != nil {
		entry.lastError = err.Error()
		s.logger.Error().
			Str("job_name", name).
			Err(err).
			Dur("duration", time.Since(now)).
			Msg("❌ Job execution failed")
	} else {
		entry.lastError = ""
		s.logger.Info().
			Str("job_name", name).
			Dur("duration", time.Since(now)).
			Msg("✅ Job execution completed successfully")
	}

	// Capture values for persistence before unlocking
	lastRun := entry.lastRun
	capturedLastRun = entry.lastRun
	schedule := entry.schedule
	description := entry.description
	enabled := entry.enabled
	s.jobMu.Unlock()

	// Persist lastRun timestamp to database
	if err := s.saveJobSettings(name, schedule, description, enabled, lastRun); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to persist job lastRun timestamp")
	}
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

	s.logger.Info().Msg("🔄 >>> SCHEDULER: Starting scheduled collection and embedding cycle")

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

	s.logger.Info().Msg("✅ >>> SCHEDULER: Collection completed successfully")
}

// saveJobSettings persists job schedule, description, enabled status, and last run timestamp to database
func (s *Service) saveJobSettings(name string, schedule string, description string, enabled bool, lastRun *time.Time) error {
	if s.db == nil {
		return nil // No database available, skip persistence
	}

	// Convert lastRun to Unix timestamp or NULL
	var lastRunUnix interface{}
	if lastRun != nil {
		lastRunUnix = lastRun.Unix()
	}

	query := `
		INSERT INTO job_settings (job_name, schedule, description, enabled, last_run, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_name) DO UPDATE SET
			schedule = excluded.schedule,
			description = excluded.description,
			enabled = excluded.enabled,
			last_run = excluded.last_run,
			updated_at = excluded.updated_at
	`

	_, err := s.db.Exec(query, name, schedule, description, enabled, lastRunUnix, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to save job settings: %w", err)
	}

	s.logger.Debug().
		Str("job_name", name).
		Str("schedule", schedule).
		Str("description", description).
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

	query := `SELECT job_name, schedule, description, enabled, last_run FROM job_settings`
	rows, err := s.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to load job settings: %w", err)
	}
	defer rows.Close()

	settingsLoaded := 0
	for rows.Next() {
		var name, schedule string
		var description sql.NullString
		var enabled bool
		var lastRunUnix sql.NullInt64

		if err := rows.Scan(&name, &schedule, &description, &enabled, &lastRunUnix); err != nil {
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

		// Restore last_run timestamp
		if lastRunUnix.Valid {
			lastRun := time.Unix(lastRunUnix.Int64, 0)
			s.jobMu.Lock()
			entry.lastRun = &lastRun
			s.jobMu.Unlock()
		}

		// Update description if provided and different
		if description.Valid && entry.description != description.String {
			s.jobMu.Lock()
			entry.description = description.String
			s.jobMu.Unlock()
			settingsLoaded++
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

// LoadJobDefinitions loads job definitions from storage and registers them with the scheduler.
// This method should be called after RegisterJob for hardcoded jobs but before Start.
// Job definitions are registered alongside hardcoded jobs in the scheduler.
// Returns error if storage query fails, but continues on individual registration failures.
func (s *Service) LoadJobDefinitions() error {
	// Graceful degradation if dependencies not available
	if s.jobDefStorage == nil || s.jobExecutor == nil {
		s.logger.Debug().Msg("Job definition storage or executor not available, skipping job definitions load")
		return nil
	}

	ctx := context.Background()

	// Fetch enabled job definitions
	jobDefs, err := s.jobDefStorage.GetEnabledJobDefinitions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load job definitions: %w", err)
	}

	if len(jobDefs) == 0 {
		s.logger.Info().Msg("No enabled job definitions found")
		return nil
	}

	// Register each job definition
	registeredCount := 0
	for _, jobDef := range jobDefs {
		// Create local copy to avoid closure capture issues
		jd := jobDef

		s.logger.Info().
			Str("job_id", jd.ID).
			Str("job_name", jd.Name).
			Str("job_type", jd.JobType).
			Str("schedule", jd.Schedule).
			Msg("Loading job definition")

		// Validate schedule
		if err := common.ValidateJobSchedule(jd.Schedule); err != nil {
			s.logger.Error().
				Str("job_id", jd.ID).
				Err(err).
				Msg("Invalid schedule for job definition, skipping")
			continue
		}

		// Create handler closure that captures the job definition
		handler := func() error {
			execCtx := context.Background()
			return s.jobExecutor.Execute(execCtx, jd)
		}

		// Register job with scheduler
		if err := s.RegisterJob(jd.ID, jd.Schedule, jd.Description, jd.AutoStart, handler); err != nil {
			s.logger.Error().
				Str("job_id", jd.ID).
				Err(err).
				Msg("Failed to register job definition")
			continue
		}

		registeredCount++
	}

	s.logger.Info().
		Int("count", registeredCount).
		Msg("Job definitions loaded and registered")

	return nil
}

// CleanupOrphanedJobs marks orphaned running jobs as failed after service restart
func (s *Service) CleanupOrphanedJobs() error {
	if s.jobStorage == nil {
		return nil // No job storage available
	}

	ctx := context.Background()

	// Get all jobs with status "running"
	runningJobs, err := s.jobStorage.GetJobsByStatus(ctx, "running")
	if err != nil {
		return fmt.Errorf("failed to get running jobs: %w", err)
	}

	if len(runningJobs) == 0 {
		return nil
	}

	s.logger.Info().Int("count", len(runningJobs)).Msg("Cleaning up orphaned jobs from previous run")

	// Mark each as failed
	cleanedCount := 0
	for _, jobInterface := range runningJobs {
		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			s.logger.Warn().Msg("Skipping non-crawler job in orphaned job cleanup")
			continue
		}

		if err := s.jobStorage.UpdateJobStatus(ctx, job.ID, "failed", "Service restarted while job was running"); err != nil {
			s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update orphaned job status")
		} else {
			cleanedCount++
		}
	}

	s.logger.Info().Int("count", cleanedCount).Msg("Orphaned jobs cleaned up")
	return nil
}

// DetectStaleJobs finds and marks stale jobs as failed
func (s *Service) DetectStaleJobs() error {
	if s.jobStorage == nil {
		return nil
	}

	ctx := context.Background()

	// Get jobs with heartbeat older than 10 minutes
	staleJobs, err := s.jobStorage.GetStaleJobs(ctx, 10)
	if err != nil {
		return fmt.Errorf("failed to get stale jobs: %w", err)
	}

	if len(staleJobs) == 0 {
		return nil
	}

	s.logger.Warn().Int("count", len(staleJobs)).Msg("Detected stale jobs (no heartbeat for 10+ minutes)")

	// Mark each as failed
	for _, jobInterface := range staleJobs {
		job, ok := jobInterface.(*crawler.CrawlJob)
		if !ok {
			s.logger.Warn().Msg("Skipping non-crawler job in stale job detection")
			continue
		}

		reason := "Job stale (no heartbeat for 10+ minutes)"

		// Try to fail job via crawler service first to update in-memory state
		failedViaService := false
		if s.crawlerService != nil {
			if err := s.crawlerService.FailJob(job.ID, reason); err != nil {
				// Job not in crawler's active jobs, will fall back to direct storage update
				s.logger.Debug().Err(err).Str("job_id", job.ID).Msg("Job not in crawler memory, updating storage directly")
			} else {
				// Successfully failed via crawler service (in-memory + storage updated)
				failedViaService = true
				s.logger.Info().Str("job_id", job.ID).Msg("Marked stale job as failed (via crawler service)")
			}
		}

		// Fallback to direct storage update if not handled by crawler service
		if !failedViaService {
			if err := s.jobStorage.UpdateJobStatus(ctx, job.ID, "failed", reason); err != nil {
				s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update stale job status")
			} else {
				s.logger.Info().Str("job_id", job.ID).Msg("Marked stale job as failed (via storage)")

				// Emit progress event for UI update
				if s.eventService != nil {
					event := interfaces.Event{
						Type: interfaces.EventCrawlProgress,
						Payload: map[string]interface{}{
							"job_id": job.ID,
							"status": "failed",
							"error":  reason,
						},
					}
					_ = s.eventService.Publish(ctx, event)
				}
			}
		}
	}

	return nil
}

// staleJobDetectorLoop runs periodically to detect and mark stale jobs
func (s *Service) staleJobDetectorLoop() {
	for range s.staleJobTicker.C {
		if err := s.DetectStaleJobs(); err != nil {
			s.logger.Error().Err(err).Msg("Stale job detection failed")
		}
	}
}

// TriggerJob manually triggers a specific job to run immediately
func (s *Service) TriggerJob(name string) error {
	s.jobMu.Lock()
	entry, exists := s.jobs[name]
	if !exists {
		s.jobMu.Unlock()
		return fmt.Errorf("job %s not found", name)
	}

	// Check if already running
	if entry.isRunning {
		s.jobMu.Unlock()
		return fmt.Errorf("job %s is already running", name)
	}
	s.jobMu.Unlock()

	s.logger.Info().
		Str("job_name", name).
		Msg("Manually triggering job execution")

	// Execute job in background goroutine
	go s.executeJob(name)

	return nil
}
