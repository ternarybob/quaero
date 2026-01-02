// -----------------------------------------------------------------------
// Test helpers for both API and UI tests
// Shared across test/api and test/ui packages
// -----------------------------------------------------------------------

package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/internal/common"
)

// =============================================================================
// Test Server URL Helpers
// =============================================================================

// GetTestServerURL returns the test server URL from environment variable or bin/quaero.toml
func GetTestServerURL() (string, error) {
	// Check if running in mock mode
	if IsMockMode() {
		return "http://localhost:9999", nil
	}

	// Check environment variable first (highest priority)
	if url := os.Getenv("TEST_SERVER_URL"); url != "" {
		return url, nil
	}

	// Read from bin/quaero.toml
	configPath := filepath.Join("..", "bin", "quaero.toml")

	// Try to read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config file doesn't exist, use default
		return "http://localhost:8085", nil
	}

	var config common.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		// If config is invalid, use default
		return "http://localhost:8085", nil
	}

	// Construct URL from config
	host := config.Server.Host
	if host == "" {
		host = "localhost"
	}
	port := config.Server.Port
	if port == 0 {
		port = 8085
	}

	return fmt.Sprintf("http://%s:%d", host, port), nil
}

// MustGetTestServerURL returns the test server URL or panics on error
func MustGetTestServerURL() string {
	url, err := GetTestServerURL()
	if err != nil {
		panic(fmt.Sprintf("Failed to get test server URL: %v", err))
	}
	return url
}

// GetExpectedPort returns the expected port from config or default
func GetExpectedPort() int {
	// Check environment variable first
	if url := os.Getenv("TEST_SERVER_URL"); url != "" {
		// Extract port from URL
		parts := strings.Split(url, ":")
		if len(parts) >= 3 {
			portStr := parts[2]
			if port, err := strconv.Atoi(portStr); err == nil {
				return port
			}
		}
	}

	// Read from bin/quaero.toml
	configPath := filepath.Join("..", "bin", "quaero.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 8085 // Default
	}

	var config common.Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return 8085 // Default
	}

	if config.Server.Port == 0 {
		return 8085
	}

	return config.Server.Port
}

// GetTestMode returns the test mode from environment variable
// Returns "mock" or "integration" (default: "integration")
// - mock: Uses in-memory mock server, no real database, fast, isolated
// - integration: Uses real Quaero service, tests full stack, requires service running
func GetTestMode() string {
	mode := os.Getenv("TEST_MODE")
	if mode == "" {
		return "integration" // Default for backward compatibility
	}
	return mode
}

// IsMockMode returns true if tests should run in mock mode
func IsMockMode() bool {
	return GetTestMode() == "mock"
}

// =============================================================================
// File Assertion Helpers
// =============================================================================

// AssertFileExistsAndNotEmpty asserts that a file exists and has content
func AssertFileExistsAndNotEmpty(t *testing.T, path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return false
	}

	if info.Size() == 0 {
		t.Errorf("File is empty: %s", path)
		return false
	}

	t.Logf("PASS: File exists and is not empty: %s (%d bytes)", path, info.Size())
	return true
}

// AssertFileExists asserts that a file exists (can be empty)
func AssertFileExists(t *testing.T, path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return false
	}
	return true
}

// RequireFileExistsAndNotEmpty requires that a file exists and has content, failing immediately if not
func RequireFileExistsAndNotEmpty(t *testing.T, path string) {
	info, err := os.Stat(path)
	require.NoError(t, err, "File must exist: %s", path)
	require.Greater(t, info.Size(), int64(0), "File must not be empty: %s", path)
	t.Logf("PASS: File exists and is not empty: %s (%d bytes)", path, info.Size())
}

// =============================================================================
// Retry Helpers
// =============================================================================

// Retry retries a function until it succeeds or max attempts reached
func Retry(fn func() error, maxAttempts int, delay int) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("retry failed after %d attempts: %w", maxAttempts, lastErr)
}

// =============================================================================
// Test Timing Helpers
// =============================================================================

// TestTimingData captures timing information for a test run
type TestTimingData struct {
	TestName      string         `json:"test_name"`
	StartTime     string         `json:"start_time"`
	EndTime       string         `json:"end_time"`
	TotalDuration string         `json:"total_duration_formatted"`
	TotalSeconds  float64        `json:"total_duration_seconds"`
	WorkerTimings []WorkerTiming `json:"worker_timings,omitempty"`
	StepTimings   []StepTiming   `json:"step_timings,omitempty"`
}

// WorkerTiming captures timing for a single worker/child job
type WorkerTiming struct {
	Name              string  `json:"name"`
	WorkerType        string  `json:"worker_type"`
	DurationFormatted string  `json:"duration_formatted"`
	DurationSeconds   float64 `json:"duration_seconds"`
	Status            string  `json:"status"`
	JobID             string  `json:"job_id"`
}

// StepTiming captures timing for a test step (e.g., API call, job execution)
type StepTiming struct {
	StepName          string  `json:"step_name"`
	DurationFormatted string  `json:"duration_formatted"`
	DurationSeconds   float64 `json:"duration_seconds"`
}

// NewTestTimingData creates a new TestTimingData with start time set
func NewTestTimingData(testName string) *TestTimingData {
	return &TestTimingData{
		TestName:      testName,
		StartTime:     time.Now().Format(time.RFC3339),
		WorkerTimings: []WorkerTiming{},
		StepTimings:   []StepTiming{},
	}
}

// Complete marks the test as complete and calculates total duration
func (t *TestTimingData) Complete() {
	t.EndTime = time.Now().Format(time.RFC3339)

	// Calculate duration
	startTime, _ := time.Parse(time.RFC3339, t.StartTime)
	endTime, _ := time.Parse(time.RFC3339, t.EndTime)
	duration := endTime.Sub(startTime)

	t.TotalSeconds = duration.Seconds()
	t.TotalDuration = FormatDuration(duration)
}

