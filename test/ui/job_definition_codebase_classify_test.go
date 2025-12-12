package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// WebSocketMessageTracker tracks WebSocket messages by type
type WebSocketMessageTracker struct {
	mu                        sync.Mutex
	refreshLogsMessages       []map[string]interface{} // All refresh_logs messages
	jobScopedRefreshCount     int                      // Count of job-scoped refresh_logs
	serviceScopedRefreshCount int                      // Count of service-scoped refresh_logs
}

// NewWebSocketMessageTracker creates a new WebSocket message tracker
func NewWebSocketMessageTracker() *WebSocketMessageTracker {
	return &WebSocketMessageTracker{
		refreshLogsMessages: make([]map[string]interface{}, 0),
	}
}

// AddMessage records a WebSocket message
func (t *WebSocketMessageTracker) AddMessage(msgType string, payload map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if msgType == "refresh_logs" {
		t.refreshLogsMessages = append(t.refreshLogsMessages, payload)
		scope, _ := payload["scope"].(string)
		if scope == "job" {
			t.jobScopedRefreshCount++
		} else if scope == "service" {
			t.serviceScopedRefreshCount++
		}
	}
}

// GetRefreshLogsCount returns the total count of refresh_logs messages
func (t *WebSocketMessageTracker) GetRefreshLogsCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.refreshLogsMessages)
}

// GetJobScopedRefreshCount returns count of job-scoped refresh_logs messages
func (t *WebSocketMessageTracker) GetJobScopedRefreshCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.jobScopedRefreshCount
}

// GetServiceScopedRefreshCount returns count of service-scoped refresh_logs messages
func (t *WebSocketMessageTracker) GetServiceScopedRefreshCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.serviceScopedRefreshCount
}

// StepExpansionTracker tracks step expansion order and log line numbers
type StepExpansionTracker struct {
	mu             sync.Mutex
	expansionOrder []string         // Order steps were expanded
	expandedSteps  map[string]bool  // Currently expanded steps
	stepLogLines   map[string][]int // Step name -> first few log line numbers
}

// NewStepExpansionTracker creates a new step expansion tracker
func NewStepExpansionTracker() *StepExpansionTracker {
	return &StepExpansionTracker{
		expansionOrder: make([]string, 0),
		expandedSteps:  make(map[string]bool),
		stepLogLines:   make(map[string][]int),
	}
}

// RecordExpansion records a step expansion
func (t *StepExpansionTracker) RecordExpansion(stepName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.expandedSteps[stepName] {
		t.expandedSteps[stepName] = true
		t.expansionOrder = append(t.expansionOrder, stepName)
	}
}

// RecordLogLines records log line numbers for a step
func (t *StepExpansionTracker) RecordLogLines(stepName string, lines []int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.stepLogLines[stepName]) == 0 {
		t.stepLogLines[stepName] = lines
	}
}

// GetExpansionOrder returns the order in which steps were expanded
func (t *StepExpansionTracker) GetExpansionOrder() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]string, len(t.expansionOrder))
	copy(result, t.expansionOrder)
	return result
}

// GetLogLines returns the recorded log lines for a step
func (t *StepExpansionTracker) GetLogLines(stepName string) []int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stepLogLines[stepName]
}

// StepIconData tracks icon class and status for each step
type StepIconData struct {
	StepName   string
	Status     string
	IconClass  string
	HasSpinner bool
}

// ParentJobIconData tracks icon class for parent job
type ParentJobIconData struct {
	JobName   string
	IconClass string
}

