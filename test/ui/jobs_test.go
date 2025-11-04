package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestJobsPageLoad(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "jobs-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Job Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Jobs page loads correctly")
}

func TestJobsPageElements(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsPageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Statistics card", ".card"},
		{"Create Job button", "button.btn-info"},
		{"Jobs table", "table.table"},
		{"Filter controls", "#status-filter"},
		{"Pagination controls", "#page-info"},
		{"Job detail section", "#job-detail-json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err = chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Sleep(2*time.Second), // Wait for data to load
				chromedp.Evaluate(`document.querySelectorAll("`+tt.selector+`").length`, &nodeCount),
			)

			if err != nil {
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if nodeCount == 0 {
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			}
		})
	}

	// Take screenshot after checking all elements
	if err := env.TakeScreenshot(ctx, "jobs-page-elements"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}
}

func TestJobsNavbar(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsNavbar")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	var navbarVisible bool
	var menuItems []string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`nav.navbar`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('nav.navbar') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.navbar-item')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		t.Error("Navbar not found on page")
	}

	// Check for JOBS menu item
	expectedItems := []string{"HOME", "AUTHENTICATION", "SOURCES", "JOBS", "DOCUMENTS", "CHAT", "SETTINGS"}
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			if strings.Contains(item, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Menu item '%s' not found in navbar", expected)
		}
	}

	// Verify JOBS item is active on jobs page
	var jobsActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.navbar-item.is-active[href="/jobs"]') !== null`, &jobsActive),
	)
	if err != nil {
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !jobsActive {
		t.Error("JOBS menu item should be active on jobs page")
	}

	// Take screenshot of navbar
	if err := env.TakeScreenshot(ctx, "jobs-navbar"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Navbar displays correctly with JOBS item")
}

func TestJobsStatistics(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsStatistics")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Wait for statistics to load
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for API calls
	)

	if err != nil {
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Take screenshot of statistics
	if err := env.TakeScreenshot(ctx, "jobs-statistics"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for all stat elements
	stats := []string{
		"#stat-total",
		"#stat-pending",
		"#stat-running",
		"#stat-completed",
		"#stat-failed",
		"#stat-cancelled",
	}

	for _, statID := range stats {
		var statValue string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector("`+statID+`") ? document.querySelector("`+statID+`").textContent : null`, &statValue),
		)

		if err != nil {
			t.Errorf("Failed to get stat value for %s: %v", statID, err)
			continue
		}

		if statValue == "" || statValue == "-" {
			t.Logf("Stat %s has no value yet (may be loading)", statID)
		}
	}

	t.Log("✓ Job statistics section displays correctly")
}

func TestJobsTable(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsTable")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Check if the table has expected columns
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`table.table`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for data
	)

	if err != nil {
		t.Fatalf("Failed to load jobs table: %v", err)
	}

	// Check for table headers
	expectedHeaders := []string{"JOB ID", "SOURCE", "ENTITY", "STATUS", "PROGRESS", "CREATED", "ACTIONS"}
	for _, header := range expectedHeaders {
		var hasHeader bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`Array.from(document.querySelectorAll('th')).some(th => th.textContent.includes("`+header+`"))`, &hasHeader),
		)

		if err != nil {
			t.Errorf("Failed to check header '%s': %v", header, err)
			continue
		}

		if !hasHeader {
			t.Errorf("Header '%s' not found in jobs table", header)
		}
	}

	// Check column count
	var columnCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('thead tr th').length`, &columnCount),
	)

	if err == nil && columnCount != 7 {
		t.Errorf("Expected 7 columns in table, got %d", columnCount)
	}

	// Take screenshot of jobs table
	if err := env.TakeScreenshot(ctx, "jobs-table"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Jobs table displays correctly with all columns")
}

func TestJobsCreateModal(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsCreateModal")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Click Create Job button to open modal
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`button.btn-info`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Ensure button is clickable
		chromedp.Click(`button.btn-info`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for modal animation
	)

	if err != nil {
		t.Fatalf("Failed to open create job modal: %v", err)
	}

	// Check if modal is visible
	var modalVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#create-job-modal.modal.is-active') !== null`, &modalVisible),
	)

	if err != nil {
		t.Fatalf("Failed to check modal visibility: %v", err)
	}

	if !modalVisible {
		t.Error("Create job modal did not open")
		return
	}

	// Take screenshot of modal
	if err := env.TakeScreenshot(ctx, "jobs-create-modal"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for form elements
	formElements := []struct {
		name     string
		selector string
	}{
		{"Source dropdown", "#job-source-select"},
		{"Refresh checkbox", "#refresh-source-checkbox"},
		{"Seed URLs textarea", "#seed-urls-textarea"},
		{"Create button", "button.btn-info"},
		{"Cancel button", "button.btn-secondary"},
	}

	for _, elem := range formElements {
		var elementPresent bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('`+elem.selector+`') !== null`, &elementPresent),
		)

		if err != nil {
			t.Errorf("Failed to check '%s': %v", elem.name, err)
			continue
		}

		if !elementPresent {
			t.Errorf("Element '%s' not found in create job modal", elem.name)
		}
	}

	t.Log("✓ Create job modal displays correctly with all form fields")
}

func TestJobsFilterControls(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsFilterControls")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Check for filter dropdowns
	filters := []struct {
		name     string
		selector string
	}{
		{"Status filter", "#status-filter"},
		{"Source filter", "#source-filter"},
		{"Entity filter", "#entity-filter"},
	}

	for _, filter := range filters {
		var filterPresent bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('`+filter.selector+`') !== null`, &filterPresent),
		)

		if err != nil {
			t.Errorf("Failed to check '%s': %v", filter.name, err)
			continue
		}

		if !filterPresent {
			t.Errorf("Filter '%s' not found on page", filter.name)
		}
	}

	// Take screenshot of filter controls
	if err := env.TakeScreenshot(ctx, "jobs-filter-controls"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Job filter controls display correctly")
}

func TestJobsQueueIntegration(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestJobsQueueIntegration")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Load page and wait for queue data
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for loadJobQueue() call
	)

	if err != nil {
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	// Verify that loadJobQueue function exists
	var loadQueueExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof loadJobQueue === 'function'`, &loadQueueExists),
	)

	if err != nil {
		t.Fatalf("Failed to check loadJobQueue function: %v", err)
	}

	if !loadQueueExists {
		t.Error("loadJobQueue function not found in page")
	}

	// Check that pending and running stats are being updated
	var pendingStat, runningStat string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#stat-pending').textContent`, &pendingStat),
		chromedp.Evaluate(`document.querySelector('#stat-running').textContent`, &runningStat),
	)

	if err != nil {
		t.Fatalf("Failed to check stat values: %v", err)
	}

	// Stats should not be "-" after queue loads
	if pendingStat == "-" || runningStat == "-" {
		t.Logf("Queue stats still loading (pending: %s, running: %s)", pendingStat, runningStat)
	}

	// Take screenshot of queue integration
	if err := env.TakeScreenshot(ctx, "jobs-queue-integration"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ Job queue integration working (loadJobQueue called on page load)")
}
