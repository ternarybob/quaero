// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 3:49:44 pm
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
	// Auth redirect handler - handles both /auth and /auth/ with query parameter preservation
	mux.HandleFunc("/auth", s.handleAuthRedirect)
	mux.HandleFunc("/auth/", s.handleAuthRedirect)
	mux.HandleFunc("/jobs", s.app.PageHandler.ServePage("jobs.html", "jobs"))        // Jobs page
	mux.HandleFunc("/jobs/add", s.app.PageHandler.ServePage("job_add.html", "jobs")) // Add job page
	mux.HandleFunc("/job_add", s.app.PageHandler.ServePage("job_add.html", "jobs"))  // Legacy route (backwards compat)
	mux.HandleFunc("/queue", s.app.PageHandler.ServePage("queue.html", "queue"))
	mux.HandleFunc("/job", s.app.PageHandler.ServePage("job.html", "job")) // Job details page (uses ?id= query param)
	mux.HandleFunc("/documents", s.app.PageHandler.ServePage("documents.html", "documents"))
	mux.HandleFunc("/search", s.app.PageHandler.ServePage("search.html", "search"))
	mux.HandleFunc("/chat", s.app.PageHandler.ServePage("chat.html", "chat"))
	mux.HandleFunc("/settings", s.app.PageHandler.ServePage("settings.html", "settings"))
	mux.HandleFunc("/settings/", s.app.PageHandler.ServePartial) // Serve partial HTML fragments

	// Static files (CSS, JS, images)
	mux.HandleFunc("/static/", s.app.PageHandler.StaticFileHandler)

	// Partial files (for AJAX loading)
	mux.HandleFunc("/partials/", s.app.PageHandler.PartialFileHandler)

	// WebSocket route
	mux.HandleFunc("/ws", s.app.WSHandler.HandleWebSocket)

	// API routes - Authentication (Chrome extension)
	mux.HandleFunc("/api/auth", s.app.AuthHandler.CaptureAuthHandler)          // POST - capture auth from extension
	mux.HandleFunc("/api/auth/status", s.app.AuthHandler.GetAuthStatusHandler) // GET - check auth status
	mux.HandleFunc("/api/auth/list", s.app.AuthHandler.ListAuthHandler)        // GET - list all auth credentials
	mux.HandleFunc("/api/auth/", s.handleAuthRoutes)                           // GET/DELETE /{id}
	mux.HandleFunc("/api/status", s.app.StatusHandler.GetStatusHandler)        // GET - application status

	// NOTE: Old data management and collector routes removed - handlers deleted during Stage 2.4 cleanup

	// API routes - Documents
	mux.HandleFunc("/api/documents/stats", s.app.DocumentHandler.StatsHandler)
	mux.HandleFunc("/api/documents/tags", s.app.DocumentHandler.TagsHandler)                    // GET - all unique tags
	mux.HandleFunc("/api/documents/capture", s.app.DocumentHandler.CaptureHandler)              // POST - capture page from Chrome extension
	mux.HandleFunc("/api/documents", s.handleDocumentsRoute)                                    // GET (list) and POST (create)
	mux.HandleFunc("/api/documents/clear-all", s.app.DocumentHandler.DeleteAllDocumentsHandler) // DELETE - danger zone: clear all documents
	mux.HandleFunc("/api/documents/", s.handleDocumentRoutes)                                   // Handles /api/documents/{id} and subpaths

	// API routes - Search
	mux.HandleFunc("/api/search", s.handleSearchRoute)

	// NOTE: MCP endpoints removed from public routes - MCPHandler kept for external API integration

	// NOTE: Processing routes removed - ProcessHandler and ProcessingStatusHandler deleted during Stage 2.4 cleanup

	// NOTE: Scheduler trigger-collection endpoint removed - automatic scheduling via cron (every 5 minutes)

	// API routes - Logs (unified endpoint for service and job logs)
	mux.HandleFunc("/api/logs", s.app.UnifiedLogsHandler.GetLogsHandler)
	mux.HandleFunc("/api/logs/recent", s.app.WSHandler.GetRecentLogsHandler) // Legacy: kept for backward compatibility

	// API routes - Jobs (crawler job management)
	mux.HandleFunc("/api/jobs/stats", s.app.JobHandler.GetJobStatsHandler)
	mux.HandleFunc("/api/jobs", s.app.JobHandler.ListJobsHandler)
	mux.HandleFunc("/api/jobs/", s.handleJobRoutes) // Handles /api/jobs/{id} and subpaths

	// API routes - Job Definitions (configurable job management)
	mux.HandleFunc("/api/job-definitions", s.handleJobDefinitionsRoute)
	mux.HandleFunc("/api/job-definitions/", s.handleJobDefinitionRoutes)

	// API routes - Key/Value Store
	mux.HandleFunc("/api/kv", s.handleKVRoute)   // GET (list), POST (create)
	mux.HandleFunc("/api/kv/", s.handleKVRoutes) // GET/PUT/DELETE /{key}

	// API routes - Connectors
	mux.HandleFunc("/api/connectors", s.handleConnectorsRoute)
	mux.HandleFunc("/api/connectors/", s.handleConnectorRoutes)

	// API routes - GitHub Jobs (repo and actions collectors)
	mux.HandleFunc("/api/github/repo/preview", s.app.GitHubJobsHandler.PreviewRepoFilesHandler)
	mux.HandleFunc("/api/github/repo/start", s.app.GitHubJobsHandler.StartRepoCollectorHandler)
	mux.HandleFunc("/api/github/actions/preview", s.app.GitHubJobsHandler.PreviewActionRunsHandler)
	mux.HandleFunc("/api/github/actions/start", s.app.GitHubJobsHandler.StartActionsCollectorHandler)

	// API routes - System
	mux.HandleFunc("/api/version", s.app.APIHandler.VersionHandler)
	mux.HandleFunc("/api/health", s.app.APIHandler.HealthHandler)
	mux.HandleFunc("/api/config", s.app.ConfigHandler.GetConfig) // GET - application configuration
	mux.HandleFunc("/api/shutdown", s.ShutdownHandler)           // Graceful shutdown endpoint (internal-only, dev mode)

	// API routes - System Logs
	mux.HandleFunc("/api/system/logs/files", s.app.SystemLogsHandler.ListLogFilesHandler)
	mux.HandleFunc("/api/system/logs/content", s.app.SystemLogsHandler.GetLogContentHandler)

	// API routes - Hybrid Scraper (chromedp + extension based scraping)
	mux.HandleFunc("/api/hybrid-scraper/init", s.app.HybridScraperHandler.InitHandler)
	mux.HandleFunc("/api/hybrid-scraper/crawl", s.app.HybridScraperHandler.CrawlHandler)
	mux.HandleFunc("/api/hybrid-scraper/navigate", s.app.HybridScraperHandler.NavigateHandler)
	mux.HandleFunc("/api/hybrid-scraper/status", s.app.HybridScraperHandler.StatusHandler)
	mux.HandleFunc("/api/hybrid-scraper/shutdown", s.app.HybridScraperHandler.ShutdownHandler)
	mux.HandleFunc("/api/hybrid-scraper/inject-stealth", s.app.HybridScraperHandler.InjectStealthHandler)
	mux.HandleFunc("/api/hybrid-scraper/session/", s.app.HybridScraperHandler.SessionHandler)

	// API routes - DevOps Enrichment Pipeline
	mux.HandleFunc("/api/devops/summary", s.app.DevOpsHandler.SummaryHandler)
	mux.HandleFunc("/api/devops/components", s.app.DevOpsHandler.ComponentsHandler)
	mux.HandleFunc("/api/devops/graph", s.app.DevOpsHandler.GraphHandler)
	mux.HandleFunc("/api/devops/platforms", s.app.DevOpsHandler.PlatformsHandler)
	mux.HandleFunc("/api/devops/enrich", s.app.DevOpsHandler.EnrichHandler)

	// 404 handler for unmatched API routes
	mux.HandleFunc("/api/", s.app.APIHandler.NotFoundHandler)

	return mux
}

