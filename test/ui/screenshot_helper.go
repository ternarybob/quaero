package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// TakeScreenshot captures a screenshot and saves it to the test results directory
func TakeScreenshot(ctx context.Context, name string) error {
	// Get results directory from environment (set by test runner)
	resultsDir := os.Getenv("TEST_RESULTS_DIR")
	if resultsDir == "" {
		// Fallback to creating a screenshots dir in current location
		resultsDir = "screenshots"
	}

	// Create screenshots subdirectory
	screenshotDir := filepath.Join(resultsDir, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create screenshots directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(screenshotDir, fmt.Sprintf("%s-%s.png", name, timestamp))

	// Capture screenshot
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Save to file
	if err := os.WriteFile(filename, buf, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot: %w", err)
	}

	return nil
}
