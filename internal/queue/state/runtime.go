package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Manager handles job state mutations.
type Manager struct {
	jobStorage   interfaces.QueueStorage
	logStorage   interfaces.LogStorage
	eventService interfaces.EventService // Optional: may be nil for testing
}

func NewManager(jobStorage interfaces.QueueStorage, logStorage interfaces.LogStorage, eventService interfaces.EventService) *Manager {
	return &Manager{
		jobStorage:   jobStorage,
		logStorage:   logStorage,
		eventService: eventService,
	}
}

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

	// Add job log for status change with job identification
	// Include job name and type for clear identification in logs
	// Child/worker jobs log at DEBUG level to reduce noise in parent step logs
	// Parent jobs (no ParentID) log at INFO level for visibility
	jobName := jobState.Name
	if jobName == "" {
		jobName = jobID[:8] // Use truncated ID if no name
	}
	logMessage := fmt.Sprintf("Status changed: %s [%s: %s]", status, jobState.Type, jobName)
	logLevel := "info"
	if jobState.ParentID != nil {
		// Child jobs log at debug level to avoid flooding parent step logs
		logLevel = "debug"
	}
	if err := m.AddJobLog(ctx, jobID, logLevel, logMessage); err != nil {
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
	jobState := jobEntityInterface.(*models.QueueJobState)

	if jobState.Metadata == nil {
		jobState.Metadata = make(map[string]interface{})
	}

	jobState.Metadata["result"] = string(resultJSON)

	return m.jobStorage.UpdateJob(ctx, jobState)
}

// SetJobFinished sets the finished_at timestamp for a job
// This should be called when a job AND all its spawned children complete or timeout
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
// This method merges new metadata with existing metadata to preserve fields like phase
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
	entry := models.LogEntry{
		Timestamp:     now.Format("15:04:05.000"),
		FullTimestamp: now.Format(time.RFC3339),
		Level:         level,
		Message:       message,
		Context:       map[string]string{models.LogCtxJobID: jobID},
	}
	_, err := m.logStorage.AppendLog(ctx, jobID, entry)
	return err
}

// AddJobError adds an error message to the job's status_report
// This is used to track and display errors in the UI
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
// This is used to track and display warnings in the UI
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

// StopAllChildJobs implements interfaces.JobManager.StopAllChildJobs
func (m *Manager) StopAllChildJobs(ctx context.Context, parentID string) (int, error) {
	// Get running/pending child jobs
	// This is inefficient without a direct update query, but Badger doesn't support SQL updates
	// We have to list and update individually

	// Get all children
	// Note: GetChildJobs returns []*models.QueueJob, but we need status which is in QueueJobState
	// We need to fix the interface or use ListJobs with filter.
	// Assuming GetChildJobs returns QueueJob which doesn't have status.
	// We need to fetch each job to check status.
	// Or use ListJobs with ParentID filter if supported.

	// Let's use ListJobs with ParentID filter
	opts := &interfaces.JobListOptions{
		ParentID: parentID,
	}

	// ListJobs returns []*models.QueueJobState
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
