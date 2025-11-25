package workers

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/githublogs"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// GitHubLogWorker handles GitHub Action Log jobs
type GitHubLogWorker struct {
	connectorService interfaces.ConnectorService
	jobManager       *queue.Manager
	documentStorage  interfaces.DocumentStorage
	eventService     interfaces.EventService
	logger           arbor.ILogger
}

// NewGitHubLogWorker creates a new GitHub log worker
func NewGitHubLogWorker(
	connectorService interfaces.ConnectorService,
	jobManager *queue.Manager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubLogWorker {
	return &GitHubLogWorker{
		connectorService: connectorService,
		jobManager:       jobManager,
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
	w.logger.Info().Str("job_id", job.ID).Msg("Processing GitHub Action Log job")

	// Update job status to running
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusRunning)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Extract configuration
	config := job.Config
	seedURL, _ := config["seed_url"].(string)
	authID, _ := job.Metadata["auth_id"].(string) // Using auth_id as connector_id for now

	if seedURL == "" {
		return fmt.Errorf("seed_url is required")
	}

	// Find connector
	var connector *models.Connector
	var err error

	if authID != "" {
		connector, err = w.connectorService.GetConnector(ctx, authID)
		if err != nil {
			w.logger.Warn().Err(err).Str("connector_id", authID).Msg("Failed to find connector by ID, falling back to listing")
		}
	}

	if connector == nil {
		// Fallback: List connectors and find the first GitHub one
		connectors, err := w.connectorService.ListConnectors(ctx)
		if err != nil {
			return fmt.Errorf("failed to list connectors: %w", err)
		}
		for _, c := range connectors {
			if c.Type == models.ConnectorTypeGitHub {
				connector = c
				break
			}
		}
	}

	if connector == nil {
		return fmt.Errorf("no GitHub connector found")
	}

	// Initialize GitHub connector
	ghConnector, err := github.NewConnector(connector)
	if err != nil {
		return fmt.Errorf("failed to create github connector: %w", err)
	}

	// Parse URL
	owner, repo, jobID, err := githublogs.ParseLogURL(seedURL)
	if err != nil {
		return fmt.Errorf("failed to parse log url: %w", err)
	}

	// Fetch log
	logContent, err := ghConnector.GetJobLog(ctx, owner, repo, jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job log: %w", err)
	}

	// Save as document
	doc := &models.Document{
		ID:              job.ID, // Use job ID as document ID for simplicity, or generate new one
		SourceID:        job.ID,
		SourceType:      models.SourceTypeGitHubActionLog,
		Title:           fmt.Sprintf("GitHub Action Log: %s/%s Job %d", owner, repo, jobID),
		ContentMarkdown: logContent,
		URL:             seedURL,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		Tags:            []string{"github", "log", repo},
		Metadata: map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"job_id": jobID,
		},
	}

	if err := w.documentStorage.SaveDocument(doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Update job status to completed
	if err := w.jobManager.UpdateJobStatus(ctx, job.ID, string(models.JobStatusCompleted)); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update progress (completed=1, failed=0)
	if err := w.jobManager.UpdateJobProgress(ctx, job.ID, 1, 0); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update job progress")
	}

	w.logger.Info().Str("job_id", job.ID).Msg("GitHub Action Log job completed successfully")
	return nil
}
