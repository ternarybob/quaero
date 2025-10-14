package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/workers"
)

// Service orchestrates URL queue, rate limiting, retries, and worker pool
type Service struct {
	authService  interfaces.AuthService
	eventService interfaces.EventService
	jobStorage   interfaces.JobStorage
	logger       arbor.ILogger

	queue       *URLQueue
	rateLimiter *RateLimiter
	retryPolicy *RetryPolicy
	workerPool  *workers.Pool

	activeJobs map[string]*CrawlJob
	jobResults map[string][]*CrawlResult
	jobsMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new crawler service
func NewService(authService interfaces.AuthService, eventService interfaces.EventService, jobStorage interfaces.JobStorage, logger arbor.ILogger, config CrawlConfig) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		authService:  authService,
		eventService: eventService,
		jobStorage:   jobStorage,
		logger:       logger,
		queue:        NewURLQueue(),
		rateLimiter:  NewRateLimiter(config.RateLimit),
		retryPolicy:  NewRetryPolicy(),
		workerPool:   workers.NewPool(config.Concurrency, logger),
		activeJobs:   make(map[string]*CrawlJob),
		jobResults:   make(map[string][]*CrawlResult),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Override retry policy if config specifies
	if config.RetryAttempts > 0 {
		s.retryPolicy.MaxAttempts = config.RetryAttempts
	}
	if config.RetryBackoff > 0 {
		s.retryPolicy.InitialBackoff = config.RetryBackoff
	}

	return s
}

// Start starts the crawler service
func (s *Service) Start() error {
	s.logger.Info().Msg("Crawler service started")
	return nil
}

// StartCrawl creates a job, seeds queue, starts workers, emits started event
func (s *Service) StartCrawl(sourceType, entityType string, seedURLs []string, config CrawlConfig) (string, error) {
	jobID := uuid.New().String()

	job := &CrawlJob{
		ID:         jobID,
		SourceType: sourceType,
		EntityType: entityType,
		Config:     config,
		Status:     JobStatusPending,
		Progress: CrawlProgress{
			TotalURLs:     len(seedURLs),
			CompletedURLs: 0,
			FailedURLs:    0,
			PendingURLs:   len(seedURLs),
			StartTime:     time.Now(),
		},
		CreatedAt: time.Now(),
	}

	// Persist job to database
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to persist job to database")
			return "", fmt.Errorf("failed to save job: %w", err)
		}
	}

	s.jobsMu.Lock()
	s.activeJobs[jobID] = job
	s.jobResults[jobID] = make([]*CrawlResult, 0)
	s.jobsMu.Unlock()

	// Seed queue
	for i, url := range seedURLs {
		item := &URLQueueItem{
			URL:      url,
			Depth:    0,
			Priority: i,
			AddedAt:  time.Now(),
			Metadata: map[string]interface{}{
				"job_id": jobID,
			},
		}
		s.queue.Push(item)
	}

	// Update job status
	s.jobsMu.Lock()
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()
	s.jobsMu.Unlock()

	// Persist status update
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job status in database")
		}
	}

	// Emit started event
	s.emitProgress(job)

	// Start workers
	s.startWorkers(jobID, config)

	return jobID, nil
}

// GetJobStatus returns the current status of a job
func (s *Service) GetJobStatus(jobID string) (*CrawlJob, error) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	return job, nil
}

// CancelJob cancels a running job
func (s *Service) CancelJob(jobID string) error {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status != JobStatusRunning {
		return fmt.Errorf("job is not running: %s", job.Status)
	}

	job.Status = JobStatusCancelled
	job.CompletedAt = time.Now()

	s.emitProgress(job)

	return nil
}

// GetJobResults returns the results of a completed job
func (s *Service) GetJobResults(jobID string) ([]*CrawlResult, error) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	results, exists := s.jobResults[jobID]
	if !exists {
		return nil, fmt.Errorf("job results not found: %s", jobID)
	}

	return results, nil
}

// startWorkers launches worker goroutines for a job
func (s *Service) startWorkers(jobID string, config CrawlConfig) {
	for i := 0; i < config.Concurrency; i++ {
		s.wg.Add(1)
		go s.workerLoop(jobID, config)
	}

	// Monitor completion
	go s.monitorCompletion(jobID)
}

