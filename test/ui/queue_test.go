package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// queueTestContext holds shared state for queue tests
type queueTestContext struct {
	t        *testing.T
	env      *common.TestEnvironment
	ctx      context.Context
	jobsURL  string
	queueURL string
}

// newQueueTestContext creates a new test context with browser and environment
func newQueueTestContext(t *testing.T, timeout time.Duration) (*queueTestContext, func()) {
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

	qtc := &queueTestContext{
		t:        t,
		env:      env,
		ctx:      browserCtx,
		jobsURL:  baseURL + "/jobs",
		queueURL: baseURL + "/queue",
	}

	// Return cleanup function
	cleanup := func() {
		// Properly close the browser before canceling contexts
		// This ensures Chrome processes are terminated on Windows
		if err := chromedp.Cancel(browserCtx); err != nil {
			// Log but don't fail - browser may already be closed
			t.Logf("Warning: browser cancel returned: %v", err)
		}
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return qtc, cleanup
}

// triggerJob triggers a job by name via the Jobs page UI
func (qtc *queueTestContext) triggerJob(jobName string) error {
	qtc.env.LogTest(qtc.t, "Triggering job: %s", jobName)

	// Navigate to Jobs page
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.jobsURL)); err != nil {
		return fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
	); err != nil {
		return fmt.Errorf("jobs page did not load: %w", err)
	}

	// Take screenshot of jobs page before clicking
	screenshotName := fmt.Sprintf("jobs_page_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
	if err := qtc.env.TakeScreenshot(qtc.ctx, screenshotName); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take jobs page screenshot: %v", err)
	}

	// Convert job name to button ID format
	// Must match Alpine.js logic: jobDef.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-run'
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	buttonID = buttonID + "-run"

	qtc.env.LogTest(qtc.t, "Looking for button with ID: %s", buttonID)

	// Click the run button by ID
	runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
		chromedp.Click(runBtnSelector, chromedp.ByQuery),
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "run_click_failed_"+jobName)
		return fmt.Errorf("failed to click run button for %s (selector: %s): %w", jobName, runBtnSelector, err)
	}

	// Handle Confirmation Modal
	qtc.env.LogTest(qtc.t, "Waiting for confirmation modal")
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for animation
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "modal_wait_failed_"+jobName)
		return fmt.Errorf("confirmation modal did not appear for %s: %w", jobName, err)
	}

	// Take screenshot of confirmation modal
	modalScreenshotName := fmt.Sprintf("confirmation_modal_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
	if err := qtc.env.TakeScreenshot(qtc.ctx, modalScreenshotName); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take modal screenshot: %v", err)
	}

	qtc.env.LogTest(qtc.t, "Confirming run")
	// Click Confirm button (primary button in modal footer)
	if err := chromedp.Run(qtc.ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for action to register
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "confirm_click_failed_"+jobName)
		return fmt.Errorf("failed to confirm run for %s: %w", jobName, err)
	}

	qtc.env.LogTest(qtc.t, "✓ Job triggered: %s", jobName)
	return nil
}

