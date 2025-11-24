package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// Manager handles job metadata and lifecycle.
type Manager struct {
	jobStorage    interfaces.JobStorage
	jobLogStorage interfaces.JobLogStorage
	queue         interfaces.QueueManager
	eventService  interfaces.EventService // Optional: may be nil for testing
}

func NewManager(jobStorage interfaces.JobStorage, jobLogStorage interfaces.JobLogStorage, queue interfaces.QueueManager, eventService interfaces.EventService) *Manager {
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
	// Convert internal Job to models.JobModel for storage
	metadata := map[string]interface{}{
		"phase": job.Phase,
	}

	config := make(map[string]interface{})

	jobModel := &models.JobModel{
		ID:        job.ID,
		Type:      job.Type,
		Name:      job.Name,
		CreatedAt: job.CreatedAt,
		Config:    config,
		Metadata:  metadata,
	}

	if job.ParentID != nil {
		jobModel.ParentID = job.ParentID
	}

	// Create job record using storage interface
	jobEntity := models.NewJob(jobModel)
	jobEntity.Status = models.JobStatus(job.Status)

	if job.ProgressTotal > 0 {
		jobEntity.UpdateProgress(job.ProgressCurrent, 0, 0, job.ProgressTotal)
	}

	if err := m.jobStorage.SaveJob(ctx, jobEntity); err != nil {
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

	jobModel := models.NewJobModel(jobType, "", config, metadata)
	jobModel.ID = jobID

	jobEntity := models.NewJob(jobModel)
	jobEntity.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobEntity); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
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

	jobModel := models.NewJobModel(jobType, "", config, metadata)
	jobModel.ID = jobID
	jobModel.ParentID = &parentID

	jobEntity := models.NewJob(jobModel)
	jobEntity.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobEntity); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
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

	jobEntity, ok := jobEntityInterface.(*models.Job)
	if !ok {
		return nil, fmt.Errorf("invalid job type from storage")
	}

	job := &Job{
		ID:          jobEntity.ID,
		Type:        jobEntity.Type,
		Name:        jobEntity.Name,
		Status:      string(jobEntity.Status),
		CreatedAt:   jobEntity.CreatedAt,
		StartedAt:   jobEntity.StartedAt,
		CompletedAt: jobEntity.CompletedAt,
		FinishedAt:  jobEntity.FinishedAt,
		ParentID:    jobEntity.ParentID,
	}

	if jobEntity.Error != "" {
		job.Error = &jobEntity.Error
	}

	// Map config to payload
	if configJSON, err := json.Marshal(jobEntity.Config); err == nil {
		job.Payload = string(configJSON)
	}

	// Map metadata fields
	if phase, ok := jobEntity.Metadata["phase"].(string); ok {
		job.Phase = phase
	}
	if result, ok := jobEntity.Metadata["result"].(string); ok {
		job.Result = result
	}

	// Map progress (Progress is now a value type, always present)
	job.ProgressCurrent = jobEntity.Progress.CompletedURLs
	job.ProgressTotal = jobEntity.Progress.TotalURLs

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

// UpdateJobStatus updates the job status
func (m *Manager) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	// Get job details before update to access parent_id and job_type
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	jobEntity, ok := jobEntityInterface.(*models.Job)
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
			"job_type":  jobEntity.Type,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		// Include parent_id if this is a child job
		if jobEntity.ParentID != nil {
			payload["parent_id"] = *jobEntity.ParentID
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

// SetJobError sets job error message and marks as failed
func (m *Manager) SetJobError(ctx context.Context, jobID string, errorMsg string) error {
	return m.jobStorage.UpdateJobStatus(ctx, jobID, string(models.JobStatusFailed), errorMsg)
}

// SetJobResult sets job result data
func (m *Manager) SetJobResult(ctx context.Context, jobID string, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	// Get job to update metadata
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		jobEntity.Metadata = make(map[string]interface{})
	}

	jobEntity.Metadata["result"] = string(resultJSON)

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}

// IncrementDocumentCount increments the document_count in job metadata
// This is used to track the number of documents saved by child jobs for a parent job
func (m *Manager) IncrementDocumentCount(ctx context.Context, jobID string) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		jobEntity.Metadata = make(map[string]interface{})
	}

	// Increment document_count
	currentCount := 0
	if count, ok := jobEntity.Metadata["document_count"].(float64); ok {
		currentCount = int(count)
	} else if count, ok := jobEntity.Metadata["document_count"].(int); ok {
		currentCount = count
	}
	jobEntity.Metadata["document_count"] = currentCount + 1

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}

