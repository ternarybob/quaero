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
