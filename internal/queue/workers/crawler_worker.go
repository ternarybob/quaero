// -----------------------------------------------------------------------
// Crawler Worker - Unified worker implementing both DefinitionWorker and JobWorker
// - DefinitionWorker: Creates parent crawl jobs via crawler service
// - JobWorker: Processes individual crawler jobs with ChromeDP rendering and content processing
// -----------------------------------------------------------------------

package workers

import (
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerWorker processes crawler jobs and implements both StepWorker and JobWorker interfaces.
// - StepWorker: Creates parent crawl jobs via crawler service
// - JobWorker: Processes individual crawler jobs with ChromeDP rendering and content processing
type CrawlerWorker struct {
	crawlerService  *crawler.Service
	jobMgr          *queue.Manager
	queueMgr        interfaces.QueueManager
	documentStorage interfaces.DocumentStorage
	authStorage     interfaces.AuthStorage
	jobDefStorage   interfaces.JobDefinitionStorage
	logger          arbor.ILogger
	eventService    interfaces.EventService

	// Content processing components
	contentProcessor *crawler.ContentProcessor
}

// Compile-time assertions: CrawlerWorker implements both interfaces
var _ interfaces.DefinitionWorker = (*CrawlerWorker)(nil)
var _ interfaces.JobWorker = (*CrawlerWorker)(nil)

// NewCrawlerWorker creates a new crawler worker that implements both DefinitionWorker and JobWorker interfaces
func NewCrawlerWorker(
	crawlerService *crawler.Service,
	jobMgr *queue.Manager,
	queueMgr interfaces.QueueManager,
	documentStorage interfaces.DocumentStorage,
	authStorage interfaces.AuthStorage,
	jobDefStorage interfaces.JobDefinitionStorage,
	logger arbor.ILogger,
	eventService interfaces.EventService,
) *CrawlerWorker {
	return &CrawlerWorker{
		crawlerService:   crawlerService,
		jobMgr:           jobMgr,
		queueMgr:         queueMgr,
		documentStorage:  documentStorage,
		authStorage:      authStorage,
		jobDefStorage:    jobDefStorage,
		logger:           logger,
		eventService:     eventService,
		contentProcessor: crawler.NewContentProcessor(logger),
	}
}

// ============================================================================
// INTERFACE METHODS
// ============================================================================

// GetWorkerType returns "crawler_url" - the job type this worker handles
func (w *CrawlerWorker) GetWorkerType() string {
	return "crawler_url"
}

// Validate validates that the queue job is compatible with this worker
func (w *CrawlerWorker) Validate(job *models.QueueJob) error {
	if job.Type != "crawler_url" {
		return fmt.Errorf("invalid job type: expected %s, got %s", "crawler_url", job.Type)
	}

	// Validate required config fields
	if _, ok := job.GetConfigString("seed_url"); !ok {
		return fmt.Errorf("missing required config field: seed_url")
	}

	if _, ok := job.GetConfigString("source_type"); !ok {
		return fmt.Errorf("missing required config field: source_type")
	}

	if _, ok := job.GetConfigString("entity_type"); !ok {
		return fmt.Errorf("missing required config field: entity_type")
	}

	// Validate crawl_config exists
	if _, ok := job.Config["crawl_config"]; !ok {
		return fmt.Errorf("missing required config field: crawl_config")
	}

	return nil
}

// Execute executes a crawler job with full workflow:
// 1. ChromeDP page rendering and JavaScript execution
// 2. Content extraction and markdown conversion
// 3. Document storage with comprehensive metadata
// 4. Link discovery and filtering
// 5. Child job spawning for discovered links (respecting depth limits)
func (w *CrawlerWorker) Execute(ctx context.Context, job *models.QueueJob) error {
	w.logger.Info().
		Str("job_id", job.ID).
		Str("job_type", job.Type).
		Msg("TRACE: CrawlerWorker.Execute called")

	// Create job-specific logger using parent context for log aggregation
	// All children log under the root parent ID for unified log viewing
	parentID := job.GetParentID()
	if parentID == "" {
		// This is a root job (shouldn't happen for crawler_url type, but handle gracefully)
		parentID = job.ID
	}
	jobLogger := w.logger.WithCorrelationId(parentID)

	// Extract configuration
	seedURL, _ := job.GetConfigString("seed_url")
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	// Extract crawl config from job config
	crawlConfig, err := w.extractCrawlConfig(job.Config)
	if err != nil {
		jobLogger.Error().Err(err).Msg("Failed to extract crawl config")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Failed: %s - invalid config", seedURL))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid crawl config: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to extract crawl config: %w", err)
	}

	// Update job status to running
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Record start time for duration tracking
	jobStartTime := time.Now()

	// Publish JOB START event for real-time UI display
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Started: %s", seedURL))

	// Publish initial progress update
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Acquiring browser from pool", seedURL)

	jobLogger.Trace().Msg("About to create browser instance")

	// Step 1: Create a fresh ChromeDP browser instance for this request
	// Using NON-HEADLESS mode with stealth options to avoid bot detection
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Creating browser instance", seedURL)

	jobLogger.Trace().Msg("Published progress update for browser creation")

	// Non-headless with stealth settings - NO headless flag to show visible browser window
	allocatorOpts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		// Force visible window
		chromedp.Flag("start-maximized", true),
		// Stealth options to avoid bot detection
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		// Realistic user agent
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	}

	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(
		context.Background(),
		allocatorOpts...,
	)
	defer allocatorCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocatorCtx)
	defer browserCancel()

	jobLogger.Trace().Msg("Created fresh browser instance (non-headless)")

	// Step 1.5: Load and inject authentication cookies into browser
	if err := w.injectAuthCookies(ctx, browserCtx, parentID, seedURL, jobLogger); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to inject authentication cookies - continuing without authentication")
		w.jobMgr.AddJobLog(ctx, job.ID, "warn", fmt.Sprintf("Failed to inject authentication cookies: %v", err))
	}

	// Step 2: Navigate to URL and render JavaScript
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Rendering page with JavaScript", seedURL)
	renderStartTime := time.Now()
	htmlContent, statusCode, err := w.renderPageWithChromeDp(ctx, browserCtx, seedURL, jobLogger)
	if err != nil {
		jobDuration := time.Since(jobStartTime)
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to render page with ChromeDP")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Failed: %s - render error (%v)", seedURL, jobDuration.Round(time.Millisecond)))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Page rendering failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to render page: %w", err)
	}
	renderTime := time.Since(renderStartTime)

	w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Rendered page (status: %d, size: %d bytes, time: %v)", statusCode, len(htmlContent), renderTime))

	// Step 3: Process HTML content and convert to markdown
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Processing HTML content and converting to markdown", seedURL)
	processedContent, err := w.contentProcessor.ProcessHTML(htmlContent, seedURL)
	if err != nil {
		jobDuration := time.Since(jobStartTime)
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to process HTML content")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Failed: %s - content processing error (%v)", seedURL, jobDuration.Round(time.Millisecond)))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Content processing failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to process content: %w", err)
	}

	w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Processed content: '%s' (%d bytes, %d links)", processedContent.Title, processedContent.ContentSize, len(processedContent.Links)))

	// Step 4: Create crawled document with comprehensive metadata
	parentJobID := job.GetParentID()
	crawledDoc := crawler.NewCrawledDocument(job.ID, parentJobID, seedURL, processedContent, crawlConfig.Tags)

	// Set crawler-specific metadata
	crawledDoc.SetCrawlerMetadata(
		job.Depth,
		job.ID, // This job discovered the URL (for seed URLs, it's self-discovered)
		statusCode,
		renderTime,
		len(processedContent.Links), // links_found
		0,                           // links_filtered (will be updated after filtering)
		0,                           // links_followed (will be updated after spawning)
		0,                           // links_skipped (will be updated after filtering)
	)

	// Step 5: Store document with metadata
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Saving document to storage", seedURL)
	docPersister := crawler.NewDocumentPersister(w.documentStorage, w.eventService, jobLogger)
	if err := docPersister.SaveCrawledDocument(crawledDoc); err != nil {
		jobDuration := time.Since(jobStartTime)
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to save crawled document")
		w.jobMgr.AddJobLog(ctx, job.ID, "error", fmt.Sprintf("Failed: %s - storage error (%v)", seedURL, jobDuration.Round(time.Millisecond)))
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Document storage failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Note: Per-document success logging removed to reduce log volume

	// Add job log for successful document storage (context auto-resolved, published to WebSocket)
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Document saved: %s (%d bytes) - %s",
		crawledDoc.Title, crawledDoc.ContentSize, seedURL))

	// Step 6: Link discovery and filtering with child job spawning
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Discovering and filtering links", seedURL)
	linkStats := &crawler.LinkProcessingResult{
		Found:    len(processedContent.Links),
		Filtered: 0,
		Followed: 0,
		Skipped:  0,
	}

	if len(processedContent.Links) > 0 && crawlConfig.FollowLinks {
		// Create link extractor for filtering
		linkExtractor := crawler.NewLinkExtractor(jobLogger)

		// Filter links using include/exclude patterns
		filterResult := linkExtractor.FilterLinks(processedContent.Links, crawlConfig.IncludePatterns, crawlConfig.ExcludePatterns)

		linkStats.Filtered = filterResult.Filtered

		// Note: Per-URL link filtering logging removed to reduce log volume

		w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Links: %d found, %d filtered, %d excluded", filterResult.Found, filterResult.Filtered, filterResult.Excluded))

		// Check depth limits for child job spawning
		if job.Depth < crawlConfig.MaxDepth && len(filterResult.FilteredLinks) > 0 {
			w.publishCrawlerProgressUpdate(ctx, job, "running", "Spawning child jobs for discovered links", seedURL)

			// Get GLOBAL child count under the step/parent job for max_pages enforcement
			// This ensures max_pages is a GLOBAL limit across the entire crawl, not per-URL
			globalChildCount := 0
			skipSpawning := false

			// Debug: Log the max_pages value being used (visible in job logs)
			w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("DEBUG: max_pages=%d, filtered=%d, depth=%d", crawlConfig.MaxPages, len(filterResult.FilteredLinks), job.Depth))

			if crawlConfig.MaxPages > 0 {
				parentID := job.GetParentID()
				if parentID != "" {
					if stats, err := w.jobMgr.GetJobChildStats(ctx, []string{parentID}); err == nil {
						if s := stats[parentID]; s != nil {
							globalChildCount = s.ChildCount
						}
					}
				}

				jobLogger.Debug().
					Int("max_pages", crawlConfig.MaxPages).
					Int("global_children", globalChildCount).
					Str("parent_id", parentID).
					Msg("Global child count check")

				// Check if we've already hit the global limit
				if globalChildCount >= crawlConfig.MaxPages {
					linkStats.Skipped = len(filterResult.FilteredLinks)
					jobLogger.Debug().
						Int("max_pages", crawlConfig.MaxPages).
						Int("global_children", globalChildCount).
						Int("skipped", linkStats.Skipped).
						Msg("Global max pages limit reached, skipping all new links")
					w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Global max pages limit (%d) reached with %d children, skipping %d links", crawlConfig.MaxPages, globalChildCount, linkStats.Skipped))
					skipSpawning = true
				}
			}

			// Spawn child jobs for filtered links (unless global limit already reached)
			childJobsSpawned := 0
			if !skipSpawning {
				for i, link := range filterResult.FilteredLinks {
					// Respect GLOBAL max pages limit (globalChildCount + childJobsSpawned)
					if crawlConfig.MaxPages > 0 && (globalChildCount+childJobsSpawned) >= crawlConfig.MaxPages {
						linkStats.Skipped = len(filterResult.FilteredLinks) - childJobsSpawned
						jobLogger.Debug().
							Int("max_pages", crawlConfig.MaxPages).
							Int("global_children", globalChildCount).
							Int("spawned_this_job", childJobsSpawned).
							Int("skipped", linkStats.Skipped).
							Msg("Reached global max pages limit, skipping remaining links")
						w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Reached global max pages limit (%d), skipping %d remaining links", crawlConfig.MaxPages, linkStats.Skipped))
						break
					}

					if err := w.spawnChildJob(ctx, job, link, crawlConfig, sourceType, entityType, i, jobLogger); err != nil {
						jobLogger.Warn().
							Err(err).
							Str("child_url", link).
							Msg("Failed to spawn child job for discovered link")
						w.jobMgr.AddJobLog(ctx, job.ID, "warn", fmt.Sprintf("Failed to spawn child job for link: %s", link))
						continue
					}
					childJobsSpawned++
				}

				linkStats.Followed = childJobsSpawned

				// Note: Per-URL child spawn logging removed to reduce log volume

				w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Spawned %d child jobs", childJobsSpawned))
			}
		} else if job.Depth >= crawlConfig.MaxDepth {
			// All filtered links are skipped due to depth limit
			linkStats.Skipped = filterResult.Filtered
			jobLogger.Debug().
				Int("current_depth", job.Depth).
				Int("max_depth", crawlConfig.MaxDepth).
				Int("links_skipped", linkStats.Skipped).
				Msg("Reached maximum depth, skipping all discovered links")

			w.jobMgr.AddJobLog(ctx, job.ID, "debug", fmt.Sprintf("Reached maximum depth (%d), skipping %d discovered links", crawlConfig.MaxDepth, linkStats.Skipped))
		}

		// Update crawler metadata with final link statistics
		crawledDoc.CrawlerMetadata.LinksFiltered = linkStats.Filtered
		crawledDoc.CrawlerMetadata.LinksFollowed = linkStats.Followed
		crawledDoc.CrawlerMetadata.LinksSkipped = linkStats.Skipped

		// Re-save document with updated link statistics
		if err := docPersister.SaveCrawledDocument(crawledDoc); err != nil {
			jobLogger.Warn().Err(err).Msg("Failed to update document with link statistics")
		}
	} else if !crawlConfig.FollowLinks {
		jobLogger.Debug().
			Int("links_found", len(processedContent.Links)).
			Msg("Link following disabled, skipping child job spawning")
	}

	// Log comprehensive link processing results (context auto-resolved, published to WebSocket)
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Links found: %d | filtered: %d | followed: %d | skipped: %d",
		linkStats.Found, linkStats.Filtered, linkStats.Followed, linkStats.Skipped))

	// Update job status to completed
	w.publishCrawlerProgressUpdate(ctx, job, "completed", "Job completed successfully", seedURL)
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Calculate total job duration
	jobDuration := time.Since(jobStartTime)

	// Publish JOB END event for real-time UI display (success)
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Completed: %s (%v)", seedURL, jobDuration.Round(time.Millisecond)))

	return nil
}

