package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// CrawlerActionDeps holds dependencies needed by crawler action handlers.
type CrawlerActionDeps struct {
	CrawlerService *crawler.Service
	AuthStorage    interfaces.AuthStorage
	EventService   interfaces.EventService
	Config         *common.Config
	Logger         arbor.ILogger
}

// startCrawlJobFunc is a package-level variable that can be swapped in tests
var startCrawlJobFunc = jobs.StartCrawlJob

// crawlAction performs actual crawling of sources and publishes collection events.
func crawlAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *CrawlerActionDeps) error {
	// Extract configuration parameters
	// Note: seed_url_overrides removed - crawling based on source configuration
	refreshSource := extractBool(step.Config, "refresh_source", true)
	waitForCompletion := extractBool(step.Config, "wait_for_completion", true)

	// Extract filtering patterns from job step config (shared across all sources)
	includePatterns := extractStringSlice(step.Config, "include_patterns")
	excludePatterns := extractStringSlice(step.Config, "exclude_patterns")

	// Extract optional crawl settings that override source defaults (shared across all sources)
	maxDepth := extractInt(step.Config, "max_depth", 0)
	maxPages := extractInt(step.Config, "max_pages", 0)
	concurrency := extractInt(step.Config, "concurrency", 0)

	// Check if follow_links is explicitly provided in config
	_, followLinksProvided := step.Config["follow_links"]
	followLinks := extractBool(step.Config, "follow_links", true)

	// Validate sources
	if len(sources) == 0 {
		return fmt.Errorf("no sources provided for crawl action")
	}

	deps.Logger.Info().
		Str("action", "crawl").
		Int("source_count", len(sources)).
		Bool("wait_for_completion", waitForCompletion).
		Int("include_pattern_count", len(includePatterns)).
		Int("exclude_pattern_count", len(excludePatterns)).
		Msg("Starting crawl action")

	// Track started jobs
	type jobInfo struct {
		jobID      string
		sourceID   string
		sourceName string
		sourceType string
	}
	var startedJobs []jobInfo
	var errors []error

	// Process each source
	for _, source := range sources {
		startTime := time.Now()

		// Build CrawlConfig for this source
		jobCrawlConfig := crawler.CrawlConfig{
			IncludePatterns: includePatterns,
			ExcludePatterns: excludePatterns,
			MaxDepth:        maxDepth,
			MaxPages:        maxPages,
			Concurrency:     concurrency,
			FollowLinks:     followLinks,
		}

		// Log the decision path for follow_links configuration
		if followLinksProvided {
			deps.Logger.Info().
				Str("source_id", source.ID).
				Str("source_name", source.Name).
				Bool("follow_links", jobCrawlConfig.FollowLinks).
				Msg("Using follow_links from job config")
		} else {
			deps.Logger.Info().
				Str("source_id", source.ID).
				Str("source_name", source.Name).
				Bool("follow_links", jobCrawlConfig.FollowLinks).
				Msg("Using follow_links from source default")
		}

		deps.Logger.Info().
			Str("action", "crawl").
			Str("source_id", source.ID).
			Str("source_type", string(source.Type)).
			Str("base_url", source.BaseURL).
			Int("include_patterns", len(includePatterns)).
			Int("exclude_patterns", len(excludePatterns)).
			Bool("follow_links", jobCrawlConfig.FollowLinks).
			Bool("follow_links_from_job", followLinksProvided).
			Msg("Starting crawl for source")

		// Start crawl job using helper (function variable for testability)
		// Note: Seed URLs removed - crawling based on source base_url and type
		jobID, err := startCrawlJobFunc(
			ctx,
			source,
			deps.AuthStorage,
			deps.CrawlerService,
			deps.Config,
			deps.Logger,
			jobCrawlConfig,
			refreshSource,
		)

		if err != nil {
			errMsg := fmt.Errorf("failed to start crawl for source %s: %w", source.ID, err)
			deps.Logger.Error().
				Err(err).
				Str("source_id", source.ID).
				Msg("Failed to start crawl job")

			errors = append(errors, errMsg)

			// Check error strategy
			if step.OnError == models.ErrorStrategyFail {
				return errMsg
			}
			continue
		}

		startedJobs = append(startedJobs, jobInfo{
			jobID:      jobID,
			sourceID:   source.ID,
			sourceName: source.Name,
			sourceType: string(source.Type),
		})

		deps.Logger.Info().
			Str("action", "crawl").
			Str("job_id", jobID).
			Str("source_id", source.ID).
			Dur("duration", time.Since(startTime)).
			Msg("Crawl job started successfully")
	}

	// Store job IDs in step config for executor polling
	jobIDs := make([]string, len(startedJobs))
	for i, job := range startedJobs {
		jobIDs[i] = job.jobID
	}

	// Guard against nil map
	if step.Config == nil {
		step.Config = make(map[string]interface{})
	}
	step.Config["crawl_job_ids"] = jobIDs

	// Log job IDs stored for async polling
	// Compute limit for logging (show first 3 job IDs)
	limit := 3
	if len(jobIDs) < limit {
		limit = len(jobIDs)
	}

	deps.Logger.Debug().
		Str("action", "crawl").
		Int("job_count", len(jobIDs)).
		Strs("job_ids", jobIDs[:limit]).
		Msg("Stored crawl job IDs in step config for async polling")

	// Note: Event publishing removed from crawlAction to avoid duplication with transformAction.
	// The dedicated transformAction should be used to trigger document transformation after crawling.

	// Return aggregated errors if any
	if len(errors) > 0 {
		return fmt.Errorf("crawl action completed with %d error(s): %v", len(errors), errors)
	}

	deps.Logger.Info().
		Str("action", "crawl").
		Int("source_count", len(sources)).
		Int("jobs_started", len(startedJobs)).
		Bool("wait_for_completion", waitForCompletion).
		Msg("Crawl action started successfully - jobs running asynchronously")

	return nil
}

