// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 8:17:54 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ternarybob/arbor"
	arbormodels "github.com/ternarybob/arbor/models"
	"github.com/ternarybob/arbor/services/logviewer"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/queue/state"
	"github.com/ternarybob/quaero/internal/queue/workers"
	"github.com/ternarybob/quaero/internal/services/agents"
	"github.com/ternarybob/quaero/internal/services/auth"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/config"
	"github.com/ternarybob/quaero/internal/services/connectors"
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
	"github.com/timshannon/badgerhold/v4"
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
	QueueManager interfaces.QueueManager
	LogService   interfaces.LogService
	LogConsumer  *logs.Consumer // Log consumer for arbor context channel
	JobManager   *queue.Manager
	StepManager  *queue.StepManager
	Orchestrator *queue.Orchestrator
	JobProcessor *workers.JobProcessor
	JobMonitor   interfaces.JobMonitor
	StepMonitor  interfaces.StepMonitor
	JobService   *jobsvc.Service

	// Source-agnostic services
	StatusService     *status.Service
	SystemLogsService *logviewer.Service

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

	// Variables service (key/value storage)
	KVService *kv.Service

	// Config service
	ConfigService interfaces.ConfigService

	// Connector service
	ConnectorService interfaces.ConnectorService

	// HTTP handlers
	APIHandler           *handlers.APIHandler
	AuthHandler          *handlers.AuthHandler
	KVHandler            *handlers.KVHandler
	WSHandler            *handlers.WebSocketHandler
	DocumentHandler      *handlers.DocumentHandler
	SearchHandler        *handlers.SearchHandler
	SchedulerHandler     *handlers.SchedulerHandler
	MCPHandler           *handlers.MCPHandler
	JobHandler           *handlers.JobHandler
	StatusHandler        *handlers.StatusHandler
	ConfigHandler        *handlers.ConfigHandler
	PageHandler          *handlers.PageHandler
	JobDefinitionHandler *handlers.JobDefinitionHandler
	SystemLogsHandler    *handlers.SystemLogsHandler
	ConnectorHandler     *handlers.ConnectorHandler
	GitHubJobsHandler    *handlers.GitHubJobsHandler
	HybridScraperHandler *handlers.HybridScraperHandler
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
		app.StorageManager.QueueStorage(),
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

	// Configure Arbor with channel from consumer
	// Use a custom writer adapter to send all logs to the channel
	logBatchChannel := logConsumer.GetChannel()

	// We can use SetChannel("context") for derived loggers
	app.Logger.SetChannel("context", logBatchChannel)

	// For the root logger, we need to ensure logs go to the channel too.
	// Since Arbor doesn't expose a direct "WithChannelWriter", we rely on the fact that
	// the consumer is now processing all logs (including those without correlation ID).
	// However, we need to make sure the root logger actually WRITES to this channel.
	// If SetChannel only affects context-aware logs, we might need to wrap the logger or use a different approach.
	//
	// Let's try to use the "context" channel which we set above.
	// If the root logger doesn't use it, we might need to add a custom writer.
	// But for now, let's assume SetChannel works for derived loggers which is the primary use case for job logs.
	// For system logs (root logger), if they are missing, we might need to add a ConsoleWriter that also writes to channel?
	// No, that's messy.

	// Let's try to use a custom writer configuration if possible, but Arbor API is limited here.
	// Actually, let's look at how MemoryWriter works. It writes to a list.
	// We want to write to a channel.

	// Reverting to SetChannel("context") as it's the standard way.
	// The issue might have been the consumer filtering.
	// Let's verify if consumer filtering fix is enough.

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
	app.Logger.Debug().Msg("Job processor started")

	// Authentication will be loaded on-demand when needed (e.g., when crawler service uses it)
	// This prevents noisy debug logs during startup when no credentials exist

	// Start WebSocket background tasks for real-time UI updates
	app.WSHandler.StartStatusBroadcaster()

	logger.Debug().Msg("WebSocket handlers started (status broadcaster)")

	// Log initialization summary
	logger.Info().
		Str("processing_enabled", fmt.Sprintf("%v", cfg.Processing.Enabled)).
		Bool("crawler_enabled", true).
		Msg("Application initialization complete")

	return app, nil
}

