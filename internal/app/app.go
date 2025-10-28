// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:30 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	arbormodels "github.com/ternarybob/arbor/models"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	jobmgr "github.com/ternarybob/quaero/internal/jobs"
	jobtypes "github.com/ternarybob/quaero/internal/jobs/types"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	"github.com/ternarybob/quaero/internal/services/auth"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/config"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/events"
	"github.com/ternarybob/quaero/internal/services/identifiers"
	"github.com/ternarybob/quaero/internal/services/jobs"
	"github.com/ternarybob/quaero/internal/services/jobs/actions"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/services/mcp"
	"github.com/ternarybob/quaero/internal/services/scheduler"
	"github.com/ternarybob/quaero/internal/services/search"
	"github.com/ternarybob/quaero/internal/services/sources"
	"github.com/ternarybob/quaero/internal/services/status"
	"github.com/ternarybob/quaero/internal/services/summary"
	"github.com/ternarybob/quaero/internal/storage"
)

// App holds all application components and dependencies
type App struct {
	Config          *common.Config // Deprecated: Use ConfigService instead
	ConfigService   interfaces.ConfigService
	Logger          arbor.ILogger
	logBatchChannel chan []arbormodels.LogEvent
	ctx             context.Context
	cancelCtx       context.CancelFunc
	StorageManager  interfaces.StorageManager

	// Document services
	LLMService        interfaces.LLMService
	AuditLogger       llm.AuditLogger
	DocumentService   interfaces.DocumentService
	SearchService     interfaces.SearchService
	IdentifierService *identifiers.Extractor
	ChatService       interfaces.ChatService

	// Event-driven services
	EventService     interfaces.EventService
	SchedulerService interfaces.SchedulerService
	SummaryService   *summary.Service

	// Job execution
	JobRegistry  *jobs.JobTypeRegistry
	JobExecutor  *jobs.JobExecutor
	QueueManager interfaces.QueueManager
	LogService   interfaces.LogService
	JobManager   interfaces.JobManager
	WorkerPool   interfaces.WorkerPool

	// Specialized transformers
	JiraTransformer       *atlassian.JiraTransformer
	ConfluenceTransformer *atlassian.ConfluenceTransformer

	// Source-agnostic services
	StatusService *status.Service
	SourceService *sources.Service

	// Authentication service (supports multiple providers)
	AuthService *auth.Service

	// Crawler service
	CrawlerService *crawler.Service

	// HTTP handlers
	APIHandler           *handlers.APIHandler
	AuthHandler          *handlers.AuthHandler
	WSHandler            *handlers.WebSocketHandler
	CollectionHandler    *handlers.CollectionHandler
	DocumentHandler      *handlers.DocumentHandler
	SearchHandler        *handlers.SearchHandler
	SchedulerHandler     *handlers.SchedulerHandler
	ChatHandler          *handlers.ChatHandler
	MCPHandler           *handlers.MCPHandler
	JobHandler           *handlers.JobHandler
	SourcesHandler       *handlers.SourcesHandler
	StatusHandler        *handlers.StatusHandler
	ConfigHandler        *handlers.ConfigHandler
	PageHandler          *handlers.PageHandler
	JobDefinitionHandler *handlers.JobDefinitionHandler
}

// New initializes the application with all dependencies
func New(cfg *common.Config, logger arbor.ILogger) (*App, error) {
	// Create ConfigService for dependency injection
	configService := config.NewService(cfg)

	app := &App{
		Config:        cfg,           // Deprecated: kept for backward compatibility
		ConfigService: configService, // Use this for new code
		Logger:        logger,
	}

	// Initialize database
	if err := app.initDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize services
	if err := app.initServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	// Initialize handlers
	if err := app.initHandlers(); err != nil {
		return nil, fmt.Errorf("failed to initialize handlers: %w", err)
	}

	// Load stored authentication if available
	if _, err := app.AuthService.LoadAuth(); err == nil {
		logger.Info().Msg("Loaded stored authentication credentials")
	}

	// Start WebSocket background tasks for real-time UI updates
	app.WSHandler.StartStatusBroadcaster()
	app.WSHandler.StartLogStreamer()
	logger.Info().Msg("WebSocket handlers started for real-time updates")

	// Log initialization summary
	enabledSources := []string{}
	if cfg.Sources.Jira.Enabled {
		enabledSources = append(enabledSources, "Jira")
	}
	if cfg.Sources.Confluence.Enabled {
		enabledSources = append(enabledSources, "Confluence")
	}
	if cfg.Sources.GitHub.Enabled {
		enabledSources = append(enabledSources, "GitHub")
	}

	logger.Info().
		Strs("enabled_sources", enabledSources).
		Str("llm_mode", cfg.LLM.Mode).
		Str("processing_enabled", fmt.Sprintf("%v", cfg.Processing.Enabled)).
		Msg("Application initialization complete")

	return app, nil
}

