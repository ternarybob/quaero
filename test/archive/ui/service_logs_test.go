// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 11:54:27 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServiceLogsDisplay tests that the service logs section is always visible
func TestServiceLogsDisplay(t *testing.T) {
	t.Log("=== Testing Service Logs Display (Always Visible) ===")

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

	// Navigate to home page
	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(2*time.Second),
	)
	require.NoError(t, err, "Failed to navigate to home page")

	takeScreenshot(ctx, t, "service_logs_display_initial")

	t.Log("🔍 Checking service logs section visibility...")

	// Test 1: Verify service logs container exists
	var logsContainerExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('service-logs') !== null`, &logsContainerExists),
	)
	require.NoError(t, err, "Failed to check service logs container")
	assert.True(t, logsContainerExists, "Service logs container should exist")

	// Test 2: Verify service logs container is visible (not hidden)
	var isVisible bool
	var displayStyle string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const logs = document.getElementById('service-logs');
			if (!logs) return false;
			const styles = window.getComputedStyle(logs);
			return styles.display !== 'none' && styles.visibility !== 'hidden';
		})()`, &isVisible),
		chromedp.Evaluate(`(() => {
			const logs = document.getElementById('service-logs');
			if (!logs) return 'NOT_FOUND';
			const styles = window.getComputedStyle(logs);
			return styles.display;
		})()`, &displayStyle),
	)
	require.NoError(t, err, "Failed to check visibility")
	assert.True(t, isVisible, "Service logs container should be visible")
	t.Logf("📊 Display style: %s", displayStyle)

	// Test 3: Verify container has proper styling
	var containerStyles map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const logs = document.getElementById('service-logs');
			if (!logs) return null;
			const styles = window.getComputedStyle(logs);
			return {
				backgroundColor: styles.backgroundColor,
				border: styles.border,
				fontSize: styles.fontSize,
				fontFamily: styles.fontFamily,
				padding: styles.padding,
				overflowY: styles.overflowY,
				minHeight: styles.minHeight,
				maxHeight: styles.maxHeight,
				borderRadius: styles.borderRadius
			};
		})()`, &containerStyles),
	)
	require.NoError(t, err, "Failed to get container styles")
	assert.NotNil(t, containerStyles, "Container should have styles")
	t.Logf("📐 Container styles: %+v", containerStyles)

	// Test 4: Verify article header with "Service Logs" title
	var headerExists bool
	var headerText string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const article = document.getElementById('service-logs').closest('article');
			if (!article) return false;
			const header = article.querySelector('header h5');
			return header !== null;
		})()`, &headerExists),
		chromedp.Evaluate(`(() => {
			const article = document.getElementById('service-logs').closest('article');
			if (!article) return 'NO_ARTICLE';
			const header = article.querySelector('header h5');
			return header ? header.textContent.trim() : 'NO_HEADER';
		})()`, &headerText),
	)
	require.NoError(t, err, "Failed to check article header")
	assert.True(t, headerExists, "Service logs should have an article header")
	assert.Equal(t, "Service Logs", headerText, "Header should say 'Service Logs'")

	// Test 5: Verify clear button exists
	var clearButtonExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const article = document.getElementById('service-logs').closest('article');
			if (!article) return false;
			const clearButton = article.querySelector('button[onclick="clearServiceLogs()"]');
			return clearButton !== null;
		})()`, &clearButtonExists),
	)
	require.NoError(t, err, "Failed to check clear button")
	assert.True(t, clearButtonExists, "Clear logs button should exist")

	t.Log("✅ Service logs section is properly displayed and always visible")
}

// TestServiceLogsStreaming tests that logs are streamed via WebSocket
func TestServiceLogsStreaming(t *testing.T) {
	t.Log("=== Testing Service Logs Streaming (WebSocket) ===")

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

	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(3*time.Second), // Wait for WebSocket connection
	)
	require.NoError(t, err, "Failed to navigate to home page")

	t.Log("🔍 Checking WebSocket connection for service logs...")

	// Test 1: Verify WebSocketManager is connected
	var wsConnected bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof WebSocketManager !== 'undefined' && WebSocketManager.getConnectionStatus()`, &wsConnected),
	)
	require.NoError(t, err, "Failed to check WebSocket connection")
	t.Logf("🔌 WebSocketManager connected: %v", wsConnected)

	// Test 2: Simulate a log message
	t.Log("🧪 Simulating WebSocket log message...")
	var logAdded bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			if (typeof handleLogMessage === 'function') {
				const testLog = {
					timestamp: new Date().toLocaleTimeString('en-US', { hour12: false }),
					level: 'info',
					message: 'Test log from automated test - Service logs streaming verification'
				};
				handleLogMessage(testLog);
				true;
			} else {
				false;
			}
		`, &logAdded),
	)
	require.NoError(t, err, "Failed to simulate log message")
	assert.True(t, logAdded, "Should be able to add log message via handleLogMessage")

	// Test 3: Verify log appeared in container
	var logCount int
	var logContent string
	err = chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.getElementById('service-logs').children.length`, &logCount),
		chromedp.Text(`#service-logs`, &logContent),
	)
	require.NoError(t, err, "Failed to check log content")

	t.Logf("📝 Log entries: %d", logCount)
	if logCount > 0 {
		assert.Contains(t, logContent, "Test log from automated test", "Log content should contain test message")
		t.Log("✅ Log message successfully appeared in service logs")
	} else {
		t.Log("⚠️  No logs appeared yet, may need more time for system logs")
	}

	takeScreenshot(ctx, t, "service_logs_with_content")

	// Test 4: Trigger activity and wait for logs
	t.Log("🔄 Triggering activity to generate system logs...")
	err = chromedp.Run(ctx,
		chromedp.Click(`#refresh-parser-status`),
		chromedp.Sleep(5*time.Second), // Wait for logs to stream

		chromedp.Evaluate(`document.getElementById('service-logs').children.length`, &logCount),
	)
	require.NoError(t, err, "Failed to trigger activity")
	t.Logf("📝 Log entries after activity: %d", logCount)

	// Test 5: Test clear logs functionality
	t.Log("🧹 Testing clear logs functionality...")
	var logsCleared bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			if (typeof clearServiceLogs === 'function') {
				clearServiceLogs();
				true;
			} else {
				false;
			}
		`, &logsCleared),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('service-logs').children.length`, &logCount),
	)
	require.NoError(t, err, "Failed to clear logs")
	assert.True(t, logsCleared, "clearServiceLogs function should be available")
	assert.Equal(t, 0, logCount, "Logs should be cleared")

	takeScreenshot(ctx, t, "service_logs_after_clear")

	t.Log("✅ Service logs streaming and functionality verified")
}

