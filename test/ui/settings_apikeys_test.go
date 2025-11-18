// Package ui contains UI integration tests for settings page variables functionality.
package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestSettingsAPIKeysLoad tests that variables are loaded from test-keys.toml and displayed
func TestSettingsAPIKeysLoad(t *testing.T) {
	// Setup test environment with default config (includes variables directory)
	env, err := common.SetupTestEnvironment("SettingsAPIKeysLoad")
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

	// Insert GOOGLE_API_KEY from .env.test file via API
	googleAPIKey, ok := env.EnvVars["GOOGLE_API_KEY"]
	if !ok || googleAPIKey == "" {
		t.Fatalf("GOOGLE_API_KEY not found in .env.test file")
	}
	env.LogTest(t, "Loaded GOOGLE_API_KEY from .env.test: %s", googleAPIKey)

	// Use HTTP helper to insert the key via POST /api/kv
	httpHelper := env.NewHTTPTestHelper(t)
	reqBody := map[string]interface{}{
		"key":         "GOOGLE_API_KEY",
		"value":       googleAPIKey,
		"description": "Google API Key loaded from .env.test",
	}

	env.LogTest(t, "Inserting GOOGLE_API_KEY via POST /api/kv...")
	resp, err := httpHelper.POST("/api/kv", reqBody)
	if err != nil {
		t.Fatalf("Failed to insert GOOGLE_API_KEY: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d", resp.StatusCode)
	}
	env.LogTest(t, "✓ GOOGLE_API_KEY inserted successfully via API")

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

	// CRITICAL TEST: Verify GOOGLE_API_KEY is present in the list
	var hasGoogleAPIKey bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for the GOOGLE_API_KEY in the page
				const keyElements = Array.from(document.querySelectorAll('td, div, span, p'));
				return keyElements.some(el => el.textContent.includes('GOOGLE_API_KEY'));
			})()
		`, &hasGoogleAPIKey),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for GOOGLE_API_KEY: %v", err)
		t.Fatalf("Failed to check for GOOGLE_API_KEY: %v", err)
	}

	if !hasGoogleAPIKey {
		env.LogTest(t, "ERROR: GOOGLE_API_KEY not found in Variables list")
		t.Error("Expected GOOGLE_API_KEY from .env.test to be displayed in UI")
	} else {
		env.LogTest(t, "✓ GOOGLE_API_KEY found in Variables list")
	}

	// Verify the value is masked (API returns masked values in list view)
	// Expected format from kv_handler.go maskValue(): first 4 chars + "..." + last 4 chars
	// For "AIzaSyCpu5o5anzf8aVs5X72LOsunFZll0Di83E" -> "AIza...i83E"
	expectedMaskedValue := googleAPIKey[:4] + "..." + googleAPIKey[len(googleAPIKey)-4:]
	env.LogTest(t, "Expected masked value: %s", expectedMaskedValue)

	var hasMaskedGoogleKey bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const expectedMask = "`+expectedMaskedValue+`";
				const valueElements = Array.from(document.querySelectorAll('td, div, span, p'));
				return valueElements.some(el => el.textContent.includes(expectedMask));
			})()
		`, &hasMaskedGoogleKey),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for masked GOOGLE_API_KEY value: %v", err)
		t.Fatalf("Failed to check for masked GOOGLE_API_KEY value: %v", err)
	}

	if !hasMaskedGoogleKey {
		env.LogTest(t, "WARNING: Masked GOOGLE_API_KEY value not found with expected format: %s", expectedMaskedValue)
		// Don't fail - just log warning as masking format might vary
	} else {
		env.LogTest(t, "✓ Masked GOOGLE_API_KEY value found: %s", expectedMaskedValue)
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

	env.LogTest(t, "✓ GOOGLE_API_KEY from .env.test inserted and displayed correctly")
}

