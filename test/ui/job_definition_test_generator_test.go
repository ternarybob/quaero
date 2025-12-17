// job_definition_test_generator_test.go - Functional tests for test_job_generator.toml
// This test validates the test job generator definition runs to completion without crashes.
// Tests include:
// - Basic completion test using test/config/job-definitions/test_job_generator.toml
// - Monitoring job progress via WebSocket/API
// - Verifying all 4 steps complete successfully

package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJobDefinitionTestJobGeneratorFunctional tests the complete execution of test_job_generator.toml
// This is a functional test that ensures the test job generator job definition:
// 1. Can be triggered successfully
// 2. Progresses through all 4 steps (fast_generator, high_volume_generator, slow_generator, recursive_generator)
// 3. Completes without crashing or timing out
// 4. All steps reach terminal status (completed/failed based on failure_rate)
func TestJobDefinitionTestJobGeneratorFunctional(t *testing.T) {
	// Use longer timeout since slow_generator takes ~2.5 minutes (300 logs * 500ms)
	// Plus recursive_generator with children
	utc := NewUITestContext(t, 10*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Test Job Generator Functional Execution ---")

	jobName := "Test Job Generator"
	jobTimeout := 8 * time.Minute // Allow plenty of time for slow_generator

	// Copy job definition to results for reference
	if err := utc.CopyJobDefinitionToResults("../config/job-definitions/test_job_generator.toml"); err != nil {
		t.Fatalf("Failed to copy job definition: %v", err)
	}

	// Trigger the job
	utc.Log("Triggering job: %s", jobName)
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	utc.Log("Job triggered successfully")

	// Navigate to Queue page for monitoring
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}

	// Wait for job to appear in queue
	utc.Log("Waiting for job to appear in queue...")
	time.Sleep(2 * time.Second)

	// Initial assessment on queue page load
	utc.Log("Taking initial queue assessment...")
	utc.FullScreenshot("test_generator_queue_initial")

	// Monitor job until completion
	utc.Log("Starting job monitoring (timeout: %v)...", jobTimeout)
	startTime := time.Now()
	lastStatus := ""
	jobID := ""
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()
	screenshotCount := 0

	for {
		// Check context
		if err := utc.Ctx.Err(); err != nil {
			t.Fatalf("Context cancelled: %v", err)
		}

		// Check timeout
		elapsed := time.Since(startTime)
		if elapsed > jobTimeout {
			utc.Screenshot("test_generator_timeout")
			t.Fatalf("Job %s did not complete within %v (possible crash)", jobName, jobTimeout)
		}

		// Log progress every 30 seconds (job runs 2+ minutes)
		if time.Since(lastProgressLog) >= 30*time.Second {
			utc.Log("[%v] Monitoring... (status: %s)", elapsed.Round(time.Second), lastStatus)
			lastProgressLog = time.Now()
		}

		// Screenshot timing: every 10 seconds for first minute, then every 30 seconds
		screenshotInterval := 30 * time.Second
		if elapsed < 1*time.Minute {
			screenshotInterval = 10 * time.Second
		}

		if time.Since(lastScreenshotTime) >= screenshotInterval {
			screenshotCount++
			utc.FullScreenshot(fmt.Sprintf("test_generator_monitor_%ds", int(elapsed.Seconds())))
			utc.Log("[%v] Screenshot %d captured (interval: %v)", elapsed.Round(time.Second), screenshotCount, screenshotInterval)
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
			utc.FullScreenshot(fmt.Sprintf("test_generator_status_%s", currentStatus))
		}

		// Check for terminal status
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", currentStatus)
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Take final screenshot
	utc.FullScreenshot("test_generator_final_state")

	// ===============================
	// ASSERTIONS
	// ===============================
	finalStatus := lastStatus
	utc.Log("--- Running Assertions ---")

	// --------------------------------------------------------------------------------
	// Assertion 1: Job completed (not failed from crash)
	// Note: With failure_rate > 0, some child jobs may fail, but the overall job
	// should complete because error_tolerance.failure_action = "continue"
	// --------------------------------------------------------------------------------
	if finalStatus != "completed" && finalStatus != "failed" {
		t.Errorf("FAIL: Test Job Generator did not reach terminal status (status=%s) - possible crash", finalStatus)
	} else {
		utc.Log("PASS: Test Job Generator reached terminal status: %s", finalStatus)
	}

	// --------------------------------------------------------------------------------
	// Assertion 2: Verify expected 4 steps exist
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 2: Verifying expected steps are present...")
	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err != nil {
			t.Errorf("FAIL: Could not get step tree from API for job_id=%s: %v", jobID, err)
		} else {
			expectedSteps := []string{"fast_generator", "high_volume_generator", "slow_generator", "recursive_generator"}
			foundSteps := make(map[string]bool)
			for _, step := range tree.Steps {
				foundSteps[step.Name] = true
			}

			for _, expected := range expectedSteps {
				if !foundSteps[expected] {
					t.Errorf("FAIL: Expected step '%s' not found in Test Job Generator job", expected)
				} else {
					utc.Log("PASS: Found expected step '%s'", expected)
				}
			}

			if len(tree.Steps) != 4 {
				t.Errorf("FAIL: Expected exactly 4 steps, got %d", len(tree.Steps))
			} else {
				utc.Log("PASS: Test Job Generator has exactly 4 steps")
			}
		}
	} else {
		t.Errorf("FAIL: Could not capture job ID to verify steps")
	}

	// --------------------------------------------------------------------------------
	// Assertion 3: All steps reached terminal status (completed or failed)
	// Due to failure_rate configuration, some steps may have failed children
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 3: Verifying all steps reached terminal status...")
	if jobID != "" {
		var tree apiJobTreeResponse
		if err := apiGetJSON(t, httpHelper, fmt.Sprintf("/api/jobs/%s/tree", jobID), &tree); err == nil {
			for _, step := range tree.Steps {
				if step.Status != "completed" && step.Status != "failed" {
					t.Errorf("FAIL: Step '%s' has non-terminal status '%s'", step.Name, step.Status)
				} else {
					utc.Log("PASS: Step '%s' reached terminal status: %s", step.Name, step.Status)
				}
			}
		}
	}

	// --------------------------------------------------------------------------------
	// Assertion 4: Job execution time is reasonable (not too fast = skipped, not too slow = hung)
	// slow_generator: 2 workers * 300 logs * 500ms = ~150 seconds minimum
	// With other steps, expect at least 2 minutes total
	// --------------------------------------------------------------------------------
	elapsed := time.Since(startTime)
	utc.Log("Assertion 4: Verifying execution time is reasonable...")
	if elapsed < 2*time.Minute {
		t.Errorf("FAIL: Job completed too quickly (%v) - slow_generator alone should take ~2.5 minutes", elapsed)
	} else {
		utc.Log("PASS: Job execution time is reasonable: %v", elapsed.Round(time.Second))
	}

	// --------------------------------------------------------------------------------
	// Assertion 5: DOM performance check - verify page handles high log volume
	// test_job_generator.toml generates ~4,500 logs total:
	// - fast_generator: 5*50 = 250
	// - high_volume_generator: 3*1200 = 3,600
	// - slow_generator: 2*300 = 600
	// - recursive_generator: 3*20 = 60 (+children)
	// The page should remain responsive with defaultLogsPerStep=500
	// --------------------------------------------------------------------------------
	utc.Log("Assertion 5: Verifying DOM performance with high log volume...")
	var domMetrics map[string]interface{}
	err := chromedp.Run(utc.Ctx,
		chromedp.Evaluate(`
			(() => {
				const result = {
					totalLogLines: 0,
					totalDOMElements: document.getElementsByTagName('*').length,
					stepsExpanded: 0,
					stepLogCounts: {}
				};

				// Count log lines per step
				const treeSteps = document.querySelectorAll('.tree-step');
				for (const step of treeSteps) {
					const stepNameEl = step.querySelector('.tree-step-name');
					if (!stepNameEl) continue;
					const stepName = stepNameEl.textContent.trim();

					const logLines = step.querySelectorAll('.tree-log-line');
					const count = logLines.length;
					result.stepLogCounts[stepName] = count;
					result.totalLogLines += count;

					if (count > 0) result.stepsExpanded++;
				}

				return result;
			})()
		`, &domMetrics),
	)
	if err != nil {
		t.Logf("Warning: Could not get DOM metrics: %v", err)
	} else {
		totalLogLines := int(domMetrics["totalLogLines"].(float64))
		totalDOMElements := int(domMetrics["totalDOMElements"].(float64))
		stepsExpanded := int(domMetrics["stepsExpanded"].(float64))

		utc.Log("DOM metrics: totalLogLines=%d, totalDOMElements=%d, stepsExpanded=%d",
			totalLogLines, totalDOMElements, stepsExpanded)
		utc.Log("Step log counts: %v", domMetrics["stepLogCounts"])

		// DOM should be manageable - less than 50,000 elements total
		// (with 500 logs/step * 4 steps * 4 elements/log = 8,000 log elements max)
		if totalDOMElements > 50000 {
			t.Errorf("FAIL: DOM element count too high (%d) - page may be unresponsive", totalDOMElements)
		} else {
			utc.Log("PASS: DOM element count is manageable: %d", totalDOMElements)
		}

		// Should have at least some logs visible (job completed, steps should have logs)
		if totalLogLines == 0 && stepsExpanded == 0 {
			utc.Log("WARNING: No log lines visible - steps may not be expanded")
		} else {
			utc.Log("PASS: Logs are visible: %d lines across %d expanded steps", totalLogLines, stepsExpanded)
		}
	}

	utc.Log("Test Job Generator functional test completed with final status: %s (elapsed: %v)", finalStatus, elapsed.Round(time.Second))
}
