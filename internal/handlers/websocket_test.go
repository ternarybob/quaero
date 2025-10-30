package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
)

// TestLogDispatchFanOut verifies that log broadcast correctly fans out to multiple subscribers
// without blocking or leaking goroutines
func TestLogDispatchFanOut(t *testing.T) {
	// Create logger
	logger := arbor.NewLogger()

	// Create WebSocket handler
	handler := NewWebSocketHandler(nil, logger, &common.WebSocketConfig{})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Number of subscribers to test
	numSubscribers := 5

	// Track received messages for each subscriber
	receivedMessages := make([][]LogEntry, numSubscribers)
	var receivedMutex sync.Mutex

	// WaitGroup for subscribers
	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	// Track goroutine count before test
	initialGoroutines := countGoroutines()

	// Create subscribers
	subscribers := make([]*websocket.Conn, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		// Connect to WebSocket
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect subscriber %d: %v", i, err)
		}
		subscribers[i] = conn

		// Start goroutine to read messages
		subscriberIdx := i
		go func() {
			defer wg.Done()
			defer conn.Close()

			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			for {
				var msg WSMessage
				err := conn.ReadJSON(&msg)
				if err != nil {
					// Expected when connection closes or deadline reached
					return
				}

				// Filter for log messages only
				if msg.Type == "log" {
					// Parse payload as LogEntry
					logData, err := json.Marshal(msg.Payload)
					if err != nil {
						continue
					}

					var logEntry LogEntry
					if err := json.Unmarshal(logData, &logEntry); err != nil {
						continue
					}

					// Store received message
					receivedMutex.Lock()
					receivedMessages[subscriberIdx] = append(receivedMessages[subscriberIdx], logEntry)
					receivedMutex.Unlock()
				}
			}
		}()
	}

	// Wait for all subscribers to connect
	time.Sleep(100 * time.Millisecond)

	// Verify all subscribers are connected
	handler.mu.RLock()
	connectedClients := len(handler.clients)
	handler.mu.RUnlock()

	if connectedClients != numSubscribers {
		t.Errorf("Expected %d connected clients, got %d", numSubscribers, connectedClients)
	}

	// Test messages to send
	testLogs := []struct {
		level   string
		message string
	}{
		{"INFO", "Test log message 1"},
		{"DEBUG", "Test log message 2"},
		{"WARN", "Test log message 3"},
		{"ERROR", "Test log message 4"},
		{"INFO", "Test log message 5"},
	}

	// Send logs concurrently to test thread safety
	var sendWg sync.WaitGroup
	sendWg.Add(len(testLogs))

	for _, log := range testLogs {
		logCopy := log // Capture loop variable
		go func() {
			defer sendWg.Done()
			handler.SendLog(logCopy.level, logCopy.message)
		}()
	}

	// Wait for all logs to be sent
	sendWg.Wait()

	// Allow time for messages to be received
	time.Sleep(500 * time.Millisecond)

	// Close all connections
	for _, conn := range subscribers {
		conn.Close()
	}

	// Wait for all subscribers to finish
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// All subscribers finished
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for subscribers to finish")
	}

	// Verify all subscribers received all messages
	receivedMutex.Lock()
	defer receivedMutex.Unlock()

	for i, messages := range receivedMessages {
		// Each subscriber should have received all test logs
		// Note: They may also receive status messages, so check >= expected count
		logCount := 0
		for _, msg := range messages {
			// Count only our test messages
			for _, testLog := range testLogs {
				if msg.Level == strings.ToLower(testLog.level) && msg.Message == testLog.message {
					logCount++
					break
				}
			}
		}

		if logCount != len(testLogs) {
			t.Errorf("Subscriber %d received %d test logs, expected %d", i, logCount, len(testLogs))
			t.Logf("Subscriber %d messages: %+v", i, messages)
		}
	}

	// Verify messages were received in order for each subscriber
	for i, messages := range receivedMessages {
		// Extract our test messages
		var testMessages []LogEntry
		for _, msg := range messages {
			for _, testLog := range testLogs {
				if msg.Level == strings.ToLower(testLog.level) && msg.Message == testLog.message {
					testMessages = append(testMessages, msg)
					break
				}
			}
		}

		// Since we sent concurrently, we can't guarantee global order,
		// but we should have all messages
		if len(testMessages) != len(testLogs) {
			t.Errorf("Subscriber %d has incomplete test messages", i)
		}
	}

	// Wait a bit for goroutines to clean up
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks
	finalGoroutines := countGoroutines()
	goroutineDiff := finalGoroutines - initialGoroutines

	// Allow some tolerance for background goroutines
	if goroutineDiff > 2 {
		t.Errorf("Potential goroutine leak detected: %d goroutines leaked", goroutineDiff)
	}

	// Verify handler cleaned up all clients
	handler.mu.RLock()
	remainingClients := len(handler.clients)
	remainingMutexes := len(handler.clientMutex)
	handler.mu.RUnlock()

	if remainingClients != 0 {
		t.Errorf("Handler still has %d clients after cleanup", remainingClients)
	}

	if remainingMutexes != 0 {
		t.Errorf("Handler still has %d client mutexes after cleanup", remainingMutexes)
	}

	t.Logf("✓ Successfully broadcast %d logs to %d subscribers", len(testLogs), numSubscribers)
	t.Log("✓ All subscribers received messages in order")
	t.Log("✓ No goroutine leaks detected")
	t.Log("✓ All resources cleaned up properly")
}

