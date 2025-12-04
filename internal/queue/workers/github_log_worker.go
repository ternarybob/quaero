// -----------------------------------------------------------------------
// GitHub Actions Worker - Unified worker implementing both DefinitionWorker and JobWorker
// - DefinitionWorker: Creates parent jobs and spawns child jobs for workflow runs
// - JobWorker: Processes individual GitHub action log jobs
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/githublogs"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubLogWorker handles GitHub Action Log jobs and implements both DefinitionWorker and JobWorker interfaces.
// - DefinitionWorker: Creates parent jobs and spawns child jobs for workflow runs
// - JobWorker: Processes individual GitHub action log jobs
type GitHubLogWorker struct {
	connectorService interfaces.ConnectorService
	jobManager       *queue.Manager
	queueMgr         interfaces.QueueManager
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
}

// Compile-time assertions: GitHubLogWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*GitHubLogWorker)(nil)
var _ interfaces.JobWorker = (*GitHubLogWorker)(nil)

// NewGitHubLogWorker creates a new GitHub log worker that implements both DefinitionWorker and JobWorker interfaces
func NewGitHubLogWorker(
	connectorService interfaces.ConnectorService,
	jobManager *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubLogWorker {
	return &GitHubLogWorker{
		connectorService: connectorService,
		jobManager:       jobManager,
		queueMgr:         queueMgr,
		documentStorage:  documentStorage,
		eventService:     eventService,
		logger:           logger,
	}
}

// GetWorkerType returns the job type this worker handles
func (w *GitHubLogWorker) GetWorkerType() string {
	return models.JobTypeGitHubActionLog
}

// Validate validates that the queue job is compatible with this worker
func (w *GitHubLogWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeGitHubActionLog {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeGitHubActionLog, job.Type)
	}
	return nil
}

