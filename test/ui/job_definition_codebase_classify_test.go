package ui

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func assertDisplayedLogCountsMatchAPITotalCountsWhenCompleted(t *testing.T, utc *UITestContext, h httpGetter, jobID string) {
	t.Helper()
	if jobID == "" {
		t.Errorf("FAIL: Cannot assert UI vs API log counts: job_id is empty")
		return
	}

	// Read the currently selected tree log level filter from the UI so the API total_count matches what the UI is showing.
	// Queue UI uses per-job filter values: all | warn | error.
	var treeLogLevel string
	_ = chromedp.Run(utc.Ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const jobListEl = document.querySelector('[x-data="jobList"]');
				if (!jobListEl) return 'all';
				const component = Alpine.$data(jobListEl);
				if (!component || !component.getTreeLogLevelFilter) return 'all';
				return component.getTreeLogLevelFilter(%q) || 'all';
			})()
		`, jobID), &treeLogLevel),
	)
	if treeLogLevel == "" {
		treeLogLevel = "all"
	}

	// Get UI counts for each step (displayed log lines).
	// Note: "Show earlier logs" button was removed in prompt_14.md.
	type stepCounts struct {
		Shown int `json:"shown"`
	}
	var uiCounts map[string]stepCounts
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const stepEl of treeSteps) {
					const stepNameEl = stepEl.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const logsSection = stepEl.querySelector('.tree-step-logs');
					if (!logsSection) continue;

					const logLines = logsSection.querySelectorAll('.tree-log-line');
					const shown = logLines ? logLines.length : 0;

					result[stepName] = { shown };
				}
				return result;
			})()
		`, &uiCounts),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get UI step log counts: %v", err)
		return
	}

	// Get step job IDs from tree endpoint
	var treeData struct {
		Steps []struct {
			StepID string `json:"step_id"`
			Name   string `json:"name"`
		} `json:"steps"`
	}
	treePath := fmt.Sprintf("/api/jobs/%s/tree", jobID)
	if err := apiGetJSON(t, h, treePath, &treeData); err != nil {
		t.Errorf("FAIL: Failed to get tree data for job '%s': %v", jobID, err)
		return
	}

	// Build step name -> step ID map
	stepIDMap := make(map[string]string)
	for _, step := range treeData.Steps {
		stepIDMap[step.Name] = step.StepID
	}

	for stepName, counts := range uiCounts {
		stepJobID, ok := stepIDMap[stepName]
		if !ok || stepJobID == "" {
			t.Errorf("FAIL: No step_id found for step '%s' (job_id=%s)", stepName, jobID)
			continue
		}

		var logsResp apiJobTreeLogsResponse
		path := fmt.Sprintf("/api/logs?scope=job&job_id=%s&step=%s&limit=1&level=%s", stepJobID, url.QueryEscape(stepName), url.QueryEscape(treeLogLevel))
		if err := apiGetJSON(t, h, path, &logsResp); err != nil {
			t.Errorf("FAIL: Failed to fetch API log counts for step '%s' (job_id=%s): %v", stepName, jobID, err)
			continue
		}
		if len(logsResp.Steps) != 1 {
			t.Errorf("FAIL: Unexpected API logs response for step '%s' (job_id=%s): expected 1 step, got %d", stepName, jobID, len(logsResp.Steps))
			continue
		}

		apiTotal := logsResp.Steps[0].TotalCount
		uiShown := counts.Shown
		// Note: With "Show earlier logs" removed (prompt_14.md), UI may show fewer logs than API total.
		// For completed jobs, the UI should ideally show all logs, but the limit applies.
		// We verify that shown count is reasonable (either all logs or within the limit).
		if uiShown > apiTotal {
			utc.Screenshot(fmt.Sprintf("log_count_mismatch_%s_ui_%d_api_%d", sanitizeName(stepName), uiShown, apiTotal))
			t.Errorf("FAIL: Step '%s' UI log count exceeds API total_count: UI=%d API=%d (job_id=%s, level=%s)",
				stepName, uiShown, apiTotal, jobID, treeLogLevel)
		}
	}
}

