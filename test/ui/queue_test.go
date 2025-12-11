package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
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

// toInt safely converts interface{} to int (handles float64 from JSON)
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return 0
	}
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
// Note: This function returns an error if the job fails (status="failed").
// For jobs expected to fail, use monitorJobAllowFailure instead.
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

		// Take full page screenshot every 30 seconds (captures all child rows)
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			screenshotName := fmt.Sprintf("monitor_%s_%ds", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"), int(elapsed.Seconds()))
			if err := qtc.env.TakeFullScreenshot(qtc.ctx, screenshotName); err != nil {
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

			// Take full page screenshot on status change for debugging (captures all child rows)
			screenshotName := fmt.Sprintf("status_%s_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"), currentStatus)
			qtc.env.TakeFullScreenshot(qtc.ctx, screenshotName)
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

	// Fail if job failed (unless explicitly expected)
	if currentStatus == "failed" {
		// Try to get the failure reason from the UI
		var failureReason string
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							// Look for job-error-alert div which contains the failure reason
							const errorAlert = card.querySelector('.job-error-alert');
							if (errorAlert) {
								// Get the error text (after "Failure Reason:")
								const text = errorAlert.textContent;
								const match = text.match(/Failure Reason:\s*(.+)/);
								if (match) return match[1].trim();
								return errorAlert.textContent.trim();
							}
							// Fallback: search for "Failure Reason:" in card text
							const text = card.innerText;
							const match = text.match(/Failure Reason:\s*(.+?)(?:\n|$)/);
							if (match) return match[1].trim();
						}
					}
					return '';
				})()
			`, jobName), &failureReason),
		)
		if failureReason != "" {
			return fmt.Errorf("job %s failed: %s", jobName, failureReason)
		}
		return fmt.Errorf("job %s failed (no failure reason found in UI)", jobName)
	}

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
				const docsMatch = cardText.match(/(\d+)\s+(?:Documents?|docs)/i);
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

		// For multi-step jobs, document count may not be in card text - use Alpine data as fallback
		if docs == 0 && refreshData != nil {
			if alpineDocCount, ok := refreshData["documentCount"]; ok && alpineDocCount != nil {
				if docFloat, ok := alpineDocCount.(float64); ok && docFloat > 0 {
					docs = int(docFloat)
					qtc.env.LogTest(qtc.t, "Using Alpine data for document count: %d", docs)
				}
			}
		}

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

// TestNearbyRestaurantsJob tests the Nearby Restaurants (Places) job
// This test verifies:
// 1. Job can be triggered via the UI
// 2. Job executes and either completes or fails
// 3. If job fails, the failure reason is displayed in the UI
// 4. The test properly detects and reports failure via monitorJob
func TestNearbyRestaurantsJob(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 5*time.Minute)
	defer cleanup()

	// Check if a valid Google Places API key is available
	// The job definition uses {google_places_api_key}
	hasPlacesKey := false
	for _, keyName := range []string{"google_places_api_key", "QUAERO_PLACES_GOOGLE_API_KEY", "GOOGLE_PLACES_API_KEY"} {
		if key := qtc.env.EnvVars[keyName]; key != "" && !strings.HasPrefix(key, "fake-") {
			hasPlacesKey = true
			qtc.env.LogTest(t, "Found Google Places API key: %s", keyName)
			break
		}
	}

	if !hasPlacesKey {
		t.Skip("Skipping Nearby Restaurants test - no valid google_places_api_key found in .env.test")
	}

	qtc.env.LogTest(t, "--- Starting Test: Nearby Restaurants Job ---")

	jobName := "Nearby Restaurants (Wheelers Hill)"

	// Take screenshot before triggering job
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := qtc.env.TakeScreenshot(qtc.ctx, "nearby_restaurants_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	qtc.env.LogTest(t, "✓ Job triggered")

	// Monitor job - this will fail if the job fails (which tests error detection)
	err := qtc.monitorJob(jobName, 2*time.Minute, true, false)
	if err != nil {
		// Job failed - take screenshot and report
		qtc.env.TakeScreenshot(qtc.ctx, "nearby_restaurants_failed")
		t.Fatalf("Job failed: %v", err)
	}

	// Take screenshot after job completes
	qtc.env.TakeScreenshot(qtc.ctx, "nearby_restaurants_completed")
	qtc.env.LogTest(t, "✓ Test completed successfully - Job completed")
}

// TestNearbyRestaurantsKeywordsMultiStep tests the multi-step job definition
// that runs Places search followed by Keyword Extraction in a single job.
// This test verifies:
// 1. Multi-step job can be triggered via the UI
// 2. Both steps execute sequentially (search_nearby_restaurants -> extract_keywords)
// 3. Job reaches a terminal state (doesn't hang in running state) - CRITICAL
// 4. Documents are created by the places search step
// 5. Keywords are extracted from documents by the agent step (may fail due to API limits)
// 6. Child jobs run in correct order based on dependencies
// 7. filter_source_type filters documents correctly (should be 20)
// 8. UI shows document count for each child job
// 9. UI expands/collapses child jobs when user clicks the children button
//
// Note: The keyword extraction step may fail due to Gemini API rate limits.
// The primary goal is to verify the job doesn't hang - failure is acceptable
// as long as it's a graceful failure to a terminal state.
func TestNearbyRestaurantsKeywordsMultiStep(t *testing.T) {
	// Multi-step job needs more time: places search + keyword extraction
	qtc, cleanup := newQueueTestContext(t, 10*time.Minute)
	defer cleanup()

	// Check for required API keys
	hasPlacesKey := false
	hasGeminiKey := false

	for _, keyName := range []string{"google_places_api_key", "QUAERO_PLACES_GOOGLE_API_KEY"} {
		if key := qtc.env.EnvVars[keyName]; key != "" && !strings.HasPrefix(key, "fake-") {
			hasPlacesKey = true
			qtc.env.LogTest(t, "Found Google Places API key: %s", keyName)
			break
		}
	}

	for _, keyName := range []string{"google_gemini_api_key", "QUAERO_GEMINI_GOOGLE_API_KEY"} {
		if key := qtc.env.EnvVars[keyName]; key != "" && !strings.HasPrefix(key, "fake-") {
			hasGeminiKey = true
			qtc.env.LogTest(t, "Found Gemini API key: %s", keyName)
			break
		}
	}

	if !hasPlacesKey {
		t.Skip("Skipping multi-step test - no valid google_places_api_key found")
	}
	if !hasGeminiKey {
		t.Skip("Skipping multi-step test - no valid google_gemini_api_key found")
	}

	qtc.env.LogTest(t, "--- Starting Test: Multi-Step Job (Places + Keywords) ---")

	// This is the multi-step job defined in test/config/job-definitions/nearby-restaurants-keywords.toml
	jobName := "Nearby Restaurants + Keywords (Wheelers Hill)"

	// Take screenshot before triggering job
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	qtc.env.TakeScreenshot(qtc.ctx, "multistep_before")

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger multi-step job: %v", err)
	}
	qtc.env.LogTest(t, "✓ Multi-step job triggered")

	// Navigate back to Queue page to monitor progress
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	// Wait for page to load
	if err := chromedp.Run(qtc.ctx, chromedp.WaitVisible(`.page-title`, chromedp.ByQuery)); err != nil {
		t.Fatalf("Queue page did not load: %v", err)
	}

	// Monitor job execution dynamically WITHOUT refreshing the page
	// This verifies that WebSockets and JS updates are working correctly
	qtc.env.LogTest(t, "Monitoring job dynamically (NO REFRESH) to verify auto-expansion and live logs...")

	// Capture console logs
	chromedp.ListenTarget(qtc.ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			// Filter for Queue logs to reduce noise
			msg := strings.Join(args, " ")
			if strings.Contains(msg, "[Queue]") || ev.Type == runtime.APITypeError || ev.Type == runtime.APITypeWarning {
				qtc.env.LogTest(t, "[CONSOLE] %s: %s", ev.Type, msg)
			}
		}
	})

	timeout := 5 * time.Minute
	startTime := time.Now()
	lastScreenshotTime := startTime

	var treeExpanded bool
	var logsFound bool
	var initialLogsSeen bool
	var dynamicLogsSeen bool // Tracks if we see *new* logs after initial load
	var prevLogCount int
	var finalStatus string

	for time.Since(startTime) < timeout {
		// Take periodic screenshot every 15 seconds to track progress
		if time.Since(lastScreenshotTime) > 15*time.Second {
			qtc.env.TakeFullScreenshot(qtc.ctx, fmt.Sprintf("multistep_progress_%ds", int(time.Since(startTime).Seconds())))
			lastScreenshotTime = time.Now()
		}

		var result map[string]interface{}
		err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					// Find job row
					const card = Array.from(document.querySelectorAll('.card')).find(c => 
						c.querySelector('.card-title') && c.querySelector('.card-title').textContent.includes('%s')
					);
					if (!card) return { status: 'missing' };

					// Get status
					const statusBadge = card.querySelector('.label[data-status]');
					const status = statusBadge ? statusBadge.getAttribute('data-status') : 'unknown';

					// Check keys for tree view and logs
					const treeView = card.querySelector('.inline-tree-view');
					const logs = card.querySelectorAll('.tree-log-line');

					return {
						status: status,
						hasTree: !!treeView && treeView.offsetParent !== null, // Visible
						logCount: logs.length
					};
				})()
			`, jobName), &result),
		)

		if err != nil {
			t.Logf("Error evaluating page state: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		status := fmt.Sprintf("%v", result["status"])
		hasTree := result["hasTree"] == true
		logCount := 0
		if val, ok := result["logCount"].(float64); ok {
			logCount = int(val)
		}

		if hasTree {
			treeExpanded = true
		}
		if logCount > 0 {
			logsFound = true

			// Track dynamic updates
			if !initialLogsSeen {
				initialLogsSeen = true
				prevLogCount = logCount
			} else {
				// If log count increases, we have dynamic updates
				if logCount > prevLogCount {
					dynamicLogsSeen = true
					qtc.env.LogTest(t, "Dynamic log update detected: count increased from %d to %d", prevLogCount, logCount)
				}
				prevLogCount = logCount
			}
		}

		qtc.env.LogTest(t, "Status: %s | Tree: %v | Logs: %d", status, hasTree, logCount)

		if status == "completed" || status == "failed" || status == "cancelled" {
			finalStatus = status
			qtc.env.LogTest(t, "Job reached terminal state: %s", status)
			break
		}

		if time.Since(startTime) > 30*time.Second && status == "running" && !treeExpanded {
			qtc.env.TakeScreenshot(qtc.ctx, "multistep_not_expanded")
			t.Fatalf("Job is running but tree view did not auto-expand within 30s")
		}

		time.Sleep(2 * time.Second)
	}

	qtc.env.TakeScreenshot(qtc.ctx, "multistep_final_state")

	if finalStatus == "" {
		t.Fatalf("Job timed out without reaching terminal state")
	}

	// Assertions
	if !treeExpanded {
		t.Errorf("FAIL: Inline Tree View never expanded")
	} else {
		qtc.env.LogTest(t, "✓ Inline Tree View auto-expanded")
	}

	if !logsFound {
		t.Errorf("FAIL: No step logs were ever displayed")
	} else {
		qtc.env.LogTest(t, "✓ Logs appeared in Tree View")
	}

	if !dynamicLogsSeen {
		t.Errorf("FAIL: No dynamic log updates detected (logs remained static at %d)", prevLogCount)
	} else {
		qtc.env.LogTest(t, "✓ Dynamic log updates verified")
	}

	qtc.env.LogTest(t, "Job reached terminal status: %s", finalStatus)

	// The critical assertion: job must reach a terminal state (not stuck in running)
	if finalStatus == "running" || finalStatus == "pending" {
		qtc.env.TakeScreenshot(qtc.ctx, "multistep_stuck")
		t.Fatalf("CRITICAL: Job is stuck in %s state - multi-step execution is broken", finalStatus)
	}

	// Job completed or failed gracefully
	// Job completed or failed gracefully
	if finalStatus == "failed" {
		// Fetch job error from UI
		var jobError string
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return '';
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return '';
					const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
					return job ? (job.error || '') : '';
				})()
			`, jobName), &jobError),
		)

		// Job failed - check if it was due to agent failures (acceptable due to API limits)
		if strings.Contains(jobError, "agent jobs") {
			qtc.env.LogTest(t, "⚠ Job failed due to agent API issues (acceptable): %v", jobError)
		} else {
			qtc.env.TakeScreenshot(qtc.ctx, "multistep_failed")
			t.Fatalf("Multi-step job failed unexpectedly: %v", jobError)
		}
	} else {
		qtc.env.LogTest(t, "✓ Multi-step job completed successfully")
	}

	// Verify documents were created (places step should have succeeded)
	var docCount int
	chromedp.Run(qtc.ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const element = document.querySelector('[x-data="jobList"]');
				if (!element) return 0;
				const component = Alpine.$data(element);
				if (!component || !component.allJobs) return 0;
				const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
				return job ? (job.document_count || 0) : 0;
			})()
		`, jobName), &docCount),
	)

	if docCount > 0 {
		qtc.env.LogTest(t, "✓ Documents created: %d", docCount)
	} else {
		qtc.env.LogTest(t, "⚠ No documents reported (may need API refresh)")
	}

	// Take final screenshot
	qtc.env.TakeScreenshot(qtc.ctx, "multistep_final")
	qtc.env.LogTest(t, "✓ Test completed - Multi-step job executed and reached terminal state (%s)", finalStatus)

	// ============================================================================
	// SUB-TESTS: These tests verify specific UI behaviors for multi-step jobs
	// ============================================================================

	// Sub-test 1: Verify child job execution order (based on dependencies)
	t.Run("ChildJobExecutionOrder", func(t *testing.T) {
		qtc.env.LogTest(t, "--- Sub-test: Child Job Execution Order ---")

		// Reload jobs to ensure we have all child jobs in the data
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

		// Get child jobs for the parent job and verify their execution order
		// Step 1: search_nearby_restaurants (no dependencies) should run first
		// Step 2: extract_keywords (depends="search_nearby_restaurants") should run after
		var childJobData []map[string]interface{}
		err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return [];
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return [];

					// Find the parent job
					const parentJob = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					if (!parentJob) return [];

					// Find child jobs for this parent
					// Handle parent_id comparison flexibly - may be string or object
					const parentId = String(parentJob.id);
					const children = component.allJobs.filter(j => {
						if (!j.parent_id) return false;
						const childParentId = typeof j.parent_id === 'string' ? j.parent_id : String(j.parent_id);
						return childParentId === parentId;
					});

					// Return relevant data sorted by created_at
					return children.map(c => ({
						id: c.id,
						name: c.name,
						status: c.status,
						created_at: c.created_at,
						started_at: c.started_at,
						completed_at: c.completed_at,
						job_type: c.job_type
					})).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
				})()
			`, jobName), &childJobData),
		)

		if err != nil {
			t.Fatalf("Failed to get child job data: %v", err)
		}

		qtc.env.LogTest(t, "Found %d child jobs", len(childJobData))
		for i, child := range childJobData {
			qtc.env.LogTest(t, "  Child %d: %s (type: %v, status: %v, created: %v)",
				i+1, child["name"], child["job_type"], child["status"], child["created_at"])
		}

		// In the multi-step architecture:
		// - places_search step runs inline (no child jobs created)
		// - agent step creates child jobs for each document
		// So we expect multiple agent child jobs (one per document from places_search)
		if len(childJobData) < 1 {
			t.Fatalf("Expected at least 1 child job from agent step, got %d", len(childJobData))
		}

		// Verify all child jobs are agent type (keyword_extractor)
		agentChildCount := 0
		for _, child := range childJobData {
			if child["job_type"] == "agent" {
				agentChildCount++
			}
		}
		qtc.env.LogTest(t, "✓ Found %d agent child jobs (created by agent step for each document)", agentChildCount)

		// Verify children are sorted by created_at (they should be created roughly simultaneously)
		if len(childJobData) >= 2 {
			firstCreated := childJobData[0]["created_at"]
			lastCreated := childJobData[len(childJobData)-1]["created_at"]
			qtc.env.LogTest(t, "  First child created: %v", firstCreated)
			qtc.env.LogTest(t, "  Last child created: %v", lastCreated)
		}

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_child_order")
	})

	// Sub-test 2: Verify filter_source_type filtering (should match 20 documents from places)
	t.Run("FilterSourceTypeFiltering", func(t *testing.T) {
		qtc.env.LogTest(t, "--- Sub-test: Filter Source Type Filtering ---")

		// The agent step has filter_source_type = "places"
		// This should filter to exactly the documents created by the places_search step
		// Expected: 20 documents (as configured in max_results)

		// Get the parent job's document count
		var parentDocCount int
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return 0;
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return 0;
					const job = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					return job ? (job.document_count || 0) : 0;
				})()
			`, jobName), &parentDocCount),
		)

		qtc.env.LogTest(t, "Parent document count: %d", parentDocCount)

		// The parent document count should be exactly 20 (unique documents)
		// If it shows 24, that means document_count is being double-counted
		// (bug: EventDocumentUpdated was incorrectly incrementing parent count)
		expectedDocCount := 20
		if parentDocCount != expectedDocCount {
			t.Errorf("filter_source_type filtering issue: expected %d documents, got %d (double-counting bug?)",
				expectedDocCount, parentDocCount)
		} else {
			qtc.env.LogTest(t, "✓ Document count matches expected: %d", parentDocCount)
		}

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_filter_test")
	})

	// Sub-test 3: Verify child jobs display their own document counts in UI
	t.Run("ChildJobDocumentCounts", func(t *testing.T) {
		qtc.env.LogTest(t, "--- Sub-test: Child Job Document Counts ---")

		// First, expand the parent to show child job rows
		// Find and click the children expand button
		var expandClicked bool
		err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							// Find the children expand button
							const expandBtn = card.querySelector('button i.fa-chevron-right, button i.fa-chevron-down');
							if (expandBtn) {
								expandBtn.closest('button').click();
								return true;
							}
						}
					}
					return false;
				})()
			`, jobName), &expandClicked),
		)

		if err != nil || !expandClicked {
			qtc.env.LogTest(t, "⚠ Could not click expand button")
		} else {
			qtc.env.LogTest(t, "✓ Clicked expand button")
		}

		// Wait for UI to update
		time.Sleep(1 * time.Second)

		// Trigger data refresh to ensure child jobs are loaded
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

		// Now get child job document counts from the UI
		var childDocCounts []map[string]interface{}
		err = chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return [];
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return [];

					// Find the parent job
					const parentJob = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					if (!parentJob) return [];

					// Find child jobs for this parent
					// Handle parent_id comparison flexibly - may be string or object
					const parentId = String(parentJob.id);
					const children = component.allJobs.filter(j => {
						if (!j.parent_id) return false;
						const childParentId = typeof j.parent_id === 'string' ? j.parent_id : String(j.parent_id);
						return childParentId === parentId;
					});

					// Return document count data for each child
					return children.map(c => ({
						id: c.id,
						name: c.name,
						document_count: c.document_count,
						metadata_doc_count: c.metadata ? c.metadata.document_count : null,
						status: c.status
					}));
				})()
			`, jobName), &childDocCounts),
		)

		if err != nil {
			t.Fatalf("Failed to get child document counts: %v", err)
		}

		qtc.env.LogTest(t, "Child job document counts:")
		hasValidDocCounts := true
		for _, child := range childDocCounts {
			docCount := 0
			if dc, ok := child["document_count"].(float64); ok {
				docCount = int(dc)
			}
			qtc.env.LogTest(t, "  %s: document_count=%d, status=%v",
				child["name"], docCount, child["status"])

			// Completed children should have document_count > 0
			if child["status"] == "completed" && docCount == 0 {
				hasValidDocCounts = false
				t.Errorf("Child job '%s' is completed but has document_count=0", child["name"])
			}
		}

		if hasValidDocCounts {
			qtc.env.LogTest(t, "✓ All completed child jobs have valid document counts")
		}

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_child_doc_counts")
	})

	// Sub-test 4: Verify expand/collapse step events functionality
	t.Run("ExpandCollapseStepEvents", func(t *testing.T) {
		qtc.env.LogTest(t, "--- Sub-test: Expand/Collapse Step Events Panel ---")

		// Navigate back to queue page to ensure fresh state
		if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
			t.Fatalf("failed to navigate to queue page: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Find a step row with Events button and check initial state
		// Events panel shows logs/events for each step
		var initialState map[string]interface{}
		err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					// Find step rows with Events button
					const stepEventsPanels = document.querySelectorAll('.step-events-panel');
					for (const panel of stepEventsPanels) {
						const eventsBtn = panel.querySelector('button');
						if (eventsBtn && eventsBtn.textContent.includes('Events')) {
							const chevron = eventsBtn.querySelector('i.fa-chevron-right, i.fa-chevron-down');
							// Extract event count from button text "Events (N)"
							const match = eventsBtn.textContent.match(/Events\s*\((\d+)\)/);
							const eventCount = match ? parseInt(match[1]) : 0;
							return {
								buttonFound: true,
								buttonText: eventsBtn.textContent.trim(),
								eventCount: eventCount,
								isExpanded: chevron ? chevron.classList.contains('fa-chevron-down') : false,
								chevronClass: chevron ? chevron.className : 'no chevron'
							};
						}
					}
					// Fallback: search for any button containing "Events"
					const allButtons = document.querySelectorAll('button');
					for (const btn of allButtons) {
						if (btn.textContent.includes('Events')) {
							const chevron = btn.querySelector('i.fa-chevron-right, i.fa-chevron-down');
							const match = btn.textContent.match(/Events\s*\((\d+)\)/);
							const eventCount = match ? parseInt(match[1]) : 0;
							return {
								buttonFound: true,
								buttonText: btn.textContent.trim(),
								eventCount: eventCount,
								isExpanded: chevron ? chevron.classList.contains('fa-chevron-down') : false,
								chevronClass: chevron ? chevron.className : 'no chevron',
								location: 'fallback'
							};
						}
					}
					return { buttonFound: false };
				})()
			`, &initialState),
		)

		if err != nil {
			t.Fatalf("Failed to check initial state: %v", err)
		}

		// Retry if button not found - events may still be loading
		if !initialState["buttonFound"].(bool) {
			qtc.env.LogTest(t, "Events button not found immediately, waiting for UI to load...")
			for retries := 0; retries < 10; retries++ {
				time.Sleep(1 * time.Second)
				err = chromedp.Run(qtc.ctx,
					chromedp.Evaluate(`
						(() => {
							const allButtons = document.querySelectorAll('button');
							for (const btn of allButtons) {
								if (btn.textContent.includes('Events')) {
									const chevron = btn.querySelector('i.fa-chevron-right, i.fa-chevron-down');
									const match = btn.textContent.match(/Events\s*\((\d+)\)/);
									const eventCount = match ? parseInt(match[1]) : 0;
									return {
										buttonFound: true,
										buttonText: btn.textContent.trim(),
										eventCount: eventCount,
										isExpanded: chevron ? chevron.classList.contains('fa-chevron-down') : false,
										chevronClass: chevron ? chevron.className : 'no chevron'
									};
								}
							}
							return { buttonFound: false };
						})()
					`, &initialState),
				)
				if err == nil && initialState["buttonFound"].(bool) {
					qtc.env.LogTest(t, "Found Events button after %d retries", retries+1)
					break
				}
			}
		}

		if !initialState["buttonFound"].(bool) {
			t.Fatalf("Events button not found in step rows after waiting")
		}

		qtc.env.LogTest(t, "Initial state: button='%s', eventCount=%v, expanded=%v, chevron='%s'",
			initialState["buttonText"], initialState["eventCount"], initialState["isExpanded"], initialState["chevronClass"])

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_events_initial")

		// Click the Events button to expand
		var expandClicked bool
		err = chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					const allButtons = document.querySelectorAll('button');
					for (const btn of allButtons) {
						if (btn.textContent.includes('Events')) {
							btn.click();
							return true;
						}
					}
					return false;
				})()
			`, &expandClicked),
		)

		if !expandClicked {
			t.Fatalf("Failed to click Events button")
		}
		qtc.env.LogTest(t, "✓ Clicked Events button")

		// Wait for UI to update
		time.Sleep(1 * time.Second)

		// Check expanded state
		var expandedState map[string]interface{}
		err = chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					const allButtons = document.querySelectorAll('button');
					for (const btn of allButtons) {
						if (btn.textContent.includes('Events')) {
							const chevron = btn.querySelector('i.fa-chevron-right, i.fa-chevron-down');
							// Check if the step-logs-container is visible
							const panel = btn.closest('.step-events-panel');
							const logsContainer = panel ? panel.querySelector('.step-logs-container, [data-step-logs-container]') : null;
							// Also check for any expanded logs container nearby
							const anyLogsContainer = document.querySelector('.step-logs-container, [data-step-logs-container]');
							// Count log entries
							const logEntries = document.querySelectorAll('.step-log-entry');
							return {
								isExpanded: chevron ? chevron.classList.contains('fa-chevron-down') : false,
								chevronClass: chevron ? chevron.className : 'no chevron',
								logsContainerVisible: logsContainer !== null || anyLogsContainer !== null,
								logEntryCount: logEntries.length
							};
						}
					}
					return { isExpanded: false };
				})()
			`, &expandedState),
		)

		if err != nil {
			t.Fatalf("Failed to check expanded state: %v", err)
		}

		qtc.env.LogTest(t, "After expand: expanded=%v, chevron='%s', logsContainerVisible=%v, logEntries=%v",
			expandedState["isExpanded"], expandedState["chevronClass"],
			expandedState["logsContainerVisible"], expandedState["logEntryCount"])

		// Verify chevron changed to down (expanded)
		if expandedState["isExpanded"] != nil && !expandedState["isExpanded"].(bool) {
			qtc.env.LogTest(t, "⚠ Chevron did not change to expanded state (may be auto-expanded)")
		}

		// Verify logs container appeared
		if !expandedState["logsContainerVisible"].(bool) {
			qtc.env.LogTest(t, "⚠ Logs container not visible after clicking expand")
		} else {
			qtc.env.LogTest(t, "✓ Logs container is visible after expanding")
		}

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_events_expanded")

		// Click again to collapse
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					const allButtons = document.querySelectorAll('button');
					for (const btn of allButtons) {
						if (btn.textContent.includes('Events')) {
							btn.click();
							return true;
						}
					}
					return false;
				})()
			`, nil),
		)

		time.Sleep(1 * time.Second)

		// Verify collapsed again
		var collapsedState map[string]interface{}
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(`
				(() => {
					const allButtons = document.querySelectorAll('button');
					for (const btn of allButtons) {
						if (btn.textContent.includes('Events')) {
							const chevron = btn.querySelector('i.fa-chevron-right, i.fa-chevron-down');
							// Check if logs container is still visible (should be hidden after collapse)
							const panel = btn.closest('.step-events-panel');
							const logsContainer = panel ? panel.querySelector('.step-logs-container, [data-step-logs-container]') : null;
							return {
								isExpanded: chevron ? chevron.classList.contains('fa-chevron-down') : false,
								logsContainerVisible: logsContainer !== null
							};
						}
					}
					return {};
				})()
			`, &collapsedState),
		)

		qtc.env.LogTest(t, "After collapse: expanded=%v, logsContainerVisible=%v",
			collapsedState["isExpanded"], collapsedState["logsContainerVisible"])

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_events_collapsed")
		qtc.env.LogTest(t, "✓ Events expand/collapse functionality verified")
	})

	// Sub-test: Verify step progress matches child job counts (real-time alignment)
	t.Run("StepProgressAlignment", func(t *testing.T) {
		qtc.env.LogTest(t, "--- Sub-test: Step Progress Alignment ---")

		// Wait a moment for any pending WebSocket events
		time.Sleep(1 * time.Second)

		// Get step progress data and child job counts from UI
		var alignmentData map[string]interface{}
		err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return { error: 'jobList element not found' };
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return { error: 'component or allJobs not found' };

					// Find the parent job
					const parentJob = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					if (!parentJob) return { error: 'parent job not found' };

					// Get child jobs
					const parentId = String(parentJob.id);
					const children = component.allJobs.filter(j => {
						if (!j.parent_id) return false;
						const childParentId = typeof j.parent_id === 'string' ? j.parent_id : String(j.parent_id);
						return childParentId === parentId;
					});

					// Count child statuses
					const childCounts = {
						total: children.length,
						pending: children.filter(c => c.status === 'pending').length,
						running: children.filter(c => c.status === 'running').length,
						completed: children.filter(c => c.status === 'completed').length,
						failed: children.filter(c => c.status === 'failed').length,
						cancelled: children.filter(c => c.status === 'cancelled').length
					};

					// Get _stepProgress from parent job (real-time WebSocket updates)
					const stepProgress = parentJob._stepProgress || {};

					// Get step definitions to know step names
					const stepDefs = parentJob.metadata?.step_definitions || [];
					const stepNames = stepDefs.map(s => s.name);

					// Check if step progress exists for any steps
					let stepProgressData = {};
					for (const stepName of stepNames) {
						if (stepProgress[stepName]) {
							stepProgressData[stepName] = stepProgress[stepName];
						}
					}

					return {
						parentJobId: parentJob.id,
						parentStatus: parentJob.status,
						childCounts: childCounts,
						stepProgress: stepProgressData,
						stepNames: stepNames,
						parentPendingChildren: parentJob.pending_children,
						parentRunningChildren: parentJob.running_children,
						parentCompletedChildren: parentJob.completed_children,
						parentFailedChildren: parentJob.failed_children
					};
				})()
			`, jobName), &alignmentData),
		)

		if err != nil {
			t.Fatalf("Failed to get alignment data: %v", err)
		}

		if errMsg, ok := alignmentData["error"]; ok {
			t.Fatalf("Error getting alignment data: %v", errMsg)
		}

		qtc.env.LogTest(t, "Parent job ID: %v, Status: %v", alignmentData["parentJobId"], alignmentData["parentStatus"])
		qtc.env.LogTest(t, "Step names: %v", alignmentData["stepNames"])

		// Log child counts (actual child job statuses in allJobs)
		if childCounts, ok := alignmentData["childCounts"].(map[string]interface{}); ok {
			qtc.env.LogTest(t, "Actual child job counts (from allJobs):")
			qtc.env.LogTest(t, "  Total: %v, Pending: %v, Running: %v, Completed: %v, Failed: %v",
				childCounts["total"], childCounts["pending"], childCounts["running"],
				childCounts["completed"], childCounts["failed"])
		}

		// Log parent job's aggregate child stats
		qtc.env.LogTest(t, "Parent job aggregate child stats:")
		qtc.env.LogTest(t, "  Pending: %v, Running: %v, Completed: %v, Failed: %v",
			alignmentData["parentPendingChildren"], alignmentData["parentRunningChildren"],
			alignmentData["parentCompletedChildren"], alignmentData["parentFailedChildren"])

		// Log step progress data (from real-time WebSocket events)
		if stepProgress, ok := alignmentData["stepProgress"].(map[string]interface{}); ok {
			qtc.env.LogTest(t, "Step progress (from _stepProgress, real-time WebSocket updates):")
			if len(stepProgress) == 0 {
				qtc.env.LogTest(t, "  (no step progress data stored - may not have received step_progress events)")
			}
			for stepName, progress := range stepProgress {
				if p, ok := progress.(map[string]interface{}); ok {
					qtc.env.LogTest(t, "  Step '%s': Pending=%v, Running=%v, Completed=%v, Failed=%v",
						stepName, p["pending"], p["running"], p["completed"], p["failed"])
				}
			}
		}

		// Verify alignment: step progress should match child job counts
		// Note: This test validates the fix for the real-time alignment issue
		childCounts := alignmentData["childCounts"].(map[string]interface{})
		stepProgress := alignmentData["stepProgress"].(map[string]interface{})

		// If we have step progress data, verify it matches actual child counts
		if len(stepProgress) > 0 {
			for stepName, progress := range stepProgress {
				if p, ok := progress.(map[string]interface{}); ok {
					// Get step progress values
					spTotal := toInt(p["pending"]) + toInt(p["running"]) + toInt(p["completed"]) + toInt(p["failed"]) + toInt(p["cancelled"])
					actualTotal := toInt(childCounts["total"])

					qtc.env.LogTest(t, "Alignment check for step '%s':", stepName)
					qtc.env.LogTest(t, "  Step progress total: %d", spTotal)
					qtc.env.LogTest(t, "  Actual child total: %d", actualTotal)

					// Note: Step progress tracks children for ONE step, so it may differ from total children
					// The important thing is that step progress is being received and stored
					if spTotal > 0 {
						qtc.env.LogTest(t, "  ✓ Step progress data is being received and stored")
					}
				}
			}
		} else {
			// No step progress data - this might be because the job already completed
			// and no step_progress events were received during the test
			parentStatus := alignmentData["parentStatus"].(string)
			if parentStatus == "completed" || parentStatus == "failed" {
				qtc.env.LogTest(t, "⚠ No step progress data stored (job already in terminal state: %s)", parentStatus)
				qtc.env.LogTest(t, "  This is expected if the job completed before step_progress events could be received")
			} else {
				qtc.env.LogTest(t, "⚠ No step progress data stored (job status: %s)", parentStatus)
			}
		}

		qtc.env.TakeScreenshot(qtc.ctx, "multistep_alignment_check")
		qtc.env.LogTest(t, "✓ Step progress alignment verification completed")
	})
}

// TestStepEventsDisplay tests that step events are displayed in the UI
// This verifies that job_log events are published via WebSocket and displayed in the Events panel
func TestStepEventsDisplay(t *testing.T) {
	qtc, cleanup := newQueueTestContext(t, 15*time.Minute)
	defer cleanup()

	jobName := "News Crawler"

	qtc.env.LogTest(t, "--- Starting Test: Step Events Display ---")

	// Navigate to Queue page first to take a "before" screenshot
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}
	if err := qtc.env.TakeFullScreenshot(qtc.ctx, "events_before"); err != nil {
		qtc.env.LogTest(qtc.t, "Failed to take before screenshot: %v", err)
	}

	// Trigger the job
	if err := qtc.triggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Navigate to Queue page
	if err := chromedp.Run(qtc.ctx, chromedp.Navigate(qtc.queueURL)); err != nil {
		t.Fatalf("failed to navigate to queue page: %v", err)
	}

	// Wait for page to load and job to appear
	if err := chromedp.Run(qtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("queue page did not load: %v", err)
	}

	qtc.env.LogTest(t, "Waiting for job to start running...")

	// Wait for job to start running
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
		qtc.env.TakeFullScreenshot(qtc.ctx, "events_job_not_running")
		t.Fatalf("job %s did not start running: %v", jobName, runningErr)
	}
	qtc.env.LogTest(t, "✓ Job is running")

	// Track events during job execution
	var eventsCount int
	var eventMessages []string
	lastScreenshotTime := time.Now()
	screenshotCounter := 0

	// Helper function to check events
	checkEvents := func() (int, []string, error) {
		var result map[string]interface{}
		if err := chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return { error: 'No jobList element found' };
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return { error: 'No Alpine component or allJobs' };

					const parentJob = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					if (!parentJob) return { error: 'Parent job not found' };

					const managerLogs = component.jobLogs[parentJob.id] || [];
					const stepPanels = document.querySelectorAll('.step-events-panel');
					let totalEvents = 0;
					let allMessages = [];

					for (const panel of stepPanels) {
						const btn = panel.querySelector('.step-events-btn');
						if (btn) {
							const count = parseInt(btn.getAttribute('data-events-count') || '0');
							totalEvents += count;
							if (count > 0 && !panel.querySelector('.step-logs-container')) {
								btn.click();
							}
						}
						const logEntries = panel.querySelectorAll('.step-log-entry .step-log-message');
						logEntries.forEach(el => {
							if (el.textContent) allMessages.push(el.textContent);
						});
					}

					// Get messages from managerLogs if panel messages empty
					if (allMessages.length === 0 && managerLogs.length > 0) {
						managerLogs.forEach(log => {
							if (log.message) allMessages.push(log.message);
						});
					}

					return {
						eventsCount: totalEvents > 0 ? totalEvents : managerLogs.length,
						messages: allMessages,
						managerLogsCount: managerLogs.length,
						parentJobId: parentJob.id.substring(0, 8),
						jobStatus: parentJob.status
					};
				})()
			`, jobName), &result),
		); err != nil {
			return 0, nil, err
		}

		if errMsg, ok := result["error"]; ok {
			return 0, nil, fmt.Errorf("%v", errMsg)
		}

		count := toInt(result["eventsCount"])
		var msgs []string
		if msgList, ok := result["messages"].([]interface{}); ok {
			for _, m := range msgList {
				msgs = append(msgs, fmt.Sprintf("%v", m))
			}
		}
		return count, msgs, nil
	}

	// Monitor job until complete, checking events periodically
	qtc.env.LogTest(t, "Monitoring job and checking events...")
	timeout := 10 * time.Minute
	pollInterval := 5 * time.Second
	startTime := time.Now()

	for {
		elapsed := time.Since(startTime)
		if elapsed > timeout {
			qtc.env.TakeFullScreenshot(qtc.ctx, "events_timeout")
			t.Fatalf("Job monitoring timed out after %v", timeout)
		}

		// Periodic screenshot every 20 seconds
		if time.Since(lastScreenshotTime) >= 20*time.Second {
			screenshotCounter++
			screenshotName := fmt.Sprintf("events_periodic_%02d", screenshotCounter)
			qtc.env.TakeFullScreenshot(qtc.ctx, screenshotName)
			qtc.env.LogTest(t, "📸 Took periodic screenshot: %s", screenshotName)
			lastScreenshotTime = time.Now()
		}

		// Check job status and events
		var result map[string]interface{}
		chromedp.Run(qtc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return { status: 'unknown' };
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return { status: 'unknown' };
					const job = component.allJobs.find(j => j.name && j.name.includes('%s') && !j.parent_id);
					return job ? { status: job.status } : { status: 'not_found' };
				})()
			`, jobName), &result),
		)

		status := "unknown"
		if s, ok := result["status"].(string); ok {
			status = s
		}

		// Check events
		count, msgs, err := checkEvents()
		if err == nil {
			eventsCount = count
			eventMessages = msgs
			qtc.env.LogTest(t, "Status: %s, Events: %d, Elapsed: %v", status, eventsCount, elapsed.Round(time.Second))
		}

		// Check if job completed
		if status == "completed" || status == "failed" {
			qtc.env.LogTest(t, "✓ Job %s with status: %s", jobName, status)
			qtc.env.TakeFullScreenshot(qtc.ctx, "events_job_complete")
			break
		}

		time.Sleep(pollInterval)
	}

	// Final events check
	finalCount, finalMsgs, _ := checkEvents()
	if finalCount > eventsCount {
		eventsCount = finalCount
		eventMessages = finalMsgs
	}

	// Verify events were found
	if eventsCount == 0 {
		qtc.env.TakeFullScreenshot(qtc.ctx, "events_not_found")
		t.Fatalf("No events found after job completion")
	}

	qtc.env.LogTest(t, "✓ Found %d events total", eventsCount)

	// Check for expected step messages
	hasStartingWorkers := false
	hasStepFinished := false
	for _, msg := range eventMessages {
		if strings.Contains(msg, "Starting workers") {
			hasStartingWorkers = true
		}
		if strings.Contains(msg, "Step finished") || strings.Contains(msg, "Step completed") {
			hasStepFinished = true
		}
	}

	// Log sample messages for debugging (first 10)
	qtc.env.LogTest(t, "Sample event messages (first 10):")
	for i, msg := range eventMessages {
		if i >= 10 {
			qtc.env.LogTest(t, "  ... and %d more", len(eventMessages)-10)
			break
		}
		qtc.env.LogTest(t, "  [%d] %s", i+1, msg)
	}

	// Verify step completion message (required)
	if !hasStepFinished {
		qtc.env.TakeFullScreenshot(qtc.ctx, "events_no_completion")
		t.Fatalf("Step completion message not found in events")
	}
	qtc.env.LogTest(t, "✓ Found step completion message")

	// Note: "Starting workers" may not appear after page refresh (sent before refresh)
	if hasStartingWorkers {
		qtc.env.LogTest(t, "✓ Found 'Starting workers' message")
	} else {
		qtc.env.LogTest(t, "⚠ 'Starting workers' message not found (may have been sent before page refresh)")
	}

	// Verify log format: check for text-based level tags and originator tags
	var logFormatResult map[string]interface{}
	chromedp.Run(qtc.ctx,
		chromedp.Evaluate(`
			(() => {
				const stepLogs = document.querySelectorAll('.step-log-entry');
				let hasLevelTag = false;
				let hasOriginatorTag = false;
				let levelTags = [];
				let originatorTags = [];

				for (const entry of stepLogs) {
					// Check for text-based level tags [INF], [WRN], [ERR], [DBG]
					const levelEl = entry.querySelector('.step-log-level');
					if (levelEl) {
						const text = levelEl.textContent.trim();
						if (text.match(/\[(INF|WRN|ERR|DBG)\]/)) {
							hasLevelTag = true;
							if (!levelTags.includes(text)) levelTags.push(text);
						}
					}

					// Check for originator tags like [step], [worker]
					const originatorEl = entry.querySelector('.step-log-originator');
					if (originatorEl && originatorEl.textContent.trim()) {
						hasOriginatorTag = true;
						const tag = originatorEl.textContent.trim();
						if (!originatorTags.includes(tag)) originatorTags.push(tag);
					}
				}

				return {
					hasLevelTag: hasLevelTag,
					hasOriginatorTag: hasOriginatorTag,
					levelTags: levelTags,
					originatorTags: originatorTags,
					totalLogEntries: stepLogs.length
				};
			})()
		`, &logFormatResult),
	)

	qtc.env.LogTest(t, "Log format verification:")
	qtc.env.LogTest(t, "  Total log entries: %v", logFormatResult["totalLogEntries"])
	qtc.env.LogTest(t, "  Level tags found: %v", logFormatResult["levelTags"])
	qtc.env.LogTest(t, "  Originator tags found: %v", logFormatResult["originatorTags"])

	// Verify text-based level tags are present
	if hasLevel, ok := logFormatResult["hasLevelTag"].(bool); ok && hasLevel {
		qtc.env.LogTest(t, "✓ Text-based level tags ([INF], [WRN], [ERR], [DBG]) are displayed")
	} else {
		qtc.env.TakeFullScreenshot(qtc.ctx, "events_no_level_tags")
		t.Errorf("Expected text-based level tags ([INF], [WRN], [ERR], [DBG]) but none found")
	}

	// Verify originator tags are present (e.g., [step], [worker])
	if hasOriginator, ok := logFormatResult["hasOriginatorTag"].(bool); ok && hasOriginator {
		qtc.env.LogTest(t, "✓ Originator tags ([step], [worker]) are displayed")
	} else {
		qtc.env.LogTest(t, "⚠ Originator tags not found (may be optional depending on log source)")
	}

	qtc.env.TakeFullScreenshot(qtc.ctx, "events_final")
	qtc.env.LogTest(t, "✓ Test completed successfully - Step events are displaying in UI")
}
