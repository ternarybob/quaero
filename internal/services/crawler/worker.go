// -----------------------------------------------------------------------
// Last Modified: Thursday, 23rd October 2025 7:21:07 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package crawler

// worker.go contains worker-related functions for processing URLs from the queue.
// Workers handle individual URL fetching, link discovery, and content extraction.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

func (s *Service) workerLoop(jobID string, workerIndex int, config CrawlConfig) {
	workerStartTime := time.Now()
	urlsProcessed := 0
	contextLogger := s.logger.WithContextWriter(jobID)

	// Log worker start
	contextLogger.Debug().
		Str("job_id", jobID).
		Int("worker_index", workerIndex).
		Int("concurrency", config.Concurrency).
		Msg("Worker started")

	// Defer worker exit logging
	defer func() {
		s.wg.Done()
		duration := time.Since(workerStartTime)
		contextLogger.Debug().
			Str("job_id", jobID).
			Int("urls_processed", urlsProcessed).
			Dur("duration", duration).
			Msg("Worker exiting")
	}()

	// Periodic diagnostics ticker (every 30 seconds)
	diagnosticsTicker := time.NewTicker(30 * time.Second)
	defer diagnosticsTicker.Stop()

	for {
		// Check diagnostics ticker
		select {
		case <-diagnosticsTicker.C:
			// Log queue diagnostics periodically
			s.logQueueDiagnostics(jobID)
		default:
			// Continue with normal processing
		}

		// Check if job is still active
		s.jobsMu.RLock()
		job, exists := s.activeJobs[jobID]
		if !exists || job.Status == JobStatusCancelled || job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
			s.jobsMu.RUnlock()
			return
		}
		s.jobsMu.RUnlock()

		// Log current queue state at start of worker iteration
		contextLogger.Debug().
			Str("job_id", jobID).
			Int("pending_urls", job.Progress.PendingURLs).
			Int("completed_urls", job.Progress.CompletedURLs).
			Int("failed_urls", job.Progress.FailedURLs).
			Msg("Worker iteration - queue state")

		// Check max pages limit
		if config.MaxPages > 0 && job.Progress.CompletedURLs >= config.MaxPages {
			// Log max pages reached
			maxPagesMsg := fmt.Sprintf("Max pages limit reached (%d/%d)", job.Progress.CompletedURLs, config.MaxPages)
			contextLogger.Info().
				Str("job_id", jobID).
				Int("completed", job.Progress.CompletedURLs).
				Int("max_pages", config.MaxPages).
				Msg(maxPagesMsg)
			return
		}

		// Pop URL from queue (blocking with timeout)
		queueLen := s.queue.Len()
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		item, err := s.queue.Pop(ctx)
		cancel()

		if err != nil {
			if err == context.DeadlineExceeded {
				// Distinguish between timeout and empty queue
				s.jobsMu.RLock()
				pendingURLs := job.Progress.PendingURLs
				s.jobsMu.RUnlock()

				// Check if queue is actually empty and no pending URLs
				if queueLen == 0 && pendingURLs == 0 {
					contextLogger.Debug().
						Str("job_id", jobID).
						Msg("Queue empty and no pending URLs - worker exiting gracefully")
					return
				}

				// Queue has items but timeout occurred - log warning and continue with backoff
				if queueLen > 0 {
					contextLogger.Warn().
						Str("job_id", jobID).
						Int("queue_len", queueLen).
						Int("pending_urls", pendingURLs).
						Msg("Queue has items but Pop() timed out - possible queue health issue")
				}

				// Continue to retry
				continue
			}
			contextLogger.Debug().Err(err).Msg("Error popping from queue")
			return
		}

		if item == nil {
			// Queue closed
			return
		}

		// Check if this URL belongs to our job
		if itemJobID, ok := item.Metadata["job_id"].(string); !ok || itemJobID != jobID {
			// URL belongs to different job, skip it (other workers will handle it)
			continue
		}

		// Log the popped URL with depth and metadata
		contextLogger.Debug().
			Str("job_id", jobID).
			Str("url", item.URL).
			Int("depth", item.Depth).
			Int("priority", item.Priority).
			Msg("Processing URL from queue")

		// Check depth limit
		if config.MaxDepth > 0 && item.Depth > config.MaxDepth {
			// Sample depth limit skips: log every 10th skipped URL to avoid spam
			s.jobsMu.RLock()
			failedCount := job.Progress.FailedURLs
			s.jobsMu.RUnlock()

			if failedCount%10 == 0 {
				// Persist depth limit skip
				depthSkipMsg := fmt.Sprintf("Depth limit skip: url=%s, depth=%d, max_depth=%d", item.URL, item.Depth, config.MaxDepth)
				contextLogger.Warn().
					Str("url", item.URL).
					Int("depth", item.Depth).
					Int("max_depth", config.MaxDepth).
					Msg(depthSkipMsg)
			}
			// Count as failed to decrement pending count
			s.updateProgress(jobID, false, true)
			continue
		}

		// Update current URL
		s.updateCurrentURL(jobID, item.URL)

		// Comment 3: Rate limiting is now handled by Colly's Limit() in HTMLScraper
		// Removed service-level rate limiting to avoid double rate limiting
		// Colly applies rate limiting more efficiently per-domain with built-in parallelism control

		// Execute request with retry
		result := s.executeRequest(item, workerIndex, config)

		// Store result
		s.jobsMu.Lock()
		s.jobResults[jobID] = append(s.jobResults[jobID], result)
		totalResults := len(s.jobResults[jobID])
		s.jobsMu.Unlock()

		// Increment URLs processed counter
		urlsProcessed++

		// Log result storage (sampled: every 10th result to avoid spam)
		if totalResults%10 == 0 {
			bodySize := len(result.Body)
			contextLogger.Debug().
				Str("job_id", jobID).
				Str("url", result.URL).
				Int("status_code", result.StatusCode).
				Int("body_size", bodySize).
				Str("error", result.Error).
				Int("total_results", totalResults).
				Dur("duration", result.Duration).
				Msg("Result stored")
		}

		// Save document immediately if crawl was successful and markdown is available
		if result.Error == "" {
			// Extract markdown from result metadata
			var markdown string
			if md, ok := result.Metadata["markdown"]; ok {
				if mdStr, ok := md.(string); ok {
					markdown = mdStr
				}
			}

			// Log markdown extraction status
			if markdown != "" {
				contextLogger.Info().
					Str("job_id", jobID).
					Str("url", item.URL).
					Int("markdown_length", len(markdown)).
					Msg("Markdown extracted successfully from page")
			} else {
				contextLogger.Warn().
					Str("url", item.URL).
					Msg("Markdown is empty - document will NOT be saved (check OnlyMainContent setting and page structure)")
			}

			// Only save document if markdown is non-empty
			if markdown != "" {
				// Extract source type from item metadata with fallback logic
				sourceType := "crawler" // Default fallback
				if st, ok := item.Metadata["source_type"]; ok {
					if stStr, ok := st.(string); ok {
						sourceType = stStr
					}
				} else {
					// Fallback: Extract from URL domain
					if strings.Contains(item.URL, "atlassian.net") {
						if strings.Contains(item.URL, "/wiki/") {
							sourceType = "confluence"
						} else if strings.Contains(item.URL, "/browse/") || strings.Contains(item.URL, "/jira/") {
							sourceType = "jira"
						}
					}
				}

				// Extract title with priority order: metadata["title"] > URL path > URL
				title := item.URL // Default fallback
				if t, ok := result.Metadata["title"]; ok {
					if tStr, ok := t.(string); ok && tStr != "" {
						title = tStr
					}
				} else {
					// Extract from URL path (last segment)
					if u, err := url.Parse(item.URL); err == nil && u.Path != "" {
						pathSegments := strings.Split(strings.Trim(u.Path, "/"), "/")
						if len(pathSegments) > 0 {
							lastSegment := pathSegments[len(pathSegments)-1]
							if lastSegment != "" {
								title = lastSegment
							}
						}
					}
				}

				// Create document with generated UUID
				doc := models.Document{
					ID:              "doc_" + uuid.New().String(),
					SourceType:      sourceType,
					SourceID:        item.URL, // Use URL as source_id for deduplication
					Title:           title,
					ContentMarkdown: markdown,
					DetailLevel:     models.DetailLevelFull,
					Metadata:        result.Metadata, // Preserve all scraped metadata
					URL:             item.URL,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				}

				// Save document to database
				if err := s.documentStorage.SaveDocument(&doc); err != nil {
					// Log error but don't fail the crawl job
					contextLogger.Error().
						Err(err).
						Str("job_id", jobID).
						Str("document_id", doc.ID).
						Str("title", doc.Title).
						Str("url", doc.URL).
						Str("source_type", doc.SourceType).
						Msg(fmt.Sprintf("Document save failed: %s (url=%s)", err.Error(), doc.URL))
				} else {
					// Increment documents saved counter (under lock)
					s.jobsMu.Lock()
					if job, ok := s.activeJobs[jobID]; ok {
						job.DocumentsSaved++
						docsSaved := job.DocumentsSaved
						s.jobsMu.Unlock()

						// Log success at INFO level
						contextLogger.Info().
							Str("job_id", jobID).
							Str("document_id", doc.ID).
							Str("title", doc.Title).
							Str("url", doc.URL).
							Int("markdown_length", len(doc.ContentMarkdown)).
							Str("source_type", doc.SourceType).
							Int("documents_saved", docsSaved).
							Msg("Document saved immediately after crawling")

						// Persist success (sampled: every 10th successful document save)
						if docsSaved%10 == 0 {
							contextLogger.Info().Msg(fmt.Sprintf("Document saved: %s (url=%s, markdown_length=%d, total_saved=%d)", doc.Title, doc.URL, len(doc.ContentMarkdown), docsSaved))
						}
					} else {
						s.jobsMu.Unlock()
					}
				}
			}
		}

		// Update progress
		if result.Error == "" {
			s.updateProgress(jobID, true, false)

			// Discover links if enabled and within depth limit (0 = unlimited depth)
			if config.FollowLinks && (config.MaxDepth == 0 || item.Depth < config.MaxDepth) {
				// Log link discovery decision at INFO level
				contextLogger.Debug().
					Str("job_id", jobID).
					Str("url", item.URL).
					Bool("follow_links", config.FollowLinks).
					Int("depth", item.Depth).
					Int("max_depth", config.MaxDepth).
					Bool("will_discover_links", true).
					Msg("Link discovery enabled - will extract and follow links")

				// Comment 2: Removed sampled DEBUG database logging to prevent log bloat
				// Link discovery details are available in console logs via s.logger.Debug() above

				links := s.discoverLinks(result, item, config)
				s.enqueueLinks(jobID, links, item)
			} else {
				// Log link discovery skip
				contextLogger.Debug().
					Str("job_id", jobID).
					Str("url", item.URL).
					Str("follow_links", fmt.Sprintf("%v", config.FollowLinks)).
					Int("depth", item.Depth).
					Int("max_depth", config.MaxDepth).
					Msg("Skipping link discovery - FollowLinks disabled or depth limit reached")

				// Comment 2: Removed sampled DEBUG database logging to prevent log bloat
				// Link skip details are available in console logs via s.logger.Debug() above
			}
		} else {
			s.updateProgress(jobID, false, true)

			// Categorize errors: timeout, auth (401/403), network, server (5xx)
			errorCategory := "unknown"
			statusCode := result.StatusCode
			errorMsg := result.Error

			if statusCode == 401 || statusCode == 403 {
				errorCategory = "auth"
			} else if statusCode >= 500 && statusCode < 600 {
				errorCategory = "server"
			} else if statusCode == 0 || strings.Contains(strings.ToLower(errorMsg), "timeout") {
				errorCategory = "timeout"
			} else if statusCode >= 400 && statusCode < 500 {
				errorCategory = "client"
			} else {
				errorCategory = "network"
			}

			// Append categorized request failure log
			if s.jobStorage != nil {
				failureMsg := fmt.Sprintf("Request failed: %s - %s: %s (status=%d, category=%s, attempt=%d)",
					item.URL, errorCategory, errorMsg, statusCode, errorCategory, item.Attempts+1)

				logEntry := models.JobLogEntry{
					Timestamp: time.Now().Format("15:04:05"),
					Level:     "error",
					Message:   failureMsg,
				}
				if err := s.jobStorage.AppendJobLog(s.ctx, jobID, logEntry); err != nil {
					contextLogger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append request failure log")
				}
			}
		}

		// Emit progress periodically (every 10 URLs)
		if (job.Progress.CompletedURLs+job.Progress.FailedURLs)%10 == 0 {
			s.emitProgress(job)

			// Calculate success rate for progress milestone
			totalProcessed := job.Progress.CompletedURLs + job.Progress.FailedURLs
			successRate := 0.0
			if totalProcessed > 0 {
				successRate = float64(job.Progress.CompletedURLs) / float64(totalProcessed) * 100
			}

			// Log progress milestone
			progressMsg := fmt.Sprintf("Progress: %d completed, %d failed, %d pending (success_rate=%.1f%%)",
				job.Progress.CompletedURLs, job.Progress.FailedURLs, job.Progress.PendingURLs, successRate)
			contextLogger.Info().
				Int("completed", job.Progress.CompletedURLs).
				Int("failed", job.Progress.FailedURLs).
				Int("pending", job.Progress.PendingURLs).
				Float64("success_rate", successRate).
				Msg(progressMsg)
		}
	}
}

