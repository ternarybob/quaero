// -----------------------------------------------------------------------
// Database Maintenance Job Executor
// -----------------------------------------------------------------------

package executor

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// DatabaseMaintenanceExecutor handles database maintenance jobs
type DatabaseMaintenanceExecutor struct {
	*BaseExecutor
	db *sql.DB
}

// NewDatabaseMaintenanceExecutor creates a new database maintenance executor
func NewDatabaseMaintenanceExecutor(
	db *sql.DB,
	jobManager *jobs.Manager,
	queueMgr *queue.Manager,
	logger arbor.ILogger,
	logService interfaces.LogService,
	wsHandler interfaces.WebSocketHandler,
) *DatabaseMaintenanceExecutor {
	return &DatabaseMaintenanceExecutor{
		BaseExecutor: NewBaseExecutor(jobManager, queueMgr, logger, logService, wsHandler),
		db:           db,
	}
}

// GetJobType returns the job type this executor handles
func (e *DatabaseMaintenanceExecutor) GetJobType() string {
	return "database_maintenance"
}

// Validate validates the job model
func (e *DatabaseMaintenanceExecutor) Validate(job *models.JobModel) error {
	if job.Type != e.GetJobType() {
		return fmt.Errorf("invalid job type: expected %s, got %s", e.GetJobType(), job.Type)
	}
	return nil
}

// Execute executes the database maintenance job
func (e *DatabaseMaintenanceExecutor) Execute(ctx context.Context, job *models.JobModel) error {
	// Create job-specific logger with correlation ID
	jobLogger := e.CreateJobLogger(job)

	// Log job start
	e.LogJobStart(jobLogger, job)

	// Update job status to running
	if err := e.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Get configuration
	operations, ok := job.GetConfigStringSlice("operations")
	if !ok || len(operations) == 0 {
		// Default operations
		operations = []string{"vacuum", "analyze", "reindex"}
	}

	totalOps := len(operations)
	jobLogger.Info().
		Int("total_operations", totalOps).
		Strs("operations", operations).
		Msg("Starting database maintenance operations")

	// Execute each operation
	for i, operation := range operations {
		jobLogger.Info().
			Str("operation", operation).
			Int("step", i+1).
			Int("total", totalOps).
			Msg("Executing database operation")

		if err := e.executeOperation(ctx, jobLogger, operation); err != nil {
			jobLogger.Error().
				Err(err).
				Str("operation", operation).
				Msg("Database operation failed")

			// Set job error
			e.SetJobError(ctx, job.ID, fmt.Sprintf("Operation %s failed: %v", operation, err))

			// Update status to failed
			e.UpdateJobStatus(ctx, job.ID, "failed")

			e.LogJobError(jobLogger, job, err)
			return fmt.Errorf("database operation %s failed: %w", operation, err)
		}

		// Update progress
		e.UpdateJobProgress(ctx, job.ID, i+1, totalOps)
		e.LogJobProgress(jobLogger, job, i+1, totalOps, fmt.Sprintf("Completed operation: %s", operation))
	}

	// Mark job as completed
	if err := e.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
	}

	e.LogJobComplete(jobLogger, job)
	return nil
}

// executeOperation executes a single database maintenance operation
func (e *DatabaseMaintenanceExecutor) executeOperation(ctx context.Context, logger arbor.ILogger, operation string) error {
	switch operation {
	case "vacuum":
		return e.vacuum(ctx, logger)
	case "analyze":
		return e.analyze(ctx, logger)
	case "reindex":
		return e.reindex(ctx, logger)
	case "optimize":
		return e.optimize(ctx, logger)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

// vacuum performs VACUUM operation
func (e *DatabaseMaintenanceExecutor) vacuum(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing VACUUM")
	
	_, err := e.db.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}
	
	logger.Info().Msg("VACUUM completed successfully")
	return nil
}

// analyze performs ANALYZE operation
func (e *DatabaseMaintenanceExecutor) analyze(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing ANALYZE")
	
	_, err := e.db.ExecContext(ctx, "ANALYZE")
	if err != nil {
		return fmt.Errorf("ANALYZE failed: %w", err)
	}
	
	logger.Info().Msg("ANALYZE completed successfully")
	return nil
}

// reindex performs REINDEX operation on all indexes
func (e *DatabaseMaintenanceExecutor) reindex(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing REINDEX")
	
	// Get all indexes
	rows, err := e.db.QueryContext(ctx, `
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
		
		_, err := e.db.ExecContext(ctx, fmt.Sprintf("REINDEX %s", indexName))
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
func (e *DatabaseMaintenanceExecutor) optimize(ctx context.Context, logger arbor.ILogger) error {
	logger.Debug().Msg("Executing OPTIMIZE")
	
	_, err := e.db.ExecContext(ctx, "PRAGMA optimize")
	if err != nil {
		return fmt.Errorf("OPTIMIZE failed: %w", err)
	}
	
	logger.Info().Msg("OPTIMIZE completed successfully")
	return nil
}

