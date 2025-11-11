// -----------------------------------------------------------------------
// Database Maintenance Worker - Processes individual database maintenance operations from the queue
// -----------------------------------------------------------------------

package worker

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// DatabaseMaintenanceWorker processes individual database maintenance operations (VACUUM, ANALYZE, REINDEX, OPTIMIZE)
type DatabaseMaintenanceWorker struct {
	db     *sql.DB
	jobMgr *jobs.Manager
	logger arbor.ILogger
}

// NewDatabaseMaintenanceWorker creates a new database maintenance worker
func NewDatabaseMaintenanceWorker(
	db *sql.DB,
	jobMgr *jobs.Manager,
	logger arbor.ILogger,
) *DatabaseMaintenanceWorker {
	return &DatabaseMaintenanceWorker{
		db:     db,
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

// Validate validates that the job model is compatible with this worker
func (w *DatabaseMaintenanceWorker) Validate(job *models.JobModel) error {
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
func (w *DatabaseMaintenanceWorker) Execute(ctx context.Context, job *models.JobModel) error {
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

	jobLogger.Info().
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

	jobLogger.Info().
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
	logger.Debug().Msg("Executing VACUUM")

	_, err := w.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}

	logger.Info().Msg("VACUUM completed successfully")
	return nil
}

// analyze performs ANALYZE operation
func (w *DatabaseMaintenanceWorker) analyze(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing ANALYZE")

	_, err := w.db.ExecContext(ctx, "ANALYZE")
	if err != nil {
		return fmt.Errorf("ANALYZE failed: %w", err)
	}

	logger.Info().Msg("ANALYZE completed successfully")
	return nil
}

// reindex performs REINDEX operation on all indexes
func (w *DatabaseMaintenanceWorker) reindex(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing REINDEX")

	// Get all indexes
	rows, err := w.db.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'index'
		AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		return fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return fmt.Errorf("failed to scan index name: %w", err)
		}
		indexes = append(indexes, indexName)
	}

	logger.Info().
		Int("index_count", len(indexes)).
		Msg("Reindexing database indexes")

	// Reindex each index
	for _, indexName := range indexes {
		logger.Debug().
			Str("index", indexName).
			Msg("Reindexing")

		_, err := w.db.ExecContext(ctx, fmt.Sprintf("REINDEX %s", indexName))
		if err != nil {
			logger.Warn().
				Err(err).
				Str("index", indexName).
				Msg("Failed to reindex - continuing")
			continue
		}
	}

	logger.Info().Msg("REINDEX completed successfully")
	return nil
}

// optimize performs database optimization
func (w *DatabaseMaintenanceWorker) optimize(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing OPTIMIZE")

	_, err := w.db.ExecContext(ctx, "PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("OPTIMIZE failed: %w", err)
	}

	logger.Info().Msg("OPTIMIZE completed successfully")
	return nil
}
