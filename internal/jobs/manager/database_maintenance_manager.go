// -----------------------------------------------------------------------
// Database Maintenance Manager - Handles "database_maintenance" action in job definitions
// -----------------------------------------------------------------------

package manager

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// DatabaseMaintenanceManager creates parent database maintenance jobs and orchestrates database
// optimization workflows (VACUUM, ANALYZE, REINDEX, OPTIMIZE)
type DatabaseMaintenanceManager struct {
	jobManager *jobs.Manager
	queueMgr   *queue.Manager
	logger     arbor.ILogger
}

// NewDatabaseMaintenanceManager creates a new database maintenance manager
func NewDatabaseMaintenanceManager(jobManager *jobs.Manager, queueMgr *queue.Manager, logger arbor.ILogger) *DatabaseMaintenanceManager {
	return &DatabaseMaintenanceManager{
		jobManager: jobManager,
		queueMgr:   queueMgr,
		logger:     logger,
	}
}

// CreateParentJob creates a parent database maintenance job and enqueues it to the queue for processing.
// The job will execute database optimization operations based on the configuration.
func (m *DatabaseMaintenanceManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	m.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Creating parent database maintenance job")

	// Generate job ID for this step
	jobID := uuid.New().String()

	// Get operations from step config
	operations := []string{"vacuum", "analyze", "reindex"} // Default
	if step.Config != nil {
		if ops, ok := step.Config["operations"].([]interface{}); ok {
			operations = make([]string, 0, len(ops))
			for _, op := range ops {
				if opStr, ok := op.(string); ok {
					operations = append(operations, opStr)
				}
			}
		} else if ops, ok := step.Config["operations"].([]string); ok {
			operations = ops
		}
	}

	// Create job model
	jobModel := models.NewChildJobModel(
		parentJobID,
		"database_maintenance",
		step.Name,
		map[string]interface{}{
			"operations": operations,
		},
		map[string]interface{}{
			"step_name": step.Name,
		},
		1, // depth
	)

	// Override job ID to match the one we generated
	jobModel.ID = jobID

	// Validate job model
	if err := jobModel.Validate(); err != nil {
		return "", fmt.Errorf("invalid job model: %w", err)
	}

	// Create job record in database BEFORE enqueueing
	// This ensures the foreign key constraint is satisfied when logs start flowing
	dbJob := &jobs.Job{
		ID:       jobID,
		ParentID: &parentJobID,
		Type:     "database_maintenance",
		Name:     "Database Maintenance", // Human-readable name
		Phase:    "core",
		Status:   "pending",
	}

	if err := m.jobManager.CreateJobRecord(ctx, dbJob); err != nil {
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	m.logger.Debug().
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Msg("Job record created in database")

	// Serialize job model to JSON
	payloadBytes, err := jobModel.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize job model: %w", err)
	}

	// Create queue message
	queueMsg := queue.Message{
		JobID:   jobID,
		Type:    "database_maintenance",
		Payload: json.RawMessage(payloadBytes),
	}

	// Enqueue job
	if err := m.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Int("operation_count", len(operations)).
		Msg("Database maintenance job created and enqueued successfully")

	return jobID, nil
}

// GetManagerType returns "database_maintenance" - the action type this manager handles
func (m *DatabaseMaintenanceManager) GetManagerType() string {
	return "database_maintenance"
}
