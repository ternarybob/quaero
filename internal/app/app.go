package app

import (
	"fmt"

	"github.com/ternarybob/arbor"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	"github.com/ternarybob/quaero/internal/storage"
)

// App holds all application components and dependencies
type App struct {
	Config            *common.Config
	Logger            arbor.ILogger
	StorageManager    interfaces.StorageManager
	AuthService       *atlassian.AtlassianAuthService
	JiraService       *atlassian.JiraScraperService
	ConfluenceService *atlassian.ConfluenceScraperService
	APIHandler        *handlers.APIHandler
	UIHandler         *handlers.UIHandler
	WSHandler         *handlers.WebSocketHandler
	ScraperHandler    *handlers.ScraperHandler
	DataHandler       *handlers.DataHandler
	CollectorHandler  *handlers.CollectorHandler
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

	// Initialize auth service with AuthStorage
	a.AuthService, err = atlassian.NewAtlassianAuthService(
		a.StorageManager.AuthStorage(),
		a.Logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// Initialize Jira service with JiraStorage
	a.JiraService = atlassian.NewJiraScraperService(
		a.StorageManager.JiraStorage(),
		a.AuthService,
		a.Logger,
	)

	// Initialize Confluence service with ConfluenceStorage
	a.ConfluenceService = atlassian.NewConfluenceScraperService(
		a.StorageManager.ConfluenceStorage(),
		a.AuthService,
		a.Logger,
	)

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

	// Set UI logger for services
	a.JiraService.SetUILogger(a.WSHandler)
	a.ConfluenceService.SetUILogger(a.WSHandler)

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	return nil
}

// Close closes all application resources
func (a *App) Close() error {
	if a.StorageManager != nil {
		if err := a.StorageManager.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
		a.Logger.Info().Msg("Storage closed")
	}
	return nil
}
