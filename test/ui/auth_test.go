package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

func TestAuthPageLoad(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/auth"
	var title string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load auth page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "auth-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Quaero - Authentication Management"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Authentication page loads correctly")
}

func TestAuthPageElements(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/auth"

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Hero section", "section.hero"},
		{"Instructions card", ".card .card-header-title"},
		{"Authentication list", ".card"},
		{"Refresh button", `button[title="Refresh Authentications"]`},
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

func TestAuthNavbar(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/auth"

	var navbarVisible bool
	var menuItems []string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`nav.navbar`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('nav.navbar') !== null`, &navbarVisible),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('.navbar-item')).map(el => el.textContent.trim())`, &menuItems),
	)

	if err != nil {
		t.Fatalf("Failed to check navbar: %v", err)
	}

	if !navbarVisible {
		t.Error("Navbar not found on page")
	}

	// Check for AUTHENTICATION menu item
	expectedItems := []string{"HOME", "AUTHENTICATION", "SOURCES", "JOBS", "DOCUMENTS", "CHAT", "SETTINGS"}
	for _, expected := range expectedItems {
		found := false
		for _, item := range menuItems {
			if strings.Contains(item, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Menu item '%s' not found in navbar", expected)
		}
	}

	// Verify AUTHENTICATION item is active on auth page
	var authActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.navbar-item.is-active[href="/auth"]') !== null`, &authActive),
	)
	if err != nil {
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !authActive {
		t.Error("AUTHENTICATION menu item should be active on auth page")
	}

	t.Log("✓ Navbar displays correctly with AUTHENTICATION item")
}

func TestAuthInstructions(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/auth"

	var instructionsText string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`.card-content ol`, chromedp.ByQuery),
		chromedp.Text(`.card-content ol`, &instructionsText, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to read instructions: %v", err)
	}

	// Check that instructions mention Chrome extension
	if !strings.Contains(instructionsText, "Chrome extension") {
		t.Error("Instructions should mention Chrome extension")
	}

	// Check that instructions mention Atlassian
	if !strings.Contains(instructionsText, "Atlassian") {
		t.Error("Instructions should mention Atlassian")
	}

	t.Log("✓ Authentication instructions display correctly")
}