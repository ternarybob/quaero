package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerStepExecutor executes "crawl" action steps
type CrawlerStepExecutor struct {
	crawlerService interfaces.CrawlerService
	logger         arbor.ILogger
}

// NewCrawlerStepExecutor creates a new crawler step executor
func NewCrawlerStepExecutor(
	crawlerService interfaces.CrawlerService,
	logger arbor.ILogger,
) *CrawlerStepExecutor {
	return &CrawlerStepExecutor{
		crawlerService: crawlerService,
		logger:         logger,
	}
}

// ExecuteStep executes a crawl step
func (e *CrawlerStepExecutor) ExecuteStep(ctx context.Context, step models.JobStep, jobDef *models.JobDefinition, parentJobID string) (string, error) {
	// No source type validation needed - crawler is agnostic
	// BaseURL validation removed - crawler uses start_urls from step config

	// Parse step config map into CrawlConfig struct
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Extract entity type from config (default to "issues" for jira, "pages" for confluence)
	entityType := "all"
	if et, ok := stepConfig["entity_type"].(string); ok {
		entityType = et
	} else {
		// Infer from source type
		switch jobDef.SourceType {
		case "jira":
			entityType = "issues"
		case "confluence":
			entityType = "pages"
		}
	}

	// Build CrawlConfig struct from map with proper defaults
	crawlConfig := e.buildCrawlConfig(stepConfig)

	// Build seed URLs - prioritize start_urls from step config, fallback to source type
	var seedURLs []string
	if startURLs, ok := stepConfig["start_urls"].([]interface{}); ok && len(startURLs) > 0 {
		// Use start_urls from job definition config
		for _, url := range startURLs {
			if urlStr, ok := url.(string); ok {
				seedURLs = append(seedURLs, urlStr)
			}
		}
		e.logger.Info().
			Str("step_name", step.Name).
			Strs("start_urls", seedURLs).
			Msg("Using start_urls from job definition config")
	} else if startURLsStr, ok := stepConfig["start_urls"].([]string); ok && len(startURLsStr) > 0 {
		// Handle case where start_urls is already []string
		seedURLs = startURLsStr
		e.logger.Info().
			Str("step_name", step.Name).
			Strs("start_urls", seedURLs).
			Msg("Using start_urls from job definition config")
	} else {
		// Fallback to building URLs based on source type and entity type
		seedURLs = e.buildSeedURLs(jobDef.BaseURL, jobDef.SourceType, entityType)
		e.logger.Info().
			Str("step_name", step.Name).
			Str("source_type", jobDef.SourceType).
			Str("base_url", jobDef.BaseURL).
			Strs("generated_urls", seedURLs).
			Msg("Using generated URLs based on source type (no start_urls in config)")
	}

	// Default source_type to "web" for generic crawler jobs
	// This maintains job-type agnostic architecture while supporting source-specific crawling
	sourceType := jobDef.SourceType
	if sourceType == "" {
		sourceType = "web"
		e.logger.Info().
			Str("step_name", step.Name).
			Msg("No source_type specified, defaulting to 'web' for generic web crawling")
	}

	e.logger.Info().
		Str("step_name", step.Name).
		Str("source_type", sourceType).
		Str("base_url", jobDef.BaseURL).
		Str("entity_type", entityType).
		Int("seed_url_count", len(seedURLs)).
		Int("max_depth", crawlConfig.MaxDepth).
		Int("max_pages", crawlConfig.MaxPages).
		Msg("Executing crawl step")

	// Start crawl job with properly typed config
	jobID, err := e.crawlerService.StartCrawl(
		sourceType,
		entityType,
		seedURLs,
		crawlConfig,   // Pass CrawlConfig struct
		jobDef.AuthID, // sourceID - use auth_id as source identifier
		false,         // refreshSource
		nil,           // sourceConfigSnapshot
		nil,           // authSnapshot
		parentJobID,   // jobDefinitionID - link to parent
	)

	if err != nil {
		return "", fmt.Errorf("failed to start crawl (source_type=%s, step=%s): %w", sourceType, step.Name, err)
	}

	e.logger.Info().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("parent_job_id", parentJobID).
		Msg("Crawl step started successfully")

	return jobID, nil
}