// executeRequest wraps makeRequest with retry policy
func (s *Service) executeRequest(item *URLQueueItem, workerIndex int, config CrawlConfig) *CrawlResult {
	startTime := time.Now()

	// Extract job ID for logging
	jobID := ""
	if item.Metadata != nil {
		if jid, ok := item.Metadata["job_id"].(string); ok {
			jobID = jid
		}
	}
	contextLogger := s.logger.WithContextWriter(jobID)

	// Log request start
	contextLogger.Debug().
		Str("job_id", jobID).
		Str("url", item.URL).
		Int("depth", item.Depth).
		Int("worker_index", workerIndex).
		Int("attempt", item.Attempts+1).
		Msg("Starting request with retry policy")

	statusCode, err := s.retryPolicy.ExecuteWithRetry(s.ctx, s.logger, func() (int, error) {
		return s.makeRequest(item, workerIndex, config)
	})

	duration := time.Since(startTime)

	result := &CrawlResult{
		URL:      item.URL,
		Duration: duration,
		Metadata: item.Metadata,
	}

	// Comment 4: Verify HTML propagation path from makeRequest to transformers
	// Path: makeRequest() → item.Metadata["response_body"] (HTML) → result.Body
	// Transformers read from: result.Body, metadata["html"], metadata["markdown"], or metadata["response_body"]
	// This ensures HTML content reaches parsers correctly after Comment 1 changes
	if item.Metadata != nil {
		if bodyRaw, ok := item.Metadata["response_body"]; ok {
			switch v := bodyRaw.(type) {
			case []byte:
				result.Body = v
			case string:
				result.Body = []byte(v)
			}
		}
	}

	if err != nil {
		result.Error = err.Error()
		result.StatusCode = statusCode
		contextLogger.Debug().
			Str("job_id", jobID).
			Str("url", item.URL).
			Int("status_code", statusCode).
			Dur("duration", duration).
			Err(err).
			Msg("Request failed after retries")
	} else {
		result.StatusCode = statusCode

		// Validate response body - warn if empty but status is 200
		bodySize := len(result.Body)
		if statusCode == 200 && bodySize == 0 {
			contextLogger.Warn().
				Str("job_id", jobID).
				Str("url", item.URL).
				Int("status_code", statusCode).
				Dur("duration", duration).
				Msg("Response body is empty despite HTTP 200 status")
		}

		// Log successful request completion
		contextLogger.Debug().
			Str("job_id", jobID).
			Str("url", item.URL).
			Int("status_code", statusCode).
			Int("body_size", bodySize).
			Dur("duration", duration).
			Msg("Request completed successfully")
	}

	return result
}