// transformAction triggers document transformation via collection events.
// This action is fire-and-forget: it publishes events but does not wait for processing completion.
// For wait-for-completion functionality, use a separate polling mechanism or workflow orchestration.
func transformAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *CrawlerActionDeps) error {
	// Extract configuration parameters
	jobID := extractString(step.Config, "job_id", "")
	forceSync := extractBool(step.Config, "force_sync", false)
	batchSize := extractInt(step.Config, "batch_size", 100)

	deps.Logger.Info().
		Str("action", "transform").
		Int("source_count", len(sources)).
		Bool("force_sync", forceSync).
		Int("batch_size", batchSize).
		Msg("Starting transform action")

	// If no sources specified, publish once for all sources
	if len(sources) == 0 {
		deps.Logger.Info().
			Str("action", "transform").
			Msg("No sources specified, publishing event for all sources")

		payload := map[string]interface{}{
			"job_id":     jobID,
			"force_sync": forceSync,
			"batch_size": batchSize,
			"timestamp":  time.Now(),
		}

		err := deps.EventService.PublishSync(ctx, interfaces.Event{
			Type:    interfaces.EventCollectionTriggered,
			Payload: payload,
		})
		if err != nil {
			return fmt.Errorf("failed to publish collection event: %w", err)
		}

		deps.Logger.Info().
			Str("action", "transform").
			Msg("Published collection triggered event for all sources")

		return nil
	}

	// Publish collection event for each source
	for _, source := range sources {
		payload := map[string]interface{}{
			"job_id":      jobID,
			"source_id":   source.ID,
			"source_type": string(source.Type),
			"force_sync":  forceSync,
			"batch_size":  batchSize,
			"timestamp":   time.Now(),
		}

		err := deps.EventService.PublishSync(ctx, interfaces.Event{
			Type:    interfaces.EventCollectionTriggered,
			Payload: payload,
		})
		if err != nil {
			return fmt.Errorf("failed to publish collection event for source %s: %w", source.ID, err)
		}

		deps.Logger.Info().
			Str("action", "transform").
			Str("source_id", source.ID).
			Str("source_type", string(source.Type)).
			Bool("force_sync", forceSync).
			Int("batch_size", batchSize).
			Msg("Published collection triggered event")
	}

	deps.Logger.Info().
		Str("action", "transform").
		Int("source_count", len(sources)).
		Msg("Transform action completed successfully")

	return nil
}

