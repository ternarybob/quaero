package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

func TestIndex(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

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
	env.LogTest(t, "Navigating to Index page: %s", baseURL)

	// 2. Navigate to Index Page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Failed to navigate to index page: %v", err)
	}

	// Take initial screenshot
	if err := TakeFullScreenshotInDir(ctx, env.ResultsDir, "index_page_loaded"); err != nil {
		t.Logf("Failed to take screenshot: %v", err)
	}

	// 3. Verify Favicon
	env.LogTest(t, "Verifying Favicon")
	err = chromedp.Run(ctx,
		chromedp.WaitReady(`link[rel="icon"]`, chromedp.ByQuery),
	)
	if err != nil {
		t.Errorf("Favicon not found: %v", err)
	} else {
		env.LogTest(t, "✓ Found Favicon")
	}

	// 4. Verify Navbar
	env.LogTest(t, "Verifying Navbar Links")
	navLinks := []string{"HOME", "JOBS", "QUEUE", "DOCUMENTS", "SEARCH", "CHAT", "SETTINGS"}
	for _, linkText := range navLinks {
		// Use a more specific selector to ensure we are targeting the desktop links
		// and use WaitReady instead of WaitVisible just in case of layout quirks,
		// though links should be visible.
		selector := fmt.Sprintf(`//div[contains(@class, "nav-links")]//a[contains(text(), "%s")]`, linkText)
		err := chromedp.Run(ctx,
			chromedp.WaitVisible(selector, chromedp.BySearch),
		)
		if err != nil {
			// Dump HTML if navbar fails
			var bodyHTML string
			chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML))
			dumpPath := filepath.Join(env.ResultsDir, "navbar_fail_dump.html")
			os.WriteFile(dumpPath, []byte(bodyHTML), 0644)

			t.Errorf("Navbar link '%s' not found: %v", linkText, err)
		}
	}
	env.LogTest(t, "✓ Verified Navbar Links")

	// 5. Verify Online Indicator
	env.LogTest(t, "Verifying Online Indicator")

	// Capture initial state (Before)
	// Since we changed default to OFFLINE, this should capture the offline state
	// or the transition state.
	if err := TakeScreenshotInDir(ctx, env.ResultsDir, "index_status_before"); err != nil {
		t.Logf("Failed to take before screenshot: %v", err)
	}

	// Wait for WebSocket connection (handled by setup.go helper, but we check UI element here)
	// The indicator should eventually say "ONLINE" and have class "label-success"
	var indicatorText string
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.status-text.label-success`, chromedp.ByQuery),
		chromedp.Text(`.status-text`, &indicatorText, chromedp.ByQuery),
	)

	// It might take a moment to switch to ONLINE, so we poll if it's not immediately ONLINE
	if err != nil || indicatorText != "ONLINE" {
		env.LogTest(t, "Indicator is '%s', waiting for ONLINE...", indicatorText)
		err = chromedp.Run(ctx,
			chromedp.Poll(`document.querySelector('.status-text')?.textContent === 'ONLINE' && document.querySelector('.status-text')?.classList.contains('label-success')`, nil, chromedp.WithPollingTimeout(5*time.Second)),
		)
		if err != nil {
			t.Errorf("Online indicator failed to become ONLINE with label-success: %v", err)
		}
	}
	env.LogTest(t, "✓ Online Indicator is ONLINE (Green)")

	// Capture final state (After)
	if err := TakeScreenshotInDir(ctx, env.ResultsDir, "index_status_after"); err != nil {
		t.Logf("Failed to take after screenshot: %v", err)
	}

	// 6. Verify Service Logs Panel
	env.LogTest(t, "Verifying Service Logs Panel")
	// Check if panel exists
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`//h2[contains(text(), "Service Logs")]`, chromedp.BySearch),
	)
	if err != nil {
		t.Fatalf("Service Logs panel header not found: %v", err)
	}

	// Check for log entries (should be present on startup)
	// We wait for at least one .terminal-line
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.terminal-line`, chromedp.ByQuery),
	)
	if err != nil {
		t.Errorf("No log entries found in Service Logs panel: %v", err)
	} else {
		env.LogTest(t, "✓ Found log entries in panel")
	}

	// 7. Verify Footer Version
	env.LogTest(t, "Verifying Footer Version")
	var footerText string
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`#footer-version`, chromedp.ByQuery),
		chromedp.Text(`#footer-version`, &footerText, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Footer version element not found: %v", err)
	}

	if !strings.Contains(footerText, "Quaero") || !strings.Contains(footerText, "Version") {
		t.Errorf("Footer text does not contain expected version info. Got: %s", footerText)
	} else {
		env.LogTest(t, "✓ Footer contains version info: %s", footerText)
	}

	// Take final screenshot to capture test completion state
	if err := TakeFullScreenshotInDir(ctx, env.ResultsDir, "index_test_complete"); err != nil {
		t.Logf("Failed to take final screenshot: %v", err)
	}
	env.LogTest(t, "✓ Test completed successfully")
}
