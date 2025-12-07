// -----------------------------------------------------------------------
// DevOpsWorker - DevOps pipeline enrichment for C/C++ code analysis
// Routes to different action handlers based on step configuration
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

// DevOpsWorker handles DevOps pipeline enrichment for C/C++ code analysis.
// This worker routes to different action handlers based on the step configuration.
// Supported actions:
//   - extract_structure: Extract C/C++ code structure (includes, defines, conditionals)
//   - analyze_build_system: Parse build files (CMake, Makefile) for targets and dependencies
//   - classify_devops: LLM-based classification of file roles and components
//   - build_dependency_graph: Build dependency graph from extracted metadata
//   - aggregate_devops_summary: Generate summary of all DevOps metadata
type DevOpsWorker struct {
	searchService   interfaces.SearchService
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	kvStorage       interfaces.KeyValueStorage
	llmService      interfaces.LLMService
	logger          arbor.ILogger
	jobMgr          *queue.Manager
}

// Compile-time assertion: DevOpsWorker implements DefinitionWorker interface
var _ interfaces.DefinitionWorker = (*DevOpsWorker)(nil)

// NewDevOpsWorker creates a new DevOps worker
func NewDevOpsWorker(
	searchService interfaces.SearchService,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	kvStorage interfaces.KeyValueStorage,
	llmService interfaces.LLMService,
	logger arbor.ILogger,
	jobMgr *queue.Manager,
) *DevOpsWorker {
	return &DevOpsWorker{
		searchService:   searchService,
		documentStorage: documentStorage,
		eventService:    eventService,
		kvStorage:       kvStorage,
		llmService:      llmService,
		logger:          logger,
		jobMgr:          jobMgr,
	}
}

// GetType returns WorkerTypeDevOps for the DefinitionWorker interface
func (w *DevOpsWorker) GetType() models.WorkerType {
	return models.WorkerTypeDevOps
}

// Init performs the initialization/setup phase for a DevOps enrichment step.
// This is where we:
//   - Extract and validate configuration (action, filter_tags, etc.)
//   - Query documents matching the filter criteria
//   - Return the document list as metadata for CreateJobs
//
// The Init phase does NOT perform enrichment - it only validates and prepares.
func (w *DevOpsWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		return nil, fmt.Errorf("step config is required for DevOps worker")
	}

	// Extract action (required) - determines which enrichment action to perform
	action, ok := stepConfig["action"].(string)
	if !ok || action == "" {
		return nil, fmt.Errorf("action is required in step config")
	}

	// Validate action is supported
	supportedActions := []string{
		"extract_structure",
		"analyze_build_system",
		"classify_devops",
		"build_dependency_graph",
		"aggregate_devops_summary",
	}
	validAction := false
	for _, a := range supportedActions {
		if action == a {
			validAction = true
			break
		}
	}
	if !validAction {
		return nil, fmt.Errorf("unsupported action '%s', must be one of: %v", action, supportedActions)
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
			Limit: 10000, // Maximum documents to process
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
		Str("action", action).
		Int("document_count", len(documents)).
		Strs("filter_tags", filterTags).
		Msg("DevOps worker initialized")

	// Create work items from documents (for reference in metadata)
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

	// Determine processing strategy based on action
	strategy := interfaces.ProcessingStrategyInline
	if action == "extract_structure" || action == "classify_devops" {
		// These actions may need async processing for large document sets
		if len(documents) > 100 {
			strategy = interfaces.ProcessingStrategyAsync
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             strategy,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"action":      action,
			"filter_tags": filterTags,
			"documents":   documents,
			"step_config": stepConfig,
		},
	}, nil
}

