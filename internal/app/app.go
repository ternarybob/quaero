package app

import (
	"fmt"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	"github.com/ternarybob/quaero/internal/services/documents"
	"github.com/ternarybob/quaero/internal/services/embeddings"
	"github.com/ternarybob/quaero/internal/services/processing"
	"github.com/ternarybob/quaero/internal/storage"
)

// App holds all application components and dependencies
type App struct {
	Config         *common.Config
	Logger         arbor.ILogger
	StorageManager interfaces.StorageManager

	// Document services
	EmbeddingService    interfaces.EmbeddingService
	DocumentService     interfaces.DocumentService
	ProcessingService   *processing.Service
	ProcessingScheduler *processing.Scheduler

	// Atlassian services
	AuthService       *atlassian.AtlassianAuthService
	JiraService       *atlassian.JiraScraperService
	ConfluenceService *atlassian.ConfluenceScraperService

	// HTTP handlers
	APIHandler       *handlers.APIHandler
	UIHandler        *handlers.UIHandler
	WSHandler        *handlers.WebSocketHandler
	ScraperHandler   *handlers.ScraperHandler
	DataHandler      *handlers.DataHandler
	CollectorHandler *handlers.CollectorHandler
	DocumentHandler  *handlers.DocumentHandler
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

	// 1. Initialize embedding service
	a.EmbeddingService = embeddings.NewService(
		a.Config.Embeddings.OllamaURL,
		a.Config.Embeddings.Model,
		a.Config.Embeddings.Dimension,
		a.Logger,
	)

	// 2. Initialize document service
	a.DocumentService = documents.NewService(
		a.StorageManager.DocumentStorage(),
		a.EmbeddingService,
		a.Logger,
	)

	// 3. Initialize auth service
	a.AuthService, err = atlassian.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// 4. Initialize Jira service with DocumentService
	a.JiraService = atlassian.NewJiraScraperService(
		a.StorageManager.JiraStorage(),
		a.DocumentService,
		a.AuthService,
		a.Logger,
	)

	// 5. Initialize Confluence service with DocumentService
	a.ConfluenceService = atlassian.NewConfluenceScraperService(
		a.StorageManager.ConfluenceStorage(),
		a.DocumentService,
		a.AuthService,
		a.Logger,
	)

	// 6. Initialize processing service
	a.ProcessingService = processing.NewService(
		a.DocumentService,
		a.StorageManager.JiraStorage(),
		a.StorageManager.ConfluenceStorage(),
		a.Logger,
	)

	// 7. Initialize and start processing scheduler (if enabled)
	if a.Config.Processing.Enabled {
		a.ProcessingScheduler = processing.NewScheduler(a.ProcessingService, a.Logger)
		if err := a.ProcessingScheduler.Start(a.Config.Processing.Schedule); err != nil {
			a.Logger.Warn().Err(err).Msg("Failed to start processing scheduler")
		}
	}

	return nil
}

// initHandlers initializes all HTTP handlers
func (a *App) initHandlers() error {
	// Initialize handlers
	a.APIHandler = handlers.NewAPIHandler()
	a.UIHandler = handlers.NewUIHandler(a.JiraService, a.ConfluenceService)
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
	a.DocumentHandler = handlers.NewDocumentHandler(
		a.DocumentService,
		a.ProcessingService,
	)

	// Set UI logger for services
	a.JiraService.SetUILogger(a.WSHandler)
	a.ConfluenceService.SetUILogger(a.WSHandler)

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	return nil
}

// Close closes all application resources
func (a *App) Close() error {
	// Stop processing scheduler
	if a.ProcessingScheduler != nil {
		a.ProcessingScheduler.Stop()
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