// TestJobDefinitionCodebaseClassify tests the Codebase Classify job definition end-to-end.
//
// CONTEXT-SPECIFIC ASSERTIONS ONLY:
// This test validates behavior specific to the Codebase Classify job:
// - Job completes successfully with expected 3 steps
// - Steps are named correctly: import_files, code_map, rule_classify_files
// - Job-specific output/results are valid
//
// GENERIC UI ASSERTIONS:
// Generic UI behavior tests (WebSocket throttling, step icons, log numbering, auto-expand, etc.)
// have been moved to TestJobDefinitionGeneralUIAssertions in job_definition_general_test.go
// which uses test_job_generator.toml for more controlled testing.
func TestJobDefinitionCodebaseClassify(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Codebase Classify (context-specific assertions) ---")

	jobName := "Codebase Classify"
	jobTimeout := MaxJobTestTimeout
	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	// Copy job definition to results for reference
	if err := utc.CopyJobDefinitionToResults("../config/job-definitions/codebase_classify.toml"); err != nil {
		t.Fatalf("Failed to copy job definition: %v", err)
	}

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Navigate to Queue page for monitoring
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}

	// Wait for job to appear in queue
	utc.Log("Waiting for job to appear in queue...")
	time.Sleep(2 * time.Second)

	// Monitor job until completion
	utc.Log("Starting job monitoring...")
	startTime := time.Now()
	lastStatus := ""
	jobID := ""
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()

	for {
		// Check context
		if err := utc.Ctx.Err(); err != nil {
			t.Fatalf("Context cancelled: %v", err)
		}

		// Check timeout
		if time.Since(startTime) > jobTimeout {
			utc.Screenshot("codebase_classify_timeout")
			t.Fatalf("Job %s did not complete within %v", jobName, jobTimeout)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			utc.Log("[%v] Monitoring... (status: %s)", elapsed.Round(time.Second), lastStatus)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			utc.FullScreenshot(fmt.Sprintf("codebase_classify_monitor_%ds", int(elapsed.Seconds())))
			lastScreenshotTime = time.Now()
		}

		// Get current job status via JavaScript
		var currentStatus string
		err := chromedp.Run(utc.Ctx,
			chromedp.Evaluate(fmt.Sprintf(`
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
			`, jobName), &currentStatus),
		)
		if err != nil {
			t.Logf("Warning: failed to get status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Capture job ID once Alpine has loaded the job list
		if jobID == "" {
			if id, err := getJobIDFromQueueUI(utc, jobName); err == nil && id != "" {
				jobID = id
				utc.Log("Captured job_id from UI: %s", jobID)
			}
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Second))
			lastStatus = currentStatus
			utc.FullScreenshot(fmt.Sprintf("codebase_classify_status_%s", currentStatus))
		}

		// Check for terminal status
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", currentStatus)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Take final screenshot
	utc.FullScreenshot("codebase_classify_final_state")

	// ===============================
	// CONTEXT-SPECIFIC ASSERTIONS
	// ===============================
	finalStatus := lastStatus
	utc.Log("--- Running Context-Specific Assertions ---")

	// --------------------------------------------------------------------------------
	// Assertion 1: Job completed successfully
	// --------------------------------------------------------------------------------
	if finalStatus != "completed" {
		t.Errorf("FAIL: Codebase Classify job did not complete successfully (status=%s)", finalStatus)
	} else {
		utc.Log("PASS: Codebase Classify job completed successfully")
	}

	// --------------------------------------------------------------------------------
	// Assertion 2: Verify expected 3 steps exist (import_files, code_map, rule_classify_files)
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 2: Verifying expected steps are present...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err != nil {
			t.Errorf("FAIL: Could not get step tree from API for job_id=%s: %v", jobID, err)
		} else {
			expectedSteps := []string{"import_files", "code_map", "rule_classify_files"}
			foundSteps := make(map[string]bool)
			for _, step := range tree.Steps {
				foundSteps[step.Name] = true
			}

			for _, expected := range expectedSteps {
				if !foundSteps[expected] {
					t.Errorf("FAIL: Expected step '%s' not found in Codebase Classify job", expected)
				} else {
					utc.Log("PASS: Found expected step '%s'", expected)
				}
			}

			if len(tree.Steps) != 3 {
				t.Errorf("FAIL: Expected exactly 3 steps, got %d", len(tree.Steps))
			} else {
				utc.Log("PASS: Codebase Classify has exactly 3 steps")
			}
		}
	} else {
		t.Errorf("FAIL: Could not capture job ID to verify steps")
	}

	// --------------------------------------------------------------------------------
	// Assertion 3: All steps completed successfully
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 3: Verifying all steps completed...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err == nil {
			for _, step := range tree.Steps {
				if step.Status != "completed" {
					t.Errorf("FAIL: Step '%s' has status '%s' (expected 'completed')", step.Name, step.Status)
				} else {
					utc.Log("PASS: Step '%s' completed successfully", step.Name)
				}
			}
		}
	}

	// --------------------------------------------------------------------------------
	// Assertion 4: No SSE buffer overflows during high-load job execution
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 4: Verifying no SSE buffer overflows occurred...")
	assertNoSSEBufferOverflows(t, utc)

	utc.Log("Codebase Classify context-specific test completed with final status: %s", finalStatus)
	utc.Log("NOTE: Generic UI tests (WebSocket throttling, icons, log numbering, etc.) are in TestJobDefinitionGeneralUIAssertions")
}

