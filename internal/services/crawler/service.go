// -----------------------------------------------------------------------
// Last Modified: Thursday, 23rd October 2025 8:18:25 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
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
	authService     interfaces.AuthService
	sourceService   *sources.Service
	authStorage     interfaces.AuthStorage
	eventService    interfaces.EventService
	jobStorage      interfaces.JobStorage
	documentStorage interfaces.DocumentStorage // Used for immediate document persistence during crawling
	logger          arbor.ILogger
	config          *common.Config

	queue       *URLQueue
	retryPolicy *RetryPolicy
	workerPool  *workers.Pool

	// Browser pool for chromedp (reusable browser instances)
	browserPool      []context.Context
	browserCancels   []context.CancelFunc
	allocatorCancels []context.CancelFunc
	browserPoolMu    sync.Mutex
	browserPoolSize  int

	activeJobs map[string]*CrawlJob
	jobResults map[string][]*CrawlResult
	jobClients map[string]*http.Client // Per-job HTTP clients built from auth snapshots
	jobsMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new crawler service
func NewService(authService interfaces.AuthService, sourceService *sources.Service, authStorage interfaces.AuthStorage, eventService interfaces.EventService, jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, logger arbor.ILogger, config *common.Config) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		authService:     authService,
		sourceService:   sourceService,
		authStorage:     authStorage,
		eventService:    eventService,
		jobStorage:      jobStorage,
		documentStorage: documentStorage,
		logger:          logger,
		config:          config,
		queue:           NewURLQueue(),
		retryPolicy:     NewRetryPolicy(),
		workerPool:      workers.NewPool(config.Crawler.MaxConcurrency, logger),
		activeJobs:      make(map[string]*CrawlJob),
		jobResults:      make(map[string][]*CrawlResult),
		jobClients:      make(map[string]*http.Client),
		ctx:             ctx,
		cancel:          cancel,
	}

	return s
}

// Start starts the crawler service
func (s *Service) Start() error {
	s.logger.Info().Msg("Crawler service started")
	return nil
}

// initBrowserPool creates a pool of reusable browser instances for efficient JavaScript rendering
// This prevents resource exhaustion from creating a new browser for every URL
func (s *Service) initBrowserPool(poolSize int) error {
	s.browserPoolMu.Lock()
	defer s.browserPoolMu.Unlock()

	s.browserPoolSize = poolSize
	s.browserPool = make([]context.Context, 0, poolSize)
	s.browserCancels = make([]context.CancelFunc, 0, poolSize)
	s.allocatorCancels = make([]context.CancelFunc, 0, poolSize)

	s.logger.Info().
		Int("pool_size", poolSize).
		Msg("Initializing browser pool for chromedp")

	for i := 0; i < poolSize; i++ {
		// Create allocator context (long-lived browser process)
		allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(
			context.Background(),
			append(
				chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
				chromedp.Flag("disable-gpu", true),
				chromedp.Flag("no-sandbox", true),
				chromedp.Flag("disable-dev-shm-usage", true),
				chromedp.UserAgent(s.config.Crawler.UserAgent),
			)...,
		)

		// Create browser context from allocator
		browserCtx, browserCancel := chromedp.NewContext(allocatorCtx)

		s.browserPool = append(s.browserPool, browserCtx)
		s.browserCancels = append(s.browserCancels, browserCancel)
		s.allocatorCancels = append(s.allocatorCancels, allocatorCancel)

		s.logger.Debug().
			Int("browser_index", i).
			Msg("Browser instance created in pool")
	}

	s.logger.Info().
		Int("browsers_created", len(s.browserPool)).
		Msg("Browser pool initialized successfully")

	return nil
}

// getBrowserFromPool returns a browser context from the pool (round-robin)
func (s *Service) getBrowserFromPool(workerIndex int) (context.Context, context.CancelFunc) {
	s.browserPoolMu.Lock()
	defer s.browserPoolMu.Unlock()

	if len(s.browserPool) == 0 {
		return nil, nil
	}

	// Use worker index for consistent browser assignment
	index := workerIndex % len(s.browserPool)
	return s.browserPool[index], s.browserCancels[index]
}

