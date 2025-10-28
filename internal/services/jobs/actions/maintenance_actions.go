package actions

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// MaintenanceActionDeps holds dependencies needed by maintenance action handlers.
type MaintenanceActionDeps struct {
	DocumentStorage interfaces.DocumentStorage
	SummaryService  interfaces.SummaryService
	Logger          arbor.ILogger
}

// reindexAction rebuilds the FTS5 full-text search index.
func reindexAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *MaintenanceActionDeps) error {
	// Extract configuration parameters
	dryRun := extractBool(step.Config, "dry_run", false)

	deps.Logger.Info().
		Str("action", "reindex").
		Bool("dry_run", dryRun).
		Msg("Starting reindex action")

	// Dry run check
	if dryRun {
		deps.Logger.Info().
			Msg("Dry run mode - no actual index rebuild will be performed")

		deps.Logger.Info().
			Str("action", "reindex").
			Bool("dry_run", dryRun).
			Msg("Reindex action completed successfully (dry run)")

		return nil
	}

	// Perform actual index rebuild
	deps.Logger.Info().Msg("Calling RebuildFTS5Index()")
	if err := deps.DocumentStorage.RebuildFTS5Index(); err != nil {
		deps.Logger.Error().
			Err(err).
			Msg("Failed to rebuild FTS5 index")

		return fmt.Errorf("failed to rebuild FTS5 index: %w", err)
	}

	deps.Logger.Info().Msg("FTS5 index rebuilt successfully")

	deps.Logger.Info().
		Str("action", "reindex").
		Bool("dry_run", dryRun).
		Msg("Reindex action completed successfully")

	return nil
}

// corpusSummaryAction generates a summary document containing statistics about the document corpus.
func corpusSummaryAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *MaintenanceActionDeps) error {
	deps.Logger.Info().
		Str("action", "corpus_summary").
		Msg("Starting corpus summary action")

	if err := deps.SummaryService.GenerateSummaryDocument(ctx); err != nil {
		deps.Logger.Error().
			Err(err).
			Msg("Failed to generate corpus summary document")

		return fmt.Errorf("failed to generate corpus summary document: %w", err)
	}

	deps.Logger.Info().Msg("Corpus summary document generated successfully")

	deps.Logger.Info().
		Str("action", "corpus_summary").
		Msg("Corpus summary action completed successfully")

	return nil
}

// RegisterMaintenanceActions registers all maintenance-related actions with the job type registry.
func RegisterMaintenanceActions(registry *jobs.JobTypeRegistry, deps *MaintenanceActionDeps) error {
	// Validate inputs
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	if deps == nil {
		return fmt.Errorf("dependencies cannot be nil")
	}
	if deps.DocumentStorage == nil {
		return fmt.Errorf("DocumentStorage dependency cannot be nil")
	}
	if deps.Logger == nil {
		return fmt.Errorf("Logger dependency cannot be nil")
	}
	if deps.SummaryService == nil {
		return fmt.Errorf("SummaryService dependency cannot be nil")
	}

	// Create closure function that captures dependencies
	reindexActionHandler := func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		return reindexAction(ctx, step, sources, deps)
	}

	corpusSummaryActionHandler := func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		return corpusSummaryAction(ctx, step, sources, deps)
	}

	// Register reindex action for custom job type
	if err := registry.RegisterAction(models.JobTypeCustom, "reindex", reindexActionHandler); err != nil {
		return fmt.Errorf("failed to register reindex action: %w", err)
	}

	// Register corpus_summary action for custom job type
	if err := registry.RegisterAction(models.JobTypeCustom, "corpus_summary", corpusSummaryActionHandler); err != nil {
		return fmt.Errorf("failed to register corpus_summary action: %w", err)
	}

	deps.Logger.Info().
		Str("job_type", string(models.JobTypeCustom)).
		Int("action_count", 2).
		Msg("Maintenance actions registered successfully")

	return nil
}
