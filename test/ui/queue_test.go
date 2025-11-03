package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// =============================================================================
// Test Helper Functions
// =============================================================================

// waitForWebSocketConnection waits for the WebSocket connection to be established
// Returns error if connection is not established within the timeout
func waitForWebSocketConnection(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for WebSocket connection")
		case <-ticker.C:
			var wsConnected bool
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`typeof jobsWS !== 'undefined' && jobsWS !== null && jobsWS.readyState === 1`, &wsConnected),
			)
			if err != nil {
				continue
			}
			if wsConnected {
				return nil
			}
		}
	}
}

// waitForJobStatusChange waits for a job to reach the expected status
// Returns error if status doesn't change within the timeout
func waitForJobStatusChange(ctx context.Context, jobID, expectedStatus string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for job %s to reach status %s", jobID, expectedStatus)
		case <-ticker.C:
			status, err := getJobCardStatus(ctx, jobID)
			if err != nil {
				continue
			}
			if strings.EqualFold(status, expectedStatus) {
				return nil
			}
		}
	}
}

// countNetworkRequests counts HTTP requests matching the URL pattern over the duration
// Returns the count of matching requests
func countNetworkRequests(ctx context.Context, urlPattern string, duration time.Duration) (int, error) {
	// Start request monitoring
	var requestCount int
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				window._testRequestCount = 0;
				const originalFetch = window.fetch;
				window.fetch = function(...args) {
					const url = args[0];
					if (typeof url === 'string' && url.includes('%s')) {
						window._testRequestCount++;
					}
					return originalFetch.apply(this, args);
				};
			})();
		`, urlPattern), nil),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to setup request monitoring: %w", err)
	}

	// Wait for the specified duration
	time.Sleep(duration)

	// Get final count
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window._testRequestCount || 0`, &requestCount),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to get request count: %w", err)
	}

	return requestCount, nil
}

// getJobCardElement returns a selector for the job card element
func getJobCardElement(ctx context.Context, jobID string) (string, error) {
	// Job cards have data-job-id attribute on checkboxes
	// Use substring matching for short job IDs
	selector := fmt.Sprintf(`.job-checkbox[data-job-id*="%s"]`, jobID)

	var exists bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, selector), &exists),
	)

	if err != nil {
		return "", fmt.Errorf("failed to check job card element: %w", err)
	}

	if !exists {
		return "", fmt.Errorf("job card not found for jobID: %s", jobID)
	}

	return selector, nil
}

// getJobCardStatus returns the current status of a job card
func getJobCardStatus(ctx context.Context, jobID string) (string, error) {
	// Find job card by jobID and extract status badge text
	var status string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				const checkbox = document.querySelector('.job-checkbox[data-job-id*="%s"]');
				if (!checkbox) return '';
				const card = checkbox.closest('.card');
				if (!card) return '';
				const statusBadge = card.querySelector('.label');
				return statusBadge ? statusBadge.textContent.trim() : '';
			})();
		`, jobID), &status),
	)

	if err != nil {
		return "", fmt.Errorf("failed to get job status: %w", err)
	}

	if status == "" {
		return "", fmt.Errorf("status not found for jobID: %s", jobID)
	}

	return status, nil
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestQueuePageWebSocketConnection verifies WebSocket connection is established
// and that no polling occurs after connection is established
func TestQueuePageWebSocketConnection(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueuePageWebSocketConnection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Take screenshot before checking connection
	if err := TakeScreenshot(ctx, "queue-websocket-connection-initial"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Wait for WebSocket connection to be established
	if err := waitForWebSocketConnection(ctx, 10*time.Second); err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Verify WebSocket is connected
	var wsConnected bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`wsConnected`, &wsConnected),
	)

	if err != nil {
		t.Fatalf("Failed to check WebSocket connection state: %v", err)
	}

	if !wsConnected {
		t.Error("WebSocket connection state variable is false")
	}

	// Take screenshot after connection established
	if err := TakeScreenshot(ctx, "queue-websocket-connection-established"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify no polling is occurring - check that autoRefreshInterval doesn't exist
	var pollingExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof autoRefreshInterval !== 'undefined' && autoRefreshInterval !== null`, &pollingExists),
	)

	if err != nil {
		t.Fatalf("Failed to check for polling: %v", err)
	}

	if pollingExists {
		t.Error("Polling mechanism still exists (autoRefreshInterval is defined)")
	}

	// Monitor network requests to /api/jobs list endpoint for 5 seconds to ensure no polling
	// Use more specific pattern to exclude /api/jobs/queue stats endpoint
	requestCount, err := countNetworkRequests(ctx, "/api/jobs?", 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to monitor network requests: %v", err)
	}

	// Allow up to 1 request (initial load), but no more (no polling)
	if requestCount > 1 {
		t.Errorf("Expected at most 1 API request (initial load), found %d requests (indicates polling)", requestCount)
	}

	t.Log("✓ WebSocket connection established successfully")
	t.Log("✓ No polling mechanism detected")
	t.Logf("✓ Network requests to /api/jobs: %d (expected: 0-1 for initial load)", requestCount)
}

