// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 8:17:54 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/jobs/manager"
	"github.com/ternarybob/quaero/internal/jobs/monitor"
	"github.com/ternarybob/quaero/internal/jobs/worker"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/services/agents"
	"github.com/ternarybob/quaero/internal/services/auth"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/events"
	"github.com/ternarybob/quaero/internal/services/identifiers"
	jobsvc "github.com/ternarybob/quaero/internal/services/jobs"
	"github.com/ternarybob/quaero/internal/services/kv"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/services/mcp"
	"github.com/ternarybob/quaero/internal/services/places"
	"github.com/ternarybob/quaero/internal/services/scheduler"
	"github.com/ternarybob/quaero/internal/services/search"
	"github.com/ternarybob/quaero/internal/services/status"
	"github.com/ternarybob/quaero/internal/services/summary"
	"github.com/ternarybob/quaero/internal/services/transform"
	"github.com/ternarybob/quaero/internal/storage"
	"github.com/ternarybob/quaero/internal/storage/sqlite"
)

// App holds all application components and dependencies
type App struct {
	Config         *common.Config
	Logger         arbor.ILogger
	ctx            context.Context
	cancelCtx      context.CancelFunc
	StorageManager interfaces.StorageManager

	// Document services
	DocumentService   interfaces.DocumentService
	SearchService     interfaces.SearchService
	IdentifierService *identifiers.Extractor

	// Event-driven services
	EventService     interfaces.EventService
	SchedulerService interfaces.SchedulerService
	SummaryService   *summary.Service

	// Job execution (using concrete types for refactored queue system)
	QueueManager              *queue.Manager
	LogService                interfaces.LogService
	LogConsumer               *logs.Consumer // Log consumer for arbor context channel
	JobManager                *jobs.Manager
	JobProcessor              *worker.JobProcessor
	JobDefinitionOrchestrator *jobs.JobDefinitionOrchestrator
	JobService                *jobsvc.Service

	// Source-agnostic services
	StatusService *status.Service

	// Authentication service (supports multiple providers)
	AuthService *auth.Service

	// Crawler service
	CrawlerService *crawler.Service

	// Transform service
	TransformService *transform.Service

	// Places service
	PlacesService interfaces.PlacesService

	// Agent service
	AgentService interfaces.AgentService

	// LLM service (Google ADK)
	LLMService interfaces.LLMService

	// Chat service (agent-based)
	ChatService interfaces.ChatService

	// Key/Value service
	KVService *kv.Service

	// HTTP handlers
	APIHandler           *handlers.APIHandler
	AuthHandler          *handlers.AuthHandler
	KVHandler            *handlers.KVHandler
	WSHandler            *handlers.WebSocketHandler
	CollectionHandler    *handlers.CollectionHandler
	DocumentHandler      *handlers.DocumentHandler
	SearchHandler        *handlers.SearchHandler
	SchedulerHandler     *handlers.SchedulerHandler
	MCPHandler           *handlers.MCPHandler
	JobHandler           *handlers.JobHandler
	StatusHandler        *handlers.StatusHandler
	ConfigHandler        *handlers.ConfigHandler
	PageHandler          *handlers.PageHandler
	JobDefinitionHandler *handlers.JobDefinitionHandler
}