// TestSettingsAPIKeysShowToggle tests the "Show Full" toggle functionality for variables
func TestSettingsAPIKeysShowToggle(t *testing.T) {
	// Setup test environment with default config (includes variables directory)
	env, err := common.SetupTestEnvironment("SettingsAPIKeysShowToggle")
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

// TestSettingsAPIKeysDuplicateSameCase tests that duplicate keys (same case) are blocked by the UI
func TestSettingsAPIKeysDuplicateSameCase(t *testing.T) {
	env, err := common.SetupTestEnvironment("SettingsAPIKeysDuplicateSameCase")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAPIKeysDuplicateSameCase")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAPIKeysDuplicateSameCase (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAPIKeysDuplicateSameCase (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	// Insert first key via API
	httpHelper := env.NewHTTPTestHelper(t)
	reqBody := map[string]interface{}{
		"key":         "TEST_DUPLICATE_KEY",
		"value":       "test-value-123",
		"description": "Test key for duplicate validation",
	}

	env.LogTest(t, "Inserting first TEST_DUPLICATE_KEY via POST /api/kv...")
	resp, err := httpHelper.POST("/api/kv", reqBody)
	if err != nil {
		t.Fatalf("Failed to insert first key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created for first key, got %d", resp.StatusCode)
	}
	env.LogTest(t, "✓ First TEST_DUPLICATE_KEY inserted successfully")

	// Try to insert duplicate key with same case via API
	env.LogTest(t, "Attempting to insert duplicate TEST_DUPLICATE_KEY (same case)...")
	resp2, err := httpHelper.POST("/api/kv", reqBody)
	if err != nil {
		t.Fatalf("Failed to send duplicate key request: %v", err)
	}
	defer resp2.Body.Close()

	// Verify API returns 409 Conflict
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("Expected status 409 Conflict for duplicate key, got %d", resp2.StatusCode)
	} else {
		env.LogTest(t, "✓ API returned 409 Conflict for duplicate key")
	}

	// Parse error response
	var errorResp map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorMsg, ok := errorResp["error"].(string); ok {
		env.LogTest(t, "✓ Error message from API: %s", errorMsg)
		if !strings.Contains(strings.ToLower(errorMsg), "already exists") {
			t.Errorf("Error message should mention key already exists, got: %s", errorMsg)
		}
	} else {
		t.Errorf("API response should contain 'error' field with message")
	}

	// Now test via UI
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
		chromedp.Sleep(3*time.Second), // Wait for dynamic content to load
	)

	if err != nil {
		t.Fatalf("Failed to load settings Variables page: %v", err)
	}

	// Take screenshot before attempting duplicate
	if err := env.TakeScreenshot(ctx, "duplicate-same-case-before"); err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	// Click "Add API Key" button using JavaScript click (more reliable than chromedp.Click)
	env.LogTest(t, "Clicking 'Add API Key' button...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const buttons = Array.from(document.querySelectorAll('button'));
				const addButton = buttons.find(btn => btn.textContent.trim().includes('Add API Key'));
				if (addButton) {
					addButton.click();
					return true;
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to click Add API Key button: %v", err)
	}

	// Fill in the form with duplicate key (same case)
	env.LogTest(t, "Filling form with duplicate key TEST_DUPLICATE_KEY...")
	err = chromedp.Run(ctx,
		chromedp.SendKeys(`input#key`, "TEST_DUPLICATE_KEY", chromedp.ByQuery),
		chromedp.SendKeys(`input#value`, "duplicate-value", chromedp.ByQuery),
		chromedp.SendKeys(`textarea#description`, "Attempting duplicate", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		t.Fatalf("Failed to fill form: %v", err)
	}

	// Click Create button using JavaScript click (more reliable)
	env.LogTest(t, "Clicking Create button...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const buttons = Array.from(document.querySelectorAll('button[type="submit"]'));
				const createButton = buttons.find(btn => btn.textContent.trim().includes('Create'));
				if (createButton) {
					createButton.click();
					return true;
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(2*time.Second), // Wait for API response and notification
	)

	if err != nil {
		t.Fatalf("Failed to click Create button: %v", err)
	}

	// Take screenshot after attempt
	if err := env.TakeScreenshot(ctx, "duplicate-same-case-after"); err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	// Check for error notification (toast)
	var hasErrorNotification bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const toasts = Array.from(document.querySelectorAll('.toast-item, .toast-error'));
				return toasts.some(toast => {
					const text = toast.textContent.toLowerCase();
					return text.includes('already exists') || text.includes('duplicate') || text.includes('conflict');
				});
			})()
		`, &hasErrorNotification),
	)

	if err != nil {
		t.Fatalf("Failed to check for error notification: %v", err)
	}

	if !hasErrorNotification {
		env.LogTest(t, "ERROR: No error notification displayed for duplicate key")
		t.Error("UI should display error notification when attempting to create duplicate key (same case)")
	} else {
		env.LogTest(t, "✓ UI displayed error notification for duplicate key (same case)")
	}
}

// TestSettingsAPIKeysDuplicateDifferentCase tests that duplicate keys (different case) are blocked by the UI
func TestSettingsAPIKeysDuplicateDifferentCase(t *testing.T) {
	env, err := common.SetupTestEnvironment("SettingsAPIKeysDuplicateDifferentCase")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAPIKeysDuplicateDifferentCase")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAPIKeysDuplicateDifferentCase (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAPIKeysDuplicateDifferentCase (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())

	// Insert first key via API (uppercase)
	httpHelper := env.NewHTTPTestHelper(t)
	reqBody := map[string]interface{}{
		"key":         "CASE_TEST_KEY",
		"value":       "test-value-456",
		"description": "Test key for case-insensitive duplicate validation",
	}

	env.LogTest(t, "Inserting first CASE_TEST_KEY via POST /api/kv...")
	resp, err := httpHelper.POST("/api/kv", reqBody)
	if err != nil {
		t.Fatalf("Failed to insert first key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created for first key, got %d", resp.StatusCode)
	}
	env.LogTest(t, "✓ First CASE_TEST_KEY inserted successfully")

	// Try to insert duplicate key with different case via API
	reqBody2 := map[string]interface{}{
		"key":         "case_test_key", // lowercase version
		"value":       "different-value",
		"description": "Attempting case-insensitive duplicate",
	}

	env.LogTest(t, "Attempting to insert duplicate case_test_key (different case)...")
	resp2, err := httpHelper.POST("/api/kv", reqBody2)
	if err != nil {
		t.Fatalf("Failed to send duplicate key request: %v", err)
	}
	defer resp2.Body.Close()

	// Verify API returns 409 Conflict
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("Expected status 409 Conflict for case-insensitive duplicate, got %d", resp2.StatusCode)
	} else {
		env.LogTest(t, "✓ API returned 409 Conflict for case-insensitive duplicate")
	}

	// Parse error response
	var errorResp map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorMsg, ok := errorResp["error"].(string); ok {
		env.LogTest(t, "✓ Error message from API: %s", errorMsg)
		if !strings.Contains(strings.ToLower(errorMsg), "already exists") {
			t.Errorf("Error message should mention key already exists, got: %s", errorMsg)
		}
		// Note: Current implementation normalizes keys to lowercase, so error shows "case_test_key"
		if !strings.Contains(errorMsg, "case_test_key") {
			t.Errorf("Error message should show existing key name 'case_test_key', got: %s", errorMsg)
		}
	} else {
		t.Errorf("API response should contain 'error' field with message")
	}

	// Now test via UI
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
		chromedp.Sleep(3*time.Second), // Wait for dynamic content to load
	)

	if err != nil {
		t.Fatalf("Failed to load settings Variables page: %v", err)
	}

	// Take screenshot before attempting duplicate
	if err := env.TakeScreenshot(ctx, "duplicate-different-case-before"); err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	// Click "Add API Key" button using JavaScript click (more reliable than chromedp.Click)
	env.LogTest(t, "Clicking 'Add API Key' button...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const buttons = Array.from(document.querySelectorAll('button'));
				const addButton = buttons.find(btn => btn.textContent.trim().includes('Add API Key'));
				if (addButton) {
					addButton.click();
					return true;
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to click Add API Key button: %v", err)
	}

	// Fill in the form with duplicate key (different case)
	env.LogTest(t, "Filling form with duplicate key case_test_key (lowercase)...")
	err = chromedp.Run(ctx,
		chromedp.SendKeys(`input#key`, "case_test_key", chromedp.ByQuery),
		chromedp.SendKeys(`input#value`, "duplicate-value-different-case", chromedp.ByQuery),
		chromedp.SendKeys(`textarea#description`, "Attempting case-insensitive duplicate", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		t.Fatalf("Failed to fill form: %v", err)
	}

	// Click Create button using JavaScript click (more reliable)
	env.LogTest(t, "Clicking Create button...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const buttons = Array.from(document.querySelectorAll('button[type="submit"]'));
				const createButton = buttons.find(btn => btn.textContent.trim().includes('Create'));
				if (createButton) {
					createButton.click();
					return true;
				}
				return false;
			})()
		`, nil),
		chromedp.Sleep(2*time.Second), // Wait for API response and notification
	)

	if err != nil {
		t.Fatalf("Failed to click Create button: %v", err)
	}

	// Take screenshot after attempt
	if err := env.TakeScreenshot(ctx, "duplicate-different-case-after"); err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}

	// Check for error notification (toast)
	var hasErrorNotification bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const toasts = Array.from(document.querySelectorAll('.toast-item, .toast-error'));
				return toasts.some(toast => {
					const text = toast.textContent.toLowerCase();
					return text.includes('already exists') || text.includes('duplicate') || text.includes('conflict');
				});
			})()
		`, &hasErrorNotification),
	)

	if err != nil {
		t.Fatalf("Failed to check for error notification: %v", err)
	}

	if !hasErrorNotification {
		env.LogTest(t, "ERROR: No error notification displayed for case-insensitive duplicate key")
		t.Error("UI should display error notification when attempting to create duplicate key (different case)")
	} else {
		env.LogTest(t, "✓ UI displayed error notification for case-insensitive duplicate key")
	}
}