// CreateJobs routes to the appropriate action handler based on step configuration.
// This delegates to specific handler functions for each supported action.
// Returns the step job ID for inline execution, or parent job ID for async execution.
func (w *DevOpsWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize DevOps worker: %w", err)
		}
	}

	// Extract metadata from init result
	action, _ := initResult.Metadata["action"].(string)
	documents, _ := initResult.Metadata["documents"].([]*models.Document)

	w.logger.Info().
		Str("phase", "run").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Str("action", action).
		Int("document_count", len(documents)).
		Str("step_id", stepID).
		Msg("Starting DevOps enrichment action")

	// Route to appropriate handler based on action
	switch action {
	case "extract_structure":
		return w.handleExtractStructure(ctx, step, jobDef, stepID, initResult)
	case "analyze_build_system":
		return w.handleAnalyzeBuildSystem(ctx, step, jobDef, stepID, initResult)
	case "classify_devops":
		return w.handleClassifyDevOps(ctx, step, jobDef, stepID, initResult)
	case "build_dependency_graph":
		return w.handleBuildDependencyGraph(ctx, step, jobDef, stepID, initResult)
	case "aggregate_devops_summary":
		return w.handleAggregateDevOpsSummary(ctx, step, jobDef, stepID, initResult)
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

// ReturnsChildJobs returns true for async actions, false for inline actions
func (w *DevOpsWorker) ReturnsChildJobs() bool {
	// This depends on the action - some actions create child jobs, others execute inline
	// We'll return true as the default since most enrichment actions will be async
	return true
}

// ValidateConfig validates step configuration for DevOps type
func (w *DevOpsWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("DevOps step requires config")
	}

	// Validate required action field
	action, ok := step.Config["action"].(string)
	if !ok || action == "" {
		return fmt.Errorf("DevOps step requires 'action' in config")
	}

	// Validate action is supported
	supportedActions := []string{
		"extract_structure",
		"analyze_build_system",
		"classify_devops",
		"build_dependency_graph",
		"aggregate_devops_summary",
	}
	validAction := false
	for _, a := range supportedActions {
		if action == a {
			validAction = true
			break
		}
	}
	if !validAction {
		return fmt.Errorf("unsupported action '%s', must be one of: %v", action, supportedActions)
	}

	return nil
}

// ============================================================================
// ACTION HANDLERS - Each handler implements a specific DevOps enrichment action
// ============================================================================

// handleExtractStructure extracts C/C++ code structure (includes, defines, conditionals)
func (w *DevOpsWorker) handleExtractStructure(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
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
		fmt.Sprintf("Extracting C/C++ structure from %d documents", len(documents)), nil)

	// Create action handler
	action := actions.NewExtractStructureAction(w.documentStorage, w.logger)

	// Process each document
	successCount := 0
	skipCount := 0
	errorCount := 0

	for _, doc := range documents {
		// Get existing DevOps metadata to check if already processed
		alreadyProcessed := false
		if doc.Metadata != nil {
			if devopsData, ok := doc.Metadata["devops"]; ok {
				if devopsMap, ok := devopsData.(map[string]interface{}); ok {
					if passes, ok := devopsMap["enrichment_passes"].([]interface{}); ok {
						for _, pass := range passes {
							if passStr, ok := pass.(string); ok && passStr == "extract_structure" {
								alreadyProcessed = true
								break
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
								if pass == "extract_structure" {
									alreadyProcessed = true
									break
								}
							}
						}
					}
				}
			}
		}

		if err := action.Execute(ctx, doc, force); err != nil {
			w.logger.Error().
				Err(err).
				Str("doc_id", doc.ID).
				Str("title", doc.Title).
				Msg("Failed to extract structure")
			w.logJobEvent(ctx, stepID, step.Name, "error",
				fmt.Sprintf("Failed to extract structure from %s: %v", doc.Title, err), nil)
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
			successCount, skipCount, errorCount), nil)

	return stepID, nil
}

// handleAnalyzeBuildSystem parses build files (CMake, Makefile) for targets and dependencies
func (w *DevOpsWorker) handleAnalyzeBuildSystem(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	w.logger.Info().
		Str("step_id", stepID).
		Msg("Handling analyze_build_system action")

	// Extract metadata from init result
	documents, _ := initResult.Metadata["documents"].([]*models.Document)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get force flag from config (default: false)
	force := false
	if forceVal, ok := stepConfig["force"].(bool); ok {
		force = forceVal
	}

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Starting build system analysis for %d documents", len(documents)), nil)

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
				fmt.Sprintf("Build system analysis cancelled after processing %d documents", processed), nil)
			return stepID, ctx.Err()
		default:
		}

		// Execute action on document
		if err := action.Execute(ctx, doc, force); err != nil {
			w.logger.Error().
				Err(err).
				Str("document_id", doc.ID).
				Str("file_path", doc.FilePath).
				Msg("Failed to analyze build system for document")
			failed++
			continue
		}

		// Check if document was actually processed (is a build file)
		if action.IsBuildFile(doc.FilePath) {
			processed++
		} else {
			skipped++
		}
	}

	// Log completion
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Build system analysis completed: %d processed, %d skipped, %d failed",
			processed, skipped, failed), nil)

	w.logger.Info().
		Str("step_id", stepID).
		Int("processed", processed).
		Int("skipped", skipped).
		Int("failed", failed).
		Msg("Build system analysis completed")

	return stepID, nil
}

