package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

func TestJobs(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Load the test job definition
	// Path is relative to test/ui directory
	if err := env.LoadTestJobDefinitions("../config/job-definitions/news-crawler.toml"); err != nil {
		t.Fatalf("Failed to load test job definition: %v", err)
	}

	// Create a timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelTimeout()

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	// Create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		// Properly close browser before canceling context
		// This ensures Chrome processes are terminated on Windows
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	// Base URL
	baseURL := env.GetBaseURL()
	jobsURL := baseURL + "/jobs"

	env.LogTest(t, "Navigating to Jobs page: %s", jobsURL)

	// 2. Navigate to Jobs Page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(jobsURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}

	// 3. Verify All Job Definitions Are Loaded
	// Expected: 5 job definitions from test/config/job-definitions/
	// 1. news-crawler.toml
	// 2. my-custom-crawler.toml
	// 3. test-agent-job.toml
	// 4. keyword-extractor-agent.toml (has {google_gemini_api_key} variable)
	// 5. nearby-restaurants-places.toml (has {google_places_api_key} variable)
	env.LogTest(t, "Verifying all 5 job definitions are loaded")

	// Wait for Alpine.js to load job definitions
	// The page uses Alpine.js x-data="jobDefinitionsManagement" which loads jobs via API
	time.Sleep(2 * time.Second)

	var actualJobCount int
	var jobNames []string

	// Query job definitions from Alpine.js component state
	// The component stores jobs in jobDefinitions array
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const component = Alpine.$data(document.querySelector('[x-data*="jobDefinitionsManagement"]'));
				return component ? component.jobDefinitions.length : 0;
			})()
		`, &actualJobCount),
	); err != nil {
		t.Fatalf("Failed to count job definitions: %v", err)
	}

	// Get job names from Alpine.js component
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const component = Alpine.$data(document.querySelector('[x-data*="jobDefinitionsManagement"]'));
				return component ? component.jobDefinitions.map(j => j.name) : [];
			})()
		`, &jobNames),
	); err != nil {
		t.Fatalf("Failed to get job names: %v", err)
	}

	env.LogTest(t, "Found %d job definitions: %v", actualJobCount, jobNames)

	if actualJobCount != 5 {
		// Take screenshot for debugging
		TakeScreenshotInDir(ctx, env.ResultsDir, "job_count_mismatch")
		t.Fatalf("Expected 5 job definitions, but found %d: %v", actualJobCount, jobNames)
	}

	// Verify expected job names are present (from TOML files)
	expectedJobNames := []string{
		"News Crawler",                       // news-crawler.toml
		"My Custom Crawler",                  // my-custom-crawler.toml
		"Test Keyword Extraction",            // test-agent-job.toml
		"Keyword Extraction",                 // keyword-extractor-agent.toml
		"Nearby Restaurants (Wheelers Hill)", // nearby-restaurants-places.toml
	}

	for _, expectedName := range expectedJobNames {
		found := false
		for _, actualName := range jobNames {
			if actualName == expectedName {
				found = true
				break
			}
		}
		if !found {
			TakeScreenshotInDir(ctx, env.ResultsDir, "missing_job_"+strings.ReplaceAll(expectedName, " ", "_"))
			t.Errorf("Expected job definition '%s' not found in page", expectedName)
		}
	}

	env.LogTest(t, "✓ All 5 job definitions verified")

	// 4. Verify Action Buttons for News Crawler
	// "News Crawler" -> "news-crawler-run"
	runBtnID := "#news-crawler-run"
	editBtnID := "#news-crawler-edit"

	env.LogTest(t, "Verifying action buttons exist")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(runBtnID, chromedp.ByQuery),
		chromedp.WaitVisible(editBtnID, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Action buttons not found: %v", err)
	}
	env.LogTest(t, "✓ Found Run and Edit buttons")

	// 5. Verify Edit Navigation
	env.LogTest(t, "Testing Edit Navigation")
	err = TakeBeforeAfterScreenshots(ctx, env.ResultsDir, "edit_navigation", func() error {
		// Click Edit button
		if err := chromedp.Run(ctx, chromedp.Click(editBtnID, chromedp.ByQuery)); err != nil {
			return err
		}

		// Wait for navigation
		if err := chromedp.Run(ctx, chromedp.Sleep(1*time.Second)); err != nil {
			return err
		}

		// Check URL
		var url string
		if err := chromedp.Run(ctx, chromedp.Location(&url)); err != nil {
			return err
		}

		if !strings.Contains(url, "/jobs/add") {
			return fmt.Errorf("expected URL to contain '/jobs/add', got %s", url)
		}

		// Navigate back
		return chromedp.Run(ctx,
			chromedp.Navigate(jobsURL),
			chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		)
	})
	if err != nil {
		t.Fatalf("Edit navigation failed: %v", err)
	}
	env.LogTest(t, "✓ Edit navigation verified")

	// 6. Verify Run Confirmation (Cancel)
	env.LogTest(t, "Testing Run Confirmation (Cancel)")
	err = TakeBeforeAfterScreenshots(ctx, env.ResultsDir, "run_confirmation", func() error {
		return chromedp.Run(ctx,
			// Click Run button
			chromedp.Click(runBtnID, chromedp.ByQuery),
			// Wait for confirmation modal (Alpine.js global confirmation)
			chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
			chromedp.WaitVisible(`//div[contains(@class, "modal-title") and contains(., "Run Job")]`, chromedp.BySearch),

			// Click Cancel
			chromedp.Click(`//button[contains(., "Cancel")]`, chromedp.BySearch),
			// Wait for modal to disappear
			chromedp.WaitNotPresent(`.modal.active`, chromedp.ByQuery),
		)
	})
	if err != nil {
		t.Fatalf("Run confirmation test failed: %v", err)
	}
	env.LogTest(t, "✓ Run confirmation dialog verified and cancelled")
}
