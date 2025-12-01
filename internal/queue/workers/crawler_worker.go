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
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
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
		w.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to extract crawl config: %v", err), map[string]interface{}{
			"url":        seedURL,
			"depth":      job.Depth,
			"child_id":   job.ID,
			"discovered": job.Metadata["discovered_by"],
		})
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid crawl config: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to extract crawl config: %w", err)
	}

	// Note: Per-URL job start logging removed to reduce log volume
	// Progress is tracked via real-time events instead

	// Publish real-time log for job start (under parent context)
	w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Starting URL: %s (depth: %d)", seedURL, job.Depth), map[string]interface{}{
		"url":          seedURL,
		"depth":        job.Depth,
		"max_depth":    crawlConfig.MaxDepth,
		"follow_links": crawlConfig.FollowLinks,
		"child_id":     job.ID,
		"discovered":   job.Metadata["discovered_by"],
	})

	// Update job status to running
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Starting crawl of URL: %s (depth: %d)", seedURL, job.Depth))

	// Publish initial progress update
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Acquiring browser from pool", seedURL)

	jobLogger.Trace().Msg("About to create browser instance")

	// Step 1: Create a fresh ChromeDP browser instance for this request
	// TEMPORARY: Bypassing pool to debug context cancellation issue
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Creating browser instance", seedURL)

	jobLogger.Trace().Msg("Published progress update for browser creation")

	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent("Quaero/1.0 (Web Crawler)"),
		)...,
	)
	defer allocatorCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocatorCtx)
	defer browserCancel()

	jobLogger.Trace().Msg("Created fresh browser instance")
	w.publishCrawlerJobLog(ctx, parentID, "debug", "Created fresh browser instance", map[string]interface{}{
		"url":      seedURL,
		"depth":    job.Depth,
		"child_id": job.ID,
	})

	// Step 1.5: Load and inject authentication cookies into browser
	if err := w.injectAuthCookies(ctx, browserCtx, parentID, seedURL, jobLogger); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to inject authentication cookies - continuing without authentication")
		w.publishCrawlerJobLog(ctx, parentID, "warn", fmt.Sprintf("Failed to inject authentication cookies: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
			"error":    err.Error(),
		})
	}

	// Step 2: Navigate to URL and render JavaScript
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Rendering page with JavaScript", seedURL)
	startTime := time.Now()
	htmlContent, statusCode, err := w.renderPageWithChromeDp(ctx, browserCtx, seedURL, jobLogger)
	if err != nil {
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to render page with ChromeDP")
		w.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to render page with ChromeDP: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
		})
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Page rendering failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to render page: %w", err)
	}
	renderTime := time.Since(startTime)

	// Note: Per-URL success logging removed to reduce log volume
	// Render stats are captured in real-time events instead

	w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Rendered page (status: %d, size: %d bytes, time: %v)", statusCode, len(htmlContent), renderTime), map[string]interface{}{
		"url":         seedURL,
		"depth":       job.Depth,
		"status_code": statusCode,
		"html_length": len(htmlContent),
		"render_time": renderTime.String(),
		"child_id":    job.ID,
	})

	// Step 3: Process HTML content and convert to markdown
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Processing HTML content and converting to markdown", seedURL)
	processedContent, err := w.contentProcessor.ProcessHTML(htmlContent, seedURL)
	if err != nil {
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to process HTML content")
		w.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to process HTML content: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
		})
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Content processing failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to process content: %w", err)
	}

	// Note: Per-URL success logging removed to reduce log volume
	// Content stats are captured in real-time events instead

	w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Processed content: '%s' (%d bytes, %d links)", processedContent.Title, processedContent.ContentSize, len(processedContent.Links)), map[string]interface{}{
		"url":          seedURL,
		"depth":        job.Depth,
		"title":        processedContent.Title,
		"content_size": processedContent.ContentSize,
		"links_found":  len(processedContent.Links),
		"process_time": processedContent.ProcessTime.String(),
		"child_id":     job.ID,
	})

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
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to save crawled document")
		w.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to save crawled document: %v", err), map[string]interface{}{
			"url":         seedURL,
			"depth":       job.Depth,
			"document_id": crawledDoc.ID,
			"child_id":    job.ID,
		})
		w.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Document storage failed: %v", err))
		w.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to save document: %w", err)
	}

	// Note: Per-document success logging removed to reduce log volume

	w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Document saved: %s (%d bytes)", crawledDoc.Title, crawledDoc.ContentSize), map[string]interface{}{
		"url":          seedURL,
		"depth":        job.Depth,
		"document_id":  crawledDoc.ID,
		"title":        crawledDoc.Title,
		"content_size": crawledDoc.ContentSize,
		"child_id":     job.ID,
	})

	// Add job log for successful document storage
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Document saved: %s (%d bytes, %s)",
		crawledDoc.Title, crawledDoc.ContentSize, crawledDoc.ID))

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

		w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Links: %d found, %d filtered, %d excluded", filterResult.Found, filterResult.Filtered, filterResult.Excluded), map[string]interface{}{
			"url":            seedURL,
			"depth":          job.Depth,
			"links_found":    filterResult.Found,
			"links_filtered": filterResult.Filtered,
			"links_excluded": filterResult.Excluded,
			"child_id":       job.ID,
		})

		// Check depth limits for child job spawning
		if job.Depth < crawlConfig.MaxDepth && len(filterResult.FilteredLinks) > 0 {
			w.publishCrawlerProgressUpdate(ctx, job, "running", "Spawning child jobs for discovered links", seedURL)
			// Spawn child jobs for filtered links
			childJobsSpawned := 0
			for i, link := range filterResult.FilteredLinks {
				// Respect max pages limit
				if crawlConfig.MaxPages > 0 && childJobsSpawned >= crawlConfig.MaxPages {
					linkStats.Skipped = len(filterResult.FilteredLinks) - childJobsSpawned
					jobLogger.Debug().
						Int("max_pages", crawlConfig.MaxPages).
						Int("skipped", linkStats.Skipped).
						Msg("Reached max pages limit, skipping remaining links")
					w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Reached max pages limit (%d), skipping %d remaining links", crawlConfig.MaxPages, linkStats.Skipped), map[string]interface{}{
						"url":       seedURL,
						"depth":     job.Depth,
						"max_pages": crawlConfig.MaxPages,
						"skipped":   linkStats.Skipped,
						"child_id":  job.ID,
					})
					break
				}

				if err := w.spawnChildJob(ctx, job, link, crawlConfig, sourceType, entityType, i, jobLogger); err != nil {
					jobLogger.Warn().
						Err(err).
						Str("child_url", link).
						Msg("Failed to spawn child job for discovered link")
					w.publishCrawlerJobLog(ctx, parentID, "warn", fmt.Sprintf("Failed to spawn child job for link: %s", link), map[string]interface{}{
						"url":       seedURL,
						"depth":     job.Depth,
						"child_url": link,
						"error":     err.Error(),
						"child_id":  job.ID,
					})
					continue
				}
				childJobsSpawned++
			}

			linkStats.Followed = childJobsSpawned

			// Note: Per-URL child spawn logging removed to reduce log volume

			w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Spawned %d child jobs", childJobsSpawned), map[string]interface{}{
				"url":                seedURL,
				"depth":              job.Depth,
				"max_depth":          crawlConfig.MaxDepth,
				"child_jobs_spawned": childJobsSpawned,
				"child_id":           job.ID,
			})
		} else if job.Depth >= crawlConfig.MaxDepth {
			// All filtered links are skipped due to depth limit
			linkStats.Skipped = filterResult.Filtered
			jobLogger.Debug().
				Int("current_depth", job.Depth).
				Int("max_depth", crawlConfig.MaxDepth).
				Int("links_skipped", linkStats.Skipped).
				Msg("Reached maximum depth, skipping all discovered links")

			w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("Reached maximum depth (%d), skipping %d discovered links", crawlConfig.MaxDepth, linkStats.Skipped), map[string]interface{}{
				"url":           seedURL,
				"current_depth": job.Depth,
				"max_depth":     crawlConfig.MaxDepth,
				"links_skipped": linkStats.Skipped,
			})
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

	// Publish comprehensive link processing results
	w.publishLinkDiscoveryEvent(ctx, job, linkStats, seedURL)

	// Log comprehensive link processing results
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Links found: %d | filtered: %d | followed: %d",
		linkStats.Found, linkStats.Filtered, linkStats.Followed))

	// Update job status to completed
	w.publishCrawlerProgressUpdate(ctx, job, "completed", "Job completed successfully", seedURL)
	if err := w.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	totalTime := time.Since(startTime)
	// Note: Per-URL completion logging removed to reduce log volume

	w.publishCrawlerJobLog(ctx, parentID, "debug", fmt.Sprintf("URL completed in %v", totalTime), map[string]interface{}{
		"url":        seedURL,
		"depth":      job.Depth,
		"total_time": totalTime.String(),
		"status":     "completed",
		"child_id":   job.ID,
	})

	// Add final job log
	w.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Crawl completed successfully in %v", totalTime))

	return nil
}

