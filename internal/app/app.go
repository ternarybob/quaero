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
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/logs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"github.com/ternarybob/quaero/internal/queue/state"
	"github.com/ternarybob/quaero/internal/queue/workers"
	"github.com/ternarybob/quaero/internal/services/agents"
	"github.com/ternarybob/quaero/internal/services/auth"
	"github.com/ternarybob/quaero/internal/services/cache"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/config"
	"github.com/ternarybob/quaero/internal/services/connectors"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/events"
	"github.com/ternarybob/quaero/internal/services/identifiers"
	"github.com/ternarybob/quaero/internal/services/imap"
	jobsvc "github.com/ternarybob/quaero/internal/services/jobs"
	"github.com/ternarybob/quaero/internal/services/kv"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/services/mailer"
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
	ConfigPaths    []string // Paths to config files for reload functionality
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
	QueueManager  interfaces.QueueManager
	LogService    interfaces.LogService
	LogConsumer   *logs.Consumer // Log consumer for arbor context channel
	JobManager    *queue.Manager
	StepManager   *queue.StepManager
	JobDispatcher *queue.JobDispatcher
	JobProcessor  *workers.JobProcessor
	JobMonitor    interfaces.JobMonitor
	StepMonitor   interfaces.StepMonitor
	JobService    *jobsvc.Service

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

	// Provider factory (multi-provider support: Gemini, Claude)
	ProviderFactory *llm.ProviderFactory

	// Chat service (agent-based)
	ChatService interfaces.ChatService

	// Variables service (key/value storage)
	KVService *kv.Service

	// Config service
	ConfigService interfaces.ConfigService

	// Connector service
	ConnectorService interfaces.ConnectorService

	// Mailer service
	MailerService *mailer.Service

	// IMAP service
	IMAPService *imap.Service

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
	DevOpsHandler        *handlers.DevOpsHandler
	UnifiedLogsHandler   *handlers.UnifiedLogsHandler
	SSELogsHandler       *handlers.SSELogsHandler
	MailerHandler        *handlers.MailerHandler
}

