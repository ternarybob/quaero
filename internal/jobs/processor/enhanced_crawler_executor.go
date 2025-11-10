// -----------------------------------------------------------------------
// Enhanced Crawler Executor - Individual URL crawling with ChromeDP and content processing
// -----------------------------------------------------------------------

package processor

import (
	"context"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

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

// EnhancedCrawlerExecutor executes enhanced crawler jobs with ChromeDP rendering,
// content processing, and child job spawning for discovered links
type EnhancedCrawlerExecutor struct {
	// Core dependencies
	crawlerService   *crawler.Service
	jobMgr           *jobs.Manager
	queueMgr         *queue.Manager
	documentStorage  interfaces.DocumentStorage
	authStorage      interfaces.AuthStorage
	jobDefStorage    interfaces.JobDefinitionStorage
	logger           arbor.ILogger
	eventService     interfaces.EventService

	// Content processing components
	contentProcessor *crawler.ContentProcessor
}

// NewEnhancedCrawlerExecutor creates a new enhanced crawler executor
func NewEnhancedCrawlerExecutor(
	crawlerService *crawler.Service,
	jobMgr *jobs.Manager,
	queueMgr *queue.Manager,
	documentStorage interfaces.DocumentStorage,
	authStorage interfaces.AuthStorage,
	jobDefStorage interfaces.JobDefinitionStorage,
	logger arbor.ILogger,
	eventService interfaces.EventService,
) *EnhancedCrawlerExecutor {
	return &EnhancedCrawlerExecutor{
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

// GetJobType returns the job type this executor handles
func (e *EnhancedCrawlerExecutor) GetJobType() string {
	return "crawler_url"
}

// Validate validates that the job model is compatible with this executor
func (e *EnhancedCrawlerExecutor) Validate(job *models.JobModel) error {
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

// Execute executes an enhanced crawler job with full workflow:
// 1. ChromeDP page rendering and JavaScript execution
// 2. Content extraction and markdown conversion
// 3. Document storage with comprehensive metadata
// 4. Link discovery and filtering
// 5. Child job spawning for discovered links (respecting depth limits)
func (e *EnhancedCrawlerExecutor) Execute(ctx context.Context, job *models.JobModel) error {
	// Create job-specific logger using parent context for log aggregation
	// All children log under the root parent ID for unified log viewing
	parentID := job.GetParentID()
	if parentID == "" {
		// This is a root job (shouldn't happen for crawler_url type, but handle gracefully)
		parentID = job.ID
	}
	jobLogger := e.logger.WithCorrelationId(parentID)

	// Extract configuration
	seedURL, _ := job.GetConfigString("seed_url")
	sourceType, _ := job.GetConfigString("source_type")
	entityType, _ := job.GetConfigString("entity_type")

	// Extract crawl config from job config
	crawlConfig, err := e.extractCrawlConfig(job.Config)
	if err != nil {
		jobLogger.Error().Err(err).Msg("Failed to extract crawl config")
		e.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to extract crawl config: %v", err), map[string]interface{}{
			"url":        seedURL,
			"depth":      job.Depth,
			"child_id":   job.ID,
			"discovered": job.Metadata["discovered_by"],
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Invalid crawl config: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
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
	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Starting enhanced crawl of URL: %s (depth: %d)", seedURL, job.Depth), map[string]interface{}{
		"url":          seedURL,
		"depth":        job.Depth,
		"max_depth":    crawlConfig.MaxDepth,
		"follow_links": crawlConfig.FollowLinks,
		"child_id":     job.ID,
		"discovered":   job.Metadata["discovered_by"],
	})

	// Update job status to running
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to running")
	}

	// Add job log for execution start
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Starting enhanced crawl of URL: %s (depth: %d)", seedURL, job.Depth))

	// Publish initial progress update
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Acquiring browser from pool", seedURL)

	jobLogger.Info().Msg("🚨 ABOUT TO CREATE BROWSER INSTANCE")

	// Step 1: Create a fresh ChromeDP browser instance for this request
	// TEMPORARY: Bypassing pool to debug context cancellation issue
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Creating browser instance", seedURL)

	jobLogger.Info().Msg("🚨 PUBLISHED PROGRESS UPDATE FOR BROWSER CREATION")

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
	e.publishCrawlerJobLog(ctx, parentID, "debug", "Created fresh browser instance", map[string]interface{}{
		"url":      seedURL,
		"depth":    job.Depth,
		"child_id": job.ID,
	})

	// Step 1.5: Load and inject authentication cookies into browser
	if err := e.injectAuthCookies(ctx, browserCtx, parentID, seedURL, jobLogger); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to inject authentication cookies - continuing without authentication")
		e.publishCrawlerJobLog(ctx, parentID, "warn", fmt.Sprintf("Failed to inject authentication cookies: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
			"error":    err.Error(),
		})
	}

	// Step 2: Navigate to URL and render JavaScript
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Rendering page with JavaScript", seedURL)
	startTime := time.Now()
	htmlContent, statusCode, err := e.renderPageWithChromeDp(ctx, browserCtx, seedURL, jobLogger)
	if err != nil {
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to render page with ChromeDP")
		e.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to render page with ChromeDP: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Page rendering failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to render page: %w", err)
	}
	renderTime := time.Since(startTime)

	jobLogger.Info().
		Str("url", seedURL).
		Int("status_code", statusCode).
		Int("html_length", len(htmlContent)).
		Dur("render_time", renderTime).
		Msg("Successfully rendered page with JavaScript")

	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Successfully rendered page (status: %d, size: %d bytes, time: %v)", statusCode, len(htmlContent), renderTime), map[string]interface{}{
		"url":         seedURL,
		"depth":       job.Depth,
		"status_code": statusCode,
		"html_length": len(htmlContent),
		"render_time": renderTime.String(),
		"child_id":    job.ID,
	})

	// Step 3: Process HTML content and convert to markdown
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Processing HTML content and converting to markdown", seedURL)
	processedContent, err := e.contentProcessor.ProcessHTML(htmlContent, seedURL)
	if err != nil {
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to process HTML content")
		e.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to process HTML content: %v", err), map[string]interface{}{
			"url":      seedURL,
			"depth":    job.Depth,
			"child_id": job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Content processing failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to process content: %w", err)
	}

	jobLogger.Info().
		Str("url", seedURL).
		Str("title", processedContent.Title).
		Int("content_size", processedContent.ContentSize).
		Int("links_found", len(processedContent.Links)).
		Dur("process_time", processedContent.ProcessTime).
		Msg("Successfully processed HTML content")

	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Successfully processed content: '%s' (%d bytes, %d links, %v)", processedContent.Title, processedContent.ContentSize, len(processedContent.Links), processedContent.ProcessTime), map[string]interface{}{
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
	crawledDoc := crawler.NewCrawledDocument(job.ID, parentJobID, seedURL, processedContent)

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
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Saving document to storage", seedURL)
	docPersister := crawler.NewDocumentPersister(e.documentStorage, e.eventService, jobLogger)
	if err := docPersister.SaveCrawledDocument(crawledDoc); err != nil {
		jobLogger.Error().Err(err).Str("url", seedURL).Msg("Failed to save crawled document")
		e.publishCrawlerJobLog(ctx, parentID, "error", fmt.Sprintf("Failed to save crawled document: %v", err), map[string]interface{}{
			"url":         seedURL,
			"depth":       job.Depth,
			"document_id": crawledDoc.ID,
			"child_id":    job.ID,
		})
		e.jobMgr.SetJobError(ctx, job.ID, fmt.Sprintf("Document storage failed: %v", err))
		e.jobMgr.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("failed to save document: %w", err)
	}

	jobLogger.Info().
		Str("document_id", crawledDoc.ID).
		Str("url", seedURL).
		Int("content_size", crawledDoc.ContentSize).
		Msg("Successfully saved crawled document")

	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Document saved: %s (%d bytes)", crawledDoc.Title, crawledDoc.ContentSize), map[string]interface{}{
		"url":          seedURL,
		"depth":        job.Depth,
		"document_id":  crawledDoc.ID,
		"title":        crawledDoc.Title,
		"content_size": crawledDoc.ContentSize,
		"child_id":     job.ID,
	})

	// Add job log for successful document storage
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Document saved: %s (%d bytes, %s)",
		crawledDoc.Title, crawledDoc.ContentSize, crawledDoc.ID))

	// Step 6: Link discovery and filtering with child job spawning
	e.publishCrawlerProgressUpdate(ctx, job, "running", "Discovering and filtering links", seedURL)
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

		e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Link filtering completed: %d found, %d filtered, %d excluded", filterResult.Found, filterResult.Filtered, filterResult.Excluded), map[string]interface{}{
			"url":            seedURL,
			"depth":          job.Depth,
			"links_found":    filterResult.Found,
			"links_filtered": filterResult.Filtered,
			"links_excluded": filterResult.Excluded,
			"child_id":       job.ID,
		})

		// Check depth limits for child job spawning
		if job.Depth < crawlConfig.MaxDepth && len(filterResult.FilteredLinks) > 0 {
			e.publishCrawlerProgressUpdate(ctx, job, "running", "Spawning child jobs for discovered links", seedURL)
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
					e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Reached max pages limit (%d), skipping %d remaining links", crawlConfig.MaxPages, linkStats.Skipped), map[string]interface{}{
						"url":       seedURL,
						"depth":     job.Depth,
						"max_pages": crawlConfig.MaxPages,
						"skipped":   linkStats.Skipped,
						"child_id":  job.ID,
					})
					break
				}

				if err := e.spawnChildJob(ctx, job, link, crawlConfig, sourceType, entityType, i, jobLogger); err != nil {
					jobLogger.Warn().
						Err(err).
						Str("child_url", link).
						Msg("Failed to spawn child job for discovered link")
					e.publishCrawlerJobLog(ctx, parentID, "warn", fmt.Sprintf("Failed to spawn child job for link: %s", link), map[string]interface{}{
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

			e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Spawned %d child jobs for discovered links", childJobsSpawned), map[string]interface{}{
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

			e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Reached maximum depth (%d), skipping %d discovered links", crawlConfig.MaxDepth, linkStats.Skipped), map[string]interface{}{
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
	e.publishLinkDiscoveryEvent(ctx, job, linkStats, seedURL)

	// Log comprehensive link processing results
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Links found: %d | filtered: %d | followed: %d",
		linkStats.Found, linkStats.Filtered, linkStats.Followed))

	// Update job status to completed
	e.publishCrawlerProgressUpdate(ctx, job, "completed", "Job completed successfully", seedURL)
	if err := e.jobMgr.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
		jobLogger.Warn().Err(err).Msg("Failed to update job status to completed")
		return fmt.Errorf("failed to update job status: %w", err)
	}

	totalTime := time.Since(startTime)
	jobLogger.Info().
		Str("job_id", job.ID).
		Str("url", seedURL).
		Dur("total_time", totalTime).
		Msg("Enhanced crawler job execution completed successfully")

	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Crawl completed successfully in %v", totalTime), map[string]interface{}{
		"url":        seedURL,
		"depth":      job.Depth,
		"total_time": totalTime.String(),
		"status":     "completed",
		"child_id":   job.ID,
	})

	// Add final job log
	e.jobMgr.AddJobLog(ctx, job.ID, "info", fmt.Sprintf("Crawl completed successfully in %v", totalTime))

	return nil
}

