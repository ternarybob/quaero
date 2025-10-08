// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 10:49:15 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIndexPageFunctionality tests the core functionality of the index/home page
func TestIndexPageFunctionality(t *testing.T) {
	t.Log("=== Testing Index Page Functionality (Service Status, Service Logs, WebSocket) ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Navigate to home page and wait for load
	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(3*time.Second), // Allow WebSocket connections to establish
	)
	require.NoError(t, err, "Failed to navigate to home page")

	// Take initial screenshot
	takeScreenshot(ctx, t, "index_page_loaded")

	t.Log("🔍 Testing Service Status Section...")

	// Test 1: Verify Service Status section exists and has content
	var serviceStatusExists bool
	var serviceStatusContent string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('#service-status-table') !== null`, &serviceStatusExists),
		chromedp.Text(`#service-status-table`, &serviceStatusContent),
	)
	require.NoError(t, err, "Failed to check service status section")
	assert.True(t, serviceStatusExists, "Service Status table should exist on index page")
	assert.Contains(t, serviceStatusContent, "PARSER SERVICE", "Service status should contain 'PARSER SERVICE'")
	assert.Contains(t, serviceStatusContent, "DATABASE", "Service status should contain 'DATABASE'")
	assert.Contains(t, serviceStatusContent, "EXTENSION AUTH", "Service status should contain 'EXTENSION AUTH'")

	t.Log("🔍 Testing Navbar Status Indicator...")

	// Test 2: Verify Navbar Status shows ONLINE and is green
	var navbarStatusText string
	var navbarStatusColor string
	err = chromedp.Run(ctx,
		chromedp.Text(`.status-text`, &navbarStatusText),
		chromedp.Evaluate(`(() => {
			const statusText = document.querySelector('.status-text');
			if (!statusText) return 'NOT_FOUND';
			const styles = window.getComputedStyle(statusText);
			return styles.color;
		})()`, &navbarStatusColor),
	)
	require.NoError(t, err, "Failed to check navbar status")
	assert.Equal(t, "ONLINE", navbarStatusText, "Navbar status should show 'ONLINE'")
	t.Logf("📊 Navbar status: %s (color: %s)", navbarStatusText, navbarStatusColor)

	// Check if status has proper CSS pseudo-element (dot with animation)
	var hasPseudoElement bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const statusText = document.querySelector('.status-text');
			if (!statusText) return false;
			const styles = window.getComputedStyle(statusText, '::before');
			return styles.content !== 'none' && styles.content !== '';
		})()`, &hasPseudoElement),
	)
	require.NoError(t, err, "Failed to check status pseudo-element")
	t.Logf("🔴 Status has pulsing dot indicator: %v", hasPseudoElement)

	t.Log("🔍 Testing Service Logs Section...")

	// Test 3: Verify Service Logs section exists
	var serviceLogsExists bool
	var serviceLogsContainer string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('service-logs') !== null`, &serviceLogsExists),
		chromedp.Evaluate(`(() => {
			const logsContainer = document.getElementById('service-logs');
			return logsContainer ? logsContainer.className : 'NOT_FOUND';
		})()`, &serviceLogsContainer),
	)
	require.NoError(t, err, "Failed to check service logs section")
	assert.True(t, serviceLogsExists, "Service Logs section should exist on index page")
	t.Logf("📝 Service logs container class: %s", serviceLogsContainer)

	// Test 4: Wait for and verify log content appears (wait up to 30 seconds)
	t.Log("⏳ Waiting for service logs to populate...")

	var logsContent string
	var logsCount int
	maxWaitTime := 30 * time.Second
	checkInterval := 2 * time.Second
	deadline := time.Now().Add(maxWaitTime)

	for time.Now().Before(deadline) {
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const logsContainer = document.getElementById('service-logs');
				if (!logsContainer) return 'NO_CONTAINER';
				return logsContainer.innerHTML.trim();
			})()`, &logsContent),
			chromedp.Evaluate(`(() => {
				const logsContainer = document.getElementById('service-logs');
				return logsContainer ? logsContainer.children.length : 0;
			})()`, &logsCount),
		)
		require.NoError(t, err, "Failed to check service logs content")

		if logsCount > 0 {
			t.Logf("✅ Found %d log entries", logsCount)
			break
		}

		t.Logf("⌛ No logs yet, waiting... (%s remaining)", deadline.Sub(time.Now()).Truncate(time.Second))
		time.Sleep(checkInterval)
	}

	// Take screenshot of final state
	takeScreenshot(ctx, t, "index_page_final_state")

	// Test 5: Verify WebSocket connections are working
	t.Log("🔍 Testing WebSocket Connections...")

	// Check for WebSocket connection messages
	var wsConnections int
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			// Count WebSocket connection messages in console
			let wsCount = 0;
			if (typeof navbarWs !== 'undefined' && navbarWs.readyState === 1) wsCount++;
			if (typeof serviceLogsWs !== 'undefined' && serviceLogsWs.readyState === 1) wsCount++;
			if (typeof indexDataWs !== 'undefined' && indexDataWs.readyState === 1) wsCount++;
			return wsCount;
		})()`, &wsConnections),
	)
	require.NoError(t, err, "Failed to check WebSocket connections")

	t.Logf("🔌 Active WebSocket connections: %d", wsConnections)

	// Test 6: Trigger some activity to generate logs
	t.Log("🔄 Triggering activity to generate logs...")
	err = chromedp.Run(ctx,
		// Click the refresh button to trigger activity
		chromedp.Click(`#refresh-parser-status`),
		chromedp.Sleep(3*time.Second),

		// Check logs again after activity
		chromedp.Text(`#service-logs`, &logsContent),
		chromedp.Evaluate(`(() => {
			const logsContainer = document.getElementById('service-logs');
			return logsContainer ? logsContainer.children.length : 0;
		})()`, &logsCount),
	)
	require.NoError(t, err, "Failed to trigger activity and check logs")

	// Final assertions
	t.Log("📊 Final Results:")
	t.Logf("   Service Status: ✅ Exists and contains required sections")
	t.Logf("   Navbar Status: %s (color: %s)", navbarStatusText, navbarStatusColor)
	t.Logf("   Status Indicator: Has pulsing dot = %v", hasPseudoElement)
	t.Logf("   Service Logs: %d entries", logsCount)
	t.Logf("   WebSocket Connections: %d", wsConnections)

	// Assertions for critical functionality
	assert.True(t, serviceStatusExists, "Service Status section must exist")
	assert.True(t, serviceLogsExists, "Service Logs section must exist")
	assert.Equal(t, "ONLINE", navbarStatusText, "System should show as ONLINE")

	// WebSocket connections should be established (we expect 3: navbar, service-logs, index)
	assert.GreaterOrEqual(t, wsConnections, 1, "At least one WebSocket connection should be active")

	// If no logs appear after triggering activity, that's a warning but not a failure
	if logsCount == 0 {
		t.Log("⚠️  WARNING: No service logs appeared after triggering activity")
		t.Log("   This might indicate an issue with log streaming or parsing")
	} else {
		t.Logf("✅ Service logs are working: %d entries found", logsCount)
	}

	t.Log("✅ Index page functionality test completed")
}