// SetJobFinished sets the finished_at timestamp for a job
// This should be called when a job AND all its spawned children complete or timeout
func (m *Manager) SetJobFinished(ctx context.Context, jobID string) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	now := time.Now()
	jobEntity.FinishedAt = &now

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}

// UpdateJobConfig updates the job configuration in the database
func (m *Manager) UpdateJobConfig(ctx context.Context, jobID string, config map[string]interface{}) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	jobEntity.Config = config

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}

// UpdateJobMetadata updates the job metadata in the database
// This method merges new metadata with existing metadata to preserve fields like phase
func (m *Manager) UpdateJobMetadata(ctx context.Context, jobID string, metadata map[string]interface{}) error {
	// Get job
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		jobEntity.Metadata = make(map[string]interface{})
	}

	// Merge metadata
	for k, v := range metadata {
		jobEntity.Metadata[k] = v
	}

	return m.jobStorage.UpdateJob(ctx, jobEntity)
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

// JobTreeStatus represents aggregated status for a job tree (parent + children)
type JobTreeStatus struct {
	ParentJob       *Job    `json:"parent_job"`
	TotalChildren   int     `json:"total_children"`
	CompletedCount  int     `json:"completed_count"`
	FailedCount     int     `json:"failed_count"`
	RunningCount    int     `json:"running_count"`
	PendingCount    int     `json:"pending_count"`
	CancelledCount  int     `json:"cancelled_count"`
	OverallProgress float64 `json:"overall_progress"`            // 0.0 to 1.0
	EstimatedTime   *int64  `json:"estimated_time_ms,omitempty"` // Estimated milliseconds to completion
}

// GetJobTreeStatus retrieves aggregated status for a parent job and all its children
// This provides efficient status reporting for hierarchical job execution
func (m *Manager) GetJobTreeStatus(ctx context.Context, parentJobID string) (*JobTreeStatus, error) {
	// Get parent job using internal method
	parentJobInternal, err := m.GetJobInternal(ctx, parentJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent job: %w", err)
	}

	// Aggregate child job statuses
	childStats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate child statuses: %w", err)
	}

	stats := childStats[parentJobID]
	if stats == nil {
		stats = &interfaces.JobChildStats{}
	}

	// Calculate overall progress
	// Progress based on completed + failed (terminal states) vs total
	var overallProgress float64
	if stats.ChildCount > 0 {
		terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
		overallProgress = float64(terminalCount) / float64(stats.ChildCount)
	} else {
		// No children yet, use parent job progress if available
		if parentJobInternal.ProgressTotal > 0 {
			overallProgress = float64(parentJobInternal.ProgressCurrent) / float64(parentJobInternal.ProgressTotal)
		}
	}

	// Estimate time to completion (simple linear extrapolation)
	var estimatedTime *int64
	if stats.RunningChildren > 0 && parentJobInternal.StartedAt != nil {
		elapsed := time.Since(*parentJobInternal.StartedAt)
		if overallProgress > 0 && overallProgress < 1.0 {
			totalEstimated := float64(elapsed) / overallProgress
			remaining := totalEstimated - float64(elapsed)
			remainingMS := int64(time.Duration(remaining) / time.Millisecond)
			estimatedTime = &remainingMS
		}
	}

	status := &JobTreeStatus{
		ParentJob:       parentJobInternal,
		TotalChildren:   stats.ChildCount,
		CompletedCount:  stats.CompletedChildren,
		FailedCount:     stats.FailedChildren,
		RunningCount:    stats.RunningChildren,
		PendingCount:    stats.PendingChildren,
		CancelledCount:  stats.CancelledChildren,
		OverallProgress: overallProgress,
		EstimatedTime:   estimatedTime,
	}

	return status, nil
}

// GetFailedChildCount returns the count of failed child jobs for a parent job
func (m *Manager) GetFailedChildCount(ctx context.Context, parentJobID string) (int, error) {
	childStats, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return 0, err
	}

	if stats, ok := childStats[parentJobID]; ok {
		return stats.FailedChildren, nil
	}

	return 0, nil
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

	jobModel := models.NewJobModel(jobType, name, config, metadata)

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create job record in storage
	jobEntity := models.NewJob(jobModel)
	jobEntity.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, jobEntity); err != nil {
		return "", fmt.Errorf("create job record: %w", err)
	}

	// Enqueue the job
	if err := m.queue.Enqueue(ctx, queue.Message{
		JobID:   jobModel.ID,
		Type:    jobType,
		Payload: configJSON,
	}); err != nil {
		return "", fmt.Errorf("enqueue job: %w", err)
	}

	return jobModel.ID, nil
}

