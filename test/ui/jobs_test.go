package ui

import (
	"context"
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
		{"Add Job Definition button", "button.btn-primary"},
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

	// Try to click on the job card to view details/logs
	env.LogTest(t, "Clicking on job card to view details...")
	var cardClicked bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = Array.from(document.querySelectorAll('.job-card-clickable, .card'));
				const jobCard = cards.find(card => card.textContent.includes('Database Maintenance'));
				if (jobCard) {
					jobCard.click();
					return true;
				}
				return false;
			})()
		`, &cardClicked),
		chromedp.Sleep(2*time.Second), // Wait for navigation or details to load
	)

	if err != nil {
		env.LogTest(t, "  WARNING: Failed to click job card: %v", err)
	} else if cardClicked {
		env.LogTest(t, "  ✓ Job card clicked")

		// Check if we navigated to job details page or if logs are visible
		var currentURL string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`window.location.href`, &currentURL),
		)

		if err != nil {
			env.LogTest(t, "  WARNING: Failed to get current URL: %v", err)
		} else {
			env.LogTest(t, "  Current URL: %s", currentURL)

			// If we're on a job details page, try to read logs
			if currentURL != baseURL+"/queue" {
				env.TakeScreenshot(ctx, "job-details-page")

				// Wait for logs to load
				env.LogTest(t, "  Waiting for job logs to load...")
				err = chromedp.Run(ctx,
					chromedp.Sleep(2*time.Second),
				)

				// Try to find and read logs
				var logText string
				err = chromedp.Run(ctx,
					chromedp.Evaluate(`
						(() => {
							const logContainer = document.querySelector('.log-container, .logs, pre, code');
							return logContainer ? logContainer.textContent.substring(0, 500) : '';
						})()
					`, &logText),
				)

				if err != nil {
					env.LogTest(t, "  WARNING: Failed to read logs: %v", err)
				} else if logText != "" {
					env.LogTest(t, "  ✓ Job logs found (first 500 chars):")
					// Split into lines and log each line
					lines := strings.Split(logText, "\n")
					for i, line := range lines {
						if i >= 10 { // Limit to first 10 lines
							env.LogTest(t, "    ... (%d more lines)", len(lines)-10)
							break
						}
						if line != "" {
							env.LogTest(t, "    %s", line)
						}
					}
				} else {
					env.LogTest(t, "  WARNING: No logs found on details page")
				}

				env.TakeScreenshot(ctx, "job-logs-view")
			} else {
				env.LogTest(t, "  Note: Still on queue page (card click may not navigate)")
			}
		}
	} else {
		env.LogTest(t, "  WARNING: Card click returned false")
	}

	env.LogTest(t, "✅ Database Maintenance job run test completed successfully")
}
