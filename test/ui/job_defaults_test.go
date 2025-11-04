// -----------------------------------------------------------------------
// Test for default job definitions created on service startup
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestJobDefaultDefinitions verifies that 2 default job definitions are created when service starts
func TestJobDefaultDefinitions(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefaultDefinitions")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout for the test
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
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

	// First load the home page to initialize the application
	env.LogTest(t, "Loading home page: %s", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Allow Alpine.js and app to initialize
	)
	if err != nil {
		env.TakeScreenshot(ctx, "home-page-load-failed")
		t.Fatalf("Failed to load home page: %v", err)
	}

	// Navigate to jobs page
	env.LogTest(t, "Navigating to jobs page: %s/jobs.html", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs.html"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to fully load
	)
	if err != nil {
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.TakeScreenshot(ctx, "jobs-page-loaded")

	// Wait for job definitions table to load
	env.LogTest(t, "Waiting for job definitions table to load")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#job-definitions-table`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Allow time for data to load
	)
	if err != nil {
		env.TakeScreenshot(ctx, "job-definitions-table-not-found")
		t.Fatalf("Job definitions table not found: %v", err)
	}

	// Count job definition rows (excluding header)
	var rowCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('#job-definitions-table tbody tr').length`, &rowCount),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "count-rows-failed")
		t.Fatalf("Failed to count job definition rows: %v", err)
	}

	env.LogTest(t, "Found %d job definitions", rowCount)

	// REQUIREMENT: There should be exactly 2 default job definitions
	if rowCount != 2 {
		env.TakeScreenshot(ctx, "incorrect-job-count")
		t.Fatalf("REQUIREMENT FAILED: Expected 2 default job definitions, found %d", rowCount)
	}

	// Verify the first job definition exists (Database Maintenance)
	var hasDbMaintenance bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('#job-definitions-table tbody tr')).some(row =>
				row.textContent.includes('Database Maintenance')
			)
		`, &hasDbMaintenance),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "check-db-maintenance-failed")
		t.Fatalf("Failed to check for Database Maintenance job: %v", err)
	}

	if !hasDbMaintenance {
		env.TakeScreenshot(ctx, "db-maintenance-not-found")
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job definition not found")
	} else {
		env.LogTest(t, "✓ Found 'Database Maintenance' job definition")
	}

	// Verify the second job definition exists (System Health Check)
	var hasSystemHealth bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('#job-definitions-table tbody tr')).some(row =>
				row.textContent.includes('System Health Check')
			)
		`, &hasSystemHealth),
	)
	if err != nil {
		env.TakeScreenshot(ctx, "check-system-health-failed")
		t.Fatalf("Failed to check for System Health Check job: %v", err)
	}

	if !hasSystemHealth {
		env.TakeScreenshot(ctx, "system-health-not-found")
		t.Error("REQUIREMENT FAILED: 'System Health Check' job definition not found")
	} else {
		env.LogTest(t, "✓ Found 'System Health Check' job definition")
	}

	env.TakeScreenshot(ctx, "job-defaults-verified")

	if !hasDbMaintenance || !hasSystemHealth {
		t.Fatal("REQUIREMENT FAILED: Not all default job definitions are present")
	}

	env.LogTest(t, "✅ All default job definitions verified successfully")
}
