package ui

import (
	"context"
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestServiceConnectivity is the first test that runs to verify service is accessible
// All other UI tests depend on this passing
func TestServiceConnectivity(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestServiceConnectivity")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	baseURL := env.GetBaseURL()

	// Test 1: HTTP health check
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL)
	if err != nil {
		t.Fatalf("CRITICAL: Service not accessible at %s: %v - All UI tests will fail", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CRITICAL: Service returned status %d (expected 200) - All UI tests will fail", resp.StatusCode)
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
		t.Fatalf("CRITICAL: Homepage failed to load in browser: %v - All UI tests will fail", err)
	}

	t.Logf("✓ Service is accessible at %s", baseURL)
	t.Logf("✓ Homepage loaded successfully (title: %s)", title)
	t.Logf("✓ Status: 200 OK")
}