// ============================================================================
// CONFIGURATION AND RENDERING HELPERS
// ============================================================================

// extractCrawlConfig extracts CrawlConfig from Job.Config map
// Config can be stored in two formats:
// 1. Nested: config["crawl_config"] = CrawlConfig{...} (from spawnChildJob)
// 2. Flat: config["max_depth"], config["max_pages"], etc. (from StartCrawl seed jobs)
func (w *CrawlerWorker) extractCrawlConfig(config map[string]interface{}) (*models.CrawlConfig, error) {
	// First, check if config has nested crawl_config
	crawlConfigRaw, hasNestedConfig := config["crawl_config"]

	// Determine which map to extract from
	var configMap map[string]interface{}
	if hasNestedConfig {
		// Try direct type assertion for nested config
		if crawlConfig, ok := crawlConfigRaw.(*models.CrawlConfig); ok {
			return crawlConfig, nil
		}
		// Try non-pointer type assertion
		if crawlConfig, ok := crawlConfigRaw.(models.CrawlConfig); ok {
			return &crawlConfig, nil
		}
		// Try map[string]interface{} for nested config
		if nestedMap, ok := crawlConfigRaw.(map[string]interface{}); ok {
			configMap = nestedMap
		} else {
			// Fallback to flat config if nested config is not a valid type
			configMap = config
		}
	} else {
		// Use flat config (fields are at top level)
		configMap = config
	}

	// Convert map to CrawlConfig struct
	crawlConfig := &models.CrawlConfig{}

	// Extract max_depth
	if maxDepth, ok := configMap["max_depth"].(float64); ok {
		crawlConfig.MaxDepth = int(maxDepth)
	} else if maxDepth, ok := configMap["max_depth"].(int); ok {
		crawlConfig.MaxDepth = maxDepth
	} else if maxDepth, ok := configMap["max_depth"].(int64); ok {
		crawlConfig.MaxDepth = int(maxDepth)
	}

	// Extract max_pages
	if maxPages, ok := configMap["max_pages"].(float64); ok {
		crawlConfig.MaxPages = int(maxPages)
	} else if maxPages, ok := configMap["max_pages"].(int); ok {
		crawlConfig.MaxPages = maxPages
	} else if maxPages, ok := configMap["max_pages"].(int64); ok {
		crawlConfig.MaxPages = int(maxPages)
	}

	// Extract concurrency
	if concurrency, ok := configMap["concurrency"].(float64); ok {
		crawlConfig.Concurrency = int(concurrency)
	} else if concurrency, ok := configMap["concurrency"].(int); ok {
		crawlConfig.Concurrency = concurrency
	} else if concurrency, ok := configMap["concurrency"].(int64); ok {
		crawlConfig.Concurrency = int(concurrency)
	}

	// Extract follow_links
	if followLinks, ok := configMap["follow_links"].(bool); ok {
		crawlConfig.FollowLinks = followLinks
	}

	// Extract include/exclude patterns
	if includePatterns, ok := configMap["include_patterns"].([]interface{}); ok {
		for _, pattern := range includePatterns {
			if patternStr, ok := pattern.(string); ok {
				crawlConfig.IncludePatterns = append(crawlConfig.IncludePatterns, patternStr)
			}
		}
	} else if includePatterns, ok := configMap["include_patterns"].([]string); ok {
		crawlConfig.IncludePatterns = includePatterns
	}

	if excludePatterns, ok := configMap["exclude_patterns"].([]interface{}); ok {
		for _, pattern := range excludePatterns {
			if patternStr, ok := pattern.(string); ok {
				crawlConfig.ExcludePatterns = append(crawlConfig.ExcludePatterns, patternStr)
			}
		}
	} else if excludePatterns, ok := configMap["exclude_patterns"].([]string); ok {
		crawlConfig.ExcludePatterns = excludePatterns
	}

	return crawlConfig, nil
}