// makeRequest performs HTML scraping using chromedp-based HTMLScraper with browser pooling
func (s *Service) makeRequest(item *URLQueueItem, workerIndex int, config CrawlConfig) (int, error) {
	startTime := time.Now()

	// Extract HTTP client and cookies for auth
	var client *http.Client
	var jobID string
	if jid, ok := item.Metadata["job_id"].(string); ok && jid != "" {
		jobID = jid
		s.jobsMu.RLock()
		client = s.jobClients[jobID]
		s.jobsMu.RUnlock()
	}
	contextLogger := s.logger.WithContextWriter(jobID)

	if client != nil {
		contextLogger.Debug().Str("url", item.URL).Str("job_id", jobID).Msg("Using per-job HTTP client with auth")
	}

	// Fallback to auth service's HTTP client if no per-job client
	if client == nil {
		client = s.authService.GetHTTPClient()
		if client != nil {
			contextLogger.Debug().Str("url", item.URL).Msg("Using auth service HTTP client")
		}
	}

	// Final fallback to default client
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
		contextLogger.Debug().Str("url", item.URL).Msg("Using default HTTP client (no auth)")
	}

	// Extract cookies from selected client for chromedp
	cookies := s.extractCookiesFromClient(client, item.URL)

	// Comment 2: Removed DEBUG database logging for HTTP client selection to prevent log bloat
	// Client selection details are available in console logs via s.logger.Debug() above

	// Comment 2: Apply job-level crawl limits to HTMLScraper config
	// Create a merged config by copying service-level config and overriding with job-specific settings
	scraperConfig := s.config.Crawler // Start with service-level config

	// Map job-level CrawlConfig fields to common.CrawlerConfig fields
	if config.RateLimit > 0 {
		// RateLimit (in CrawlConfig) maps to RequestDelay (in common.CrawlerConfig)
		scraperConfig.RequestDelay = time.Duration(config.RateLimit) * time.Millisecond
	}
	if config.Concurrency > 0 {
		// Constrain MaxConcurrency per job (don't exceed job's concurrency setting)
		if config.Concurrency < scraperConfig.MaxConcurrency {
			scraperConfig.MaxConcurrency = config.Concurrency
		}
	}
	if config.MaxDepth > 0 {
		// MaxDepth is available in both configs
		scraperConfig.MaxDepth = config.MaxDepth
	}

	// Comment 2: Removed DEBUG database logging for scraper config to prevent log bloat
	// Scraper config details are available in console logs during initialization

	// Create HTMLScraper instance with merged config
	var scraper *HTMLScraper

	// Use browser pool if JavaScript rendering is enabled
	if s.config.Crawler.EnableJavaScript {
		browserCtx, browserCancel := s.getBrowserFromPool(workerIndex)
		if browserCtx != nil {
			scraper = NewHTMLScraperWithBrowser(scraperConfig, s.logger, client, cookies, browserCtx, browserCancel)
			contextLogger.Debug().
				Str("url", item.URL).
				Int("worker_index", workerIndex).
				Msg("Using pooled browser for scraping")
		} else {
			// Fallback if pool not initialized
			scraper = NewHTMLScraper(scraperConfig, s.logger, client, cookies)
			contextLogger.Warn().
				Str("url", item.URL).
				Msg("Browser pool not available, creating new browser instance")
		}
	} else {
		// JavaScript disabled, use regular scraper
		scraper = NewHTMLScraper(scraperConfig, s.logger, client, cookies)
	}

	// Execute scraping
	scrapeResult, err := scraper.ScrapeURL(s.ctx, item.URL)

	// DIAGNOSTIC: Log immediately after ScrapeURL returns
	contextLogger.Debug().Str("url", item.URL).Msg("DIAGNOSTIC: ScrapeURL returned")

	// DIAGNOSTIC: Check if err is nil before accessing it
	contextLogger.Debug().Str("url", item.URL).Bool("err_is_nil", err == nil).Msg("DIAGNOSTIC: Checking err")

	if err != nil {
		// Check if context was cancelled
		if err == context.Canceled || err == context.DeadlineExceeded {
			contextLogger.Debug().Err(err).Str("url", item.URL).Msg("Scraping cancelled")
			return 0, err
		}
		contextLogger.Warn().Err(err).Str("url", item.URL).Msg("Scraping failed")
		return 0, fmt.Errorf("scraping failed: %w", err)
	}

	// DIAGNOSTIC: Check if scrapeResult is nil before accessing it
	contextLogger.Debug().Str("url", item.URL).Bool("result_is_nil", scrapeResult == nil).Msg("DIAGNOSTIC: Checking scrapeResult")

	// Log scrape result details before conversion
	contextLogger.Debug().
		Str("url", item.URL).
		Bool("success", scrapeResult.Success).
		Int("status", scrapeResult.StatusCode).
		Int("html_length", len(scrapeResult.RawHTML)).
		Int("markdown_length", len(scrapeResult.Markdown)).
		Int("links_count", len(scrapeResult.Links)).
		Msg("About to convert ScrapeResult to CrawlResult")

	// Convert ScrapeResult to CrawlResult-compatible format with panic recovery
	var crawlResult *CrawlResult
	func() {
		defer func() {
			if r := recover(); r != nil {
				contextLogger.Error().
					Str("url", item.URL).
					Str("panic", fmt.Sprintf("%v", r)).
					Str("stack", string(debug.Stack())).
					Msg("PANIC in ToCrawlResult()")
			}
		}()
		crawlResult = scrapeResult.ToCrawlResult()
	}()

	if crawlResult == nil {
		contextLogger.Error().Str("url", item.URL).Msg("ToCrawlResult returned nil")
		return 0, fmt.Errorf("result conversion failed")
	}

	contextLogger.Debug().
		Str("url", item.URL).
		Int("body_length", len(crawlResult.Body)).
		Int("metadata_keys", len(crawlResult.Metadata)).
		Msg("Successfully converted to CrawlResult")

	// Check for scraper failures (Comment 3)
	if !scrapeResult.Success || crawlResult.Error != "" {
		// Failure case: don't default statusCode, return error
		statusCode := crawlResult.StatusCode
		errorMsg := crawlResult.Error
		if errorMsg == "" {
			errorMsg = "scraping failed"
		}

		// Persist enhanced scraping failure
		contextLogger.Error().
			Str("url", item.URL).
			Int("status_code", statusCode).
			Str("error", errorMsg).
			Msg(fmt.Sprintf("Scraping failed: %s (status=%d, error=%s)", item.URL, statusCode, errorMsg))

		return statusCode, fmt.Errorf("%s", errorMsg)
	}

	// Success case: continue with processing
	// Store converted result in metadata for backward compatibility
	if item.Metadata == nil {
		item.Metadata = make(map[string]interface{})
	}

	// Comment 1: Store HTML (not markdown) in response_body
	// Priority: metadata["html"] > Body (if HTML) > nothing
	if htmlRaw, ok := crawlResult.Metadata["html"]; ok {
		// Use HTML from metadata if available
		if htmlStr, isString := htmlRaw.(string); isString && htmlStr != "" {
			item.Metadata["response_body"] = []byte(htmlStr)
		} else if htmlBytes, isBytes := htmlRaw.([]byte); isBytes && len(htmlBytes) > 0 {
			item.Metadata["response_body"] = htmlBytes
		}
	} else if crawlResult.Body != nil && len(crawlResult.Body) > 0 {
		// Fallback: use Body only if it's HTML (check content type or first bytes)
		// If Body contains markdown (text starting with # or typical markdown), skip it
		bodyStr := string(crawlResult.Body)
		// Simple heuristic: if it starts with HTML tags, it's HTML
		if strings.HasPrefix(strings.TrimSpace(bodyStr), "<") {
			item.Metadata["response_body"] = crawlResult.Body
		}
		// Otherwise, don't store markdown in response_body
	}

	// Store status code (Comment 4: only default to 200 if scraping was successful)
	statusCode := crawlResult.StatusCode
	if statusCode == 0 && scrapeResult.Success {
		statusCode = 200 // Default to 200 only for successful scrapes
	}
	item.Metadata["status_code"] = statusCode

	// Merge scrape result metadata with existing metadata (preserve critical fields)
	// Note: markdown should be in metadata["markdown"] for RAG use
	for key, value := range crawlResult.Metadata {
		// Don't overwrite job_id, source_type, entity_type
		if key != "job_id" && key != "source_type" && key != "entity_type" {
			item.Metadata[key] = value
		}
	}

	duration := time.Since(startTime)
	contextLogger.Debug().
		Str("url", item.URL).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Int("body_length", len(crawlResult.Body)).
		Msg("Scraping completed")

	// Comment 2: Removed DEBUG database logging for scraping success to prevent log bloat
	// Scraping success details are visible in console logs above

	// Check for HTTP errors
	if statusCode >= 400 {
		return statusCode, fmt.Errorf("HTTP %d", statusCode)
	}

	return statusCode, nil
}

