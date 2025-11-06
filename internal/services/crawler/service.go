// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 9:02:58 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/httpclient"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// Source type constants (moved from deleted models/source.go)
const (
	SourceTypeJira       = "jira"
	SourceTypeConfluence = "confluence"
	SourceTypeGithub     = "github"
)

// ============================================================================
// JOB SYSTEM ARCHITECTURE
// ============================================================================
//
// This crawler service interacts with TWO distinct job systems that serve
// different purposes and operate independently:
//
// 1. QUEUE-BASED JOB SYSTEM (Task Execution)
//    - Purpose: Execute individual crawler tasks (URLs, pages)
//    - Components: QueueManager (goqite), WorkerPool, CrawlerJob
//    - Location: internal/queue/, internal/jobs/types/crawler.go
//    - Characteristics:
//      * Task-level granularity (one job message per URL)
//      * Worker pool processes messages from persistent queue
//      * Automatic retries, visibility timeouts, dead-letter handling
//      * Horizontal scalability (multiple workers, multiple instances)
//    - Use When:
//      * Executing individual crawler tasks
//      * Need retry semantics and fault tolerance
//      * Want distributed processing across workers
//      * Processing user-triggered crawl operations
//
// 2. JOB DEFINITION SYSTEM (Multi-Step Job Coordination)
//    - Purpose: Coordinate multi-step jobs and scheduled jobs
//    - Components: JobExecutor, JobRegistry, Action Handlers
//    - Location: internal/services/jobs/, internal/services/jobs/actions/
//    - Characteristics:
//      * Workflow-level granularity (entire multi-step process)
//      * Declarative job definitions with steps and dependencies
//      * Scheduler integration for cron-based execution
//      * Supports post-job triggers and chaining
//      * Polling-based completion detection for async operations
//    - Use When:
//      * Defining scheduled jobs (cron jobs)
//      * Coordinating multi-step processes (crawl → summarize → cleanup)
//      * Need job chaining with post-job triggers
//      * Require job-level configuration and metadata
//
// INTERACTION BETWEEN SYSTEMS:
//
// JobExecutor (multi-step jobs) → QueueManager (task execution)
//   - JobExecutor triggers crawl workflows via CrawlerActions
//   - CrawlerActions enqueue URL tasks into QueueManager
//   - WorkerPool processes URL tasks using CrawlerJob handlers
//   - Completion detection uses polling via GetJobStatus()
//
// EXAMPLE JOB FLOW:
//
// 1. User creates JobDefinition: "Crawl Jira + Summarize"
// 2. JobExecutor processes definition, executes CrawlerAction
// 3. CrawlerAction calls StartCrawl() → enqueues seed URLs
// 4. WorkerPool workers process URL messages via CrawlerJob
// 5. JobExecutor polls GetJobStatus() until crawl completes
// 6. Post-job trigger fires SummarizerAction (if configured)
// 7. Job completes, status persisted to database
//
// KEY DESIGN PRINCIPLES:
//
// - Single Responsibility: Each system handles its domain well
// - Loose Coupling: Systems communicate via interfaces (JobStorage, QueueManager)
// - Persistence: Both systems store state in database for recovery
// - Scalability: Queue system scales horizontally, executor scales vertically
// - Separation of Concerns: Task execution vs. job coordination
//
// MIGRATION NOTES:
//
// - Worker management migrated from Service to queue.WorkerPool
// - Progress tracking moved from Service to CrawlJob (via JobStorage)
// - URL queue replaced with goqite-backed persistent queue
// - Retry logic handled by queue system (visibility timeout)
//
// ============================================================================