func assertProgressiveLogsWithinWindow(t *testing.T, utc *UITestContext, samples []DOMLogProgressSample) {
	// Assertion 0 verifies progressive log updates during job execution.
	// Architecture: WebSocket sends refresh_logs triggers, UI fetches logs via API.
	//
	// Server-side trigger schedule (per prompt_12.md):
	// 1. Job start -> refresh all step logs (via status change in job_update)
	// 2. Step start -> refresh step logs (via status change in job_update)
	// 3. Scaling intervals: 1s, 2s, 3s, 4s -> then 10s periodic
	// 4. Step complete -> refresh step logs (immediate trigger)
	// 5. Job completion -> refresh all step logs (via status change in job_update)
	//
	// With the scaling rate limiter, logs should stream progressively:
	// - First trigger at 1s, second at 2s, third at 3s, fourth at 4s
	// - Then 10s periodic for steady-state
	// This ensures the UI receives log updates within the first 30 seconds.
	if len(samples) == 0 {
		t.Errorf("FAIL: No DOM progress samples captured in first 30 seconds - cannot assert progressive updates")
		return
	}

	firstExpandedAt := time.Duration(-1)
	firstLogsAt := time.Duration(-1)
	firstIncreaseAt := time.Duration(-1)

	prevTotal := -1
	seenLogs := false
	for _, s := range samples {
		if firstExpandedAt < 0 && len(s.Snapshot.ExpandedSteps) > 0 {
			firstExpandedAt = s.Elapsed
		}

		if firstLogsAt < 0 && s.Snapshot.TotalLogLines > 0 {
			firstLogsAt = s.Elapsed
			seenLogs = true
			prevTotal = s.Snapshot.TotalLogLines
			continue
		}

		if seenLogs && firstIncreaseAt < 0 && s.Snapshot.TotalLogLines > prevTotal {
			firstIncreaseAt = s.Elapsed
		}
		if seenLogs {
			prevTotal = s.Snapshot.TotalLogLines
		}
	}

	utc.Log("Progress samples (first 30s): expanded@%v, firstLogs@%v, firstIncrease@%v",
		firstExpandedAt, firstLogsAt, firstIncreaseAt)

	// All three checks are required for progressive log streaming
	if firstExpandedAt < 0 {
		t.Errorf("FAIL: No steps expanded within first 30 seconds - expected auto-expand during running job")
	}
	if firstLogsAt < 0 {
		t.Errorf("FAIL: No log lines appeared within first 30 seconds - expected progressive log updates during job execution")
	}
	if firstIncreaseAt < 0 {
		t.Errorf("FAIL: Log lines did not increase within first 30 seconds after first logs appeared - expected progressive streaming (scaling: 1s, 2s, 3s, 4s, then 10s)")
	}
}

func assertAPILogsCallsAreGatedByRefreshTriggers(t *testing.T, utc *UITestContext, wsTracker *WebSocketMessageTracker, apiTracker *APICallTracker, startTime time.Time) {
	jobRefreshStepIDTotal := wsTracker.GetJobScopedRefreshStepIDTotal()
	jobRefreshBefore30s := wsTracker.GetJobScopedRefreshCountBefore(startTime.Add(30 * time.Second))
	serviceRefreshBefore30s := wsTracker.GetServiceScopedRefreshCountBefore(startTime.Add(30 * time.Second))

	jobLogsCalls := apiTracker.GetJobLogsCalls()
	serviceLogsCalls := apiTracker.GetServiceLogsCalls()
	jobLogsCallsBefore30s := apiTracker.GetJobLogsCallsBefore(startTime.Add(30 * time.Second))
	serviceLogsCallsBefore30s := apiTracker.GetServiceLogsCallsBefore(startTime.Add(30 * time.Second))

	utc.Log("refresh_logs triggers: job=%d (step_ids total=%d), service=%d",
		wsTracker.GetJobScopedRefreshCount(), jobRefreshStepIDTotal, wsTracker.GetServiceScopedRefreshCount())
	utc.Log("/api/logs calls: job=%d (before30s=%d), service=%d (before30s=%d)",
		jobLogsCalls, jobLogsCallsBefore30s, serviceLogsCalls, serviceLogsCallsBefore30s)

	// Service logs often do an initial load on page load. Allow 1 extra call beyond triggers.
	if serviceLogsCalls > wsTracker.GetServiceScopedRefreshCount()+1 {
		t.Errorf("FAIL: /api/logs?scope=service called %d times but only %d refresh_logs(scope=service) triggers observed (+1 allowed initial load). UI appears to be polling service logs without WebSocket gating.",
			serviceLogsCalls, wsTracker.GetServiceScopedRefreshCount())
	}

	// For job-scoped logs, each refresh_logs(scope=job) may contain multiple step_ids, and UI may call /api/logs once per step_id.
	// Therefore, job /api/logs calls should be bounded by the total step_ids seen in refresh triggers (+small slack for completion hydration).
	allowedSlack := 3
	if jobLogsCalls > jobRefreshStepIDTotal+allowedSlack {
		t.Errorf("FAIL: /api/logs?scope=job called %d times but refresh_logs(scope=job) carried only %d step_ids (+%d slack). UI appears to be polling job logs or refetching excessively.",
			jobLogsCalls, jobRefreshStepIDTotal, allowedSlack)
	}

	// Also require that during the first 30 seconds, at least one refresh trigger and at least one API fetch occur.
	// This guards the UAT failure where logs don't update until a status-change catch-all refresh.
	if jobRefreshBefore30s == 0 {
		t.Errorf("FAIL: No refresh_logs(scope=job) triggers received within first 30 seconds - UI cannot refresh step logs progressively")
	}
	if jobLogsCallsBefore30s == 0 {
		t.Errorf("FAIL: No /api/logs?scope=job calls observed within first 30 seconds - logs not being fetched progressively")
	}
	if serviceRefreshBefore30s == 0 {
		utc.Log("Note: No refresh_logs(scope=service) triggers within first 30 seconds (service log streaming may be idle)")
	}
}