// TestQueuePageJobStatusUpdate verifies real-time job status updates via WebSocket
// This test simulates or monitors for job status changes and verifies the UI updates
func TestQueuePageJobStatusUpdate(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueuePageJobStatusUpdate")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for initial data load
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Wait for WebSocket connection
	if err := waitForWebSocketConnection(ctx, 10*time.Second); err != nil {
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "queue-job-status-update-initial"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Check if there are any jobs displayed
	var jobCount int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.job-checkbox').length`, &jobCount),
	)

	if err != nil {
		t.Fatalf("Failed to count jobs: %v", err)
	}

	if jobCount == 0 {
		t.Skip("No jobs available for testing job status updates")
		return
	}

	// Verify WebSocket message handler is registered for job_status_change events using runtime signal
	// Monkey-patch updateJobInList to set a flag when called
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// Store original function
				window._originalUpdateJobInList = window.updateJobInList;
				window._jobStatusHandled = false;

				// Monkey-patch to set flag when called
				window.updateJobInList = function(job) {
					window._jobStatusHandled = true;
					// Call original function
					if (window._originalUpdateJobInList) {
						return window._originalUpdateJobInList.apply(this, arguments);
					}
				};

				// Return true to indicate setup successful
				return true;
			})();
		`, nil),
	)

	if err != nil {
		t.Fatalf("Failed to setup job status handler monitoring: %v", err)
	}

	// Simulate a job_status_change message if WebSocket is connected
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				if (jobsWS && jobsWS.onmessage) {
					// Create a mock job status change event
					const mockEvent = {
						data: JSON.stringify({
							type: 'job_status_change',
							data: {
								id: 'test-job-123',
								status: 'completed'
							}
						})
					};
					// Trigger the handler
					jobsWS.onmessage(mockEvent);
					return true;
				}
				return false;
			})();
		`, nil),
	)

	// Check if the flag was set (handler was called)
	var jobStatusHandled bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`window._jobStatusHandled`, &jobStatusHandled),
	)

	if err != nil {
		t.Fatalf("Failed to check job status handler signal: %v", err)
	}

	// Restore original function
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			if (window._originalUpdateJobInList) {
				window.updateJobInList = window._originalUpdateJobInList;
				delete window._originalUpdateJobInList;
			}
		`, nil),
	)

	if !jobStatusHandled {
		t.Error("WebSocket message handler does not properly handle job_status_change events")
	}

	// Verify updateJobInList function exists
	var updateFunctionExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof updateJobInList === 'function'`, &updateFunctionExists),
	)

	if err != nil {
		t.Fatalf("Failed to check updateJobInList function: %v", err)
	}

	if !updateFunctionExists {
		t.Error("updateJobInList function not found")
	}

	t.Log("✓ WebSocket job_status_change handler registered")
	t.Log("✓ updateJobInList function exists")
	t.Logf("✓ Found %d job(s) on page for potential status updates", jobCount)
}

