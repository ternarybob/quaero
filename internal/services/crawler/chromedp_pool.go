// -----------------------------------------------------------------------
// Last Modified: Friday, 7th November 2025
// Modified By: Kiro AI Assistant
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/arbor"
)

// ChromeDPPool manages a pool of ChromeDP browser contexts for efficient JavaScript rendering
// Provides round-robin allocation and proper resource management
type ChromeDPPool struct {
	browsers         []context.Context
	browserCancels   []context.CancelFunc
	allocatorCancels []context.CancelFunc
	mu               sync.Mutex
	maxInstances     int
	currentIndex     int
	logger           arbor.ILogger
	userAgent        string
	initialized      bool
}

// ChromeDPPoolConfig holds configuration for the browser pool
type ChromeDPPoolConfig struct {
	MaxInstances       int           `json:"max_instances"`
	UserAgent          string        `json:"user_agent"`
	Headless           bool          `json:"headless"`
	DisableGPU         bool          `json:"disable_gpu"`
	NoSandbox          bool          `json:"no_sandbox"`
	JavaScriptWaitTime time.Duration `json:"javascript_wait_time"`
	RequestTimeout     time.Duration `json:"request_timeout"`
}

// NewChromeDPPool creates a new ChromeDP browser pool
func NewChromeDPPool(config ChromeDPPoolConfig, logger arbor.ILogger) *ChromeDPPool {
	return &ChromeDPPool{
		maxInstances: config.MaxInstances,
		userAgent:    config.UserAgent,
		logger:       logger,
		initialized:  false,
	}
}

// InitBrowserPool initializes the browser pool with the specified configuration
func (p *ChromeDPPool) InitBrowserPool(config ChromeDPPoolConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return fmt.Errorf("browser pool already initialized")
	}

	// Validate configuration
	if config.MaxInstances <= 0 {
		return fmt.Errorf("max_instances must be greater than 0, got: %d", config.MaxInstances)
	}
	if config.MaxInstances > 20 {
		p.logger.Warn().
			Int("max_instances", config.MaxInstances).
			Msg("Large browser pool size detected - this may consume significant memory")
	}
	if config.UserAgent == "" {
		config.UserAgent = "Quaero-Crawler/1.0"
		p.logger.Debug().Msg("Using default user agent")
	}

	p.maxInstances = config.MaxInstances
	p.userAgent = config.UserAgent
	p.browsers = make([]context.Context, 0, p.maxInstances)
	p.browserCancels = make([]context.CancelFunc, 0, p.maxInstances)
	p.allocatorCancels = make([]context.CancelFunc, 0, p.maxInstances)
	p.currentIndex = 0

	p.logger.Info().
		Int("pool_size", p.maxInstances).
		Str("user_agent", p.userAgent).
		Bool("headless", config.Headless).
		Dur("js_wait_time", config.JavaScriptWaitTime).
		Msg("Initializing ChromeDP browser pool")

	// Create browser instances with error handling
	successCount := 0
	var lastErr error
	for i := 0; i < p.maxInstances; i++ {
		if err := p.createBrowserInstance(i, config); err != nil {
			lastErr = err
			p.logger.Warn().
				Err(err).
				Int("browser_index", i).
				Int("successful_instances", successCount).
				Msg("Failed to create browser instance")

			// If we can't create any instances, fail completely
			if successCount == 0 {
				p.cleanupInstances()
				return fmt.Errorf("failed to create any browser instances, last error: %w", err)
			}

			// If we have some instances, continue but log the failure
			continue
		}
		successCount++
	}

	// Update maxInstances to reflect actual created instances
	if successCount < p.maxInstances {
		p.logger.Warn().
			Int("requested", p.maxInstances).
			Int("created", successCount).
			Err(lastErr).
			Msg("Created fewer browser instances than requested")
		p.maxInstances = successCount
	}

	p.initialized = true
	p.logger.Info().
		Int("browsers_created", len(p.browsers)).
		Int("requested", config.MaxInstances).
		Msg("ChromeDP browser pool initialized successfully")

	return nil
}

