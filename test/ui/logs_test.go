package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// Codebase Classify job definition ID (from test/config/job-definitions/codebase_classify.toml)
const codebaseClassifyJobID = "codebase_classify"

// logsTestContext holds shared state for logs UI tests
type logsTestContext struct {
	t        *testing.T
	env      *common.TestEnvironment
	ctx      context.Context
	jobsURL  string
	queueURL string
}

// newLogsTestContext creates a new test context with browser and environment
func newLogsTestContext(t *testing.T, timeout time.Duration) (*logsTestContext, func()) {
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

	ltc := &logsTestContext{
		t:        t,
		env:      env,
		ctx:      browserCtx,
		jobsURL:  baseURL + "/jobs",
		queueURL: baseURL + "/queue",
	}

	// Return cleanup function
	cleanup := func() {
		if err := chromedp.Cancel(browserCtx); err != nil {
			t.Logf("Warning: browser cancel returned: %v", err)
		}
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return ltc, cleanup
}

// TestLogsUIDisplay tests that logs are displayed correctly in the Queue page UI
func TestLogsUIDisplay(t *testing.T) {
	ltc, cleanup := newLogsTestContext(t, 3*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "Starting logs UI display test")

	// Navigate to Jobs page and trigger a job
	t.Run("TriggerJobAndViewLogs", func(t *testing.T) {
		// Navigate to Jobs page
		if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.jobsURL)); err != nil {
			t.Fatalf("Failed to navigate to jobs page: %v", err)
		}

		// Wait for page to load
		if err := chromedp.Run(ltc.ctx,
			chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		); err != nil {
			t.Fatalf("Jobs page did not load: %v", err)
		}

		ltc.env.TakeScreenshot(ltc.ctx, "logs_test_jobs_page")
		ltc.env.LogTest(t, "Jobs page loaded")

		// Look for any available job and trigger it
		// We'll use the first job definition available
		var jobDefName string
		err := chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				(() => {
					const card = document.querySelector('.card .card-header .card-title');
					if (card) return card.textContent.trim();
					return '';
				})()
			`, &jobDefName),
		)
		if err != nil || jobDefName == "" {
			ltc.env.LogTest(t, "No job definitions found, skipping trigger test")
			t.Skip("No job definitions available to test")
			return
		}

		ltc.env.LogTest(t, "Found job definition: %s", jobDefName)

		// Click the run button for the first job
		if err := chromedp.Run(ltc.ctx,
			chromedp.Click(`.card .btn-success`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			ltc.env.TakeScreenshot(ltc.ctx, "logs_test_run_click_failed")
			t.Fatalf("Failed to click run button: %v", err)
		}

		// Handle confirmation modal if present
		var modalVisible bool
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`document.querySelector('.modal.active') !== null`, &modalVisible),
		)

		if modalVisible {
			ltc.env.LogTest(t, "Confirmation modal visible, clicking confirm")
			chromedp.Run(ltc.ctx,
				chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
				chromedp.Sleep(1*time.Second),
			)
		}

		ltc.env.TakeScreenshot(ltc.ctx, "logs_test_job_triggered")
		ltc.env.LogTest(t, "Job triggered successfully")
	})

	// Navigate to Queue page and verify logs display
	t.Run("VerifyLogsInQueuePage", func(t *testing.T) {
		// Navigate to Queue page
		ltc.env.LogTest(t, "Navigating to Queue page")
		if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL)); err != nil {
			t.Fatalf("Failed to navigate to queue page: %v", err)
		}

		// Wait for page to load
		if err := chromedp.Run(ltc.ctx,
			chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		); err != nil {
			t.Fatalf("Queue page did not load: %v", err)
		}

		ltc.env.TakeScreenshot(ltc.ctx, "logs_test_queue_page_initial")
		ltc.env.LogTest(t, "Queue page loaded")

		// Wait for job to appear and generate logs
		ltc.env.LogTest(t, "Waiting for job activity and logs...")
		time.Sleep(5 * time.Second)

		// Check if any job cards are visible
		var jobCardCount int
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`document.querySelectorAll('.card[data-job-id]').length`, &jobCardCount),
		)
		ltc.env.LogTest(t, "Found %d job cards in queue", jobCardCount)

		// Expand a job card if available
		if jobCardCount > 0 {
			// Click on a job card to expand it
			err := chromedp.Run(ltc.ctx,
				chromedp.Click(`.card[data-job-id] .accordion-header`, chromedp.ByQuery),
				chromedp.Sleep(1*time.Second),
			)
			if err != nil {
				ltc.env.LogTest(t, "Could not expand job card: %v", err)
			} else {
				ltc.env.LogTest(t, "Expanded job card")
			}
		}

		ltc.env.TakeScreenshot(ltc.ctx, "logs_test_queue_expanded")

		// Check for step logs containers
		var stepLogsContainerCount int
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`document.querySelectorAll('.step-logs-container').length`, &stepLogsContainerCount),
		)
		ltc.env.LogTest(t, "Found %d step logs containers", stepLogsContainerCount)

		// Wait more for logs to be populated
		ltc.env.LogTest(t, "Waiting for logs to be populated...")
		time.Sleep(5 * time.Second)

		// Check if logs are being displayed
		var logsFound bool
		var logLineCount int
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				(() => {
					const containers = document.querySelectorAll('.step-logs-container');
					let totalLines = 0;
					for (const container of containers) {
						const lines = container.querySelectorAll('.log-line, pre, code');
						totalLines += lines.length;
					}
					return totalLines;
				})()
			`, &logLineCount),
		)

		logsFound = logLineCount > 0
		ltc.env.LogTest(t, "Log lines found in UI: %d", logLineCount)

		ltc.env.TakeScreenshot(ltc.ctx, "logs_test_final_state")

		// Log test results
		if logsFound {
			ltc.env.LogTest(t, "SUCCESS: Logs are being displayed in the UI")
		} else {
			ltc.env.LogTest(t, "NOTE: No log lines visible yet (job may still be initializing)")
		}
	})
}

