// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:48:52 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:28:57 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:26:15 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:26:05 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:25:48 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 12:21:52 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 10:21:25 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 10:21:24 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Last Modified: Wednesday, 5th November 2025 10:21:14 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

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

	// Wait for page to be fully loaded (navbar with status indicator)
	env.LogTest(t, "Waiting for page to be fully loaded...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.status-text`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Brief pause for WebSocket to connect
	)
	if err != nil {
		env.TakeScreenshot(ctx, "01-page-load-failed")
		t.Fatalf("Page did not load properly: %v", err)
	}
	env.LogTest(t, "✓ Page loaded successfully")

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
		chromedp.Sleep(2*time.Second),                    // Allow time for data to populate
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

	env.LogTest(t, "Step 5: Executing 'Database Maintenance' job")

	// Override window.confirm to auto-accept
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			window.originalConfirm = window.confirm;
			window.confirm = function() { return true; };
		`, nil),
		chromedp.Sleep(500*time.Millisecond), // Wait for Alpine.js to fully initialize
	)
	if err != nil {
		env.TakeScreenshot(ctx, "05-confirm-override-failed")
		t.Fatalf("Failed to override confirm: %v", err)
	}

	// Click the run button
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
		chromedp.Sleep(3*time.Second), // Wait for API call to complete and notification to appear
	)

	if err != nil {
		env.TakeScreenshot(ctx, "05-execute-failed")
		t.Fatalf("Failed to execute job: %v", err)
	}

	if !clickSuccess {
		env.TakeScreenshot(ctx, "05-button-not-found")
		t.Fatal("REQUIREMENT FAILED: Run button not found")
	}

	env.TakeScreenshot(ctx, "05-job-executed")
	env.LogTest(t, "✓ Job execution triggered")

	// Step 6: Wait for and verify success notification (non-fatal - notification may auto-dismiss)
	env.LogTest(t, "Step 6: Checking for success notification")
	var notificationFound bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(500*time.Millisecond), // Brief wait for notification to appear
		chromedp.Evaluate(`
			// Look for toast notification (uses .toast-item class)
			const notifications = document.querySelectorAll('.toast-item, .notification, .alert, .success-message, [role="alert"]');
			Array.from(notifications).some(notif =>
				notif.textContent.toLowerCase().includes('success') ||
				notif.textContent.toLowerCase().includes('started') ||
				notif.textContent.toLowerCase().includes('queued')
			);
		`, &notificationFound),
	)

	if err == nil && !notificationFound {
		env.TakeScreenshot(ctx, "06-no-success-notification")
		env.LogTest(t, "⚠️  Success notification not found (may have auto-dismissed)")
	} else if err != nil {
		env.TakeScreenshot(ctx, "06-notification-check-failed")
		env.LogTest(t, "⚠️  Failed to check for notification: %v", err)
	} else {
		env.TakeScreenshot(ctx, "06-success-notification-shown")
		env.LogTest(t, "✓ Success notification displayed")
	}

	// Step 7: Navigate to Queue Management page
	env.LogTest(t, "Step 7: Navigating to Queue Management page")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "07-queue-page-load-failed")
		t.Fatalf("Failed to load Queue Management page: %v", err)
	}

	env.TakeScreenshot(ctx, "07-queue-page-loaded")

	// Step 8: Wait for Queue Management page content to load
	env.LogTest(t, "Step 8: Waiting for Queue Management content to load")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`h1, h2, h3`, chromedp.ByQuery), // Wait for page heading
		chromedp.Sleep(2*time.Second),                        // Allow time for jobs to load
	)
	if err != nil {
		env.TakeScreenshot(ctx, "08-queue-page-not-loaded")
		t.Fatalf("Queue Management page not loaded: %v", err)
	}

	// Step 9: Search for "Database Maintenance" job in Queue Management
	env.LogTest(t, "Step 9: Searching for 'Database Maintenance' job in Queue Management")
	var jobInQueue bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Look for Database Maintenance anywhere on the Queue Management page
			// Could be in cards, list items, table rows, or any container
			document.body.textContent.includes('Database Maintenance')
		`, &jobInQueue),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "09-queue-search-failed")
		t.Fatalf("Failed to search Queue Management for job: %v", err)
	}

	env.TakeScreenshot(ctx, "09-queue-search-complete")

	// REQUIREMENT: Job should be listed in Queue Management
	if !jobInQueue {
		env.TakeScreenshot(ctx, "09-job-not-in-queue")
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job not found in Queue Management")
	} else {
		env.LogTest(t, "✓ Found 'Database Maintenance' in Queue Management")
	}

	// Step 10: Verify chevron button is NOT present (removed as per requirements)
	env.LogTest(t, "Step 10: Verifying chevron button is removed")
	var chevronExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			// Look for chevron/expand button (fa-chevron-down or fa-chevron-right)
			document.querySelector('.expand-collapse-btn, .fa-chevron-down, .fa-chevron-right, .fa-angle-down, .fa-angle-right') !== null
		`, &chevronExists),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "10-chevron-check-failed")
		t.Fatalf("Failed to check for chevron button: %v", err)
	}

	// REQUIREMENT: Chevron button should NOT be present
	if chevronExists {
		env.TakeScreenshot(ctx, "10-chevron-still-present")
		t.Error("REQUIREMENT FAILED: Chevron button should be removed but is still present")
	} else {
		env.LogTest(t, "✓ Chevron button removed as expected")
	}

	// Final screenshot showing Queue Management state
	env.TakeScreenshot(ctx, "final-queue-state")

	// Log expected failure
	if !notificationFound || !jobInQueue {
		env.LogTest(t, "⚠️  TEST FAILED AS EXPECTED: Job execution functionality not yet implemented")
	} else {
		env.LogTest(t, "✅ All job execution requirements verified successfully")
	}
}