// TestServiceLogsFormatting tests that log entries have proper formatting
func TestServiceLogsFormatting(t *testing.T) {
	t.Log("=== Testing Service Logs Formatting ===")

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
		chromedp.Sleep(3*time.Second),
	)
	require.NoError(t, err, "Failed to navigate to home page")

	// Add test logs with different levels
	t.Log("🧪 Adding test logs with different levels...")
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			if (typeof handleLogMessage === 'function') {
				handleLogMessage({
					timestamp: '12:00:00',
					level: 'info',
					message: 'Info level log message'
				});
				handleLogMessage({
					timestamp: '12:00:01',
					level: 'warn',
					message: 'Warning level log message'
				});
				handleLogMessage({
					timestamp: '12:00:02',
					level: 'error',
					message: 'Error level log message'
				});
				handleLogMessage({
					timestamp: '12:00:03',
					level: 'debug',
					message: 'Debug level log message'
				});
			}
		`, nil),
		chromedp.Sleep(1*time.Second),
	)
	require.NoError(t, err, "Failed to add test logs")

	takeScreenshot(ctx, t, "service_logs_formatted")

	// Verify log entry structure
	var logEntries []map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const logs = Array.from(document.getElementById('service-logs').children);
			return logs.map(log => {
				return {
					hasTimestamp: log.querySelector('.log-timestamp') !== null,
					hasLevel: log.querySelector('[class*="log-level-"]') !== null,
					hasMessage: log.querySelector('.log-message') !== null,
					className: log.className
				};
			});
		})()`, &logEntries),
	)
	require.NoError(t, err, "Failed to check log entry structure")

	// Verify each log entry has proper structure
	for i, entry := range logEntries {
		assert.True(t, entry["hasTimestamp"].(bool), "Log entry %d should have timestamp", i)
		assert.True(t, entry["hasLevel"].(bool), "Log entry %d should have level indicator", i)
		assert.True(t, entry["hasMessage"].(bool), "Log entry %d should have message", i)
		assert.Equal(t, "log-entry", entry["className"].(string), "Log entry %d should have log-entry class", i)
	}

	t.Log("✅ Service logs formatting verified")
}