// shutdownBrowserPool cleans up all browser instances in the pool
func (s *Service) shutdownBrowserPool() {
	s.browserPoolMu.Lock()
	defer s.browserPoolMu.Unlock()

	s.logger.Info().
		Int("browser_count", len(s.browserPool)).
		Msg("Shutting down browser pool")

	// Cancel all browser contexts
	for i, cancel := range s.browserCancels {
		if cancel != nil {
			cancel()
			s.logger.Debug().
				Int("browser_index", i).
				Msg("Browser context cancelled")
		}
	}

	// Cancel all allocator contexts
	for i, cancel := range s.allocatorCancels {
		if cancel != nil {
			cancel()
			s.logger.Debug().
				Int("browser_index", i).
				Msg("Browser allocator cancelled")
		}
	}

	// Clear the pools
	s.browserPool = nil
	s.browserCancels = nil
	s.allocatorCancels = nil

	s.logger.Info().Msg("Browser pool shut down successfully")
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

	// Create context logger for this job (logs automatically sent to database)
	contextLogger := s.logger.WithContextWriter(jobID)

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
			contextLogger.Warn().
				Err(err).
				Str("seed_url", seedURL).
				Msg(fmt.Sprintf("Invalid seed URL: %s - %v", seedURL, err))
		}
		if isTestURL {
			testURLCount++
			testURLWarnings = append(testURLWarnings, warnings...)
		}
	}

	// Reject test URLs in production mode
	if s.config.IsProduction() && testURLCount > 0 {
		errMsg := fmt.Sprintf("Test URLs are not allowed in production mode: %d of %d seed URLs are test URLs (localhost/127.0.0.1). Set environment=\"development\" in config to allow test URLs.", testURLCount, len(seedURLs))
		contextLogger.Error().
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Strs("warnings", testURLWarnings).
			Msg(errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}

	// Log test URL warnings if any detected (development mode)
	if testURLCount > 0 {
		warningMsg := fmt.Sprintf("Test URLs detected: %d of %d seed URLs are test URLs (localhost/127.0.0.1) - allowed in development mode",
			testURLCount, len(seedURLs))
		contextLogger.Warn().
			Int("test_url_count", testURLCount).
			Int("total_urls", len(seedURLs)).
			Strs("warnings", testURLWarnings).
			Msg(warningMsg)
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
	contextLogger.Debug().
		Int("seed_url_count", len(seedURLs)).
		Int("test_url_count", testURLCount).
		Msg(seedURLsMsg)

	// Log crawler configuration summary
	configMsg := fmt.Sprintf("Crawler configuration: max_depth=%d, max_pages=%d, concurrency=%d, rate_limit=%dms, follow_links=%v",
		config.MaxDepth, config.MaxPages, config.Concurrency, config.RateLimit.Milliseconds(), config.FollowLinks)
	contextLogger.Debug().
		Int("max_depth", config.MaxDepth).
		Int("max_pages", config.MaxPages).
		Int("concurrency", config.Concurrency).
		Int64("rate_limit", config.RateLimit.Milliseconds()).
		Bool("follow_links", config.FollowLinks).
		Msg(configMsg)

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
					contextLogger.Warn().Err(err).Str("auth_id", sourceConfig.AuthID).Msg(fmt.Sprintf("Failed to fetch auth credentials: auth_id=%s", sourceConfig.AuthID))
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
				contextLogger.Warn().Err(err).Str("auth_id", latestConfig.AuthID).Msg(fmt.Sprintf("Failed to refresh auth credentials: auth_id=%s", latestConfig.AuthID))
			} else {
				authSnapshot = latestAuth
			}
		}
	}

	// Validate source config snapshot if provided
	if sourceConfigSnapshot != nil {
		if err := sourceConfigSnapshot.Validate(); err != nil {
			// Log validation failure before returning error
			contextLogger.Error().Err(err).Msg("Source config validation failed")
			return "", fmt.Errorf("source configuration validation failed: %w", err)
		}

		// Store snapshot in job
		if err := job.SetSourceConfigSnapshot(sourceConfigSnapshot); err != nil {
			// Log snapshot serialization failure
			contextLogger.Error().Err(err).Msg("Failed to serialize source config snapshot")
			return "", fmt.Errorf("failed to set source config snapshot: %w", err)
		}

		// Log validation success with base URL
		baseURLInfo := "unknown"
		if sourceConfigSnapshot.BaseURL != "" {
			baseURLInfo = sourceConfigSnapshot.BaseURL
		}
		contextLogger.Debug().Str("base_url", baseURLInfo).Msg(fmt.Sprintf("Source config validated and stored: base_url=%s", baseURLInfo))
	} else {
		// Log missing source config snapshot
		contextLogger.Info().Msg("No source config snapshot provided")
	}

	// Store auth snapshot if provided
	var httpClientType string
	if authSnapshot != nil {
		if err := job.SetAuthSnapshot(authSnapshot); err != nil {
			// Log auth snapshot serialization failure
			contextLogger.Error().Err(err).Msg("Failed to serialize auth snapshot")
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
		contextLogger.Debug().Int("cookie_count", cookieCount).Msg(fmt.Sprintf("Auth snapshot stored: %d cookies available", cookieCount))

		// Build HTTP client from auth snapshot for this job
		client, err := buildHTTPClientFromAuth(authSnapshot)
		if err != nil {
			contextLogger.Warn().Err(err).Msg(fmt.Sprintf("Failed to build HTTP client from auth: %v - will use default", err))
			httpClientType = "default (auth build failed)"
		} else {
			s.jobsMu.Lock()
			s.jobClients[jobID] = client
			s.jobsMu.Unlock()
			s.logger.Debug().Str("job_id", jobID).Msg("Per-job HTTP client configured from auth snapshot")
			httpClientType = "per-job (from auth snapshot)"
		}
	} else {
		// Log missing auth snapshot
		if sourceConfigSnapshot == nil || sourceConfigSnapshot.AuthID == "" {
			contextLogger.Info().Msg("No auth snapshot provided - requests will use default HTTP client")
		} else {
			contextLogger.Warn().Msg("No auth snapshot provided - requests will use default HTTP client")
		}
		httpClientType = "default (no auth)"
	}

	// Log HTTP client configuration
	contextLogger.Debug().Str("client_type", httpClientType).Msg(fmt.Sprintf("HTTP client configured: type=%s", httpClientType))

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

		contextLogger.Info().
			Str("source_type", sourceType).
			Str("entity_type", entityType).
			Int("seed_count", len(seedURLs)).
			Int("max_depth", config.MaxDepth).
			Int("max_pages", config.MaxPages).
			Int("concurrency", config.Concurrency).
			Str("auth", authStatus).
			Msg(jobStartMsg)
	}

	// Emit started event
	s.emitProgress(job)

	// Initialize browser pool if JavaScript rendering is enabled
	if s.config.Crawler.EnableJavaScript {
		if err := s.initBrowserPool(config.Concurrency); err != nil {
			s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to initialize browser pool")
			return "", fmt.Errorf("failed to initialize browser pool: %w", err)
		}
	}

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
		contextLogger := s.logger.WithContextWriter(jobID)
		contextLogger.Warn().Msg(cancelMsg)
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
		contextLogger := s.logger.WithContextWriter(jobID)
		contextLogger.Error().Msg(failMsg)
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

