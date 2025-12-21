// job_test_generator_streaming_test.go - UI tests for test_job_generator.toml
// Tests real-time SSE log streaming requirements:
// 1. Logs appear in step panels while job is running (no page refresh)
// 2. SSE connection established and receiving events
// 3. Step logs update in real-time via SSE, not API polling

package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTestJobGeneratorSSEStreaming verifies real-time log streaming via SSE
// This test ensures logs appear while job is running, not just after completion
func TestTestJobGeneratorSSEStreaming(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Test Job Generator SSE Streaming ---")

	// Step 1: Trigger the job
	jobName := "Test Job Generator"
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	utc.Log("Job triggered: %s", jobName)

	// Step 2: Navigate to Queue page
	err := utc.Navigate(utc.QueueURL)
	require.NoError(t, err, "Failed to navigate to Queue page")
	utc.Screenshot("queue_page_initial")

	// Step 3: Wait for job to appear and expand
	time.Sleep(3 * time.Second)
	utc.Screenshot("queue_after_3s")

	// Step 4: Monitor log counts while job is running
	// Key assertion: logs should appear BEFORE job completes
	utc.Log("Monitoring real-time log streaming...")
	startTime := time.Now()
	maxWait := 90 * time.Second
	logsObservedDuringRun := false
	var lastLogCounts map[string]int
	sseConnected := false

	for time.Since(startTime) < maxWait {
		// Check if SSE is connected
		var connected bool
		chromedp.Run(utc.Ctx, chromedp.Evaluate(`
			(() => {
				const el = document.querySelector('[x-data="jobList"]');
				if (!el) return false;
				const component = Alpine.$data(el);
				if (!component) return false;
				// Check if any job has SSE connected
				for (const [jobId, connected] of Object.entries(component.jobSSEConnected || {})) {
					if (connected) return true;
				}
				return false;
			})()
		`, &connected))
		if connected && !sseConnected {
			utc.Log("✓ SSE connection established")
			sseConnected = true
		}

		// Get current job status
		var jobStatus string
		chromedp.Run(utc.Ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const statusBadge = card.querySelector('span.label[data-status]');
						if (statusBadge) return statusBadge.getAttribute('data-status');
					}
				}
				return '';
			})()
		`, jobName), &jobStatus))

		// Get log counts for expanded steps
		logCounts, err := getUIStepLogCountMap(utc)
		if err == nil && len(logCounts) > 0 {
			// Check if any step has logs
			totalLogs := 0
			for _, count := range logCounts {
				totalLogs += count
			}

			// Log progress periodically
			if lastLogCounts == nil || totalLogs != sumMapValues(lastLogCounts) {
				utc.Log("[%v] Job status: %s, Step log counts: %v, Total: %d",
					time.Since(startTime).Round(time.Second), jobStatus, logCounts, totalLogs)
				lastLogCounts = logCounts
			}

			// Key assertion: logs visible while job is still running
			if totalLogs > 0 && (jobStatus == "running" || jobStatus == "pending") {
				logsObservedDuringRun = true
				utc.Log("✓ SUCCESS: Logs observed while job status is '%s' (count: %d)", jobStatus, totalLogs)
				utc.Screenshot("logs_during_running")
			}
		}

		// Check for completion
		if jobStatus == "completed" || jobStatus == "failed" || jobStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", jobStatus)
			utc.Screenshot("job_completed")
			break
		}

		time.Sleep(2 * time.Second)
	}

	// Final assertions
	utc.Screenshot("final_state")

	// Verify SSE connection was established
	assert.True(t, sseConnected, "SSE connection should have been established")

	// Verify logs were observed during job execution (not just at end)
	assert.True(t, logsObservedDuringRun,
		"Logs should be visible while job is running (real-time streaming)")

	// Get final log counts
	finalLogCounts, err := getUIStepLogCountMap(utc)
	require.NoError(t, err, "Should be able to get final log counts")
	utc.Log("Final log counts: %v", finalLogCounts)

	// Verify at least one step has logs
	totalFinalLogs := sumMapValues(finalLogCounts)
	assert.Greater(t, totalFinalLogs, 0, "Should have logs after job completion")

	utc.Log("✓ Test completed - Real-time SSE streaming verified")
}

// TestTestJobGeneratorStepAutoExpand verifies steps auto-expand when they start running
func TestTestJobGeneratorStepAutoExpand(t *testing.T) {
	utc := NewUITestContext(t, 4*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Test Job Generator Step Auto-Expand ---")

	// Trigger job
	jobName := "Test Job Generator"
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Navigate to Queue page
	err := utc.Navigate(utc.QueueURL)
	require.NoError(t, err, "Failed to navigate to Queue page")

	// Wait for job to start
	time.Sleep(3 * time.Second)

	// Monitor for step auto-expansion
	utc.Log("Monitoring step auto-expansion...")
	startTime := time.Now()
	maxWait := 60 * time.Second
	expandedStepsObserved := make(map[string]bool)

	for time.Since(startTime) < maxWait {
		// Get expanded steps from DOM
		var expandedSteps []string
		chromedp.Run(utc.Ctx, chromedp.Evaluate(`
			(() => {
				const expanded = [];
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const logsSection = step.querySelector('.tree-step-logs');
					if (logsSection) {
						const nameEl = step.querySelector('.tree-step-name');
						if (nameEl) expanded.push(nameEl.textContent.trim());
					}
				}
				return expanded;
			})()
		`, &expandedSteps))

		// Record newly expanded steps
		for _, stepName := range expandedSteps {
			if !expandedStepsObserved[stepName] {
				utc.Log("✓ Step auto-expanded: %s (at %v)", stepName, time.Since(startTime).Round(time.Millisecond))
				expandedStepsObserved[stepName] = true
				utc.Screenshot(fmt.Sprintf("step_expanded_%s", sanitizeName(stepName)))
			}
		}

		// Check for job completion
		var jobStatus string
		chromedp.Run(utc.Ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const statusBadge = card.querySelector('span.label[data-status]');
						if (statusBadge) return statusBadge.getAttribute('data-status');
					}
				}
				return '';
			})()
		`, jobName), &jobStatus))

		if jobStatus == "completed" || jobStatus == "failed" || jobStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", jobStatus)
			break
		}

		time.Sleep(1 * time.Second)
	}

	utc.Screenshot("final_state")

	// Verify steps were auto-expanded
	assert.Greater(t, len(expandedStepsObserved), 0,
		"At least one step should have auto-expanded")

	utc.Log("Auto-expanded steps observed: %v", mapKeys(expandedStepsObserved))
	utc.Log("✓ Test completed - Step auto-expansion verified")
}