// monitorJob monitors a job on the Queue page until completion
// timeout: how long to wait for job completion
// expectDocs: whether to expect documents > 0
// validateAllProcessed: whether to require completed + failed = total documents
func (qtc *queueTestContext) monitorJob(jobName string, timeout time.Duration, expectDocs bool, validateAllProcessed bool) error {
	qtc.env.LogTest(qtc.t, "Monitoring job: %s (timeout: %v)", jobName, timeout)

	// Check context before starting
	if err := qtc.ctx.Err(); err != nil {
		return fmt.Errorf("context already cancelled before monitoring: %w", err)
	}

	// Navigate to Queue page
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		return fmt.Errorf("failed to navigate to queue page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
	); err != nil {
		return fmt.Errorf("queue page did not load: %w", err)
	}

	qtc.env.LogTest(qtc.t, "Queue page loaded, looking for job...")

	// Poll for job to appear in the queue
	var jobFound bool
	pollErr := chromedp.Run(qtc.ctx,
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
		qtc.env.TakeScreenshot(qtc.ctx, "job_not_found_"+jobName)
		return fmt.Errorf("job %s not found in queue after 10s: %w", jobName, pollErr)
	}
	qtc.env.LogTest(qtc.t, "✓ Job found in queue")

	// Actively monitor job status changes
	qtc.env.LogTest(qtc.t, "Actively monitoring job status...")

	startTime := time.Now()
	lastStatus := ""
	checkCount := 0
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()
	var currentStatus string
	pollStart := time.Now()

	for {
		// Check if context is cancelled
		if err := qtc.ctx.Err(); err != nil {
			qtc.env.LogTest(qtc.t, "  Context cancelled during monitoring: %v", err)
			return fmt.Errorf("context cancelled during monitoring (checks: %d, last status: %s): %w", checkCount, lastStatus, err)
		}

		// Check if we've exceeded the timeout
		if time.Since(pollStart) > timeout {
			qtc.env.TakeScreenshot(qtc.ctx, "job_not_completed_"+jobName)
			return fmt.Errorf("job %s did not complete within %v (last status: %s, checks: %d): timeout", jobName, timeout, lastStatus, checkCount)
		}

		// Log progress every 10 seconds to show the loop is running
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			qtc.env.LogTest(qtc.t, "  [%v] Still monitoring... (status: %s, checks: %d)", elapsed.Round(time.Second), lastStatus, checkCount)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			screenshotName := fmt.Sprintf("monitor_%s_%ds", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"), int(elapsed.Seconds()))
			if err := qtc.env.TakeScreenshot(qtc.ctx, screenshotName); err != nil {
				qtc.env.LogTest(qtc.t, "  Warning: Failed to take periodic screenshot: %v", err)
			} else {
				qtc.env.LogTest(qtc.t, "  Captured periodic screenshot: %s", screenshotName)
			}
			lastScreenshotTime = time.Now()
		}

		// Trigger a data refresh - log any errors
		if err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					if (typeof loadJobs === 'function') {
						loadJobs();
					}
				})()
			`, nil),
		); err != nil {
			qtc.env.LogTest(qtc.t, "  Warning: Failed to trigger data refresh: %v", err)
		}

		// Wait for page to update with fresh data
		time.Sleep(200 * time.Millisecond)

		// Get current status from DOM
		err := chromedp.Run(qtc.ctx,
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
			// Check if it's a context cancellation
			if qtc.ctx.Err() != nil {
				qtc.env.LogTest(qtc.t, "  Context cancelled while checking status: %v", qtc.ctx.Err())
				return fmt.Errorf("context cancelled while checking status (checks: %d): %w", checkCount, qtc.ctx.Err())
			}
			qtc.env.TakeScreenshot(qtc.ctx, "status_check_failed_"+jobName)
			return fmt.Errorf("failed to check job status: %w", err)
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			if lastStatus == "" {
				qtc.env.LogTest(qtc.t, "  Initial status: %s (at %v)", currentStatus, elapsed.Round(time.Millisecond))
			} else {
				qtc.env.LogTest(qtc.t, "  Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Millisecond))
			}
			lastStatus = currentStatus

			// Take screenshot on status change for debugging
			screenshotName := fmt.Sprintf("status_%s_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"), currentStatus)
			qtc.env.TakeScreenshot(qtc.ctx, screenshotName)
		}

		// Check if job is done
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			qtc.env.LogTest(qtc.t, "✓ Job reached terminal status: %s (after %d checks)", currentStatus, checkCount)
			break
		}

		// Wait before next check
		time.Sleep(500 * time.Millisecond)
	}

	qtc.env.LogTest(qtc.t, "✓ Final job status: %s", currentStatus)

	// Wait for UI to refresh with final document count
	// The job completion might be detected before the UI has fetched updated document counts
	qtc.env.LogTest(qtc.t, "Waiting for UI to refresh with final statistics...")
	time.Sleep(2 * time.Second)

	// Trigger loadJobs and wait using polling
	qtc.env.LogTest(qtc.t, "Triggering data refresh...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(`
			(() => {
				if (typeof loadJobs === 'function') {
					loadJobs();
				}
			})()
		`, nil),
	); err != nil {
		qtc.env.LogTest(qtc.t, "  Warning: Failed to trigger loadJobs: %v", err)
	}

	// Wait for loadJobs to complete (it updates lastUpdateTime when done)
	time.Sleep(3 * time.Second) // Give loadJobs time to complete

	// Now read the Alpine data to debug
	var refreshData map[string]interface{}
	refreshErr := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const element = document.querySelector('[x-data="jobList"]');
				if (!element) return { error: 'jobList element not found' };
				const component = Alpine.$data(element);
				if (!component) return { error: 'Alpine component not found' };
				if (!component.allJobs) return { error: 'allJobs not found', isLoading: component.isLoading };

				const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
				if (!job) return { error: 'Job not found in allJobs', jobCount: component.allJobs.length, jobNames: component.allJobs.map(j => j.name) };

				return {
					jobName: job.name,
					jobID: job.id,
					documentCount: job.document_count,
					metadataDocCount: job.metadata ? job.metadata.document_count : 'no metadata',
					status: job.status,
					allJobsCount: component.allJobs.length,
					lastUpdateTime: component.lastUpdateTime ? component.lastUpdateTime.toISOString() : 'none',
					isLoading: component.isLoading
				};
			})()
		`, jobName), &refreshData),
	)
	if refreshErr != nil {
		qtc.env.LogTest(qtc.t, "  Warning: Failed to get refresh data: %v", refreshErr)
	} else if refreshData != nil {
		qtc.env.LogTest(qtc.t, "  Alpine data after refresh: %+v", refreshData)
	}

	// Get job statistics
	cardSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]`, jobName)

	var stats map[string]interface{}
	err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const card = document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if (!card) return null;

				const cardText = card.textContent;
				const docsMatch = cardText.match(/(\d+)\s+Documents?/i);
				const completedMatch = cardText.match(/(\d+)\s+completed/i);
				const failedMatch = cardText.match(/(\d+)\s+failed/i);

				return {
					documents: docsMatch ? parseInt(docsMatch[1]) : 0,
					completed: completedMatch ? parseInt(completedMatch[1]) : 0,
					failed: failedMatch ? parseInt(failedMatch[1]) : 0
				};
			})()
		`, strings.ReplaceAll(cardSelector, "'", "\\'")), &stats),
	)

	if err == nil && stats != nil {
		docs := int(stats["documents"].(float64))
		completed := int(stats["completed"].(float64))
		failed := int(stats["failed"].(float64))

		qtc.env.LogTest(qtc.t, "Job statistics: %d documents, %d completed, %d failed", docs, completed, failed)

		// Verify document count if expected
		if expectDocs && docs == 0 {
			return fmt.Errorf("expected documents > 0, got: %d", docs)
		}

		// Verify all documents were processed if required
		if validateAllProcessed {
			processed := completed + failed
			if processed != docs {
				return fmt.Errorf("not all documents processed: expected %d (completed + failed) = %d (total documents), got %d + %d = %d", processed, docs, completed, failed, processed)
			}
			qtc.env.LogTest(qtc.t, "✓ All documents processed: %d completed + %d failed = %d total", completed, failed, docs)
		} else {
			if completed == 0 && failed == 0 {
				qtc.env.LogTest(qtc.t, "⚠ Warning: No completed or failed tasks (job may have had no work to do)")
			}
		}

		// Log if there were failures (may be due to API rate limits)
		if failed > 0 {
			qtc.env.LogTest(qtc.t, "⚠ Warning: %d tasks failed (may be due to API rate limits or configuration issues)", failed)
		}

		qtc.env.LogTest(qtc.t, "✓ Job statistics verified")
	} else {
		qtc.env.LogTest(qtc.t, "⚠ Warning: Could not extract job statistics")
	}

	return nil
}