// TestJobDefinitionCodebaseClassify tests the Codebase Classify job definition end-to-end
// with detailed assertions for:
// - WebSocket refresh_logs messages < 20 (server-side throttling)
// - Steps auto-expand in completion order
// - Log lines start at 1 (not 5) and increment sequentially for steps with < 100 logs
// - For steps with > 100 logs, only latest 100 are shown (ordered by latest at bottom)
// - Step icons match parent job icon standard
// - All steps auto-expand and have logs (including step 2: import_files)
func TestJobDefinitionCodebaseClassify(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Codebase Classify (with assertions) ---")

	jobName := "Codebase Classify"
	jobTimeout := MaxJobTestTimeout

	// Copy job definition to results for reference
	if err := utc.CopyJobDefinitionToResults("../config/job-definitions/codebase_classify.toml"); err != nil {
		t.Fatalf("Failed to copy job definition: %v", err)
	}

	// Create trackers
	wsTracker := NewWebSocketMessageTracker()
	expansionTracker := NewStepExpansionTracker()

	// Enable network tracking via Chrome DevTools Protocol
	// Track both HTTP API calls and WebSocket frames
	utc.Log("Enabling network and WebSocket frame tracking...")
	chromedp.ListenTarget(utc.Ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventWebSocketFrameReceived:
			// Parse WebSocket frame payload for refresh_logs messages
			payloadData := e.Response.PayloadData
			if strings.Contains(payloadData, "refresh_logs") {
				var msg struct {
					Type    string                 `json:"type"`
					Payload map[string]interface{} `json:"payload"`
				}
				if err := json.Unmarshal([]byte(payloadData), &msg); err == nil {
					if msg.Type == "refresh_logs" {
						wsTracker.AddMessage(msg.Type, msg.Payload)
					}
				}
			}
		}
	})

	// Enable network domain (includes WebSocket frame tracking)
	if err := chromedp.Run(utc.Ctx, network.Enable()); err != nil {
		t.Fatalf("Failed to enable network tracking: %v", err)
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

	// Monitor job WITHOUT page refresh - using WebSocket updates
	utc.Log("Starting job monitoring (NO page refresh)...")
	startTime := time.Now()
	lastStatus := ""
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()
	lastExpansionCheck := time.Now()

	for {
		// Check context
		if err := utc.Ctx.Err(); err != nil {
			t.Fatalf("Context cancelled: %v", err)
		}

		// Check timeout
		if time.Since(startTime) > jobTimeout {
			utc.Screenshot("job_timeout_" + sanitizeName(jobName))
			t.Fatalf("Job %s did not complete within %v", jobName, jobTimeout)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			wsMsgs := wsTracker.GetRefreshLogsCount()
			utc.Log("[%v] Monitoring... (status: %s, WebSocket refresh_logs: %d)",
				elapsed.Round(time.Second), lastStatus, wsMsgs)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= 30*time.Second {
			elapsed := time.Since(startTime)
			utc.FullScreenshot(fmt.Sprintf("monitor_%s_%ds", sanitizeName(jobName), int(elapsed.Seconds())))
			lastScreenshotTime = time.Now()
		}

		// Check step expansion state every 2 seconds (via JavaScript)
		if time.Since(lastExpansionCheck) >= 2*time.Second {
			checkStepExpansionState(utc, expansionTracker)
			lastExpansionCheck = time.Now()
		}

		// Get current job status via JavaScript (NO page refresh)
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

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Second))
			lastStatus = currentStatus
			utc.FullScreenshot(fmt.Sprintf("status_%s_%s", sanitizeName(jobName), currentStatus))
		}

		// Check for terminal status
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("✓ Job reached terminal status: %s", currentStatus)
			break
		}

		// Short sleep - relying on WebSocket updates, not polling
		time.Sleep(500 * time.Millisecond)
	}

	// Final expansion check and log line capture
	time.Sleep(1 * time.Second) // Let final updates settle
	checkStepExpansionState(utc, expansionTracker)
	captureLogLineNumbers(utc, expansionTracker)

	// Take final screenshot
	utc.FullScreenshot("final_state_" + sanitizeName(jobName))

	// ===============================
	// ASSERTIONS
	// ===============================
	utc.Log("--- Running Assertions ---")

	// Assertion 1: WebSocket refresh_logs messages < 20
	// This tests server-side throttling/debouncing of log refresh triggers
	totalRefreshLogs := wsTracker.GetRefreshLogsCount()
	jobRefreshLogs := wsTracker.GetJobScopedRefreshCount()
	serviceRefreshLogs := wsTracker.GetServiceScopedRefreshCount()
	utc.Log("Assertion 1: WebSocket refresh_logs messages = %d (job: %d, service: %d, max allowed: 20)",
		totalRefreshLogs, jobRefreshLogs, serviceRefreshLogs)
	if totalRefreshLogs >= 20 {
		t.Errorf("FAIL: WebSocket refresh_logs message count %d >= 20 (expected < 20). Too many WebSocket messages - server-side throttling not working!", totalRefreshLogs)
	} else {
		utc.Log("✓ PASS: WebSocket refresh_logs messages within limit")
	}

	// Assertion 2: Step icons match parent job icon standard
	utc.Log("Assertion 2: Checking step icons match parent job icon standard...")
	assertStepIconsMatchStandard(t, utc)

	// Assertion 3: ALL steps have logs (including import_files)
	utc.Log("Assertion 3: Checking all steps have logs (not 'No logs for this step')...")
	assertAllStepsHaveLogs(t, utc)

	// Assertion 4: Log line numbering is correct
	// - Steps with < 100 logs: sequential 1→N
	// - Steps with > 100 logs: only latest 100 shown, ordered by latest at bottom
	utc.Log("Assertion 4: Checking log line numbering for all steps...")
	assertLogLineNumberingCorrect(t, utc, expansionTracker)

	// Assertion 5: ALL steps auto-expand (not just some)
	expansionOrder := expansionTracker.GetExpansionOrder()
	utc.Log("Assertion 5: Step expansion order = %v", expansionOrder)
	assertAllStepsAutoExpand(t, utc, expansionOrder)

	utc.Log("✓ Codebase Classify job definition test completed with all assertions")
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

