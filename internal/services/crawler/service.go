package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/sources"
	"github.com/ternarybob/quaero/internal/services/workers"
)

// Service orchestrates URL queue, retries, and worker pool
// Rate limiting strategy: Rate limiting is exclusively managed by HTMLScraper via common.CrawlerConfig mapping.
// Per-job rate limits are applied via Colly's built-in Limit() mechanism in HTMLScraper to avoid double rate limiting
// and leverage Colly's efficient per-domain parallelism control.
type Service struct {
	authService   interfaces.AuthService
	sourceService *sources.Service
	authStorage   interfaces.AuthStorage
	eventService  interfaces.EventService
	jobStorage    interfaces.JobStorage
	logger        arbor.ILogger
	config        *common.Config

	queue       *URLQueue
	retryPolicy *RetryPolicy
	workerPool  *workers.Pool

	activeJobs map[string]*CrawlJob
	jobResults map[string][]*CrawlResult
	jobClients map[string]*http.Client // Per-job HTTP clients built from auth snapshots
	jobsMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new crawler service
func NewService(authService interfaces.AuthService, sourceService *sources.Service, authStorage interfaces.AuthStorage, eventService interfaces.EventService, jobStorage interfaces.JobStorage, logger arbor.ILogger, config *common.Config) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		authService:   authService,
		sourceService: sourceService,
		authStorage:   authStorage,
		eventService:  eventService,
		jobStorage:    jobStorage,
		logger:        logger,
		config:        config,
		queue:         NewURLQueue(),
		retryPolicy:   NewRetryPolicy(),
		workerPool:    workers.NewPool(config.Crawler.MaxConcurrency, logger),
		activeJobs:    make(map[string]*CrawlJob),
		jobResults:    make(map[string][]*CrawlResult),
		jobClients:    make(map[string]*http.Client),
		ctx:           ctx,
		cancel:        cancel,
	}

	return s
}

// Start starts the crawler service
func (s *Service) Start() error {
	s.logger.Info().Msg("Crawler service started")
	return nil
}

// logToDatabase appends a log entry to the job's database logs with consistent error handling
func (s *Service) logToDatabase(jobID string, level string, message string) {
	if s.jobStorage == nil {
		return
	}

	logEntry := models.JobLogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     level,
		Message:   message,
	}

	if err := s.jobStorage.AppendJobLog(s.ctx, jobID, logEntry); err != nil {
		s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append log to database")
	}
}

// logInfoToConsoleAndDatabase logs info level to both console and database
func (s *Service) logInfoToConsoleAndDatabase(jobID string, message string, fields map[string]interface{}) {
	// Build console log
	event := s.logger.Info()
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		default:
			// Convert other types to string
			event = event.Str(key, fmt.Sprintf("%v", v))
		}
	}
	event.Msg(message)

	// Log to database
	s.logToDatabase(jobID, "info", message)
}

// logWarnToConsoleAndDatabase logs warning level to both console and database
func (s *Service) logWarnToConsoleAndDatabase(jobID string, message string, fields map[string]interface{}) {
	// Build console log
	event := s.logger.Warn()
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		default:
			// Convert other types to string
			event = event.Str(key, fmt.Sprintf("%v", v))
		}
	}
	event.Msg(message)

	// Log to database
	s.logToDatabase(jobID, "warn", message)
}

// logErrorToConsoleAndDatabase logs error level to both console and database
func (s *Service) logErrorToConsoleAndDatabase(jobID string, message string, err error, fields map[string]interface{}) {
	// Build console log
	event := s.logger.Error()
	if err != nil {
		event = event.Err(err)
	}
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		default:
			// Convert other types to string
			event = event.Str(key, fmt.Sprintf("%v", v))
		}
	}
	event.Msg(message)

	// Log to database
	s.logToDatabase(jobID, "error", message)
}

