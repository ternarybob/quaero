package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubActionsManager creates parent GitHub actions jobs and spawns child jobs for each workflow run
type GitHubActionsManager struct {
	connectorService interfaces.ConnectorService
	jobMgr           *queue.Manager
	queueMgr         interfaces.QueueManager
	logger           arbor.ILogger
}

// Compile-time assertion: GitHubActionsManager implements StepManager interface
var _ interfaces.StepManager = (*GitHubActionsManager)(nil)

// NewGitHubActionsManager creates a new GitHub actions manager
func NewGitHubActionsManager(
	connectorService interfaces.ConnectorService,
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	logger arbor.ILogger,
) *GitHubActionsManager {
	return &GitHubActionsManager{
		connectorService: connectorService,
		jobMgr:           jobMgr,
		queueMgr:         queueMgr,
		logger:           logger,
	}
}

// GetManagerType returns "github_actions_fetch" - the action type this manager handles
func (m *GitHubActionsManager) GetManagerType() string {
	return "github_actions_fetch"
}

// ReturnsChildJobs returns true since this manager creates child jobs for each workflow run
func (m *GitHubActionsManager) ReturnsChildJobs() bool {
	return true
}

// CreateParentJob creates a parent job and spawns children for each workflow run
func (m *GitHubActionsManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract required config
	connectorID := getStringConfig(stepConfig, "connector_id", "")
	connectorName := getStringConfig(stepConfig, "connector_name", "")
	owner := getStringConfig(stepConfig, "owner", "")
	repo := getStringConfig(stepConfig, "repo", "")

	if connectorID == "" && connectorName == "" {
		return "", fmt.Errorf("connector_id or connector_name is required")
	}
	if owner == "" {
		return "", fmt.Errorf("owner is required")
	}
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}

	// Extract optional config with defaults
	limit := getIntConfig(stepConfig, "limit", 10)
	statusFilter := getStringConfig(stepConfig, "status_filter", "")
	branchFilter := getStringConfig(stepConfig, "branch_filter", "")

	m.logger.Debug().
		Str("step_name", step.Name).
		Str("connector_id", connectorID).
		Str("connector_name", connectorName).
		Str("owner", owner).
		Str("repo", repo).
		Int("limit", limit).
		Msg("Creating GitHub actions parent job")

	// Get GitHub connector - by ID or by name
	var connector *models.Connector
	var err error
	if connectorID != "" {
		connector, err = m.connectorService.GetConnector(ctx, connectorID)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by ID: %w", err)
		}
	} else {
		connector, err = m.connectorService.GetConnectorByName(ctx, connectorName)
		if err != nil {
			return "", fmt.Errorf("failed to get connector by name '%s': %w", connectorName, err)
		}
		// Set connectorID for downstream use (metadata, child jobs)
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
			"owner":         owner,
			"repo":          repo,
			"limit":         limit,
			"status_filter": statusFilter,
			"branch_filter": branchFilter,
		},
		map[string]interface{}{
			"connector_id":      connectorID,
			"job_definition_id": jobDef.ID,
			"tags":              jobDef.Tags,
		},
	)
	parentJob.ParentID = &parentJobID

	// Serialize parent job to JSON
	parentPayloadBytes, err := parentJob.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize parent job: %w", err)
	}

	// Create parent job record in database
	if err := m.jobMgr.CreateJobRecord(ctx, &queue.Job{
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

	// List workflow runs
	runs, err := ghConnector.ListWorkflowRuns(ctx, owner, repo, limit, statusFilter, branchFilter)
	if err != nil {
		m.logger.Error().Err(err).Msg("Failed to list workflow runs")
		return "", fmt.Errorf("failed to list workflow runs: %w", err)
	}

	// Enqueue child jobs for each workflow run
	totalRunsEnqueued := 0

	for _, run := range runs {
		// Create child job for each run
		childJob := models.NewQueueJobChild(
			parentJob.ID,
			models.JobTypeGitHubActionLog,
			fmt.Sprintf("Fetch log: %s/%s run %d", owner, repo, run.ID),
			map[string]interface{}{
				"owner":          owner,
				"repo":           repo,
				"run_id":         run.ID,
				"workflow_name":  run.WorkflowName,
				"run_started_at": run.RunStartedAt.Format(time.RFC3339),
				"branch":         run.Branch,
				"commit_sha":     run.CommitSHA,
				"conclusion":     run.Conclusion,
			},
			map[string]interface{}{
				"connector_id":   connectorID,
				"tags":           jobDef.Tags,
				"root_parent_id": parentJobID, // Root parent (JobDefParent) for document count tracking
			},
			parentJob.Depth+1,
		)

		// Serialize child job to JSON
		childPayloadBytes, err := childJob.ToJSON()
		if err != nil {
			m.logger.Warn().Err(err).
				Int64("run_id", run.ID).
				Msg("Failed to serialize child job, skipping")
			continue
		}

		// Create child job record in database
		if err := m.jobMgr.CreateJobRecord(ctx, &queue.Job{
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
			m.logger.Warn().Err(err).
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

		if err := m.queueMgr.Enqueue(ctx, queueMsg); err != nil {
			m.logger.Warn().Err(err).
				Int64("run_id", run.ID).
				Msg("Failed to enqueue workflow run job, skipping")
			continue
		}

		totalRunsEnqueued++
	}

	m.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", owner).
		Str("repo", repo).
		Int("runs_enqueued", totalRunsEnqueued).
		Msg("GitHub actions parent job created, child jobs enqueued")

	// Update parent job with total count
	if err := m.jobMgr.UpdateJobProgress(ctx, parentJob.ID, 0, totalRunsEnqueued); err != nil {
		m.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	return parentJob.ID, nil
}
