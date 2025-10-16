package jobs

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlCollectJob implements the crawl and collect default job
type CrawlCollectJob struct {
	crawlerService interfaces.CrawlerService
	sourceService  interfaces.SourceService
	authStorage    interfaces.AuthStorage
	logger         arbor.ILogger
}

// NewCrawlCollectJob creates a new crawl and collect job
func NewCrawlCollectJob(
	crawlerService interfaces.CrawlerService,
	sourceService interfaces.SourceService,
	authStorage interfaces.AuthStorage,
	logger arbor.ILogger,
) *CrawlCollectJob {
	return &CrawlCollectJob{
		crawlerService: crawlerService,
		sourceService:  sourceService,
		authStorage:    authStorage,
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
	seedURLs := j.deriveSeedURLs(source)
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
	authSnapshot, err := j.authStorage.GetAuth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get auth credentials: %w", err)
	}

	// Create crawler config
	crawlerConfig := &crawler.CrawlConfig{
		SourceConfig:  *source,
		SeedURLs:      seedURLs,
		EntityType:    entityType,
		DetailLevel:   "full",
		AuthSnapshot:  authSnapshot,
		RefreshSource: true,
	}

	// Start crawl
	jobID, err := j.crawlerService.StartCrawl(crawlerConfig)
	if err != nil {
		return fmt.Errorf("failed to start crawl: %w", err)
	}

	j.logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID).
		Msg("Crawl job started, waiting for completion")

	// Wait for job completion
	if err := j.crawlerService.WaitForJob(ctx, jobID); err != nil {
		return fmt.Errorf("crawl job failed: %w", err)
	}

	j.logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID).
		Msg("Crawl job completed successfully")

	return nil
}

// deriveSeedURLs determines the appropriate seed URLs based on source type
func (j *CrawlCollectJob) deriveSeedURLs(source *models.SourceConfig) []string {
	parsedURL, err := url.Parse(source.BaseURL)
	if err != nil {
		j.logger.Warn().
			Err(err).
			Str("base_url", source.BaseURL).
			Msg("Failed to parse base URL")
		return []string{}
	}

	path := strings.TrimRight(parsedURL.Path, "/")

	// If already a REST API endpoint, use as-is
	if strings.Contains(path, "/rest/") {
		return []string{source.BaseURL}
	}

	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	switch source.Type {
	case models.SourceTypeJira:
		return []string{fmt.Sprintf("%s/rest/api/3/project", baseURL)}

	case models.SourceTypeConfluence:
		// Handle /wiki prefix
		if strings.HasPrefix(path, "/wiki") {
			return []string{fmt.Sprintf("%s/wiki/rest/api/space", baseURL)}
		}
		return []string{fmt.Sprintf("%s/wiki/rest/api/space", baseURL)}

	case models.SourceTypeGithub:
		// Check for org filter
		if org, ok := source.Filters["org"].(string); ok {
			return []string{fmt.Sprintf("%s/orgs/%s/repos", baseURL, org)}
		}
		// Check for user filter
		if user, ok := source.Filters["user"].(string); ok {
			return []string{fmt.Sprintf("%s/users/%s/repos", baseURL, user)}
		}
		return []string{}

	default:
		j.logger.Warn().
			Str("source_type", string(source.Type)).
			Msg("Unknown source type")
		return []string{}
	}
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
