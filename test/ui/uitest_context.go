// uitest_context.go - Shared UI test context and helpers for Quaero
// This provides UITestContext and helper functions used by all UI tests.
// NOTE: This is NOT a test file - it contains shared test infrastructure.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// Job/Queue test constants
const (
	// MaxJobTestTimeout is the maximum timeout for all job/queue tests (10 minutes)
	MaxJobTestTimeout = 10 * time.Minute

	// ScreenshotInterval is the interval for periodic screenshots during job monitoring
	ScreenshotInterval = 30 * time.Second

	// ProgressLogInterval is the interval for logging progress during job monitoring
	ProgressLogInterval = 10 * time.Second
)

// UITestContext holds shared state for UI tests
// This replaces the separate queueTestContext, codebaseTestContext, etc.
type UITestContext struct {
	T       *testing.T
	Env     *common.TestEnvironment
	Ctx     context.Context
	BaseURL string

	// Common page URLs
	JobsURL     string
	QueueURL    string
	DocsURL     string
	SettingsURL string

	// Internal cleanup functions
	cleanup []func()

	// Screenshot counter for sequential naming
	screenshotNum int
}

// NewUITestContext creates a new UI test context with browser and environment
func NewUITestContext(t *testing.T, timeout time.Duration) *UITestContext {
	// Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	// Create allocator context with headless Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	baseURL := env.GetBaseURL()

	utc := &UITestContext{
		T:           t,
		Env:         env,
		Ctx:         browserCtx,
		BaseURL:     baseURL,
		JobsURL:     baseURL + "/jobs",
		QueueURL:    baseURL + "/queue",
		DocsURL:     baseURL + "/documents",
		SettingsURL: baseURL + "/settings",
		cleanup:     make([]func(), 0),
	}

	// Add cleanup functions in reverse order (LIFO)
	utc.cleanup = append(utc.cleanup, func() { env.Cleanup() })
	utc.cleanup = append(utc.cleanup, func() { cancelTimeout() })
	utc.cleanup = append(utc.cleanup, func() { cancelAlloc() })
	utc.cleanup = append(utc.cleanup, func() { cancelBrowser() })
	utc.cleanup = append(utc.cleanup, func() {
		if err := chromedp.Cancel(browserCtx); err != nil {
			t.Logf("Warning: browser cancel returned: %v", err)
		}
	})

	return utc
}

// Cleanup releases all resources. Call this with defer.
func (utc *UITestContext) Cleanup() {
	// Write test result to log file before cleanup
	// This ensures PASS/FAIL status is captured in test.log
	if utc.T.Failed() {
		utc.Log("=== TEST RESULT: FAIL ===")
	} else {
		utc.Log("=== TEST RESULT: PASS ===")
	}

	// Execute cleanup functions in reverse order
	for i := len(utc.cleanup) - 1; i >= 0; i-- {
		utc.cleanup[i]()
	}
}

// Log writes a message to the test log
func (utc *UITestContext) Log(format string, args ...interface{}) {
	utc.Env.LogTest(utc.T, format, args...)
}

// Screenshot takes a full page screenshot with a sequential number prefix
func (utc *UITestContext) Screenshot(name string) error {
	utc.screenshotNum++
	fullName := fmt.Sprintf("%02d_%s", utc.screenshotNum, name)
	return TakeFullScreenshotInDir(utc.Ctx, utc.Env.ResultsDir, fullName)
}

// FullScreenshot takes a full page screenshot with sequential number prefix
func (utc *UITestContext) FullScreenshot(name string) error {
	utc.screenshotNum++
	fullName := fmt.Sprintf("%02d_%s", utc.screenshotNum, name)
	return TakeFullScreenshotInDir(utc.Ctx, utc.Env.ResultsDir, fullName)
}

// Navigate navigates to a URL and waits for the page title
func (utc *UITestContext) Navigate(url string) error {
	if err := chromedp.Run(utc.Ctx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}
	// Wait for page to load
	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		return fmt.Errorf("page did not load at %s: %w", url, err)
	}
	return nil
}