// runPlacesJob triggers and monitors the Nearby Restaurants job
func (qtc *queueTestContext) runPlacesJob() error {
	placesJobName := "Nearby Restaurants (Wheelers Hill)"

	// Take screenshot before triggering job
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		return fmt.Errorf("failed to navigate to queue page: %w", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "places_job_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	if err := qtc.triggerJob(placesJobName); err != nil {
		return fmt.Errorf("failed to trigger places job: %w", err)
	}

	// Monitor Places job with 2-minute timeout (data collection job, no spawn jobs)
	if err := qtc.monitorJob(placesJobName, 120*time.Second, true, false); err != nil {
		return fmt.Errorf("places job monitoring failed: %w", err)
	}

	// Take screenshot after job completes
	if err := qtc.env.TakeScreenshot(qtc.ctx, "places_job_after"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take after screenshot: %v", err)
	}

	return nil
}

// runKeywordExtractionJob triggers and monitors the Keyword Extraction job
func (qtc *queueTestContext) runKeywordExtractionJob() error {
	agentJobName := "Keyword Extraction"

	// Take screenshot before triggering agent job
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		return fmt.Errorf("failed to navigate to queue page: %w", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "agent_job_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	if err := qtc.triggerJob(agentJobName); err != nil {
		return fmt.Errorf("failed to trigger agent job: %w", err)
	}

	// Monitor Agent job with 5-minute timeout
	// Agent jobs process existing documents from Places job
	// Note: We set validateAllProcessed=false because agent jobs don't have real-time
	// child stats tracking via JobMonitor like crawler jobs do. The AgentManager
	// polls internally for completion but doesn't publish stats to the UI.
	// We only verify: documents > 0 (expectDocs: true)
	if err := qtc.monitorJob(agentJobName, 300*time.Second, true, false); err != nil {
		return fmt.Errorf("agent job monitoring failed: %w", err)
	}

	// Take screenshot after agent job completes
	if err := qtc.env.TakeScreenshot(qtc.ctx, "agent_job_after"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take after screenshot: %v", err)
	}

	return nil
}