// workerLoop processes URLs from the queue
func (s *Service) workerLoop(jobID string, config CrawlConfig) {
	defer s.wg.Done()

	for {
		// Check if job is still active
		s.jobsMu.RLock()
		job, exists := s.activeJobs[jobID]
		if !exists || job.Status == JobStatusCancelled || job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
			s.jobsMu.RUnlock()
			return
		}
		s.jobsMu.RUnlock()

		// Check max pages limit
		if config.MaxPages > 0 && job.Progress.CompletedURLs >= config.MaxPages {
			s.logger.Debug().
				Str("job_id", jobID).
				Int("completed", job.Progress.CompletedURLs).
				Int("max_pages", config.MaxPages).
				Msg("Max pages reached, stopping worker")
			return
		}

		// Pop URL from queue (blocking with timeout)
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		item, err := s.queue.Pop(ctx)
		cancel()

		if err != nil {
			if err == context.DeadlineExceeded {
				// No items available, check if job is complete
				continue
			}
			s.logger.Debug().Err(err).Msg("Error popping from queue")
			return
		}

		if item == nil {
			// Queue closed
			return
		}

		// Check depth limit
		if config.MaxDepth > 0 && item.Depth > config.MaxDepth {
			s.logger.Debug().
				Str("url", item.URL).
				Int("depth", item.Depth).
				Int("max_depth", config.MaxDepth).
				Msg("Skipping URL beyond max depth")
			continue
		}

		// Update current URL
		s.updateCurrentURL(jobID, item.URL)

		// Apply rate limiting
		if err := s.rateLimiter.Wait(s.ctx, item.URL); err != nil {
			s.logger.Debug().Err(err).Str("url", item.URL).Msg("Rate limiter cancelled")
			return
		}

		// Execute request with retry
		result := s.executeRequest(item, config)

		// Store result
		s.jobsMu.Lock()
		s.jobResults[jobID] = append(s.jobResults[jobID], result)
		s.jobsMu.Unlock()

		// Update progress
		if result.Error == "" {
			s.updateProgress(jobID, true, false)

			// Discover links if enabled
			if config.FollowLinks && item.Depth < config.MaxDepth {
				links := s.discoverLinks(result, item, config)
				s.enqueueLinks(jobID, links, item)
			}
		} else {
			s.updateProgress(jobID, false, true)
		}

		// Emit progress periodically (every 10 URLs)
		if (job.Progress.CompletedURLs+job.Progress.FailedURLs)%10 == 0 {
			s.emitProgress(job)
		}
	}
}

// executeRequest wraps makeRequest with retry policy
func (s *Service) executeRequest(item *URLQueueItem, config CrawlConfig) *CrawlResult {
	startTime := time.Now()

	statusCode, err := s.retryPolicy.ExecuteWithRetry(s.ctx, s.logger, func() (int, error) {
		return s.makeRequest(item, config)
	})

	duration := time.Since(startTime)

	result := &CrawlResult{
		URL:      item.URL,
		Duration: duration,
		Metadata: item.Metadata,
	}

	if err != nil {
		result.Error = err.Error()
		result.StatusCode = statusCode
		s.logger.Debug().
			Str("url", item.URL).
			Int("status_code", statusCode).
			Err(err).
			Msg("Request failed after retries")
	} else {
		result.StatusCode = statusCode
	}

	return result
}

// makeRequest performs HTTP request with auth
func (s *Service) makeRequest(item *URLQueueItem, config CrawlConfig) (int, error) {
	req, err := http.NewRequestWithContext(s.ctx, "GET", item.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - auth will be handled by the HTTP client with cookies
	req.Header.Set("User-Agent", "Quaero/1.0")
	req.Header.Set("Accept", "application/json")

	// Use auth service's HTTP client which has cookies configured
	client := s.authService.GetHTTPClient()
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	// Store body in result metadata for later processing
	if item.Metadata == nil {
		item.Metadata = make(map[string]interface{})
	}
	item.Metadata["response_body"] = body
	item.Metadata["status_code"] = resp.StatusCode

	if resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}

// discoverLinks extracts links from JSON responses
func (s *Service) discoverLinks(result *CrawlResult, parent *URLQueueItem, config CrawlConfig) []string {
	links := make([]string, 0)

	// Get response body from metadata
	bodyRaw, ok := parent.Metadata["response_body"]
	if !ok {
		return links
	}

	body, ok := bodyRaw.([]byte)
	if !ok {
		return links
	}

	// Parse JSON response
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		s.logger.Debug().Err(err).Msg("Failed to parse JSON response for link discovery")
		return links
	}

	// Extract links based on source type
	sourceType := ""
	if st, ok := parent.Metadata["source_type"].(string); ok {
		sourceType = st
	}

	switch sourceType {
	case "confluence":
		links = s.extractConfluenceLinks(data, parent.URL)
	case "jira":
		links = s.extractJiraLinks(data, parent.URL)
	}

	// Apply include/exclude patterns
	links = s.filterLinks(links, config)

	return links
}

