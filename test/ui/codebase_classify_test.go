package ui

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// TestCodebaseClassify_StepLogsExpansion
// Tests that step logs and tree view expand in real-time without page refresh
// =============================================================================

// TestCodebaseClassify_StepLogsExpansion tests the live log expansion for codebase_classify job
func TestCodebaseClassify_StepLogsExpansion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}

	ctc, cleanup := newClassifyTestContext(t, 10*time.Minute)
	defer cleanup()

	ctc.env.LogTest(t, "--- Starting Test: Codebase Classify Step Logs Expansion ---")

	// Load the codebase_classify job definition
	if err := ctc.loadJobDefinition(); err != nil {
		t.Fatalf("Failed to load codebase_classify definition: %v", err)
	}

	// Navigate to Jobs page and trigger the job
	jobsURL := ctc.baseURL + "/jobs"
	if err := chromedp.Run(ctc.ctx,
		chromedp.Navigate(jobsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}
	ctc.takeScreenshot("01_jobs_page_before_trigger")

	// Trigger the codebase_classify job via API
	jobID, err := ctc.triggerJob()
	if err != nil {
		t.Fatalf("Failed to trigger codebase_classify: %v", err)
	}
	ctc.env.LogTest(t, "Job triggered: %s", jobID)

	// Navigate to Queue page to monitor - DO NOT refresh after this
	queueURL := ctc.baseURL + "/queue"
	if err := chromedp.Run(ctc.ctx,
		chromedp.Navigate(queueURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	ctc.takeScreenshot("02_queue_after_trigger")

	// Capture console logs for debugging
	chromedp.ListenTarget(ctc.ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			args := make([]string, len(ev.Args))
			for i, arg := range ev.Args {
				args[i] = string(arg.Value)
			}
			msg := strings.Join(args, " ")
			if strings.Contains(msg, "[Queue]") || ev.Type == runtime.APITypeError {
				ctc.env.LogTest(t, "[CONSOLE] %s: %s", ev.Type, msg)
			}
		}
	})

	// Monitor job WITHOUT refreshing page - verify dynamic updates
	ctc.env.LogTest(t, "Monitoring job dynamically (NO REFRESH) to verify step/log expansion...")

	timeout := 5 * time.Minute
	startTime := time.Now()
	lastScreenshotTime := startTime
	screenshotNum := 3

	var treeExpanded bool
	var logsFound bool
	var dynamicLogsSeen bool
	var allStepsExpanded bool // Track if ALL steps with logs are expanded
	var prevLogCount int
	var finalStatus string
	var maxExpandedSteps int // Track max expanded steps seen

	// NEW: Track step status updates and log scrolling
	var stepStatusChanged bool                     // Track if any step status changed during execution
	var logScrollingSeen bool                      // Track if logs are scrolling (showing new logs beyond initial 100)
	var prevFirstLogNum int                        // Track first log line number to detect scrolling
	stepStatusHistory := make(map[string][]string) // Track status changes per step
	var maxFirstLogNum int                         // Track highest first log number seen (indicates scrolling)

	for time.Since(startTime) < timeout {
		// Take periodic screenshots every 20 seconds
		if time.Since(lastScreenshotTime) > 20*time.Second {
			ctc.takeScreenshot(fmt.Sprintf("%02d_progress_%ds", screenshotNum, int(time.Since(startTime).Seconds())))
			screenshotNum++
			lastScreenshotTime = time.Now()
		}

		// Evaluate page state - check for tree view, logs, step status, and log scrolling
		var result map[string]interface{}
		err := chromedp.Run(ctc.ctx,
			chromedp.Evaluate(`
				(() => {
					// Find the job card (Codebase Classify)
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const title = card.querySelector('.card-title');
						if (title && title.textContent.includes('Codebase Classify')) {
							// Get status
							const statusBadge = card.querySelector('.label[data-status]');
							const status = statusBadge ? statusBadge.getAttribute('data-status') : 'unknown';

							// Check for tree view visibility
							const treeView = card.querySelector('.inline-tree-view');
							const hasTree = !!treeView && treeView.offsetParent !== null;

							// Count visible log lines
							const logs = card.querySelectorAll('.tree-log-line');
							const visibleLogs = Array.from(logs).filter(l => l.offsetParent !== null).length;

							// Count step entries and track which are expanded, WITH STATUS
							const steps = card.querySelectorAll('.tree-step');
							const stepDetails = [];
							let expandedCount = 0;
							let stepsWithLogs = 0;

							steps.forEach((step, idx) => {
								// Check if step header has down chevron (expanded) or right chevron (collapsed)
								const chevron = step.querySelector('.tree-step-header i.fas');
								const isExpanded = chevron && chevron.classList.contains('fa-chevron-down');

								// Get step name from header
								const nameEl = step.querySelector('.tree-step-header span[style*="flex: 1"]');
								const stepName = nameEl ? nameEl.textContent.trim() : 'step-' + idx;

								// Get step status from badge (look for label with status classes)
								const stepBadge = step.querySelector('.tree-step-header .label');
								let stepStatus = 'unknown';
								if (stepBadge) {
									// Check for status attribute or class
									stepStatus = stepBadge.getAttribute('data-status') ||
										(stepBadge.classList.contains('label-success') ? 'completed' :
										stepBadge.classList.contains('label-primary') ? 'running' :
										stepBadge.classList.contains('label-warning') ? 'pending' :
										stepBadge.classList.contains('label-error') ? 'failed' : 'unknown');
								}

								// Count logs visible in this step's section AND get first log line number
								const logLines = step.querySelectorAll('.tree-log-line');
								const visibleStepLogs = Array.from(logLines).filter(l => l.offsetParent !== null).length;

								// Get first log line number to detect scrolling
								let firstLogNum = 0;
								let lastLogNum = 0;
								if (logLines.length > 0) {
									const firstLine = logLines[0];
									const lastLine = logLines[logLines.length - 1];
									// Look for line number in the log line (usually first element)
									const firstNumEl = firstLine.querySelector('.tree-log-num, [class*="log-num"]');
									const lastNumEl = lastLine.querySelector('.tree-log-num, [class*="log-num"]');
									if (firstNumEl) {
										firstLogNum = parseInt(firstNumEl.textContent.trim()) || 0;
									}
									if (lastNumEl) {
										lastLogNum = parseInt(lastNumEl.textContent.trim()) || 0;
									}
									// Fallback: try to parse from text content
									if (firstLogNum === 0) {
										const match = firstLine.textContent.match(/^\s*(\d+)/);
										if (match) firstLogNum = parseInt(match[1]);
									}
									if (lastLogNum === 0) {
										const match = lastLine.textContent.match(/^\s*(\d+)/);
										if (match) lastLogNum = parseInt(match[1]);
									}
								}

								if (isExpanded) expandedCount++;
								if (visibleStepLogs > 0 || isExpanded) stepsWithLogs++;

								stepDetails.push({
									name: stepName,
									expanded: isExpanded,
									logCount: visibleStepLogs,
									status: stepStatus,
									firstLogNum: firstLogNum,
									lastLogNum: lastLogNum
								});
							});

							return {
								found: true,
								status: status,
								hasTree: hasTree,
								logCount: visibleLogs,
								stepCount: steps.length,
								expandedStepCount: expandedCount,
								stepsWithLogs: stepsWithLogs,
								stepDetails: stepDetails,
								allStepsExpanded: expandedCount === steps.length && steps.length > 0
							};
						}
					}
					return { found: false, status: 'not found' };
				})()
			`, &result),
		)

		if err != nil {
			ctc.env.LogTest(t, "Error evaluating page: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		found := result["found"] == true
		if !found {
			ctc.env.LogTest(t, "Job card not found yet, waiting...")
			time.Sleep(1 * time.Second)
			continue
		}

		status := fmt.Sprintf("%v", result["status"])
		hasTree := result["hasTree"] == true
		logCount := 0
		stepCount := 0
		expandedStepCount := 0
		if val, ok := result["logCount"].(float64); ok {
			logCount = int(val)
		}
		if val, ok := result["stepCount"].(float64); ok {
			stepCount = int(val)
		}
		if val, ok := result["expandedStepCount"].(float64); ok {
			expandedStepCount = int(val)
		}

		// Track tree expansion
		if hasTree {
			treeExpanded = true
		}

		// Track log presence
		if logCount > 0 {
			logsFound = true
			if prevLogCount > 0 && logCount > prevLogCount {
				dynamicLogsSeen = true
				ctc.env.LogTest(t, "Dynamic log update detected: %d -> %d logs", prevLogCount, logCount)
			}
			prevLogCount = logCount
		}

		// Track step expansion - all steps should expand as they receive logs
		if expandedStepCount > maxExpandedSteps {
			maxExpandedSteps = expandedStepCount
		}
		if result["allStepsExpanded"] == true && stepCount > 0 {
			allStepsExpanded = true
		}

		// Log step details for debugging AND track status changes + log scrolling
		if stepDetails, ok := result["stepDetails"].([]interface{}); ok && len(stepDetails) > 0 {
			var stepInfo []string
			for _, sd := range stepDetails {
				if detail, ok := sd.(map[string]interface{}); ok {
					name := fmt.Sprintf("%v", detail["name"])
					expanded := detail["expanded"] == true
					logs := 0
					if v, ok := detail["logCount"].(float64); ok {
						logs = int(v)
					}
					stepStatus := fmt.Sprintf("%v", detail["status"])
					firstLogNum := 0
					lastLogNum := 0
					if v, ok := detail["firstLogNum"].(float64); ok {
						firstLogNum = int(v)
					}
					if v, ok := detail["lastLogNum"].(float64); ok {
						lastLogNum = int(v)
					}

					// Track step status changes
					history := stepStatusHistory[name]
					if len(history) == 0 || history[len(history)-1] != stepStatus {
						stepStatusHistory[name] = append(history, stepStatus)
						if len(stepStatusHistory[name]) > 1 {
							stepStatusChanged = true
							ctc.env.LogTest(t, "Step status changed: %s -> %s (for %s)", history[len(history)-1], stepStatus, name)
						}
					}

					// Track log scrolling - if first log number > 1, logs are scrolling
					if firstLogNum > 1 && firstLogNum > prevFirstLogNum {
						logScrollingSeen = true
						ctc.env.LogTest(t, "Log scrolling detected: first log now at line %d (was %d) for %s", firstLogNum, prevFirstLogNum, name)
					}
					if firstLogNum > maxFirstLogNum {
						maxFirstLogNum = firstLogNum
						prevFirstLogNum = firstLogNum
					}

					state := "collapsed"
					if expanded {
						state = "EXPANDED"
					}
					stepInfo = append(stepInfo, fmt.Sprintf("%s(%s,%s,logs=%d,lines=%d-%d)", name, state, stepStatus, logs, firstLogNum, lastLogNum))
				}
			}
			ctc.env.LogTest(t, "Steps: %s", strings.Join(stepInfo, " | "))
		}

		ctc.env.LogTest(t, "Status: %s | Tree: %v | Steps: %d/%d expanded | Logs: %d", status, hasTree, expandedStepCount, stepCount, logCount)

		// Check for terminal state
		if status == "completed" || status == "failed" || status == "cancelled" {
			finalStatus = status
			ctc.env.LogTest(t, "Job reached terminal state: %s", status)
			break
		}

		// Fail early if tree not expanded within 30s while job is running
		if time.Since(startTime) > 30*time.Second && status == "running" && !treeExpanded {
			ctc.takeScreenshot("tree_not_expanded_failure")
			t.Fatalf("Job is running but tree view did not auto-expand within 30s")
		}

		time.Sleep(2 * time.Second)
	}

	// Final screenshot
	ctc.takeScreenshot(fmt.Sprintf("%02d_final_state", screenshotNum))

	// Assertions
	if finalStatus == "" {
		t.Fatalf("Job timed out without reaching terminal state")
	}

	if !treeExpanded {
		t.Errorf("FAIL: Inline Tree View never expanded (should expand without refresh)")
	} else {
		ctc.env.LogTest(t, "✓ Inline Tree View auto-expanded")
	}

	if !logsFound {
		t.Errorf("FAIL: No step logs were ever displayed (should show without refresh)")
	} else {
		ctc.env.LogTest(t, "✓ Logs appeared in Tree View")
	}

	if !dynamicLogsSeen {
		t.Errorf("FAIL: No dynamic log updates detected (logs remained static at %d)", prevLogCount)
	} else {
		ctc.env.LogTest(t, "✓ Dynamic log updates verified")
	}

	// KEY ASSERTION: All steps should be expanded (not just the first one)
	if !allStepsExpanded {
		ctc.takeScreenshot("steps_not_all_expanded_failure")
		t.Errorf("FAIL: Not all steps expanded - max expanded: %d (all steps should auto-expand their logs)", maxExpandedSteps)
	} else {
		ctc.env.LogTest(t, "✓ All steps auto-expanded with logs")
	}

	// NEW ASSERTION: Step status should update during execution
	if !stepStatusChanged {
		ctc.takeScreenshot("step_status_not_changing")
		// Log the status history for debugging
		for stepName, history := range stepStatusHistory {
			ctc.env.LogTest(t, "Step '%s' status history: %v", stepName, history)
		}
		t.Errorf("FAIL: Step status never changed during execution (should transition from running to completed)")
	} else {
		ctc.env.LogTest(t, "✓ Step status updates verified")
	}

	// NEW ASSERTION: Logs should scroll (show latest 100, not first 100)
	if !logScrollingSeen && maxFirstLogNum <= 1 {
		ctc.takeScreenshot("logs_not_scrolling")
		ctc.env.LogTest(t, "Max first log number seen: %d (should be >1 if logs are scrolling)", maxFirstLogNum)
		t.Errorf("FAIL: Logs are not scrolling - stuck at first logs (max first line: %d, should show latest 100)", maxFirstLogNum)
	} else {
		ctc.env.LogTest(t, "✓ Log scrolling verified (first line reached: %d)", maxFirstLogNum)
	}

	ctc.env.LogTest(t, "Job reached terminal status: %s", finalStatus)

	if finalStatus == "failed" {
		ctc.takeScreenshot("job_failed_final")
		ctc.env.LogTest(t, "⚠ Job failed - check logs for details")
	} else if finalStatus == "completed" {
		ctc.env.LogTest(t, "✓ Job completed successfully")
	}

	ctc.env.LogTest(t, "✓ Test completed - step logs expanded without page refresh")
}

// =============================================================================
// Test Context and Helper Functions
// =============================================================================

// classifyTestContext holds shared state for codebase classify tests
type classifyTestContext struct {
	t             *testing.T
	env           *common.TestEnvironment
	ctx           context.Context
	baseURL       string
	helper        *common.HTTPTestHelper
	screenshotNum int
}

// newClassifyTestContext creates a new test context with browser and environment
func newClassifyTestContext(t *testing.T, timeout time.Duration) (*classifyTestContext, func()) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	baseURL := env.GetBaseURL()

	ctc := &classifyTestContext{
		t:       t,
		env:     env,
		ctx:     browserCtx,
		baseURL: baseURL,
		helper:  env.NewHTTPTestHelperWithTimeout(t, 5*time.Minute),
	}

	cleanup := func() {
		if err := chromedp.Cancel(browserCtx); err != nil {
			t.Logf("Warning: browser cancel returned: %v", err)
		}
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return ctc, cleanup
}

// takeScreenshot takes a screenshot with the given name
func (ctc *classifyTestContext) takeScreenshot(name string) {
	if err := ctc.env.TakeFullScreenshot(ctc.ctx, name); err != nil {
		ctc.env.LogTest(ctc.t, "Warning: Failed to take screenshot %s: %v", name, err)
	} else {
		ctc.env.LogTest(ctc.t, "Screenshot: %s", name)
	}
}

// loadJobDefinition loads the codebase_classify.toml job definition
func (ctc *classifyTestContext) loadJobDefinition() error {
	possiblePaths := []string{
		"config/job-definitions/codebase_classify.toml",
		"../config/job-definitions/codebase_classify.toml",
		"../../test/config/job-definitions/codebase_classify.toml",
		"test/config/job-definitions/codebase_classify.toml",
	}

	var foundPath string
	var content []byte
	var err error
	for _, p := range possiblePaths {
		absPath, _ := filepath.Abs(p)
		content, err = os.ReadFile(absPath)
		if err == nil {
			foundPath = absPath
			break
		}
	}

	if err != nil {
		return fmt.Errorf("could not find codebase_classify.toml: %w", err)
	}

	ctc.env.LogTest(ctc.t, "Found codebase_classify definition at: %s", foundPath)

	// Save to results directory
	destPath := filepath.Join(ctc.env.GetResultsDir(), "codebase_classify.toml")
	if writeErr := os.WriteFile(destPath, content, 0644); writeErr != nil {
		ctc.env.LogTest(ctc.t, "Warning: Could not save job definition TOML: %v", writeErr)
	}

	// Load the job definition into the service via API
	if loadErr := ctc.env.LoadJobDefinitionFile(foundPath); loadErr != nil {
		return fmt.Errorf("could not load job definition into service: %w", loadErr)
	}

	return nil
}

// triggerJob triggers the codebase_classify job via API
func (ctc *classifyTestContext) triggerJob() (string, error) {
	ctc.env.LogTest(ctc.t, "Triggering codebase_classify job via API...")

	resp, err := ctc.helper.POST("/api/job-definitions/codebase_classify/execute", nil)
	if err != nil {
		return "", fmt.Errorf("failed to execute job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	ctc.env.LogTest(ctc.t, "Job definition execution triggered, polling for manager job...")

	// Poll for the manager job to appear
	var jobID string
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)

		jobsResp, err := ctc.helper.GET("/api/jobs?limit=10&order=desc")
		if err != nil {
			continue
		}

		var jobsResult map[string]interface{}
		if err := ctc.helper.ParseJSONResponse(jobsResp, &jobsResult); err != nil {
			jobsResp.Body.Close()
			continue
		}
		jobsResp.Body.Close()

		jobs, ok := jobsResult["jobs"].([]interface{})
		if !ok || len(jobs) == 0 {
			continue
		}

		for _, jobRaw := range jobs {
			job, ok := jobRaw.(map[string]interface{})
			if !ok {
				continue
			}

			jobType, _ := job["type"].(string)
			if jobType != "manager" {
				continue
			}

			metadata, _ := job["metadata"].(map[string]interface{})
			jobDefID, _ := metadata["job_definition_id"].(string)
			if jobDefID == "codebase_classify" {
				jobID, _ = job["id"].(string)
				break
			}
		}

		if jobID != "" {
			break
		}
	}

	if jobID == "" {
		return "", fmt.Errorf("manager job not found after polling")
	}

	ctc.env.LogTest(ctc.t, "✓ Job created: %s", jobID)
	return jobID, nil
}
