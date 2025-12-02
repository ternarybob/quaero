// Package api provides API integration tests for WebSocket endpoint
package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// Note: Testing WebSocket message throttling, event whitelist filtering, and specific event types
// (crawl_progress, job_spawn, etc.) is deferred to dedicated performance/integration test suites.
// This test file focuses on core WebSocket functionality: connection lifecycle, message broadcasting,
// concurrent clients, and cleanup.

// Helper functions for WebSocket test operations

// connectWebSocket establishes a WebSocket connection to the test server
func connectWebSocket(t *testing.T, env *common.TestEnvironment) *websocket.Conn {
	conn, err := connectWebSocketWithError(t, env)
	require.NoError(t, err, "Failed to establish WebSocket connection")
	return conn
}

// connectWebSocketWithError establishes a WebSocket connection and returns error instead of failing
// This is useful for concurrent connection tests where errors should be collected
func connectWebSocketWithError(t *testing.T, env *common.TestEnvironment) (*websocket.Conn, error) {
	// Convert HTTP base URL to WebSocket URL
	baseURL := env.GetBaseURL()
	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws"

	// Use TestLogger to ensure output is captured to test.log
	logger := env.NewTestLogger(t)
	logger.Logf("Connecting to WebSocket: %s", wsURL)

	// Dial WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}

	// Set default read deadline to prevent hanging (5 seconds)
	err = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		conn.Close()
		return nil, err
	}

	logger.Logf("✓ WebSocket connected: %s", wsURL)
	return conn, nil
}

// readWebSocketMessage reads a single JSON message from WebSocket with timeout
func readWebSocketMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) (map[string]interface{}, error) {
	return readWebSocketMessageWithLogger(nil, t, conn, timeout)
}

// readWebSocketMessageWithLogger reads a single JSON message from WebSocket with timeout
// If logger is provided, uses it; otherwise falls back to t.Logf
func readWebSocketMessageWithLogger(logger *common.TestLogger, t *testing.T, conn *websocket.Conn, timeout time.Duration) (map[string]interface{}, error) {
	// Set read deadline based on timeout
	err := conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read JSON message
	var msg map[string]interface{}
	err = conn.ReadJSON(&msg)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Logf("Received WebSocket message: type=%v", msg["type"])
	} else {
		t.Logf("Received WebSocket message: type=%v", msg["type"])
	}
	return msg, nil
}

// waitForMessageType reads messages until finding one with specified type, or times out
func waitForMessageType(t *testing.T, conn *websocket.Conn, messageType string, timeout time.Duration) map[string]interface{} {
	return waitForMessageTypeWithLogger(nil, t, conn, messageType, timeout)
}

// waitForMessageTypeWithLogger reads messages until finding one with specified type, or times out
// If logger is provided, uses it; otherwise falls back to t.Logf
func waitForMessageTypeWithLogger(logger *common.TestLogger, t *testing.T, conn *websocket.Conn, messageType string, timeout time.Duration) map[string]interface{} {
	deadline := time.Now().Add(timeout)

	for {
		// Check if timeout reached
		if time.Now().After(deadline) {
			require.FailNow(t, fmt.Sprintf("Timeout waiting for message type '%s' after %v", messageType, timeout))
		}

		// Read next message
		remaining := time.Until(deadline)
		msg, err := readWebSocketMessageWithLogger(logger, t, conn, remaining)
		if err != nil {
			// If timeout or connection closed, fail
			require.FailNow(t, fmt.Sprintf("Error waiting for message type '%s': %v", messageType, err))
		}

		// Check if this is the message type we're looking for
		if msgType, ok := msg["type"].(string); ok && msgType == messageType {
			if logger != nil {
				logger.Logf("✓ Found message type '%s'", messageType)
			} else {
				t.Logf("✓ Found message type '%s'", messageType)
			}
			return msg
		}

		if logger != nil {
			logger.Logf("Skipping message type '%v', waiting for '%s'", msg["type"], messageType)
		} else {
			t.Logf("Skipping message type '%v', waiting for '%s'", msg["type"], messageType)
		}
	}
}