// renderPageWithChromeDp renders a page using ChromeDP and returns HTML content and status code
func (w *CrawlerWorker) renderPageWithChromeDp(ctx context.Context, browserCtx context.Context, url string, logger arbor.ILogger) (string, int, error) {
	var htmlContent string
	var statusCode int64 = 200 // Default status code

	// Check if browser context is already cancelled
	if err := browserCtx.Err(); err != nil {
		logger.Error().Err(err).Str("url", url).Msg("Browser context already cancelled before navigation")
		return "", 0, fmt.Errorf("browser context cancelled: %w", err)
	}

	// ===== ENABLE NETWORK DOMAIN FOR COOKIE OPERATIONS =====
	logger.Trace().Msg("Enabling ChromeDP network domain for cookie operations")
	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		logger.Error().Err(err).Msg("Failed to enable network domain")
		return "", 0, fmt.Errorf("failed to enable network domain: %w", err)
	}
	logger.Trace().Msg("Network domain enabled successfully")

	// Enable log domain for capturing browser console messages
	logger.Trace().Msg("Enabling ChromeDP log domain for browser console messages")
	if err := chromedp.Run(browserCtx, log.Enable()); err != nil {
		logger.Warn().Err(err).Msg("Failed to enable log domain")
	} else {
		logger.Trace().Msg("Log domain enabled successfully")
	}
	// ===== END NETWORK DOMAIN ENABLEMENT =====

	// ===== PHASE 3: COOKIE MONITORING BEFORE NAVIGATION =====
	logger.Trace().
		Str("url", url).
		Msg("Checking cookies before navigation")

	var cookiesBeforeNav []*network.Cookie
	err := chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().WithURLs([]string{url}).Do(ctx)
			if err != nil {
				return err
			}
			cookiesBeforeNav = cookies
			return nil
		}),
	)

	if err != nil {
		logger.Trace().
			Err(err).
			Str("url", url).
			Msg("Failed to read cookies before navigation")
	} else {
		logger.Trace().
			Int("cookie_count", len(cookiesBeforeNav)).
			Str("url", url).
			Msg("Cookies applicable to URL before navigation")

		if len(cookiesBeforeNav) == 0 {
			logger.Trace().
				Str("url", url).
				Msg("No cookies found for URL - navigating without authentication")
		} else {
			// Parse target URL for domain comparison
			targetURLParsed, parseErr := neturl.Parse(url)
			if parseErr != nil {
				logger.Trace().Err(parseErr).Msg("Failed to parse target URL for domain analysis")
			}

			// Log each cookie with domain matching analysis
			for i, cookie := range cookiesBeforeNav {
				// Perform domain matching analysis
				var matchType string
				isMatch := false
				if targetURLParsed != nil && targetURLParsed.Host != "" {
					cookieDomain := cookie.Domain
					normalizedCookieDomain := strings.TrimPrefix(cookieDomain, ".")
					targetHost := targetURLParsed.Host

					if normalizedCookieDomain == targetHost {
						matchType = "exact_match"
						isMatch = true
					} else if strings.HasSuffix(targetHost, "."+normalizedCookieDomain) {
						matchType = "parent_domain_match"
						isMatch = true
					} else if strings.HasSuffix(normalizedCookieDomain, "."+targetHost) {
						matchType = "subdomain_of_target"
						isMatch = false
					} else {
						matchType = "domain_mismatch"
						isMatch = false
					}

					logger.Trace().
						Int("cookie_index", i).
						Str("cookie_name", cookie.Name).
						Str("cookie_domain", cookie.Domain).
						Str("normalized_cookie_domain", normalizedCookieDomain).
						Str("target_domain", targetHost).
						Str("cookie_path", cookie.Path).
						Str("match_type", matchType).
						Bool("will_be_sent", isMatch).
						Bool("secure", cookie.Secure).
						Bool("http_only", cookie.HTTPOnly).
						Str("same_site", string(cookie.SameSite)).
						Msg("Cookie domain analysis before navigation")

					if !isMatch {
						logger.Trace().
							Str("cookie_name", cookie.Name).
							Str("cookie_domain", cookie.Domain).
							Str("target_domain", targetHost).
							Msg("Cookie domain mismatch - cookie may not be sent")
					}
				} else {
					logger.Trace().
						Int("cookie_index", i).
						Str("cookie_name", cookie.Name).
						Str("cookie_domain", cookie.Domain).
						Str("cookie_path", cookie.Path).
						Bool("secure", cookie.Secure).
						Bool("http_only", cookie.HTTPOnly).
						Str("same_site", string(cookie.SameSite)).
						Msg("Cookie will be sent with navigation")
				}
			}
		}
	}
	// ===== END PHASE 3 PART 1 =====

	// ===== PHASE 3 PART 1.5: EVENT LISTENERS FOR DIAGNOSTICS =====
	// Subscribe to browser console log events for cookie-related messages
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch evTyped := ev.(type) {
		case *log.EventEntryAdded:
			// Log browser console messages that mention cookies
			if strings.Contains(strings.ToLower(evTyped.Entry.Text), "cookie") {
				logger.Trace().
					Str("source", evTyped.Entry.Source.String()).
					Str("level", evTyped.Entry.Level.String()).
					Str("message", evTyped.Entry.Text).
					Msg("Browser console message about cookies")
			}

		case *network.EventRequestWillBeSent:
			// Log the request and any Cookie header sent
			cookieHeader := ""
			if evTyped.Request.Headers != nil {
				if cookie, exists := evTyped.Request.Headers["Cookie"]; exists {
					if cookieStr, ok := cookie.(string); ok {
						cookieHeader = cookieStr
					}
				}
			}

			if cookieHeader != "" {
				logger.Trace().
					Str("request_url", evTyped.Request.URL).
					Str("cookie_header", cookieHeader).
					Msg("Cookie header sent with request")
			} else {
				logger.Trace().
					Str("request_url", evTyped.Request.URL).
					Msg("No Cookie header in request")
			}

		case *network.EventLoadingFailed:
			// Log network failures that might indicate cookie issues
			logger.Trace().
				Str("request_url", evTyped.RequestID.String()).
				Str("error_text", evTyped.ErrorText).
				Str("type", evTyped.Type.String()).
				Msg("Network request failed (possible cookie issue)")

		case *network.EventResponseReceivedExtraInfo:
			// Log Set-Cookie headers in responses
			if evTyped.Headers != nil {
				if setCookie, exists := evTyped.Headers["Set-Cookie"]; exists {
					logger.Trace().
						Str("set_cookie_header", fmt.Sprintf("%v", setCookie)).
						Msg("Server sent Set-Cookie header")
				}
			}
		}
	})
	// ===== END PHASE 3 PART 1.5 =====

	// Inject stealth JavaScript to avoid bot detection BEFORE navigation
	stealthJS := `
		// Override navigator.webdriver to hide automation
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined, configurable: true });
		// Override navigator.plugins to appear like a real browser
		Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5], configurable: true });
		// Override navigator.languages
		Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'], configurable: true });
		// Override chrome.runtime to hide automation
		if (!window.chrome) { window.chrome = {}; }
		window.chrome.runtime = {};
		// Override permissions query
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);
		// Set realistic screen dimensions
		Object.defineProperty(screen, 'width', { get: () => 1920 });
		Object.defineProperty(screen, 'height', { get: () => 1080 });
		Object.defineProperty(screen, 'availWidth', { get: () => 1920 });
		Object.defineProperty(screen, 'availHeight', { get: () => 1040 });
		Object.defineProperty(screen, 'colorDepth', { get: () => 24 });
		Object.defineProperty(screen, 'pixelDepth', { get: () => 24 });
	`

	// Navigate to URL and wait for JavaScript rendering
	// Use the browserCtx (ChromeDP context) for ChromeDP operations
	err = chromedp.Run(browserCtx,
		// Inject stealth script before any navigation
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(stealthJS).Do(ctx)
			return err
		}),
		// Set viewport to realistic size
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.Sleep(2*time.Second), // Wait for JavaScript to render
		chromedp.OuterHTML("html", &htmlContent),
		// Try to get status code from performance API - just use default 200 if not available
		chromedp.Evaluate(`window.performance?.getEntriesByType?.('navigation')?.[0]?.responseStatus || 200`, &statusCode),
	)

	if err != nil {
		// Check if context was cancelled during operation
		if browserCtx.Err() != nil {
			logger.Error().Err(browserCtx.Err()).Str("url", url).Msg("Browser context cancelled during navigation")
		}
		logger.Error().Err(err).Str("url", url).Msg("ChromeDP navigation failed")
		return "", 0, fmt.Errorf("chromedp navigation failed: %w", err)
	}

	if htmlContent == "" {
		logger.Warn().Str("url", url).Msg("ChromeDP returned empty HTML content")
		return "", int(statusCode), fmt.Errorf("empty HTML content returned")
	}

	// ===== PHASE 3 PART 2: COOKIE MONITORING AFTER NAVIGATION =====
	logger.Trace().
		Str("url", url).
		Msg("Checking cookies after navigation")

	var cookiesAfterNav []*network.Cookie
	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().WithURLs([]string{url}).Do(ctx)
			if err != nil {
				return err
			}
			cookiesAfterNav = cookies
			return nil
		}),
	)

	if err != nil {
		logger.Trace().
			Err(err).
			Str("url", url).
			Msg("Failed to read cookies after navigation")
	} else {
		logger.Trace().
			Int("cookie_count", len(cookiesAfterNav)).
			Str("url", url).
			Msg("Cookies after navigation")

		// Compare before/after cookie counts
		if len(cookiesBeforeNav) > 0 && len(cookiesAfterNav) < len(cookiesBeforeNav) {
			logger.Warn().
				Int("cookies_before", len(cookiesBeforeNav)).
				Int("cookies_after", len(cookiesAfterNav)).
				Int("cookies_lost", len(cookiesBeforeNav)-len(cookiesAfterNav)).
				Msg("Cookies were cleared during navigation")
		} else if len(cookiesAfterNav) > len(cookiesBeforeNav) {
			logger.Trace().
				Int("cookies_before", len(cookiesBeforeNav)).
				Int("cookies_after", len(cookiesAfterNav)).
				Int("cookies_gained", len(cookiesAfterNav)-len(cookiesBeforeNav)).
				Msg("New cookies set during navigation")
		} else if len(cookiesBeforeNav) > 0 {
			logger.Trace().
				Int("cookie_count", len(cookiesAfterNav)).
				Msg("Cookies persisted through navigation")
		}
	}
	// ===== END PHASE 3 =====

	logger.Trace().
		Str("url", url).
		Int("status_code", int(statusCode)).
		Int("html_length", len(htmlContent)).
		Msg("ChromeDP page rendering completed")

	return htmlContent, int(statusCode), nil
}