// WaitForElement waits for an element to be visible
func (utc *UITestContext) WaitForElement(selector string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(utc.Ctx, timeout)
	defer cancel()
	return chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery))
}

// Click clicks an element
func (utc *UITestContext) Click(selector string) error {
	return chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// GetText gets the text content of an element
func (utc *UITestContext) GetText(selector string) (string, error) {
	var text string
	err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.Text(selector, &text, chromedp.ByQuery),
	)
	return text, err
}

// JobNameToButtonID converts a job name to the button ID format used in the UI
func JobNameToButtonID(jobName string) string {
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	return buttonID + "-run"
}

// TriggerJob triggers a job by name via the Jobs page UI
func (utc *UITestContext) TriggerJob(jobName string) error {
	utc.Log("Triggering job: %s", jobName)

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		return err
	}

	// Wait for Alpine.js to load jobs
	time.Sleep(2 * time.Second)
	utc.Screenshot("jobs_page_" + sanitizeName(jobName))

	// Click the run button by ID
	buttonID := JobNameToButtonID(jobName)
	runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
	utc.Log("Looking for button with ID: %s", buttonID)

	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
		chromedp.Click(runBtnSelector, chromedp.ByQuery),
	); err != nil {
		utc.Screenshot("run_click_failed_" + sanitizeName(jobName))
		return fmt.Errorf("failed to click run button for %s (selector: %s): %w", jobName, runBtnSelector, err)
	}

	// Handle Confirmation Modal
	utc.Log("Waiting for confirmation modal")
	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		utc.Screenshot("modal_wait_failed_" + sanitizeName(jobName))
		return fmt.Errorf("confirmation modal did not appear for %s: %w", jobName, err)
	}
	utc.Screenshot("confirmation_modal_" + sanitizeName(jobName))

	// Click Confirm button
	utc.Log("Confirming run")
	if err := chromedp.Run(utc.Ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		utc.Screenshot("confirm_click_failed_" + sanitizeName(jobName))
		return fmt.Errorf("failed to confirm run for %s: %w", jobName, err)
	}

	utc.Log("✓ Job triggered: %s", jobName)
	return nil
}

// sanitizeName converts a name to a safe filename format
func sanitizeName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "_")
}

// MonitorJobOptions configures job monitoring behavior
type MonitorJobOptions struct {
	Timeout              time.Duration // How long to wait for job completion
	ExpectDocuments      bool          // Whether to expect documents > 0
	ValidateAllProcessed bool          // Whether to require completed + failed = total
	AllowFailure         bool          // If true, don't fail test if job fails
}

// DefaultMonitorOptions returns sensible defaults for monitoring
func DefaultMonitorOptions(timeout time.Duration) MonitorJobOptions {
	return MonitorJobOptions{
		Timeout:              timeout,
		ExpectDocuments:      true,
		ValidateAllProcessed: false,
		AllowFailure:         false,
	}
}

