// -----------------------------------------------------------------------
// Agent Worker - Unified worker implementing both DefinitionWorker and JobWorker
// - DefinitionWorker: Creates and enqueues agent jobs for documents
// - JobWorker: Executes individual agent jobs with document processing
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// AgentWorker processes agent jobs and implements both DefinitionWorker and JobWorker interfaces.
// - DefinitionWorker: Creates and enqueues agent jobs for documents matching filter criteria
// - JobWorker: Executes individual agent jobs from the queue (document processing with AI agents)
type AgentWorker struct {
	agentService    interfaces.AgentService
	jobMgr          *queue.Manager
	queueMgr        interfaces.QueueManager
	searchService   interfaces.SearchService
	kvStorage       interfaces.KeyValueStorage
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	eventService    interfaces.EventService
}

// Compile-time assertions: AgentWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*AgentWorker)(nil)
var _ interfaces.JobWorker = (*AgentWorker)(nil)

// NewAgentWorker creates a new agent worker that implements both DefinitionWorker and JobWorker interfaces
func NewAgentWorker(
	agentService interfaces.AgentService,
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	searchService interfaces.SearchService,
	kvStorage interfaces.KeyValueStorage,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	eventService interfaces.EventService,
) *AgentWorker {
	return &AgentWorker{
		agentService:    agentService,
		jobMgr:          jobMgr,
		queueMgr:        queueMgr,
		searchService:   searchService,
		kvStorage:       kvStorage,
		documentStorage: documentStorage,
		logger:          logger,
		eventService:    eventService,
	}
}

// GetWorkerType returns "agent" - the job type this worker handles
func (w *AgentWorker) GetWorkerType() string {
	return "agent"
}

// Validate validates that the queue job is compatible with this worker
func (w *AgentWorker) Validate(job *models.QueueJob) error {
	if job.Type != "agent" {
		return fmt.Errorf("invalid job type: expected %s, got %s", "agent", job.Type)
	}

	// Validate required config fields
	if _, ok := job.GetConfigString("document_id"); !ok {
		return fmt.Errorf("missing required config field: document_id")
	}

	if _, ok := job.GetConfigString("agent_type"); !ok {
		return fmt.Errorf("missing required config field: agent_type")
	}

	return nil
}

// Execute executes an agent job with full workflow:
// 1. Load document from storage
// 2. Execute agent with document content
// 3. Update document metadata with agent results
// 4. Publish DocumentUpdated event
func (w *AgentWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	// Create job-specific logger
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}
	jobLogger := w.logger.WithCorrelationId(parentID)

	// Extract configuration
	documentID, _ := job.GetConfigString("document_id")
	agentType, _ := job.GetConfigString("agent_type")

	jobLogger.Debug().
		Str("job_id", job.ID).
		Str("document_id", documentID).
		Str("agent_type", agentType).
		Msg("Starting agent job execution")

	// Log job start
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("AI: %s (document: %s) - starting", agentType, documentID))

	// Update job status to running
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Step 1: Load document from storage
	jobLogger.Trace().Str("document_id", documentID).Msg("Loading document from storage")

	doc, err := w.documentStorage.GetDocument(documentID)
	if err != nil {
		jobLogger.Error().Err(err).Str("document_id", documentID).Msg("Failed to load document")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("AI: %s (document: %s) - failed to load document: %v", agentType, documentID, err))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Document load failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to load document: %w", err)
	}

	jobLogger.Trace().
		Str("document_id", documentID).
		Str("title", doc.Title).
		Int("content_size", len(doc.ContentMarkdown)).
		Msg("Document loaded successfully")

	// Step 2: Prepare agent input
	agentInput := map[string]interface{}{
		"document_id": documentID,
		"content":     doc.ContentMarkdown,
	}

	// Add optional parameters from job config
	if maxKeywords, ok := job.Config["max_keywords"]; ok {
		agentInput["max_keywords"] = maxKeywords
	}

	// Add Gemini override settings from job config (allows per-job API key, model, etc.)
	if apiKey, ok := job.Config["gemini_api_key"].(string); ok && apiKey != "" {
		agentInput["gemini_api_key"] = apiKey
	}
	if model, ok := job.Config["gemini_model"].(string); ok && model != "" {
		agentInput["gemini_model"] = model
	}
	if timeout, ok := job.Config["gemini_timeout"].(string); ok && timeout != "" {
		agentInput["gemini_timeout"] = timeout
	}
	if rateLimit, ok := job.Config["gemini_rate_limit"].(string); ok && rateLimit != "" {
		agentInput["gemini_rate_limit"] = rateLimit
	}

	// Step 3: Execute agent
	jobLogger.Trace().Str("agent_type", agentType).Msg("Executing agent")

	startTime := time.Now()
	agentOutput, err := w.agentService.Execute(ctx, agentType, agentInput)
	duration := time.Since(startTime)

	if err != nil {
		jobLogger.Error().Err(err).Str("agent_type", agentType).Msg("Agent execution failed")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("AI: %s (document: %s) - failed: %v", agentType, documentID, err))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Agent execution failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("agent execution failed: %w", err)
	}

	jobLogger.Trace().
		Str("agent_type", agentType).
		Dur("duration", duration).
		Msg("Agent execution completed successfully")

	// Step 4: Update document metadata with agent results
	jobLogger.Trace().Msg("Updating document metadata with agent results")

	// Initialize metadata if nil
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	// Store agent results under agent type key
	doc.Metadata[agentType] = agentOutput

	// Update document in storage
	if err := w.documentStorage.UpdateDocument(doc); err != nil {
		jobLogger.Error().Err(err).Str("document_id", documentID).Msg("Failed to update document metadata")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("AI: %s (document: %s) - failed to save: %v", agentType, documentID, err))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Metadata update failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to update document metadata: %w", err)
	}

	jobLogger.Trace().
		Str("document_id", documentID).
		Msg("Document metadata updated successfully")

	// Add job log for successful completion (auto-resolves step context and publishes to WebSocket)
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Agent processing completed: %s (document: %s)",
		agentType, documentID))

	// Update job status to completed
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	totalTime := time.Since(startTime)
	jobLogger.Debug().
		Str("job_id", job.ID).
		Str("document_id", documentID).
		Str("agent_type", agentType).
		Dur("total_time", totalTime).
		Msg("Agent job execution completed successfully")

	// Log completion with timing (context auto-resolved)
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("AI: %s (document: %s) - completed in %v", agentType, documentID, totalTime))

	return nil
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeAgent for the DefinitionWorker interface
func (w *AgentWorker) GetType() models.WorkerType {
	return models.WorkerTypeAgent
}

