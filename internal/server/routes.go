// -----------------------------------------------------------------------
// Last Modified: Thursday, 9th October 2025 8:53:55 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package server

import "net/http"

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// UI routes
	mux.HandleFunc("/", s.app.UIHandler.IndexHandler)
	mux.HandleFunc("/sources", s.app.UIHandler.SourcesPageHandler)
	mux.HandleFunc("/jira", s.app.UIHandler.JiraPageHandler)             // Deprecated: use /sources
	mux.HandleFunc("/confluence", s.app.UIHandler.ConfluencePageHandler) // Deprecated: use /sources
	mux.HandleFunc("/documents", s.app.UIHandler.DocumentsPageHandler)
	mux.HandleFunc("/chat", s.app.UIHandler.ChatPageHandler)
	mux.HandleFunc("/jobs", s.app.UIHandler.JobsPageHandler)
	mux.HandleFunc("/settings", s.app.UIHandler.SettingsPageHandler)
	mux.HandleFunc("/static/common.css", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/static/theme-sandstone.css", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/static/theme-yeti.css", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/static/common.js", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/static/websocket-manager.js", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/favicon.ico", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/navbar.html", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/footer.html", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/head.html", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/service-status.html", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/service-logs.html", s.app.UIHandler.StaticFileHandler)
	mux.HandleFunc("/partials/snackbar.html", s.app.UIHandler.StaticFileHandler)

	// WebSocket route
	mux.HandleFunc("/ws", s.app.WSHandler.HandleWebSocket)

	// API routes - Authentication
	mux.HandleFunc("/api/auth/status", s.app.ScraperHandler.AuthStatusHandler)
	mux.HandleFunc("/api/auth", s.app.ScraperHandler.AuthUpdateHandler)
	mux.HandleFunc("/api/auth/details", s.app.ScraperHandler.AuthDetailsHandler)

	// API routes - Status
	mux.HandleFunc("/api/status/parser", s.app.ScraperHandler.ParserStatusHandler)

	// API routes - Scraping (UI-triggered collection)
	mux.HandleFunc("/api/scrape", s.app.ScraperHandler.ScrapeHandler)
	mux.HandleFunc("/api/scrape/projects", s.app.ScraperHandler.ScrapeProjectsHandler)
	mux.HandleFunc("/api/scrape/spaces", s.app.ScraperHandler.ScrapeSpacesHandler)

	// API routes - Cache management
	mux.HandleFunc("/api/projects/refresh-cache", s.app.ScraperHandler.RefreshProjectsCacheHandler)
	mux.HandleFunc("/api/projects/get-issues", s.app.ScraperHandler.GetProjectIssuesHandler)
	mux.HandleFunc("/api/spaces/refresh-cache", s.app.ScraperHandler.RefreshSpacesCacheHandler)
	mux.HandleFunc("/api/spaces/get-pages", s.app.ScraperHandler.GetSpacePagesHandler)

	// API routes - Source management (NEW)
	mux.HandleFunc("/api/sources", s.handleSourcesRoute)                // GET (list), POST (create)
	mux.HandleFunc("/api/sources/", s.handleSourceRoutes)               // GET/PUT/DELETE /{id}
	mux.HandleFunc("/api/status", s.app.StatusHandler.GetStatusHandler) // GET - application status

	// API routes - Data management
	mux.HandleFunc("/api/data", s.handleDataRoute)                                                // DELETE - clear all data (NEW)
	mux.HandleFunc("/api/data/", s.handleDataRoutes)                                              // DELETE /{sourceType} - clear by source (NEW)
	mux.HandleFunc("/api/data/clear-all", s.app.ScraperHandler.ClearAllDataHandler)               // Deprecated: use DELETE /api/data
	mux.HandleFunc("/api/data/jira/clear", s.app.ScraperHandler.ClearJiraDataHandler)             // Deprecated: use DELETE /api/data/jira
	mux.HandleFunc("/api/data/confluence/clear", s.app.ScraperHandler.ClearConfluenceDataHandler) // Deprecated: use DELETE /api/data/confluence
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
	mux.HandleFunc("/api/documents/", s.app.DocumentHandler.ReprocessDocumentHandler) // Handles /api/documents/{id}/reprocess

	// API routes - Chat (RAG-enabled chat)
	mux.HandleFunc("/api/chat", s.app.ChatHandler.ChatHandler)
	mux.HandleFunc("/api/chat/health", s.app.ChatHandler.HealthHandler)

	// MCP (Model Context Protocol) endpoints
	mux.HandleFunc("/mcp", s.app.MCPHandler.HandleRPC)
	mux.HandleFunc("/mcp/info", s.app.MCPHandler.InfoHandler)

	// API routes - Processing
	mux.HandleFunc("/api/processing/status", s.app.DocumentHandler.ProcessingStatusHandler)

	// API routes - Scheduler
	mux.HandleFunc("/api/scheduler/trigger-collection", s.app.SchedulerHandler.TriggerCollectionHandler)

	// API routes - Logs
	mux.HandleFunc("/api/logs/recent", s.app.WSHandler.GetRecentLogsHandler)

	// API routes - Jobs (crawler job management)
	mux.HandleFunc("/api/jobs/stats", s.app.JobHandler.GetJobStatsHandler)
	mux.HandleFunc("/api/jobs", s.app.JobHandler.ListJobsHandler)
	mux.HandleFunc("/api/jobs/", s.handleJobRoutes) // Handles /api/jobs/{id} and subpaths

	// API routes - System
	mux.HandleFunc("/api/version", s.app.APIHandler.VersionHandler)
	mux.HandleFunc("/api/health", s.app.APIHandler.HealthHandler)
	mux.HandleFunc("/api/shutdown", s.ShutdownHandler) // Graceful shutdown endpoint (dev mode)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.app.APIHandler.NotFoundHandler)

	return mux
}