// ============================================================================
// AUTHENTICATION HELPERS
// ============================================================================

// injectAuthCookies loads authentication credentials from storage and injects cookies into ChromeDP browser
func (w *CrawlerWorker) injectAuthCookies(ctx context.Context, browserCtx context.Context, parentJobID, targetURL string, logger arbor.ILogger) error {
	logger.Trace().
		Str("parent_job_id", parentJobID).
		Str("target_url", targetURL).
		Msg("Cookie injection process initiated")

	// Check if authStorage is available
	if w.authStorage == nil {
		logger.Trace().Msg("Auth storage not configured, skipping cookie injection")
		return nil
	}
	logger.Trace().Msg("Auth storage is configured")

	// Get parent job from database to retrieve AuthID from job metadata
	logger.Trace().Str("parent_job_id", parentJobID).Msg("Fetching parent job from database")
	parentJobInterface, err := w.jobMgr.GetJob(ctx, parentJobID)
	if err != nil {
		logger.Error().Err(err).Str("parent_job_id", parentJobID).Msg("Failed to get parent job for auth lookup")
		return fmt.Errorf("failed to get parent job: %w", err)
	}
	logger.Trace().Msg("Parent job retrieved from database")

	// Extract QueueJob from QueueJobState
	var authID string
	var queueJob *models.QueueJob

	// Type assert to QueueJobState and extract QueueJob
	if jobState, ok := parentJobInterface.(*models.QueueJobState); ok {
		logger.Trace().Msg("Parent job is QueueJobState type (extracting QueueJob)")
		queueJob = jobState.ToQueueJob()
	} else if qj, ok := parentJobInterface.(*models.QueueJob); ok {
		logger.Trace().Msg("Parent job is QueueJob type")
		queueJob = qj
	} else {
		logger.Error().
			Str("actual_type", fmt.Sprintf("%T", parentJobInterface)).
			Msg("Parent job is neither QueueJobState nor QueueJob type")
		return nil
	}

	if queueJob == nil {
		logger.Error().Msg("QueueJob is nil after extraction")
		return nil
	}

	logger.Trace().
		Int("metadata_count", len(queueJob.Metadata)).
		Msg("QueueJob extracted successfully")

	// Log all metadata keys for debugging
	metadataKeys := make([]string, 0, len(queueJob.Metadata))
	for k := range queueJob.Metadata {
		metadataKeys = append(metadataKeys, k)
	}
	logger.Trace().
		Strs("metadata_keys", metadataKeys).
		Msg("Parent job metadata keys")

	// Check metadata for auth_id
	if authIDVal, exists := queueJob.Metadata["auth_id"]; exists {
		if authIDStr, ok := authIDVal.(string); ok && authIDStr != "" {
			authID = authIDStr
			logger.Trace().
				Str("auth_id", authID).
				Msg("Auth ID found in job metadata")
		} else {
			logger.Trace().
				Str("auth_id_value", fmt.Sprintf("%v", authIDVal)).
				Msg("auth_id exists but is not a valid string")
		}
	} else {
		logger.Trace().Msg("auth_id NOT found in job metadata")
	}

	// If not in metadata, try job_definition_id
	if authID == "" {
		logger.Trace().Msg("Trying job_definition_id fallback")
		if jobDefID, exists := queueJob.Metadata["job_definition_id"]; exists {
			if jobDefIDStr, ok := jobDefID.(string); ok && jobDefIDStr != "" {
				logger.Trace().
					Str("job_def_id", jobDefIDStr).
					Msg("Found job_definition_id, fetching job definition")
				jobDef, err := w.jobDefStorage.GetJobDefinition(ctx, jobDefIDStr)
				if err != nil {
					logger.Error().Err(err).Str("job_def_id", jobDefIDStr).Msg("Failed to get job definition for auth lookup")
					return fmt.Errorf("failed to get job definition: %w", err)
				}
				if jobDef != nil && jobDef.AuthID != "" {
					authID = jobDef.AuthID
					logger.Trace().
						Str("auth_id", authID).
						Str("job_def_id", jobDefIDStr).
						Msg("Auth ID found from job definition")
				} else {
					logger.Trace().
						Str("job_def_id", jobDefIDStr).
						Msg("Job definition has no AuthID")
				}
			}
		} else {
			logger.Trace().Msg("job_definition_id NOT found in metadata")
		}
	}

	if authID == "" {
		logger.Trace().Msg("No auth_id found - skipping cookie injection")
		return nil
	}

	// Load authentication credentials from storage using AuthID
	logger.Trace().
		Str("auth_id", authID).
		Msg("Loading auth credentials from storage")
	authCreds, err := w.authStorage.GetCredentialsByID(ctx, authID)
	if err != nil {
		logger.Error().Err(err).Str("auth_id", authID).Msg("Failed to load auth credentials from storage")
		return fmt.Errorf("failed to load auth credentials: %w", err)
	}

	if authCreds == nil {
		logger.Error().Str("auth_id", authID).Msg("Auth credentials not found in storage")
		return fmt.Errorf("auth credentials not found for ID: %s", authID)
	}
	logger.Trace().
		Str("auth_id", authID).
		Str("site_domain", authCreds.SiteDomain).
		Msg("Auth credentials loaded successfully")

	// Unmarshal cookies from JSON
	logger.Trace().Msg("Unmarshaling cookies from JSON")
	var extensionCookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &extensionCookies); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal cookies from auth credentials")
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	if len(extensionCookies) == 0 {
		logger.Trace().Msg("No cookies found in auth credentials")
		return nil
	}

	logger.Trace().
		Int("cookie_count", len(extensionCookies)).
		Str("site_domain", authCreds.SiteDomain).
		Msg("Cookies loaded - preparing to inject into browser")

	// Parse target URL to get domain
	targetURLParsed, err := neturl.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}

	// ===== PHASE 1: PRE-INJECTION DOMAIN DIAGNOSTICS =====
	logger.Trace().
		Str("target_url", targetURL).
		Str("target_domain", targetURLParsed.Host).
		Str("target_scheme", targetURLParsed.Scheme).
		Msg("Target URL parsed for domain analysis")

	// Analyze each cookie's domain compatibility with target URL
	logger.Trace().Msg("Analyzing cookie domain compatibility with target URL")
	for i, c := range extensionCookies {
		cookieDomain := c.Domain
		if cookieDomain == "" {
			logger.Trace().
				Int("cookie_index", i).
				Str("cookie_name", c.Name).
				Msg("Cookie has no domain (will use target domain)")
			continue
		}

		// Normalize cookie domain (remove leading dot for comparison)
		normalizedCookieDomain := strings.TrimPrefix(cookieDomain, ".")
		targetHost := targetURLParsed.Host

		// Check domain matching logic
		var matchType string
		isMatch := false
		if normalizedCookieDomain == targetHost {
			matchType = "exact_match"
			isMatch = true
		} else if strings.HasSuffix(targetHost, "."+normalizedCookieDomain) {
			matchType = "parent_domain_match"
			isMatch = true
		} else if strings.HasSuffix(normalizedCookieDomain, "."+targetHost) {
			matchType = "subdomain_of_target"
			isMatch = false
		} else {
			matchType = "domain_mismatch"
			isMatch = false
		}

		logger.Trace().
			Int("cookie_index", i).
			Str("cookie_name", c.Name).
			Str("cookie_domain", cookieDomain).
			Str("normalized_cookie_domain", normalizedCookieDomain).
			Str("target_domain", targetHost).
			Str("match_type", matchType).
			Bool("will_be_sent", isMatch).
			Msg("Cookie domain analysis")

		if !isMatch {
			logger.Trace().
				Str("cookie_name", c.Name).
				Str("cookie_domain", cookieDomain).
				Str("target_domain", targetHost).
				Msg("Cookie domain mismatch - cookie may not be sent with requests")
		}

		// Check secure flag compatibility with scheme
		if c.Secure && targetURLParsed.Scheme != "https" {
			logger.Trace().
				Str("cookie_name", c.Name).
				Str("target_scheme", targetURLParsed.Scheme).
				Msg("Secure cookie will not be sent to non-HTTPS URL")
		}
	}
	// ===== END PHASE 1 =====

	// Convert extension cookies to ChromeDP network cookies
	logger.Trace().
		Int("cookie_count", len(extensionCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("Converting extension cookies to ChromeDP format")
	var chromeDPCookies []*network.CookieParam
	for i, c := range extensionCookies {
		// Calculate expiration timestamp
		var expires *cdp.TimeSinceEpoch
		if c.Expires > 0 {
			expiresTime := time.Unix(c.Expires, 0)
			// Only set expiration if it's in the future
			if expiresTime.After(time.Now()) {
				timestamp := cdp.TimeSinceEpoch(expiresTime)
				expires = &timestamp
			}
		}

		// Determine the domain to use for this cookie
		cookieDomain := c.Domain
		if cookieDomain == "" {
			cookieDomain = targetURLParsed.Host
		}
		// Keep leading dot for subdomain cookies (e.g., .atlassian.net)
		// ChromeDP network.SetCookie handles this correctly

		chromeDPCookie := &network.CookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   cookieDomain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			Expires:  expires,
		}

		// Set SameSite attribute if available
		if c.SameSite != "" {
			switch strings.ToLower(c.SameSite) {
			case "strict":
				chromeDPCookie.SameSite = network.CookieSameSiteStrict
			case "lax":
				chromeDPCookie.SameSite = network.CookieSameSiteLax
			case "none":
				chromeDPCookie.SameSite = network.CookieSameSiteNone
			}
		}

		chromeDPCookies = append(chromeDPCookies, chromeDPCookie)

		logger.Trace().
			Int("cookie_index", i).
			Str("name", c.Name).
			Str("domain", cookieDomain).
			Str("path", c.Path).
			Bool("secure", c.Secure).
			Bool("http_only", c.HTTPOnly).
			Msg("Prepared cookie for injection")
	}

	// ===== PHASE 2: NETWORK DOMAIN ENABLEMENT =====
	logger.Trace().Msg("Enabling ChromeDP network domain for cookie operations")
	err = chromedp.Run(browserCtx, network.Enable())
	if err != nil {
		logger.Error().Err(err).Msg("Failed to enable network domain")
		return fmt.Errorf("failed to enable network domain: %w", err)
	}
	logger.Trace().Msg("Network domain enabled successfully")
	// ===== END PHASE 2 PART 1 =====

	// Inject cookies into browser using ChromeDP
	logger.Trace().
		Int("cookie_count", len(chromeDPCookies)).
		Msg("Starting browser cookie injection via ChromeDP")

	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			successCount := 0
			failCount := 0

			// Set all cookies with URL parameter for proper domain association
			for _, cookie := range chromeDPCookies {
				if err := network.SetCookie(cookie.Name, cookie.Value).
					WithURL(targetURL).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSameSite(cookie.SameSite).
					WithExpires(cookie.Expires).
					Do(ctx); err != nil {
					failCount++
					logger.Error().
						Err(err).
						Str("cookie_name", cookie.Name).
						Str("domain", cookie.Domain).
						Str("path", cookie.Path).
						Msg("Failed to inject cookie into browser")
					// Continue with other cookies even if one fails
				} else {
					successCount++
					logger.Trace().
						Str("cookie_name", cookie.Name).
						Str("domain", cookie.Domain).
						Msg("Cookie injected successfully")
				}
			}

			logger.Trace().
				Int("success_count", successCount).
				Int("fail_count", failCount).
				Int("total_cookies", len(chromeDPCookies)).
				Msg("Cookie injection batch complete")

			return nil
		}),
	)

	if err != nil {
		logger.Error().
			Err(err).
			Str("target_url", targetURL).
			Int("cookies_attempted", len(chromeDPCookies)).
			Msg("ChromeDP failed to inject cookies into browser")
		return fmt.Errorf("failed to inject cookies: %w", err)
	}

	logger.Debug().
		Int("cookies_injected", len(chromeDPCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("Authentication cookies injected into browser")

	// ===== PHASE 2 PART 2: POST-INJECTION VERIFICATION =====
	logger.Trace().
		Str("target_url", targetURL).
		Msg("Verifying cookies after injection using network.GetCookies()")

	var verifiedCookies []*network.Cookie
	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err := network.GetCookies().WithURLs([]string{targetURL}).Do(ctx)
			if err != nil {
				return err
			}
			verifiedCookies = cookies
			return nil
		}),
	)

	if err != nil {
		logger.Error().
			Err(err).
			Str("target_url", targetURL).
			Msg("Failed to verify cookies after injection")
		// Don't return error - continue with warning
	} else {
		logger.Trace().
			Int("verified_cookie_count", len(verifiedCookies)).
			Int("injected_cookie_count", len(chromeDPCookies)).
			Msg("Cookie verification complete")

		// Log details of each verified cookie
		for i, cookie := range verifiedCookies {
			// Truncate value for security (show first 20 chars)
			valuePreview := cookie.Value
			if len(valuePreview) > 20 {
				valuePreview = valuePreview[:20] + "..."
			}

			expiryStr := "session"
			if cookie.Expires > 0 {
				expiryStr = time.Unix(int64(cookie.Expires), 0).Format(time.RFC3339)
			}

			logger.Trace().
				Int("cookie_index", i).
				Str("name", cookie.Name).
				Str("value_preview", valuePreview).
				Str("domain", cookie.Domain).
				Str("path", cookie.Path).
				Bool("secure", cookie.Secure).
				Bool("http_only", cookie.HTTPOnly).
				Str("same_site", string(cookie.SameSite)).
				Str("expires", expiryStr).
				Msg("Verified cookie details")
		}

		// Compare injected vs verified cookies
		injectedCookieNames := make(map[string]bool)
		for _, cookie := range chromeDPCookies {
			injectedCookieNames[cookie.Name] = true
		}

		verifiedCookieNames := make(map[string]bool)
		for _, cookie := range verifiedCookies {
			verifiedCookieNames[cookie.Name] = true
		}

		// Check for missing cookies (injected but not verified)
		missingCookies := []string{}
		for name := range injectedCookieNames {
			if !verifiedCookieNames[name] {
				missingCookies = append(missingCookies, name)
			}
		}

		// Check for unexpected cookies (verified but not injected)
		unexpectedCookies := []string{}
		for name := range verifiedCookieNames {
			if !injectedCookieNames[name] {
				unexpectedCookies = append(unexpectedCookies, name)
			}
		}

		// Log mismatches
		if len(missingCookies) > 0 {
			logger.Error().
				Strs("missing_cookies", missingCookies).
				Int("missing_count", len(missingCookies)).
				Msg("Cookies were injected but not verified (failed to persist)")
		}

		if len(unexpectedCookies) > 0 {
			logger.Trace().
				Strs("unexpected_cookies", unexpectedCookies).
				Int("unexpected_count", len(unexpectedCookies)).
				Msg("Cookies verified but not injected (pre-existing or set by page)")
		}

		// Final verdict
		if len(verifiedCookies) == len(chromeDPCookies) && len(missingCookies) == 0 {
			logger.Trace().
				Int("cookie_count", len(verifiedCookies)).
				Msg("All injected cookies verified successfully")
		} else {
			logger.Warn().
				Int("injected", len(chromeDPCookies)).
				Int("verified", len(verifiedCookies)).
				Int("missing", len(missingCookies)).
				Msg("Cookie injection/verification mismatch detected")
		}
	}
	// ===== END PHASE 2 =====

	// Log cookie injection using Job Manager's uniform logging
	w.jobMgr.AddJobLog(ctx, parentJobID, "info", fmt.Sprintf("Injected %d authentication cookies into browser (domain: %s)", len(chromeDPCookies), targetURLParsed.Host))

	return nil
}