// StartCrawl creates a job, seeds queue, starts workers, emits started event
func (s *Service) StartCrawl(sourceType, entityType string, seedURLs []string, configInterface interface{}, sourceID string, refreshSource bool, sourceConfigSnapshotInterface interface{}, authSnapshotInterface interface{}) (string, error) {
	// Type assert config
	config, ok := configInterface.(CrawlConfig)
	if !ok {
		return "", fmt.Errorf("invalid config type: expected CrawlConfig")
	}

	// Type assert source config snapshot (can be nil)
	var sourceConfigSnapshot *models.SourceConfig
	if sourceConfigSnapshotInterface != nil {
		snapshot, ok := sourceConfigSnapshotInterface.(*models.SourceConfig)
		if !ok {
			return "", fmt.Errorf("invalid source config snapshot type: expected *models.SourceConfig")
		}
		sourceConfigSnapshot = snapshot
	}

	// Type assert auth snapshot (can be nil)
	var authSnapshot *models.AuthCredentials
	if authSnapshotInterface != nil {
		snapshot, ok := authSnapshotInterface.(*models.AuthCredentials)
		if !ok {
			return "", fmt.Errorf("invalid auth snapshot type: expected *models.AuthCredentials")
		}
		authSnapshot = snapshot
	}

	jobID := uuid.New().String()

	job := &CrawlJob{
		ID:            jobID,
		SourceType:    sourceType,
		EntityType:    entityType,
		Config:        config,
		RefreshSource: refreshSource,
		SeedURLs:      seedURLs, // Store seed URLs for rerun capability
		Status:        JobStatusPending,
		Progress: CrawlProgress{
			TotalURLs:     len(seedURLs),
			CompletedURLs: 0,
			FailedURLs:    0,
			PendingURLs:   len(seedURLs),
			StartTime:     time.Now(),
		},
		CreatedAt: time.Now(),
	}

	// Validate seed URLs and detect test URLs
	testURLCount := 0
	var testURLWarnings []string
	for _, seedURL := range seedURLs {
		isValid, isTestURL, warnings, err := common.ValidateBaseURL(seedURL, s.logger)
		if !isValid || err != nil {
			s.logger.Warn().
				Err(err).
				Str("seed_url", seedURL).
				Str("job_id", jobID).
				Msg("Invalid seed URL detected")
			s.logToDatabase(jobID, "warn", fmt.Sprintf("Invalid seed URL: %s - %v", seedURL, err))
		}
		if isTestURL {
			testURLCount++
			testURLWarnings = append(testURLWarnings, warnings...)
		}
	}

	// Reject test URLs in production mode
	if s.config.IsProduction() && testURLCount > 0 {
		errMsg := fmt.Sprintf("Test URLs are not allowed in production mode: %d of %d seed URLs are test URLs (localhost/127.0.0.1). Set environment=\"development\" in config to allow test URLs.", testURLCount, len(seedURLs))
		s.logger.Error().
			Str("job_id", jobID).
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Strs("warnings", testURLWarnings).
			Msg("Job rejected: test URLs detected in production mode")
		s.logToDatabase(jobID, "error", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	// Log test URL warnings if any detected (development mode)
	if testURLCount > 0 {
		warningMsg := fmt.Sprintf("Test URLs detected: %d of %d seed URLs are test URLs (localhost/127.0.0.1) - allowed in development mode",
			testURLCount, len(seedURLs))
		s.logger.Warn().
			Str("job_id", jobID).
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Strs("warnings", testURLWarnings).
			Msg(warningMsg)
		s.logToDatabase(jobID, "warn", warningMsg)
	}

	// Log seed URLs configuration (truncate if > 5 URLs)
	seedURLsMsg := fmt.Sprintf("Seed URLs configured: %d total", len(seedURLs))
	if len(seedURLs) > 0 && len(seedURLs) <= 5 {
		seedURLsMsg += fmt.Sprintf(" - %v", seedURLs)
	} else if len(seedURLs) > 5 {
		seedURLsMsg += fmt.Sprintf(" - First 5: %v (and %d more)", seedURLs[:5], len(seedURLs)-5)
	}
	// Append test URL warning if any detected
	if testURLCount > 0 {
		seedURLsMsg += fmt.Sprintf(" ⚠️  WARNING: %d test URLs detected (localhost/127.0.0.1)", testURLCount)
	}
	s.logInfoToConsoleAndDatabase(jobID, seedURLsMsg, map[string]interface{}{
		"seed_url_count": len(seedURLs),
		"test_url_count": testURLCount,
	})

	// Log crawler configuration summary
	configMsg := fmt.Sprintf("Crawler configuration: max_depth=%d, max_pages=%d, concurrency=%d, rate_limit=%dms, follow_links=%v",
		config.MaxDepth, config.MaxPages, config.Concurrency, config.RateLimit.Milliseconds(), config.FollowLinks)
	s.logInfoToConsoleAndDatabase(jobID, configMsg, map[string]interface{}{
		"max_depth":    config.MaxDepth,
		"max_pages":    config.MaxPages,
		"concurrency":  config.Concurrency,
		"rate_limit":   config.RateLimit.Milliseconds(),
		"follow_links": config.FollowLinks,
	})

	// Handle snapshot logic
	// If sourceID is provided and snapshots are nil, fetch from services
	if sourceID != "" && sourceConfigSnapshot == nil {
		if s.sourceService != nil {
			sourceConfig, err := s.sourceService.GetSource(s.ctx, sourceID)
			if err != nil {
				return "", fmt.Errorf("failed to fetch source config: %w", err)
			}
			sourceConfigSnapshot = sourceConfig

			// Fetch auth if source has auth_id
			if sourceConfig.AuthID != "" && s.authStorage != nil {
				auth, err := s.authStorage.GetCredentialsByID(s.ctx, sourceConfig.AuthID)
				if err != nil {
					s.logger.Warn().Err(err).Str("auth_id", sourceConfig.AuthID).Msg("Failed to fetch auth credentials")
					// Persist auth fetch failure to database
					authFailMsg := fmt.Sprintf("Failed to fetch auth credentials: auth_id=%s, error=%v", sourceConfig.AuthID, err)
					s.logToDatabase(jobID, "warn", authFailMsg)
				} else {
					authSnapshot = auth
				}
			}
		}
	}

	// If refreshSource is true, re-fetch latest config and auth
	if refreshSource && sourceID != "" && s.sourceService != nil {
		latestConfig, err := s.sourceService.GetSource(s.ctx, sourceID)
		if err != nil {
			return "", fmt.Errorf("failed to refresh source config: %w", err)
		}

		// Validate latest config
		if err := latestConfig.Validate(); err != nil {
			return "", fmt.Errorf("source configuration validation failed: %w", err)
		}

		sourceConfigSnapshot = latestConfig

		// Re-fetch auth if present
		if latestConfig.AuthID != "" && s.authStorage != nil {
			latestAuth, err := s.authStorage.GetCredentialsByID(s.ctx, latestConfig.AuthID)
			if err != nil {
				s.logger.Warn().Err(err).Str("auth_id", latestConfig.AuthID).Msg("Failed to refresh auth credentials")
				// Persist auth refresh failure to database
				authFailMsg := fmt.Sprintf("Failed to refresh auth credentials: auth_id=%s, error=%v", latestConfig.AuthID, err)
				s.logToDatabase(jobID, "warn", authFailMsg)
			} else {
				authSnapshot = latestAuth
			}
		}
	}

	// Validate source config snapshot if provided
	if sourceConfigSnapshot != nil {
		if err := sourceConfigSnapshot.Validate(); err != nil {
			// Log validation failure before returning error
			s.logErrorToConsoleAndDatabase(jobID, "Source config validation failed", err, map[string]interface{}{
				"error": err.Error(),
			})
			return "", fmt.Errorf("source configuration validation failed: %w", err)
		}

		// Store snapshot in job
		if err := job.SetSourceConfigSnapshot(sourceConfigSnapshot); err != nil {
			// Log snapshot serialization failure
			s.logErrorToConsoleAndDatabase(jobID, "Failed to serialize source config snapshot", err, map[string]interface{}{
				"error": err.Error(),
			})
			return "", fmt.Errorf("failed to set source config snapshot: %w", err)
		}

		// Log validation success with base URL
		baseURLInfo := "unknown"
		if sourceConfigSnapshot.BaseURL != "" {
			baseURLInfo = sourceConfigSnapshot.BaseURL
		}
		s.logInfoToConsoleAndDatabase(jobID, fmt.Sprintf("Source config validated and stored: base_url=%s", baseURLInfo), map[string]interface{}{
			"base_url": baseURLInfo,
		})
	} else {
		// Log missing source config snapshot
		s.logToDatabase(jobID, "info", "No source config snapshot provided")
	}

	// Store auth snapshot if provided
	var httpClientType string
	if authSnapshot != nil {
		if err := job.SetAuthSnapshot(authSnapshot); err != nil {
			// Log auth snapshot serialization failure
			s.logErrorToConsoleAndDatabase(jobID, "Failed to serialize auth snapshot", err, map[string]interface{}{
				"error": err.Error(),
			})
			return "", fmt.Errorf("failed to set auth snapshot: %w", err)
		}

		// Log auth snapshot presence
		cookieCount := 0
		if authSnapshot.Cookies != nil {
			var cookies []*interfaces.AtlassianExtensionCookie
			if err := json.Unmarshal(authSnapshot.Cookies, &cookies); err == nil {
				cookieCount = len(cookies)
			}
		}
		s.logInfoToConsoleAndDatabase(jobID, fmt.Sprintf("Auth snapshot stored: %d cookies available", cookieCount), map[string]interface{}{
			"cookie_count": cookieCount,
		})

		// Build HTTP client from auth snapshot for this job
		client, err := buildHTTPClientFromAuth(authSnapshot)
		if err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to build HTTP client from auth snapshot, will use default")
			s.logToDatabase(jobID, "warn", fmt.Sprintf("Failed to build HTTP client from auth: %v - will use default", err))
			httpClientType = "default (auth build failed)"
		} else {
			s.jobsMu.Lock()
			s.jobClients[jobID] = client
			s.jobsMu.Unlock()
			s.logger.Debug().Str("job_id", jobID).Msg("Per-job HTTP client configured from auth snapshot")
			httpClientType = "per-job (from auth snapshot)"
		}
	} else {
		// Log missing auth snapshot (Comment 3: downgrade to info if source doesn't declare AuthID)
		logLevel := "warn"
		if sourceConfigSnapshot == nil || sourceConfigSnapshot.AuthID == "" {
			logLevel = "info" // Auth not configured or optional for this source
		}
		s.logToDatabase(jobID, logLevel, "No auth snapshot provided - requests will use default HTTP client")
		httpClientType = "default (no auth)"
	}

	// Log HTTP client configuration
	s.logInfoToConsoleAndDatabase(jobID, fmt.Sprintf("HTTP client configured: type=%s", httpClientType), map[string]interface{}{
		"client_type": httpClientType,
	})

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

	// Seed queue with metadata for link discovery
	// Track how many URLs are actually added (excluding duplicates)
	actuallyEnqueued := 0
	for i, url := range seedURLs {
		item := &URLQueueItem{
			URL:      url,
			Depth:    0,
			Priority: i,
			AddedAt:  time.Now(),
			Metadata: map[string]interface{}{
				"job_id":      jobID,
				"source_type": sourceType,
				"entity_type": entityType,
			},
		}
		// Check if URL was actually added (not a duplicate)
		if s.queue.Push(item) {
			actuallyEnqueued++
		}
	}

	// Update PendingURLs and TotalURLs to match actual queue state
	s.jobsMu.Lock()
	job.Progress.PendingURLs = actuallyEnqueued
	job.Progress.TotalURLs = actuallyEnqueued
	s.jobsMu.Unlock()

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

		// Initialize heartbeat for the running job
		if err := s.jobStorage.UpdateJobHeartbeat(s.ctx, jobID); err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to initialize job heartbeat")
		}

		// Append detailed job initialization summary log
		authStatus := "no auth"
		if authSnapshot != nil {
			authStatus = "authenticated"
		}
		jobStartMsg := fmt.Sprintf("Job started: source=%s/%s, seeds=%d, max_depth=%d, max_pages=%d, concurrency=%d, auth=%s",
			sourceType, entityType, len(seedURLs), config.MaxDepth, config.MaxPages, config.Concurrency, authStatus)

		// Use helper for consistency with other log calls
		s.logToDatabase(jobID, "info", jobStartMsg)
	}

	// Emit started event
	s.emitProgress(job)

	// Start workers
	s.startWorkers(jobID, config)

	return jobID, nil
}