// handleJobRoutes routes job-related requests to the appropriate handler
func (s *Server) handleJobRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// GET /api/jobs/{id}
	if r.Method == "GET" && len(path) > len("/api/jobs/") {
		pathSuffix := path[len("/api/jobs/"):]
		if pathSuffix == "stats" {
			s.app.JobHandler.GetJobStatsHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/results
		if len(pathSuffix) > 0 && pathSuffix[len(pathSuffix)-8:] == "/results" {
			s.app.JobHandler.GetJobResultsHandler(w, r)
			return
		}
		// Otherwise it's /api/jobs/{id}
		s.app.JobHandler.GetJobHandler(w, r)
		return
	}

	// POST /api/jobs/{id}/rerun
	if r.Method == "POST" && len(path) > len("/api/jobs/") {
		pathSuffix := path[len("/api/jobs/"):]
		if len(pathSuffix) > 6 && pathSuffix[len(pathSuffix)-6:] == "/rerun" {
			s.app.JobHandler.RerunJobHandler(w, r)
			return
		}
		// POST /api/jobs/{id}/cancel
		if len(pathSuffix) > 7 && pathSuffix[len(pathSuffix)-7:] == "/cancel" {
			s.app.JobHandler.CancelJobHandler(w, r)
			return
		}
	}

	// DELETE /api/jobs/{id}
	if r.Method == "DELETE" && len(path) > len("/api/jobs/") {
		s.app.JobHandler.DeleteJobHandler(w, r)
		return
	}

	// Default to list handler for GET /api/jobs
	if r.Method == "GET" && path == "/api/jobs" {
		s.app.JobHandler.ListJobsHandler(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleSourcesRoute routes /api/sources requests (list and create)
func (s *Server) handleSourcesRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.app.SourcesHandler.ListSourcesHandler(w, r)
	case "POST":
		s.app.SourcesHandler.CreateSourceHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSourceRoutes routes /api/sources/{id} requests
func (s *Server) handleSourceRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.app.SourcesHandler.GetSourceHandler(w, r)
	case "PUT":
		s.app.SourcesHandler.UpdateSourceHandler(w, r)
	case "DELETE":
		s.app.SourcesHandler.DeleteSourceHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDataRoute routes /api/data requests (clear all)
func (s *Server) handleDataRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		s.app.DataHandler.ClearAllDataHandler(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDataRoutes routes /api/data/{sourceType} requests (clear by source)
func (s *Server) handleDataRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		s.app.DataHandler.ClearDataBySourceHandler(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
