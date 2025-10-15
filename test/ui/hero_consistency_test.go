package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test"
)

// TestHeroSectionConsistency verifies that navbar, hero sections, and footer
// are in the same location across all pages
func TestHeroSectionConsistency(t *testing.T) {
	serverURL := test.MustGetTestServerURL()

	// Create browser context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Navigate to home page first
	if err := chromedp.Run(ctx, chromedp.Navigate(serverURL)); err != nil {
		t.Fatalf("Failed to navigate to home page: %v", err)
	}

	// Wait for page to load
	time.Sleep(2 * time.Second)

	// Define pages to test with their menu selectors
	pages := []struct {
		name         string
		menuSelector string
		url          string
	}{
		{"Home", `a[href="/"]`, "/"},
		{"Sources", `a[href="/sources"]`, "/sources"},
		{"Jobs", `a[href="/jobs"]`, "/jobs"},
		{"Documents", `a[href="/documents"]`, "/documents"},
		{"Chat", `a[href="/chat"]`, "/chat"},
		{"Auth", `a[href="/auth"]`, "/auth"},
		{"Config", `a[href="/config"]`, "/config"},
		{"Settings", `a[href="/settings"]`, "/settings"},
	}

	// Store reference measurements from first page (Jobs - the correct one)
	var referenceNavbarX, referenceNavbarWidth float64
	var referenceHeroX, referenceHeroWidth float64
	var referenceFooterX, referenceFooterWidth float64

	for i, page := range pages {
		t.Run(page.name, func(t *testing.T) {
			// Navigate to page with consistent viewport
			if err := chromedp.Run(ctx,
				chromedp.EmulateViewport(1920, 1080),
				chromedp.Navigate(serverURL+page.url),
				chromedp.WaitReady("body"),
			); err != nil {
				t.Fatalf("Failed to navigate to %s: %v", page.name, err)
			}

			// Wait for page to render
			time.Sleep(1 * time.Second)

			// Take screenshot - CRITICAL for visual verification
			screenshotName := fmt.Sprintf("hero-consistency-%s", page.name)
			if err := TakeScreenshot(ctx, screenshotName); err != nil {
				t.Errorf("%s: Failed to take screenshot: %v", page.name, err)
			} else {
				t.Logf("%s: Screenshot saved successfully", page.name)
			}

			// Get navbar measurements
			var navbarMeasurements map[string]interface{}
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`(() => {
					const navbar = document.querySelector('nav.navbar');
					if (!navbar) return { x: 0, width: 0 };
					const rect = navbar.getBoundingClientRect();
					return { x: rect.x, width: rect.width };
				})()`, &navbarMeasurements),
			)
			if err != nil {
				t.Errorf("%s: Failed to get navbar measurements: %v", page.name, err)
				return
			}
			navbarX := navbarMeasurements["x"].(float64)
			navbarWidth := navbarMeasurements["width"].(float64)

			// Get hero section measurements
			var heroMeasurements map[string]interface{}
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`(() => {
					const hero = document.querySelector('section.hero');
					if (!hero) return { x: 0, width: 0 };
					const rect = hero.getBoundingClientRect();
					return { x: rect.x, width: rect.width };
				})()`, &heroMeasurements),
			)
			if err != nil {
				t.Errorf("%s: Failed to get hero measurements: %v", page.name, err)
				return
			}
			heroX := heroMeasurements["x"].(float64)
			heroWidth := heroMeasurements["width"].(float64)

			// Get footer measurements
			var footerMeasurements map[string]interface{}
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`(() => {
					const footer = document.querySelector('footer.footer');
					if (!footer) return { x: 0, width: 0 };
					const rect = footer.getBoundingClientRect();
					return { x: rect.x, width: rect.width };
				})()`, &footerMeasurements),
			)
			if err != nil {
				t.Errorf("%s: Failed to get footer measurements: %v", page.name, err)
				return
			}
			footerX := footerMeasurements["x"].(float64)
			footerWidth := footerMeasurements["width"].(float64)

			// Log measurements
			t.Logf("%s measurements:", page.name)
			t.Logf("  Navbar: x=%.2f, width=%.2f", navbarX, navbarWidth)
			t.Logf("  Hero:   x=%.2f, width=%.2f", heroX, heroWidth)
			t.Logf("  Footer: x=%.2f, width=%.2f", footerX, footerWidth)

			// Set reference measurements from Jobs page (index 2)
			if page.name == "Jobs" {
				referenceNavbarX = navbarX
				referenceNavbarWidth = navbarWidth
				referenceHeroX = heroX
				referenceHeroWidth = heroWidth
				referenceFooterX = footerX
				referenceFooterWidth = footerWidth
				t.Logf("Set reference measurements from Jobs page (Hero: x=%.2f, width=%.2f)", heroX, heroWidth)
				return
			}

			// Skip comparison if this is before Jobs page
			if i < 2 {
				return
			}

			// Compare with reference measurements (allow 1px tolerance for rounding)
			tolerance := 1.0

			// Check navbar position
			if abs(navbarX-referenceNavbarX) > tolerance {
				t.Errorf("%s: Navbar X position mismatch. Expected: %.2f, Got: %.2f (diff: %.2f)",
					page.name, referenceNavbarX, navbarX, abs(navbarX-referenceNavbarX))
			}
			if abs(navbarWidth-referenceNavbarWidth) > tolerance {
				t.Errorf("%s: Navbar width mismatch. Expected: %.2f, Got: %.2f (diff: %.2f)",
					page.name, referenceNavbarWidth, navbarWidth, abs(navbarWidth-referenceNavbarWidth))
			}

			// Check hero position - THIS IS THE CRITICAL CHECK
			if abs(heroX-referenceHeroX) > tolerance {
				t.Errorf("%s: Hero X position mismatch. Expected: %.2f, Got: %.2f (diff: %.2f) - Hero should be full-width edge-to-edge",
					page.name, referenceHeroX, heroX, abs(heroX-referenceHeroX))
			}
			if abs(heroWidth-referenceHeroWidth) > tolerance {
				t.Errorf("%s: Hero width mismatch. Expected: %.2f, Got: %.2f (diff: %.2f) - Hero should span entire viewport width",
					page.name, referenceHeroWidth, heroWidth, abs(heroWidth-referenceHeroWidth))
			}

			// Check footer position
			if abs(footerX-referenceFooterX) > tolerance {
				t.Errorf("%s: Footer X position mismatch. Expected: %.2f, Got: %.2f (diff: %.2f)",
					page.name, referenceFooterX, footerX, abs(footerX-referenceFooterX))
			}
			if abs(footerWidth-referenceFooterWidth) > tolerance {
				t.Errorf("%s: Footer width mismatch. Expected: %.2f, Got: %.2f (diff: %.2f)",
					page.name, referenceFooterWidth, footerWidth, abs(footerWidth-referenceFooterWidth))
			}

			// Additional check: Hero should start at x=0 (edge-to-edge)
			if heroX != 0 {
				t.Errorf("%s: Hero section does not start at left edge (x=%.2f, should be 0)", page.name, heroX)
			}

			// Hero width should equal window width
			var windowWidth float64
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(`window.innerWidth`, &windowWidth),
			); err == nil {
				if abs(heroWidth-windowWidth) > tolerance {
					t.Errorf("%s: Hero width (%.2f) does not match window width (%.2f)",
						page.name, heroWidth, windowWidth)
				}
			}
		})
	}

	// Verify all screenshots were created
	t.Run("VerifyScreenshots", func(t *testing.T) {
		screenshotDir := GetScreenshotsDir()

		// Check if directory exists
		if _, err := os.Stat(screenshotDir); os.IsNotExist(err) {
			t.Errorf("Screenshots directory does not exist: %s", screenshotDir)
			return
		}

		// Read directory contents
		files, err := os.ReadDir(screenshotDir)
		if err != nil {
			t.Errorf("Failed to read screenshots directory: %v", err)
			return
		}

		// Find screenshots for each page from this test run
		pageScreenshots := make(map[string]bool)
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "hero-consistency-") && strings.HasSuffix(file.Name(), ".png") {
				// Extract page name from filename (e.g., "hero-consistency-Home-2025-10-15_13-20-01.png")
				parts := strings.Split(file.Name(), "-")
				if len(parts) >= 3 {
					pageName := parts[2] // e.g., "Home"
					pageScreenshots[pageName] = true
				}
			}
		}

		// Verify we have screenshots for all pages
		expectedCount := len(pages)
		actualCount := len(pageScreenshots)

		t.Logf("Screenshots found for pages:")
		for page := range pageScreenshots {
			t.Logf("  ✓ %s", page)
		}

		if actualCount < expectedCount {
			// Find missing pages
			missing := []string{}
			for _, page := range pages {
				if !pageScreenshots[page.name] {
					missing = append(missing, page.name)
				}
			}
			t.Errorf("Missing screenshots for %d pages: %v", len(missing), missing)
		} else {
			t.Logf("✓ All %d page screenshots verified in %s", expectedCount, screenshotDir)
		}
	})
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