// MonitorJob monitors a job on the Queue page until completion
func (utc *UITestContext) MonitorJob(jobName string, opts MonitorJobOptions) error {
	utc.Log("Monitoring job: %s (timeout: %v)", jobName, opts.Timeout)

	// Check context before starting
	if err := utc.Ctx.Err(); err != nil {
		return fmt.Errorf("context already cancelled before monitoring: %w", err)
	}

	// Navigate to Queue page
	if err := utc.Navigate(utc.QueueURL); err != nil {
		return err
	}

	// Wait for Alpine.js to load
	time.Sleep(2 * time.Second)
	utc.Log("Queue page loaded, looking for job...")

	// Poll for job to appear in the queue
	var jobFound bool
	pollErr := chromedp.Run(utc.Ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return false;
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return false;
					return component.allJobs.some(j => j.name && j.name.includes('%s'));
				})()
			`, jobName),
			&jobFound,
			chromedp.WithPollingTimeout(10*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if pollErr != nil {
		utc.Screenshot("job_not_found_" + sanitizeName(jobName))
		return fmt.Errorf("job %s not found in queue after 10s: %w", jobName, pollErr)
	}
	utc.Log("✓ Job found in queue")

	// Monitor job status
	return utc.pollJobStatus(jobName, opts)
}

// pollJobStatus actively monitors job status until completion
func (utc *UITestContext) pollJobStatus(jobName string, opts MonitorJobOptions) error {
	startTime := time.Now()
	lastStatus := ""
	checkCount := 0
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()
	var currentStatus string

	for {
		// Check if context is cancelled
		if err := utc.Ctx.Err(); err != nil {
			utc.Log("Context cancelled during monitoring: %v", err)
			return fmt.Errorf("context cancelled (checks: %d, last status: %s): %w", checkCount, lastStatus, err)
		}

		// Check timeout
		if time.Since(startTime) > opts.Timeout {
			utc.Screenshot("job_timeout_" + sanitizeName(jobName))
			return fmt.Errorf("job %s did not complete within %v (last status: %s)", jobName, opts.Timeout, lastStatus)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			utc.Log("[%v] Still monitoring... (status: %s, checks: %d)", elapsed.Round(time.Second), lastStatus, checkCount)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			utc.FullScreenshot(fmt.Sprintf("monitor_%s_%ds", sanitizeName(jobName), int(elapsed.Seconds())))
			lastScreenshotTime = time.Now()
		}

		// Trigger data refresh
		chromedp.Run(utc.Ctx, chromedp.Evaluate(`
			(() => { if (typeof loadJobs === 'function') { loadJobs(); } })()
		`, nil))
		time.Sleep(200 * time.Millisecond)

		// Get current status from DOM
		err := chromedp.Run(utc.Ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge) return statusBadge.getAttribute('data-status');
						}
					}
					return null;
				})()
			`, jobName), &currentStatus),
		)
		checkCount++

		if err != nil {
			if utc.Ctx.Err() != nil {
				return fmt.Errorf("context cancelled while checking status: %w", utc.Ctx.Err())
			}
			utc.Screenshot("status_check_failed_" + sanitizeName(jobName))
			return fmt.Errorf("failed to check job status: %w", err)
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			if lastStatus == "" {
				utc.Log("Initial status: %s (at %v)", currentStatus, elapsed.Round(time.Millisecond))
			} else {
				utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Millisecond))
			}
			lastStatus = currentStatus
			utc.FullScreenshot(fmt.Sprintf("status_%s_%s", sanitizeName(jobName), currentStatus))
		}

		// Check terminal states
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("✓ Job reached terminal status: %s (after %d checks)", currentStatus, checkCount)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Handle failure
	if currentStatus == "failed" && !opts.AllowFailure {
		reason := utc.getJobFailureReason(jobName)
		if reason != "" {
			return fmt.Errorf("job %s failed: %s", jobName, reason)
		}
		return fmt.Errorf("job %s failed (no failure reason found)", jobName)
	}

	utc.Log("✓ Final job status: %s", currentStatus)
	return nil
}

