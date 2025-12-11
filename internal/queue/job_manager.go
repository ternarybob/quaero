package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/interfaces/jobtypes"
	"github.com/ternarybob/quaero/internal/models"
)

// Manager handles job metadata, lifecycle, and worker routing.
// This is the single manager for the queue system - it routes job definition steps to workers.
type Manager struct {
	jobStorage   interfaces.QueueStorage
	logStorage   interfaces.LogStorage
	queue        interfaces.QueueManager
	eventService interfaces.EventService // Optional: may be nil for testing
	logger       arbor.ILogger

	// Worker registry moved to StepManager
	kvStorage interfaces.KeyValueStorage // For resolving {key-name} placeholders

	// Job stats throttling to avoid excessive WebSocket broadcasts
	lastStatsPublish time.Time
	statsMutex       sync.Mutex
}

func NewManager(jobStorage interfaces.QueueStorage, logStorage interfaces.LogStorage, queue interfaces.QueueManager, eventService interfaces.EventService, logger arbor.ILogger) *Manager {
	return &Manager{
		jobStorage:   jobStorage,
		logStorage:   logStorage,
		queue:        queue,
		eventService: eventService,
		logger:       logger,
	}
}

// SetKVStorage sets the key-value storage for resolving placeholders in step configs.
// This should be called during initialization if placeholder resolution is needed.
func (m *Manager) SetKVStorage(kvStorage interfaces.KeyValueStorage) {
	m.kvStorage = kvStorage
}

// Job represents job metadata
type Job struct {
	ID              string     `json:"id"`
	ParentID        *string    `json:"parent_id,omitempty"`
	Type            string     `json:"job_type"`
	Name            string     `json:"name"` // Human-readable job name
	Phase           string     `json:"phase"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"` // Set when job AND all children complete or timeout
	Payload         string     `json:"payload,omitempty"`
	Result          string     `json:"result,omitempty"`
	Error           *string    `json:"error,omitempty"`
	ProgressCurrent int        `json:"progress_current"`
	ProgressTotal   int        `json:"progress_total"`
}