// initDatabase initializes the storage layer (Badger)
func (a *App) initDatabase() error {
	storageManager, err := storage.NewStorageManager(a.Logger, a.Config)
	if err != nil {
		return fmt.Errorf("failed to create storage manager: %w", err)
	}

	a.StorageManager = storageManager
	a.Logger.Debug().
		Str("storage", "badger").
		Str("path", a.Config.Storage.Badger.Path).
		Msg("Storage layer initialized")

	// Load variables from files (e.g. API keys, secrets)
	// This must happen before config replacement so that loaded variables can be used
	if err := a.StorageManager.LoadVariablesFromFiles(context.Background(), a.Config.Variables.Dir); err != nil {
		// Log warning but don't fail startup (consistent with other loaders)
		a.Logger.Warn().Err(err).Msg("Failed to load variables from files")
	}

	// Load variables from .env file (takes precedence over TOML variables)
	// This allows API keys to be stored in .env files for security
	if err := a.StorageManager.LoadEnvFile(context.Background(), ".env"); err != nil {
		// Log warning but don't fail startup (consistent with other loaders)
		a.Logger.Warn().Err(err).Msg("Failed to load .env file")
	}

	// Load job definitions from files
	// This happens after variables are loaded so that job definitions can reference variables
	if err := a.StorageManager.LoadJobDefinitionsFromFiles(context.Background(), a.Config.Jobs.DefinitionsDir); err != nil {
		// Log warning but don't fail startup (consistent with other loaders)
		a.Logger.Warn().Err(err).Msg("Failed to load job definitions from files")
	}

	// Load connectors from files
	// This happens after variables are loaded so that connector tokens can reference variables
	if err := a.StorageManager.LoadConnectorsFromFiles(context.Background(), a.Config.Connectors.Dir); err != nil {
		// Log warning but dont fail startup (consistent with other loaders)
		a.Logger.Warn().Err(err).Msg("Failed to load connectors from files")
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
			a.Logger.Debug().Int("keys", len(kvMap)).Msg("Applied key/value replacements to config")
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
	a.LLMService, err = llm.NewGeminiService(&a.Config.Gemini, a.StorageManager, a.Logger)
	if err != nil {
		a.LLMService = nil // Explicitly set to nil on error
		a.Logger.Warn().Err(err).Msg("Failed to initialize LLM service - chat features will be unavailable")
		a.Logger.Info().Msg("To enable LLM features, set QUAERO_GEMINI_GOOGLE_API_KEY or gemini.google_api_key in config")
	} else {
		// Perform health check to validate API key and connectivity
		if err := a.LLMService.HealthCheck(context.Background()); err != nil {
			// Set to nil if health check fails (invalid/placeholder API key)
			a.LLMService = nil
			a.Logger.Warn().Err(err).Msg("LLM service health check failed - service disabled")
			a.Logger.Info().Msg("To enable LLM features, provide a valid Google Gemini API key")
		} else {
			a.Logger.Debug().Msg("LLM service initialized and health check passed")
		}
	}

	// Initialize event service (already created in New() before LogService setup)

	// 5.5. Initialize status service
	a.StatusService = status.NewService(a.EventService, a.Logger)
	a.StatusService.SubscribeToCrawlerEvents()
	a.Logger.Debug().Msg("Status service initialized")

	// 5.5.1 Initialize system logs service
	// Calculate logs directory (same logic as main.go)
	execPath, err := os.Executable()
	var logsDir string
	if err == nil {
		logsDir = filepath.Join(filepath.Dir(execPath), "logs")
	} else {
		logsDir = "logs" // Fallback
	}

	// Create writer config for log viewer
	// Note: We point to the same file that the logger is writing to
	logViewerConfig := arbormodels.WriterConfiguration{
		Type:       arbormodels.LogWriterTypeFile,
		FileName:   filepath.Join(logsDir, "quaero.log"),
		TimeFormat: "15:04:05",
	}

	a.SystemLogsService = logviewer.NewService(logViewerConfig)
	a.Logger.Debug().Str("logs_dir", logsDir).Msg("System logs service initialized")

	// 5.6. Initialize queue manager (Badger-backed)
	// Obtain underlying Badger DB from storage manager
	// StorageManager.DB() returns *badgerhold.Store, we need to extract the underlying *badger.DB
	badgerStore, ok := a.StorageManager.DB().(*badgerhold.Store)
	if !ok {
		return fmt.Errorf("storage manager is not backed by BadgerDB (got %T)", a.StorageManager.DB())
	}

	// Extract underlying *badger.DB from BadgerHold wrapper
	badgerDB := badgerStore.Badger()

	queueMgr, err := queue.NewBadgerManager(
		badgerDB,
		a.Config.Queue.QueueName,
		parseDuration(a.Config.Queue.VisibilityTimeout),
		a.Config.Queue.MaxReceive,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize queue manager: %w", err)
	}
	a.QueueManager = queueMgr
	a.Logger.Debug().Str("queue_name", a.Config.Queue.QueueName).Msg("Queue manager initialized")

	// 5.8. Initialize job manager with storage interfaces
	jobMgr := queue.NewManager(
		a.StorageManager.QueueStorage(),
		a.StorageManager.JobLogStorage(),
		queueMgr,
		a.EventService,
		a.Logger,
	)
	a.JobManager = jobMgr
	a.Logger.Debug().Msg("Job manager initialized")

	// 5.8.1 Initialize StepManager
	stepMgr := queue.NewStepManager(a.Logger)
	a.StepManager = stepMgr
	a.Logger.Debug().Msg("Step manager initialized")

	// 5.9. Initialize job processor (replaces worker pool)
	jobProcessor := workers.NewJobProcessor(queueMgr, jobMgr, a.Logger, a.Config.Queue.Concurrency)
	a.JobProcessor = jobProcessor
	a.Logger.Debug().Msg("Job processor initialized")

	// 5.10. Initialize job service for high-level job operations
	a.JobService = jobsvc.NewService(jobMgr, queueMgr, a.Logger)
	a.Logger.Debug().Msg("Job service initialized")

	// 5.11. Initialize variables service with event publishing
	a.KVService = kv.NewService(
		a.StorageManager.KeyValueStorage(),
		a.EventService,
		a.Logger,
	)
	a.Logger.Debug().Msg("Variables service initialized")

	// 5.12. Initialize config service with event-driven cache invalidation
	a.ConfigService, err = config.NewService(
		a.Config,
		a.StorageManager.KeyValueStorage(),
		a.EventService,
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize config service: %w", err)
	}
	a.Logger.Debug().Msg("Config service initialized")

	// 5.13. Initialize connector service
	a.ConnectorService = connectors.NewService(
		a.StorageManager.ConnectorStorage(),
		a.Logger,
	)
	a.Logger.Debug().Msg("Connector service initialized")

	// 6. Initialize auth service (Atlassian)
	a.AuthService, err = auth.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 6.5. Initialize crawler service with queue manager for job enqueueing
	a.CrawlerService = crawler.NewService(a.AuthService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.QueueStorage(), a.StorageManager.DocumentStorage(), queueMgr, a.ConnectorService, a.Logger, a.Config)
	if err := a.CrawlerService.Start(); err != nil {
		return fmt.Errorf("failed to start crawler service: %w", err)
	}
	a.Logger.Debug().Msg("Crawler service initialized")

	// 6.6. Register job executors with job processor
	// NOTE: Unified workers are created later after managers are initialized
	// This is just a placeholder comment - actual worker registration happens after manager creation

	// Create job monitor for monitoring parent job lifecycle
	// NOTE: Parent jobs are NOT registered with JobProcessor - they run in separate goroutines
	// to avoid blocking queue workers with long-running monitoring loops
	jobMonitor := state.NewJobMonitor(
		jobMgr,
		a.EventService,
		a.Logger,
	)
	a.JobMonitor = jobMonitor
	a.Logger.Debug().Msg("Job monitor created")

	// Create step monitor for monitoring step job children (ARCH: Manager -> Steps -> Jobs)
	stepMonitor := state.NewStepMonitor(
		jobMgr,
		a.EventService,
		a.Logger,
	)
	a.StepMonitor = stepMonitor
	a.Logger.Debug().Msg("Step monitor created")

	// Set KV storage on JobManager for placeholder resolution
	jobMgr.SetKVStorage(a.StorageManager.KeyValueStorage())

	// Register database maintenance worker (ARCH-008)
	// Note: BadgerDB handles maintenance automatically, operations are no-ops
	dbMaintenanceWorker := workers.NewDatabaseMaintenanceWorker(
		jobMgr,
		a.Logger,
	)
	jobProcessor.RegisterExecutor(dbMaintenanceWorker)
	a.Logger.Debug().Msg("Database maintenance worker registered")

	// 6.8. Initialize Transform service
	a.TransformService = transform.NewService(a.Logger)
	a.Logger.Debug().Msg("Transform service initialized")

	// 6.8.1. Initialize Places service (Google Places API integration)
	a.PlacesService = places.NewService(
		&a.Config.PlacesAPI,
		a.StorageManager,
		a.EventService,
		a.Logger,
	)
	a.Logger.Debug().Msg("Places service initialized")

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
			a.Logger.Debug().Msg("Chat service initialized and health check passed")
		}
	} else {
		a.ChatService = nil
		a.Logger.Debug().Msg("Chat service not initialized (LLM service unavailable)")
	}

	// 6.8.3. Initialize Agent service (Google ADK with Gemini)
	a.AgentService, err = agents.NewService(
		&a.Config.Gemini,
		a.StorageManager,
		a.Logger,
	)
	if err != nil {
		a.AgentService = nil // Explicitly set to nil on error
		a.Logger.Warn().Err(err).Msg("Failed to initialize agent service - agent features will be unavailable")
		a.Logger.Info().Msg("To enable agents, set QUAERO_GEMINI_GOOGLE_API_KEY or gemini.google_api_key in config")
	} else {
		// Perform health check to validate API key and connectivity
		if err := a.AgentService.HealthCheck(context.Background()); err != nil {
			// Set to nil if health check fails (invalid/placeholder API key)
			a.AgentService = nil
			a.Logger.Warn().Err(err).Msg("Agent service health check failed - service disabled")
			a.Logger.Info().Msg("To enable agents, provide a valid Google Gemini API key")
		} else {
			a.Logger.Debug().Msg("Agent service initialized and health check passed")
		}
	}

	// ============================================================================
	// UNIFIED WORKER REGISTRATION (CONSOLIDATED QUEUE ARCHITECTURE)
	// Register workers directly with JobManager for step routing
	// Workers implement both StepWorker (CreateJobs) and JobWorker (Execute)
	// ============================================================================

	// Register crawler worker (implements both StepWorker and JobWorker)
	crawlerWorker := workers.NewCrawlerWorker(
		a.CrawlerService,
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.StorageManager.AuthStorage(),
		a.StorageManager.JobDefinitionStorage(),
		a.Logger,
		a.EventService,
	)
	a.StepManager.RegisterWorker(crawlerWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(crawlerWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", crawlerWorker.GetType().String()).Str("job_type", crawlerWorker.GetWorkerType()).Msg("Crawler worker registered")

	// Register GitHub Repo worker (implements both StepWorker and JobWorker)
	githubRepoWorker := workers.NewGitHubRepoWorker(
		a.ConnectorService,
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.Logger,
	)
	a.StepManager.RegisterWorker(githubRepoWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(githubRepoWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", githubRepoWorker.GetType().String()).Str("job_type", githubRepoWorker.GetWorkerType()).Msg("GitHub Repo worker registered")

	// Register GitHub Actions worker (implements both StepWorker and JobWorker)
	githubLogWorker := workers.NewGitHubLogWorker(
		a.ConnectorService,
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.Logger,
	)
	a.StepManager.RegisterWorker(githubLogWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(githubLogWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", githubLogWorker.GetType().String()).Str("job_type", githubLogWorker.GetWorkerType()).Msg("GitHub Actions worker registered")

	// Register GitHub Git worker (git clone-based, faster for bulk file downloads)
	githubGitWorker := workers.NewGitHubGitWorker(
		a.ConnectorService,
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.Logger,
	)
	a.StepManager.RegisterWorker(githubGitWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(githubGitWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", githubGitWorker.GetType().String()).Str("job_type", githubGitWorker.GetWorkerType()).Msg("GitHub Git worker registered")

	// Register agent worker if AgentService is available (implements both StepWorker and JobWorker)
	if a.AgentService != nil {
		agentWorker := workers.NewAgentWorker(
			a.AgentService,
			jobMgr,
			queueMgr,
			a.SearchService,
			a.StorageManager.KeyValueStorage(),
			a.StorageManager.DocumentStorage(),
			a.Logger,
			a.EventService,
		)
		a.StepManager.RegisterWorker(agentWorker)  // Register with StepManager for step routing
		jobProcessor.RegisterExecutor(agentWorker) // Register with JobProcessor for job execution
		a.Logger.Debug().Str("step_type", agentWorker.GetType().String()).Str("job_type", agentWorker.GetWorkerType()).Msg("Agent worker registered")
	}

	// Register Places search worker (synchronous execution, no child jobs)
	placesWorker := workers.NewPlacesWorker(
		a.PlacesService,
		a.DocumentService,
		a.EventService,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(placesWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", placesWorker.GetType().String()).Msg("Places search worker registered")

	// Register Web search worker (synchronous execution, no child jobs)
	webSearchWorker := workers.NewWebSearchWorker(
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(webSearchWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", webSearchWorker.GetType().String()).Msg("Web search worker registered")

	a.Logger.Debug().Msg("All workers registered with StepManager")

	// Initialize Orchestrator
	a.Orchestrator = queue.NewOrchestrator(
		jobMgr,
		a.StepManager,
		a.EventService,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)

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
		a.StorageManager.KeyValueStorage(),
		a.CrawlerService,
		a.StorageManager.QueueStorage(),
		a.StorageManager.JobDefinitionStorage(),
		nil, // JobManager handles job execution via ExecuteJobDefinition
	)

	// NOTE: Scheduler triggers event-driven processing:
	// - EventCollectionTriggered: Specialized transformers (Jira/Confluence) transform scraped data to documents
	// Scraping (downloading from Jira/Confluence APIs) remains user-driven via UI
	// Start scheduler BEFORE loading job settings to ensure job definitions are loaded first
	a.Logger.Debug().Msg("Calling SchedulerService.Start()")
	if err := a.SchedulerService.Start("*/5 * * * *"); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to start scheduler service")
	} else {
		a.Logger.Debug().Msg("Scheduler service started")
	}
	a.Logger.Debug().Msg("SchedulerService.Start() returned")

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
	a.Logger.Debug().
		Int("allowed_events", len(a.Config.WebSocket.AllowedEvents)).
		Int("throttle_intervals", len(a.Config.WebSocket.ThrottleIntervals)).
		Msg("EventSubscriber initialized")

	a.AuthHandler = handlers.NewAuthHandler(a.AuthService, a.StorageManager.AuthStorage(), a.WSHandler, a.Logger)

	a.KVHandler = handlers.NewKVHandler(a.KVService, a.Logger)
	a.Logger.Debug().Msg("KV handler initialized")

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
	)

	// Initialize MCP handler with SearchService
	mcpService := mcp.NewDocumentService(
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.Logger,
	)
	a.MCPHandler = handlers.NewMCPHandler(mcpService, a.Logger)

	// Initialize job handler with JobManager and EventService
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.QueueStorage(), a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.EventService, a.Config, a.Logger)

	// Initialize status handler
	a.StatusHandler = handlers.NewStatusHandler(a.StatusService, a.Logger)

	// Initialize system logs handler
	a.SystemLogsHandler = handlers.NewSystemLogsHandler(a.SystemLogsService, a.Logger)

	// Initialize config handler with ConfigService for dynamic key injection
	a.ConfigHandler = handlers.NewConfigHandler(a.Logger, a.Config, a.ConfigService)

	// Initialize connector handler
	a.ConnectorHandler = handlers.NewConnectorHandler(a.ConnectorService, a.Logger)

	// Initialize GitHub jobs handler
	a.GitHubJobsHandler = handlers.NewGitHubJobsHandler(
		a.ConnectorService,
		a.JobManager,
		a.Orchestrator,
		a.QueueManager,
		a.JobMonitor,
		a.StepMonitor,
		a.Logger,
	)

	// Initialize page handler for serving HTML templates
	a.PageHandler = handlers.NewPageHandler(a.Logger, a.Config.Logging.ClientDebug)

	// Initialize job definition handler
	// Note: JobManager handles job execution via ExecuteJobDefinition
	a.JobDefinitionHandler = handlers.NewJobDefinitionHandler(
		a.StorageManager.JobDefinitionStorage(),
		a.StorageManager.QueueStorage(),
		a.JobManager,
		a.Orchestrator,
		a.JobMonitor,
		a.StepMonitor,
		a.StorageManager.AuthStorage(),
		a.StorageManager.KeyValueStorage(), // For {key-name} replacement in job definitions
		a.StorageManager,                   // For reloading job definitions from disk
		a.Config.Jobs.DefinitionsDir,       // Path to job definitions directory
		a.AgentService,                     // Pass agent service for runtime validation (can be nil)
		a.DocumentService,                  // For direct document capture from extension
		a.Logger,
	)

	// Initialize hybrid scraper handler (lazy initialization - browser launched on-demand)
	a.HybridScraperHandler = handlers.NewHybridScraperHandler(a.Logger)
	a.Logger.Debug().Msg("Hybrid scraper handler initialized")

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	// Start queue stats broadcaster and stale job detector with cancellable context
	a.ctx, a.cancelCtx = context.WithCancel(context.Background())

	// Start stale job detector (runs every 5 minutes)
	go func() {
		// Panic recovery to prevent service crash from stale job detector errors
		defer func() {
			if r := recover(); r != nil {
				a.Logger.Error().
					Str("panic", fmt.Sprintf("%v", r)).
					Str("stack", common.GetStackTrace()).
					Msg("Recovered from panic in stale job detector - detector stopped")
			}
		}()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check for jobs that have been running for more than 15 minutes without heartbeat
				staleJobs, err := a.StorageManager.QueueStorage().GetStaleJobs(context.Background(), 15)
				if err != nil {
					a.Logger.Warn().Err(err).Msg("Failed to check for stale jobs")
					continue
				}

				if len(staleJobs) > 0 {
					a.Logger.Warn().
						Int("count", len(staleJobs)).
						Msg("Detected stale jobs - marking as failed")

					for _, job := range staleJobs {
						if err := a.StorageManager.QueueStorage().UpdateJobStatus(
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
	a.Logger.Debug().Msg("Stale job detector started")

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

	// Shutdown orchestrator (cancels all background polling tasks)
	// TODO Phase 8-11: Re-enable once Orchestrator is re-integrated
	// if a.Orchestrator != nil {
	// 	a.Orchestrator.Shutdown()
	// 	a.Logger.Info().Msg("Orchestrator shutdown complete")
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

	// Close config service
	if a.ConfigService != nil {
		if err := a.ConfigService.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to close config service")
		} else {
			a.Logger.Info().Msg("Config service closed")
		}
	}

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

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
