// -----------------------------------------------------------------------
// Last Modified: Sunday, 15th December 2025
// Modified By: Bob McAllan
// Reverted to use robfig/cron (backed out go-quartz)
// -----------------------------------------------------------------------

package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// jobEntry represents a registered job with metadata
type jobEntry struct {
	name        string
	schedule    string
	description string
	handler     func() error
	enabled     bool
	autoStart   bool
	entryID     cron.EntryID // robfig/cron entry ID
	lastRun     *time.Time
	nextRun     *time.Time
	isRunning   bool
	lastError   string
}

// Service implements SchedulerService interface using robfig/cron
type Service struct {
	eventService   interfaces.EventService
	crawlerService *crawler.Service        // For shutdown coordination
	jobStorage     interfaces.QueueStorage // For stale job detection
	cron           *cron.Cron
	logger         arbor.ILogger
	kvStorage      interfaces.KeyValueStorage // For persisting job settings
	mu             sync.Mutex                 // Protects isProcessing
	jobMu          sync.Mutex                 // Protects jobs map
	globalMu       sync.Mutex                 // Prevents concurrent job execution
	jobs           map[string]*jobEntry
	isProcessing   bool
	running        bool
	staleJobTicker *time.Ticker // For stale job detection cleanup goroutine
	jobDefStorage  interfaces.JobDefinitionStorage
	jobExecutor    interface{} // *jobs.JobExecutor - temporarily disabled during queue refactor
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
func NewServiceWithDB(eventService interfaces.EventService, logger arbor.ILogger, kvStorage interfaces.KeyValueStorage, crawlerService *crawler.Service, jobStorage interfaces.QueueStorage, jobDefStorage interfaces.JobDefinitionStorage, jobExecutor interface{}) interfaces.SchedulerService {
	return &Service{
		eventService:   eventService,
		crawlerService: crawlerService,
		jobStorage:     jobStorage,
		jobDefStorage:  jobDefStorage,
		jobExecutor:    jobExecutor,
		cron:           cron.New(),
		logger:         logger,
		kvStorage:      kvStorage,
		jobs:           make(map[string]*jobEntry),
	}
}

// Start begins the scheduler with the given cron expression
func (s *Service) Start(cronExpr string) error {
	if s.running {
		return fmt.Errorf("scheduler already running")
	}

	if s.cron == nil {
		return fmt.Errorf("scheduler not initialized")
	}

	if cronExpr == "" {
		cronExpr = "*/1 * * * *" // Default: every 1 minute
	}

	// Load job definitions from storage and register them BEFORE checking for legacy task
	if err := s.LoadJobDefinitions(); err != nil {
		s.logger.Warn().Err(err).Msg("Failed to load job definitions from storage")
	}

	// Only add legacy scheduled task if no jobs are registered after loading definitions
	s.jobMu.Lock()
	hasDefaultJobs := len(s.jobs) > 0
	s.jobMu.Unlock()

	if !hasDefaultJobs {
		// Register legacy scheduled task
		if err := s.RegisterJob("legacy_collection", cronExpr, "Legacy scheduled collection task", false, s.runScheduledTask); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to register legacy scheduled task")
		} else {
			s.logger.Debug().
				Str("cron_expr", cronExpr).
				Msg("Legacy scheduled task enabled (no job definitions registered)")
		}
	} else {
		s.logger.Debug().
			Msg("Legacy scheduled task disabled (job definitions are registered)")
	}

	// Start the cron scheduler
	s.cron.Start()
	s.running = true

	// Launch stale job detector goroutine (runs every 5 minutes)
	if s.jobStorage != nil {
		s.staleJobTicker = time.NewTicker(5 * time.Minute)
		go s.staleJobDetectorLoop()
		s.logger.Debug().Msg("Stale job detector started (5 minute interval)")
	}

	s.logger.Info().Msg("Scheduler started (robfig/cron)")

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
		s.logger.Debug().Msg("Stale job detector stopped")
	}

	// Stop cron scheduler
	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done() // Wait for running jobs to complete
	}
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
		s.logger.Debug().
			Str("job_name", jobName).
			Msg("Executing auto-start job")
		go s.executeJob(jobName)
	}

	if len(autoStartJobs) > 0 {
		s.logger.Debug().
			Int("count", len(autoStartJobs)).
			Msg("Auto-start jobs initiated")
	} else {
		s.logger.Debug().Msg("No auto-start jobs configured")
	}
}

