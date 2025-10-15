package ui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

// TestMain runs before all tests in the ui package
// It verifies the service is accessible before running any UI tests
func TestMain(m *testing.M) {
	// Verify service connectivity before running tests
	if err := verifyServiceConnectivity(); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ CRITICAL: Service connectivity check FAILED\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   All UI tests will be skipped\n\n")
		os.Exit(1)
	}

	fmt.Println("✓ Service connectivity verified - proceeding with UI tests")

	// Run all tests
	exitCode := m.Run()

	os.Exit(exitCode)
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
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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