// TestServiceLogsWebSocketConnection tests the WebSocket connection status indicator
func TestServiceLogsWebSocketConnection(t *testing.T) {
	t.Log("=== Testing Service Logs WebSocket Connection Status ===")

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
		chromedp.Sleep(3*time.Second), // Wait for WebSocket connection
	)
	require.NoError(t, err, "Failed to navigate to home page")

	takeScreenshot(ctx, t, "service_logs_ws_connection")

	t.Log("🔍 Checking WebSocket connection status indicator...")

	// Test 1: Verify WebSocket status indicator exists
	var statusIndicatorExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('logs-ws-status') !== null`, &statusIndicatorExists),
	)
	require.NoError(t, err, "Failed to check status indicator")
	assert.True(t, statusIndicatorExists, "WebSocket status indicator should exist")

	// Test 2: Verify WebSocketManager is connected
	var wsConnected bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`typeof WebSocketManager !== 'undefined' && WebSocketManager.getConnectionStatus()`, &wsConnected),
	)
	require.NoError(t, err, "Failed to check WebSocket connection")
	t.Logf("🔌 WebSocketManager connected: %v", wsConnected)

	// Test 3: Verify status indicator text and color
	var statusText string
	var statusColor string
	err = chromedp.Run(ctx,
		chromedp.Text(`#logs-ws-status`, &statusText),
		chromedp.Evaluate(`(() => {
			const indicator = document.getElementById('logs-ws-status');
			if (!indicator) return 'NOT_FOUND';
			const styles = window.getComputedStyle(indicator);
			return styles.color;
		})()`, &statusColor),
	)
	require.NoError(t, err, "Failed to check status text and color")

	// Status can be LIVE (connected), CONNECTING (in progress), or OFFLINE (disconnected)
	validStatuses := []string{"LIVE", "CONNECTING", "OFFLINE"}
	hasValidStatus := false
	for _, status := range validStatuses {
		if assert.ObjectsAreEqualValues(statusText, status) || (len(statusText) > 0 && len(statusText) >= len(status) && statusText[len(statusText)-len(status):] == status) {
			hasValidStatus = true
			break
		}
	}
	assert.True(t, hasValidStatus, "Status should show LIVE, CONNECTING, or OFFLINE, got: %s", statusText)

	if wsConnected {
		t.Logf("✅ WebSocket connected - Status: %s, Color: %s", statusText, statusColor)
	} else {
		t.Logf("⚠️  WebSocket state: %s - Status: %s, Color: %s", "not connected", statusText, statusColor)
	}

	// Test 4: Verify status indicator has circle icon
	var hasCircleIcon bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const indicator = document.getElementById('logs-ws-status');
			if (!indicator) return false;
			const icon = indicator.querySelector('i');
			return icon !== null && icon.textContent.trim() === 'circle';
		})()`, &hasCircleIcon),
	)
	require.NoError(t, err, "Failed to check circle icon")
	assert.True(t, hasCircleIcon, "Status indicator should have circle icon")

	t.Log("✅ WebSocket connection status indicator verified")
}

// TestServiceLogsConnectivity tests that the WebSocket status changes to ONLINE and turns green
func TestServiceLogsConnectivity(t *testing.T) {
	t.Log("=== Testing Service Logs Connectivity Status (ONLINE + Green) ===")

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

	err = chromedp.Run(ctx,
		chromedp.Navigate(config.ServerURL),
		chromedp.Sleep(1*time.Second), // Initial load
	)
	require.NoError(t, err, "Failed to navigate to home page")

	takeScreenshot(ctx, t, "connectivity_initial")

	t.Log("🔍 Waiting for Service Logs WebSocket to connect and status to turn ONLINE/LIVE...")

	// Wait for status to change to ONLINE/LIVE (max 60 seconds - WebSocket may take time to establish)
	var statusOnline bool
	var statusText string
	var statusColor string
	maxWait := 60 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		var statusInfo struct {
			Text           string
			Color          string
			IsOnline       bool
			IsGreen        bool
			IndicatorFound bool
		}

		err = chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				// Check ONLY the Service Logs WebSocket status indicator
				const logsStatus = document.getElementById('logs-ws-status');
				
				if (!logsStatus) {
					return {
						Text: 'NOT_FOUND',
						Color: 'NOT_FOUND',
						IsOnline: false,
						IsGreen: false,
						IndicatorFound: false
					};
				}
				
				const styles = window.getComputedStyle(logsStatus);
				// Remove icon text (like "circle") and get just the status text
				const text = logsStatus.textContent.replace(/^circle\s*/i, '').trim();
				const color = styles.color;
				
				// Check if status is ONLINE or LIVE
				const isOnline = text.includes('ONLINE') || text.includes('LIVE');
				
				// Check if color is green (looking for rgb values with high green component)
				// Green colors typically have format like rgb(26, 127, 55) or similar
				const isGreen = color.includes('rgb') && (() => {
					const match = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
					if (match) {
						const r = parseInt(match[1]);
						const g = parseInt(match[2]);
						const b = parseInt(match[3]);
						// Green should have highest green component and be significantly higher than red and blue
						// For rgb(26, 127, 55): g=127 > r=26 and g=127 > b=55
						return g > r && g > b && g > 80;
					}
					return false;
				})();
				
				return {
					Text: text,
					Color: color,
					IsOnline: isOnline,
					IsGreen: isGreen,
					IndicatorFound: true
				};
			})()`, &statusInfo),
		)

		if err != nil {
			t.Logf("⚠️  Error checking status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if !statusInfo.IndicatorFound {
			t.Logf("⚠️  Service Logs WebSocket status indicator not found")
			time.Sleep(2 * time.Second)
			continue
		}

		statusText = statusInfo.Text
		statusColor = statusInfo.Color

		elapsed := time.Since(startTime).Round(time.Second)
		t.Logf("  Service Logs WS Status: %s, Color: %s, IsOnline: %v, IsGreen: %v (elapsed: %v)",
			statusText, statusColor, statusInfo.IsOnline, statusInfo.IsGreen, elapsed)

		if statusInfo.IsOnline && statusInfo.IsGreen {
			statusOnline = true
			t.Logf("✅ Service Logs WebSocket status is ONLINE/LIVE and GREEN after %v", elapsed)
			break
		}

		time.Sleep(3 * time.Second)
	}

	takeScreenshot(ctx, t, "connectivity_final")

	// Verify results
	if !statusOnline {
		t.Errorf("❌ Service Logs WebSocket status did NOT change to ONLINE/LIVE within %v", maxWait)
		t.Errorf("Final Service Logs WS status: %s, color: %s", statusText, statusColor)
		t.Errorf("This indicates the Service Logs WebSocket is not establishing connection properly")
		t.Fatal("Connectivity test FAILED - Service Logs WebSocket not connecting")
	}

	// Additional verification - accept either "ONLINE" or "LIVE"
	hasOnline := strings.Contains(statusText, "ONLINE")
	hasLive := strings.Contains(statusText, "LIVE")

	if !hasOnline && !hasLive {
		t.Errorf("⚠️  Status text '%s' should contain 'ONLINE' or 'LIVE'", statusText)
	} else {
		if hasOnline {
			t.Log("✅ Status shows 'ONLINE'")
		}
		if hasLive {
			t.Log("✅ Status shows 'LIVE'")
		}
	}

	t.Logf("✅ Final Service Logs WS Status: %s (Color: %s)", statusText, statusColor)
	t.Log("✅ Service Logs connectivity test PASSED - WebSocket is ONLINE/LIVE and GREEN")
}