// extractConfluenceLinks extracts page links from Confluence API responses
func (s *Service) extractConfluenceLinks(data map[string]interface{}, baseURL string) []string {
	links := make([]string, 0)

	// Extract from _links.next for pagination
	if linksObj, ok := data["_links"].(map[string]interface{}); ok {
		if next, ok := linksObj["next"].(string); ok {
			links = append(links, makeAbsoluteURL(baseURL, next))
		}
	}

	// Extract child pages from body.storage.value
	if body, ok := data["body"].(map[string]interface{}); ok {
		if storage, ok := body["storage"].(map[string]interface{}); ok {
			if value, ok := storage["value"].(string); ok {
				// Extract page links from HTML
				pageLinks := extractHTMLLinks(value, baseURL)
				links = append(links, pageLinks...)
			}
		}
	}

	return links
}

// extractJiraLinks extracts issue links from Jira API responses
func (s *Service) extractJiraLinks(data map[string]interface{}, baseURL string) []string {
	links := make([]string, 0)

	// Extract pagination (startAt, maxResults, total)
	if total, ok := data["total"].(float64); ok {
		if startAt, ok := data["startAt"].(float64); ok {
			if maxResults, ok := data["maxResults"].(float64); ok {
				nextStartAt := int(startAt) + int(maxResults)
				if nextStartAt < int(total) {
					// Build next page URL
					nextURL := fmt.Sprintf("%s&startAt=%d", baseURL, nextStartAt)
					links = append(links, nextURL)
				}
			}
		}
	}

	return links
}

// filterLinks applies include/exclude patterns
func (s *Service) filterLinks(links []string, config CrawlConfig) []string {
	filtered := make([]string, 0)

	for _, link := range links {
		// Apply exclude patterns
		excluded := false
		for _, pattern := range config.ExcludePatterns {
			if matched, _ := regexp.MatchString(pattern, link); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Apply include patterns (if any)
		if len(config.IncludePatterns) > 0 {
			included := false
			for _, pattern := range config.IncludePatterns {
				if matched, _ := regexp.MatchString(pattern, link); matched {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		filtered = append(filtered, link)
	}

	return filtered
}

// enqueueLinks adds discovered links to queue with depth tracking
func (s *Service) enqueueLinks(jobID string, links []string, parent *URLQueueItem) {
	for i, link := range links {
		item := &URLQueueItem{
			URL:       link,
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Priority:  parent.Priority + i + 1,
			AddedAt:   time.Now(),
			Metadata: map[string]interface{}{
				"job_id": jobID,
			},
		}

		if s.queue.Push(item) {
			s.updatePendingCount(jobID, 1)
		}
	}
}

// updateProgress updates job progress stats
func (s *Service) updateProgress(jobID string, success bool, failed bool) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	if success {
		job.Progress.CompletedURLs++
	}
	if failed {
		job.Progress.FailedURLs++
	}

	job.Progress.PendingURLs = s.queue.Len()
	job.Progress.Percentage = float64(job.Progress.CompletedURLs+job.Progress.FailedURLs) / float64(job.Progress.TotalURLs) * 100

	// Estimate completion
	elapsed := time.Since(job.Progress.StartTime)
	if job.Progress.CompletedURLs > 0 {
		avgTime := elapsed / time.Duration(job.Progress.CompletedURLs)
		remaining := job.Progress.TotalURLs - job.Progress.CompletedURLs - job.Progress.FailedURLs
		job.Progress.EstimatedCompletion = time.Now().Add(avgTime * time.Duration(remaining))
	}
}

// updateCurrentURL updates the current URL being processed
func (s *Service) updateCurrentURL(jobID string, url string) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	job.Progress.CurrentURL = url
}

// updatePendingCount updates pending URL count
func (s *Service) updatePendingCount(jobID string, delta int) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	job, exists := s.activeJobs[jobID]
	if !exists {
		return
	}

	job.Progress.TotalURLs += delta
	job.Progress.PendingURLs += delta
}

// emitProgress publishes progress event
func (s *Service) emitProgress(job *CrawlJob) {
	payload := map[string]interface{}{
		"job_id":               job.ID,
		"source_type":          job.SourceType,
		"entity_type":          job.EntityType,
		"status":               string(job.Status),
		"total_urls":           job.Progress.TotalURLs,
		"completed_urls":       job.Progress.CompletedURLs,
		"failed_urls":          job.Progress.FailedURLs,
		"pending_urls":         job.Progress.PendingURLs,
		"current_url":          job.Progress.CurrentURL,
		"percentage":           job.Progress.Percentage,
		"estimated_completion": job.Progress.EstimatedCompletion,
	}

	event := interfaces.Event{
		Type:    interfaces.EventCrawlProgress,
		Payload: payload,
	}

	if err := s.eventService.Publish(s.ctx, event); err != nil {
		s.logger.Debug().Err(err).Msg("Failed to publish crawl progress event")
	}
}

// monitorCompletion monitors job completion
func (s *Service) monitorCompletion(jobID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.jobsMu.RLock()
			job, exists := s.activeJobs[jobID]
			if !exists {
				s.jobsMu.RUnlock()
				return
			}

			// Check if job is complete
			if job.Status == JobStatusRunning && s.queue.Len() == 0 && job.Progress.CompletedURLs+job.Progress.FailedURLs >= job.Progress.TotalURLs {
				s.jobsMu.RUnlock()

				// Mark job as completed
				s.jobsMu.Lock()
				job.Status = JobStatusCompleted
				job.CompletedAt = time.Now()
				job.ResultCount = job.Progress.CompletedURLs
				job.FailedCount = job.Progress.FailedURLs
				s.jobsMu.Unlock()

				// Persist job completion to database
				if s.jobStorage != nil {
					if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to persist job completion")
					}
				}

				s.emitProgress(job)

				s.logger.Info().
					Str("job_id", jobID).
					Int("completed", job.Progress.CompletedURLs).
					Int("failed", job.Progress.FailedURLs).
					Msg("Crawl job completed")

				return
			}
			s.jobsMu.RUnlock()
		}
	}
}

