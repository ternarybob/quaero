// -----------------------------------------------------------------------
// Hybrid ChromeDP & Extension Scraper
// Uses authenticated Chrome session via User Data Directory
// Extension acts as execution engine, service acts as controller
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/arbor"
)

// HybridScraperConfig holds configuration for the hybrid scraper
type HybridScraperConfig struct {
	// UserDataDir is the path to Chrome's user data directory (authenticated session)
	UserDataDir string `json:"user_data_dir"`

	// ExtensionPath is the path to the unpacked Chrome extension
	ExtensionPath string `json:"extension_path"`

	// ServerPort is the port for the API server that receives crawled data
	ServerPort int `json:"server_port"`

	// UserAgent overrides the default user agent
	UserAgent string `json:"user_agent"`

	// Headless runs Chrome in headless mode (default: false for max stealth)
	Headless bool `json:"headless"`

	// JavaScriptWaitTime is time to wait for JS rendering
	JavaScriptWaitTime time.Duration `json:"javascript_wait_time"`

	// CrawlTimeout is the max time for entire crawl operation
	CrawlTimeout time.Duration `json:"crawl_timeout"`

	// PageTimeout is the max time for a single page
	PageTimeout time.Duration `json:"page_timeout"`
}

// DefaultHybridScraperConfig returns sensible defaults
func DefaultHybridScraperConfig() HybridScraperConfig {
	return HybridScraperConfig{
		ServerPort:         8086, // Different from main Quaero server
		UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Headless:           false, // Non-headless for max stealth
		JavaScriptWaitTime: 3 * time.Second,
		CrawlTimeout:       30 * time.Minute,
		PageTimeout:        60 * time.Second,
	}
}