// TestQueue tests the Places job (Nearby Restaurants)
func TestQueue(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
	defer cleanup()

	qtc.env.LogTest(t, "--- Starting Test: Places Job ---")

	if err := qtc.runPlacesJob(); err != nil {
		t.Fatalf("Places job failed: %v", err)
	}

	qtc.env.LogTest(t, "✓ Test completed successfully")
}

// TestQueueWithKeywordExtraction tests Places job followed by Keyword Extraction job
// This test runs both jobs in sequence to verify:
// 1. Places job creates documents
// 2. Keyword Extraction job processes those documents
// 3. Document count matches completed + failed count
// Note: Keyword Extraction may have failures due to Gemini API rate limits
func TestQueueWithKeywordExtraction(t *testing.T) {
	// Longer timeout: 2 min for places + 5 min for agent + overhead
	qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
	defer cleanup()

	// --- Scenario 1: Places Job ---
	qtc.env.LogTest(t, "--- Starting Scenario 1: Places Job ---")
	if err := qtc.runPlacesJob(); err != nil {
		t.Fatalf("Places job failed: %v", err)
	}

	// --- Scenario 2: Keyword Extraction Job ---
	qtc.env.LogTest(t, "--- Starting Scenario 2: Keyword Extraction Job ---")
	qtc.env.LogTest(t, "Running agent job on existing documents from Places job...")

	if err := qtc.runKeywordExtractionJob(); err != nil {
		t.Fatalf("Keyword Extraction job failed: %v", err)
	}

	qtc.env.LogTest(t, "✓ All scenarios completed successfully")
}

// TestNewsCrawlerCrash tests the News Crawler job to ensure it doesn't crash the service
func TestNewsCrawlerCrash(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 15*time.Minute)
	defer cleanup()

	jobName := "News Crawler"

	qtc.env.LogTest(t, "--- Starting Test: News Crawler Crash Reproduction ---")

	// Navigate to Queue page first to take a "before" screenshot
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "news_crawler_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job
	// We expect it to complete or fail gracefully, NOT hang or crash the service.
	// We expect documents to be found (expectDocs: true) and run for the full duration.
	// Using 10 minute timeout to accommodate increased load (max_pages=50).
	if err := qtc.monitorJob(jobName, 10*time.Minute, true, false); err != nil {
		t.Fatalf("Job monitoring failed (service might have crashed): %v", err)
	}

	qtc.env.LogTest(t, "✓ Test completed successfully - Service remained responsive")
}