// GetStepType returns "crawl"
func (e *CrawlerStepExecutor) GetStepType() string {
	return "crawl"
}

// buildCrawlConfig constructs a CrawlConfig struct from a config map
func (e *CrawlerStepExecutor) buildCrawlConfig(configMap map[string]interface{}) crawler.CrawlConfig {
	config := crawler.CrawlConfig{
		MaxDepth:      2,
		MaxPages:      100,
		Concurrency:   5,
		RateLimit:     time.Second,
		RetryAttempts: 3,
		RetryBackoff:  time.Second,
		FollowLinks:   true,
		DetailLevel:   "full",
	}

	// Override with values from config map
	if v, ok := configMap["max_depth"].(float64); ok {
		config.MaxDepth = int(v)
	} else if v, ok := configMap["max_depth"].(int); ok {
		config.MaxDepth = v
	}

	if v, ok := configMap["max_pages"].(float64); ok {
		config.MaxPages = int(v)
	} else if v, ok := configMap["max_pages"].(int); ok {
		config.MaxPages = v
	}

	if v, ok := configMap["concurrency"].(float64); ok {
		config.Concurrency = int(v)
	} else if v, ok := configMap["concurrency"].(int); ok {
		config.Concurrency = v
	}

	if v, ok := configMap["rate_limit"].(float64); ok {
		config.RateLimit = time.Duration(v) * time.Millisecond
	} else if v, ok := configMap["rate_limit"].(int); ok {
		config.RateLimit = time.Duration(v) * time.Millisecond
	}

	if v, ok := configMap["retry_attempts"].(float64); ok {
		config.RetryAttempts = int(v)
	} else if v, ok := configMap["retry_attempts"].(int); ok {
		config.RetryAttempts = v
	}

	if v, ok := configMap["retry_backoff"].(float64); ok {
		config.RetryBackoff = time.Duration(v) * time.Millisecond
	} else if v, ok := configMap["retry_backoff"].(int); ok {
		config.RetryBackoff = time.Duration(v) * time.Millisecond
	}

	if v, ok := configMap["follow_links"].(bool); ok {
		config.FollowLinks = v
	}

	if v, ok := configMap["detail_level"].(string); ok {
		config.DetailLevel = v
	}

	if v, ok := configMap["include_patterns"].([]string); ok {
		config.IncludePatterns = v
	} else if v, ok := configMap["include_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(v))
		for _, pattern := range v {
			if s, ok := pattern.(string); ok {
				patterns = append(patterns, s)
			}
		}
		config.IncludePatterns = patterns
	}

	if v, ok := configMap["exclude_patterns"].([]string); ok {
		config.ExcludePatterns = v
	} else if v, ok := configMap["exclude_patterns"].([]interface{}); ok {
		patterns := make([]string, 0, len(v))
		for _, pattern := range v {
			if s, ok := pattern.(string); ok {
				patterns = append(patterns, s)
			}
		}
		config.ExcludePatterns = patterns
	}

	return config
}

// buildSeedURLs constructs seed URLs based on source type and entity type
func (e *CrawlerStepExecutor) buildSeedURLs(baseURL, sourceType, entityType string) []string {
	switch sourceType {
	case "jira":
		switch entityType {
		case "projects":
			return []string{baseURL + "/rest/api/2/project"}
		case "issues":
			return []string{baseURL + "/rest/api/2/search"}
		default:
			return []string{baseURL + "/rest/api/2/project"}
		}
	case "confluence":
		switch entityType {
		case "spaces":
			return []string{baseURL + "/rest/api/space"}
		case "pages":
			return []string{baseURL + "/rest/api/content"}
		default:
			return []string{baseURL + "/rest/api/space"}
		}
	default:
		return []string{baseURL}
	}
}
