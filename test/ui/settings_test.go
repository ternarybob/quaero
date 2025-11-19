// Package ui contains UI integration tests for settings page.
package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// TestSettingsPageLoad tests that the settings page loads without errors
func TestSettingsPageLoad(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsPageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsPageLoad")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsPageLoad (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsPageLoad (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"
	var title string
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

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		// Check for Refresh button in header
		chromedp.WaitVisible(`//button[@title='Refresh Logs']`, chromedp.BySearch),
		// Check for File Select in body
		chromedp.WaitVisible(`//select[@x-model="selectedFile"]`, chromedp.BySearch),
		// Check for Filter dropdown
		chromedp.WaitVisible(`//a[contains(., 'Filter')]`, chromedp.BySearch),
		// Check for terminal
		chromedp.WaitVisible(`//div[contains(@class, 'terminal')]`, chromedp.BySearch),
		chromedp.Sleep(500*time.Millisecond), // Wait for page to fully load
		chromedp.Title(&title),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	env.LogTest(t, "Page loaded successfully, title: %s", title)

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "settings-page-load"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-page-load"))

	// Check title
	expectedTitle := "Settings - Quaero"
	if title != expectedTitle {
		env.LogTest(t, "ERROR: Title mismatch - expected '%s', got '%s'", expectedTitle, title)
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	} else {
		env.LogTest(t, "✓ Title verified: %s", title)
	}

	// Check for console errors
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Settings page loaded with %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected")
	}

	// Verify settings menu structure exists (standard Spectre nav)
	var hasSettingsMenu bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.nav') !== null`, &hasSettingsMenu),
	)
	if err != nil {
		env.LogTest(t, "ERROR: Failed to check settings menu: %v", err)
		t.Fatalf("Failed to check settings menu: %v", err)
	}

	if !hasSettingsMenu {
		env.LogTest(t, "ERROR: Settings menu structure not found")
		t.Error("Page does not contain settings menu structure")
	} else {
		env.LogTest(t, "✓ Settings menu structure found")
	}

	env.LogTest(t, "✓ Settings page loaded successfully without errors")
}

