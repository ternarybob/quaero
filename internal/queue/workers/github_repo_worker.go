// -----------------------------------------------------------------------
// GitHub Repo Worker - Unified worker implementing both DefinitionWorker and JobWorker
// - DefinitionWorker: Creates parent jobs and spawns child jobs for repository files
// - JobWorker: Processes individual GitHub repo file jobs
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubRepoWorker handles GitHub repository jobs and implements both DefinitionWorker and JobWorker interfaces.
// - DefinitionWorker: Creates parent jobs and spawns child jobs for repository files
// - JobWorker: Processes individual GitHub repo file jobs
type GitHubRepoWorker struct {
	connectorService interfaces.ConnectorService
	jobManager       *queue.Manager
	queueMgr         interfaces.QueueManager
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
}

// Compile-time assertions: GitHubRepoWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*GitHubRepoWorker)(nil)
var _ interfaces.JobWorker = (*GitHubRepoWorker)(nil)

// NewGitHubRepoWorker creates a new GitHub repo worker that implements both DefinitionWorker and JobWorker interfaces
func NewGitHubRepoWorker(
	connectorService interfaces.ConnectorService,
	jobManager *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubRepoWorker {
	return &GitHubRepoWorker{
		connectorService: connectorService,
		jobManager:       jobManager,
		queueMgr:         queueMgr,
		documentStorage:  documentStorage,
		eventService:     eventService,
		logger:           logger,
	}
}

// GetWorkerType returns the job type this worker handles
func (w *GitHubRepoWorker) GetWorkerType() string {
	return models.JobTypeGitHubRepoFile
}

// Validate validates that the queue job is compatible with this worker
func (w *GitHubRepoWorker) Validate(job *models.QueueJob) error {
	if job.Type != models.JobTypeGitHubRepoFile {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeGitHubRepoFile, job.Type)
	}

	requiredFields := []string{"owner", "repo", "branch", "path"}
	for _, field := range requiredFields {
		if _, ok := job.GetConfigString(field); !ok {
			return fmt.Errorf("missing required config field: %s", field)
		}
	}
	return nil
}

// Execute processes a GitHub repo file job
func (w *GitHubRepoWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	w.logger.Debug().Str("job_id", job.ID).Msg("Processing GitHub repo file job")

	// Update job status to running
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusRunning)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Extract configuration from job
	owner, _ := job.GetConfigString("owner")
	repo, _ := job.GetConfigString("repo")
	branch, _ := job.GetConfigString("branch")
	path, _ := job.GetConfigString("path")
	folder, _ := job.GetConfigString("folder")
	sha, _ := job.GetConfigString("sha")

	// Get connector ID from metadata
	connectorID, _ := job.GetMetadataString("connector_id")
	if connectorID == "" {
		return fmt.Errorf("connector_id not found in job metadata")
	}

	// Get tags from metadata
	baseTags := getTagsFromMetadata(job.Metadata)

	// Get GitHub connector
	connector, err := w.connectorService.GetConnector(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		return fmt.Errorf("failed to create GitHub connector: %w", err)
	}

	// Fetch file content with timeout to prevent hanging on rate limits
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	file, err := ghConnector.GetFileContent(fetchCtx, owner, repo, branch, path)
	if err != nil {
		return fmt.Errorf("failed to fetch file content: %w", err)
	}

	// Create document
	doc := &models.Document{
		ID:              fmt.Sprintf("doc_%s", uuid.New().String()),
		SourceType:      models.SourceTypeGitHubRepo,
		SourceID:        fmt.Sprintf("%s/%s/%s/%s", owner, repo, branch, path),
		Title:           filepath.Base(path),
		ContentMarkdown: file.Content,
		URL:             fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, path),
		Tags:            mergeTags(baseTags, []string{"github", repo, branch}),
		Metadata: map[string]interface{}{
			"owner":     owner,
			"repo":      repo,
			"branch":    branch,
			"folder":    folder,
			"path":      path,
			"sha":       sha,
			"file_type": filepath.Ext(path),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save document
	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Log document saved via Job Manager's unified logging (routes to WebSocket/DB)
	w.logDocumentSaved(ctx, job, doc.ID, doc.Title, path)

	// Update job status to completed
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusCompleted)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update progress (completed=1, failed=0)
	if err := w.jobManager.UpdateJobProgress(ctx, job.ID, 1, 0); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update job progress")
	}

	w.logger.Debug().
		Str("job_id", job.ID).
		Str("path", path).
		Msg("GitHub repo file job completed successfully")

	return nil
}

// getTagsFromMetadata extracts tags from job metadata
func getTagsFromMetadata(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}

	// Try []string first
	if tags, ok := metadata["tags"].([]string); ok {
		return tags
	}

	// Try []interface{} (from JSON unmarshaling)
	if tagsInterface, ok := metadata["tags"].([]interface{}); ok {
		tags := make([]string, 0, len(tagsInterface))
		for _, t := range tagsInterface {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
		return tags
	}

	return nil
}

// mergeTags combines base tags with additional tags, removing duplicates
func mergeTags(baseTags []string, additionalTags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(baseTags)+len(additionalTags))

	// Add base tags first
	for _, tag := range baseTags {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}

	// Add additional tags
	for _, tag := range additionalTags {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}

	return result
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeGitHubRepo for the DefinitionWorker interface
func (w *GitHubRepoWorker) GetType() models.WorkerType {
	return models.WorkerTypeGitHubRepo
}

