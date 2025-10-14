package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

func TestHomepageTitle(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL()
	var title string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load homepage: %v", err)
	}

	// Take screenshot of homepage
	if err := TakeScreenshot(ctx, "homepage"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Quaero - Home"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}
}

func TestHomepageElements(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL()

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Navbar", "nav.navbar"},
		{"Hero section", "section.hero"},
		{"Title", ".title"},
		{"Quick Actions card", ".card"},
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
}

func TestNavigation(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL()

	tests := []struct {
		linkText      string
		linkHref      string
		expectedTitle string
	}{
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
			if err := TakeScreenshot(ctx, screenshotName); err != nil {
				t.Logf("Warning: Failed to take screenshot for %s: %v", tt.linkText, err)
			}

			if !strings.Contains(title, tt.expectedTitle) {
				t.Errorf("After clicking '%s', expected title to contain '%s', got '%s'",
					tt.linkText, tt.expectedTitle, title)
			}
		})
	}
}