// JobLog represents a job log entry
type JobLog struct {
	ID        int       `json:"id"`
	JobID     string    `json:"job_id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// Helper functions for time conversions
func timeToUnix(t time.Time) int64 {
	return t.Unix()
}

func unixToTime(unix int64) time.Time {
	return time.Unix(unix, 0)
}

// CreateJobRecord creates a new job record without enqueueing (for tracking only)
func (m *Manager) CreateJobRecord(ctx context.Context, job *Job) error {
	// Convert internal Job to models.QueueJob for storage
	metadata := map[string]interface{}{
		"phase": job.Phase,
	}

	config := make(map[string]interface{})

	// Extract metadata from Payload JSON if present (e.g., step_name from child jobs)
	if job.Payload != "" {
		var payloadData struct {
			Metadata map[string]interface{} `json:"metadata"`
			Config   map[string]interface{} `json:"config"`
		}
		if err := json.Unmarshal([]byte(job.Payload), &payloadData); err == nil {
			// Merge extracted metadata with base metadata
			for k, v := range payloadData.Metadata {
				metadata[k] = v
			}
			// Merge extracted config
			for k, v := range payloadData.Config {
				config[k] = v
			}
		}
	}

	queueJob := &models.QueueJob{
		ID:        job.ID,
		Type:      job.Type,
		Name:      job.Name,
		CreatedAt: job.CreatedAt,
		Config:    config,
		Metadata:  metadata,
	}

	if job.ParentID != nil {
		queueJob.ParentID = job.ParentID
	}

	// Create job record using storage interface
	jobState := models.NewQueueJobState(queueJob)
	jobState.Status = models.JobStatus(job.Status)

	if job.ProgressTotal > 0 {
		jobState.UpdateProgress(job.ProgressCurrent, 0, 0, job.ProgressTotal)
	}

	if err := m.jobStorage.SaveJob(ctx, jobState); err != nil {
		return fmt.Errorf("create job record: %w", err)
	}

	return nil
}

// CreateParentJob creates a new parent job and enqueues it
func (m *Manager) CreateParentJob(ctx context.Context, jobType string, payload interface{}) (string, error) {
	jobID := uuid.New().String()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Parse payload as config map if possible
	var config map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &config); err != nil {
		// If not a map, wrap it
		config = map[string]interface{}{"payload": payload}
	}

	metadata := map[string]interface{}{
		"phase":          "core",
		"document_count": 0,
	}

	queueJob := models.NewQueueJob(jobType, "", config, metadata)
	queueJob.ID = jobID

	jobState := models.NewQueueJobState(queueJob)
	jobState.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobState); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, Message{
		JobID:   jobID,
		Type:    jobType,
		Payload: payloadJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobID, nil
}

// CreateChildJob creates a child job and enqueues it
func (m *Manager) CreateChildJob(ctx context.Context, parentID, jobType, phase string, payload interface{}) (string, error) {
	jobID := uuid.New().String()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	// Parse payload as config map if possible
	var config map[string]interface{}
	if err := json.Unmarshal(payloadJSON, &config); err != nil {
		config = map[string]interface{}{"payload": payload}
	}

	metadata := map[string]interface{}{
		"phase": phase,
	}

	queueJob := models.NewQueueJob(jobType, "", config, metadata)
	queueJob.ID = jobID
	queueJob.ParentID = &parentID

	jobState := models.NewQueueJobState(queueJob)
	jobState.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobState); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, Message{
		JobID:   jobID,
		Type:    jobType,
		Payload: payloadJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobID, nil
}

// GetJobInternal retrieves a job by ID (internal jobs.Job type)
func (m *Manager) GetJobInternal(ctx context.Context, jobID string) (*Job, error) {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	jobState, ok := jobEntityInterface.(*models.QueueJobState)
	if !ok {
		return nil, fmt.Errorf("invalid job type from storage")
	}

	job := &Job{
		ID:          jobState.ID,
		Type:        jobState.Type,
		Name:        jobState.Name,
		Status:      string(jobState.Status),
		CreatedAt:   jobState.CreatedAt,
		StartedAt:   jobState.StartedAt,
		CompletedAt: jobState.CompletedAt,
		FinishedAt:  jobState.FinishedAt,
		ParentID:    jobState.ParentID,
	}

	if jobState.Error != "" {
		job.Error = &jobState.Error
	}

	// Map config to payload
	if configJSON, err := json.Marshal(jobState.Config); err == nil {
		job.Payload = string(configJSON)
	}

	// Map metadata fields
	if phase, ok := jobState.Metadata["phase"].(string); ok {
		job.Phase = phase
	}
	if result, ok := jobState.Metadata["result"].(string); ok {
		job.Result = result
	}

	// Map progress (Progress is now a value type, always present)
	job.ProgressCurrent = jobState.Progress.CompletedURLs
	job.ProgressTotal = jobState.Progress.TotalURLs

	return job, nil
}

// ListParentJobs returns all parent jobs (parent_id IS NULL)
func (m *Manager) ListParentJobs(ctx context.Context, limit, offset int) ([]Job, error) {
	opts := &interfaces.JobListOptions{
		ParentID: "root", // Special value for root jobs
		Limit:    limit,
		Offset:   offset,
		OrderBy:  "created_at",
		OrderDir: "DESC",
	}

	jobEntities, err := m.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	for _, je := range jobEntities {
		job := Job{
			ID:          je.ID,
			Type:        je.Type,
			Name:        je.Name,
			Status:      string(je.Status),
			CreatedAt:   je.CreatedAt,
			StartedAt:   je.StartedAt,
			CompletedAt: je.CompletedAt,
			FinishedAt:  je.FinishedAt,
			ParentID:    je.ParentID,
		}

		if je.Error != "" {
			job.Error = &je.Error
		}

		// Map config to payload
		if configJSON, err := json.Marshal(je.Config); err == nil {
			job.Payload = string(configJSON)
		}

		// Map metadata fields
		if phase, ok := je.Metadata["phase"].(string); ok {
			job.Phase = phase
		}
		if result, ok := je.Metadata["result"].(string); ok {
			job.Result = result
		}

		// Map progress (Progress is now a value type, always present)
		job.ProgressCurrent = je.Progress.CompletedURLs
		job.ProgressTotal = je.Progress.TotalURLs

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// ListChildJobs returns all child jobs for a parent
func (m *Manager) ListChildJobs(ctx context.Context, parentID string) ([]Job, error) {
	jobModels, err := m.jobStorage.GetChildJobs(ctx, parentID)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	for _, jm := range jobModels {
		// Same issue as ListParentJobs
		job := Job{
			ID:        jm.ID,
			ParentID:  jm.ParentID,
			Type:      jm.Type,
			CreatedAt: jm.CreatedAt,
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJobLogs retrieves logs for a job
func (m *Manager) GetJobLogs(ctx context.Context, jobID string, limit int) ([]JobLog, error) {
	entries, err := m.logStorage.GetLogs(ctx, jobID, limit)
	if err != nil {
		return nil, err
	}

	var logs []JobLog
	for _, entry := range entries {
		ts, _ := time.Parse(time.RFC3339, entry.FullTimestamp)
		logs = append(logs, JobLog{
			JobID:     entry.JobID(),
			Timestamp: ts,
			Level:     entry.Level,
			Message:   entry.Message,
		})
	}

	return logs, nil
}

// ============================================================================
// Interface Adapter Methods (interfaces.JobManager)
// ============================================================================

// CreateJob implements interfaces.JobManager.CreateJob
// Creates a new job with the given source type, source ID, and configuration
func (m *Manager) CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error) {
	// Create job model
	jobType := sourceType // Use source type as job type
	name := fmt.Sprintf("%s job for %s", sourceType, sourceID)

	metadata := map[string]interface{}{
		"source_id": sourceID,
	}

	queueJob := models.NewQueueJob(jobType, name, config, metadata)

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create job record in storage
	jobState := models.NewQueueJobState(queueJob)
	jobState.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobState); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, Message{
		JobID:   queueJob.ID,
		Type:    jobType,
		Payload: configJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return queueJob.ID, nil
}

// GetJob implements interfaces.JobManager.GetJob
// Returns interface{} to match the interface, but the actual type is *models.QueueJobState
func (m *Manager) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	return m.jobStorage.GetJob(ctx, jobID)
}

// ListJobs implements interfaces.JobManager.ListJobs
// Returns []*models.QueueJobState directly from storage
func (m *Manager) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.QueueJobState, error) {
	return m.jobStorage.ListJobs(ctx, opts)
}

// CountJobs implements interfaces.JobManager.CountJobs
func (m *Manager) CountJobs(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	return m.jobStorage.CountJobsWithFilters(ctx, opts)
}

// UpdateJob implements interfaces.JobManager.UpdateJob
func (m *Manager) UpdateJob(ctx context.Context, job interface{}) error {
	return m.jobStorage.UpdateJob(ctx, job)
}

// DeleteJob implements interfaces.JobManager.DeleteJob
func (m *Manager) DeleteJob(ctx context.Context, jobID string) (int, error) {
	// Count children before deletion (CASCADE will delete them)
	// Note: Badger storage implementation of DeleteJob might not return count of deleted children
	// We'll try to count them first
	childStats, _ := m.jobStorage.GetJobChildStats(ctx, []string{jobID})
	childCount := 0
	if stats, ok := childStats[jobID]; ok {
		childCount = stats.ChildCount
	}

	if err := m.jobStorage.DeleteJob(ctx, jobID); err != nil {
		return 0, err
	}

	return childCount, nil
}

// CopyJob implements interfaces.JobManager.CopyJob
func (m *Manager) CopyJob(ctx context.Context, jobID string) (string, error) {
	// Get original job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	// Create new job with same configuration
	newJobID := uuid.New().String()

	// Clone queue job
	newQueueJob := jobState.ToQueueJob().Clone()
	newQueueJob.ID = newJobID
	newQueueJob.Name = jobState.Name + " (Copy)"
	newQueueJob.CreatedAt = time.Now()

	newJobState := models.NewQueueJobState(newQueueJob)
	newJobState.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, newJobState); err != nil {
		return "", fmt.Errorf("failed to create job copy: %w", err)
	}

	return newJobID, nil
}

// GetQueue returns the queue manager for enqueueing jobs
func (m *Manager) GetQueue() interfaces.QueueManager {
	return m.queue
}

// GetQueueInterface returns the queue manager interface
func (m *Manager) GetQueueInterface() interfaces.QueueManager {
	return m.queue
}

// ============================================================================
// State Management Methods (delegate to storage layer)
// ============================================================================

// UpdateJobStatus updates the job status
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	m.logger.Debug().
		Str("job_id", jobID).
		Str("status", status).
		Msg("UpdateJobStatus called")

	// Get job details before update to access parent_id and job_type
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		m.logger.Error().Err(err).Str("job_id", jobID).Msg("UpdateJobStatus: failed to get job details")
		return fmt.Errorf("failed to get job details: %w", err)
	}

	jobState, ok := jobEntityInterface.(*models.QueueJobState)
	if !ok {
		m.logger.Error().Str("job_id", jobID).Msg("UpdateJobStatus: invalid job type")
		return fmt.Errorf("invalid job type")
	}

	// Update status via storage interface
	if err := m.jobStorage.UpdateJobStatus(ctx, jobID, status, ""); err != nil {
		m.logger.Error().Err(err).Str("job_id", jobID).Msg("UpdateJobStatus: failed to update status in storage")
		return err
	}

	// Add job log for status change
	logMessage := fmt.Sprintf("Status changed: %s", status)
	if err := m.AddJobLog(ctx, jobID, "info", logMessage); err != nil {
		m.logger.Warn().Err(err).Str("job_id", jobID).Msg("UpdateJobStatus: failed to add job log")
		// Log error but don't fail the status update (logging is non-critical)
	}

	// Publish job status change event for parent job monitoring
	// Only publish if eventService is available (optional dependency)
	m.logger.Debug().
		Str("job_id", jobID).
		Bool("has_event_service", m.eventService != nil).
		Msg("UpdateJobStatus: checking eventService")
	if m.eventService != nil {
		payload := map[string]interface{}{
			"job_id":    jobID,
			"status":    status,
			"job_type":  jobState.Type,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		// Include parent_id if this is a child job
		if jobState.ParentID != nil {
			payload["parent_id"] = *jobState.ParentID
		}

		// Include document_count from metadata for completed jobs
		if jobState.Metadata != nil {
			if docCount, ok := jobState.Metadata["document_count"].(float64); ok {
				payload["document_count"] = int(docCount)
			} else if docCount, ok := jobState.Metadata["document_count"].(int); ok {
				payload["document_count"] = docCount
			}
		}

		// Publish EventJobStatusChange for monitor tracking
		statusChangeEvent := interfaces.Event{
			Type:    interfaces.EventJobStatusChange,
			Payload: payload,
		}

		// Determine lifecycle event type based on terminal status
		var lifecycleEventType interfaces.EventType
		switch status {
		case string(models.JobStatusCompleted):
			lifecycleEventType = interfaces.EventJobCompleted
		case string(models.JobStatusFailed):
			lifecycleEventType = interfaces.EventJobFailed
		case string(models.JobStatusCancelled):
			lifecycleEventType = interfaces.EventJobCancelled
		}

		// Publish asynchronously to avoid blocking status updates
		go func() {
			// Always publish status change event (for monitor)
			if err := m.eventService.Publish(ctx, statusChangeEvent); err != nil {
				// Log error but don't fail the status update
			}

			// Also publish specific lifecycle event for WebSocket handler
			// This enables real-time UI updates via job_status_change WebSocket messages
			if lifecycleEventType != "" {
				lifecycleEvent := interfaces.Event{
					Type:    lifecycleEventType,
					Payload: payload,
				}
				if err := m.eventService.Publish(ctx, lifecycleEvent); err != nil {
					// Log error but don't fail
				}
			}

			// Publish job stats event for real-time dashboard updates (throttled)
			m.publishJobStats(ctx)
		}()
	}

	return nil
}

// publishJobStats publishes current job statistics via EventJobStats.
// Throttled to publish at most once per 500ms to avoid flooding WebSocket clients.
func (m *Manager) publishJobStats(ctx context.Context) {
	if m.eventService == nil {
		return
	}

	// Throttle stats publishing to avoid excessive broadcasts
	m.statsMutex.Lock()
	if time.Since(m.lastStatsPublish) < 500*time.Millisecond {
		m.statsMutex.Unlock()
		return
	}
	m.lastStatsPublish = time.Now()
	m.statsMutex.Unlock()

	// Query current stats from storage
	totalCount, _ := m.jobStorage.CountJobs(ctx)
	pendingCount, _ := m.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusPending))
	runningCount, _ := m.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusRunning))
	completedCount, _ := m.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusCompleted))
	failedCount, _ := m.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusFailed))
	cancelledCount, _ := m.jobStorage.CountJobsByStatus(ctx, string(models.JobStatusCancelled))

	statsPayload := map[string]interface{}{
		"total_jobs":     totalCount,
		"pending_jobs":   pendingCount,
		"running_jobs":   runningCount,
		"completed_jobs": completedCount,
		"failed_jobs":    failedCount,
		"cancelled_jobs": cancelledCount,
		"timestamp":      time.Now().Format(time.RFC3339),
	}

	statsEvent := interfaces.Event{
		Type:    interfaces.EventJobStats,
		Payload: statsPayload,
	}

	if err := m.eventService.Publish(ctx, statsEvent); err != nil {
		m.logger.Debug().Err(err).Msg("Failed to publish job stats event")
	}
}

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
	return m.jobStorage.UpdateJobStatus(ctx, jobID, string(models.JobStatusFailed), errorMsg)
}

// SetJobFinished sets the finished_at timestamp for a job
func (m *Manager) SetJobFinished(ctx context.Context, jobID string) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	now := time.Now()
	jobState.FinishedAt = &now

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// UpdateJobConfig updates the job configuration in the database
func (m *Manager) UpdateJobConfig(ctx context.Context, jobID string, config map[string]interface{}) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	jobState.Config = config

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// UpdateJobMetadata updates the job metadata in the database
func (m *Manager) UpdateJobMetadata(ctx context.Context, jobID string, metadata map[string]interface{}) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		jobState.Metadata = make(map[string]interface{})
	}

	// Merge metadata
	for k, v := range metadata {
		jobState.Metadata[k] = v
	}

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// UpdateStepStatInManager updates a step's status in the manager's step_stats metadata.
// This is called by StepMonitor when a step completes to update the UI display.
func (m *Manager) UpdateStepStatInManager(ctx context.Context, stepID, managerID, status string) error {
	// Get manager job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, managerID)
	if err != nil {
		return fmt.Errorf("get manager job %s: %w", managerID, err)
	}
	managerJob := jobEntityInterface.(*models.QueueJobState)

	if managerJob.Metadata == nil {
		return fmt.Errorf("manager job %s has no metadata", managerID)
	}

	// Get step_stats array from metadata
	stepStatsInterface, ok := managerJob.Metadata["step_stats"]
	if !ok {
		return fmt.Errorf("manager job %s has no step_stats in metadata", managerID)
	}

	// step_stats is stored as []interface{} in JSON
	stepStats, ok := stepStatsInterface.([]interface{})
	if !ok {
		return fmt.Errorf("step_stats is not an array: %T", stepStatsInterface)
	}

	// Find and update the step by step_id
	found := false
	for i, statInterface := range stepStats {
		stat, ok := statInterface.(map[string]interface{})
		if !ok {
			continue
		}
		if stat["step_id"] == stepID {
			stat["status"] = status
			stepStats[i] = stat
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("step %s not found in step_stats", stepID)
	}

	// Update metadata with modified step_stats
	managerJob.Metadata["step_stats"] = stepStats

	// Also update current_step_status if this is the current step
	if currentStepID, ok := managerJob.Metadata["current_step_id"].(string); ok && currentStepID == stepID {
		managerJob.Metadata["current_step_status"] = status
	}

	return m.jobStorage.UpdateJob(ctx, managerJob)
}

// AddJobLog adds a log entry for a job to the database and publishes to WebSocket.
// Automatically resolves step context from job metadata/parent chain for UI display.
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
	// Default behavior: determine originator based on job type
	return m.AddJobLogWithPhase(ctx, jobID, level, message, "", "")
}

// AddJobLogWithPhase adds a job log with an explicit execution phase.
// Phase values:
//   - "init" - initialization/assessment phase (cloning, scanning, assessing work)
//   - "run" - execution/job creation phase (creating jobs, processing files)
//   - "orchestrator" - orchestrator coordination (step monitoring, job tracking)
//   - "" (empty) - no specific phase
func (m *Manager) AddJobLogWithPhase(ctx context.Context, jobID, level, message, originator, phase string) error {
	// Resolve step context from job metadata
	stepName, _, _ := m.resolveJobContext(ctx, jobID)
	return m.AddJobLogFull(ctx, jobID, level, message, stepName, originator, phase)
}

// AddJobLogWithOriginator adds a job log with an explicit originator.
// Originator values:
//   - "step" - for StepMonitor logs (e.g., "Starting workers", "Step finished")
//   - "worker" - for worker-generated logs (e.g., "Document saved", "Started:", "Completed:")
//   - "" (empty) - for JobMonitor/system logs (e.g., "Child job X → completed")
//
// If originator is empty, it will be determined based on job type (legacy behavior).
func (m *Manager) AddJobLogWithOriginator(ctx context.Context, jobID, level, message, originator string) error {
	// Resolve step context from job metadata
	stepName, _, _ := m.resolveJobContext(ctx, jobID)
	return m.AddJobLogFull(ctx, jobID, level, message, stepName, originator, "")
}

// AddJobLogWithContext adds a job log with explicit step name and originator.
// Use this when the caller knows the step context (e.g., StepMonitor).
func (m *Manager) AddJobLogWithContext(ctx context.Context, jobID, level, message, stepName, originator string) error {
	return m.AddJobLogFull(ctx, jobID, level, message, stepName, originator, "")
}

// AddJobLogFull adds a job log with all explicit parameters.
// This is the most flexible logging function - use when you need full control.
func (m *Manager) AddJobLogFull(ctx context.Context, jobID, level, message, stepName, originator, phase string) error {
	now := time.Now()

	// Resolve full job hierarchy context for indexed queries
	resolvedStepName, managerID, stepID, parentID, resolvedOriginator := m.resolveJobHierarchy(ctx, jobID)

	// Use explicit originator if provided, otherwise use resolved originator
	if originator != "" {
		resolvedOriginator = originator
	}

	// Use explicit stepName if provided
	if stepName != "" {
		resolvedStepName = stepName
	}

	// For manager jobs, manager_id should be the job ID itself (for WebSocket routing)
	if managerID == "" {
		managerID = jobID
	}

	// Build context map with all metadata
	context := map[string]string{
		models.LogCtxJobID: jobID,
	}
	if resolvedStepName != "" {
		context[models.LogCtxStepName] = resolvedStepName
	}
	if resolvedOriginator != "" {
		context[models.LogCtxOriginator] = resolvedOriginator
	}
	if phase != "" {
		context[models.LogCtxPhase] = phase
	}
	if managerID != "" {
		context[models.LogCtxManagerID] = managerID
	}
	if stepID != "" {
		context[models.LogCtxStepID] = stepID
	}
	if parentID != "" {
		context[models.LogCtxParentID] = parentID
	}

	entry := models.LogEntry{
		Timestamp:     now.Format("15:04:05"),
		FullTimestamp: now.Format(time.RFC3339),
		Level:         level,
		Message:       message,
		Context:       context,
	}

	if err := m.logStorage.AppendLog(ctx, jobID, entry); err != nil {
		return fmt.Errorf("failed to append log: %w", err)
	}

	// Publish to WebSocket for real-time UI display (INFO+ levels only)
	if m.eventService != nil && m.shouldPublishLogLevel(level) {
		payload := map[string]interface{}{
			"job_id":     jobID,
			"manager_id": managerID,
			"step_name":  stepName,
			"originator": resolvedOriginator,
			"phase":      phase,
			"level":      level,
			"message":    message,
			"timestamp":  now.Format(time.RFC3339),
		}

		event := interfaces.Event{
			Type:    interfaces.EventJobLog,
			Payload: payload,
		}

		go func() {
			if err := m.eventService.Publish(ctx, event); err != nil {
				m.logger.Debug().Err(err).
					Str("job_id", jobID).
					Msg("Failed to publish job log event")
			}
		}()
	}

	return nil
}

// shouldPublishLogLevel returns true if the log level should be published to WebSocket
func (m *Manager) shouldPublishLogLevel(level string) bool {
	level = strings.ToLower(level)
	return level == "info" || level == "warn" || level == "error" || level == "fatal"
}

// resolveJobContext resolves step_name, manager_id, and originator from job metadata.
// Originator is determined by job type: "manager", "step", or "worker".
func (m *Manager) resolveJobContext(ctx context.Context, jobID string) (stepName, managerID, originator string) {
	jobInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return "", "", ""
	}

	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		return "", "", ""
	}

	// Determine originator based on job type
	switch jobState.Type {
	case string(models.JobTypeManager):
		originator = "manager"
	case string(models.JobTypeStep):
		originator = "step"
	default:
		originator = "worker"
	}

	// Check job metadata
	if jobState.Metadata != nil {
		if sn, ok := jobState.Metadata["step_name"].(string); ok {
			stepName = sn
		}
		if mid, ok := jobState.Metadata["manager_id"].(string); ok {
			managerID = mid
		}
	}

	return stepName, managerID, originator
}

// resolveJobHierarchy resolves the full job hierarchy context for log indexing.
// Returns: stepName, managerID, stepID, parentID, originator
// This enables efficient queries across the job hierarchy: Manager -> Step -> Worker
func (m *Manager) resolveJobHierarchy(ctx context.Context, jobID string) (stepName, managerID, stepID, parentID, originator string) {
	jobInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return "", "", "", "", ""
	}

	jobState, ok := jobInterface.(*models.QueueJobState)
	if !ok {
		return "", "", "", "", ""
	}

	// Determine originator based on job type
	switch jobState.Type {
	case string(models.JobTypeManager):
		originator = "manager"
		managerID = jobID // Manager is itself
	case string(models.JobTypeStep):
		originator = "step"
		stepID = jobID // Step is itself
	default:
		originator = "worker"
	}

	// Get direct parent ID
	if jobState.ParentID != nil && *jobState.ParentID != "" {
		parentID = *jobState.ParentID
	}

	// Extract hierarchy IDs from metadata
	if jobState.Metadata != nil {
		// Step name for display
		if sn, ok := jobState.Metadata["step_name"].(string); ok {
			stepName = sn
		}
		// Manager ID (root of hierarchy)
		if mid, ok := jobState.Metadata["manager_id"].(string); ok {
			managerID = mid
		}
		// Step ID (for workers - their parent step)
		if sid, ok := jobState.Metadata["step_id"].(string); ok && stepID == "" {
			stepID = sid
		}
	}

	// For workers, try to resolve step_id from parent if not in metadata
	if originator == "worker" && stepID == "" && parentID != "" {
		// Check if parent is a step job
		if parentJob, err := m.jobStorage.GetJob(ctx, parentID); err == nil {
			if parentState, ok := parentJob.(*models.QueueJobState); ok {
				if parentState.Type == string(models.JobTypeStep) {
					stepID = parentID
				}
			}
		}
	}

	return stepName, managerID, stepID, parentID, originator
}

// UpdateJobProgress updates job progress
func (m *Manager) UpdateJobProgress(ctx context.Context, jobID string, current, total int) error {
	progress := &models.JobProgress{
		CompletedURLs: current,
		TotalURLs:     total,
	}

	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("marshal progress: %w", err)
	}

	return m.jobStorage.UpdateJobProgress(ctx, jobID, string(progressJSON))
}

// GetFailedChildCount returns the number of failed child jobs for a parent job
func (m *Manager) GetFailedChildCount(ctx context.Context, parentID string) (int, error) {
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
		Status:   string(models.JobStatusFailed),
	}

	jobs, err := m.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return 0, err
	}

	return len(jobs), nil
}

// GetJobChildStats retrieves child job statistics for multiple parent jobs
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*jobtypes.JobChildStats, error) {
	return m.jobStorage.GetJobChildStats(ctx, parentIDs)
}

// GetStepStats retrieves aggregate statistics for step jobs under a manager
// Used by ManagerMonitor to track overall progress of multi-step job definitions
func (m *Manager) GetStepStats(ctx context.Context, managerID string) (*interfaces.StepStats, error) {
	return m.jobStorage.GetStepStats(ctx, managerID)
}

// ListStepJobs returns all step jobs under a manager
// Used for displaying step-level hierarchy in the UI
func (m *Manager) ListStepJobs(ctx context.Context, managerID string) ([]*models.QueueJob, error) {
	return m.jobStorage.ListStepJobs(ctx, managerID)
}

// IncrementDocumentCount atomically increments the document_count for a job
// Uses the storage layer's atomic increment to prevent race conditions
func (m *Manager) IncrementDocumentCount(ctx context.Context, jobID string) error {
	_, err := m.jobStorage.IncrementDocumentCountAtomic(ctx, jobID)
	if err != nil {
		return fmt.Errorf("IncrementDocumentCount: failed to increment for job %s: %w", jobID, err)
	}
	return nil
}

// AddJobError adds an error message to the job's status_report
func (m *Manager) AddJobError(ctx context.Context, jobID, errorMessage string) error {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		jobState.Metadata = make(map[string]interface{})
	}

	var statusReport map[string]interface{}
	if sr, ok := jobState.Metadata["status_report"].(map[string]interface{}); ok {
		statusReport = sr
	} else {
		statusReport = make(map[string]interface{})
	}

	var errors []interface{}
	if e, ok := statusReport["errors"].([]interface{}); ok {
		errors = e
	} else {
		errors = make([]interface{}, 0)
	}

	errors = append(errors, errorMessage)
	statusReport["errors"] = errors
	jobState.Metadata["status_report"] = statusReport

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// AddJobWarning adds a warning message to the job's status_report
func (m *Manager) AddJobWarning(ctx context.Context, jobID, warningMessage string) error {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		jobState.Metadata = make(map[string]interface{})
	}

	var statusReport map[string]interface{}
	if sr, ok := jobState.Metadata["status_report"].(map[string]interface{}); ok {
		statusReport = sr
	} else {
		statusReport = make(map[string]interface{})
	}

	var warnings []interface{}
	if w, ok := statusReport["warnings"].([]interface{}); ok {
		warnings = w
	} else {
		warnings = make([]interface{}, 0)
	}

	warnings = append(warnings, warningMessage)
	statusReport["warnings"] = warnings
	jobState.Metadata["status_report"] = statusReport

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// GetDocumentCount retrieves the document_count from job metadata
func (m *Manager) GetDocumentCount(ctx context.Context, jobID string) (int, error) {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return 0, err
	}
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		return 0, nil
	}

	if count, ok := jobState.Metadata["document_count"].(float64); ok {
		return int(count), nil
	} else if count, ok := jobState.Metadata["document_count"].(int); ok {
		return count, nil
	}

	return 0, nil
}

// StopAllChildJobs cancels all running child jobs of the specified parent job.
// This method recursively cancels children (handles Manager → Step → Job hierarchy)
// and publishes EventJobCancelled for each running job so the JobProcessor can
// cancel them immediately via context cancellation.
func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error) {
	// Get all child jobs
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
	}

	children, err := m.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, child := range children {
		// Recursively cancel children of step jobs (handles Manager → Step → Job hierarchy)
		if child.Type == "step" || child.Type == "manager" || child.Type == "parent" {
			nestedCount, _ := m.StopAllChildJobs(ctx, child.ID)
			count += nestedCount
		}

		if child.Status == models.JobStatusRunning || child.Status == models.JobStatusPending {
			if err := m.jobStorage.UpdateJobStatus(ctx, child.ID, string(models.JobStatusCancelled), ""); err == nil {
				count++

				// Publish EventJobCancelled for running jobs so JobProcessor can cancel them
				if child.Status == models.JobStatusRunning && m.eventService != nil {
					cancelEvent := interfaces.Event{
						Type: interfaces.EventJobCancelled,
						Payload: map[string]interface{}{
							"job_id":    child.ID,
							"status":    "cancelled",
							"parent_id": parentID,
						},
					}
					if err := m.eventService.Publish(ctx, cancelEvent); err != nil {
						m.logger.Warn().Err(err).Str("job_id", child.ID).Msg("Failed to publish cancel event for child job")
					}
				}
			}
		}
	}

	return count, nil
}

// ============================================================================
// Job Definition Execution (Step Orchestration)
// ============================================================================

// ExecuteJobDefinition executes a job definition by routing steps to workers.
// Creates a manager job that orchestrates steps. Each step becomes a step job
// that monitors its spawned child jobs.
//
// Hierarchy: Manager -> Steps -> Jobs
//   - Manager (type="manager"): Top-level orchestrator, monitors steps
//   - Step (type="step"): Step container, monitors its spawned jobs
//   - Job (type=various): Individual work units, children of steps
//
// Returns the manager job ID for tracking.
/*
func (m *Manager) ExecuteJobDefinition(ctx context.Context, jobDef *models.JobDefinition, jobMonitor interfaces.JobMonitor, stepMonitor interfaces.StepMonitor) (string, error) {
	// Generate manager job ID (top-level orchestrator)
	managerID := uuid.New().String()

	// Create manager job record in database
	managerJob := &Job{
		ID:              managerID,
		ParentID:        nil,
		Type:            string(models.JobTypeManager),
		Name:            jobDef.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       time.Now(),
		ProgressCurrent: 0,
		ProgressTotal:   len(jobDef.Steps),
	}

	if err := m.CreateJobRecord(ctx, managerJob); err != nil {
		return "", fmt.Errorf("failed to create manager job: %w", err)
	}

	// Persist metadata immediately after job creation
	managerMetadata := make(map[string]interface{})
	if jobDef.AuthID != "" {
		managerMetadata["auth_id"] = jobDef.AuthID
	}
	if jobDef.ID != "" {
		managerMetadata["job_definition_id"] = jobDef.ID
	}
	managerMetadata["phase"] = "execution"

	if err := m.UpdateJobMetadata(ctx, managerID, managerMetadata); err != nil {
		// Log warning but continue
	}

	// Add initial job log
	initialLog := fmt.Sprintf("Starting job definition execution: %s (ID: %s, Steps: %d)",
		jobDef.Name, jobDef.ID, len(jobDef.Steps))
	m.AddJobLog(ctx, managerID, "info", initialLog)

	// Build job definition config for manager job
	jobDefConfig := make(map[string]interface{})
	for i, step := range jobDef.Steps {
		stepKey := fmt.Sprintf("step_%d_%s", i+1, step.Type.String())
		jobDefConfig[stepKey] = step.Config
	}
	jobDefConfig["job_definition_id"] = jobDef.ID
	jobDefConfig["source_type"] = jobDef.SourceType
	jobDefConfig["base_url"] = jobDef.BaseURL
	jobDefConfig["schedule"] = jobDef.Schedule
	jobDefConfig["timeout"] = jobDef.Timeout
	jobDefConfig["enabled"] = jobDef.Enabled
	if jobDef.AuthID != "" {
		jobDefConfig["auth_id"] = jobDef.AuthID
	}

	if err := m.UpdateJobConfig(ctx, managerID, jobDefConfig); err != nil {
		// Log warning but continue
	}

	// Build step_definitions for UI display
	// This allows the queue UI to show step progress even before steps start executing
	stepDefs := make([]map[string]interface{}, len(jobDef.Steps))
	for i, step := range jobDef.Steps {
		stepDefs[i] = map[string]interface{}{
			"name":        step.Name,
			"type":        step.Type.String(),
			"description": step.Description,
		}
	}
	initialMetadata := map[string]interface{}{
		"step_definitions": stepDefs,
		"total_steps":      len(jobDef.Steps),
		"current_step":     0,
	}
	if err := m.UpdateJobMetadata(ctx, managerID, initialMetadata); err != nil {
		// Log warning but continue
	}

	// Mark manager job as running
	if err := m.UpdateJobStatus(ctx, managerID, "running"); err != nil {
		// Log warning but continue
	}

	// Track if any steps have child jobs
	hasChildJobs := false

	// Track validation errors that were skipped due to on_error="continue"
	var lastValidationError string

	// Track per-step statistics for UI display
	// Store cumulative child counts at step completion to calculate per-step deltas
	stepStats := make([]map[string]interface{}, len(jobDef.Steps))

	// Track step job IDs for monitoring (map step name -> step job ID)
	stepJobIDs := make(map[string]string, len(jobDef.Steps))

	// Execute steps sequentially
	for i, step := range jobDef.Steps {
		// Create step job (child of manager, parent of spawned jobs)
		stepID := uuid.New().String()
		stepJobIDs[step.Name] = stepID

		stepConfig := make(map[string]interface{})
		for k, v := range step.Config {
			stepConfig[k] = v
		}
		stepConfig["step_index"] = i
		stepConfig["step_name"] = step.Name
		stepConfig["step_type"] = step.Type.String()

		stepJob := &Job{
			ID:              stepID,
			ParentID:        &managerID,
			Type:            string(models.JobTypeStep),
			Name:            step.Name,
			Phase:           "execution",
			Status:          "pending",
			CreatedAt:       time.Now(),
			ProgressCurrent: 0,
			ProgressTotal:   0, // Will be updated when jobs are created
		}

		if err := m.CreateJobRecord(ctx, stepJob); err != nil {
			m.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Failed to create step job: %v", err))
			continue
		}

		// Store step metadata
		stepJobMetadata := map[string]interface{}{
			"manager_id":  managerID,
			"step_index":  i,
			"step_name":   step.Name,
			"step_type":   step.Type.String(),
			"description": step.Description,
		}
		if err := m.UpdateJobMetadata(ctx, stepID, stepJobMetadata); err != nil {
			// Log but continue
		}

		// Mark step as running
		if err := m.UpdateJobStatus(ctx, stepID, "running"); err != nil {
			// Log but continue
		}

		// Get document count BEFORE step execution (for delta calculation)
		docCountBefore, _ := m.GetDocumentCount(ctx, managerID)

		// Update manager metadata with current step info (persisted for UI on page reload)
		managerStepMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": "running",
			"current_step_id":     stepID,
			"total_steps":         len(jobDef.Steps),
		}
		if err := m.UpdateJobMetadata(ctx, managerID, managerStepMetadata); err != nil {
			// Log but continue
		}

		// Publish step starting event for WebSocket clients
		if m.eventService != nil {
			payload := map[string]interface{}{
				"job_id":       managerID,
				"step_id":      stepID,
				"job_name":     jobDef.Name,
				"step_index":   i,
				"step_name":    step.Name,
				"step_type":    step.Type.String(),
				"current_step": i + 1,
				"total_steps":  len(jobDef.Steps),
				"step_status":  "running",
				"timestamp":    time.Now().Format(time.RFC3339),
			}
			event := interfaces.Event{
				Type:    interfaces.EventJobProgress,
				Payload: payload,
			}
			// Publish asynchronously
			go func() {
				if err := m.eventService.Publish(ctx, event); err != nil {
					// Log but don't fail
				}
			}()
		}

		// Resolve placeholders in step config
		resolvedStep := step
		if step.Config != nil && m.kvStorage != nil {
			resolvedStep.Config = m.resolvePlaceholders(ctx, step.Config)
		}

		// Get worker for this step type
		worker := m.GetWorker(step.Type)
		if worker == nil {
			err := fmt.Errorf("no worker registered for step type: %s", step.Type.String())
			m.AddJobLog(ctx, managerID, "error", err.Error())
			m.AddJobLog(ctx, stepID, "error", err.Error())
			m.UpdateJobStatus(ctx, stepID, "failed")

			// Handle based on error strategy
			if step.OnError == models.ErrorStrategyFail {
				m.SetJobError(ctx, managerID, err.Error())
				return managerID, err
			}
			continue
		}

		// Validate step configuration
		if err := worker.ValidateConfig(resolvedStep); err != nil {
			m.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Step validation failed: %v", err))
			m.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Validation failed: %v", err))
			m.UpdateJobStatus(ctx, stepID, "failed")

			if step.OnError == models.ErrorStrategyFail {
				m.SetJobError(ctx, managerID, err.Error())
				return managerID, fmt.Errorf("step %s validation failed: %w", step.Name, err)
			}
			// Track last validation error for final status check
			lastValidationError = fmt.Sprintf("Step %s validation failed: %v", step.Name, err)
			continue
		}

		// Execute step via worker
		// Pass stepID so worker creates jobs under the step (not manager)
		childJobID, err := worker.CreateJobs(ctx, resolvedStep, *jobDef, stepID)
		if err != nil {
			m.AddJobLog(ctx, managerID, "error", fmt.Sprintf("Step %s failed: %v", step.Name, err))
			m.AddJobLog(ctx, stepID, "error", fmt.Sprintf("Failed: %v", err))
			m.SetJobError(ctx, managerID, err.Error())
			m.UpdateJobStatus(ctx, stepID, "failed")

			if step.OnError == models.ErrorStrategyFail {
				return managerID, fmt.Errorf("step %s failed: %w", step.Name, err)
			}

			// Check error tolerance
			if jobDef.ErrorTolerance != nil {
				shouldStop, _ := m.checkErrorTolerance(ctx, managerID, jobDef.ErrorTolerance)
				if shouldStop {
					m.UpdateJobStatus(ctx, managerID, "failed")
					return managerID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			continue
		}

		// Track child jobs for this step
		if worker.ReturnsChildJobs() {
			hasChildJobs = true
			m.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step %s spawned child jobs", step.Name))
			m.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Spawned child jobs (job: %s)", childJobID))
		} else {
			m.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step %s completed", step.Name))
			m.AddJobLog(ctx, stepID, "info", fmt.Sprintf("Completed (job: %s)", childJobID))
		}

		// Update manager progress
		if err := m.UpdateJobProgress(ctx, managerID, i+1, len(jobDef.Steps)); err != nil {
			// Log warning but continue
		}

		// Get child stats for this step (jobs under step, not manager)
		var stepChildCount int
		if stats, err := m.jobStorage.GetJobChildStats(ctx, []string{stepID}); err == nil {
			if s := stats[stepID]; s != nil {
				stepChildCount = s.ChildCount
			}
		}

		// Get document count AFTER step execution
		docCountAfter, _ := m.GetDocumentCount(ctx, managerID)

		// Calculate documents created by this step (delta)
		stepDocCount := docCountAfter - docCountBefore

		// Store step statistics for UI
		stepStats[i] = map[string]interface{}{
			"step_index":     i,
			"step_id":        stepID,
			"step_name":      step.Name,
			"step_type":      step.Type.String(),
			"child_count":    stepChildCount,
			"document_count": stepDocCount,
		}

		// Determine step status based on whether it has child jobs
		// If step creates child jobs, mark as "spawned" (not completed until children finish)
		// If step doesn't create children, mark as "completed"
		stepStatus := "completed"
		m.logger.Debug().
			Str("step_id", stepID).
			Bool("returns_child_jobs", worker.ReturnsChildJobs()).
			Int("step_child_count", stepChildCount).
			Bool("step_monitor_nil", stepMonitor == nil).
			Msg("Determining step status for step monitor")
		// Log to job events for visibility
		m.AddJobLog(ctx, managerID, "info", fmt.Sprintf("Step status check: returns_child_jobs=%v, step_child_count=%d, step_monitor_nil=%v",
			worker.ReturnsChildJobs(), stepChildCount, stepMonitor == nil))
		if worker.ReturnsChildJobs() && stepChildCount > 0 {
			stepStatus = "spawned" // Children created but not yet finished
		}

		// Update step job status
		if stepStatus == "completed" {
			m.UpdateJobStatus(ctx, stepID, "completed")
			m.SetJobFinished(ctx, stepID)
		} else if stepStatus == "spawned" && stepMonitor != nil {
			// Start StepMonitor for this step - it will monitor children and mark step complete
			stepQueueJob := &models.QueueJob{
				ID:        stepID,
				ParentID:  &managerID,
				ManagerID: &managerID,
				Type:      string(models.JobTypeStep),
				Name:      step.Name,
				Config:    stepConfig,
				Metadata:  stepJobMetadata,
				CreatedAt: time.Now(),
				Depth:     1,
			}
			stepMonitor.StartMonitoring(ctx, stepQueueJob)
			m.AddJobLog(ctx, stepID, "info", "Step monitor started for spawned children")
		}

		// Update manager metadata with step progress
		managerCompletedMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": stepStatus,
			"current_step_id":     stepID,
			"completed_steps":     i + 1,
			"step_stats":          stepStats[:i+1], // Include all completed step stats
		}
		if err := m.UpdateJobMetadata(ctx, managerID, managerCompletedMetadata); err != nil {
			// Log but continue
		}

		// Publish step progress event for WebSocket clients
		if m.eventService != nil {
			payload := map[string]interface{}{
				"job_id":           managerID,
				"step_id":          stepID,
				"job_name":         jobDef.Name,
				"step_index":       i,
				"step_name":        step.Name,
				"step_type":        step.Type.String(),
				"current_step":     i + 1,
				"total_steps":      len(jobDef.Steps),
				"step_status":      stepStatus, // "spawned" if children created, "completed" otherwise
				"step_child_count": stepChildCount,
				"timestamp":        time.Now().Format(time.RFC3339),
			}
			event := interfaces.Event{
				Type:    interfaces.EventJobProgress,
				Payload: payload,
			}
			// Publish asynchronously to avoid blocking step execution
			go func() {
				if err := m.eventService.Publish(ctx, event); err != nil {
					// Log but don't fail
				}
			}()
		}
	}

	// Handle completion based on whether steps have child jobs
	if hasChildJobs && jobMonitor != nil {
		// Start monitoring manager job (monitors steps which monitor their children)
		m.AddJobLog(ctx, managerID, "info", "Steps have child jobs - starting manager job monitoring")

		// Store step IDs in manager metadata for monitoring
		stepIDsMetadata := map[string]interface{}{
			"step_job_ids": stepJobIDs,
		}
		if err := m.UpdateJobMetadata(ctx, managerID, stepIDsMetadata); err != nil {
			// Log but continue
		}

		managerQueueJob := &models.QueueJob{
			ID:        managerID,
			ParentID:  nil,
			ManagerID: nil, // Manager has no manager
			Type:      string(models.JobTypeManager),
			Name:      jobDef.Name,
			Config:    jobDefConfig,
			Metadata:  managerMetadata,
			CreatedAt: time.Now(),
			Depth:     0,
		}

		jobMonitor.StartMonitoring(ctx, managerQueueJob)
	} else {
		// Check if validation errors occurred with no children created
		if lastValidationError != "" {
			// Mark as failed when validation failed and no children were created
			m.AddJobLog(ctx, managerID, "error", "Job failed: "+lastValidationError)
			m.SetJobError(ctx, managerID, lastValidationError)
			m.UpdateJobStatus(ctx, managerID, "failed")
			m.SetJobFinished(ctx, managerID)
		} else {
			// Mark as completed immediately for jobs without children
			m.AddJobLog(ctx, managerID, "info", "Job completed (no child jobs)")
			m.UpdateJobStatus(ctx, managerID, "completed")
			m.SetJobFinished(ctx, managerID)
		}
	}

	return managerID, nil
}
*/

// resolvePlaceholders recursively resolves {key-name} placeholders in step config values

// resolveValue recursively resolves placeholders in a single value

// checkErrorTolerance checks if the error tolerance threshold has been exceeded
