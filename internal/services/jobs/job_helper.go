package jobs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// StartCrawlJob creates and starts a crawl job with common logic extracted from handlers and cron jobs
// Note: Seed URLs have been removed. Crawling is based on source configuration (base_url and type).
// jobDefinitionID: Optional job definition ID for traceability (empty string if not from a job definition)
func StartCrawlJob(
	ctx context.Context,
	source *models.SourceConfig,
	authStorage interfaces.AuthStorage,
	crawlerService *crawler.Service,
	config *common.Config,
	logger arbor.ILogger,
	jobCrawlConfig crawler.CrawlConfig, // Job-level crawl configuration (filtering patterns and overrides)
	refreshSource bool, // Whether to refresh source config and auth before starting
	jobDefinitionID string, // Optional job definition ID for traceability
) (jobID string, err error) {
	// 1. Validate source configuration
	if source.BaseURL == "" {
		return "", fmt.Errorf("source base_url is required")
	}

	// 2. Validate base URL
	isValid, isTestURL, warnings, err := common.ValidateBaseURL(source.BaseURL, logger)
	if !isValid || err != nil {
		logger.Warn().
			Err(err).
			Str("base_url", source.BaseURL).
			Str("source_id", source.ID).
			Msg("Invalid base URL in source configuration")
		return "", fmt.Errorf("invalid base URL: %s - %w", source.BaseURL, err)
	}

	// 2a. Check for test URLs
	if isTestURL {
		for _, warning := range warnings {
			logger.Warn().
				Str("base_url", source.BaseURL).
				Str("source_id", source.ID).
				Msg(warning)
		}
	}

	// 2b. Reject test URLs in production mode
	if config.IsProduction() && isTestURL {
		errMsg := fmt.Sprintf("test URLs are not allowed in production mode: %s (localhost/127.0.0.1)", source.BaseURL)
		logger.Error().
			Str("source_id", source.ID).
			Str("base_url", source.BaseURL).
			Msg("Rejecting job: test URL detected in production mode")
		return "", fmt.Errorf("%s", errMsg)
	}

	// 2c. Log warning if test URL detected in development mode
	if !config.IsProduction() && isTestURL {
		logger.Warn().
			Str("source_id", source.ID).
			Str("base_url", source.BaseURL).
			Msg("Test URL detected in development mode (allowed)")
	}

	// 3. Derive entity type
	entityType := deriveEntityType(source)

	logger.Debug().
		Str("source_id", source.ID).
		Str("base_url", source.BaseURL).
		Str("entity_type", entityType).
		Bool("is_test_url", isTestURL).
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

	// 5. Build crawler config by merging job config with source defaults
	crawlerConfig := crawler.CrawlConfig{
		// Use job config values if provided (non-zero), else fall back to source defaults
		MaxDepth:    jobCrawlConfig.MaxDepth,
		MaxPages:    jobCrawlConfig.MaxPages,
		Concurrency: jobCrawlConfig.Concurrency,
		FollowLinks: jobCrawlConfig.FollowLinks,
		// Filtering patterns will be populated from source configuration below
		IncludePatterns: []string{},
		ExcludePatterns: []string{},
		// Source-level defaults for unspecified values
		RateLimit:     time.Duration(source.CrawlConfig.RateLimit) * time.Millisecond,
		DetailLevel:   "full",
		RetryAttempts: 3,
		RetryBackoff:  2 * time.Second,
	}

	// Apply source defaults for zero-value fields
	if crawlerConfig.MaxDepth == 0 {
		crawlerConfig.MaxDepth = source.CrawlConfig.MaxDepth
	}
	if crawlerConfig.MaxPages == 0 {
		crawlerConfig.MaxPages = source.CrawlConfig.MaxPages
	}
	if crawlerConfig.Concurrency == 0 {
		crawlerConfig.Concurrency = source.CrawlConfig.Concurrency
	}
	// Note: FollowLinks defaults to false, so we use job config value directly

	// 5a. Extract filters from source configuration (filters are source-level only)
	if source.Filters != nil && len(source.Filters) > 0 {
		// Extract include_patterns from source
		if includeVal, ok := source.Filters["include_patterns"]; ok && includeVal != nil {
			if includeStr, ok := includeVal.(string); ok && includeStr != "" {
				// Parse comma-delimited patterns
				patterns := strings.Split(includeStr, ",")
				var cleanedKeywords []string
				for _, pattern := range patterns {
					trimmed := strings.TrimSpace(pattern)
					if trimmed != "" {
						cleanedKeywords = append(cleanedKeywords, trimmed)
					}
				}
				// Convert keywords to regex patterns
				regexPatterns := convertKeywordsToRegex(cleanedKeywords, logger)

				if len(regexPatterns) > 0 {
					crawlerConfig.IncludePatterns = regexPatterns
					logger.Info().
						Str("source_id", source.ID).
						Strs("keywords", cleanedKeywords).
						Strs("regex_patterns", regexPatterns).
						Msg("Converted source include keywords to regex patterns")
				}
			}
		}

		// Extract exclude_patterns from source
		if excludeVal, ok := source.Filters["exclude_patterns"]; ok && excludeVal != nil {
			if excludeStr, ok := excludeVal.(string); ok && excludeStr != "" {
				// Parse comma-delimited patterns
				patterns := strings.Split(excludeStr, ",")
				var cleanedKeywords []string
				for _, pattern := range patterns {
					trimmed := strings.TrimSpace(pattern)
					if trimmed != "" {
						cleanedKeywords = append(cleanedKeywords, trimmed)
					}
				}
				// Convert keywords to regex patterns
				regexPatterns := convertKeywordsToRegex(cleanedKeywords, logger)

				if len(regexPatterns) > 0 {
					crawlerConfig.ExcludePatterns = regexPatterns
					logger.Info().
						Str("source_id", source.ID).
						Strs("keywords", cleanedKeywords).
						Strs("regex_patterns", regexPatterns).
						Msg("Converted source exclude keywords to regex patterns")
				}
			}
		}

		// Log filter extraction
		logger.Debug().
			Str("source_id", source.ID).
			Int("include_patterns_count", len(crawlerConfig.IncludePatterns)).
			Int("exclude_patterns_count", len(crawlerConfig.ExcludePatterns)).
			Strs("include_patterns", crawlerConfig.IncludePatterns).
			Strs("exclude_patterns", crawlerConfig.ExcludePatterns).
			Msg("Extracted and converted filters from source configuration")
	}

	// 6. Log detailed crawler configuration for debugging
	logger.Debug().
		Str("source_id", source.ID).
		Int("max_depth", crawlerConfig.MaxDepth).
		Int("max_pages", crawlerConfig.MaxPages).
		Str("follow_links", fmt.Sprintf("%v", crawlerConfig.FollowLinks)).
		Int("concurrency", crawlerConfig.Concurrency).
		Dur("rate_limit", crawlerConfig.RateLimit).
		Int("include_pattern_count", len(crawlerConfig.IncludePatterns)).
		Int("exclude_pattern_count", len(crawlerConfig.ExcludePatterns)).
		Str("detail_level", crawlerConfig.DetailLevel).
		Msg("Crawler configuration merged and validated")

	// 6a. Determine configuration source for each field (for INFO logging)
	var followLinksSource string
	if crawlerConfig.FollowLinks {
		followLinksSource = "job"
	} else if source.CrawlConfig.FollowLinks {
		followLinksSource = "source"
	} else {
		followLinksSource = "default"
	}

	var maxDepthSource string
	if jobCrawlConfig.MaxDepth > 0 {
		maxDepthSource = "job"
	} else if source.CrawlConfig.MaxDepth > 0 {
		maxDepthSource = "source"
	} else {
		maxDepthSource = "default"
	}

	var maxPagesSource string
	if jobCrawlConfig.MaxPages > 0 {
		maxPagesSource = "job"
	} else if source.CrawlConfig.MaxPages > 0 {
		maxPagesSource = "source"
	} else {
		maxPagesSource = "default"
	}

	var concurrencySource string
	if jobCrawlConfig.Concurrency > 0 {
		concurrencySource = "job"
	} else if source.CrawlConfig.Concurrency > 0 {
		concurrencySource = "source"
	} else {
		concurrencySource = "default"
	}

	// Filters always come from source configuration
	var includePatternsSource string
	if len(crawlerConfig.IncludePatterns) > 0 {
		includePatternsSource = "source"
	} else {
		includePatternsSource = "none"
	}

	var excludePatternsSource string
	if len(crawlerConfig.ExcludePatterns) > 0 {
		excludePatternsSource = "source"
	} else {
		excludePatternsSource = "none"
	}

	// 6b. Log final configuration with source attribution at INFO level
	logger.Info().
		Str("source_id", source.ID).
		Bool("follow_links", crawlerConfig.FollowLinks).
		Str("follow_links_source", followLinksSource).
		Int("max_depth", crawlerConfig.MaxDepth).
		Str("max_depth_source", maxDepthSource).
		Int("max_pages", crawlerConfig.MaxPages).
		Str("max_pages_source", maxPagesSource).
		Int("concurrency", crawlerConfig.Concurrency).
		Str("concurrency_source", concurrencySource).
		Int("include_patterns_count", len(crawlerConfig.IncludePatterns)).
		Str("include_patterns_source", includePatternsSource).
		Int("exclude_patterns_count", len(crawlerConfig.ExcludePatterns)).
		Str("exclude_patterns_source", excludePatternsSource).
		Msg("Final crawler configuration for job")

	// 7. Warn about limiting crawl settings
	if !crawlerConfig.FollowLinks {
		logger.Warn().Str("source_id", source.ID).Msg("FollowLinks is disabled - crawler will only process initial URLs")
	}
	if crawlerConfig.MaxDepth == 0 {
		logger.Warn().Str("source_id", source.ID).Msg("MaxDepth is 0 - crawler will only process initial URLs")
	}
	if crawlerConfig.MaxPages == 1 {
		logger.Warn().Str("source_id", source.ID).Msg("MaxPages is 1 - crawler will stop after first page")
	}
	if crawlerConfig.MaxPages > 0 && crawlerConfig.MaxPages < 10 {
		logger.Info().Str("source_id", source.ID).Int("max_pages", crawlerConfig.MaxPages).Msg("MaxPages is low - crawler may stop before all content is collected")
	}

	// 8. Generate seed URLs based on source type and base URL
	seedURLs, err := generateSeedURLs(source, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate seed URLs: %w", err)
	}

	// 8a. Validate seed URLs were generated
	if len(seedURLs) == 0 {
		return "", fmt.Errorf("no seed URLs generated for source %s (type: %s)", source.ID, source.Type)
	}

	// 8b. Log seed URL generation success
	logEvent := logger.Info().
		Str("source_id", source.ID).
		Str("source_type", source.Type).
		Int("seed_url_count", len(seedURLs))

	if len(seedURLs) <= 5 {
		logEvent.Strs("seed_urls", seedURLs)
	} else {
		logEvent.Strs("seed_urls_sample", seedURLs[:5]).Int("remaining_count", len(seedURLs)-5)
	}
	logEvent.Msg("Generated seed URLs for crawl job")

	// 8c. Validate source type before starting crawl
	// This prevents jobs from being created with invalid source types like "crawler" at the source
	validSourceTypes := map[string]bool{
		models.SourceTypeJira:       true,
		models.SourceTypeConfluence: true,
		models.SourceTypeGithub:     true,
	}
	if !validSourceTypes[source.Type] {
		err := fmt.Errorf("invalid source type '%s' for source %s: must be one of: jira, confluence, github", source.Type, source.ID)
		logger.Error().Str("source_id", source.ID).Str("source_type", source.Type).Msg("Invalid source type detected")
		return "", err
	}

	// Start crawl with generated seed URLs based on source type and base URL
	jobID, err = crawlerService.StartCrawl(
		source.Type,
		entityType,
		seedURLs, // Generated seed URLs from source configuration
		crawlerConfig,
		source.ID,
		refreshSource, // Pass through refreshSource parameter
		source,
		authCreds,
		jobDefinitionID, // Optional job definition ID for traceability
	)
	if err != nil {
		return "", fmt.Errorf("failed to start crawl: %w", err)
	}

	logEvent = logger.Info().
		Str("job_id", jobID).
		Str("source_id", source.ID)
	if jobDefinitionID != "" {
		logEvent = logEvent.Str("job_definition_id", jobDefinitionID)
		logger.Info().
			Str("job_definition_id", jobDefinitionID).
			Str("job_id", jobID).
			Msg("Crawl job linked to job definition")
	}
	logEvent.Msg("Crawl job started successfully")

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

// generateSeedURLs returns the base URL as the single starting point for crawling
func generateSeedURLs(source *models.SourceConfig, logger arbor.ILogger) ([]string, error) {
	// Validate input
	if source.BaseURL == "" {
		return nil, fmt.Errorf("base URL is empty for source %s", source.ID)
	}

	// Normalize base URL by removing trailing slash for consistency
	originalURL := source.BaseURL
	normalizedURL := strings.TrimRight(source.BaseURL, "/")

	// Log normalization if URL changed
	if originalURL != normalizedURL {
		logger.Debug().
			Str("source_id", source.ID).
			Str("original_url", originalURL).
			Str("normalized_url", normalizedURL).
			Msg("Base URL normalized: trailing slash removed")
	}

	// Use normalized base URL as single starting point
	logger.Debug().
		Str("source_id", source.ID).
		Str("source_type", source.Type).
		Str("base_url", normalizedURL).
		Msg("Using base URL as single starting point for crawl")

	return []string{normalizedURL}, nil
}

// convertKeywordsToRegex converts simple comma-delimited keywords to regex patterns
// If a pattern already looks like regex (contains regex metacharacters), it's used as-is
// Otherwise, simple keywords are converted to substring matching patterns
func convertKeywordsToRegex(keywords []string, logger arbor.ILogger) []string {
	regexPatterns := make([]string, 0, len(keywords))

	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}

		// Check if pattern already looks like regex (contains regex metacharacters)
		isRegex := strings.ContainsAny(keyword, ".+*?[]{}()^$|\\")

		if isRegex {
			// Validate the regex
			_, err := regexp.Compile(keyword)
			if err != nil {
				// Invalid regex, treat as literal keyword
				logger.Warn().
					Str("keyword", keyword).
					Err(err).
					Msg("Invalid regex pattern, treating as literal keyword")
				escapedKeyword := regexp.QuoteMeta(keyword)
				regexPatterns = append(regexPatterns, ".*"+escapedKeyword+".*")
			} else {
				// Valid regex, use as-is
				regexPatterns = append(regexPatterns, keyword)
			}
		} else {
			// Simple keyword, convert to substring matching pattern
			// Escape any special characters and wrap in .*pattern.*
			escapedKeyword := regexp.QuoteMeta(keyword)
			regexPatterns = append(regexPatterns, ".*"+escapedKeyword+".*")
		}
	}

	return regexPatterns
}