// handleJobRoutes routes job-related requests to the appropriate handler
func (s *Server) handleJobRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// POST /api/jobs/create
	if r.Method == "POST" && len(path) > len("/api/jobs/") {
		pathSuffix := path[len("/api/jobs/"):]
		if pathSuffix == "create" {
			s.app.JobHandler.CreateJobHandler(w, r)
			return
		}
		// POST /api/jobs/{id}/rerun
		if strings.HasSuffix(pathSuffix, "/rerun") {
			s.app.JobHandler.RerunJobHandler(w, r)
			return
		}
		// POST /api/jobs/{id}/cancel
		if strings.HasSuffix(pathSuffix, "/cancel") {
			s.app.JobHandler.CancelJobHandler(w, r)
			return
		}
		// POST /api/jobs/{id}/copy
		if strings.HasSuffix(pathSuffix, "/copy") {
			s.app.JobHandler.CopyJobHandler(w, r)
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
		if len(pathSuffix) > 0 && len(pathSuffix) >= 8 && pathSuffix[len(pathSuffix)-8:] == "/results" {
			s.app.JobHandler.GetJobResultsHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/structure (lightweight status endpoint)
		if len(pathSuffix) > 0 && strings.HasSuffix(pathSuffix, "/structure") {
			s.app.JobHandler.GetJobStructureHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/tree/logs (tree view logs endpoint)
		if len(pathSuffix) > 0 && strings.HasSuffix(pathSuffix, "/tree/logs") {
			s.app.JobHandler.GetJobTreeLogsHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/tree (GitHub Actions-style tree view)
		if len(pathSuffix) > 0 && strings.HasSuffix(pathSuffix, "/tree") {
			s.app.JobHandler.GetJobTreeHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/logs/aggregated (must come before /logs check)
		if len(pathSuffix) > 0 && strings.HasSuffix(pathSuffix, "/logs/aggregated") {
			s.app.JobHandler.GetAggregatedJobLogsHandler(w, r)
			return
		}
		// Check if it's /api/jobs/{id}/logs
		if len(pathSuffix) > 0 && len(pathSuffix) >= 5 && pathSuffix[len(pathSuffix)-5:] == "/logs" {
			s.app.JobHandler.GetJobLogsHandler(w, r)
			return
		}
		// Otherwise it's /api/jobs/{id}
		s.app.JobHandler.GetJobHandler(w, r)
		return
	}

	// PUT /api/jobs/{id}
	if r.Method == "PUT" && len(path) > len("/api/jobs/") {
		s.app.JobHandler.UpdateJobHandler(w, r)
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

// handleAuthRoutes routes /api/auth/{id} requests
func (s *Server) handleAuthRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Skip if path is /api/auth/status or /api/auth/list (already handled)
	if path == "/api/auth/status" || path == "/api/auth/list" {
		return
	}

	// API key routes removed - API keys are now managed via /api/kv endpoints (Phase 4 cleanup)

	// Handle /api/auth/{id}/cookies - debug endpoint for cookie testing
	if strings.HasSuffix(path, "/cookies") && r.Method == "GET" {
		s.app.AuthHandler.GetAuthCookiesHandler(w, r)
		return
	}

	// Handle /api/auth/{id}
	if len(path) > len("/api/auth/") {
		RouteResourceItem(w, r,
			s.app.AuthHandler.GetAuthHandler,
			nil, // No PUT for auth
			s.app.AuthHandler.DeleteAuthHandler,
		)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleKVRoute routes /api/kv requests (list and create)
func (s *Server) handleKVRoute(w http.ResponseWriter, r *http.Request) {
	RouteResourceCollection(w, r,
		s.app.KVHandler.ListKVHandler,
		s.app.KVHandler.CreateKVHandler,
	)
}

// handleKVRoutes routes /api/kv/{key} requests
func (s *Server) handleKVRoutes(w http.ResponseWriter, r *http.Request) {
	RouteResourceItem(w, r,
		s.app.KVHandler.GetKVHandler,
		s.app.KVHandler.UpdateKVHandler,
		s.app.KVHandler.DeleteKVHandler,
	)
}

// handleConnectorsRoute routes /api/connectors requests (list and create)
func (s *Server) handleConnectorsRoute(w http.ResponseWriter, r *http.Request) {
	RouteResourceCollection(w, r,
		s.app.ConnectorHandler.ListConnectorsHandler,
		s.app.ConnectorHandler.CreateConnectorHandler,
	)
}

// handleConnectorRoutes routes /api/connectors/{id} requests
func (s *Server) handleConnectorRoutes(w http.ResponseWriter, r *http.Request) {
	RouteResourceItem(w, r,
		nil, // Get not exposed yet
		s.app.ConnectorHandler.UpdateConnectorHandler,
		s.app.ConnectorHandler.DeleteConnectorHandler,
	)
}

// handleJobDefinitionsRoute routes /api/job-definitions requests (list and create)
func (s *Server) handleJobDefinitionsRoute(w http.ResponseWriter, r *http.Request) {
	RouteResourceCollection(w, r,
		s.app.JobDefinitionHandler.ListJobDefinitionsHandler,
		s.app.JobDefinitionHandler.CreateJobDefinitionHandler,
	)
}

// handleJobDefinitionRoutes routes /api/job-definitions/{id} requests
func (s *Server) handleJobDefinitionRoutes(w http.ResponseWriter, r *http.Request) {
	// Check for /validate suffix (TOML validation)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/validate") {
		s.app.JobDefinitionHandler.ValidateJobDefinitionTOMLHandler(w, r)
		return
	}

	// Check for /upload suffix (TOML upload)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/upload") {
		s.app.JobDefinitionHandler.UploadJobDefinitionTOMLHandler(w, r)
		return
	}

	// Check for /save-invalid suffix (save invalid TOML for later editing)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/save-invalid") {
		s.app.JobDefinitionHandler.SaveInvalidJobDefinitionHandler(w, r)
		return
	}

	// Check for /quick-crawl suffix (create and execute from Chrome extension)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/quick-crawl") {
		s.app.JobDefinitionHandler.CreateAndExecuteQuickCrawlHandler(w, r)
		return
	}

	// Check for /match-config (find matching job config for URL)
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/match-config") {
		s.app.JobDefinitionHandler.GetMatchingConfigHandler(w, r)
		return
	}

	// Check for /crawl-links suffix (crawl with provided links from extension)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/crawl-links") {
		s.app.JobDefinitionHandler.CrawlWithLinksHandler(w, r)
		return
	}

	// Check for /reload suffix (reload job definitions from disk)
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/reload") {
		s.app.JobDefinitionHandler.ReloadJobDefinitionsHandler(w, r)
		return
	}

	// Check for /execute suffix first
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/execute") {
		s.app.JobDefinitionHandler.ExecuteJobDefinitionHandler(w, r)
		return
	}

	// Check for /export suffix (download as TOML)
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/export") {
		s.app.JobDefinitionHandler.ExportJobDefinitionHandler(w, r)
		return
	}

	// Check for /status suffix (job tree status aggregation)
	if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/status") {
		s.app.JobDefinitionHandler.GetJobTreeStatusHandler(w, r)
		return
	}

	// Standard CRUD operations
	RouteResourceItem(w, r,
		s.app.JobDefinitionHandler.GetJobDefinitionHandler,
		s.app.JobDefinitionHandler.UpdateJobDefinitionHandler,
		s.app.JobDefinitionHandler.DeleteJobDefinitionHandler,
	)
}