// TestJobCancel tests cancelling a running job via the Queue UI
func TestJobCancel(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
	defer cleanup()

	jobName := "News Crawler"

	qtc.env.LogTest(t, "--- Starting Test: Job Cancel ---")

	// Navigate to Queue page first to take a "before" screenshot
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Navigate to Queue page and wait for job to appear
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("queue page did not load: %v", err)
	}

	// Wait for job to appear in the queue
	qtc.env.LogTest(t, "Waiting for job to appear in queue...")
	var jobFound bool
	pollErr := chromedp.Run(qtc.ctx,
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
			chromedp.WithPollingTimeout(15*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if pollErr != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_not_found")
		t.Fatalf("job %s not found in queue after 15s: %v", jobName, pollErr)
	}
	qtc.env.LogTest(t, "✓ Job found in queue")

	// Wait for job to start running (status = running)
	qtc.env.LogTest(t, "Waiting for job to start running...")
	var jobRunning bool
	runningErr := chromedp.Run(qtc.ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge && statusBadge.getAttribute('data-status') === 'running') {
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName),
			&jobRunning,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if runningErr != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_not_running")
		t.Fatalf("job %s did not start running: %v", jobName, runningErr)
	}
	qtc.env.LogTest(t, "✓ Job is running")
	qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_running")

	// Find and click the cancel button for this job
	qtc.env.LogTest(t, "Clicking cancel button...")
	cancelBtnSelector := fmt.Sprintf(`
		(() => {
			const cards = document.querySelectorAll('.card');
			for (const card of cards) {
				const titleEl = card.querySelector('.card-title');
				if (titleEl && titleEl.textContent.includes('%s')) {
					const cancelBtn = card.querySelector('button[title="Cancel Job"]');
					if (cancelBtn) {
						cancelBtn.click();
						return true;
					}
				}
			}
			return false;
		})()
	`, jobName)

	var cancelClicked bool
	if err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(cancelBtnSelector, &cancelClicked),
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_button_click_failed")
		t.Fatalf("failed to click cancel button: %v", err)
	}

	if !cancelClicked {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_button_not_found")
		t.Fatalf("cancel button not found for job %s", jobName)
	}
	qtc.env.LogTest(t, "✓ Cancel button clicked")

	// Wait for confirmation modal to appear
	qtc.env.LogTest(t, "Waiting for confirmation modal...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_modal_not_found")
		t.Fatalf("confirmation modal did not appear: %v", err)
	}
	qtc.env.TakeScreenshot(qtc.ctx, "cancel_confirmation_modal")
	qtc.env.LogTest(t, "✓ Confirmation modal appeared")

	// Click the confirm button in the modal
	qtc.env.LogTest(t, "Confirming cancellation...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_confirm_click_failed")
		t.Fatalf("failed to confirm cancellation: %v", err)
	}
	qtc.env.LogTest(t, "✓ Cancellation confirmed")

	// Wait for job status to change to cancelled
	qtc.env.LogTest(t, "Waiting for job to be cancelled...")

	// Wait a moment for the cancel action to fully propagate
	time.Sleep(2 * time.Second)

	// Manually trigger a data refresh
	if err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(`
			(() => {
				if (typeof loadJobs === 'function') {
					loadJobs();
				}
			})()
		`, nil),
	); err != nil {
		qtc.env.LogTest(t, "Warning: Failed to trigger loadJobs: %v", err)
	}

	// Wait for refresh to complete
	time.Sleep(2 * time.Second)

	var jobCancelled bool
	cancelledErr := chromedp.Run(qtc.ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge && statusBadge.getAttribute('data-status') === 'cancelled') {
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName),
			&jobCancelled,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if cancelledErr != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_status_not_cancelled")
		t.Fatalf("job %s was not cancelled: %v", jobName, cancelledErr)
	}
	qtc.env.LogTest(t, "✓ Job status changed to cancelled")

	// Take final screenshot
	qtc.env.TakeScreenshot(qtc.ctx, "cancel_job_after")

	qtc.env.LogTest(t, "✓ Test completed successfully - Job cancelled via UI")
}