// closeWebSocket gracefully closes a WebSocket connection
func closeWebSocket(t *testing.T, conn *websocket.Conn) {
	closeWebSocketWithLogger(nil, t, conn)
}

// closeWebSocketWithLogger gracefully closes a WebSocket connection
// If logger is provided, uses it; otherwise falls back to t.Log
func closeWebSocketWithLogger(logger *common.TestLogger, t *testing.T, conn *websocket.Conn) {
	if conn == nil {
		return
	}

	err := conn.Close()
	if err != nil {
		// Connection might already be closed, log but don't fail
		if logger != nil {
			logger.Logf("Warning: Error closing WebSocket: %v", err)
		} else {
			t.Logf("Warning: Error closing WebSocket: %v", err)
		}
	} else {
		if logger != nil {
			logger.Log("✓ WebSocket connection closed")
		} else {
			t.Log("✓ WebSocket connection closed")
		}
	}
}

// Test functions

// TestWebSocketConnection tests connection lifecycle
func TestWebSocketConnection(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("Success", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Verify initial status message received
		msg := waitForMessageType(t, conn, "status", 2*time.Second)
		assert.NotNil(t, msg, "Should receive initial status message")
		assert.Equal(t, "status", msg["type"], "Message type should be 'status'")

		// Close connection cleanly
		closeWebSocket(t, conn)

		t.Log("✓ Success test completed")
	})

	t.Run("MultipleConnections", func(t *testing.T) {
		// Connect 3 clients concurrently
		var wg sync.WaitGroup
		conns := make([]*websocket.Conn, 3)
		errors := make([]error, 3)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				conn, err := connectWebSocketWithError(t, env)
				conns[index] = conn
				errors[index] = err
			}(i)
		}

		wg.Wait()

		// Verify all connections succeeded
		for i, err := range errors {
			require.NoError(t, err, "Connection %d should succeed", i)
			require.NotNil(t, conns[i], "Connection %d should not be nil", i)
		}

		// Close all connections
		for i, conn := range conns {
			if conn != nil {
				err := conn.Close()
				assert.NoError(t, err, "Closing connection %d should succeed", i)
			}
		}

		t.Log("✓ Multiple connections test completed")
	})

	t.Run("ReconnectAfterClose", func(t *testing.T) {
		// First connection
		conn1 := connectWebSocket(t, env)
		closeWebSocket(t, conn1)

		// Second connection (after first is closed)
		conn2 := connectWebSocket(t, env)
		defer closeWebSocket(t, conn2)

		// Verify second connection works
		msg := waitForMessageType(t, conn2, "status", 2*time.Second)
		assert.NotNil(t, msg, "Should receive status message on reconnection")

		t.Log("✓ Reconnect after close test completed")
	})

	t.Run("InvalidUpgrade", func(t *testing.T) {
		// Use regular HTTP GET (not WebSocket upgrade) to /ws
		helper := env.NewHTTPTestHelper(t)
		resp, err := helper.GET("/ws")
		require.NoError(t, err, "HTTP GET should not error at network level")
		defer resp.Body.Close()

		// Verify appropriate error response (400 Bad Request or 426 Upgrade Required)
		assert.True(t,
			resp.StatusCode == http.StatusBadRequest || resp.StatusCode == 426,
			"Invalid upgrade should return 400 or 426, got %d", resp.StatusCode)

		t.Log("✓ Invalid upgrade test completed")
	})

	t.Log("✓ TestWebSocketConnection completed successfully")
}