// Execute processes a GitHub Action Log job
func (w *GitHubLogWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	w.logger.Debug().Str("job_id", job.ID).Msg("Processing GitHub Action Log job")

	// Update job status to running
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusRunning)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Try new-style config first (from GitHubActionsManager)
	owner, hasOwner := job.GetConfigString("owner")
	repo, hasRepo := job.GetConfigString("repo")
	runID, hasRunID := job.GetConfigInt("run_id")
	workflowName, _ := job.GetConfigString("workflow_name")
	runStartedAt, _ := job.GetConfigString("run_started_at")
	branch, _ := job.GetConfigString("branch")
	commitSHA, _ := job.GetConfigString("commit_sha")
	conclusion, _ := job.GetConfigString("conclusion")

	// Get connector ID from metadata
	connectorID, _ := job.GetMetadataString("connector_id")
	if connectorID == "" {
		// Fallback for legacy auth_id
		connectorID, _ = job.Metadata["auth_id"].(string)
	}

	// Get tags from metadata
	baseTags := getTagsFromMetadata(job.Metadata)

	var connector *models.Connector
	var err error
	var logContent string

	// Check if we have new-style config (from manager)
	if hasOwner && hasRepo && hasRunID {
		// New-style: fetch workflow run logs directly
		if connectorID != "" {
			connector, err = w.connectorService.GetConnector(ctx, connectorID)
			if err != nil {
				w.logger.Warn().Err(err).Str("connector_id", connectorID).Msg("Failed to find connector by ID")
			}
		}

		if connector == nil {
			connector, err = w.findGitHubConnector(ctx)
			if err != nil {
				return err
			}
		}

		ghConnector, err := github.NewConnector(connector)
		if err != nil {
			return fmt.Errorf("failed to create GitHub connector: %w", err)
		}

		logContent, err = ghConnector.GetWorkflowRunLogs(ctx, owner, repo, int64(runID))
		if err != nil {
			return fmt.Errorf("failed to fetch workflow run logs: %w", err)
		}

		// Parse run_started_at for proper timestamp
		var runTime time.Time
		if runStartedAt != "" {
			runTime, _ = time.Parse(time.RFC3339, runStartedAt)
		}

		// Create document with full metadata
		doc := &models.Document{
			ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
			SourceType:      models.SourceTypeGitHubActionLog,
			SourceID:        fmt.Sprintf("%s/%s/actions/runs/%d", owner, repo, runID),
			Title:           fmt.Sprintf("GitHub Actions: %s - %s/%s #%d", workflowName, owner, repo, runID),
			ContentMarkdown: logContent,
			URL:             fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", owner, repo, runID),
			Tags:            mergeTags(baseTags, []string{"github", "actions", repo, conclusion}),
			Metadata: map[string]interface{}{
				"owner":          owner,
				"repo":           repo,
				"run_id":         runID,
				"workflow_name":  workflowName,
				"run_started_at": runStartedAt,
				"run_date":       runTime.Format("2006-01-02"),
				"branch":         branch,
				"commit_sha":     commitSHA,
				"conclusion":     conclusion,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := w.documentStorage.SaveDocument(doc); err != nil {
			return fmt.Errorf("failed to save document: %w", err)
		}

		// Log document saved via Job Manager's unified logging (routes to WebSocket/DB)
		w.logDocumentSaved(ctx, job, doc.ID, doc.Title, workflowName)
	} else {
		// Legacy mode: parse URL and fetch job log
		seedURL, _ := job.Config["seed_url"].(string)
		if seedURL == "" {
			return fmt.Errorf("seed_url is required (legacy mode) or owner/repo/run_id (new mode)")
		}

		if connectorID != "" {
			connector, err = w.connectorService.GetConnector(ctx, connectorID)
			if err != nil {
				w.logger.Warn().Err(err).Str("connector_id", connectorID).Msg("Failed to find connector by ID")
			}
		}

		if connector == nil {
			connector, err = w.findGitHubConnector(ctx)
			if err != nil {
				return err
			}
		}

		ghConnector, err := github.NewConnector(connector)
		if err != nil {
			return fmt.Errorf("failed to create GitHub connector: %w", err)
		}

		// Parse URL
		owner, repo, jobID, err := githublogs.ParseLogURL(seedURL)
		if err != nil {
			return fmt.Errorf("failed to parse log url: %w", err)
		}

		// Fetch log
		logContent, err = ghConnector.GetJobLog(ctx, owner, repo, jobID)
		if err != nil {
			return fmt.Errorf("failed to fetch job log: %w", err)
		}

		// Save as document (legacy format)
		doc := &models.Document{
			ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
			SourceID:        fmt.Sprintf("%s/%s/job/%d", owner, repo, jobID),
			SourceType:      models.SourceTypeGitHubActionLog,
			Title:           fmt.Sprintf("GitHub Action Log: %s/%s Job %d", owner, repo, jobID),
			ContentMarkdown: logContent,
			URL:             seedURL,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
			Tags:            mergeTags(baseTags, []string{"github", "actions", repo}),
			Metadata: map[string]interface{}{
				"owner":  owner,
				"repo":   repo,
				"job_id": jobID,
			},
		}

		if err := w.documentStorage.SaveDocument(doc); err != nil {
			return fmt.Errorf("failed to save document: %w", err)
		}

		// Log document saved via Job Manager's unified logging (routes to WebSocket/DB)
		w.logDocumentSaved(ctx, job, doc.ID, doc.Title, "")
	}

	// Update job status to completed
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusCompleted)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update progress (completed=1, failed=0)
	if err := w.jobManager.UpdateJobProgress(ctx, job.ID, 1, 0); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update job progress")
	}

	w.logger.Debug().Str("job_id", job.ID).Msg("GitHub Action Log job completed successfully")
	return nil
}

// findGitHubConnector finds the first available GitHub connector
func (w *GitHubLogWorker) findGitHubConnector(ctx context.Context) (*models.Connector, error) {
	connectors, err := w.connectorService.ListConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}

	for _, c := range connectors {
		if c.Type == models.ConnectorTypeGitHub {
			return c, nil
		}
	}

	return nil, fmt.Errorf("no GitHub connector found")
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeGitHubActions for the DefinitionWorker interface
func (w *GitHubLogWorker) GetType() models.WorkerType {
	return models.WorkerTypeGitHubActions
}