// extractCookiesFromClient extracts cookies from HTTP client's cookie jar for a specific URL
func (s *Service) extractCookiesFromClient(client *http.Client, targetURL string) []*http.Cookie {
	// Check if client is nil
	if client == nil {
		s.logger.Warn().Str("url", targetURL).Msg("Client is nil, cannot extract cookies")
		return []*http.Cookie{}
	}

	// Check if client's cookie jar is nil
	if client.Jar == nil {
		s.logger.Warn().Str("url", targetURL).Msg("Client cookie jar is nil (auth not configured)")
		return []*http.Cookie{}
	}

	// Parse target URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		s.logger.Warn().Err(err).Str("url", targetURL).Msg("Failed to parse target URL for cookie extraction")
		return []*http.Cookie{}
	}

	// Get cookies for the URL
	cookies := client.Jar.Cookies(parsedURL)

	return cookies
}

// discoverLinks extracts links from HTML responses
func (s *Service) discoverLinks(result *CrawlResult, parent *URLQueueItem, config CrawlConfig) []string {
	// Extract jobID from parent metadata for logging
	jobID := ""
	if jid, ok := parent.Metadata["job_id"].(string); ok {
		jobID = jid
	}
	contextLogger := s.logger.WithContextWriter(jobID)

	links := make([]string, 0)

	// Comment 6: Check if links are already provided in ScrapeResult metadata
	var allLinks []string
	if result.Metadata != nil {
		if linksRaw, ok := result.Metadata["links"]; ok {
			// Fast path: []string
			if linksSlice, ok := linksRaw.([]string); ok && len(linksSlice) > 0 {
				// Use links provided by ScrapeResult
				allLinks = linksSlice
				contextLogger.Debug().
					Str("url", parent.URL).
					Int("links_count", len(allLinks)).
					Msg("Using links from ScrapeResult metadata")
			} else if linksInterface, ok := linksRaw.([]interface{}); ok && len(linksInterface) > 0 {
				// Defensive fallback: []interface{} -> convert to []string
				allLinks = make([]string, 0, len(linksInterface))
				for _, linkRaw := range linksInterface {
					if linkStr, ok := linkRaw.(string); ok {
						allLinks = append(allLinks, linkStr)
					} else {
						contextLogger.Debug().
							Str("url", parent.URL).
							Str("type", fmt.Sprintf("%T", linkRaw)).
							Msg("Skipping non-string element in links metadata")
					}
				}
				if len(allLinks) > 0 {
					contextLogger.Debug().
						Str("url", parent.URL).
						Int("links_count", len(allLinks)).
						Int("total_elements", len(linksInterface)).
						Msg("Converted []interface{} links to []string")
				}
			}
		}
	}

	// Fallback: Extract links from HTML if not provided in metadata
	if len(allLinks) == 0 {
		// Extract HTML from result.Body or metadata["response_body"]
		var body []byte
		if result.Body != nil && len(result.Body) > 0 {
			body = result.Body
		} else if bodyRaw, ok := parent.Metadata["response_body"]; ok {
			switch v := bodyRaw.(type) {
			case []byte:
				body = v
			case string:
				body = []byte(v)
			}
		}

		if len(body) == 0 {
			return links
		}

		// Convert to HTML string
		html := string(body)

		// Extract all links from HTML using regex
		allLinks = s.extractLinksFromHTML(html, parent.URL)
	}

	// Log discovered links summary with samples (Comment 2: console-only DEBUG)
	if jobID != "" && len(allLinks) > 0 {
		contextLogger.Debug().
			Str("job_id", jobID).
			Str("parent_url", parent.URL).
			Int("total_discovered", len(allLinks)).
			Msg("Discovered links from page")

		// Log sample of discovered URLs (first 5) - console only
		sampleSize := 5
		if len(allLinks) < sampleSize {
			sampleSize = len(allLinks)
		}

		// Comment 2: Removed DEBUG database logging to prevent log bloat
		// Link discovery details are visible in console logs above
	}

	// Warn on zero links discovered
	if len(allLinks) == 0 {
		contextLogger.Warn().
			Str("url", parent.URL).
			Msg(fmt.Sprintf("Zero links discovered from %s - check page structure", parent.URL))
		return nil
	}

	// Parse parent URL to get base host for same-host filtering
	baseHost := ""
	if parsedParent, err := url.Parse(parent.URL); err == nil {
		baseHost = strings.ToLower(parsedParent.Host)
	}

	// Get source type for source-specific filtering
	sourceType := ""
	if st, ok := parent.Metadata["source_type"].(string); ok {
		sourceType = st
	} else {
		contextLogger.Warn().Str("url", parent.URL).Msg("source_type not found in metadata, skipping source-specific filtering")
	}

	// Apply source-specific filtering
	var filteredLinks []string
	switch sourceType {
	case "jira":
		filteredLinks = s.filterJiraLinks(contextLogger, allLinks, baseHost, config)
	case "confluence":
		filteredLinks = s.filterConfluenceLinks(contextLogger, allLinks, baseHost, config)
	default:
		// No source-specific filtering for other types
		filteredLinks = allLinks
	}

	// Apply include/exclude patterns (Comment 9: collect filtered samples)
	links, excludedSamples, notIncludedSamples := s.filterLinks(jobID, filteredLinks, config)

	// Log detailed filtering breakdown (Info level for visibility)
	contextLogger.Info().
		Str("url", parent.URL).
		Int("total_discovered", len(allLinks)).
		Int("after_source_filter", len(filteredLinks)).
		Int("after_pattern_filter", len(links)).
		Int("filtered_out", len(allLinks)-len(links)).
		Str("source_type", sourceType).
		Int("parent_depth", parent.Depth).
		Str("follow_links", fmt.Sprintf("%v", config.FollowLinks)).
		Int("max_depth", config.MaxDepth).
		Msg("Link filtering complete")

	// Persist link filtering summary to database (Comment 3: use helper for consistency)
	if jobID != "" {
		sourceFilteredOut := len(allLinks) - len(filteredLinks)
		patternFilteredOut := len(filteredLinks) - len(links)
		totalFilteredOut := len(allLinks) - len(links)

		// Clear message: discovered -> after source filter -> after pattern filter -> following
		filterMsg := fmt.Sprintf("Found %d links, filtered %d (source:%d + pattern:%d), following %d",
			len(allLinks), totalFilteredOut, sourceFilteredOut, patternFilteredOut, len(links))
		contextLogger.Info().
			Int("discovered", len(allLinks)).
			Int("source_filtered", sourceFilteredOut).
			Int("pattern_filtered", patternFilteredOut).
			Int("total_filtered", totalFilteredOut).
			Int("following", len(links)).
			Msg(filterMsg)

		// Warn if all links were filtered out despite discovery (Comment 9: include samples)
		if len(allLinks) > 0 && len(links) == 0 {
			warnMsg := fmt.Sprintf("All %d discovered links filtered out - check include/exclude patterns and source filters", len(allLinks))

			// Append sample URLs to help diagnose filtering issues
			if len(excludedSamples) > 0 {
				warnMsg += fmt.Sprintf(" | Excluded samples: %v", excludedSamples)
			}
			if len(notIncludedSamples) > 0 {
				warnMsg += fmt.Sprintf(" | Not included samples: %v", notIncludedSamples)
			}

			contextLogger.Warn().
				Str("url", parent.URL).
				Int("discovered_count", len(allLinks)).
				Int("after_source_filter", len(filteredLinks)).
				Int("after_pattern_filter", len(links)).
				Str("source_type", sourceType).
				Msg(warnMsg)
		}

		// Comment 3: Removed verbose link samples from database to conserve log capacity
		// Link details are available in console logs and the summary INFO log above provides counts
	}

	return links
}

