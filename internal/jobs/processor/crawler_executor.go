package processor

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerURLExecutor executes crawler_url jobs (individual URL crawling)
// This executor implements the JobExecutor interface for the queue-based job system
type CrawlerURLExecutor struct {
	crawlerService *crawler.Service
	jobMgr         *jobs.Manager
	queueMgr       *queue.Manager
	jobStorage     interfaces.JobStorage
	logger         arbor.ILogger
}

// NewCrawlerURLExecutor creates a new crawler URL executor
func NewCrawlerURLExecutor(
	crawlerService *crawler.Service,
	jobMgr *jobs.Manager,
	queueMgr *queue.Manager,
	jobStorage interfaces.JobStorage,
	logger arbor.ILogger,
) *CrawlerURLExecutor {
	return &CrawlerURLExecutor{
		crawlerService: crawlerService,
		jobMgr:         jobMgr,
		queueMgr:       queueMgr,
		jobStorage:     jobStorage,
		logger:         logger,
	}
}

// GetJobType returns the job type this executor handles
func (e *CrawlerURLExecutor) GetJobType() string {
	return string(models.JobTypeCrawlerURL)
}

// Validate validates that the job model is compatible with this executor
func (e *CrawlerURLExecutor) Validate(job *models.JobModel) error {
	if job.Type != string(models.JobTypeCrawlerURL) {
		return fmt.Errorf("invalid job type: expected %s, got %s", models.JobTypeCrawlerURL, job.Type)
	}

	// Validate required config fields
	if _, ok := job.Config["seed_url"]; !ok {
		return fmt.Errorf("missing required config field: seed_url")
	}

	if _, ok := job.Config["source_type"]; !ok {
		return fmt.Errorf("missing required config field: source_type")
	}

	if _, ok := job.Config["entity_type"]; !ok {
		return fmt.Errorf("missing required config field: entity_type")
	}

	return nil
}

// Execute executes a crawler_url job
func (e *CrawlerURLExecutor) Execute(ctx context.Context, job *models.JobModel) error {
	// Extract config fields
	seedURL, _ := job.GetConfigString("seed_url")
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	e.logger.Info().
		Str("job_id", job.ID).
		Str("seed_url", seedURL).
		Str("source_type", sourceType).
		Str("entity_type", entityType).
		Int("depth", job.Depth).
		Msg("Executing crawler_url job")

	// Add job log
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Crawling URL: %s (depth: %d)", seedURL, job.Depth))

	// Update job status to running
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		e.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update job status to running")
	}

	// TODO: Implement actual URL crawling logic here
	// For now, just mark as completed
	// This is where you would:
	// 1. Fetch the URL using crawler service
	// 2. Extract content and links
	// 3. Store results
	// 4. Spawn child jobs for discovered links (if needed)

	e.logger.Info().Str("job_id", job.ID).Msg("Crawler URL job completed (stub implementation)")
	e.jobMgr.AddJobLog(ctx, job.ID, "info", "Crawl completed successfully")

	// Update job status to completed
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		e.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}
