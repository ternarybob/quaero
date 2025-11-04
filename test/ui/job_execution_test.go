// -----------------------------------------------------------------------
// Test for basic job execution workflow
// NOTE: This test is EXPECTED TO FAIL as job execution is not yet implemented
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestJobBasicExecution verifies the basic job execution workflow
// NOTE: This test is EXPECTED TO FAIL - job execution is not yet implemented
func TestJobBasicExecution(t *testing.T) {
	env, err := common.SetupTestEnvironment("JobBasicExecution")
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

	// Wait for WebSocket connection on home page
	env.LogTest(t, "Waiting for WebSocket connection on home page...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.TakeScreenshot(ctx, "01-websocket-failed")
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	env.TakeScreenshot(ctx, "01-home-page-ready")

	// Step 2: Navigate to jobs page
	env.LogTest(t, "Step 2: Navigating to jobs page")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "02-jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.TakeScreenshot(ctx, "02-jobs-page-loaded")

	// Step 3: Wait for job definitions section to load
	env.LogTest(t, "Step 3: Waiting for job definitions to load")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`h2, h3`, chromedp.ByQuery), // Wait for heading to appear
		chromedp.Sleep(2*time.Second), // Allow time for data to populate
	)
	if err != nil {
		env.TakeScreenshot(ctx, "03-job-definitions-not-loaded")
		t.Fatalf("Job definitions section not found: %v", err)
	}

	// Step 4: Find "Database Maintenance" job definition
	env.LogTest(t, "Step 4: Finding 'Database Maintenance' job definition")
	var dbMaintenanceExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Look for "Database Maintenance" in the page content
			// It could be in a card, list item, or other container
			document.body.textContent.includes('Database Maintenance')
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

	// Step 5: Check if the run button is enabled and click it
	env.LogTest(t, "Step 5: Waiting for run button to be visible")

	// Wait for the button to be rendered by Alpine.js (ID: database-maintenance-run)
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#database-maintenance-run`, chromedp.ByQuery),
	)

	if err != nil {
		env.TakeScreenshot(ctx, "05-run-button-not-found")
		t.Fatalf("Failed to find run button with ID 'database-maintenance-run': %v", err)
	}

	// Check if the button is disabled
	var isDisabled bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			const btn = document.querySelector('#database-maintenance-run');
			btn ? btn.disabled : true
		`, &isDisabled),
	)

	if err != nil {
		env.TakeScreenshot(ctx, "05-button-state-check-failed")
		t.Fatalf("Failed to check button state: %v", err)
	}

	if isDisabled {
		env.TakeScreenshot(ctx, "05-button-disabled")
		t.Fatal("REQUIREMENT FAILED: Run button is disabled - job definition may not be enabled")
	}

	env.LogTest(t, "Step 5: Clicking run button for 'Database Maintenance'")

	// Set up dialog handler to automatically accept confirm dialogs
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			go func() {
				if err := chromedp.Run(ctx,
					page.HandleJavaScriptDialog(true), // Accept the dialog
				); err != nil {
					env.LogTest(t, "Warning: Failed to handle dialog: %v", err)
				}
			}()
		}
	})

	// Click the button (this will trigger a confirm dialog which we'll auto-accept)
	var clickSuccess bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const btn = document.querySelector('#database-maintenance-run');
				if (!btn) return false;
				btn.click();
				return true;
			})()
		`, &clickSuccess),
		chromedp.Sleep(1*time.Second), // Wait for dialog to be handled and request to be sent
	)

	if err != nil {
		env.TakeScreenshot(ctx, "05-run-button-click-failed")
		t.Fatalf("Failed to execute click script: %v", err)
	}

	if !clickSuccess {
		env.TakeScreenshot(ctx, "05-button-not-found-by-script")
		t.Fatal("REQUIREMENT FAILED: Button not found by click script")
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
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "07-queue-page-load-failed")
		t.Fatalf("Failed to load queue page: %v", err)
	}

	env.TakeScreenshot(ctx, "07-queue-page-loaded")

	// Step 8: Wait for queue page content to load
	env.LogTest(t, "Step 8: Waiting for job queue content to load")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`h1, h2, h3`, chromedp.ByQuery), // Wait for page heading
		chromedp.Sleep(2*time.Second), // Allow time for jobs to load
	)
	if err != nil {
		env.TakeScreenshot(ctx, "08-job-queue-page-not-loaded")
		t.Fatalf("Job queue page not loaded: %v", err)
	}

	// Step 9: Search for "Database Maintenance" job in the queue
	env.LogTest(t, "Step 9: Searching for 'Database Maintenance' job in queue")
	var jobInQueue bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Look for Database Maintenance anywhere on the queue page
			// Could be in cards, list items, table rows, or any container
			document.body.textContent.includes('Database Maintenance')
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
