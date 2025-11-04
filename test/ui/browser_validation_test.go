// -----------------------------------------------------------------------
// Last Modified: Tuesday, 4th November 2025 4:39:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestBrowserAutomation validates that ChromeDP/browser automation is working
// by testing against a simple local server on port 3333
func TestBrowserAutomation(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := "http://localhost:3333"
	var title string
	var message string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Text(`#test-message`, &message, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Browser automation failed: %v", err)
	}

	expectedTitle := "Test Server - Working"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	expectedMessage := "If you can see this, browser automation is working!"
	if message != expectedMessage {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, message)
	}

	t.Log("✓ Browser automation is working correctly")
}

// TestBrowserInteraction validates that browser interaction (clicks, etc.) works
func TestBrowserInteraction(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := "http://localhost:3333"
	var outputText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#test-button`, chromedp.ByQuery),
		chromedp.Click(`#test-button`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Text(`#test-output`, &outputText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Browser interaction failed: %v", err)
	}

	expectedText := "Button clicked!"
	if outputText != expectedText {
		t.Errorf("Expected output '%s', got '%s'", expectedText, outputText)
	}

	t.Log("✓ Browser interaction is working correctly")
}