// getJobFailureReason extracts the failure reason from the UI
func (utc *UITestContext) getJobFailureReason(jobName string) string {
	var reason string
	chromedp.Run(utc.Ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const errorAlert = card.querySelector('.job-error-alert');
						if (errorAlert) {
							const text = errorAlert.textContent;
							const match = text.match(/Failure Reason:\\s*(.+)/);
							if (match) return match[1].trim();
							return errorAlert.textContent.trim();
						}
					}
				}
				return '';
			})()
		`, jobName), &reason),
	)
	return reason
}

// TriggerAndMonitorJob is a convenience method that triggers and monitors a job
func (utc *UITestContext) TriggerAndMonitorJob(jobName string, timeout time.Duration) error {
	if err := utc.TriggerJob(jobName); err != nil {
		return err
	}
	return utc.MonitorJob(jobName, DefaultMonitorOptions(timeout))
}

// JobDefinitionTestConfig configures a job definition end-to-end test
type JobDefinitionTestConfig struct {
	JobName           string        // Name as shown in UI (e.g., "News Crawler")
	JobDefinitionPath string        // Path to TOML file (relative to test/ui/)
	Timeout           time.Duration // Max time to wait for job completion
	RequiredEnvVars   []string      // Env vars that must be set (skip if missing)
	AllowFailure      bool          // If true, don't fail test if job fails
}

// SaveToResults saves content to a file in the test results directory
func (utc *UITestContext) SaveToResults(filename string, content string) error {
	destPath := filepath.Join(utc.Env.ResultsDir, filename)
	if err := os.WriteFile(destPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save %s to results: %w", filename, err)
	}
	utc.Log("✓ Saved to results: %s", destPath)
	return nil
}

// CopyJobDefinitionToResults copies the job definition TOML to test results directory
func (utc *UITestContext) CopyJobDefinitionToResults(jobDefPath string) error {
	utc.Log("Copying job definition: %s", jobDefPath)

	// Resolve absolute path from relative path
	testUIDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	sourcePath := filepath.Join(testUIDir, jobDefPath)

	// Open source file
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open job definition file %s: %w", sourcePath, err)
	}
	defer srcFile.Close()

	// Create destination path in results directory
	fileName := filepath.Base(jobDefPath)
	destPath := filepath.Join(utc.Env.ResultsDir, fileName)

	// Create destination file
	dstFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy job definition: %w", err)
	}

	utc.Log("✓ Job definition copied to: %s", destPath)
	return nil
}

// RefreshAndScreenshot refreshes the page and takes a screenshot
func (utc *UITestContext) RefreshAndScreenshot(name string) error {
	utc.Log("Refreshing page and taking screenshot: %s", name)

	// Get current URL
	var currentURL string
	if err := chromedp.Run(utc.Ctx, chromedp.Location(&currentURL)); err != nil {
		return fmt.Errorf("failed to get current URL: %w", err)
	}

	// Navigate to current URL (refresh)
	if err := chromedp.Run(utc.Ctx, chromedp.Navigate(currentURL)); err != nil {
		return fmt.Errorf("failed to refresh page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		return fmt.Errorf("page did not load after refresh: %w", err)
	}

	// Take screenshot
	if err := utc.FullScreenshot(name); err != nil {
		return fmt.Errorf("failed to take screenshot after refresh: %w", err)
	}

	utc.Log("✓ Page refreshed and screenshot taken")
	return nil
}

// RunJobDefinitionTest runs a complete job definition test with monitoring and screenshots
func (utc *UITestContext) RunJobDefinitionTest(config JobDefinitionTestConfig) error {
	utc.Log("Starting job definition test: %s", config.JobName)

	// Check required environment variables
	if len(config.RequiredEnvVars) > 0 {
		missingVars := make([]string, 0)
		for _, envVar := range config.RequiredEnvVars {
			if os.Getenv(envVar) == "" {
				missingVars = append(missingVars, envVar)
			}
		}
		if len(missingVars) > 0 {
			utc.Log("Skipping test: missing required environment variables: %v", missingVars)
			utc.T.Skipf("Missing required environment variables: %v", missingVars)
			return nil
		}
	}

	// Copy job definition to results directory
	if err := utc.CopyJobDefinitionToResults(config.JobDefinitionPath); err != nil {
		return fmt.Errorf("failed to copy job definition: %w", err)
	}

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		return fmt.Errorf("failed to navigate to Jobs page: %w", err)
	}

	// Wait for page to fully load
	time.Sleep(2 * time.Second)

	// Take job definition screenshot
	if err := utc.Screenshot("job_definition"); err != nil {
		return fmt.Errorf("failed to take job definition screenshot: %w", err)
	}

	// Trigger the job
	if err := utc.TriggerJob(config.JobName); err != nil {
		return fmt.Errorf("failed to trigger job: %w", err)
	}

	// Monitor the job until completion
	monitorOpts := MonitorJobOptions{
		Timeout:              config.Timeout,
		ExpectDocuments:      false,
		ValidateAllProcessed: false,
		AllowFailure:         config.AllowFailure,
	}
	if err := utc.MonitorJob(config.JobName, monitorOpts); err != nil {
		return fmt.Errorf("failed to monitor job: %w", err)
	}

	// Refresh page and take final screenshot
	if err := utc.RefreshAndScreenshot("final_state"); err != nil {
		return fmt.Errorf("failed to refresh and screenshot: %w", err)
	}

	utc.Log("✓ Job definition test completed: %s", config.JobName)
	return nil
}

// =============================================================================
// Shared Test Tracker Types and Helper Functions
// =============================================================================

// WebSocketMessageTracker tracks WebSocket messages by type
type WebSocketMessageTracker struct {
	mu                        sync.Mutex
	refreshLogsMessages       []map[string]interface{} // All refresh_logs messages
	jobScopedRefreshCount     int                      // Count of job-scoped refresh_logs
	serviceScopedRefreshCount int                      // Count of service-scoped refresh_logs
	jobScopedStepIDTotal      int                      // Total step_ids observed in job-scoped refresh_logs payloads
	jobScopedReceivedAt       []time.Time              // Local receive times for job-scoped refresh_logs
	serviceScopedReceivedAt   []time.Time              // Local receive times for service-scoped refresh_logs
}

// NewWebSocketMessageTracker creates a new WebSocket message tracker
func NewWebSocketMessageTracker() *WebSocketMessageTracker {
	return &WebSocketMessageTracker{
		refreshLogsMessages:     make([]map[string]interface{}, 0),
		jobScopedReceivedAt:     make([]time.Time, 0),
		serviceScopedReceivedAt: make([]time.Time, 0),
	}
}

// AddRefreshLogs records a refresh_logs WebSocket message (notify-pull trigger).
func (t *WebSocketMessageTracker) AddRefreshLogs(payload map[string]interface{}, receivedAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.refreshLogsMessages = append(t.refreshLogsMessages, payload)
	scope, _ := payload["scope"].(string)
	switch scope {
	case "job":
		t.jobScopedRefreshCount++
		t.jobScopedReceivedAt = append(t.jobScopedReceivedAt, receivedAt)
		if stepIDs, ok := payload["step_ids"].([]interface{}); ok {
			t.jobScopedStepIDTotal += len(stepIDs)
		}
	case "service":
		t.serviceScopedRefreshCount++
		t.serviceScopedReceivedAt = append(t.serviceScopedReceivedAt, receivedAt)
	}
}

// GetRefreshLogsCount returns the total count of refresh_logs messages
func (t *WebSocketMessageTracker) GetRefreshLogsCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.refreshLogsMessages)
}

// GetJobScopedRefreshCount returns count of job-scoped refresh_logs messages
func (t *WebSocketMessageTracker) GetJobScopedRefreshCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.jobScopedRefreshCount
}

// GetServiceScopedRefreshCount returns count of service-scoped refresh_logs messages
func (t *WebSocketMessageTracker) GetServiceScopedRefreshCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.serviceScopedRefreshCount
}

// GetJobScopedRefreshStepIDTotal returns the total number of step_ids seen in job-scoped refresh_logs triggers.
func (t *WebSocketMessageTracker) GetJobScopedRefreshStepIDTotal() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.jobScopedStepIDTotal
}

// GetJobScopedRefreshCountBefore returns count of job-scoped refresh_logs before deadline
func (t *WebSocketMessageTracker) GetJobScopedRefreshCountBefore(deadline time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, ts := range t.jobScopedReceivedAt {
		if ts.Before(deadline) {
			n++
		}
	}
	return n
}

// GetServiceScopedRefreshCountBefore returns count of service-scoped refresh_logs before deadline
func (t *WebSocketMessageTracker) GetServiceScopedRefreshCountBefore(deadline time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, ts := range t.serviceScopedReceivedAt {
		if ts.Before(deadline) {
			n++
		}
	}
	return n
}

// APILogsCall represents a single /api/logs call
type APILogsCall struct {
	Scope      string
	URL        string
	JobID      string
	ReceivedAt time.Time
}

// APICallTracker tracks /api/logs calls so we can assert they are gated by WebSocket refresh_logs triggers.
type APICallTracker struct {
	mu           sync.Mutex
	logsCalls    []APILogsCall
	jobLogsCalls int
	svcLogsCalls int
}

// NewAPICallTracker creates a new API call tracker
func NewAPICallTracker() *APICallTracker {
	return &APICallTracker{
		logsCalls: make([]APILogsCall, 0),
	}
}

// AddRequest records an API request
func (t *APICallTracker) AddRequest(requestURL string, receivedAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !strings.Contains(requestURL, "/api/logs") {
		return
	}

	scope := ""
	jobID := ""
	if u, err := url.Parse(requestURL); err == nil {
		scope = u.Query().Get("scope")
		jobID = u.Query().Get("job_id")
	}

	t.logsCalls = append(t.logsCalls, APILogsCall{
		Scope:      scope,
		URL:        requestURL,
		JobID:      jobID,
		ReceivedAt: receivedAt,
	})

	switch scope {
	case "job":
		t.jobLogsCalls++
	case "service":
		t.svcLogsCalls++
	}
}

// GetJobLogsCalls returns count of job logs calls
func (t *APICallTracker) GetJobLogsCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.jobLogsCalls
}

// GetServiceLogsCalls returns count of service logs calls
func (t *APICallTracker) GetServiceLogsCalls() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.svcLogsCalls
}

// GetJobLogsCallsBefore returns count of job logs calls before deadline
func (t *APICallTracker) GetJobLogsCallsBefore(deadline time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, c := range t.logsCalls {
		if c.Scope == "job" && c.ReceivedAt.Before(deadline) {
			n++
		}
	}
	return n
}

// GetServiceLogsCallsBefore returns count of service logs calls before deadline
func (t *APICallTracker) GetServiceLogsCallsBefore(deadline time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, c := range t.logsCalls {
		if c.Scope == "service" && c.ReceivedAt.Before(deadline) {
			n++
		}
	}
	return n
}

// StepExpansionTracker tracks step expansion order and log line numbers
type StepExpansionTracker struct {
	mu             sync.Mutex
	expansionOrder []string         // Order steps were expanded
	expandedSteps  map[string]bool  // Currently expanded steps
	stepLogLines   map[string][]int // Step name -> first few log line numbers
}

// NewStepExpansionTracker creates a new step expansion tracker
func NewStepExpansionTracker() *StepExpansionTracker {
	return &StepExpansionTracker{
		expansionOrder: make([]string, 0),
		expandedSteps:  make(map[string]bool),
		stepLogLines:   make(map[string][]int),
	}
}

// RecordExpansion records a step expansion
func (t *StepExpansionTracker) RecordExpansion(stepName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.expandedSteps[stepName] {
		t.expandedSteps[stepName] = true
		t.expansionOrder = append(t.expansionOrder, stepName)
	}
}

// RecordLogLines records log line numbers for a step
func (t *StepExpansionTracker) RecordLogLines(stepName string, lines []int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.stepLogLines[stepName]) == 0 {
		t.stepLogLines[stepName] = lines
	}
}

// GetExpansionOrder returns the order in which steps were expanded
func (t *StepExpansionTracker) GetExpansionOrder() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]string, len(t.expansionOrder))
	copy(result, t.expansionOrder)
	return result
}

// GetLogLines returns the recorded log lines for a step
func (t *StepExpansionTracker) GetLogLines(stepName string) []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stepLogLines[stepName]
}

// StepIconData tracks icon class and status for each step
type StepIconData struct {
	StepName   string
	Status     string
	IconClass  string
	HasSpinner bool
}

// ParentJobIconData tracks icon class for parent job
type ParentJobIconData struct {
	JobName   string
	IconClass string
}

// DOMLogProgressSnapshot captures log progress state from DOM
type DOMLogProgressSnapshot struct {
	ExpandedSteps []string       `json:"expandedSteps"`
	StepLogCounts map[string]int `json:"stepLogCounts"`
	TotalLogLines int            `json:"totalLogLines"`
}

// DOMLogProgressSample is a timestamped progress snapshot
type DOMLogProgressSample struct {
	Elapsed  time.Duration
	Snapshot DOMLogProgressSnapshot
}

// httpGetter interface for API helpers
type httpGetter interface {
	GET(path string) (*http.Response, error)
	Logf(format string, args ...interface{})
}

// apiGetJSON fetches JSON from an API endpoint
func apiGetJSON(t *testing.T, h httpGetter, path string, dest interface{}) error {
	t.Helper()

	resp, err := h.GET(path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("failed to decode %s: %w", path, err)
	}

	return nil
}

// apiJobTreeStep represents a step in the job tree API response
type apiJobTreeStep struct {
	StepID string `json:"step_id,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// apiJobTreeResponse represents the job tree API response
