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

// GitHubRepoWorker handles GitHub repository file jobs
type GitHubRepoWorker struct {
	connectorService interfaces.ConnectorService
	jobManager       *queue.Manager
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
}

// Compile-time assertion: GitHubRepoWorker implements JobWorker interface
var _ interfaces.JobWorker = (*GitHubRepoWorker)(nil)

// NewGitHubRepoWorker creates a new GitHub repo worker
func NewGitHubRepoWorker(
	connectorService interfaces.ConnectorService,
	jobManager *queue.Manager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubRepoWorker {
	return &GitHubRepoWorker{
		connectorService: connectorService,
		jobManager:       jobManager,
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

	// Fetch file content
	file, err := ghConnector.GetFileContent(ctx, owner, repo, branch, path)
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

	// Publish event for real-time UI updates
	if w.eventService != nil {
		// Use root_parent_id for document count tracking (points to JobDefParent)
		// Fall back to immediate parent if root_parent_id is not set
		rootParentID, _ := job.GetMetadataString("root_parent_id")
		if rootParentID == "" {
			rootParentID = job.GetParentID()
		}

		event := interfaces.Event{
			Type: interfaces.EventDocumentSaved,
			Payload: map[string]interface{}{
				"job_id":        job.ID,
				"parent_job_id": rootParentID, // Root parent (JobDefParent) for document count tracking
				"document_id":   doc.ID,
				"title":         doc.Title,
				"path":          path,
				"timestamp":     time.Now().Format(time.RFC3339),
			},
		}
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to publish document saved event")
		}
	}

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
