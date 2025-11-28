package ui

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// githubTestContext holds shared state for GitHub job tests
type githubTestContext struct {
	t           *testing.T
	env         *common.TestEnvironment
	ctx         context.Context
	jobsURL     string
	queueURL    string
	connectorID string
}

// newGitHubTestContext creates a new test context with browser and environment
func newGitHubTestContext(t *testing.T, timeout time.Duration) (*githubTestContext, func()) {
	// Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	baseURL := env.GetBaseURL()

	gtc := &githubTestContext{
		t:        t,
		env:      env,
		ctx:      browserCtx,
		jobsURL:  baseURL + "/jobs",
		queueURL: baseURL + "/queue",
	}

	// Return cleanup function
	cleanup := func() {
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return gtc, cleanup
}

// createGitHubConnector creates a GitHub connector for tests using the token from .env.test
func (gtc *githubTestContext) createGitHubConnector() error {
	token := gtc.env.EnvVars["github_test_token"]
	if token == "" {
		return fmt.Errorf("github_test_token not found in .env.test")
	}

	gtc.env.LogTest(gtc.t, "Creating GitHub connector...")

	// Create connector via API
	helper := gtc.env.NewHTTPTestHelper(gtc.t)

	body := map[string]interface{}{
		"name": "Test GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": token,
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("connector creation failed with status: %d", resp.StatusCode)
	}

	var connector map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &connector); err != nil {
		return fmt.Errorf("failed to parse connector response: %w", err)
	}

	connectorID, ok := connector["id"].(string)
	if !ok {
		return fmt.Errorf("connector ID not found in response")
	}

	gtc.connectorID = connectorID
	gtc.env.LogTest(gtc.t, "✓ Created GitHub connector: %s", connectorID)

	// Store connector ID in KV store for job definitions to use
	kvBody := map[string]string{
		"value":       connectorID,
		"description": "GitHub connector for tests",
	}
	resp, err = helper.PUT("/api/kv/github_connector_id", kvBody)
	if err != nil {
		return fmt.Errorf("failed to store connector ID in KV: %w", err)
	}
	defer resp.Body.Close()

	gtc.env.LogTest(gtc.t, "✓ Stored connector ID in KV store as github_connector_id")

	return nil
}