type apiJobTreeResponse struct {
	JobID  string           `json:"job_id"`
	Status string           `json:"status"`
	Steps  []apiJobTreeStep `json:"steps"`
}

// apiJobResponse represents a job API response
type apiJobResponse struct {
	Status string `json:"status"`
}

// apiLogEntry represents a log entry in the logs API response
type apiLogEntry struct {
	LineNumber int    `json:"line_number"`
	Level      string `json:"level"`
	Message    string `json:"message"`
}

// apiJobTreeLogsStep represents a step in the logs API response
type apiJobTreeLogsStep struct {
	StepName   string        `json:"step_name"`
	Logs       []apiLogEntry `json:"logs"`
	TotalCount int           `json:"total_count"`
}

// apiJobTreeLogsResponse represents the logs API response
type apiJobTreeLogsResponse struct {
	Steps []apiJobTreeLogsStep `json:"steps"`
}

// apiDocument represents a document from the API
type apiDocument struct {
	ID              string `json:"id"`
	SourceType      string `json:"source_type"`
	Title           string `json:"title"`
	ContentMarkdown string `json:"content_markdown"`
}

// apiDocumentsResponse represents the documents API response
type apiDocumentsResponse struct {
	Documents  []apiDocument `json:"documents"`
	TotalCount int           `json:"total_count"`
}

