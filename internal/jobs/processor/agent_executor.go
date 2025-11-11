// -----------------------------------------------------------------------
// Agent Executor - Individual agent job execution with document processing
// -----------------------------------------------------------------------

package processor

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
)

// AgentExecutor executes agent jobs with document loading,
// agent processing, and metadata updates
type AgentExecutor struct {
	// Core dependencies
	agentService    interfaces.AgentService
	jobMgr          *jobs.Manager
	documentStorage interfaces.DocumentStorage
	logger          arbor.ILogger
	eventService    interfaces.EventService
}

// NewAgentExecutor creates a new agent executor
func NewAgentExecutor(
	agentService interfaces.AgentService,
	jobMgr *jobs.Manager,
	documentStorage interfaces.DocumentStorage,
	logger arbor.ILogger,
	eventService interfaces.EventService,
) *AgentExecutor {
	return &AgentExecutor{
		agentService:    agentService,
		jobMgr:          jobMgr,
		documentStorage: documentStorage,
		logger:          logger,
		eventService:    eventService,
	}
}

// GetJobType returns the job type this executor handles
func (e *AgentExecutor) GetJobType() string {
	return "agent"
}

// Validate validates that the job model is compatible with this executor
func (e *AgentExecutor) Validate(job *models.JobModel) error {
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
func (e *AgentExecutor) Execute(ctx context.Context, job *models.JobModel) error {
	// Create job-specific logger
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}
	jobLogger := e.logger.WithCorrelationId(parentID)

	// Extract configuration
	documentID, _ := job.GetConfigString("document_id")
	agentType, _ := job.GetConfigString("agent_type")

	jobLogger.Info().
		Str("job_id", job.ID).
		Str("document_id", documentID).
		Str("agent_type", agentType).
		Msg("Starting agent job execution")

	// Publish real-time log for job start
	e.publishAgentJobLog(ctx, parentID, "info", fmt.Sprintf("Starting agent processing: %s", agentType), map[string]interface{}{
		"document_id": documentID,
		"agent_type":  agentType,
		"job_id":      job.ID,
	})

	// Update job status to running
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Step 1: Load document from storage
	jobLogger.Debug().Str("document_id", documentID).Msg("Loading document from storage")
	e.publishAgentJobLog(ctx, parentID, "info", "Loading document from storage", map[string]interface{}{
		"document_id": documentID,
		"job_id":      job.ID,
	})

	doc, err := e.documentStorage.GetDocument(documentID)
	if err != nil {
		jobLogger.Error().Err(err).Str("document_id", documentID).Msg("Failed to load document")
		e.publishAgentJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to load document: %v", err), map[string]interface{}{
			"document_id": documentID,
			"job_id":      job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Document load failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to load document: %w", err)
	}

	jobLogger.Info().
		Str("document_id", documentID).
		Str("title", doc.Title).
		Int("content_size", len(doc.ContentMarkdown)).
		Msg("Document loaded successfully")

	e.publishAgentJobLog(ctx, parentID, "info", fmt.Sprintf("Document loaded: %s (%d bytes)", doc.Title, len(doc.ContentMarkdown)), map[string]interface{}{
		"document_id":  documentID,
		"title":        doc.Title,
		"content_size": len(doc.ContentMarkdown),
		"job_id":       job.ID,
	})

	// Step 2: Prepare agent input
	agentInput := map[string]interface{}{
		"document_id": documentID,
		"content":     doc.ContentMarkdown,
	}

	// Add optional parameters from job config
	if maxKeywords, ok := job.Config["max_keywords"]; ok {
		agentInput["max_keywords"] = maxKeywords
	}

	// Step 3: Execute agent
	jobLogger.Debug().Str("agent_type", agentType).Msg("Executing agent")
	e.publishAgentJobLog(ctx, parentID, "info", fmt.Sprintf("Executing %s agent", agentType), map[string]interface{}{
		"document_id": documentID,
		"agent_type":  agentType,
		"job_id":      job.ID,
	})

	startTime := time.Now()
	agentOutput, err := e.agentService.Execute(ctx, agentType, agentInput)
	duration := time.Since(startTime)

	if err != nil {
		jobLogger.Error().Err(err).Str("agent_type", agentType).Msg("Agent execution failed")
		e.publishAgentJobLog(ctx, parentID, "error", fmt.Sprintf("Agent execution failed: %v", err), map[string]interface{}{
			"document_id": documentID,
			"agent_type":  agentType,
			"job_id":      job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Agent execution failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("agent execution failed: %w", err)
	}

	jobLogger.Info().
		Str("agent_type", agentType).
		Dur("duration", duration).
		Msg("Agent execution completed successfully")

	e.publishAgentJobLog(ctx, parentID, "info", fmt.Sprintf("Agent execution completed in %v", duration), map[string]interface{}{
		"document_id": documentID,
		"agent_type":  agentType,
		"duration":    duration.String(),
		"job_id":      job.ID,
	})

	// Step 4: Update document metadata with agent results
	jobLogger.Debug().Msg("Updating document metadata with agent results")
	e.publishAgentJobLog(ctx, parentID, "info", "Updating document metadata", map[string]interface{}{
		"document_id": documentID,
		"job_id":      job.ID,
	})

	// Initialize metadata if nil
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}

	// Store agent results under agent type key
	doc.Metadata[agentType] = agentOutput

	// Update document in storage
	if err := e.documentStorage.UpdateDocument(doc); err != nil {
		jobLogger.Error().Err(err).Str("document_id", documentID).Msg("Failed to update document metadata")
		e.publishAgentJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to update document metadata: %v", err), map[string]interface{}{
			"document_id": documentID,
			"job_id":      job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Metadata update failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to update document metadata: %w", err)
	}

	jobLogger.Info().
		Str("document_id", documentID).
		Msg("Document metadata updated successfully")

	e.publishAgentJobLog(ctx, parentID, "info", "Document metadata updated successfully", map[string]interface{}{
		"document_id": documentID,
		"job_id":      job.ID,
	})

	// Add job log for successful completion
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Agent processing completed: %s (document: %s)",
		agentType, documentID))

	// Step 5: Publish DocumentSaved event (reusing existing event type)
	if e.eventService != nil {
		event := interfaces.Event{
			Type: interfaces.EventDocumentSaved,
			Payload: map[string]interface{}{
				"job_id":        job.ID,
				"parent_job_id": parentID,
				"document_id":   documentID,
				"source_url":    doc.URL,
				"timestamp":     time.Now().Format(time.RFC3339),
			},
		}

		go func() {
			if err := e.eventService.Publish(context.Background(), event); err != nil {
				jobLogger.Warn().Err(err).Msg("Failed to publish DocumentSaved event")
			}
		}()
	}

	// Update job status to completed
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	totalTime := time.Since(startTime)
	jobLogger.Info().
		Str("job_id", job.ID).
		Str("document_id", documentID).
		Str("agent_type", agentType).
		Dur("total_time", totalTime).
		Msg("Agent job execution completed successfully")

	e.publishAgentJobLog(ctx, parentID, "info", fmt.Sprintf("Agent job completed successfully in %v", totalTime), map[string]interface{}{
		"document_id": documentID,
		"agent_type":  agentType,
		"total_time":  totalTime.String(),
		"status":      "completed",
		"job_id":      job.ID,
	})

	return nil
}

// publishAgentJobLog publishes an agent job log event for real-time streaming
func (e *AgentExecutor) publishAgentJobLog(ctx context.Context, jobID, level, message string, metadata map[string]interface{}) {
	if e.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"job_id":    jobID,
		"level":     level,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if metadata != nil {
		payload["metadata"] = metadata
	}

	event := interfaces.Event{
		Type:    "agent_job_log",
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking job execution
	go func() {
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish agent job log event")
		}
	}()
}
