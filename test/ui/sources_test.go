package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

func TestSourcesPageLoad(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"
	var title string

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		t.Fatalf("Failed to load sources page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "sources-page"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	expectedTitle := "Source Management - Quaero"
	if title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	}

	t.Log("✓ Sources page loads correctly")
}

func TestSourcesPageElements(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Hero section", "section.hero"},
		{"Sources list card", ".card"},
		{"Add Source button", "button.is-small.is-info"},
		{"Sources table", "table.table"},
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

func TestSourcesNavbar(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

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

	// Check for SOURCES menu item
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

	// Verify SOURCES item is active on sources page
	var sourcesActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.navbar-item.is-active[href="/sources"]') !== null`, &sourcesActive),
	)
	if err != nil {
		t.Fatalf("Failed to check active menu item: %v", err)
	}
	if !sourcesActive {
		t.Error("SOURCES menu item should be active on sources page")
	}

	t.Log("✓ Navbar displays correctly with SOURCES item")
}

func TestSourcesModalWithAuthentication(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Click Add Source button to open modal
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`button.is-small.is-info`, chromedp.ByQuery),
		chromedp.Click(`button.is-small.is-info`, chromedp.ByQuery),
		chromedp.WaitVisible(`.modal.is-active`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to open source modal: %v", err)
	}

	// Take screenshot of modal
	if err := TakeScreenshot(ctx, "sources-modal-auth"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check for authentication dropdown
	var authSelectPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('select[x-model="currentSource.auth_id"]') !== null`, &authSelectPresent),
	)

	if err != nil {
		t.Fatalf("Failed to check auth dropdown: %v", err)
	}

	if !authSelectPresent {
		t.Error("Authentication dropdown not found in modal")
	}

	// Check for authentication help text
	var helpTextPresent bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.help') !== null &&
			document.querySelector('.help').textContent.includes('Select authentication')`, &helpTextPresent),
	)

	if err == nil && !helpTextPresent {
		t.Error("Authentication help text not found")
	}

	t.Log("✓ Source modal includes authentication dropdown")
}

func TestSourcesTableWithAuthColumn(t *testing.T) {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := test.MustGetTestServerURL() + "/sources"

	// Check if the table has authentication column
	var hasAuthColumn bool
	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`table.table`, chromedp.ByQuery),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('th')).some(th => th.textContent.includes('AUTHENTICATION'))`, &hasAuthColumn),
	)

	if err != nil {
		t.Fatalf("Failed to check table headers: %v", err)
	}

	if !hasAuthColumn {
		t.Error("Authentication column not found in sources table")
	}

	// Check column count (should be 7 with authentication)
	var columnCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('thead tr th').length`, &columnCount),
	)

	if err == nil && columnCount != 7 {
		t.Errorf("Expected 7 columns in table, got %d", columnCount)
	}

	t.Log("✓ Sources table includes authentication column")
}