// extractCrawlConfig extracts CrawlConfig from Job.Config map
func (e *EnhancedCrawlerExecutor) extractCrawlConfig(config map[string]interface{}) (*models.CrawlConfig, error) {
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
func (e *EnhancedCrawlerExecutor) renderPageWithChromeDp(ctx context.Context, browserCtx context.Context, url string, logger arbor.ILogger) (string, int, error) {
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
		logger.Warn().
			Err(err).
			Str("url", url).
			Msg("🔐 WARNING: Failed to read cookies before navigation")
	} else {
		logger.Debug().
			Int("cookie_count", len(cookiesBeforeNav)).
			Str("url", url).
			Msg("🔐 DIAGNOSTIC: Cookies applicable to URL before navigation")

		if len(cookiesBeforeNav) == 0 {
			logger.Warn().
				Str("url", url).
				Msg("🔐 WARNING: No cookies found for URL - navigating without authentication")
		} else {
			// Parse target URL for domain comparison
			targetURLParsed, parseErr := neturl.Parse(url)
			if parseErr != nil {
				logger.Warn().Err(parseErr).Msg("🔐 WARNING: Failed to parse target URL for domain analysis")
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
						logger.Warn().
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
			logger.Warn().
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
		logger.Warn().
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

// spawnChildJob creates and enqueues a child job for a discovered link
func (e *EnhancedCrawlerExecutor) spawnChildJob(ctx context.Context, parentJob *models.JobModel, childURL string, crawlConfig *models.CrawlConfig, sourceType, entityType string, linkIndex int, logger arbor.ILogger) error {
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
	if err := e.jobMgr.CreateJobRecord(ctx, &jobs.Job{
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

	if err := e.queueMgr.Enqueue(ctx, queueMsg); err != nil {
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
	e.publishJobSpawnEvent(ctx, parentJob, childJob.ID, childURL)

	return nil
}

// ============================================================================
// Real-Time Logging and Event Publishing Methods (Task 4.3)
// ============================================================================

// publishCrawlerJobLog publishes a crawler job log event for real-time streaming
func (e *EnhancedCrawlerExecutor) publishCrawlerJobLog(ctx context.Context, jobID, level, message string, metadata map[string]interface{}) {
	if e.eventService == nil {
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
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to publish crawler job log event")
		}
	}()
}

// publishCrawlerProgressUpdate publishes a crawler job progress update for real-time monitoring
func (e *EnhancedCrawlerExecutor) publishCrawlerProgressUpdate(ctx context.Context, job *models.JobModel, status, activity, currentURL string) {
	if e.eventService == nil {
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
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("job_id", job.ID).Msg("Failed to publish crawler progress event")
		}
	}()
}

// publishLinkDiscoveryEvent publishes link discovery and following statistics
func (e *EnhancedCrawlerExecutor) publishLinkDiscoveryEvent(ctx context.Context, job *models.JobModel, linkStats *crawler.LinkProcessingResult, currentURL string) {
	if e.eventService == nil {
		return
	}

	// Use parent ID for log aggregation
	parentID := job.GetParentID()
	if parentID == "" {
		parentID = job.ID
	}

	e.publishCrawlerJobLog(ctx, parentID, "info", fmt.Sprintf("Links found: %d | filtered: %d | followed: %d | skipped: %d",
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
func (e *EnhancedCrawlerExecutor) publishJobSpawnEvent(ctx context.Context, parentJob *models.JobModel, childJobID, childURL string) {
	if e.eventService == nil {
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
		if err := e.eventService.Publish(ctx, event); err != nil {
			e.logger.Warn().Err(err).Str("parent_job_id", parentJob.ID).Str("child_job_id", childJobID).Msg("Failed to publish job spawn event")
		}
	}()
}