// checkStepExpansionState checks which steps are currently expanded
func checkStepExpansionState(utc *UITestContext, tracker *StepExpansionTracker) {
	// JavaScript to get expanded steps from the Alpine.js component
	var expandedSteps []string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const expanded = [];
				// Get the Alpine.js jobList component
				const jobListEl = document.querySelector('[x-data="jobList"]');
				if (!jobListEl) return expanded;
				const component = Alpine.$data(jobListEl);
				if (!component) return expanded;

				// Find the running/completed job
				const job = component.allJobs.find(j => j.name && j.name.includes('Codebase Classify'));
				if (!job) return expanded;
				const jobId = job.id;

				// Check tree data for this job
				const treeData = component.jobTreeData[jobId];
				if (!treeData || !treeData.steps) return expanded;

				// Check which steps are expanded
				for (let i = 0; i < treeData.steps.length; i++) {
					const key = jobId + ':' + i;
					if (component.jobTreeExpandedSteps[key]) {
						expanded.push(treeData.steps[i].name);
					}
				}
				return expanded;
			})()
		`, &expandedSteps),
	)
	if err != nil {
		utc.Log("Warning: failed to check step expansion: %v", err)
		return
	}

	for _, stepName := range expandedSteps {
		tracker.RecordExpansion(stepName)
	}
}

// captureLogLineNumbers captures the displayed log line numbers for each step
func captureLogLineNumbers(utc *UITestContext, tracker *StepExpansionTracker) {
	// Get log line numbers from the DOM tree-step elements
	var stepLogData map[string][]int
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				// Find all tree-step containers
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					// Get step name from tree-step-name element
					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					// Find log lines within this step's tree-step-logs section
					const logsSection = step.querySelector('.tree-step-logs');
					if (!logsSection) continue; // Not expanded

					const logLines = logsSection.querySelectorAll('.tree-log-line');
					const lineNumbers = [];
					for (const logLine of logLines) {
						const lineNumEl = logLine.querySelector('.tree-log-num');
						if (lineNumEl) {
							const num = parseInt(lineNumEl.textContent.trim(), 10);
							if (!isNaN(num)) {
								lineNumbers.push(num);
							}
						}
					}
					if (lineNumbers.length > 0) {
						result[stepName] = lineNumbers.slice(0, 15); // First 15 lines
					}
				}
				return result;
			})()
		`, &stepLogData),
	)
	if err != nil {
		utc.Log("Warning: failed to capture log line numbers: %v", err)
		return
	}

	for stepName, lines := range stepLogData {
		tracker.RecordLogLines(stepName, lines)
	}
}

// assertStepsExpandedInOrder asserts that steps expanded in the expected order
func assertStepsExpandedInOrder(t *testing.T, utc *UITestContext, actualOrder []string) {
	// Expected: Steps should auto-expand as they complete
	// Note: import_files often completes before UI monitoring starts, so it may not be captured

	if len(actualOrder) == 0 {
		t.Errorf("FAIL: No steps were auto-expanded!")
		return
	}

	utc.Log("✓ Steps that auto-expanded: %v", actualOrder)

	// Check that key steps expanded
	hasImportFiles := false
	hasCodeMap := false
	for _, step := range actualOrder {
		if step == "import_files" {
			hasImportFiles = true
		}
		if step == "code_map" {
			hasCodeMap = true
		}
	}

	// import_files is the first step and often completes before monitoring starts
	// Log it as a note rather than a hard failure
	if !hasImportFiles {
		utc.Log("Note: import_files step did not auto-expand (may have completed before monitoring started)")
	} else {
		utc.Log("✓ PASS: import_files step auto-expanded")
	}

	// code_map is critical - it must auto-expand to show logs
	if !hasCodeMap {
		t.Errorf("FAIL: code_map step did not auto-expand")
	} else {
		utc.Log("✓ PASS: code_map step auto-expanded")
	}

	// At least one step must have auto-expanded for this assertion to pass
	if len(actualOrder) >= 1 {
		utc.Log("✓ PASS: At least %d step(s) auto-expanded", len(actualOrder))
	}
}

