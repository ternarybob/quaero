package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Manager handles job metadata, lifecycle, and worker routing.
// This is the single manager for the queue system - it routes job definition steps to workers.
type Manager struct {
	jobStorage    interfaces.QueueStorage
	jobLogStorage interfaces.JobLogStorage
	queue         interfaces.QueueManager
	eventService  interfaces.EventService // Optional: may be nil for testing

	// Worker registry for definition worker routing
	workers   map[models.WorkerType]interfaces.DefinitionWorker
	kvStorage interfaces.KeyValueStorage // For resolving {key-name} placeholders
}

func NewManager(jobStorage interfaces.QueueStorage, jobLogStorage interfaces.JobLogStorage, queue interfaces.QueueManager, eventService interfaces.EventService) *Manager {
	return &Manager{
		jobStorage:    jobStorage,
		jobLogStorage: jobLogStorage,
		queue:         queue,
		eventService:  eventService,
		workers:       make(map[models.WorkerType]interfaces.DefinitionWorker),
	}
}

// SetKVStorage sets the key-value storage for resolving placeholders in step configs.
// This should be called during initialization if placeholder resolution is needed.
func (m *Manager) SetKVStorage(kvStorage interfaces.KeyValueStorage) {
	m.kvStorage = kvStorage
}

// RegisterWorker registers a DefinitionWorker for its declared WorkerType.
// If a worker for the same type is already registered, it will be replaced.
func (m *Manager) RegisterWorker(worker interfaces.DefinitionWorker) {
	if worker == nil {
		return
	}
	m.workers[worker.GetType()] = worker
}

// HasWorker checks if a worker is registered for the given WorkerType.
func (m *Manager) HasWorker(workerType models.WorkerType) bool {
	_, exists := m.workers[workerType]
	return exists
}

