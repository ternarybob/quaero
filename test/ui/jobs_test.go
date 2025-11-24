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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

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
	err = env.TakeBeforeAfterScreenshots(ctx, "edit_navigation", func() error {
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
	err = env.TakeBeforeAfterScreenshots(ctx, "run_confirmation", func() error {
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