// New initializes the application with all dependencies
func New(cfg *common.Config, logger arbor.ILogger) (*App, error) {
	app := &App{
		Config: cfg,
		Logger: logger,
	}

	// Initialize database
	if err := app.initDatabase(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize WebSocket handler (required for LogService)
	// Must be created early so LogService can broadcast logs via WebSocket
	// EventService is needed for WebSocketHandler initialization
	app.EventService = events.NewService(app.Logger)
	app.WSHandler = handlers.NewWebSocketHandler(app.EventService, app.Logger, &app.Config.WebSocket)

	// Initialize log service (simplified to storage operations only)
	logService := logs.NewService(
		app.StorageManager.JobLogStorage(),
		app.StorageManager.JobStorage(),
		app.Logger,
	)
	app.LogService = logService

	// Create log consumer for arbor context channel
	// Consumer handles log batching, storage, and event publishing
	logConsumer := logs.NewConsumer(
		app.StorageManager.JobLogStorage(),
		app.EventService,
		app.Logger,
		app.Config.Logging.MinEventLevel, // Minimum log level for UI events
	)
	if err := logConsumer.Start(); err != nil {
		return nil, fmt.Errorf("failed to start log consumer: %w", err)
	}
	app.LogConsumer = logConsumer

	// Configure Arbor with context channel from consumer
	// This ensures all derived loggers (via WithCorrelationId) send logs to the consumer
	logBatchChannel := logConsumer.GetChannel()
	app.Logger.SetChannel("context", logBatchChannel)

	app.Logger.Info().
		Int("channel_capacity", cap(logBatchChannel)).
		Int("channel_length", len(logBatchChannel)).
		Msg("Log consumer initialized with Arbor context channel")

	// Initialize services (AFTER LogService is configured)
	if err := app.initServices(); err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	// Initialize handlers
	if err := app.initHandlers(); err != nil {
		return nil, fmt.Errorf("failed to initialize handlers: %w", err)
	}

	// Start job processor AFTER all handlers are initialized
	// This prevents log channel blocking during initialization
	app.JobProcessor.Start()
	app.Logger.Info().Msg("Job processor started")

	// Authentication will be loaded on-demand when needed (e.g., when crawler service uses it)
	// This prevents noisy debug logs during startup when no credentials exist

	// Start WebSocket background tasks for real-time UI updates
	app.WSHandler.StartStatusBroadcaster()

	logger.Info().Msg("WebSocket handlers started (status broadcaster)")

	// Log initialization summary
	logger.Info().
		Str("processing_enabled", fmt.Sprintf("%v", cfg.Processing.Enabled)).
		Bool("crawler_enabled", true).
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

	// Load user-defined job definitions from TOML/JSON files
	if sqliteMgr, ok := storageManager.(*sqlite.Manager); ok {
		ctx := context.Background()
		if err := sqliteMgr.LoadJobDefinitionsFromFiles(ctx, a.Config.Jobs.DefinitionsDir); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to load job definitions from files")
		}
	}

	// Load cookie-based auth credentials from files (after job definitions)
	// Note: This is for cookie-based authentication only (captured via Chrome extension or manual TOML files)
	// API keys are loaded separately via LoadKeysFromFiles() below
	if sqliteMgr, ok := storageManager.(*sqlite.Manager); ok {
		ctx := context.Background()
		if err := sqliteMgr.LoadAuthCredentialsFromFiles(ctx, a.Config.Auth.CredentialsDir); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to load cookie-based auth credentials from files")
			// Don't fail startup - auth files are optional
		} else {
			a.Logger.Info().Str("dir", a.Config.Auth.CredentialsDir).Msg("Cookie-based auth credentials loaded from files")
		}

		// Load key/value pairs from files (after auth credentials)
		// This is separate from auth - auth is for cookies, keys are for API keys and generic secrets
		if err := sqliteMgr.LoadKeysFromFiles(ctx, a.Config.Keys.Dir); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to load key/value pairs from files")
			// Don't fail startup - key files are optional
		} else {
			a.Logger.Info().Str("dir", a.Config.Keys.Dir).Msg("Key/value pairs loaded from files")
		}

		// Migrate API keys from auth_credentials to key_value_store (idempotent)
		// This separates concerns: auth_credentials for cookie auth, key_value_store for API keys
		if err := sqliteMgr.MigrateAPIKeysToKVStore(ctx); err != nil {
			a.Logger.Warn().Err(err).Msg("API key migration to KV store failed")
			// Don't fail startup - migration is not critical (backward compatibility maintained)
		} else {
			a.Logger.Info().Msg("API key migration to KV store completed successfully")
		}
	}

	// Phase 2: Perform {key-name} replacement in config after storage initialization
	// This replaces any {key-name} references in config values with actual KV store values
	// Must happen BEFORE services (LLM, Agent, Places) are initialized
	ctx := context.Background()
	kvMap, err := a.StorageManager.KeyValueStorage().GetAll(ctx)
	if err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to fetch KV map for config replacement, skipping replacement")
	} else if len(kvMap) > 0 {
		if err := common.ReplaceInStruct(a.Config, kvMap, a.Logger); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to replace key references in config")
		} else {
			a.Logger.Info().Int("keys", len(kvMap)).Msg("Applied key/value replacements to config")
		}
	} else {
		a.Logger.Debug().Msg("No key/value pairs found, skipping config replacement")
	}

	return nil
}