func (s *Service) extractLinksFromHTML(html string, baseURL string) []string {
	// Parse base URL for resolving relative links
	base, err := url.Parse(baseURL)
	if err != nil {
		s.logger.Warn().Err(err).Str("base_url", baseURL).Msg("Failed to parse base URL")
		return []string{}
	}

	// Extract href attributes using two regex patterns:
	// 1. Quoted hrefs: href="url" or href='url'
	// 2. Unquoted hrefs: href=url (value ends at whitespace or >)

	// Use map for deduplication
	linkMap := make(map[string]bool)

	// Helper function to process href values
	processHref := func(href string) {
		// Skip unwanted link types
		if strings.HasPrefix(href, "javascript:") ||
			strings.HasPrefix(href, "mailto:") ||
			strings.HasPrefix(href, "tel:") ||
			strings.HasPrefix(href, "data:") ||
			strings.HasPrefix(href, "#") {
			return
		}

		// Parse and resolve URL
		parsedURL, err := url.Parse(href)
		if err != nil {
			return
		}

		// Resolve relative URLs
		absoluteURL := base.ResolveReference(parsedURL)

		// Normalize URL: lowercase scheme and host
		absoluteURL.Scheme = strings.ToLower(absoluteURL.Scheme)
		absoluteURL.Host = strings.ToLower(absoluteURL.Host)

		// Remove fragment
		absoluteURL.Fragment = ""

		normalizedURL := absoluteURL.String()

		// Skip file downloads (common extensions)
		lowerURL := strings.ToLower(normalizedURL)
		if strings.HasSuffix(lowerURL, ".pdf") ||
			strings.HasSuffix(lowerURL, ".zip") ||
			strings.HasSuffix(lowerURL, ".rar") ||
			strings.HasSuffix(lowerURL, ".7z") ||
			strings.HasSuffix(lowerURL, ".tar") ||
			strings.HasSuffix(lowerURL, ".gz") ||
			strings.HasSuffix(lowerURL, ".tar.gz") ||
			strings.HasSuffix(lowerURL, ".exe") ||
			strings.HasSuffix(lowerURL, ".dmg") ||
			strings.HasSuffix(lowerURL, ".pkg") ||
			strings.HasSuffix(lowerURL, ".deb") ||
			strings.HasSuffix(lowerURL, ".rpm") ||
			strings.HasSuffix(lowerURL, ".doc") ||
			strings.HasSuffix(lowerURL, ".docx") ||
			strings.HasSuffix(lowerURL, ".xls") ||
			strings.HasSuffix(lowerURL, ".xlsx") ||
			strings.HasSuffix(lowerURL, ".ppt") ||
			strings.HasSuffix(lowerURL, ".pptx") ||
			strings.HasSuffix(lowerURL, ".csv") ||
			strings.HasSuffix(lowerURL, ".jpg") ||
			strings.HasSuffix(lowerURL, ".jpeg") ||
			strings.HasSuffix(lowerURL, ".png") ||
			strings.HasSuffix(lowerURL, ".gif") ||
			strings.HasSuffix(lowerURL, ".svg") ||
			strings.HasSuffix(lowerURL, ".webp") ||
			strings.HasSuffix(lowerURL, ".mp4") ||
			strings.HasSuffix(lowerURL, ".mov") ||
			strings.HasSuffix(lowerURL, ".avi") {
			return
		}

		// Add to map for deduplication
		linkMap[normalizedURL] = true
	}

	// Pattern 1: Quoted hrefs (case-insensitive, tolerates whitespace around =)
	quotedHrefRegex := regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	quotedMatches := quotedHrefRegex.FindAllStringSubmatch(html, -1)

	for _, match := range quotedMatches {
		if len(match) >= 2 {
			processHref(match[1])
		}
	}

	// Pattern 2: Unquoted hrefs (case-insensitive, value ends at whitespace or >)
	unquotedHrefRegex := regexp.MustCompile(`(?i)href\s*=\s*([^\s">]+)`)
	unquotedMatches := unquotedHrefRegex.FindAllStringSubmatch(html, -1)

	for _, match := range unquotedMatches {
		if len(match) >= 2 {
			processHref(match[1])
		}
	}

	// Convert map to slice
	links := make([]string, 0, len(linkMap))
	for link := range linkMap {
		links = append(links, link)
	}

	return links
}

