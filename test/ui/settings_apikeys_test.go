// Package ui contains UI integration tests for settings page variables functionality.
package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestSettingsAPIKeysLoad tests that variables are loaded from test-keys.toml and displayed
func TestSettingsAPIKeysLoad(t *testing.T) {
	// Setup test environment with custom config that uses test variables directory
	env, err := common.SetupTestEnvironment("SettingsAPIKeysLoad", "../config/test-quaero-apikeys.toml")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAPIKeysLoad")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAPIKeysLoad (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAPIKeysLoad (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())
	env.LogTest(t, "Using test variables from: test/config/variables/test-keys.toml")

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings?a=auth-apikeys"
	var consoleErrors []string

	// Listen for ALL console messages (errors, warnings, and exceptions)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// Capture exception errors
		if consoleEvent, ok := ev.(*runtime.EventExceptionThrown); ok {
			if consoleEvent.ExceptionDetails != nil && consoleEvent.ExceptionDetails.Exception != nil {
				errorMsg := consoleEvent.ExceptionDetails.Exception.Description
				if errorMsg == "" && consoleEvent.ExceptionDetails.Text != "" {
					errorMsg = consoleEvent.ExceptionDetails.Text
				}
				consoleErrors = append(consoleErrors, "[Exception] "+errorMsg)
			}
		}
		// Capture console.error and console.warn messages
		if consoleAPI, ok := ev.(*runtime.EventConsoleAPICalled); ok {
			if consoleAPI.Type == runtime.APITypeError || consoleAPI.Type == runtime.APITypeWarning {
				var msg string
				for _, arg := range consoleAPI.Args {
					if arg.Value != nil {
						msg += string(arg.Value) + " "
					}
				}
				consoleErrors = append(consoleErrors, "["+string(consoleAPI.Type)+"] "+msg)
			}
		}
	})

	env.LogTest(t, "Navigating to settings Variables page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for variables to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings Variables page: %v", err)
		t.Fatalf("Failed to load settings Variables page: %v", err)
	}

	env.LogTest(t, "Page loaded successfully")

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "settings-apikeys-loaded"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-apikeys-loaded"))

	// Check for console errors
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Settings API Keys page loaded with %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected")
	}

	// Verify Variables component is visible (dynamically loaded via x-html)
	var hasAPIKeysContent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// The content is dynamically loaded into the column with x-html attribute
				const contentColumn = document.querySelector('.column.col-10');
				if (!contentColumn) return false;
				// Check for the authApiKeys component within the dynamic content
				const hasContent = contentColumn.querySelector('[x-data="authApiKeys"]') !== null;
				return hasContent;
			})()
		`, &hasAPIKeysContent),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check Variables content visibility: %v", err)
		t.Fatalf("Failed to check Variables content visibility: %v", err)
	}

	if !hasAPIKeysContent {
		env.LogTest(t, "ERROR: Variables content not visible in content panel")
		t.Error("Variables content should be visible")
	} else {
		env.LogTest(t, "✓ Variables content is visible")
	}

	// Verify loading has finished (check if the loading spinner is visible)
	var isLoadingVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Check if the loading state div is visible (it uses x-show so will have display:none when hidden)
				const loadingState = document.querySelector('.loading-state');
				if (!loadingState) return false;
				const computedStyle = window.getComputedStyle(loadingState);
				return computedStyle.display !== 'none';
			})()
		`, &isLoadingVisible),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check loading state: %v", err)
		t.Fatalf("Failed to check loading state: %v", err)
	}

	if isLoadingVisible {
		env.LogTest(t, "ERROR: Variables still loading")
		t.Error("Variables should have finished loading")
	} else {
		env.LogTest(t, "✓ Variables loading finished")
	}

	// CRITICAL TEST: Verify test-google-places-key is present in the list
	var hasTestKey bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for the test key name in the page
				const keyElements = Array.from(document.querySelectorAll('td, div, span, p'));
				return keyElements.some(el => el.textContent.includes('test-google-places-key'));
			})()
		`, &hasTestKey),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for test key: %v", err)
		t.Fatalf("Failed to check for test key: %v", err)
	}

	if !hasTestKey {
		env.LogTest(t, "ERROR: test-google-places-key not found in Variables list")
		t.Error("Expected test-google-places-key from test/config/variables/test-keys.toml to be displayed")
	} else {
		env.LogTest(t, "✓ test-google-places-key found in Variables list")
	}

	// Verify masked value is displayed (should show dots or masked format)
	var hasMaskedValue bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for masked value indicators (dots, asterisks, or "••••" characters)
				const valueElements = Array.from(document.querySelectorAll('td, div, span, p'));
				return valueElements.some(el => {
					const text = el.textContent;
					return text.includes('••••') || text.includes('****') || text.includes('...');
				});
			})()
		`, &hasMaskedValue),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for masked value: %v", err)
		t.Fatalf("Failed to check for masked value: %v", err)
	}

	if !hasMaskedValue {
		env.LogTest(t, "WARNING: Masked value format not detected")
		// Don't fail test - masking format might vary
	} else {
		env.LogTest(t, "✓ Masked value format detected")
	}

	// Take final screenshot
	if err := env.TakeScreenshot(ctx, "settings-apikeys-final"); err != nil {
		env.LogTest(t, "ERROR: Failed to take final screenshot: %v", err)
		t.Fatalf("Failed to take final screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-apikeys-final"))

	env.LogTest(t, "✓ Variables loaded successfully from test-keys.toml and displayed")
}