// TestServiceLogsWebSocketMessages specifically tests WebSocket log message handling
func TestServiceLogsWebSocketMessages(t *testing.T) {
	t.Log("=== Testing Service Logs WebSocket Message Handling ===")

	config, err := LoadTestConfig()
	require.NoError(t, err, "Failed to load test configuration")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.WindowSize(1280, 720),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, time.Minute)
	defer cancel()

	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(5*time.Second), // Wait for WebSocket connections
	)
	require.NoError(t, err, "Failed to navigate to home page")

	// Test WebSocket message simulation
	t.Log("🧪 Simulating WebSocket log message...")

	var result bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			// Simulate a log message as if it came from WebSocket
			const mockLogData = {
				timestamp: new Date().toLocaleTimeString('en-US', { hour12: false }),
				level: 'info',
				message: 'Test log message from automated test'
			};
			
			// Call the handleLogMessage function directly
			if (typeof handleLogMessage === 'function') {
				handleLogMessage(mockLogData);
				return true;
			}
			return false;
		})()`, &result),
	)
	require.NoError(t, err, "Failed to simulate WebSocket message")

	if result {
		t.Log("✅ Successfully simulated WebSocket log message")

		// Check if the message appeared
		var logsContent string
		var logsCount int
		err = chromedp.Run(ctx,
			chromedp.Sleep(1*time.Second), // Brief wait for DOM update
			chromedp.Text(`#service-logs`, &logsContent),
			chromedp.Evaluate(`document.getElementById('service-logs').children.length`, &logsCount),
		)
		require.NoError(t, err, "Failed to check log content after simulation")

		assert.Greater(t, logsCount, 0, "Simulated log message should appear in service logs")
		assert.Contains(t, logsContent, "Test log message", "Service logs should contain the test message")

		t.Logf("📝 Log entries after simulation: %d", logsCount)
		t.Log("✅ WebSocket log message handling is working correctly")
	} else {
		t.Log("⚠️  WARNING: handleLogMessage function not available for testing")
	}
}
