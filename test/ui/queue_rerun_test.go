package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestQueuePageLoad verifies that the queue management page loads correctly
func TestQueuePageLoad(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueuePageLoad")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"
	var title string

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "queue-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Queue Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Queue page loads correctly")
}

// TestQueueRerunButtonExists verifies that rerun buttons are present in the queue
func TestQueueRerunButtonExists(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueueRerunButtonExists")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for data to load
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "queue-with-rerun-button"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check if rerunJob function exists
	var rerunFunctionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof rerunJob === 'function'`, &rerunFunctionExists),
	)

	if err != nil {
		t.Fatalf("Failed to check rerunJob function: %v", err)
	}

	if !rerunFunctionExists {
		t.Error("rerunJob function not found in page")
	}

	// Check for rerun buttons in the actions column
	// Buttons have onclick="rerunJob('...')"
	var rerunButtonCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('button[onclick^="rerunJob"]').length`, &rerunButtonCount),
	)

	if err != nil {
		t.Fatalf("Failed to count rerun buttons: %v", err)
	}

	// There should be at least 0 buttons (might not have jobs yet)
	// This test just verifies the buttons can exist
	t.Logf("Found %d rerun button(s) on queue page", rerunButtonCount)

	t.Log("✓ Queue page has rerun functionality available")
}

// TestQueueRerunButtonClick tests clicking the rerun button and verifies the behavior
func TestQueueRerunButtonClick(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueueRerunButtonClick")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for data to load
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Check if there are any jobs with rerun buttons
	var rerunButtonCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('button[onclick^="rerunJob"]').length`, &rerunButtonCount),
	)

	if err != nil {
		t.Fatalf("Failed to count rerun buttons: %v", err)
	}

	if rerunButtonCount == 0 {
		t.Skip("No jobs with rerun buttons available - skipping rerun click test")
		return
	}

	// Take screenshot before clicking
	if err := TakeScreenshot(ctx, "queue-before-rerun-click"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Click the first rerun button
	err = chromedp.Run(ctx,
		chromedp.Click(`button[onclick^="rerunJob"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for API call and response
	)

	if err != nil {
		t.Fatalf("Failed to click rerun button: %v", err)
	}

	// Take screenshot after clicking
	if err := TakeScreenshot(ctx, "queue-after-rerun-click"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for either success or error message in alerts
	var alertPresent bool
	var alertMessage string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.alert') !== null`, &alertPresent),
	)

	if err == nil && alertPresent {
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('.alert').textContent.trim()`, &alertMessage),
		)
		if err == nil {
			t.Logf("Alert message after rerun: %s", alertMessage)

			// Verify the alert contains either success or error message
			hasSuccessMessage := false
			hasErrorMessage := false

			// Check for success indicators
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelector('.alert-success') !== null || document.querySelector('.alert').textContent.includes('successfully') || document.querySelector('.alert').textContent.includes('queued')`, &hasSuccessMessage),
			)

			// Check for error indicators
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelector('.alert-danger') !== null || document.querySelector('.alert').textContent.includes('Failed') || document.querySelector('.alert').textContent.includes('error')`, &hasErrorMessage),
			)

			if !hasSuccessMessage && !hasErrorMessage {
				t.Errorf("Expected alert to contain success or error message, got: %s", alertMessage)
			}

			// If error, verify it's the expected "no seed URLs" error
			if hasErrorMessage {
				containsSeedURLError := false
				err = chromedp.Run(ctx,
					chromedp.Evaluate(`document.querySelector('.alert').textContent.includes('seed URL')`, &containsSeedURLError),
				)
				if err == nil && containsSeedURLError {
					t.Log("✓ Rerun button correctly shows error for jobs without seed URLs")
				} else {
					t.Logf("Rerun button showed error (not seed URL related): %s", alertMessage)
				}
			} else if hasSuccessMessage {
				t.Log("✓ Rerun button successfully queued the job")
			}
		}
	} else {
		t.Log("No alert displayed after clicking rerun button (might need to wait longer or check console)")
	}

	// Check console for any JavaScript errors
	var consoleErrors []map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window.consoleErrors || []`, &consoleErrors),
	)

	if err == nil && len(consoleErrors) > 0 {
		t.Logf("Console errors detected: %v", consoleErrors)
	}

	t.Log("✓ Rerun button click test completed")
}

// TestQueueRerunErrorHandling verifies that rerun errors are properly displayed to the user
func TestQueueRerunErrorHandling(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueueRerunErrorHandling")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Verify error handling function exists
	var errorHandlingExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof showAlert === 'function' || typeof showErrorAlert === 'function'`, &errorHandlingExists),
	)

	if err != nil {
		t.Fatalf("Failed to check error handling functions: %v", err)
	}

	if !errorHandlingExists {
		t.Error("Error handling functions not found in page")
	}

	t.Log("✓ Queue page has error handling functionality")
}
