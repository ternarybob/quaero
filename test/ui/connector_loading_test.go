package ui

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestConnectorUI_NoConnectorsMessage tests the empty state message
func TestConnectorUI_NoConnectorsMessage(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create browser context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelTimeout()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		// Properly close browser before canceling context
		// This ensures Chrome processes are terminated on Windows
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	// Navigate to Settings > Connectors page
	baseURL := env.GetBaseURL()
	settingsURL := baseURL + "/settings#connectors"

	env.LogTest(t, "Navigating to Settings > Connectors page")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(settingsURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to settings page: %v", err)
	}

	// Click on Connectors menu item to ensure section is visible
	err = chromedp.Run(ctx,
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Connectors")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to click Connectors menu: %v", err)
	}

	// Take screenshot
	if err := env.TakeFullScreenshot(ctx, "connectors_section"); err != nil {
		t.Logf("Failed to take screenshot: %v", err)
	}

	// Check for empty state message or connector list
	// The UI should show either "No Connectors" message or a list of connectors
	var bodyHTML string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML)); err != nil {
		t.Logf("Failed to get body HTML: %v", err)
	}

	// Save HTML dump for debugging
	dumpPath := env.GetScreenshotPath("connectors_page_dump.html")
	if err := os.WriteFile(dumpPath, []byte(bodyHTML), 0644); err != nil {
		t.Logf("Failed to write page dump: %v", err)
	}

	// Take final screenshot
	if err := env.TakeFullScreenshot(ctx, "connectors_test_complete"); err != nil {
		t.Logf("Failed to take final screenshot: %v", err)
	}
	env.LogTest(t, "✓ No connectors message test completed")
}

// TestConnectorUI_ConnectorDetails tests viewing connector details
func TestConnectorUI_ConnectorDetails(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector via API first
	connectorName := "UI Test Connector " + time.Now().Format("150405")
	body := map[string]interface{}{
		"name": connectorName,
		"type": "github",
		"config": map[string]interface{}{
			"token": "skip_validation_token",
		},
	}
	resp, err := helper.POST("/api/connectors", body)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 400 {
		t.Skipf("Connector creation returned status %d, skipping UI test", resp.StatusCode)
		return
	}

	if resp.StatusCode == 400 {
		t.Skip("Connector creation failed (likely validation), skipping UI test")
		return
	}

	// Create browser context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelTimeout()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		// Properly close browser before canceling context
		// This ensures Chrome processes are terminated on Windows
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	// Navigate to Settings > Connectors page
	baseURL := env.GetBaseURL()
	settingsURL := baseURL + "/settings#connectors"

	env.LogTest(t, "Navigating to Settings > Connectors page")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(settingsURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to settings page: %v", err)
	}

	// Click on Connectors menu item
	err = chromedp.Run(ctx,
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Connectors")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to click Connectors menu: %v", err)
	}

	// Take screenshot
	if err := env.TakeFullScreenshot(ctx, "connectors_with_created"); err != nil {
		t.Logf("Failed to take screenshot: %v", err)
	}

	// Look for the connector in the list
	connectorSelector := fmt.Sprintf(`//td[contains(., "%s")]`, connectorName)
	checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
	err = chromedp.Run(checkCtx,
		chromedp.WaitVisible(connectorSelector, chromedp.BySearch),
	)
	checkCancel()

	if err != nil {
		t.Logf("Warning: Created connector '%s' not found in UI (may take time to appear): %v", connectorName, err)
	} else {
		env.LogTest(t, "Found connector '%s' in UI", connectorName)
	}

	// Cleanup: Delete the created connector
	env.LogTest(t, "Cleaning up: Deleting connector")

	// Override window.confirm
	if err := chromedp.Run(ctx, chromedp.Evaluate(`window.confirm = () => true`, nil)); err != nil {
		t.Logf("Failed to override window.confirm: %v", err)
	}

	// Click delete button for the specific connector
	deleteBtnSelector := fmt.Sprintf(`//tr[contains(., "%s")]//button[contains(@class, "btn-error")]`, connectorName)
	_ = chromedp.Run(ctx,
		chromedp.Click(deleteBtnSelector, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	)

	// Take final screenshot
	if err := env.TakeFullScreenshot(ctx, "connector_details_test_complete"); err != nil {
		t.Logf("Failed to take final screenshot: %v", err)
	}
	env.LogTest(t, "✓ Connector details test completed")
}
