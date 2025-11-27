// -----------------------------------------------------------------------
// Database Maintenance Worker - Processes individual database maintenance operations from the queue
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// DatabaseMaintenanceWorker processes individual database maintenance operations (VACUUM, ANALYZE, REINDEX, OPTIMIZE)
// NOTE: This worker is deprecated as BadgerDB handles maintenance automatically.
// It is kept for interface compatibility but operations are no-ops.
type DatabaseMaintenanceWorker struct {
	jobMgr *queue.Manager
	logger arbor.ILogger
}

// Compile-time assertion: DatabaseMaintenanceWorker implements JobWorker interface
var _ interfaces.JobWorker = (*DatabaseMaintenanceWorker)(nil)

// NewDatabaseMaintenanceWorker creates a new database maintenance worker
func NewDatabaseMaintenanceWorker(
	jobMgr *queue.Manager,
	logger arbor.ILogger,
) *DatabaseMaintenanceWorker {
	return &DatabaseMaintenanceWorker{
		jobMgr: jobMgr,
		logger: logger,
	}
}

// ============================================================================
// INTERFACE METHODS
// ============================================================================

// GetWorkerType returns "database_maintenance_operation" - the job type this worker handles
func (w *DatabaseMaintenanceWorker) GetWorkerType() string {
	return "database_maintenance_operation"
}

// Validate validates that the queue job is compatible with this worker
func (w *DatabaseMaintenanceWorker) Validate(job *models.QueueJob) error {
	if job.Type != "database_maintenance_operation" {
		return fmt.Errorf("invalid job type: expected %s, got %s", "database_maintenance_operation", job.Type)
	}

	// Validate required config field: operation
	if _, ok := job.GetConfigString("operation"); !ok {
		return fmt.Errorf("missing required config field: operation")
	}

	return nil
}

// Execute executes a single database maintenance operation
func (w *DatabaseMaintenanceWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// Create job-specific logger with correlation ID
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}
	jobLogger := w.logger.WithCorrelationId(parentID)

	// Get operation from config
	operation, ok := job.GetConfigString("operation")
	if !ok {
		return fmt.Errorf("missing operation in job config")
	}

	jobLogger.Debug().
		Str("job_id", job.ID).
		Str("parent_id", parentID).
		Str("operation", operation).
		Msg("Starting database maintenance operation")

	// Update job status to running
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Starting %s operation", operation))

	// Execute the operation
	if err := w.executeOperation(ctx, jobLogger, operation); err != nil {
		jobLogger.Error().
			Err(err).
			Str("operation", operation).
			Msg("Database operation failed")

		// Set job error
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Operation %s failed: %v", operation, err))

		// Update status to failed
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")

		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Operation %s failed: %v", operation, err))
		return fmt.Errorf("database operation %s failed: %w", operation, err)
	}

	// Mark job as completed
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
	}

	jobLogger.Debug().
		Str("job_id", job.ID).
		Str("operation", operation).
		Msg("Database maintenance operation completed successfully")

	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Operation %s completed successfully", operation))

	return nil
}

// ============================================================================
// OPERATION EXECUTION
// ============================================================================

// executeOperation executes a single database maintenance operation
func (w *DatabaseMaintenanceWorker) executeOperation(ctx context.Context, logger arbor.ILogger, operation string) error {
	switch operation {
	case "vacuum":
		return w.vacuum(ctx, logger)
	case "analyze":
		return w.analyze(ctx, logger)
	case "reindex":
		return w.reindex(ctx, logger)
	case "optimize":
		return w.optimize(ctx, logger)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

// vacuum performs VACUUM operation
func (w *DatabaseMaintenanceWorker) vacuum(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("VACUUM operation skipped (BadgerDB handles compaction automatically)")
	return nil
}

// analyze performs ANALYZE operation
func (w *DatabaseMaintenanceWorker) analyze(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("ANALYZE operation skipped (Not applicable to BadgerDB)")
	return nil
}

// reindex performs REINDEX operation on all indexes
func (w *DatabaseMaintenanceWorker) reindex(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("REINDEX operation skipped (Not applicable to BadgerDB)")
	return nil
}

// optimize performs database optimization
func (w *DatabaseMaintenanceWorker) optimize(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("OPTIMIZE operation skipped (BadgerDB handles optimization automatically)")
	return nil
}
