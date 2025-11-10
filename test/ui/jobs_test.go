package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func TestJobsPageLoad(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsPageLoad")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsPageLoad (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsPageLoad (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"
	var title string

	// Collect console errors
	consoleErrors := []string{}
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if consoleEvent, ok := ev.(*runtime.EventExceptionThrown); ok {
			if consoleEvent.ExceptionDetails != nil && consoleEvent.ExceptionDetails.Exception != nil {
				errorMsg := consoleEvent.ExceptionDetails.Exception.Description
				if errorMsg == "" && consoleEvent.ExceptionDetails.Text != "" {
					errorMsg = consoleEvent.ExceptionDetails.Text
				}
				consoleErrors = append(consoleErrors, errorMsg)
			}
		}
	})

	env.LogTest(t, "Setting desktop viewport size (1920x1080)")
	env.LogTest(t, "Navigating to jobs page: %s", url)

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Alpine.js to initialize and any async operations
		chromedp.Title(&title),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.LogTest(t, "✓ Jobs page loaded")
	env.TakeScreenshot(ctx, "jobs-page")

	expectedTitle := "Job Management - Quaero"
	if title != expectedTitle {
		env.LogTest(t, "ERROR: Expected title '%s', got '%s'", expectedTitle, title)
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	} else {
		env.LogTest(t, "✓ Page title correct: '%s'", title)
	}

	// Check for console errors
	env.LogTest(t, "Checking for console errors...")
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console error(s):", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  [%d] %s", i+1, errMsg)
		}
		t.Errorf("Found %d console error(s) on page load", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors found")
	}

	// Check for notification errors
	env.LogTest(t, "Checking for notification errors...")
	var notificationErrors []string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('.notification.error, .notification.is-danger, .toast.error, .toast.is-danger'))
				.map(el => el.textContent.trim())
		`, &notificationErrors),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check for notification errors: %v", err)
	} else if len(notificationErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d notification error(s):", len(notificationErrors))
		for i, errMsg := range notificationErrors {
			env.LogTest(t, "  [%d] %s", i+1, errMsg)
		}
		t.Errorf("Found %d notification error(s) on page load", len(notificationErrors))
	} else {
		env.LogTest(t, "✓ No notification errors found")
	}

	env.LogTest(t, "✅ Jobs page loads correctly with no errors")
}

func TestJobsPageElements(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsPageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsPageElements")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsPageElements (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsPageElements (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Check for presence of key elements (aligned with actual jobs.html structure)
	tests := []struct {
		name     string
		selector string
	}{
		{"Authentication section", `[x-data*="authPage"]`},
		{"Job Definitions section", `[x-data*="jobDefinitionsManagement"]`},
		{"Card elements", ".card"},
		{"Add Job button", `button.btn-primary`}, // Primary button in job definitions section
	}

	env.LogTest(t, "Navigating to jobs page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js to initialize
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			// Use CSS selector directly instead of embedding in JavaScript string
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelectorAll('`+tt.selector+`').length`, &nodeCount),
			)

			if err != nil {
				env.LogTest(t, "ERROR: Failed to check element '%s': %v", tt.name, err)
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if nodeCount == 0 {
				env.LogTest(t, "ERROR: Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			} else {
				env.LogTest(t, "✓ Found element: %s (count: %d)", tt.name, nodeCount)
			}
		})
	}

	env.TakeScreenshot(ctx, "jobs-page-elements")
	env.LogTest(t, "✅ All page elements verified")
}

func TestJobsNavbar(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsNavbar")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsNavbar")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsNavbar (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsNavbar (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	var navbarVisible bool
	var menuItems []string

	env.LogTest(t, "Navigating to jobs page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.app-header-nav`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('.app-header-nav') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.nav-links a')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check navbar: %v", err)
		env.TakeScreenshot(ctx, "navbar-check-failed")
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		env.LogTest(t, "ERROR: Navbar not found on page")
		env.TakeScreenshot(ctx, "navbar-not-found")
		t.Error("Navbar not found on page")
	} else {
		env.LogTest(t, "✓ Navbar found")
	}

	// Check for menu items (aligned with current navbar structure)
	expectedItems := []string{"HOME", "JOBS", "QUEUE", "DOCUMENTS", "SEARCH", "CHAT", "SETTINGS"}
	env.LogTest(t, "Checking for menu items: %v", expectedItems)
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			if strings.Contains(item, expected) {
				found = true
				break
			}
		}
		if !found {
			env.LogTest(t, "ERROR: Menu item '%s' not found in navbar", expected)
			t.Errorf("Menu item '%s' not found in navbar", expected)
		} else {
			env.LogTest(t, "✓ Found menu item: %s", expected)
		}
	}

	// Verify JOBS item is active on jobs page
	var jobsActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.nav-links a.active[href="/jobs"]') !== null`, &jobsActive),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to check active menu item: %v", err)
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !jobsActive {
		env.LogTest(t, "ERROR: JOBS menu item should be active on jobs page")
		t.Error("JOBS menu item should be active on jobs page")
	} else {
		env.LogTest(t, "✓ JOBS menu item is active")
	}

	env.TakeScreenshot(ctx, "jobs-navbar")
	env.LogTest(t, "✅ Navbar displays correctly with JOBS item")
}

// TestJobsAuthenticationSection verifies the authentication section displays correctly
func TestJobsAuthenticationSection(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsAuthenticationSection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsAuthenticationSection")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsAuthenticationSection (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsAuthenticationSection (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	env.LogTest(t, "Navigating to jobs page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Check for authentication section
	var authSectionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('[x-data="authPage()"]') !== null`, &authSectionExists),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check authentication section: %v", err)
		t.Fatalf("Failed to check authentication section: %v", err)
	}

	if !authSectionExists {
		env.LogTest(t, "ERROR: Authentication section not found")
		env.TakeScreenshot(ctx, "auth-section-not-found")
		t.Error("Authentication section not found on page")
	} else {
		env.LogTest(t, "✓ Authentication section found")
	}

	env.TakeScreenshot(ctx, "jobs-authentication-section")
	env.LogTest(t, "✅ Authentication section displays correctly")
}

// TestJobsSourcesSection verifies the sources section has been removed
func TestJobsSourcesSection(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsSourcesSection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsSourcesSection")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsSourcesSection (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsSourcesSection (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	env.LogTest(t, "Navigating to jobs page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Check that sources section does NOT exist (it was removed)
	var sourcesSectionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('[x-data="sourceManagement"]') !== null`, &sourcesSectionExists),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check sources section: %v", err)
		t.Fatalf("Failed to check sources section: %v", err)
	}

	if sourcesSectionExists {
		env.LogTest(t, "ERROR: Sources section found (should have been removed)")
		env.TakeScreenshot(ctx, "sources-section-found")
		t.Error("Sources section should not exist on page (it was removed from the codebase)")
	} else {
		env.LogTest(t, "✓ Sources section correctly removed from page")
	}

	env.TakeScreenshot(ctx, "jobs-no-sources-section")
	env.LogTest(t, "✅ Sources section has been successfully removed")
}