// ============================================================================
// CHILD JOB MANAGEMENT
// ============================================================================

// spawnChildJob creates and enqueues a child job for a discovered link
func (w *CrawlerWorker) spawnChildJob(ctx context.Context, parentJob *models.QueueJob, childURL string, crawlConfig *models.CrawlConfig, sourceType, entityType string, linkIndex int, logger arbor.ILogger) error {
	// Create child job configuration
	childConfig := make(map[string]interface{})
	childConfig["crawl_config"] = crawlConfig
	childConfig["source_type"] = sourceType
	childConfig["entity_type"] = entityType
	childConfig["seed_url"] = childURL

	// Create child job metadata
	childMetadata := make(map[string]interface{})
	if parentJob.Metadata != nil {
		// Copy parent metadata (includes step_id, step_name, manager_id if present)
		for k, v := range parentJob.Metadata {
			childMetadata[k] = v
		}
	}
	childMetadata["discovered_by"] = parentJob.ID
	childMetadata["link_index"] = linkIndex

	// Ensure step context is propagated to child jobs for proper event aggregation
	// If not already in metadata, try to resolve from parent chain
	if _, hasStepID := childMetadata["step_id"]; !hasStepID {
		sc := w.extractStepContext(ctx, parentJob)
		if sc.stepID != "" {
			childMetadata["step_id"] = sc.stepID
		}
		if sc.stepName != "" {
			childMetadata["step_name"] = sc.stepName
		}
		if sc.managerID != "" {
			childMetadata["manager_id"] = sc.managerID
		}
	}

	// Create child queue job with incremented depth
	childJob := models.NewQueueJobChild(
		parentJob.GetParentID(), // All children reference the same root parent (flat hierarchy)
		models.JobTypeCrawlerURL,
		fmt.Sprintf("URL: %s", childURL),
		childConfig,
		childMetadata,
		parentJob.Depth+1, // Increment depth for child
	)

	// Validate child job
	if err := childJob.Validate(); err != nil {
		return fmt.Errorf("invalid child queue job: %w", err)
	}

	// Serialize job model to JSON for payload
	payloadBytes, err := childJob.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize child job model: %w", err)
	}

	// Create job record in database
	if err := w.jobMgr.CreateJobRecord(ctx, &queue.Job{
		ID:              childJob.ID,
		ParentID:        childJob.ParentID,
		Type:            childJob.Type,
		Name:            childJob.Name,
		Phase:           "execution",
		Status:          "pending",
		CreatedAt:       childJob.CreatedAt,
		ProgressCurrent: 0,
		ProgressTotal:   1, // Single URL to process
		Payload:         string(payloadBytes),
	}); err != nil {
		return fmt.Errorf("failed to create child job record: %w", err)
	}

	// Use the same payload for queue message
	jobBytes := payloadBytes

	// Enqueue child job
	queueMsg := queue.Message{
		JobID:   childJob.ID,
		Type:    childJob.Type,
		Payload: jobBytes,
	}

	if err := w.queueMgr.Enqueue(ctx, queueMsg); err != nil {
		return fmt.Errorf("failed to enqueue child job: %w", err)
	}

	logger.Trace().
		Str("parent_job_id", parentJob.ID).
		Str("child_job_id", childJob.ID).
		Str("child_url", childURL).
		Int("child_depth", childJob.Depth).
		Int("link_index", linkIndex).
		Msg("Child job spawned and enqueued for discovered link")

	// Log job spawn event
	w.jobMgr.AddJobLog(ctx, parentJob.ID, "debug", fmt.Sprintf("Spawned child job %s for URL: %s (depth: %d)", childJob.ID[:8], childURL, parentJob.Depth+1))

	return nil
}

