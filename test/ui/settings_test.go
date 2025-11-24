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

func TestSettings(t *testing.T) {
	// 1. Setup Test Environment
	// This uses the standardized setup:
	// - Builds service to test/bin
	// - Deploys config (setup.toml + test-quaero.toml)
	// - Starts service on port 18085 (default for UI tests)
	// - Redirects output to test/results/ui/{suite}/{test}
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create a timeout context for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	// Create a new allocator context for the browser
	// This ensures we get a fresh browser instance if needed, or reuse existing if configured
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create the browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// Base URL for the service
	baseURL := env.GetBaseURL()
	settingsURL := baseURL + "/settings"

	env.LogTest(t, "Navigating to Settings page: %s", settingsURL)

	// 2. Navigate to Settings Page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(settingsURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Failed to navigate to settings page: %v", err)
	}

	// Take initial screenshot
	if err := env.TakeFullScreenshot(ctx, "settings_page_loaded"); err != nil {
		t.Logf("Failed to take screenshot: %v", err)
	}

	// 3. Configuration Check (Variables) - MOVED UP
	// Verify that auth-apikeys displays keys loaded from test/config/variables/variables.toml
	// The variables.toml defines [test-google-places-key] and [google_api_key]
	// Since API Keys is the default section, we can check immediately.
	env.LogTest(t, "Verifying API Key Configuration loading")

	// Dump HTML to file for debugging
	var bodyHTML string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML)); err != nil {
		t.Logf("Failed to get body HTML: %v", err)
	} else {
		dumpPath := env.GetScreenshotPath("page_dump.html")
		if err := os.WriteFile(dumpPath, []byte(bodyHTML), 0644); err != nil {
			t.Logf("Failed to write page dump: %v", err)
		}
	}

	// Check for the presence of the test keys in the UI
	expectedKeys := []string{"test-google-places-key", "google_api_key"}
	for _, key := range expectedKeys {
		// Check for text content OR value attribute (for input fields)
		selector := fmt.Sprintf(`//*[contains(., "%s") or @value="%s"]`, key, key)

		// Use a shorter timeout for the check to fail fast if not found
		checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
		err := chromedp.Run(checkCtx,
			chromedp.WaitVisible(selector, chromedp.BySearch),
		)
		checkCancel()

		if err != nil {
			t.Errorf("Expected API key '%s' not found in UI: %v", key, err)
		} else {
			env.LogTest(t, "✓ Found API key: %s", key)
		}
	}

	env.TakeScreenshot(ctx, "api_keys_verified")

	// 4. Verify Menu Items and Navigation
	menuItems := []struct {
		id          string
		selector    string
		expectedURL string
		checkText   string
	}{
		{"auth-apikeys", "API Keys", "#auth-apikeys", "API Key Configuration"},
		{"auth-cookies", "Authentication", "#auth-cookies", "Cookie Configuration"},
		{"connectors", "Connectors", "#connectors", "Connector Configuration"},
		{"config", "Configuration", "#config", "System Configuration"},
		{"status", "Status", "#status", "System Status"},
		{"logs", "System Logs", "#logs", "System Logs"},
		// "danger" is skipped for now to avoid accidental resets during basic nav test
	}

	for _, item := range menuItems {
		env.LogTest(t, "Testing menu item: %s", item.id)

		// Click the menu item
		linkSelector := fmt.Sprintf(`//ul[@class="nav"]//li//a[contains(., "%s")]`, item.selector)

		err := env.TakeBeforeAfterScreenshots(ctx, "menu_"+item.id, func() error {
			if err := chromedp.Run(ctx,
				chromedp.Click(linkSelector, chromedp.BySearch),
				chromedp.Sleep(1*time.Second), // Fixed wait for transition
			); err != nil {
				return err
			}

			// Special verification for Logs page
			if item.id == "logs" {
				// 1. Open Filter dropdown
				// 2. Enable Debug and Info filters (default is Warn/Error only)
				// 3. Verify logs appear
				return chromedp.Run(ctx,
					// Open filter dropdown
					chromedp.Click(`//a[contains(., "Filter")]`, chromedp.BySearch),
					chromedp.Sleep(200*time.Millisecond),

					// Enable Debug
					chromedp.Click(`//label[contains(., "Debug")]/input`, chromedp.BySearch),
					chromedp.Sleep(200*time.Millisecond),

					// Enable Info
					chromedp.Click(`//label[contains(., "Info")]/input`, chromedp.BySearch),
					chromedp.Sleep(500*time.Millisecond), // Wait for fetch

					// Verify at least one log line exists
					chromedp.WaitVisible(`.terminal-line`, chromedp.ByQuery),
				)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Failed to verify %s: %v", item.id, err)
			continue
		}
	}

	// 5. User Action: Add a new Connector
	// We'll add a dummy GitHub connector to verify the "Add Item" functionality.
	env.LogTest(t, "Testing User Action: Add Connector")

	// Navigate to Connectors section
	if err := chromedp.Run(ctx,
		chromedp.Click(`//ul[@class="nav"]//li//a[contains(., "Connectors")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to Connectors section: %v", err)
	}

	// Define test data
	connectorName := "Test Connector " + time.Now().Format("150405")
	connectorToken := "skip_validation_token"

	// Perform Add Connector action
	err = chromedp.Run(ctx,
		// Click "New Connector" button
		chromedp.Click(`//button[contains(., "New Connector")]`, chromedp.BySearch),
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Wait for animation

		// Fill form
		chromedp.SendKeys(`//input[@placeholder="e.g. My GitHub"]`, connectorName, chromedp.BySearch),
		chromedp.SendKeys(`//input[@placeholder="ghp_..."]`, connectorToken, chromedp.BySearch),

		// Click Save
		chromedp.Click(`//button[contains(., "Save")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second), // Wait for save and reload
	)

	if err != nil {
		t.Fatalf("Failed to add connector: %v", err)
	}

	env.TakeScreenshot(ctx, "connector_added")

	// Verify connector appears in the list
	// We look for a table row containing the connector name
	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()

	err = chromedp.Run(checkCtx,
		chromedp.WaitVisible(fmt.Sprintf(`//td[contains(., "%s")]`, connectorName), chromedp.BySearch),
	)

	if err != nil {
		// Dump HTML for debugging
		var bodyHTML string
		if dumpErr := chromedp.Run(ctx, chromedp.OuterHTML("body", &bodyHTML)); dumpErr == nil {
			dumpPath := env.GetScreenshotPath("connector_fail_dump.html")
			os.WriteFile(dumpPath, []byte(bodyHTML), 0644)
		}
		t.Errorf("Newly added connector '%s' not found in list: %v", connectorName, err)
	} else {
		env.LogTest(t, "✓ Successfully added and verified connector: %s", connectorName)
	}

	// 6. Cleanup: Delete the created connector
	env.LogTest(t, "Cleaning up: Deleting connector %s", connectorName)

	// Override window.confirm to always return true
	if err := chromedp.Run(ctx, chromedp.Evaluate(`window.confirm = () => true`, nil)); err != nil {
		t.Logf("Failed to override window.confirm: %v", err)
	}

	// Click delete button for the specific connector
	// The delete button is in the same row as the name
	deleteBtnSelector := fmt.Sprintf(`//tr[contains(., "%s")]//button[contains(@class, "btn-error")]`, connectorName)

	err = chromedp.Run(ctx,
		chromedp.Click(deleteBtnSelector, chromedp.BySearch),
		chromedp.Sleep(1*time.Second), // Wait for deletion and reload
	)
	if err != nil {
		t.Errorf("Failed to delete connector: %v", err)
	} else {
		// Verify it's gone
		checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
		defer checkCancel()
		err := chromedp.Run(checkCtx,
			chromedp.WaitNotPresent(fmt.Sprintf(`//td[contains(., "%s")]`, connectorName), chromedp.BySearch),
		)
		if err != nil {
			t.Errorf("Connector '%s' still present after deletion: %v", connectorName, err)
		} else {
			env.LogTest(t, "✓ Successfully deleted connector")
		}
	}
}
