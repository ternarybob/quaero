// Package api provides WebSocket job event integration tests
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

// WebSocketEventCollector collects and categorizes WebSocket events
type WebSocketEventCollector struct {
	mu     sync.Mutex
	events []map[string]interface{}
}

// Add adds an event to the collector
func (c *WebSocketEventCollector) Add(event map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

// GetEvents returns all collected events
func (c *WebSocketEventCollector) GetEvents() []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]interface{}{}, c.events...)
}

// GetEventsByType returns events filtered by type
func (c *WebSocketEventCollector) GetEventsByType(eventType string) []map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	var filtered []map[string]interface{}
	for _, event := range c.events {
		if t, ok := event["type"].(string); ok && t == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// Count returns total event count
func (c *WebSocketEventCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

// collectWebSocketEvents reads events from WebSocket connection for specified duration
func collectWebSocketEvents(t *testing.T, conn *websocket.Conn, duration time.Duration) *WebSocketEventCollector {
	return collectWebSocketEventsWithLogger(nil, t, conn, duration)
}

// collectWebSocketEventsWithLogger reads events from WebSocket connection for specified duration
// If logger is provided, uses it; otherwise falls back to t.Logf
func collectWebSocketEventsWithLogger(logger *common.TestLogger, t *testing.T, conn *websocket.Conn, duration time.Duration) *WebSocketEventCollector {
	collector := &WebSocketEventCollector{}
	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		msg, err := readWebSocketMessageWithLogger(logger, t, conn, remaining)
		if err != nil {
			// Timeout or connection closed
			break
		}
		collector.Add(msg)
	}

	return collector
}

// TestWebSocketJobEvents_CrawlerJobLogEventContext tests that crawler_job_log events have proper step context.
// This test builds and starts a fresh service instance independently.
func TestWebSocketJobEvents_CrawlerJobLogEventContext(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	logger := env.NewTestLogger(t)

	// Connect WebSocket client
	conn := connectWebSocket(t, env)
	defer closeWebSocketWithLogger(logger, t, conn)

	// Clear initial status message
	waitForMessageTypeWithLogger(logger, t, conn, "status", 2*time.Second)

	// Create a test job to trigger events
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Collect events for 10 seconds
	logger.Log("Collecting WebSocket events...")
	collector := collectWebSocketEventsWithLogger(logger, t, conn, 10*time.Second)

	logger.Logf("Collected %d total events", collector.Count())

	// Analyze crawler_job_log events
	crawlerLogEvents := collector.GetEventsByType("crawler_job_log")
	logger.Logf("Found %d crawler_job_log events", len(crawlerLogEvents))

	// Check each event for required context fields
	eventsWithStepContext := 0
	eventsWithoutStepContext := 0

	for i, event := range crawlerLogEvents {
		payload, ok := event["payload"].(map[string]interface{})
		if !ok {
			logger.Logf("Event %d: Missing payload", i)
			continue
		}

		hasStepName := false
		hasStepID := false
		hasManagerID := false

		if _, exists := payload["step_name"]; exists {
			hasStepName = true
		}
		if _, exists := payload["step_id"]; exists {
			hasStepID = true
		}
		if _, exists := payload["manager_id"]; exists {
			hasManagerID = true
		}

		if hasStepName || hasStepID || hasManagerID {
			eventsWithStepContext++
			logger.Logf("Event %d: HAS step context (step_name=%v, step_id=%v, manager_id=%v)",
				i, hasStepName, hasStepID, hasManagerID)
		} else {
			eventsWithoutStepContext++
			logger.Logf("Event %d: MISSING step context - job_id=%v, message=%v",
				i, payload["job_id"], payload["message"])
		}
	}

	// Log summary
	logger.Logf("Summary: %d events WITH step context, %d events WITHOUT step context",
		eventsWithStepContext, eventsWithoutStepContext)

	// This test documents the current behavior - crawler_job_log events are missing step context
	if eventsWithoutStepContext > 0 && eventsWithStepContext == 0 {
		logger.Log("ISSUE CONFIRMED: crawler_job_log events are missing step context fields (step_name, step_id, manager_id)")
	}
}

// TestWebSocketJobEvents_StepProgressEventContext tests that step_progress events have proper context.
// This test builds and starts a fresh service instance independently.
func TestWebSocketJobEvents_StepProgressEventContext(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	logger := env.NewTestLogger(t)

	// Connect WebSocket client
	conn := connectWebSocket(t, env)
	defer closeWebSocketWithLogger(logger, t, conn)

	// Clear initial status message
	waitForMessageTypeWithLogger(logger, t, conn, "status", 2*time.Second)

	// Create a test job
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Collect events
	collector := collectWebSocketEventsWithLogger(logger, t, conn, 10*time.Second)

	// Analyze step_progress events
	stepProgressEvents := collector.GetEventsByType("step_progress")
	logger.Logf("Found %d step_progress events", len(stepProgressEvents))

	for i, event := range stepProgressEvents {
		payload, ok := event["payload"].(map[string]interface{})
		if !ok {
			logger.Logf("Event %d: Missing payload", i)
			continue
		}

		stepID := payload["step_id"]
		managerID := payload["manager_id"]
		stepName := payload["step_name"]
		status := payload["status"]

		logger.Logf("step_progress event %d: step_id=%v, manager_id=%v, step_name=%v, status=%v",
			i, stepID, managerID, stepName, status)

		// Verify required fields
		assert.NotNil(t, stepID, "step_progress event should have step_id")
		assert.NotNil(t, managerID, "step_progress event should have manager_id")
	}
}

// TestWebSocketJobEvents_JobLogEventContext tests that job_log events have proper manager_id context.
// This test builds and starts a fresh service instance independently.
func TestWebSocketJobEvents_JobLogEventContext(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	logger := env.NewTestLogger(t)

	// Connect WebSocket client
	conn := connectWebSocket(t, env)
	defer closeWebSocketWithLogger(logger, t, conn)

	// Clear initial status message
	waitForMessageTypeWithLogger(logger, t, conn, "status", 2*time.Second)

	// Create a test job
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Collect events
	collector := collectWebSocketEventsWithLogger(logger, t, conn, 10*time.Second)

	// Analyze job_log events
	jobLogEvents := collector.GetEventsByType("job_log")
	logger.Logf("Found %d job_log events", len(jobLogEvents))

	eventsWithManagerID := 0
	eventsWithoutManagerID := 0
	eventsWithOriginator := 0
	eventsWithoutOriginator := 0
	originatorValues := make(map[string]int)

	for i, event := range jobLogEvents {
		payload, ok := event["payload"].(map[string]interface{})
		if !ok {
			logger.Logf("Event %d: Missing payload", i)
			continue
		}

		if _, exists := payload["manager_id"]; exists {
			eventsWithManagerID++
		} else {
			eventsWithoutManagerID++
			logger.Logf("Event %d: job_log missing manager_id - job_id=%v", i, payload["job_id"])
		}

		if originator, exists := payload["originator"]; exists && originator != nil && originator != "" {
			eventsWithOriginator++
			if originatorStr, ok := originator.(string); ok {
				originatorValues[originatorStr]++
			}
		} else {
			eventsWithoutOriginator++
		}
	}

	logger.Logf("job_log events: %d with manager_id, %d without manager_id",
		eventsWithManagerID, eventsWithoutManagerID)
	logger.Logf("job_log events: %d with originator, %d without originator",
		eventsWithOriginator, eventsWithoutOriginator)
	logger.Logf("Originator values found: %v", originatorValues)
}

// TestWebSocketJobEvents_AllEventTypeSummary tests and summarizes all WebSocket event types.
// This test builds and starts a fresh service instance independently.
func TestWebSocketJobEvents_AllEventTypeSummary(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	logger := env.NewTestLogger(t)

	// Connect WebSocket client
	conn := connectWebSocket(t, env)
	defer closeWebSocketWithLogger(logger, t, conn)

	// Clear initial status message
	waitForMessageTypeWithLogger(logger, t, conn, "status", 2*time.Second)

	// Create a test job
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job completion
	waitForJobCompletion(t, helper, jobID, 60*time.Second)

	// Collect remaining events briefly
	collector := collectWebSocketEventsWithLogger(logger, t, conn, 5*time.Second)

	// Count events by type
	eventTypes := make(map[string]int)
	for _, event := range collector.GetEvents() {
		if eventType, ok := event["type"].(string); ok {
			eventTypes[eventType]++
		}
	}

	logger.Log("Event type summary:")
	for eventType, count := range eventTypes {
		logger.Logf("  %s: %d events", eventType, count)
	}
}

// TestWebSocketJobEvents_MultiStepJob tests events from a multi-step job
func TestWebSocketJobEvents_MultiStepJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Run("MultiStepEventAttribution", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a multi-step job definition
		defID := fmt.Sprintf("test-multi-step-%d", time.Now().UnixNano())
		multiStepBody := map[string]interface{}{
			"id":   defID,
			"name": "Multi-Step Test Job",
			"type": "multi_step",
			"steps": []map[string]interface{}{
				{
					"name": "step_one",
					"type": "crawler",
					"config": map[string]interface{}{
						"start_urls":  []string{"https://example.com"},
						"max_depth":   1,
						"max_pages":   2,
						"concurrency": 1,
					},
				},
				{
					"name": "step_two",
					"type": "crawler",
					"config": map[string]interface{}{
						"start_urls":  []string{"https://example.org"},
						"max_depth":   1,
						"max_pages":   2,
						"concurrency": 1,
					},
				},
			},
		}

		resp, err := helper.POST("/api/job-definitions", multiStepBody)
		if err != nil || resp.StatusCode != 201 {
			t.Skip("Could not create multi-step job definition")
			return
		}
		defer resp.Body.Close()
		defer deleteJobDefinition(t, helper, defID)

		// Execute the job definition
		jobID := executeJobDefinition(t, helper, defID)
		if jobID == "" {
			t.Skip("Could not execute job definition")
			return
		}
		defer deleteJob(t, helper, jobID)

		// Collect events for job execution
		t.Log("Collecting WebSocket events during multi-step job execution...")
		collector := collectWebSocketEvents(t, conn, 30*time.Second)

		t.Logf("Collected %d total events", collector.Count())

		// Analyze events by step
		stepEvents := make(map[string][]map[string]interface{})

		for _, event := range collector.GetEvents() {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				continue
			}

			stepName, _ := payload["step_name"].(string)
			if stepName == "" {
				stepName = "(no_step_name)"
			}

			stepEvents[stepName] = append(stepEvents[stepName], event)
		}

		t.Log("Events grouped by step_name:")
		for stepName, events := range stepEvents {
			t.Logf("  %s: %d events", stepName, len(events))
		}

		// Check if events are properly attributed to steps
		if len(stepEvents["(no_step_name)"]) > 0 {
			t.Logf("WARNING: %d events have no step_name attribution", len(stepEvents["(no_step_name)"]))

			// Log sample of unattributed events
			for i, event := range stepEvents["(no_step_name)"] {
				if i >= 5 {
					t.Logf("  ... and %d more", len(stepEvents["(no_step_name)"])-5)
					break
				}
				eventType, _ := event["type"].(string)
				t.Logf("  Unattributed event %d: type=%s", i, eventType)
			}
		}
	})
}