// filterJiraLinks filters links to exclude bad Jira URLs (admin, API, etc.) on the same host
// Only applies universal exclude patterns. Include pattern matching is handled by filterLinks().
func (s *Service) filterJiraLinks(contextLogger arbor.ILogger, links []string, baseHost string, config CrawlConfig) []string {
	// Exclude patterns for non-content pages (universal exclusions for Jira)
	excludePatterns := []string{
		`(?i)/rest/api/`,            // API endpoints (case-insensitive)
		`(?i)/rest/agile/`,          // Agile REST API (case-insensitive)
		`(?i)/rest/auth/`,           // Auth REST API (case-insensitive)
		`(?i)/secure/attachment/`,   // File downloads (case-insensitive)
		`(?i)/plugins/servlet/`,     // Plugin servlets (case-insensitive)
		`(?i)/secure/admin/`,        // Admin pages (case-insensitive)
		`(?i)/login(\.jsp|\.jspa)?`, // Login pages (case-insensitive)
		`(?i)/logout`,               // Logout pages (case-insensitive)
		`/projects/[^/]+/settings`,  // Project settings pages
	}

	// Precompile exclude patterns
	excludeRegexes := make([]*regexp.Regexp, 0, len(excludePatterns))
	for _, pattern := range excludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			excludeRegexes = append(excludeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var crossDomainLinks, excludedLinks []string

	for _, link := range links {
		// Check same-host restriction
		if baseHost != "" {
			if parsedLink, err := url.Parse(link); err == nil {
				linkHost := strings.ToLower(parsedLink.Host)
				if linkHost != "" && linkHost != baseHost {
					crossDomainLinks = append(crossDomainLinks, link)
					contextLogger.Debug().
						Str("link", link).
						Str("link_host", linkHost).
						Str("base_host", baseHost).
						Msg("Jira link excluded - cross-domain")
					continue // Skip cross-domain links
				}
			}
		}

		// Check exclude patterns first
		excluded := false
		var matchedExcludePattern string
		for _, re := range excludeRegexes {
			if re.MatchString(link) {
				excluded = true
				matchedExcludePattern = re.String()
				break
			}
		}
		if excluded {
			excludedLinks = append(excludedLinks, link)
			contextLogger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Jira link excluded by pattern")
			continue
		}

		// Accept all non-excluded links (include filtering handled by filterLinks)
		filtered = append(filtered, link)
	}

	// Summary log for Jira filtering
	if len(crossDomainLinks) > 0 || len(excludedLinks) > 0 {
		contextLogger.Debug().
			Int("cross_domain_count", len(crossDomainLinks)).
			Int("excluded_count", len(excludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Jira link filtering summary")
	}

	return filtered
}

// filterConfluenceLinks filters links to exclude non-content Confluence URLs on the same host
// Only applies universal exclude patterns. Include pattern matching is handled by filterLinks().
func (s *Service) filterConfluenceLinks(contextLogger arbor.ILogger, links []string, baseHost string, config CrawlConfig) []string {
	// Exclude patterns for non-content pages (universal exclusions for Confluence)
	excludePatterns := []string{
		`(?i)/rest/api/`,             // API endpoints (case-insensitive)
		`(?i)/download/attachments/`, // File downloads (case-insensitive)
		`(?i)/download/thumbnails/`,  // Thumbnail downloads (case-insensitive)
		`(?i)/(?:wiki/)?admin/`,      // Admin pages (case-insensitive)
		`(?i)/(?:wiki/)?people/`,     // User profiles (case-insensitive)
	}

	// Precompile exclude patterns
	excludeRegexes := make([]*regexp.Regexp, 0, len(excludePatterns))
	for _, pattern := range excludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			excludeRegexes = append(excludeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var crossDomainLinks, excludedLinks []string

	for _, link := range links {
		// Check same-host restriction
		if baseHost != "" {
			if parsedLink, err := url.Parse(link); err == nil {
				linkHost := strings.ToLower(parsedLink.Host)
				if linkHost != "" && linkHost != baseHost {
					crossDomainLinks = append(crossDomainLinks, link)
					contextLogger.Debug().
						Str("link", link).
						Str("link_host", linkHost).
						Str("base_host", baseHost).
						Msg("Confluence link excluded - cross-domain")
					continue // Skip cross-domain links
				}
			}
		}

		// Check exclude patterns first
		excluded := false
		var matchedExcludePattern string
		for _, re := range excludeRegexes {
			if re.MatchString(link) {
				excluded = true
				matchedExcludePattern = re.String()
				break
			}
		}
		if excluded {
			excludedLinks = append(excludedLinks, link)
			contextLogger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Confluence link excluded by pattern")
			continue
		}

		// Accept all non-excluded links (include filtering handled by filterLinks)
		filtered = append(filtered, link)
	}

	// Summary log for Confluence filtering
	if len(crossDomainLinks) > 0 || len(excludedLinks) > 0 {
		contextLogger.Debug().
			Int("cross_domain_count", len(crossDomainLinks)).
			Int("excluded_count", len(excludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Confluence link filtering summary")
	}

	return filtered
}
