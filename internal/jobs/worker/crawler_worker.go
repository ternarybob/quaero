// -----------------------------------------------------------------------
// Crawler Worker - Processes individual crawler jobs from the queue with ChromeDP rendering, content processing, and child job spawning
// -----------------------------------------------------------------------

package worker

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
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerWorker processes individual crawler jobs from the queue, rendering pages with ChromeDP,
// extracting content, and spawning child jobs for discovered links
type CrawlerWorker struct {
	// Core dependencies
	crawlerService  *crawler.Service
	jobMgr          *jobs.Manager
	queueMgr        interfaces.QueueManager
	documentStorage interfaces.DocumentStorage
	authStorage     interfaces.AuthStorage
	jobDefStorage   interfaces.JobDefinitionStorage
	logger          arbor.ILogger
	eventService    interfaces.EventService

	// Content processing components
	contentProcessor *crawler.ContentProcessor
}

// Compile-time assertion: CrawlerWorker implements JobWorker interface
var _ interfaces.JobWorker = (*CrawlerWorker)(nil)

// NewCrawlerWorker creates a new crawler worker for processing individual crawler jobs from the queue
func NewCrawlerWorker(
	crawlerService *crawler.Service,
	jobMgr *jobs.Manager,
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

// Validate validates that the job model is compatible with this worker
func (w *CrawlerWorker) Validate(job *models.JobModel) error {
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
func (w *CrawlerWorker) Execute(ctx context.Context, job *models.JobModel) error {
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

	jobLogger.Info().
		Str("job_id", job.ID).
		Str("parent_id", parentID).
		Str("seed_url", seedURL).
		Str("source_type", sourceType).
		Str("entity_type", entityType).
		Int("depth", job.Depth).
		Int("max_depth", crawlConfig.MaxDepth).
		Bool("follow_links", crawlConfig.FollowLinks).
		Msg("Starting enhanced crawler job execution")

	// Publish real-time log for job start (under parent context)
	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Starting crawl of URL: %s (depth: %d)", seedURL, job.Depth), map[string]interface{}{
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

	jobLogger.Debug().Msg("🚨 ABOUT TO CREATE BROWSER INSTANCE")

	// Step 1: Create a fresh ChromeDP browser instance for this request
	// TEMPORARY: Bypassing pool to debug context cancellation issue
	w.publishCrawlerProgressUpdate(ctx, job, "running", "Creating browser instance", seedURL)

	jobLogger.Debug().Msg("🚨 PUBLISHED PROGRESS UPDATE FOR BROWSER CREATION")

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

	jobLogger.Debug().Msg("Created fresh browser instance (not using pool)")
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

	jobLogger.Info().
		Str("url", seedURL).
		Int("status_code", statusCode).
		Int("html_length", len(htmlContent)).
		Dur("render_time", renderTime).
		Msg("Successfully rendered page with JavaScript")

	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Successfully rendered page (status: %d, size: %d bytes, time: %v)", statusCode, len(htmlContent), renderTime), map[string]interface{}{
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

	jobLogger.Info().
		Str("url", seedURL).
		Str("title", processedContent.Title).
		Int("content_size", processedContent.ContentSize).
		Int("links_found", len(processedContent.Links)).
		Dur("process_time", processedContent.ProcessTime).
		Msg("Successfully processed HTML content")

	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Successfully processed content: '%s' (%d bytes, %d links, %v)", processedContent.Title, processedContent.ContentSize, len(processedContent.Links), processedContent.ProcessTime), map[string]interface{}{
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

	jobLogger.Info().
		Str("document_id", crawledDoc.ID).
		Str("url", seedURL).
		Int("content_size", crawledDoc.ContentSize).
		Msg("Successfully saved crawled document")

	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Document saved: %s (%d bytes)", crawledDoc.Title, crawledDoc.ContentSize), map[string]interface{}{
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

		jobLogger.Info().
			Int("links_found", filterResult.Found).
			Int("links_filtered", filterResult.Filtered).
			Int("links_excluded", filterResult.Excluded).
			Msg("Link filtering completed")

		w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Link filtering completed: %d found, %d filtered, %d excluded", filterResult.Found, filterResult.Filtered, filterResult.Excluded), map[string]interface{}{
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
					jobLogger.Info().
						Int("max_pages", crawlConfig.MaxPages).
						Int("skipped", linkStats.Skipped).
						Msg("Reached max pages limit, skipping remaining links")
					w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Reached max pages limit (%d), skipping %d remaining links", crawlConfig.MaxPages, linkStats.Skipped), map[string]interface{}{
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

			jobLogger.Info().
				Int("child_jobs_spawned", childJobsSpawned).
				Int("depth", job.Depth).
				Int("max_depth", crawlConfig.MaxDepth).
				Msg("Child jobs spawned for discovered links")

			w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Spawned %d child jobs for discovered links", childJobsSpawned), map[string]interface{}{
				"url":                seedURL,
				"depth":              job.Depth,
				"max_depth":          crawlConfig.MaxDepth,
				"child_jobs_spawned": childJobsSpawned,
				"child_id":           job.ID,
			})
		} else if job.Depth >= crawlConfig.MaxDepth {
			// All filtered links are skipped due to depth limit
			linkStats.Skipped = filterResult.Filtered
			jobLogger.Info().
				Int("current_depth", job.Depth).
				Int("max_depth", crawlConfig.MaxDepth).
				Int("links_skipped", linkStats.Skipped).
				Msg("Reached maximum depth, skipping all discovered links")

			w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Reached maximum depth (%d), skipping %d discovered links", crawlConfig.MaxDepth, linkStats.Skipped), map[string]interface{}{
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
	jobLogger.Info().
		Str("job_id", job.ID).
		Str("url", seedURL).
		Dur("total_time", totalTime).
		Msg("Enhanced crawler job execution completed successfully")

	w.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Crawl completed successfully in %v", totalTime), map[string]interface{}{
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
	logger.Debug().Msg("🔐 DIAGNOSTIC: Enabling ChromeDP network domain for cookie operations")
	if err := chromedp.Run(browserCtx, network.Enable()); err != nil {
		logger.Error().Err(err).Msg("🔐 ERROR: Failed to enable network domain")
		return "", 0, fmt.Errorf("failed to enable network domain: %w", err)
	}
	logger.Debug().Msg("🔐 SUCCESS: Network domain enabled successfully")

	// Enable log domain for capturing browser console messages
	logger.Debug().Msg("🔐 DIAGNOSTIC: Enabling ChromeDP log domain for browser console messages")
	if err := chromedp.Run(browserCtx, log.Enable()); err != nil {
		logger.Warn().Err(err).Msg("🔐 WARNING: Failed to enable log domain")
	} else {
		logger.Debug().Msg("🔐 SUCCESS: Log domain enabled successfully")
	}
	// ===== END NETWORK DOMAIN ENABLEMENT =====

	// ===== PHASE 3: COOKIE MONITORING BEFORE NAVIGATION =====
	logger.Debug().
		Str("url", url).
		Msg("🔐 DIAGNOSTIC: Checking cookies before navigation")

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
		logger.Debug().
			Err(err).
			Str("url", url).
			Msg("🔐 WARNING: Failed to read cookies before navigation")
	} else {
		logger.Debug().
			Int("cookie_count", len(cookiesBeforeNav)).
			Str("url", url).
			Msg("🔐 DIAGNOSTIC: Cookies applicable to URL before navigation")

		if len(cookiesBeforeNav) == 0 {
			logger.Debug().
				Str("url", url).
				Msg("🔐 WARNING: No cookies found for URL - navigating without authentication")
		} else {
			// Parse target URL for domain comparison
			targetURLParsed, parseErr := neturl.Parse(url)
			if parseErr != nil {
				logger.Debug().Err(parseErr).Msg("🔐 WARNING: Failed to parse target URL for domain analysis")
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

					logger.Debug().
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
						Msg("🔐 DIAGNOSTIC: Cookie domain analysis before navigation")

					if !isMatch {
						logger.Debug().
							Str("cookie_name", cookie.Name).
							Str("cookie_domain", cookie.Domain).
							Str("target_domain", targetHost).
							Msg("🔐 WARNING: Cookie domain mismatch - cookie may not be sent")
					}
				} else {
					logger.Debug().
						Int("cookie_index", i).
						Str("cookie_name", cookie.Name).
						Str("cookie_domain", cookie.Domain).
						Str("cookie_path", cookie.Path).
						Bool("secure", cookie.Secure).
						Bool("http_only", cookie.HTTPOnly).
						Str("same_site", string(cookie.SameSite)).
						Msg("🔐 DIAGNOSTIC: Cookie will be sent with navigation")
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
				logger.Debug().
					Str("source", evTyped.Entry.Source.String()).
					Str("level", evTyped.Entry.Level.String()).
					Str("message", evTyped.Entry.Text).
					Msg("🔐 DIAGNOSTIC: Browser console message about cookies")
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
				logger.Debug().
					Str("request_url", evTyped.Request.URL).
					Str("cookie_header", cookieHeader).
					Msg("🔐 DIAGNOSTIC: Cookie header sent with request")
			} else {
				logger.Debug().
					Str("request_url", evTyped.Request.URL).
					Msg("🔐 DIAGNOSTIC: No Cookie header in request")
			}

		case *network.EventLoadingFailed:
			// Log network failures that might indicate cookie issues
			logger.Debug().
				Str("request_url", evTyped.RequestID.String()).
				Str("error_text", evTyped.ErrorText).
				Str("type", evTyped.Type.String()).
				Msg("🔐 WARNING: Network request failed (possible cookie issue)")

		case *network.EventResponseReceivedExtraInfo:
			// Log Set-Cookie headers in responses
			if evTyped.Headers != nil {
				if setCookie, exists := evTyped.Headers["Set-Cookie"]; exists {
					logger.Debug().
						Str("set_cookie_header", fmt.Sprintf("%v", setCookie)).
						Msg("🔐 DIAGNOSTIC: Server sent Set-Cookie header")
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
	logger.Debug().
		Str("url", url).
		Msg("🔐 DIAGNOSTIC: Checking cookies after navigation")

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
		logger.Debug().
			Err(err).
			Str("url", url).
			Msg("🔐 WARNING: Failed to read cookies after navigation")
	} else {
		logger.Debug().
			Int("cookie_count", len(cookiesAfterNav)).
			Str("url", url).
			Msg("🔐 DIAGNOSTIC: Cookies after navigation")

		// Compare before/after cookie counts
		if len(cookiesBeforeNav) > 0 && len(cookiesAfterNav) < len(cookiesBeforeNav) {
			logger.Warn().
				Int("cookies_before", len(cookiesBeforeNav)).
				Int("cookies_after", len(cookiesAfterNav)).
				Int("cookies_lost", len(cookiesBeforeNav)-len(cookiesAfterNav)).
				Msg("🔐 WARNING: Cookies were cleared during navigation")
		} else if len(cookiesAfterNav) > len(cookiesBeforeNav) {
			logger.Debug().
				Int("cookies_before", len(cookiesBeforeNav)).
				Int("cookies_after", len(cookiesAfterNav)).
				Int("cookies_gained", len(cookiesAfterNav)-len(cookiesBeforeNav)).
				Msg("🔐 DIAGNOSTIC: New cookies set during navigation")
		} else if len(cookiesBeforeNav) > 0 {
			logger.Debug().
				Int("cookie_count", len(cookiesAfterNav)).
				Msg("🔐 SUCCESS: Cookies persisted through navigation")
		}
	}
	// ===== END PHASE 3 =====

	logger.Debug().
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
	logger.Debug().
		Str("parent_job_id", parentJobID).
		Str("target_url", targetURL).
		Msg("🔐 START: Cookie injection process initiated")

	// Check if authStorage is available
	if w.authStorage == nil {
		logger.Debug().Msg("🔐 SKIP: Auth storage not configured, skipping cookie injection")
		return nil
	}
	logger.Debug().Msg("🔐 OK: Auth storage is configured")

	// Get parent job from database to retrieve AuthID from job metadata
	logger.Debug().Str("parent_job_id", parentJobID).Msg("🔐 Fetching parent job from database")
	parentJobInterface, err := w.jobMgr.GetJob(ctx, parentJobID)
	if err != nil {
		logger.Error().Err(err).Str("parent_job_id", parentJobID).Msg("🔐 ERROR: Failed to get parent job for auth lookup")
		return fmt.Errorf("failed to get parent job: %w", err)
	}
	logger.Debug().Msg("🔐 OK: Parent job retrieved from database")

	// Extract JobModel from either JobModel or Job (which embeds JobModel)
	var authID string
	var jobModel *models.JobModel

	// Try Job first (embeds JobModel)
	if job, ok := parentJobInterface.(*models.Job); ok {
		logger.Debug().Msg("🔐 OK: Parent job is Job type (with embedded JobModel)")
		jobModel = job.JobModel
	} else if jm, ok := parentJobInterface.(*models.JobModel); ok {
		logger.Debug().Msg("🔐 OK: Parent job is JobModel type")
		jobModel = jm
	} else {
		logger.Error().
			Str("actual_type", fmt.Sprintf("%T", parentJobInterface)).
			Msg("🔐 ERROR: Parent job is neither Job nor JobModel type")
		return nil
	}

	if jobModel == nil {
		logger.Error().Msg("🔐 ERROR: JobModel is nil after extraction")
		return nil
	}

	logger.Debug().
		Int("metadata_count", len(jobModel.Metadata)).
		Msg("🔐 OK: JobModel extracted successfully")

	// Log all metadata keys for debugging
	metadataKeys := make([]string, 0, len(jobModel.Metadata))
	for k := range jobModel.Metadata {
		metadataKeys = append(metadataKeys, k)
	}
	logger.Debug().
		Strs("metadata_keys", metadataKeys).
		Msg("🔐 DEBUG: Parent job metadata keys")

	// Check metadata for auth_id
	if authIDVal, exists := jobModel.Metadata["auth_id"]; exists {
		if authIDStr, ok := authIDVal.(string); ok && authIDStr != "" {
			authID = authIDStr
			logger.Debug().
				Str("auth_id", authID).
				Msg("🔐 FOUND: Auth ID in job metadata")
		} else {
			logger.Debug().
				Str("auth_id_value", fmt.Sprintf("%v", authIDVal)).
				Msg("🔐 WARNING: auth_id exists but is not a valid string")
		}
	} else {
		logger.Debug().Msg("🔐 WARNING: auth_id NOT found in job metadata")
	}

	// If not in metadata, try job_definition_id
	if authID == "" {
		logger.Debug().Msg("🔐 Trying job_definition_id fallback")
		if jobDefID, exists := jobModel.Metadata["job_definition_id"]; exists {
			if jobDefIDStr, ok := jobDefID.(string); ok && jobDefIDStr != "" {
				logger.Debug().
					Str("job_def_id", jobDefIDStr).
					Msg("🔐 Found job_definition_id, fetching job definition")
				jobDef, err := w.jobDefStorage.GetJobDefinition(ctx, jobDefIDStr)
				if err != nil {
					logger.Error().Err(err).Str("job_def_id", jobDefIDStr).Msg("🔐 ERROR: Failed to get job definition for auth lookup")
					return fmt.Errorf("failed to get job definition: %w", err)
				}
				if jobDef != nil && jobDef.AuthID != "" {
					authID = jobDef.AuthID
					logger.Debug().
						Str("auth_id", authID).
						Str("job_def_id", jobDefIDStr).
						Msg("🔐 FOUND: Auth ID from job definition")
				} else {
					logger.Debug().
						Str("job_def_id", jobDefIDStr).
						Msg("🔐 WARNING: Job definition has no AuthID")
				}
			}
		} else {
			logger.Debug().Msg("🔐 WARNING: job_definition_id NOT found in metadata")
		}
	}

	if authID == "" {
		logger.Debug().Msg("🔐 SKIP: No auth_id found - skipping cookie injection")
		return nil
	}

	// Load authentication credentials from storage using AuthID
	logger.Debug().
		Str("auth_id", authID).
		Msg("🔐 Loading auth credentials from storage")
	authCreds, err := w.authStorage.GetCredentialsByID(ctx, authID)
	if err != nil {
		logger.Error().Err(err).Str("auth_id", authID).Msg("🔐 ERROR: Failed to load auth credentials from storage")
		return fmt.Errorf("failed to load auth credentials: %w", err)
	}

	if authCreds == nil {
		logger.Error().Str("auth_id", authID).Msg("🔐 ERROR: Auth credentials not found in storage")
		return fmt.Errorf("auth credentials not found for ID: %s", authID)
	}
	logger.Debug().
		Str("auth_id", authID).
		Str("site_domain", authCreds.SiteDomain).
		Msg("🔐 OK: Auth credentials loaded successfully")

	// Unmarshal cookies from JSON
	logger.Debug().Msg("🔐 Unmarshaling cookies from JSON")
	var extensionCookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &extensionCookies); err != nil {
		logger.Error().Err(err).Msg("🔐 ERROR: Failed to unmarshal cookies from auth credentials")
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	if len(extensionCookies) == 0 {
		logger.Debug().Msg("🔐 WARNING: No cookies found in auth credentials")
		return nil
	}

	logger.Debug().
		Int("cookie_count", len(extensionCookies)).
		Str("site_domain", authCreds.SiteDomain).
		Msg("🔐 SUCCESS: Cookies loaded - preparing to inject into browser")

	// Parse target URL to get domain
	targetURLParsed, err := neturl.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("failed to parse target URL: %w", err)
	}

	// ===== PHASE 1: PRE-INJECTION DOMAIN DIAGNOSTICS =====
	logger.Debug().
		Str("target_url", targetURL).
		Str("target_domain", targetURLParsed.Host).
		Str("target_scheme", targetURLParsed.Scheme).
		Msg("🔐 DIAGNOSTIC: Target URL parsed for domain analysis")

	// Analyze each cookie's domain compatibility with target URL
	logger.Debug().Msg("🔐 DIAGNOSTIC: Analyzing cookie domain compatibility with target URL")
	for i, c := range extensionCookies {
		cookieDomain := c.Domain
		if cookieDomain == "" {
			logger.Debug().
				Int("cookie_index", i).
				Str("cookie_name", c.Name).
				Msg("🔐 DIAGNOSTIC: Cookie has no domain (will use target domain)")
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

		logger.Debug().
			Int("cookie_index", i).
			Str("cookie_name", c.Name).
			Str("cookie_domain", cookieDomain).
			Str("normalized_cookie_domain", normalizedCookieDomain).
			Str("target_domain", targetHost).
			Str("match_type", matchType).
			Bool("will_be_sent", isMatch).
			Msg("🔐 DIAGNOSTIC: Cookie domain analysis")

		if !isMatch {
			logger.Debug().
				Str("cookie_name", c.Name).
				Str("cookie_domain", cookieDomain).
				Str("target_domain", targetHost).
				Msg("🔐 WARNING: Cookie domain mismatch - cookie may not be sent with requests")
		}

		// Check secure flag compatibility with scheme
		if c.Secure && targetURLParsed.Scheme != "https" {
			logger.Debug().
				Str("cookie_name", c.Name).
				Str("target_scheme", targetURLParsed.Scheme).
				Msg("🔐 WARNING: Secure cookie will not be sent to non-HTTPS URL")
		}
	}
	// ===== END PHASE 1 =====

	// Convert extension cookies to ChromeDP network cookies
	logger.Debug().
		Int("cookie_count", len(extensionCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("🔐 Converting extension cookies to ChromeDP format")
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

		logger.Debug().
			Int("cookie_index", i).
			Str("name", c.Name).
			Str("domain", cookieDomain).
			Str("path", c.Path).
			Bool("secure", c.Secure).
			Bool("http_only", c.HTTPOnly).
			Msg("🔐 OK: Prepared cookie for injection")
	}

	// ===== PHASE 2: NETWORK DOMAIN ENABLEMENT =====
	logger.Debug().Msg("🔐 DIAGNOSTIC: Enabling ChromeDP network domain for cookie operations")
	err = chromedp.Run(browserCtx, network.Enable())
	if err != nil {
		logger.Error().Err(err).Msg("🔐 ERROR: Failed to enable network domain")
		return fmt.Errorf("failed to enable network domain: %w", err)
	}
	logger.Debug().Msg("🔐 SUCCESS: Network domain enabled successfully")
	// ===== END PHASE 2 PART 1 =====

	// Inject cookies into browser using ChromeDP
	logger.Debug().
		Int("cookie_count", len(chromeDPCookies)).
		Msg("🔐 Starting browser cookie injection via ChromeDP")

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
						Msg("🔐 ERROR: Failed to inject cookie into browser")
					// Continue with other cookies even if one fails
				} else {
					successCount++
					logger.Debug().
						Str("cookie_name", cookie.Name).
						Str("domain", cookie.Domain).
						Msg("🔐 OK: Cookie injected successfully")
				}
			}

			logger.Debug().
				Int("success_count", successCount).
				Int("fail_count", failCount).
				Int("total_cookies", len(chromeDPCookies)).
				Msg("🔐 Cookie injection batch complete")

			return nil
		}),
	)

	if err != nil {
		logger.Error().
			Err(err).
			Str("target_url", targetURL).
			Int("cookies_attempted", len(chromeDPCookies)).
			Msg("🔐 ERROR: ChromeDP failed to inject cookies into browser")
		return fmt.Errorf("failed to inject cookies: %w", err)
	}

	logger.Info().
		Int("cookies_injected", len(chromeDPCookies)).
		Str("target_domain", targetURLParsed.Host).
		Msg("🔐 SUCCESS: Authentication cookies injected into browser")

	// ===== PHASE 2 PART 2: POST-INJECTION VERIFICATION =====
	logger.Debug().
		Str("target_url", targetURL).
		Msg("🔐 DIAGNOSTIC: Verifying cookies after injection using network.GetCookies()")

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
			Msg("🔐 ERROR: Failed to verify cookies after injection")
		// Don't return error - continue with warning
	} else {
		logger.Debug().
			Int("verified_cookie_count", len(verifiedCookies)).
			Int("injected_cookie_count", len(chromeDPCookies)).
			Msg("🔐 DIAGNOSTIC: Cookie verification complete")

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

			logger.Debug().
				Int("cookie_index", i).
				Str("name", cookie.Name).
				Str("value_preview", valuePreview).
				Str("domain", cookie.Domain).
				Str("path", cookie.Path).
				Bool("secure", cookie.Secure).
				Bool("http_only", cookie.HTTPOnly).
				Str("same_site", string(cookie.SameSite)).
				Str("expires", expiryStr).
				Msg("🔐 DIAGNOSTIC: Verified cookie details")
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
				Msg("🔐 ERROR: Cookies were injected but not verified (failed to persist)")
		}

		if len(unexpectedCookies) > 0 {
			logger.Debug().
				Strs("unexpected_cookies", unexpectedCookies).
				Int("unexpected_count", len(unexpectedCookies)).
				Msg("🔐 WARNING: Cookies verified but not injected (pre-existing or set by page)")
		}

		// Final verdict
		if len(verifiedCookies) == len(chromeDPCookies) && len(missingCookies) == 0 {
			logger.Debug().
				Int("cookie_count", len(verifiedCookies)).
				Msg("🔐 SUCCESS: All injected cookies verified successfully")
		} else {
			logger.Warn().
				Int("injected", len(chromeDPCookies)).
				Int("verified", len(verifiedCookies)).
				Int("missing", len(missingCookies)).
				Msg("🔐 WARNING: Cookie injection/verification mismatch detected")
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
func (w *CrawlerWorker) spawnChildJob(ctx context.Context, parentJob *models.JobModel, childURL string, crawlConfig *models.CrawlConfig, sourceType, entityType string, linkIndex int, logger arbor.ILogger) error {
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

	// Create child job model with incremented depth
	childJob := models.NewChildJobModel(
		parentJob.GetParentID(), // All children reference the same root parent (flat hierarchy)
		"crawler_url",
		fmt.Sprintf("URL: %s", childURL),
		childConfig,
		childMetadata,
		parentJob.Depth+1, // Increment depth for child
	)

	// Validate child job
	if err := childJob.Validate(); err != nil {
		return fmt.Errorf("invalid child job model: %w", err)
	}

	// Serialize job model to JSON for payload
	payloadBytes, err := childJob.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize child job model: %w", err)
	}

	// Create job record in database
	if err := w.jobMgr.CreateJobRecord(ctx, &jobs.Job{
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

	logger.Debug().
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
	go func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish crawler job log event")
		}
	}()
}

// publishCrawlerProgressUpdate publishes a crawler job progress update for real-time monitoring
func (w *CrawlerWorker) publishCrawlerProgressUpdate(ctx context.Context, job *models.JobModel, status, activity, currentURL string) {
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
	go func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish crawler progress event")
		}
	}()
}

// publishLinkDiscoveryEvent publishes link discovery and following statistics
func (w *CrawlerWorker) publishLinkDiscoveryEvent(ctx context.Context, job *models.JobModel, linkStats *crawler.LinkProcessingResult, currentURL string) {
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
func (w *CrawlerWorker) publishJobSpawnEvent(ctx context.Context, parentJob *models.JobModel, childJobID, childURL string) {
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
	go func() {
		if err := w.eventService.Publish(ctx, event); err != nil {
			w.logger.Warn().Err(err).Str("parent_job_id", parentJob.ID).Str("child_job_id", childJobID).Msg("Failed to publish job spawn event")
		}
	}()
}