// Init performs the initialization/setup phase for a GitHub repo step.
// This is where we validate configuration and prepare repo parameters.
func (w *GitHubRepoWorker) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
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
		return nil, fmt.Errorf("connector_id or connector_name is required")
	}
	if owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Extract optional config with defaults
	branches := getStringSliceConfig(stepConfig, "branches", []string{"main"})
	extensions := getStringSliceConfig(stepConfig, "extensions", []string{".go", ".ts", ".tsx", ".js", ".jsx", ".md", ".yaml", ".yml", ".toml", ".json"})
	excludePaths := getStringSliceConfig(stepConfig, "exclude_paths", []string{"vendor/", "node_modules/", ".git/", "dist/", "build/"})
	maxFiles := getIntConfig(stepConfig, "max_files", 1000)

	w.logger.Info().
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Strs("branches", branches).
		Int("max_files", maxFiles).
		Msg("[step] GitHub repo worker initialized")

	// Create work items for each branch
	workItems := make([]interfaces.WorkItem, len(branches))
	for i, branch := range branches {
		workItems[i] = interfaces.WorkItem{
			ID:   fmt.Sprintf("%s/%s@%s", owner, repo, branch),
			Name: fmt.Sprintf("Branch: %s", branch),
			Type: "branch",
			Config: map[string]interface{}{
				"branch": branch,
			},
		}
	}

	return &interfaces.WorkerInitResult{
		WorkItems:            workItems,
		TotalCount:           len(branches),
		Strategy:             interfaces.ProcessingStrategyParallel,
		SuggestedConcurrency: 5,
		Metadata: map[string]interface{}{
			"owner":          owner,
			"repo":           repo,
			"branches":       branches,
			"extensions":     extensions,
			"exclude_paths":  excludePaths,
			"max_files":      maxFiles,
			"connector_id":   connectorID,
			"connector_name": connectorName,
			"step_config":    stepConfig,
		},
	}, nil
}

// CreateJobs creates a parent job and spawns child jobs for each file in the repository.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
func (w *GitHubRepoWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string, initResult *interfaces.WorkerInitResult) (string, error) {
	// Call Init if not provided
	if initResult == nil {
		var err error
		initResult, err = w.Init(ctx, step, jobDef)
		if err != nil {
			return "", fmt.Errorf("failed to initialize github_repo worker: %w", err)
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
	branches, _ := initResult.Metadata["branches"].([]string)
	extensions, _ := initResult.Metadata["extensions"].([]string)
	excludePaths, _ := initResult.Metadata["exclude_paths"].([]string)
	maxFiles, _ := initResult.Metadata["max_files"].(int)

	w.logger.Info().
		Str("step_name", step.Name).
		Str("owner", owner).
		Str("repo", repo).
		Strs("branches", branches).
		Int("max_files", maxFiles).
		Msg("[worker] Creating GitHub repo jobs from init result")

	// Preserve the original validation variables for use below
	_ = extensions
	_ = excludePaths

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

	// List and enqueue files for each branch
	totalFilesEnqueued := 0

	for _, branch := range branches {
		files, err := ghConnector.ListFiles(ctx, owner, repo, branch, extensions, excludePaths)
		if err != nil {
			w.logger.Warn().Err(err).
				Str("branch", branch).
				Msg("Failed to list files for branch, skipping")
			continue
		}

		for _, file := range files {
			if totalFilesEnqueued >= maxFiles {
				w.logger.Warn().
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
					"root_parent_id": stepID, // Root parent for document count tracking
				},
				parentJob.Depth+1,
			)

			// Serialize child job to JSON
			childPayloadBytes, err := childJob.ToJSON()
			if err != nil {
				w.logger.Warn().Err(err).
					Str("file", file.Path).
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

			if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
				w.logger.Warn().Err(err).
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

	w.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", owner).
		Str("repo", repo).
		Int("files_enqueued", totalFilesEnqueued).
		Msg("GitHub repo parent job created, child jobs enqueued")

	// Update parent job with total count
	if err := w.jobManager.UpdateJobProgress(ctx, parentJob.ID, 0, totalFilesEnqueued); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	return parentJob.ID, nil
}

// ReturnsChildJobs returns true since GitHub repo creates child jobs for each file
func (w *GitHubRepoWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for GitHub repo type (DefinitionWorker interface)
func (w *GitHubRepoWorker) ValidateConfig(step models.JobStep) error {
	// Validate step config exists
	if step.Config == nil {
		return fmt.Errorf("github_repo step requires config")
	}

	// Validate connector_id or connector_name
	connectorID, hasConnectorID := step.Config["connector_id"].(string)
	connectorName, hasConnectorName := step.Config["connector_name"].(string)

	if (!hasConnectorID || connectorID == "") && (!hasConnectorName || connectorName == "") {
		return fmt.Errorf("github_repo step requires either 'connector_id' or 'connector_name' in config")
	}

	// Validate required owner field
	owner, ok := step.Config["owner"].(string)
	if !ok || owner == "" {
		return fmt.Errorf("github_repo step requires 'owner' in config")
	}

	// Validate required repo field
	repo, ok := step.Config["repo"].(string)
	if !ok || repo == "" {
		return fmt.Errorf("github_repo step requires 'repo' in config")
	}

	return nil
}

// ============================================================================
// HELPER FUNCTIONS FOR CONFIG EXTRACTION
// ============================================================================

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

// logDocumentSaved logs a document saved event via Job Manager's unified logging.
func (w *GitHubRepoWorker) logDocumentSaved(ctx context.Context, job *models.QueueJob, docID, title, path string) {
	message := fmt.Sprintf("Document saved: %s (ID: %s)", title, docID[:8])
	if path != "" {
		message = fmt.Sprintf("Document saved: %s - %s (ID: %s)", path, title, docID[:8])
	}
	w.jobManager.AddJobLog(ctx, job.ID, "info", message)
}