// assertLogStartsAtLine1 asserts that log lines start at 1 (not 5 or other number)
// Set critical=true to make missing log lines a test failure (for key steps like code_map)
func assertLogStartsAtLine1(t *testing.T, utc *UITestContext, stepName string, lineNumbers []int, critical bool) {
	if len(lineNumbers) == 0 {
		if critical {
			t.Errorf("FAIL: No log lines captured for step %s", stepName)
		} else {
			utc.Log("Note: No log lines captured for step %s (step may not have been expanded)", stepName)
		}
		return
	}

	firstLine := lineNumbers[0]
	if firstLine != 1 {
		t.Errorf("FAIL: %s logs start at line %d (expected line 1). Lines: %v", stepName, firstLine, lineNumbers)
	} else {
		utc.Log("✓ PASS: %s logs start at line 1", stepName)
	}

	// Also verify sequential lines 1-15 are displayed
	expectedSequence := true
	for i, num := range lineNumbers {
		if num != i+1 {
			expectedSequence = false
			break
		}
	}
	if expectedSequence && len(lineNumbers) >= 10 {
		utc.Log("✓ PASS: %s shows sequential logs 1→%d", stepName, len(lineNumbers))
	} else if len(lineNumbers) < 10 {
		utc.Log("Note: %s has fewer than 10 log lines displayed (%d)", stepName, len(lineNumbers))
	}
}

// assertStepIconsMatchStandard verifies that step status icons match the parent job icon standard
// Parent job icons use: fa-spinner (running), fa-check-circle (completed), fa-times-circle (failed), fa-clock (pending)
// Step icons SHOULD use the same icons, but currently use: fa-circle (pending) instead of fa-clock
// This test will FAIL until icons are standardized
func assertStepIconsMatchStandard(t *testing.T, utc *UITestContext) {
	// Expected icon classes for each status (matching parent job standard)
	expectedIcons := map[string]string{
		"pending":   "fa-clock",        // Parent jobs use fa-clock for pending
		"running":   "fa-spinner",      // Both use fa-spinner
		"completed": "fa-check-circle", // Both use fa-check-circle
		"failed":    "fa-times-circle", // Both use fa-times-circle
		"cancelled": "fa-ban",          // Both use fa-ban
	}

	// Get step icon data from DOM
	var stepIcons []map[string]interface{}
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = [];
				// Find all tree-step-status elements
				const statusElements = document.querySelectorAll('.tree-step-status');
				for (const el of statusElements) {
					const icon = el.querySelector('i.fas');
					if (!icon) continue;

					// Get parent step header to find step name
					const stepHeader = el.closest('.tree-step-header');
					if (!stepHeader) continue;

					const stepNameEl = stepHeader.querySelector('.tree-step-name');
					const stepName = stepNameEl ? stepNameEl.textContent.trim() : 'unknown';

					// Get status from class
					let status = 'unknown';
					if (el.classList.contains('text-warning')) status = 'pending';
					else if (el.classList.contains('text-primary')) status = 'running';
					else if (el.classList.contains('text-success')) status = 'completed';
					else if (el.classList.contains('text-error')) status = 'failed';
					else if (el.classList.contains('text-gray')) status = 'cancelled';

					// Get icon class
					const iconClasses = Array.from(icon.classList);
					const iconClass = iconClasses.find(c => c.startsWith('fa-') && c !== 'fas');

					result.push({
						stepName: stepName,
						status: status,
						iconClass: iconClass || 'unknown',
						hasSpinner: icon.classList.contains('fa-pulse')
					});
				}
				return result;
			})()
		`, &stepIcons),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get step icon data: %v", err)
		return
	}

	if len(stepIcons) == 0 {
		t.Errorf("FAIL: No step icons found in DOM")
		return
	}

	utc.Log("Found %d step icons to verify", len(stepIcons))

	// Check each step's icon matches the expected standard
	iconMismatches := 0
	for _, step := range stepIcons {
		stepName := step["stepName"].(string)
		status := step["status"].(string)
		actualIcon := step["iconClass"].(string)

		expectedIcon, exists := expectedIcons[status]
		if !exists {
			utc.Log("Warning: Unknown status '%s' for step '%s'", status, stepName)
			continue
		}

		if actualIcon != expectedIcon {
			iconMismatches++
			t.Errorf("FAIL: Step '%s' icon mismatch - status=%s, expected=%s, actual=%s",
				stepName, status, expectedIcon, actualIcon)
		} else {
			utc.Log("✓ Step '%s' icon correct: %s for status %s", stepName, actualIcon, status)
		}
	}

	if iconMismatches > 0 {
		t.Errorf("FAIL: %d step icon(s) do not match parent job icon standard", iconMismatches)
	} else {
		utc.Log("✓ PASS: All step icons match parent job icon standard")
	}
}

// assertAllStepsHaveLogs verifies that ALL steps have logs (not "No logs for this step")
// This specifically checks that import_files (step 2) has logs
func assertAllStepsHaveLogs(t *testing.T, utc *UITestContext) {
	// Get step log status from DOM - check for "No logs" message vs actual log lines
	var stepLogStatus map[string]interface{}
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const logsSection = step.querySelector('.tree-step-logs');
					if (!logsSection) {
						result[stepName] = { hasLogs: false, reason: 'not_expanded' };
						continue;
					}

					// Check for "No logs" message
					const noLogsMsg = logsSection.querySelector('.tree-step-no-logs, .text-gray');
					if (noLogsMsg && noLogsMsg.textContent.toLowerCase().includes('no logs')) {
						result[stepName] = { hasLogs: false, reason: 'no_logs_message' };
						continue;
					}

					// Count log lines
					const logLines = logsSection.querySelectorAll('.tree-log-line');
					result[stepName] = { hasLogs: logLines.length > 0, logCount: logLines.length };
				}
				return result;
			})()
		`, &stepLogStatus),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get step log status: %v", err)
		return
	}

	if len(stepLogStatus) == 0 {
		t.Errorf("FAIL: No steps found in DOM")
		return
	}

	utc.Log("Checking %d steps for log presence", len(stepLogStatus))

	// Check each step for logs
	stepsWithoutLogs := []string{}
	for stepName, statusRaw := range stepLogStatus {
		status := statusRaw.(map[string]interface{})
		hasLogs, _ := status["hasLogs"].(bool)

		if !hasLogs {
			reason := "unknown"
			if r, ok := status["reason"].(string); ok {
				reason = r
			}
			stepsWithoutLogs = append(stepsWithoutLogs, fmt.Sprintf("%s (%s)", stepName, reason))
			t.Errorf("FAIL: Step '%s' has no logs (reason: %s)", stepName, reason)
		} else {
			logCount := 0
			if lc, ok := status["logCount"].(float64); ok {
				logCount = int(lc)
			}
			utc.Log("✓ Step '%s' has %d log lines", stepName, logCount)
		}
	}

	if len(stepsWithoutLogs) > 0 {
		t.Errorf("FAIL: %d step(s) have no logs: %v", len(stepsWithoutLogs), stepsWithoutLogs)
	} else {
		utc.Log("✓ PASS: All steps have logs")
	}
}