// assertLogLineNumberingCorrect verifies log line numbering:
// - Steps with < 100 logs: sequential 1→N starting at line 1
// - Steps with > 100 logs: only latest 100 shown, ordered by latest at bottom
func assertLogLineNumberingCorrect(t *testing.T, utc *UITestContext, tracker *StepExpansionTracker) {
	// Get all step log line data from DOM
	var allStepLogs map[string][]int
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
						result[stepName] = lineNumbers;
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
	stepsWithBadNumbering := 0
	for stepName, lineNumbers := range allStepLogs {
		numLines := len(lineNumbers)
		firstLine := lineNumbers[0]
		lastLine := lineNumbers[numLines-1]

		utc.Log("Step '%s': %d lines shown (first=%d, last=%d)", stepName, numLines, firstLine, lastLine)

		// Case 1: Steps with < 100 logs should start at 1 and be sequential
		if numLines < 100 {
			// First line should be 1
			if firstLine != 1 {
				stepsWithBadNumbering++
				t.Errorf("FAIL: Step '%s' has %d logs but does NOT start at line 1 (starts at %d)",
					stepName, numLines, firstLine)
				continue
			}

			// Lines should be sequential (1, 2, 3, ...)
			sequential := true
			for i := 1; i < numLines; i++ {
				expected := lineNumbers[i-1] + 1
				actual := lineNumbers[i]
				if actual != expected {
					sequential = false
					stepsWithBadNumbering++
					t.Errorf("FAIL: Step '%s' log lines not sequential - expected %d after %d, got %d",
						stepName, expected, lineNumbers[i-1], actual)
					break
				}
			}
			if sequential {
				utc.Log("✓ Step '%s': sequential logs 1→%d", stepName, numLines)
			}
		} else {
			// Case 2: Steps with >= 100 logs should show only latest 100
			// Line numbers should be sequential but NOT start at 1
			// They should be ordered with latest at bottom (ascending order)

			// Check that exactly 100 lines are shown
			if numLines != 100 {
				utc.Log("Note: Step '%s' has %d lines (expected 100 for large log sets)", stepName, numLines)
			}

			// Check that lines are in ascending order (oldest at top, newest at bottom)
			ascending := true
			for i := 1; i < numLines; i++ {
				if lineNumbers[i] <= lineNumbers[i-1] {
					ascending = false
					stepsWithBadNumbering++
					t.Errorf("FAIL: Step '%s' logs not in ascending order - line %d came after %d",
						stepName, lineNumbers[i], lineNumbers[i-1])
					break
				}
			}

			// Check that lines are sequential (consecutive numbers)
			sequential := true
			for i := 1; i < numLines; i++ {
				expected := lineNumbers[i-1] + 1
				actual := lineNumbers[i]
				if actual != expected {
					sequential = false
					stepsWithBadNumbering++
					t.Errorf("FAIL: Step '%s' log lines not sequential - expected %d after %d, got %d",
						stepName, expected, lineNumbers[i-1], actual)
					break
				}
			}

			if ascending && sequential {
				utc.Log("✓ Step '%s': latest 100 logs shown (%d→%d)", stepName, firstLine, lastLine)
			}
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