// Init performs the initialization/setup phase for a GitHub Actions log step.
// This is where we validate configuration and prepare parameters.
func (w *GitHubLogWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract required config
	connectorID := getLogStringConfig(stepConfig, "connector_id", "")
	connectorName := getLogStringConfig(stepConfig, "connector_name", "")
	owner := getLogStringConfig(stepConfig, "owner", "")
	repo := getLogStringConfig(stepConfig, "repo", "")

	if connectorID == "" && connectorName == "" {
		return nil, fmt.Errorf("connector_id or connector_name is required")
	}
	if owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional config with defaults
	workflowFiles := getLogStringSliceConfig(stepConfig, "workflow_files", []string{})
	status := getLogStringConfig(stepConfig, "status", "failure")
	maxRuns := getLogIntConfig(stepConfig, "max_runs", 100)

	w.logger.Info().
		Str("phase", "step").
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Str("status", status).
		Int("max_runs", maxRuns).
		Msg("GitHub Actions log worker initialized")

	return &interfaces.WorkerInitResult{
		WorkItems: []interfaces.WorkItem{
			{
				ID:   fmt.Sprintf("%s/%s/actions", owner, repo),
				Name: fmt.Sprintf("Actions: %s/%s", owner, repo),
				Type: "github_actions",
			},
		},
		TotalCount:           1,
		Strategy:             interfaces.ProcessingStrategyParallel,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"owner":          owner,
			"repo":           repo,
			"workflow_files": workflowFiles,
			"status":         status,
			"max_runs":       maxRuns,
			"connector_id":   connectorID,
			"connector_name": connectorName,
			"step_config":    stepConfig,
		},
	}, nil
}

// CreateJobs creates a parent job and spawns child jobs for each workflow run.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
func (w *GitHubLogWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize github_actions worker: %w", err)
		}
	}

	stepConfig, _ := initResult.Metadata["step_config"].(map[string]interface{})
	if stepConfig == nil {
		stepConfig = step.Config
	}

	// Extract metadata from init result
	connectorID, _ := initResult.Metadata["connector_id"].(string)
	connectorName, _ := initResult.Metadata["connector_name"].(string)
	owner, _ := initResult.Metadata["owner"].(string)
	repo, _ := initResult.Metadata["repo"].(string)
	workflowFiles, _ := initResult.Metadata["workflow_files"].([]string)
	status, _ := initResult.Metadata["status"].(string)
	maxRuns, _ := initResult.Metadata["max_runs"].(int)

	w.logger.Info().
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Str("status", status).
		Int("max_runs", maxRuns).
		Msg("[worker] Creating GitHub Actions jobs from init result")

	_ = workflowFiles // TODO: filter by workflow files not yet implemented

	// Get GitHub connector - by ID or by name
	var connector *models.Connector
	var err error
	if connectorID != "" {
		connector, err = w.connectorService.GetConnector(ctx, connectorID)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by ID: %w", err)
		}
	} else {
		connector, err = w.connectorService.GetConnectorByName(ctx, connectorName)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by name '%s': %w", connectorName, err)
		}
		connectorID = connector.ID
	}

	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub connector: %w", err)
	}

	// Create parent job first
	parentJob := models.NewQueueJob(
		"github_actions_parent",
		fmt.Sprintf("GitHub Actions: %s/%s", owner, repo),
		map[string]interface{}{
			"owner":          owner,
			"repo":           repo,
			"workflow_files": workflowFiles,
			"status":         status,
		},
		map[string]interface{}{
			"connector_id":      connectorID,
			"job_definition_id": jobDef.ID,
			"tags":              jobDef.Tags,
		},
	)
	parentJob.ParentID = &stepID

	// Serialize parent job to JSON
	parentPayloadBytes, err := parentJob.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize parent job: %w", err)
	}

	// Create parent job record in database
	if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
		ID:              parentJob.ID,
		ParentID:        parentJob.ParentID,
		Type:            parentJob.Type,
		Name:            parentJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       parentJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   0,
		Payload:         string(parentPayloadBytes),
	}); err != nil {
		return "", fmt.Errorf("failed to create parent job record: %w", err)
	}

	// List workflow runs matching the criteria
	runs, err := ghConnector.ListWorkflowRuns(ctx, owner, repo, maxRuns, status, "")
	if err != nil {
		return "", fmt.Errorf("failed to list workflow runs: %w", err)
	}

	// Create child jobs for each run
	totalRunsEnqueued := 0

	for _, run := range runs {
		if totalRunsEnqueued >= maxRuns {
			break
		}

		// Create child job for each run
		childJob := models.NewQueueJobChild(
			parentJob.ID,
			models.JobTypeGitHubActionLog,
			fmt.Sprintf("Fetch Logs: %s/%s Run #%d", repo, run.WorkflowName, run.ID),
			map[string]interface{}{
				"owner":         owner,
				"repo":          repo,
				"run_id":        run.ID,
				"run_attempt":   run.RunAttempt,
				"workflow_name": run.WorkflowName,
				"status":        run.Status,
				"conclusion":    run.Conclusion,
				"url":           run.URL,
				"started_at":    run.RunStartedAt.Format(time.RFC3339),
			},
			map[string]interface{}{
				"connector_id":   connectorID,
				"tags":           jobDef.Tags,
				"root_parent_id": stepID,
			},
			parentJob.Depth+1,
		)

		// Serialize child job to JSON
		childPayloadBytes, err := childJob.ToJSON()
		if err != nil {
			w.logger.Warn().Err(err).
				Int64("run_id", run.ID).
				Msg("Failed to serialize child job, skipping")
			continue
		}

		// Create child job record in database
		if err := w.jobManager.CreateJobRecord(ctx, &queue.Job{
			ID:              childJob.ID,
			ParentID:        childJob.ParentID,
			Type:            childJob.Type,
			Name:            childJob.Name,
			Phase:           "execution",
			Status:          "pending",
			CreatedAt:       childJob.CreatedAt,
			ProgressCurrent: 0,
			ProgressTotal:   1,
			Payload:         string(childPayloadBytes),
		}); err != nil {
			w.logger.Warn().Err(err).
				Int64("run_id", run.ID).
				Msg("Failed to create child job record, skipping")
			continue
		}

		// Create queue message and enqueue
		queueMsg := models.QueueMessage{
			JobID:   childJob.ID,
			Type:    childJob.Type,
			Payload: childPayloadBytes,
		}

		if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
			w.logger.Warn().Err(err).
				Int64("run_id", run.ID).
				Msg("Failed to enqueue run job, skipping")
			continue
		}

		totalRunsEnqueued++
	}

	w.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", owner).
		Str("repo", repo).
		Int("runs_enqueued", totalRunsEnqueued).
		Msg("GitHub Actions parent job created, child jobs enqueued")

	// Update parent job with total count
	if err := w.jobManager.UpdateJobProgress(ctx, parentJob.ID, 0, totalRunsEnqueued); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	return parentJob.ID, nil
}

