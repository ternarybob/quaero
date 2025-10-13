// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:57:30 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package app

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	"github.com/ternarybob/quaero/internal/services/chat"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/events"
	"github.com/ternarybob/quaero/internal/services/identifiers"
	"github.com/ternarybob/quaero/internal/services/llm"
	"github.com/ternarybob/quaero/internal/services/mcp"
	"github.com/ternarybob/quaero/internal/services/processing"
	"github.com/ternarybob/quaero/internal/services/scheduler"
	"github.com/ternarybob/quaero/internal/services/search"
	"github.com/ternarybob/quaero/internal/services/summary"
	"github.com/ternarybob/quaero/internal/storage"
)

// App holds all application components and dependencies
type App struct {
	Config         *common.Config
	Logger         arbor.ILogger
	StorageManager interfaces.StorageManager

	// Document services
	LLMService          interfaces.LLMService
	AuditLogger         llm.AuditLogger
	DocumentService     interfaces.DocumentService
	SearchService       interfaces.SearchService
	IdentifierService   *identifiers.Extractor
	ChatService         interfaces.ChatService
	ProcessingService   *processing.Service
	ProcessingScheduler *processing.Scheduler

	// Event-driven services
	EventService     interfaces.EventService
	SchedulerService interfaces.SchedulerService
	SummaryService   *summary.Service

	// Atlassian services
	AuthService       *atlassian.AtlassianAuthService
	JiraService       *atlassian.JiraScraperService
	ConfluenceService *atlassian.ConfluenceScraperService

	// HTTP handlers
	APIHandler        *handlers.APIHandler
	UIHandler         *handlers.UIHandler
	WSHandler         *handlers.WebSocketHandler
	ScraperHandler    *handlers.ScraperHandler
	DataHandler       *handlers.DataHandler
	CollectorHandler  *handlers.CollectorHandler
	CollectionHandler *handlers.CollectionHandler
	DocumentHandler   *handlers.DocumentHandler
	SchedulerHandler  *handlers.SchedulerHandler
	ChatHandler       *handlers.ChatHandler
	MCPHandler        *handlers.MCPHandler
}

// New initializes the application with all dependencies
func New(config *common.Config, logger arbor.ILogger) (*App, error) {
	app := &App{
		Config: config,
		Logger: logger,
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
		logger.Info().Msg("Loaded stored authentication from database")
	}

	// Start WebSocket background tasks
	app.WSHandler.StartStatusBroadcaster()
	app.WSHandler.StartLogStreamer()

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
		Msg("Storage initialized")

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

	// 6. Initialize auth service
	a.AuthService, err = atlassian.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 7. Initialize Jira service with EventService (auto-subscribes to events)
	a.JiraService = atlassian.NewJiraScraperService(
		a.StorageManager.JiraStorage(),
		a.DocumentService,
		a.AuthService,
		a.EventService,
		a.Logger,
	)

	// 8. Initialize Confluence service with EventService (auto-subscribes to events)
	a.ConfluenceService = atlassian.NewConfluenceScraperService(
		a.StorageManager.ConfluenceStorage(),
		a.DocumentService,
		a.AuthService,
		a.EventService,
		a.Logger,
	)

	// 9. Initialize processing service
	a.ProcessingService = processing.NewService(
		a.DocumentService,
		a.StorageManager.JiraStorage(),
		a.StorageManager.ConfluenceStorage(),
		a.Logger,
	)

	// 10. Initialize and start processing scheduler (if enabled)
	if a.Config.Processing.Enabled {
		a.ProcessingScheduler = processing.NewScheduler(a.ProcessingService, a.Logger)
		if err := a.ProcessingScheduler.Start(a.Config.Processing.Schedule); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to start processing scheduler")
		}
	}

	// 11. Initialize embedding coordinator
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
	// NOTE: Scheduler triggers event-driven processing:
	// - EventCollectionTriggered: Transforms scraped data (issues/pages → documents)
	// - EventEmbeddingTriggered: Generates embeddings for unembedded documents
	// Scraping (downloading from Jira/Confluence APIs) remains user-driven via UI
	if err := a.SchedulerService.Start("*/5 * * * *"); err != nil {
		a.Logger.Warn().Err(err).Msg("Failed to start scheduler service")
	} else {
		a.Logger.Info().Msg("Scheduler service started (runs every 5 minutes)")
	}

	return nil
}

// initHandlers initializes all HTTP handlers
func (a *App) initHandlers() error {
	// Initialize handlers
	a.APIHandler = handlers.NewAPIHandler()
	a.UIHandler = handlers.NewUIHandler(a.JiraService, a.ConfluenceService, a.AuthService)
	a.WSHandler = handlers.NewWebSocketHandler()
	a.ScraperHandler = handlers.NewScraperHandler(
		a.AuthService,
		a.JiraService,
		a.ConfluenceService,
		a.WSHandler,
	)
	a.DataHandler = handlers.NewDataHandler(a.JiraService, a.ConfluenceService)
	a.CollectorHandler = handlers.NewCollectorHandler(
		a.JiraService,
		a.ConfluenceService,
		a.Logger,
	)
	a.CollectionHandler = handlers.NewCollectionHandler(
		a.EventService,
		a.Logger,
	)
	a.DocumentHandler = handlers.NewDocumentHandler(
		a.DocumentService,
		a.StorageManager.DocumentStorage(),
		a.ProcessingService,
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

	// Set UI logger for services
	a.JiraService.SetUILogger(a.WSHandler)
	a.ConfluenceService.SetUILogger(a.WSHandler)

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

	// Stop processing scheduler
	if a.ProcessingScheduler != nil {
		a.ProcessingScheduler.Stop()
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
