// -----------------------------------------------------------------------
// Last Modified: Thursday, 9th October 2025 8:53:55 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package server

import (
	"net/http"
	"strings"
)

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// UI Page routes (HTML templates)
	mux.HandleFunc("/", s.app.PageHandler.ServePage("index.html", "home"))
	mux.HandleFunc("/auth", s.app.PageHandler.ServePage("auth.html", "auth"))
	mux.HandleFunc("/sources", s.app.PageHandler.ServePage("sources.html", "sources"))
	mux.HandleFunc("/jobs", s.app.PageHandler.ServePage("jobs.html", "jobs"))
	mux.HandleFunc("/documents", s.app.PageHandler.ServePage("documents.html", "documents"))
	mux.HandleFunc("/chat", s.app.PageHandler.ServePage("chat.html", "chat"))
	mux.HandleFunc("/settings", s.app.PageHandler.ServePage("settings.html", "settings"))
	mux.HandleFunc("/config", s.app.PageHandler.ServePage("config.html", "config"))

	// Static files (CSS, JS, images)
	mux.HandleFunc("/static/", s.app.PageHandler.StaticFileHandler)

	// WebSocket route
	mux.HandleFunc("/ws", s.app.WSHandler.HandleWebSocket)

	// API routes - Authentication (Chrome extension)
	mux.HandleFunc("/api/auth", s.app.AuthHandler.CaptureAuthHandler)          // POST - capture auth from extension
	mux.HandleFunc("/api/auth/status", s.app.AuthHandler.GetAuthStatusHandler) // GET - check auth status
	mux.HandleFunc("/api/auth/list", s.app.AuthHandler.ListAuthHandler)        // GET - list all auth credentials
	mux.HandleFunc("/api/auth/", s.handleAuthRoutes)                           // GET/DELETE /{id}

	// API routes - Source management (NEW)
	mux.HandleFunc("/api/sources", s.handleSourcesRoute)                // GET (list), POST (create)
	mux.HandleFunc("/api/sources/", s.handleSourceRoutes)               // GET/PUT/DELETE /{id}
	mux.HandleFunc("/api/status", s.app.StatusHandler.GetStatusHandler) // GET - application status

	// NOTE: Old data management and collector routes removed - handlers deleted during Stage 2.4 cleanup

	// API routes - Collection (manual data sync)
	mux.HandleFunc("/api/collection/jira/sync", s.app.CollectionHandler.SyncJiraHandler)
	mux.HandleFunc("/api/collection/confluence/sync", s.app.CollectionHandler.SyncConfluenceHandler)
	mux.HandleFunc("/api/collection/sync-all", s.app.CollectionHandler.SyncAllHandler)

	// API routes - Documents
	mux.HandleFunc("/api/documents/stats", s.app.DocumentHandler.StatsHandler)
	mux.HandleFunc("/api/documents", s.app.DocumentHandler.ListHandler)
	mux.HandleFunc("/api/documents/force-sync", s.app.SchedulerHandler.ForceSyncDocumentHandler)
	mux.HandleFunc("/api/documents/", s.app.DocumentHandler.ReprocessDocumentHandler) // Handles /api/documents/{id}/reprocess

	// API routes - Chat (RAG-enabled chat)
	mux.HandleFunc("/api/chat", s.app.ChatHandler.ChatHandler)
	mux.HandleFunc("/api/chat/health", s.app.ChatHandler.HealthHandler)

	// MCP (Model Context Protocol) endpoints
	mux.HandleFunc("/mcp", s.app.MCPHandler.HandleRPC)
	mux.HandleFunc("/mcp/info", s.app.MCPHandler.InfoHandler)

	// NOTE: Processing routes removed - ProcessHandler and ProcessingStatusHandler deleted during Stage 2.4 cleanup

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
	mux.HandleFunc("/api/config", s.app.ConfigHandler.GetConfig) // GET - application configuration
	mux.HandleFunc("/api/shutdown", s.ShutdownHandler)           // Graceful shutdown endpoint (dev mode)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.app.APIHandler.NotFoundHandler)

	return mux
}

// handleJobRoutes routes job-related requests to the appropriate handler
func (s *Server) handleJobRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// GET /api/jobs/default
	if r.Method == "GET" && path == "/api/jobs/default" {
		s.app.JobHandler.GetDefaultJobsHandler(w, r)
		return
	}

	// Handle default job routes
	if strings.Contains(path, "/default/") {
		// POST /api/jobs/default/{name}/enable
		if r.Method == "POST" && strings.HasSuffix(path, "/enable") {
			s.app.JobHandler.EnableDefaultJobHandler(w, r)
			return
		}

		// POST /api/jobs/default/{name}/disable
		if r.Method == "POST" && strings.HasSuffix(path, "/disable") {
			s.app.JobHandler.DisableDefaultJobHandler(w, r)
			return
		}

		// PUT /api/jobs/default/{name}/schedule
		if r.Method == "PUT" && strings.HasSuffix(path, "/schedule") {
			s.app.JobHandler.UpdateDefaultJobScheduleHandler(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// POST /api/jobs/create
	if r.Method == "POST" && len(path) > len("/api/jobs/") {
		pathSuffix := path[len("/api/jobs/"):]
		if pathSuffix == "create" {
			s.app.JobHandler.CreateJobHandler(w, r)
			return
		}
		// POST /api/jobs/{id}/rerun
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

	// GET /api/jobs/{id}
	if r.Method == "GET" && len(path) > len("/api/jobs/") {
		pathSuffix := path[len("/api/jobs/"):]
		// GET /api/jobs/queue
		if pathSuffix == "queue" {
			s.app.JobHandler.GetJobQueueHandler(w, r)
			return
		}
		// GET /api/jobs/stats
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

// handleAuthRoutes routes /api/auth/{id} requests
func (s *Server) handleAuthRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Skip if path is /api/auth/status or /api/auth/list (already handled)
	if path == "/api/auth/status" || path == "/api/auth/list" {
		return
	}

	// Handle /api/auth/{id}
	if len(path) > len("/api/auth/") {
		switch r.Method {
		case "GET":
			s.app.AuthHandler.GetAuthHandler(w, r)
		case "DELETE":
			s.app.AuthHandler.DeleteAuthHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// NOTE: handleDataRoute and handleDataRoutes removed - DataHandler deleted during Stage 2.4 cleanup