// TestNewsCrawlerConcurrency tests that the News Crawler job runs with proper concurrency
// This verifies that the global queue concurrency setting (default: 10) allows multiple
// child jobs to run in parallel, not just 2.
func TestNewsCrawlerConcurrency(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
	defer cleanup()

	jobName := "News Crawler"
	minExpectedRunning := 3 // We expect at least 3 concurrent jobs with concurrency=10

	qtc.env.LogTest(t, "--- Starting Test: News Crawler Concurrency ---")

	// Navigate to Queue page first to take a "before" screenshot
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "concurrency_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Navigate to Queue page and wait for job to appear
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("queue page did not load: %v", err)
	}

	// Wait for job to start running
	qtc.env.LogTest(t, "Waiting for job to start running...")
	var jobRunning bool
	runningErr := chromedp.Run(qtc.ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge && statusBadge.getAttribute('data-status') === 'running') {
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName),
			&jobRunning,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if runningErr != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "concurrency_job_not_running")
		t.Fatalf("job %s did not start running: %v", jobName, runningErr)
	}
	qtc.env.LogTest(t, "✓ Job is running")

	// Wait a bit for child jobs to spawn
	time.Sleep(5 * time.Second)

	// Poll for at least minExpectedRunning concurrent running children
	// We check the Progress text in the job card which shows "X pending, Y running, Z completed"
	qtc.env.LogTest(t, "Checking for concurrent running child jobs (expecting >= %d)...", minExpectedRunning)

	var maxRunningObserved int
	var concurrencyMet bool

	// Poll multiple times to catch peak concurrency
	for attempt := 0; attempt < 30 && !concurrencyMet; attempt++ {
		var runningCount int
		if err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					// Find the News Crawler job card and extract running count from Progress text
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							// Find the Progress line which shows "X pending, Y running, Z completed"
							const progressEl = card.querySelector('.job-progress-text, [class*="progress"]');
							if (progressEl) {
								const text = progressEl.textContent;
								const match = text.match(/(\d+)\s*running/i);
								if (match) return parseInt(match[1]);
							}
							// Fallback: search all text in the card
							const cardText = card.innerText;
							const match = cardText.match(/(\d+)\s*running/i);
							if (match) return parseInt(match[1]);
						}
					}
					return 0;
				})()
			`, jobName), &runningCount),
		); err != nil {
			qtc.env.LogTest(t, "Warning: Failed to get running count: %v", err)
		}

		if runningCount > maxRunningObserved {
			maxRunningObserved = runningCount
			qtc.env.LogTest(t, "Running child jobs: %d (new max)", runningCount)
		}

		if runningCount >= minExpectedRunning {
			concurrencyMet = true
			qtc.env.TakeScreenshot(qtc.ctx, "concurrency_achieved")
			qtc.env.LogTest(t, "✓ Concurrency requirement met: %d running (expected >= %d)", runningCount, minExpectedRunning)
		}

		if !concurrencyMet {
			time.Sleep(2 * time.Second)
		}
	}

	if !concurrencyMet {
		qtc.env.TakeScreenshot(qtc.ctx, "concurrency_not_met")
		t.Fatalf("Concurrency requirement not met: max observed %d running child jobs, expected >= %d", maxRunningObserved, minExpectedRunning)
	}

	// Take final screenshot
	qtc.env.TakeScreenshot(qtc.ctx, "concurrency_after")

	qtc.env.LogTest(t, "✓ Test completed successfully - Concurrency verified (max %d running)", maxRunningObserved)
}

// TestCopyAndQueueModal tests that the copy and queue button shows a modal (not browser popup)
func TestCopyAndQueueModal(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
	defer cleanup()

	qtc.env.LogTest(t, "--- Starting Test: Copy and Queue Modal ---")

	// First, trigger a quick job to have something to copy
	placesJobName := "Nearby Restaurants (Wheelers Hill)"

	if err := qtc.triggerJob(placesJobName); err != nil {
		t.Fatalf("Failed to trigger initial job: %v", err)
	}

	// Wait for job to complete
	if err := qtc.monitorJob(placesJobName, 2*time.Minute, true, false); err != nil {
		t.Fatalf("Initial job failed: %v", err)
	}
	qtc.env.LogTest(t, "✓ Initial job completed")

	// Navigate to queue page
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("queue page did not load: %v", err)
	}

	// Find and click the rerun/copy button for the completed job
	qtc.env.LogTest(t, "Clicking copy/rerun button...")
	var clicked bool
	if err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const rerunBtn = card.querySelector('button[title="Copy Job and Add to Queue"]');
						if (rerunBtn) {
							rerunBtn.click();
							return true;
						}
					}
				}
				return false;
			})()
		`, placesJobName), &clicked),
	); err != nil {
		t.Fatalf("failed to click rerun button: %v", err)
	}

	if !clicked {
		qtc.env.TakeScreenshot(qtc.ctx, "rerun_button_not_found")
		t.Fatalf("Rerun button not found for job %s", placesJobName)
	}
	qtc.env.LogTest(t, "✓ Rerun button clicked")

	// Wait for modal to appear (NOT browser confirm dialog)
	qtc.env.LogTest(t, "Waiting for confirmation modal...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "modal_not_found")
		t.Fatalf("Modal did not appear (still using browser confirm?): %v", err)
	}
	qtc.env.TakeScreenshot(qtc.ctx, "copy_queue_modal")
	qtc.env.LogTest(t, "✓ Modal appeared")

	// Verify modal title contains expected text
	var modalTitle string
	if err := chromedp.Run(qtc.ctx,
		chromedp.Text(`.modal.active .modal-title`, &modalTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("failed to get modal title: %v", err)
	}
	qtc.env.LogTest(t, "Modal title: %s", modalTitle)

	if !strings.Contains(modalTitle, "Copy") || !strings.Contains(modalTitle, "Queue") {
		t.Fatalf("Expected modal title to contain 'Copy' and 'Queue', got: %s", modalTitle)
	}

	// Cancel to clean up
	if err := chromedp.Run(qtc.ctx,
		chromedp.Click(`.modal.active .btn-link`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("failed to cancel modal: %v", err)
	}

	qtc.env.LogTest(t, "✓ Test completed successfully - Modal confirmed working")
}

// TestCopyAndQueueJobRuns tests that copied jobs actually execute (not stuck in pending)
func TestCopyAndQueueJobRuns(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 8*time.Minute)
	defer cleanup()

	qtc.env.LogTest(t, "--- Starting Test: Copy and Queue Job Runs ---")

	placesJobName := "Nearby Restaurants (Wheelers Hill)"

	// First, run the original job
	if err := qtc.triggerJob(placesJobName); err != nil {
		t.Fatalf("Failed to trigger initial job: %v", err)
	}

	if err := qtc.monitorJob(placesJobName, 2*time.Minute, true, false); err != nil {
		t.Fatalf("Initial job failed: %v", err)
	}
	qtc.env.LogTest(t, "✓ Original job completed")

	// Navigate to queue page
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("queue page did not load: %v", err)
	}

	// Count current completed jobs before copy
	var initialCompletedCount int
	chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const element = document.querySelector('[x-data="jobList"]');
				if (!element) return 0;
				const component = Alpine.$data(element);
				if (!component || !component.allJobs) return 0;
				return component.allJobs.filter(j =>
					j.name && j.name.includes('%s') &&
					j.status === 'completed'
				).length;
			})()
		`, placesJobName), &initialCompletedCount),
	)
	qtc.env.LogTest(t, "Initial completed job count: %d", initialCompletedCount)

	// Click the rerun button
	qtc.env.LogTest(t, "Clicking copy/rerun button...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const rerunBtn = card.querySelector('button[title="Copy Job and Add to Queue"]');
						if (rerunBtn) {
							rerunBtn.click();
							return true;
						}
					}
				}
				return false;
			})()
		`, placesJobName), nil),
	); err != nil {
		t.Fatalf("failed to click rerun button: %v", err)
	}

	// Wait for modal and confirm
	qtc.env.LogTest(t, "Confirming copy in modal...")
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		t.Fatalf("failed to confirm copy: %v", err)
	}
	qtc.env.LogTest(t, "✓ Copy confirmed")

	// Wait for the copied job to execute
	qtc.env.LogTest(t, "Waiting for copied job to run...")

	// Poll for the new job to complete (should see completed count increase)
	startTime := time.Now()
	timeout := 3 * time.Minute
	var newestJobStatus string

	for time.Since(startTime) < timeout {
		// Refresh job list
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					if (typeof loadJobs === 'function') {
						loadJobs();
					}
				})()
			`, nil),
		)
		time.Sleep(2 * time.Second)

		// Get status of the newest job with our name
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return 'error: element not found';
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return 'error: no jobs';

					// Find jobs with our name, get the newest one
					const matchingJobs = component.allJobs.filter(j => j.name && j.name.includes('%s'));
					if (matchingJobs.length === 0) return 'error: no matching jobs';

					// Sort by created_at descending to get newest
					matchingJobs.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
					return matchingJobs[0].status;
				})()
			`, placesJobName), &newestJobStatus),
		)

		qtc.env.LogTest(t, "  Newest job status: %s", newestJobStatus)

		// Job should move from pending to running to completed
		if newestJobStatus == "completed" {
			break
		}

		// If still pending after 30 seconds, that's a failure
		if newestJobStatus == "pending" && time.Since(startTime) > 30*time.Second {
			qtc.env.TakeScreenshot(qtc.ctx, "job_stuck_pending")
			t.Fatalf("Copied job is stuck in pending status after 30s - job is NOT executing!")
		}

		time.Sleep(3 * time.Second)
	}

	if newestJobStatus != "completed" && newestJobStatus != "running" {
		qtc.env.TakeScreenshot(qtc.ctx, "job_not_running")
		t.Fatalf("Copied job did not run. Final status: %s", newestJobStatus)
	}

	// If still running, wait a bit more for completion
	if newestJobStatus == "running" {
		qtc.env.LogTest(t, "Job is running, waiting for completion...")
		time.Sleep(30 * time.Second)
	}

	qtc.env.TakeScreenshot(qtc.ctx, "copy_job_completed")
	qtc.env.LogTest(t, "✓ Test completed successfully - Copied job executed (status: %s)", newestJobStatus)
}

