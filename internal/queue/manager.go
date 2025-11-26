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

// Manager handles job metadata and lifecycle.
type Manager struct {
	jobStorage    interfaces.QueueStorage
	jobLogStorage interfaces.JobLogStorage
	queue         interfaces.QueueManager
	eventService  interfaces.EventService // Optional: may be nil for testing
}

func NewManager(jobStorage interfaces.QueueStorage, jobLogStorage interfaces.JobLogStorage, queue interfaces.QueueManager, eventService interfaces.EventService) *Manager {
	return &Manager{
		jobStorage:    jobStorage,
		jobLogStorage: jobLogStorage,
		queue:         queue,
		eventService:  eventService,
	}
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

		event := interfaces.Event{
			Type:    interfaces.EventJobStatusChange,
			Payload: payload,
		}

		// Publish asynchronously to avoid blocking status updates
		go func() {
			if err := m.eventService.Publish(ctx, event); err != nil {
				// Log error but don't fail the status update
				// EventService will handle logging via its subscribers
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
