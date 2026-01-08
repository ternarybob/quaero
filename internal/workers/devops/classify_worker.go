// -----------------------------------------------------------------------
// ClassifyWorker - LLM-based classification of C/C++ files
// Classifies file roles, components, test types, and external dependencies
// -----------------------------------------------------------------------

package devops

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs/actions"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ClassifyWorker handles LLM-based classification of C/C++ files.
// This worker uses an LLM to classify file roles, identify components,
// detect test types/frameworks, and identify external dependencies.
type ClassifyWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: ClassifyWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*ClassifyWorker)(nil)

// NewClassifyWorker creates a new classify worker
func NewClassifyWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ClassifyWorker {
	return &ClassifyWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		llmService:      llmService,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeClassify for the DefinitionWorker interface
func (w *ClassifyWorker) GetType() models.WorkerType {
	return models.WorkerTypeClassify
}

// Init performs the initialization/setup phase for classify step.
func (w *ClassifyWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for classify worker")
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
		Msg("Classify worker initialized")

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

	// Use parallel processing for large document sets
	strategy := interfaces.ProcessingStrategyInline
	if len(documents) > 100 {
		strategy = interfaces.ProcessingStrategyParallel
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             strategy,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"filter_tags": filterTags,
			"documents":   documents,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes the classify action on all documents.
func (w *ClassifyWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize classify worker: %w", err)
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
		Msg("Starting classify_devops action")

	// Create the classify action
	classifyAction := actions.NewClassifyDevOpsAction(
		w.documentStorage,
		w.llmService,
		w.logger,
	)

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Classifying %d documents using LLM", len(documents)))

	// Process each document
	successCount := 0
	failureCount := 0
	skippedCount := 0

	for i, doc := range documents {
		w.logger.Debug().
			Int("index", i+1).
			Int("total", len(documents)).
			Str("doc_id", doc.ID).
			Str("title", doc.Title).
			Msg("Classifying document")

		err := classifyAction.Execute(ctx, doc, force)
		if err != nil {
			w.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to classify document")
			failureCount++

			// Log individual failure but continue processing
			w.logJobEvent(ctx, stepID, step.Name, "warning",
				fmt.Sprintf("Failed to classify %s: %v", doc.Title, err))
		} else {
			// Check if document was actually processed or skipped
			if !force {
				// Check if document had already been classified
				alreadyClassified := w.checkAlreadyClassified(doc)
				if alreadyClassified {
					skippedCount++
				} else {
					successCount++
				}
			} else {
				successCount++
			}
		}

		// Check context cancellation
		if ctx.Err() != nil {
			w.logJobEvent(ctx, stepID, step.Name, "warning",
				"Classification cancelled by context")
			return stepID, fmt.Errorf("classification cancelled: %w", ctx.Err())
		}
	}

	// Log completion summary
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Classification complete: %d succeeded, %d failed, %d skipped",
			successCount, failureCount, skippedCount))

	w.logger.Info().
		Str("step_id", stepID).
		Int("success", successCount).
		Int("failure", failureCount).
		Int("skipped", skippedCount).
		Msg("Classify DevOps action completed")

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *ClassifyWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ClassifyWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("classify step requires config")
	}
	return nil
}

// checkAlreadyClassified checks if document has already been classified
func (w *ClassifyWorker) checkAlreadyClassified(doc *models.Document) bool {
	if doc.Metadata == nil {
		return false
	}

	devopsData, ok := doc.Metadata["devops"]
	if !ok {
		return false
	}

	if devopsMap, ok := devopsData.(map[string]interface{}); ok {
		if passes, ok := devopsMap["enrichment_passes"].([]interface{}); ok {
			for _, pass := range passes {
				if passStr, ok := pass.(string); ok && passStr == "classify_devops" {
					return true
				}
			}
		}
	}

	return false
}

// logJobEvent logs a job event for real-time UI display
func (w *ClassifyWorker) logJobEvent(ctx context.Context, parentJobID, stepName, level, message string) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
