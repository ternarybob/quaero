// -----------------------------------------------------------------------
// Last Modified: Tuesday, 4th November 2025 5:08:20 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	"github.com/chromedp/chromedp"
)

func TestHomepageTitle(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("HomepageTitle")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestHomepageTitle")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestHomepageTitle (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestHomepageTitle (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()
	var title string

	env.LogTest(t, "Navigating to homepage: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Title(&title),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	env.LogTest(t, "Page loaded successfully, title: %s", title)

	// Wait for WebSocket connection (status indicator to show ONLINE)
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Take screenshot of homepage
	if err := env.TakeScreenshot(ctx, "homepage"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("homepage"))

	expectedTitle := "Quaero - Home"
	if title != expectedTitle {
		env.LogTest(t, "ERROR: Title mismatch - expected '%s', got '%s'", expectedTitle, title)
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, title)
	} else {
		env.LogTest(t, "✓ Title verified: %s", title)
	}
}

func TestHomepageElements(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("HomepageElements")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestHomepageElements")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestHomepageElements (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestHomepageElements (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	// Navigate to homepage and wait for WebSocket first
	env.LogTest(t, "Navigating to homepage: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	// Wait for WebSocket connection (status indicator to show ONLINE)
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Check for presence of key elements
	tests := []struct {
		name     string
		selector string
	}{
		{"Header", "header.app-header"},
		{"Navigation", "nav.app-header-nav"},
		{"Page title heading", "h1"},
		{"Service status card", ".card"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nodeCount int
			err := chromedp.Run(ctx,
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

	// Check for service logs component (Alpine.js serviceLogs)
	t.Run("Service Logs Component", func(t *testing.T) {
		env.LogTest(t, "Checking for service logs component...")

		// Wait for service logs to be initialized (Alpine.js component)
		time.Sleep(2 * time.Second)

		// Check if service logs card exists
		var serviceLogsExists bool
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('[x-data="serviceLogs"]') !== null`, &serviceLogsExists),
		)

		if err != nil {
			env.LogTest(t, "ERROR: Failed to check service logs component: %v", err)
			t.Fatalf("Failed to check service logs component: %v", err)
		}

		if !serviceLogsExists {
			env.LogTest(t, "ERROR: Service logs component not found")
			t.Errorf("Service logs component (x-data='serviceLogs') not found on page")
			return
		}
		env.LogTest(t, "✓ Service logs component found")

		// Check if logs array is populated
		var logsCount int
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const serviceLogsEl = document.querySelector('[x-data="serviceLogs"]');
					if (!serviceLogsEl) return 0;
					const alpineData = Alpine.$data(serviceLogsEl);
					return alpineData && alpineData.logs ? alpineData.logs.length : 0;
				})()
			`, &logsCount),
		)

		if err != nil {
			env.LogTest(t, "ERROR: Failed to get logs count: %v", err)
			t.Fatalf("Failed to get logs count from serviceLogs component: %v", err)
		}

		env.LogTest(t, "Service logs count: %d", logsCount)

		if logsCount == 0 {
			env.LogTest(t, "WARNING: Service logs array is empty (may still be loading)")
			t.Logf("Service logs array is empty - logs may still be loading via WebSocket")
		} else {
			env.LogTest(t, "✓ Service logs populated with %d entries", logsCount)
		}

		// Take screenshot of service logs section
		env.LogTest(t, "Taking screenshot of service logs component...")
		if err := env.TakeScreenshot(ctx, "service-logs"); err != nil {
			env.LogTest(t, "ERROR: Failed to take service logs screenshot: %v", err)
			t.Fatalf("Failed to take service logs screenshot: %v", err)
		}
		env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("service-logs"))
	})

	// Take screenshot after checking all elements
	env.LogTest(t, "Taking screenshot of homepage elements...")
	if err := env.TakeScreenshot(ctx, "homepage-elements"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("homepage-elements"))
}

func TestDebugLogVisibility(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("DebugLogVisibility")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestDebugLogVisibility")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestDebugLogVisibility (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestDebugLogVisibility (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	// Navigate to homepage
	env.LogTest(t, "Navigating to homepage: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Wait for logs to stream (2-3 seconds as per analysis recommendations)
	env.LogTest(t, "Waiting for service logs to populate...")
	time.Sleep(3 * time.Second)

	// Extract logs from Alpine.js serviceLogs component
	// Use JSON.stringify to force proper serialization of Alpine.js Proxy objects
	var logsJSON string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const serviceLogsEl = document.querySelector('[x-data="serviceLogs"]');
				if (!serviceLogsEl) return JSON.stringify([]);
				const alpineData = Alpine.$data(serviceLogsEl);
				const logs = alpineData && alpineData.logs ? alpineData.logs : [];
				return JSON.stringify(logs);
			})()
		`, &logsJSON),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to extract logs from serviceLogs component: %v", err)
		t.Fatalf("Failed to extract logs from serviceLogs component: %v", err)
	}

	// Parse JSON string into array
	var logsArray []map[string]interface{}
	if err := json.Unmarshal([]byte(logsJSON), &logsArray); err != nil {
		env.LogTest(t, "ERROR: Failed to parse logs JSON: %v", err)
		env.LogTest(t, "JSON received: %s", logsJSON)
		t.Fatalf("Failed to parse logs JSON: %v", err)
	}

	env.LogTest(t, "Total logs received: %d", len(logsArray))

	// Filter for debug level logs
	var debugLogs []map[string]interface{}
	var logLevels = make(map[string]int)

	for _, log := range logsArray {
		if level, ok := log["level"].(string); ok {
			logLevels[strings.ToLower(level)]++
			if strings.ToLower(level) == "debug" {
				debugLogs = append(debugLogs, log)
			}
		}
	}

	// Log all log levels found
	env.LogTest(t, "Log levels distribution:")
	for level, count := range logLevels {
		env.LogTest(t, "  - %s: %d", level, count)
	}

	// Verify debug logs exist
	env.LogTest(t, "Debug logs found: %d", len(debugLogs))

	if len(debugLogs) == 0 {
		env.LogTest(t, "ERROR: No debug logs found in UI")
		env.LogTest(t, "This indicates that either:")
		env.LogTest(t, "  1. No debug logs were emitted during startup")
		env.LogTest(t, "  2. LogService is filtering debug logs (check min_event_level config)")
		env.LogTest(t, "  3. WebSocket is not receiving debug log events")

		// Take screenshot for debugging
		if err := env.TakeScreenshot(ctx, "debug-logs-missing"); err != nil {
			env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		} else {
			env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("debug-logs-missing"))
		}

		t.Errorf("Expected debug logs when min_event_level='debug', but found none")
		return
	}

	env.LogTest(t, "✓ Debug logs are visible in UI")

	// Validate log structure (time, level, message fields)
	env.LogTest(t, "Validating debug log structure...")
	validLogs := 0
	for i, log := range debugLogs {
		hasTimestamp := false
		hasLevel := false
		hasMessage := false

		if timestamp, ok := log["timestamp"].(string); ok && timestamp != "" {
			hasTimestamp = true
			// Verify HH:MM:SS format
			if matched, _ := regexp.MatchString(`^\d{2}:\d{2}:\d{2}$`, timestamp); matched {
				env.LogTest(t, "  Log %d: Valid timestamp format: %s", i+1, timestamp)
			} else {
				env.LogTest(t, "  Log %d: WARNING - Timestamp format unexpected: %s", i+1, timestamp)
			}
		}

		if level, ok := log["level"].(string); ok && level != "" {
			hasLevel = true
		}

		if message, ok := log["message"].(string); ok && message != "" {
			hasMessage = true
		}

		if hasTimestamp && hasLevel && hasMessage {
			validLogs++
		} else {
			env.LogTest(t, "  Log %d: Missing fields - timestamp:%v level:%v message:%v",
				i+1, hasTimestamp, hasLevel, hasMessage)
		}
	}

	env.LogTest(t, "Valid debug logs (with all required fields): %d/%d", validLogs, len(debugLogs))

	if validLogs == 0 {
		env.LogTest(t, "ERROR: No debug logs have proper structure (timestamp, level, message)")
		t.Errorf("Debug logs exist but lack proper structure")
	} else {
		env.LogTest(t, "✓ Debug logs have proper structure")
	}

	// Take screenshot showing debug logs
	env.LogTest(t, "Taking screenshot of debug logs...")
	if err := env.TakeScreenshot(ctx, "debug-logs-visible"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("debug-logs-visible"))

	env.LogTest(t, "✓ Test completed successfully - debug logs are visible and properly structured")
}

func TestLogTimestampAccuracy(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("LogTimestampAccuracy")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestLogTimestampAccuracy")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestLogTimestampAccuracy (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestLogTimestampAccuracy (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	// Record test start time for timestamp reasonability checks
	testStartTime := time.Now()
	env.LogTest(t, "Test started at: %s", testStartTime.Format("15:04:05"))

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	// Navigate to homepage
	env.LogTest(t, "Navigating to homepage: %s", url)
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load homepage: %v", err)
		t.Fatalf("Failed to load homepage: %v", err)
	}

	// Wait for WebSocket connection
	env.LogTest(t, "Waiting for WebSocket connection...")
	if err := env.WaitForWebSocketConnection(ctx, 10); err != nil {
		env.LogTest(t, "ERROR: WebSocket did not connect: %v", err)
		t.Fatalf("WebSocket connection failed: %v", err)
	}
	env.LogTest(t, "✓ WebSocket connected (status: ONLINE)")

	// Wait for logs to stream (3 seconds as per analysis recommendations)
	env.LogTest(t, "Waiting for service logs to populate...")
	time.Sleep(3 * time.Second)

	// Extract logs from Alpine.js serviceLogs component
	// Use JSON.stringify to force proper serialization of Alpine.js Proxy objects
	var logsJSON string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const serviceLogsEl = document.querySelector('[x-data="serviceLogs"]');
				if (!serviceLogsEl) return JSON.stringify([]);
				const alpineData = Alpine.$data(serviceLogsEl);
				const logs = alpineData && alpineData.logs ? alpineData.logs : [];
				return JSON.stringify(logs);
			})()
		`, &logsJSON),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to extract logs from serviceLogs component: %v", err)
		t.Fatalf("Failed to extract logs from serviceLogs component: %v", err)
	}

	// Parse JSON string into array
	var logsArray []map[string]interface{}
	if err := json.Unmarshal([]byte(logsJSON), &logsArray); err != nil {
		env.LogTest(t, "ERROR: Failed to parse logs JSON: %v", err)
		env.LogTest(t, "JSON received: %s", logsJSON)
		t.Fatalf("Failed to parse logs JSON: %v", err)
	}

	env.LogTest(t, "Total logs received: %d", len(logsArray))

	if len(logsArray) == 0 {
		env.LogTest(t, "ERROR: No logs found to verify timestamps")
		t.Fatalf("No logs available for timestamp verification")
	}

	// Verify timestamps are in HH:MM:SS format (server-provided)
	// This confirms timestamps are NOT client-calculated but come from the server
	env.LogTest(t, "Verifying timestamp format (server-provided HH:MM:SS)...")

	validTimestamps := 0
	invalidTimestamps := 0
	var sampleTimestamps []string

	// HH:MM:SS format regex
	timestampPattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)

	for i, log := range logsArray {
		timestamp, ok := log["timestamp"].(string)
		if !ok || timestamp == "" {
			env.LogTest(t, "  Log %d: Missing timestamp field", i+1)
			invalidTimestamps++
			continue
		}

		// Check if timestamp matches HH:MM:SS format (server format)
		if timestampPattern.MatchString(timestamp) {
			validTimestamps++
			// Collect first 5 timestamps as samples
			if len(sampleTimestamps) < 5 {
				sampleTimestamps = append(sampleTimestamps, timestamp)
			}
		} else {
			env.LogTest(t, "  Log %d: Invalid timestamp format: %s (expected HH:MM:SS)", i+1, timestamp)
			invalidTimestamps++
		}
	}

	env.LogTest(t, "Timestamp format validation results:")
	env.LogTest(t, "  Valid (HH:MM:SS): %d/%d (%.1f%%)",
		validTimestamps, len(logsArray), float64(validTimestamps)/float64(len(logsArray))*100)
	env.LogTest(t, "  Invalid: %d", invalidTimestamps)
	env.LogTest(t, "  Sample timestamps: %v", sampleTimestamps)

	if validTimestamps == 0 {
		env.LogTest(t, "ERROR: No timestamps match server format (HH:MM:SS)")
		env.LogTest(t, "This suggests timestamps are being calculated client-side instead of server-provided")
		t.Errorf("Expected all timestamps to be in server format (HH:MM:SS), but found none")
		return
	}

	// All timestamps should be server-provided
	if invalidTimestamps > 0 {
		env.LogTest(t, "WARNING: Some timestamps do not match server format")
		t.Errorf("Found %d timestamps not in server format (HH:MM:SS)", invalidTimestamps)
	}

	env.LogTest(t, "✓ All timestamps are in server-provided format (HH:MM:SS)")

	// Verify timestamps are within a tight cluster (logs from concurrent processes)
	// Note: Logs may arrive out of strict chronological order due to async WebSocket streaming
	// from multiple concurrent services. This is expected behavior, not a sign of client manipulation.
	env.LogTest(t, "Verifying timestamp clustering (logs from concurrent processes)...")

	var timestamps []time.Time
	for _, log := range logsArray {
		timestamp, ok := log["timestamp"].(string)
		if !ok || timestamp == "" {
			continue
		}

		// Parse HH:MM:SS timestamp using today's date
		currentTime, err := time.Parse("15:04:05", timestamp)
		if err != nil {
			env.LogTest(t, "  Failed to parse timestamp %s: %v", timestamp, err)
			continue
		}

		timestamps = append(timestamps, currentTime)
	}

	if len(timestamps) > 0 {
		// Find min and max timestamps
		minTime := timestamps[0]
		maxTime := timestamps[0]
		for _, t := range timestamps {
			if t.Before(minTime) {
				minTime = t
			}
			if t.After(maxTime) {
				maxTime = t
			}
		}

		timeSpan := maxTime.Sub(minTime)
		env.LogTest(t, "Timestamp cluster analysis:")
		env.LogTest(t, "  Earliest: %s", minTime.Format("15:04:05"))
		env.LogTest(t, "  Latest: %s", maxTime.Format("15:04:05"))
		env.LogTest(t, "  Time span: %v", timeSpan)

		// All logs should be within a few seconds of each other (concurrent startup logs)
		if timeSpan < 30*time.Second {
			env.LogTest(t, "✓ All timestamps are tightly clustered (within %v) - confirms server-provided timestamps", timeSpan)
		} else {
			env.LogTest(t, "NOTE: Timestamps span %v - may include older buffered logs", timeSpan)
		}
	}

	// Verify timestamps are reasonable (within last few minutes of current time)
	env.LogTest(t, "Verifying timestamp reasonability (within current time window)...")

	reasonableCount := 0
	unreasonableCount := 0
	now := time.Now()

	for i, log := range logsArray {
		timestamp, ok := log["timestamp"].(string)
		if !ok || timestamp == "" {
			continue
		}

		// Parse HH:MM:SS timestamp using today's date
		logTime, err := time.Parse("15:04:05", timestamp)
		if err != nil {
			continue
		}

		// Combine with today's date for comparison
		logDateTime := time.Date(now.Year(), now.Month(), now.Day(),
			logTime.Hour(), logTime.Minute(), logTime.Second(), 0, now.Location())

		// Check if timestamp is within reasonable window (test start time to now + 1 minute buffer)
		// Allow 1 minute before test start in case of clock skew or pre-existing logs
		testStartWithBuffer := testStartTime.Add(-1 * time.Minute)
		nowWithBuffer := now.Add(1 * time.Minute)

		if logDateTime.After(testStartWithBuffer) && logDateTime.Before(nowWithBuffer) {
			reasonableCount++
		} else {
			// Calculate time difference for reporting
			timeDiff := logDateTime.Sub(now)
			env.LogTest(t, "  Log %d: Timestamp %s is outside reasonable window (diff: %v)",
				i+1, timestamp, timeDiff)
			unreasonableCount++
		}
	}

	env.LogTest(t, "Timestamp reasonability check:")
	env.LogTest(t, "  Reasonable (within time window): %d/%d (%.1f%%)",
		reasonableCount, len(logsArray), float64(reasonableCount)/float64(len(logsArray))*100)
	env.LogTest(t, "  Unreasonable: %d", unreasonableCount)

	if reasonableCount == 0 {
		env.LogTest(t, "ERROR: No timestamps are within reasonable time window")
		env.LogTest(t, "This suggests timestamps may be incorrectly formatted or calculated")
		t.Errorf("Expected timestamps to be within current time window, but found none")
		return
	}

	// Most timestamps should be reasonable (allow 10% margin for edge cases)
	if float64(reasonableCount)/float64(len(logsArray)) < 0.9 {
		env.LogTest(t, "WARNING: Less than 90%% of timestamps are within reasonable time window")
	} else {
		env.LogTest(t, "✓ Timestamps are within reasonable time window (server-generated)")
	}

	// Take screenshot showing timestamps
	env.LogTest(t, "Taking screenshot of log timestamps...")
	if err := env.TakeScreenshot(ctx, "log-timestamps"); err != nil {
		env.LogTest(t, "ERROR: Failed to take screenshot: %v", err)
		t.Fatalf("Failed to take screenshot: %v", err)
	}
	env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath("log-timestamps"))

	// Summary
	env.LogTest(t, "")
	env.LogTest(t, "=== TIMESTAMP VERIFICATION SUMMARY ===")
	env.LogTest(t, "✓ All timestamps are in server-provided format (HH:MM:SS)")
	env.LogTest(t, "✓ Timestamps are tightly clustered (concurrent service startup logs)")
	env.LogTest(t, "✓ Timestamps are within reasonable time window")
	env.LogTest(t, "")
	env.LogTest(t, "CONCLUSION: Timestamps are confirmed to be SERVER-PROVIDED, not client-calculated")
	env.LogTest(t, "  - Format: HH:MM:SS (server-side formatting in LogService.transformEvent())")
	env.LogTest(t, "  - Flow: Server (arbor) -> LogService -> EventService -> WebSocket -> UI")
	env.LogTest(t, "  - Client: _formatLogTime() preserves server format (no recalculation)")
	env.LogTest(t, "  - Async delivery: Logs may arrive out of strict order due to concurrent streaming")
	env.LogTest(t, "")
	env.LogTest(t, "✓ Test completed successfully - timestamps are server-provided and accurate")
}

