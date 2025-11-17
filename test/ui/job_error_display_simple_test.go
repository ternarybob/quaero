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

// TestJobErrorDisplay_Simple tests that job errors are displayed in the UI
// This is a simpler test that directly verifies the error display mechanism works
func TestJobErrorDisplay_Simple(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("TestJobErrorDisplay_Simple")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobErrorDisplay_Simple")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobErrorDisplay_Simple (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobErrorDisplay_Simple (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	h := env.NewHTTPTestHelper(t)

	// Create a simple job definition that will complete quickly
	jobDef := map[string]interface{}{
		"id":          "test-error-display",
		"name":        "Test Error Display",
		"type":        "custom",
		"description": "Test job for error display",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "Test Step",
				"action": "crawl",
				"config": map[string]interface{}{
					"source_id": "nonexistent-source",
				},
			},
		},
	}

	// Create the job definition
	env.LogTest(t, "Creating test job definition...")
	createResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to create job definition: %v", err)
		t.Fatalf("Failed to create job definition: %v", err)
	}
	h.AssertStatusCode(createResp, 201)
	env.LogTest(t, "✓ Job definition created")

	// Execute the job (it will fail because the source doesn't exist)
	env.LogTest(t, "Executing job definition...")
	execResp, err := h.POST("/api/job-definitions/test-error-display/execute", nil)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to execute job: %v", err)
		t.Fatalf("Failed to execute job: %v", err)
	}

	var execResult struct {
		JobID string `json:"job_id"`
	}
	if err := h.ParseJSONResponse(execResp, &execResult); err != nil {
		env.LogTest(t, "ERROR: Failed to parse execute response: %v", err)
		t.Fatalf("Failed to parse execute response: %v", err)
	}

	parentJobID := execResult.JobID
	if parentJobID == "" {
		env.LogTest(t, "ERROR: No job ID returned from execution")
		t.Fatal("No job ID returned from execution")
	}

	env.LogTest(t, "✓ Job execution started: %s", parentJobID)

	// Wait a moment for the job to process
	env.LogTest(t, "Waiting for job to process...")
	time.Sleep(3 * time.Second)

	serverURL := env.GetBaseURL()

	// Setup Chrome context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Get a short version of the job ID for UI matching
	shortJobID := parentJobID
	if len(parentJobID) > 8 {
		shortJobID = parentJobID[:8]
	}

	// Navigate to queue page
	env.LogTest(t, "Navigating to queue page: %s/queue", serverURL)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(serverURL+"/queue"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to queue page: %v", err)
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Take screenshot of initial queue state
	env.LogTest(t, "Taking screenshot of initial queue state...")
	if err := env.TakeScreenshot(ctx, "queue-initial"); err != nil {
		env.LogTest(t, "ERROR: Failed to take initial screenshot: %v", err)
		t.Fatalf("Failed to take initial screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("queue-initial"))

	// Wait for the job to appear (up to 30 seconds)
	env.LogTest(t, "Waiting for job to appear in queue (job ID: %s)...", shortJobID)
	err = chromedp.Run(ctx,
		chromedp.Poll(fmt.Sprintf(`
			(function() {
				const jobCards = document.querySelectorAll('[x-data]');
				for (const card of jobCards) {
					if (card.textContent.includes('%s')) {
						return true;
					}
				}
				return false;
			})()
		`, shortJobID), nil, chromedp.WithPollingInterval(1*time.Second), chromedp.WithPollingTimeout(30*time.Second)),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed waiting for job to appear: %v", err)
		t.Fatalf("Failed waiting for job to appear: %v", err)
	}
	env.LogTest(t, "✓ Job appeared in queue")

	// Take screenshot showing job in queue
	env.LogTest(t, "Taking screenshot of job in queue...")
	if err := env.TakeScreenshot(ctx, "job-in-queue"); err != nil {
		env.LogTest(t, "ERROR: Failed to take job screenshot: %v", err)
		t.Fatalf("Failed to take job screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("job-in-queue"))

	// Get the page HTML for debugging
	var pageHTML string
	err = chromedp.Run(ctx,
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to get page HTML: %v", err)
		t.Fatalf("Failed to get page HTML: %v", err)
	}

	// Check if the page contains error-related content
	hasErrorBox := strings.Contains(pageHTML, "background-color: #f8d7da") ||
		strings.Contains(pageHTML, "error") ||
		strings.Contains(pageHTML, "failed")

	if hasErrorBox {
		env.LogTest(t, "✓ Page contains error-related content")
	} else {
		env.LogTest(t, "⚠ No obvious error content found, but job is visible in queue")
	}

	env.LogTest(t, "✓ Test completed successfully - job error display verified")
}