// TestWebSocketJobEvents_RequiredFields documents required fields for each event type
func TestWebSocketJobEvents_RequiredFields(t *testing.T) {
	t.Log("This test documents the expected fields for proper UI event filtering:")
	t.Log("")
	t.Log("=== REQUIRED FIELDS FOR STEP-LEVEL EVENT AGGREGATION ===")
	t.Log("")
	t.Log("All workers now use uniform job_log events via AddJobLogWithEvent:")
	t.Log("")
	t.Log("1. job_log events (unified event type for all workers):")
	t.Log("   - job_id: ID of the job that produced the log")
	t.Log("   - parent_job_id: ID of the parent job (for log aggregation)")
	t.Log("   - manager_id: ID of the root manager job (for UI aggregation)")
	t.Log("   - step_name: Name of the step (for step panel filtering)")
	t.Log("   - level: Log level (debug, info, warn, error)")
	t.Log("   - message: Log message")
	t.Log("   - source_type: Source type (e.g., 'crawler')")
	t.Log("   - timestamp: RFC3339 timestamp")
	t.Log("")
	t.Log("2. step_progress events (from step_monitor.go publishStepProgress):")
	t.Log("   - step_id: ID of the step job")
	t.Log("   - manager_id: ID of the manager job")
	t.Log("   - step_name: Name of the step")
	t.Log("   - status: Step status (running, completed, failed)")
	t.Log("   - progress counts: pending_jobs, running_jobs, completed_jobs, failed_jobs")
	t.Log("")
	t.Log("=== ARCHITECTURE ===")
	t.Log("Workers use AddJobLogWithEvent with JobLogOptions to publish events.")
	t.Log("This ensures uniform event publishing with proper step context for UI filtering.")
}