// embedAction triggers embedding generation via embedding events.
// This action is fire-and-forget: it publishes events but does not wait for processing completion.
// For wait-for-completion functionality, use a separate polling mechanism or workflow orchestration.
func embedAction(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig, deps *CrawlerActionDeps) error {
	// Extract configuration parameters
	forceEmbed := extractBool(step.Config, "force_embed", false)
	batchSize := extractInt(step.Config, "batch_size", 50)
	modelName := extractString(step.Config, "model_name", "")
	filterSourceIDs := extractStringSlice(step.Config, "filter_source_ids")

	deps.Logger.Info().
		Str("action", "embed").
		Bool("force_embed", forceEmbed).
		Int("batch_size", batchSize).
		Str("model_name", modelName).
		Int("filter_source_count", len(filterSourceIDs)).
		Msg("Starting embed action")

	// Build payload
	payload := map[string]interface{}{
		"force_embed": forceEmbed,
		"batch_size":  batchSize,
		"timestamp":   time.Now(),
	}

	if modelName != "" {
		payload["model_name"] = modelName
	}

	if len(filterSourceIDs) > 0 {
		payload["filter_source_ids"] = filterSourceIDs
	} else if len(sources) > 0 {
		// If sources specified, add their IDs to filter
		var sourceIDs []string
		for _, source := range sources {
			sourceIDs = append(sourceIDs, source.ID)
		}
		payload["filter_source_ids"] = sourceIDs
	}

	// Publish embedding event
	err := deps.EventService.PublishSync(ctx, interfaces.Event{
		Type:    interfaces.EventEmbeddingTriggered,
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish embedding event: %w", err)
	}

	deps.Logger.Info().
		Str("action", "embed").
		Bool("force_embed", forceEmbed).
		Int("batch_size", batchSize).
		Msg("Published embedding triggered event")

	deps.Logger.Info().
		Str("action", "embed").
		Msg("Embed action completed successfully")

	return nil
}

// RegisterCrawlerActions registers all crawler-related actions with the job type registry.
func RegisterCrawlerActions(registry *jobs.JobTypeRegistry, deps *CrawlerActionDeps) error {
	// Validate inputs
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}
	if deps == nil {
		return fmt.Errorf("dependencies cannot be nil")
	}
	if deps.CrawlerService == nil {
		return fmt.Errorf("CrawlerService dependency cannot be nil")
	}
	if deps.AuthStorage == nil {
		return fmt.Errorf("AuthStorage dependency cannot be nil")
	}
	if deps.EventService == nil {
		return fmt.Errorf("EventService dependency cannot be nil")
	}
	if deps.Config == nil {
		return fmt.Errorf("Config dependency cannot be nil")
	}
	if deps.Logger == nil {
		return fmt.Errorf("Logger dependency cannot be nil")
	}

	// Create closure functions that capture dependencies
	crawlActionHandler := func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		return crawlAction(ctx, step, sources, deps)
	}

	transformActionHandler := func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		return transformAction(ctx, step, sources, deps)
	}

	embedActionHandler := func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		return embedAction(ctx, step, sources, deps)
	}

	// Register actions
	if err := registry.RegisterAction(models.JobTypeCrawler, "crawl", crawlActionHandler); err != nil {
		return fmt.Errorf("failed to register crawl action: %w", err)
	}

	if err := registry.RegisterAction(models.JobTypeCrawler, "transform", transformActionHandler); err != nil {
		return fmt.Errorf("failed to register transform action: %w", err)
	}

	if err := registry.RegisterAction(models.JobTypeCrawler, "embed", embedActionHandler); err != nil {
		return fmt.Errorf("failed to register embed action: %w", err)
	}

	deps.Logger.Info().
		Str("job_type", string(models.JobTypeCrawler)).
		Int("action_count", 3).
		Msg("Crawler actions registered successfully")

	return nil
}

// Helper functions for config extraction

// extractStringSlice extracts a string slice from config map with type assertion.
func extractStringSlice(config map[string]interface{}, key string) []string {
	if config == nil {
		return nil
	}

	value, ok := config[key]
	if !ok {
		return nil
	}

	// Try direct string slice
	if strSlice, ok := value.([]string); ok {
		return strSlice
	}

	// Try []interface{} with string elements
	if ifaceSlice, ok := value.([]interface{}); ok {
		result := make([]string, 0, len(ifaceSlice))
		for _, item := range ifaceSlice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

// extractBool extracts a boolean from config map with type assertion.
func extractBool(config map[string]interface{}, key string, defaultValue bool) bool {
	if config == nil {
		return defaultValue
	}

	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	if boolVal, ok := value.(bool); ok {
		return boolVal
	}

	return defaultValue
}

// extractInt extracts an integer from config map with type assertion.
func extractInt(config map[string]interface{}, key string, defaultValue int) int {
	if config == nil {
		return defaultValue
	}

	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	// Try direct int
	if intVal, ok := value.(int); ok {
		return intVal
	}

	// Try float64 (JSON unmarshaling)
	if floatVal, ok := value.(float64); ok {
		return int(floatVal)
	}

	return defaultValue
}

// extractString extracts a string from config map with type assertion.
func extractString(config map[string]interface{}, key string, defaultValue string) string {
	if config == nil {
		return defaultValue
	}

	value, ok := config[key]
	if !ok {
		return defaultValue
	}

	if strVal, ok := value.(string); ok {
		return strVal
	}

	return defaultValue
}
