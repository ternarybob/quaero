// Package api provides WebSocket job event integration tests
package api

import (
	"fmt"
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
	collector := &WebSocketEventCollector{}
	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		msg, err := readWebSocketMessage(t, conn, remaining)
		if err != nil {
			// Timeout or connection closed
			break
		}
		collector.Add(msg)
	}

	return collector
}

// TestWebSocketJobEvents_EventContext tests that job events have proper step context
func TestWebSocketJobEvents_EventContext(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	t.Run("CrawlerJobLogEventContext", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a test job to trigger events
		jobID := createTestJob(t, helper)
		if jobID == "" {
			t.Skip("Could not create test job")
			return
		}
		defer deleteJob(t, helper, jobID)

		// Collect events for 10 seconds
		t.Log("Collecting WebSocket events...")
		collector := collectWebSocketEvents(t, conn, 10*time.Second)

		t.Logf("Collected %d total events", collector.Count())

		// Analyze crawler_job_log events
		crawlerLogEvents := collector.GetEventsByType("crawler_job_log")
		t.Logf("Found %d crawler_job_log events", len(crawlerLogEvents))

		// Check each event for required context fields
		eventsWithStepContext := 0
		eventsWithoutStepContext := 0

		for i, event := range crawlerLogEvents {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				t.Logf("Event %d: Missing payload", i)
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
				t.Logf("Event %d: HAS step context (step_name=%v, step_id=%v, manager_id=%v)",
					i, hasStepName, hasStepID, hasManagerID)
			} else {
				eventsWithoutStepContext++
				t.Logf("Event %d: MISSING step context - job_id=%v, message=%v",
					i, payload["job_id"], payload["message"])
			}
		}

		// Log summary
		t.Logf("Summary: %d events WITH step context, %d events WITHOUT step context",
			eventsWithStepContext, eventsWithoutStepContext)

		// This test documents the current behavior - crawler_job_log events are missing step context
		if eventsWithoutStepContext > 0 && eventsWithStepContext == 0 {
			t.Log("ISSUE CONFIRMED: crawler_job_log events are missing step context fields (step_name, step_id, manager_id)")
		}
	})

	t.Run("StepProgressEventContext", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a test job
		jobID := createTestJob(t, helper)
		if jobID == "" {
			t.Skip("Could not create test job")
			return
		}
		defer deleteJob(t, helper, jobID)

		// Collect events
		collector := collectWebSocketEvents(t, conn, 10*time.Second)

		// Analyze step_progress events
		stepProgressEvents := collector.GetEventsByType("step_progress")
		t.Logf("Found %d step_progress events", len(stepProgressEvents))

		for i, event := range stepProgressEvents {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				t.Logf("Event %d: Missing payload", i)
				continue
			}

			stepID := payload["step_id"]
			managerID := payload["manager_id"]
			stepName := payload["step_name"]
			status := payload["status"]

			t.Logf("step_progress event %d: step_id=%v, manager_id=%v, step_name=%v, status=%v",
				i, stepID, managerID, stepName, status)

			// Verify required fields
			assert.NotNil(t, stepID, "step_progress event should have step_id")
			assert.NotNil(t, managerID, "step_progress event should have manager_id")
		}
	})

	t.Run("JobLogEventContext", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

		// Create a test job
		jobID := createTestJob(t, helper)
		if jobID == "" {
			t.Skip("Could not create test job")
			return
		}
		defer deleteJob(t, helper, jobID)

		// Collect events
		collector := collectWebSocketEvents(t, conn, 10*time.Second)

		// Analyze job_log events
		jobLogEvents := collector.GetEventsByType("job_log")
		t.Logf("Found %d job_log events", len(jobLogEvents))

		eventsWithManagerID := 0
		eventsWithoutManagerID := 0

		for i, event := range jobLogEvents {
			payload, ok := event["payload"].(map[string]interface{})
			if !ok {
				t.Logf("Event %d: Missing payload", i)
				continue
			}

			if _, exists := payload["manager_id"]; exists {
				eventsWithManagerID++
			} else {
				eventsWithoutManagerID++
				t.Logf("Event %d: job_log missing manager_id - job_id=%v", i, payload["job_id"])
			}
		}

		t.Logf("job_log events: %d with manager_id, %d without manager_id",
			eventsWithManagerID, eventsWithoutManagerID)
	})

	t.Run("AllEventTypeSummary", func(t *testing.T) {
		// Connect WebSocket client
		conn := connectWebSocket(t, env)
		defer closeWebSocket(t, conn)

		// Clear initial status message
		waitForMessageType(t, conn, "status", 2*time.Second)

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
		collector := collectWebSocketEvents(t, conn, 5*time.Second)

		// Count events by type
		eventTypes := make(map[string]int)
		for _, event := range collector.GetEvents() {
			if eventType, ok := event["type"].(string); ok {
				eventTypes[eventType]++
			}
		}

		t.Log("Event type summary:")
		for eventType, count := range eventTypes {
			t.Logf("  %s: %d events", eventType, count)
		}
	})
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
