// -----------------------------------------------------------------------
// ExtractStructureWorker - Extract C/C++ code structure
// Extracts includes, defines, conditionals, and platform-specific code
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs/actions"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// ExtractStructureWorker handles extraction of C/C++ code structure.
// This worker extracts includes, defines, conditionals, and detects platform-specific code.
type ExtractStructureWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: ExtractStructureWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*ExtractStructureWorker)(nil)

// NewExtractStructureWorker creates a new extract structure worker
func NewExtractStructureWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *ExtractStructureWorker {
	return &ExtractStructureWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeExtractStructure for the DefinitionWorker interface
func (w *ExtractStructureWorker) GetType() models.WorkerType {
	return models.WorkerTypeExtractStructure
}

// Init performs the initialization/setup phase for extract structure step.
func (w *ExtractStructureWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for extract structure worker")
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
		Msg("Extract structure worker initialized")

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

	// Determine processing strategy based on document count
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

// CreateJobs executes the extract structure action on all documents.
func (w *ExtractStructureWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize extract structure worker: %w", err)
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
		Msg("Starting extract_structure action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Extracting C/C++ structure from %d documents", len(documents)))

	// Create action handler
	action := actions.NewExtractStructureAction(w.documentStorage, w.logger)

	// Process each document
	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, doc := range documents {
		// Get existing DevOps metadata to check if already processed
		alreadyProcessed := w.checkAlreadyProcessed(doc, "extract_structure")

		if err := action.Execute(ctx, doc, force); err != nil {
			w.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Str("title", doc.Title).
				Msg("Failed to extract structure")
			w.logJobEvent(ctx, stepID, step.Name, "error",
				fmt.Sprintf("Failed to extract structure from %s: %v", doc.Title, err))
			errorCount++
			continue
		}

		// Check if document was processed or skipped
		if !force && alreadyProcessed {
			skipCount++
		} else {
			successCount++
		}
	}

	// Log completion summary
	w.logger.Info().
		Str("step_id", stepID).
		Int("success", successCount).
		Int("skipped", skipCount).
		Int("errors", errorCount).
		Int("total", len(documents)).
		Msg("Extract structure action completed")

	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Extract structure completed: %d processed, %d skipped, %d errors",
			successCount, skipCount, errorCount))

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *ExtractStructureWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *ExtractStructureWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("extract structure step requires config")
	}
	return nil
}

// checkAlreadyProcessed checks if document has already been processed by a pass
func (w *ExtractStructureWorker) checkAlreadyProcessed(doc *models.Document, passName string) bool {
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
				if passStr, ok := pass.(string); ok && passStr == passName {
					return true
				}
			}
		}
	} else {
		// Try direct type assertion
		var devops models.DevOpsMetadata
		data, err := json.Marshal(devopsData)
		if err == nil {
			if err := json.Unmarshal(data, &devops); err == nil {
				for _, pass := range devops.EnrichmentPasses {
					if pass == passName {
						return true
					}
				}
			}
		}
	}

	return false
}

// logJobEvent logs a job event for real-time UI display
func (w *ExtractStructureWorker) logJobEvent(ctx context.Context, parentJobID, stepName, level, message string) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