// ============================================================================
// STEP CONTEXT HELPERS
// ============================================================================

// stepContext holds step-related information for event publishing
type stepContext struct {
	stepID    string
	stepName  string
	managerID string
}

// extractStepContext extracts step context from job metadata or parent chain
func (w *CrawlerWorker) extractStepContext(ctx context.Context, job *models.QueueJob) *stepContext {
	sc := &stepContext{}

	// Try to get step context from job metadata first
	if job.Metadata != nil {
		if stepID, ok := job.Metadata["step_id"].(string); ok {
			sc.stepID = stepID
		}
		if stepName, ok := job.Metadata["step_name"].(string); ok {
			sc.stepName = stepName
		}
		if managerID, ok := job.Metadata["manager_id"].(string); ok {
			sc.managerID = managerID
		}
	}

	// If step context not in metadata, try to resolve from parent chain
	if sc.stepName == "" {
		parentID := job.GetParentID()
		if parentID != "" {
			parentJob, err := w.jobMgr.GetJob(ctx, parentID)
			if err == nil {
				if parentState, ok := parentJob.(*models.QueueJobState); ok {
					// Check if parent is a step job
					if parentState.Type == "step" {
						sc.stepID = parentState.ID
						sc.stepName = parentState.Name
						// Get manager_id from step job's parent
						if parentState.ParentID != nil && *parentState.ParentID != "" {
							sc.managerID = *parentState.ParentID
						}
					} else if parentState.Metadata != nil {
						// Parent is not a step, but might have step context in metadata
						if stepID, ok := parentState.Metadata["step_id"].(string); ok {
							sc.stepID = stepID
						}
						if stepName, ok := parentState.Metadata["step_name"].(string); ok {
							sc.stepName = stepName
						}
						if managerID, ok := parentState.Metadata["manager_id"].(string); ok {
							sc.managerID = managerID
						}
					}
				}
			}
		}
	}

	return sc
}