// initDatabase initializes the storage layer (SQLite)
func (a *App) initDatabase() error {
	storageManager, err := storage.NewStorageManager(a.Logger, a.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage manager: %w", err)
	}

	a.StorageManager = storageManager
	a.Logger.Info().
		Str("type", a.Config.Storage.Type).
		Str("path", a.Config.Storage.SQLite.Path).
		Str("fts5_enabled", fmt.Sprintf("%v", a.Config.Storage.SQLite.EnableFTS5)).
		Str("vector_enabled", fmt.Sprintf("%v", a.Config.Storage.SQLite.EnableVector)).
		Msg("Storage layer initialized")

	return nil
}

// initServices initializes all business services in dependency order.
//
// QUEUE-BASED JOB ARCHITECTURE:
// 1. QueueManager (goqite-backed) - Persistent queue with worker pool
// 2. JobManager - CRUD operations for jobs
// 3. WorkerPool - Registers handlers for job types (crawler_url, summarizer, cleanup, reindex, parent)
// 4. Job Types - CrawlerJob, SummarizerJob, CleanupJob, ReindexJob (handle individual tasks)
//
// JOB DEFINITION ARCHITECTURE:
// 1. JobRegistry - Maps job types to action handlers
// 2. JobExecutor - Orchestrates multi-step workflows with retry and polling
// 3. Action Handlers - CrawlerActions, SummarizerActions (registered with JobRegistry)
//
// Both systems coexist:
// - Queue system: Handles individual task execution (URLs, summaries, cleanup)
// - JobExecutor: Orchestrates user-defined workflows (JobDefinitions)
// - JobDefinitions can trigger crawl jobs, which are executed by the queue system
func (a *App) initServices() error {
	var err error

	// 1. Initialize LLM service (required for embeddings)
	a.LLMService, a.AuditLogger, err = llm.NewLLMService(
		a.Config,
		a.StorageManager.DB(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM service: %w", err)
	}

	// Log LLM mode
	mode := a.LLMService.GetMode()
	a.Logger.Info().
		Str("mode", string(mode)).
		Msg("LLM service initialized")

	// 1.5. Initialize log service (before context logging)
	logService := logs.NewService(a.StorageManager.JobLogStorage(), a.Logger)
	if err := logService.Start(); err != nil {
		return fmt.Errorf("failed to start log service: %w", err)
	}
	a.LogService = logService
	a.Logger.Info().Msg("Log service initialized")

	// 1.6. Initialize context logging (after log service)
	// Create channel for log batches (buffer size 10 allows up to 10 batches to queue)
	logBatchChannel := make(chan []arbormodels.LogEvent, 10)
	a.logBatchChannel = logBatchChannel

	// Configure Arbor with context channel (default buffering: batch size 5, flush interval 1s)
	a.Logger.SetContextChannel(logBatchChannel)

	// Start consumer goroutine to process log batches and write to database
	go func() {
		for batch := range logBatchChannel {
			for _, event := range batch {
				// Extract jobID from CorrelationID
				jobID := event.CorrelationID
				if jobID == "" {
					continue // Skip logs without jobID
				}

				// Convert Level.String() to lowercase (already lowercase from phuslu/log)
				levelStr := event.Level.String()

				// Format Timestamp to "15:04:05" format
				formattedTime := event.Timestamp.Format("15:04:05")

				// Build message with fields if present
				message := event.Message
				if len(event.Fields) > 0 {
					// Append fields to message for database persistence
					for key, value := range event.Fields {
						message += fmt.Sprintf(" %s=%v", key, value)
					}
				}

				// Create JobLogEntry
				logEntry := models.JobLogEntry{
					Timestamp: formattedTime,
					Level:     levelStr,
					Message:   message,
				}

				// Write to database (non-blocking, use background context)
				a.LogService.AppendLog(context.Background(), jobID, logEntry)
			}
		}
	}()
	a.Logger.Info().Msg("Context logging initialized with database persistence")

	// 2. Initialize embedding service (now uses LLM abstraction)
	// NOTE: Phase 4 - EmbeddingService removed completely
	// a.EmbeddingService = embeddings.NewService(
	// 	a.LLMService,
	// 	a.AuditLogger,
	// 	a.Config.Embeddings.Dimension,
	// 	a.Logger,
	// )

	// 3. Initialize document service (no longer uses EmbeddingService)
	a.DocumentService = documents.NewService(
		a.StorageManager.DocumentStorage(),
		a.Logger,
	)

	// 3.5 Initialize search service (Advanced search with Google-style query parsing)
	// AdvancedSearchService requires FTS5 for full-text search capabilities
	if !a.Config.Storage.SQLite.EnableFTS5 {
		// FTS5 is disabled: use no-op service to allow app to start
		// The handler will return 503 Service Unavailable for search requests
		a.Logger.Warn().
			Bool("fts5_enabled", a.Config.Storage.SQLite.EnableFTS5).
			Msg("FTS5 is disabled: search service will be unavailable (using DisabledSearchService)")
		a.SearchService = search.NewDisabledSearchService(a.Logger)
	} else {
		// FTS5 is enabled: use full-featured search service
		a.SearchService = search.NewAdvancedSearchService(
			a.StorageManager.DocumentStorage(),
			a.Logger,
			a.Config,
		)
		a.Logger.Info().
			Bool("fts5_enabled", a.Config.Storage.SQLite.EnableFTS5).
			Msg("Advanced search service initialized with FTS5 support")
	}

	// 4. Initialize chat service (agent-based chat with LLM)
	a.ChatService = chat.NewChatService(
		a.LLMService,
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.Logger,
	)

	// 5. Initialize event service (must be early for subscriptions)
	a.EventService = events.NewService(a.Logger)

	// 5.5. Initialize status service
	a.StatusService = status.NewService(a.EventService, a.Logger)
	a.StatusService.SubscribeToCrawlerEvents()
	a.Logger.Info().Msg("Status service initialized")

	// 5.6. Initialize source service
	a.SourceService = sources.NewService(
		a.StorageManager.SourceStorage(),
		a.StorageManager.AuthStorage(),
		a.EventService,
		a.Logger,
	)
	a.Logger.Info().Msg("Source service initialized")

	// 5.7. Initialize queue manager
	// Parse configured queue settings
	pollInterval, err := time.ParseDuration(a.Config.Queue.PollInterval)
	if err != nil {
		return fmt.Errorf("failed to parse queue poll interval: %w", err)
	}

	visibilityTimeout, err := time.ParseDuration(a.Config.Queue.VisibilityTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse queue visibility timeout: %w", err)
	}

	queueConfig := queue.Config{
		PollInterval:      pollInterval,
		Concurrency:       a.Config.Queue.Concurrency,
		VisibilityTimeout: visibilityTimeout,
		MaxReceive:        a.Config.Queue.MaxReceive,
		QueueName:         a.Config.Queue.QueueName,
	}

	queueMgr, err := queue.NewManager(a.StorageManager.DB().(*sql.DB), queueConfig, a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize queue manager: %w", err)
	}
	if err := queueMgr.Start(); err != nil {
		return fmt.Errorf("failed to start queue manager: %w", err)
	}
	a.QueueManager = queueMgr
	a.Logger.Info().Msg("Queue manager initialized")

	// 5.9. Initialize job manager
	jobMgr := jobmgr.NewManager(a.StorageManager.JobStorage(), queueMgr, logService, a.Logger)
	a.JobManager = jobMgr
	a.Logger.Info().Msg("Job manager initialized")

	// 5.10. Initialize worker pool with job storage for lifecycle management
	workerPool := queue.NewWorkerPool(queueMgr, a.StorageManager.JobStorage(), a.Logger)
	a.WorkerPool = workerPool
	a.Logger.Info().Msg("Worker pool initialized")

	// 5.11. Startup recovery: Mark orphaned running jobs from previous session
	// These jobs were interrupted by ungraceful shutdown or crash
	orphanedCount, err := a.StorageManager.JobStorage().MarkRunningJobsAsPending(
		context.Background(),
		"Service restart detected - resuming interrupted jobs",
	)
	if err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to mark orphaned running jobs on startup")
	} else if orphanedCount > 0 {
		a.Logger.Warn().
			Int("count", orphanedCount).
			Msg("Marked orphaned running jobs as pending for recovery")
	}

	// 6. Initialize auth service (Atlassian)
	a.AuthService, err = auth.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 6.5. Initialize crawler service
	a.CrawlerService = crawler.NewService(a.AuthService, a.SourceService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.StorageManager.DocumentStorage(), a.QueueManager, a.Logger, a.Config)
	if err := a.CrawlerService.Start(); err != nil {
		return fmt.Errorf("failed to start crawler service: %w", err)
	}
	a.Logger.Info().Msg("Crawler service initialized")

	// 6.6. Initialize specialized transformers (subscribe to collection events)
	// NOTE: Must be initialized after crawler service to access GetJobResults()
	a.JiraTransformer = atlassian.NewJiraTransformer(
		a.StorageManager.JobStorage(),
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.CrawlerService, // Add crawler service parameter
		a.Logger,
		true, // enableEmptyOutputFallback
	)
	a.Logger.Info().Msg("Jira transformer initialized and subscribed to collection events")

	a.ConfluenceTransformer = atlassian.NewConfluenceTransformer(
		a.StorageManager.JobStorage(),
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.CrawlerService, // Add crawler service parameter
		a.Logger,
		true, // enableEmptyOutputFallback
	)
	a.Logger.Info().Msg("Confluence transformer initialized and subscribed to collection events")

	// 6.7. Register job handlers with worker pool
	// Crawler job handler
	crawlerJobDeps := &jobtypes.CrawlerJobDeps{
		CrawlerService:  a.CrawlerService,
		LogService:      a.LogService,
		DocumentStorage: a.StorageManager.DocumentStorage(),
		QueueManager:    a.QueueManager,
		JobStorage:      a.StorageManager.JobStorage(),
		EventService:    a.EventService,
	}
	crawlerJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewCrawlerJob(baseJob, crawlerJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("crawler_url", crawlerJobHandler)
	a.Logger.Info().Msg("Crawler job handler registered")

	// Summarizer job handler
	summarizerJobDeps := &jobtypes.SummarizerJobDeps{
		LLMService:      a.LLMService,
		DocumentStorage: a.StorageManager.DocumentStorage(),
	}
	summarizerJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewSummarizerJob(baseJob, summarizerJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("summarizer", summarizerJobHandler)
	a.Logger.Info().Msg("Summarizer job handler registered")

	// Cleanup job handler
	cleanupJobDeps := &jobtypes.CleanupJobDeps{
		JobStorage: a.StorageManager.JobStorage(),
		LogService: a.LogService,
	}
	cleanupJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewCleanupJob(baseJob, cleanupJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("cleanup", cleanupJobHandler)
	a.Logger.Info().Msg("Cleanup job handler registered")

	// Reindex job handler
	reindexJobDeps := &jobtypes.ReindexJobDeps{
		DocumentStorage: a.StorageManager.DocumentStorage(),
	}
	reindexJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewReindexJob(baseJob, reindexJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("reindex", reindexJobHandler)
	a.Logger.Info().Msg("Reindex job handler registered")

	// Parent job handler (for job definition execution)
	parentJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())

		// Determine target job ID for status updates (fallback to msg.ID if ParentID is empty)
		targetID := msg.ParentID
		if targetID == "" {
			targetID = msg.ID
		}

		// Extract job definition ID from message
		jobDefID := msg.JobDefinitionID
		if jobDefID == "" {
			if val, ok := msg.Config["job_definition_id"]; ok {
				if id, ok := val.(string); ok {
					jobDefID = id
				}
			}
		}

		if jobDefID == "" {
			baseJob.UpdateJobStatus(ctx, targetID, "failed", "Job definition ID not found in message")
			return fmt.Errorf("job definition ID not found in parent message")
		}

		// Load job definition from storage
		jobDef, err := a.StorageManager.JobDefinitionStorage().GetJobDefinition(ctx, jobDefID)
		if err != nil {
			baseJob.UpdateJobStatus(ctx, targetID, "failed", fmt.Sprintf("Failed to load job definition: %v", err))
			a.Logger.Error().Err(err).Str("job_def_id", jobDefID).Msg("Failed to load job definition")
			return fmt.Errorf("failed to load job definition: %w", err)
		}

		// Update job status to running
		baseJob.UpdateJobStatus(ctx, targetID, "running", fmt.Sprintf("Executing job definition: %s", jobDef.Name))

		// Extract execution chain from parent message metadata for cycle prevention
		executionChain := make(map[string]bool)
		if msg.Metadata != nil {
			if chain, ok := msg.Metadata["job_execution_chain"]; ok {
				if chainSlice, ok := chain.([]interface{}); ok {
					for _, id := range chainSlice {
						if idStr, ok := id.(string); ok {
							executionChain[idStr] = true
						}
					}
				}
			}
		}

		// Add current job to execution chain
		executionChain[jobDefID] = true
		a.Logger.Debug().
			Str("job_def_id", jobDefID).
			Int("chain_length", len(executionChain)).
			Msg("Job execution chain initialized")

		// Create post-job trigger callback for post-job execution
		postJobCallback := func(callbackCtx context.Context, postJobDef *models.JobDefinition) error {
			a.Logger.Info().
				Str("parent_job_id", targetID).
				Str("post_job_id", postJobDef.ID).
				Str("post_job_name", postJobDef.Name).
				Msg("Post-job trigger requested")

			// Cycle detection: Check if postJobID is already in the execution chain
			if executionChain[postJobDef.ID] {
				a.Logger.Warn().
					Str("post_job_id", postJobDef.ID).
					Str("post_job_name", postJobDef.Name).
					Str("parent_job_id", targetID).
					Int("chain_length", len(executionChain)).
					Msg("Cycle detected: post-job already in execution chain - skipping to prevent infinite loop")
				return nil // Skip this post-job, but don't fail the parent
			}

			// Create a new parent job message for the post-job
			config := map[string]interface{}{
				"job_definition_id": postJobDef.ID,
				"job_name":          postJobDef.Name,
				"job_type":          string(postJobDef.Type),
			}

			// Add sources if present
			if len(postJobDef.Sources) > 0 {
				config["sources"] = postJobDef.Sources
			}

			// Add steps if present
			if len(postJobDef.Steps) > 0 {
				config["steps"] = postJobDef.Steps
			}

			// Add timeout if present
			if postJobDef.Timeout != "" {
				config["timeout"] = postJobDef.Timeout
			}

			parentMsg := queue.NewJobDefinitionMessage(postJobDef.ID, config)

			// Propagate execution chain to post-job message for cycle prevention
			// Convert map to slice for JSON serialization
			chainSlice := make([]string, 0, len(executionChain))
			for jobID := range executionChain {
				chainSlice = append(chainSlice, jobID)
			}
			if parentMsg.Metadata == nil {
				parentMsg.Metadata = make(map[string]interface{})
			}
			parentMsg.Metadata["job_execution_chain"] = chainSlice

			a.Logger.Debug().
				Str("post_job_id", postJobDef.ID).
				Str("parent_job_id", targetID).
				Int("chain_length", len(chainSlice)).
				Msg("Propagated execution chain to post-job message")

			// Create a job record in database
			job := &models.CrawlJob{
				ID:     parentMsg.ID,
				Name:   postJobDef.Name,
				Status: models.JobStatusPending,
			}
			if err := a.StorageManager.JobStorage().SaveJob(callbackCtx, job); err != nil {
				a.Logger.Error().
					Err(err).
					Str("post_job_id", postJobDef.ID).
					Str("message_id", parentMsg.ID).
					Str("parent_job_id", targetID).
					Msg("Failed to save post-job record")
				return fmt.Errorf("failed to save post-job record: %w", err)
			}

			// Enqueue the message
			if err := a.QueueManager.Enqueue(callbackCtx, parentMsg); err != nil {
				a.Logger.Error().
					Err(err).
					Str("post_job_id", postJobDef.ID).
					Str("message_id", parentMsg.ID).
					Str("parent_job_id", targetID).
					Msg("Failed to enqueue post-job")
				return fmt.Errorf("failed to enqueue post-job: %w", err)
			}

			a.Logger.Info().
				Str("post_job_id", postJobDef.ID).
				Str("post_job_name", postJobDef.Name).
				Str("message_id", parentMsg.ID).
				Str("parent_job_id", targetID).
				Msg("Post-job enqueued successfully")

			return nil
		}

		// Create status update callback for async polling completion
		statusCallback := func(callbackCtx context.Context, status string, errorMsg string) error {
			if err := baseJob.UpdateJobStatus(callbackCtx, targetID, status, errorMsg); err != nil {
				a.Logger.Error().
					Err(err).
					Str("parent_job_id", targetID).
					Str("status", status).
					Msg("Failed to update parent job status from async polling callback")
				return err
			}

			a.Logger.Info().
				Str("parent_job_id", targetID).
				Str("job_def_id", jobDefID).
				Str("job_name", jobDef.Name).
				Str("status", status).
				Msg("Parent job status updated from async polling callback")

			return nil
		}

		// Execute job definition steps with status callback and post-job callback
		result, err := a.JobExecutor.Execute(ctx, jobDef, statusCallback, postJobCallback)
		if err != nil {
			// Only update status if async polling is NOT active
			// (polling goroutine will handle status update via callback)
			if result == nil || !result.AsyncPollingActive {
				baseJob.UpdateJobStatus(ctx, targetID, "failed", fmt.Sprintf("Job execution failed: %v", err))
			}
			a.Logger.Error().Err(err).Str("job_def_id", jobDefID).Str("job_name", jobDef.Name).Msg("Job definition execution failed")
			return fmt.Errorf("job definition execution failed: %w", err)
		}

		// Update job status to completed only if async polling is NOT active
		// If async polling is active, the polling goroutine will handle status update via callback
		if result != nil && result.AsyncPollingActive {
			a.Logger.Debug().
				Str("parent_job_id", targetID).
				Str("job_def_id", jobDefID).
				Str("job_name", jobDef.Name).
				Msg("Async polling active - parent job status update deferred to polling callback")
		} else {
			baseJob.UpdateJobStatus(ctx, targetID, "completed", "Job definition executed successfully")
			a.Logger.Info().Str("job_def_id", jobDefID).Str("job_name", jobDef.Name).Msg("Job definition executed successfully")
		}

		return nil
	}
	a.WorkerPool.RegisterHandler("parent", parentJobHandler)
	a.Logger.Info().Msg("Parent job handler registered")

	// Start worker pool
	if err := a.WorkerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}
	a.Logger.Info().Msg("Worker pool started")

	// 7. Initialize embedding coordinator
	// NOTE: Embedding coordinator disabled during embedding removal (Phase 3)
	// EmbeddingService kept temporarily for backward compatibility
	// a.EmbeddingCoordinator = embeddings.NewCoordinatorService(
	// 	a.EmbeddingService,
	// 	a.StorageManager.DocumentStorage(),
	// 	a.EventService,
	// 	a.Logger,
	// 	a.Config.Processing.Limit,
	// )
	// if err := a.EmbeddingCoordinator.Start(); err != nil {
	// 	return fmt.Errorf("failed to start embedding coordinator: %w", err)
	// }

	// 11.5 Initialize summary service
	a.SummaryService = summary.NewService(
		a.StorageManager.DocumentStorage(),
		a.DocumentService,
		a.Logger,
	)

	// Initialize job executor for job definition execution
	a.JobRegistry = jobs.NewJobTypeRegistry(a.Logger)
	a.JobExecutor, err = jobs.NewJobExecutor(a.JobRegistry, a.SourceService, a.EventService, a.CrawlerService, a.StorageManager.JobDefinitionStorage(), a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize job executor: %w", err)
	}
	a.Logger.Info().Msg("Job executor initialized")

	// Register crawler actions with the job type registry
	crawlerDeps := &actions.CrawlerActionDeps{
		CrawlerService: a.CrawlerService,
		AuthStorage:    a.StorageManager.AuthStorage(),
		EventService:   a.EventService,
		Config:         a.Config,
		Logger:         a.Logger,
	}
	if err = actions.RegisterCrawlerActions(a.JobRegistry, crawlerDeps); err != nil {
		return fmt.Errorf("failed to register crawler actions: %w", err)
	}
	a.Logger.Info().Msg("Crawler actions registered with job type registry")

	// Register summarizer actions with the job type registry
	summarizerDeps := &actions.SummarizerActionDeps{
		DocStorage: a.StorageManager.DocumentStorage(),
		LLMService: a.LLMService,
		Logger:     a.Logger,
	}
	if err = actions.RegisterSummarizerActions(a.JobRegistry, summarizerDeps); err != nil {
		return fmt.Errorf("failed to register summarizer actions: %w", err)
	}
	a.Logger.Info().Msg("Summarizer actions registered with job type registry")

	// Register maintenance actions with the job type registry
	maintenanceDeps := &actions.MaintenanceActionDeps{
		DocumentStorage: a.StorageManager.DocumentStorage(),
		SummaryService:  a.SummaryService,
		Logger:          a.Logger,
	}
	if err = actions.RegisterMaintenanceActions(a.JobRegistry, maintenanceDeps); err != nil {
		return fmt.Errorf("failed to register maintenance actions: %w", err)
	}
	a.Logger.Info().Msg("Maintenance actions registered with job type registry")

	// 12. Initialize scheduler service with database persistence and job definition support
	a.SchedulerService = scheduler.NewServiceWithDB(
		a.EventService,
		a.Logger,
		a.StorageManager.DB().(*sql.DB),
		a.CrawlerService,
		a.StorageManager.JobStorage(),
		a.StorageManager.JobDefinitionStorage(),
		a.JobExecutor,
	)

	// Load persisted job settings from database
	if loadSvc, ok := a.SchedulerService.(interface{ LoadJobSettings() error }); ok {
		if err := loadSvc.LoadJobSettings(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to load job settings from database")
		}
	}

	// Cleanup orphaned jobs from previous run before starting scheduler
	if err := a.SchedulerService.CleanupOrphanedJobs(); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to cleanup orphaned jobs")
	}

	// NOTE: Scheduler triggers event-driven processing:
	// - EventCollectionTriggered: Specialized transformers (Jira/Confluence) transform scraped data to documents
	// - EventEmbeddingTriggered: Generates embeddings for unembedded documents
	// Scraping (downloading from Jira/Confluence APIs) remains user-driven via UI
	if err := a.SchedulerService.Start("*/5 * * * *"); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to start scheduler service")
	} else {
		a.Logger.Info().Msg("Scheduler service started")
	}

	return nil
}

