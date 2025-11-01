// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:30 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	jobmgr "github.com/ternarybob/quaero/internal/services/jobs"
	jobtypes "github.com/ternarybob/quaero/internal/jobs/types"
	"github.com/ternarybob/quaero/internal/logs"
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

	// Initialize log service (after WSHandler is created)
	logService := logs.NewService(app.StorageManager.JobLogStorage(), app.WSHandler, app.Logger)
	if err := logService.Start(); err != nil {
		return nil, fmt.Errorf("failed to start log service: %w", err)
	}
	app.LogService = logService

	// Configure Arbor with context channel (default buffering: batch size 5, flush interval 1s)
	logBatchChannel := logService.GetChannel()
	app.Logger.SetContextChannel(logBatchChannel)

	app.Logger.Info().Msg("Log service initialized with Arbor context channel")

	// Load stored authentication if available
	if _, err := app.AuthService.LoadAuth(); err == nil {
		logger.Info().Msg("Loaded stored authentication credentials")
	}

	// Start WebSocket background tasks for real-time UI updates
	app.WSHandler.StartStatusBroadcaster()

	logger.Info().Msg("WebSocket handlers started (status broadcaster)")

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

	// 3.5 Initialize search service using factory (supports fts5, advanced, disabled modes)
	a.SearchService, err = search.NewSearchService(
		a.StorageManager.DocumentStorage(),
		a.Logger,
		a.Config,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize search service: %w", err)
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

	// 5.9. Initialize job manager (LogService will be set later)
	jobMgr := jobmgr.NewManager(a.StorageManager.JobStorage(), queueMgr, nil, a.Logger)
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
		CrawlerService:       a.CrawlerService,
		LogService:           a.LogService,
		DocumentStorage:      a.StorageManager.DocumentStorage(),
		QueueManager:         a.QueueManager,
		JobStorage:           a.StorageManager.JobStorage(),
		EventService:         a.EventService,
		JobDefinitionStorage: a.StorageManager.JobDefinitionStorage(),
		JobManager:           a.JobManager,
	}
	crawlerJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewCrawlerJob(baseJob, crawlerJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("crawler_url", crawlerJobHandler)
	a.Logger.Info().Msg("Crawler job handler registered")

	// Crawler completion probe handler (for delayed completion verification)
	completionProbeHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewCrawlerJob(baseJob, crawlerJobDeps)
		return job.ExecuteCompletionProbe(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("crawler_completion_probe", completionProbeHandler)
	a.Logger.Info().Msg("Crawler completion probe handler registered")

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
		JobManager: a.JobManager,
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

	// Pre-validation job handler
	preValidationJobDeps := &jobtypes.PreValidationJobDeps{
		AuthStorage:   a.StorageManager.AuthStorage(),
		SourceStorage: a.StorageManager.SourceStorage(),
		HTTPClient:    &http.Client{Timeout: 10 * time.Second},
	}
	preValidationJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewPreValidationJob(baseJob, preValidationJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("pre_validation", preValidationJobHandler)
	a.Logger.Info().Msg("Pre-validation job handler registered")

	// Post-summarization job handler
	postSummarizationJobDeps := &jobtypes.PostSummarizationJobDeps{
		LLMService:      a.LLMService,
		DocumentStorage: a.StorageManager.DocumentStorage(),
		JobStorage:      a.StorageManager.JobStorage(),
	}
	postSummarizationJobHandler := func(ctx context.Context, msg *queue.JobMessage) error {
		baseJob := jobtypes.NewBaseJob(msg.ID, msg.JobDefinitionID, a.Logger, a.JobManager, a.QueueManager, a.StorageManager.JobLogStorage())
		job := jobtypes.NewPostSummarizationJob(baseJob, postSummarizationJobDeps)
		return job.Execute(ctx, msg)
	}
	a.WorkerPool.RegisterHandler("post_summarization", postSummarizationJobHandler)
	a.Logger.Info().Msg("Post-summarization job handler registered")


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
	a.WSHandler = handlers.NewWebSocketHandler(a.EventService, a.Logger, &a.Config.WebSocket)

	// Initialize EventSubscriber for job lifecycle events with config-driven filtering
	// Subscribes to EventJobCreated, EventJobStarted, EventJobCompleted, EventJobFailed, EventJobCancelled
	// Transforms events and broadcasts to WebSocket clients via BroadcastJobStatusChange
	_ = handlers.NewEventSubscriber(a.WSHandler, a.EventService, a.Logger, &a.Config.WebSocket)
	a.Logger.Info().
		Int("allowed_events", len(a.Config.WebSocket.AllowedEvents)).
		Int("throttle_intervals", len(a.Config.WebSocket.ThrottleIntervals)).
		Msg("EventSubscriber initialized with config-driven filtering and throttling")

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
