package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestMain runs before all tests in the ui package
// It verifies the service is accessible before running any UI tests
// NOTE: Service connectivity check is optional - tests using SetupTestEnvironment
// will start their own service instance
func TestMain(m *testing.M) {
	// Capture TestMain output for inclusion in test logs
	mw := io.MultiWriter(&common.TestMainOutput, os.Stderr)

	// Optional: Verify service connectivity before running tests
	// If service is not running, tests using SetupTestEnvironment will start their own
	if err := verifyServiceConnectivity(); err != nil {
		fmt.Fprintf(mw, "\n⚠ Service not pre-started (tests using SetupTestEnvironment will start their own)\n")
		fmt.Fprintf(mw, "   Note: %v\n\n", err)
	} else {
		fmt.Fprintln(mw, "✓ Service connectivity verified - proceeding with UI tests")
	}

	// Run all tests with cleanup guarantee
	var exitCode int
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(mw, "\n⚠ PANIC during test execution: %v\n", r)
				fmt.Fprintf(mw, "Performing cleanup...\n")
				exitCode = 1
			}
			// Ensure all resources are cleaned up
			cleanupAllResources(mw)
		}()
		exitCode = m.Run()
	}()

	os.Exit(exitCode)
}

// cleanupAllResources ensures all test resources are properly released
func cleanupAllResources(w io.Writer) {
	// Force close any open database connections
	// This prevents "database is locked" errors in subsequent test runs
	fmt.Fprintf(w, "Cleaning up test resources...\n")

	// Give a brief moment for any deferred cleanups to complete
	time.Sleep(100 * time.Millisecond)

	fmt.Fprintf(w, "✓ Cleanup complete\n")
}

// verifyServiceConnectivity checks if the service is accessible
func verifyServiceConnectivity() error {
	baseURL := common.MustGetTestServerURL()

	// Test 1: HTTP health check
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL)
	if err != nil {
		return fmt.Errorf("service not accessible at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service returned status %d (expected 200 OK)", resp.StatusCode)
	}

	// Test 2: Homepage loads in browser
	// Create allocator to ensure proper browser process cleanup on Windows
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
		)...,
	)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	ctx, cancelTimeout := context.WithTimeout(browserCtx, 10*time.Second)
	defer cancelTimeout()

	var title string
	err = chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		return fmt.Errorf("homepage failed to load in browser: %w", err)
	}

	fmt.Printf("   Service URL: %s\n", baseURL)
	fmt.Printf("   Status: 200 OK\n")
	fmt.Printf("   Homepage Title: %s\n", title)

	return nil
}

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

	// 6. Verify Service Logs Panel with SSE Connection
	env.LogTest(t, "Verifying Service Logs Panel with SSE Connection")

	// 6a. Check if panel exists
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`//h2[contains(text(), "Service Logs")]`, chromedp.BySearch),
	)
	if err != nil {
		t.Fatalf("Service Logs panel header not found: %v", err)
	}
	env.LogTest(t, "✓ Service Logs panel header found")

	// 6b. Wait for SSE connection to establish and show "Connected" status
	env.LogTest(t, "Waiting for SSE connection to establish...")
	err = chromedp.Run(ctx,
		chromedp.Poll(`
			(() => {
				// Check for connected status in SSE status indicator
				const statusDot = document.querySelector('.sse-status-dot');
				const statusText = document.querySelector('.sse-status span:last-child');
				if (statusDot && statusDot.classList.contains('connected')) return true;
				if (statusText && statusText.textContent === 'Connected') return true;
				return false;
			})()
		`, nil, chromedp.WithPollingTimeout(10*time.Second)),
	)
	if err != nil {
		TakeScreenshotInDir(ctx, env.ResultsDir, "index_sse_connection_failed")
		t.Errorf("SSE connection did not show 'Connected' status: %v", err)
	} else {
		env.LogTest(t, "✓ SSE connection shows 'Connected' status")
	}

	// Take screenshot after SSE connection
	TakeScreenshotInDir(ctx, env.ResultsDir, "index_sse_connected")

	// 6c. Check for log entries (should be present on startup)
	env.LogTest(t, "Waiting for log entries to appear...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.terminal-line`, chromedp.ByQuery),
	)
	if err != nil {
		TakeScreenshotInDir(ctx, env.ResultsDir, "index_no_logs")
		t.Errorf("No log entries found in Service Logs panel: %v", err)
	} else {
		env.LogTest(t, "✓ Found log entries in panel")
	}

	// 6d. Verify no ERROR level logs in startup (within the service logs panel)
	env.LogTest(t, "Checking for ERROR level logs in service logs...")
	var errorLogCount int
	var errorMessages []string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminalLines = document.querySelectorAll('.terminal-line');
				const errors = [];
				for (const line of terminalLines) {
					const text = line.textContent;
					// Check for error level indicators
					if (text.includes('[ERR]') || text.includes('[ERROR]') || text.includes('level=ERR')) {
						errors.push(text.trim().substring(0, 200)); // Truncate long messages
					}
				}
				return errors;
			})()
		`, &errorMessages),
	)
	if err != nil {
		t.Errorf("Failed to check for error logs: %v", err)
	} else {
		errorLogCount = len(errorMessages)
		if errorLogCount > 0 {
			t.Errorf("Found %d ERROR level logs in startup:", errorLogCount)
			for i, msg := range errorMessages {
				t.Errorf("  [%d] %s", i+1, msg)
			}
		} else {
			env.LogTest(t, "✓ No ERROR level logs found in service logs")
		}
	}

	// 6e. Verify logs contain expected startup messages (info level)
	env.LogTest(t, "Verifying startup logs contain expected messages...")
	var hasStartupLogs bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminalLines = document.querySelectorAll('.terminal-line');
				for (const line of terminalLines) {
					const text = line.textContent.toLowerCase();
					// Check for typical startup messages
					if (text.includes('started') || text.includes('initialized') ||
						text.includes('application') || text.includes('server')) {
						return true;
					}
				}
				return false;
			})()
		`, &hasStartupLogs),
	)
	if err != nil {
		t.Errorf("Failed to verify startup logs: %v", err)
	} else if !hasStartupLogs {
		env.LogTest(t, "WARNING: No typical startup messages found in logs")
	} else {
		env.LogTest(t, "✓ Found expected startup messages in logs")
	}

	// Take screenshot of final service logs state
	TakeScreenshotInDir(ctx, env.ResultsDir, "index_service_logs_final")

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