// Init performs the initialization/setup phase for an agent step.
// This is where we:
//   - Extract and validate configuration
//   - Query documents matching the filter criteria
//   - Return document list as work items
//
// The Init phase does NOT create any jobs - it only gathers information.
func (w *AgentWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract agent type from step config
	agentType, ok := stepConfig["agent_type"].(string)
	if !ok || agentType == "" {
		return nil, fmt.Errorf("missing required config field: agent_type")
	}

	// Check for API key in step config and resolve it from KV store
	resolvedAPIKey := ""
	if apiKeyName, ok := stepConfig["api_key"].(string); ok && apiKeyName != "" {
		cleanAPIKeyName := strings.Trim(apiKeyName, "{}")
		var err error
		resolvedAPIKey, err = common.ResolveAPIKey(ctx, w.kvStorage, cleanAPIKeyName, "")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
		}
		w.logger.Debug().
			Str("phase", "step").
			Str("step_name", step.Name).
			Str("api_key_name", cleanAPIKeyName).
			Msg("Resolved API key from storage")
	}

	// Extract document filter from step config
	documentFilter := make(map[string]interface{})
	for k, v := range stepConfig {
		if len(k) > 7 && k[:7] == "filter_" {
			filterKey := k[7:]
			documentFilter[filterKey] = v
		}
	}

	w.logger.Info().
		Str("phase", "step").
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Msg("Initializing agent worker - querying documents")

	// Query documents to process
	documents, err := w.queryDocuments(ctx, &jobDef, documentFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents for agent processing: %w", err)
	}

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

	w.logger.Info().
		Str("phase", "step").
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("document_count", len(documents)).
		Msg("Agent worker initialized - found documents")

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(documents),
		Strategy:             interfaces.ProcessingStrategyParallel,
		SuggestedConcurrency: 5, // Reasonable default for API calls
		Metadata: map[string]interface{}{
			"agent_type":       agentType,
			"resolved_api_key": resolvedAPIKey,
			"document_filter":  documentFilter,
			"step_config":      stepConfig,
		},
	}, nil
}

