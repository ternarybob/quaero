package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
	bolt "go.etcd.io/bbolt"

	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/services/atlassian"
)

// App holds all application components and dependencies
type App struct {
	Config            *common.Config
	Logger            arbor.ILogger
	DB                *bolt.DB
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

// initDatabase initializes the BoltDB database
func (a *App) initDatabase() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)
	dbPath := filepath.Join(execDir, "data", "quaero.db")

	// Create data directory
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open database
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	a.DB = db
	a.Logger.Info().Str("path", dbPath).Msg("Database opened")
	return nil
}

// initServices initializes all business services
func (a *App) initServices() error {
	var err error

	// Initialize auth service
	a.AuthService, err = atlassian.NewAtlassianAuthService(a.DB, a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize auth service: %w", err)
	}

	// Initialize Jira service
	a.JiraService, err = atlassian.NewJiraScraperService(a.DB, a.AuthService, a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira service: %w", err)
	}

	// Initialize Confluence service
	a.ConfluenceService, err = atlassian.NewConfluenceScraperService(a.DB, a.AuthService, a.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize Confluence service: %w", err)
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

	// Set UI logger for services
	a.JiraService.SetUILogger(a.WSHandler)
	a.ConfluenceService.SetUILogger(a.WSHandler)

	// Set auth loader for WebSocket handler
	a.WSHandler.SetAuthLoader(a.AuthService)

	return nil
}

// Close closes all application resources
func (a *App) Close() error {
	if a.DB != nil {
		if err := a.DB.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		a.Logger.Info().Msg("Database closed")
	}
	return nil
}