// initServices initializes all business services in dependency order.
//
// QUEUE-BASED JOB ARCHITECTURE:
// 1. QueueManager (goqite-backed) - Persistent queue
// 2. JobManager - CRUD operations for jobs
// 3. JobProcessor - Processes jobs from the queue (replaced legacy WorkerPool)
// 4. Job Executors - CrawlerExecutor handles crawler_url jobs
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

	// Initialize document service
	a.DocumentService = documents.NewService(
		a.StorageManager.DocumentStorage(),
		a.Logger,
	)

	// 3.5 Initialize search service using factory (supports fts5, advanced, disabled modes)
	a.SearchService, err = search.NewSearchService(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		a.Config,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize search service: %w", err)
	}

	// 3.6. Initialize LLM service (Google ADK with Gemini)
	a.LLMService, err = llm.NewGeminiService(a.Config, a.StorageManager, a.Logger)
	if err != nil {
		a.LLMService = nil // Explicitly set to nil on error
		a.Logger.Warn().Err(err).Msg("Failed to initialize LLM service - chat features will be unavailable")
		a.Logger.Info().Msg("To enable LLM features, set QUAERO_LLM_GOOGLE_API_KEY or llm.google_api_key in config")
	} else {
		// Perform health check to validate API key and connectivity
		if err := a.LLMService.HealthCheck(context.Background()); err != nil {
			a.Logger.Warn().Err(err).Msg("LLM service health check failed - API key may be invalid")
		} else {
			a.Logger.Info().Msg("LLM service initialized and health check passed")
		}
	}

	// Initialize event service (already created in New() before LogService setup)

	// 5.5. Initialize status service
	a.StatusService = status.NewService(a.EventService, a.Logger)
	a.StatusService.SubscribeToCrawlerEvents()
	a.Logger.Info().Msg("Status service initialized")

	// 5.6. Initialize queue manager (goqite-backed)
	queueMgr, err := queue.NewManager(a.StorageManager.DB().(*sql.DB), a.Config.Queue.QueueName)
	if err != nil {
		return fmt.Errorf("failed to initialize queue manager: %w", err)
	}
	a.QueueManager = queueMgr
	a.Logger.Info().Str("queue_name", a.Config.Queue.QueueName).Msg("Queue manager initialized")

	// 5.8. Initialize job manager with event service for status change publishing
	jobMgr := jobs.NewManager(a.StorageManager.DB().(*sql.DB), queueMgr, a.EventService)
	a.JobManager = jobMgr
	a.Logger.Info().Msg("Job manager initialized")

	// 5.9. Initialize job processor (replaces worker pool)
	jobProcessor := worker.NewJobProcessor(queueMgr, jobMgr, a.Logger)
	a.JobProcessor = jobProcessor
	a.Logger.Info().Msg("Job processor initialized")

	// 5.10. Initialize job service for high-level job operations
	a.JobService = jobsvc.NewService(jobMgr, queueMgr, a.Logger)
	a.Logger.Info().Msg("Job service initialized")

	// 5.11. Initialize key/value service
	a.KVService = kv.NewService(
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)
	a.Logger.Info().Msg("Key/value service initialized")

	// 6. Initialize auth service (Atlassian)
	a.AuthService, err = auth.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 6.5. Initialize crawler service with queue manager for job enqueueing
	a.CrawlerService = crawler.NewService(a.AuthService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.StorageManager.DocumentStorage(), queueMgr, a.Logger, a.Config)
	if err := a.CrawlerService.Start(); err != nil {
		return fmt.Errorf("failed to start crawler service: %w", err)
	}
	a.Logger.Info().Msg("Crawler service initialized")

	// 6.6. Register job executors with job processor

	// Register crawler_url worker (ChromeDP rendering and content processing)
	crawlerWorker := worker.NewCrawlerWorker(
		a.CrawlerService,
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.StorageManager.AuthStorage(),
		a.StorageManager.JobDefinitionStorage(),
		a.Logger,
		a.EventService,
	)
	jobProcessor.RegisterExecutor(crawlerWorker)
	a.Logger.Info().Msg("Crawler URL worker registered for job type: crawler_url")

	// Create job monitor for monitoring parent job lifecycle
	// NOTE: Parent jobs are NOT registered with JobProcessor - they run in separate goroutines
	// to avoid blocking queue workers with long-running monitoring loops
	jobMonitor := monitor.NewJobMonitor(
		jobMgr,
		a.EventService,
		a.Logger,
	)
	a.Logger.Info().Msg("Job monitor created (runs in background goroutines, not via queue)")

	// Register agent worker (if agent service is available)
	if a.AgentService != nil {
		agentWorker := worker.NewAgentWorker(
			a.AgentService,
			jobMgr,
			a.StorageManager.DocumentStorage(),
			a.Logger,
			a.EventService,
		)
		jobProcessor.RegisterExecutor(agentWorker)
		a.Logger.Info().Msg("Agent worker registered for job type: agent")
	}

	// Register database maintenance worker (ARCH-008)
	dbMaintenanceWorker := worker.NewDatabaseMaintenanceWorker(
		a.StorageManager.DB().(*sql.DB),
		jobMgr,
		a.Logger,
	)
	jobProcessor.RegisterExecutor(dbMaintenanceWorker)
	a.Logger.Info().Msg("Database maintenance worker registered for job type: database_maintenance_operation")

	// 6.8. Initialize Transform service
	a.TransformService = transform.NewService(a.Logger)
	a.Logger.Info().Msg("Transform service initialized")

	// 6.8.1. Initialize Places service (Google Places API integration)
	a.PlacesService = places.NewService(
		&a.Config.PlacesAPI,
		a.StorageManager,
		a.EventService,
		a.Logger,
	)
	a.Logger.Info().Msg("Places service initialized")

	// 6.8.2. Initialize Chat service (depends on LLM service)
	if a.LLMService != nil {
		a.ChatService = chat.NewChatService(
			a.LLMService,
			a.StorageManager.DocumentStorage(),
			a.SearchService,
			a.Logger,
		)
		// Perform health check to validate service is operational
		if err := a.ChatService.HealthCheck(context.Background()); err != nil {
			a.Logger.Warn().Err(err).Msg("Chat service health check failed")
		} else {
			a.Logger.Info().Msg("Chat service initialized and health check passed")
		}
	} else {
		a.ChatService = nil
		a.Logger.Info().Msg("Chat service not initialized (LLM service unavailable)")
	}

	// 6.8.3. Initialize Agent service (Google ADK with Gemini)
	a.AgentService, err = agents.NewService(
		&a.Config.Agent,
		a.StorageManager,
		a.Logger,
	)
	if err != nil {
		a.AgentService = nil // Explicitly set to nil on error
		a.Logger.Warn().Err(err).Msg("Failed to initialize agent service - agent features will be unavailable")
		a.Logger.Info().Msg("To enable agents, set QUAERO_AGENT_GOOGLE_API_KEY or agent.google_api_key in config")
	} else {
		// Perform health check to validate API key and connectivity
		if err := a.AgentService.HealthCheck(context.Background()); err != nil {
			a.Logger.Warn().Err(err).Msg("Agent service health check failed - API key may be invalid")
		} else {
			a.Logger.Info().Msg("Agent service initialized and health check passed")
		}
	}

	// 6.9. Initialize JobDefinitionOrchestrator for job definition execution (ARCH-009)
	// Pass jobMonitor so it can start monitoring goroutines for crawler jobs
	a.JobDefinitionOrchestrator = jobs.NewJobDefinitionOrchestrator(jobMgr, jobMonitor, a.Logger)

	// Register managers for job definition steps
	crawlerManager := manager.NewCrawlerManager(a.CrawlerService, a.Logger)
	a.JobDefinitionOrchestrator.RegisterStepExecutor(crawlerManager)
	a.Logger.Info().Msg("Crawler manager registered")

	transformManager := manager.NewTransformManager(a.TransformService, a.JobManager, a.Logger)
	a.JobDefinitionOrchestrator.RegisterStepExecutor(transformManager)
	a.Logger.Info().Msg("Transform manager registered")

	reindexManager := manager.NewReindexManager(a.StorageManager.DocumentStorage(), a.JobManager, a.Logger)
	a.JobDefinitionOrchestrator.RegisterStepExecutor(reindexManager)
	a.Logger.Info().Msg("Reindex manager registered")

	dbMaintenanceManager := manager.NewDatabaseMaintenanceManager(a.JobManager, queueMgr, jobMonitor, a.Logger)
	a.JobDefinitionOrchestrator.RegisterStepExecutor(dbMaintenanceManager)
	a.Logger.Info().Msg("Database maintenance manager registered (ARCH-008)")

	placesSearchManager := manager.NewPlacesSearchManager(a.PlacesService, a.DocumentService, a.EventService, a.StorageManager.KeyValueStorage(), a.StorageManager.AuthStorage(), a.Logger)
	a.JobDefinitionOrchestrator.RegisterStepExecutor(placesSearchManager)
	a.Logger.Info().Msg("Places search manager registered")

	// Register agent manager (if agent service is available)
	if a.AgentService != nil {
		agentManager := manager.NewAgentManager(jobMgr, queueMgr, a.SearchService, a.StorageManager.KeyValueStorage(), a.StorageManager.AuthStorage(), a.Logger)
		a.JobDefinitionOrchestrator.RegisterStepExecutor(agentManager)
		a.Logger.Info().Msg("Agent manager registered")
	}

	a.Logger.Info().Msg("JobDefinitionOrchestrator initialized with all managers (ARCH-009)")

	// NOTE: Job processor will be started AFTER scheduler initialization to avoid deadlock

	// Initialize summary service
	a.SummaryService = summary.NewService(
		a.StorageManager.DocumentStorage(),
		a.DocumentService,
		a.Logger,
	)

	// NOTE: Old job executor/registry/actions system removed
	// Queue-based system (goqite + JobProcessor + Executors) now handles all job execution

	// 12. Initialize scheduler service with database persistence and job definition support
	a.SchedulerService = scheduler.NewServiceWithDB(
		a.EventService,
		a.Logger,
		a.StorageManager.DB().(*sql.DB),
		a.CrawlerService,
		a.StorageManager.JobStorage(),
		a.StorageManager.JobDefinitionStorage(),
		nil, // JobDefinitionOrchestrator temporarily disabled
	)

	// NOTE: Scheduler triggers event-driven processing:
	// - EventCollectionTriggered: Specialized transformers (Jira/Confluence) transform scraped data to documents
	// - EventEmbeddingTriggered: Generates embeddings for unembedded documents
	// Scraping (downloading from Jira/Confluence APIs) remains user-driven via UI
	// Start scheduler BEFORE loading job settings to ensure job definitions are loaded first
	a.Logger.Info().Msg("Calling SchedulerService.Start()")
	if err := a.SchedulerService.Start("*/5 * * * *"); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to start scheduler service")
	} else {
		a.Logger.Info().Msg("Scheduler service started")
	}
	a.Logger.Info().Msg("SchedulerService.Start() returned")

	// Load persisted job settings from database AFTER scheduler has started
	// This ensures job definitions are loaded before applying settings
	if loadSvc, ok := a.SchedulerService.(interface{ LoadJobSettings() error }); ok {
		if err := loadSvc.LoadJobSettings(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to load job settings from database")
		}
	}

	// Cleanup orphaned jobs from previous run
	if err := a.SchedulerService.CleanupOrphanedJobs(); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to cleanup orphaned jobs")
	}

	// NOTE: JobProcessor.Start() moved to New() after initHandlers() completes
	// This prevents log channel blocking during handler initialization

	return nil
}

