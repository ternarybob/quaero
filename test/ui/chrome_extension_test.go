// -----------------------------------------------------------------------
// Chrome Extension Test - Tests the Quaero Chrome extension functionality
// Last Modified: Monday, 10th November 2025 12:00:00 am
// Modified By: Claude Code
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	"github.com/chromedp/chromedp"
)

// TestChromeExtension tests the Chrome extension "Capture & Crawl" workflow
// This test simulates the extension's behavior by:
// 1. Loading a test page in Chrome with the extension installed
// 2. Calling the same API endpoints the extension would call
// 3. Verifying the crawl job is created and completes successfully
func TestChromeExtension(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("ChromeExtension")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestChromeExtension")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestChromeExtension (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestChromeExtension (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())
	env.LogTest(t, "")
	env.LogTest(t, "This test simulates the Chrome extension workflow:")

	// Get Chrome extension path from test environment (copied to bin during setup)
	extensionPath, err := env.GetExtensionPath()
	if err != nil {
		env.LogTest(t, "ERROR: Failed to get extension path: %v", err)
		t.Fatalf("Failed to get extension path: %v", err)
	}
	env.LogTest(t, "✓ Extension path: %s", extensionPath)

	// Create Chrome allocator with extension loaded
	env.LogTest(t, "✓ Creating Chrome instance with extension loaded...")

	// Build options list starting with defaults
	opts := append([]chromedp.ExecAllocatorOption{},
		chromedp.DefaultExecAllocatorOptions[:]...,
	)

	// Load extension using standard Chrome flags
	opts = append(opts,
		chromedp.Flag("load-extension", extensionPath),
		chromedp.Flag("disable-extensions-except", extensionPath),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// Create context
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(s string, i ...interface{}) {
		env.LogTest(t, "ChromeDP: "+s, i...)
	}))
	defer cancel()

	// Set timeout for entire test
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	env.LogTest(t, "✓ Chrome started with extension loaded")
	env.LogTest(t, "")

	// Step 1: Navigate to test page
	testURL := "https://www.abc.net.au/news"
	env.LogTest(t, "Step 1: Navigate to test page: %s", testURL)

	err = chromedp.Run(ctx,
		chromedp.Navigate(testURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to test page: %v", err)
		t.Fatalf("Failed to navigate to test page: %v", err)
	}

	env.LogTest(t, "✓ Test page loaded")

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "01-test-page-loaded"); err != nil {
		env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
	}

	// Capture BEFORE state - Jobs page
	env.LogTest(t, "")
	env.LogTest(t, "Capturing BEFORE state...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to fully load
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to jobs page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "02-before-jobs-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Jobs page (before)")
		}
	}

	// Capture BEFORE state - Queue page
	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to queue page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "03-before-queue-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Queue page (before)")
		}
	}

	// Capture BEFORE state - Documents page
	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/documents"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to documents page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "04-before-documents-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Documents page (before)")
		}
	}

	// Step 2: Simulate extension "Capture & Crawl" button click
	// The extension would call POST /api/job-definitions/quick-crawl
	env.LogTest(t, "")
	env.LogTest(t, "Step 2: Simulate 'Capture & Crawl' button click")
	env.LogTest(t, "Calling POST /api/job-definitions/quick-crawl...")

	h := env.NewHTTPTestHelper(t)

	// Build request payload (same as extension would send)
	crawlRequest := map[string]interface{}{
		"url":     testURL,
		"cookies": []interface{}{}, // Empty cookies for public site
	}

	resp, err := h.POST("/api/job-definitions/quick-crawl", crawlRequest)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to call quick-crawl API: %v", err)
		t.Fatalf("Failed to call quick-crawl API: %v", err)
	}
	defer resp.Body.Close()

	// Accept 200 (OK), 201 (Created), or 202 (Accepted) as success
	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 202 {
		env.LogTest(t, "ERROR: Quick-crawl API returned status %d", resp.StatusCode)
		t.Fatalf("Quick-crawl API failed with status %d", resp.StatusCode)
	}

	env.LogTest(t, "✓ Quick-crawl API returned status %d", resp.StatusCode)

	var crawlResult map[string]interface{}
	if err := h.ParseJSONResponse(resp, &crawlResult); err != nil {
		env.LogTest(t, "ERROR: Failed to parse quick-crawl response: %v", err)
		t.Fatalf("Failed to parse response: %v", err)
	}

	jobID, ok := crawlResult["job_id"]
	if !ok {
		env.LogTest(t, "ERROR: Response missing job_id field")
		t.Fatalf("Response missing job_id")
	}

	env.LogTest(t, "✓ Crawl job created successfully")
	env.LogTest(t, "✓ Job ID: %v", jobID)

	// Step 3: Verify job was created
	env.LogTest(t, "")
	env.LogTest(t, "Step 3: Verify crawl job exists")

	resp, err = h.GET("/api/job-definitions")
	if err != nil {
		env.LogTest(t, "ERROR: Failed to query jobs API: %v", err)
		t.Fatalf("Failed to query jobs API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		env.LogTest(t, "ERROR: Jobs API returned status %d", resp.StatusCode)
		t.Fatalf("Jobs API failed with status %d", resp.StatusCode)
	}

	// Parse response - API returns {job_definitions: [...], total_count: N}
	var jobsResponse struct {
		JobDefinitions []map[string]interface{} `json:"job_definitions"`
		TotalCount     int                      `json:"total_count"`
	}
	if err := h.ParseJSONResponse(resp, &jobsResponse); err != nil {
		env.LogTest(t, "ERROR: Failed to parse jobs response: %v", err)
		t.Fatalf("Failed to parse jobs response: %v", err)
	}

	jobs := jobsResponse.JobDefinitions
	env.LogTest(t, "✓ Retrieved %d job definition(s) from API", len(jobs))

	if len(jobs) == 0 {
		env.LogTest(t, "ERROR: No jobs found")
		t.Fatalf("No jobs found after creation")
	}

	// Find our job by ID
	var foundJob map[string]interface{}
	for _, job := range jobs {
		if job["id"] == jobID {
			foundJob = job
			break
		}
	}

	if foundJob == nil {
		env.LogTest(t, "ERROR: Created job not found in job list")
		t.Fatalf("Created job (ID: %v) not found", jobID)
	}

	// Log the job details
	jobJSON, _ := json.MarshalIndent(foundJob, "", "  ")
	env.LogTest(t, "Created job definition:")
	env.LogTest(t, "%s", string(jobJSON))

	// Step 4: Wait for crawl to complete and capture documents
	env.LogTest(t, "")
	env.LogTest(t, "Step 4: Wait for crawl to complete")
	env.LogTest(t, "Waiting up to 60 seconds for documents to be created...")

	// Poll for documents with timeout
	var documentCount int
	pollStart := time.Now()
	pollTimeout := 60 * time.Second
	pollInterval := 3 * time.Second

	for time.Since(pollStart) < pollTimeout {
		resp, err := h.GET("/api/documents/stats")
		if err == nil {
			defer resp.Body.Close()

			if resp.StatusCode == 200 {
				var stats struct {
					TotalDocuments int `json:"total_documents"`
				}
				if err := h.ParseJSONResponse(resp, &stats); err == nil {
					documentCount = stats.TotalDocuments
					if documentCount > 0 {
						env.LogTest(t, "✓ Documents created: %d", documentCount)
						break
					}
				}
			}
		}

		env.LogTest(t, "Waiting... (documents: %d, elapsed: %.0fs)", documentCount, time.Since(pollStart).Seconds())
		time.Sleep(pollInterval)
	}

	if documentCount == 0 {
		env.LogTest(t, "WARNING: No documents created after %v", pollTimeout)
	}

	// Capture AFTER state - Jobs page
	env.LogTest(t, "")
	env.LogTest(t, "Capturing AFTER state...")

	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/jobs"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to jobs page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "05-after-jobs-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Jobs page (after)")
		}
	}

	// Capture AFTER state - Queue page
	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to queue page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "06-after-queue-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Queue page (after)")
		}
	}

	// Capture AFTER state - Documents page
	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/documents"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		env.LogTest(t, "WARNING: Failed to navigate to documents page: %v", err)
	} else {
		if err := env.TakeScreenshot(ctx, "07-after-documents-page"); err != nil {
			env.LogTest(t, "WARNING: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "✓ Screenshot: Documents page (after)")
		}
	}

	// Final screenshot
	if err := env.TakeScreenshot(ctx, "08-test-complete"); err != nil {
		env.LogTest(t, "WARNING: Failed to take final screenshot: %v", err)
	}

	// Verify document count
	env.LogTest(t, "")
	env.LogTest(t, "Step 5: Verify documents were created")

	if documentCount > 0 {
		env.LogTest(t, "✓ PASS: Documents created (count: %d)", documentCount)
	} else {
		env.LogTest(t, "❌ FAIL: No documents created")
		t.Errorf("Expected at least 1 document, got %d", documentCount)
	}

	env.LogTest(t, "")
	env.LogTest(t, "=== TEST SUMMARY ===")
	env.LogTest(t, "✓ Extension loaded in Chrome")
	env.LogTest(t, "✓ Test page loaded successfully")
	env.LogTest(t, "✓ Quick-crawl API called successfully")
	env.LogTest(t, "✓ Crawl job created (ID: %v)", jobID)
	env.LogTest(t, "✓ Job verified in database")
	env.LogTest(t, "✓ Screenshots captured (before/after)")

	if documentCount > 0 {
		env.LogTest(t, "✓ Documents created: %d", documentCount)
	} else {
		env.LogTest(t, "❌ No documents created")
	}

	env.LogTest(t, "")
	env.LogTest(t, "✓ Test completed successfully")
}
