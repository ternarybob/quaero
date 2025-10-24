package types

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerJobDeps holds dependencies for crawler jobs
type CrawlerJobDeps struct {
	CrawlerService  *crawler.Service
	LogService      interfaces.LogService
	DocumentStorage interfaces.DocumentStorage
	QueueManager    interfaces.QueueManager
	JobStorage      interfaces.JobStorage
	EventService    interfaces.EventService
}

// CrawlerJob handles URL crawling jobs
type CrawlerJob struct {
	*BaseJob
	deps *CrawlerJobDeps
}

// NewCrawlerJob creates a new crawler job
func NewCrawlerJob(base *BaseJob, deps *CrawlerJobDeps) *CrawlerJob {
	return &CrawlerJob{
		BaseJob: base,
		deps:    deps,
	}
}

// Execute processes a crawler URL job
func (c *CrawlerJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	c.logger.Info().
		Str("message_id", msg.ID).
		Str("url", msg.URL).
		Int("depth", msg.Depth).
		Str("parent_id", msg.ParentID).
		Msg("Processing crawler URL job")

	// Validate message
	if err := c.Validate(msg); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	// Extract configuration from message
	maxDepth := 3 // Default
	if depth, ok := msg.Config["max_depth"].(float64); ok {
		maxDepth = int(depth)
	}

	followLinks := true // Default
	if follow, ok := msg.Config["follow_links"].(bool); ok {
		followLinks = follow
	}

	// Check if we should process this URL (depth check)
	if msg.Depth > maxDepth {
		c.logger.Debug().
			Str("url", msg.URL).
			Int("depth", msg.Depth).
			Int("max_depth", maxDepth).
			Msg("Skipping URL due to depth limit")
		return nil
	}

	// Log job start
	if err := c.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Processing URL: %s (depth=%d)", msg.URL, msg.Depth)); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to log job start event")
	}

	// Extract source metadata
	sourceType := "web"
	if st, ok := msg.Config["source_type"].(string); ok {
		sourceType = st
	}

	// Fetch URL content using crawler service with auth-aware HTTP and HTML parsing
	c.logger.Debug().
		Str("url", msg.URL).
		Int("depth", msg.Depth).
		Msg("Fetching URL content using crawler service")

	// Build auth-aware HTTP client from crawler service
	httpClient, err := c.deps.CrawlerService.BuildHTTPClientFromAuth(ctx)
	if err != nil {
		c.logger.Warn().
			Err(err).
			Str("url", msg.URL).
			Msg("Failed to build HTTP client from auth, using default client")
		httpClient = nil // Will use default client in HTMLScraper
	}

	// Get base crawler config from service (contains user agent, timeouts, etc.)
	crawlerConfig := c.deps.CrawlerService.GetConfig()

	// Apply job-level config overrides from message
	// Make a copy to avoid modifying the base config
	mergedConfig := crawlerConfig

	// Apply rate limit override (msg.Config["rate_limit"] -> RequestDelay)
	if rateLimit, ok := msg.Config["rate_limit"].(float64); ok && rateLimit > 0 {
		mergedConfig.RequestDelay = time.Duration(rateLimit) * time.Millisecond
	}

	// Apply concurrency override (constrain to reasonable limits)
	if concurrency, ok := msg.Config["concurrency"].(float64); ok {
		if concurrency > 0 && concurrency <= 10 { // Max 10 concurrent workers
			mergedConfig.MaxConcurrency = int(concurrency)
		}
	}

	// Apply max depth override
	if maxDepthOverride, ok := msg.Config["max_depth"].(float64); ok && maxDepthOverride > 0 {
		mergedConfig.MaxDepth = int(maxDepthOverride)
	}

	// Apply JavaScript rendering override
	if jsRendering, ok := msg.Config["javascript_rendering"].(bool); ok {
		mergedConfig.EnableJavaScript = jsRendering
	}

	// Apply timeout override
	if timeout, ok := msg.Config["timeout"].(float64); ok && timeout > 0 {
		mergedConfig.RequestTimeout = time.Duration(timeout) * time.Second
	}

	c.logger.Debug().
		Dur("request_delay", mergedConfig.RequestDelay).
		Int("max_concurrency", mergedConfig.MaxConcurrency).
		Int("max_depth", mergedConfig.MaxDepth).
		Bool("enable_javascript", mergedConfig.EnableJavaScript).
		Dur("request_timeout", mergedConfig.RequestTimeout).
		Msg("Merged job-level config with base crawler config")

	// Create HTMLScraper with auth-aware HTTP client and merged config
	var cookies []*http.Cookie
	if httpClient != nil && httpClient.Jar != nil {
		parsedURL, err := url.Parse(msg.URL)
		if err == nil {
			cookies = httpClient.Jar.Cookies(parsedURL)
		}
	}

	scraper := crawler.NewHTMLScraper(mergedConfig, c.logger, httpClient, cookies)
	defer scraper.Close()

	// Scrape the URL (handles JavaScript rendering, HTML parsing, markdown conversion)
	scrapeResult, err := scraper.ScrapeURL(ctx, msg.URL)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("url", msg.URL).
			Int("depth", msg.Depth).
			Msg("Failed to scrape URL")

		// Update parent job progress even on failure
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*crawler.CrawlJob); ok {
				job.Progress.CompletedURLs++ // Count as processed (failed)
				if job.Progress.PendingURLs > 0 {
					job.Progress.PendingURLs--
				}
				job.Progress.FailedURLs++ // Track failures
				if job.Progress.TotalURLs > 0 {
					job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
				}
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to persist progress after scraping failure")
				}
			}
		}

		return fmt.Errorf("failed to scrape URL: %w", err)
	}

	// Check if scraping was successful (2xx status code)
	if !scrapeResult.Success {
		c.logger.Warn().
			Str("url", msg.URL).
			Int("status_code", scrapeResult.StatusCode).
			Str("error", scrapeResult.Error).
			Msg("Scraping returned non-success status")

		// Log non-success scraping
		if err := c.LogJobEvent(ctx, msg.ParentID, "warning",
			fmt.Sprintf("URL returned status %d: %s", scrapeResult.StatusCode, msg.URL)); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to log scraping warning")
		}

		// Update parent job progress for non-success status
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*crawler.CrawlJob); ok {
				job.Progress.CompletedURLs++ // Count as processed (failed)
				if job.Progress.PendingURLs > 0 {
					job.Progress.PendingURLs--
				}
				job.Progress.FailedURLs++ // Track failures
				if job.Progress.TotalURLs > 0 {
					job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
				}
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to persist progress after non-success status")
				}
			}
		}

		// Return error for non-2xx responses
		return fmt.Errorf("scraping failed with status %d: %s", scrapeResult.StatusCode, scrapeResult.Error)
	}

	// Extract content (prioritize markdown for LLM consumption)
	content := scrapeResult.GetContent()
	if content == "" {
		c.logger.Warn().
			Str("url", msg.URL).
			Msg("Scraping produced empty content")

		// Log empty content warning
		if err := c.LogJobEvent(ctx, msg.ParentID, "warning",
			fmt.Sprintf("Empty content from URL: %s", msg.URL)); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to log empty content warning")
		}

		// Continue with empty content rather than failing
	}

	// Build real document with actual scraped data
	document := &models.Document{
		ID:              fmt.Sprintf("doc_%s_%d", msg.ID, time.Now().Unix()),
		SourceID:        msg.URL,
		SourceType:      sourceType,
		Title:           scrapeResult.Title,
		ContentMarkdown: content,
		DetailLevel:     models.DetailLevelFull,
		URL:             msg.URL,
		Metadata: map[string]interface{}{
			"crawled_depth": msg.Depth,
			"parent_id":     msg.ParentID,
			"url":           msg.URL,
			"title":         scrapeResult.Title,
			"description":   scrapeResult.Description,
			"language":      scrapeResult.Language,
			"status_code":   scrapeResult.StatusCode,
			"scraped_at":    scrapeResult.Timestamp.Format(time.RFC3339),
			"duration_ms":   scrapeResult.Duration.Milliseconds(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	c.logger.Debug().
		Str("url", msg.URL).
		Str("title", scrapeResult.Title).
		Int("content_length", len(content)).
		Int("links_found", len(scrapeResult.Links)).
		Msg("Successfully scraped URL")

	// Save document to storage
	if err := c.deps.DocumentStorage.SaveDocument(document); err != nil {
		c.logger.Error().Err(err).Str("url", msg.URL).Msg("Failed to save document")
		return fmt.Errorf("failed to save document: %w", err)
	}

	c.logger.Info().
		Str("url", msg.URL).
		Str("document_id", document.ID).
		Msg("Document saved successfully")

	// Update parent job progress after successful URL processing
	jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
	if err != nil {
		c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to load parent job for progress update")
	} else {
		if job, ok := jobInterface.(*crawler.CrawlJob); ok {
			// Increment completed, decrement pending
			job.Progress.CompletedURLs++
			if job.Progress.PendingURLs > 0 {
				job.Progress.PendingURLs--
			}
			// Recompute percentage
			if job.Progress.TotalURLs > 0 {
				job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
			}

			// VERIFICATION COMMENT 1: Completion check moved to after child URL increments
			// Don't check completion here - children may be spawned which would increment PendingURLs
			// Completion check happens after link discovery/enqueueing (see below)

			// Save updated progress
			if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
				c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to persist progress update")
			} else {
				c.logger.Debug().
					Str("parent_id", msg.ParentID).
					Int("completed", job.Progress.CompletedURLs).
					Int("pending", job.Progress.PendingURLs).
					Float64("percentage", job.Progress.Percentage).
					Str("status", string(job.Status)).
					Msg("Progress updated successfully")
			}

			// Emit progress event (only if not completed, completion event already logged)
			if job.Status != crawler.JobStatusCompleted {
				if err := c.LogJobEvent(ctx, msg.ParentID, "info",
					fmt.Sprintf("Progress: %d/%d URLs completed (%.1f%%)",
						job.Progress.CompletedURLs, job.Progress.TotalURLs, job.Progress.Percentage)); err != nil {
					c.logger.Warn().Err(err).Msg("Failed to log progress event")
				}
			}
		} else {
			c.logger.Warn().Str("parent_id", msg.ParentID).Msg("Failed to type assert job to *crawler.CrawlJob")
		}
	}

	// Discover and enqueue child links if follow_links is enabled
	if followLinks && msg.Depth < maxDepth {
		c.logger.Debug().
			Str("url", msg.URL).
			Int("depth", msg.Depth).
			Int("max_depth", maxDepth).
			Msg("Link discovery enabled, extracting and enqueueing child URLs")

		// VERIFICATION COMMENT 1: URL deduplication now handled by database (job_seen_urls table)
		// No need to load parent job or manage in-memory SeenURLs map

		// Extract include/exclude patterns from config
		includePatterns := []string{}
		if patterns, ok := msg.Config["include_patterns"].([]interface{}); ok {
			for _, p := range patterns {
				if str, ok := p.(string); ok {
					includePatterns = append(includePatterns, str)
				}
			}
		}

		excludePatterns := []string{}
		if patterns, ok := msg.Config["exclude_patterns"].([]interface{}); ok {
			for _, p := range patterns {
				if str, ok := p.(string); ok {
					excludePatterns = append(excludePatterns, str)
				}
			}
		}

		// Extract max_pages limit
		maxPages := 0 // 0 means unlimited
		if pages, ok := msg.Config["max_pages"].(float64); ok {
			maxPages = int(pages)
		}

		// Use links already extracted by HTMLScraper (normalized and deduplicated)
		discoveredLinks := scrapeResult.Links

		c.logger.Debug().
			Int("discovered_count", len(discoveredLinks)).
			Int("include_patterns", len(includePatterns)).
			Int("exclude_patterns", len(excludePatterns)).
			Int("max_pages", maxPages).
			Msg("Processing discovered links")

		// VERIFICATION COMMENT 3: Use shared LinkFilter helper (DRY principle)
		// Replaces inline regex compilation with consolidated filtering logic
		linkFilter := crawler.NewLinkFilter(includePatterns, excludePatterns, sourceType, c.logger)

		// Filter and enqueue child jobs for discovered links
		enqueuedCount := 0
		filteredCount := 0
		duplicateCount := 0

		for _, childURL := range discoveredLinks {
			// VERIFICATION COMMENT 1: Concurrency-safe URL deduplication using database
			// MarkURLSeen atomically checks and records URL as seen using INSERT OR IGNORE
			// Returns true if URL was newly added (safe to enqueue), false if already exists (duplicate)
			isNew, err := c.deps.JobStorage.MarkURLSeen(ctx, msg.ParentID, childURL)
			if err != nil {
				c.logger.Warn().
					Err(err).
					Str("url", childURL).
					Msg("Failed to check URL uniqueness, skipping to avoid potential duplicate")
				continue
			}

			if !isNew {
				duplicateCount++
				c.logger.Debug().
					Str("url", childURL).
					Msg("URL already enqueued (duplicate detected by database), skipping")
				continue
			}

			// Respect max_pages limit if configured
			if maxPages > 0 && enqueuedCount >= maxPages {
				c.logger.Info().
					Int("enqueued", enqueuedCount).
					Int("max_pages", maxPages).
					Msg("Reached max_pages limit, stopping link enqueueing")
				break
			}

			// VERIFICATION COMMENT 3: Apply consolidated filtering using shared LinkFilter
			filterResult := linkFilter.FilterURL(childURL)
			if !filterResult.ShouldEnqueue {
				filteredCount++
				c.logger.Debug().
					Str("url", childURL).
					Str("reason", filterResult.Reason).
					Str("excluded_by", filterResult.ExcludedBy).
					Msg("URL filtered out, skipping")
				continue
			}

			// Create child job message
			// VERIFICATION COMMENT 7: ParentID hierarchy uses FLAT structure.
			// All child messages inherit the root job's ParentID rather than pointing
			// to their immediate parent. This design choice supports:
			// - Centralized progress tracking at root job level
			// - Job-level URL deduplication via database table
			// - Simplified job completion detection (single PendingURLs counter)
			// - Consistent logging and event tracking
			// Alternative tree structure would require recursive traversal and
			// complex aggregation, adding unnecessary complexity for crawler use case.
			childMsg := &queue.JobMessage{
				ID:              fmt.Sprintf("%s-child-%d", msg.ID, enqueuedCount),
				Type:            "crawler_url",
				URL:             childURL,
				Depth:           msg.Depth + 1,
				ParentID:        msg.ParentID, // Inherit root job ID (flat structure)
				JobDefinitionID: msg.JobDefinitionID,
				Config:          msg.Config, // Inherit parent config
			}

			// Enqueue child job via BaseJob.EnqueueChildJob
			if err := c.EnqueueChildJob(ctx, childMsg); err != nil {
				c.logger.Warn().
					Err(err).
					Str("child_url", childURL).
					Msg("Failed to enqueue child job")
				// NOTE: URL already marked as seen in database, so subsequent workers won't retry
				// This prevents duplicate enqueueing even if this worker fails to enqueue
				continue
			}

			enqueuedCount++

			// Publish job spawn event for real-time UI updates
			if c.deps.EventService != nil {
				spawnEvent := interfaces.Event{
					Type: interfaces.EventJobSpawn,
					Payload: map[string]interface{}{
						"parent_job_id": msg.ParentID,
						"child_job_id":  childMsg.ID,
						"job_type":      "crawler_url",
						"url":           childURL,
						"depth":         msg.Depth + 1,
						"timestamp":     time.Now(),
					},
				}
				c.deps.EventService.Publish(ctx, spawnEvent)
			}

			c.logger.Debug().
				Str("child_url", childURL).
				Int("depth", msg.Depth+1).
				Msg("Child job enqueued successfully")
		}

		c.logger.Info().
			Int("discovered", len(discoveredLinks)).
			Int("duplicates", duplicateCount).
			Int("filtered", filteredCount).
			Int("enqueued", enqueuedCount).
			Msg("Child jobs enqueued for discovered links")

		// VERIFICATION COMMENT 1: Increment TotalURLs and PendingURLs for spawned children
		// Child URLs were successfully enqueued, so update parent job counters
		if enqueuedCount > 0 {
			// Load parent job to update counters
			parentJobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
			if err != nil {
				c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to load parent job for counter update")
			} else if parentJob, ok := parentJobInterface.(*crawler.CrawlJob); ok {
				// Increment counters by the number of successfully enqueued children
				parentJob.Progress.TotalURLs += enqueuedCount
				parentJob.Progress.PendingURLs += enqueuedCount

				// Recompute percentage with new totals
				if parentJob.Progress.TotalURLs > 0 {
					parentJob.Progress.Percentage = float64(parentJob.Progress.CompletedURLs) / float64(parentJob.Progress.TotalURLs) * 100
				}

				// Persist updated counters
				if err := c.deps.JobStorage.SaveJob(ctx, parentJob); err != nil {
					c.logger.Warn().
						Err(err).
						Str("parent_id", msg.ParentID).
						Int("enqueued_count", enqueuedCount).
						Msg("Failed to persist progress counters after child enqueueing")
				} else {
					c.logger.Debug().
						Str("parent_id", msg.ParentID).
						Int("total_urls", parentJob.Progress.TotalURLs).
						Int("pending_urls", parentJob.Progress.PendingURLs).
						Int("enqueued_children", enqueuedCount).
						Float64("percentage", parentJob.Progress.Percentage).
						Msg("Updated progress counters for spawned children")
				}
			}
		}
	}

	// VERIFICATION COMMENT 1: Check job completion after child URL increments
	// This ensures we account for any spawned children before marking job complete
	jobInterfaceForCompletion, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
	if err == nil {
		if jobForCompletion, ok := jobInterfaceForCompletion.(*crawler.CrawlJob); ok {
			// Check if job is complete (all URLs processed, including any spawned children)
			if jobForCompletion.Progress.PendingURLs == 0 && jobForCompletion.Progress.TotalURLs > 0 {
				jobForCompletion.Status = crawler.JobStatusCompleted
				c.logger.Info().
					Str("parent_id", msg.ParentID).
					Int("total_urls", jobForCompletion.Progress.TotalURLs).
					Int("completed_urls", jobForCompletion.Progress.CompletedURLs).
					Int("failed_urls", jobForCompletion.Progress.FailedURLs).
					Msg("Job completed - all URLs processed (including spawned children)")

				// Log job completion event
				if err := c.LogJobEvent(ctx, msg.ParentID, "info",
					fmt.Sprintf("Job completed: %d/%d URLs processed (%d failed)",
						jobForCompletion.Progress.CompletedURLs, jobForCompletion.Progress.TotalURLs, jobForCompletion.Progress.FailedURLs)); err != nil {
					c.logger.Warn().Err(err).Msg("Failed to log job completion event")
				}

				// Persist completion status
				if err := c.deps.JobStorage.SaveJob(ctx, jobForCompletion); err != nil {
					c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to persist job completion status")
				}
			}
		}
	}

	// Log job completion
	if err := c.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Completed URL: %s (depth=%d)", msg.URL, msg.Depth)); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to log job completion event")
	}

	// Update parent job progress
	// NOTE: Progress tracking is now handled via queue-based architecture:
	// - Queue stats track pending/completed message counts
	// - JobStorage maintains persistent job state
	// - Progress can be queried via QueueManager.GetQueueStats()
	// - UI displays progress from queue stats + job storage status
	c.logger.Debug().
		Str("parent_id", msg.ParentID).
		Str("url", msg.URL).
		Int("depth", msg.Depth).
		Msg("URL processing completed - progress tracked via queue stats")

	c.logger.Info().
		Str("message_id", msg.ID).
		Str("url", msg.URL).
		Msg("Crawler URL job completed successfully")

	return nil
}

// Validate validates the crawler message
func (c *CrawlerJob) Validate(msg *queue.JobMessage) error {
	if msg.URL == "" {
		return fmt.Errorf("URL is required")
	}
	if msg.ParentID == "" {
		return fmt.Errorf("parent_id is required")
	}
	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}
	return nil
}

// GetType returns the job type
func (c *CrawlerJob) GetType() string {
	return "crawler"
}

// VERIFICATION COMMENT 3: Removed duplicate filtering logic (DRY principle)
// - isValidJiraURL() moved to crawler.IsValidJiraURL() in filters.go
// - isValidConfluenceURL() moved to crawler.IsValidConfluenceURL() in filters.go
// - shouldEnqueueURL() replaced with crawler.LinkFilter.FilterURL() in filters.go
// All filtering logic now consolidated in shared LinkFilter helper