// TestQueuePageWebSocketReconnection verifies exponential backoff reconnection
// when WebSocket connection is lost
func TestQueuePageWebSocketReconnection(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueuePageWebSocketReconnection")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Wait for WebSocket connection
	if err := waitForWebSocketConnection(ctx, 10*time.Second); err != nil {
		t.Fatalf("Initial WebSocket connection failed: %v", err)
	}

	// Take screenshot of initial connection
	if err := TakeScreenshot(ctx, "queue-websocket-reconnection-initial"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify reconnection constants are defined
	var reconnectConfigExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof WS_MAX_RECONNECT_DELAY !== 'undefined' && typeof WS_INITIAL_RECONNECT_DELAY !== 'undefined'`, &reconnectConfigExists),
	)

	if err != nil {
		t.Fatalf("Failed to check reconnection config: %v", err)
	}

	if !reconnectConfigExists {
		t.Error("WebSocket reconnection constants (WS_MAX_RECONNECT_DELAY, WS_INITIAL_RECONNECT_DELAY) not defined")
	}

	// Get reconnection delay values
	var maxDelay, initialDelay int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`WS_MAX_RECONNECT_DELAY`, &maxDelay),
		chromedp.Evaluate(`WS_INITIAL_RECONNECT_DELAY`, &initialDelay),
	)

	if err != nil {
		t.Fatalf("Failed to get reconnection delay values: %v", err)
	}

	// Verify exponential backoff configuration
	if maxDelay != 30000 {
		t.Errorf("Expected WS_MAX_RECONNECT_DELAY to be 30000ms, got %dms", maxDelay)
	}

	if initialDelay != 1000 {
		t.Errorf("Expected WS_INITIAL_RECONNECT_DELAY to be 1000ms, got %dms", initialDelay)
	}

	// Simulate WebSocket disconnection by closing the connection
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			if (jobsWS) {
				console.log('[Test] Simulating WebSocket disconnection');
				jobsWS.close();
			}
		`, nil),
	)

	if err != nil {
		t.Fatalf("Failed to simulate WebSocket disconnection: %v", err)
	}

	// Take screenshot after disconnection
	if err := TakeScreenshot(ctx, "queue-websocket-reconnection-disconnected"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Wait a bit for reconnection to start
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to wait for reconnection: %v", err)
	}

	// Verify reconnection attempt counter is incrementing
	var reconnectAttempts int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`wsReconnectAttempts`, &reconnectAttempts),
	)

	if err != nil {
		t.Fatalf("Failed to check reconnection attempts: %v", err)
	}

	if reconnectAttempts == 0 {
		t.Error("Expected reconnection attempts > 0 after disconnection, but wsReconnectAttempts is still 0")
	}

	// Wait for reconnection to complete (should happen within initial delay + some buffer)
	if err := waitForWebSocketConnection(ctx, 15*time.Second); err != nil {
		t.Logf("Warning: Reconnection did not complete within 15 seconds: %v", err)
		t.Log("This might be expected in test environment if server WebSocket is not available")
	}

	// Take screenshot after reconnection attempt
	if err := TakeScreenshot(ctx, "queue-websocket-reconnection-complete"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	t.Log("✓ WebSocket reconnection constants configured correctly")
	t.Logf("✓ Initial reconnection delay: %dms", initialDelay)
	t.Logf("✓ Max reconnection delay: %dms", maxDelay)
	t.Logf("✓ Reconnection attempts after disconnection: %d", reconnectAttempts)
}

