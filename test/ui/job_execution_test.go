// -----------------------------------------------------------------------
// Test for basic job execution workflow
// NOTE: This test is EXPECTED TO FAIL as job execution is not yet implemented
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestJobBasicExecution verifies the basic job execution workflow
// NOTE: This test is EXPECTED TO FAIL - job execution is not yet implemented
func TestJobBasicExecution(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobBasicExecution")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout for the test
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Set desktop viewport size (1920x1080)
	env.LogTest(t, "Setting desktop viewport size")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
	)
	if err != nil {
		t.Fatalf("Failed to set viewport size: %v", err)
	}

	// Step 1: Load home page to initialize the application
	env.LogTest(t, "Step 1: Loading home page: %s", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Allow Alpine.js and app to initialize
	)
	if err != nil {
		env.TakeScreenshot(ctx, "01-home-page-load-failed")
		t.Fatalf("Failed to load home page: %v", err)
	}

	// Step 2: Navigate to jobs page
	env.LogTest(t, "Step 2: Navigating to jobs page")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs.html"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "02-jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.TakeScreenshot(ctx, "02-jobs-page-loaded")

	// Step 3: Wait for job definitions table to load
	env.LogTest(t, "Step 3: Waiting for job definitions to load")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#job-definitions-table`, chromedp.ByID),
		chromedp.Sleep(2*time.Second), // Allow time for data to populate
	)
	if err != nil {
		env.TakeScreenshot(ctx, "03-job-definitions-not-loaded")
		t.Fatalf("Job definitions table not found: %v", err)
	}

	// Step 4: Find "Database Maintenance" job definition row
	env.LogTest(t, "Step 4: Finding 'Database Maintenance' job definition")
	var dbMaintenanceExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('#job-definitions-table tbody tr')).some(row =>
				row.textContent.includes('Database Maintenance')
			)
		`, &dbMaintenanceExists),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "03-db-maintenance-check-failed")
		t.Fatalf("Failed to check for Database Maintenance: %v", err)
	}

	if !dbMaintenanceExists {
		env.TakeScreenshot(ctx, "04-db-maintenance-not-found")
		t.Fatal("REQUIREMENT FAILED: 'Database Maintenance' job definition not found")
	}

	env.TakeScreenshot(ctx, "04-db-maintenance-found")
	env.LogTest(t, "✓ Found 'Database Maintenance' job definition")

	// Step 5: Click the run button (green arrow) for Database Maintenance
	env.LogTest(t, "Step 5: Clicking run button for 'Database Maintenance'")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Find the row containing "Database Maintenance"
			const row = Array.from(document.querySelectorAll('#job-definitions-table tbody tr'))
				.find(row => row.textContent.includes('Database Maintenance'));

			if (!row) {
				throw new Error('Database Maintenance row not found');
			}

			// Find the run button (green arrow/play icon) in that row
			const runButton = row.querySelector('button[title="Run"], button.run-job, button[onclick*="execute"]');

			if (!runButton) {
				throw new Error('Run button not found in Database Maintenance row');
			}

			// Click the button
			runButton.click();
			true;
		`, nil),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "05-run-button-click-failed")
		t.Fatalf("Failed to click run button: %v", err)
	}

	env.TakeScreenshot(ctx, "05-run-button-clicked")
	env.LogTest(t, "✓ Clicked run button")

	// Step 6: Wait for and verify success notification
	env.LogTest(t, "Step 6: Waiting for success notification")
	var notificationFound bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for notification to appear
		chromedp.Evaluate(`
			// Look for success notification (could be toast, alert, or status message)
			const notifications = document.querySelectorAll('.notification, .toast, .alert, .success-message, [role="alert"]');
			Array.from(notifications).some(notif =>
				notif.textContent.toLowerCase().includes('success') ||
				notif.textContent.toLowerCase().includes('started') ||
				notif.textContent.toLowerCase().includes('queued')
			);
		`, &notificationFound),
	)

	if err == nil && !notificationFound {
		env.TakeScreenshot(ctx, "06-no-success-notification")
		t.Error("REQUIREMENT FAILED: No success notification displayed after clicking run")
	} else if err != nil {
		env.TakeScreenshot(ctx, "06-notification-check-failed")
		t.Logf("Warning: Failed to check for notification: %v", err)
	} else {
		env.TakeScreenshot(ctx, "06-success-notification-shown")
		env.LogTest(t, "✓ Success notification displayed")
	}

	// Step 7: Navigate to queue page
	env.LogTest(t, "Step 7: Navigating to queue page")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue.html"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "07-queue-page-load-failed")
		t.Fatalf("Failed to load queue page: %v", err)
	}

	env.TakeScreenshot(ctx, "07-queue-page-loaded")

	// Step 8: Wait for job queue panel to load
	env.LogTest(t, "Step 8: Waiting for job queue panel")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.job-queue-panel, #job-queue-panel, #jobs-queue, .queue-table`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Allow time for jobs to load
	)
	if err != nil {
		env.TakeScreenshot(ctx, "08-job-queue-panel-not-found")
		t.Fatalf("Job queue panel not found: %v", err)
	}

	// Step 9: Search for "Database Maintenance" job in the queue
	env.LogTest(t, "Step 9: Searching for 'Database Maintenance' job in queue")
	var jobInQueue bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Look for Database Maintenance in job queue table/list
			const queueElements = document.querySelectorAll('.job-queue-panel tbody tr, #job-queue-panel tbody tr, .queue-table tbody tr');
			Array.from(queueElements).some(row =>
				row.textContent.includes('Database Maintenance')
			);
		`, &jobInQueue),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "09-queue-search-failed")
		t.Fatalf("Failed to search queue for job: %v", err)
	}

	env.TakeScreenshot(ctx, "09-queue-search-complete")

	// REQUIREMENT: Job should be listed in queue
	if !jobInQueue {
		env.TakeScreenshot(ctx, "09-job-not-in-queue")
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job not found in job queue")
	} else {
		env.LogTest(t, "✓ Found 'Database Maintenance' in job queue")
	}

	// Final screenshot showing queue state
	env.TakeScreenshot(ctx, "final-queue-state")

	// Log expected failure
	if !notificationFound || !jobInQueue {
		env.LogTest(t, "⚠️  TEST FAILED AS EXPECTED: Job execution functionality not yet implemented")
	} else {
		env.LogTest(t, "✅ All job execution requirements verified successfully")
	}
}
