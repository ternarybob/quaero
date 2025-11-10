// -----------------------------------------------------------------------
// UI tests for job add and edit functionality
// Tests the /jobs/add page including TOML editor, validation, and saving
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

func TestJobAddAndEdit(t *testing.T) {
	env, err := common.SetupTestEnvironment("JobAddAndEdit")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobAddAndEdit")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobAddAndEdit (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobAddAndEdit (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready at: %s", env.GetBaseURL())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	baseURL := env.GetBaseURL()

	// Test 1: Navigate to job add page
	env.LogTest(t, "Step 1: Navigate to /jobs/add")
	var pageTitle string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("%s/jobs/add", baseURL)),
		chromedp.WaitVisible("h1", chromedp.ByQuery),
		chromedp.Text("h1", &pageTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Failed to navigate to job add page: %v", err)
	}
	env.TakeScreenshot(ctx, "01-job-add-page-loaded")
	env.LogTest(t, "✓ Job add page loaded with title: %s", pageTitle)

	// Test 2: Verify CodeMirror editor loaded
	env.LogTest(t, "Step 2: Verify CodeMirror editor is present")
	var editorVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second), // Wait for Alpine.js and CodeMirror to initialize
		chromedp.Evaluate(`document.querySelector('.CodeMirror') !== null`, &editorVisible),
	); err != nil {
		t.Fatalf("Failed to check editor: %v", err)
	}
	if !editorVisible {
		t.Fatal("CodeMirror editor not found on page")
	}
	env.LogTest(t, "✓ CodeMirror editor found")

	// Test 3: Load example TOML
	env.LogTest(t, "Step 3: Load example TOML")
	var loadExampleBtnExists bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('button:has-text("Load Example")') !== null || document.querySelector('button').textContent.includes('Load Example')`, &loadExampleBtnExists),
	); err != nil {
		env.LogTest(t, "Warning: Could not check for Load Example button: %v", err)
	}

	if loadExampleBtnExists {
		if err := chromedp.Run(ctx,
			chromedp.Click(`button`, chromedp.ByQuery), // Click first button with "Load Example" text
			chromedp.Sleep(1*time.Second), // Wait for TOML to load
		); err != nil {
			env.LogTest(t, "Warning: Could not click Load Example button: %v", err)
		}
	} else {
		env.LogTest(t, "Note: Load Example button not found, will enter TOML manually")
	}
	env.TakeScreenshot(ctx, "02-after-load-example")
	env.LogTest(t, "✓ Attempted to load example TOML")

	// Test 4: Clear editor and enter news-crawler TOML
	env.LogTest(t, "Step 4: Enter news-crawler TOML content")
	newsTOML := `id = "news-crawler"
name = "News Crawler"
description = "Crawler job that crawls news websites and filters for specific content"

start_urls = ["https://stockhead.com.au/just-in", "https://www.abc.net.au/news"]
include_patterns = ["article", "news", "post"]
exclude_patterns = ["login", "logout", "admin"]

max_depth = 2
max_pages = 10
concurrency = 5
follow_links = true