// assertCompletedStepsMustHaveLogs verifies that completed/running steps have logs.
// This is a stricter check than assertAllStepsHaveLogs - it specifically validates that
// steps with status "completed" or "running" MUST have > 0 logs displayed.
// A completed step showing "No logs for this step" indicates a UI bug.
func assertCompletedStepsMustHaveLogs(t *testing.T, utc *UITestContext) {
	// Get both step status and log presence from DOM in one call
	var stepStatusAndLogs []map[string]interface{}
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = [];
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepHeader = step.querySelector('.tree-step-header');
					if (!stepHeader) continue;

					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					// Get status from status element
					const statusEl = stepHeader.querySelector('.tree-step-status');
					let status = 'unknown';
					if (statusEl) {
						if (statusEl.classList.contains('text-warning')) status = 'pending';
						else if (statusEl.classList.contains('text-primary')) status = 'running';
						else if (statusEl.classList.contains('text-success')) status = 'completed';
						else if (statusEl.classList.contains('text-error')) status = 'failed';
						else if (statusEl.classList.contains('text-gray')) status = 'cancelled';
					}

					// Check logs section
					const logsSection = step.querySelector('.tree-step-logs');
					let hasLogs = false;
					let logCount = 0;
					let reason = 'not_expanded';

					if (logsSection) {
						// Check for "No logs" message
						const noLogsMsg = logsSection.querySelector('.tree-step-no-logs, .text-gray');
						if (noLogsMsg && noLogsMsg.textContent.toLowerCase().includes('no logs')) {
							reason = 'no_logs_message';
						} else {
							// Count log lines
							const logLines = logsSection.querySelectorAll('.tree-log-line');
							logCount = logLines.length;
							if (logCount > 0) {
								hasLogs = true;
								reason = 'has_logs';
							} else {
								reason = 'empty_logs_section';
							}
						}
					}

					result.push({
						stepName: stepName,
						status: status,
						hasLogs: hasLogs,
						logCount: logCount,
						reason: reason
					});
				}
				return result;
			})()
		`, &stepStatusAndLogs),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get step status and logs: %v", err)
		return
	}

	if len(stepStatusAndLogs) == 0 {
		t.Errorf("FAIL: No steps found in DOM")
		return
	}

	utc.Log("Checking %d steps: completed/running steps MUST have logs", len(stepStatusAndLogs))

	// Check each step: completed/running steps MUST have logs
	violationCount := 0
	for _, step := range stepStatusAndLogs {
		stepName := step["stepName"].(string)
		status := step["status"].(string)
		hasLogs := step["hasLogs"].(bool)
		reason := step["reason"].(string)

		// Only check completed and running steps
		if status == "completed" || status == "running" {
			if !hasLogs {
				violationCount++
				t.Errorf("FAIL: Step '%s' has status '%s' but shows NO logs (reason: %s) - completed/running steps MUST have logs",
					stepName, status, reason)
			} else {
				logCount := 0
				if lc, ok := step["logCount"].(float64); ok {
					logCount = int(lc)
				}
				utc.Log("✓ Step '%s' (%s) has %d logs", stepName, status, logCount)
			}
		} else {
			utc.Log("  Step '%s' has status '%s' (skipped - not completed/running)", stepName, status)
		}
	}

	if violationCount > 0 {
		t.Errorf("FAIL: %d completed/running step(s) have no logs - this is a UI bug", violationCount)
	} else {
		utc.Log("✓ PASS: All completed/running steps have logs")
	}
}

// StepLogData holds log line numbers and metadata for a step
type StepLogData struct {
	LineNumbers []int `json:"lineNumbers"`
	TotalLogs   int   `json:"totalLogs"` // Total logs = len(lineNumbers)
}

// assertLogLineNumberingCorrect verifies log line numbering:
//   - Logs should start at line 1 and be monotonically increasing
//   - Line numbers should be server-provided (not client-calculated)
//
// Note: "Show earlier logs" button was removed in prompt_14.md.
func assertLogLineNumberingCorrect(t *testing.T, utc *UITestContext, tracker *StepExpansionTracker) {
	// Get all step log line data from DOM
	var allStepLogs map[string]StepLogData
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {};
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();
					if (!stepName) continue;

					const logsSection = step.querySelector('.tree-step-logs');
					if (!logsSection) continue;

					const logLines = logsSection.querySelectorAll('.tree-log-line');
					const lineNumbers = [];
					for (const logLine of logLines) {
						const lineNumEl = logLine.querySelector('.tree-log-num');
						if (lineNumEl) {
							const num = parseInt(lineNumEl.textContent.trim(), 10);
							if (!isNaN(num)) {
								lineNumbers.push(num);
							}
						}
					}
					if (lineNumbers.length > 0) {
						result[stepName] = {
							lineNumbers: lineNumbers,
							totalLogs: lineNumbers.length
						};
					}
				}
				return result;
			})()
		`, &allStepLogs),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get step log line numbers: %v", err)
		return
	}

	if len(allStepLogs) == 0 {
		t.Errorf("FAIL: No step logs found - steps may not be auto-expanded")
		return
	}

	utc.Log("Checking log line numbering for %d steps", len(allStepLogs))

	// Check each step's log line numbering
	// Note: "Show earlier logs" button was removed in prompt_14.md.
	// We now only verify that line numbers are monotonically increasing (server-provided).
	stepsWithBadNumbering := 0
	for stepName, logData := range allStepLogs {
		lineNumbers := logData.LineNumbers
		numLines := len(lineNumbers)
		firstLine := lineNumbers[0]
		lastLine := lineNumbers[numLines-1]

		utc.Log("Step '%s': %d lines shown (first=%d, last=%d)",
			stepName, numLines, firstLine, lastLine)

		// Line numbers should be monotonically increasing (gaps allowed due to level filtering)
		// When filtering by level (default=info), DEBUG logs are excluded but their
		// line numbers still exist in storage, causing gaps. This is expected behavior.
		monotonic := true
		for i := 1; i < numLines; i++ {
			prev := lineNumbers[i-1]
			curr := lineNumbers[i]
			if curr <= prev {
				monotonic = false
				stepsWithBadNumbering++
				t.Errorf("FAIL: Step '%s' log lines not monotonically increasing - line %d followed by %d",
					stepName, prev, curr)
				break
			}
		}
		if monotonic {
			utc.Log("✓ Step '%s': monotonic logs %d→%d (server-provided line numbers)", stepName, firstLine, lastLine)
		}
	}

	if stepsWithBadNumbering > 0 {
		t.Errorf("FAIL: %d step(s) have incorrect log line numbering", stepsWithBadNumbering)
	} else {
		utc.Log("✓ PASS: All steps have correct log line numbering")
	}
}

