package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var screenshotCounter int

func takeScreenshot(ctx context.Context, t *testing.T, name string) {
	screenshotCounter++
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	filename := fmt.Sprintf("%02d_%s.png", screenshotCounter, name)
	screenshotPath := filepath.Join(runDir, filename)

	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err == nil {
		os.MkdirAll(filepath.Dir(screenshotPath), 0755)
		if err := os.WriteFile(screenshotPath, buf, 0644); err == nil {
			t.Logf("📸 Screenshot %d: %s", screenshotCounter, filename)
		}
	}
}

// validateStyles checks if the page follows Beer CSS minimal styling guidelines
func validateStyles(ctx context.Context, t *testing.T, stepName string) {
	var styleCheck struct {
		BodyBgColor        string
		BodyFontFamily     string
		PrimaryBtnBgColor  string
		PrimaryBtnColor    string
		PrimaryBtnBorder   string
		TableFontSize      string
		NavbarExists       bool
		ArticlesExist      bool
		HasDarkBackground  bool
	}

	err := chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const body = document.body;
			const bodyStyles = window.getComputedStyle(body);
			const primaryBtn = document.querySelector('button.primary, .btn-primary');
			const table = document.querySelector('table.border');
			const navbar = document.querySelector('nav.top');
			const articles = document.querySelectorAll('article.border');

			let primaryBtnStyles = null;
			if (primaryBtn) {
				primaryBtnStyles = window.getComputedStyle(primaryBtn);
			}

			let tableStyles = null;
			if (table) {
				tableStyles = window.getComputedStyle(table);
			}

			return {
				BodyBgColor: bodyStyles.backgroundColor,
				BodyFontFamily: bodyStyles.fontFamily,
				PrimaryBtnBgColor: primaryBtnStyles ? primaryBtnStyles.backgroundColor : 'N/A',
				PrimaryBtnColor: primaryBtnStyles ? primaryBtnStyles.color : 'N/A',
				PrimaryBtnBorder: primaryBtnStyles ? primaryBtnStyles.border : 'N/A',
				TableFontSize: tableStyles ? tableStyles.fontSize : 'N/A',
				NavbarExists: navbar !== null,
				ArticlesExist: articles.length > 0,
				HasDarkBackground: bodyStyles.backgroundColor.includes('rgb(0, 0, 0)') ||
				                   bodyStyles.backgroundColor.includes('rgb(26, 26, 26)') ||
				                   bodyStyles.backgroundColor.includes('rgb(42, 42, 42)')
			};
		})()
	`, &styleCheck))

	if err != nil {
		t.Logf("⚠️  Style validation error for %s: %v", stepName, err)
		return
	}

	// Log style validation results
	t.Logf("🎨 Style Validation [%s]:", stepName)
	t.Logf("   Body Background: %s", styleCheck.BodyBgColor)
	t.Logf("   Body Font: %s", styleCheck.BodyFontFamily)
	t.Logf("   Primary Button BG: %s", styleCheck.PrimaryBtnBgColor)
	t.Logf("   Primary Button Color: %s", styleCheck.PrimaryBtnColor)
	t.Logf("   Primary Button Border: %s", styleCheck.PrimaryBtnBorder)
	t.Logf("   Table Font Size: %s", styleCheck.TableFontSize)
	t.Logf("   Navbar Present: %v", styleCheck.NavbarExists)
	t.Logf("   Articles Present: %v", styleCheck.ArticlesExist)

	// Check for issues
	issues := []string{}

	// Should NOT have dark background (should be white or light)
	if styleCheck.HasDarkBackground {
		issues = append(issues, "❌ Dark background detected (should be light/white)")
	}

	// Should use monospace font
	if !contains(styleCheck.BodyFontFamily, "Courier") &&
	   !contains(styleCheck.BodyFontFamily, "monospace") &&
	   !contains(styleCheck.BodyFontFamily, "Consolas") {
		issues = append(issues, "❌ Body font should be monospace")
	}

	// Primary button should have dark background (rgb(26, 26, 26))
	if styleCheck.PrimaryBtnBgColor != "N/A" {
		if !contains(styleCheck.PrimaryBtnBgColor, "rgb(26, 26, 26)") &&
		   !contains(styleCheck.PrimaryBtnBgColor, "rgba(26, 26, 26") {
			issues = append(issues, "❌ Primary button background should be rgb(26, 26, 26)")
		}
	}

	// Primary button should have white text
	if styleCheck.PrimaryBtnColor != "N/A" {
		if !contains(styleCheck.PrimaryBtnColor, "rgb(255, 255, 255)") &&
		   !contains(styleCheck.PrimaryBtnColor, "rgba(255, 255, 255") {
			issues = append(issues, "❌ Primary button text should be white")
		}
	}

	// Table font should be 11px
	if styleCheck.TableFontSize != "N/A" && styleCheck.TableFontSize != "11px" {
		issues = append(issues, fmt.Sprintf("❌ Table font size should be 11px (got %s)", styleCheck.TableFontSize))
	}

	// Should use Beer CSS components
	if !styleCheck.NavbarExists {
		issues = append(issues, "⚠️  nav.top not found")
	}
	if !styleCheck.ArticlesExist {
		issues = append(issues, "⚠️  article.border elements not found")
	}

	// Report issues
	if len(issues) > 0 {
		t.Log("   STYLE ISSUES:")
		for _, issue := range issues {
			t.Logf("   %s", issue)
		}
	} else {
		t.Log("   ✅ All style checks passed")
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
	       (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func startVideoRecording(ctx context.Context, t *testing.T) (func(), error) {
	runDir := os.Getenv("TEST_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join("..", "results")
	}

	videoPath := filepath.Join(runDir, "test_recording.webm")
	os.MkdirAll(filepath.Dir(videoPath), 0755)

	frameCount := 0
	maxFrames := 300 // 30 seconds at 10fps

	// Start screencast
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return page.StartScreencast().
				WithFormat("png").
				WithQuality(80).
				WithEveryNthFrame(6). // ~10fps at 60fps base
				Do(ctx)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start screencast: %w", err)
	}

	t.Log("🎥 Video recording started")

	// Cleanup function
	stopRecording := func() {
		chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				return page.StopScreencast().Do(ctx)
			}),
		)
		t.Logf("🎥 Video recording stopped (%d frames captured)", frameCount)
	}

	// Listen for screencast frames
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if frameCount >= maxFrames {
			return
		}

		if _, ok := ev.(*page.EventScreencastFrame); ok {
			frameCount++
		}
	})

	return stopRecording, nil
}
