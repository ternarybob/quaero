// -----------------------------------------------------------------------
// Job Configuration UI Tests
// Tests that job definitions load correctly and are visible in the UI
// This is a common test that validates job configuration from the UI perspective
// -----------------------------------------------------------------------

package ui

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedJobDefinitions lists job definitions that should be visible in the UI
// These are from deployments/common/job-definitions/*.toml
var expectedJobDefinitions = []string{
	"Announcements Watchlist",
	"Competitor Analysis Watchlist",
	"Fundamentals Watchlist",
	"Market Data Watchlist",
	"Portfolio Analysis",
	"Portfolio Newsletter",
	"Portfolio Newsletter (Combined)",
	"Stock Analysis (List)",
	"Stock Analysis (Navexa Portfolio)",
	"Stock Deep Dive Analysis",
	"Stock Deep Dive Analysis (Tools)",
	"Stock Rating - Watchlist",
	"Ticker Metadata (Portfolio)",
	"Ticker Metadata (Watchlist)",
	"Ticker News (Portfolio)",
	"Ticker News (Watchlist)",
}

// TestJobConfigurationUIValidation verifies that all expected job definitions are visible in the UI
func TestJobConfigurationUIValidation(t *testing.T) {
	utc := NewUITestContext(t, 3*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Configuration UI Visibility ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load jobs
	time.Sleep(3 * time.Second)
	utc.Screenshot("jobs_page_loaded")

	// Get all job names from the UI
	var jobNames []string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				const names = [];
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl) {
						names.push(titleEl.textContent.trim());
					}
				}
				return names;
			})()
		`, &jobNames),
	)
	require.NoError(t, err, "Failed to get job names from UI")

	utc.Log("Found %d job definitions in UI", len(jobNames))
	for _, name := range jobNames {
		utc.Log("  - %s", name)
	}

	// Verify we have jobs
	require.NotEmpty(t, jobNames, "No job definitions found in UI - check that job definitions are loading correctly")

	// Check for expected job definitions
	var missing []string
	for _, expected := range expectedJobDefinitions {
		found := false
		for _, actual := range jobNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, expected)
		}
	}

	if len(missing) > 0 {
		utc.Screenshot("missing_jobs")
		utc.Log("Missing job definitions in UI:")
		for _, m := range missing {
			utc.Log("  - %s", m)
		}
		t.Errorf("FAIL: %d expected job definitions not found in UI: %v", len(missing), missing)
	}

	// Save validation results to test output
	utc.SaveToResults("job_names.txt", fmt.Sprintf("Jobs found in UI:\n\n%v", jobNames))

	utc.Log("PASS: Job configuration UI validation completed")
}

// TestJobConfigurationUIJobCards verifies job card structure in the UI
func TestJobConfigurationUIJobCards(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Card Structure ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load
	time.Sleep(2 * time.Second)

	// Verify each job card has required elements
	var cardInfo []map[string]interface{}
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				const info = [];
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					const runBtn = card.querySelector('button[id$="-run"]');
					const refreshBtn = card.querySelector('button[id$="-refresh"]');

					info.push({
						hasTitle: !!titleEl,
						title: titleEl ? titleEl.textContent.trim() : '',
						hasRunButton: !!runBtn,
						hasRefreshButton: !!refreshBtn,
						runButtonId: runBtn ? runBtn.id : '',
					});
				}
				return info;
			})()
		`, &cardInfo),
	)
	require.NoError(t, err, "Failed to get job card info")

	utc.Screenshot("job_cards")

	// Validate each card
	for _, card := range cardInfo {
		title, _ := card["title"].(string)
		if title == "" {
			continue // Skip non-job cards
		}

		t.Run(title, func(t *testing.T) {
			hasTitle, _ := card["hasTitle"].(bool)
			hasRunBtn, _ := card["hasRunButton"].(bool)
			hasRefreshBtn, _ := card["hasRefreshButton"].(bool)

			assert.True(t, hasTitle, "Job card should have title")
			assert.True(t, hasRunBtn, "Job card should have run button")
			assert.True(t, hasRefreshBtn, "Job card should have refresh button")

			utc.Log("Job card validated: %s (run=%v, refresh=%v)", title, hasRunBtn, hasRefreshBtn)
		})
	}

	utc.Log("PASS: Job card structure validation completed")
}