// GetJobStatus returns the current status of a job
func (s *Service) GetJobStatus(jobID string) (interface{}, error) {
	// Fast path: Check in-memory storage first (for running jobs)
	s.jobsMu.RLock()
	job, exists := s.activeJobs[jobID]
	s.jobsMu.RUnlock()

	if exists {
		return job, nil
	}

	// Database fallback: Query persistent storage for completed/failed/cancelled jobs
	if s.jobStorage != nil {
		jobInterface, err := s.jobStorage.GetJob(s.ctx, jobID)
		if err == nil {
			// Type assertion to convert interface{} to *CrawlJob
			if crawlJob, ok := jobInterface.(*CrawlJob); ok {
				s.logger.Debug().
					Str("job_id", jobID).
					Str("status", string(crawlJob.Status)).
					Msg("Retrieved job from database (not in active jobs)")
				return crawlJob, nil
			}
		} else {
			// Log non-"not found" errors as they indicate database issues
			errMsg := err.Error()
			if !strings.Contains(errMsg, "job not found") && !strings.Contains(errMsg, "not found") {
				s.logger.Warn().
					Err(err).
					Str("job_id", jobID).
					Msg("Database error while retrieving job")
				return nil, fmt.Errorf("database error retrieving job %s: %w", jobID, err)
			}
		}
	}

	return nil, fmt.Errorf("job not found: %s", jobID)
}

// CancelJob cancels a running job
func (s *Service) CancelJob(jobID string) error {
	// Acquire lock to check job and update status
	s.jobsMu.Lock()
	job, exists := s.activeJobs[jobID]
	if !exists {
		s.jobsMu.Unlock()
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.Status != JobStatusRunning {
		s.jobsMu.Unlock()
		return fmt.Errorf("job is not running: %s", job.Status)
	}

	job.Status = JobStatusCancelled
	job.CompletedAt = time.Now()
	s.jobsMu.Unlock()

	// Persist cancellation status to database (outside lock to avoid contention)
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to persist job cancellation")
		}

		// Append cancellation log with progress summary
		cancelMsg := fmt.Sprintf("Job cancelled by user: %d completed, %d failed, %d pending",
			job.Progress.CompletedURLs, job.Progress.FailedURLs, job.Progress.PendingURLs)
		s.logToDatabase(jobID, "warn", cancelMsg)
	}

	// Reacquire lock to clean up per-job HTTP client map and remove from activeJobs
	s.jobsMu.Lock()
	if _, exists := s.jobClients[jobID]; exists {
		delete(s.jobClients, jobID)
		s.logger.Debug().Str("job_id", jobID).Msg("Cleaned up per-job HTTP client after cancellation")
	}
	// Remove from activeJobs since job is now in terminal state
	delete(s.activeJobs, jobID)
	s.logger.Debug().Str("job_id", jobID).Msg("Removed cancelled job from active jobs")
	s.jobsMu.Unlock()

	// Emit progress after persistence
	s.emitProgress(job)

	return nil
}

// FailJob marks a job as failed with a reason (called by scheduler for stale job detection)
func (s *Service) FailJob(jobID string, reason string) error {
	// Acquire lock to check job and update status
	s.jobsMu.Lock()
	job, exists := s.activeJobs[jobID]
	if !exists {
		s.jobsMu.Unlock()
		return fmt.Errorf("job not found in active jobs: %s", jobID)
	}

	// Set job status to failed
	job.Status = JobStatusFailed
	job.CompletedAt = time.Now()
	job.Error = reason
	s.jobsMu.Unlock()

	// Persist failed status to database (outside lock to avoid contention)
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to persist job failure")
		}

		// Append failure log with progress summary
		failMsg := fmt.Sprintf("Job failed: %s - %d completed, %d failed, %d pending",
			reason, job.Progress.CompletedURLs, job.Progress.FailedURLs, job.Progress.PendingURLs)
		s.logToDatabase(jobID, "error", failMsg)
	}

	// Reacquire lock to clean up per-job HTTP client map and remove from activeJobs
	s.jobsMu.Lock()
	if _, exists := s.jobClients[jobID]; exists {
		delete(s.jobClients, jobID)
		s.logger.Debug().Str("job_id", jobID).Msg("Cleaned up per-job HTTP client after failure")
	}
	// Remove from activeJobs since job is now in terminal state
	delete(s.activeJobs, jobID)
	s.logger.Debug().Str("job_id", jobID).Str("reason", reason).Msg("Removed failed job from active jobs")
	s.jobsMu.Unlock()

	// Emit progress after persistence
	s.emitProgress(job)

	return nil
}

// GetJobResults returns the results of a completed job
func (s *Service) GetJobResults(jobID string) (interface{}, error) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	results, exists := s.jobResults[jobID]
	if !exists {
		return nil, fmt.Errorf("job results not found: %s", jobID)
	}

	return results, nil
}

// GetActiveJobIDs returns a list of all active job IDs
func (s *Service) GetActiveJobIDs() []string {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	jobIDs := make([]string, 0, len(s.activeJobs))
	for jobID := range s.activeJobs {
		jobIDs = append(jobIDs, jobID)
	}

	return jobIDs
}