// TestSettingsMenuClick tests clicking the first menu item (API Keys) and verifies content loads
func TestSettingsMenuClick(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsMenuClick")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsMenuClick")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsMenuClick (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsMenuClick (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"
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

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	env.LogTest(t, "Page loaded successfully")

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "settings-before-apikeys-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-before-apikeys-click"))

	// Find and click the first menu item (API Keys) - updated for standard Spectre nav
	env.LogTest(t, "Clicking API Keys menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`.nav-item:first-child a`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for content to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click API Keys menu item: %v", err)
		t.Fatalf("Failed to click API Keys menu item: %v", err)
	}

	env.LogTest(t, "✓ Clicked API Keys menu item")

	// Take screenshot after clicking
	if err := env.TakeScreenshot(ctx, "settings-after-apikeys-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after click: %v", err)
		t.Fatalf("Failed to take screenshot after click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-after-apikeys-click"))

	// Verify menu item is active (standard Spectre nav-item)
	var isActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.nav-item:first-child').classList.contains('active')`, &isActive),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check menu item state: %v", err)
		t.Fatalf("Failed to check menu item state: %v", err)
	}

	if !isActive {
		env.LogTest(t, "ERROR: API Keys menu item not active after click")
		t.Error("API Keys menu item should be active after click")
	} else {
		env.LogTest(t, "✓ API Keys menu item is active")
	}

	// Wait a bit more for any async content loading
	chromedp.Sleep(1 * time.Second).Do(ctx)

	// Check for console errors after accordion interaction
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors after clicking accordion:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Accordion interaction caused %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected after accordion interaction")
	}

	// Verify API Keys content is visible in the content panel (standard column)
	var contentVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const loadingState = contentPanel.querySelector('.loading-state');
				const hasContent = contentPanel.querySelector('[x-data*="authApiKeys"]') !== null;
				const isLoading = loadingState && window.getComputedStyle(loadingState).display !== 'none';
				return hasContent && !isLoading;
			})()
		`, &contentVisible),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check content visibility: %v", err)
		t.Fatalf("Failed to check content visibility: %v", err)
	}

	if !contentVisible {
		env.LogTest(t, "ERROR: API Keys content not visible in content panel")
		t.Error("API Keys content should be visible when menu item is active")
	} else {
		env.LogTest(t, "✓ API Keys content is visible")
	}

	env.LogTest(t, "✓ API Keys menu item clicked and content loaded without errors")
}

// TestSettingsAuthenticationMenu tests clicking the Authentication menu item and verifies no console errors
func TestSettingsAuthenticationMenu(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsAuthenticationMenu")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAuthenticationMenu")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAuthenticationMenu (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAuthenticationMenu (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"
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

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	env.LogTest(t, "Page loaded successfully")

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "settings-before-authentication-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-before-authentication-click"))

	// Find and click the Authentication menu item (2nd menu item for auth-cookies)
	env.LogTest(t, "Clicking Authentication menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`.nav-item:nth-child(2) a`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for content to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click Authentication menu item: %v", err)
		t.Fatalf("Failed to click Authentication menu item: %v", err)
	}

	env.LogTest(t, "✓ Clicked Authentication menu item")

	// Take screenshot after clicking
	if err := env.TakeScreenshot(ctx, "settings-after-authentication-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after click: %v", err)
		t.Fatalf("Failed to take screenshot after click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-after-authentication-click"))

	// Verify menu item is active
	var isActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.settings-menu-item:nth-child(2)').classList.contains('active')`, &isActive),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check menu item state: %v", err)
		t.Fatalf("Failed to check menu item state: %v", err)
	}

	if !isActive {
		env.LogTest(t, "ERROR: Authentication menu item not active after click")
		t.Error("Authentication menu item should be active after click")
	} else {
		env.LogTest(t, "✓ Authentication menu item is active")
	}

	// Wait a bit more for any async content loading
	chromedp.Sleep(1 * time.Second).Do(ctx)

	// Check for console errors after menu interaction
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors after clicking menu:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Menu interaction caused %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected after menu interaction")
	}

	// Verify Authentication content is visible in the content panel
	var contentVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const hasContent = contentPanel.querySelector('[x-data*="authCookies"]') !== null;
				const loadingState = contentPanel.querySelector('.loading-state');
				const isLoading = loadingState && window.getComputedStyle(loadingState).display !== 'none';
				return hasContent && !isLoading;
			})()
		`, &contentVisible),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check content visibility: %v", err)
		t.Fatalf("Failed to check content visibility: %v", err)
	}

	if !contentVisible {
		env.LogTest(t, "ERROR: Authentication content not visible in content panel")
		t.Error("Authentication content should be visible when menu item is active")
	} else {
		env.LogTest(t, "✓ Authentication content is visible")
	}

	env.LogTest(t, "✓ Authentication menu item clicked and content loaded without errors")
}

// TestSettingsLogsMenu tests clicking the System Logs menu item and verifies no console errors
func TestSettingsLogsMenu(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsLogsMenu")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsLogsMenu")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsLogsMenu (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsLogsMenu (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"
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

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	env.LogTest(t, "Page loaded successfully")

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "settings-before-logs-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-before-logs-click"))

	// Find and click the System Logs menu item
	env.LogTest(t, "Clicking System Logs menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`//a[contains(., 'System Logs')]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second), // Wait for content to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click System Logs menu item: %v", err)
		t.Fatalf("Failed to click System Logs menu item: %v", err)
	}

	env.LogTest(t, "✓ Clicked System Logs menu item")

	// Take screenshot after clicking
	if err := env.TakeScreenshot(ctx, "settings-after-logs-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after click: %v", err)
		t.Fatalf("Failed to take screenshot after click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-after-logs-click"))

	// Verify menu item is active
	var isActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const link = document.evaluate("//a[contains(., 'System Logs')]", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if (!link) return false;
				const li = link.closest('.nav-item');
				return li && li.classList.contains('active');
			})()
		`, &isActive),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check menu item state: %v", err)
		t.Fatalf("Failed to check menu item state: %v", err)
	}

	if !isActive {
		env.LogTest(t, "ERROR: System Logs menu item not active after click")
		t.Error("System Logs menu item should be active after click")
	} else {
		env.LogTest(t, "✓ System Logs menu item is active")
	}

	// Wait a bit more for any async content loading
	chromedp.Sleep(1 * time.Second).Do(ctx)

	// Check for console errors after menu interaction
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors after clicking menu:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Menu interaction caused %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected after menu interaction")
	}

	// Verify Logs content is visible in the content panel
	var isVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminal = document.querySelector('.terminal');
				if (!terminal) return false;
				
				// Check if we have the "no logs" message
				const hasNoLogsMsg = terminal.textContent.includes('No logs found');
				if (hasNoLogsMsg) return false; // Fail if we see "No logs found"

				// Check if we have actual log lines
				const hasLogs = terminal.querySelectorAll('.terminal-line').length > 0;
				return hasLogs;
			})()
		`, &isVisible),
	)

	if err != nil {
		t.Fatalf("System Logs content verification failed: %v", err)
	}
	if !isVisible {
		t.Fatal("System Logs content not visible or 'No logs found' message displayed")
	}
	env.LogTest(t, "✓ System Logs content is visible and populated")

	// Verify log level color coding
	var hasColorCoding bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminal = document.querySelector('.terminal');
				if (!terminal) return false;
				
				// Check if any log lines have color-coded level classes
				const logLines = terminal.querySelectorAll('.terminal-line');
				if (logLines.length === 0) return false;
				
				// Look for text-* color classes (text-error, text-warning, text-primary, text-gray)
				let foundColorClass = false;
				logLines.forEach(line => {
					const spans = line.querySelectorAll('span');
					spans.forEach(span => {
						const classes = span.className;
						if (classes.includes('text-error') || 
						    classes.includes('text-warning') || 
						    classes.includes('text-primary') || 
						    classes.includes('text-gray')) {
							foundColorClass = true;
						}
					});
				});
				
				return foundColorClass;
			})()
		`, &hasColorCoding),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check log level color coding: %v", err)
	} else if !hasColorCoding {
		env.LogTest(t, "WARNING: Log level color coding not detected")
	} else {
		env.LogTest(t, "✓ Log level color coding is applied")
	}

	// Verify terminal is scrollable
	var scrollInfo map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminal = document.querySelector('.terminal');
				if (!terminal) return { error: 'Terminal not found' };
				
				const computedStyle = window.getComputedStyle(terminal);
				const overflowY = computedStyle.overflowY;
				const scrollHeight = terminal.scrollHeight;
				const clientHeight = terminal.clientHeight;
				const isScrollable = scrollHeight > clientHeight;
				
				return {
					overflowY: overflowY,
					scrollHeight: scrollHeight,
					clientHeight: clientHeight,
					isScrollable: isScrollable,
					hasOverflowAuto: overflowY === 'auto' || overflowY === 'scroll'
				};
			})()
		`, &scrollInfo),
	)

	if err != nil {
		env.LogTest(t, "WARNING: Failed to check terminal scrollability: %v", err)
	} else if scrollInfo["error"] != nil {
		env.LogTest(t, "WARNING: Terminal scroll check error: %v", scrollInfo["error"])
	} else {
		hasOverflow := scrollInfo["hasOverflowAuto"].(bool)
		if !hasOverflow {
			env.LogTest(t, "WARNING: Terminal does not have overflow-y: auto/scroll (found: %v)", scrollInfo["overflowY"])
		} else {
			env.LogTest(t, "✓ Terminal is configured for scrolling (overflow-y: %v)", scrollInfo["overflowY"])
		}

		// Log scroll dimensions for debugging
		env.LogTest(t, "  Terminal dimensions: scrollHeight=%v, clientHeight=%v, isScrollable=%v",
			scrollInfo["scrollHeight"], scrollInfo["clientHeight"], scrollInfo["isScrollable"])
	}

	// Verify System Logs use 3-letter format (INF, WRN, ERR, DBG)
	var systemLogLevels map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminal = document.querySelector('.terminal');
				if (!terminal) return { error: 'Terminal not found' };
				
				const logLines = terminal.querySelectorAll('.terminal-line');
				const levels = [];
				const invalidLevels = [];
				
				logLines.forEach(line => {
					// Extract level from log line text - format is [HH:MM:SS] [LEVEL] message
					const text = line.textContent;
					const levelMatch = text.match(/\[(\d{2}:\d{2}:\d{2})\]\s*\[([A-Z]+)\]/);
					if (levelMatch) {
						const level = levelMatch[2];
						levels.push(level);
						// Check if it's a 3-letter code
						if (level.length !== 3 || !['INF', 'WRN', 'ERR', 'DBG'].includes(level)) {
							invalidLevels.push(level);
						}
					}
				});
				
				return {
					totalLogs: logLines.length,
					levels: levels,
					invalidLevels: invalidLevels,
					hasInvalidLevels: invalidLevels.length > 0
				};
			})()
		`, &systemLogLevels),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check System Log level format: %v", err)
		t.Errorf("Failed to check System Log level format: %v", err)
	} else if systemLogLevels["error"] != nil {
		env.LogTest(t, "ERROR: System Log level check error: %v", systemLogLevels["error"])
		t.Errorf("System Log level check error: %v", systemLogLevels["error"])
	} else {
		totalLogs := int(systemLogLevels["totalLogs"].(float64))
		hasInvalidLevels := systemLogLevels["hasInvalidLevels"].(bool)

		if hasInvalidLevels {
			invalidLevels := systemLogLevels["invalidLevels"].([]interface{})
			env.LogTest(t, "ERROR: System Logs contain invalid level formats: %v", invalidLevels)
			t.Errorf("System Logs should use 3-letter format (INF, WRN, ERR, DBG), found: %v", invalidLevels)
		} else {
			env.LogTest(t, "✓ All %d System Log entries use 3-letter format", totalLogs)
		}
	}

	// Navigate to home page to check Service Logs
	env.LogTest(t, "Navigating to home page to check Service Logs...")
	err = chromedp.Run(ctx,
		chromedp.Navigate(env.GetBaseURL()+"/"),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to home page: %v", err)
		t.Fatalf("Failed to navigate to home page: %v", err)
	}

	// Verify Service Logs use 3-letter format (INF, WRN, ERR, DBG)
	var serviceLogLevels map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const terminal = document.querySelector('[x-data="serviceLogs"] .terminal');
				if (!terminal) return { error: 'Service Logs terminal not found' };
				
				const logLines = terminal.querySelectorAll('.terminal-line');
				const levels = [];
				const invalidLevels = [];
				
				logLines.forEach(line => {
					// Extract level from log line text - format is [HH:MM:SS] [LEVEL] message
					const text = line.textContent;
					const levelMatch = text.match(/\[(\d{2}:\d{2}:\d{2})\]\s*\[([A-Z]+)\]/);
					if (levelMatch) {
						const level = levelMatch[2];
						levels.push(level);
						// Check if it's a 3-letter code
						if (level.length !== 3 || !['INF', 'WRN', 'ERR', 'DBG'].includes(level)) {
							invalidLevels.push(level);
						}
					}
				});
				
				return {
					totalLogs: logLines.length,
					levels: levels,
					invalidLevels: invalidLevels,
					hasInvalidLevels: invalidLevels.length > 0
				};
			})()
		`, &serviceLogLevels),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check Service Log level format: %v", err)
		t.Errorf("Failed to check Service Log level format: %v", err)
	} else if serviceLogLevels["error"] != nil {
		env.LogTest(t, "ERROR: Service Log level check error: %v", serviceLogLevels["error"])
		t.Errorf("Service Log level check error: %v", serviceLogLevels["error"])
	} else {
		totalLogs := int(serviceLogLevels["totalLogs"].(float64))
		hasInvalidLevels := serviceLogLevels["hasInvalidLevels"].(bool)

		if hasInvalidLevels {
			invalidLevels := serviceLogLevels["invalidLevels"].([]interface{})
			env.LogTest(t, "ERROR: Service Logs contain invalid level formats: %v", invalidLevels)
			t.Errorf("Service Logs should use 3-letter format (INF, WRN, ERR, DBG), found: %v", invalidLevels)
		} else {
			env.LogTest(t, "✓ All %d Service Log entries use 3-letter format", totalLogs)
		}
	}

	env.LogTest(t, "✓ System Logs menu item clicked and content loaded without errors")
}

