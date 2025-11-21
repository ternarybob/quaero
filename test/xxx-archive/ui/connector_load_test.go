package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestConnectorLoading verifies that connectors defined in TOML files are loaded on startup
func TestConnectorLoading(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("ConnectorLoading")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestConnectorLoading")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestConnectorLoading (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestConnectorLoading (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Navigate to Settings -> Connectors
	url := env.GetBaseURL() + "/settings"
	env.LogTest(t, "Navigating to settings page: %s", url)

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Failed to load settings page: %v", err)
	}

	// Click Connectors menu item (3rd item)
	env.LogTest(t, "Clicking Connectors menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`//a[contains(., 'Connectors')]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to click Connectors menu item: %v", err)
	}

	// Verify the test connector is visible
	// The connector name matches what is defined in test/config/connectors/ui_test.toml
	testConnectorName := "ui-test-connector"
	env.LogTest(t, "Verifying connector '%s' is visible...", testConnectorName)

	// The connectors are displayed in a table, so we look for a td containing the name
	xpath := fmt.Sprintf(`//td[contains(text(), '%s')]`, testConnectorName)

	// Wait for the element to be visible
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(xpath, chromedp.BySearch),
	)

	if err != nil {
		// Take screenshot on failure
		if screenshotErr := env.TakeScreenshot(ctx, "connector-load-fail"); screenshotErr != nil {
			env.LogTest(t, "Warning: Failed to take failure screenshot: %v", screenshotErr)
		}
		t.Fatalf("Connector '%s' not found in UI (selector: %s): %v", testConnectorName, xpath, err)
	}

	env.LogTest(t, "✓ Connector '%s' found in UI", testConnectorName)

	// Take success screenshot
	if err := env.TakeScreenshot(ctx, "connector-load-success"); err != nil {
		env.LogTest(t, "Warning: Failed to take screenshot: %v", err)
	}
}
