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
)

// StartCrawlJob creates and starts a crawl job with common logic extracted from handlers and cron jobs
func StartCrawlJob(
	ctx context.Context,
	source *models.SourceConfig,
	authStorage interfaces.AuthStorage,
	crawlerService *crawler.Service,
	config *common.Config,
	logger arbor.ILogger,
	seedURLOverrides []string, // Optional: allows API to override seed URLs
	refreshSource bool, // Whether to refresh source config and auth before starting
) (jobID string, err error) {
	// 1. Derive seed URLs
	var seedURLs []string
	if len(seedURLOverrides) > 0 {
		seedURLs = seedURLOverrides
	} else {
		seedURLs = common.DeriveSeedURLs(source, config.Crawler.UseHTMLSeeds, logger)
	}

	// 2. Validate seed URLs
	if len(seedURLs) == 0 {
		return "", fmt.Errorf("failed to derive seed URLs for source")
	}

	// 2a. Validate each seed URL and check for test URLs
	testURLCount := 0
	for _, seedURL := range seedURLs {
		isValid, isTestURL, warnings, err := common.ValidateBaseURL(seedURL, logger)
		if !isValid || err != nil {
			logger.Warn().
				Err(err).
				Str("seed_url", seedURL).
				Str("source_id", source.ID).
				Msg("Invalid seed URL detected during early validation")
			return "", fmt.Errorf("invalid seed URL: %s - %w", seedURL, err)
		}
		if isTestURL {
			testURLCount++
			// Log individual test URL warnings
			for _, warning := range warnings {
				logger.Warn().
					Str("seed_url", seedURL).
					Str("source_id", source.ID).
					Msg(warning)
			}
		}
	}

	// 2b. Reject test URLs in production mode
	if config.IsProduction() && testURLCount > 0 {
		errMsg := fmt.Sprintf("test URLs are not allowed in production mode: %d of %d seed URLs are test URLs (localhost/127.0.0.1)", testURLCount, len(seedURLs))
		logger.Error().
			Str("source_id", source.ID).
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Msg("Rejecting job: test URLs detected in production mode")
		return "", fmt.Errorf("%s", errMsg)
	}

	// 2c. Log warning if test URLs detected in development mode
	if !config.IsProduction() && testURLCount > 0 {
		logger.Warn().
			Str("source_id", source.ID).
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Msg("Test URLs detected in development mode (allowed)")
	}

	// 3. Derive entity type
	entityType := deriveEntityType(source)

	logger.Debug().
		Str("source_id", source.ID).
		Strs("seed_urls", seedURLs).
		Str("entity_type", entityType).
		Int("test_url_count", testURLCount).
		Int("total_urls", len(seedURLs)).
		Msg("Derived crawl parameters")

	// 4. Fetch auth credentials
	var authCreds *models.AuthCredentials
	if source.AuthID != "" {
		var err error
		authCreds, err = authStorage.GetCredentialsByID(ctx, source.AuthID)
		if err != nil {
			return "", fmt.Errorf("failed to get auth credentials: %w", err)
		}
	}

	// 5. Build crawler config
	// TODO: Extract filtering configuration from job step config in subsequent phases
	crawlerConfig := crawler.CrawlConfig{
		MaxDepth:        source.CrawlConfig.MaxDepth,
		MaxPages:        source.CrawlConfig.MaxPages,
		FollowLinks:     source.CrawlConfig.FollowLinks,
		Concurrency:     source.CrawlConfig.Concurrency,
		RateLimit:       time.Duration(source.CrawlConfig.RateLimit) * time.Millisecond,
		IncludePatterns: []string{}, // Will be populated from job definition config in future phases
		ExcludePatterns: []string{}, // Will be populated from job definition config in future phases
		DetailLevel:     "full",
		RetryAttempts:   3,
		RetryBackoff:    2 * time.Second,
	}

	// 6. Log detailed crawler configuration for debugging
	logger.Debug().
		Str("source_id", source.ID).
		Int("max_depth", crawlerConfig.MaxDepth).
		Int("max_pages", crawlerConfig.MaxPages).
		Str("follow_links", fmt.Sprintf("%v", crawlerConfig.FollowLinks)).
		Int("concurrency", crawlerConfig.Concurrency).
		Dur("rate_limit", crawlerConfig.RateLimit).
		Str("detail_level", crawlerConfig.DetailLevel).
		Msg("Crawler configuration validated and ready")

	// 7. Warn about limiting crawl settings
	if !crawlerConfig.FollowLinks {
		logger.Warn().Str("source_id", source.ID).Msg("FollowLinks is disabled - crawler will only process seed URLs")
	}
	if crawlerConfig.MaxDepth == 0 {
		logger.Warn().Str("source_id", source.ID).Msg("MaxDepth is 0 - crawler will only process seed URLs")
	}
	if crawlerConfig.MaxPages == 1 {
		logger.Warn().Str("source_id", source.ID).Msg("MaxPages is 1 - crawler will stop after first page")
	}
	if crawlerConfig.MaxPages > 0 && crawlerConfig.MaxPages < 10 {
		logger.Info().Str("source_id", source.ID).Int("max_pages", crawlerConfig.MaxPages).Msg("MaxPages is low - crawler may stop before all content is collected")
	}

	// 8. Start crawl with correct signature
	jobID, err = crawlerService.StartCrawl(
		source.Type,
		entityType,
		seedURLs,
		crawlerConfig,
		source.ID,
		refreshSource, // Pass through refreshSource parameter
		source,
		authCreds,
	)
	if err != nil {
		return "", fmt.Errorf("failed to start crawl: %w", err)
	}

	logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID).
		Msg("Crawl job started successfully")

	// 9. Return job ID
	return jobID, nil
}

// deriveEntityType determines the appropriate entity type based on source type
func deriveEntityType(source *models.SourceConfig) string {
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
