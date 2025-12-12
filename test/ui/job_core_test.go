// core_test.go - Core UI tests for Quaero
// Tests fundamental UI functionality: page loads, navigation, basic interactions

package ui

import (
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestPagesLoad verifies all main pages load without errors
func TestPagesLoad(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	pages := []struct {
		name string
		url  string
	}{
		{"Home", utc.BaseURL},
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