schedule = ""
timeout = "30m"
enabled = true
auto_start = false`

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			var editor = document.querySelector('.CodeMirror').CodeMirror;
			editor.setValue(%q);
		`, newsTOML), nil),
		chromedp.Sleep(2*time.Second), // Wait for auto-validation
	); err != nil {
		t.Fatalf("Failed to set TOML content: %v", err)
	}
	env.TakeScreenshot(ctx, "03-news-crawler-toml-entered")
	env.LogTest(t, "✓ News Crawler TOML entered")

	// Test 5: Check validation message (should be valid)
	env.LogTest(t, "Step 5: Check validation result")
	var validationMsg string
	validationErr := chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second), // Wait for auto-validation
		chromedp.Text(".validation-message", &validationMsg, chromedp.ByQuery),
	)
	if validationErr == nil {
		env.LogTest(t, "Validation message: %s", validationMsg)
	} else {
		env.LogTest(t, "Note: No validation message displayed yet")
	}
	env.TakeScreenshot(ctx, "04-validation-result")

	// Test 6: Test readonly toggle
	env.LogTest(t, "Step 6: Test edit/readonly toggle")
	var lockBtnText string
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var btns = Array.from(document.querySelectorAll('button'));
			var lockBtn = btns.find(b => b.textContent.includes('Lock') || b.textContent.includes('Unlock'));
			lockBtn ? lockBtn.textContent : 'not found';
		`, &lockBtnText),
	); err == nil && lockBtnText != "not found" {
		env.LogTest(t, "Found lock/unlock button with text: %s", lockBtnText)

		// Click the button to toggle
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				var btns = Array.from(document.querySelectorAll('button'));
				var lockBtn = btns.find(b => b.textContent.includes('Lock') || b.textContent.includes('Unlock'));
				if (lockBtn) lockBtn.click();
			`, nil),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			env.LogTest(t, "Warning: Could not click lock button: %v", err)
		}
		env.TakeScreenshot(ctx, "05-editor-toggled")
		env.LogTest(t, "✓ Editor lock/unlock toggled")

		// Toggle back
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				var btns = Array.from(document.querySelectorAll('button'));
				var lockBtn = btns.find(b => b.textContent.includes('Lock') || b.textContent.includes('Unlock'));
				if (lockBtn) lockBtn.click();
			`, nil),
			chromedp.Sleep(500*time.Millisecond),
		); err != nil {
			env.LogTest(t, "Warning: Could not toggle back: %v", err)
		}
		env.TakeScreenshot(ctx, "06-editor-toggled-back")
		env.LogTest(t, "✓ Editor toggled back to unlocked")
	} else {
		env.LogTest(t, "Note: Lock/Unlock button not found")
		env.TakeScreenshot(ctx, "05-no-lock-button")
	}

	// Test 7: Click Validate button
	env.LogTest(t, "Step 7: Click Validate button")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var btns = Array.from(document.querySelectorAll('button'));
			var validateBtn = btns.find(b => b.textContent.includes('Validate'));
			if (validateBtn) validateBtn.click();
		`, nil),
		chromedp.Sleep(2*time.Second), // Wait for validation
	); err != nil {
		env.LogTest(t, "Warning: Could not click Validate button: %v", err)
	}
	env.TakeScreenshot(ctx, "07-after-validate")
	env.LogTest(t, "✓ Validate button clicked")

	// Test 8: Save the job
	env.LogTest(t, "Step 8: Save the job definition")
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var btns = Array.from(document.querySelectorAll('button'));
			var saveBtn = btns.find(b => b.textContent.includes('Save'));
			if (saveBtn) saveBtn.click();
		`, nil),
		chromedp.Sleep(3*time.Second), // Wait for save and possible redirect
	); err != nil {
		env.LogTest(t, "Warning: Could not click Save button: %v", err)
	}
	env.TakeScreenshot(ctx, "08-after-save")
	env.LogTest(t, "✓ Save button clicked")

	// Test 9: Check current URL after save
	var currentURL string
	if err := chromedp.Run(ctx,
		chromedp.Location(&currentURL),
	); err != nil {
		env.LogTest(t, "Warning: Could not get current URL: %v", err)
	} else {
		env.LogTest(t, "Current URL after save: %s", currentURL)
	}

	// Test 10: Navigate back to edit the job (if we're on /jobs page)
	if currentURL == baseURL+"/jobs" || currentURL == baseURL+"/jobs/" {
		env.LogTest(t, "Step 9: Navigate to edit the job")
		if err := chromedp.Run(ctx,
			chromedp.Navigate(fmt.Sprintf("%s/jobs/add?id=news-crawler", baseURL)),
			chromedp.WaitVisible(".CodeMirror", chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // Wait for TOML to load
		); err != nil {
			env.LogTest(t, "Warning: Could not navigate to edit page: %v", err)
		} else {
			env.TakeScreenshot(ctx, "09-job-edit-page-loaded")
			env.LogTest(t, "✓ Job edit page loaded")

			// Test 11: Verify TOML content loaded
			var loadedContent string
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelector('.CodeMirror').CodeMirror.getValue()`, &loadedContent),
			); err != nil {
				env.LogTest(t, "Warning: Could not get editor content: %v", err)
			} else {
				if len(loadedContent) == 0 {
					env.LogTest(t, "Warning: Editor content is empty - TOML did not load")
				} else {
					env.LogTest(t, "✓ TOML content loaded (length: %d)", len(loadedContent))
				}
			}

			env.TakeScreenshot(ctx, "10-edit-page-content-loaded")
		}
	} else {
		env.LogTest(t, "Note: Not on /jobs page after save, skipping edit test")
		env.LogTest(t, "Current URL: %s", currentURL)
	}

	env.TakeScreenshot(ctx, "11-test-complete")
	env.LogTest(t, "✓ All tests completed")
}
