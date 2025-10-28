package types

import (
	"context"
	"fmt"

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
	if err := r.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Starting FTS5 index rebuild: dry_run=%v", dryRun)); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to log job start event")
	}

	r.logger.Info().
		Bool("dry_run", dryRun).
		Msg("Starting FTS5 index rebuild")

	// Dry run check
	if dryRun {
		r.logger.Info().
			Msg("Dry run mode - no actual index rebuild will be performed")

		// Log completion
		if err := r.LogJobEvent(ctx, msg.ParentID, "info",
			"FTS5 index rebuild completed (dry run - no changes made)"); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to log job completion event")
		}

		r.logger.Info().
			Str("message_id", msg.ID).
			Bool("dry_run", dryRun).
			Msg("Reindex job completed successfully (dry run)")

		return nil
	}

	// Perform actual index rebuild
	r.logger.Info().Msg("Calling RebuildFTS5Index()")
	if err := r.deps.DocumentStorage.RebuildFTS5Index(); err != nil {
		r.logger.Error().
			Err(err).
			Msg("Failed to rebuild FTS5 index")

		// Log failure
		if logErr := r.LogJobEvent(ctx, msg.ParentID, "error",
			fmt.Sprintf("FTS5 index rebuild failed: %v", err)); logErr != nil {
			r.logger.Warn().Err(logErr).Msg("Failed to log job failure event")
		}

		return fmt.Errorf("failed to rebuild FTS5 index: %w", err)
	}

	r.logger.Info().Msg("FTS5 index rebuilt successfully")

	// Log completion
	if err := r.LogJobEvent(ctx, msg.ParentID, "info",
		"FTS5 index rebuild completed successfully"); err != nil {
		r.logger.Warn().Err(err).Msg("Failed to log job completion event")
	}

	r.logger.Info().
		Str("message_id", msg.ID).
		Bool("dry_run", dryRun).
		Msg("Reindex job completed successfully")

	return nil
}

// Validate validates the reindex message
func (r *ReindexJob) Validate(msg *queue.JobMessage) error {
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate ParentID is present (required for logging)
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required for logging job events")
	}

	// Validate dry_run if present (optional but check type)
	if dry, ok := msg.Config["dry_run"]; ok {
		if _, isBool := dry.(bool); !isBool {
			return fmt.Errorf("dry_run must be a boolean, got: %T", dry)
		}
	}

	return nil
}

// GetType returns the job type
func (r *ReindexJob) GetType() string {
	return "reindex"
}