// GetJob implements interfaces.JobManager.GetJob
// Returns interface{} to match the interface, but the actual type is *models.Job
func (m *Manager) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	return m.jobStorage.GetJob(ctx, jobID)
}

// ListJobs implements interfaces.JobManager.ListJobs
// Returns []*models.Job directly from storage
func (m *Manager) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.Job, error) {
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
	jobEntity := jobEntityInterface.(*models.Job)

	// Create new job with same configuration
	newJobID := uuid.New().String()

	// Clone job model
	newJobModel := jobEntity.ToJobModel().Clone()
	newJobModel.ID = newJobID
	newJobModel.Name = jobEntity.Name + " (Copy)"
	newJobModel.CreatedAt = time.Now()

	newJobEntity := models.NewJob(newJobModel)
	newJobEntity.Status = models.JobStatusPending

	if err := m.jobStorage.SaveJob(ctx, newJobEntity); err != nil {
		return "", fmt.Errorf("failed to create job copy: %w", err)
	}

	return newJobID, nil
}

// GetJobChildStats implements interfaces.JobManager.GetJobChildStats
func (m *Manager) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	return m.jobStorage.GetJobChildStats(ctx, parentIDs)
}

// StopAllChildJobs implements interfaces.JobManager.StopAllChildJobs
func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error) {
	// Get running/pending child jobs
	// This is inefficient without a direct update query, but Badger doesn't support SQL updates
	// We have to list and update individually

	// Get all children
	// Note: GetChildJobs returns []*models.JobModel, but we need status which is in models.Job
	// We need to fix the interface or use ListJobs with filter.
	// Assuming GetChildJobs returns JobModel which doesn't have status.
	// We need to fetch each job to check status.
	// Or use ListJobs with ParentID filter if supported.

	// Let's use ListJobs with ParentID filter
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
	}

	// ListJobs returns []*models.JobModel in interface, but we know it's broken.
	// We will use m.ListJobs which returns []*models.Job (our wrapper).

	children, err := m.ListJobs(ctx, opts)
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
// Real-Time Progress Tracking Methods (Task 4.1)
// ============================================================================