// ============================================================================
// CONFIGURATION AND RENDERING HELPERS
// ============================================================================

// extractCrawlConfig extracts CrawlConfig from Job.Config map
func (w *CrawlerWorker) extractCrawlConfig(config map[string]interface{}) (*models.CrawlConfig, error) {
	crawlConfigRaw, ok := config["crawl_config"]
	if !ok {
		return &models.CrawlConfig{}, nil // Return empty config if not found
	}

	// Try direct type assertion first
	if crawlConfig, ok := crawlConfigRaw.(*models.CrawlConfig); ok {
		return crawlConfig, nil
	}

	// Try map[string]interface{} conversion
	if crawlConfigMap, ok := crawlConfigRaw.(map[string]interface{}); ok {
		// Convert map to CrawlConfig struct
		crawlConfig := &models.CrawlConfig{}

		if maxDepth, ok := crawlConfigMap["max_depth"].(float64); ok {
			crawlConfig.MaxDepth = int(maxDepth)
		} else if maxDepth, ok := crawlConfigMap["max_depth"].(int); ok {
			crawlConfig.MaxDepth = maxDepth
		}

		if maxPages, ok := crawlConfigMap["max_pages"].(float64); ok {
			crawlConfig.MaxPages = int(maxPages)
		} else if maxPages, ok := crawlConfigMap["max_pages"].(int); ok {
			crawlConfig.MaxPages = maxPages
		}

		if concurrency, ok := crawlConfigMap["concurrency"].(float64); ok {
			crawlConfig.Concurrency = int(concurrency)
		} else if concurrency, ok := crawlConfigMap["concurrency"].(int); ok {
			crawlConfig.Concurrency = concurrency
		}

		if followLinks, ok := crawlConfigMap["follow_links"].(bool); ok {
			crawlConfig.FollowLinks = followLinks
		}

		// Extract include/exclude patterns
		if includePatterns, ok := crawlConfigMap["include_patterns"].([]interface{}); ok {
			for _, pattern := range includePatterns {
				if patternStr, ok := pattern.(string); ok {
					crawlConfig.IncludePatterns = append(crawlConfig.IncludePatterns, patternStr)
				}
			}
		} else if includePatterns, ok := crawlConfigMap["include_patterns"].([]string); ok {
			crawlConfig.IncludePatterns = includePatterns
		}

		if excludePatterns, ok := crawlConfigMap["exclude_patterns"].([]interface{}); ok {
			for _, pattern := range excludePatterns {
				if patternStr, ok := pattern.(string); ok {
					crawlConfig.ExcludePatterns = append(crawlConfig.ExcludePatterns, patternStr)
				}
			}
		} else if excludePatterns, ok := crawlConfigMap["exclude_patterns"].([]string); ok {
			crawlConfig.ExcludePatterns = excludePatterns
		}

		return crawlConfig, nil
	}

	return &models.CrawlConfig{}, nil
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

	// Navigate to URL and wait for JavaScript rendering
	// Use the browserCtx (ChromeDP context) for ChromeDP operations
	err = chromedp.Run(browserCtx,
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
		// Remove leading dot if present (ChromeDP doesn't like it)
		cookieDomain = strings.TrimPrefix(cookieDomain, ".")

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

			// Set all cookies
			for _, cookie := range chromeDPCookies {
				if err := network.SetCookie(cookie.Name, cookie.Value).
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

	w.publishCrawlerJobLog(ctx, parentJobID, "info", fmt.Sprintf("Injected %d authentication cookies into browser", len(chromeDPCookies)), map[string]interface{}{
		"cookie_count":  len(chromeDPCookies),
		"site_domain":   authCreds.SiteDomain,
		"target_domain": targetURLParsed.Host,
	})

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
		// Copy parent metadata
		for k, v := range parentJob.Metadata {
			childMetadata[k] = v
		}
	}
	childMetadata["discovered_by"] = parentJob.ID
	childMetadata["link_index"] = linkIndex

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

	// Publish job spawn event for real-time monitoring
	w.publishJobSpawnEvent(ctx, parentJob, childJob.ID, childURL)

	return nil
}

// ============================================================================
// REAL-TIME EVENT PUBLISHING
// ============================================================================

// publishCrawlerJobLog publishes a crawler job log event for real-time streaming
func (w *CrawlerWorker) publishCrawlerJobLog(ctx context.Context, jobID, level, message string, metadata map[string]interface{}) {
	if w.eventService == nil {
		return
	}

	payload := map[string]interface{}{
		"job_id":    jobID,
		"level":     level,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if metadata != nil {
		payload["metadata"] = metadata
	}

	event := interfaces.Event{
		Type:    "crawler_job_log",
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking job execution
	common.SafeGo(w.logger, "publishCrawlerJobLog", func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish crawler job log event")
		}
	})
}

// publishCrawlerProgressUpdate publishes a crawler job progress update for real-time monitoring
func (w *CrawlerWorker) publishCrawlerProgressUpdate(ctx context.Context, job *models.QueueJob, status, activity, currentURL string) {
	if w.eventService == nil {
		return
	}

	// Get current progress statistics from job manager
	parentJobID := job.GetParentID()
	if parentJobID == "" {
		parentJobID = job.ID // For root jobs, use self as parent
	}

	// Create basic progress payload
	payload := map[string]interface{}{
		"job_id":           job.ID,
		"parent_id":        parentJobID,
		"status":           status,
		"job_type":         job.Type,
		"current_url":      currentURL,
		"current_activity": activity,
		"timestamp":        time.Now().Format(time.RFC3339),
		"depth":            job.Depth,
	}

	// Add source information
	if sourceType, ok := job.GetConfigString("source_type"); ok {
		payload["source_type"] = sourceType
	}
	if entityType, ok := job.GetConfigString("entity_type"); ok {
		payload["entity_type"] = entityType
	}

	event := interfaces.Event{
		Type:    "crawler_job_progress",
		Payload: payload,
	}

	// Publish asynchronously to avoid blocking job execution
	common.SafeGo(w.logger, "publishCrawlerProgress", func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish crawler progress event")
		}
	})
}