// TestLogLineNumbersAreServerProvided verifies that log line numbers come from the server
// and are sequential, matching the total count displayed in the UI.
// This catches bugs where line numbers are generated by UI (1,2,3...) instead of server values.
func TestLogLineNumbersAreServerProvided(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Log Line Numbers Are Server-Provided ---")

	// Step 1: Trigger a job that generates many logs
	jobName := "Test Job Generator"
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	utc.Log("Job triggered: %s", jobName)

	// Step 2: Navigate to Queue page
	err := utc.Navigate(utc.QueueURL)
	require.NoError(t, err, "Failed to navigate to Queue page")

	// Step 3: Wait for job to complete
	utc.Log("Waiting for job to complete...")
	startTime := time.Now()
	maxWait := 120 * time.Second

	for time.Since(startTime) < maxWait {
		var jobStatus string
		chromedp.Run(utc.Ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const cards = document.querySelectorAll('.card');
				for (const card of cards) {
					const titleEl = card.querySelector('.card-title');
					if (titleEl && titleEl.textContent.includes('%s')) {
						const statusBadge = card.querySelector('span.label[data-status]');
						if (statusBadge) return statusBadge.getAttribute('data-status');
					}
				}
				return '';
			})()
		`, jobName), &jobStatus))

		if jobStatus == "completed" || jobStatus == "failed" {
			utc.Log("Job reached terminal status: %s", jobStatus)
			break
		}
		time.Sleep(2 * time.Second)
	}

	utc.Screenshot("job_completed_for_line_numbers")

	// Step 4: Find a step with substantial logs (high_volume_generator has 1200+ logs)
	time.Sleep(2 * time.Second) // Allow UI to settle

	// Get step log data including line numbers and totals
	type StepLogData struct {
		StepName     string `json:"stepName"`
		LineNumbers  []int  `json:"lineNumbers"`
		DisplayedLog int    `json:"displayedLog"`
		TotalLog     int    `json:"totalLog"`
	}

	var stepData []StepLogData
	err = chromedp.Run(utc.Ctx, chromedp.Evaluate(`
		(() => {
			const results = [];
			const treeSteps = document.querySelectorAll('.tree-step');

			for (const step of treeSteps) {
				const nameEl = step.querySelector('.tree-step-name');
				if (!nameEl) continue;
				const stepName = nameEl.textContent.trim();

				// Get log count from badge (format: "logs: X/Y")
				const badgeEl = step.querySelector('.label.bg-secondary');
				let displayedLog = 0;
				let totalLog = 0;
				if (badgeEl) {
					const badgeText = badgeEl.textContent.trim();
					const match = badgeText.match(/logs:\s*(\d+)\/(\d+)/);
					if (match) {
						displayedLog = parseInt(match[1], 10);
						totalLog = parseInt(match[2], 10);
					}
				}

				// Get line numbers from displayed logs
				const lineNumbers = [];
				const logLines = step.querySelectorAll('.tree-log-line');
				for (const line of logLines) {
					const numEl = line.querySelector('.tree-log-num');
					if (numEl) {
						const num = parseInt(numEl.textContent.trim(), 10);
						if (!isNaN(num)) {
							lineNumbers.push(num);
						}
					}
				}

				if (lineNumbers.length > 0) {
					results.push({
						stepName: stepName,
						lineNumbers: lineNumbers,
						displayedLog: displayedLog,
						totalLog: totalLog
					});
				}
			}
			return results;
		})()
	`, &stepData))
	require.NoError(t, err, "Failed to get step log data from UI")

	utc.Log("Found %d steps with logs", len(stepData))
	require.Greater(t, len(stepData), 0, "Should have at least one step with logs")

	// Step 5: Verify line numbers for each step
	for _, step := range stepData {
		utc.Log("Checking step: %s (displayed: %d, total: %d, line numbers: %v)",
			step.StepName, step.DisplayedLog, step.TotalLog, step.LineNumbers[:min(5, len(step.LineNumbers))])

		if len(step.LineNumbers) == 0 {
			continue
		}

		// Assertion 1: Line numbers should be sequential (no gaps in displayed logs)
		for i := 1; i < len(step.LineNumbers); i++ {
			diff := step.LineNumbers[i] - step.LineNumbers[i-1]
			assert.Equal(t, 1, diff,
				"Line numbers should be sequential in step %s: %d -> %d (diff: %d)",
				step.StepName, step.LineNumbers[i-1], step.LineNumbers[i], diff)
		}

		// Assertion 2: If displaying last N logs of a larger set, line numbers should NOT start at 1
		// (This catches the bug where UI generates 1,2,3... instead of using server line_number)
		if step.TotalLog > step.DisplayedLog && step.DisplayedLog > 0 {
			firstLineNum := step.LineNumbers[0]
			lastLineNum := step.LineNumbers[len(step.LineNumbers)-1]

			// The first displayed line should be approximately (totalLog - displayedLog + 1)
			// Allow some margin for filtering, but it should NOT be 1 if we're showing latest logs
			expectedFirstLineApprox := step.TotalLog - step.DisplayedLog + 1

			utc.Log("  First line: %d, Last line: %d, Expected first ~%d",
				firstLineNum, lastLineNum, expectedFirstLineApprox)

			// If total > displayed, first line should NOT be 1 (that would mean UI is generating numbers)
			assert.NotEqual(t, 1, firstLineNum,
				"Step %s: First line number should NOT be 1 when showing last %d of %d logs "+
					"(line_number should come from server, not be UI-generated)",
				step.StepName, step.DisplayedLog, step.TotalLog)

			// Assertion 3: Last line number should be close to totalLog (within reasonable margin)
			// This verifies we're showing the LATEST logs with correct line numbers
			assert.InDelta(t, float64(step.TotalLog), float64(lastLineNum), float64(step.DisplayedLog),
				"Step %s: Last line number (%d) should be close to total (%d)",
				step.StepName, lastLineNum, step.TotalLog)
		}

		// Assertion 4: Displayed count should match actual line count
		assert.Equal(t, len(step.LineNumbers), step.DisplayedLog,
			"Step %s: Actual displayed log count (%d) should match badge count (%d)",
			step.StepName, len(step.LineNumbers), step.DisplayedLog)
	}

	utc.Screenshot("line_numbers_verified")
	utc.Log("✓ Test completed - Log line numbers verified")
}

// Helper functions

// sumMapValues sums all int values in a map
func sumMapValues(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// mapKeys returns all keys from a map
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