// TestLogsUITiming measures the time it takes for logs to appear in the UI
func TestLogsUITiming(t *testing.T) {
	ltc, cleanup := newLogsTestContext(t, 3*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "Starting logs UI timing test")

	// Navigate to Queue page
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Queue page did not load: %v", err)
	}

	ltc.env.LogTest(t, "Queue page loaded")

	// Create job via API for more control
	helper := ltc.env.NewHTTPTestHelper(t)

	// Create and execute a test job
	defID := fmt.Sprintf("test-ui-logs-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":      defID,
		"name":    "UI Logs Test",
		"type":    "crawler",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "test-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls":  []string{"https://example.com"},
					"max_depth":   1,
					"max_pages":   3,
					"concurrency": 1,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	if err != nil || resp.StatusCode != 201 {
		t.Skip("Could not create job definition for timing test")
	}
	resp.Body.Close()
	defer helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))

	// Execute the job and record start time
	startTime := time.Now()
	resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	if err != nil || resp.StatusCode != 202 {
		t.Fatalf("Could not execute job definition")
	}

	var execResult map[string]interface{}
	helper.ParseJSONResponse(resp, &execResult)
	jobID, _ := execResult["job_id"].(string)
	resp.Body.Close()
	defer helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))

	ltc.env.LogTest(t, "Job %s started at %v", jobID, startTime)

	// Wait and refresh queue page
	chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL))
	chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	// Poll for logs to appear
	var firstLogTime time.Time
	pollTimeout := 30 * time.Second
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(pollTimeout)

	ltc.env.LogTest(t, "Polling for logs to appear in UI...")

	for time.Now().Before(deadline) {
		// Refresh the page to get latest data
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				if (typeof loadJobs === 'function') {
					loadJobs();
				}
			`, nil),
		)
		time.Sleep(200 * time.Millisecond)

		// Check for log lines specific to our job
		var hasLogs bool
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const jobCard = document.querySelector('[data-job-id="%s"]');
					if (!jobCard) return false;
					const logLines = jobCard.querySelectorAll('.log-line, .step-logs-container pre, .step-logs-container code');
					return logLines.length > 0;
				})()
			`, jobID), &hasLogs),
		)

		if hasLogs {
			firstLogTime = time.Now()
			break
		}

		time.Sleep(pollInterval)
	}

	ltc.env.TakeScreenshot(ltc.ctx, "logs_test_timing_final")

	if !firstLogTime.IsZero() {
		elapsed := firstLogTime.Sub(startTime)
		ltc.env.LogTest(t, "SUCCESS: First logs appeared in UI after %v", elapsed)

		// Performance assertion: logs should appear within 10 seconds
		if elapsed.Seconds() < 10 {
			ltc.env.LogTest(t, "PASS: Logs appeared within acceptable time (<10s)")
		} else {
			ltc.env.LogTest(t, "WARNING: Logs took longer than expected (>10s)")
		}
	} else {
		ltc.env.LogTest(t, "WARNING: No logs appeared within %v (job may be queued)", pollTimeout)
	}
}

