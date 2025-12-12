// framework_test.go - Unified UI test framework for Quaero
// This provides a shared UITestContext and helper functions for all UI tests.

package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
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
	// Execute cleanup functions in reverse order
	for i := len(utc.cleanup) - 1; i >= 0; i-- {
		utc.cleanup[i]()
	}
}

// Log writes a message to the test log
func (utc *UITestContext) Log(format string, args ...interface{}) {
	utc.Env.LogTest(utc.T, format, args...)
}

// Screenshot takes a screenshot with a sequential number prefix
func (utc *UITestContext) Screenshot(name string) error {
	utc.screenshotNum++
	fullName := fmt.Sprintf("%02d_%s", utc.screenshotNum, name)
	return utc.Env.TakeScreenshot(utc.Ctx, fullName)
}

// FullScreenshot takes a full page screenshot with sequential number prefix
func (utc *UITestContext) FullScreenshot(name string) error {
	utc.screenshotNum++
	fullName := fmt.Sprintf("%02d_%s", utc.screenshotNum, name)
	return utc.Env.TakeFullScreenshot(utc.Ctx, fullName)
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