// GetRunningJobIDs returns a list of job IDs with status = running
func (s *Service) GetRunningJobIDs() []string {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	jobIDs := make([]string, 0)
	for jobID, job := range s.activeJobs {
		if job.Status == JobStatusRunning {
			jobIDs = append(jobIDs, jobID)
		}
	}

	return jobIDs
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
	workerStartTime := time.Now()
	urlsProcessed := 0

	// Log worker start
	s.logger.Debug().
		Str("job_id", jobID).
		Int("concurrency", config.Concurrency).
		Msg("Worker started")

	// Defer worker exit logging
	defer func() {
		s.wg.Done()
		duration := time.Since(workerStartTime)
		s.logger.Debug().
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
		s.logger.Debug().
			Str("job_id", jobID).
			Int("pending_urls", job.Progress.PendingURLs).
			Int("completed_urls", job.Progress.CompletedURLs).
			Int("failed_urls", job.Progress.FailedURLs).
			Msg("Worker iteration - queue state")

		// Check max pages limit
		if config.MaxPages > 0 && job.Progress.CompletedURLs >= config.MaxPages {
			s.logger.Debug().
				Str("job_id", jobID).
				Int("completed", job.Progress.CompletedURLs).
				Int("max_pages", config.MaxPages).
				Msg("Max pages reached, stopping worker")

			// Use logToDatabase helper for consistency (Comment 2)
			maxPagesMsg := fmt.Sprintf("Max pages limit reached (%d/%d)", job.Progress.CompletedURLs, config.MaxPages)
			s.logToDatabase(jobID, "info", maxPagesMsg)
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
					s.logger.Debug().
						Str("job_id", jobID).
						Msg("Queue empty and no pending URLs - worker exiting gracefully")
					return
				}

				// Queue has items but timeout occurred - log warning and continue with backoff
				if queueLen > 0 {
					s.logger.Warn().
						Str("job_id", jobID).
						Int("queue_len", queueLen).
						Int("pending_urls", pendingURLs).
						Msg("Queue has items but Pop() timed out - possible queue health issue")
				}

				// Continue to retry
				continue
			}
			s.logger.Debug().Err(err).Msg("Error popping from queue")
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
		s.logger.Debug().
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
				s.logger.Debug().
					Str("url", item.URL).
					Int("depth", item.Depth).
					Int("max_depth", config.MaxDepth).
					Msg("Skipping URL beyond max depth")

				// Persist depth limit skip to database
				depthSkipMsg := fmt.Sprintf("Depth limit skip: url=%s, depth=%d, max_depth=%d", item.URL, item.Depth, config.MaxDepth)
				s.logToDatabase(jobID, "warn", depthSkipMsg)
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
		result := s.executeRequest(item, config)

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
			s.logger.Debug().
				Str("job_id", jobID).
				Str("url", result.URL).
				Int("status_code", result.StatusCode).
				Int("body_size", bodySize).
				Str("error", result.Error).
				Int("total_results", totalResults).
				Dur("duration", result.Duration).
				Msg("Result stored")
		}

		// Update progress
		if result.Error == "" {
			s.updateProgress(jobID, true, false)

			// Discover links if enabled and within depth limit (0 = unlimited depth)
			if config.FollowLinks && (config.MaxDepth == 0 || item.Depth < config.MaxDepth) {
				links := s.discoverLinks(result, item, config)
				s.enqueueLinks(jobID, links, item)
			} else {
				// Log and persist link discovery skip
				skipMsg := fmt.Sprintf("Link discovery skipped: follow_links=%v, depth=%d/%d", config.FollowLinks, item.Depth, config.MaxDepth)

				s.logger.Debug().
					Str("job_id", jobID).
					Str("url", item.URL).
					Str("follow_links", fmt.Sprintf("%v", config.FollowLinks)).
					Int("depth", item.Depth).
					Int("max_depth", config.MaxDepth).
					Msg("Skipping link discovery - FollowLinks disabled or depth limit reached")

				// Persist to database (sampled: every 20th skip to avoid log spam)
				s.jobsMu.RLock()
				completedCount := job.Progress.CompletedURLs
				s.jobsMu.RUnlock()

				if completedCount%20 == 0 {
					s.logToDatabase(jobID, "debug", skipMsg)
				}
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
					s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append request failure log")
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

			// Use logToDatabase helper for consistency (Comment 2)
			progressMsg := fmt.Sprintf("Progress: %d completed, %d failed, %d pending (success_rate=%.1f%%)",
				job.Progress.CompletedURLs, job.Progress.FailedURLs, job.Progress.PendingURLs, successRate)
			s.logToDatabase(jobID, "info", progressMsg)
		}
	}
}

// executeRequest wraps makeRequest with retry policy
func (s *Service) executeRequest(item *URLQueueItem, config CrawlConfig) *CrawlResult {
	startTime := time.Now()

	// Extract job ID for logging
	jobID := ""
	if item.Metadata != nil {
		if jid, ok := item.Metadata["job_id"].(string); ok {
			jobID = jid
		}
	}

	// Log request start
	s.logger.Debug().
		Str("job_id", jobID).
		Str("url", item.URL).
		Int("depth", item.Depth).
		Int("attempt", item.Attempts+1).
		Msg("Starting request with retry policy")

	statusCode, err := s.retryPolicy.ExecuteWithRetry(s.ctx, s.logger, func() (int, error) {
		return s.makeRequest(item, config)
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
		s.logger.Debug().
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
			s.logger.Warn().
				Str("job_id", jobID).
				Str("url", item.URL).
				Int("status_code", statusCode).
				Dur("duration", duration).
				Msg("Response body is empty despite HTTP 200 status")
		}

		// Log successful request completion
		s.logger.Debug().
			Str("job_id", jobID).
			Str("url", item.URL).
			Int("status_code", statusCode).
			Int("body_size", bodySize).
			Dur("duration", duration).
			Msg("Request completed successfully")
	}

	return result
}

