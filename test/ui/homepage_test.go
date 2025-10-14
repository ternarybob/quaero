package ui

import (
	"context"
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

	url := test.GetTestServerURL()
	var title string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load homepage: %v", err)
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

	url := test.GetTestServerURL()

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Navbar", "nav.navbar"},
		{"Hero section", "section.hero"},
		{"Title", ".title"},
		{"Application Status card", ".card"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err := chromedp.Run(ctx,
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

	url := test.GetTestServerURL()

	tests := []struct {
		linkText      string
		expectedTitle string
	}{
		{"Sources", "Sources"},
		{"Jobs", "Jobs"},
		{"Documents", "Documents"},
		{"Chat", "Chat"},
	}

	for _, tt := range tests {
		t.Run(tt.linkText, func(t *testing.T) {
			var title string
			err := chromedp.Run(ctx,
				chromedp.Navigate(url),
				chromedp.WaitVisible(`body`, chromedp.ByQuery),
				chromedp.Click(`a[href="/`+strings.ToLower(tt.linkText)+`"]`, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.Title(&title),
			)

			if err != nil {
				t.Fatalf("Failed to navigate to %s: %v", tt.linkText, err)
			}

			if !strings.Contains(title, tt.expectedTitle) {
				t.Errorf("After clicking '%s', expected title to contain '%s', got '%s'",
					tt.linkText, tt.expectedTitle, title)
			}
		})
	}
}

func TestApplicationStatus(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.GetTestServerURL()

	var statusText string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.card-header-title`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for Alpine.js to hydrate
		chromedp.Text(`.card .content p:first-child`, &statusText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to read application status: %v", err)
	}

	if statusText == "" {
		t.Error("Application status text is empty")
	}

	t.Logf("Application status: %s", statusText)
}