// initHandlers initializes all HTTP handlers
func (a *App) initHandlers() error {
	// Initialize handlers
	a.APIHandler = handlers.NewAPIHandler(a.Logger)
	a.WSHandler = handlers.NewWebSocketHandler(a.EventService, a.Logger)
	a.AuthHandler = handlers.NewAuthHandler(a.AuthService, a.StorageManager.AuthStorage(), a.WSHandler, a.Logger)
	a.CollectionHandler = handlers.NewCollectionHandler(
		a.EventService,
		a.Logger,
	)
	a.DocumentHandler = handlers.NewDocumentHandler(
		a.DocumentService,
		a.StorageManager.DocumentStorage(),
		a.Logger,
	)
	a.SearchHandler = handlers.NewSearchHandler(
		a.SearchService,
		a.Logger,
	)
	a.SchedulerHandler = handlers.NewSchedulerHandler(
		a.SchedulerService,
		a.StorageManager.DocumentStorage(),
	)
	// NOTE: Phase 4 - EmbeddingHandler removed (no longer needed)
	// a.EmbeddingHandler = handlers.NewEmbeddingHandler(
	// 	a.EmbeddingService,
	// 	a.StorageManager.DocumentStorage(),
	// 	a.Logger,
	// )
	a.ChatHandler = handlers.NewChatHandler(
		a.ChatService,
		a.Logger,
	)

	// Initialize MCP handler with SearchService
	mcpService := mcp.NewDocumentService(
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.Logger,
	)
	a.MCPHandler = handlers.NewMCPHandler(mcpService, a.Logger)

	// Initialize job handler
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.Config, a.Logger)

	// Initialize sources handler
	a.SourcesHandler = handlers.NewSourcesHandler(a.SourceService, a.Logger)

	// Initialize status handler
	a.StatusHandler = handlers.NewStatusHandler(a.StatusService, a.Logger)

	// Initialize config handler
	a.ConfigHandler = handlers.NewConfigHandler(a.Logger, a.Config)

	// Initialize page handler for serving HTML templates
	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.Logging.ClientDebug)

	// Initialize job definition handler
	a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
		a.StorageManager.JobDefinitionStorage(),
		a.StorageManager.JobStorage(),
		a.JobExecutor,
		a.SourceService,
		a.JobRegistry,
		a.QueueManager,
		a.Logger,
	)
	a.Logger.Info().Msg("Job definition handler initialized")

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	// Start queue stats broadcaster and stale job detector with cancellable context
	a.ctx, a.cancelCtx = context.WithCancel(context.Background())

	// Start stale job detector (runs every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check for jobs that have been running for more than 15 minutes without heartbeat
				staleJobs, err := a.StorageManager.JobStorage().GetStaleJobs(context.Background(), 15)
				if err != nil {
					a.Logger.Warn().Err(err).Msg("Failed to check for stale jobs")
					continue
				}

				if len(staleJobs) > 0 {
					a.Logger.Warn().
						Int("count", len(staleJobs)).
						Msg("Detected stale jobs - marking as failed")

					for _, job := range staleJobs {
						if err := a.StorageManager.JobStorage().UpdateJobStatus(
							context.Background(),
							job.ID,
							"failed",
							"Job stalled - no heartbeat for 15+ minutes",
						); err != nil {
							a.Logger.Warn().
								Err(err).
								Str("job_id", job.ID).
								Msg("Failed to mark stale job as failed")
						} else {
							a.Logger.Info().
								Str("job_id", job.ID).
								Str("job_name", job.Name).
								Msg("Marked stale job as failed")
						}
					}
				}
			case <-a.ctx.Done():
				a.Logger.Info().Msg("Stale job detector shutting down")
				return
			}
		}
	}()
	a.Logger.Info().Msg("Stale job detector started (checks every 5 minutes)")

	// Start queue stats broadcaster
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Get queue stats
				stats, err := a.QueueManager.GetQueueStats(context.Background())
				if err != nil {
					a.Logger.Warn().Err(err).Msg("Failed to get queue stats")
					continue
				}

				// Broadcast to WebSocket clients
				update := handlers.QueueStatsUpdate{
					TotalMessages:    getInt(stats, "total_messages"),
					PendingMessages:  getInt(stats, "pending_messages"),
					InFlightMessages: getInt(stats, "in_flight_messages"),
					QueueName:        getString(stats, "queue_name"),
					Concurrency:      getInt(stats, "concurrency"),
					Timestamp:        time.Now(),
				}
				a.WSHandler.BroadcastQueueStats(update)
			case <-a.ctx.Done():
				a.Logger.Info().Msg("Queue stats broadcaster shutting down")
				return
			}
		}
	}()
	a.Logger.Info().Msg("Queue stats broadcaster started")

	return nil
}