// createBrowserInstance creates a single browser instance and adds it to the pool
func (p *ChromeDPPool) createBrowserInstance(index int, config ChromeDPPoolConfig) error {
	startTime := time.Now()

	// Create allocator context with configuration options
	allocatorOpts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", config.Headless),
		chromedp.Flag("disable-gpu", config.DisableGPU),
		chromedp.Flag("no-sandbox", config.NoSandbox),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-background-timer-throttling", false),
		chromedp.Flag("disable-backgrounding-occluded-windows", false),
		chromedp.Flag("disable-renderer-backgrounding", false),
		chromedp.UserAgent(config.UserAgent),
	)

	allocatorCtx, allocatorCancel := chromedp.NewExecAllocator(
		context.Background(),
		allocatorOpts...,
	)

	// Create browser context from allocator
	browserCtx, browserCancel := chromedp.NewContext(allocatorCtx)

	// Test the browser instance by running a simple task with timeout
	testTimeout := 30 * time.Second
	if config.RequestTimeout > 0 {
		testTimeout = config.RequestTimeout
	}

	testCtx, testCancel := context.WithTimeout(browserCtx, testTimeout)
	defer testCancel()

	// Run startup test
	if err := chromedp.Run(testCtx, chromedp.Navigate("about:blank")); err != nil {
		// Clean up failed instance
		browserCancel()
		allocatorCancel()
		return fmt.Errorf("browser instance failed startup test: %w", err)
	}

	// Additional test to ensure browser is responsive
	var title string
	if err := chromedp.Run(testCtx, chromedp.Title(&title)); err != nil {
		browserCancel()
		allocatorCancel()
		return fmt.Errorf("browser instance failed responsiveness test: %w", err)
	}

	// Add to pool
	p.browsers = append(p.browsers, browserCtx)
	p.browserCancels = append(p.browserCancels, browserCancel)
	p.allocatorCancels = append(p.allocatorCancels, allocatorCancel)

	duration := time.Since(startTime)
	p.logger.Debug().
		Int("browser_index", index).
		Dur("startup_time", duration).
		Msg("Browser instance created and tested successfully")

	return nil
}

// GetBrowser returns a browser context from the pool using round-robin allocation
// Returns the browser context and a release function that should be called when done
func (p *ChromeDPPool) GetBrowser() (context.Context, func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, nil, fmt.Errorf("browser pool not initialized")
	}

	if len(p.browsers) == 0 {
		return nil, nil, fmt.Errorf("no browser instances available")
	}

	// Round-robin allocation
	index := p.currentIndex % len(p.browsers)
	p.currentIndex = (p.currentIndex + 1) % len(p.browsers)

	browserCtx := p.browsers[index]

	// Create a release function (currently no-op since we're using round-robin)
	// In the future, this could track usage or implement more sophisticated allocation
	releaseFunc := func() {
		p.logger.Debug().
			Int("browser_index", index).
			Msg("Browser context released")
	}

	p.logger.Debug().
		Int("browser_index", index).
		Int("total_browsers", len(p.browsers)).
		Msg("Browser context allocated from pool")

	return browserCtx, releaseFunc, nil
}

// ReleaseBrowser releases a browser context back to the pool
// Currently a no-op with round-robin allocation, but provides interface for future enhancements
func (p *ChromeDPPool) ReleaseBrowser(ctx context.Context) {
	// With round-robin allocation, no specific release action is needed
	// This method is kept for interface compatibility and future enhancements
	p.logger.Debug().Msg("Browser context released to pool")
}

// ShutdownBrowserPool cleans up all browser instances in the pool
func (p *ChromeDPPool) ShutdownBrowserPool() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		p.logger.Debug().Msg("Browser pool already shut down or never initialized")
		return nil
	}

	startTime := time.Now()
	browserCount := len(p.browsers)

	p.logger.Info().
		Int("browser_count", browserCount).
		Msg("Shutting down ChromeDP browser pool")

	// Cleanup with timeout to prevent hanging
	done := make(chan struct{})
	go func() {
		p.cleanupInstances()
		close(done)
	}()

	// Wait for cleanup with timeout
	select {
	case <-done:
		// Cleanup completed successfully
	case <-time.After(30 * time.Second):
		p.logger.Warn().
			Int("browser_count", browserCount).
			Msg("Browser pool shutdown timed out, forcing cleanup")
		// Force cleanup anyway
		p.cleanupInstances()
	}

	p.initialized = false
	duration := time.Since(startTime)

	p.logger.Info().
		Int("browsers_shutdown", browserCount).
		Dur("shutdown_time", duration).
		Msg("ChromeDP browser pool shut down successfully")

	return nil
}

// cleanupInstances cleans up all browser instances (must be called with mutex held)
func (p *ChromeDPPool) cleanupInstances() {
	// Cancel all browser contexts
	for i, cancel := range p.browserCancels {
		if cancel != nil {
			cancel()
			p.logger.Debug().
				Int("browser_index", i).
				Msg("Browser context cancelled")
		}
	}

	// Cancel all allocator contexts
	for i, cancel := range p.allocatorCancels {
		if cancel != nil {
			cancel()
			p.logger.Debug().
				Int("browser_index", i).
				Msg("Browser allocator cancelled")
		}
	}

	// Clear the pools
	p.browsers = nil
	p.browserCancels = nil
	p.allocatorCancels = nil
	p.currentIndex = 0
}

// GetPoolStats returns statistics about the browser pool
func (p *ChromeDPPool) GetPoolStats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	return map[string]interface{}{
		"max_instances":    p.maxInstances,
		"active_instances": len(p.browsers),
		"initialized":      p.initialized,
		"current_index":    p.currentIndex,
	}
}

// IsInitialized returns whether the browser pool has been initialized
func (p *ChromeDPPool) IsInitialized() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initialized
}
