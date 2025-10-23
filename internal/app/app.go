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

// initServices initializes all business services
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

	// 3.5 Initialize search service (FTS5-based search)
	a.SearchService = search.NewFTS5SearchService(
		a.StorageManager.DocumentStorage(),
		a.Logger,
	)

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

	// 5.10. Initialize worker pool
	workerPool := queue.NewWorkerPool(queueMgr, a.Logger)
	a.WorkerPool = workerPool
	a.Logger.Info().Msg("Worker pool initialized")

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

	// 11.5 Initialize summary service (subscribes to embedding events)
	a.SummaryService = summary.NewService(
		a.StorageManager.DocumentStorage(),
		a.DocumentService,
		a.EventService,
		a.Logger,
	)
	// Generate initial summary document at startup
	a.Logger.Info().Msg("Generating initial corpus summary document at startup")
	if err := a.SummaryService.GenerateSummaryDocument(context.Background()); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to generate initial summary document (non-critical)")
	}

	// Initialize job executor for job definition execution
	a.JobRegistry = jobs.NewJobTypeRegistry(a.Logger)
	a.JobExecutor, err = jobs.NewJobExecutor(a.JobRegistry, a.SourceService, a.EventService, a.CrawlerService, a.Logger)
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
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.LogService, a.Config, a.Logger)

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
		a.JobExecutor,
		a.SourceService,
		a.JobRegistry,
		a.Logger,
	)
	a.Logger.Info().Msg("Job definition handler initialized")

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	return nil
}

// Close closes all application resources
func (a *App) Close() error {
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