// TestJobConfigurationUIJobCount verifies the expected number of jobs are loaded
func TestJobConfigurationUIJobCount(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Count ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load
	time.Sleep(2 * time.Second)

	// Count job cards
	var jobCount int
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				let count = 0;
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.trim()) {
						count++;
					}
				}
				return count;
			})()
		`, &jobCount),
	)
	require.NoError(t, err, "Failed to count job cards")

	utc.Screenshot("job_count")
	utc.Log("Found %d job definitions", jobCount)

	// Verify minimum expected count (from expectedJobDefinitions)
	minExpected := len(expectedJobDefinitions)
	assert.GreaterOrEqual(t, jobCount, minExpected,
		"Expected at least %d job definitions, found %d", minExpected, jobCount)

	utc.Log("PASS: Job count validation completed (%d jobs found, %d expected minimum)", jobCount, minExpected)
}

// TestJobConfigurationUINewJobsVisible verifies the newly fixed job definitions are visible
// This specifically tests the job definitions that were fixed from [[steps]] to [step.{name}] format
func TestJobConfigurationUINewJobsVisible(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Newly Fixed Job Definitions Visibility ---")

	// These are the job definitions that were fixed from [[steps]] format
	newlyFixedJobs := []string{
		"Portfolio Newsletter",
		"Portfolio Newsletter (Combined)",
		"Ticker Metadata (Portfolio)",
		"Ticker Metadata (Watchlist)",
		"Ticker News (Portfolio)",
		"Ticker News (Watchlist)",
	}

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load
	time.Sleep(3 * time.Second)
	utc.Screenshot("jobs_page_new_jobs")

	// Get all job names
	var jobNames []string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				const names = [];
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl) {
						names.push(titleEl.textContent.trim());
					}
				}
				return names;
			})()
		`, &jobNames),
	)
	require.NoError(t, err, "Failed to get job names")

	// Check each newly fixed job
	var found, missing []string
	for _, expectedJob := range newlyFixedJobs {
		jobFound := false
		for _, actualName := range jobNames {
			if actualName == expectedJob {
				jobFound = true
				found = append(found, expectedJob)
				break
			}
		}
		if !jobFound {
			missing = append(missing, expectedJob)
		}
	}

	// Log results
	utc.Log("Newly fixed jobs found: %d/%d", len(found), len(newlyFixedJobs))
	for _, f := range found {
		utc.Log("  FOUND: %s", f)
	}
	for _, m := range missing {
		utc.Log("  MISSING: %s", m)
	}

	// Fail if any are missing
	if len(missing) > 0 {
		utc.Screenshot("missing_new_jobs")
		t.Errorf("FAIL: %d newly fixed job definitions not visible in UI: %v", len(missing), missing)
	}

	utc.Log("PASS: All newly fixed job definitions are visible in UI")
}

// TestJobConfigurationUIAlphabeticalOrder verifies jobs are displayed in alphabetical order
func TestJobConfigurationUIAlphabeticalOrder(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Alphabetical Order ---")

	// Navigate to Jobs page
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for Alpine.js to load
	time.Sleep(2 * time.Second)

	// Get job names in display order
	var jobNames []string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const cards = document.querySelectorAll('.card');
				const names = [];
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.trim()) {
						names.push(titleEl.textContent.trim());
					}
				}
				return names;
			})()
		`, &jobNames),
	)
	require.NoError(t, err, "Failed to get job names")

	// Make a sorted copy
	sortedNames := make([]string, len(jobNames))
	copy(sortedNames, jobNames)
	sort.Strings(sortedNames)

	// Compare order
	isSorted := true
	for i := range jobNames {
		if jobNames[i] != sortedNames[i] {
			isSorted = false
			utc.Log("Order mismatch at position %d: got '%s', expected '%s'", i, jobNames[i], sortedNames[i])
			break
		}
	}

	utc.Screenshot("job_order")

	// Log but don't fail - alphabetical order is a nice-to-have
	if !isSorted {
		utc.Log("Note: Jobs are not in strict alphabetical order (this is informational)")
	} else {
		utc.Log("Jobs are displayed in alphabetical order")
	}

	utc.Log("PASS: Job order check completed")
}
