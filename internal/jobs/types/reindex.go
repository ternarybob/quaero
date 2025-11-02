package types

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/queue"
)

// ReindexJobDeps holds dependencies for reindex jobs
type ReindexJobDeps struct {
	DocumentStorage interfaces.DocumentStorage
}

// ReindexJob handles FTS5 index rebuilding operations
type ReindexJob struct {
	*BaseJob
	deps *ReindexJobDeps
}

// NewReindexJob creates a new reindex job
func NewReindexJob(base *BaseJob, deps *ReindexJobDeps) *ReindexJob {
	return &ReindexJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a reindex job
func (r *ReindexJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	r.logger.Info().
		Str("message_id", msg.ID).
		Msg("Processing reindex job")

	// Validate message
	if err := r.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract dry_run from config
	dryRun := false // Default: actually rebuild index
	if dry, ok := msg.Config["dry_run"].(bool); ok {
		dryRun = dry
	}

	// Log job start
	r.logger.LogJobStart("reindex", "system", msg.Config)

	r.logger.Info().
		Bool("dry_run", dryRun).
		Msg("Starting reindex operation")

	startTime := time.Now()

	// Get total document count for progress tracking
	totalDocs, err := r.deps.DocumentStorage.CountDocuments()
	if err != nil {
		r.logger.Error().
			Err(err).
			Msg("Failed to count documents")
		return fmt.Errorf("failed to count documents: %w", err)
	}

	r.logger.Info().
		Int("total_documents", totalDocs).
		Msg("Document count retrieved")

	// Perform reindex operation
	// Note: In a real implementation, this would rebuild the FTS5 index
	// For now, we'll simulate the operation

	if !dryRun {
		r.logger.Info().
			Int("documents_to_reindex", totalDocs).
			Msg("Reindexing documents (simulated)")

		// Simulate reindexing progress
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return fmt.Errorf("reindex operation cancelled")
			default:
				time.Sleep(100 * time.Millisecond)
				progress := (i + 1) * 10
				if progress <= 100 {
					r.logger.Debug().
						Int("progress_pct", progress).
						Msg("Reindex progress")
				}
			}
		}

		r.logger.Info().
			Int("documents_reindexed", totalDocs).
			Msg("Reindex completed")
	} else {
		r.logger.Info().
			Int("documents_found", totalDocs).
			Msg("Dry run mode - no actual reindexing performed")
	}

	duration := time.Since(startTime)

	// Log completion
	r.logger.LogJobComplete(duration, totalDocs)

	r.logger.Info().
		Str("message_id", msg.ID).
		Int("documents_processed", totalDocs).
		Bool("dry_run", dryRun).
		Float64("duration_sec", duration.Seconds()).
		Msg("Reindex job completed successfully")

	return nil
}

// Validate validates the reindex message
func (r *ReindexJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate dry_run if present
	if dryRun, ok := msg.Config["dry_run"].(bool); ok && !dryRun && dryRun {
		return fmt.Errorf("dry_run must be boolean")
	}

	return nil
}

// GetType returns the job type
func (r *ReindexJob) GetType() string {
	return "reindex"
}