// publishLinkDiscoveryEvent publishes link discovery and following statistics
func (w *CrawlerWorker) publishLinkDiscoveryEvent(ctx context.Context, job *models.QueueJob, linkStats *crawler.LinkProcessingResult, currentURL string) {
	if w.eventService == nil {
		return
	}

	// Use parent ID for log aggregation
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}

	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Links found: %d | filtered: %d | followed: %d | skipped: %d",
		linkStats.Found, linkStats.Filtered, linkStats.Followed, linkStats.Skipped), map[string]interface{}{
		"url":            currentURL,
		"depth":          job.Depth,
		"links_found":    linkStats.Found,
		"links_filtered": linkStats.Filtered,
		"links_followed": linkStats.Followed,
		"links_skipped":  linkStats.Skipped,
		"child_id":       job.ID,
	})
}

// publishJobSpawnEvent publishes a job spawn event when child jobs are created
func (w *CrawlerWorker) publishJobSpawnEvent(ctx context.Context, parentJob *models.QueueJob, childJobID, childURL string) {
	if w.eventService == nil {
		return
	}

	// Get root parent ID for hierarchy tracking
	rootParentID := parentJob.GetParentID()
	if rootParentID == "" {
		rootParentID = parentJob.ID // This job is the root parent
	}

	payload := map[string]interface{}{
		"parent_job_id": rootParentID, // Root parent for flat hierarchy
		"discovered_by": parentJob.ID, // Immediate parent that discovered this link
		"child_job_id":  childJobID,
		"job_type":      "crawler_url",
		"url":           childURL,
		"depth":         parentJob.Depth + 1,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	event := interfaces.Event{
		Type:    interfaces.EventJobSpawn,
		Payload: payload,
	}

	// Publish asynchronously
	common.SafeGo(w.logger, "publishJobSpawn", func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("parent_job_id", parentJob.ID).Str("child_job_id", childJobID).Msg("Failed to publish job spawn event")
		}
	})
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