// Service orchestrates crawler jobs using queue manager
// Worker management has been migrated to queue.WorkerPool
// Job execution is handled by queue-based job types (internal/jobs/types/crawler.go)
type Service struct {
	authService     interfaces.AuthService
	authStorage     interfaces.AuthStorage
	eventService    interfaces.EventService
	jobStorage      interfaces.JobStorage
	documentStorage interfaces.DocumentStorage // Used for immediate document persistence during crawling
	queueManager    interfaces.QueueManager    // Replaces custom URLQueue with goqite-backed queue
	logger          arbor.ILogger
	config          *common.Config

	// Browser pool for chromedp (reusable browser instances)
	browserPool      []context.Context
	browserCancels   []context.CancelFunc
	allocatorCancels []context.CancelFunc
	browserPoolMu    sync.Mutex
	browserPoolSize  int

	activeJobs map[string]*models.Job
	jobResults map[string][]*CrawlResult
	jobClients map[string]*http.Client // Per-job HTTP clients built from auth snapshots
	jobsMu     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewService creates a new crawler service
func NewService(authService interfaces.AuthService, authStorage interfaces.AuthStorage, eventService interfaces.EventService, jobStorage interfaces.JobStorage, documentStorage interfaces.DocumentStorage, queueManager interfaces.QueueManager, logger arbor.ILogger, config *common.Config) *Service {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		authService:     authService,
		authStorage:     authStorage,
		eventService:    eventService,
		jobStorage:      jobStorage,
		documentStorage: documentStorage,
		queueManager:    queueManager,
		logger:          logger,
		config:          config,
		activeJobs:      make(map[string]*models.Job),
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
// jobDefinitionID: Optional job definition ID for traceability (empty string if not from a job definition)
func (s *Service) StartCrawl(sourceType, entityType string, seedURLs []string, configInterface interface{}, sourceID string, refreshSource bool, sourceConfigSnapshotInterface interface{}, authSnapshotInterface interface{}, jobDefinitionID string) (string, error) {
	// Type assert config
	config, ok := configInterface.(CrawlConfig)
	if !ok {
		return "", fmt.Errorf("invalid config type: expected CrawlConfig")
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

	// Validate source type to prevent invalid values like "crawler"
	// Explicitly reject "crawler" - this is a job definition type, not a source type
	if sourceType == "crawler" {
		err := fmt.Errorf("invalid source type 'crawler': this is a job definition type, not a source type. Expected: jira, confluence, or github")
		contextLogger.Error().Str("source_type", sourceType).Msg("Invalid source type 'crawler' detected - this is a job definition type not a source type")
		return "", err
	}

	validSourceTypes := map[string]bool{
		SourceTypeJira:       true,
		SourceTypeConfluence: true,
		SourceTypeGithub:     true,
	}
	if !validSourceTypes[sourceType] {
		err := fmt.Errorf("invalid source type: %s (must be one of: jira, confluence, github)", sourceType)
		contextLogger.Error().Str("source_type", sourceType).Msg("Invalid source type detected")
		return "", err
	}

	// Build config map for new Job model
	jobConfig := make(map[string]interface{})
	jobConfig["crawl_config"] = config
	jobConfig["source_type"] = sourceType
	jobConfig["entity_type"] = entityType
	jobConfig["refresh_source"] = refreshSource
	jobConfig["seed_urls"] = seedURLs

	// Build metadata map
	metadata := make(map[string]interface{})
	if jobDefinitionID != "" {
		metadata["job_definition_id"] = jobDefinitionID
		contextLogger.Debug().
			Str("job_definition_id", jobDefinitionID).
			Msg("Job definition ID stored in job metadata")
	}

	// Create JobModel
	jobModel := &models.JobModel{
		ID:        jobID,
		ParentID:  nil, // Parent jobs have no parent
		Type:      string(models.JobTypeParent),
		Name:      fmt.Sprintf("Crawl %s %s", sourceType, entityType),
		Config:    jobConfig,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		Depth:     0,
	}

	// Create Job with runtime state
	job := &models.Job{
		JobModel: jobModel,
		Status:   JobStatusPending,
		Progress: &models.JobProgress{
			TotalURLs:     len(seedURLs),
			CompletedURLs: 0,
			FailedURLs:    0,
			PendingURLs:   len(seedURLs),
			Percentage:    0,
		},
		ResultCount: 0,
		FailedCount: 0,
	}

	// Log the source type being used for audit trail and debugging
	contextLogger.Info().Str("source_type", sourceType).Str("entity_type", entityType).Msg("Creating crawl job with source type")

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

	// Store auth snapshot if provided
	var httpClientType string
	if authSnapshot != nil {
		// Store auth snapshot in job config
		authJSON, err := json.Marshal(authSnapshot)
		if err != nil {
			// Log auth snapshot serialization failure
			contextLogger.Error().Err(err).Msg("Failed to serialize auth snapshot")
			return "", fmt.Errorf("failed to serialize auth snapshot: %w", err)
		}
		job.Config["auth_snapshot"] = string(authJSON)

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
		contextLogger.Info().Msg("No auth snapshot provided - requests will use default HTTP client")
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

	// Publish EventJobCreated after successful job persistence
	if s.eventService != nil {
		createdEvent := interfaces.Event{
			Type: interfaces.EventJobCreated,
			Payload: map[string]interface{}{
				"job_id":         jobID,
				"status":         "pending",
				"source_type":    sourceType,
				"entity_type":    entityType,
				"seed_url_count": len(seedURLs),
				"max_depth":      config.MaxDepth,
				"max_pages":      config.MaxPages,
				"follow_links":   config.FollowLinks,
				"timestamp":      time.Now(),
			},
		}
		if err := s.eventService.Publish(s.ctx, createdEvent); err != nil {
			contextLogger.Warn().Err(err).Msg("Failed to publish job created event")
		}
	}

	// NOTE: Pre-validation disabled - no executor registered for this job type yet
	// To implement: create PreValidationExecutor and register with JobProcessor
	contextLogger.Debug().
		Str("job_id", jobID).
		Msg("Pre-validation skipped (not yet implemented in new queue system)")

	s.jobsMu.Lock()
	s.activeJobs[jobID] = job
	s.jobResults[jobID] = make([]*CrawlResult, 0)
	s.jobsMu.Unlock()

	// Seed queue with job messages for crawler workers
	// Build config map for job messages
	messageConfig := map[string]interface{}{
		"max_depth":      config.MaxDepth,
		"max_pages":      config.MaxPages,
		"follow_links":   config.FollowLinks,
		"source_type":    sourceType,
		"entity_type":    entityType,
		"rate_limit":     config.RateLimit.Milliseconds(),
		"concurrency":    config.Concurrency,
		"retry_attempts": config.RetryAttempts,
		"retry_backoff":  config.RetryBackoff.Milliseconds(),
	}

	// Add include/exclude patterns from config to job message
	if len(config.IncludePatterns) > 0 {
		messageConfig["include_patterns"] = config.IncludePatterns
	}
	if len(config.ExcludePatterns) > 0 {
		messageConfig["exclude_patterns"] = config.ExcludePatterns
	}

	// Enqueue seed URLs as job messages
	actuallyEnqueued := 0
	for i, seedURL := range seedURLs {
		// Generate child job ID
		childID := fmt.Sprintf("%s-seed-%d", jobID, i)

		// Persist seed URL as child Job record before enqueueing
		// This ensures child job exists in database when worker picks up the message

		// Build child job config
		childConfig := make(map[string]interface{})
		childConfig["crawl_config"] = config
		childConfig["source_type"] = sourceType
		childConfig["entity_type"] = entityType
		childConfig["seed_url"] = seedURL

		// Create child JobModel
		childJobModel := &models.JobModel{
			ID:        childID,
			ParentID:  &jobID,
			Type:      string(models.JobTypeCrawlerURL),
			Name:      fmt.Sprintf("URL: %s", seedURL),
			Config:    childConfig,
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			Depth:     1,
		}

		// Create child Job with runtime state
		childJob := &models.Job{
			JobModel: childJobModel,
			Status:   JobStatusPending,
			Progress: &models.JobProgress{
				TotalURLs:     1,
				PendingURLs:   1,
				CompletedURLs: 0,
				FailedURLs:    0,
				Percentage:    0,
			},
			ResultCount: 0,
			FailedCount: 0,
		}

		// Save child job to database
		if s.jobStorage != nil {
			if err := s.jobStorage.SaveJob(s.ctx, childJob); err != nil {
				contextLogger.Warn().
					Err(err).
					Str("child_id", childID).
					Str("seed_url", seedURL).
					Msg("Failed to persist seed child job to database, continuing with enqueue")
				// Continue on save error - don't block enqueueing
			} else {
				contextLogger.Debug().
					Str("child_id", childID).
					Str("seed_url", seedURL).
					Str("parent_id", jobID).
					Msg("Seed child job persisted to database")
			}
		}

		// Enqueue message to queue for processing
		if s.queueManager != nil {
			// Serialize the child JobModel to JSON for queue payload
			payloadJSON, err := childJob.JobModel.ToJSON()
			if err != nil {
				contextLogger.Warn().
					Err(err).
					Str("seed_url", seedURL).
					Str("child_id", childID).
					Msg("Failed to serialize child job model")
				continue
			}

			msg := queue.Message{
				JobID:   childID,
				Type:    string(models.JobTypeCrawlerURL),
				Payload: payloadJSON,
			}

			if err := s.queueManager.Enqueue(s.ctx, msg); err != nil {
				contextLogger.Warn().
					Err(err).
					Str("seed_url", seedURL).
					Str("child_id", childID).
					Msg("Failed to enqueue seed URL")
				continue
			}

			contextLogger.Debug().
				Str("child_id", childID).
				Str("seed_url", seedURL).
				Msg("Seed URL enqueued successfully")
		}

		actuallyEnqueued++
	}

	// Update PendingURLs and TotalURLs to match actual queue state
	s.jobsMu.Lock()
	job.Progress.PendingURLs = actuallyEnqueued
	job.Progress.TotalURLs = actuallyEnqueued
	s.jobsMu.Unlock()

	// Note: Job remains in JobStatusPending state until first worker picks up a URL
	// At that point, Execute() will transition to JobStatusRunning and publish EventJobStarted

	// Initialize browser pool if JavaScript rendering is enabled
	if s.config.Crawler.EnableJavaScript {
		if err := s.initBrowserPool(config.Concurrency); err != nil {
			s.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to initialize browser pool")
			return "", fmt.Errorf("failed to initialize browser pool: %w", err)
		}
	}

	// Note: Workers are managed globally by queue.WorkerPool (started at app initialization)
	// Job messages are already enqueued and will be picked up by workers automatically

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
	// Create contextLogger at function start for consistent logging to both console and database
	contextLogger := s.logger.WithContextWriter(jobID)

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
	now := time.Now()
	job.CompletedAt = &now

	// Sync result counts with progress counters before terminating
	if job.Progress != nil {
		job.ResultCount = job.Progress.CompletedURLs
		job.FailedCount = job.Progress.FailedURLs
	}

	// Extract source_type and entity_type from config
	sourceType, _ := job.Config["source_type"].(string)
	entityType, _ := job.Config["entity_type"].(string)

	s.jobsMu.Unlock()

	// Persist cancellation status to database (outside lock to avoid contention)
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			contextLogger.Warn().Err(err).Msg("Failed to persist job cancellation")
		}

		// Publish EventJobCancelled after successful job cancellation
		if s.eventService != nil {
			completedURLs := 0
			pendingURLs := 0
			if job.Progress != nil {
				completedURLs = job.Progress.CompletedURLs
				pendingURLs = job.Progress.PendingURLs
			}

			cancelledEvent := interfaces.Event{
				Type: interfaces.EventJobCancelled,
				Payload: map[string]interface{}{
					"job_id":         jobID,
					"status":         "cancelled",
					"source_type":    sourceType,
					"entity_type":    entityType,
					"result_count":   job.ResultCount,
					"failed_count":   job.FailedCount,
					"completed_urls": completedURLs,
					"pending_urls":   pendingURLs,
					"timestamp":      time.Now(),
				},
			}
			if err := s.eventService.Publish(s.ctx, cancelledEvent); err != nil {
				contextLogger.Warn().Err(err).Msg("Failed to publish job cancelled event")
			}
		}

		// Append cancellation log with structured fields for queryability
		contextLogger.Warn().
			Int("completed", job.Progress.CompletedURLs).
			Int("failed", job.Progress.FailedURLs).
			Int("pending", job.Progress.PendingURLs).
			Msg("Job cancelled by user")
	}

	// Reacquire lock to clean up per-job HTTP client map and remove from activeJobs
	s.jobsMu.Lock()
	if _, exists := s.jobClients[jobID]; exists {
		delete(s.jobClients, jobID)
		contextLogger.Debug().Msg("Cleaned up per-job HTTP client after cancellation")
	}
	// Remove from activeJobs since job is now in terminal state
	delete(s.activeJobs, jobID)
	contextLogger.Debug().Msg("Removed cancelled job from active jobs")
	s.jobsMu.Unlock()

	return nil
}

// FailJob marks a job as failed with a reason (called by scheduler for stale job detection)
func (s *Service) FailJob(jobID string, reason string) error {
	// Create contextLogger at function start for consistent logging to both console and database
	contextLogger := s.logger.WithContextWriter(jobID)

	// Acquire lock to check job and update status
	s.jobsMu.Lock()
	job, exists := s.activeJobs[jobID]
	if !exists {
		s.jobsMu.Unlock()
		return fmt.Errorf("job not found in active jobs: %s", jobID)
	}

	// Set job status to failed
	job.Status = JobStatusFailed
	now := time.Now()
	job.CompletedAt = &now
	job.Error = reason

	// Sync result counts with progress counters before terminating
	if job.Progress != nil {
		job.ResultCount = job.Progress.CompletedURLs
		job.FailedCount = job.Progress.FailedURLs
	}

	// Extract source_type and entity_type from config
	sourceType, _ := job.Config["source_type"].(string)
	entityType, _ := job.Config["entity_type"].(string)

	s.jobsMu.Unlock()

	// Persist failed status to database (outside lock to avoid contention)
	if s.jobStorage != nil {
		if err := s.jobStorage.SaveJob(s.ctx, job); err != nil {
			contextLogger.Warn().Err(err).Msg("Failed to persist job failure")
		}

		// Publish EventJobFailed after successful job failure persistence
		if s.eventService != nil {
			completedURLs := 0
			pendingURLs := 0
			if job.Progress != nil {
				completedURLs = job.Progress.CompletedURLs
				pendingURLs = job.Progress.PendingURLs
			}

			failedEvent := interfaces.Event{
				Type: interfaces.EventJobFailed,
				Payload: map[string]interface{}{
					"job_id":         jobID,
					"status":         "failed",
					"source_type":    sourceType,
					"entity_type":    entityType,
					"result_count":   job.ResultCount,
					"failed_count":   job.FailedCount,
					"error":          reason,
					"completed_urls": completedURLs,
					"pending_urls":   pendingURLs,
					"timestamp":      time.Now(),
				},
			}
			if err := s.eventService.Publish(s.ctx, failedEvent); err != nil {
				contextLogger.Warn().Err(err).Msg("Failed to publish job failed event")
			}
		}

		// Append failure log with structured fields for queryability
		contextLogger.Error().
			Str("reason", reason).
			Int("completed", job.Progress.CompletedURLs).
			Int("failed", job.Progress.FailedURLs).
			Int("pending", job.Progress.PendingURLs).
			Msg("Job failed")
	}

	// Reacquire lock to clean up per-job HTTP client map and remove from activeJobs
	s.jobsMu.Lock()
	if _, exists := s.jobClients[jobID]; exists {
		delete(s.jobClients, jobID)
		contextLogger.Debug().Msg("Cleaned up per-job HTTP client after failure")
	}
	// Remove from activeJobs since job is now in terminal state
	delete(s.activeJobs, jobID)
	contextLogger.Debug().Str("reason", reason).Msg("Removed failed job from active jobs")
	s.jobsMu.Unlock()

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
func (s *Service) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) (interface{}, error) {
	if s.jobStorage == nil {
		return nil, fmt.Errorf("job storage not configured")
	}

	jobs, err := s.jobStorage.ListJobs(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	// Jobs are already []*CrawlJob, no conversion needed
	return jobs, nil
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

	originalJob, ok := jobInterface.(*models.Job)
	if !ok || originalJob == nil {
		return "", fmt.Errorf("invalid job type or nil job")
	}

	// Create new job ID
	newJobID := uuid.New().String()
	now := time.Now()

	// Extract seed URLs from original job config
	var seedURLs []string
	if seedURLsRaw, ok := originalJob.Config["seed_urls"]; ok {
		if seedURLsSlice, ok := seedURLsRaw.([]interface{}); ok {
			for _, url := range seedURLsSlice {
				if urlStr, ok := url.(string); ok {
					seedURLs = append(seedURLs, urlStr)
				}
			}
		} else if seedURLsStrSlice, ok := seedURLsRaw.([]string); ok {
			seedURLs = seedURLsStrSlice
		}
	}

	// Copy config map from original job
	newConfig := make(map[string]interface{})
	for k, v := range originalJob.Config {
		newConfig[k] = v
	}

	// Apply config update if provided
	if updateConfig != nil {
		if crawlConfig, ok := updateConfig.(*CrawlConfig); ok && crawlConfig != nil {
			newConfig["crawl_config"] = *crawlConfig
		}
	}

	// Copy metadata from original job
	newMetadata := make(map[string]interface{})
	if originalJob.Metadata != nil {
		for k, v := range originalJob.Metadata {
			newMetadata[k] = v
		}
	}

	// Create new JobModel
	newJobModel := &models.JobModel{
		ID:        newJobID,
		ParentID:  originalJob.ParentID, // Preserve parent relationship
		Type:      originalJob.Type,
		Name:      originalJob.Name,
		Config:    newConfig,
		Metadata:  newMetadata,
		CreatedAt: now,
		Depth:     originalJob.Depth,
	}

	// Create new Job with fresh runtime state
	newJob := &models.Job{
		JobModel: newJobModel,
		Status:   JobStatusPending,
		Progress: &models.JobProgress{
			TotalURLs:     len(seedURLs),
			CompletedURLs: 0,
			FailedURLs:    0,
			PendingURLs:   len(seedURLs),
			Percentage:    0,
		},
		StartedAt:   nil,
		CompletedAt: nil,
		FinishedAt:  nil,
		Error:       "",
		ResultCount: 0,
		FailedCount: 0,
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

// BuildHTTPClientFromAuth creates an HTTP client from auth credentials (wrapper for job use)
func (s *Service) BuildHTTPClientFromAuth(ctx context.Context) (*http.Client, error) {
	if s.authStorage == nil {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	// Get all auth credentials and use the first one
	credsList, err := s.authStorage.ListCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list auth credentials: %w", err)
	}

	// Return default client if no credentials available
	if len(credsList) == 0 {
		return &http.Client{Timeout: 30 * time.Second}, nil
	}

	// Use first available credential
	return buildHTTPClientFromAuth(credsList[0])
}

// GetConfig returns the crawler configuration
func (s *Service) GetConfig() common.CrawlerConfig {
	return s.config.Crawler
}

// buildHTTPClientFromAuth creates an HTTP client with cookies from AuthCredentials
func buildHTTPClientFromAuth(authCreds *models.AuthCredentials) (*http.Client, error) {
	return httpclient.NewHTTPClientWithAuth(authCreds)
}