// initHandlers initializes all HTTP handlers
func (a *App) initHandlers() error {
	// Initialize handlers
	a.APIHandler = handlers.NewAPIHandler(a.Logger)
	// WSHandler already initialized in New() before LogService setup
	// NOTE: WebSocketHandler is created early to support LogService broadcasting

	// Initialize EventSubscriber for job lifecycle events with config-driven filtering
	// Subscribes to EventJobCreated, EventJobStarted, EventJobCompleted, EventJobFailed, EventJobCancelled
	// Transforms events and broadcasts to WebSocket clients via BroadcastJobStatusChange
	_ = handlers.NewEventSubscriber(a.WSHandler, a.EventService, a.Logger, &a.Config.WebSocket)
	a.Logger.Info().
		Int("allowed_events", len(a.Config.WebSocket.AllowedEvents)).
		Int("throttle_intervals", len(a.Config.WebSocket.ThrottleIntervals)).
		Msg("EventSubscriber initialized with config-driven filtering and throttling")

	a.AuthHandler = handlers.NewAuthHandler(a.AuthService, a.StorageManager.AuthStorage(), a.WSHandler, a.Logger)

	a.KVHandler = handlers.NewKVHandler(a.KVService, a.Logger)
	a.Logger.Info().Msg("KV handler initialized")

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

	// Initialize MCP handler with SearchService
	mcpService := mcp.NewDocumentService(
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.Logger,
	)
	a.MCPHandler = handlers.NewMCPHandler(mcpService, a.Logger)

	// Initialize job handler with JobManager
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.Config, a.Logger)

	// Initialize status handler
	a.StatusHandler = handlers.NewStatusHandler(a.StatusService, a.Logger)

	// Initialize config handler
	a.ConfigHandler = handlers.NewConfigHandler(a.Logger, a.Config)

	// Initialize page handler for serving HTML templates
	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.Logging.ClientDebug)

	// Initialize job definition handler
	// Note: JobExecutor and JobRegistry are nil during queue refactor, but handler can work without them
	db, ok := a.StorageManager.DB().(*sql.DB)
	if !ok {
		return fmt.Errorf("storage manager DB is not *sql.DB")
	}
	a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
		a.StorageManager.JobDefinitionStorage(),
		a.StorageManager.JobStorage(),
		a.JobDefinitionOrchestrator,
		a.StorageManager.AuthStorage(),
		a.StorageManager.KeyValueStorage(), // For {key-name} replacement in job definitions
		a.AgentService,                     // Pass agent service for runtime validation (can be nil)
		db,                                 // Pass *sql.DB for validation service
		a.Logger,
	)

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
							"Timeout: No activity for 15+ minutes - check network connectivity, increase timeout, or verify job is not stuck",
						); err != nil {
							a.Logger.Warn().
								Err(err).
								Str("job_id", job.ID).
								Msg("Failed to mark stale job as failed")
						} else {
							// Log with job context for better debugging
							var url string
							if seedURLs, ok := job.Config["seed_urls"].([]interface{}); ok && len(seedURLs) > 0 {
								if urlStr, ok := seedURLs[0].(string); ok {
									url = urlStr
								}
							}
							a.Logger.Info().
								Str("job_id", job.ID).
								Str("job_name", job.Name).
								Str("job_type", job.Type).
								Str("url", url).
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
	// TODO Phase 8-11: Re-enable when queue manager is integrated
	// go func() {
	// 	ticker := time.NewTicker(5 * time.Second)
	// 	defer ticker.Stop()

	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			// Get queue stats
	// 			stats, err := a.QueueManager.GetQueueStats(context.Background())
	// 			if err != nil {
	// 				a.Logger.Warn().Err(err).Msg("Failed to get queue stats")
	// 				continue
	// 			}

	// 			// Broadcast to WebSocket clients
	// 			update := handlers.QueueStatsUpdate{
	// 				TotalMessages:    getInt(stats, "total_messages"),
	// 				PendingMessages:  getInt(stats, "pending_messages"),
	// 				InFlightMessages: getInt(stats, "in_flight_messages"),
	// 				QueueName:        getString(stats, "queue_name"),
	// 				Concurrency:      getInt(stats, "concurrency"),
	// 				Timestamp:        time.Now(),
	// 			}
	// 			a.WSHandler.BroadcastQueueStats(update)
	// 		case <-a.ctx.Done():
	// 			a.Logger.Info().Msg("Queue stats broadcaster shutting down")
	// 			return
	// 		}
	// 	}
	// }()
	// a.Logger.Info().Msg("Queue stats broadcaster started")

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
	// Note: Arbor's Stop() is idempotent and safe to call multiple times
	// but should only be called once at end of shutdown sequence
	a.Logger.Info().Msg("Flushing context logs")
	common.Stop()

	// Stop scheduler service
	if a.SchedulerService != nil {
		if err := a.SchedulerService.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop scheduler service")
		}
	}

	// Stop job processor
	if a.JobProcessor != nil {
		a.JobProcessor.Stop()
		a.Logger.Info().Msg("Job processor stopped")
	}

	// Stop log consumer
	if a.LogConsumer != nil {
		if err := a.LogConsumer.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop log consumer")
		} else {
			a.Logger.Info().Msg("Log consumer stopped")
		}
	}

	// Note: QueueManager (goqite) doesn't require explicit stop - it's stateless

	// Shutdown job definition orchestrator (cancels all background polling tasks)
	// TODO Phase 8-11: Re-enable once JobDefinitionOrchestrator is re-integrated
	// if a.JobDefinitionOrchestrator != nil {
	// 	a.JobDefinitionOrchestrator.Shutdown()
	// 	a.Logger.Info().Msg("Job definition orchestrator shutdown complete")
	// }

	// Close crawler service
	if a.CrawlerService != nil {
		if err := a.CrawlerService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close crawler service")
		}
	}

	// Close agent service
	if a.AgentService != nil {
		if err := a.AgentService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close agent service")
		}
	}

	// Close LLM service
	if a.LLMService != nil {
		if err := a.LLMService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close LLM service")
		} else {
			a.Logger.Info().Msg("LLM service closed")
		}
	}

	// Close chat service (no explicit Close method, just nil reference)
	a.ChatService = nil

	// Close event service
	if a.EventService != nil {
		if err := a.EventService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close event service")
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