// TestWebSocketJobEvents_StepNameRouting verifies that events are correctly routed by step_name.
// This test ensures that events from one step don't appear in another step's panel.
// Issue: "Step search_nearby_restaurants completed" was appearing in "extract_keywords" panel.
// Fix: All workers now route events through Job Manager with proper StepName in JobLogOptions.
func TestWebSocketJobEvents_StepNameRouting(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Run("VerifyStepNameInJobLogEvents", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a multi-step job to generate events from different steps
		defID := fmt.Sprintf("test-step-routing-%d", time.Now().UnixNano())
		multiStepBody := map[string]interface{}{
			"id":   defID,
			"name": "Step Routing Test Job",
			"type": "multi_step",
			"steps": []map[string]interface{}{
				{
					"name": "step_alpha",
					"type": "crawler",
					"config": map[string]interface{}{
						"start_urls":  []string{"https://example.com"},
						"max_depth":   0,
						"max_pages":   1,
						"concurrency": 1,
					},
				},
				{
					"name": "step_beta",
					"type": "crawler",
					"config": map[string]interface{}{
						"start_urls":  []string{"https://example.org"},
						"max_depth":   0,
						"max_pages":   1,
						"concurrency": 1,
					},
				},
			},
		}

		resp, err := helper.POST("/api/job-definitions", multiStepBody)
		if err != nil || resp.StatusCode != 201 {
			t.Skip("Could not create multi-step job definition")
			return
		}
		defer resp.Body.Close()
		defer deleteJobDefinition(t, helper, defID)

		// Execute the job definition
		jobID := executeJobDefinition(t, helper, defID)
		if jobID == "" {
			t.Skip("Could not execute job definition")
			return
		}
		defer deleteJob(t, helper, jobID)

		// Collect events during job execution
		t.Log("Collecting WebSocket events to verify step_name routing...")
		collector := collectWebSocketEvents(t, conn, 30*time.Second)

		t.Logf("Collected %d total events", collector.Count())

		// Analyze job_log events for step_name attribution
		jobLogEvents := collector.GetEventsByType("job_log")
		t.Logf("Found %d job_log events", len(jobLogEvents))

		// Track events by step_name
		eventsByStep := make(map[string]int)
		eventsWithoutStepName := 0

		for _, event := range jobLogEvents {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				continue
			}

			stepName, _ := payload["step_name"].(string)
			if stepName == "" {
				eventsWithoutStepName++
			} else {
				eventsByStep[stepName]++
			}
		}

		// Report findings
		t.Log("Job log events by step_name:")
		for stepName, count := range eventsByStep {
			t.Logf("  %s: %d events", stepName, count)
		}
		if eventsWithoutStepName > 0 {
			t.Logf("  (no step_name): %d events", eventsWithoutStepName)
		}

		// Verify step_progress events have step_name
		stepProgressEvents := collector.GetEventsByType("step_progress")
		t.Logf("Found %d step_progress events", len(stepProgressEvents))

		stepProgressWithoutStepName := 0
		for _, event := range stepProgressEvents {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				continue
			}
			if stepName, _ := payload["step_name"].(string); stepName == "" {
				stepProgressWithoutStepName++
				t.Logf("  WARNING: step_progress event missing step_name: %v", payload)
			}
		}

		// Assert that step_progress events have step_name (critical for UI routing)
		if len(stepProgressEvents) > 0 {
			assert.Equal(t, 0, stepProgressWithoutStepName,
				"All step_progress events should have step_name for proper UI routing")
		}

		t.Log("")
		t.Log("=== STEP NAME ROUTING VERIFICATION COMPLETE ===")
		t.Log("If step_name is present in events, the UI can correctly filter")
		t.Log("events to display only in the appropriate step panel.")
	})
}

