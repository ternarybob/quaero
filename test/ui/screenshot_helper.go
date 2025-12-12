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

// FullScreenshotQuality is the JPEG quality for full page screenshots (0-100)
const FullScreenshotQuality = 90

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

// TakeScreenshot captures a viewport screenshot and saves it to test/results/{run-timestamp}/screenshots/
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

	return TakeScreenshotToPath(ctx, filename)
}

// TakeFullScreenshot captures a full page screenshot (including content below fold)
// and saves it to test/results/{run-timestamp}/screenshots/
func TakeFullScreenshot(ctx context.Context, name string) error {
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

	return TakeFullScreenshotToPath(ctx, filename)
}

// TakeScreenshotToPath captures a viewport screenshot and saves it to the specified path
func TakeScreenshotToPath(ctx context.Context, path string) error {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		return fmt.Errorf("failed to capture screenshot: %w", err)
	}

	if err := os.WriteFile(path, buf, 0644); err != nil {
		return fmt.Errorf("failed to save screenshot to %s: %w", path, err)
	}

	return nil
}

// TakeFullScreenshotToPath captures a full page screenshot (including content below fold)
// and saves it to the specified path. This captures the entire scrollable page content.
func TakeFullScreenshotToPath(ctx context.Context, path string) error {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, FullScreenshotQuality)); err != nil {
		return fmt.Errorf("failed to capture full screenshot: %w", err)
	}

	if err := os.WriteFile(path, buf, 0644); err != nil {
		return fmt.Errorf("failed to save full screenshot to %s: %w", path, err)
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

// ============================================================================
// Functions for use with TestEnvironment (pass resultsDir from env.ResultsDir)
// ============================================================================

// GetScreenshotPath returns the path for a screenshot in the given results directory
func GetScreenshotPath(resultsDir, name string) string {
	return filepath.Join(resultsDir, fmt.Sprintf("%s.png", name))
}

// TakeScreenshotInDir captures a viewport screenshot and saves it to the specified results directory
func TakeScreenshotInDir(ctx context.Context, resultsDir, name string) error {
	path := GetScreenshotPath(resultsDir, name)
	return TakeScreenshotToPath(ctx, path)
}

// TakeFullScreenshotInDir captures a full page screenshot (including content below fold)
// and saves it to the specified results directory
func TakeFullScreenshotInDir(ctx context.Context, resultsDir, name string) error {
	path := GetScreenshotPath(resultsDir, name)
	return TakeFullScreenshotToPath(ctx, path)
}

// TakeBeforeAfterScreenshots captures before/after screenshots around an action.
// Takes a "before" screenshot, executes the action function, then takes an "after" screenshot.
// If the action fails, takes an "after_error" full screenshot to capture the failure state.
func TakeBeforeAfterScreenshots(ctx context.Context, resultsDir, baseName string, action func() error) error {
	// Take "before" screenshot
	beforeName := fmt.Sprintf("%s_before", baseName)
	if err := TakeScreenshotInDir(ctx, resultsDir, beforeName); err != nil {
		return fmt.Errorf("failed to take before screenshot: %w", err)
	}

	// Execute the action
	if err := action(); err != nil {
		// Still take "after" screenshot even if action fails
		afterName := fmt.Sprintf("%s_after_error", baseName)
		TakeFullScreenshotInDir(ctx, resultsDir, afterName)
		return err
	}

	// Take "after" screenshot
	afterName := fmt.Sprintf("%s_after", baseName)
	if err := TakeScreenshotInDir(ctx, resultsDir, afterName); err != nil {
		return fmt.Errorf("failed to take after screenshot: %w", err)
	}

	return nil
}
