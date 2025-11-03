package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestHomepageTitle(t *testing.T) {
	// Setup test environment with test name
	env, err := SetupTestEnvironment("HomepageTitle")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestHomepageTitle")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestHomepageTitle (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestHomepageTitle (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()
	var title string

	env.LogTest(t, "Navigating to homepage: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	env.LogTest(t, "Page loaded successfully, title: %s", title)

	// Take screenshot of homepage
	if err := env.TakeScreenshot(ctx, "homepage"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("homepage"))

	expectedTitle := "Quaero - Home"
	if title != expectedTitle {
		env.LogTest(t, "ERROR: Title mismatch - expected '%s', got '%s'", expectedTitle, title)
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	} else {
		env.LogTest(t, "✓ Title verified: %s", title)
	}
}

func TestHomepageElements(t *testing.T) {
	// Setup test environment with test name
	env, err := SetupTestEnvironment("HomepageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestHomepageElements")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestHomepageElements (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestHomepageElements (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Header", "header.app-header"},
		{"Navigation", "nav.app-header-nav"},
		{"Page title heading", "h1"},
		{"Service status card", ".card"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err := chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Evaluate(`document.querySelectorAll("`+tt.selector+`").length`, &nodeCount),
			)

			if err != nil {
				t.Fatalf("Failed to check element '%s': %v", tt.name, err)
			}

			if nodeCount == 0 {
				t.Errorf("Element '%s' (selector: %s) not found on page", tt.name, tt.selector)
			}
		})
	}

	// Take screenshot after checking all elements
	if err := env.TakeScreenshot(ctx, "homepage-elements"); err != nil {
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	t.Logf("Screenshot saved: %s", env.GetScreenshotPath("homepage-elements"))
}

func TestNavigation(t *testing.T) {
	// Setup test environment with test name
	env, err := SetupTestEnvironment("Navigation")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	t.Logf("Test environment ready, service running at: %s", env.GetBaseURL())
	t.Logf("Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	tests := []struct {
		linkText      string
		linkHref      string
		expectedTitle string
	}{
		{"Sources", "/sources", "Source Management"},
		{"Jobs", "/jobs", "Job Management"},
		{"Documents", "/documents", "Document Management"},
		{"Chat", "/chat", "Chat"},
		{"Settings", "/config", "Configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.linkText, func(t *testing.T) {
			var title string
			err := chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Click(`a[href="`+tt.linkHref+`"]`, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.Title(&title),
			)

			if err != nil {
				t.Fatalf("Failed to navigate to %s: %v", tt.linkText, err)
			}

			// Take screenshot of the navigated page
			screenshotName := fmt.Sprintf("navigation-%s", strings.ToLower(tt.linkText))
			if err := env.TakeScreenshot(ctx, screenshotName); err != nil {
				t.Fatalf("Failed to take screenshot for %s: %v", tt.linkText, err)
			}
			t.Logf("Screenshot saved: %s", env.GetScreenshotPath(screenshotName))

			if !strings.Contains(title, tt.expectedTitle) {
				t.Errorf("After clicking '%s', expected title to contain '%s', got '%s'",
					tt.linkText, tt.expectedTitle, title)
			}
		})
	}
}