// TestSettingsMenuPersistence tests that menu state persists on page refresh
func TestSettingsMenuPersistence(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsMenuPersistence")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsMenuPersistence")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsMenuPersistence (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsMenuPersistence (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second) // Longer timeout for multiple operations
	defer cancel()

	url := env.GetBaseURL() + "/settings"
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

	// Step 1: Load page and click API Keys menu item
	env.LogTest(t, "Step 1: Loading settings page and clicking API Keys menu item...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "settings-before-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-before-click"))

	// Click first menu item (API Keys)
	err = chromedp.Run(ctx,
		chromedp.Click(`.nav-item:first-child`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click menu item: %v", err)
		t.Fatalf("Failed to click menu item: %v", err)
	}

	env.LogTest(t, "✓ API Keys menu item clicked")

	// Verify menu item is active before refresh
	var isActiveBefore bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.settings-menu-item:first-child').classList.contains('active')`, &isActiveBefore),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check menu item state before refresh: %v", err)
		t.Fatalf("Failed to check menu item state before refresh: %v", err)
	}

	if !isActiveBefore {
		env.LogTest(t, "ERROR: API Keys menu item not active before refresh")
		t.Fatal("API Keys menu item should be active before refresh")
	}

	env.LogTest(t, "✓ API Keys menu item confirmed active before refresh")

	// Get current URL with menu state (should have ?a=auth-apikeys)
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.location.href`, &currentURL),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to get current URL: %v", err)
		t.Fatalf("Failed to get current URL: %v", err)
	}

	env.LogTest(t, "Current URL: %s", currentURL)

	if !strings.Contains(currentURL, "a=auth-apikeys") {
		env.LogTest(t, "WARNING: URL does not contain expected menu state parameter ?a=auth-apikeys")
	}

	// Take screenshot before refresh
	if err := env.TakeScreenshot(ctx, "settings-before-refresh"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before refresh: %v", err)
		t.Fatalf("Failed to take screenshot before refresh: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-before-refresh"))

	// Step 2: Refresh the page
	env.LogTest(t, "Step 2: Refreshing page...")
	consoleErrors = nil // Reset console errors for refresh
	err = chromedp.Run(ctx,
		chromedp.Reload(),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for page to fully load and menu to restore state
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to refresh page: %v", err)
		t.Fatalf("Failed to refresh page: %v", err)
	}

	env.LogTest(t, "✓ Page refreshed successfully")

	// Take screenshot after refresh
	if err := env.TakeScreenshot(ctx, "settings-after-refresh"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after refresh: %v", err)
		t.Fatalf("Failed to take screenshot after refresh: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-after-refresh"))

	// Step 3: Verify menu item is still active after refresh
	env.LogTest(t, "Step 3: Verifying menu item state persists after refresh...")
	var isActiveAfter bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.settings-menu-item:first-child').classList.contains('active')`, &isActiveAfter),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check menu item state after refresh: %v", err)
		t.Fatalf("Failed to check menu item state after refresh: %v", err)
	}

	if !isActiveAfter {
		env.LogTest(t, "ERROR: API Keys menu item not active after refresh - state did not persist")
		t.Error("API Keys menu item should remain active after page refresh")
	} else {
		env.LogTest(t, "✓ API Keys menu item state persisted after refresh")
	}

	// Verify content is still visible in content panel
	var contentVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const hasContent = contentPanel.querySelector('[x-data*="authApiKeys"]') !== null;
				const loadingState = contentPanel.querySelector('.loading-state');
				const isLoading = loadingState && window.getComputedStyle(loadingState).display !== 'none';
				return hasContent && !isLoading;
			})()
		`, &contentVisible),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check content visibility after refresh: %v", err)
		t.Fatalf("Failed to check content visibility after refresh: %v", err)
	}

	if !contentVisible {
		env.LogTest(t, "ERROR: API Keys content not visible after refresh")
		t.Error("API Keys content should be visible after page refresh")
	} else {
		env.LogTest(t, "✓ API Keys content visible after refresh")
	}

	// Check for console errors after refresh
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors after refresh:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Errorf("Page refresh caused %d console errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ No console errors detected after refresh")
	}

	env.LogTest(t, "✓ Menu item state persisted successfully after page refresh")
}

// TestSettingsNavigation tests navigation from homepage to settings page
func TestSettingsNavigation(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsNavigation")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsNavigation")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsNavigation (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsNavigation (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	env.LogTest(t, "Testing navigation from homepage to Settings page")

	var title string
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	// Take screenshot before navigation
	if err := env.TakeScreenshot(ctx, "navigation-before-settings"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before navigation: %v", err)
		t.Fatalf("Failed to take screenshot before navigation: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("navigation-before-settings"))

	// Click Settings link
	err = chromedp.Run(ctx,
		chromedp.Click(`a[href="/settings"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Title(&title),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to navigate to Settings: %v", err)
		t.Fatalf("Failed to navigate to Settings: %v", err)
	}

	// Take screenshot after navigation
	screenshotName := "navigation-after-settings"
	if err := env.TakeScreenshot(ctx, screenshotName); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after navigation: %v", err)
		t.Fatalf("Failed to take screenshot after navigation: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath(screenshotName))

	expectedTitle := "Settings"
	if !strings.Contains(title, expectedTitle) {
		env.LogTest(t, "ERROR: Title mismatch - expected to contain '%s', got '%s'",
			expectedTitle, title)
		t.Errorf("After clicking 'Settings', expected title to contain '%s', got '%s'",
			expectedTitle, title)
	} else {
		env.LogTest(t, "✓ Navigation to Settings successful, title: %s", title)
	}
}