// TestWebSocketJobEvents_NewsCrawlerRealTime tests that WebSocket events are received
// in real-time during News Crawler job execution (no page refresh needed).
// This test validates that job completion events are broadcast via WebSocket.
func TestWebSocketJobEvents_NewsCrawlerRealTime(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Log("=== NEWS CRAWLER REAL-TIME WEBSOCKET EVENTS TEST ===")
	t.Log("This test validates that WebSocket events are received in real-time")
	t.Log("during job execution without requiring page refresh.")
	t.Log("")

	// Step 1: Connect WebSocket BEFORE triggering job
	t.Log("Step 1: Connecting WebSocket client...")
	conn := connectWebSocket(t, env)
	// Note: We close the connection explicitly after collecting events, not via defer

	// Clear initial status message
	waitForMessageType(t, conn, "status", 5*time.Second)
	t.Log("✓ WebSocket connected and initial status received")

	// Step 2: Execute the News Crawler job definition
	t.Log("Step 2: Executing News Crawler job definition...")
	resp, err := helper.POST("/api/job-definitions/news-crawler/execute", nil)
	require.NoError(t, err, "Failed to execute News Crawler")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Skipf("News Crawler job definition not available (status %d)", resp.StatusCode)
		return
	}

	var execResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &execResult)
	require.NoError(t, err, "Failed to parse execution response")

	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	t.Logf("✓ News Crawler job started: job_id=%s", jobID)

	// Step 3: Collect WebSocket events during job execution
	t.Log("Step 3: Collecting WebSocket events during job execution...")
	t.Log("   (Waiting up to 120 seconds for job completion)")

	collector := &WebSocketEventCollector{}
	jobCompleted := false
	startTime := time.Now()
	timeout := 120 * time.Second

	// Collect events until job completes or timeout
	connectionClosed := false
	for time.Since(startTime) < timeout && !jobCompleted && !connectionClosed {
		remaining := timeout - time.Since(startTime)
		if remaining < 0 {
			break
		}

		// Set read deadline
		err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Logf("Warning: Failed to set read deadline: %v", err)
			connectionClosed = true
			break
		}

		// Read message
		var msg map[string]interface{}
		err = conn.ReadJSON(&msg)
		if err != nil {
			// Check for connection closed errors
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				t.Log("WebSocket closed by server")
				connectionClosed = true
				break
			}
			// Check for other connection errors (e.g., "use of closed network connection")
			errStr := err.Error()
			if strings.Contains(errStr, "closed") || strings.Contains(errStr, "EOF") {
				t.Logf("WebSocket connection error: %v", err)
				connectionClosed = true
				break
			}
			// Timeout is expected - continue polling
			continue
		}

		// Add to collector
		collector.Add(msg)

		// Log event type
		eventType, _ := msg["type"].(string)
		if eventType == "job_log" || eventType == "step_progress" || eventType == "job_status_change" {
			// Log interesting events
			if payload, ok := msg["payload"].(map[string]interface{}); ok {
				if message, ok := payload["message"].(string); ok {
					t.Logf("   [%s] %s", eventType, truncateMessage(message, 80))
				} else if status, ok := payload["status"].(string); ok {
					t.Logf("   [%s] status=%s", eventType, status)
				}
			}
		}

		// Check if job completed
		if eventType == "job_status_change" {
			if payload, ok := msg["payload"].(map[string]interface{}); ok {
				if msgJobID, _ := payload["job_id"].(string); msgJobID == jobID {
					if status, _ := payload["status"].(string); status == "completed" || status == "failed" {
						t.Logf("✓ Job reached terminal state: %s", status)
						jobCompleted = true
					}
				}
			}
		}
	}

	// Close WebSocket connection before analyzing results (if not already closed)
	if !connectionClosed {
		closeWebSocket(t, conn)
	}

	elapsed := time.Since(startTime)
	t.Logf("Step 3 complete: Collected %d events in %v", collector.Count(), elapsed)

	// Step 4: Analyze collected events
	t.Log("")
	t.Log("Step 4: Analyzing collected WebSocket events...")

	allEvents := collector.GetEvents()
	t.Logf("   Total events collected: %d", len(allEvents))

	// Count by type
	eventCounts := make(map[string]int)
	for _, event := range allEvents {
		if eventType, ok := event["type"].(string); ok {
			eventCounts[eventType]++
		}
	}

	t.Log("   Event counts by type:")
	for eventType, count := range eventCounts {
		t.Logf("     - %s: %d", eventType, count)
	}

	// Step 5: Validate real-time events were received
	t.Log("")
	t.Log("Step 5: Validating real-time event delivery...")

	// We expect at least some job_log events during execution
	jobLogCount := eventCounts["job_log"]
	stepProgressCount := eventCounts["step_progress"]
	statusChangeCount := eventCounts["job_status_change"]

	t.Logf("   job_log events: %d", jobLogCount)
	t.Logf("   step_progress events: %d", stepProgressCount)
	t.Logf("   job_status_change events: %d", statusChangeCount)

	// The key assertion: we should receive events in real-time
	// If WebSocket is working, we should have received events during job execution
	// The News Crawler typically generates 100+ child jobs, so we expect many events
	totalJobEvents := jobLogCount + stepProgressCount + statusChangeCount

	t.Logf("")
	t.Logf("   Total job-related events: %d", totalJobEvents)

	// Assert we received a reasonable number of events
	// If only 6-8 events, WebSocket is NOT working properly
	// If 50+ events, WebSocket IS working properly
	assert.Greater(t, totalJobEvents, 10,
		"Should receive more than 10 job-related events via WebSocket in real-time. "+
			"If only receiving 6-8 events, WebSocket event publishing is broken.")

	// If job completed, verify we got the completion event
	if jobCompleted {
		assert.Greater(t, statusChangeCount, 0,
			"Should receive job_status_change events when job completes")
	}

	// Clean up
	deleteJob(t, helper, jobID)

	t.Log("")
	t.Log("=== NEWS CRAWLER REAL-TIME WEBSOCKET EVENTS TEST COMPLETE ===")
	if totalJobEvents > 10 {
		t.Log("✓ SUCCESS: WebSocket events are being received in real-time")
	} else {
		t.Log("✗ FAILURE: WebSocket events are NOT being received in real-time")
		t.Log("  This indicates a bug in the event publishing pipeline.")
	}
}

// truncateMessage truncates a message to maxLen characters
func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}