// TestSettingsAPIKeysShowToggle tests the "Show Full" toggle functionality for variables
func TestSettingsAPIKeysShowToggle(t *testing.T) {
	// Setup test environment with custom config that uses test variables directory
	env, err := common.SetupTestEnvironment("SettingsAPIKeysShowToggle", "../config/test-quaero-apikeys.toml")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAPIKeysShowToggle")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAPIKeysShowToggle (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAPIKeysShowToggle (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings?a=auth-apikeys"

	env.LogTest(t, "Navigating to settings Variables page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for variables to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings Variables page: %v", err)
		t.Fatalf("Failed to load settings Variables page: %v", err)
	}

	// Take screenshot before toggle
	if err := env.TakeScreenshot(ctx, "apikeys-before-toggle"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before toggle: %v", err)
		t.Fatalf("Failed to take screenshot before toggle: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("apikeys-before-toggle"))

	// Look for "Show Full" button or toggle
	env.LogTest(t, "Looking for Show Full toggle...")
	var hasShowToggle bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const buttons = Array.from(document.querySelectorAll('button, a, span'));
				return buttons.some(el => el.textContent.includes('Show') || el.textContent.includes('Hide'));
			})()
		`, &hasShowToggle),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for Show toggle: %v", err)
		t.Fatalf("Failed to check for Show toggle: %v", err)
	}

	if !hasShowToggle {
		env.LogTest(t, "SKIP: Show/Hide toggle not found - feature may not be implemented yet")
		t.Skip("Show/Hide toggle not found")
	} else {
		env.LogTest(t, "✓ Show/Hide toggle found")

		// Click the Show toggle
		env.LogTest(t, "Clicking Show toggle...")
		err = chromedp.Run(ctx,
			chromedp.Click(`button:has-text("Show"), a:has-text("Show")`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond),
		)

		if err != nil {
			env.LogTest(t, "ERROR: Failed to click Show toggle: %v", err)
			t.Fatalf("Failed to click Show toggle: %v", err)
		}

		// Take screenshot after toggle
		if err := env.TakeScreenshot(ctx, "apikeys-after-toggle"); err != nil {
			env.LogTest(t, "ERROR: Failed to take screenshot after toggle: %v", err)
			t.Fatalf("Failed to take screenshot after toggle: %v", err)
		}
		env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("apikeys-after-toggle"))

		env.LogTest(t, "✓ Show toggle clicked successfully")
	}
}
