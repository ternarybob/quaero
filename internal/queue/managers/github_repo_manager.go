package managers

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubRepoManager creates parent GitHub repo jobs and spawns child jobs for each file
type GitHubRepoManager struct {
	connectorService interfaces.ConnectorService
	jobMgr           *queue.Manager
	queueMgr         interfaces.QueueManager
	logger           arbor.ILogger
}

// Compile-time assertion: GitHubRepoManager implements StepManager interface
var _ interfaces.StepManager = (*GitHubRepoManager)(nil)

// NewGitHubRepoManager creates a new GitHub repo manager
func NewGitHubRepoManager(
	connectorService interfaces.ConnectorService,
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	logger arbor.ILogger,
) *GitHubRepoManager {
	return &GitHubRepoManager{
		connectorService: connectorService,
		jobMgr:           jobMgr,
		queueMgr:         queueMgr,
		logger:           logger,
	}
}

// GetManagerType returns "github_repo_fetch" - the action type this manager handles
func (m *GitHubRepoManager) GetManagerType() string {
	return "github_repo_fetch"
}

// ReturnsChildJobs returns true since this manager creates child jobs for each file
func (m *GitHubRepoManager) ReturnsChildJobs() bool {
	return true
}

// CreateParentJob creates a parent job and spawns children for each file in the repo
func (m *GitHubRepoManager) CreateParentJob(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract required config
	connectorID := getStringConfig(stepConfig, "connector_id", "")
	owner := getStringConfig(stepConfig, "owner", "")
	repo := getStringConfig(stepConfig, "repo", "")

	if connectorID == "" {
		return "", fmt.Errorf("connector_id is required")
	}
	if owner == "" {
		return "", fmt.Errorf("owner is required")
	}
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}

	// Extract optional config with defaults
	branches := getStringSliceConfig(stepConfig, "branches", []string{"main"})
	extensions := getStringSliceConfig(stepConfig, "extensions", []string{".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"})
	excludePaths := getStringSliceConfig(stepConfig, "exclude_paths", []string{"vendor/", "node_modules/", ".git/", "dist/", "build/"})
	maxFiles := getIntConfig(stepConfig, "max_files", 1000)

	m.logger.Debug().
		Str("step_name", step.Name).
		Str("connector_id", connectorID).
		Str("owner", owner).
		Str("repo", repo).
		Strs("branches", branches).
		Int("max_files", maxFiles).
		Msg("Creating GitHub repo parent job")

	// Get GitHub connector
	connector, err := m.connectorService.GetConnector(ctx, connectorID)
	if err != nil {
		return "", fmt.Errorf("failed to get connector: %w", err)
	}

	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		return "", fmt.Errorf("failed to create GitHub connector: %w", err)
	}

	// Create parent job first
	parentJob := models.NewQueueJob(
		"github_repo_parent",
		fmt.Sprintf("GitHub Repo: %s/%s", owner, repo),
		map[string]interface{}{
			"owner":    owner,
			"repo":     repo,
			"branches": branches,
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

	// List and enqueue files for each branch
	totalFilesEnqueued := 0

	for _, branch := range branches {
		files, err := ghConnector.ListFiles(ctx, owner, repo, branch, extensions, excludePaths)
		if err != nil {
			m.logger.Warn().Err(err).
				Str("branch", branch).
				Msg("Failed to list files for branch, skipping")
			continue
		}

		for _, file := range files {
			if totalFilesEnqueued >= maxFiles {
				m.logger.Warn().
					Int("max_files", maxFiles).
					Msg("Reached max_files limit, stopping file enumeration")
				break
			}

			// Create child job for each file
			childJob := models.NewQueueJobChild(
				parentJob.ID,
				models.JobTypeGitHubRepoFile,
				fmt.Sprintf("Fetch: %s@%s:%s", repo, branch, file.Path),
				map[string]interface{}{
					"owner":  owner,
					"repo":   repo,
					"branch": branch,
					"path":   file.Path,
					"folder": file.Folder,
					"sha":    file.SHA,
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
					Str("file", file.Path).
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
					Str("file", file.Path).
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
					Str("file", file.Path).
					Msg("Failed to enqueue file job, skipping")
				continue
			}

			totalFilesEnqueued++
		}

		if totalFilesEnqueued >= maxFiles {
			break
		}
	}

	m.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", owner).
		Str("repo", repo).
		Int("files_enqueued", totalFilesEnqueued).
		Msg("GitHub repo parent job created, child jobs enqueued")

	// Update parent job with total count
	if err := m.jobMgr.UpdateJobProgress(ctx, parentJob.ID, 0, totalFilesEnqueued); err != nil {
		m.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	return parentJob.ID, nil
}

// Helper functions for config extraction
func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return defaultValue
}

func getIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	if v, ok := config[key].(int); ok {
		return v
	}
	return defaultValue
}

func getStringSliceConfig(config map[string]interface{}, key string, defaultValue []string) []string {
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