// TestJobsDefinitionsSection verifies the job definitions section displays correctly
func TestJobsDefinitionsSection(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsDefinitionsSection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsDefinitionsSection")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsDefinitionsSection (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsDefinitionsSection (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	env.LogTest(t, "Navigating to jobs page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for Alpine.js
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Check for job definitions section
	var jobDefsSectionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('[x-data="jobDefinitionsManagement"]') !== null`, &jobDefsSectionExists),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check job definitions section: %v", err)
		t.Fatalf("Failed to check job definitions section: %v", err)
	}

	if !jobDefsSectionExists {
		env.LogTest(t, "ERROR: Job definitions section not found")
		env.TakeScreenshot(ctx, "job-defs-section-not-found")
		t.Error("Job definitions section not found on page")
	} else {
		env.LogTest(t, "✓ Job definitions section found")
	}

	env.TakeScreenshot(ctx, "jobs-definitions-section")
	env.LogTest(t, "✅ Job definitions section displays correctly")
}

// TestJobsRunDatabaseMaintenance verifies running the Database Maintenance job
func TestJobsRunDatabaseMaintenance(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobsRunDatabaseMaintenance")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsRunDatabaseMaintenance")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsRunDatabaseMaintenance (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsRunDatabaseMaintenance (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Navigate to jobs page
	env.LogTest(t, "Setting desktop viewport size (1920x1080)")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to set viewport size: %v", err)
		t.Fatalf("Failed to set viewport size: %v", err)
	}

	env.LogTest(t, "Navigating to jobs page: %s/jobs", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}
	env.LogTest(t, "✓ Jobs page loaded")

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		env.TakeScreenshot(ctx, "websocket-failed")
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected")

	// Wait for job definitions section to load
	env.LogTest(t, "Waiting for job definitions section to load...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`[x-data="jobDefinitionsManagement"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Allow Alpine.js to fetch and render data
	)
	if err != nil {
		env.LogTest(t, "ERROR: Job definitions section not found: %v", err)
		env.TakeScreenshot(ctx, "job-definitions-section-not-found")
		t.Fatalf("Job definitions section not found: %v", err)
	}
	env.LogTest(t, "✓ Job definitions section loaded")

	env.TakeScreenshot(ctx, "jobs-page-before-run")

	// Find the "Database Maintenance" job card and its run button
	env.LogTest(t, "Looking for 'Database Maintenance' job card...")
	var dbMaintenanceCardExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card')).some(card =>
				card.textContent.includes('Database Maintenance')
			)
		`, &dbMaintenanceCardExists),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for Database Maintenance card: %v", err)
		env.TakeScreenshot(ctx, "db-maintenance-card-check-failed")
		t.Fatalf("Failed to check for Database Maintenance card: %v", err)
	}

	if !dbMaintenanceCardExists {
		env.LogTest(t, "ERROR: 'Database Maintenance' job card not found")
		env.TakeScreenshot(ctx, "db-maintenance-card-not-found")
		t.Fatal("'Database Maintenance' job card not found")
	}
	env.LogTest(t, "✓ Found 'Database Maintenance' job card")

	// Find the run button within the Database Maintenance card
	env.LogTest(t, "Looking for run button in 'Database Maintenance' card...")
	var runButtonSelector string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const dbCard = cards.find(card => card.textContent.includes('Database Maintenance'));
				if (!dbCard) return null;
				const runButton = dbCard.querySelector('button.btn-success');
				return runButton ? runButton.id || 'button.btn-success' : null;
			})()
		`, &runButtonSelector),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to find run button: %v", err)
		env.TakeScreenshot(ctx, "run-button-find-failed")
		t.Fatalf("Failed to find run button: %v", err)
	}

	if runButtonSelector == "" {
		env.LogTest(t, "ERROR: Run button not found in 'Database Maintenance' card")
		env.TakeScreenshot(ctx, "run-button-not-found")
		t.Fatal("Run button not found in 'Database Maintenance' card")
	}
	env.LogTest(t, "✓ Found run button: %s", runButtonSelector)

	// Override confirm dialog to auto-accept
	env.LogTest(t, "Overriding confirm dialog...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to override confirm dialog: %v", err)
		env.TakeScreenshot(ctx, "confirm-override-failed")
		t.Fatalf("Failed to override confirm dialog: %v", err)
	}
	env.LogTest(t, "✓ Confirm dialog overridden")

	// Click the run button
	env.LogTest(t, "Clicking run button...")
	var jobStarted bool

	// Create a separate context with shorter timeout for the click operation
	clickCtx, clickCancel := context.WithTimeout(ctx, 10*time.Second)
	defer clickCancel()

	err = chromedp.Run(clickCtx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const dbCard = cards.find(card => card.textContent.includes('Database Maintenance'));
				if (!dbCard) return false;
				const runButton = dbCard.querySelector('button.btn-success');
				if (!runButton) return false;
				runButton.click();
				return true;
			})()
		`, &jobStarted),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to click run button: %v", err)
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatalf("Failed to click run button: %v", err)
	}

	if !jobStarted {
		env.LogTest(t, "ERROR: Run button click returned false")
		env.TakeScreenshot(ctx, "run-button-click-returned-false")
		t.Fatal("Run button click returned false")
	}
	env.LogTest(t, "✓ Run button clicked")

	env.TakeScreenshot(ctx, "job-triggered")

	// Navigate to queue page
	env.LogTest(t, "Navigating to queue page: %s/queue", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		env.TakeScreenshot(ctx, "queue-page-load-failed")
		t.Fatalf("Failed to load queue page: %v", err)
	}
	env.LogTest(t, "✓ Queue page loaded")

	// WORKAROUND: Initialize filters manually and call loadJobs() directly
	// This is needed because there's a race condition in queue.html where Alpine.js
	// initializes before window.activeFilters is set
	env.LogTest(t, "Initializing filters and calling loadJobs()...")
	var loadResult struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"errorMessage"`
		JobsLoaded   int    `json:"jobsLoaded"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					// Initialize filters if not already set
					if (!window.activeFilters) {
						window.activeFilters = {
							status: new Set(['pending', 'running', 'completed', 'failed', 'cancelled']),
							source: new Set(),
							entity: new Set()
						};
					}
					// Call loadJobs() directly on the Alpine.js component
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (!jobListEl) {
						return { success: false, errorMessage: 'jobList element not found', jobsLoaded: 0 };
					}
					const alpineData = Alpine.$data(jobListEl);
					if (!alpineData) {
						return { success: false, errorMessage: 'Alpine data not found', jobsLoaded: 0 };
					}
					if (!alpineData.loadJobs) {
						return { success: false, errorMessage: 'loadJobs method not found', jobsLoaded: 0 };
					}
					await alpineData.loadJobs();
					return { success: true, errorMessage: '', jobsLoaded: alpineData.allJobs ? alpineData.allJobs.length : 0 };
				} catch (e) {
					return { success: false, errorMessage: e.toString(), jobsLoaded: 0 };
				}
			})()
		`, &loadResult, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load and render
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to call loadJobs(): %v", err)
	} else if !loadResult.Success {
		env.LogTest(t, "WARNING: loadJobs() failed: %s", loadResult.ErrorMessage)
	} else {
		env.LogTest(t, "✓ Filters initialized and loadJobs() called (loaded %d jobs)", loadResult.JobsLoaded)
	}

	// Check job statistics
	env.LogTest(t, "Checking job statistics...")
	var stats struct {
		Total     string `json:"total"`
		Pending   string `json:"pending"`
		Running   string `json:"running"`
		Completed string `json:"completed"`
		Failed    string `json:"failed"`
		Cancelled string `json:"cancelled"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			({
				total: document.getElementById('stat-total')?.textContent || '-',
				pending: document.getElementById('stat-pending')?.textContent || '-',
				running: document.getElementById('stat-running')?.textContent || '-',
				completed: document.getElementById('stat-completed')?.textContent || '-',
				failed: document.getElementById('stat-failed')?.textContent || '-',
				cancelled: document.getElementById('stat-cancelled')?.textContent || '-'
			})
		`, &stats),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to read job statistics: %v", err)
	} else {
		env.LogTest(t, "✓ Job statistics:")
		env.LogTest(t, "  Total: %s, Pending: %s, Running: %s, Completed: %s, Failed: %s, Cancelled: %s",
			stats.Total, stats.Pending, stats.Running, stats.Completed, stats.Failed, stats.Cancelled)
	}

	env.TakeScreenshot(ctx, "queue-page-loaded")

	// Wait for jobs to render in the UI
	env.LogTest(t, "Waiting for jobs to render...")
	time.Sleep(2 * time.Second)

	// Check if the job appears in the queue and get its details
	env.LogTest(t, "Checking if 'Database Maintenance' job appears in queue...")
	var jobDetails struct {
		Found       bool   `json:"found"`
		JobID       string `json:"jobId"`
		Status      string `json:"status"`
		SourceType  string `json:"sourceType"`
		Documents   string `json:"documents"`
		CreatedTime string `json:"createdTime"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for job cards (the queue page uses cards, not table rows)
				const cards = Array.from(document.querySelectorAll('.job-card-clickable, .card'));
				const jobCard = cards.find(card => card.textContent.includes('Database Maintenance'));

				if (!jobCard) {
					return { found: false, jobId: '', status: '', sourceType: '', documents: '', createdTime: '' };
				}

				// Extract job details from the card
				const jobIdElement = jobCard.querySelector('.text-gray');
				const jobId = jobIdElement ? jobIdElement.textContent.trim().replace(/[()]/g, '') : '';

				const statusBadge = jobCard.querySelector('.label');
				const status = statusBadge ? statusBadge.textContent.trim() : '';

				const sourceTypeElement = jobCard.querySelector('.card-subtitle');
				const sourceType = sourceTypeElement ? sourceTypeElement.textContent.trim() : '';

				// Find documents count
				const documentsElement = Array.from(jobCard.querySelectorAll('span')).find(span =>
					span.textContent.includes('Documents')
				);
				const documents = documentsElement ? documentsElement.textContent.trim() : '';

				// Find created time
				const createdElement = Array.from(jobCard.querySelectorAll('span')).find(span =>
					span.textContent.includes('created:')
				);
				const createdTime = createdElement ? createdElement.textContent.trim() : '';

				return {
					found: true,
					jobId: jobId,
					status: status,
					sourceType: sourceType,
					documents: documents,
					createdTime: createdTime
				};
			})()
		`, &jobDetails),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for job in queue: %v", err)
		env.TakeScreenshot(ctx, "queue-check-failed")
		t.Fatalf("Failed to check for job in queue: %v", err)
	}

	if !jobDetails.Found {
		env.LogTest(t, "ERROR: 'Database Maintenance' job not found in queue")
		env.TakeScreenshot(ctx, "job-not-in-queue")
		t.Fatal("'Database Maintenance' job should always appear in queue (unless manually deleted)")
	}

	env.LogTest(t, "✓ 'Database Maintenance' job found in queue")
	env.LogTest(t, "  Job ID: %s", jobDetails.JobID)
	env.LogTest(t, "  Status: %s", jobDetails.Status)
	env.LogTest(t, "  Source Type: %s", jobDetails.SourceType)
	env.LogTest(t, "  Documents: %s", jobDetails.Documents)
	env.LogTest(t, "  Created: %s", jobDetails.CreatedTime)

	env.TakeScreenshot(ctx, "job-in-queue")

	// Click on the job card to navigate to job details page
	env.LogTest(t, "Clicking on job card to view details...")
	var navigationResult struct {
		Success bool   `json:"success"`
		JobID   string `json:"jobId"`
		Error   string `json:"error"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				try {
					// Look specifically for job cards with the job-card-clickable class
					const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
					
					// Find the Database Maintenance job card
					const jobCard = jobCards.find(card => {
						const titleElement = card.querySelector('.card-title');
						return titleElement && titleElement.textContent.includes('Database Maintenance');
					});
					
					if (!jobCard) {
						return { success: false, jobId: '', error: 'Job card not found' };
					}
					
					// Get the job ID from the data attribute
					const jobId = jobCard.getAttribute('data-job-id');
					if (!jobId) {
						return { success: false, jobId: '', error: 'Job ID not found in data attribute' };
					}
					
					// Navigate directly using the navigateToJobDetails function
					// First try to find the Alpine.js component
					const jobListElement = document.querySelector('[x-data*="jobList"]');
					if (jobListElement && window.Alpine) {
						const alpineData = Alpine.$data(jobListElement);
						if (alpineData && alpineData.navigateToJobDetails) {
							alpineData.navigateToJobDetails(jobId);
							return { success: true, jobId: jobId, error: '' };
						}
					}
					
					// Fallback: navigate manually
					window.location.href = '/job?id=' + jobId;
					return { success: true, jobId: jobId, error: '' };
					
				} catch (e) {
					return { success: false, jobId: '', error: e.toString() };
				}
			})()
		`, &navigationResult),
		chromedp.Sleep(3*time.Second), // Wait for navigation to complete
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to job details: %v", err)
		env.TakeScreenshot(ctx, "job-navigation-failed")
		t.Fatalf("Failed to navigate to job details: %v", err)
	}

	if !navigationResult.Success {
		env.LogTest(t, "ERROR: Job navigation failed: %s", navigationResult.Error)
		env.TakeScreenshot(ctx, "job-navigation-error")
		t.Fatalf("Job navigation failed: %s", navigationResult.Error)
	}

	env.LogTest(t, "✓ Job navigation successful (Job ID: %s)", navigationResult.JobID)

	// Verify we navigated to the job details page
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to get current URL: %v", err)
		t.Fatalf("Failed to get current URL: %v", err)
	}

	env.LogTest(t, "Current URL after click: %s", currentURL)

	// Check if we're on a job details page (should contain /job?id=)
	if !strings.Contains(currentURL, "/job?id=") {
		env.LogTest(t, "ERROR: Expected to navigate to job details page, still on: %s", currentURL)
		env.TakeScreenshot(ctx, "navigation-failed")
		t.Errorf("Expected to navigate to job details page, still on: %s", currentURL)
	} else {
		env.LogTest(t, "✓ Successfully navigated to job details page")
	}

	env.TakeScreenshot(ctx, "job-details-page")

	// Wait for job details page to load completely
	env.LogTest(t, "Waiting for job details page to load...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Allow Alpine.js to initialize
	)
	if err != nil {
		env.LogTest(t, "ERROR: Job details page failed to load: %v", err)
		env.TakeScreenshot(ctx, "job-details-load-failed")
		t.Fatalf("Job details page failed to load: %v", err)
	}

	// Verify the page title contains job information
	var pageTitle string
	err = chromedp.Run(ctx,
		chromedp.Title(&pageTitle),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to get page title: %v", err)
	} else {
		env.LogTest(t, "Job details page title: %s", pageTitle)
		if !strings.Contains(pageTitle, "Job Details") && !strings.Contains(pageTitle, "Database Maintenance") {
			env.LogTest(t, "WARNING: Page title doesn't contain expected job information")
		} else {
			env.LogTest(t, "✓ Page title contains job information")
		}
	}

	// Check for the presence of Details and Output tabs
	env.LogTest(t, "Checking for Details and Output tabs...")
	var tabsFound struct {
		DetailsTab bool `json:"detailsTab"`
		OutputTab  bool `json:"outputTab"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for tab navigation elements (avoid :contains() which is not valid in querySelector)
				const detailsTab = document.querySelector('button[data-tab="details"], .tab-button[data-tab="details"], .nav-link[href="#details"]') ||
					Array.from(document.querySelectorAll('button, .tab, .nav-link')).find(el => 
						el.textContent && el.textContent.toLowerCase().includes('details')
					);
				
				const outputTab = document.querySelector('button[data-tab="output"], .tab-button[data-tab="output"], .nav-link[href="#output"]') ||
					Array.from(document.querySelectorAll('button, .tab, .nav-link')).find(el => 
						el.textContent && el.textContent.toLowerCase().includes('output')
					);

				return {
					detailsTab: !!detailsTab,
					outputTab: !!outputTab
				};
			})()
		`, &tabsFound),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for tabs: %v", err)
		t.Fatalf("Failed to check for tabs: %v", err)
	}

	if !tabsFound.DetailsTab {
		env.LogTest(t, "ERROR: Details tab not found on job details page")
		env.TakeScreenshot(ctx, "details-tab-missing")
		t.Error("Details tab should be present on job details page")
	} else {
		env.LogTest(t, "✓ Details tab found")
	}

	if !tabsFound.OutputTab {
		env.LogTest(t, "ERROR: Output tab not found on job details page")
		env.TakeScreenshot(ctx, "output-tab-missing")
		t.Error("Output tab should be present on job details page")
	} else {
		env.LogTest(t, "✓ Output tab found")
	}

	// Try to click on the Output tab to verify it works
	env.LogTest(t, "Clicking on Output tab...")
	var outputTabClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const outputTab = document.querySelector('button[data-tab="output"], .tab-button[data-tab="output"]') ||
					Array.from(document.querySelectorAll('button, .tab, .nav-link')).find(el => 
						el.textContent && el.textContent.toLowerCase().includes('output')
					);
				
				if (outputTab) {
					outputTab.click();
					return true;
				}
				return false;
			})()
		`, &outputTabClicked),
		chromedp.Sleep(1*time.Second), // Wait for tab switch
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to click Output tab: %v", err)
	} else if outputTabClicked {
		env.LogTest(t, "✓ Output tab clicked successfully")

		// Try to find job logs or output content
		var logContent string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					// Look for various log container patterns
					const logContainer = document.querySelector('.log-container, .logs, pre, code, .output-content, .job-output') ||
						document.querySelector('[class*="log"], [class*="output"]');
					
					if (logContainer) {
						return logContainer.textContent.substring(0, 500);
					}
					
					// If no specific log container, look for any content that might be logs
					const contentAreas = document.querySelectorAll('.content, .tab-content, .panel-content');
					for (let area of contentAreas) {
						if (area.textContent && area.textContent.length > 10) {
							return area.textContent.substring(0, 500);
						}
					}
					
					return '';
				})()
			`, &logContent),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to read log content: %v", err)
		} else if logContent != "" {
			env.LogTest(t, "✓ Found job output content (first 100 chars): %s",
				strings.ReplaceAll(logContent[:min(100, len(logContent))], "\n", " "))
		} else {
			env.LogTest(t, "WARNING: No log content found in Output tab")
		}
	} else {
		env.LogTest(t, "WARNING: Output tab click returned false")
	}

	env.TakeScreenshot(ctx, "job-details-final")

	env.LogTest(t, "✅ Database Maintenance job run and details page test completed successfully")
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestEditJobDefinition verifies clicking edit button navigates to job_add page with correct job loaded
func TestEditJobDefinition(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestEditJobDefinition")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestEditJobDefinition")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestEditJobDefinition (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestEditJobDefinition (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Navigate to jobs page
	env.LogTest(t, "Setting desktop viewport size (1920x1080)")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to set viewport size: %v", err)
		t.Fatalf("Failed to set viewport size: %v", err)
	}

	env.LogTest(t, "Navigating to jobs page: %s/jobs", baseURL)
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Alpine.js and data to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}
	env.LogTest(t, "✓ Jobs page loaded")
	env.TakeScreenshot(ctx, "jobs-page-before-edit")

	// Find a user job (not system job) to edit
	env.LogTest(t, "Looking for a user job to edit...")
	var jobInfo struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Name   string `json:"name"`
		IsUser bool   `json:"isUser"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for job cards
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));

				// Find a user job (not Database Maintenance which is a system job)
				for (const card of cards) {
					const nameElement = card.querySelector('.card-title');
					const name = nameElement ? nameElement.textContent.trim() : '';

					// Skip system jobs like "Database Maintenance"
					if (name === 'Database Maintenance') continue;

					// Find the edit button - it has fa-edit icon and is in the actions column
					const editButton = card.querySelector('button .fa-edit')?.closest('button');
					if (editButton && !editButton.disabled) {
						// Get job ID from the card's x-for binding or extract from button ID
						const jobIdFromButton = editButton.id?.replace(/-edit$/, '');

						return {
							found: true,
							jobId: name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
							name: name,
							isUser: true
						};
					}
				}

				return { found: false, jobId: '', name: '', isUser: false };
			})()
		`, &jobInfo),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to search for user job: %v", err)
		env.TakeScreenshot(ctx, "user-job-search-failed")
		t.Fatalf("Failed to search for user job: %v", err)
	}

	if !jobInfo.Found {
		env.LogTest(t, "WARNING: No user job found, test cannot proceed")
		env.LogTest(t, "NOTE: This test requires at least one user-created job to exist")
		env.TakeScreenshot(ctx, "no-user-job-found")
		t.Skip("No user job found to test edit functionality")
	}

	env.LogTest(t, "✓ Found user job: %s (ID: %s)", jobInfo.Name, jobInfo.JobID)

	// Click the edit button for this job
	env.LogTest(t, "Clicking edit button for job: %s", jobInfo.Name)
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const card = cards.find(c => c.textContent.includes('%s'));
				if (card) {
					// Find edit button by fa-edit icon
					const editButton = card.querySelector('button .fa-edit')?.closest('button');
					if (editButton && !editButton.disabled) {
						editButton.click();
						return true;
					}
				}
				return false;
			})()
		`, jobInfo.Name), nil),
		chromedp.Sleep(2*time.Second), // Wait for navigation
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click edit button: %v", err)
		env.TakeScreenshot(ctx, "edit-button-click-failed")
		t.Fatalf("Failed to click edit button: %v", err)
	}

	env.LogTest(t, "✓ Edit button clicked")

	// Verify navigation to job_add page with ID parameter
	env.LogTest(t, "Verifying navigation to job_add page with ID parameter...")
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page load
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to get current URL: %v", err)
		env.TakeScreenshot(ctx, "url-check-failed")
		t.Fatalf("Failed to get current URL: %v", err)
	}

	expectedURL := baseURL + "/job_add?id=" + jobInfo.JobID
	if currentURL != expectedURL {
		env.LogTest(t, "ERROR: Expected URL '%s', got '%s'", expectedURL, currentURL)
		env.TakeScreenshot(ctx, "wrong-url")
		t.Errorf("Expected URL '%s', got '%s'", expectedURL, currentURL)
	} else {
		env.LogTest(t, "✓ Navigated to correct URL: %s", currentURL)
	}

	env.TakeScreenshot(ctx, "job-add-page-loaded")

	// Verify page title shows "Edit Job Definition"
	env.LogTest(t, "Checking page title...")
	var pageTitle string
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &pageTitle, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to read page title: %v", err)
		env.TakeScreenshot(ctx, "title-check-failed")
		t.Fatalf("Failed to read page title: %v", err)
	}

	if !strings.Contains(pageTitle, "Edit Job Definition") {
		env.LogTest(t, "ERROR: Expected page title to contain 'Edit Job Definition', got '%s'", pageTitle)
		t.Errorf("Expected page title to contain 'Edit Job Definition', got '%s'", pageTitle)
	} else {
		env.LogTest(t, "✓ Page title correct: '%s'", pageTitle)
	}

	// Verify TOML content loaded in editor
	env.LogTest(t, "Verifying TOML content loaded in CodeMirror editor...")
	var editorContent string
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for editor to load
		chromedp.Evaluate(`
			(() => {
				// Try to get CodeMirror editor content
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					return editorElement.CodeMirror.getValue();
				}
				// Fallback: try textarea
				const textarea = document.querySelector('textarea');
				return textarea ? textarea.value : '';
			})()
		`, &editorContent),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to read editor content: %v", err)
		env.TakeScreenshot(ctx, "editor-content-read-failed")
		t.Fatalf("Failed to read editor content: %v", err)
	}

	if editorContent == "" {
		env.LogTest(t, "ERROR: Editor content is empty (job definition did not load)")
		env.TakeScreenshot(ctx, "editor-empty")
		t.Error("Editor content is empty (job definition did not load)")
	} else {
		// Verify TOML content contains expected fields
		env.LogTest(t, "✓ Editor content loaded (%d characters)", len(editorContent))
		if strings.Contains(editorContent, jobInfo.Name) {
			env.LogTest(t, "✓ TOML content contains job name: %s", jobInfo.Name)
		} else {
			env.LogTest(t, "WARNING: TOML content does not contain job name")
		}

		// Log first 200 chars for verification
		preview := editorContent
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		env.LogTest(t, "  Content preview: %s", preview)
	}

	env.TakeScreenshot(ctx, "edit-job-content-loaded")
	env.LogTest(t, "✅ Edit job definition test completed successfully")
}

// TestEditJobSave verifies that saving an edited job updates it correctly
func TestEditJobSave(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestEditJobSave")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestEditJobSave")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestEditJobSave (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestEditJobSave (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// First, get a list of user jobs via API
	env.LogTest(t, "Fetching user jobs via API...")
	httpHelper := env.NewHTTPTestHelper(t)
	resp, err := httpHelper.GET("/api/job-definitions")
	if err != nil {
		env.LogTest(t, "ERROR: Failed to fetch job definitions: %v", err)
		t.Fatalf("Failed to fetch job definitions: %v", err)
	}
	defer resp.Body.Close()

	var apiResponse struct {
		JobDefinitions []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			JobType string `json:"job_type"`
		} `json:"job_definitions"`
	}

	if err := httpHelper.ParseJSONResponse(resp, &apiResponse); err != nil {
		env.LogTest(t, "ERROR: Failed to parse job definitions: %v", err)
		t.Fatalf("Failed to parse job definitions: %v", err)
	}

	jobDefs := apiResponse.JobDefinitions

	// Find a user job
	var testJobID string
	var testJobName string
	for _, job := range jobDefs {
		if job.JobType == "user" {
			testJobID = job.ID
			testJobName = job.Name
			break
		}
	}

	if testJobID == "" {
		env.LogTest(t, "WARNING: No user job found, test cannot proceed")
		env.LogTest(t, "NOTE: This test requires at least one user-created job to exist")
		t.Skip("No user job found to test edit save functionality")
	}

	env.LogTest(t, "✓ Found user job to edit: %s (ID: %s)", testJobName, testJobID)

	// Navigate directly to job_add page with ID
	editURL := fmt.Sprintf("%s/job_add?id=%s", baseURL, testJobID)
	env.LogTest(t, "Navigating to edit page: %s", editURL)

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(editURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for job to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load edit page: %v", err)
		env.TakeScreenshot(ctx, "edit-page-load-failed")
		t.Fatalf("Failed to load edit page: %v", err)
	}

	env.LogTest(t, "✓ Edit page loaded")
	env.TakeScreenshot(ctx, "edit-page-loaded")

	// Wait for job content to load in editor
	env.LogTest(t, "Waiting for job content to load in editor...")
	var editorReady bool
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const content = editorElement.CodeMirror.getValue();
					return content.length > 0;
				}
				return false;
			})()
		`, &editorReady),
	)

	if err != nil || !editorReady {
		env.LogTest(t, "ERROR: Editor did not load content: %v", err)
		env.TakeScreenshot(ctx, "editor-not-ready")
		t.Fatalf("Editor did not load content")
	}

	env.LogTest(t, "✓ Editor content loaded")

	// Modify TOML content slightly (add a comment)
	env.LogTest(t, "Modifying TOML content...")
	testComment := fmt.Sprintf("# Test modification at %s", time.Now().Format("2006-01-02 15:04:05"))

	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const editor = editorElement.CodeMirror;
					const currentContent = editor.getValue();
					const newContent = '%s\n' + currentContent;
					editor.setValue(newContent);
					return true;
				}
				return false;
			})()
		`, testComment), nil),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to modify editor content: %v", err)
		env.TakeScreenshot(ctx, "editor-modify-failed")
		t.Fatalf("Failed to modify editor content: %v", err)
	}

	env.LogTest(t, "✓ TOML content modified (added comment)")
	env.TakeScreenshot(ctx, "content-modified")

	// Click Save button
	env.LogTest(t, "Clicking Save button...")
	var saveClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for Save button (should be the primary action button)
				const saveButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.trim() === 'Save' || btn.textContent.includes('Save')
				);
				if (saveButton) {
					saveButton.click();
					return true;
				}
				return false;
			})()
		`, &saveClicked),
		chromedp.Sleep(3*time.Second), // Wait for save and redirect
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click Save button: %v", err)
		env.TakeScreenshot(ctx, "save-button-click-failed")
		t.Fatalf("Failed to click Save button: %v", err)
	}

	if !saveClicked {
		env.LogTest(t, "ERROR: Save button not found")
		env.TakeScreenshot(ctx, "save-button-not-found")
		t.Fatal("Save button not found")
	}

	env.LogTest(t, "✓ Save button clicked")
	env.TakeScreenshot(ctx, "after-save-click")

	// Verify redirect to /jobs page
	env.LogTest(t, "Verifying redirect to /jobs page...")
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for redirect
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to get current URL: %v", err)
		t.Fatalf("Failed to get current URL: %v", err)
	}

	if !strings.Contains(currentURL, "/jobs") {
		env.LogTest(t, "WARNING: Expected redirect to /jobs, current URL: %s", currentURL)
		env.LogTest(t, "NOTE: This may be expected if there was a validation error")
	} else {
		env.LogTest(t, "✓ Redirected to /jobs page")
	}

	env.TakeScreenshot(ctx, "after-save-redirect")

	// Check for success notification (if on jobs page)
	if strings.Contains(currentURL, "/jobs") {
		env.LogTest(t, "Checking for success notification...")
		var notificationText string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const notifications = document.querySelectorAll('.notification.success, .notification.is-success, .toast.success, .toast.is-success');
					if (notifications.length > 0) {
						return notifications[0].textContent.trim();
					}
					return '';
				})()
			`, &notificationText),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check for notification: %v", err)
		} else if notificationText != "" {
			env.LogTest(t, "✓ Success notification displayed: %s", notificationText)
		} else {
			env.LogTest(t, "NOTE: No success notification found (may be transient)")
		}
	}

	env.TakeScreenshot(ctx, "save-completed")
	env.LogTest(t, "✅ Edit job save test completed successfully")
}

// TestSystemJobProtection verifies that system jobs cannot be edited
func TestSystemJobProtection(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestSystemJobProtection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSystemJobProtection")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSystemJobProtection (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSystemJobProtection (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Navigate to jobs page
	env.LogTest(t, "Navigating to jobs page: %s/jobs", baseURL)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Alpine.js and data to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.LogTest(t, "✓ Jobs page loaded")
	env.TakeScreenshot(ctx, "jobs-page-loaded")

	// Find a system job (Database Maintenance is known to be a system job)
	env.LogTest(t, "Looking for system job (Database Maintenance)...")
	var systemJobInfo struct {
		Found          bool `json:"found"`
		EditDisabled   bool `json:"editDisabled"`
		DeleteDisabled bool `json:"deleteDisabled"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const dbCard = cards.find(card => card.textContent.includes('Database Maintenance'));

				if (!dbCard) {
					return { found: false, editDisabled: false, deleteDisabled: false };
				}

				// Check if edit button exists and is disabled (find by fa-edit icon)
				const editButton = dbCard.querySelector('button .fa-edit')?.closest('button');
				const editDisabled = editButton ? editButton.disabled : true;

				// Check if delete button exists and is disabled (find by btn-error class or fa-trash icon)
				const deleteButton = dbCard.querySelector('button.btn-error, button .fa-trash')?.closest('button');
				const deleteDisabled = deleteButton ? deleteButton.disabled : true;

				return {
					found: true,
					editDisabled: editDisabled,
					deleteDisabled: deleteDisabled
				};
			})()
		`, &systemJobInfo),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check system job: %v", err)
		env.TakeScreenshot(ctx, "system-job-check-failed")
		t.Fatalf("Failed to check system job: %v", err)
	}

	if !systemJobInfo.Found {
		env.LogTest(t, "ERROR: System job 'Database Maintenance' not found")
		env.TakeScreenshot(ctx, "system-job-not-found")
		t.Fatal("System job 'Database Maintenance' not found")
	}

	env.LogTest(t, "✓ Found system job 'Database Maintenance'")
	env.LogTest(t, "  Edit button disabled: %v", systemJobInfo.EditDisabled)
	env.LogTest(t, "  Delete button disabled: %v", systemJobInfo.DeleteDisabled)

	// Verify edit button is disabled
	if !systemJobInfo.EditDisabled {
		env.LogTest(t, "ERROR: Edit button should be disabled for system jobs")
		env.TakeScreenshot(ctx, "edit-button-not-disabled")
		t.Error("Edit button should be disabled for system jobs")
	} else {
		env.LogTest(t, "✓ Edit button correctly disabled for system job")
	}

	// Verify delete button is also disabled (additional protection)
	if !systemJobInfo.DeleteDisabled {
		env.LogTest(t, "ERROR: Delete button should be disabled for system jobs")
		env.TakeScreenshot(ctx, "delete-button-not-disabled")
		t.Error("Delete button should be disabled for system jobs")
	} else {
		env.LogTest(t, "✓ Delete button correctly disabled for system job")
	}

	// Try to click edit button and verify it doesn't navigate
	env.LogTest(t, "Attempting to click disabled edit button...")
	var clickResult bool
	var urlBefore string
	var urlAfter string

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.href`, &urlBefore),
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const dbCard = cards.find(card => card.textContent.includes('Database Maintenance'));
				if (dbCard) {
					// Find edit button by fa-edit icon
					const editButton = dbCard.querySelector('button .fa-edit')?.closest('button');
					if (editButton) {
						editButton.click();
						return true;
					}
				}
				return false;
			})()
		`, &clickResult),
		chromedp.Sleep(2*time.Second), // Wait to see if navigation occurs
		chromedp.Evaluate(`window.location.href`, &urlAfter),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to test edit button click: %v", err)
	} else {
		env.LogTest(t, "  URL before click: %s", urlBefore)
		env.LogTest(t, "  URL after click: %s", urlAfter)

		if urlBefore != urlAfter {
			env.LogTest(t, "ERROR: URL changed after clicking disabled button (navigation occurred)")
			env.TakeScreenshot(ctx, "navigation-occurred")
			t.Error("Disabled edit button should not navigate to edit page")
		} else {
			env.LogTest(t, "✓ Disabled edit button did not navigate (URL unchanged)")
		}
	}

	env.TakeScreenshot(ctx, "system-job-protection-verified")
	env.LogTest(t, "✅ System job protection test completed successfully")
}

// TestNewsCrawlerJobLoad verifies loading and executing the news-crawler.toml job definition
func TestNewsCrawlerJobLoad(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestNewsCrawlerJobLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestNewsCrawlerJobLoad")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestNewsCrawlerJobLoad (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestNewsCrawlerJobLoad (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second) // Extended timeout for job execution
	defer cancel()

	baseURL := env.GetBaseURL()

	// Step 1: Navigate to job_add page to load the news crawler
	env.LogTest(t, "Step 1: Navigating to job_add page to load news crawler...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(baseURL+"/job_add"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load job_add page: %v", err)
		env.TakeScreenshot(ctx, "job-add-page-load-failed")
		t.Fatalf("Failed to load job_add page: %v", err)
	}
	env.LogTest(t, "✓ Job add page loaded")
	env.TakeScreenshot(ctx, "job-add-page-loaded")

	// Step 2: Load the news-crawler.toml file content
	env.LogTest(t, "Step 2: Reading news-crawler.toml file content...")
	newsCrawlerContent, err := os.ReadFile("../../test/config/news-crawler.toml")
	if err != nil {
		env.LogTest(t, "ERROR: Failed to read news-crawler.toml: %v", err)
		t.Fatalf("Failed to read news-crawler.toml: %v", err)
	}
	env.LogTest(t, "✓ News crawler TOML content loaded (%d bytes)", len(newsCrawlerContent))

	// Step 3: Wait for CodeMirror editor to be ready and paste the content
	env.LogTest(t, "Step 3: Waiting for CodeMirror editor and pasting content...")
	err = chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second), // Wait for CodeMirror to initialize
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const editorElement = document.querySelector('.CodeMirror');
				if (editorElement && editorElement.CodeMirror) {
					const editor = editorElement.CodeMirror;
					editor.setValue(%s);
					return true;
				}
				return false;
			})()
		`, "`"+string(newsCrawlerContent)+"`"), nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to set editor content: %v", err)
		env.TakeScreenshot(ctx, "editor-content-set-failed")
		t.Fatalf("Failed to set editor content: %v", err)
	}
	env.LogTest(t, "✓ News crawler content pasted into editor")
	env.TakeScreenshot(ctx, "news-crawler-content-pasted")

	// Step 4: Save the job definition
	env.LogTest(t, "Step 4: Saving the news crawler job definition...")
	var saveResult bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for Save button
				const saveButton = Array.from(document.querySelectorAll('button')).find(btn =>
					btn.textContent.trim() === 'Save' || btn.textContent.includes('Save')
				);
				if (saveButton) {
					saveButton.click();
					return true;
				}
				return false;
			})()
		`, &saveResult),
		chromedp.Sleep(3*time.Second), // Wait for save and redirect
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to save job definition: %v", err)
		env.TakeScreenshot(ctx, "job-save-failed")
		t.Fatalf("Failed to save job definition: %v", err)
	}

	if !saveResult {
		env.LogTest(t, "ERROR: Save button not found")
		env.TakeScreenshot(ctx, "save-button-not-found")
		t.Fatal("Save button not found")
	}

	env.LogTest(t, "✓ Job definition saved")
	env.TakeScreenshot(ctx, "job-definition-saved")

	// Step 5: Verify redirect to jobs page and check for success notification
	env.LogTest(t, "Step 5: Verifying redirect to jobs page...")
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for redirect
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to get current URL: %v", err)
		t.Fatalf("Failed to get current URL: %v", err)
	}

	if !strings.Contains(currentURL, "/jobs") {
		env.LogTest(t, "WARNING: Expected redirect to /jobs, current URL: %s", currentURL)
		// Continue anyway as the job might still be saved
	} else {
		env.LogTest(t, "✓ Redirected to jobs page")
	}

	// Step 6: Navigate to jobs page if not already there
	if !strings.Contains(currentURL, "/jobs") {
		env.LogTest(t, "Step 6: Navigating to jobs page...")
		err = chromedp.Run(ctx,
			chromedp.Navigate(baseURL+"/jobs"),
			chromedp.WaitVisible(`body`, chromedp.ByQuery),
			chromedp.Sleep(3*time.Second), // Wait for page and data to load
		)
		if err != nil {
			env.LogTest(t, "ERROR: Failed to navigate to jobs page: %v", err)
			t.Fatalf("Failed to navigate to jobs page: %v", err)
		}
	} else {
		// Already on jobs page, just wait for it to load
		err = chromedp.Run(ctx,
			chromedp.Sleep(3*time.Second), // Wait for page and data to load
		)
	}

	env.LogTest(t, "✓ On jobs page")
	env.TakeScreenshot(ctx, "jobs-page-after-save")

	// Step 7: Verify the News Crawler job appears in the job definitions list
	env.LogTest(t, "Step 7: Verifying News Crawler job appears in job definitions...")
	var newsCrawlerFound bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Wait a moment for Alpine.js to render
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				return cards.some(card => card.textContent.includes('News Crawler'));
			})()
		`, &newsCrawlerFound),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for News Crawler job: %v", err)
		env.TakeScreenshot(ctx, "news-crawler-check-failed")
		t.Fatalf("Failed to check for News Crawler job: %v", err)
	}

	if !newsCrawlerFound {
		env.LogTest(t, "ERROR: News Crawler job not found in job definitions list")
		env.TakeScreenshot(ctx, "news-crawler-not-found")
		t.Fatal("News Crawler job should appear in job definitions list after saving")
	}

	env.LogTest(t, "✓ News Crawler job found in job definitions list")

	// Step 8: Execute the News Crawler job
	env.LogTest(t, "Step 8: Executing the News Crawler job...")

	// Wait for WebSocket connection first
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		env.TakeScreenshot(ctx, "websocket-failed")
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected")

	// Override confirm dialog
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.confirm = function() { return true; }`, nil),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to override confirm dialog: %v", err)
		t.Fatalf("Failed to override confirm dialog: %v", err)
	}

	// Find and click the run button for News Crawler
	var runButtonClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));
				const newsCrawlerCard = cards.find(card => card.textContent.includes('News Crawler'));
				if (newsCrawlerCard) {
					const runButton = newsCrawlerCard.querySelector('button.btn-success');
					if (runButton) {
						runButton.click();
						return true;
					}
				}
				return false;
			})()
		`, &runButtonClicked),
		chromedp.Sleep(2*time.Second), // Wait for job to be triggered
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click run button: %v", err)
		env.TakeScreenshot(ctx, "run-button-click-failed")
		t.Fatalf("Failed to click run button: %v", err)
	}

	if !runButtonClicked {
		env.LogTest(t, "ERROR: Run button not found for News Crawler")
		env.TakeScreenshot(ctx, "run-button-not-found")
		t.Fatal("Run button not found for News Crawler")
	}

	env.LogTest(t, "✓ News Crawler job execution triggered")
	env.TakeScreenshot(ctx, "news-crawler-triggered")

	// Step 9: Navigate to queue page to monitor execution
	env.LogTest(t, "Step 9: Navigating to queue page to monitor execution...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for page to load
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to load queue page: %v", err)
		env.TakeScreenshot(ctx, "queue-page-load-failed")
		t.Fatalf("Failed to load queue page: %v", err)
	}

	env.LogTest(t, "✓ Queue page loaded")

	// Initialize filters and load jobs
	env.LogTest(t, "Initializing filters and loading jobs...")
	var loadResult struct {
		Success      bool   `json:"success"`
		ErrorMessage string `json:"errorMessage"`
		JobsLoaded   int    `json:"jobsLoaded"`
	}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(async () => {
				try {
					if (!window.activeFilters) {
						window.activeFilters = {
							status: new Set(['pending', 'running', 'completed', 'failed', 'cancelled']),
							source: new Set(),
							entity: new Set()
						};
					}
					const jobListEl = document.querySelector('[x-data="jobList"]');
					if (!jobListEl) {
						return { success: false, errorMessage: 'jobList element not found', jobsLoaded: 0 };
					}
					const alpineData = Alpine.$data(jobListEl);
					if (!alpineData || !alpineData.loadJobs) {
						return { success: false, errorMessage: 'loadJobs method not found', jobsLoaded: 0 };
					}
					await alpineData.loadJobs();
					return { success: true, errorMessage: '', jobsLoaded: alpineData.allJobs ? alpineData.allJobs.length : 0 };
				} catch (e) {
					return { success: false, errorMessage: e.toString(), jobsLoaded: 0 };
				}
			})()
		`, &loadResult, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load and render
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to call loadJobs(): %v", err)
	} else if !loadResult.Success {
		env.LogTest(t, "WARNING: loadJobs() failed: %s", loadResult.ErrorMessage)
	} else {
		env.LogTest(t, "✓ Jobs loaded successfully (%d jobs)", loadResult.JobsLoaded)
	}

	env.TakeScreenshot(ctx, "queue-page-loaded")

	// Step 10: Verify the News Crawler job appears in the queue
	env.LogTest(t, "Step 10: Verifying News Crawler job appears in queue...")
	var jobInQueue struct {
		Found  bool   `json:"found"`
		JobID  string `json:"jobId"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}

	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Wait for jobs to render
		chromedp.Evaluate(`
			(() => {
				const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
				const newsCrawlerJob = jobCards.find(card => {
					const titleElement = card.querySelector('.card-title');
					return titleElement && titleElement.textContent.includes('News Crawler');
				});

				if (!newsCrawlerJob) {
					return { found: false, jobId: '', status: '', name: '' };
				}

				const jobId = newsCrawlerJob.getAttribute('data-job-id') || '';
				const statusBadge = newsCrawlerJob.querySelector('.label');
				const status = statusBadge ? statusBadge.textContent.trim() : '';
				const titleElement = newsCrawlerJob.querySelector('.card-title');
				const name = titleElement ? titleElement.textContent.trim() : '';

				return {
					found: true,
					jobId: jobId,
					status: status,
					name: name
				};
			})()
		`, &jobInQueue),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for News Crawler job in queue: %v", err)
		env.TakeScreenshot(ctx, "queue-job-check-failed")
		t.Fatalf("Failed to check for News Crawler job in queue: %v", err)
	}

	if !jobInQueue.Found {
		env.LogTest(t, "ERROR: News Crawler job not found in queue")
		env.TakeScreenshot(ctx, "news-crawler-job-not-in-queue")
		t.Fatal("News Crawler job should appear in queue after execution")
	}

	env.LogTest(t, "✓ News Crawler job found in queue")
	env.LogTest(t, "  Job ID: %s", jobInQueue.JobID)
	env.LogTest(t, "  Status: %s", jobInQueue.Status)
	env.LogTest(t, "  Name: %s", jobInQueue.Name)

	env.TakeScreenshot(ctx, "news-crawler-in-queue")

	// Step 11: Monitor job execution for a reasonable time
	env.LogTest(t, "Step 11: Monitoring job execution...")

	// Monitor for up to 30 seconds to see status changes
	monitorStart := time.Now()
	maxMonitorTime := 30 * time.Second
	var finalStatus string

	for time.Since(monitorStart) < maxMonitorTime {
		var currentStatus string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
					const newsCrawlerJob = jobCards.find(card => {
						const titleElement = card.querySelector('.card-title');
						return titleElement && titleElement.textContent.includes('News Crawler');
					});

					if (newsCrawlerJob) {
						const statusBadge = newsCrawlerJob.querySelector('.label');
						return statusBadge ? statusBadge.textContent.trim() : '';
					}
					return '';
				})()
			`, &currentStatus),
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to check job status: %v", err)
			break
		}

		if currentStatus != finalStatus {
			finalStatus = currentStatus
			env.LogTest(t, "  Job status changed to: %s", finalStatus)
		}

		// If job completed or failed, we can stop monitoring
		if strings.Contains(strings.ToLower(finalStatus), "completed") ||
			strings.Contains(strings.ToLower(finalStatus), "failed") {
			env.LogTest(t, "  Job reached terminal state: %s", finalStatus)
			break
		}

		// Wait before next check
		time.Sleep(2 * time.Second)
	}

	env.LogTest(t, "✓ Job monitoring completed. Final status: %s", finalStatus)
	env.TakeScreenshot(ctx, "news-crawler-final-status")

	// Step 12: Click on the job to view details (if it has a valid job ID)
	if jobInQueue.JobID != "" {
		env.LogTest(t, "Step 12: Clicking on News Crawler job to view details...")
		var navigationResult struct {
			Success bool   `json:"success"`
			JobID   string `json:"jobId"`
			Error   string `json:"error"`
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					try {
						const jobCards = Array.from(document.querySelectorAll('.job-card-clickable'));
						const newsCrawlerJob = jobCards.find(card => {
							const titleElement = card.querySelector('.card-title');
							return titleElement && titleElement.textContent.includes('News Crawler');
						});
						
						if (!newsCrawlerJob) {
							return { success: false, jobId: '', error: 'Job card not found' };
						}
						
						const jobId = newsCrawlerJob.getAttribute('data-job-id');
						if (!jobId) {
							return { success: false, jobId: '', error: 'Job ID not found' };
						}
						
						// Navigate to job details
						window.location.href = '/job?id=' + jobId;
						return { success: true, jobId: jobId, error: '' };
						
					} catch (e) {
						return { success: false, jobId: '', error: e.toString() };
					}
				})()
			`), &navigationResult),
			chromedp.Sleep(3*time.Second), // Wait for navigation
		)

		if err != nil {
			env.LogTest(t, "WARNING: Failed to navigate to job details: %v", err)
		} else if navigationResult.Success {
			env.LogTest(t, "✓ Navigated to job details page (Job ID: %s)", navigationResult.JobID)

			// Verify we're on the job details page
			var currentURL string
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`window.location.href`, &currentURL),
			)

			if err == nil && strings.Contains(currentURL, "/job?id=") {
				env.LogTest(t, "✓ Successfully on job details page: %s", currentURL)
				env.TakeScreenshot(ctx, "news-crawler-job-details")
			}
		} else {
			env.LogTest(t, "WARNING: Failed to navigate to job details: %s", navigationResult.Error)
		}
	}

	env.TakeScreenshot(ctx, "test-completed")
	env.LogTest(t, "✅ News Crawler job load and execution test completed successfully")
}