// ListJobs returns a list of jobs with optional filtering
func (s *Service) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]*CrawlJob, error) {
	if s.jobStorage == nil {
		return nil, fmt.Errorf("job storage not configured")
	}

	jobs, err := s.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Convert []interface{} to []*CrawlJob
	crawlJobs := make([]*CrawlJob, 0, len(jobs))
	for _, j := range jobs {
		if job, ok := j.(*CrawlJob); ok {
			crawlJobs = append(crawlJobs, job)
		}
	}

	return crawlJobs, nil
}

// RerunJob re-executes a previous job with the same or updated configuration
func (s *Service) RerunJob(ctx context.Context, jobID string, updateConfig *CrawlConfig) (string, error) {
	if s.jobStorage == nil {
		return "", fmt.Errorf("job storage not configured")
	}

	// Get original job from database
	jobInterface, err := s.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	originalJob, ok := jobInterface.(*CrawlJob)
	if !ok {
		return "", fmt.Errorf("invalid job type")
	}

	// Use original config or updated config
	config := originalJob.Config
	if updateConfig != nil {
		config = *updateConfig
	}

	// Extract seed URLs from original job (this would need to be stored in config)
	// For now, we'll need to reconstruct or store seed URLs in the job
	// This is a limitation of the current design - we should store seed URLs in the job
	seedURLs := []string{} // TODO: Store seed URLs in job config for rerun capability

	// Start new crawl with original parameters
	newJobID, err := s.StartCrawl(originalJob.SourceType, originalJob.EntityType, seedURLs, config)
	if err != nil {
		return "", fmt.Errorf("failed to start crawl: %w", err)
	}

	s.logger.Info().
		Str("original_job_id", jobID).
		Str("new_job_id", newJobID).
		Msg("Job rerun started")

	return newJobID, nil
}

// WaitForJob blocks until a job completes or context is cancelled
func (s *Service) WaitForJob(ctx context.Context, jobID string) ([]*CrawlResult, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			job, err := s.GetJobStatus(jobID)
			if err != nil {
				return nil, fmt.Errorf("failed to get job status: %w", err)
			}

			// Check if job is complete
			if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
				results, err := s.GetJobResults(jobID)
				if err != nil {
					return nil, fmt.Errorf("failed to get job results: %w", err)
				}
				return results, nil
			}
		}
	}
}

// Shutdown stops the crawler service
func (s *Service) Shutdown() error {
	s.cancel()
	s.queue.Close()
	s.wg.Wait()
	s.logger.Info().Msg("Crawler service stopped")
	return nil
}

// Close cleans up resources
func (s *Service) Close() error {
	return s.Shutdown()
}

// Helper functions

func makeAbsoluteURL(base, relative string) string {
	if strings.HasPrefix(relative, "http://") || strings.HasPrefix(relative, "https://") {
		return relative
	}

	if strings.HasPrefix(relative, "/") {
		// Parse base URL to get scheme and host
		if idx := strings.Index(base, "//"); idx != -1 {
			if idx2 := strings.Index(base[idx+2:], "/"); idx2 != -1 {
				return base[:idx+2+idx2] + relative
			}
		}
	}

	return base + relative
}

func extractHTMLLinks(html, baseURL string) []string {
	links := make([]string, 0)

	// Simple regex to extract href attributes
	re := regexp.MustCompile(`href=["']([^"']+)["']`)
	matches := re.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) > 1 {
			link := makeAbsoluteURL(baseURL, match[1])
			links = append(links, link)
		}
	}

	return links
}