// Helper functions for safe type conversion from map[string]interface{}
func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Close closes all application resources
func (a *App) Close() error {
	// Cancel queue stats broadcaster goroutine
	if a.cancelCtx != nil {
		a.Logger.Info().Msg("Cancelling background goroutines")
		a.cancelCtx()
		// Allow goroutine to finish gracefully
		time.Sleep(100 * time.Millisecond)
	}

	// Flush context logs before stopping services
	a.Logger.Info().Msg("Flushing context logs")
	common.Stop()

	// Close log batch channel
	if a.logBatchChannel != nil {
		close(a.logBatchChannel)
		// Allow consumer goroutine to process final batch
		time.Sleep(100 * time.Millisecond)
		a.Logger.Info().Msg("Context log channel closed")
	}

	// Stop scheduler service
	if a.SchedulerService != nil {
		if err := a.SchedulerService.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop scheduler service")
		}
	}

	// Stop worker pool
	if a.WorkerPool != nil {
		if err := a.WorkerPool.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop worker pool")
		} else {
			a.Logger.Info().Msg("Worker pool stopped")
		}
	}

	// Stop log service
	if a.LogService != nil {
		if err := a.LogService.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop log service")
		} else {
			a.Logger.Info().Msg("Log service stopped")
		}
	}

	// Stop queue manager
	if a.QueueManager != nil {
		if err := a.QueueManager.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop queue manager")
		}
		a.Logger.Info().Msg("Queue manager stopped")
	}

	// Shutdown job executor (cancels all background polling tasks)
	if a.JobExecutor != nil {
		a.JobExecutor.Shutdown()
		a.Logger.Info().Msg("Job executor shutdown complete")
	}

	// Close crawler service
	if a.CrawlerService != nil {
		if err := a.CrawlerService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close crawler service")
		}
	}

	// Close event service
	if a.EventService != nil {
		if err := a.EventService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close event service")
		}
	}

	// Close LLM service
	if a.LLMService != nil {
		if err := a.LLMService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close LLM service")
		}
	}

	// Close storage
	if a.StorageManager != nil {
		if err := a.StorageManager.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
		a.Logger.Info().Msg("Storage closed")
	}
	return nil
}
