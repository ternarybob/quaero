// -----------------------------------------------------------------------
// AnalyzeBuildWorker - Analyze build system files
// Parses CMake, Makefile, and other build files for targets and dependencies
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

// AnalyzeBuildWorker handles analysis of build system files.
// This worker parses CMake, Makefile, vcxproj and other build files
// to extract build targets, compiler flags, and linked libraries.
type AnalyzeBuildWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: AnalyzeBuildWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*AnalyzeBuildWorker)(nil)

// NewAnalyzeBuildWorker creates a new analyze build worker
func NewAnalyzeBuildWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *AnalyzeBuildWorker {
	return &AnalyzeBuildWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		llmService:      llmService,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeAnalyzeBuild for the DefinitionWorker interface
func (w *AnalyzeBuildWorker) GetType() models.WorkerType {
	return models.WorkerTypeAnalyzeBuild
}

// Init performs the initialization/setup phase for analyze build step.
func (w *AnalyzeBuildWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for analyze build worker")
	}

	// Extract filter_tags (optional) - documents to process
	var filterTags []string
	if tags, ok := stepConfig["filter_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				filterTags = append(filterTags, tagStr)
			}
		}
	} else if tags, ok := stepConfig["filter_tags"].([]string); ok {
		filterTags = tags
	}

	// Query documents matching filter tags (if specified)
	var documents []*models.Document
	var err error
	if len(filterTags) > 0 {
		opts := interfaces.SearchOptions{
			Tags:  filterTags,
			Limit: 10000,
		}

		documents, err = w.searchService.Search(ctx, "", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to query documents: %w", err)
		}

		if len(documents) == 0 {
			w.logger.Warn().
				Str("phase", "init").
				Str("step_name", step.Name).
				Strs("filter_tags", filterTags).
				Msg("No documents found matching tags")
		}
	}

	w.logger.Info().
		Str("phase", "init").
		Str("step_name", step.Name).
		Int("document_count", len(documents)).
		Strs("filter_tags", filterTags).
		Msg("Analyze build worker initialized")

	// Create work items from documents
	workItems := make([]interfaces.WorkItem, len(documents))
	for i, doc := range documents {
		workItems[i] = interfaces.WorkItem{
			ID:   doc.ID,
			Name: doc.Title,
			Type: "document",
			Config: map[string]interface{}{
				"document_id": doc.ID,
				"title":       doc.Title,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             interfaces.ProcessingStrategyInline,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"filter_tags": filterTags,
			"documents":   documents,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes the analyze build system action on all documents.
func (w *AnalyzeBuildWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize analyze build worker: %w", err)
		}
	}

	// Extract metadata from init result
	documents, _ := initResult.Metadata["documents"].([]*models.Document)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get force flag from config (default: false)
	force := false
	if forceVal, ok := stepConfig["force"].(bool); ok {
		force = forceVal
	}

	w.logger.Info().
		Str("step_id", stepID).
		Int("document_count", len(documents)).
		Bool("force", force).
		Msg("Starting analyze_build_system action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Starting build system analysis for %d documents", len(documents)))

	// Create the action
	action := actions.NewAnalyzeBuildSystemAction(
		w.documentStorage,
		w.llmService,
		w.logger,
	)

	// Process each document
	processed := 0
	skipped := 0
	failed := 0

	for _, doc := range documents {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			w.logJobEvent(ctx, stepID, step.Name, "warn",
				fmt.Sprintf("Build system analysis cancelled after processing %d documents", processed))
			return stepID, ctx.Err()
		default:
		}

		// Execute action on document
		if err := action.Execute(ctx, doc, force); err != nil {
			w.logger.Error().
				Err(err).
				Str("document_id", doc.ID).
				Str("file_path", doc.URL).
				Msg("Failed to analyze build system for document")
			failed++
			continue
		}

		// Check if document was actually processed (is a build file)
		if action.IsBuildFile(doc.URL) {
			processed++
		} else {
			skipped++
		}
	}

	// Log completion
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Build system analysis completed: %d processed, %d skipped, %d failed",
			processed, skipped, failed))

	w.logger.Info().
		Str("step_id", stepID).
		Int("processed", processed).
		Int("skipped", skipped).
		Int("failed", failed).
		Msg("Build system analysis completed")

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *AnalyzeBuildWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *AnalyzeBuildWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("analyze build step requires config")
	}
	return nil
}

// logJobEvent logs a job event for real-time UI display
func (w *AnalyzeBuildWorker) logJobEvent(ctx context.Context, parentJobID, stepName, level, message string) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