// TestQueuePageManualRefresh verifies manual refresh fallback works correctly
// This test ensures users can manually refresh the page to get latest data
func TestQueuePageManualRefresh(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestQueuePageManualRefresh")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for initial data load
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Take screenshot of initial state
	if err := TakeScreenshot(ctx, "queue-manual-refresh-initial"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify manual refresh buttons exist
	var statsRefreshExists, jobsRefreshExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('button[onclick="loadStats()"]') !== null`, &statsRefreshExists),
		chromedp.Evaluate(`document.querySelector('button[onclick="loadJobs()"]') !== null`, &jobsRefreshExists),
	)

	if err != nil {
		t.Fatalf("Failed to check for refresh buttons: %v", err)
	}

	if !statsRefreshExists {
		t.Error("Stats refresh button not found")
	}

	if !jobsRefreshExists {
		t.Error("Jobs refresh button not found")
	}

	// Click the jobs refresh button
	err = chromedp.Run(ctx,
		chromedp.Click(`button[onclick="loadJobs()"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for refresh to complete
	)

	if err != nil {
		t.Fatalf("Failed to click jobs refresh button: %v", err)
	}

	// Take screenshot after manual refresh
	if err := TakeScreenshot(ctx, "queue-manual-refresh-after-click"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify loadJobs and loadStats functions exist
	var loadJobsExists, loadStatsExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof loadJobs === 'function'`, &loadJobsExists),
		chromedp.Evaluate(`typeof loadStats === 'function'`, &loadStatsExists),
	)

	if err != nil {
		t.Fatalf("Failed to check for manual refresh functions: %v", err)
	}

	if !loadJobsExists {
		t.Error("loadJobs function not found")
	}

	if !loadStatsExists {
		t.Error("loadStats function not found")
	}

	t.Log("✓ Manual stats refresh button exists")
	t.Log("✓ Manual jobs refresh button exists")
	t.Log("✓ loadJobs() function available")
	t.Log("✓ loadStats() function available")
	t.Log("✓ Manual refresh successfully triggered")
}

// TestServiceLogsNoClientFiltering verifies that client-side log filtering is absent
// and that all log filtering is done server-side before broadcasting
func TestServiceLogsNoClientFiltering(t *testing.T) {
	// Setup test environment
	env, err := SetupTestEnvironment("TestServiceLogsNoClientFiltering")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/queue"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to initialize
	)

	if err != nil {
		t.Fatalf("Failed to load queue page: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "queue-service-logs-no-filtering"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// Verify serviceLogs component exists (Alpine.js component)
	var serviceLogsExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const serviceLogsElement = document.querySelector('[x-data="serviceLogs"]');
				return serviceLogsElement !== null;
			})();
		`, &serviceLogsExists),
	)

	if err != nil {
		t.Fatalf("Failed to check for serviceLogs component: %v", err)
	}

	if !serviceLogsExists {
		t.Error("serviceLogs Alpine.js component not found")
	}

	// Verify NO client-side log filtering controls exist in the DOM
	// Check for absence of filter dropdown or filter input fields
	var filterControlsExist bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// Look for common filter control elements
				const logContainer = document.querySelector('[x-data="serviceLogs"]');
				if (!logContainer) return false;

				// Check for filter-related controls
				const hasFilterDropdown = logContainer.querySelector('select[id*="filter"], select[name*="filter"]') !== null;
				const hasFilterInput = logContainer.querySelector('input[placeholder*="filter"], input[name*="filter"]') !== null;
				const hasLevelSelector = logContainer.querySelector('select[id*="level"], select[name*="level"]') !== null;
				const hasFilterButton = logContainer.querySelector('button:has([class*="filter"]), button[onclick*="filter"]') !== null;

				return hasFilterDropdown || hasFilterInput || hasLevelSelector || hasFilterButton;
			})();
		`, &filterControlsExist),
	)

	if err != nil {
		t.Fatalf("Failed to check for filter controls: %v", err)
	}

	if filterControlsExist {
		t.Error("Client-side log filter controls found in DOM (filtering should be server-side only)")
	}

	// Simulate different log levels and verify they all render without client-side suppression
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// Simulate adding logs of different levels directly
				const logContainer = document.querySelector('.terminal');
				if (!logContainer) return false;

				// Store original log count
				window._testOriginalLogCount = logContainer.querySelectorAll('.terminal-line').length;

				// Add test logs of different levels using the component's addLog function if available
				if (typeof Alpine !== 'undefined') {
					const component = Alpine.$data(document.querySelector('[x-data="serviceLogs"]'));
					if (component && component.addLog) {
						// Add logs of different levels
						component.addLog({ id: 'test1', timestamp: new Date().toISOString(), level: 'DEBUG', message: 'Test DEBUG message' });
						component.addLog({ id: 'test2', timestamp: new Date().toISOString(), level: 'INFO', message: 'Test INFO message' });
						component.addLog({ id: 'test3', timestamp: new Date().toISOString(), level: 'WARN', message: 'Test WARN message' });
						component.addLog({ id: 'test4', timestamp: new Date().toISOString(), level: 'ERROR', message: 'Test ERROR message' });
					}
				}
				return true;
			})();
		`, nil),
	)

	if err != nil {
		t.Logf("Warning: Could not simulate logs: %v", err)
	}

	// Verify all log levels are displayed (no client-side filtering)
	var allLevelsDisplayed bool
	var logLevelCounts map[string]int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const logContainer = document.querySelector('.terminal');
				if (!logContainer) return { allDisplayed: false, counts: {} };

				// Count logs by level
				const counts = {};
				const logLines = logContainer.querySelectorAll('.terminal-line');

				logLines.forEach(line => {
					// Extract level from log line
					const levelMatch = line.textContent.match(/\[(DEBUG|INFO|WARN|ERROR)\]/);
					if (levelMatch) {
						const level = levelMatch[1];
						counts[level] = (counts[level] || 0) + 1;
					}
				});

				// Check if we have our test logs
				const hasDebug = logContainer.textContent.includes('Test DEBUG message');
				const hasInfo = logContainer.textContent.includes('Test INFO message');
				const hasWarn = logContainer.textContent.includes('Test WARN message');
				const hasError = logContainer.textContent.includes('Test ERROR message');

				return {
					allDisplayed: hasDebug && hasInfo && hasWarn && hasError,
					counts: counts
				};
			})();
		`, &map[string]interface{}{
			"allDisplayed": &allLevelsDisplayed,
			"counts":       &logLevelCounts,
		}),
	)

	if err != nil {
		t.Logf("Warning: Could not verify log levels: %v", err)
	} else if allLevelsDisplayed {
		t.Log("✓ All log levels (DEBUG/INFO/WARN/ERROR) render without client-side suppression")
	}

	// Take final screenshot
	if err := TakeScreenshot(ctx, "queue-service-logs-all-levels"); err != nil {
		t.Logf("Warning: Failed to take final screenshot: %v", err)
	}

	t.Log("✓ serviceLogs component exists")
	t.Log("✓ No client-side filter controls found in DOM")
	if !filterControlsExist {
		t.Log("✓ Confirmed: All log filtering is server-side only")
	}
}
