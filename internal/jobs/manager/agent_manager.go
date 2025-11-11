package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// AgentManager creates parent agent jobs and orchestrates AI-powered document processing workflows
type AgentManager struct {
	jobMgr        *jobs.Manager
	queueMgr      *queue.Manager
	searchService interfaces.SearchService
	logger        arbor.ILogger
}

// Compile-time assertion: AgentManager implements StepManager interface
var _ interfaces.StepManager = (*AgentManager)(nil)

// NewAgentManager creates a new agent manager
func NewAgentManager(
	jobMgr *jobs.Manager,
	queueMgr *queue.Manager,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
) *AgentManager {
	return &AgentManager{
		jobMgr:        jobMgr,
		queueMgr:      queueMgr,
		searchService: searchService,
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

// GetManagerType returns "agent" - the action type this manager handles
func (m *AgentManager) GetManagerType() string {
	return "agent"
}

// queryDocuments queries documents to process based on job definition and filter
func (m *AgentManager) queryDocuments(ctx context.Context, jobDef *models.JobDefinition, filter map[string]interface{}) ([]*models.Document, error) {
	// Build search options based on job definition and filter
	opts := interfaces.SearchOptions{
		SourceTypes: []string{jobDef.SourceType},
		Limit:       1000, // Process up to 1000 documents per step
	}

	// Apply additional filters if specified
	if filter != nil {
		// Support limit override
		if limit, ok := filter["limit"].(int); ok && limit > 0 {
			opts.Limit = limit
		} else if limitFloat, ok := filter["limit"].(float64); ok && limitFloat > 0 {
			opts.Limit = int(limitFloat)
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

	// Create job model
	jobModel := models.NewChildJobModel(
		parentJobID,
		"agent",
		fmt.Sprintf("Agent: %s (document: %s)", agentType, documentID),
		jobConfig,
		nil,   // metadata
		0,     // depth (not used for agent jobs)
	)

	// Validate job model
	if err := jobModel.Validate(); err != nil {
		return "", fmt.Errorf("invalid job model: %w", err)
	}

	// Serialize job model to JSON
	payloadBytes, err := jobModel.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize job model: %w", err)
	}

	// Create job record in database
	if err := m.jobMgr.CreateJobRecord(ctx, &jobs.Job{
		ID:              jobModel.ID,
		ParentID:        jobModel.ParentID,
		Type:            jobModel.Type,
		Name:            jobModel.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       jobModel.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   1,
		Payload:         string(payloadBytes),
	}); err != nil {
		return "", fmt.Errorf("failed to create job record: %w", err)
	}

	// Enqueue job
	queueMsg := queue.Message{
		JobID:   jobModel.ID,
		Type:    jobModel.Type,
		Payload: payloadBytes,
	}

	if err := m.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}

	m.logger.Debug().
		Str("job_id", jobModel.ID).
		Str("parent_job_id", parentJobID).
		Str("agent_type", agentType).
		Str("document_id", documentID).
		Msg("Agent job created and enqueued")

	return jobModel.ID, nil
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

				// Type assert to *jobs.Job
				job, ok := jobInterface.(*jobs.Job)
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