// CrawlResult represents a single page crawl result from the extension
type CrawlResult struct {
	URL         string                 `json:"url"`
	HTML        string                 `json:"html"`
	Title       string                 `json:"title"`
	Metadata    map[string]interface{} `json:"metadata"`
	Links       []string               `json:"links,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CrawledAt   time.Time              `json:"crawled_at"`
	RenderTime  int64                  `json:"render_time_ms"`
	ContentSize int                    `json:"content_size"`
}

// CrawlSession represents an active crawl session
type CrawlSession struct {
	ID           string        `json:"id"`
	StartURL     string        `json:"start_url"`
	LinksToCrawl []string      `json:"links_to_crawl"`
	Results      []CrawlResult `json:"results"`
	Status       string        `json:"status"` // "pending", "running", "completed", "failed"
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Error        string        `json:"error,omitempty"`
	mu           sync.Mutex
}

// CrawlSessionData is a thread-safe snapshot of session data for external use
type CrawlSessionData struct {
	ID           string        `json:"id"`
	StartURL     string        `json:"start_url"`
	LinksToCrawl []string      `json:"links_to_crawl"`
	Results      []CrawlResult `json:"results"`
	ResultsCount int           `json:"results_count"`
	Status       string        `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Duration     string        `json:"duration,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// GetData returns a thread-safe snapshot of the session
func (s *CrawlSession) GetData() CrawlSessionData {
	s.mu.Lock()
	defer s.mu.Unlock()

	data := CrawlSessionData{
		ID:           s.ID,
		StartURL:     s.StartURL,
		LinksToCrawl: append([]string{}, s.LinksToCrawl...),
		Results:      append([]CrawlResult{}, s.Results...),
		ResultsCount: len(s.Results),
		Status:       s.Status,
		StartedAt:    s.StartedAt,
		CompletedAt:  s.CompletedAt,
		Error:        s.Error,
	}

	if s.CompletedAt != nil {
		data.Duration = s.CompletedAt.Sub(s.StartedAt).String()
	}

	return data
}

// HybridScraper orchestrates Chrome browser with extension for stealthy scraping
type HybridScraper struct {
	config          HybridScraperConfig
	logger          arbor.ILogger
	browserCtx      context.Context
	browserCancel   context.CancelFunc
	allocatorCancel context.CancelFunc

	// HTTP server for receiving data from extension
	server *http.Server
	mux    *http.ServeMux

	// Active sessions
	sessions   map[string]*CrawlSession
	sessionsMu sync.RWMutex

	// Results channel for streaming results
	resultsChan chan CrawlResult

	// State
	initialized bool
	mu          sync.Mutex
}

// NewHybridScraper creates a new hybrid scraper instance
func NewHybridScraper(config HybridScraperConfig, logger arbor.ILogger) *HybridScraper {
	return &HybridScraper{
		config:      config,
		logger:      logger,
		sessions:    make(map[string]*CrawlSession),
		resultsChan: make(chan CrawlResult, 100),
	}
}

// Initialize sets up the Chrome browser with user data directory and extension
func (h *HybridScraper) Initialize(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.initialized {
		return fmt.Errorf("hybrid scraper already initialized")
	}

	h.logger.Info().
		Str("user_data_dir", h.config.UserDataDir).
		Str("extension_path", h.config.ExtensionPath).
		Int("server_port", h.config.ServerPort).
		Bool("headless", h.config.Headless).
		Msg("Initializing hybrid scraper")

	// Validate user data directory
	if h.config.UserDataDir != "" {
		if _, err := os.Stat(h.config.UserDataDir); os.IsNotExist(err) {
			return fmt.Errorf("user data directory does not exist: %s", h.config.UserDataDir)
		}
	}

	// Validate extension path
	if h.config.ExtensionPath != "" {
		manifestPath := filepath.Join(h.config.ExtensionPath, "manifest.json")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return fmt.Errorf("extension manifest not found at: %s", manifestPath)
		}
	}

	// Start the API server first (extension needs to POST to it)
	if err := h.startAPIServer(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	// Build Chrome allocator options for maximum stealth
	opts := h.buildAllocatorOptions()

	// Create allocator with user data directory
	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(ctx, opts...)
	h.allocatorCancel = allocatorCancel

	// Create browser context
	browserCtx, browserCancel := chromedp.NewContext(allocatorCtx,
		chromedp.WithLogf(func(s string, i ...interface{}) {
			h.logger.Debug().Msgf("chromedp: "+s, i...)
		}),
	)
	h.browserCtx = browserCtx
	h.browserCancel = browserCancel

	// Test browser startup
	h.logger.Debug().Msg("Testing browser startup...")
	testCtx, testCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer testCancel()

	if err := chromedp.Run(testCtx, chromedp.Navigate("about:blank")); err != nil {
		h.Shutdown()
		return fmt.Errorf("browser failed startup test: %w", err)
	}

	h.initialized = true
	h.logger.Info().Msg("Hybrid scraper initialized successfully")

	return nil
}

// buildAllocatorOptions creates Chrome allocator options for maximum stealth
func (h *HybridScraper) buildAllocatorOptions() []chromedp.ExecAllocatorOption {
	opts := []chromedp.ExecAllocatorOption{
		// Basic Chrome flags
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,

		// User agent
		chromedp.UserAgent(h.config.UserAgent),

		// STEALTH FLAGS - Critical for bypassing bot detection
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-extensions-except", h.config.ExtensionPath),
		chromedp.Flag("disable-popup-blocking", true),

		// Prevent automation detection
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("useAutomationExtension", false),

		// WebGL and Canvas fingerprint spoofing hints
		chromedp.Flag("disable-reading-from-canvas", false),
		chromedp.Flag("enable-webgl", true),

		// Network stack preferences
		chromedp.Flag("disable-background-networking", false),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),

		// GPU settings for rendering
		chromedp.Flag("disable-gpu", false),

		// Window settings
		chromedp.Flag("start-maximized", true),
		chromedp.WindowSize(1920, 1080),
	}

	// User Data Directory - CRITICAL for authenticated sessions
	if h.config.UserDataDir != "" {
		opts = append(opts, chromedp.UserDataDir(h.config.UserDataDir))
		h.logger.Debug().Str("path", h.config.UserDataDir).Msg("Using user data directory")
	}

	// Extension loading
	if h.config.ExtensionPath != "" {
		opts = append(opts, chromedp.Flag("load-extension", h.config.ExtensionPath))
		h.logger.Debug().Str("path", h.config.ExtensionPath).Msg("Loading extension")
	}

	// Headless mode
	if h.config.Headless {
		// Use new headless mode which is less detectable
		opts = append(opts, chromedp.Flag("headless", "new"))
		h.logger.Debug().Msg("Running in new headless mode")
	}

	return opts
}

// startAPIServer starts the HTTP server that receives crawled data from extension
func (h *HybridScraper) startAPIServer() error {
	h.mux = http.NewServeMux()

	// Endpoint for receiving crawled page data from extension
	h.mux.HandleFunc("/api/crawl-data", h.handleCrawlData)

	// Endpoint for extension health check
	h.mux.HandleFunc("/api/health", h.handleHealth)

	// Endpoint to get current session status
	h.mux.HandleFunc("/api/session/status", h.handleSessionStatus)

	addr := fmt.Sprintf("127.0.0.1:%d", h.config.ServerPort)
	h.server = &http.Server{
		Addr:         addr,
		Handler:      h.corsMiddleware(h.mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		h.logger.Info().Str("addr", addr).Msg("Starting hybrid scraper API server")
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error().Err(err).Msg("API server error")
		}
	}()

	// Wait briefly for server to start
	time.Sleep(100 * time.Millisecond)

	return nil
}

// corsMiddleware adds CORS headers for extension communication
func (h *HybridScraper) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from Chrome extension
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleCrawlData receives crawled page HTML from the extension
func (h *HybridScraper) handleCrawlData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result CrawlResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		h.logger.Error().Err(err).Msg("Failed to decode crawl result")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result.CrawledAt = time.Now()
	result.ContentSize = len(result.HTML)

	h.logger.Debug().
		Str("url", result.URL).
		Str("title", result.Title).
		Int("content_size", result.ContentSize).
		Int64("render_time_ms", result.RenderTime).
		Msg("Received crawl result from extension")

	// Extract session ID from query param or header
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}

	// Add to session if available
	if sessionID != "" {
		h.addResultToSession(sessionID, result)
	}

	// Send to results channel for streaming
	select {
	case h.resultsChan <- result:
	default:
		h.logger.Warn().Msg("Results channel full, dropping result")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "received",
		"url":     result.URL,
		"message": "Page data received successfully",
	})
}

// handleHealth returns server health status
func (h *HybridScraper) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "ok",
		"initialized": h.initialized,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// handleSessionStatus returns current crawl session status
func (h *HybridScraper) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")

	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// addResultToSession adds a crawl result to a session
func (h *HybridScraper) addResultToSession(sessionID string, result CrawlResult) {
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !exists {
		h.logger.Warn().Str("session_id", sessionID).Msg("Session not found for result")
		return
	}

	session.mu.Lock()
	session.Results = append(session.Results, result)
	session.mu.Unlock()

	h.logger.Debug().
		Str("session_id", sessionID).
		Int("results_count", len(session.Results)).
		Msg("Added result to session")
}

// StartCrawlSession initiates a new crawl session
func (h *HybridScraper) StartCrawlSession(ctx context.Context, startURL string, linksToCrawl []string) (*CrawlSession, error) {
	h.mu.Lock()
	if !h.initialized {
		h.mu.Unlock()
		return nil, fmt.Errorf("hybrid scraper not initialized")
	}
	h.mu.Unlock()

	// Generate session ID
	sessionID := fmt.Sprintf("crawl_%d", time.Now().UnixNano())

	session := &CrawlSession{
		ID:           sessionID,
		StartURL:     startURL,
		LinksToCrawl: linksToCrawl,
		Results:      []CrawlResult{},
		Status:       "pending",
		StartedAt:    time.Now(),
	}

	// Store session
	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	h.logger.Info().
		Str("session_id", sessionID).
		Str("start_url", startURL).
		Int("links_count", len(linksToCrawl)).
		Msg("Starting crawl session")

	// Execute crawl in goroutine
	go h.executeCrawlSession(ctx, session)

	return session, nil
}

// executeCrawlSession runs the crawl session
func (h *HybridScraper) executeCrawlSession(ctx context.Context, session *CrawlSession) {
	session.mu.Lock()
	session.Status = "running"
	session.mu.Unlock()

	// Create timeout context
	crawlCtx, cancel := context.WithTimeout(ctx, h.config.CrawlTimeout)
	defer cancel()

	// Navigate to start URL
	h.logger.Debug().Str("url", session.StartURL).Msg("Navigating to start URL")

	err := chromedp.Run(crawlCtx,
		chromedp.Navigate(session.StartURL),
		chromedp.Sleep(h.config.JavaScriptWaitTime),
	)

	if err != nil {
		session.mu.Lock()
		session.Status = "failed"
		session.Error = fmt.Sprintf("navigation failed: %v", err)
		now := time.Now()
		session.CompletedAt = &now
		session.mu.Unlock()
		h.logger.Error().Err(err).Str("url", session.StartURL).Msg("Failed to navigate to start URL")
		return
	}

	// Prepare data for extension
	crawlData := map[string]interface{}{
		"sessionId":    session.ID,
		"links":        session.LinksToCrawl,
		"serverUrl":    fmt.Sprintf("http://127.0.0.1:%d", h.config.ServerPort),
		"pageTimeout":  h.config.PageTimeout.Milliseconds(),
		"waitTime":     h.config.JavaScriptWaitTime.Milliseconds(),
		"includeLinks": true,
	}

	crawlDataJSON, err := json.Marshal(crawlData)
	if err != nil {
		session.mu.Lock()
		session.Status = "failed"
		session.Error = fmt.Sprintf("failed to marshal crawl data: %v", err)
		now := time.Now()
		session.CompletedAt = &now
		session.mu.Unlock()
		return
	}

	// Call extension's startCrawl function
	h.logger.Debug().Int("links", len(session.LinksToCrawl)).Msg("Calling extension startCrawl function")

	var result interface{}
	startCrawlJS := fmt.Sprintf(`
		(function() {
			if (typeof window.quaeroStartCrawl === 'function') {
				return window.quaeroStartCrawl(%s);
			} else if (typeof window.startCrawl === 'function') {
				return window.startCrawl(%s);
			} else {
				return { error: 'startCrawl function not found. Extension may not be loaded.' };
			}
		})()
	`, string(crawlDataJSON), string(crawlDataJSON))

	err = chromedp.Run(crawlCtx,
		chromedp.Evaluate(startCrawlJS, &result),
	)

	if err != nil {
		session.mu.Lock()
		session.Status = "failed"
		session.Error = fmt.Sprintf("failed to call startCrawl: %v", err)
		now := time.Now()
		session.CompletedAt = &now
		session.mu.Unlock()
		h.logger.Error().Err(err).Msg("Failed to call extension startCrawl")
		return
	}

	h.logger.Debug().Interface("result", result).Msg("startCrawl called")

	// Wait for crawl to complete (extension will POST results back)
	// We wait for all expected results or timeout
	expectedResults := len(session.LinksToCrawl)
	if expectedResults == 0 {
		expectedResults = 1 // At least the start URL
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-crawlCtx.Done():
			session.mu.Lock()
			if session.Status == "running" {
				session.Status = "completed"
				now := time.Now()
				session.CompletedAt = &now
			}
			session.mu.Unlock()
			h.logger.Info().
				Str("session_id", session.ID).
				Int("results", len(session.Results)).
				Msg("Crawl session completed (timeout)")
			return

		case <-ticker.C:
			session.mu.Lock()
			resultsCount := len(session.Results)
			session.mu.Unlock()

			if resultsCount >= expectedResults {
				session.mu.Lock()
				session.Status = "completed"
				now := time.Now()
				session.CompletedAt = &now
				session.mu.Unlock()
				h.logger.Info().
					Str("session_id", session.ID).
					Int("results", resultsCount).
					Msg("Crawl session completed (all results received)")
				return
			}
		}
	}
}

// NavigateAndCrawl navigates to a URL and extracts content using the extension
func (h *HybridScraper) NavigateAndCrawl(ctx context.Context, targetURL string) (*CrawlResult, error) {
	h.mu.Lock()
	if !h.initialized {
		h.mu.Unlock()
		return nil, fmt.Errorf("hybrid scraper not initialized")
	}
	h.mu.Unlock()

	pageCtx, cancel := context.WithTimeout(ctx, h.config.PageTimeout)
	defer cancel()

	h.logger.Debug().Str("url", targetURL).Msg("Navigating to URL")

	// Navigate to URL
	err := chromedp.Run(pageCtx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(h.config.JavaScriptWaitTime),
	)

	if err != nil {
		return nil, fmt.Errorf("navigation failed: %w", err)
	}

	// Extract page content via extension
	var html, title string
	var metadata map[string]interface{}

	err = chromedp.Run(pageCtx,
		chromedp.OuterHTML("html", &html),
		chromedp.Title(&title),
		chromedp.Evaluate(`({
			url: window.location.href,
			description: document.querySelector('meta[name="description"]')?.content || '',
			language: document.documentElement.lang || 'en',
			canonical: document.querySelector('link[rel="canonical"]')?.href || window.location.href
		})`, &metadata),
	)

	if err != nil {
		return nil, fmt.Errorf("content extraction failed: %w", err)
	}

	result := &CrawlResult{
		URL:         targetURL,
		HTML:        html,
		Title:       title,
		Metadata:    metadata,
		CrawledAt:   time.Now(),
		ContentSize: len(html),
	}

	return result, nil
}

// GetSession returns a crawl session by ID
func (h *HybridScraper) GetSession(sessionID string) (*CrawlSession, bool) {
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()
	session, exists := h.sessions[sessionID]
	return session, exists
}

// GetResultsChannel returns the channel for streaming results
func (h *HybridScraper) GetResultsChannel() <-chan CrawlResult {
	return h.resultsChan
}

// Shutdown cleanly shuts down the hybrid scraper
func (h *HybridScraper) Shutdown() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.logger.Info().Msg("Shutting down hybrid scraper")

	// Stop API server
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.server.Shutdown(ctx); err != nil {
			h.logger.Warn().Err(err).Msg("Error shutting down API server")
		}
	}

	// Cancel browser context
	if h.browserCancel != nil {
		h.browserCancel()
	}

	// Cancel allocator
	if h.allocatorCancel != nil {
		h.allocatorCancel()
	}

	// Close results channel
	close(h.resultsChan)

	h.initialized = false
	h.logger.Info().Msg("Hybrid scraper shut down successfully")

	return nil
}

// IsInitialized returns whether the scraper is initialized
func (h *HybridScraper) IsInitialized() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.initialized
}

// GetBrowserContext returns the browser context for advanced operations
func (h *HybridScraper) GetBrowserContext() context.Context {
	return h.browserCtx
}

// InjectStealthScript injects stealth JavaScript into the current page
func (h *HybridScraper) InjectStealthScript(ctx context.Context) error {
	stealthJS := `
		// Override navigator.webdriver
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined,
			configurable: true
		});

		// Override navigator.plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => {
				const plugins = [
					{ name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer' },
					{ name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai' },
					{ name: 'Native Client', filename: 'internal-nacl-plugin' }
				];
				plugins.length = 3;
				return plugins;
			},
			configurable: true
		});

		// Override navigator.languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en'],
			configurable: true
		});

		// Override chrome.runtime
		if (!window.chrome) window.chrome = {};
		window.chrome.runtime = { id: undefined };

		// Override permissions.query
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);

		// Fix WebGL vendor/renderer
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) return 'Intel Inc.';
			if (parameter === 37446) return 'Intel Iris OpenGL Engine';
			return getParameter.call(this, parameter);
		};

		console.log('Quaero stealth scripts injected');
	`

	return chromedp.Run(ctx, chromedp.Evaluate(stealthJS, nil))
}