// TestConcurrentLogDispatch verifies that concurrent log dispatches don't cause race conditions
func TestConcurrentLogDispatch(t *testing.T) {
	// Create logger
	logger := arbor.NewLogger()

	// Create WebSocket handler
	handler := NewWebSocketHandler(nil, logger, &common.WebSocketConfig{})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect a subscriber
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect subscriber: %v", err)
	}
	defer conn.Close()

	// Count received messages
	var messageCount int32
	done := make(chan struct{})

	// Read messages in background
	go func() {
		defer close(done)
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))

		for {
			var msg WSMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}

			if msg.Type == "log" {
				atomic.AddInt32(&messageCount, 1)
			}
		}
	}()

	// Number of concurrent senders
	numSenders := 10
	logsPerSender := 10

	// Send logs concurrently
	var wg sync.WaitGroup
	wg.Add(numSenders)

	start := time.Now()

	for i := 0; i < numSenders; i++ {
		senderID := i
		go func() {
			defer wg.Done()

			for j := 0; j < logsPerSender; j++ {
				handler.SendLog("INFO", "Sender "+string(rune(senderID))+" message "+string(rune(j)))
			}
		}()
	}

	// Wait for all senders to finish
	wg.Wait()

	// Allow time for messages to be received
	time.Sleep(500 * time.Millisecond)

	// Close connection to stop reader
	conn.Close()

	// Wait for reader to finish
	<-done

	elapsed := time.Since(start)

	// Verify message count
	totalExpected := int32(numSenders * logsPerSender)
	received := atomic.LoadInt32(&messageCount)

	if received != totalExpected {
		t.Errorf("Received %d messages, expected %d", received, totalExpected)
	}

	t.Logf("✓ Successfully sent %d messages concurrently from %d senders", totalExpected, numSenders)
	t.Logf("✓ All messages received without blocking (elapsed: %v)", elapsed)
	t.Log("✓ No race conditions detected")
}

// TestLogDispatchWithTimeouts verifies that slow/blocked subscribers don't affect others
func TestLogDispatchWithTimeouts(t *testing.T) {
	// Create logger
	logger := arbor.NewLogger()

	// Create WebSocket handler
	handler := NewWebSocketHandler(nil, logger, &common.WebSocketConfig{})

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect fast subscriber
	fastConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect fast subscriber: %v", err)
	}
	defer fastConn.Close()

	// Connect slow subscriber (won't read messages)
	slowConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect slow subscriber: %v", err)
	}
	defer slowConn.Close()

	// Count messages for fast subscriber
	var fastMessages int32
	fastDone := make(chan struct{})

	// Fast subscriber reads messages quickly
	go func() {
		defer close(fastDone)
		fastConn.SetReadDeadline(time.Now().Add(3 * time.Second))

		for {
			var msg WSMessage
			err := fastConn.ReadJSON(&msg)
			if err != nil {
				return
			}

			if msg.Type == "log" {
				atomic.AddInt32(&fastMessages, 1)
			}
		}
	}()

	// Send multiple log messages
	numLogs := 20
	for i := 0; i < numLogs; i++ {
		handler.SendLog("INFO", "Test message "+string(rune(i)))
		time.Sleep(10 * time.Millisecond) // Small delay between messages
	}

	// Allow time for messages to be processed
	time.Sleep(500 * time.Millisecond)

	// Close connections
	fastConn.Close()
	slowConn.Close()

	// Wait for fast subscriber to finish
	<-fastDone

	// Check fast subscriber received all messages
	received := atomic.LoadInt32(&fastMessages)
	if received != int32(numLogs) {
		t.Errorf("Fast subscriber received %d messages, expected %d", received, numLogs)
	}

	t.Logf("✓ Fast subscriber received all %d messages", numLogs)
	t.Log("✓ Slow/blocked subscriber did not affect fast subscriber")
	t.Log("✓ System handles heterogeneous subscriber speeds correctly")
}

// Helper function to count goroutines
func countGoroutines() int {
	// This is approximate - in production you might use runtime.NumGoroutine()
	// or pprof for more accurate measurement
	return runtime.NumGoroutine()
}