// GetWorker returns the worker registered for the given WorkerType, or nil if not found.
func (m *Manager) GetWorker(workerType models.WorkerType) interfaces.DefinitionWorker {
	return m.workers[workerType]
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
	entries, err := m.jobLogStorage.GetLogs(ctx, jobID, limit)
	if err != nil {
		return nil, err
	}

	var logs []JobLog
	for _, entry := range entries {
		ts, _ := time.Parse(time.RFC3339, entry.FullTimestamp)
		logs = append(logs, JobLog{
			JobID:     entry.AssociatedJobID,
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
	// Get job details before update to access parent_id and job_type
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	jobState, ok := jobEntityInterface.(*models.QueueJobState)
	if !ok {
		return fmt.Errorf("invalid job type")
	}

	// Update status via storage interface
	if err := m.jobStorage.UpdateJobStatus(ctx, jobID, status, ""); err != nil {
		return err
	}

	// Add job log for status change
	logMessage := fmt.Sprintf("Status changed: %s", status)
	if err := m.AddJobLog(ctx, jobID, "info", logMessage); err != nil {
		// Log error but don't fail the status update (logging is non-critical)
	}

	// Publish job status change event for parent job monitoring
	// Only publish if eventService is available (optional dependency)
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
		}()
	}

	return nil
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

// AddJobLog adds a log entry for a job
func (m *Manager) AddJobLog(ctx context.Context, jobID, level, message string) error {
	now := time.Now()
	entry := models.JobLogEntry{
		AssociatedJobID: jobID,
		Timestamp:       now.Format("15:04:05"),
		FullTimestamp:   now.Format(time.RFC3339),
		Level:           level,
		Message:         message,
	}
	return m.jobLogStorage.AppendLog(ctx, jobID, entry)
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
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	return m.jobStorage.GetJobChildStats(ctx, parentIDs)
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

// StopAllChildJobs cancels all running child jobs of the specified parent job
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
		if child.Status == models.JobStatusRunning || child.Status == models.JobStatusPending {
			if err := m.jobStorage.UpdateJobStatus(ctx, child.ID, string(models.JobStatusCancelled), ""); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// ============================================================================
// Job Definition Execution (Step Orchestration)
// ============================================================================

// ExecuteJobDefinition executes a job definition by routing steps to workers.
// Creates a parent job record and orchestrates step execution sequentially.
// Returns the parent job ID for tracking.
func (m *Manager) ExecuteJobDefinition(ctx context.Context, jobDef *models.JobDefinition, jobMonitor interfaces.JobMonitor) (string, error) {
	// Generate parent job ID
	parentJobID := uuid.New().String()

	// Create parent job record in database
	parentJob := &Job{
		ID:              parentJobID,
		ParentID:        nil,
		Type:            "parent",
		Name:            jobDef.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       time.Now(),
		ProgressCurrent: 0,
		ProgressTotal:   len(jobDef.Steps),
	}

	if err := m.CreateJobRecord(ctx, parentJob); err != nil {
		return "", fmt.Errorf("failed to create parent job: %w", err)
	}

	// Persist metadata immediately after job creation
	parentMetadata := make(map[string]interface{})
	if jobDef.AuthID != "" {
		parentMetadata["auth_id"] = jobDef.AuthID
	}
	if jobDef.ID != "" {
		parentMetadata["job_definition_id"] = jobDef.ID
	}
	parentMetadata["phase"] = "execution"

	if err := m.UpdateJobMetadata(ctx, parentJobID, parentMetadata); err != nil {
		// Log warning but continue
	}

	// Add initial job log
	initialLog := fmt.Sprintf("Starting job definition execution: %s (ID: %s, Steps: %d)",
		jobDef.Name, jobDef.ID, len(jobDef.Steps))
	m.AddJobLog(ctx, parentJobID, "info", initialLog)

	// Build job definition config for parent job
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

	if err := m.UpdateJobConfig(ctx, parentJobID, jobDefConfig); err != nil {
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
	if err := m.UpdateJobMetadata(ctx, parentJobID, initialMetadata); err != nil {
		// Log warning but continue
	}

	// Mark parent job as running
	if err := m.UpdateJobStatus(ctx, parentJobID, "running"); err != nil {
		// Log warning but continue
	}

	// Track if any child jobs were created
	hasChildJobs := false

	// Track per-step statistics for UI display
	// Store cumulative child counts at step completion to calculate per-step deltas
	stepStats := make([]map[string]interface{}, len(jobDef.Steps))
	var prevChildCount int

	// Execute steps sequentially
	for i, step := range jobDef.Steps {
		// Get child stats BEFORE step execution (for delta calculation after completion)
		var childCountBefore int
		if stats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID}); err == nil {
			if s := stats[parentJobID]; s != nil {
				childCountBefore = s.ChildCount
			}
		}

		// Update job metadata with current step info (persisted for UI on page reload)
		stepMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": "running",
			"total_steps":         len(jobDef.Steps),
		}
		if err := m.UpdateJobMetadata(ctx, parentJobID, stepMetadata); err != nil {
			// Log but continue
		}

		// Publish step starting event for WebSocket clients
		if m.eventService != nil {
			payload := map[string]interface{}{
				"job_id":       parentJobID,
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
			m.AddJobLog(ctx, parentJobID, "error", err.Error())

			// Handle based on error strategy
			if step.OnError == models.ErrorStrategyFail {
				m.SetJobError(ctx, parentJobID, err.Error())
				return parentJobID, err
			}
			continue
		}

		// Validate step configuration
		if err := worker.ValidateConfig(resolvedStep); err != nil {
			m.AddJobLog(ctx, parentJobID, "error", fmt.Sprintf("Step validation failed: %v", err))

			if step.OnError == models.ErrorStrategyFail {
				m.SetJobError(ctx, parentJobID, err.Error())
				return parentJobID, fmt.Errorf("step %s validation failed: %w", step.Name, err)
			}
			continue
		}

		// Execute step via worker
		childJobID, err := worker.CreateJobs(ctx, resolvedStep, *jobDef, parentJobID)
		if err != nil {
			m.AddJobLog(ctx, parentJobID, "error", fmt.Sprintf("Step %s failed: %v", step.Name, err))
			m.SetJobError(ctx, parentJobID, err.Error())

			if step.OnError == models.ErrorStrategyFail {
				return parentJobID, fmt.Errorf("step %s failed: %w", step.Name, err)
			}

			// Check error tolerance
			if jobDef.ErrorTolerance != nil {
				shouldStop, _ := m.checkErrorTolerance(ctx, parentJobID, jobDef.ErrorTolerance)
				if shouldStop {
					m.UpdateJobStatus(ctx, parentJobID, "failed")
					return parentJobID, fmt.Errorf("execution stopped: error tolerance threshold exceeded")
				}
			}
			continue
		}

		// Track child jobs
		if worker.ReturnsChildJobs() {
			hasChildJobs = true
		}

		m.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Step %s completed (job: %s)", step.Name, childJobID))

		// Update progress
		if err := m.UpdateJobProgress(ctx, parentJobID, i+1, len(jobDef.Steps)); err != nil {
			// Log warning but continue
		}

		// Get child stats AFTER step execution (for per-step statistics)
		var childCountAfter int
		if stats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID}); err == nil {
			if s := stats[parentJobID]; s != nil {
				childCountAfter = s.ChildCount
			}
		}

		// Calculate children created by this step (delta)
		stepChildCount := childCountAfter - childCountBefore

		// Store step statistics for UI
		stepStats[i] = map[string]interface{}{
			"step_index":       i,
			"step_name":        step.Name,
			"step_type":        step.Type.String(),
			"child_count":      stepChildCount,
			"cumulative_count": childCountAfter,
		}

		// Update metadata with completed step status and step statistics
		completedStepMetadata := map[string]interface{}{
			"current_step":        i + 1,
			"current_step_name":   step.Name,
			"current_step_type":   step.Type.String(),
			"current_step_status": "completed",
			"completed_steps":     i + 1,
			"step_stats":          stepStats[:i+1], // Include all completed step stats
		}
		if err := m.UpdateJobMetadata(ctx, parentJobID, completedStepMetadata); err != nil {
			// Log but continue
		}

		// Track cumulative count for next iteration
		prevChildCount = childCountAfter
		_ = prevChildCount // Used in next iteration

		// Publish step progress event for WebSocket clients
		if m.eventService != nil {
			payload := map[string]interface{}{
				"job_id":           parentJobID,
				"job_name":         jobDef.Name,
				"step_index":       i,
				"step_name":        step.Name,
				"step_type":        step.Type.String(),
				"current_step":     i + 1,
				"total_steps":      len(jobDef.Steps),
				"step_status":      "completed",
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

	// Handle completion based on job type
	if hasChildJobs && jobMonitor != nil {
		// Start monitoring for jobs with children
		m.AddJobLog(ctx, parentJobID, "info", "Child jobs detected - starting parent job monitoring")

		parentQueueJob := &models.QueueJob{
			ID:        parentJobID,
			ParentID:  nil,
			Type:      "parent",
			Name:      jobDef.Name,
			Config:    jobDefConfig,
			Metadata:  parentMetadata,
			CreatedAt: time.Now(),
			Depth:     0,
		}

		jobMonitor.StartMonitoring(ctx, parentQueueJob)
	} else {
		// Mark as completed immediately for jobs without children
		m.AddJobLog(ctx, parentJobID, "info", "Job completed (no child jobs)")
		m.UpdateJobStatus(ctx, parentJobID, "completed")
		m.SetJobFinished(ctx, parentJobID)
	}

	return parentJobID, nil
}

// resolvePlaceholders recursively resolves {key-name} placeholders in step config values
func (m *Manager) resolvePlaceholders(ctx context.Context, config map[string]interface{}) map[string]interface{} {
	if config == nil || m.kvStorage == nil {
		return config
	}

	resolved := make(map[string]interface{})
	for key, value := range config {
		resolved[key] = m.resolveValue(ctx, value)
	}
	return resolved
}

// resolveValue recursively resolves placeholders in a single value
func (m *Manager) resolveValue(ctx context.Context, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if len(v) > 2 && v[0] == '{' && v[len(v)-1] == '}' {
			keyName := v[1 : len(v)-1]
			kvValue, err := m.kvStorage.Get(ctx, keyName)
			if err == nil && kvValue != "" {
				return kvValue
			}
		}
		return v
	case map[string]interface{}:
		return m.resolvePlaceholders(ctx, v)
	case []interface{}:
		resolved := make([]interface{}, len(v))
		for i, item := range v {
			resolved[i] = m.resolveValue(ctx, item)
		}
		return resolved
	default:
		return v
	}
}

// checkErrorTolerance checks if the error tolerance threshold has been exceeded
func (m *Manager) checkErrorTolerance(ctx context.Context, parentJobID string, tolerance *models.ErrorTolerance) (bool, error) {
	if tolerance == nil || tolerance.MaxChildFailures == 0 {
		return false, nil
	}

	failedCount, err := m.GetFailedChildCount(ctx, parentJobID)
	if err != nil {
		return false, fmt.Errorf("failed to query failed job count: %w", err)
	}

	if failedCount >= tolerance.MaxChildFailures {
		switch tolerance.FailureAction {
		case "stop_all":
			return true, nil
		default:
			return false, nil
		}
	}

	return false, nil
}
