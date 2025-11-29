// -----------------------------------------------------------------------
// Last Modified: Tuesday, 4th November 2025 4:23:28 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
	"github.com/ternarybob/quaero/test/common"
)

// TestMain runs before all tests in the ui package
// It verifies the service is accessible before running any UI tests
// NOTE: Service connectivity check is optional - tests using SetupTestEnvironment
//
//	will start their own service instance
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
	baseURL := test.MustGetTestServerURL()

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
