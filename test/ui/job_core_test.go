// job_core_test.go - Job-related UI tests for Quaero
// Tests job and queue page functionality, navigation between job pages
// NOTE: Index/Home page tests are in index_test.go - do not duplicate here

package ui

import (
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJobRelatedPagesLoad verifies job-related pages load without errors
// NOTE: Home page is tested in index_test.go - not duplicated here
func TestJobRelatedPagesLoad(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	pages := []struct {
		name string
		url  string
	}{
		{"Jobs", utc.JobsURL},
		{"Queue", utc.QueueURL},
		{"Documents", utc.DocsURL},
		{"Settings", utc.SettingsURL},
	}

	for _, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			utc.Log("Testing page load: %s", page.name)

			if err := chromedp.Run(utc.Ctx, chromedp.Navigate(page.url)); err != nil {
				t.Fatalf("Failed to navigate to %s: %v", page.name, err)
			}

			// Wait for body to be visible (basic page load check)
			if err := chromedp.Run(utc.Ctx,
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
			); err != nil {
				t.Fatalf("Page %s did not load properly: %v", page.name, err)
			}

			utc.Screenshot(page.name + "_loaded")
			utc.Log("✓ Page %s loaded successfully", page.name)
		})
	}
}

// TestJobsPageShowsJobs verifies the Jobs page displays job definitions
func TestJobsPageShowsJobs(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Jobs Page Shows Job Definitions ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load jobs
	time.Sleep(2 * time.Second)

	// Check that at least one job card is visible
	var jobCount int
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`document.querySelectorAll('.card').length`, &jobCount),
	)
	if err != nil {
		t.Fatalf("Failed to count job cards: %v", err)
	}

	utc.Screenshot("jobs_page_with_jobs")

	if jobCount == 0 {
		t.Fatal("No job cards found on Jobs page")
	}

	utc.Log("✓ Found %d job cards on Jobs page", jobCount)
}

// TestQueuePageShowsQueue verifies the Queue page displays queued jobs
func TestQueuePageShowsQueue(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Queue Page ---")

	// Navigate to Queue page
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}

	// Wait for Alpine.js to initialize
	time.Sleep(2 * time.Second)

	// Check that the jobList component exists
	var componentExists bool
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`!!document.querySelector('[x-data="jobList"]')`, &componentExists),
	)
	if err != nil {
		t.Fatalf("Failed to check for jobList component: %v", err)
	}

	utc.Screenshot("queue_page")

	if !componentExists {
		t.Fatal("jobList Alpine component not found on Queue page")
	}

	utc.Log("✓ Queue page loaded with jobList component")
}

// TestNavigationBetweenPages verifies navigation works correctly
func TestNavigationBetweenPages(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Navigation Between Pages ---")

	// Start at home page
	if err := chromedp.Run(utc.Ctx, chromedp.Navigate(utc.BaseURL)); err != nil {
		t.Fatalf("Failed to navigate to home: %v", err)
	}
	utc.Screenshot("nav_start_home")

	// Navigate to Jobs via navbar link
	if err := utc.Click(`a[href="/jobs"]`); err != nil {
		t.Fatalf("Failed to click Jobs link: %v", err)
	}
	time.Sleep(1 * time.Second)
	utc.Screenshot("nav_to_jobs")

	// Verify we're on Jobs page
	currentURL, _ := utc.getCurrentURL()
	if currentURL != utc.JobsURL {
		t.Fatalf("Expected to be on Jobs page, but URL is %s", currentURL)
	}

	utc.Log("✓ Navigation working correctly")
}

// getCurrentURL gets the current page URL
func (utc *UITestContext) getCurrentURL() (string, error) {
	var url string
	err := chromedp.Run(utc.Ctx, chromedp.Location(&url))
	return url, err
}

// TestJobsPageActionButtons verifies Run and Refresh buttons exist on job cards
func TestJobsPageActionButtons(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Jobs Page Action Buttons ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load jobs
	time.Sleep(2 * time.Second)

	// Check that Run and Refresh buttons exist for at least one job
	var hasRunButton, hasRefreshButton bool
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`document.querySelectorAll('button[id$="-run"]').length > 0`, &hasRunButton),
		chromedp.Evaluate(`document.querySelectorAll('button[id$="-refresh"]').length > 0`, &hasRefreshButton),
	)
	if err != nil {
		t.Fatalf("Failed to check for action buttons: %v", err)
	}

	utc.Screenshot("jobs_action_buttons")

	if !hasRunButton {
		t.Error("No Run buttons found on Jobs page")
	}
	if !hasRefreshButton {
		t.Error("No Refresh buttons found on Jobs page")
	}

	utc.Log("✓ Found Run and Refresh buttons on Jobs page")
}

// TestJobsPageRunConfirmation verifies the Run confirmation modal appears and can be cancelled
func TestJobsPageRunConfirmation(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Run Confirmation Modal ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load jobs
	time.Sleep(2 * time.Second)

	utc.Screenshot("before_run_click")

	// Click the first Run button
	err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`button[id$="-run"]`, chromedp.ByQuery),
		chromedp.Click(`button[id$="-run"]`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Failed to click Run button: %v", err)
	}

	// Wait for confirmation modal
	err = chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
	)
	if err != nil {
		utc.Screenshot("modal_not_found")
		t.Fatalf("Confirmation modal did not appear: %v", err)
	}

	utc.Screenshot("confirmation_modal")

	// Click Cancel button
	err = chromedp.Run(utc.Ctx,
		chromedp.Click(`//button[contains(., "Cancel")]`, chromedp.BySearch),
		chromedp.WaitNotPresent(`.modal.active`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Failed to cancel confirmation: %v", err)
	}

	utc.Screenshot("after_cancel")
	utc.Log("✓ Run confirmation modal verified and cancelled")
}

// TestJobAddPage tests the job add page UI has required elements
// This tests PAGE functionality, not job execution
func TestJobAddPage(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Add Page ---")

	jobAddURL := utc.BaseURL + "/jobs/add"

	// Navigate to job add page
	if err := chromedp.Run(utc.Ctx, chromedp.Navigate(jobAddURL)); err != nil {
		t.Fatalf("Failed to navigate to job add page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(utc.Ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Job add page did not load: %v", err)
	}

	utc.Screenshot("job_add_page_loaded")

	// Verify TOML editor exists
	var editorExists bool
	if err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`document.getElementById('toml-editor') !== null`, &editorExists),
	); err != nil {
		t.Fatalf("Failed to check for TOML editor: %v", err)
	}

	if !editorExists {
		utc.Screenshot("editor_missing")
		t.Fatal("TOML editor not found on job add page")
	}

	utc.Log("✓ TOML editor found on job add page")
	utc.Screenshot("job_add_test_complete")
}