// NOTE: handleDataRoute and handleDataRoutes removed - DataHandler deleted during Stage 2.4 cleanup

// handleDocumentsRoute routes /api/documents requests (list and create)
func (s *Server) handleDocumentsRoute(w http.ResponseWriter, r *http.Request) {
	RouteResourceCollection(w, r,
		s.app.DocumentHandler.ListHandler,
		s.app.DocumentHandler.CreateDocumentHandler,
	)
}

// handleDocumentRoutes routes document-related requests to the appropriate handler
func (s *Server) handleDocumentRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// POST /api/documents/{id}/reprocess
	if r.Method == "POST" && len(path) > len("/api/documents/") {
		pathSuffix := path[len("/api/documents/"):]
		if len(pathSuffix) > 10 && pathSuffix[len(pathSuffix)-10:] == "/reprocess" {
			s.app.DocumentHandler.ReprocessDocumentHandler(w, r)
			return
		}
	}

	// DELETE /api/documents/{id}
	if r.Method == "DELETE" && len(path) > len("/api/documents/") {
		s.app.DocumentHandler.DeleteDocumentHandler(w, r)
		return
	}

	// GET /api/documents/{id}
	if r.Method == "GET" && len(path) > len("/api/documents/") {
		s.app.DocumentHandler.GetDocumentHandler(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleSearchRoute delegates to SearchHandler (method enforcement happens at handler level)
func (s *Server) handleSearchRoute(w http.ResponseWriter, r *http.Request) {
	s.app.SearchHandler.SearchHandler(w, r)
}

// handleAuthRedirect redirects /auth and /auth/ to settings page with auth accordions expanded
// Preserves any existing query parameters and uses 308 to maintain HTTP method
func (s *Server) handleAuthRedirect(w http.ResponseWriter, r *http.Request) {
	// Start with existing query parameters
	params := r.URL.Query()

	// Set or append the accordion parameter
	// If 'a' parameter exists, append our auth sections; otherwise set it
	existingA := params.Get("a")
	if existingA != "" {
		// Merge existing accordion sections with auth sections
		params.Set("a", existingA+",kv,auth-cookies")
	} else {
		params.Set("a", "kv,auth-cookies")
	}

	// Build redirect URL
	redirectURL := "/settings?" + params.Encode()

	// Use 308 Permanent Redirect to preserve HTTP method
	http.Redirect(w, r, redirectURL, http.StatusPermanentRedirect)
}
