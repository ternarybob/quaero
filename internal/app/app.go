// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:30 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/auth"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/config"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/events"
	"github.com/ternarybob/quaero/internal/services/identifiers"
	"github.com/ternarybob/quaero/internal/services/jobs"
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
	Config         *common.Config // Deprecated: Use ConfigService instead
	ConfigService  interfaces.ConfigService
	Logger         arbor.ILogger
	StorageManager interfaces.StorageManager

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

	// Source-agnostic services
	StatusService *status.Service
	SourceService *sources.Service

	// Authentication service (supports multiple providers)
	AuthService *auth.Service

	// Crawler service
	CrawlerService *crawler.Service

	// HTTP handlers
	APIHandler        *handlers.APIHandler
	AuthHandler       *handlers.AuthHandler
	WSHandler         *handlers.WebSocketHandler
	CollectionHandler *handlers.CollectionHandler
	DocumentHandler   *handlers.DocumentHandler
	SchedulerHandler  *handlers.SchedulerHandler
	ChatHandler       *handlers.ChatHandler
	MCPHandler        *handlers.MCPHandler
	JobHandler        *handlers.JobHandler
	SourcesHandler    *handlers.SourcesHandler
	StatusHandler     *handlers.StatusHandler
	ConfigHandler     *handlers.ConfigHandler
	PageHandler       *handlers.PageHandler
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

	// 6. Initialize auth service (Atlassian)
	a.AuthService, err = auth.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 6.5. Initialize crawler service
	crawlerConfig := crawler.CrawlConfig{
		MaxDepth:      3,
		FollowLinks:   true,
		Concurrency:   2,
		RateLimit:     time.Second,
		DetailLevel:   "full",
		RetryAttempts: 3,
		RetryBackoff:  time.Second * 2,
	}
	a.CrawlerService = crawler.NewService(a.AuthService, a.SourceService, a.StorageManager.AuthStorage(), a.EventService, a.StorageManager.JobStorage(), a.Logger, crawlerConfig)
	if err := a.CrawlerService.Start(); err != nil {
		return fmt.Errorf("failed to start crawler service: %w", err)
	}
	a.Logger.Info().Msg("Crawler service initialized")

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

	// 12. Initialize scheduler service
	a.SchedulerService = scheduler.NewService(a.EventService, a.Logger)

	// Register default jobs
	jobsRegistered := 0

	// Register crawl and collect job if enabled
	if a.Config.Jobs.CrawlAndCollect.Enabled {
		crawlCollectJob := jobs.NewCrawlCollectJob(
			a.CrawlerService,
			a.SourceService,
			a.StorageManager.AuthStorage(),
			a.Logger,
		)
		if err := a.SchedulerService.RegisterJob(
			"crawl_and_collect",
			a.Config.Jobs.CrawlAndCollect.Schedule,
			crawlCollectJob.Execute,
		); err != nil {
			a.Logger.Error().Err(err).Msg("Failed to register crawl_and_collect job")
		} else {
			jobsRegistered++
			a.Logger.Info().
				Str("schedule", a.Config.Jobs.CrawlAndCollect.Schedule).
				Msg("Registered crawl_and_collect job")
		}
	}

	// Register scan and summarize job if enabled
	if a.Config.Jobs.ScanAndSummarize.Enabled {
		scanSummarizeJob := jobs.NewScanSummarizeJob(
			a.StorageManager.DocumentStorage(),
			a.LLMService,
			a.Logger,
		)
		if err := a.SchedulerService.RegisterJob(
			"scan_and_summarize",
			a.Config.Jobs.ScanAndSummarize.Schedule,
			scanSummarizeJob.Execute,
		); err != nil {
			a.Logger.Error().Err(err).Msg("Failed to register scan_and_summarize job")
		} else {
			jobsRegistered++
			a.Logger.Info().
				Str("schedule", a.Config.Jobs.ScanAndSummarize.Schedule).
				Msg("Registered scan_and_summarize job")
		}
	}

	// NOTE: Scheduler triggers event-driven processing:
	// - EventCollectionTriggered: Transforms scraped data (issues/pages → documents)
	// - EventEmbeddingTriggered: Generates embeddings for unembedded documents
	// Scraping (downloading from Jira/Confluence APIs) remains user-driven via UI
	// PLUS: Default jobs run on schedule (crawl_and_collect, scan_and_summarize)
	if err := a.SchedulerService.Start("*/5 * * * *"); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to start scheduler service")
	} else {
		a.Logger.Info().
			Int("registered_jobs", jobsRegistered).
			Msg("Scheduler service started with default jobs")
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
	a.JobHandler = handlers.NewJobHandler(a.CrawlerService, a.StorageManager.JobStorage(), a.SourceService, a.StorageManager.AuthStorage(), a.SchedulerService, a.Logger)

	// Initialize sources handler
	a.SourcesHandler = handlers.NewSourcesHandler(a.SourceService, a.Logger)

	// Initialize status handler
	a.StatusHandler = handlers.NewStatusHandler(a.StatusService, a.Logger)

	// Initialize config handler
	a.ConfigHandler = handlers.NewConfigHandler(a.Logger, a.Config)

	// Initialize page handler for serving HTML templates
	a.PageHandler = handlers.NewPageHandler(a.Logger)

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	return nil
}

// Close closes all application resources
func (a *App) Close() error {
	// Stop scheduler service
	if a.SchedulerService != nil {
		if err := a.SchedulerService.Stop(); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to stop scheduler service")
		}
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