// CreateJobs creates agent jobs for documents matching the filter criteria.
// Queries documents based on job definition, creates child jobs for each document,
// and enqueues them for processing.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
// If initResult is provided, it uses the document list from init.
func (w *AgentWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize agent worker: %w", err)
		}
	}

	// Extract metadata from init result
	agentType, _ := initResult.Metadata["agent_type"].(string)
	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})

	// Apply resolved API key if available
	if resolvedAPIKey, ok := initResult.Metadata["resolved_api_key"].(string); ok && resolvedAPIKey != "" {
		if stepConfig == nil {
			stepConfig = make(map[string]interface{})
		}
		stepConfig["resolved_api_key"] = resolvedAPIKey
	}

	// Get manager_id from step job's parent_id for event aggregation
	managerID := ""
	if stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID); err == nil && stepJobInterface != nil {
		if stepJob, ok := stepJobInterface.(*models.QueueJobState); ok && stepJob != nil && stepJob.ParentID != nil {
			managerID = *stepJob.ParentID
		}
	}

	// Check if there are any work items
	if len(initResult.WorkItems) == 0 {
		w.logger.Warn().
			Str("phase", "step").
			Str("step_name", step.Name).
			Str("source_type", jobDef.SourceType).
			Msg("No documents found for agent processing")
		return stepID, nil
	}

	w.logger.Info().
		Str("phase", "step").
		Str("originator", "worker").
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("document_count", len(initResult.WorkItems)).
		Msg("Creating agent jobs from init result")

	// Create and enqueue agent jobs for each document
	jobIDs := make([]string, 0, len(initResult.WorkItems))
	for _, workItem := range initResult.WorkItems {
		docID := workItem.ID
		jobID, err := w.createAgentJob(ctx, agentType, docID, stepConfig, stepID, step.Name, managerID)
		if err != nil {
			w.logger.Warn().
				Err(err).
				Str("document_id", docID).
				Str("agent_type", agentType).
				Msg("[worker] Failed to create agent job for document")
			continue
		}
		jobIDs = append(jobIDs, jobID)
	}

	if len(jobIDs) == 0 {
		errMsg := fmt.Sprintf("Failed to create any agent jobs for step %s", step.Name)
		w.jobMgr.AddJobLog(ctx, stepID, "error", errMsg)
		return "", fmt.Errorf("failed to create any agent jobs for step %s", step.Name)
	}

	w.logger.Info().
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("jobs_created", len(jobIDs)).
		Msg("[worker] Agent jobs created and enqueued")

	// Poll for job completion (wait for all agent jobs to complete)
	if err := w.pollJobCompletion(ctx, jobIDs); err != nil {
		return "", fmt.Errorf("agent jobs did not complete successfully: %w", err)
	}

	w.logger.Info().
		Str("phase", "step").
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("jobs_completed", len(jobIDs)).
		Msg("Agent job orchestration completed")

	return stepID, nil
}

// ReturnsChildJobs returns true since agent creates child jobs for each document
func (w *AgentWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for agent type (DefinitionWorker interface)
func (w *AgentWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("agent step requires config")
	}

	// Validate required agent_type field
	agentType, ok := step.Config["agent_type"].(string)
	if !ok || agentType == "" {
		return fmt.Errorf("agent step requires 'agent_type' in config")
	}

	// Validate known agent types
	validAgentTypes := map[string]bool{
		"keyword_extractor":   true,
		"document_generator":  true,
		"web_enricher":        true,
		"content_summarizer":  true,
		"metadata_enricher":   true,
		"sentiment_analyzer":  true,
		"entity_recognizer":   true,
		"category_classifier": true,
		"relation_extractor":  true,
		"question_answerer":   true,
	}

	if !validAgentTypes[agentType] {
		return fmt.Errorf("unknown agent_type: %s", agentType)
	}

	return nil
}

// ============================================================================
// HELPER METHODS FOR JOB CREATION
// ============================================================================

// queryDocuments queries documents to process based on job definition and filter
func (w *AgentWorker) queryDocuments(ctx context.Context, jobDef *models.JobDefinition, filter map[string]interface{}) ([]*models.Document, error) {
	// Build search options based on job definition and filter
	opts := interfaces.SearchOptions{
		Limit: 1000, // Process up to 1000 documents per step
	}

	// Only filter by source type if specified in job definition
	if jobDef.SourceType != "" {
		opts.SourceTypes = []string{jobDef.SourceType}
	}

	// Apply additional filters if specified
	if filter != nil {
		// Support source_type override in filter
		if sourceType, ok := filter["source_type"].(string); ok && sourceType != "" {
			opts.SourceTypes = []string{sourceType}
		}

		// Support tags filter - only return documents with ALL specified tags
		if tags, ok := filter["tags"].([]interface{}); ok && len(tags) > 0 {
			tagStrings := make([]string, 0, len(tags))
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					tagStrings = append(tagStrings, tagStr)
				}
			}
			if len(tagStrings) > 0 {
				opts.Tags = tagStrings
			}
		} else if tags, ok := filter["tags"].([]string); ok && len(tags) > 0 {
			opts.Tags = tags
		}

		// Support limit override
		if limit, ok := filter["limit"].(int); ok && limit > 0 {
			opts.Limit = limit
		} else if limitFloat, ok := filter["limit"].(float64); ok && limitFloat > 0 {
			opts.Limit = int(limitFloat)
		}

		// Support created_after date filter
		if createdAfter, ok := filter["created_after"].(string); ok && createdAfter != "" {
			opts.CreatedAfter = createdAfter
		}

		// Support updated_after date filter
		if updatedAfter, ok := filter["updated_after"].(string); ok && updatedAfter != "" {
			opts.UpdatedAfter = updatedAfter
		}
	}

	// Search documents (empty query returns all documents matching filters)
	results, err := w.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return results, nil
}