// publishCrawlerProgressUpdate logs a crawler job progress update.
// Uses debug level to reduce UI noise while tracking activity.
func (w *CrawlerWorker) publishCrawlerProgressUpdate(ctx context.Context, job *models.QueueJob, status, activity, currentURL string) {
	message := fmt.Sprintf("[%s] %s", status, activity)
	if currentURL != "" {
		message = fmt.Sprintf("[%s] %s: %s", status, activity, currentURL)
	}
	w.jobMgr.AddJobLog(ctx, job.ID, "debug", message)
}

// ============================================================================
// DEFINITIONWORKER INTERFACE METHODS (for job definition step handling)
// ============================================================================

// GetType returns WorkerTypeCrawler for the DefinitionWorker interface
func (w *CrawlerWorker) GetType() models.WorkerType {
	return models.WorkerTypeCrawler
}

// CreateJobs creates a parent crawler job and triggers the crawler service to start crawling.
// The crawler service will create child jobs for each URL discovered.
// stepID is the ID of the step job - all jobs should have parent_id = stepID
func (w *CrawlerWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, stepID string) (string, error) {
	// Parse step config map into CrawlConfig struct
	stepConfig := step.Config
	if stepConfig == nil {
		stepConfig = make(map[string]interface{})
	}

	// Get manager_id from step job's parent_id for event aggregation
	// Step jobs have parent_id = manager_id in the 3-level hierarchy
	managerID := ""
	if stepJobInterface, err := w.jobMgr.GetJob(ctx, stepID); err == nil && stepJobInterface != nil {
		if stepJob, ok := stepJobInterface.(*models.QueueJobState); ok && stepJob != nil && stepJob.ParentID != nil {
			managerID = *stepJob.ParentID
		}
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
	crawlConfig := w.buildCrawlConfig(stepConfig)

	// Apply tags from job definition to all documents created by this crawl
	crawlConfig.Tags = jobDef.Tags

	// Set step context for job_log event aggregation in UI
	crawlConfig.StepName = step.Name
	crawlConfig.ManagerID = managerID

	// Build seed URLs - prioritize start_urls from step config, fallback to source type
	var seedURLs []string
	if startURLs, ok := stepConfig["start_urls"].([]interface{}); ok && len(startURLs) > 0 {
		for _, url := range startURLs {
			if urlStr, ok := url.(string); ok {
				seedURLs = append(seedURLs, urlStr)
			}
		}
		w.logger.Debug().
			Str("step_name", step.Name).
			Strs("start_urls", seedURLs).
			Msg("Using start_urls from job definition config")
	} else if startURLsStr, ok := stepConfig["start_urls"].([]string); ok && len(startURLsStr) > 0 {
		seedURLs = startURLsStr
		w.logger.Debug().
			Str("step_name", step.Name).
			Strs("start_urls", seedURLs).
			Msg("Using start_urls from job definition config")
	} else {
		seedURLs = w.buildSeedURLs(jobDef.BaseURL, jobDef.SourceType, entityType)
		w.logger.Debug().
			Str("step_name", step.Name).
			Str("source_type", jobDef.SourceType).
			Str("base_url", jobDef.BaseURL).
			Strs("generated_urls", seedURLs).
			Msg("Using generated URLs based on source type (no start_urls in config)")
	}

	// Default source_type to "web" for generic crawler jobs
	sourceType := jobDef.SourceType
	if sourceType == "" {
		sourceType = "web"
		w.logger.Debug().
			Str("step_name", step.Name).
			Msg("No source_type specified, defaulting to 'web' for generic web crawling")
	}

	w.logger.Debug().
		Str("step_name", step.Name).
		Str("source_type", sourceType).
		Str("base_url", jobDef.BaseURL).
		Str("entity_type", entityType).
		Int("seed_url_count", len(seedURLs)).
		Int("max_depth", crawlConfig.MaxDepth).
		Int("max_pages", crawlConfig.MaxPages).
		Msg("Creating parent crawler job")

	// Start crawl job with properly typed config
	jobID, err := w.crawlerService.StartCrawl(
		sourceType,
		entityType,
		seedURLs,
		crawlConfig,   // Pass CrawlConfig struct
		jobDef.AuthID, // sourceID - use auth_id as source identifier
		false,         // refreshSource
		nil,           // sourceConfigSnapshot
		nil,           // authSnapshot
		stepID,        // jobDefinitionID - link to step
	)

	if err != nil {
		return "", fmt.Errorf("failed to start crawl (source_type=%s, step=%s): %w", sourceType, step.Name, err)
	}

	w.logger.Debug().
		Str("step_name", step.Name).
		Str("job_id", jobID).
		Str("step_id", stepID).
		Msg("Parent crawler job created")

	return jobID, nil
}

// ReturnsChildJobs returns true since crawler creates child jobs for each URL
func (w *CrawlerWorker) ReturnsChildJobs() bool {
	return true
}

// ValidateConfig validates step configuration for crawler type (DefinitionWorker interface)
func (w *CrawlerWorker) ValidateConfig(step models.JobStep) error {
	// Crawler is agnostic - any configuration is valid
	// Validation happens during execution by the crawler service
	return nil
}

// buildCrawlConfig constructs a CrawlConfig struct from a config map
func (w *CrawlerWorker) buildCrawlConfig(configMap map[string]interface{}) crawler.CrawlConfig {
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
	// TOML parser uses int64, JSON uses float64, Go uses int
	if v, ok := configMap["max_depth"].(float64); ok {
		config.MaxDepth = int(v)
	} else if v, ok := configMap["max_depth"].(int); ok {
		config.MaxDepth = v
	} else if v, ok := configMap["max_depth"].(int64); ok {
		config.MaxDepth = int(v)
	}

	if v, ok := configMap["max_pages"].(float64); ok {
		config.MaxPages = int(v)
	} else if v, ok := configMap["max_pages"].(int); ok {
		config.MaxPages = v
	} else if v, ok := configMap["max_pages"].(int64); ok {
		config.MaxPages = int(v)
	}

	if v, ok := configMap["concurrency"].(float64); ok {
		config.Concurrency = int(v)
	} else if v, ok := configMap["concurrency"].(int); ok {
		config.Concurrency = v
	} else if v, ok := configMap["concurrency"].(int64); ok {
		config.Concurrency = int(v)
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
func (w *CrawlerWorker) buildSeedURLs(baseURL, sourceType, entityType string) []string {
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