// CrawlerProgressStats represents comprehensive progress statistics for crawler jobs
type CrawlerProgressStats struct {
	// Basic job information
	JobID    string `json:"job_id"`
	ParentID string `json:"parent_id,omitempty"`
	Status   string `json:"status"`
	JobType  string `json:"job_type"`

	// Child job statistics
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
	CancelledChildren int `json:"cancelled_children"`

	// Progress calculation
	OverallProgress float64 `json:"overall_progress"` // 0.0 to 1.0
	ProgressText    string  `json:"progress_text"`    // Human-readable progress

	// Link following statistics (crawler-specific)
	LinksFound    int `json:"links_found"`
	LinksFiltered int `json:"links_filtered"`
	LinksFollowed int `json:"links_followed"`
	LinksSkipped  int `json:"links_skipped"`

	// Timing information
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EstimatedEnd *time.Time `json:"estimated_end,omitempty"`
	Duration     *float64   `json:"duration_seconds,omitempty"`

	// Error information
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GetCrawlerProgressStats retrieves comprehensive progress statistics for a crawler job
// This method calculates parent job progress from child job statistics and includes
// link following metrics for real-time monitoring
func (m *Manager) GetCrawlerProgressStats(ctx context.Context, jobID string) (*CrawlerProgressStats, error) {
	// Get the job details
	job, err := m.GetJobInternal(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	stats := &CrawlerProgressStats{
		JobID:     job.ID,
		Status:    job.Status,
		JobType:   job.Type,
		StartedAt: job.StartedAt,
	}

	if job.ParentID != nil {
		stats.ParentID = *job.ParentID
	}

	// Calculate duration if job has started
	if job.StartedAt != nil {
		var endTime time.Time
		if job.CompletedAt != nil {
			endTime = *job.CompletedAt
		} else {
			endTime = time.Now()
		}
		duration := endTime.Sub(*job.StartedAt).Seconds()
		stats.Duration = &duration
	}

	// Get child job statistics
	childStats, err := m.getChildJobStatistics(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child statistics: %w", err)
	}

	stats.TotalChildren = childStats.TotalChildren
	stats.CompletedChildren = childStats.CompletedChildren
	stats.FailedChildren = childStats.FailedChildren
	stats.RunningChildren = childStats.RunningChildren
	stats.PendingChildren = childStats.PendingChildren
	stats.CancelledChildren = childStats.CancelledChildren

	// Calculate overall progress
	if stats.TotalChildren > 0 {
		terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
		stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
	} else {
		// No children yet, use parent job progress if available
		if job.ProgressTotal > 0 {
			stats.OverallProgress = float64(job.ProgressCurrent) / float64(job.ProgressTotal)
		}
	}

	// Generate progress text
	stats.ProgressText = m.generateProgressText(stats)

	// Get link following statistics from crawler metadata
	linkStats, err := m.getLinkFollowingStats(ctx, jobID)
	if err == nil {
		stats.LinksFound = linkStats.LinksFound
		stats.LinksFiltered = linkStats.LinksFiltered
		stats.LinksFollowed = linkStats.LinksFollowed
		stats.LinksSkipped = linkStats.LinksSkipped
	}

	// Estimate completion time
	if stats.OverallProgress > 0 && stats.OverallProgress < 1.0 && stats.StartedAt != nil && stats.RunningChildren > 0 {
		elapsed := time.Since(*stats.StartedAt)
		totalEstimated := float64(elapsed) / stats.OverallProgress
		remaining := totalEstimated - float64(elapsed)
		estimatedEnd := time.Now().Add(time.Duration(remaining))
		stats.EstimatedEnd = &estimatedEnd
	}

	// Extract errors and warnings
	if job.Error != nil && *job.Error != "" {
		stats.Errors = []string{*job.Error}
	}

	return stats, nil
}

// childJobStatistics holds detailed child job statistics
type childJobStatistics struct {
	TotalChildren     int
	CompletedChildren int
	FailedChildren    int
	RunningChildren   int
	PendingChildren   int
	CancelledChildren int
}

// getChildJobStatistics retrieves detailed child job statistics
func (m *Manager) getChildJobStatistics(ctx context.Context, parentJobID string) (*childJobStatistics, error) {
	statsMap, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, err
	}

	stats := &childJobStatistics{}
	if s, ok := statsMap[parentJobID]; ok {
		stats.TotalChildren = s.ChildCount
		stats.CompletedChildren = s.CompletedChildren
		stats.FailedChildren = s.FailedChildren
		stats.RunningChildren = s.RunningChildren
		stats.PendingChildren = s.PendingChildren
		stats.CancelledChildren = s.CancelledChildren
	}

	return stats, nil
}

// linkFollowingStats holds link following statistics
type linkFollowingStats struct {
	LinksFound    int
	LinksFiltered int
	LinksFollowed int
	LinksSkipped  int
}

// getLinkFollowingStats retrieves link following statistics from crawler metadata
// This aggregates link statistics across all child jobs for a parent crawler job
func (m *Manager) getLinkFollowingStats(ctx context.Context, jobID string) (*linkFollowingStats, error) {
	// For now, return empty stats as this would require parsing crawler metadata
	// In a full implementation, this would query the documents table or job metadata
	// to aggregate link statistics from all child jobs
	return &linkFollowingStats{}, nil
}

// generateProgressText creates human-readable progress text
func (m *Manager) generateProgressText(stats *CrawlerProgressStats) string {
	if stats.TotalChildren == 0 {
		return "No child jobs spawned yet"
	}

	return fmt.Sprintf("%d URLs (%d completed, %d failed, %d running, %d pending)",
		stats.TotalChildren,
		stats.CompletedChildren,
		stats.FailedChildren,
		stats.RunningChildren,
		stats.PendingChildren,
	)
}

// GetJobTreeProgressStats retrieves progress statistics for multiple parent jobs
// This is optimized for bulk operations when displaying multiple jobs in the UI
func (m *Manager) GetJobTreeProgressStats(ctx context.Context, parentJobIDs []string) (map[string]*CrawlerProgressStats, error) {
	if len(parentJobIDs) == 0 {
		return make(map[string]*CrawlerProgressStats), nil
	}

	result := make(map[string]*CrawlerProgressStats)

	// Get all parent jobs
	// We have to loop because ListJobs doesn't support IN clause for IDs
	// Or we can use ListJobs with no filter and filter in memory if list is small?
	// Better to loop GetJob for now or add GetJobs(ids) to interface.
	// Since interface is fixed, we loop.

	for _, id := range parentJobIDs {
		jobEntityInterface, err := m.jobStorage.GetJob(ctx, id)
		if err != nil {
			continue
		}
		jobEntity := jobEntityInterface.(*models.Job)

		stats := &CrawlerProgressStats{
			JobID:   jobEntity.ID,
			Status:  string(jobEntity.Status),
			JobType: jobEntity.Type,
		}

		if jobEntity.ParentID != nil {
			stats.ParentID = *jobEntity.ParentID
		}

		if jobEntity.StartedAt != nil {
			stats.StartedAt = jobEntity.StartedAt
		}

		if jobEntity.Error != "" {
			stats.Errors = []string{jobEntity.Error}
		}

		// Get child statistics for this parent
		childStats, err := m.getChildJobStatistics(ctx, id)
		if err == nil {
			stats.TotalChildren = childStats.TotalChildren
			stats.CompletedChildren = childStats.CompletedChildren
			stats.FailedChildren = childStats.FailedChildren
			stats.RunningChildren = childStats.RunningChildren
			stats.PendingChildren = childStats.PendingChildren
			stats.CancelledChildren = childStats.CancelledChildren

			// Calculate progress
			if stats.TotalChildren > 0 {
				terminalCount := stats.CompletedChildren + stats.FailedChildren + stats.CancelledChildren
				stats.OverallProgress = float64(terminalCount) / float64(stats.TotalChildren)
			}

			stats.ProgressText = m.generateProgressText(stats)
		}

		result[id] = stats
	}

	return result, nil
}

// ChildJobStats represents statistics for child jobs of a parent job
type ChildJobStats struct {
	TotalChildren     int `json:"total_children"`
	CompletedChildren int `json:"completed_children"`
	FailedChildren    int `json:"failed_children"`
	CancelledChildren int `json:"cancelled_children"`
	RunningChildren   int `json:"running_children"`
	PendingChildren   int `json:"pending_children"`
}

// GetChildJobStats retrieves child job statistics for a single parent job
// This is used by the JobMonitor to monitor child job progress
func (m *Manager) GetChildJobStats(ctx context.Context, parentJobID string) (*ChildJobStats, error) {
	statsMap, err := m.jobStorage.GetJobChildStats(ctx, []string{parentJobID})
	if err != nil {
		return nil, err
	}

	s := statsMap[parentJobID]
	if s == nil {
		return &ChildJobStats{}, nil
	}

	return &ChildJobStats{
		TotalChildren:     s.ChildCount,
		CompletedChildren: s.CompletedChildren,
		FailedChildren:    s.FailedChildren,
		CancelledChildren: s.CancelledChildren,
		RunningChildren:   s.RunningChildren,
		PendingChildren:   s.PendingChildren,
	}, nil
}

// GetQueue returns the queue manager for enqueueing jobs
func (m *Manager) GetQueue() interfaces.QueueManager {
	return m.queue
}

// GetQueueInterface returns the queue manager interface
func (m *Manager) GetQueueInterface() interfaces.QueueManager {
	return m.queue
}

// GetDocumentCount retrieves the document_count from job metadata
// Returns 0 if document_count is not present in metadata
func (m *Manager) GetDocumentCount(ctx context.Context, jobID string) (int, error) {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return 0, err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		return 0, nil
	}

	if count, ok := jobEntity.Metadata["document_count"].(float64); ok {
		return int(count), nil
	} else if count, ok := jobEntity.Metadata["document_count"].(int); ok {
		return count, nil
	}

	return 0, nil
}

// AddJobError adds an error message to the job's status_report
// This is used to track and display errors in the UI
func (m *Manager) AddJobError(ctx context.Context, jobID, errorMessage string) error {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		jobEntity.Metadata = make(map[string]interface{})
	}

	var statusReport map[string]interface{}
	if sr, ok := jobEntity.Metadata["status_report"].(map[string]interface{}); ok {
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
	jobEntity.Metadata["status_report"] = statusReport

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}

// AddJobWarning adds a warning message to the job's status_report
// This is used to track and display warnings in the UI
func (m *Manager) AddJobWarning(ctx context.Context, jobID, warningMessage string) error {
	jobEntityInterface, err := m.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	jobEntity := jobEntityInterface.(*models.Job)

	if jobEntity.Metadata == nil {
		jobEntity.Metadata = make(map[string]interface{})
	}

	var statusReport map[string]interface{}
	if sr, ok := jobEntity.Metadata["status_report"].(map[string]interface{}); ok {
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
	jobEntity.Metadata["status_report"] = statusReport

	return m.jobStorage.UpdateJob(ctx, jobEntity)
}
