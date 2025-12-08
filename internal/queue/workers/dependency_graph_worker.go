// -----------------------------------------------------------------------
// DependencyGraphWorker - Build dependency graph from metadata
// Creates a graph of file dependencies based on extracted includes
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

// DependencyGraphWorker handles building dependency graphs from DevOps metadata.
// This worker creates a graph showing how files depend on each other based on
// includes, library links, and build dependencies.
type DependencyGraphWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	kvStorage       interfaces.KeyValueStorage
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: DependencyGraphWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*DependencyGraphWorker)(nil)

// NewDependencyGraphWorker creates a new dependency graph worker
func NewDependencyGraphWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	kvStorage interfaces.KeyValueStorage,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *DependencyGraphWorker {
	return &DependencyGraphWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		kvStorage:       kvStorage,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeDependencyGraph for the DefinitionWorker interface
func (w *DependencyGraphWorker) GetType() models.WorkerType {
	return models.WorkerTypeDependencyGraph
}

// Init performs the initialization/setup phase for dependency graph step.
func (w *DependencyGraphWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for dependency graph worker")
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
		Msg("Dependency graph worker initialized")

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
		SuggestedConcurrency: 1,
		Metadata: map[string]interface{}{
			"filter_tags": filterTags,
			"documents":   documents,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs executes the dependency graph action on all documents.
func (w *DependencyGraphWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize dependency graph worker: %w", err)
		}
	}

	w.logger.Info().
		Str("step_id", stepID).
		Msg("Starting build_dependency_graph action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"Building dependency graph from DevOps metadata")

	// Extract documents from init result
	documents, ok := initResult.Metadata["documents"].([]*models.Document)
	if !ok || len(documents) == 0 {
		w.logger.Warn().
			Str("step_id", stepID).
			Msg("No documents found in init result")
		w.logJobEvent(ctx, stepID, step.Name, "warn",
			"No documents found to build dependency graph")
		return stepID, nil
	}

	// Create action instance
	action := actions.NewBuildDependencyGraphAction(
		w.documentStorage,
		w.kvStorage,
		w.searchService,
		w.logger,
	)

	// Execute action
	err := action.Execute(ctx, documents)
	if err != nil {
		w.logger.Error().
			Err(err).
			Str("step_id", stepID).
			Msg("Failed to build dependency graph")
		w.logJobEvent(ctx, stepID, step.Name, "error",
			fmt.Sprintf("Failed to build dependency graph: %v", err))
		return stepID, fmt.Errorf("dependency graph build failed: %w", err)
	}

	// Update documents with enrichment tracking
	if err := w.updateEnrichmentTracking(ctx, documents, "build_dependency_graph"); err != nil {
		w.logger.Warn().
			Err(err).
			Str("step_id", stepID).
			Msg("Failed to update enrichment tracking")
		// Don't fail the step, just log the warning
	}

	w.logger.Info().
		Str("step_id", stepID).
		Int("document_count", len(documents)).
		Msg("Dependency graph built successfully")

	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Successfully built dependency graph from %d documents", len(documents)))

	return stepID, nil
}

// ReturnsChildJobs returns false since this executes synchronously
func (w *DependencyGraphWorker) ReturnsChildJobs() bool {
	return false
}

// ValidateConfig validates step configuration
func (w *DependencyGraphWorker) ValidateConfig(step models.JobStep) error {
	if step.Config == nil {
		return fmt.Errorf("dependency graph step requires config")
	}
	return nil
}

// updateEnrichmentTracking updates the enrichment_passes field in DevOps metadata
func (w *DependencyGraphWorker) updateEnrichmentTracking(ctx context.Context, documents []*models.Document, passName string) error {
	for _, doc := range documents {
		if doc.Metadata == nil {
			doc.Metadata = make(map[string]interface{})
		}

		// Get or create devops metadata
		var devops models.DevOpsMetadata
		if devopsData, ok := doc.Metadata["devops"]; ok {
			// Marshal and unmarshal to convert map to struct
			jsonData, err := json.Marshal(devopsData)
			if err != nil {
				w.logger.Warn().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to marshal existing devops metadata")
				continue
			}
			if err := json.Unmarshal(jsonData, &devops); err != nil {
				w.logger.Warn().
					Err(err).
					Str("doc_id", doc.ID).
					Msg("Failed to unmarshal existing devops metadata")
				continue
			}
		}

		// Add pass to enrichment tracking if not already present
		alreadyTracked := false
		for _, pass := range devops.EnrichmentPasses {
			if pass == passName {
				alreadyTracked = true
				break
			}
		}
		if !alreadyTracked {
			devops.EnrichmentPasses = append(devops.EnrichmentPasses, passName)
		}

		// Convert back to map and update document
		jsonData, err := json.Marshal(devops)
		if err != nil {
			w.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to marshal updated devops metadata")
			continue
		}
		var devopsMap map[string]interface{}
		if err := json.Unmarshal(jsonData, &devopsMap); err != nil {
			w.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to unmarshal to map")
			continue
		}
		doc.Metadata["devops"] = devopsMap

		// Save document
		if err := w.documentStorage.UpdateDocument(doc); err != nil {
			w.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("Failed to update document with enrichment tracking")
			// Continue processing other documents
		}
	}

	return nil
}

// logJobEvent logs a job event for real-time UI display
func (w *DependencyGraphWorker) logJobEvent(ctx context.Context, parentJobID, stepName, level, message string) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}