// getJobIDFromQueueUI extracts the job ID from the Queue UI using Alpine.js
func getJobIDFromQueueUI(utc *UITestContext, jobName string) (string, error) {
	var jobID string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				if (typeof Alpine === 'undefined') return '';
				const jobListEl = document.querySelector('[x-data="jobList"]');
				if (!jobListEl) return '';
				const component = Alpine.$data(jobListEl);
				if (!component || !component.allJobs) return '';
				const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
				return job ? job.id : '';
			})()
		`, jobName), &jobID),
	)
	if err != nil {
		return "", err
	}
	return jobID, nil
}

// getUIStepStatusMap gets step status map from UI DOM
func getUIStepStatusMap(utc *UITestContext) (map[string]string, error) {
	var stepStatuses map[string]string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepHeader = step.querySelector('.tree-step-header');
					if (!stepHeader) continue;

					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const statusEl = stepHeader.querySelector('.tree-step-status');
					let status = 'unknown';
					if (statusEl) {
						if (statusEl.classList.contains('text-warning')) status = 'pending';
						else if (statusEl.classList.contains('text-primary')) status = 'running';
						else if (statusEl.classList.contains('text-success')) status = 'completed';
						else if (statusEl.classList.contains('text-error')) status = 'failed';
						else if (statusEl.classList.contains('text-gray')) status = 'cancelled';
					}
					result[stepName] = status;
				}
				return result;
			})()
		`, &stepStatuses),
	)
	if err != nil {
		return nil, err
	}
	return stepStatuses, nil
}