// triggerJob triggers a job by name via the Jobs page UI
func (gtc *githubTestContext) triggerJob(jobName string) error {
	gtc.env.LogTest(gtc.t, "Triggering job: %s", jobName)

	// Navigate to Jobs page
	if err := chromedp.Run(gtc.ctx, chromedp.Navigate(gtc.jobsURL)); err != nil {
		return fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(gtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
	); err != nil {
		return fmt.Errorf("jobs page did not load: %w", err)
	}

	// Take screenshot of jobs page before clicking
	screenshotName := fmt.Sprintf("jobs_page_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
	if err := gtc.env.TakeScreenshot(gtc.ctx, screenshotName); err != nil {
		gtc.env.LogTest(gtc.t, "Failed to take jobs page screenshot: %v", err)
	}

	// Convert job name to button ID format
	// Must match Alpine.js logic: jobDef.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-run'
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	buttonID = buttonID + "-run"

	gtc.env.LogTest(gtc.t, "Looking for button with ID: %s", buttonID)

	// Click the run button by ID
	runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
	if err := chromedp.Run(gtc.ctx,
		chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
		chromedp.Click(runBtnSelector, chromedp.ByQuery),
	); err != nil {
		gtc.env.TakeScreenshot(gtc.ctx, "run_click_failed_"+jobName)
		return fmt.Errorf("failed to click run button for %s (selector: %s): %w", jobName, runBtnSelector, err)
	}

	// Handle Confirmation Modal
	gtc.env.LogTest(gtc.t, "Waiting for confirmation modal")
	if err := chromedp.Run(gtc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for animation
	); err != nil {
		gtc.env.TakeScreenshot(gtc.ctx, "modal_wait_failed_"+jobName)
		return fmt.Errorf("confirmation modal did not appear for %s: %w", jobName, err)
	}

	// Take screenshot of confirmation modal
	modalScreenshotName := fmt.Sprintf("confirmation_modal_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
	if err := gtc.env.TakeScreenshot(gtc.ctx, modalScreenshotName); err != nil {
		gtc.env.LogTest(gtc.t, "Failed to take modal screenshot: %v", err)
	}

	gtc.env.LogTest(gtc.t, "Confirming run")
	// Click Confirm button (primary button in modal footer)
	if err := chromedp.Run(gtc.ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for action to register
	); err != nil {
		gtc.env.TakeScreenshot(gtc.ctx, "confirm_click_failed_"+jobName)
		return fmt.Errorf("failed to confirm run for %s: %w", jobName, err)
	}

	gtc.env.LogTest(gtc.t, "✓ Job triggered: %s", jobName)
	return nil
}

// monitorJobResult holds the results of monitoring a job
type monitorJobResult struct {
	Status        string
	DocumentCount int
	JobID         string
}

// getJobDocumentCount fetches the document count for a job via API
func (gtc *githubTestContext) getJobDocumentCount(jobID string) (int, error) {
	helper := gtc.env.NewHTTPTestHelper(gtc.t)
	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to get job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("job fetch failed with status: %d", resp.StatusCode)
	}

	var job map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		return 0, fmt.Errorf("failed to parse job response: %w", err)
	}

	// Check document_count field
	if docCount, ok := job["document_count"].(float64); ok {
		return int(docCount), nil
	}

	return 0, nil
}

// monitorJob monitors a job on the Queue page until completion
func (gtc *githubTestContext) monitorJob(jobName string, timeout time.Duration, expectDocs bool) error {
	gtc.env.LogTest(gtc.t, "Monitoring job: %s (timeout: %v, expectDocs: %v)", jobName, timeout, expectDocs)

	// Check context before starting
	if err := gtc.ctx.Err(); err != nil {
		return fmt.Errorf("context already cancelled before monitoring: %w", err)
	}

	// Navigate to Queue page
	if err := chromedp.Run(gtc.ctx, chromedp.Navigate(gtc.queueURL)); err != nil {
		return fmt.Errorf("failed to navigate to queue page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(gtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
	); err != nil {
		return fmt.Errorf("queue page did not load: %w", err)
	}

	gtc.env.LogTest(gtc.t, "Queue page loaded, looking for job...")

	// Poll for job to appear in the queue and get its ID
	var jobID string
	pollErr := chromedp.Run(gtc.ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return null;
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return null;
					const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
					return job ? job.id : null;
				})()
			`, jobName),
			&jobID,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if pollErr != nil || jobID == "" {
		gtc.env.TakeScreenshot(gtc.ctx, "job_not_found_"+jobName)
		return fmt.Errorf("job %s not found in queue after 30s: %w", jobName, pollErr)
	}
	gtc.env.LogTest(gtc.t, "✓ Job found in queue (ID: %s)", jobID)

	// Actively monitor job status changes
	gtc.env.LogTest(gtc.t, "Actively monitoring job status...")

	startTime := time.Now()
	lastStatus := ""
	checkCount := 0
	lastProgressLog := time.Now()
	var currentStatus string
	pollStart := time.Now()

	for {
		// Check if context is cancelled
		if err := gtc.ctx.Err(); err != nil {
			gtc.env.LogTest(gtc.t, "  Context cancelled during monitoring: %v", err)
			return fmt.Errorf("context cancelled during monitoring (checks: %d, last status: %s): %w", checkCount, lastStatus, err)
		}

		// Check if we've exceeded the timeout
		if time.Since(pollStart) > timeout {
			gtc.env.TakeScreenshot(gtc.ctx, "job_not_completed_"+jobName)
			return fmt.Errorf("job %s did not complete within %v (last status: %s, checks: %d): timeout", jobName, timeout, lastStatus, checkCount)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			gtc.env.LogTest(gtc.t, "  [%v] Still monitoring... (status: %s, checks: %d)", elapsed.Round(time.Second), lastStatus, checkCount)
			lastProgressLog = time.Now()
		}

		// Trigger a data refresh
		if err := chromedp.Run(gtc.ctx,
			chromedp.Evaluate(`
				(() => {
					if (typeof loadJobs === 'function') {
						loadJobs();
					}
				})()
			`, nil),
		); err != nil {
			gtc.env.LogTest(gtc.t, "  Warning: Failed to trigger data refresh: %v", err)
		}

		// Wait for page to update with fresh data
		time.Sleep(200 * time.Millisecond)

		// Get current status from DOM
		err := chromedp.Run(gtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge) {
								return statusBadge.getAttribute('data-status');
							}
						}
					}
					return null;
				})()
			`, jobName), &currentStatus),
		)

		checkCount++

		if err != nil {
			if gtc.ctx.Err() != nil {
				return fmt.Errorf("context cancelled while checking status (checks: %d): %w", checkCount, gtc.ctx.Err())
			}
			gtc.env.TakeScreenshot(gtc.ctx, "status_check_failed_"+jobName)
			return fmt.Errorf("failed to check job status: %w", err)
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			if lastStatus == "" {
				gtc.env.LogTest(gtc.t, "  Initial status: %s (at %v)", currentStatus, elapsed.Round(time.Millisecond))
			} else {
				gtc.env.LogTest(gtc.t, "  Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Millisecond))
			}
			lastStatus = currentStatus

			// Take screenshot on status change
			screenshotName := fmt.Sprintf("status_%s_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"), currentStatus)
			gtc.env.TakeScreenshot(gtc.ctx, screenshotName)
		}

		// Check if job is done
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			gtc.env.LogTest(gtc.t, "✓ Job reached terminal status: %s (after %d checks)", currentStatus, checkCount)
			break
		}

		// Wait before next check
		time.Sleep(500 * time.Millisecond)
	}

	gtc.env.LogTest(gtc.t, "✓ Final job status: %s", currentStatus)

	// Take final screenshot
	gtc.env.TakeScreenshot(gtc.ctx, fmt.Sprintf("final_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_")))

	// Verify document count via API if expectDocs is true
	if expectDocs {
		gtc.env.LogTest(gtc.t, "Verifying document count for job %s...", jobID)
		docCount, err := gtc.getJobDocumentCount(jobID)
		if err != nil {
			return fmt.Errorf("failed to get document count: %w", err)
		}
		gtc.env.LogTest(gtc.t, "  Document count: %d", docCount)

		if docCount == 0 {
			return fmt.Errorf("job %s completed but document_count is 0 (expected > 0)", jobName)
		}
		gtc.env.LogTest(gtc.t, "✓ Document count verified: %d documents collected", docCount)
	}

	return nil
}