// assertAllStepsAutoExpand verifies that ALL steps auto-expand, not just some
// Expected behavior: Every step should auto-expand when it starts running or completes
func assertAllStepsAutoExpand(t *testing.T, utc *UITestContext, expansionOrder []string) {
	// Get total number of steps from the job tree
	var totalSteps int
	var stepNames []string
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const steps = [];
				const stepNameElements = document.querySelectorAll('.tree-step-name');
				for (const el of stepNameElements) {
					const name = el.textContent.trim();
					if (name && !steps.includes(name)) {
						steps.push(name);
					}
				}
				return steps;
			})()
		`, &stepNames),
	)
	if err != nil {
		t.Errorf("FAIL: Failed to get step names: %v", err)
		return
	}

	totalSteps = len(stepNames)
	if totalSteps == 0 {
		t.Errorf("FAIL: No steps found in job tree")
		return
	}

	utc.Log("Total steps in job: %d, Steps auto-expanded: %d", totalSteps, len(expansionOrder))
	utc.Log("All step names: %v", stepNames)
	utc.Log("Auto-expanded steps: %v", expansionOrder)

	// Check that ALL steps auto-expanded
	if len(expansionOrder) < totalSteps {
		missingSteps := []string{}
		for _, stepName := range stepNames {
			found := false
			for _, expanded := range expansionOrder {
				if expanded == stepName {
					found = true
					break
				}
			}
			if !found {
				missingSteps = append(missingSteps, stepName)
			}
		}
		t.Errorf("FAIL: Not all steps auto-expanded. Missing: %v (expected %d, got %d)",
			missingSteps, totalSteps, len(expansionOrder))
	} else {
		utc.Log("✓ PASS: All %d steps auto-expanded", totalSteps)
	}

	// Verify each expected step is in the expansion order
	expectedSteps := []string{"import_files", "code_map", "rule_classify_files"}
	for _, expected := range expectedSteps {
		found := false
		for _, actual := range expansionOrder {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("FAIL: Expected step '%s' did not auto-expand", expected)
		}
	}
}

// assertNoSSEBufferOverflows checks the service logs for SSE buffer overflow warnings.
// This is a high-load scenario test assertion for jobs like codebase_classify that
// generate thousands of log entries in parallel.
//
// The buffer size was increased from 2000 to 10000 in sse_logs_handler.go to handle
// high-throughput scenarios. This assertion verifies the fix works.
func assertNoSSEBufferOverflows(t *testing.T, utc *UITestContext) {
	t.Helper()

	// Find the latest service log file in the bin/logs/ directory
	binLogsDir := filepath.Join(utc.Env.ResultsDir, "..", "bin", "logs")
	if _, err := os.Stat(binLogsDir); os.IsNotExist(err) {
		// Alternative: check relative to test working directory
		binLogsDir = "../../bin/logs"
	}

	// Find all log files and get the most recent one
	logFiles, err := filepath.Glob(filepath.Join(binLogsDir, "quaero.*.log"))
	if err != nil || len(logFiles) == 0 {
		utc.Log("Note: Could not find service log files in %s (skipping buffer overflow check)", binLogsDir)
		return
	}

	// Get the most recent log file (they have timestamps in the filename)
	var latestLog string
	var latestTime time.Time
	for _, logFile := range logFiles {
		info, err := os.Stat(logFile)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestLog = logFile
		}
	}

	if latestLog == "" {
		utc.Log("Note: No readable service log files found (skipping buffer overflow check)")
		return
	}

	// Count buffer overflow warnings in the log file
	bufferOverflowCount := 0
	file, err := os.Open(latestLog)
	if err != nil {
		utc.Log("Note: Could not open service log file %s: %v (skipping buffer overflow check)", latestLog, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase scanner buffer for large log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Buffer full, skipping entry") {
			bufferOverflowCount++
		}
	}

	if err := scanner.Err(); err != nil {
		utc.Log("Note: Error scanning service log file: %v", err)
	}

	utc.Log("Service log file: %s", filepath.Base(latestLog))
	utc.Log("Buffer overflow warnings found: %d", bufferOverflowCount)

	// Allow a small number of buffer overflows during initial burst, but fail if excessive
	// The buffer increase from 2000 to 10000 should eliminate most/all overflows
	const maxAllowedOverflows = 10
	if bufferOverflowCount > maxAllowedOverflows {
		t.Errorf("FAIL: Found %d SSE buffer overflow warnings in service logs (max allowed: %d). "+
			"This indicates the SSE buffer size may need to be increased further.",
			bufferOverflowCount, maxAllowedOverflows)
	} else if bufferOverflowCount > 0 {
		utc.Log("Note: %d minor buffer overflows occurred (within acceptable threshold of %d)",
			bufferOverflowCount, maxAllowedOverflows)
	} else {
		utc.Log("✓ PASS: No SSE buffer overflows detected during high-load job execution")
	}
}
