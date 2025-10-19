package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// CrawlCollectJob implements the crawl and collect default job
type CrawlCollectJob struct {
	crawlerService *crawler.Service
	sourceService  *sources.Service
	authStorage    interfaces.AuthStorage
	eventService   interfaces.EventService
	config         *common.Config
	logger         arbor.ILogger
}

// NewCrawlCollectJob creates a new crawl and collect job
func NewCrawlCollectJob(
	crawlerService *crawler.Service,
	sourceService *sources.Service,
	authStorage interfaces.AuthStorage,
	eventService interfaces.EventService,
	config *common.Config,
	logger arbor.ILogger,
) *CrawlCollectJob {
	return &CrawlCollectJob{
		crawlerService: crawlerService,
		sourceService:  sourceService,
		authStorage:    authStorage,
		eventService:   eventService,
		config:         config,
		logger:         logger,
	}
}

// Execute runs the crawl and collect job
func (j *CrawlCollectJob) Execute() error {
	ctx := context.Background()

	j.logger.Info().Msg("Starting crawl and collect job")

	// Get enabled sources
	sources, err := j.sourceService.GetEnabledSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get enabled sources: %w", err)
	}

	if len(sources) == 0 {
		j.logger.Info().Msg("No enabled sources found, skipping crawl")
		return nil
	}

	j.logger.Info().Int("source_count", len(sources)).Msg("Processing enabled sources")

	// Process each source
	var errors []error
	for _, source := range sources {
		if err := j.processSource(ctx, source); err != nil {
			j.logger.Error().
				Err(err).
				Str("source_id", source.ID).
				Str("source_type", string(source.Type)).
				Msg("Failed to process source")
			errors = append(errors, fmt.Errorf("source %s: %w", source.ID, err))
		}
	}

	// Return aggregated errors if any
	if len(errors) > 0 {
		return fmt.Errorf("crawl job completed with %d error(s): %v", len(errors), errors)
	}

	j.logger.Info().Msg("Crawl and collect job completed successfully")
	return nil
}

// processSource handles crawling for a single source
func (j *CrawlCollectJob) processSource(ctx context.Context, source *models.SourceConfig) error {
	j.logger.Info().
		Str("source_id", source.ID).
		Str("source_type", string(source.Type)).
		Str("base_url", source.BaseURL).
		Msg("Processing source")

	// Derive seed URLs and entity type
	seedURLs := common.DeriveSeedURLs(source, j.config.Crawler.UseHTMLSeeds, j.logger)
	if len(seedURLs) == 0 {
		return fmt.Errorf("failed to derive seed URLs for source")
	}

	entityType := j.deriveEntityType(source)

	j.logger.Debug().
		Str("source_id", source.ID).
		Strs("seed_urls", seedURLs).
		Str("entity_type", entityType).
		Msg("Derived crawl parameters")

	// Get auth credentials for this source
	var authCreds *models.AuthCredentials
	if source.AuthID != "" {
		var err error
		authCreds, err = j.authStorage.GetCredentialsByID(ctx, source.AuthID)
		if err != nil {
			return fmt.Errorf("failed to get auth credentials: %w", err)
		}
	}

	// Create crawler config
	crawlerConfig := crawler.CrawlConfig{
		MaxDepth:        source.CrawlConfig.MaxDepth,
		MaxPages:        source.CrawlConfig.MaxPages,
		FollowLinks:     source.CrawlConfig.FollowLinks,
		Concurrency:     source.CrawlConfig.Concurrency,
		RateLimit:       time.Duration(source.CrawlConfig.RateLimit) * time.Millisecond,
		IncludePatterns: source.CrawlConfig.IncludePatterns,
		ExcludePatterns: source.CrawlConfig.ExcludePatterns,
		DetailLevel:     "full",
		RetryAttempts:   3,
		RetryBackoff:    2 * time.Second,
	}

	// Start crawl with correct signature
	jobID, err := j.crawlerService.StartCrawl(
		source.Type,
		entityType,
		seedURLs,
		crawlerConfig,
		source.ID,
		true, // refreshSource
		source,
		authCreds,
	)
	if err != nil {
		return fmt.Errorf("failed to start crawl: %w", err)
	}

	j.logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID).
		Msg("Crawl job started, waiting for completion")

	// Wait for job completion
	resultsInterface, err := j.crawlerService.WaitForJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("crawl job failed: %w", err)
	}

	// Type assert to []*crawler.CrawlResult
	results, ok := resultsInterface.([]*crawler.CrawlResult)
	if !ok {
		j.logger.Warn().Str("job_id", jobID).Msg("Unexpected result type from WaitForJob")
		// Continue with empty results count
		results = nil
	}

	j.logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID).
		Int("results_count", len(results)).
		Msg("Crawl job completed successfully")

	// Trigger transformation of crawled data to documents
	if err := j.eventService.PublishSync(ctx, interfaces.Event{
		Type: interfaces.EventCollectionTriggered,
		Payload: map[string]interface{}{
			"job_id":      jobID,
			"source_id":   source.ID,
			"source_type": string(source.Type),
		},
	}); err != nil {
		j.logger.Warn().Err(err).Msg("Failed to publish collection event")
		// Non-critical - continue
	}

	return nil
}

// deriveEntityType determines the appropriate entity type based on source type
func (j *CrawlCollectJob) deriveEntityType(source *models.SourceConfig) string {
	switch source.Type {
	case models.SourceTypeJira:
		return "projects"
	case models.SourceTypeConfluence:
		return "spaces"
	case models.SourceTypeGithub:
		return "repos"
	default:
		return "all"
	}
}