// TestGitHubRepoCollector tests the GitHub Repository Collector job via UI
func TestGitHubRepoCollector(t *testing.T) {
	gtc, cleanup := newGitHubTestContext(t, 5*time.Minute)
	defer cleanup()

	gtc.env.LogTest(t, "--- Starting Test: GitHub Repository Collector ---")

	// Create GitHub connector
	if err := gtc.createGitHubConnector(); err != nil {
		t.Fatalf("Failed to create GitHub connector: %v", err)
	}

	jobName := "GitHub Repository Collector"

	// Take screenshot before triggering job
	if err := chromedp.Run(gtc.ctx, chromedp.Navigate(gtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := gtc.env.TakeScreenshot(gtc.ctx, "github_repo_before"); err != nil {
		gtc.env.LogTest(gtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := gtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job (3 minute timeout for repo fetching)
	if err := gtc.monitorJob(jobName, 3*time.Minute, true); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	gtc.env.LogTest(t, "✓ Test completed successfully")
}

// TestGitHubActionsCollector tests the GitHub Actions Log Collector job via UI
func TestGitHubActionsCollector(t *testing.T) {
	gtc, cleanup := newGitHubTestContext(t, 5*time.Minute)
	defer cleanup()

	gtc.env.LogTest(t, "--- Starting Test: GitHub Actions Log Collector ---")

	// Create GitHub connector
	if err := gtc.createGitHubConnector(); err != nil {
		t.Fatalf("Failed to create GitHub connector: %v", err)
	}

	jobName := "GitHub Actions Log Collector"

	// Take screenshot before triggering job
	if err := chromedp.Run(gtc.ctx, chromedp.Navigate(gtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := gtc.env.TakeScreenshot(gtc.ctx, "github_actions_before"); err != nil {
		gtc.env.LogTest(gtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := gtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job (3 minute timeout for actions fetching)
	if err := gtc.monitorJob(jobName, 3*time.Minute, true); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	gtc.env.LogTest(t, "✓ Test completed successfully")
}
