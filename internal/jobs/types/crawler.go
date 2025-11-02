package types

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// CrawlerJobDeps holds minimal dependencies for crawler jobs
type CrawlerJobDeps struct {
	DocumentStorage interfaces.DocumentStorage
	JobStorage      interfaces.JobStorage
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
//
// Categories:
//   - Validation: Invalid input (e.g., "Validation: URL is required")
//   - Network: Connection issues (e.g., "Network: Connection refused for https://...")
//   - HTTP: HTTP status errors (e.g., "HTTP 404: Not Found for https://...")
//   - Timeout: Request timeouts (e.g., "Timeout: Request timeout for https://...")
//   - Scraping: Content extraction errors (e.g., "Scraping: Failed to parse HTML")
//   - Storage: Database errors (e.g., "Storage: Database locked")
//   - System: Internal errors (e.g., "System: Parent job is not a CrawlJob")
//
// Error Detection:
//   - Checks error type (context.DeadlineExceeded, net.OpError)
//   - Checks error message for keywords (404, timeout, connection refused)
//   - Extracts HTTP status codes from error messages
//   - Truncates long error messages to 200 characters
//
// URL Context:
//   - If url parameter is provided, includes it in the error message
//   - Format: "Category: Description for https://example.com/page1"
//   - If url is empty, omits URL context
//
// Usage:
//   err := fetchURL("https://example.com")
//   if err != nil {
//       errorMsg := formatJobError("Network", err, "https://example.com", timeout)
//       // errorMsg = "Network: Connection refused for https://example.com"
//       jobStorage.UpdateJobStatus(ctx, jobID, "failed", errorMsg)
//   }
//
// This format is displayed in the UI and should be actionable for users.
// See crawler_job.go lines 65-69 for Error field documentation.
func formatJobError(category string, err error, url string, timeout time.Duration) string {
	if err == nil {
		return ""
	}

	// Check for timeout errors
	if errors.Is(err, context.DeadlineExceeded) {
		if url != "" {
			if timeout > 0 {
				return fmt.Sprintf("Timeout: Request exceeded %v for %s", timeout, url)
			}
			return fmt.Sprintf("Timeout: Request exceeded deadline for %s", url)
		}
		if timeout > 0 {
			return fmt.Sprintf("Timeout: Request exceeded %v", timeout)
		}
		return "Timeout: Request exceeded deadline"
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Check for HTTP status codes (404, 500, etc.)
	if strings.Contains(errMsgLower, "404") || strings.Contains(errMsgLower, "not found") {
		if url != "" {
			return fmt.Sprintf("HTTP 404: Not Found for %s", url)
		}
		return "HTTP 404: Not Found"
	}
	if strings.Contains(errMsgLower, "401") || strings.Contains(errMsgLower, "unauthorized") {
		if url != "" {
			return fmt.Sprintf("HTTP 401: Unauthorized for %s", url)
		}
		return "HTTP 401: Unauthorized"
	}
	if strings.Contains(errMsgLower, "403") || strings.Contains(errMsgLower, "forbidden") {
		if url != "" {
			return fmt.Sprintf("HTTP 403: Forbidden for %s", url)
		}
		return "HTTP 403: Forbidden"
	}
	if strings.Contains(errMsgLower, "500") || strings.Contains(errMsgLower, "server error") {
		if url != "" {
			return fmt.Sprintf("HTTP 500: Server Error for %s", url)
		}
		return "HTTP 500: Server Error"
	}

	// Check for timeout in error message
	if strings.Contains(errMsgLower, "timeout") || strings.Contains(errMsgLower, "deadline exceeded") {
		// Use configured timeout if available
		timeoutDuration := ""
		if timeout > 0 {
			timeoutDuration = timeout.String()
		} else {
			// Try to extract timeout duration from error message
			// Look for patterns like "10s", "5ms", "2m" anywhere in the message
			parts := strings.Split(errMsg, ":")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				// Check for duration pattern
				if strings.HasSuffix(part, "s") || strings.HasSuffix(part, "ms") || strings.HasSuffix(part, "m") {
					timeoutDuration = part
					break
				}
			}

			// If not found in parts, try to find in the original message
			if timeoutDuration == "" {
				// Try to extract duration using simple pattern matching
				words := strings.Fields(errMsg)
				for _, word := range words {
					word = strings.Trim(word, ".,:;")
					if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "ms") || strings.HasSuffix(word, "m") {
						timeoutDuration = word
						break
					}
				}
			}
		}

		timeoutStr := ""
		if timeoutDuration != "" {
			timeoutStr = fmt.Sprintf(" (%s)", timeoutDuration)
		}

		if category == "Scraping" {
			if url != "" {
				return fmt.Sprintf("Timeout: Scraping timeout%s for %s", timeoutStr, url)
			}
			return fmt.Sprintf("Timeout: Scraping timeout%s", timeoutStr)
		}
		if url != "" {
			return fmt.Sprintf("Timeout: Request timeout%s for %s", timeoutStr, url)
		}
		return fmt.Sprintf("Timeout: Request timeout%s", timeoutStr)
	}

	// Check for network errors
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		// Extract brief cause from network error
		briefCause := errMsg
		if len(briefCause) > 100 {
			// Try to extract the most relevant part
			if idx := strings.LastIndex(briefCause, ":"); idx != -1 && idx < len(briefCause)-1 {
				briefCause = strings.TrimSpace(briefCause[idx+1:])
			}
		}
		if url != "" {
			return fmt.Sprintf("Network: %s for %s", briefCause, url)
		}
		return fmt.Sprintf("Network: %s", briefCause)
	}

	// Truncate long error messages
	// Account for category prefix and URL suffix to keep total message reasonable
	const maxLength = 200
	if url != "" {
		// Account for " (URL: <url>" suffix - rough estimate
		maxAllowedErrLen := maxLength - len(category) - len(" (URL: )")
		if len(errMsg) > maxAllowedErrLen {
			errMsg = errMsg[:maxAllowedErrLen-3] + "..." // -3 for "..."
		}
	} else {
		// Just account for ": " prefix
		maxAllowedErrLen := maxLength - len(category) - 2
		if len(errMsg) > maxAllowedErrLen {
			errMsg = errMsg[:maxAllowedErrLen-3] + "..." // -3 for "..."
		}
	}

	// Format as "Category: Brief description"
	if url != "" {
		return fmt.Sprintf("%s: %s (URL: %s)", category, errMsg, url)
	}
	return fmt.Sprintf("%s: %s", category, errMsg)
}