// makeRequest performs HTML scraping using Colly-based HTMLScraper
func (s *Service) makeRequest(item *URLQueueItem, config CrawlConfig) (int, error) {
	startTime := time.Now()

	// Extract HTTP client and cookies for auth
	var client *http.Client
	var jobID string
	var clientType string // Track which client type was selected
	if jid, ok := item.Metadata["job_id"].(string); ok && jid != "" {
		jobID = jid
		s.jobsMu.RLock()
		client = s.jobClients[jobID]
		s.jobsMu.RUnlock()

		if client != nil {
			clientType = "per-job"
			s.logger.Debug().Str("url", item.URL).Str("job_id", jobID).Msg("Using per-job HTTP client with auth")
		}
	}

	// Fallback to auth service's HTTP client if no per-job client
	if client == nil {
		client = s.authService.GetHTTPClient()
		if client != nil {
			clientType = "auth-service"
			s.logger.Debug().Str("url", item.URL).Msg("Using auth service HTTP client")
		}
	}

	// Final fallback to default client
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
		clientType = "default"
		s.logger.Debug().Str("url", item.URL).Msg("Using default HTTP client (no auth)")
	}

	// Persist HTTP client selection to database (sampled: only at depth=0 to avoid spam)
	if jobID != "" && item.Depth == 0 {
		var clientMsg string
		switch clientType {
		case "per-job":
			clientMsg = fmt.Sprintf("Using per-job HTTP client with auth: url=%s, job_id=%s", item.URL, jobID)
		case "auth-service":
			clientMsg = fmt.Sprintf("Using auth service HTTP client: url=%s", item.URL)
		case "default":
			clientMsg = fmt.Sprintf("Using default HTTP client (no auth): url=%s", item.URL)
		default:
			clientMsg = fmt.Sprintf("HTTP client selected: type=%s, url=%s", clientType, item.URL)
		}
		s.logToDatabase(jobID, "debug", clientMsg)
	}

	// Extract cookies from client's cookie jar for the target URL
	cookies := s.extractCookiesFromClient(client, item.URL)
	s.logger.Debug().
		Str("url", item.URL).
		Int("cookie_count", len(cookies)).
		Msg("Extracted cookies from client")

	// Warn if no cookies available on seed URLs (depth 0) despite auth being configured
	// Only warn on initial requests to avoid noise - cookies may not be set for all URLs
	if jobID != "" && len(cookies) == 0 && item.Depth == 0 {
		s.jobsMu.RLock()
		_, hasClient := s.jobClients[jobID]
		s.jobsMu.RUnlock()

		if hasClient || client.Jar != nil {
			// Persist warning if auth is configured but cookies are missing on seed URL
			s.logToDatabase(jobID, "warn", fmt.Sprintf("No cookies available for seed URL %s despite auth configuration", item.URL))
		}
	}

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

	// Log scraper config (sampled: once per job by checking depth=0)
	if jobID != "" && item.Depth == 0 {
		scraperMsg := fmt.Sprintf("Scraper config: request_delay=%dms, max_concurrency=%d, max_depth=%d, user_agent=%s",
			scraperConfig.RequestDelay.Milliseconds(), scraperConfig.MaxConcurrency, scraperConfig.MaxDepth,
			func() string {
				if scraperConfig.UserAgent != "" {
					return scraperConfig.UserAgent[:min(50, len(scraperConfig.UserAgent))]
				}
				return "default"
			}())
		s.logToDatabase(jobID, "info", scraperMsg)
	}

	// Create HTMLScraper instance with merged config
	scraper := NewHTMLScraper(scraperConfig, s.logger, client, cookies)

	// Execute scraping
	scrapeResult, err := scraper.ScrapeURL(s.ctx, item.URL)
	if err != nil {
		// Check if context was cancelled
		if err == context.Canceled || err == context.DeadlineExceeded {
			s.logger.Debug().Err(err).Str("url", item.URL).Msg("Scraping cancelled")
			return 0, err
		}
		s.logger.Warn().Err(err).Str("url", item.URL).Msg("Scraping failed")
		return 0, fmt.Errorf("scraping failed: %w", err)
	}

	// Convert ScrapeResult to CrawlResult-compatible format
	crawlResult := scrapeResult.ToCrawlResult()

	// Check for scraper failures (Comment 3)
	if !scrapeResult.Success || crawlResult.Error != "" {
		// Failure case: don't default statusCode, return error
		statusCode := crawlResult.StatusCode
		errorMsg := crawlResult.Error
		if errorMsg == "" {
			errorMsg = "scraping failed"
		}

		s.logger.Warn().
			Str("url", item.URL).
			Int("status_code", statusCode).
			Str("error", errorMsg).
			Msg("Scraping failed or returned error")

		// Persist enhanced scraping failure log to database
		if jobID != "" {
			failureMsg := fmt.Sprintf("Scraping failed: %s (status=%d, error=%s)", item.URL, statusCode, errorMsg)
			s.logToDatabase(jobID, "error", failureMsg)
		}

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
	s.logger.Debug().
		Str("url", item.URL).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Int("body_length", len(crawlResult.Body)).
		Msg("Scraping completed")

	// Log successful scrapes (sampled: every 50th success)
	if jobID != "" {
		s.jobsMu.RLock()
		completedCount := 0
		if job, exists := s.activeJobs[jobID]; exists {
			completedCount = job.Progress.CompletedURLs
		}
		s.jobsMu.RUnlock()

		if completedCount%50 == 0 && completedCount > 0 {
			successMsg := fmt.Sprintf("Scraping successful: %s (status=%d, duration=%dms, body_length=%d)",
				item.URL, statusCode, duration.Milliseconds(), len(crawlResult.Body))
			s.logToDatabase(jobID, "info", successMsg)
		}
	}

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
	links := make([]string, 0)

	// Comment 6: Check if links are already provided in ScrapeResult metadata
	var allLinks []string
	if result.Metadata != nil {
		if linksRaw, ok := result.Metadata["links"]; ok {
			// Fast path: []string
			if linksSlice, ok := linksRaw.([]string); ok && len(linksSlice) > 0 {
				// Use links provided by ScrapeResult
				allLinks = linksSlice
				s.logger.Debug().
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
						s.logger.Debug().
							Str("url", parent.URL).
							Str("type", fmt.Sprintf("%T", linkRaw)).
							Msg("Skipping non-string element in links metadata")
					}
				}
				if len(allLinks) > 0 {
					s.logger.Debug().
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

	// Log all discovered links for debugging
	if jobID, ok := parent.Metadata["job_id"].(string); ok && jobID != "" && len(allLinks) > 0 {
		s.logger.Debug().
			Str("job_id", jobID).
			Str("parent_url", parent.URL).
			Int("count", len(allLinks)).
			Msg("Extracted links from page")

		// Log each discovered URL
		for i, link := range allLinks {
			s.logToDatabase(jobID, "debug", fmt.Sprintf("Discovered link %d/%d: %s", i+1, len(allLinks), link))
		}
	}

	// Warn on zero links discovered
	if len(allLinks) == 0 {
		s.logger.Warn().Str("url", parent.URL).Msg("Zero links discovered - check page structure or link extraction logic")
		if jobID, ok := parent.Metadata["job_id"].(string); ok && jobID != "" {
			s.logToDatabase(jobID, "warn", fmt.Sprintf("Zero links discovered from %s - check page structure", parent.URL))
		}
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
		s.logger.Warn().Str("url", parent.URL).Msg("source_type not found in metadata, skipping source-specific filtering")
	}

	// Apply source-specific filtering
	var filteredLinks []string
	switch sourceType {
	case "jira":
		filteredLinks = s.filterJiraLinks(allLinks, baseHost, config)
	case "confluence":
		filteredLinks = s.filterConfluenceLinks(allLinks, baseHost)
	default:
		// No source-specific filtering for other types
		filteredLinks = allLinks
	}

	// Apply include/exclude patterns (Comment 9: collect filtered samples)
	links, excludedSamples, notIncludedSamples := s.filterLinks(filteredLinks, config)

	// Log detailed filtering breakdown (Info level for visibility)
	s.logger.Info().
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

	// Persist link filtering summary to database
	if jobID, ok := parent.Metadata["job_id"].(string); ok && jobID != "" {
		sourceFilteredOut := len(allLinks) - len(filteredLinks)
		patternFilteredOut := len(filteredLinks) - len(links)
		totalFilteredOut := len(allLinks) - len(links)

		// Clear message: discovered -> after source filter -> after pattern filter -> following
		filterMsg := fmt.Sprintf("Found %d links, filtered %d (source:%d + pattern:%d), following %d",
			len(allLinks), totalFilteredOut, sourceFilteredOut, patternFilteredOut, len(links))
		s.logToDatabase(jobID, "info", filterMsg)

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

			s.logWarnToConsoleAndDatabase(jobID, warnMsg, map[string]interface{}{
				"url":                  parent.URL,
				"discovered_count":     len(allLinks),
				"after_source_filter":  len(filteredLinks),
				"after_pattern_filter": len(links),
				"source_type":          sourceType,
			})
		}
	}

	return links
}

// filterLinks applies include/exclude patterns
// Returns: (filteredLinks, excludedSamples, notIncludedSamples)
func (s *Service) filterLinks(links []string, config CrawlConfig) ([]string, []string, []string) {
	// Precompile exclude patterns
	excludeRegexes := make([]*regexp.Regexp, 0, len(config.ExcludePatterns))
	for _, pattern := range config.ExcludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			excludeRegexes = append(excludeRegexes, re)
		}
	}

	// Precompile include patterns
	includeRegexes := make([]*regexp.Regexp, 0, len(config.IncludePatterns))
	for _, pattern := range config.IncludePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			includeRegexes = append(includeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var excludedLinks, notIncludedLinks []string

	for _, link := range links {
		// Apply exclude patterns
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
			s.logger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Link excluded by pattern")
			continue
		}

		// Apply include patterns (if any)
		if len(includeRegexes) > 0 {
			included := false
			for _, re := range includeRegexes {
				if re.MatchString(link) {
					included = true
					break
				}
			}
			if !included {
				notIncludedLinks = append(notIncludedLinks, link)
				s.logger.Debug().
					Str("link", link).
					Msg("Link did not match any include pattern")
				continue
			}
		}

		filtered = append(filtered, link)
	}

	// Summary log for filtered links
	if len(excludedLinks) > 0 || len(notIncludedLinks) > 0 {
		s.logger.Debug().
			Int("excluded_count", len(excludedLinks)).
			Int("not_included_count", len(notIncludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Pattern filtering summary")
	}

	// Collect samples for database logging (Comment 9: limit to 2 each to avoid log spam)
	excludedSamples := []string{}
	if len(excludedLinks) > 0 {
		sampleCount := 2
		if len(excludedLinks) < 2 {
			sampleCount = len(excludedLinks)
		}
		excludedSamples = excludedLinks[:sampleCount]
	}

	notIncludedSamples := []string{}
	if len(notIncludedLinks) > 0 {
		sampleCount := 2
		if len(notIncludedLinks) < 2 {
			sampleCount = len(notIncludedLinks)
		}
		notIncludedSamples = notIncludedLinks[:sampleCount]
	}

	return filtered, excludedSamples, notIncludedSamples
}

// extractLinksFromHTML extracts and normalizes all links from HTML content
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
// Include pattern matching is handled by filterLinks() which runs after this
func (s *Service) filterJiraLinks(links []string, baseHost string, config CrawlConfig) []string {
	// Only apply include patterns if NO user patterns are provided
	// If user has custom patterns, skip include filtering here and let filterLinks() handle it
	var includePatterns []string
	if len(config.IncludePatterns) == 0 {
		// No user patterns - use default Jira patterns
		includePatterns = []string{
			`/browse/[A-Z][A-Z0-9]+-\d+`,       // Issue pages: /browse/PROJ-123, /browse/ABC2-456
			`/browse/[A-Z0-9]+(?:[?#/]|$)`,     // Project browse pages (with optional query/fragment/slash)
			`/projects/[A-Z0-9]+`,              // Project pages
			`/jira/projects`,                   // Projects listing page
			`(?i)/secure/RapidBoard\.jspa`,     // Agile boards (case-insensitive)
			`(?i)/secure/IssueNavigator\.jspa`, // Classic issue navigator (case-insensitive)
			`(?i)/issues/?`,                    // Issue search/list (case-insensitive)
			`(?i)/issues/\?jql=`,               // JQL query pages (case-insensitive)
			`(?i)/issues/\?filter=`,            // Filter pages (case-insensitive)
		}
	}

	// Exclude patterns for non-content pages
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

	// Precompile include patterns
	includeRegexes := make([]*regexp.Regexp, 0, len(includePatterns))
	for _, pattern := range includePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			includeRegexes = append(includeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var crossDomainLinks, excludedLinks, notIncludedLinks []string

	for _, link := range links {
		// Check same-host restriction
		if baseHost != "" {
			if parsedLink, err := url.Parse(link); err == nil {
				linkHost := strings.ToLower(parsedLink.Host)
				if linkHost != "" && linkHost != baseHost {
					crossDomainLinks = append(crossDomainLinks, link)
					s.logger.Debug().
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
			s.logger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Jira link excluded by pattern")
			continue
		}

		// If no user patterns were provided, check default include patterns
		// Otherwise, accept all non-excluded links (user patterns will be checked in filterLinks)
		if len(includeRegexes) > 0 {
			// Check include patterns (only when using defaults)
			included := false
			for _, re := range includeRegexes {
				if re.MatchString(link) {
					included = true
					break
				}
			}
			if included {
				filtered = append(filtered, link)
			} else {
				notIncludedLinks = append(notIncludedLinks, link)
				s.logger.Debug().
					Str("link", link).
					Msg("Jira link did not match any include pattern")
			}
		} else {
			// User provided custom patterns - accept all non-excluded links
			// (user patterns will be applied in filterLinks)
			filtered = append(filtered, link)
		}
	}

	// Summary log for Jira filtering
	if len(crossDomainLinks) > 0 || len(excludedLinks) > 0 || len(notIncludedLinks) > 0 {
		s.logger.Debug().
			Int("cross_domain_count", len(crossDomainLinks)).
			Int("excluded_count", len(excludedLinks)).
			Int("not_included_count", len(notIncludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Jira link filtering summary")
	}

	return filtered
}

// filterConfluenceLinks filters links to include only Confluence content pages on the same host
func (s *Service) filterConfluenceLinks(links []string, baseHost string) []string {
	// Include patterns for Confluence content pages
	includePatterns := []string{
		`(?i)/(?:wiki/)?spaces/[^/]+/pages/\d+`,   // Page with ID: /wiki/spaces/SPACE/pages/123456 or /spaces/SPACE/pages/123456 (case-insensitive)
		`(?i)/(?:wiki/)?spaces/[^/]+/?$`,          // Space home: /wiki/spaces/SPACE or /spaces/SPACE/ (case-insensitive)
		`(?i)/(?:wiki/)?spaces/[^/]+/overview`,    // Space overview (case-insensitive)
		`(?i)/(?:wiki/)?spaces/[^/]+/blog/`,       // Blog posts (case-insensitive)
		`(?i)/display/[^/]+/.+`,                   // Display format: /display/SPACE/PageTitle (case-insensitive)
		`(?i)/pages/viewpage\.action\?pageId=\d+`, // Legacy page URLs (case-insensitive)
		`(?i)/x/[A-Za-z0-9\-]+`,                   // Tiny links: /x/{id} (case-insensitive)
	}

	// Exclude patterns for non-content pages
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

	// Precompile include patterns
	includeRegexes := make([]*regexp.Regexp, 0, len(includePatterns))
	for _, pattern := range includePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			includeRegexes = append(includeRegexes, re)
		}
	}

	filtered := make([]string, 0)
	var crossDomainLinks, excludedLinks, notIncludedLinks []string

	for _, link := range links {
		// Check same-host restriction
		if baseHost != "" {
			if parsedLink, err := url.Parse(link); err == nil {
				linkHost := strings.ToLower(parsedLink.Host)
				if linkHost != "" && linkHost != baseHost {
					crossDomainLinks = append(crossDomainLinks, link)
					s.logger.Debug().
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
			s.logger.Debug().
				Str("link", link).
				Str("excluded_by_pattern", matchedExcludePattern).
				Msg("Confluence link excluded by pattern")
			continue
		}

		// Check include patterns
		included := false
		for _, re := range includeRegexes {
			if re.MatchString(link) {
				included = true
				break
			}
		}
		if included {
			filtered = append(filtered, link)
		} else {
			notIncludedLinks = append(notIncludedLinks, link)
			s.logger.Debug().
				Str("link", link).
				Msg("Confluence link did not match any include pattern")
		}
	}

	// Summary log for Confluence filtering
	if len(crossDomainLinks) > 0 || len(excludedLinks) > 0 || len(notIncludedLinks) > 0 {
		s.logger.Debug().
			Int("cross_domain_count", len(crossDomainLinks)).
			Int("excluded_count", len(excludedLinks)).
			Int("not_included_count", len(notIncludedLinks)).
			Int("passed_count", len(filtered)).
			Msg("Confluence link filtering summary")
	}

	return filtered
}

// enqueueLinks adds discovered links to queue with depth tracking
// Propagates source_type and entity_type from parent for link discovery
func (s *Service) enqueueLinks(jobID string, links []string, parent *URLQueueItem) {
	var enqueuedCount int
	for i, link := range links {
		// Propagate metadata from parent
		metadata := map[string]interface{}{
			"job_id": jobID,
		}
		if sourceType, ok := parent.Metadata["source_type"]; ok {
			metadata["source_type"] = sourceType
		}
		if entityType, ok := parent.Metadata["entity_type"]; ok {
			metadata["entity_type"] = entityType
		}

		item := &URLQueueItem{
			URL:       link,
			Depth:     parent.Depth + 1,
			ParentURL: parent.URL,
			Priority:  parent.Priority + i + 1,
			AddedAt:   time.Now(),
			Metadata:  metadata,
		}

		if s.queue.Push(item) {
			s.updatePendingCount(jobID, 1)
			enqueuedCount++

			// Log individual link enqueue decision
			s.logger.Debug().
				Str("job_id", jobID).
				Str("link", link).
				Int("depth", item.Depth).
				Str("parent_url", parent.URL).
				Int("priority", item.Priority).
				Msg("Link enqueued for processing")
		}
	}

	// Summary log for enqueued links
	if enqueuedCount > 0 {
		s.logger.Debug().
			Str("job_id", jobID).
			Str("parent_url", parent.URL).
			Int("enqueued_count", enqueuedCount).
			Int("total_links", len(links)).
			Msg("Link enqueueing complete")
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
		job.Progress.PendingURLs-- // Decrement pending when URL is completed
	}
	if failed {
		job.Progress.FailedURLs++
		job.Progress.PendingURLs-- // Decrement pending when URL fails
	}

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

	heartbeatCounter := 0 // Track ticks for heartbeat updates (every 15 ticks = 30 seconds)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			heartbeatCounter++

			s.jobsMu.RLock()
			job, exists := s.activeJobs[jobID]
			if !exists {
				s.jobsMu.RUnlock()
				return
			}

			// Check if job is in a terminal state (cancelled or failed) and exit
			// Comment 7: Terminal state logs removed - already logged by CancelJob/FailJob with progress details
			if job.Status == JobStatusCancelled || job.Status == JobStatusFailed {
				s.jobsMu.RUnlock()

				// Only remove from activeJobs if jobStorage is nil or job was already persisted
				// When jobStorage is nil, keep job in memory for lookup
				if s.jobStorage == nil {
					s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Keeping terminal state job in memory (no job storage)")
				} else {
					// Job should have been persisted by CancelJob/FailJob, safe to remove
					s.jobsMu.Lock()
					delete(s.activeJobs, jobID)
					s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Removed terminal state job from active jobs")
					s.jobsMu.Unlock()
				}

				s.logger.Debug().Str("job_id", jobID).Str("status", string(job.Status)).Msg("Monitor goroutine exiting for terminal job status")
				return
			}

			// Update heartbeat every 30 seconds (15 ticks)
			if heartbeatCounter >= 15 {
				if s.jobStorage != nil {
					if err := s.jobStorage.UpdateJobHeartbeat(s.ctx, jobID); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to update job heartbeat")
					}
				}
				heartbeatCounter = 0
			}

			// Check if job is complete
			if job.Status == JobStatusRunning && job.Progress.PendingURLs == 0 && job.Progress.CompletedURLs+job.Progress.FailedURLs >= job.Progress.TotalURLs {
				s.jobsMu.RUnlock()

				// Mark job as completed
				s.jobsMu.Lock()
				job.Status = JobStatusCompleted
				job.CompletedAt = time.Now()
				job.ResultCount = job.Progress.CompletedURLs
				job.FailedCount = job.Progress.FailedURLs
				s.jobsMu.Unlock()

				// Persist job completion to database
				persistSucceeded := false
				if s.jobStorage != nil {
					if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to persist job completion - keeping job in memory")
					} else {
						persistSucceeded = true
					}

					// Comment 8: Append enhanced job completion log with duration and success rate
					duration := job.CompletedAt.Sub(job.StartedAt)
					totalProcessed := job.Progress.CompletedURLs + job.Progress.FailedURLs
					successRate := 0.0
					if totalProcessed > 0 {
						successRate = float64(job.Progress.CompletedURLs) / float64(totalProcessed) * 100
					}

					completionMsg := fmt.Sprintf("Job completed: %d successful, %d failed, duration=%s, success_rate=%.1f%%",
						job.Progress.CompletedURLs, job.Progress.FailedURLs, duration.Round(time.Second), successRate)

					logEntry := models.JobLogEntry{
						Timestamp: time.Now().Format("15:04:05"),
						Level:     "info",
						Message:   completionMsg,
					}
					if err := s.jobStorage.AppendJobLog(s.ctx, jobID, logEntry); err != nil {
						s.logger.Warn().Err(err).Str("job_id", jobID).Msg("Failed to append completion log")
					}
				}

				s.emitProgress(job)

				// Only remove from activeJobs if persistence succeeded or jobStorage is nil
				if s.jobStorage == nil {
					s.logger.Debug().Str("job_id", jobID).Msg("Keeping completed job in memory (no job storage)")
				} else if persistSucceeded {
					// Persistence succeeded, safe to clean up and remove from memory
					s.jobsMu.Lock()
					if _, exists := s.jobClients[jobID]; exists {
						delete(s.jobClients, jobID)
						s.logger.Debug().Str("job_id", jobID).Msg("Cleaned up per-job HTTP client")
					}
					delete(s.activeJobs, jobID)
					s.logger.Debug().Str("job_id", jobID).Msg("Removed completed job from active jobs")
					s.jobsMu.Unlock()
				} else {
					// Persistence failed, keep job in memory for subsequent lookups
					s.logger.Debug().Str("job_id", jobID).Msg("Keeping completed job in memory due to persistence failure")
				}

				s.logger.Info().
					Str("job_id", jobID).
					Int("completed", job.Progress.CompletedURLs).
					Int("failed", job.Progress.FailedURLs).
					Msg("Crawl job completed")

				// Continue monitoring if persistence failed (job still in activeJobs)
				if !persistSucceeded && s.jobStorage != nil {
					continue
				}

				return
			}
			s.jobsMu.RUnlock()
		}
	}
}

// ListJobs returns a list of jobs with optional filtering
func (s *Service) ListJobs(ctx context.Context, opts *interfaces.ListOptions) (interface{}, error) {
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
func (s *Service) RerunJob(ctx context.Context, jobID string, updateConfig interface{}) (string, error) {
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
		// Type assert to *CrawlConfig
		crawlConfig, ok := updateConfig.(*CrawlConfig)
		if !ok {
			return "", fmt.Errorf("invalid config type: expected *CrawlConfig")
		}
		config = *crawlConfig
	}

	// Use stored seed URLs from original job
	seedURLs := originalJob.SeedURLs
	if len(seedURLs) == 0 {
		return "", fmt.Errorf("cannot rerun job: no seed URLs stored in original job")
	}

	// Get snapshots from original job
	sourceConfigSnapshot, _ := originalJob.GetSourceConfigSnapshot()
	authSnapshot, _ := originalJob.GetAuthSnapshot()

	// Start new crawl with original parameters and snapshots
	newJobID, err := s.StartCrawl(originalJob.SourceType, originalJob.EntityType, seedURLs, config, "", originalJob.RefreshSource, sourceConfigSnapshot, authSnapshot)
	if err != nil {
		return "", fmt.Errorf("failed to start crawl: %w", err)
	}

	s.logger.Info().
		Str("original_job_id", jobID).
		Str("new_job_id", newJobID).
		Msg("Job rerun started")

	return newJobID, nil
}

// applyURLPatternFilters applies generic URL pattern filtering from source filters
// Returns filtered links and count of links that were filtered out
func (s *Service) applyURLPatternFilters(allLinks []string, parent *URLQueueItem, config CrawlConfig) ([]string, int) {
	// Get source config from parent metadata to access filters
	var sourceConfig *models.SourceConfig
	if configSnapshot, ok := parent.Metadata["source_config"].(string); ok {
		// Deserialize source config snapshot
		var config models.SourceConfig
		if err := json.Unmarshal([]byte(configSnapshot), &config); err == nil {
			sourceConfig = &config
		}
	}

	if sourceConfig == nil || sourceConfig.Filters == nil {
		// No filters configured - return all links
		return allLinks, 0
	}

	// Extract include and exclude patterns from source filters
	var includePatterns, excludePatterns []string

	if includeRaw, exists := sourceConfig.Filters["include_patterns"]; exists {
		switch patterns := includeRaw.(type) {
		case []string:
			includePatterns = patterns
		case []interface{}:
			for _, p := range patterns {
				if str, ok := p.(string); ok {
					includePatterns = append(includePatterns, str)
				}
			}
		case string:
			includePatterns = []string{patterns}
		}
	}

	if excludeRaw, exists := sourceConfig.Filters["exclude_patterns"]; exists {
		switch patterns := excludeRaw.(type) {
		case []string:
			excludePatterns = patterns
		case []interface{}:
			for _, p := range patterns {
				if str, ok := p.(string); ok {
					excludePatterns = append(excludePatterns, str)
				}
			}
		case string:
			excludePatterns = []string{patterns}
		}
	}

	if len(includePatterns) == 0 && len(excludePatterns) == 0 {
		// No patterns configured - return all links
		return allLinks, 0
	}

	// Apply pattern filtering
	filtered := make([]string, 0, len(allLinks))
	filteredCount := 0

linkLoop:
	for _, link := range allLinks {
		// Check exclude patterns first
		for _, pattern := range excludePatterns {
			if strings.Contains(strings.ToLower(link), strings.ToLower(pattern)) {
				s.logger.Debug().Str("link", link).Str("pattern", pattern).Msg("Link excluded by pattern")
				filteredCount++
				continue linkLoop
			}
		}

		// Check include patterns (if any)
		if len(includePatterns) > 0 {
			included := false
			for _, pattern := range includePatterns {
				if strings.Contains(strings.ToLower(link), strings.ToLower(pattern)) {
					included = true
					break
				}
			}
			if !included {
				s.logger.Debug().Str("link", link).Msg("Link not included by any pattern")
				filteredCount++
				continue linkLoop
			}
		}

		// Link passed all filters
		filtered = append(filtered, link)
	}

	return filtered, filteredCount
}

// WaitForJob blocks until a job completes or context is cancelled
//
// IMPORTANT: This function expects in-process waiting where the job is running in the same
// service instance and s.jobResults is populated during execution. When GetJobStatus() falls
// back to the database and retrieves a completed job from another instance or a previous run,
// the results may not be available in s.jobResults (which is in-memory only).
//
// Edge case: If the database returns a completed job but GetJobResults() returns "not found",
// this indicates the job was completed by a different service instance or the service was
// restarted. In this case, the function returns an error. Callers should handle this by
// checking the job's ResultCount field from GetJobStatus() for summary information.
func (s *Service) WaitForJob(ctx context.Context, jobID string) (interface{}, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	pollCount := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			pollCount++

			jobInterface, err := s.GetJobStatus(jobID)
			if err != nil {
				s.logger.Debug().
					Err(err).
					Str("job_id", jobID).
					Int("poll_count", pollCount).
					Msg("Failed to get job status while waiting")
				return nil, fmt.Errorf("failed to get job status: %w", err)
			}

			// Type assert to *CrawlJob
			job, ok := jobInterface.(*CrawlJob)
			if !ok {
				s.logger.Error().Str("job_id", jobID).Msg("Unexpected result type from GetJobStatus")
				return nil, fmt.Errorf("unexpected result type from GetJobStatus")
			}

			// Periodic status logging every 10 polls
			if pollCount%10 == 0 {
				s.logger.Debug().
					Str("job_id", jobID).
					Str("status", string(job.Status)).
					Int("poll_count", pollCount).
					Msg("Job status polling update")
			}

			// Check if job is complete
			if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
				resultsInterface, err := s.GetJobResults(jobID)
				if err != nil {
					// Edge case: Job completed in another instance or before service restart
					s.logger.Warn().
						Err(err).
						Str("job_id", jobID).
						Str("status", string(job.Status)).
						Int("result_count", job.ResultCount).
						Int("failed_count", job.FailedCount).
						Msg("Job completed but results not available (completed in different instance or before restart)")
					return nil, fmt.Errorf("job %s completed but results unavailable (result_count: %d, failed_count: %d): %w",
						jobID, job.ResultCount, job.FailedCount, err)
				}

				// Type assert to []*CrawlResult
				results, ok := resultsInterface.([]*CrawlResult)
				if !ok {
					s.logger.Error().Str("job_id", jobID).Msg("Unexpected result type from GetJobResults")
					return nil, fmt.Errorf("unexpected result type from GetJobResults")
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

	// Clean up all per-job HTTP clients
	s.jobsMu.Lock()
	clientCount := len(s.jobClients)
	s.jobClients = make(map[string]*http.Client)
	s.jobsMu.Unlock()

	if clientCount > 0 {
		s.logger.Debug().Int("count", clientCount).Msg("Cleaned up per-job HTTP clients on shutdown")
	}

	s.logger.Info().Msg("Crawler service stopped")
	return nil
}

// Close cleans up resources
func (s *Service) Close() error {
	return s.Shutdown()
}

// Helper functions

// buildHTTPClientFromAuth creates an HTTP client with cookies from AuthCredentials
func buildHTTPClientFromAuth(authCreds *models.AuthCredentials) (*http.Client, error) {
	if authCreds == nil {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// Parse base URL and set cookies
	baseURL, err := url.Parse(authCreds.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Unmarshal cookies from JSON
	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Convert to http.Cookie
	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, c := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Unix(c.Expires, 0),
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	client.Jar.SetCookies(baseURL, httpCookies)

	return client, nil
}

// logQueueDiagnostics logs current queue state and job progress for debugging
// This method helps diagnose queue health issues, stalled jobs, and worker activity
func (s *Service) logQueueDiagnostics(jobID string) {
	// Get current queue length
	queueLen := s.queue.Len()

	// Get job progress with lock
	s.jobsMu.RLock()
	job, exists := s.activeJobs[jobID]
	if !exists {
		s.jobsMu.RUnlock()
		return
	}

	pendingURLs := job.Progress.PendingURLs
	completedURLs := job.Progress.CompletedURLs
	failedURLs := job.Progress.FailedURLs
	totalURLs := job.Progress.TotalURLs
	s.jobsMu.RUnlock()

	// Calculate queue health indicators
	queueHealthy := true
	var healthIssues []string

	// Issue 1: Queue has items but pending count is zero
	if queueLen > 0 && pendingURLs == 0 {
		queueHealthy = false
		healthIssues = append(healthIssues, "queue_has_items_but_pending_zero")
	}

	// Issue 2: Pending count is non-zero but queue is empty
	if queueLen == 0 && pendingURLs > 0 {
		queueHealthy = false
		healthIssues = append(healthIssues, "pending_nonzero_but_queue_empty")
	}

	// Issue 3: Total processed (completed + failed) doesn't match expected
	totalProcessed := completedURLs + failedURLs
	expectedProcessed := totalURLs - pendingURLs
	if totalProcessed != expectedProcessed {
		queueHealthy = false
		healthIssues = append(healthIssues, fmt.Sprintf("count_mismatch_processed=%d_expected=%d", totalProcessed, expectedProcessed))
	}

	// Log queue diagnostics with health status
	logEvent := s.logger.Info().
		Str("job_id", jobID).
		Int("queue_len", queueLen).
		Int("pending_urls", pendingURLs).
		Int("completed_urls", completedURLs).
		Int("failed_urls", failedURLs).
		Int("total_urls", totalURLs).
		Str("queue_healthy", fmt.Sprintf("%v", queueHealthy))

	if !queueHealthy {
		logEvent = logEvent.Strs("health_issues", healthIssues)
	}

	logEvent.Msg("Queue diagnostics")

	// Persist diagnostics to database if health issues detected
	if !queueHealthy && s.jobStorage != nil {
		diagMsg := fmt.Sprintf("Queue health issues detected: queue_len=%d, pending=%d, completed=%d, failed=%d, issues=%v",
			queueLen, pendingURLs, completedURLs, failedURLs, healthIssues)
		s.logToDatabase(jobID, "warn", diagMsg)
	}
}