// createAgentJob creates and enqueues an agent job for a document
func (w *AgentWorker) createAgentJob(ctx context.Context, agentType, documentID string, stepConfig map[string]interface{}, parentJobID string, stepName string, managerID string) (string, error) {
	// Build job config
	jobConfig := map[string]interface{}{
		"agent_type":  agentType,
		"document_id": documentID,
	}

	// Copy optional parameters from step config
	if maxKeywords, ok := stepConfig["max_keywords"]; ok {
		jobConfig["max_keywords"] = maxKeywords
	}

	// Copy Gemini settings from step config (allows per-job override)
	if resolvedAPIKey, ok := stepConfig["resolved_api_key"].(string); ok && resolvedAPIKey != "" {
		jobConfig["gemini_api_key"] = resolvedAPIKey
	}
	if model, ok := stepConfig["model"].(string); ok && model != "" {
		jobConfig["gemini_model"] = model
	}
	if timeout, ok := stepConfig["timeout"].(string); ok && timeout != "" {
		jobConfig["gemini_timeout"] = timeout
	}
	if rateLimit, ok := stepConfig["rate_limit"].(string); ok && rateLimit != "" {
		jobConfig["gemini_rate_limit"] = rateLimit
	}

	// Create queue job with metadata for UI filtering and event aggregation
	// - step_name: Human-readable step name (for UI grouping)
	// - step_id: The step job's ID (parent_id for hierarchy tracking)
	// - manager_id: The manager job's ID (for top-level event aggregation)
	metadata := map[string]interface{}{
		"step_name":  stepName,    // Used by UI to group children under step rows
		"step_id":    parentJobID, // Used by step_progress events
		"manager_id": managerID,   // Used by job_log events for aggregation
	}
	queueJob := models.NewQueueJobChild(
		parentJobID,
		"agent", // Agent job type for AI-powered document processing
		fmt.Sprintf("AI: %s (document: %s)", agentType, documentID),
		jobConfig,
		metadata,
		0, // depth (not used for AI jobs)
	)

	// Validate queue job
	if err := queueJob.Validate(); err != nil {
		return "", fmt.Errorf("invalid queue job: %w", err)
	}

	// Serialize queue job to JSON
	payloadBytes, err := queueJob.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize queue job: %w", err)
	}

	// Create job record in database
	if err := w.jobMgr.CreateJobRecord(ctx, &queue.Job{
		ID:              queueJob.ID,
		ParentID:        queueJob.ParentID,
		Type:            queueJob.Type,
		Name:            queueJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       queueJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   1,
		Payload:         string(payloadBytes),
	}); err != nil {
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Enqueue job
	queueMsg := queue.Message{
		JobID:   queueJob.ID,
		Type:    queueJob.Type,
		Payload: payloadBytes,
	}

	if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	w.logger.Debug().
		Str("job_id", queueJob.ID).
		Str("parent_job_id", parentJobID).
		Str("agent_type", agentType).
		Str("document_id", documentID).
		Msg("Agent job created and enqueued")

	return queueJob.ID, nil
}

// pollJobCompletion polls for job completion with timeout
func (w *AgentWorker) pollJobCompletion(ctx context.Context, jobIDs []string) error {
	timeout := time.After(10 * time.Minute) // 10 minute timeout for agent jobs
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	w.logger.Debug().
		Int("job_count", len(jobIDs)).
		Msg("Polling for agent job completion")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for agent jobs to complete")
		case <-ticker.C:
			// Check job statuses
			allCompleted := true
			anyFailed := false
			for _, jobID := range jobIDs {
				jobInterface, err := w.jobMgr.GetJob(ctx, jobID)
				if err != nil {
					w.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get job status")
					continue
				}

				// Type assert to *models.QueueJobState
				jobState, ok := jobInterface.(*models.QueueJobState)
				if !ok {
					w.logger.Warn().Str("job_id", jobID).Msg("Failed to type assert job to QueueJobState")
					continue
				}

				status := string(jobState.Status)
				if status == "failed" {
					w.logger.Error().Str("job_id", jobID).Msg("Agent job failed")
					anyFailed = true
				}

				if status != "completed" && status != "failed" {
					allCompleted = false
				}
			}

			if anyFailed {
				return fmt.Errorf("one or more agent jobs failed")
			}

			if allCompleted {
				w.logger.Debug().
					Int("job_count", len(jobIDs)).
					Msg("All agent jobs completed successfully")
				return nil
			}
		}
	}
}
