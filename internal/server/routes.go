package server

import "net/http"

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// UI routes
	mux.HandleFunc("/", s.app.UIHandler.IndexHandler)
	mux.HandleFunc("/jira", s.app.UIHandler.JiraPageHandler)
	mux.HandleFunc("/confluence", s.app.UIHandler.ConfluencePageHandler)
	mux.HandleFunc("/documents", s.app.UIHandler.DocumentsPageHandler)
	mux.HandleFunc("/embeddings", s.app.UIHandler.EmbeddingsPageHandler)
	mux.HandleFunc("/settings", s.app.UIHandler.SettingsPageHandler)
	mux.HandleFunc("/static/common.css", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/favicon.ico", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/ui/status", s.app.UIHandler.StatusHandler)
	mux.HandleFunc("/ui/parser-status", s.app.UIHandler.ParserStatusHandler)

	// WebSocket route
	mux.HandleFunc("/ws", s.app.WSHandler.HandleWebSocket)

	// API routes - Authentication
	mux.HandleFunc("/api/auth", s.app.ScraperHandler.AuthUpdateHandler)

	// API routes - Scraping (UI-triggered collection)
	mux.HandleFunc("/api/scrape", s.app.ScraperHandler.ScrapeHandler)
	mux.HandleFunc("/api/scrape/projects", s.app.ScraperHandler.ScrapeProjectsHandler)
	mux.HandleFunc("/api/scrape/spaces", s.app.ScraperHandler.ScrapeSpacesHandler)

	// API routes - Cache management
	mux.HandleFunc("/api/projects/refresh-cache", s.app.ScraperHandler.RefreshProjectsCacheHandler)
	mux.HandleFunc("/api/projects/get-issues", s.app.ScraperHandler.GetProjectIssuesHandler)
	mux.HandleFunc("/api/spaces/refresh-cache", s.app.ScraperHandler.RefreshSpacesCacheHandler)
	mux.HandleFunc("/api/spaces/get-pages", s.app.ScraperHandler.GetSpacePagesHandler)

	// API routes - Data management
	mux.HandleFunc("/api/data/clear-all", s.app.ScraperHandler.ClearAllDataHandler)
	mux.HandleFunc("/api/data/jira/clear", s.app.ScraperHandler.ClearJiraDataHandler)
	mux.HandleFunc("/api/data/confluence/clear", s.app.ScraperHandler.ClearConfluenceDataHandler)
	mux.HandleFunc("/api/data/jira", s.app.DataHandler.GetJiraDataHandler)
	mux.HandleFunc("/api/data/jira/issues", s.app.DataHandler.GetJiraIssuesHandler)
	mux.HandleFunc("/api/data/confluence", s.app.DataHandler.GetConfluenceDataHandler)
	mux.HandleFunc("/api/data/confluence/pages", s.app.DataHandler.GetConfluencePagesHandler)

	// API routes - Collector (paginated data)
	mux.HandleFunc("/api/collector/projects", s.app.CollectorHandler.GetProjectsHandler)
	mux.HandleFunc("/api/collector/spaces", s.app.CollectorHandler.GetSpacesHandler)
	mux.HandleFunc("/api/collector/issues", s.app.CollectorHandler.GetIssuesHandler)
	mux.HandleFunc("/api/collector/pages", s.app.CollectorHandler.GetPagesHandler)

	// API routes - Collection (manual data sync)
	mux.HandleFunc("/api/collection/jira/sync", s.app.CollectionHandler.SyncJiraHandler)
	mux.HandleFunc("/api/collection/confluence/sync", s.app.CollectionHandler.SyncConfluenceHandler)
	mux.HandleFunc("/api/collection/sync-all", s.app.CollectionHandler.SyncAllHandler)

	// API routes - Documents
	mux.HandleFunc("/api/documents/stats", s.app.DocumentHandler.StatsHandler)
	mux.HandleFunc("/api/documents", s.app.DocumentHandler.ListHandler)
	mux.HandleFunc("/api/documents/process", s.app.DocumentHandler.ProcessHandler)
	mux.HandleFunc("/api/documents/force-sync", s.app.SchedulerHandler.ForceSyncDocumentHandler)
	mux.HandleFunc("/api/documents/force-embed", s.app.SchedulerHandler.ForceEmbedDocumentHandler)

	// API routes - Processing
	mux.HandleFunc("/api/processing/status", s.app.DocumentHandler.ProcessingStatusHandler)

	// API routes - Scheduler
	mux.HandleFunc("/api/scheduler/trigger-collection", s.app.SchedulerHandler.TriggerCollectionHandler)
	mux.HandleFunc("/api/scheduler/trigger-embedding", s.app.SchedulerHandler.TriggerEmbeddingHandler)

	// API routes - System
	mux.HandleFunc("/api/version", s.app.APIHandler.VersionHandler)
	mux.HandleFunc("/api/health", s.app.APIHandler.HealthHandler)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.app.APIHandler.NotFoundHandler)

	return mux
}