// New initializes the application with all dependencies
// configPaths are stored for reload functionality (optional)
func New(cfg *common.Config, logger arbor.ILogger, configPaths ...string) (*App, error) {
	app := &App{
		Config:      cfg,
		ConfigPaths: configPaths,
		Logger:      logger,
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
		app.StorageManager.LogStorage(),
		app.StorageManager.QueueStorage(),
		app.Logger,
	)
	app.LogService = logService

	// Create log consumer for arbor context channel
	// Consumer handles log batching, storage, and event publishing
	logConsumer := logs.NewConsumer(
		app.StorageManager.LogStorage(),
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

	// Process delete_on_startup array - delete specified data categories
	// This ensures a clean slate before loading fresh configuration from files
	// Each category is processed independently; errors don't prevent other categories from running
	for _, category := range a.Config.DeleteOnStartup {
		switch category {
		case "settings":
			// Clear KV pairs, connectors, and job definitions
			a.Logger.Info().Msg("delete_on_startup: clearing settings (KV pairs, connectors, job definitions)")
			if err := a.StorageManager.ClearAllConfigData(context.Background()); err != nil {
				a.Logger.Error().Err(err).Str("category", category).Msg("Failed to clear settings data on startup")
			} else {
				a.Logger.Info().Msg("Settings data cleared successfully")
			}
		case "jobs":
			// Clear job definitions only
			a.Logger.Info().Msg("delete_on_startup: clearing job definitions")
			if err := a.StorageManager.JobDefinitionStorage().DeleteAllJobDefinitions(context.Background()); err != nil {
				a.Logger.Error().Err(err).Str("category", category).Msg("Failed to clear job definitions on startup")
			} else {
				a.Logger.Info().Msg("Job definitions cleared successfully")
			}
		case "queue":
			// Clear all queue jobs/execution state
			a.Logger.Info().Msg("delete_on_startup: clearing queue jobs")
			if err := a.StorageManager.QueueStorage().ClearAllJobs(context.Background()); err != nil {
				a.Logger.Error().Err(err).Str("category", category).Msg("Failed to clear queue jobs on startup")
			} else {
				a.Logger.Info().Msg("Queue jobs cleared successfully")
			}
		case "documents":
			// Clear all documents
			a.Logger.Info().Msg("delete_on_startup: clearing all documents")
			if err := a.StorageManager.DocumentStorage().ClearAll(); err != nil {
				a.Logger.Error().Err(err).Str("category", category).Msg("Failed to clear documents on startup")
			} else {
				a.Logger.Info().Msg("Documents cleared successfully")
			}
		default:
			a.Logger.Warn().Str("category", category).Msg("Unknown delete_on_startup category - skipping")
		}
	}

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
	// Set cache service first to enable document cleanup for changed job definitions
	jobDefCacheService := cache.NewService(a.StorageManager.DocumentStorage(), a.Logger)
	a.StorageManager.SetCacheService(jobDefCacheService)
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

	// Load email configuration from file
	// This happens after variables are loaded so that email settings can reference variables
	if err := a.StorageManager.LoadEmailFromFile(context.Background(), a.Config.Connectors.Dir); err != nil {
		// Log warning but dont fail startup (consistent with other loaders)
		a.Logger.Warn().Err(err).Msg("Failed to load email config from file")
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

	// 3.7. Initialize Provider Factory (multi-provider support: Gemini, Claude)
	// The provider factory enables workers to use different AI providers (Gemini, Claude)
	// based on the model specified in job/step configuration.
	a.ProviderFactory = llm.NewProviderFactory(
		&a.Config.Gemini,
		&a.Config.Claude,
		&a.Config.LLM,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)
	a.Logger.Debug().
		Str("default_provider", string(a.Config.LLM.DefaultProvider)).
		Msg("Provider factory initialized for multi-provider AI support")

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
		TimeFormat: "15:04:05.000",
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

	visibilityTimeout, err := time.ParseDuration(a.Config.Queue.VisibilityTimeout)
	if err != nil {
		visibilityTimeout = 5 * time.Minute // Default to 5 minutes
		a.Logger.Warn().Str("value", a.Config.Queue.VisibilityTimeout).Msg("Invalid visibility timeout, using default 5m")
	}

	queueMgr, err := queue.NewBadgerManager(
		badgerDB,
		a.Config.Queue.QueueName,
		visibilityTimeout,
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
		a.StorageManager.LogStorage(),
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

	// 5.8.2 Initialize Cache Service and wire to StepManager
	cacheService := cache.NewService(a.StorageManager.DocumentStorage(), a.Logger)
	stepMgr.SetCacheService(cacheService)
	stepMgr.SetJobManager(jobMgr)
	a.Logger.Debug().Msg("Cache service initialized and wired to step manager")

	// 5.9. Initialize job processor (replaces worker pool)
	jobProcessor := workers.NewJobProcessor(queueMgr, jobMgr, a.Logger, a.Config.Queue.Concurrency)
	jobProcessor.SetEventService(a.EventService) // Enable event-based job cancellation
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

	// Seed default KV values (only creates if not already set)
	// These are service-specific defaults that workers fall back to
	a.seedDefaultKVValues(context.Background())

	// 5.12. Initialize config service with event-driven cache invalidation
	a.ConfigService, err = config.NewService(
		a.Config,
		a.StorageManager.KeyValueStorage(),
		a.EventService,
		a.Logger,
		a.ConfigPaths..., // Pass config paths for reload functionality
	)
	if err != nil {
		return fmt.Errorf("failed to initialize config service: %w", err)
	}
	a.Logger.Debug().Strs("paths", a.ConfigPaths).Msg("Config service initialized")

	// 5.13. Initialize connector service
	a.ConnectorService = connectors.NewService(
		a.StorageManager.ConnectorStorage(),
		a.Logger,
	)
	a.Logger.Debug().Msg("Connector service initialized")

	// 5.14. Initialize mailer service
	a.MailerService = mailer.NewService(
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)
	a.Logger.Debug().Msg("Mailer service initialized")

	// 5.15. Initialize IMAP service for email reading
	a.IMAPService = imap.NewService(
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)
	a.Logger.Debug().Msg("IMAP service initialized")

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

	// Register Local Directory worker (local filesystem indexing)
	localDirWorker := workers.NewLocalDirWorker(
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.Logger,
	)
	a.StepManager.RegisterWorker(localDirWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(localDirWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", localDirWorker.GetType().String()).Str("job_type", localDirWorker.GetWorkerType()).Msg("Local Directory worker registered")

	// Register Code Map worker (hierarchical code structure analysis - optimized for large codebases)
	codeMapWorker := workers.NewCodeMapWorker(
		jobMgr,
		queueMgr,
		a.StorageManager.DocumentStorage(),
		a.AgentService, // May be nil if AI not configured
		a.EventService,
		a.Logger,
	)
	a.StepManager.RegisterWorker(codeMapWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(codeMapWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", codeMapWorker.GetType().String()).Str("job_type", codeMapWorker.GetWorkerType()).Msg("Code Map worker registered")

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

	// Register Tool Execution worker (processes tool_execution jobs from orchestrator)
	// This worker executes individual tool calls as queue citizens with independent status tracking
	toolExecutionWorker := workers.NewToolExecutionWorker(
		a.StepManager,
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		jobMgr,
		a.Logger,
	)
	jobProcessor.RegisterExecutor(toolExecutionWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("job_type", toolExecutionWorker.GetWorkerType()).Msg("Tool execution worker registered")

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

	// Register ASX Announcements worker (synchronous execution, fetches ASX company announcements)
	asxAnnouncementsWorker := workers.NewASXAnnouncementsWorker(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(asxAnnouncementsWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", asxAnnouncementsWorker.GetType().String()).Msg("ASX Announcements worker registered")

	// Register ASX Index Data worker (synchronous execution, fetches index benchmarks like XJO, XSO)
	asxIndexDataWorker := workers.NewASXIndexDataWorker(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(asxIndexDataWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", asxIndexDataWorker.GetType().String()).Msg("ASX Index Data worker registered")

	// Register ASX Director Interest worker (synchronous execution, fetches Appendix 3Y filings)
	asxDirectorInterestWorker := workers.NewASXDirectorInterestWorker(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(asxDirectorInterestWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", asxDirectorInterestWorker.GetType().String()).Msg("ASX Director Interest worker registered")

	// Register ASX Stock Collector worker (consolidated: price, analyst coverage, historical financials)
	asxStockCollectorWorker := workers.NewASXStockCollectorWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(asxStockCollectorWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", asxStockCollectorWorker.GetType().String()).Msg("ASX Stock Collector worker registered")

	// Register asx_stock_data as an alias for asx_stock_collector (backward compatibility)
	a.StepManager.RegisterWorkerAlias(asxStockCollectorWorker, models.WorkerTypeASXStockData)
	a.Logger.Debug().Str("step_type", models.WorkerTypeASXStockData.String()).Msg("ASX Stock Data worker alias registered (deprecated)")

	// Register Macro Data worker (synchronous execution, fetches RBA rates and commodity prices)
	macroDataWorker := workers.NewMacroDataWorker(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(macroDataWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", macroDataWorker.GetType().String()).Msg("Macro Data worker registered")

	// Register Competitor Analysis worker (identifies competitors via LLM and fetches their stock data)
	competitorAnalysisWorker := workers.NewCompetitorAnalysisWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		jobMgr,
		a.Logger,
	)
	a.StepManager.RegisterWorker(competitorAnalysisWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", competitorAnalysisWorker.GetType().String()).Msg("Competitor Analysis worker registered")

	// Register Navexa Portfolios worker (fetches all user portfolios from Navexa API)
	navexaPortfoliosWorker := workers.NewNavexaPortfoliosWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(navexaPortfoliosWorker)
	a.Logger.Debug().Str("step_type", navexaPortfoliosWorker.GetType().String()).Msg("Navexa Portfolios worker registered")

	// Register Navexa Holdings worker (fetches holdings for a specific portfolio)
	navexaHoldingsWorker := workers.NewNavexaHoldingsWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(navexaHoldingsWorker)
	a.Logger.Debug().Str("step_type", navexaHoldingsWorker.GetType().String()).Msg("Navexa Holdings worker registered")

	// Register Navexa Performance worker (fetches P/L performance for a portfolio)
	navexaPerformanceWorker := workers.NewNavexaPerformanceWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(navexaPerformanceWorker)
	a.Logger.Debug().Str("step_type", navexaPerformanceWorker.GetType().String()).Msg("Navexa Performance worker registered")

	// Register Summary worker (synchronous execution, aggregates tagged documents)
	// Supports multi-provider AI (Gemini, Claude) via provider factory
	summaryWorker := workers.NewSummaryWorker(
		a.SearchService,
		a.StorageManager.DocumentStorage(),
		a.EventService,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
		a.ProviderFactory,
	)
	a.StepManager.RegisterWorker(summaryWorker) // Register with StepManager for step routing
	a.Logger.Debug().Str("step_type", summaryWorker.GetType().String()).Msg("Summary worker registered")

	// Register Orchestrator worker (AI-powered cognitive orchestration with LLM reasoning)
	orchestratorWorker := workers.NewOrchestratorWorker(
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.StorageManager.KeyValueStorage(),
		a.EventService,
		a.Logger,
		jobMgr,
		a.ProviderFactory,
		"./job-templates", // Templates directory for goal_template support
	)
	a.StepManager.RegisterWorker(orchestratorWorker)
	orchestratorWorker.SetStepManager(a.StepManager) // Set after registration to avoid circular dependency
	a.Logger.Debug().Str("step_type", orchestratorWorker.GetType().String()).Msg("Orchestrator worker registered")

	// Register enrichment pipeline workers (each handles a specific enrichment step)
	analyzeBuildWorker := workers.NewAnalyzeBuildWorker(
		a.SearchService,
		a.StorageManager.DocumentStorage(),
		a.LLMService,
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(analyzeBuildWorker)
	a.Logger.Debug().Str("step_type", analyzeBuildWorker.GetType().String()).Msg("Analyze build worker registered")

	classifyWorker := workers.NewClassifyWorker(
		a.SearchService,
		a.StorageManager.DocumentStorage(),
		a.LLMService,
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(classifyWorker)
	a.Logger.Debug().Str("step_type", classifyWorker.GetType().String()).Msg("Classify worker registered")

	dependencyGraphWorker := workers.NewDependencyGraphWorker(
		a.SearchService,
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(dependencyGraphWorker)
	a.Logger.Debug().Str("step_type", dependencyGraphWorker.GetType().String()).Msg("Dependency graph worker registered")

	aggregateSummaryWorker := workers.NewAggregateSummaryWorker(
		a.SearchService,
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.LLMService,
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(aggregateSummaryWorker)
	a.Logger.Debug().Str("step_type", aggregateSummaryWorker.GetType().String()).Msg("Aggregate summary worker registered")

	// Register Email worker (notification step for job definitions)
	emailWorker := workers.NewEmailWorker(
		a.MailerService,
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(emailWorker)
	a.Logger.Debug().Str("step_type", emailWorker.GetType().String()).Msg("Email worker registered")

	// Register Email Watcher worker (monitors IMAP inbox for job execution commands)
	emailWatcherWorker := workers.NewEmailWatcherWorker(
		a.IMAPService,
		a.StorageManager.JobDefinitionStorage(),
		a.JobDispatcher,
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(emailWatcherWorker)
	a.Logger.Debug().Str("step_type", emailWatcherWorker.GetType().String()).Msg("Email watcher worker registered")

	// Register Test Job Generator worker (testing worker for logging, error tolerance, and job hierarchy validation)
	testJobGeneratorWorker := workers.NewTestJobGeneratorWorker(
		jobMgr,
		queueMgr,
		a.Logger,
		a.EventService,
	)
	a.StepManager.RegisterWorker(testJobGeneratorWorker)  // Register with StepManager for step routing
	jobProcessor.RegisterExecutor(testJobGeneratorWorker) // Register with JobProcessor for job execution
	a.Logger.Debug().Str("step_type", testJobGeneratorWorker.GetType().String()).Str("job_type", testJobGeneratorWorker.GetWorkerType()).Msg("Test Job Generator worker registered")

	// Register Signal Computer worker (computes PBAS, VLI, Regime signals from stock data)
	signalComputerWorker := workers.NewSignalComputerWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(signalComputerWorker)
	a.Logger.Debug().Str("step_type", signalComputerWorker.GetType().String()).Msg("Signal Computer worker registered")

	// Register Portfolio Rollup worker (aggregates ticker signals into portfolio-level metrics)
	portfolioRollupWorker := workers.NewPortfolioRollupWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
	)
	a.StepManager.RegisterWorker(portfolioRollupWorker)
	a.Logger.Debug().Str("step_type", portfolioRollupWorker.GetType().String()).Msg("Portfolio Rollup worker registered")

	// Register AI Assessor worker (AI-powered stock assessment with validation)
	aiAssessorWorker := workers.NewAIAssessorWorker(
		a.StorageManager.DocumentStorage(),
		a.StorageManager.KeyValueStorage(),
		a.Logger,
		jobMgr,
		a.LLMService,
	)
	a.StepManager.RegisterWorker(aiAssessorWorker)
	a.Logger.Debug().Str("step_type", aiAssessorWorker.GetType().String()).Msg("AI Assessor worker registered")

	a.Logger.Debug().Msg("All workers registered with StepManager")

	// Initialize JobDispatcher (mechanical job execution coordinator)
	a.JobDispatcher = queue.NewJobDispatcher(
		jobMgr,
		a.StepManager,
		a.EventService,
		a.StorageManager.KeyValueStorage(),
		a.Logger,
	)

	// Register Job Template worker (must be after JobDispatcher is created since it needs it)
	jobTemplateWorker := workers.NewJobTemplateWorker(
		a.StorageManager.JobDefinitionStorage(),
		jobs.NewService(a.StorageManager.KeyValueStorage(), a.AgentService, a.Logger),
		a.JobDispatcher,
		jobMgr,
		a.EventService,
		a.Logger,
		"./job-templates", // Templates directory relative to executable
	)
	a.StepManager.RegisterWorker(jobTemplateWorker)
	a.Logger.Debug().Str("step_type", jobTemplateWorker.GetType().String()).Msg("Job template worker registered")

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

	// Initialize job handler with JobManager, QueueManager and EventService
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.QueueStorage(), a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.JobManager, a.QueueManager, a.EventService, a.Config, a.Logger)

	// Initialize status handler
	a.StatusHandler = handlers.NewStatusHandler(a.StatusService, a.Logger)

	// Initialize system logs handler
	a.SystemLogsHandler = handlers.NewSystemLogsHandler(a.SystemLogsService, a.Logger)

	// Initialize unified logs handler (single endpoint for service and job logs)
	a.UnifiedLogsHandler = handlers.NewUnifiedLogsHandler(a.LogService, a.Logger)
	a.Logger.Debug().Msg("Unified logs handler initialized")

	// Initialize SSE logs handler (real-time log streaming via Server-Sent Events)
	a.SSELogsHandler = handlers.NewSSELogsHandler(a.LogService, a.EventService, a.Logger)
	a.Logger.Debug().Msg("SSE logs handler initialized")

	// Initialize config handler with ConfigService for dynamic key injection
	a.ConfigHandler = handlers.NewConfigHandler(a.Logger, a.Config, a.ConfigService, a.StorageManager)

	// Initialize connector handler
	a.ConnectorHandler = handlers.NewConnectorHandler(a.ConnectorService, a.Logger)

	// Initialize mailer handler
	a.MailerHandler = handlers.NewMailerHandler(a.MailerService, a.Logger)
	a.Logger.Debug().Msg("Mailer handler initialized")

	// Initialize GitHub jobs handler
	a.GitHubJobsHandler = handlers.NewGitHubJobsHandler(
		a.ConnectorService,
		a.JobManager,
		a.JobDispatcher,
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
		a.JobDispatcher,
		a.JobMonitor,
		a.StepMonitor,
		a.StorageManager.AuthStorage(),
		a.StorageManager.KeyValueStorage(), // For {key-name} replacement in job definitions
		a.StorageManager,                   // For reloading job definitions from disk
		a.Config.Jobs.DefinitionsDir,       // Path to job definitions directory
		a.Config.Jobs.TemplatesDir,         // Path to job templates directory
		a.AgentService,                     // Pass agent service for runtime validation (can be nil)
		a.DocumentService,                  // For direct document capture from extension
		a.Logger,
	)

	// Initialize hybrid scraper handler (lazy initialization - browser launched on-demand)
	a.HybridScraperHandler = handlers.NewHybridScraperHandler(a.Logger)
	a.Logger.Debug().Msg("Hybrid scraper handler initialized")

	// Initialize DevOps handler for enrichment pipeline endpoints
	a.DevOpsHandler = handlers.NewDevOpsHandler(
		a.StorageManager.KeyValueStorage(),
		a.StorageManager.DocumentStorage(),
		a.SearchService,
		a.StorageManager.JobDefinitionStorage(),
		a.JobDispatcher,
		a.JobMonitor,
		a.StepMonitor,
		a.Logger,
	)
	a.Logger.Debug().Msg("DevOps handler initialized")

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

	return nil
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

// seedDefaultKVValues seeds default key/value pairs that workers depend on.
// These are only created if they don't already exist, preserving user-configured values.
// This ensures workers have sensible defaults without requiring manual configuration.
func (a *App) seedDefaultKVValues(ctx context.Context) {
	defaults := common.GetDefaultKVValues()

	// Seed each default
	seededCount := 0
	for _, d := range defaults {
		created, err := a.KVService.SetIfNotExists(ctx, d.Key, d.Value, d.Description)
		if err != nil {
			a.Logger.Warn().Err(err).Str("key", d.Key).Msg("Failed to seed default KV value")
			continue
		}
		if created {
			seededCount++
		}
	}

	if seededCount > 0 {
		a.Logger.Info().Int("count", seededCount).Msg("Seeded default KV values")
	}
}