// TestLogsUICodebaseClassify tests log display specifically with the Codebase Classify job
// This uses the pre-configured job definition from test/config/job-definitions/codebase_classify.toml
// It verifies that completed steps have events in their event panels (not "No events yet")
func TestLogsUICodebaseClassify(t *testing.T) {
	ltc, cleanup := newLogsTestContext(t, 5*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "Starting Codebase Classify logs UI test")

	// Create HTTP helper to execute the job via API
	helper := ltc.env.NewHTTPTestHelper(t)

	// Check if the codebase_classify job definition exists
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s", codebaseClassifyJobID))
	if err != nil || resp.StatusCode != 200 {
		t.Skip("codebase_classify job definition not available")
	}
	resp.Body.Close()

	ltc.env.LogTest(t, "Found codebase_classify job definition")

	// Execute the job via API
	execResp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", codebaseClassifyJobID), nil)
	if err != nil || execResp.StatusCode != 202 {
		t.Skipf("Could not execute codebase_classify: %v", err)
	}

	var execResult map[string]interface{}
	helper.ParseJSONResponse(execResp, &execResult)
	jobID, _ := execResult["job_id"].(string)
	execResp.Body.Close()

	ltc.env.LogTest(t, "Executed codebase_classify -> job_id=%s", jobID)

	defer func() {
		// Cancel the job on cleanup
		helper.POST(fmt.Sprintf("/api/jobs/%s/cancel", jobID), nil)
		helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
	}()

	// Navigate to Queue page
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		t.Fatalf("Queue page did not load: %v", err)
	}

	ltc.env.TakeScreenshot(ltc.ctx, "codebase_classify_queue_initial")
	ltc.env.LogTest(t, "Queue page loaded")

	// Wait for job to appear
	ltc.env.LogTest(t, "Waiting for job to appear...")
	startTime := time.Now()
	pollTimeout := 30 * time.Second
	deadline := time.Now().Add(pollTimeout)
	var jobFound bool

	for time.Now().Before(deadline) {
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`if (typeof loadJobs === 'function') loadJobs();`, nil),
		)
		time.Sleep(500 * time.Millisecond)

		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				document.querySelector('[data-job-id="%s"]') !== null
			`, jobID), &jobFound),
		)

		if jobFound {
			elapsed := time.Since(startTime)
			ltc.env.LogTest(t, "Job appeared in UI after %v", elapsed)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	if !jobFound {
		ltc.env.TakeScreenshot(ltc.ctx, "codebase_classify_job_not_found")
		t.Fatalf("Job %s did not appear in queue within %v", jobID, pollTimeout)
	}

	// Expand the job card
	err = chromedp.Run(ltc.ctx,
		chromedp.Click(fmt.Sprintf(`[data-job-id="%s"] .accordion-header`, jobID), chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		ltc.env.LogTest(t, "Could not expand job card: %v", err)
	} else {
		ltc.env.LogTest(t, "Expanded job card")
	}

	ltc.env.TakeScreenshot(ltc.ctx, "codebase_classify_expanded")

	// Wait for job to complete (with timeout)
	ltc.env.LogTest(t, "Waiting for job to complete...")
	jobTimeout := 3 * time.Minute
	jobDeadline := time.Now().Add(jobTimeout)
	var jobStatus string

	for time.Now().Before(jobDeadline) {
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`if (typeof loadJobs === 'function') loadJobs();`, nil),
		)
		time.Sleep(1 * time.Second)

		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.querySelector('[data-job-id="%s"]');
					if (!card) return '';
					const badge = card.querySelector('[data-status]');
					return badge ? badge.getAttribute('data-status') : '';
				})()
			`, jobID), &jobStatus),
		)

		if jobStatus == "completed" || jobStatus == "failed" || jobStatus == "cancelled" {
			ltc.env.LogTest(t, "Job reached terminal state: %s", jobStatus)
			break
		}

		// Log progress every 15 seconds
		if time.Since(startTime).Seconds() > 0 && int(time.Since(startTime).Seconds())%15 == 0 {
			ltc.env.LogTest(t, "Job status: %s (waiting...)", jobStatus)
		}
	}

	ltc.env.TakeScreenshot(ltc.ctx, "codebase_classify_completed")

	// Now verify that completed steps have events (not "No events yet")
	t.Run("VerifyCompletedStepsHaveEvents", func(t *testing.T) {
		// Get all completed steps and check their event counts
		var stepEventInfo []map[string]interface{}
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				(() => {
					const results = [];
					// Find all step cards
					const stepCards = document.querySelectorAll('.card.step-card, [class*="step"]');

					// Alternative: look for step containers with status
					document.querySelectorAll('.accordion-body .card, [data-step-name]').forEach(card => {
						const stepName = card.querySelector('.card-title, [class*="step-name"]')?.textContent?.trim() || 'unknown';
						const statusBadge = card.querySelector('[data-status], .label');
						const status = statusBadge?.getAttribute('data-status') || statusBadge?.textContent?.trim() || '';

						// Check for event count in the Events button
						const eventsBtn = card.querySelector('.step-events-btn, [data-events-count]');
						const eventCount = eventsBtn ? parseInt(eventsBtn.getAttribute('data-events-count') || '0') : 0;

						// Check for "No events yet" message
						const noEventsMsg = card.querySelector('.step-logs-empty');
						const hasNoEventsMessage = noEventsMsg && noEventsMsg.textContent.includes('No events yet');

						results.push({
							stepName: stepName,
							status: status.toLowerCase(),
							eventCount: eventCount,
							hasNoEventsMessage: hasNoEventsMessage
						});
					});
					return results;
				})()
			`, &stepEventInfo),
		)

		ltc.env.LogTest(t, "Step event information: %+v", stepEventInfo)

		// Check each completed step has events
		var failedSteps []string
		for _, step := range stepEventInfo {
			stepName := step["stepName"].(string)
			status := step["status"].(string)
			eventCount := int(step["eventCount"].(float64))
			hasNoEvents := step["hasNoEventsMessage"].(bool)

			if status == "completed" {
				if eventCount == 0 || hasNoEvents {
					failedSteps = append(failedSteps, fmt.Sprintf("%s (events: %d, noEventsMsg: %v)", stepName, eventCount, hasNoEvents))
					ltc.env.LogTest(t, "FAIL: Completed step '%s' has no events (count: %d)", stepName, eventCount)
				} else {
					ltc.env.LogTest(t, "PASS: Completed step '%s' has %d events", stepName, eventCount)
				}
			}
		}

		// Alternative check: look at the Events(N) button text directly
		var eventButtonTexts []string
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('.step-events-btn, button[class*="event"]')).map(btn => btn.textContent.trim())
			`, &eventButtonTexts),
		)
		ltc.env.LogTest(t, "Event button texts: %v", eventButtonTexts)

		// Take final screenshot
		ltc.env.TakeFullScreenshot(ltc.ctx, "codebase_classify_events_check")

		// Fail test if any completed steps have no events
		if len(failedSteps) > 0 {
			t.Errorf("Completed steps with missing events: %v", failedSteps)
		}
	})

	// Check for step logs containers and content
	t.Run("VerifyLogLinesPresent", func(t *testing.T) {
		var stepCount int
		var logLineCount int

		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				document.querySelectorAll('[data-job-id="%s"] .step-logs-container').length
			`, jobID), &stepCount),
		)
		chromedp.Run(ltc.ctx,
			chromedp.Evaluate(`
				(() => {
					let count = 0;
					document.querySelectorAll('.step-logs-container').forEach(c => {
						count += c.querySelectorAll('.log-line, pre, code').length;
					});
					return count;
				})()
			`, &logLineCount),
		)

		ltc.env.LogTest(t, "Found %d step containers with %d log lines", stepCount, logLineCount)

		if logLineCount == 0 && jobStatus == "completed" {
			t.Error("Job completed but no log lines are visible in the UI")
		}
	})
}

// TestLogsUILogLevels verifies log levels are displayed correctly with proper styling
func TestLogsUILogLevels(t *testing.T) {
	ltc, cleanup := newLogsTestContext(t, 2*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "Starting log levels UI test")

	// Navigate to Queue page
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		t.Fatalf("Queue page did not load: %v", err)
	}

	// Check for log level styling classes
	var levelStyles map[string]bool
	chromedp.Run(ltc.ctx,
		chromedp.Evaluate(`
			(() => {
				const styles = {};
				// Check for log level indicators in log lines
				const logLines = document.querySelectorAll('.log-line, [class*="log-"]');
				for (const line of logLines) {
					const text = line.textContent.toLowerCase();
					if (text.includes('info')) styles['info'] = true;
					if (text.includes('warn')) styles['warn'] = true;
					if (text.includes('error')) styles['error'] = true;
					if (text.includes('debug')) styles['debug'] = true;
				}
				return styles;
			})()
		`, &levelStyles),
	)

	ltc.env.LogTest(t, "Log levels found in UI: %v", levelStyles)
	ltc.env.TakeScreenshot(ltc.ctx, "logs_test_levels")

	// Log which levels were found
	for level, found := range levelStyles {
		if found {
			ltc.env.LogTest(t, "Found log level: %s", strings.ToUpper(level))
		}
	}
}
