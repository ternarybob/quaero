// -----------------------------------------------------------------------
// Hybrid Scraper Handler
// API endpoints for controlling the hybrid chromedp + extension scraper
// -----------------------------------------------------------------------

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/services/crawler"
)

// HybridScraperHandler handles API requests for hybrid scraping operations
type HybridScraperHandler struct {
	scraper *crawler.HybridScraper
	logger  arbor.ILogger
	mu      sync.Mutex
}

// NewHybridScraperHandler creates a new hybrid scraper handler
func NewHybridScraperHandler(logger arbor.ILogger) *HybridScraperHandler {
	return &HybridScraperHandler{
		logger: logger,
	}
}

// InitRequest represents a request to initialize the hybrid scraper
type InitRequest struct {
	UserDataDir        string `json:"user_data_dir"`
	ExtensionPath      string `json:"extension_path"`
	ServerPort         int    `json:"server_port"`
	UserAgent          string `json:"user_agent"`
	Headless           bool   `json:"headless"`
	JavaScriptWaitTime int    `json:"javascript_wait_time_ms"`
	CrawlTimeout       int    `json:"crawl_timeout_minutes"`
	PageTimeout        int    `json:"page_timeout_seconds"`
}

// CrawlRequest represents a request to start a crawl session
type CrawlRequest struct {
	StartURL string   `json:"start_url"`
	Links    []string `json:"links"`
}

// InitHandler handles POST /api/hybrid-scraper/init
// Initializes the hybrid scraper with configuration
func (h *HybridScraperHandler) InitHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already initialized
	if h.scraper != nil && h.scraper.IsInitialized() {
		WriteError(w, http.StatusConflict, "Hybrid scraper already initialized. Call /shutdown first.")
		return
	}

	// Parse request
	var req InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode init request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.UserDataDir == "" {
		WriteError(w, http.StatusBadRequest, "user_data_dir is required for authenticated sessions")
		return
	}

	// Build configuration
	config := crawler.DefaultHybridScraperConfig()
	config.UserDataDir = req.UserDataDir

	if req.ExtensionPath != "" {
		config.ExtensionPath = req.ExtensionPath
	}
	if req.ServerPort > 0 {
		config.ServerPort = req.ServerPort
	}
	if req.UserAgent != "" {
		config.UserAgent = req.UserAgent
	}
	config.Headless = req.Headless

	if req.JavaScriptWaitTime > 0 {
		config.JavaScriptWaitTime = time.Duration(req.JavaScriptWaitTime) * time.Millisecond
	}
	if req.CrawlTimeout > 0 {
		config.CrawlTimeout = time.Duration(req.CrawlTimeout) * time.Minute
	}
	if req.PageTimeout > 0 {
		config.PageTimeout = time.Duration(req.PageTimeout) * time.Second
	}

	h.logger.Info().
		Str("user_data_dir", config.UserDataDir).
		Str("extension_path", config.ExtensionPath).
		Int("server_port", config.ServerPort).
		Bool("headless", config.Headless).
		Msg("Initializing hybrid scraper")

	// Create and initialize scraper
	h.scraper = crawler.NewHybridScraper(config, h.logger)

	if err := h.scraper.Initialize(r.Context()); err != nil {
		h.logger.Error().Err(err).Msg("Failed to initialize hybrid scraper")
		h.scraper = nil
		WriteError(w, http.StatusInternalServerError, "Failed to initialize: "+err.Error())
		return
	}

	h.logger.Info().Msg("Hybrid scraper initialized successfully")

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "initialized",
		"server_port": config.ServerPort,
		"message":     "Hybrid scraper initialized. Browser launched with authenticated session.",
	})
}

// CrawlHandler handles POST /api/hybrid-scraper/crawl
// Starts a new crawl session
func (h *HybridScraperHandler) CrawlHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.mu.Lock()
	scraper := h.scraper
	h.mu.Unlock()

	if scraper == nil || !scraper.IsInitialized() {
		WriteError(w, http.StatusPreconditionFailed, "Hybrid scraper not initialized. Call /init first.")
		return
	}

	// Parse request
	var req CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode crawl request")
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate
	if req.StartURL == "" {
		WriteError(w, http.StatusBadRequest, "start_url is required")
		return
	}

	h.logger.Info().
		Str("start_url", req.StartURL).
		Int("links_count", len(req.Links)).
		Msg("Starting crawl session")

	// Start crawl session
	session, err := scraper.StartCrawlSession(r.Context(), req.StartURL, req.Links)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to start crawl session")
		WriteError(w, http.StatusInternalServerError, "Failed to start crawl: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]interface{}{
		"session_id":   session.ID,
		"status":       session.Status,
		"start_url":    session.StartURL,
		"links_count":  len(session.LinksToCrawl),
		"started_at":   session.StartedAt.Format(time.RFC3339),
		"message":      "Crawl session started. Extension will process links and POST results.",
		"results_url":  "/api/hybrid-scraper/session/" + session.ID,
	})
}

