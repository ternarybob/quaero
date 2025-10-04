package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"

	"github.com/ternarybob/quaero/internal/handlers"
	"github.com/ternarybob/quaero/internal/services/atlassian"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server to receive auth from extension",
	Long:  `Starts the Quaero server which receives authentication from the browser extension and runs background collection.`,
	Run:   runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	logger.Info().
		Int("port", config.Server.Port).
		Str("host", config.Server.Host).
		Msg("Starting Quaero server")

	// Initialize database
	execPath, err := os.Executable()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to get executable path")
	}
	execDir := filepath.Dir(execPath)
	dbPath := filepath.Join(execDir, "data", "quaero.db")

	// Create data directory if it doesn't exist
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create data directory")
	}

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	logger.Info().Str("path", dbPath).Msg("Database opened")

	// Initialize centralized AuthService
	authService, err := atlassian.NewAtlassianAuthService(db, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize AuthService")
	}

	// Initialize Jira service
	jiraService, err := atlassian.NewJiraScraperService(db, authService, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Jira service")
	}

	// Initialize Confluence service
	confluenceService, err := atlassian.NewConfluenceScraperService(db, authService, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Confluence service")
	}

	// Initialize handlers
	apiHandler := handlers.NewAPIHandler()
	uiHandler := handlers.NewUIHandler(jiraService, confluenceService)
	wsHandler := handlers.NewWebSocketHandler()
	scraperHandler := handlers.NewScraperHandler(authService, jiraService, confluenceService, wsHandler)
	dataHandler := handlers.NewDataHandler(jiraService, confluenceService)
	collectorHandler := handlers.NewCollectorHandler(jiraService, confluenceService, logger)

	// Set UI logger for services
	jiraService.SetUILogger(wsHandler)
	confluenceService.SetUILogger(wsHandler)

	// Set auth loader for WebSocket handler
	wsHandler.SetAuthLoader(authService)

	// Load stored authentication if available
	if _, err := authService.LoadAuth(); err == nil {
		logger.Info().Msg("Loaded stored authentication from database")
	} else {
		logger.Debug().Err(err).Msg("No stored authentication found")
	}

	// Start WebSocket status broadcaster and log streamer
	wsHandler.StartStatusBroadcaster()
	wsHandler.StartLogStreamer()

	// Register routes
	// UI routes
	http.HandleFunc("/", uiHandler.IndexHandler)
	http.HandleFunc("/jira", uiHandler.JiraPageHandler)
	http.HandleFunc("/confluence", uiHandler.ConfluencePageHandler)
	http.HandleFunc("/static/common.css", uiHandler.StaticFileHandler)
	http.HandleFunc("/favicon.ico", uiHandler.StaticFileHandler)
	http.HandleFunc("/ui/status", uiHandler.StatusHandler)
	http.HandleFunc("/ui/parser-status", uiHandler.ParserStatusHandler)

	// WebSocket route
	http.HandleFunc("/ws", wsHandler.HandleWebSocket)

	// API routes
	http.HandleFunc("/api/auth", scraperHandler.AuthUpdateHandler)
	http.HandleFunc("/api/scrape", scraperHandler.ScrapeHandler)
	http.HandleFunc("/api/scrape/projects", scraperHandler.ScrapeProjectsHandler)
	http.HandleFunc("/api/scrape/spaces", scraperHandler.ScrapeSpacesHandler)
	http.HandleFunc("/api/projects/refresh-cache", scraperHandler.RefreshProjectsCacheHandler)
	http.HandleFunc("/api/projects/get-issues", scraperHandler.GetProjectIssuesHandler)
	http.HandleFunc("/api/spaces/refresh-cache", scraperHandler.RefreshSpacesCacheHandler)
	http.HandleFunc("/api/spaces/get-pages", scraperHandler.GetSpacePagesHandler)
	http.HandleFunc("/api/data/clear-all", scraperHandler.ClearAllDataHandler)
	http.HandleFunc("/api/data/jira", dataHandler.GetJiraDataHandler)
	http.HandleFunc("/api/data/jira/issues", dataHandler.GetJiraIssuesHandler)
	http.HandleFunc("/api/data/confluence", dataHandler.GetConfluenceDataHandler)
	http.HandleFunc("/api/data/confluence/pages", dataHandler.GetConfluencePagesHandler)
	http.HandleFunc("/api/collector/projects", collectorHandler.GetProjectsHandler)
	http.HandleFunc("/api/collector/spaces", collectorHandler.GetSpacesHandler)
	http.HandleFunc("/api/collector/issues", collectorHandler.GetIssuesHandler)
	http.HandleFunc("/api/collector/pages", collectorHandler.GetPagesHandler)
	http.HandleFunc("/api/version", apiHandler.VersionHandler)
	http.HandleFunc("/api/health", apiHandler.HealthHandler)

	// 404 handler for unmatched API routes
	http.HandleFunc("/api/", apiHandler.NotFoundHandler)

	// Start server in goroutine
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	go func() {
		logger.Info().Str("address", addr).Msg("HTTP server starting")
		logger.Info().Msg("Install Chrome extension and click icon when logged into Jira/Confluence")
		logger.Info().Str("url", fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)).Msg("Web UI available")

		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	logger.Info().Msg("Server ready")
	fmt.Printf("\nServer running on http://%s:%d\n", config.Server.Host, config.Server.Port)
	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("Shutting down server...")
	fmt.Println("\nServer stopped")
}