// RerunJob creates a copy of a previous job and adds it to the queue with "pending" status
// This is the same as "execute on-demand" but uses an existing job as the template.
// Jobs are self-contained with all config stored (source_config_snapshot, auth_snapshot, seed_urls).
// The job is NOT executed immediately - it is queued and will be picked up by workers when available.
func (s *Service) RerunJob(ctx context.Context, jobID string, updateConfig interface{}) (string, error) {
	if s == nil {
		return "", fmt.Errorf("service is nil")
	}

	if s.logger == nil {
		return "", fmt.Errorf("logger is nil")
	}

	if s.jobStorage == nil {
		return "", fmt.Errorf("job storage not configured")
	}

	// Get original job from database
	jobInterface, err := s.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get job: %w", err)
	}

	originalJob, ok := jobInterface.(*CrawlJob)
	if !ok || originalJob == nil {
		return "", fmt.Errorf("invalid job type or nil job")
	}

	// Create new job ID
	newJobID := uuid.New().String()
	now := time.Now()

	// Build fresh job copying only essential fields from original
	// This avoids any potential issues with struct copying or nil pointers
	newJob := &CrawlJob{
		ID:                   newJobID,
		Name:                 originalJob.Name,
		Description:          originalJob.Description,
		SourceType:           originalJob.SourceType,
		EntityType:           originalJob.EntityType,
		Config:               originalJob.Config,
		SourceConfigSnapshot: originalJob.SourceConfigSnapshot,
		AuthSnapshot:         originalJob.AuthSnapshot,
		RefreshSource:        originalJob.RefreshSource,
		SeedURLs:             originalJob.SeedURLs,

		// Reset to fresh state
		Status:         JobStatusPending,
		CreatedAt:      now,
		StartedAt:      time.Time{},
		CompletedAt:    time.Time{},
		Error:          "",
		ResultCount:    0,
		FailedCount:    0,
		DocumentsSaved: 0,

		// Fresh progress
		Progress: CrawlProgress{
			TotalURLs:     len(originalJob.SeedURLs),
			CompletedURLs: 0,
			FailedURLs:    0,
			PendingURLs:   len(originalJob.SeedURLs),
			StartTime:     now,
		},
	}

	// Apply config update if provided
	if updateConfig != nil {
		if crawlConfig, ok := updateConfig.(*CrawlConfig); ok && crawlConfig != nil {
			newJob.Config = *crawlConfig
		}
	}

	// Save the new job
	if err := s.jobStorage.SaveJob(ctx, newJob); err != nil {
		return "", fmt.Errorf("failed to save job: %w", err)
	}

	s.logger.Info().
		Str("original_job_id", jobID).
		Str("new_job_id", newJobID).
		Msg("Job rerun created successfully")

	return newJobID, nil
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

	// Shutdown browser pool
	s.shutdownBrowserPool()

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

	// Parse base URL for fallback
	baseURL, err := url.Parse(authCreds.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Unmarshal cookies from JSON
	var cookies []*interfaces.AtlassianExtensionCookie
	if err := json.Unmarshal(authCreds.Cookies, &cookies); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// Group cookies by domain to set them with appropriate URLs
	// This ensures cookie jar accepts cookies based on their declared domain
	cookiesByDomain := make(map[string][]*http.Cookie)
	for _, c := range cookies {
		// Calculate expiration time
		// If expiration is 0 or in the past, treat as session cookie (no expiration)
		// This prevents cookie jar from rejecting cookies with zero/invalid timestamps
		var expires time.Time
		if c.Expires > 0 {
			expires = time.Unix(c.Expires, 0)
			// If cookie expired more than a day ago, treat as session cookie
			if expires.Before(time.Now().Add(-24 * time.Hour)) {
				expires = time.Time{} // Zero value = session cookie
			}
		}
		// Zero time.Time = session cookie (no expiration)

		httpCookie := &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  expires,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		}

		// Use cookie's domain, removing leading dot if present
		domain := strings.TrimPrefix(c.Domain, ".")
		if domain == "" {
			domain = baseURL.Host // Fallback to base URL host
		}

		cookiesByDomain[domain] = append(cookiesByDomain[domain], httpCookie)
	}

	// Set cookies for each domain using a URL that matches that domain
	for domain, domainCookies := range cookiesByDomain {
		// Build URL for this domain (always use https for Atlassian)
		domainURL, err := url.Parse(fmt.Sprintf("https://%s/", domain))
		if err != nil {
			// Log warning and skip this domain
			continue
		}

		// Set cookies for this domain
		client.Jar.SetCookies(domainURL, domainCookies)
	}

	return client, nil
}