// NavigateHandler handles POST /api/hybrid-scraper/navigate
// Navigates to a single URL and extracts content
func (h *HybridScraperHandler) NavigateHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.mu.Lock()
	scraper := h.scraper
	h.mu.Unlock()

	if scraper == nil || !scraper.IsInitialized() {
		WriteError(w, http.StatusPreconditionFailed, "Hybrid scraper not initialized. Call /init first.")
		return
	}

	// Parse request
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "url is required")
		return
	}

	h.logger.Debug().Str("url", req.URL).Msg("Navigating to URL")

	result, err := scraper.NavigateAndCrawl(r.Context(), req.URL)
	if err != nil {
		h.logger.Error().Err(err).Str("url", req.URL).Msg("Navigation failed")
		WriteError(w, http.StatusInternalServerError, "Navigation failed: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"url":          result.URL,
		"title":        result.Title,
		"content_size": result.ContentSize,
		"metadata":     result.Metadata,
		"links_count":  len(result.Links),
		"crawled_at":   result.CrawledAt.Format(time.RFC3339),
		"html":         result.HTML,
	})
}

// SessionHandler handles GET /api/hybrid-scraper/session/{id}
// Returns the status and results of a crawl session
func (h *HybridScraperHandler) SessionHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	h.mu.Lock()
	scraper := h.scraper
	h.mu.Unlock()

	if scraper == nil {
		WriteError(w, http.StatusPreconditionFailed, "Hybrid scraper not initialized")
		return
	}

	// Extract session ID from path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		WriteError(w, http.StatusBadRequest, "Session ID required")
		return
	}
	sessionID := parts[len(parts)-1]

	session, exists := scraper.GetSession(sessionID)
	if !exists {
		WriteError(w, http.StatusNotFound, "Session not found")
		return
	}

	// Get thread-safe snapshot of session data
	data := session.GetData()

	response := map[string]interface{}{
		"session_id":     data.ID,
		"status":         data.Status,
		"start_url":      data.StartURL,
		"links_to_crawl": data.LinksToCrawl,
		"results_count":  data.ResultsCount,
		"started_at":     data.StartedAt.Format(time.RFC3339),
	}

	if data.CompletedAt != nil {
		response["completed_at"] = data.CompletedAt.Format(time.RFC3339)
		response["duration"] = data.Duration
	}

	if data.Error != "" {
		response["error"] = data.Error
	}

	// Include results if requested
	if r.URL.Query().Get("include_results") == "true" {
		results := make([]map[string]interface{}, 0, len(data.Results))
		for _, res := range data.Results {
			results = append(results, map[string]interface{}{
				"url":          res.URL,
				"title":        res.Title,
				"content_size": res.ContentSize,
				"render_time":  res.RenderTime,
				"crawled_at":   res.CrawledAt.Format(time.RFC3339),
				"error":        res.Error,
			})
		}
		response["results"] = results
	}

	WriteJSON(w, http.StatusOK, response)
}

// StatusHandler handles GET /api/hybrid-scraper/status
// Returns the current status of the hybrid scraper
func (h *HybridScraperHandler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodGet) {
		return
	}

	h.mu.Lock()
	scraper := h.scraper
	h.mu.Unlock()

	status := map[string]interface{}{
		"initialized": scraper != nil && scraper.IsInitialized(),
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	if scraper != nil && scraper.IsInitialized() {
		status["message"] = "Hybrid scraper is running with authenticated browser session"
	} else {
		status["message"] = "Hybrid scraper not initialized. Call POST /api/hybrid-scraper/init"
	}

	WriteJSON(w, http.StatusOK, status)
}

// ShutdownHandler handles POST /api/hybrid-scraper/shutdown
// Shuts down the hybrid scraper
func (h *HybridScraperHandler) ShutdownHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.scraper == nil {
		WriteError(w, http.StatusNotFound, "Hybrid scraper not initialized")
		return
	}

	h.logger.Info().Msg("Shutting down hybrid scraper")

	if err := h.scraper.Shutdown(); err != nil {
		h.logger.Error().Err(err).Msg("Error shutting down hybrid scraper")
		WriteError(w, http.StatusInternalServerError, "Shutdown error: "+err.Error())
		return
	}

	h.scraper = nil

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "shutdown",
		"message": "Hybrid scraper shut down successfully",
	})
}

// InjectStealthHandler handles POST /api/hybrid-scraper/inject-stealth
// Injects stealth JavaScript into the current page
func (h *HybridScraperHandler) InjectStealthHandler(w http.ResponseWriter, r *http.Request) {
	if !RequireMethod(w, r, http.MethodPost) {
		return
	}

	h.mu.Lock()
	scraper := h.scraper
	h.mu.Unlock()

	if scraper == nil || !scraper.IsInitialized() {
		WriteError(w, http.StatusPreconditionFailed, "Hybrid scraper not initialized")
		return
	}

	ctx := scraper.GetBrowserContext()
	if ctx == nil {
		WriteError(w, http.StatusInternalServerError, "Browser context not available")
		return
	}

	if err := scraper.InjectStealthScript(ctx); err != nil {
		h.logger.Error().Err(err).Msg("Failed to inject stealth script")
		WriteError(w, http.StatusInternalServerError, "Failed to inject: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "injected",
		"message": "Stealth scripts injected into current page",
	})
}
