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

func TestQueue(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create a timeout context for the entire test
	// Increased timeout for job execution
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	baseURL := env.GetBaseURL()
	jobsURL := baseURL + "/jobs"
	queueURL := baseURL + "/queue"

	// Helper to trigger a job by name
	triggerJob := func(jobName string) error {
		env.LogTest(t, "Triggering job: %s", jobName)

		// Navigate to Jobs page
		if err := chromedp.Run(ctx, chromedp.Navigate(jobsURL)); err != nil {
			return fmt.Errorf("failed to navigate to jobs page: %w", err)
		}

		// Wait for page to load
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
		); err != nil {
			return fmt.Errorf("jobs page did not load: %w", err)
		}

		// Take screenshot of jobs page before clicking
		screenshotName := fmt.Sprintf("jobs_page_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
		if err := env.TakeScreenshot(ctx, screenshotName); err != nil {
			env.LogTest(t, "Failed to take jobs page screenshot: %v", err)
		}

		// Convert job name to button ID format
		// Must match Alpine.js logic: jobDef.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-run'
		// Example: "Nearby Restaurants (Wheelers Hill)" → "nearby-restaurants-wheelers-hill--run"
		// The regex replaces each sequence of non-alphanumeric chars with a single dash
		buttonID := strings.ToLower(jobName)
		re := regexp.MustCompile(`[^a-z0-9]+`)
		buttonID = re.ReplaceAllString(buttonID, "-")
		buttonID = buttonID + "-run"

		env.LogTest(t, "Looking for button with ID: %s", buttonID)

		// Click the run button by ID
		runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
			chromedp.Click(runBtnSelector, chromedp.ByQuery),
		); err != nil {
			env.TakeScreenshot(ctx, "run_click_failed_"+jobName)
			return fmt.Errorf("failed to click run button for %s (selector: %s): %w", jobName, runBtnSelector, err)
		}

		// Handle Confirmation Modal
		env.LogTest(t, "Waiting for confirmation modal")
		// Wait for modal to appear (body gets class modal-open)
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for animation
		); err != nil {
			env.TakeScreenshot(ctx, "modal_wait_failed_"+jobName)
			return fmt.Errorf("confirmation modal did not appear for %s: %w", jobName, err)
		}

		// Take screenshot of confirmation modal
		modalScreenshotName := fmt.Sprintf("confirmation_modal_%s", strings.ReplaceAll(strings.ToLower(jobName), " ", "_"))
		if err := env.TakeScreenshot(ctx, modalScreenshotName); err != nil {
			env.LogTest(t, "Failed to take modal screenshot: %v", err)
		}

		env.LogTest(t, "Confirming run")
		// Click Confirm button (primary button in modal footer)
		if err := chromedp.Run(ctx,
			chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
			chromedp.Sleep(1*time.Second), // Wait for action to register
		); err != nil {
			env.TakeScreenshot(ctx, "confirm_click_failed_"+jobName)
			return fmt.Errorf("failed to confirm run for %s: %w", jobName, err)
		}

		env.LogTest(t, "✓ Job triggered: %s", jobName)
		return nil
	}

	// Helper to monitor job on Queue page
	monitorJob := func(jobName string, expectDocs bool) error {
		env.LogTest(t, "Monitoring job: %s", jobName)

		// Navigate to Queue page
		if err := chromedp.Run(ctx, chromedp.Navigate(queueURL)); err != nil {
			return fmt.Errorf("failed to navigate to queue page: %w", err)
		}

		// Wait for page to load
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for Alpine.js to load jobs
		); err != nil {
			return fmt.Errorf("queue page did not load: %w", err)
		}

		env.LogTest(t, "Queue page loaded, looking for job...")

		// Poll for job to appear in the queue (it may take a moment to be created)
		// Use JavaScript to check Alpine.js component state
		// The queue page uses x-data="jobList" component
		var jobFound bool
		pollErr := chromedp.Run(ctx,
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
			env.TakeScreenshot(ctx, "job_not_found_"+jobName)
			return fmt.Errorf("job %s not found in queue after 10s: %w", jobName, pollErr)
		}
		env.LogTest(t, "✓ Job found in queue")

		// Wait for status to be 'completed' or 'completed_with_errors'
		// The status badge is in the same card.
		// We can poll for the status text associated with this job.
		// XPath: Find title -> ancestor card -> find status label
		statusSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]//span[contains(@class, "label")]//span[text()="Completed" or text()="Completed with Errors"]`, jobName)

		env.LogTest(t, "Waiting for job completion...")
		// Poll for completion (timeout 120s for API-heavy jobs)
		err := chromedp.Run(ctx,
			chromedp.Poll(
				fmt.Sprintf(`document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue != null`, strings.ReplaceAll(statusSelector, "'", "\\'")),
				nil,
				chromedp.WithPollingTimeout(120*time.Second),
				chromedp.WithPollingInterval(2*time.Second),
			),
		)
		if err != nil {
			// Capture screenshot on failure
			env.TakeScreenshot(ctx, "job_failed_"+jobName)
			return fmt.Errorf("job %s did not complete: %w", jobName, err)
		}

		// Get the actual status text
		var statusText string
		if err := chromedp.Run(ctx,
			chromedp.Text(statusSelector, &statusText, chromedp.BySearch),
		); err == nil {
			env.LogTest(t, "✓ Job status: %s", statusText)
		}

		// Get job statistics (completed, failed, total)
		cardSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]`, jobName)

		// Extract statistics using JavaScript
		var stats map[string]interface{}
		err = chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const card = document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
					if (!card) return null;

					const getText = (selector) => {
						const el = card.querySelector(selector);
						return el ? el.textContent.trim() : '';
					};

					// Extract numbers from text like "5 Documents", "3 Completed", "2 Failed"
					const extractNumber = (text) => {
						const match = text.match(/(\d+)/);
						return match ? parseInt(match[1]) : 0;
					};

					const cardText = card.textContent;
					const docsMatch = cardText.match(/(\d+)\s+Documents?/);
					const completedMatch = cardText.match(/(\d+)\s+Completed/);
					const failedMatch = cardText.match(/(\d+)\s+Failed/);

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

			env.LogTest(t, "Job statistics: %d documents, %d completed, %d failed", docs, completed, failed)

			// Verify document count if expected
			if expectDocs && docs == 0 {
				return fmt.Errorf("expected documents > 0, got: %d", docs)
			}

			// Verify at least some work was done
			if completed == 0 && failed == 0 {
				env.LogTest(t, "⚠ Warning: No completed or failed tasks (job may have had no work to do)")
			}

			// Log if there were failures (may be due to API rate limits)
			if failed > 0 {
				env.LogTest(t, "⚠ Warning: %d tasks failed (may be due to API rate limits or configuration issues)", failed)
			}

			env.LogTest(t, "✓ Job statistics verified")
		} else {
			env.LogTest(t, "⚠ Warning: Could not extract job statistics")
		}

		return nil
	}

	// --- Test Scenario 1: Places Job ---
	// Run "Nearby Restaurants" and verify documents count > 0
	env.LogTest(t, "--- Starting Scenario 1: Places Job ---")
	placesJobName := "Nearby Restaurants (Wheelers Hill)"

	// Take screenshot before triggering job
	if err := chromedp.Run(ctx, chromedp.Navigate(queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	if err := env.TakeScreenshot(ctx, "places_job_before"); err != nil {
		t.Logf("Failed to take before screenshot: %v", err)
	}

	if err := triggerJob(placesJobName); err != nil {
		t.Fatalf("Failed to trigger places job: %v", err)
	}

	if err := monitorJob(placesJobName, true); err != nil { // Expect documents > 0
		t.Fatalf("Places job monitoring failed: %v", err)
	}

	// Take screenshot after job completes
	if err := env.TakeScreenshot(ctx, "places_job_after"); err != nil {
		t.Logf("Failed to take after screenshot: %v", err)
	}

	// --- Test Scenario 2: Agent Job ---
	// Run "Keyword Extraction" on existing documents (from Places job or News Crawler)
	// Note: Agent job requires documents to exist before running
	env.LogTest(t, "--- Starting Scenario 2: Agent Job ---")
	agentJobName := "Keyword Extraction"

	// Documents should exist from Places job (Scenario 1)
	// We'll proceed with agent job and let it process existing documents
	env.LogTest(t, "Running agent job on existing documents from Places job...")

	// Take screenshot before triggering agent job
	if err := chromedp.Run(ctx, chromedp.Navigate(queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	if err := env.TakeScreenshot(ctx, "agent_job_before"); err != nil {
		t.Logf("Failed to take before screenshot: %v", err)
	}

	if err := triggerJob(agentJobName); err != nil {
		t.Fatalf("Failed to trigger agent job: %v", err)
	}

	if err := monitorJob(agentJobName, false); err != nil { // Don't expect new documents, just processing
		t.Fatalf("Agent job monitoring failed: %v", err)
	}

	// Take screenshot after agent job completes
	if err := env.TakeScreenshot(ctx, "agent_job_after"); err != nil {
		t.Logf("Failed to take after screenshot: %v", err)
	}

	env.LogTest(t, "✓ All scenarios completed successfully")
}