// TestSettingsNoConsoleErrorsOnLoad tests that NO console errors occur on initial page load
func TestSettingsNoConsoleErrorsOnLoad(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsNoConsoleErrorsOnLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsNoConsoleErrorsOnLoad")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsNoConsoleErrorsOnLoad (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsNoConsoleErrorsOnLoad (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"
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

	env.LogTest(t, "Loading settings page and checking for console errors...")
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to fully initialize
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	// Take screenshot
	if err := env.TakeScreenshot(ctx, "settings-no-console-errors"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("settings-no-console-errors"))

	// CRITICAL TEST: Verify NO console errors exist
	if len(consoleErrors) > 0 {
		env.LogTest(t, "ERROR: Found %d console errors on initial page load:", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  Console error %d: %s", i+1, errMsg)
		}
		t.Fatalf("FAIL: Settings page loaded with %d console errors - expected ZERO errors", len(consoleErrors))
	} else {
		env.LogTest(t, "✓ PASS: NO console errors detected on initial page load")
	}
}

// TestSettingsAuthenticationMenuLoadsAndStops tests that Authentication menu loads and stops (not infinite loading)
func TestSettingsAuthenticationMenuLoadsAndStops(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsAuthenticationMenuLoadsAndStops")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsAuthenticationMenuLoadsAndStops")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsAuthenticationMenuLoadsAndStops (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsAuthenticationMenuLoadsAndStops (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "auth-before-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("auth-before-click"))

	// Click Authentication menu item (2nd menu item for auth-cookies)
	env.LogTest(t, "Clicking Authentication menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`.nav-item:nth-child(2) a`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Give it time to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click Authentication menu item: %v", err)
		t.Fatalf("Failed to click Authentication menu item: %v", err)
	}

	// Take screenshot after clicking
	if err := env.TakeScreenshot(ctx, "auth-after-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after click: %v", err)
		t.Fatalf("Failed to take screenshot after click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("auth-after-click"))

	// CRITICAL TEST: Verify loading spinner is NOT visible (loading has stopped)
	var isLoadingVisible bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const loadingState = contentPanel.querySelector('.loading-state');
				if (!loadingState) return false;
				const loadingText = Array.from(loadingState.querySelectorAll('p'))
					.find(p => p.textContent.includes('Loading authentications'));
				if (!loadingText) return false;
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
		env.LogTest(t, "ERROR: Authentication menu stuck in loading state (infinite loading)")
		t.Fatal("FAIL: Authentication menu is still loading - should have stopped loading")
	} else {
		env.LogTest(t, "✓ PASS: Authentication menu finished loading (not infinite loading)")
	}

	// Verify the component initialized properly (either shows data or "no authentications" message)
	var hasContent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				// Check if content loaded (not loading spinner)
				const hasAuthContent = contentPanel.querySelector('[x-data*="authCookies"]') !== null;
				return hasAuthContent;
			})()
		`, &hasContent),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check content initialization: %v", err)
		t.Fatalf("Failed to check content initialization: %v", err)
	}

	if !hasContent {
		env.LogTest(t, "ERROR: Authentication content did not initialize")
		t.Error("Authentication component should be initialized (even if empty)")
	} else {
		env.LogTest(t, "✓ Authentication component initialized successfully")
	}

	env.LogTest(t, "✓ PASS: Authentication menu loads and stops (not infinite loading)")
}

// TestSettingsConfigurationMenuLoads tests that Configuration menu panel loads correctly
func TestSettingsConfigurationMenuLoads(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("SettingsConfigurationMenuLoads")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestSettingsConfigurationMenuLoads")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestSettingsConfigurationMenuLoads (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestSettingsConfigurationMenuLoads (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/settings"

	env.LogTest(t, "Navigating to settings page: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load settings page: %v", err)
		t.Fatalf("Failed to load settings page: %v", err)
	}

	// Take screenshot before clicking
	if err := env.TakeScreenshot(ctx, "config-before-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot before click: %v", err)
		t.Fatalf("Failed to take screenshot before click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("config-before-click"))

	// Click Configuration menu item (3rd menu item for config)
	env.LogTest(t, "Clicking Configuration menu item...")
	err = chromedp.Run(ctx,
		chromedp.Click(`.nav-item:nth-child(3) a`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Give it time to load
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to click Configuration menu item: %v", err)
		t.Fatalf("Failed to click Configuration menu item: %v", err)
	}

	// Take screenshot after clicking
	if err := env.TakeScreenshot(ctx, "config-after-click"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot after click: %v", err)
		t.Fatalf("Failed to take screenshot after click: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("config-after-click"))

	// CRITICAL TEST: Verify "No configuration loaded" error does NOT appear in content panel
	var hasErrorMessage bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const errorText = Array.from(contentPanel.querySelectorAll('p, div'))
					.find(el => el.textContent.includes('No configuration loaded'));
				if (!errorText) return false;
				const computedStyle = window.getComputedStyle(errorText);
				return computedStyle.display !== 'none';
			})()
		`, &hasErrorMessage),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for error message: %v", err)
		t.Fatalf("Failed to check for error message: %v", err)
	}

	if hasErrorMessage {
		env.LogTest(t, "ERROR: Configuration menu showing 'No configuration loaded' error")
		t.Fatal("FAIL: Configuration menu should load successfully without error")
	} else {
		env.LogTest(t, "✓ Configuration menu loaded without error message")
	}

	// Verify Configuration content is present in content panel
	var hasConfigContent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const contentPanel = document.querySelector('.column.col-9, .column.col-sm-12');
				if (!contentPanel) return false;
				const configPanel = contentPanel.querySelector('.config-panel, [x-data*="settingsConfig"], pre, code');
				return configPanel !== null;
			})()
		`, &hasConfigContent),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for configuration content: %v", err)
		t.Fatalf("Failed to check for configuration content: %v", err)
	}

	if !hasConfigContent {
		env.LogTest(t, "ERROR: Configuration content not found in content panel")
		t.Error("Configuration menu should show configuration data")
	} else {
		env.LogTest(t, "✓ Configuration content is present in content panel")
	}

	env.LogTest(t, "✓ PASS: Configuration menu panel loads correctly")
}