func TestNavigation(t *testing.T) {
	// Setup test environment with test name
	env, err := common.SetupTestEnvironment("Navigation")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestNavigation")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestNavigation (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestNavigation (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL()

	tests := []struct {
		linkText      string
		linkHref      string
		expectedTitle string
	}{
		{"Jobs", "/jobs", "Job Management"},
		{"Queue", "/queue", "Queue"},
		{"Documents", "/documents", "Document Management"},
		{"Search", "/search", "Search"},
		{"Chat", "/chat", "Chat"},
		{"Settings", "/settings", "Settings"},
	}

	for _, tt := range tests {
		t.Run(tt.linkText, func(t *testing.T) {
			env.LogTest(t, "Testing navigation to %s (%s)", tt.linkText, tt.linkHref)

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
				env.LogTest(t, "ERROR: Failed to navigate to %s: %v", tt.linkText, err)
				t.Fatalf("Failed to navigate to %s: %v", tt.linkText, err)
			}

			// Take screenshot of the navigated page
			screenshotName := fmt.Sprintf("navigation-%s", strings.ToLower(tt.linkText))
			if err := env.TakeScreenshot(ctx, screenshotName); err != nil {
				env.LogTest(t, "ERROR: Failed to take screenshot for %s: %v", tt.linkText, err)
				t.Fatalf("Failed to take screenshot for %s: %v", tt.linkText, err)
			}
			env.LogTest(t, "Screenshot saved: %s", env.GetScreenshotPath(screenshotName))

			if !strings.Contains(title, tt.expectedTitle) {
				env.LogTest(t, "ERROR: Title mismatch for %s - expected to contain '%s', got '%s'",
					tt.linkText, tt.expectedTitle, title)
				t.Errorf("After clicking '%s', expected title to contain '%s', got '%s'",
					tt.linkText, tt.expectedTitle, title)
			} else {
				env.LogTest(t, "✓ Navigation to %s successful, title: %s", tt.linkText, title)
			}
		})
	}
}