// AddWorkerTiming adds a worker timing entry
func (t *TestTimingData) AddWorkerTiming(name, workerType string, durationSeconds float64, status, jobID string) {
	duration := time.Duration(durationSeconds * float64(time.Second))
	t.WorkerTimings = append(t.WorkerTimings, WorkerTiming{
		Name:              name,
		WorkerType:        workerType,
		DurationFormatted: FormatDuration(duration),
		DurationSeconds:   durationSeconds,
		Status:            status,
		JobID:             jobID,
	})
}

// AddStepTiming adds a step timing entry
func (t *TestTimingData) AddStepTiming(stepName string, durationSeconds float64) {
	duration := time.Duration(durationSeconds * float64(time.Second))
	t.StepTimings = append(t.StepTimings, StepTiming{
		StepName:          stepName,
		DurationFormatted: FormatDuration(duration),
		DurationSeconds:   durationSeconds,
	})
}

// FormatDuration formats a duration for display (e.g., "2m15s", "45.3s")
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// SaveTimingData saves timing data to timing.json in the specified directory
func SaveTimingData(t *testing.T, resultsDir string, timing *TestTimingData) error {
	if resultsDir == "" || timing == nil {
		return nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Logf("Warning: Failed to create results directory: %v", err)
		return err
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(timing, "", "  ")
	if err != nil {
		t.Logf("Warning: Failed to marshal timing data: %v", err)
		return err
	}

	// Write to file
	timingPath := filepath.Join(resultsDir, "timing.json")
	if err := os.WriteFile(timingPath, data, 0644); err != nil {
		t.Logf("Warning: Failed to write timing.json: %v", err)
		return err
	}

	t.Logf("Saved timing data to: %s", timingPath)
	return nil
}

// =============================================================================
// Test Results Directory Helpers
// =============================================================================

// GetTestResultsDir returns a results directory path that includes test identification.
// Format: test/results/api/{prefix}-{timestamp}-{sanitized-test-name}/
// Example: test/results/api/orchestrator-20260102-150405-StockAnalysisGoal-SingleStock/
func GetTestResultsDir(prefix, testName string) string {
	timestamp := time.Now().Format("20060102-150405")

	// Sanitize test name: remove "Test" prefix, replace / with -, remove special chars
	sanitized := testName
	if strings.HasPrefix(sanitized, "Test") {
		sanitized = sanitized[4:]
	}
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")

	// Limit length to avoid filesystem issues
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	dirName := fmt.Sprintf("%s-%s-%s", prefix, timestamp, sanitized)
	return filepath.Join("..", "results", "api", dirName)
}

// EnsureResultsDir creates the results directory if it doesn't exist
func EnsureResultsDir(t *testing.T, resultsDir string) error {
	if resultsDir == "" {
		return nil
	}

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Logf("Warning: Failed to create results directory %s: %v", resultsDir, err)
		return err
	}
	return nil
}

// =============================================================================
// TDD Integration Helpers
// =============================================================================

// CopyTDDSummary checks for TDD workdir and copies summary.md to results if found.
// TDD workdirs are in format: .claude/workdir/DATE-TIME-tdd-TASK/
// Copies to: {resultsDir}/tdd-summary.md
func CopyTDDSummary(t *testing.T, resultsDir string) error {
	if resultsDir == "" {
		return nil
	}

	// Look for TDD workdir
	workdirBase := filepath.Join("..", "..", ".claude", "workdir")

	// Check if workdir exists
	entries, err := os.ReadDir(workdirBase)
	if err != nil {
		// Workdir doesn't exist, that's fine
		return nil
	}

	// Find most recent TDD directory (contains "-tdd-" in name)
	var latestTDDDir string
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.Contains(name, "-tdd-") && !strings.Contains(name, "-3agents-") {
			continue
		}

		// Parse date from directory name (format: YYYY-MM-DD-HHMM-...)
		parts := strings.Split(name, "-")
		if len(parts) < 4 {
			continue
		}

		dateStr := fmt.Sprintf("%s-%s-%s %s", parts[0], parts[1], parts[2], parts[3])
		parsedTime, err := time.Parse("2006-01-02 1504", dateStr)
		if err != nil {
			continue
		}

		if parsedTime.After(latestTime) {
			latestTime = parsedTime
			latestTDDDir = filepath.Join(workdirBase, name)
		}
	}

	if latestTDDDir == "" {
		t.Log("No TDD workdir found - skipping summary copy")
		return nil
	}

	// Create tdd-workdir directory in results
	tddDestDir := filepath.Join(resultsDir, "tdd-workdir")
	if err := os.MkdirAll(tddDestDir, 0755); err != nil {
		t.Logf("Warning: Failed to create TDD workdir destination: %v", err)
		return err
	}

	// Copy all files from TDD workdir
	tddFiles := []string{"summary.md", "tdd_state.md", "test_issues.md"}
	copiedCount := 0

	for _, filename := range tddFiles {
		srcPath := filepath.Join(latestTDDDir, filename)
		content, err := os.ReadFile(srcPath)
		if err != nil {
			// File doesn't exist, skip it
			continue
		}

		destPath := filepath.Join(tddDestDir, filename)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			t.Logf("Warning: Failed to copy %s: %v", filename, err)
			continue
		}
		copiedCount++
	}

	if copiedCount > 0 {
		t.Logf("Copied %d TDD workdir files from %s to %s", copiedCount, latestTDDDir, tddDestDir)
	} else {
		t.Logf("No TDD files found in %s", latestTDDDir)
	}

	return nil
}
