package managers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs/queue"
	"github.com/ternarybob/quaero/internal/models"
	internalqueue "github.com/ternarybob/quaero/internal/queue"
)

// AgentManager creates parent agent jobs and orchestrates AI-powered document processing workflows
type AgentManager struct {
	jobMgr        *queue.Manager
	queueMgr      interfaces.QueueManager
	searchService interfaces.SearchService
	kvStorage     interfaces.KeyValueStorage
	authStorage   interfaces.AuthStorage
	eventService  interfaces.EventService
	logger        arbor.ILogger
}

// Compile-time assertion: AgentManager implements StepManager interface
var _ interfaces.StepManager = (*AgentManager)(nil)

// NewAgentManager creates a new agent manager
func NewAgentManager(
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	searchService interfaces.SearchService,
	kvStorage interfaces.KeyValueStorage,
	authStorage interfaces.AuthStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *AgentManager {
	return &AgentManager{
		jobMgr:        jobMgr,
		queueMgr:      queueMgr,
		searchService: searchService,
		kvStorage:     kvStorage,
		authStorage:   authStorage,
		eventService:  eventService,
		logger:        logger,
	}
}

// CreateParentJob creates a parent agent job, queries documents matching the filter, and enqueues
// individual agent jobs for each document. Returns the parent job ID for tracking.
func (m *AgentManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	// Parse step config
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract agent type from step config
	agentType, ok := stepConfig["agent_type"].(string)
	if !ok || agentType == "" {
		return "", fmt.Errorf("missing required config field: agent_type")
	}

	// Check for API key in step config and resolve it from KV store
	if apiKeyName, ok := stepConfig["api_key"].(string); ok && apiKeyName != "" {
		// Strip curly braces if present (e.g., "{google_gemini_api_key}" → "google_gemini_api_key")
		// This handles cases where variable substitution didn't happen during job definition loading
		cleanAPIKeyName := strings.Trim(apiKeyName, "{}")

		resolvedAPIKey, err := common.ResolveAPIKey(ctx, m.kvStorage, cleanAPIKeyName, "")
		if err != nil {
			return "", fmt.Errorf("failed to resolve API key '%s' from storage: %w", cleanAPIKeyName, err)
		}
		m.logger.Info().
			Str("step_name", step.Name).
			Str("api_key_name", cleanAPIKeyName).
			Msg("Resolved API key from storage for agent execution")
		stepConfig["resolved_api_key"] = resolvedAPIKey
	}

	// Extract document filter from step config (optional)
	// If not specified, process all documents for the job definition's source
	var documentFilter map[string]interface{}
	if filter, ok := stepConfig["document_filter"].(map[string]interface{}); ok {
		documentFilter = filter
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Str("parent_job_id", parentJobID).
		Msg("Creating parent agent job")

	// Query documents to process
	documents, err := m.queryDocuments(ctx, jobDef, documentFilter)
	if err != nil {
		return "", fmt.Errorf("failed to query documents for agent processing: %w", err)
	}

	if len(documents) == 0 {
		m.logger.Warn().
			Str("step_name", step.Name).
			Str("source_type", jobDef.SourceType).
			Msg("No documents found for agent processing")
		return parentJobID, nil // No documents to process, but not an error
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("document_count", len(documents)).
		Msg("Found documents for agent processing")

	// Create and enqueue agent jobs for each document
	jobIDs := make([]string, 0, len(documents))
	for _, doc := range documents {
		jobID, err := m.createAgentJob(ctx, agentType, doc.ID, stepConfig, parentJobID)
		if err != nil {
			m.logger.Warn().
				Err(err).
				Str("document_id", doc.ID).
				Str("agent_type", agentType).
				Msg("Failed to create agent job for document")
			continue
		}
		jobIDs = append(jobIDs, jobID)
	}

	if len(jobIDs) == 0 {
		errMsg := fmt.Sprintf("Failed to create any agent jobs for step %s", step.Name)

		// Publish error event for real-time display
		if m.eventService != nil {
			m.publishJobError(ctx, parentJobID, errMsg)
		}

		return "", fmt.Errorf("failed to create any agent jobs for step %s", step.Name)
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("jobs_created", len(jobIDs)).
		Msg("Agent jobs created and enqueued")

	// Poll for job completion (wait for all agent jobs to complete)
	if err := m.pollJobCompletion(ctx, jobIDs); err != nil {
		return "", fmt.Errorf("agent jobs did not complete successfully: %w", err)
	}

	m.logger.Info().
		Str("step_name", step.Name).
		Str("agent_type", agentType).
		Int("jobs_completed", len(jobIDs)).
		Msg("Agent job orchestration completed successfully")

	// Return parent job ID for chaining
	return parentJobID, nil
}

// GetManagerType returns "ai" - the action type this manager handles
func (m *AgentManager) GetManagerType() string {
	return "ai"
}

// ReturnsChildJobs returns true since agent creates child jobs for each document
func (m *AgentManager) ReturnsChildJobs() bool {
	return true
}

// queryDocuments queries documents to process based on job definition and filter
func (m *AgentManager) queryDocuments(ctx context.Context, jobDef *models.JobDefinition, filter map[string]interface{}) ([]*models.Document, error) {
	// Build search options based on job definition and filter
	opts := interfaces.SearchOptions{
		Limit: 1000, // Process up to 1000 documents per step
	}

	// Only filter by source type if specified in job definition
	// This allows agent jobs to process ALL documents when source_type is not specified
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
	results, err := m.searchService.Search(ctx, "", opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return results, nil
}

// createAgentJob creates and enqueues an agent job for a document
func (m *AgentManager) createAgentJob(ctx context.Context, agentType, documentID string, stepConfig map[string]interface{}, parentJobID string) (string, error) {
	// Build job config
	jobConfig := map[string]interface{}{
		"agent_type":  agentType,
		"document_id": documentID,
	}

	// Copy optional parameters from step config
	if maxKeywords, ok := stepConfig["max_keywords"]; ok {
		jobConfig["max_keywords"] = maxKeywords
	}

	// Create queue job
	queueJob := models.NewQueueJobChild(
		parentJobID,
		"ai",  // AI job type for AI-powered document processing
		fmt.Sprintf("AI: %s (document: %s)", agentType, documentID),
		jobConfig,
		map[string]interface{}{}, // metadata (must be non-nil)
		0,                        // depth (not used for AI jobs)
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
	if err := m.jobMgr.CreateJobRecord(ctx, &queue.Job{
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
	queueMsg := internalqueue.Message{
		JobID:   queueJob.ID,
		Type:    queueJob.Type,
		Payload: payloadBytes,
	}

	if err := m.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	m.logger.Debug().
		Str("job_id", queueJob.ID).
		Str("parent_job_id", parentJobID).
		Str("agent_type", agentType).
		Str("document_id", documentID).
		Msg("Agent job created and enqueued")

	return queueJob.ID, nil
}

// pollJobCompletion polls for job completion with timeout
func (m *AgentManager) pollJobCompletion(ctx context.Context, jobIDs []string) error {
	timeout := time.After(10 * time.Minute) // 10 minute timeout for agent jobs
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	m.logger.Debug().
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
				jobInterface, err := m.jobMgr.GetJob(ctx, jobID)
				if err != nil {
					m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to get job status")
					continue
				}

				// Type assert to *queue.Job
				job, ok := jobInterface.(*queue.Job)
				if !ok {
					m.logger.Warn().Str("job_id", jobID).Msg("Failed to type assert job")
					continue
				}

				if job.Status == "failed" {
					m.logger.Error().Str("job_id", jobID).Msg("Agent job failed")
					anyFailed = true
				}

				if job.Status != "completed" && job.Status != "failed" {
					allCompleted = false
				}
			}

			if anyFailed {
				return fmt.Errorf("one or more agent jobs failed")
			}

			if allCompleted {
				m.logger.Debug().
					Int("job_count", len(jobIDs)).
					Msg("All agent jobs completed successfully")
				return nil
			}
		}
	}
}

// publishJobError publishes a job error event for real-time display
func (m *AgentManager) publishJobError(ctx context.Context, jobID, errorMessage string) {
	if m.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"job_id":        jobID,
		"parent_job_id": jobID, // For parent jobs, parent_job_id is same as job_id
		"error_message": errorMessage,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    interfaces.EventJobError,
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking
	go func() {
		if err := m.eventService.Publish(ctx, event); err != nil {
			m.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish job error event")
		}
	}()
}