// ReturnsChildJobs returns true since GitHub Actions creates child jobs for each workflow run
func (w *GitHubLogWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for GitHub Actions type (DefinitionWorker interface)
func (w *GitHubLogWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("github_actions step requires config")
	}

	// Validate connector_id or connector_name
	connectorID, hasConnectorID := step.Config["connector_id"].(string)
	connectorName, hasConnectorName := step.Config["connector_name"].(string)

	if (!hasConnectorID || connectorID == "") && (!hasConnectorName || connectorName == "") {
		return fmt.Errorf("github_actions step requires either 'connector_id' or 'connector_name' in config")
	}

	// Validate required owner field
	owner, ok := step.Config["owner"].(string)
	if !ok || owner == "" {
		return fmt.Errorf("github_actions step requires 'owner' in config")
	}

	// Validate required repo field
	repo, ok := step.Config["repo"].(string)
	if !ok || repo == "" {
		return fmt.Errorf("github_actions step requires 'repo' in config")
	}

	return nil
}

// ============================================================================
// HELPER FUNCTIONS FOR CONFIG EXTRACTION
// ============================================================================

func getLogStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return defaultValue
}

func getLogIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	if v, ok := config[key].(int); ok {
		return v
	}
	return defaultValue
}

func getLogStringSliceConfig(config map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := config[key].([]string); ok {
		return v
	}
	if v, ok := config[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// logDocumentSaved logs a document saved event via Job Manager's unified logging.
func (w *GitHubLogWorker) logDocumentSaved(ctx context.Context, job *models.QueueJob, docID, title, workflowName string) {
	message := fmt.Sprintf("Document saved: %s (ID: %s)", title, docID[:8])
	if workflowName != "" {
		message = fmt.Sprintf("Document saved: %s - %s (ID: %s)", workflowName, title, docID[:8])
	}
	w.jobManager.AddJobLog(ctx, job.ID, "info", message)
}
