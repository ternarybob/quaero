package managers

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

// GitHubRepoBatchProcessor handles batch mode processing for GitHub repos
// using GraphQL bulk fetch for improved performance
type GitHubRepoBatchProcessor struct {
	ghConnector     *github.Connector
	jobMgr          *queue.Manager
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// NewGitHubRepoBatchProcessor creates a new batch processor
func NewGitHubRepoBatchProcessor(
	ghConnector *github.Connector,
	jobMgr *queue.Manager,
	documentStorage interfaces.DocumentStorage,
	eventService interfaces.EventService,
	logger arbor.ILogger,
) *GitHubRepoBatchProcessor {
	return &GitHubRepoBatchProcessor{
		ghConnector:     ghConnector,
		jobMgr:          jobMgr,
		documentStorage: documentStorage,
		eventService:    eventService,
		logger:          logger,
	}
}

// BatchProcessConfig contains configuration for batch processing
type BatchProcessConfig struct {
	Owner        string
	Repo         string
	Branches     []string
	Extensions   []string
	ExcludePaths []string
	MaxFiles     int
	BatchSize    int
	Tags         []string
	RootParentID string
}

// Process performs batch fetching of repository files using GraphQL
func (p *GitHubRepoBatchProcessor) Process(ctx context.Context, parentJob *models.QueueJob, config BatchProcessConfig) error {
	startTime := time.Now()

	// Update job status to running
	if err := p.jobMgr.UpdateJobStatus(ctx, parentJob.ID, string(models.JobStatusRunning)); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Create batch fetcher
	batchFetcher := github.NewBatchFetcher(p.ghConnector).
		WithBatchSize(config.BatchSize)

	totalDocuments := 0
	totalErrors := 0

	for _, branch := range config.Branches {
		// List files for this branch
		files, err := p.ghConnector.ListFiles(ctx, config.Owner, config.Repo, branch, config.Extensions, config.ExcludePaths)
		if err != nil {
			p.logger.Warn().Err(err).
				Str("branch", branch).
				Msg("Failed to list files for branch, skipping")
			continue
		}

		// Limit files
		if len(files) > config.MaxFiles-totalDocuments {
			files = files[:config.MaxFiles-totalDocuments]
		}

		if len(files) == 0 {
			continue
		}

		p.logger.Info().
			Str("branch", branch).
			Int("file_count", len(files)).
			Msg("Starting batch fetch for branch")

		// Fetch files in bulk
		result, err := batchFetcher.FetchFilesWithProgress(ctx, config.Owner, config.Repo, branch, files, func(processed, total, batch int) {
			p.logger.Debug().
				Int("processed", processed).
				Int("total", total).
				Int("batch", batch).
				Msg("Batch progress")
		})

		if err != nil {
			p.logger.Error().Err(err).
				Str("branch", branch).
				Msg("Batch fetch failed")
			totalErrors++
			continue
		}

		// Save documents
		for _, doc := range result.Documents {
			// Enhance document with proper fields
			doc.ID = fmt.Sprintf("doc_%s", uuid.New().String())
			doc.SourceType = models.SourceTypeGitHubRepo
			doc.SourceID = fmt.Sprintf("%s/%s/%s/%s", config.Owner, config.Repo, branch, doc.Title)
			doc.URL = fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", config.Owner, config.Repo, branch, doc.Title)
			doc.Tags = p.mergeTags(config.Tags, []string{"github", config.Repo, branch})
			doc.CreatedAt = time.Now()
			doc.UpdatedAt = time.Now()

			// Get path from metadata
			if path, ok := doc.Metadata["path"].(string); ok {
				doc.Title = filepath.Base(path)
				doc.SourceID = fmt.Sprintf("%s/%s/%s/%s", config.Owner, config.Repo, branch, path)
				doc.URL = fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", config.Owner, config.Repo, branch, path)
			}

			if err := p.documentStorage.SaveDocument(doc); err != nil {
				p.logger.Warn().Err(err).
					Str("path", doc.Title).
					Msg("Failed to save document")
				totalErrors++
				continue
			}

			totalDocuments++

			// Publish event for real-time UI updates
			if p.eventService != nil {
				event := interfaces.Event{
					Type: interfaces.EventDocumentSaved,
					Payload: map[string]interface{}{
						"job_id":        parentJob.ID,
						"parent_job_id": config.RootParentID,
						"document_id":   doc.ID,
						"title":         doc.Title,
						"timestamp":     time.Now().Format(time.RFC3339),
					},
				}
				if err := p.eventService.Publish(ctx, event); err != nil {
					p.logger.Warn().Err(err).Msg("Failed to publish document saved event")
				}
			}
		}

		// Log errors
		for _, fileErr := range result.Errors {
			p.logger.Warn().
				Str("path", fileErr.Path).
				Err(fileErr.Error).
				Msg("Failed to fetch file")
			totalErrors++
		}

		if totalDocuments >= config.MaxFiles {
			break
		}
	}

	duration := time.Since(startTime)

	p.logger.Info().
		Str("parent_job_id", parentJob.ID).
		Str("owner", config.Owner).
		Str("repo", config.Repo).
		Int("documents_saved", totalDocuments).
		Int("errors", totalErrors).
		Dur("duration", duration).
		Msg("Batch mode completed")

	// Update parent job with final count and status
	if err := p.jobMgr.UpdateJobProgress(ctx, parentJob.ID, totalDocuments, totalDocuments); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to update parent job progress")
	}

	if err := p.jobMgr.UpdateJobStatus(ctx, parentJob.ID, string(models.JobStatusCompleted)); err != nil {
		p.logger.Warn().Err(err).Msg("Failed to update parent job status")
	}

	return nil
}

// mergeTags combines base tags with additional tags, removing duplicates
func (p *GitHubRepoBatchProcessor) mergeTags(baseTags []string, additionalTags []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(baseTags)+len(additionalTags))

	for _, tag := range baseTags {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}

	for _, tag := range additionalTags {
		if tag != "" && !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}

	return result
}

// Helper function for batch mode config extraction
func getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := config[key].(bool); ok {
		return v
	}
	return defaultValue
}
