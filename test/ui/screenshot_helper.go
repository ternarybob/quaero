package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	testRunDir     string
	testRunDirOnce sync.Once
)

// getOrCreateTestRunDir returns the test run directory, creating it if necessary
// This ensures all screenshots from a single test run go to the same directory
func getOrCreateTestRunDir() (string, error) {
	var err error
	testRunDirOnce.Do(func() {
		// Check if TEST_RESULTS_DIR is set by runner
		if envDir := os.Getenv("TEST_RESULTS_DIR"); envDir != "" {
			testRunDir = envDir
			return
		}

		// Create timestamped directory in test/results/
		timestamp := time.Now().Format("run-2006-01-02-15-04-05")

		// Get path to test/results/ relative to where tests run
		// When running from test/ directory: ../test/results/
		// When running from project root: test/results/
		resultsBase := filepath.Join("..", "results")
		if _, err := os.Stat("results"); err == nil {
			// We're in test/ directory already
			resultsBase = "results"
		}

		testRunDir = filepath.Join(resultsBase, timestamp)
		err = os.MkdirAll(testRunDir, 0755)
	})

	if err != nil {
		return "", fmt.Errorf("failed to create test run directory: %w", err)
	}

	return testRunDir, nil
}

// TakeScreenshot captures a screenshot and saves it to test/results/{run-timestamp}/screenshots/
func TakeScreenshot(ctx context.Context, name string) error {
	// Get or create test run directory
	runDir, err := getOrCreateTestRunDir()
	if err != nil {
		return err
	}

	// Create screenshots subdirectory
	screenshotDir := filepath.Join(runDir, "screenshots")
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

// GetScreenshotsDir returns the screenshots directory path for the current test run
func GetScreenshotsDir() string {
	runDir, err := getOrCreateTestRunDir()
	if err != nil {
		// Fallback - should not happen if TakeScreenshot was called first
		return filepath.Join("..", "results", "screenshots")
	}
	return filepath.Join(runDir, "screenshots")
}