// Execute processes a crawler URL job.
//
// Execution Flow:
//   1. Validate message (URL, config, depth)
//   2. Extract configuration (max_depth, follow_links, source_type)
//   3. Check depth limit (skip if depth > max_depth)
//   4. Log job start with structured fields
//   5. Process URL (currently simulated - TODO: implement real processing)
//   6. Update parent job progress (increment completed, decrement pending)
//   7. Discover and enqueue child jobs (if follow_links enabled and depth < max_depth)
//   8. Log job completion
//
// Error Handling:
//   - Validation errors: Log, update job status to 'failed', return error
//   - Processing errors: Log, update job status to 'failed', return error
//   - Progress update errors: Log warning, continue (non-critical)
//   - Child enqueue errors: Log warning, continue (partial success acceptable)
//
// Parent-Child Hierarchy:
//   - Child jobs inherit parent's jobID via msg.ParentID (flat hierarchy)
//   - All children reference the root parent, not immediate parent
//   - Progress tracked at parent job level (TotalURLs, CompletedURLs, PendingURLs)
//
// TODO: Real URL Processing:
//   - Replace simulation (lines 186-200) with actual crawler service call
//   - Extract links from scraped content
//   - Store documents in DocumentStorage
//   - Handle HTTP errors, timeouts, network failures
//   - Use formatJobError() for user-friendly error messages
func (c *CrawlerJob) Execute(ctx context.Context, msg *queue.JobMessage) error {
	// Extract timeout from configuration
	timeout := 0 * time.Second
	if to, ok := msg.Config["timeout_ms"].(float64); ok {
		timeout = time.Duration(to) * time.Millisecond
	}

	// Validate message
	if err := c.Validate(msg); err != nil {
		c.logger.LogJobError(err, fmt.Sprintf("Validation failed for URL=%s, depth=%d", msg.URL, msg.Depth))

		// If this is a child job, update parent counters before failing
		if msg.ParentID != "" {
			c.updateParentOnChildFailure(ctx, msg.ParentID, "Validation failed")
		}

		return c.failJobWithError(ctx, c.GetLogger().GetJobID(), "Validation", err, msg.URL, timeout)
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

	// Extract sourceType from config
	sourceType := ""
	if st, ok := msg.Config["source_type"].(string); ok {
		sourceType = st
	}

	// Log job start
	c.logger.LogJobStart(
		fmt.Sprintf("Processing URL: %s (depth=%d)", msg.URL, msg.Depth),
		sourceType,
		msg.Config,
	)

	// Process URL using crawler service
	startTime := time.Now()

	// Note: In a real implementation, the crawler job would process the URL
	// For now, we'll simulate processing and enqueue child jobs
	// In the actual system, URLs are processed through the CrawlerService

	// TODO: Implement Real URL Processing
	//
	// Replace the simulation below with actual crawler service integration:
	//
	// 1. Call crawler service to fetch and parse URL:
	//    result, err := c.deps.CrawlerService.ScrapeURL(ctx, msg.URL, msg.Config)
	//    if err != nil {
	//        return c.failJobWithError(ctx, msg.JobID, "Scraping", err, msg.URL, timeout)
	//    }
	//
	// 2. Store document in DocumentStorage:
	//    doc := &models.Document{
	//        ID:          generateDocumentID(),
	//        SourceType:  sourceType,
	//        SourceID:    msg.URL,
	//        Title:       result.Title,
	//        Content:     result.Content,
	//        // ... other fields
	//    }
	//    if err := c.deps.DocumentStorage.SaveDocument(doc); err != nil {
	//        return c.failJobWithError(ctx, msg.JobID, "Storage", err, msg.URL, timeout)
	//    }
	//
	// 3. Extract links from scraped content:
	//    links := result.Links // Extracted by crawler service
	//
	// 4. Replace simulatedLinks (line 222) with actual links
	//
	// 5. Handle edge cases:
	//    - HTTP errors (404, 500, etc.) - use formatJobError("HTTP", err, url)
	//    - Timeouts - use formatJobError("Timeout", err, url)
	//    - Network errors - use formatJobError("Network", err, url)
	//    - Invalid content - use formatJobError("Scraping", err, url)

	c.logger.Info().
		Str("url", msg.URL).
		Int("depth", msg.Depth).
		Msg("URL processing simulated (no-op)")

	duration := time.Since(startTime)

	// Update progress if we have a parent job
	if msg.ParentID != "" {
		// Load parent job to check current state and prevent underflow
		if jobInterface, jobErr := c.deps.JobStorage.GetJob(ctx, msg.ParentID); jobErr == nil {
			if job, ok := jobInterface.(*models.CrawlJob); ok {
				// Check if we can safely decrement PendingURLs (bounds check to prevent underflow)
				if job.Progress.PendingURLs > 0 {
					// Use atomic update to prevent race conditions from concurrent workers
					// UpdateProgressCountersAtomic uses single atomic SQL UPDATE with +=/-=
					if updateErr := c.deps.JobStorage.UpdateProgressCountersAtomic(ctx, msg.ParentID, +1, -1, 0, 0); updateErr != nil {
						c.logger.Warn().Err(updateErr).Str("parent_id", msg.ParentID).Msg("Failed to atomically update parent job progress")
					} else {
						// Log progress after successful atomic update
						c.logger.LogJobProgress(job.Progress.CompletedURLs+1, job.Progress.TotalURLs, fmt.Sprintf("Processed URL: %s", msg.URL))
					}
				} else {
					// PendingURLs already at zero, skip decrement to prevent underflow
					c.logger.Warn().
						Str("parent_id", msg.ParentID).
						Int("pending_urls", job.Progress.PendingURLs).
						Str("url", msg.URL).
						Msg("Skipping progress update: PendingURLs already at zero")
				}
			}
		}
	}

	// Enqueue child jobs for discovered URLs if followLinks is enabled
	// Note: In a real implementation, links would be extracted from the scraped content
	if followLinks && msg.Depth < maxDepth {
		// Simulate finding some links
		simulatedLinks := []string{
			fmt.Sprintf("%s/page1", msg.URL),
			fmt.Sprintf("%s/page2", msg.URL),
		}

		c.logger.Info().
			Int("links_found", len(simulatedLinks)).
			Int("next_depth", msg.Depth+1).
			Msg("Enqueueing child jobs for discovered links")

		for _, link := range simulatedLinks {
			// Generate unique IDs for message and child job record
			childMsgID := generateMessageID()
			childJobID := generateJobID()

			// Create child job message
			childMsg := &queue.JobMessage{
				ID:       childMsgID,
				Type:     "crawler_url",
				URL:      link,
				Depth:    msg.Depth + 1,
				ParentID: msg.ParentID, // Inherit parent ID for flat hierarchy
				Config:   msg.Config,
			}

			// Create child job record in database for proper job tracking
			// Extract sourceType from config
			childSourceType := sourceType
			if st, ok := msg.Config["source_type"].(string); ok {
				childSourceType = st
			}

			// Extract entityType from config (default to parent's entityType)
			childEntityType := ""
			if et, ok := msg.Config["entity_type"].(string); ok {
				childEntityType = et
			}

			// Create child job database record
			childJobConfig := models.CrawlConfig{
				MaxDepth:    maxDepth,
				FollowLinks: followLinks,
			}

			if err := c.CreateChildJobRecord(ctx, msg.ParentID, childJobID, link, childSourceType, childEntityType, childJobConfig); err != nil {
				c.logger.Warn().
					Err(err).
					Str("url", link).
					Str("child_id", childJobID).
					Msg("Failed to create child job record, skipping enqueue")
				continue
			}

			// Enqueue child job
			if err := c.EnqueueChildJob(ctx, childMsg); err != nil {
				c.logger.Warn().
					Err(err).
					Str("url", link).
					Str("child_id", childJobID).
					Msg("Failed to enqueue child job")
			}
		}
	}

	// Log completion
	c.logger.LogJobComplete(duration, 1)

	return nil
}

// failJobWithError consolidates error handling logic for failing a job.
//
// This helper method:
//   1. Formats error message using formatJobError()
//   2. Updates job status to 'failed' via JobStorage.UpdateJobStatus()
//   3. Logs error with context via JobLogger.LogJobError()
//   4. Returns original error for worker to handle
//
// Parameters:
//   - ctx: Context for database operations
//   - jobID: ID of the job to fail
//   - category: Error category (Validation, Network, Scraping, Storage, System)
//   - err: The original error
//   - url: URL being processed (empty string if not applicable)
//
// Usage:
//   if err := processURL(msg.URL); err != nil {
//       return c.failJobWithError(ctx, msg.JobID, "Scraping", err, msg.URL, timeout)
//   }
//
// This method should be used consistently for all job failure paths to ensure:
//   - User-friendly error messages in the UI
//   - Consistent error logging format
//   - Proper job status updates
func (c *CrawlerJob) failJobWithError(ctx context.Context, jobID string, category string, err error, url string, timeout time.Duration) error {
	errorMsg := formatJobError(category, err, url, timeout)
	if updateErr := c.deps.JobStorage.UpdateJobStatus(ctx, jobID, "failed", errorMsg); updateErr != nil {
		c.logger.Warn().Err(updateErr).Str("job_id", jobID).Msg("Failed to update job status")
	}
	c.logger.LogJobError(err, errorMsg)
	return err
}

// ExecuteCompletionProbe processes a crawler completion probe
func (c *CrawlerJob) ExecuteCompletionProbe(ctx context.Context, msg *queue.JobMessage) error {
	c.logger.Info().
		Str("message_id", msg.ID).
		Str("parent_id", msg.ParentID).
		Msg("Processing crawler completion probe")

	// Load parent job
	jobInterface, err := c.deps.JobStorage.GetJob(ctx, msg.ParentID)
	if err != nil {
		c.logger.LogJobError(err, fmt.Sprintf("Failed to load parent job: parent_id=%s", msg.ParentID))
		return c.failJobWithError(ctx, c.GetLogger().GetJobID(), "System", err, "", 0)
	}

	job, ok := jobInterface.(*models.CrawlJob)
	if !ok {
		c.logger.LogJobError(fmt.Errorf("parent job is not a CrawlJob"), fmt.Sprintf("Parent job is not a CrawlJob: parent_id=%s", msg.ParentID))
		return c.failJobWithError(ctx, c.GetLogger().GetJobID(), "System", fmt.Errorf("parent job is not a CrawlJob"), "", 0)
	}

	// Check if all URLs have been processed
	if job.Progress.PendingURLs == 0 && job.Progress.CompletedURLs+job.Progress.FailedURLs > 0 {
		// All URLs processed, mark job as completed
		job.Status = models.JobStatusCompleted
		job.CompletedAt = time.Now()

		if err := c.deps.JobStorage.SaveJob(ctx, job); err != nil {
			// Log the error but don't mark parent as failed
			c.logger.LogJobError(err, fmt.Sprintf("Failed to save parent job (will retry): parent_id=%s", msg.ParentID))
			// Don't mark the parent as failed due to SaveJob failure
			// This could be a temporary issue (database locked, etc.)
			// Return error so the probe can be retried later
			return fmt.Errorf("failed to save parent job (retry needed): %w", err)
		}

		c.logger.Info().
			Str("parent_id", msg.ParentID).
			Int("completed_urls", job.Progress.CompletedURLs).
			Int("failed_urls", job.Progress.FailedURLs).
			Msg("Parent job marked as completed")
	}

	return nil
}

// updateParentOnChildFailure updates parent job counters when a child job fails.
// This ensures parent job progress tracking stays accurate even when children fail.
// Only updates counters - does not fail the parent job (reserved for threshold logic).
func (c *CrawlerJob) updateParentOnChildFailure(ctx context.Context, parentID, reason string) {
	// Use atomic update to increment FailedURLs and decrement PendingURLs
	// This is safe for concurrent updates from multiple failing children
	if updateErr := c.deps.JobStorage.UpdateProgressCountersAtomic(ctx, parentID, 0, -1, 0, +1); updateErr != nil {
		c.logger.Warn().
			Err(updateErr).
			Str("parent_id", parentID).
			Str("reason", reason).
			Msg("Failed to update parent counters on child failure")
	} else {
		c.logger.Debug().
			Str("parent_id", parentID).
			Str("reason", reason).
			Msg("Updated parent counters for child failure")
	}
}

// Validate validates the crawler message
func (c *CrawlerJob) Validate(msg *queue.JobMessage) error {
	if msg.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if msg.Config == nil {
		return fmt.Errorf("config is required")
	}

	// Validate max_depth if present
	if maxDepth, ok := msg.Config["max_depth"].(float64); ok {
		if maxDepth < 0 {
			return fmt.Errorf("max_depth must be non-negative")
		}
		if maxDepth > 10 {
			return fmt.Errorf("max_depth cannot exceed 10")
		}
	}

	return nil
}

// GetType returns the job type
func (c *CrawlerJob) GetType() string {
	return "crawler_url"
}

// generateMessageID generates a unique message ID
// This is a placeholder - in production, use a proper UUID generator
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// generateJobID generates a unique job ID
// This is a placeholder - in production, use a proper UUID generator
func generateJobID() string {
	return fmt.Sprintf("job_%d", time.Now().UnixNano())
}