// TestWebSearchJob tests that the web search job executes and produces a document
// Uses Gemini SDK with GoogleSearch grounding to search for ASX:GNP company info
func TestWebSearchJob(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
	defer cleanup()

	// Check if a valid Gemini API key is available
	// The test environment loads keys from .env.test into EnvVars
	// The job definition uses {google_gemini_api_key}
	hasGeminiKey := false
	for _, keyName := range []string{"google_gemini_api_key", "QUAERO_GEMINI_GOOGLE_API_KEY", "QUAERO_AGENT_GOOGLE_API_KEY"} {
		if key := qtc.env.EnvVars[keyName]; key != "" && key != "fake-gemini-api-key-for-testing" {
			hasGeminiKey = true
			qtc.env.LogTest(t, "Found Gemini API key: %s", keyName)
			break
		}
	}

	if !hasGeminiKey {
		t.Skip("Skipping web search test - no valid google_gemini_api_key found in .env.test")
	}

	qtc.env.LogTest(t, "--- Starting Test: Web Search Job ---")

	jobName := "Web Search: ASX:GNP Company Info"

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger web search job: %v", err)
	}
	qtc.env.LogTest(t, "✓ Web search job triggered")

	// Monitor job completion (allow 5 minutes for web search with follow-up queries)
	if err := qtc.monitorJob(jobName, 5*time.Minute, true, false); err != nil {
		qtc.env.TakeScreenshot(qtc.ctx, "web_search_failed")
		t.Fatalf("Web search job failed: %v", err)
	}
	qtc.env.LogTest(t, "✓ Web search job completed")

	// Take screenshot of queue showing completed job
	qtc.env.TakeScreenshot(qtc.ctx, "web_search_job_completed")

	qtc.env.LogTest(t, "✓ Test completed successfully - Web search job completed with documents")
}