// handleClassifyDevOps performs LLM-based classification of file roles and components
func (w *DevOpsWorker) handleClassifyDevOps(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	w.logger.Info().
		Str("step_id", stepID).
		Msg("Handling classify_devops action")

	// Extract metadata from init result
	documents, _ := initResult.Metadata["documents"].([]*models.Document)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Get force flag from config (default: false)
	force := false
	if forceVal, ok := stepConfig["force"].(bool); ok {
		force = forceVal
	}

	// Create the classify action
	classifyAction := actions.NewClassifyDevOpsAction(
		w.documentStorage,
		w.llmService,
		w.logger,
	)

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Classifying %d documents using LLM", len(documents)), nil)

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
				fmt.Sprintf("Failed to classify %s: %v", doc.Title, err), nil)
		} else {
			// Check if document was actually processed or skipped
			if !force {
				// Check if document had already been classified
				alreadyClassified := false
				if doc.Metadata != nil {
					if devopsData, ok := doc.Metadata["devops"]; ok {
						if devopsMap, ok := devopsData.(map[string]interface{}); ok {
							if passes, ok := devopsMap["enrichment_passes"].([]interface{}); ok {
								for _, pass := range passes {
									if passStr, ok := pass.(string); ok && passStr == "classify_devops" {
										alreadyClassified = true
										break
									}
								}
							}
						}
					}
				}
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
				"Classification cancelled by context", nil)
			return stepID, fmt.Errorf("classification cancelled: %w", ctx.Err())
		}
	}

	// Log completion summary
	w.logJobEvent(ctx, stepID, step.Name, "info",
		fmt.Sprintf("Classification complete: %d succeeded, %d failed, %d skipped",
			successCount, failureCount, skippedCount), nil)

	w.logger.Info().
		Str("step_id", stepID).
		Int("success", successCount).
		Int("failure", failureCount).
		Int("skipped", skippedCount).
		Msg("Classify DevOps action completed")

	return stepID, nil
}

// handleBuildDependencyGraph builds dependency graph from extracted metadata
func (w *DevOpsWorker) handleBuildDependencyGraph(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	w.logger.Info().
		Str("step_id", stepID).
		Msg("Handling build_dependency_graph action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"Building dependency graph from DevOps metadata", nil)

	// Extract documents from init result
	documents, ok := initResult.Metadata["documents"].([]*models.Document)
	if !ok || len(documents) == 0 {
		w.logger.Warn().
			Str("step_id", stepID).
			Msg("No documents found in init result")
		w.logJobEvent(ctx, stepID, step.Name, "warn",
			"No documents found to build dependency graph", nil)
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
			fmt.Sprintf("Failed to build dependency graph: %v", err), nil)
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
		fmt.Sprintf("Successfully built dependency graph from %d documents", len(documents)), nil)

	return stepID, nil
}

// handleAggregateDevOpsSummary generates summary of all DevOps metadata
func (w *DevOpsWorker) handleAggregateDevOpsSummary(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	w.logger.Info().
		Str("step_id", stepID).
		Msg("Handling aggregate_devops_summary action")

	// Log step start for UI
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"Starting DevOps summary aggregation", nil)

	// Check if LLM service is available
	if w.llmService == nil {
		w.logger.Warn().Msg("LLM service not available, skipping aggregate_devops_summary")
		w.logJobEvent(ctx, stepID, step.Name, "warning",
			"LLM service not available - summary generation skipped", nil)
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
			fmt.Sprintf("Failed to aggregate summary: %v", err), nil)
		return stepID, fmt.Errorf("aggregate_devops_summary failed: %w", err)
	}

	w.logger.Info().
		Str("step_id", stepID).
		Msg("Successfully aggregated DevOps summary")
	w.logJobEvent(ctx, stepID, step.Name, "info",
		"DevOps summary successfully generated and stored", nil)

	return stepID, nil
}

// logJobEvent logs a job event for real-time UI display using the unified logging system
func (w *DevOpsWorker) logJobEvent(ctx context.Context, parentJobID, _, level, message string, _ map[string]interface{}) {
	if w.jobMgr == nil {
		return
	}
	w.jobMgr.AddJobLog(ctx, parentJobID, level, message)
}

// updateEnrichmentTracking updates the enrichment_passes field in DevOps metadata
func (w *DevOpsWorker) updateEnrichmentTracking(ctx context.Context, documents []*models.Document, passName string) error {
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