// TriggerCollectionNow manually triggers collection
func (s *Service) TriggerCollectionNow() error {
	s.logger.Debug().Msg("Manual collection trigger requested")

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

	if s.cron == nil {
		return fmt.Errorf("scheduler not initialized")
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	if _, exists := s.jobs[name]; exists {
		return fmt.Errorf("job %s already registered", name)
	}

	// Create job entry for tracking
	entry := &jobEntry{
		name:        name,
		schedule:    schedule,
		description: description,
		handler:     handler,
		enabled:     true,
		autoStart:   autoStart,
	}

	// Create wrapper function that updates job status
	wrappedHandler := func() {
		s.executeJobHandler(name)
	}

	// Add to cron scheduler
	entryID, err := s.cron.AddFunc(schedule, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	entry.entryID = entryID
	s.jobs[name] = entry

	s.logger.Debug().
		Str("job_name", name).
		Str("schedule", schedule).
		Int("entry_id", int(entryID)).
		Msg("Job registered with robfig/cron")

	return nil
}

// executeJobHandler is called by cron and wraps the actual job execution
func (s *Service) executeJobHandler(name string) {
	s.jobMu.Lock()
	entry, exists := s.jobs[name]
	if !exists {
		s.jobMu.Unlock()
		return
	}

	// Check if enabled
	if !entry.enabled {
		s.jobMu.Unlock()
		return
	}

	// Mark as running
	entry.isRunning = true
	handler := entry.handler
	s.jobMu.Unlock()

	// Execute the handler
	startTime := time.Now()
	err := handler()

	// Update status after execution
	completionTime := time.Now()
	s.jobMu.Lock()
	if entry, exists := s.jobs[name]; exists {
		entry.isRunning = false
		entry.lastRun = &completionTime
		if err != nil {
			entry.lastError = err.Error()
		} else {
			entry.lastError = ""
		}
	}
	s.jobMu.Unlock()

	// Log execution result
	if err != nil {
		s.logger.Error().
			Str("job_name", name).
			Err(err).
			Dur("duration", time.Since(startTime)).
			Msg("Job execution failed")
	} else {
		s.logger.Debug().
			Str("job_name", name).
			Dur("duration", time.Since(startTime)).
			Msg("Job execution completed successfully")
	}

	// Persist lastRun timestamp
	s.jobMu.Lock()
	if entry, exists := s.jobs[name]; exists {
		if saveErr := s.saveJobSettings(name, entry.schedule, entry.description, entry.enabled, entry.lastRun); saveErr != nil {
			s.logger.Warn().Err(saveErr).Msg("Failed to persist job lastRun timestamp")
		}
	}
	s.jobMu.Unlock()
}

// EnableJob enables a disabled job
func (s *Service) EnableJob(name string) error {
	if s.cron == nil {
		return fmt.Errorf("scheduler not initialized")
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	if entry.enabled {
		return nil // Already enabled
	}

	// Re-add to cron scheduler
	wrappedHandler := func() {
		s.executeJobHandler(name)
	}

	entryID, err := s.cron.AddFunc(entry.schedule, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to re-add cron job: %w", err)
	}

	entry.entryID = entryID
	entry.enabled = true

	s.logger.Debug().
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
	if s.cron == nil {
		return fmt.Errorf("scheduler not initialized")
	}

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
	s.cron.Remove(entry.entryID)
	entry.enabled = false

	s.logger.Debug().
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

	if s.cron == nil {
		return fmt.Errorf("scheduler not initialized")
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()

	entry, exists := s.jobs[name]
	if !exists {
		return fmt.Errorf("job %s not found", name)
	}

	// Remove old entry from cron
	if entry.enabled {
		s.cron.Remove(entry.entryID)
	}

	// Create wrapper function
	wrappedHandler := func() {
		s.executeJobHandler(name)
	}

	// Add with new schedule
	entryID, err := s.cron.AddFunc(schedule, wrappedHandler)
	if err != nil {
		return fmt.Errorf("failed to update cron job: %w", err)
	}

	// Update entry
	entry.schedule = schedule
	entry.entryID = entryID

	// If job was disabled, remove from cron again
	if !entry.enabled {
		s.cron.Remove(entryID)
	}

	s.logger.Debug().
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
		s.logger.Debug().
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

	// Get next run time from cron entry
	var nextRun *time.Time
	if entry.enabled && s.cron != nil {
		cronEntry := s.cron.Entry(entry.entryID)
		if !cronEntry.Next.IsZero() {
			nextRun = &cronEntry.Next
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
	// Copy job names while holding lock
	s.jobMu.Lock()
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	s.jobMu.Unlock()

	// Build statuses without holding lock
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
			Msg("Job execution failed")
	} else {
		entry.lastError = ""
		s.logger.Debug().
			Str("job_name", name).
			Dur("duration", time.Since(now)).
			Msg("Job execution completed successfully")
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
func (s *Service) runScheduledTask() error {
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
		return nil
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

	s.logger.Debug().Msg("Starting scheduled collection and embedding cycle")

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
		return err
	}
	s.logger.Debug().Msg(">>> SCHEDULER: Step 7 - Collection event published successfully")

	s.logger.Debug().Msg("Collection completed successfully")
	return nil
}

// JobSettings represents persisted job configuration
type JobSettings struct {
	JobName     string     `json:"job_name"`
	Schedule    string     `json:"schedule"`
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled"`
	LastRun     *time.Time `json:"last_run"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// saveJobSettings persists job schedule, description, enabled status, and last run timestamp to KV storage
func (s *Service) saveJobSettings(name string, schedule string, description string, enabled bool, lastRun *time.Time) error {
	if s.kvStorage == nil {
		return nil // No KV storage available, skip persistence
	}

	settings := JobSettings{
		JobName:     name,
		Schedule:    schedule,
		Description: description,
		Enabled:     enabled,
		LastRun:     lastRun,
		UpdatedAt:   time.Now(),
	}

	// Serialize to JSON
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal job settings: %w", err)
	}

	// Store in KV with prefix
	key := fmt.Sprintf("job_settings:%s", name)
	if err := s.kvStorage.Set(context.Background(), key, string(data), "Job scheduler settings"); err != nil {
		return fmt.Errorf("failed to save job settings: %w", err)
	}

	s.logger.Debug().
		Str("job_name", name).
		Str("schedule", schedule).
		Str("description", description).
		Str("enabled", fmt.Sprintf("%t", enabled)).
		Msg("Job settings persisted to KV storage")

	return nil
}

// LoadJobSettings loads job settings from KV storage and applies them
func (s *Service) LoadJobSettings() error {
	if s.kvStorage == nil {
		s.logger.Debug().Msg("No KV storage available, skipping job settings load")
		return nil
	}

	ctx := context.Background()
	allKeys, err := s.kvStorage.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to load job settings: %w", err)
	}

	settingsLoaded := 0

	for key, value := range allKeys {
		if !strings.HasPrefix(key, "job_settings:") {
			continue
		}

		var settings JobSettings
		if err := json.Unmarshal([]byte(value), &settings); err != nil {
			s.logger.Warn().Err(err).Str("key", key).Msg("Failed to unmarshal job settings")
			continue
		}

		name := settings.JobName
		if name == "" {
			name = strings.TrimPrefix(key, "job_settings:")
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
		if settings.LastRun != nil {
			s.jobMu.Lock()
			entry.lastRun = settings.LastRun
			s.jobMu.Unlock()
		}

		// Update description if provided and different
		if settings.Description != "" && entry.description != settings.Description {
			s.jobMu.Lock()
			entry.description = settings.Description
			s.jobMu.Unlock()
			settingsLoaded++
		}

		// Update schedule if different
		if entry.schedule != settings.Schedule {
			if err := s.UpdateJobSchedule(name, settings.Schedule); err != nil {
				s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to update job schedule from storage")
			} else {
				settingsLoaded++
			}
		}

		// Update enabled status if different
		if entry.enabled != settings.Enabled {
			if settings.Enabled {
				if err := s.EnableJob(name); err != nil {
					s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to enable job from storage")
				}
			} else {
				if err := s.DisableJob(name); err != nil {
					s.logger.Error().Err(err).Str("job_name", name).Msg("Failed to disable job from storage")
				}
			}
			settingsLoaded++
		}
	}

	if settingsLoaded > 0 {
		s.logger.Debug().Int("count", settingsLoaded).Msg("Loaded job settings from storage")
	}

	return nil
}

// LoadJobDefinitions loads job definitions from storage and registers them with the scheduler.
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
		s.logger.Debug().Msg("No enabled job definitions found")
		return nil
	}

	// Register each job definition
	registeredCount := 0
	onDemandCount := 0
	for _, jobDef := range jobDefs {
		jd := jobDef

		s.logger.Debug().
			Str("job_id", jd.ID).
			Str("job_name", jd.Name).
			Str("job_type", string(jd.Type)).
			Str("schedule", jd.Schedule).
			Msg("Loading job definition")

		// Check if this is an on-demand job (empty schedule)
		if jd.Schedule == "" {
			s.logger.Debug().
				Str("job_id", jd.ID).
				Str("job_name", jd.Name).
				Msg("On-demand job definition (no schedule) - can be triggered manually")
			onDemandCount++
			continue
		}

		// Validate schedule for scheduled jobs
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
			_ = execCtx
			s.logger.Warn().Str("job_id", jd.ID).Msg("Job executor temporarily disabled during queue refactor")
			return fmt.Errorf("job executor not available during queue refactor")
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

	s.logger.Debug().
		Int("scheduled_count", registeredCount).
		Int("on_demand_count", onDemandCount).
		Msg("Job definitions loaded")

	return nil
}

// CleanupOrphanedJobs marks orphaned running jobs as failed after service restart
func (s *Service) CleanupOrphanedJobs() error {
	if s.jobStorage == nil {
		return nil
	}

	ctx := context.Background()

	runningJobs, err := s.jobStorage.GetJobsByStatus(ctx, "running")
	if err != nil {
		return fmt.Errorf("failed to get running jobs: %w", err)
	}

	if len(runningJobs) == 0 {
		return nil
	}

	s.logger.Debug().Int("count", len(runningJobs)).Msg("Cleaning up orphaned jobs from previous run")

	cleanedCount := 0
	for _, job := range runningJobs {
		if err := s.jobStorage.UpdateJobStatus(ctx, job.ID, "failed", "Service restarted while job was running"); err != nil {
			s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update orphaned job status")
		} else {
			cleanedCount++
		}
	}

	s.logger.Warn().Int("count", cleanedCount).Msg("Orphaned jobs cleaned up (service was likely not shutdown gracefully)")
	return nil
}

// DetectStaleJobs finds and marks stale jobs as failed
func (s *Service) DetectStaleJobs() error {
	if s.jobStorage == nil {
		return nil
	}

	ctx := context.Background()

	staleJobs, err := s.jobStorage.GetStaleJobs(ctx, 10)
	if err != nil {
		return fmt.Errorf("failed to get stale jobs: %w", err)
	}

	if len(staleJobs) == 0 {
		return nil
	}

	s.logger.Warn().Int("count", len(staleJobs)).Msg("Detected stale jobs (no heartbeat for 10+ minutes)")

	for _, job := range staleJobs {
		reason := "Job stale (no heartbeat for 10+ minutes)"

		failedViaService := false
		if s.crawlerService != nil {
			if err := s.crawlerService.FailJob(job.ID, reason); err != nil {
				s.logger.Debug().Err(err).Str("job_id", job.ID).Msg("Job not in crawler memory, updating storage directly")
			} else {
				failedViaService = true
				s.logger.Debug().Str("job_id", job.ID).Msg("Marked stale job as failed (via crawler service)")
			}
		}

		if !failedViaService {
			if err := s.jobStorage.UpdateJobStatus(ctx, job.ID, "failed", reason); err != nil {
				s.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update stale job status")
			} else {
				s.logger.Debug().Str("job_id", job.ID).Msg("Marked stale job as failed (via storage)")

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
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Str("stack", common.GetStackTrace()).
				Msg("Recovered from panic in stale job detector loop - detector stopped")
		}
	}()

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

	if entry.isRunning {
		s.jobMu.Unlock()
		return fmt.Errorf("job %s is already running", name)
	}
	s.jobMu.Unlock()

	s.logger.Debug().
		Str("job_name", name).
		Msg("Manually triggering job execution")

	// Execute job in background goroutine
	go s.executeJob(name)

	return nil
}