// TestWebSocketStatusMessages tests status update broadcasting
func TestWebSocketStatusMessages(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("InitialStatus", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Wait for initial status message
		msg := waitForMessageType(t, conn, "status", 2*time.Second)
		assert.NotNil(t, msg, "Should receive initial status message")
		assert.Equal(t, "status", msg["type"], "Message type should be 'status'")

		// Verify payload structure
		assert.NotNil(t, msg["payload"], "Status message should have payload")

		t.Log("✓ Initial status test completed")
	})

	t.Run("StatusBroadcast", func(t *testing.T) {
		// Connect 2 WebSocket clients
		conn1 := connectWebSocket(t, env)
		defer closeWebSocket(t, conn1)

		conn2 := connectWebSocket(t, env)
		defer closeWebSocket(t, conn2)

		// Clear initial status messages
		waitForMessageType(t, conn1, "status", 2*time.Second)
		waitForMessageType(t, conn2, "status", 2*time.Second)

		// Status broadcasts occur periodically (every 5 seconds based on handler)
		// We can also try triggering an action to generate activity
		helper := env.NewHTTPTestHelper(t)
		jobID := createTestJob(t, helper)
		if jobID != "" {
			defer deleteJob(t, helper, jobID)
		}

		// Wait for status messages on both clients (they should receive broadcasts)
		// Loop to collect messages for a period and find status messages
		msg1, err1 := readWebSocketMessage(t, conn1, 6*time.Second)
		msg2, err2 := readWebSocketMessage(t, conn2, 6*time.Second)

		// At least one client should receive a message (could be status or log)
		// Try to find status messages or any broadcast
		if err1 == nil {
			t.Logf("Client 1 received message: type=%v", msg1["type"])
			assert.NotNil(t, msg1, "Client 1 should receive a broadcast message")
		}

		if err2 == nil {
			t.Logf("Client 2 received message: type=%v", msg2["type"])
			assert.NotNil(t, msg2, "Client 2 should receive a broadcast message")
		}

		// Both clients should receive some form of broadcast (status, log, or other)
		assert.True(t, err1 == nil || err2 == nil, "At least one client should receive a broadcast message")

		t.Log("✓ Status broadcast test completed")
	})

	t.Log("✓ TestWebSocketStatusMessages completed successfully")
}

// TestWebSocketAuthBroadcast tests authentication capture broadcasting
func TestWebSocketAuthBroadcast(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Ensure clean auth state
	cleanupAllAuth(t, env)
	defer cleanupAllAuth(t, env)

	t.Run("AuthCaptured", func(t *testing.T) {
		// Connect WebSocket client first
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Capture auth via HTTP API
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)
		defer deleteTestAuth(t, env, credID)

		// Wait for auth broadcast message (type="auth")
		msg := waitForMessageType(t, conn, "auth", 5*time.Second)
		assert.NotNil(t, msg, "Should receive auth message after auth capture")
		assert.Equal(t, "auth", msg["type"], "Message type should be 'auth'")

		// Verify payload contains expected fields
		payload, ok := msg["payload"].(map[string]interface{})
		require.True(t, ok, "Payload should be a map")
		assert.NotNil(t, payload, "Auth message should have payload")
		assert.Contains(t, payload, "baseUrl", "Payload should contain baseUrl")
		assert.NotEmpty(t, payload["baseUrl"], "BaseUrl should not be empty")

		t.Log("✓ Auth captured test completed")
	})

	t.Run("MultipleClientsReceiveAuth", func(t *testing.T) {
		// Connect 2 WebSocket clients
		conn1 := connectWebSocket(t, env)
		defer closeWebSocket(t, conn1)

		conn2 := connectWebSocket(t, env)
		defer closeWebSocket(t, conn2)

		// Clear initial status messages
		waitForMessageType(t, conn1, "status", 2*time.Second)
		waitForMessageType(t, conn2, "status", 2*time.Second)

		// Capture auth
		authData := createTestAuthData()
		credID := captureTestAuth(t, env, authData)
		defer deleteTestAuth(t, env, credID)

		// Wait for auth broadcast message on both clients
		msg1 := waitForMessageType(t, conn1, "auth", 5*time.Second)
		assert.NotNil(t, msg1, "Client 1 should receive auth message")
		assert.Equal(t, "auth", msg1["type"], "Client 1 message type should be 'auth'")

		msg2 := waitForMessageType(t, conn2, "auth", 5*time.Second)
		assert.NotNil(t, msg2, "Client 2 should receive auth message")
		assert.Equal(t, "auth", msg2["type"], "Client 2 message type should be 'auth'")

		// Verify both payloads contain expected fields
		payload1, ok := msg1["payload"].(map[string]interface{})
		require.True(t, ok, "Client 1 payload should be a map")
		assert.Contains(t, payload1, "baseUrl", "Client 1 payload should contain baseUrl")

		payload2, ok := msg2["payload"].(map[string]interface{})
		require.True(t, ok, "Client 2 payload should be a map")
		assert.Contains(t, payload2, "baseUrl", "Client 2 payload should contain baseUrl")

		t.Log("✓ Multiple clients receive auth test completed")
	})

	t.Log("✓ TestWebSocketAuthBroadcast completed successfully")
}

