package types

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// CrawlerJobDeps holds dependencies for crawler jobs
type CrawlerJobDeps struct {
	CrawlerService         *crawler.Service
	LogService             interfaces.LogService
	DocumentStorage        interfaces.DocumentStorage
	QueueManager           interfaces.QueueManager
	JobStorage             interfaces.JobStorage
	EventService           interfaces.EventService
	JobDefinitionStorage   interfaces.JobDefinitionStorage
	JobManager             interfaces.JobManager
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

// formatJobError formats a concise, user-friendly error message from a Go error.
// Returns messages in the format "Category: Brief description" suitable for UI display.
// Handles common error types:
//   - HTTP errors: "HTTP 404: Not Found"
//   - Timeout errors: "Timeout: Request exceeded 30s"
//   - Network errors: "Network: Connection refused"
//   - Storage errors: "Storage: Database locked"
//   - Generic errors: "Category: error.Error()" (truncated to 200 chars)
func formatJobError(category string, err error) string {
	if err == nil {
		return ""
	}

	// Check for timeout errors
	if errors.Is(err, context.DeadlineExceeded) {
		return "Timeout: Request exceeded deadline"
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Check for timeout in error message
	if strings.Contains(errMsgLower, "timeout") || strings.Contains(errMsgLower, "deadline exceeded") {
		if category == "Scraping" {
			return "Timeout: Scraping timeout"
		}
		return "Timeout: Request timeout"
	}

	// Check for network errors
	var urlErr *url.Error
	var netOpErr *net.OpError
	if errors.As(err, &urlErr) || errors.As(err, &netOpErr) {
		// Extract brief cause from network error
		briefCause := errMsg
		if len(briefCause) > 100 {
			// Try to extract the most relevant part
			if idx := strings.LastIndex(briefCause, ":"); idx != -1 && idx < len(briefCause)-1 {
				briefCause = strings.TrimSpace(briefCause[idx+1:])
			}
		}
		return fmt.Sprintf("Network: %s", briefCause)
	}

	// Truncate long error messages
	const maxLength = 200
	if len(errMsg) > maxLength {
		errMsg = errMsg[:maxLength] + "..."
	}

	// Format as "Category: Brief description"
	return fmt.Sprintf("%s: %s", category, errMsg)
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
		// Load parent job and populate error field
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				job.Error = formatJobError("Validation", err)
				job.Status = models.JobStatusFailed
				job.CompletedAt = time.Now()
				job.ResultCount = job.Progress.CompletedURLs
				job.FailedCount = job.Progress.FailedURLs
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to save job with validation error")
				} else {
					c.logger.Info().
						Str("parent_id", msg.ParentID).
						Str("error", job.Error).
						Msg("Job marked as failed due to validation error")
				}
			}
		}
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

	entityType := "url"
	if et, ok := msg.Config["entity_type"].(string); ok {
		entityType = et
	}

	// Publish EventJobStarted if this is the first URL (job transitioning from pending to running)
	if c.deps.EventService != nil {
		// Load job to check status
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				// Only publish if job is still pending (first URL)
				if job.Status == models.JobStatusPending {
					// Update job status to running
					job.Status = models.JobStatusRunning
					if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
						c.logger.Warn().Err(saveErr).Msg("Failed to update job status to running")
					} else {
						// Publish EventJobStarted event
						startedEvent := interfaces.Event{
							Type: interfaces.EventJobStarted,
							Payload: map[string]interface{}{
								"job_id":      msg.ParentID,
								"status":      "running",
								"source_type": sourceType,
								"entity_type": entityType,
								"url":         msg.URL,
								"depth":       msg.Depth,
								"timestamp":   time.Now().Format(time.RFC3339),
							},
						}
						if err := c.deps.EventService.Publish(ctx, startedEvent); err != nil {
							c.logger.Warn().Err(err).Msg("Failed to publish job started event")
						}
					}
				}
			}
		}
	}

	// Update child job status to running if this is a child job being processed
	// Guard: Verify message ID matches expected child job pattern (contains "-child-" or "-seed-")
	if msg.ParentID != "" && (strings.Contains(msg.ID, "-child-") || strings.Contains(msg.ID, "-seed-")) {
		childJobInterface, childErr := c.deps.JobStorage.GetJob(ctx, msg.ID)
		if childErr != nil {
			// Child job row missing - this can happen during upgrade or if persistence failed
			c.logger.Warn().
				Err(childErr).
				Str("child_id", msg.ID).
				Str("parent_id", msg.ParentID).
				Msg("Child job row not found in database (upgrade or persistence failure) - continuing without status update")
		} else if childJob, ok := childJobInterface.(*models.CrawlJob); ok {
			if childJob.Status == models.JobStatusPending {
				childJob.Status = models.JobStatusRunning
				childJob.StartedAt = time.Now()
				if saveErr := c.deps.JobStorage.SaveJob(ctx, childJob); saveErr != nil {
					c.logger.Warn().
						Err(saveErr).
						Str("child_id", msg.ID).
						Msg("Failed to update child job status to running")
				} else {
					c.logger.Debug().
						Str("child_id", msg.ID).
						Str("parent_id", msg.ParentID).
						Msg("Child job status updated to running")
				}
			}
		} else {
			c.logger.Warn().
				Str("child_id", msg.ID).
				Str("parent_id", msg.ParentID).
				Msg("Child job type assertion failed - expected *models.CrawlJob")
		}
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

	// Publish child-level started event for this URL processing
	if c.deps.EventService != nil {
		// Try to enrich with current progress stats if available
		var childCount, completedChildren, failedChildren int
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				childCount = job.Progress.TotalURLs
				completedChildren = job.Progress.CompletedURLs
				failedChildren = job.Progress.FailedURLs
			}
		}
		startedChild := interfaces.Event{
			Type: interfaces.EventJobStarted,
			Payload: map[string]interface{}{
				"job_id":             msg.ID,
				"child_job_id":       msg.ID,
				"parent_job_id":      msg.ParentID,
				"status":             "running",
				"source_type":        sourceType,
				"entity_type":        entityType,
				"url":                msg.URL,
				"depth":              msg.Depth,
				"child_count":        childCount,
				"completed_children": completedChildren,
				"failed_children":    failedChildren,
				"timestamp":          time.Now().Format(time.RFC3339),
			},
		}
		if err := c.deps.EventService.Publish(ctx, startedChild); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to publish child job started event")
		}
	}

	// Scrape the URL (handles JavaScript rendering, HTML parsing, markdown conversion)
	scrapeResult, err := scraper.ScrapeURL(ctx, msg.URL)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("url", msg.URL).
			Int("depth", msg.Depth).
			Msg("Failed to scrape URL")

			// Publish EventJobFailed for critical scraping failure (child-level)
		if c.deps.EventService != nil {
			failedEvent := interfaces.Event{
				Type: interfaces.EventJobFailed,
				Payload: map[string]interface{}{
					"job_id":        msg.ID,
					"child_job_id":  msg.ID,
					"parent_job_id": msg.ParentID,
					"status":        "failed",
					"source_type":   sourceType,
					"entity_type":   entityType,
					"error":         err.Error(),
					"timestamp":     time.Now().Format(time.RFC3339),
				},
			}
			if pubErr := c.deps.EventService.Publish(ctx, failedEvent); pubErr != nil {
				c.logger.Warn().Err(pubErr).Msg("Failed to publish job failed event")
			}
		}

		// Update parent job error field
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				job.Error = formatJobError("Scraping", err)
				// Note: Don't change status here - individual URL failures don't fail the entire job
				// Only update the Error field to show the most recent failure
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to save job with scraping error")
				}
			}
		}

		// Update parent job progress even on failure
		// Update heartbeat to track last activity
		if err := c.deps.JobStorage.UpdateJobHeartbeat(ctx, msg.ParentID); err != nil {
			c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to update job heartbeat on error")
		}

		// Perform atomic progress update for failed URL
		// completedDelta: +1 (URL processed but failed)
		// pendingDelta: -1 (URL no longer pending)
		// totalDelta: 0 (no new children)
		// failedDelta: +1 (one failure)
		if atomicErr := c.deps.JobStorage.UpdateProgressCountersAtomic(ctx, msg.ParentID, 1, -1, 0, 1); atomicErr != nil {
			c.logger.Warn().Err(atomicErr).Str("parent_id", msg.ParentID).Msg("Failed to atomically update progress after scraping failure")
		}

		// Load job for error tolerance threshold check and completion probe (after atomic update)
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				// Check error tolerance threshold immediately after child failure
				// This provides prompt enforcement instead of waiting for completion probe
				if c.checkErrorToleranceThreshold(ctx, msg, job) {
					// Job was failed due to threshold, return early
					return fmt.Errorf("job failed due to error tolerance threshold")
				}

				// Check completion and enqueue probe if needed
				c.checkAndEnqueueCompletionProbe(ctx, job, msg)
			}
		}

		// Update child job status to failed if this is a child job (scraping error)
		// Guard: Verify message ID matches expected child job pattern (contains "-child-" or "-seed-")
		if msg.ParentID != "" && (strings.Contains(msg.ID, "-child-") || strings.Contains(msg.ID, "-seed-")) {
			childJobInterface, childErr := c.deps.JobStorage.GetJob(ctx, msg.ID)
			if childErr != nil {
				// Child job row missing - this can happen during upgrade or if persistence failed
				c.logger.Warn().
					Err(childErr).
					Str("child_id", msg.ID).
					Str("parent_id", msg.ParentID).
					Msg("Child job row not found in database (upgrade or persistence failure) - continuing without status update")
			} else if childJob, ok := childJobInterface.(*models.CrawlJob); ok {
				childJob.Status = models.JobStatusFailed
				childJob.Error = formatJobError("Scraping", err)
				childJob.CompletedAt = time.Now()
				childJob.FailedCount = 1
				if saveErr := c.deps.JobStorage.SaveJob(ctx, childJob); saveErr != nil {
					c.logger.Warn().
						Err(saveErr).
						Str("child_id", msg.ID).
						Msg("Failed to update child job status to failed (scraping error)")
				} else {
					c.logger.Debug().
						Str("child_id", msg.ID).
						Str("parent_id", msg.ParentID).
						Str("error", childJob.Error).
						Msg("Child job status updated to failed (scraping error)")
				}
			} else {
				c.logger.Warn().
					Str("child_id", msg.ID).
					Str("parent_id", msg.ParentID).
					Msg("Child job type assertion failed - expected *models.CrawlJob")
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
		// Update heartbeat to track last activity
		if err := c.deps.JobStorage.UpdateJobHeartbeat(ctx, msg.ParentID); err != nil {
			c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to update job heartbeat on error")
		}

		// Perform atomic progress update for failed URL
		// completedDelta: +1 (URL processed but failed)
		// pendingDelta: -1 (URL no longer pending)
		// totalDelta: 0 (no new children)
		// failedDelta: +1 (one failure)
		if atomicErr := c.deps.JobStorage.UpdateProgressCountersAtomic(ctx, msg.ParentID, 1, -1, 0, 1); atomicErr != nil {
			c.logger.Warn().Err(atomicErr).Str("parent_id", msg.ParentID).Msg("Failed to atomically update progress after non-success status")
		}

		// Load job for error tolerance threshold check and completion probe (after atomic update)
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				// Check error tolerance threshold immediately after child failure
				// This provides prompt enforcement instead of waiting for completion probe
				if c.checkErrorToleranceThreshold(ctx, msg, job) {
					// Job was failed due to threshold, return early
					return fmt.Errorf("job failed due to error tolerance threshold")
				}

				// Check completion and enqueue probe if needed
				c.checkAndEnqueueCompletionProbe(ctx, job, msg)
			}
		}

		// Compute HTTP error message with fallback to status text
		msgText := scrapeResult.Error
		if msgText == "" {
			msgText = http.StatusText(scrapeResult.StatusCode)
		}
		httpErrorMsg := fmt.Sprintf("HTTP %d: %s", scrapeResult.StatusCode, msgText)

		// Publish EventJobFailed for non-success HTTP status (child-level)
		if c.deps.EventService != nil {
			failedEvent := interfaces.Event{
				Type: interfaces.EventJobFailed,
				Payload: map[string]interface{}{
					"job_id":        msg.ID,
					"child_job_id":  msg.ID,
					"parent_job_id": msg.ParentID,
					"status":        "failed",
					"source_type":   sourceType,
					"entity_type":   entityType,
					"error":         httpErrorMsg,
					"timestamp":     time.Now().Format(time.RFC3339),
				},
			}
			if pubErr := c.deps.EventService.Publish(ctx, failedEvent); pubErr != nil {
				c.logger.Warn().Err(pubErr).Msg("Failed to publish job failed event")
			}
		}

		// Update parent job error field with HTTP error
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				job.Error = httpErrorMsg
				// Note: Don't change status here - individual URL failures don't fail the entire job
				// Only update the Error field to show the most recent failure
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to save job with HTTP error")
				}
			}
		}

		// Update child job status to failed if this is a child job (HTTP error)
		// Guard: Verify message ID matches expected child job pattern (contains "-child-" or "-seed-")
		if msg.ParentID != "" && (strings.Contains(msg.ID, "-child-") || strings.Contains(msg.ID, "-seed-")) {
			childJobInterface, childErr := c.deps.JobStorage.GetJob(ctx, msg.ID)
			if childErr != nil {
				// Child job row missing - this can happen during upgrade or if persistence failed
				c.logger.Warn().
					Err(childErr).
					Str("child_id", msg.ID).
					Str("parent_id", msg.ParentID).
					Msg("Child job row not found in database (upgrade or persistence failure) - continuing without status update")
			} else if childJob, ok := childJobInterface.(*models.CrawlJob); ok {
				childJob.Status = models.JobStatusFailed
				childJob.Error = httpErrorMsg
				childJob.CompletedAt = time.Now()
				childJob.FailedCount = 1
				if saveErr := c.deps.JobStorage.SaveJob(ctx, childJob); saveErr != nil {
					c.logger.Warn().
						Err(saveErr).
						Str("child_id", msg.ID).
						Msg("Failed to update child job status to failed (HTTP error)")
				} else {
					c.logger.Debug().
						Str("child_id", msg.ID).
						Str("parent_id", msg.ParentID).
						Str("error", childJob.Error).
						Msg("Child job status updated to failed (HTTP error)")
				}
			} else {
				c.logger.Warn().
					Str("child_id", msg.ID).
					Str("parent_id", msg.ParentID).
					Msg("Child job type assertion failed - expected *models.CrawlJob")
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

		// Update parent job error field with storage error
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				job.Error = formatJobError("Storage", err)
				// Note: Don't change status here - storage errors might be transient
				// Only update the Error field to show the most recent failure
				if saveErr := c.deps.JobStorage.SaveJob(ctx, job); saveErr != nil {
					c.logger.Warn().Err(saveErr).Msg("Failed to save job with storage error")
				} else {
					c.logger.Info().
						Str("parent_id", msg.ParentID).
						Str("error", job.Error).
						Msg("Job error field updated with storage failure")
				}
			}
		}

		return fmt.Errorf("failed to save document: %w", err)
	}

	c.logger.Info().
		Str("url", msg.URL).
		Str("document_id", document.ID).
		Msg("Document saved successfully")

	// Update child job status to completed if this is a child job
	// Guard: Verify message ID matches expected child job pattern (contains "-child-" or "-seed-")
	if msg.ParentID != "" && (strings.Contains(msg.ID, "-child-") || strings.Contains(msg.ID, "-seed-")) {
		childJobInterface, childErr := c.deps.JobStorage.GetJob(ctx, msg.ID)
		if childErr != nil {
			// Child job row missing - this can happen during upgrade or if persistence failed
			c.logger.Warn().
				Err(childErr).
				Str("child_id", msg.ID).
				Str("parent_id", msg.ParentID).
				Msg("Child job row not found in database (upgrade or persistence failure) - continuing without status update")
		} else if childJob, ok := childJobInterface.(*models.CrawlJob); ok {
			childJob.Status = models.JobStatusCompleted
			childJob.CompletedAt = time.Now()
			childJob.ResultCount = 1 // Successfully processed this URL
			if saveErr := c.deps.JobStorage.SaveJob(ctx, childJob); saveErr != nil {
				c.logger.Warn().
					Err(saveErr).
					Str("child_id", msg.ID).
					Msg("Failed to update child job status to completed")
			} else {
				c.logger.Debug().
					Str("child_id", msg.ID).
					Str("parent_id", msg.ParentID).
					Msg("Child job status updated to completed")
			}
		} else {
			c.logger.Warn().
				Str("child_id", msg.ID).
				Str("parent_id", msg.ParentID).
				Msg("Child job type assertion failed - expected *models.CrawlJob")
		}
	}

	// Track how many children will be spawned (needed for accurate progress tracking)
	var childrenToSpawn int

	// Discover and enqueue child links if follow_links is enabled
	if followLinks && msg.Depth < maxDepth {
		c.logger.Debug().
			Str("url", msg.URL).
			Int("depth", msg.Depth).
			Int("max_depth", maxDepth).
			Msg("Link discovery enabled, extracting and enqueueing child URLs")

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

		// Use shared LinkFilter helper for URL filtering
		linkFilter := crawler.NewLinkFilter(includePatterns, excludePatterns, sourceType, c.logger)

		// Filter and enqueue child jobs for discovered links
		enqueuedCount := 0
		filteredCount := 0
		duplicateCount := 0

		for _, childURL := range discoveredLinks {
			// Check for duplicate URLs using database-backed deduplication
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

			// Persist child job to database after URL marked as seen using helper method
			childJobID := fmt.Sprintf("%s-child-%d", msg.ID, enqueuedCount)

			// Build CrawlConfig for child job (inherit from parent)
			childConfig := models.CrawlConfig{
				MaxDepth:        maxDepth,
				FollowLinks:     followLinks,
				IncludePatterns: includePatterns,
				ExcludePatterns: excludePatterns,
			}

			// Use CreateChildJobRecord helper to persist child job consistently
			if err := c.CreateChildJobRecord(ctx, msg.ParentID, childJobID, childURL, sourceType, entityType, childConfig); err != nil {
				c.logger.Warn().
					Err(err).
					Str("child_id", childJobID).
					Str("child_url", childURL).
					Msg("Failed to persist child job to database via helper, continuing with enqueue")
				// Continue on save error - don't block enqueueing
			}

			// Respect max_pages limit if configured
			if maxPages > 0 && enqueuedCount >= maxPages {
				c.logger.Info().
					Int("enqueued", enqueuedCount).
					Int("max_pages", maxPages).
					Msg("Reached max_pages limit, stopping link enqueueing")
				break
			}

			// Apply URL filtering
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

			// Create child job message (flat hierarchy - all children point to root job)
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
						"timestamp":     time.Now().Format(time.RFC3339),
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

		// Track enqueued children for progress update
		childrenToSpawn = enqueuedCount
	}

	// Update heartbeat to track "last URL processed" timestamp
	if err := c.deps.JobStorage.UpdateJobHeartbeat(ctx, msg.ParentID); err != nil {
		c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to update job heartbeat")
	}

	// Update parent job progress AFTER determining how many children will be spawned
	// This ensures PendingURLs is accurate and prevents premature job completion
	jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
	if err != nil {
		c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to load parent job for progress update")
	} else {
		if job, ok := jobInterface.(*models.CrawlJob); ok {
			// Calculate deltas for atomic progress update:
			// - completedDelta: +1 (this URL completed successfully)
			// - pendingDelta: -1 (this URL no longer pending) + childrenToSpawn (new children added)
			// - totalDelta: +childrenToSpawn (new children added to total)
			// - failedDelta: 0 (no failures on success path)
			completedDelta := 1
			pendingDelta := -1 + childrenToSpawn
			totalDelta := childrenToSpawn
			failedDelta := 0

			// Determine if we should persist this update (batched saves every 10 URLs)
			// Check using pre-update value: will the NEW completed count be a multiple of 10?
			newCompletedURLs := job.Progress.CompletedURLs + completedDelta
			shouldSave := newCompletedURLs%10 == 0 || // Every 10th URL
				job.Status == models.JobStatusCompleted || // Always save on completion
				job.Status == models.JobStatusFailed // Always save on failure

			if shouldSave {
				// Perform atomic progress update (eliminates read-modify-write race)
				if err := c.deps.JobStorage.UpdateProgressCountersAtomic(ctx, msg.ParentID, completedDelta, pendingDelta, totalDelta, failedDelta); err != nil {
					c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to atomically update progress counters")
				} else {
					// Update in-memory job object to reflect changes (for completion probe check and logging)
					job.Progress.CompletedURLs += completedDelta
					job.Progress.PendingURLs += pendingDelta
					job.Progress.TotalURLs += totalDelta
					if job.Progress.TotalURLs > 0 {
						job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
					}

					c.logger.Debug().
						Str("parent_id", msg.ParentID).
						Int("completed", job.Progress.CompletedURLs).
						Int("pending", job.Progress.PendingURLs).
						Int("total", job.Progress.TotalURLs).
						Int("spawned_children", childrenToSpawn).
						Float64("percentage", job.Progress.Percentage).
						Str("status", string(job.Status)).
						Msg("Progress updated atomically with spawned children (batched)")
				}
			} else {
				// Skip update for non-10th URLs to reduce database writes
				// Update in-memory for accurate logging (note: not persisted until next batch)
				job.Progress.CompletedURLs += completedDelta
				job.Progress.PendingURLs += pendingDelta
				job.Progress.TotalURLs += totalDelta

				c.logger.Debug().
					Str("parent_id", msg.ParentID).
					Int("completed", job.Progress.CompletedURLs).
					Int("pending", job.Progress.PendingURLs).
					Msg("Progress update batched (will save at next 10-URL boundary)")
			}

			// Check completion and enqueue probe if needed (uses updated in-memory values)
			c.checkAndEnqueueCompletionProbe(ctx, job, msg)

			// Emit progress event (only if not completed)
			if job.Status != models.JobStatusCompleted {
				if err := c.LogJobEvent(ctx, msg.ParentID, "info",
					fmt.Sprintf("Progress: %d/%d URLs completed (%.1f%%)",
						job.Progress.CompletedURLs, job.Progress.TotalURLs, job.Progress.Percentage)); err != nil {
					c.logger.Warn().Err(err).Msg("Failed to log progress event")
				}
			}

			// Publish child-level completion event for this processed URL
			if c.deps.EventService != nil {
				completedChild := interfaces.Event{
					Type: interfaces.EventJobCompleted,
					Payload: map[string]interface{}{
						"job_id":             msg.ID,
						"child_job_id":       msg.ID,
						"parent_job_id":      msg.ParentID,
						"status":             "completed",
						"source_type":        sourceType,
						"entity_type":        entityType,
						"url":                msg.URL,
						"depth":              msg.Depth,
						"child_count":        job.Progress.TotalURLs,
						"completed_children": job.Progress.CompletedURLs,
						"failed_children":    job.Progress.FailedURLs,
						"timestamp":          time.Now().Format(time.RFC3339),
					},
				}
				if err := c.deps.EventService.Publish(ctx, completedChild); err != nil {
					c.logger.Warn().Err(err).Msg("Failed to publish child job completed event")
				}
			}
		} else {
			c.logger.Warn().Str("parent_id", msg.ParentID).Msg("Failed to type assert job to *crawler.CrawlJob")
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

// checkErrorToleranceThreshold checks if the parent job's error tolerance threshold is exceeded
// and takes action if configured (stop_all/continue/mark_warning).
// Returns true if the job was failed due to threshold (caller should not continue), false otherwise.
func (c *CrawlerJob) checkErrorToleranceThreshold(ctx context.Context, msg *queue.JobMessage, job *models.CrawlJob) bool {
	// Only check threshold for root jobs with error tolerance configured
	if job.ParentID != "" || msg.JobDefinitionID == "" || c.deps.JobDefinitionStorage == nil || c.deps.JobManager == nil {
		return false
	}

	// Load job definition to check error tolerance configuration
	jobDef, err := c.deps.JobDefinitionStorage.GetJobDefinition(ctx, msg.JobDefinitionID)
	if err != nil {
		c.logger.Warn().
			Err(err).
			Str("job_def_id", msg.JobDefinitionID).
			Msg("Failed to load job definition for error tolerance check")
		return false
	}

	if jobDef == nil || jobDef.ErrorTolerance == nil {
		return false
	}

	// Get current child failure statistics
	childStats, err := c.deps.JobStorage.GetJobChildStats(ctx, []string{msg.ParentID})
	if err != nil {
		c.logger.Warn().
			Err(err).
			Str("parent_id", msg.ParentID).
			Msg("Failed to get child stats for error tolerance check")
		return false
	}

	stats, ok := childStats[msg.ParentID]
	if !ok {
		return false
	}

	// Check if failure threshold is exceeded
	maxFailures := jobDef.ErrorTolerance.MaxChildFailures
	currentFailures := stats.FailedChildren

	if maxFailures == 0 || currentFailures < maxFailures {
		return false // Threshold not exceeded
	}

	c.logger.Warn().
		Str("parent_id", msg.ParentID).
		Int("failed_children", currentFailures).
		Int("max_failures", maxFailures).
		Str("failure_action", jobDef.ErrorTolerance.FailureAction).
		Msg("Error tolerance threshold exceeded during job execution")

	// Handle based on failure action
	switch jobDef.ErrorTolerance.FailureAction {
	case "stop_all":
		// Cancel all running child jobs immediately
		cancelledCount, err := c.deps.JobManager.StopAllChildJobs(ctx, msg.ParentID)
		if err != nil {
			c.logger.Error().
				Err(err).
				Str("parent_id", msg.ParentID).
				Msg("Failed to stop child jobs during threshold enforcement")
		} else {
			c.logger.Info().
				Str("parent_id", msg.ParentID).
				Int("cancelled_count", cancelledCount).
				Msg("Stopped all running child jobs due to error tolerance threshold (immediate enforcement)")
		}

		// Mark parent job as failed
		job.Status = models.JobStatusFailed
		job.Error = fmt.Sprintf("Error tolerance exceeded: %d/%d child jobs failed (max: %d)",
			currentFailures, stats.ChildCount, maxFailures)
		job.CompletedAt = time.Now()
		job.ResultCount = job.Progress.CompletedURLs
		job.FailedCount = job.Progress.FailedURLs

		if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
			c.logger.Error().
				Err(err).
				Str("parent_id", msg.ParentID).
				Msg("Failed to save job as failed due to error tolerance")
			return true // Still return true to stop processing
		}

		// Publish EventJobFailed for error tolerance threshold exceeded
		if c.deps.EventService != nil {
			// Get status_report from job (stats already available from line 1021)
			statusReport := job.GetStatusReport(stats)

			failedEvent := interfaces.Event{
				Type: interfaces.EventJobFailed,
				Payload: map[string]interface{}{
					"job_id":          msg.ParentID,
					"status":          "failed",
					"source_type":     job.SourceType,
					"entity_type":     job.EntityType,
					"error":           job.Error,
					"result_count":    job.ResultCount,
					"failed_count":    job.FailedCount,
					"child_count":     stats.ChildCount,
					"failed_children": currentFailures,
					"error_tolerance": maxFailures,
					"timestamp":       time.Now().Format(time.RFC3339),
					// Add status_report fields
					"progress_text":    statusReport.ProgressText,
					"errors":           statusReport.Errors,
					"warnings":         statusReport.Warnings,
					"running_children": statusReport.RunningChildren,
				},
			}
			if pubErr := c.deps.EventService.Publish(ctx, failedEvent); pubErr != nil {
				c.logger.Warn().Err(pubErr).Msg("Failed to publish job failed event for threshold")
			}
		}

		c.logger.Info().
			Str("parent_id", msg.ParentID).
			Str("error", job.Error).
			Msg("Job marked as failed due to error tolerance threshold (stop_all, immediate enforcement)")

		return true // Job failed, stop processing

	case "continue":
		// Log warning but continue processing
		c.logger.Warn().
			Str("parent_id", msg.ParentID).
			Int("failed_children", currentFailures).
			Int("max_failures", maxFailures).
			Msg("Error tolerance threshold exceeded but continuing (action: continue)")
		return false

	case "mark_warning":
		// Set warning in job.Error field but continue
		warningMsg := fmt.Sprintf("Warning: %d/%d child jobs failed (threshold: %d)",
			currentFailures, stats.ChildCount, maxFailures)
		if job.Error == "" {
			job.Error = warningMsg
		} else {
			job.Error = fmt.Sprintf("%s; %s", job.Error, warningMsg)
		}

		if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
			c.logger.Warn().
				Err(err).
				Str("parent_id", msg.ParentID).
				Msg("Failed to save job with warning")
		}

		c.logger.Warn().
			Str("parent_id", msg.ParentID).
			Str("warning", warningMsg).
			Msg("Error tolerance threshold exceeded, marked as warning (action: mark_warning)")
		return false
	}

	return false
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

// checkAndEnqueueCompletionProbe checks if the job is eligible for completion and enqueues a delayed probe
// This is called after each URL is processed (success or failure) to detect job completion
func (c *CrawlerJob) checkAndEnqueueCompletionProbe(ctx context.Context, job *models.CrawlJob, msg *queue.JobMessage) {
	// Completion detection with delayed probe mechanism (Comment 3 & 8)
	// When PendingURLs reaches 0, enqueue a delayed probe to verify completion after grace period
	isCompletionCandidate := job.Progress.PendingURLs == 0 && job.Progress.TotalURLs > 0

	if isCompletionCandidate && job.Status != models.JobStatusCompleted {
		// Enqueue a delayed completion probe message (5 second grace period)
		// This allows time for any in-flight URL processing to update the heartbeat
		probeMsg := &queue.JobMessage{
			ID:              fmt.Sprintf("%s-completion-probe-%d", msg.ParentID, time.Now().Unix()),
			Type:            "crawler_completion_probe",
			ParentID:        msg.ParentID,
			JobDefinitionID: msg.JobDefinitionID,
			Config:          msg.Config,
		}

		if err := c.deps.QueueManager.EnqueueWithDelay(ctx, probeMsg, 5*time.Second); err != nil {
			c.logger.Warn().
				Err(err).
				Str("parent_id", msg.ParentID).
				Msg("Failed to enqueue completion probe - completion may be delayed")
		} else {
			c.logger.Info().
				Str("parent_id", msg.ParentID).
				Int("completed", job.Progress.CompletedURLs).
				Int("total", job.Progress.TotalURLs).
				Msg("Enqueued completion probe with 5s grace period")
		}
	}
}

// ExecuteCompletionProbe handles delayed completion verification (Comment 3 & 8)
// This is called after a 5-second grace period to verify job completion
func (c *CrawlerJob) ExecuteCompletionProbe(ctx context.Context, msg *queue.JobMessage) error {
	c.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Msg("Processing completion probe")

	// Load the current job state
	jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
	if err != nil {
		c.logger.Error().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to load job for completion probe")
		return fmt.Errorf("failed to load job: %w", err)
	}

	job, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		return fmt.Errorf("invalid job type: expected *models.CrawlJob")
	}

	// ERROR TOLERANCE THRESHOLD CHECKING
	// Check if this is a root job with error tolerance configured
	if job.ParentID == "" && msg.JobDefinitionID != "" && c.deps.JobDefinitionStorage != nil && c.deps.JobManager != nil {
		// Load job definition to check error tolerance configuration
		jobDef, err := c.deps.JobDefinitionStorage.GetJobDefinition(ctx, msg.JobDefinitionID)
		if err != nil {
			c.logger.Warn().
				Err(err).
				Str("job_def_id", msg.JobDefinitionID).
				Msg("Failed to load job definition for error tolerance check")
		} else if jobDef != nil && jobDef.ErrorTolerance != nil {
			// Get child job failure statistics
			childStats, err := c.deps.JobStorage.GetJobChildStats(ctx, []string{msg.ParentID})
			if err != nil {
				c.logger.Warn().
					Err(err).
					Str("parent_id", msg.ParentID).
					Msg("Failed to get child stats for error tolerance check")
			} else if stats, ok := childStats[msg.ParentID]; ok {
				// Check if failure threshold is exceeded
				maxFailures := jobDef.ErrorTolerance.MaxChildFailures
				currentFailures := stats.FailedChildren

				if maxFailures > 0 && currentFailures >= maxFailures {
					c.logger.Warn().
						Str("parent_id", msg.ParentID).
						Int("failed_children", currentFailures).
						Int("max_failures", maxFailures).
						Str("failure_action", jobDef.ErrorTolerance.FailureAction).
						Msg("Error tolerance threshold exceeded")

					// Handle based on failure action
					switch jobDef.ErrorTolerance.FailureAction {
					case "stop_all":
						// Cancel all running child jobs
						cancelledCount, err := c.deps.JobManager.StopAllChildJobs(ctx, msg.ParentID)
						if err != nil {
							c.logger.Error().
								Err(err).
								Str("parent_id", msg.ParentID).
								Msg("Failed to stop child jobs")
						} else {
							c.logger.Info().
								Str("parent_id", msg.ParentID).
								Int("cancelled_count", cancelledCount).
								Msg("Stopped all running child jobs due to error tolerance threshold")
						}

						// Mark parent job as failed
						job.Status = models.JobStatusFailed
						job.Error = fmt.Sprintf("Error tolerance exceeded: %d/%d child jobs failed (max: %d)",
							currentFailures, stats.ChildCount, maxFailures)
						job.CompletedAt = time.Now()
						job.ResultCount = job.Progress.CompletedURLs
						job.FailedCount = job.Progress.FailedURLs

						if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
							c.logger.Error().
								Err(err).
								Str("parent_id", msg.ParentID).
								Msg("Failed to save job as failed due to error tolerance")
							return fmt.Errorf("failed to save job: %w", err)
						}

						// Publish EventJobFailed for error tolerance threshold exceeded
						if c.deps.EventService != nil {
							// Get status_report from job (stats already available from line 1238)
							statusReport := job.GetStatusReport(stats)

							failedEvent := interfaces.Event{
								Type: interfaces.EventJobFailed,
								Payload: map[string]interface{}{
									"job_id":          msg.ParentID,
									"status":          "failed",
									"source_type":     job.SourceType,
									"entity_type":     job.EntityType,
									"error":           job.Error,
									"result_count":    job.ResultCount,
									"failed_count":    job.FailedCount,
									"child_count":     stats.ChildCount,
									"failed_children": currentFailures,
									"error_tolerance": maxFailures,
									"timestamp":       time.Now().Format(time.RFC3339),
									// Add status_report fields
									"progress_text":    statusReport.ProgressText,
									"errors":           statusReport.Errors,
									"warnings":         statusReport.Warnings,
									"running_children": statusReport.RunningChildren,
								},
							}
							if pubErr := c.deps.EventService.Publish(ctx, failedEvent); pubErr != nil {
								c.logger.Warn().Err(pubErr).Msg("Failed to publish job failed event")
							}
						}

						c.logger.Info().
							Str("parent_id", msg.ParentID).
							Str("error", job.Error).
							Msg("Job marked as failed due to error tolerance threshold (stop_all)")

						return nil // Job failed due to threshold, don't continue completion check

					case "continue":
						// Log warning but continue processing
						c.logger.Warn().
							Str("parent_id", msg.ParentID).
							Int("failed_children", currentFailures).
							Int("max_failures", maxFailures).
							Msg("Error tolerance threshold exceeded but continuing (action: continue)")

					case "mark_warning":
						// Set warning in job.Error field but continue
						warningMsg := fmt.Sprintf("Warning: %d/%d child jobs failed (threshold: %d)",
							currentFailures, stats.ChildCount, maxFailures)
						if job.Error == "" {
							job.Error = warningMsg
						} else {
							job.Error = fmt.Sprintf("%s; %s", job.Error, warningMsg)
						}

						c.logger.Warn().
							Str("parent_id", msg.ParentID).
							Str("warning", warningMsg).
							Msg("Error tolerance threshold exceeded, marked as warning (action: mark_warning)")
					}
				}
			}
		}
	}

	// Check completion conditions:
	// 1. PendingURLs must still be 0
	// 2. LastHeartbeat must be older than 5 seconds (indicates no recent activity)
	// 3. Status must not already be completed
	const gracePeriod = 5 * time.Second

	if job.Status == models.JobStatusCompleted {
		c.logger.Debug().Str("parent_id", msg.ParentID).Msg("Job already completed, skipping probe")
		return nil
	}

	// Check if heartbeat is old enough (no activity during grace period)
	timeSinceHeartbeat := time.Since(job.LastHeartbeat)

	// Stale job detection: If job has been idle for too long AND still has pending URLs,
	// mark it as failed (indicates stuck/crashed workers or queue issues)
	const staleThreshold = 10 * time.Minute
	if timeSinceHeartbeat > staleThreshold && job.Progress.PendingURLs > 0 {
		idleDuration := timeSinceHeartbeat
		job.Error = fmt.Sprintf("Timeout: No activity for %s (pending: %d URLs)", idleDuration.Round(time.Second), job.Progress.PendingURLs)
		job.Status = models.JobStatusFailed
		job.CompletedAt = time.Now()
		job.ResultCount = job.Progress.CompletedURLs
		job.FailedCount = job.Progress.FailedURLs

		if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
			c.logger.Error().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to save stale job as failed")
			return fmt.Errorf("failed to save stale job: %w", err)
		}

		// Publish EventJobFailed for stale job timeout
		if c.deps.EventService != nil {
			// Fetch child statistics and generate status_report for parent job
			childStats, statErr := c.deps.JobStorage.GetJobChildStats(ctx, []string{msg.ParentID})
			if statErr != nil {
				c.logger.Warn().Err(statErr).Str("parent_id", msg.ParentID).Msg("Failed to get child stats for stale job event")
			}
			var stats *interfaces.JobChildStats
			if statsData, ok := childStats[msg.ParentID]; ok {
				stats = statsData
			}

			// Get status_report from job
			statusReport := job.GetStatusReport(stats)

			failedEvent := interfaces.Event{
				Type: interfaces.EventJobFailed,
				Payload: map[string]interface{}{
					"job_id":       msg.ParentID,
					"status":       "failed",
					"source_type":  job.SourceType,
					"entity_type":  job.EntityType,
					"error":        job.Error,
					"result_count": job.ResultCount,
					"failed_count": job.FailedCount,
					"timestamp":    time.Now().Format(time.RFC3339),
					// Add status_report fields
					"progress_text":    statusReport.ProgressText,
					"errors":           statusReport.Errors,
					"warnings":         statusReport.Warnings,
					"running_children": statusReport.RunningChildren,
				},
			}
			if pubErr := c.deps.EventService.Publish(ctx, failedEvent); pubErr != nil {
				c.logger.Warn().Err(pubErr).Msg("Failed to publish stale job failed event")
			}
		}

		c.logger.Warn().
			Str("parent_id", msg.ParentID).
			Dur("idle_duration", idleDuration).
			Int("pending_urls", job.Progress.PendingURLs).
			Msg("Job marked as failed due to inactivity")

		return nil
	}

	if job.Progress.PendingURLs > 0 {
		c.logger.Info().
			Str("parent_id", msg.ParentID).
			Int("pending_urls", job.Progress.PendingURLs).
			Msg("Job no longer eligible for completion - new URLs appeared during grace period")
		return nil
	}

	// Check if heartbeat is recent (indicates ongoing activity)
	if timeSinceHeartbeat < gracePeriod {
		c.logger.Info().
			Str("parent_id", msg.ParentID).
			Dur("time_since_heartbeat", timeSinceHeartbeat).
			Dur("grace_period", gracePeriod).
			Msg("Job has recent activity - enqueuing another completion probe")

		// Heartbeat is too recent - enqueue another probe for later
		retryProbeMsg := &queue.JobMessage{
			ID:              fmt.Sprintf("%s-completion-probe-retry-%d", msg.ParentID, time.Now().Unix()),
			Type:            "crawler_completion_probe",
			ParentID:        msg.ParentID,
			JobDefinitionID: msg.JobDefinitionID,
			Config:          msg.Config,
		}

		remainingWait := gracePeriod - timeSinceHeartbeat + (1 * time.Second) // Add 1s buffer
		if err := c.deps.QueueManager.EnqueueWithDelay(ctx, retryProbeMsg, remainingWait); err != nil {
			c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to enqueue retry probe")
		}

		return nil
	}

	// All conditions met - mark job as completed (only if not already failed)
	job.ResultCount = job.Progress.CompletedURLs
	job.FailedCount = job.Progress.FailedURLs
	job.Status = models.JobStatusCompleted
	job.CompletedAt = time.Now()

	if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
		c.logger.Error().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to save completed job")
		return fmt.Errorf("failed to save completed job: %w", err)
	}

	// Publish EventJobCompleted after successful job completion
	if c.deps.EventService != nil {
		// Calculate duration: use StartedAt if available (for accurate processing time), otherwise CreatedAt
		duration := job.CompletedAt.Sub(job.CreatedAt) // Default to full lifetime
		if !job.StartedAt.IsZero() {
			duration = job.CompletedAt.Sub(job.StartedAt) // Prefer actual processing time
		}

		// Fetch child statistics and generate status_report for parent job
		childStats, err := c.deps.JobStorage.GetJobChildStats(ctx, []string{msg.ParentID})
		if err != nil {
			c.logger.Warn().Err(err).Str("parent_id", msg.ParentID).Msg("Failed to get child stats for status_report")
		}
		var stats *interfaces.JobChildStats
		if statsData, ok := childStats[msg.ParentID]; ok {
			stats = statsData
		}

		// Get status_report from job
		statusReport := job.GetStatusReport(stats)

		completedEvent := interfaces.Event{
			Type: interfaces.EventJobCompleted,
			Payload: map[string]interface{}{
				"job_id":           msg.ParentID,
				"status":           "completed",
				"source_type":      job.SourceType,
				"entity_type":      job.EntityType,
				"result_count":     job.ResultCount,
				"failed_count":     job.FailedCount,
				"total_urls":       job.Progress.TotalURLs,
				"duration_seconds": duration.Seconds(),
				"timestamp":        time.Now(),
				// Add status_report fields
				"progress_text":    statusReport.ProgressText,
				"errors":           statusReport.Errors,
				"warnings":         statusReport.Warnings,
				"running_children": statusReport.RunningChildren,
			},
		}
		if err := c.deps.EventService.Publish(ctx, completedEvent); err != nil {
			c.logger.Warn().Err(err).Msg("Failed to publish job completed event")
		}
	}

	c.logger.Info().
		Str("parent_id", msg.ParentID).
		Int("total_urls", job.Progress.TotalURLs).
		Int("completed_urls", job.Progress.CompletedURLs).
		Int("failed_urls", job.Progress.FailedURLs).
		Dur("time_since_last_heartbeat", timeSinceHeartbeat).
		Msg("Job marked as completed after grace period verification")

	// Enqueue post-summarization job after completion
	postSummaryMsgID := fmt.Sprintf("%s-post-summary", msg.ParentID)
	postSummaryConfig := map[string]interface{}{
		"source_type":   job.SourceType,
		"entity_type":   job.EntityType,
		"parent_job_id": msg.ParentID,
	}

	postSummaryMsg := &queue.JobMessage{
		ID:              postSummaryMsgID,
		Type:            "post_summarization",
		ParentID:        msg.ParentID,
		JobDefinitionID: msg.JobDefinitionID,
		Config:          postSummaryConfig,
	}

	c.logger.Info().
		Str("message_id", postSummaryMsgID).
		Str("parent_id", msg.ParentID).
		Msg("Enqueueing post-summarization job")

	if err := c.deps.QueueManager.Enqueue(ctx, postSummaryMsg); err != nil {
		c.logger.Warn().
			Err(err).
			Str("message_id", postSummaryMsgID).
			Msg("Failed to enqueue post-summarization job - completion not affected")
	} else {
		c.logger.Info().
			Str("message_id", postSummaryMsgID).
			Msg("Post-summarization job enqueued")

		// Create post-summarization CrawlJob record
		postSummaryJob := &models.CrawlJob{
			ID:         postSummaryMsgID,
			ParentID:   msg.ParentID,
			JobType:    models.JobTypePostSummary,
			Name:       "Post-summarization",
			SourceType: job.SourceType,
			EntityType: job.EntityType,
			Status:     models.JobStatusPending,
			CreatedAt:  time.Now(),
		}

		if err := c.deps.JobStorage.SaveJob(ctx, postSummaryJob); err != nil {
			c.logger.Warn().
				Err(err).
				Str("post_summary_job_id", postSummaryMsgID).
				Msg("Failed to persist post-summarization job to database, continuing")
		} else {
			c.logger.Debug().
				Str("post_summary_job_id", postSummaryMsgID).
				Msg("Post-summarization job persisted to database")
		}
	}

	// Log job completion event
	if err := c.LogJobEvent(ctx, msg.ParentID, "info",
		fmt.Sprintf("Job completed: %d/%d URLs processed (%d failed)",
			job.Progress.CompletedURLs, job.Progress.TotalURLs, job.Progress.FailedURLs)); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to log job completion event")
	}

	return nil
}
