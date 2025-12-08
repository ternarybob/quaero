// -----------------------------------------------------------------------
// AggregateSummaryWorker - Generate summary of enrichment metadata
// Aggregates all enrichment data and generates a comprehensive summary
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs/actions"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// AggregateSummaryWorker handles generating a summary of all DevOps metadata.
// This worker aggregates data from all enrichment passes and generates a
// comprehensive summary document using an LLM.
type AggregateSummaryWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: AggregateSummaryWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*AggregateSummaryWorker)(nil)

// NewAggregateSummaryWorker creates a new aggregate summary worker
func NewAggregateSummaryWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *AggregateSummaryWorker {
	return &AggregateSummaryWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		llmService:      llmService,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeAggregateSummary for the DefinitionWorker interface
func (w *AggregateSummaryWorker) GetType() models.WorkerType {
	return models.WorkerTypeAggregateSummary
}

// Init performs the initialization/setup phase for aggregate summary step.
func (w *AggregateSummaryWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Msg("Aggregate summary worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems:            []interfaces.WorkItem{},
		TotalCount:           1, // Single aggregation task
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes the aggregate summary action.
func (w *AggregateSummaryWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize aggregate summary worker: %w", err)
		}
	}

	w.logger.Info().
		Str("step_id", stepID).
		Msg("Starting aggregate_devops_summary action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"Starting DevOps summary aggregation")

	// Check if LLM service is available
	if w.llmService == nil {
		w.logger.Warn().Msg("LLM service not available, skipping aggregate_devops_summary")
		w.logJobEvent(ctx, stepID, step.Name, "warning",
			"LLM service not available - summary generation skipped")
		return stepID, nil
	}

	// Create and execute the action
	action := actions.NewAggregateDevOpsSummaryAction(
		w.documentStorage,
		w.kvStorage,
		w.searchService,
		w.llmService,
		w.logger,
	)

	err := action.Execute(ctx)
	if err != nil {
		w.logger.Error().
			Err(err).
			Str("step_id", stepID).
			Msg("Failed to aggregate DevOps summary")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Failed to aggregate summary: %v", err))
		return stepID, fmt.Errorf("aggregate_devops_summary failed: %w", err)
	}

	w.logger.Info().
		Str("step_id", stepID).
		Msg("Successfully aggregated DevOps summary")
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"DevOps summary successfully generated and stored")

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *AggregateSummaryWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *AggregateSummaryWorker) ValidateConfig(step models.JobStep) error {
	// No required config for aggregate summary
	return nil
}

// logJobEvent logs a job event for real-time UI display
func (w *AggregateSummaryWorker) logJobEvent(ctx context.Context, parentJobID, stepName, level, message string) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
