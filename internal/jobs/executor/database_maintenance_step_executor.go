// -----------------------------------------------------------------------
// Database Maintenance Step Executor - Handles "database_maintenance" action in job definitions
// -----------------------------------------------------------------------

package executor

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

// DatabaseMaintenanceStepExecutor handles "database_maintenance" action steps
// It creates a database_maintenance job and enqueues it to the queue
type DatabaseMaintenanceStepExecutor struct {
	jobManager *jobs.Manager
	queueMgr   *queue.Manager
	logger     arbor.ILogger
}

// NewDatabaseMaintenanceStepExecutor creates a new database maintenance step executor
func NewDatabaseMaintenanceStepExecutor(jobManager *jobs.Manager, queueMgr *queue.Manager, logger arbor.ILogger) *DatabaseMaintenanceStepExecutor {
	return &DatabaseMaintenanceStepExecutor{
		jobManager: jobManager,
		queueMgr:   queueMgr,
		logger:     logger,
	}
}

// ExecuteStep executes a database maintenance step
func (e *DatabaseMaintenanceStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	e.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Starting database maintenance step")

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

	if err := e.jobManager.CreateJobRecord(ctx, dbJob); err != nil {
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	e.logger.Debug().
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
	if err := e.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	e.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Int("operation_count", len(operations)).
		Msg("Database maintenance step enqueued successfully")

	return jobID, nil
}

// GetStepType returns "database_maintenance"
func (e *DatabaseMaintenanceStepExecutor) GetStepType() string {
	return "database_maintenance"
}