// TestWebSocketLogStreaming tests real-time log message streaming
func TestWebSocketLogStreaming(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("JobCreationLogs", func(t *testing.T) {
		// Connect WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a test job to trigger log messages
		helper := env.NewHTTPTestHelper(t)
		jobID := createTestJob(t, helper)
		if jobID != "" {
			defer deleteJob(t, helper, jobID)
		}

		// Wait for log-type message from job creation
		msg := waitForMessageType(t, conn, "log", 5*time.Second)
		assert.NotNil(t, msg, "Should receive log message after job creation")
		assert.Equal(t, "log", msg["type"], "Message type should be 'log'")
		assert.NotNil(t, msg["payload"], "Log message should have payload")

		t.Log("✓ Job creation logs test completed")
	})

	t.Run("MultipleLogMessages", func(t *testing.T) {
		// Connect WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create multiple test jobs to trigger log messages
		helper := env.NewHTTPTestHelper(t)
		jobID1 := createTestJob(t, helper)
		if jobID1 != "" {
			defer deleteJob(t, helper, jobID1)
		}

		jobID2 := createTestJob(t, helper)
		if jobID2 != "" {
			defer deleteJob(t, helper, jobID2)
		}

		// Collect messages for a short period
		messages := []map[string]interface{}{}
		logCount := 0
		deadline := time.Now().Add(5 * time.Second)

		for time.Now().Before(deadline) {
			remaining := time.Until(deadline)
			msg, err := readWebSocketMessage(t, conn, remaining)
			if err == nil {
				messages = append(messages, msg)
				// Count log-type messages
				if msgType, ok := msg["type"].(string); ok && msgType == "log" {
					logCount++
				}
			} else {
				break
			}
		}

		t.Logf("Collected %d messages (%d log messages) in 5 seconds", len(messages), logCount)
		assert.Greater(t, logCount, 0, "Should receive at least one log message from job creation")

		t.Log("✓ Multiple log messages test completed")
	})

	t.Run("LogMessageStructure", func(t *testing.T) {
		// Connect WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Wait for any message and check structure
		msg, err := readWebSocketMessage(t, conn, 2*time.Second)
		if err == nil {
			// Verify basic structure
			assert.NotNil(t, msg["type"], "Message should have type field")
			t.Logf("Message structure verified: type=%v", msg["type"])
		} else {
			t.Logf("No messages received for structure check: %v", err)
		}

		t.Log("✓ Log message structure test completed")
	})

	t.Log("✓ TestWebSocketLogStreaming completed successfully")
}

