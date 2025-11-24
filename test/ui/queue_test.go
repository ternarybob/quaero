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

		// Find the job card
		// We look for the card title containing the job name
		// Then find the "Run Job" button within that card's container (parent of parent)

		// XPath: Find div with class 'card-title' containing text, then go up to card
		cardSelector := fmt.Sprintf(`//div[contains(@class, "card-title") and contains(text(), "%s")]/ancestor::div[contains(@class, "card")]`, jobName)

		// Wait for card to be visible
		if err := chromedp.Run(ctx, chromedp.WaitVisible(cardSelector, chromedp.BySearch)); err != nil {
			// Dump body HTML for debugging
			var bodyHTML string
			chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML))
			env.LogTest(t, "Body HTML (Card Not Found): %s", bodyHTML)
			env.TakeScreenshot(ctx, "card_not_found_"+jobName)
			return fmt.Errorf("job card for %s not found: %w", jobName, err)
		}

		// Find the run button within that card
		// The button has title "Run Job" and icon fa-play
		runBtnSelector := cardSelector + `//button[.//i[contains(@class, "fa-play")]]`

		env.LogTest(t, "Clicking Run button for %s", jobName)

		// Debug: Dump the card HTML to see what IDs are actually there
		var cardHTML string
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(cardSelector, chromedp.BySearch),
			chromedp.OuterHTML(cardSelector, &cardHTML, chromedp.BySearch),
		); err == nil {
			env.LogTest(t, "Card HTML: %s", cardHTML)
		}

		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(runBtnSelector, chromedp.BySearch),
			chromedp.Click(runBtnSelector, chromedp.BySearch),
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
	monitorJob := func(jobName string, expectedDocs int) error {
		env.LogTest(t, "Monitoring job: %s", jobName)

		// Navigate to Queue page
		if err := chromedp.Run(ctx, chromedp.Navigate(queueURL)); err != nil {
			return fmt.Errorf("failed to navigate to queue page: %w", err)
		}

		// Wait for job to appear in the list
		// We look for the job name in the queue list
		jobSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]`, jobName)
		if err := chromedp.Run(ctx,
			chromedp.WaitVisible(jobSelector, chromedp.BySearch),
		); err != nil {
			return fmt.Errorf("job %s not found in queue: %w", jobName, err)
		}
		env.LogTest(t, "✓ Job found in queue")

		// Wait for status to be 'completed'
		// The status badge is in the same card.
		// We can poll for the status text associated with this job.
		// XPath: Find title -> ancestor card -> find status label
		statusSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]//span[contains(@class, "label")]//span[text()="Completed"]`, jobName)

		env.LogTest(t, "Waiting for job completion...")
		// Poll for completion (timeout 60s)
		err := chromedp.Run(ctx,
			chromedp.Poll(
				fmt.Sprintf(`document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue != null`, strings.ReplaceAll(statusSelector, "'", "\\'")),
				nil,
				chromedp.WithPollingTimeout(60*time.Second),
				chromedp.WithPollingInterval(1*time.Second),
			),
		)
		if err != nil {
			// Capture screenshot on failure
			env.TakeScreenshot(ctx, "job_failed_"+jobName)
			return fmt.Errorf("job %s did not complete: %w", jobName, err)
		}
		env.LogTest(t, "✓ Job completed")

		// Verify Document Count
		// Selector for document count: ancestor card -> find span with "Documents" text
		// The text is usually "X Documents"
		if expectedDocs > 0 {
			env.LogTest(t, "Verifying document count > 0")
			// We just check if we can find a text matching regex like "\d+ Documents" where digits > 0
			// XPath to find the document count span within the card
			docCountSelector := fmt.Sprintf(`//div[contains(@class, "card-title")]//span[contains(text(), "%s")]/ancestor::div[contains(@class, "card")]//span[contains(text(), "Documents")]`, jobName)

			var docText string
			if err := chromedp.Run(ctx,
				chromedp.Text(docCountSelector, &docText, chromedp.BySearch),
			); err != nil {
				return fmt.Errorf("failed to read document count: %w", err)
			}

			if docText == "0 Documents" || docText == "N/A Documents" {
				return fmt.Errorf("expected documents > 0, got: %s", docText)
			}
			env.LogTest(t, "✓ Document count verified: %s", docText)
		}

		return nil
	}

	// --- Test Scenario 1: Places Job ---
	env.LogTest(t, "--- Starting Scenario 1: Places Job ---")
	placesJobName := "Nearby Restaurants (Wheelers Hill)"

	if err := triggerJob(placesJobName); err != nil {
		t.Fatalf("Failed to trigger places job: %v", err)
	}

	if err := monitorJob(placesJobName, 1); err != nil {
		t.Fatalf("Places job monitoring failed: %v", err)
	}

	// --- Test Scenario 2: Agent Job ---
	// Note: Agent job depends on documents existing (which Places job created)
	env.LogTest(t, "--- Starting Scenario 2: Agent Job ---")
	agentJobName := "Keyword Extraction"

	if err := triggerJob(agentJobName); err != nil {
		t.Fatalf("Failed to trigger agent job: %v", err)
	}

	if err := monitorJob(agentJobName, 0); err != nil { // We expect updates, but exact count might vary
		t.Fatalf("Agent job monitoring failed: %v", err)
	}

	env.LogTest(t, "✓ All scenarios completed successfully")
}
