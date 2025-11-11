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
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// DatabaseMaintenanceManager creates parent database maintenance jobs and orchestrates database
// optimization workflows (VACUUM, ANALYZE, REINDEX, OPTIMIZE)
type DatabaseMaintenanceManager struct {
	jobManager            *jobs.Manager
	queueMgr              *queue.Manager
	parentJobOrchestrator interfaces.ParentJobOrchestrator
	logger                arbor.ILogger
}

// Compile-time assertion: DatabaseMaintenanceManager implements StepManager interface
var _ interfaces.StepManager = (*DatabaseMaintenanceManager)(nil)

// NewDatabaseMaintenanceManager creates a new database maintenance manager
func NewDatabaseMaintenanceManager(jobManager *jobs.Manager, queueMgr *queue.Manager, parentJobOrchestrator interfaces.ParentJobOrchestrator, logger arbor.ILogger) *DatabaseMaintenanceManager {
	return &DatabaseMaintenanceManager{
		jobManager:            jobManager,
		queueMgr:              queueMgr,
		parentJobOrchestrator: parentJobOrchestrator,
		logger:                logger,
	}
}

// CreateParentJob creates a parent database maintenance job and child jobs for each operation.
// Each operation (VACUUM, ANALYZE, REINDEX, OPTIMIZE) is executed as a separate job for granular tracking.
func (m *DatabaseMaintenanceManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	m.logger.Info().
		Str("step_name", step.Name).
		Str("action", step.Action).
		Str("parent_job_id", parentJobID).
		Msg("Creating parent database maintenance job and child jobs for each operation")

	// Generate parent job ID
	dbMaintenanceParentJobID := uuid.New().String()

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

	// Create parent job record for orchestration tracking
	parentJob := &jobs.Job{
		ID:       dbMaintenanceParentJobID,
		ParentID: &parentJobID, // Reference to job definition parent
		Type:     "database_maintenance_parent",
		Name:     "Database Maintenance",
		Phase:    "orchestration",
		Status:   "running",
	}

	if err := m.jobManager.CreateJobRecord(ctx, parentJob); err != nil {
		return "", fmt.Errorf("failed to create parent job record: %w", err)
	}

	m.logger.Debug().
		Str("parent_job_id", dbMaintenanceParentJobID).
		Int("operation_count", len(operations)).
		Msg("Parent job record created, creating child jobs for each operation")

	// Create child job for each operation
	for _, operation := range operations {
		childJobID := uuid.New().String()

		// Create child job model
		childJob := models.NewChildJobModel(
			dbMaintenanceParentJobID,
			"database_maintenance_operation",
			operation, // Use operation as name
			map[string]interface{}{
				"operation": operation, // Single operation
			},
			map[string]interface{}{
				"step_name": step.Name,
			},
			1, // depth
		)
		childJob.ID = childJobID

		// Validate job model
		if err := childJob.Validate(); err != nil {
			return "", fmt.Errorf("invalid child job model for operation %s: %w", operation, err)
		}

		// Create child job record in database
		dbJob := &jobs.Job{
			ID:       childJobID,
			ParentID: &dbMaintenanceParentJobID,
			Type:     "database_maintenance_operation",
			Name:     fmt.Sprintf("Database Maintenance: %s", operation),
			Phase:    "execution",
			Status:   "pending",
		}

		if err := m.jobManager.CreateJobRecord(ctx, dbJob); err != nil {
			return "", fmt.Errorf("failed to create child job record for operation %s: %w", operation, err)
		}

		// Serialize job model to JSON
		payloadBytes, err := childJob.ToJSON()
		if err != nil {
			return "", fmt.Errorf("failed to serialize child job model for operation %s: %w", operation, err)
		}

		// Create queue message
		queueMsg := queue.Message{
			JobID:   childJobID,
			Type:    "database_maintenance_operation",
			Payload: json.RawMessage(payloadBytes),
		}

		// Enqueue child job
		if err := m.queueMgr.Enqueue(ctx, queueMsg); err != nil {
			return "", fmt.Errorf("failed to enqueue child job for operation %s: %w", operation, err)
		}

		m.logger.Debug().
			Str("child_job_id", childJobID).
			Str("operation", operation).
			Msg("Child job created and enqueued")
	}

	// Start ParentJobOrchestrator monitoring
	parentJobModel := &models.JobModel{
		ID:       dbMaintenanceParentJobID,
		ParentID: &parentJobID,
		Type:     "database_maintenance_parent",
		Name:     "Database Maintenance",
		Config: map[string]interface{}{
			"source_type": "database",
			"entity_type": "maintenance",
		},
		Metadata: map[string]interface{}{
			"step_name": step.Name,
		},
		Depth: 0,
	}

	m.parentJobOrchestrator.StartMonitoring(ctx, parentJobModel)

	m.logger.Info().
		Str("step_name", step.Name).
		Str("parent_job_id", dbMaintenanceParentJobID).
		Int("child_job_count", len(operations)).
		Msg("Database maintenance parent job created with child jobs enqueued successfully")

	return dbMaintenanceParentJobID, nil
}

// GetManagerType returns "database_maintenance" - the action type this manager handles
func (m *DatabaseMaintenanceManager) GetManagerType() string {
	return "database_maintenance"
}