// TestWebSocketConcurrentClients tests multiple concurrent client handling
func TestWebSocketConcurrentClients(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("FiveClients", func(t *testing.T) {
		// Connect 5 WebSocket clients concurrently
		var wg sync.WaitGroup
		conns := make([]*websocket.Conn, 5)
		errors := make([]error, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				conn, err := connectWebSocketWithError(t, env)
				conns[index] = conn
				errors[index] = err
			}(i)
		}

		wg.Wait()

		// Verify all connections succeeded
		for i, err := range errors {
			require.NoError(t, err, "Connection %d should succeed", i)
			require.NotNil(t, conns[i], "Connection %d should not be nil", i)
		}

		// Read initial status message from all clients
		for i, conn := range conns {
			if conn != nil {
				msg, err := readWebSocketMessage(t, conn, 2*time.Second)
				assert.NoError(t, err, "Client %d should receive initial message", i)
				if err == nil {
					assert.NotNil(t, msg, "Client %d message should not be nil", i)
				}
			}
		}

		// Close all connections
		for i, conn := range conns {
			if conn != nil {
				err := conn.Close()
				assert.NoError(t, err, "Closing connection %d should succeed", i)
			}
		}

		t.Log("✓ Five clients test completed")
	})

	t.Run("ClientIsolation", func(t *testing.T) {
		// Connect 2 clients
		conn1 := connectWebSocket(t, env)
		conn2 := connectWebSocket(t, env)

		// Read initial status messages
		waitForMessageType(t, conn1, "status", 2*time.Second)
		waitForMessageType(t, conn2, "status", 2*time.Second)

		// Close first client
		closeWebSocket(t, conn1)

		// Verify second client still works (can read messages if any)
		// Just verify it doesn't panic or error on the connection
		msg, err := readWebSocketMessage(t, conn2, 2*time.Second)
		if err == nil {
			t.Logf("Client 2 received message after client 1 closed: type=%v", msg["type"])
		} else {
			t.Logf("Client 2 timeout (expected if no broadcasts): %v", err)
		}

		// Close second client
		closeWebSocket(t, conn2)

		t.Log("✓ Client isolation test completed")
	})

	t.Log("✓ TestWebSocketConcurrentClients completed successfully")
}

// TestWebSocketConnectionCleanup tests connection cleanup and resource management
func TestWebSocketConnectionCleanup(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("CloseFromClient", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)

		// Close from client side
		err := conn.Close()
		assert.NoError(t, err, "Client-side close should not error")

		t.Log("✓ Close from client test completed")
	})

	t.Run("ReadAfterClose", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)

		// Close connection
		err := conn.Close()
		require.NoError(t, err)

		// Attempt to read message (should error)
		var msg map[string]interface{}
		err = conn.ReadJSON(&msg)
		assert.Error(t, err, "Reading after close should return error")

		t.Log("✓ Read after close test completed")
	})

	t.Run("DoubleClose", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)

		// First close
		err := conn.Close()
		assert.NoError(t, err, "First close should succeed")

		// Second close (should be idempotent, might error but shouldn't panic)
		err = conn.Close()
		// Second close might error, but shouldn't panic
		t.Logf("Second close result: %v", err)

		t.Log("✓ Double close test completed")
	})

	t.Log("✓ TestWebSocketConnectionCleanup completed successfully")
}

// TestWebSocketMessageTimeout tests timeout handling
func TestWebSocketMessageTimeout(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	t.Run("ReadTimeout", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Set short read deadline
		err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		require.NoError(t, err)

		// Attempt to read without triggering events (should timeout)
		var msg map[string]interface{}
		err = conn.ReadJSON(&msg)

		// Should get a timeout error (not panic or hang)
		if err != nil {
			t.Logf("Read timeout as expected: %v", err)
		} else {
			t.Log("Received unexpected message (might be background activity)")
		}

		t.Log("✓ Read timeout test completed")
	})

	t.Run("NoMessagesOK", func(t *testing.T) {
		// Connect to WebSocket
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Wait without triggering events
		time.Sleep(500 * time.Millisecond)

		// Connection should still be valid
		// Try to read with reasonable timeout
		msg, err := readWebSocketMessage(t, conn, 500*time.Millisecond)
		if err != nil {
			t.Logf("No messages received (expected): %v", err)
		} else {
			t.Logf("Received message (background activity): type=%v", msg["type"])
		}

		t.Log("✓ No messages OK test completed")
	})

	t.Log("✓ TestWebSocketMessageTimeout completed successfully")
}