// getUIStepLogCountMap gets step log count map from UI DOM
func getUIStepLogCountMap(utc *UITestContext) (map[string]int, error) {
	var stepLogCounts map[string]int
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const logsSection = step.querySelector('.tree-step-logs');
					if (!logsSection) {
						result[stepName] = 0;
						continue;
					}

					const logLines = logsSection.querySelectorAll('.tree-log-line');
					result[stepName] = logLines ? logLines.length : 0;
				}
				return result;
			})()
		`, &stepLogCounts),
	)
	if err != nil {
		return nil, err
	}
	return stepLogCounts, nil
}

// assertAPIParentJobStatusMatchesUI asserts that API job status matches UI
func assertAPIParentJobStatusMatchesUI(t *testing.T, utc *UITestContext, h httpGetter, jobID string, uiStatus string) {
	t.Helper()
	if jobID == "" || uiStatus == "" {
		return
	}

	var job apiJobResponse
	if err := apiGetJSON(t, h, fmt.Sprintf("/api/jobs/%s", jobID), &job); err != nil {
		t.Errorf("FAIL: Could not get parent job status from API for job_id=%s: %v", jobID, err)
		return
	}

	if job.Status != uiStatus {
		utc.Screenshot(fmt.Sprintf("status_mismatch_parent_%s_api_%s_ui_%s", sanitizeName(jobID), job.Status, uiStatus))
		t.Errorf("FAIL: Parent job status mismatch: API=%s UI=%s (job_id=%s)", job.Status, uiStatus, jobID)
	}
}

// assertAPIStepStatusesMatchUI asserts that API step statuses match UI
func assertAPIStepStatusesMatchUI(t *testing.T, utc *UITestContext, h httpGetter, jobID string) {
	t.Helper()
	if jobID == "" {
		return
	}

	var tree apiJobTreeResponse
	if err := apiGetJSON(t, h, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err != nil {
		t.Errorf("FAIL: Could not get step statuses from API for job_id=%s: %v", jobID, err)
		return
	}

	apiStepStatus := make(map[string]string, len(tree.Steps))
	for _, s := range tree.Steps {
		apiStepStatus[s.Name] = s.Status
	}

	uiStepStatus, err := getUIStepStatusMap(utc)
	if err != nil {
		t.Errorf("FAIL: Could not get step statuses from UI DOM: %v", err)
		return
	}

	for stepName, uiStatus := range uiStepStatus {
		apiStatus, ok := apiStepStatus[stepName]
		if !ok {
			t.Errorf("FAIL: Step '%s' present in UI but missing from API tree response (job_id=%s)", stepName, jobID)
			continue
		}
		if apiStatus != uiStatus {
			utc.Screenshot(fmt.Sprintf("status_mismatch_step_%s_api_%s_ui_%s", sanitizeName(stepName), apiStatus, uiStatus))
			t.Errorf("FAIL: Step status mismatch for '%s': API=%s UI=%s (job_id=%s)", stepName, apiStatus, uiStatus, jobID)
		}
	}
}

// captureDOMLogProgressSnapshot captures log progress from DOM
func captureDOMLogProgressSnapshot(utc *UITestContext) (DOMLogProgressSnapshot, error) {
	var snapshot DOMLogProgressSnapshot
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const stepLogCounts = {};
				const expandedSteps = [];
				let totalLogLines = 0;

				const treeSteps = document.querySelectorAll('.tree-step');
				for (const stepEl of treeSteps) {
					const stepNameEl = stepEl.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const logsSection = stepEl.querySelector('.tree-step-logs');
					if (!logsSection) continue; // Not expanded

					const logLines = logsSection.querySelectorAll('.tree-log-line');
					const count = logLines ? logLines.length : 0;
					stepLogCounts[stepName] = count;
					totalLogLines += count;
					if (!expandedSteps.includes(stepName)) {
						expandedSteps.push(stepName);
					}
				}

				return { expandedSteps, stepLogCounts, totalLogLines };
			})()
		`, &snapshot),
	)
	if err != nil {
		return DOMLogProgressSnapshot{}, err
	}
	return snapshot, nil
}
