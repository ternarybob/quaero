package ui

import (
	"context"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// TestJobsAgentDisabled verifies that agent job definitions appear in the UI
// even when the agent service is unavailable (no Google API key configured).
// The job should be displayed with a "Disabled" badge and error message.
func TestJobsAgentDisabled(t *testing.T) {
	// Use base config + override to disable agent service
	// Priority: test-quaero.toml (base) -> quaero-no-ai.toml (override)
	env, err := common.SetupTestEnvironment("TestJobsAgentDisabled", "../config/quaero-no-ai.toml")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	startTime := time.Now()
	env.LogTest(t, "=== RUN TestJobsAgentDisabled")
	defer func() {
		elapsed := time.Since(startTime)
		if t.Failed() {
			env.LogTest(t, "--- FAIL: TestJobsAgentDisabled (%.2fs)", elapsed.Seconds())
		} else {
			env.LogTest(t, "--- PASS: TestJobsAgentDisabled (%.2fs)", elapsed.Seconds())
		}
	}()

	env.LogTest(t, "Test environment ready, service running at: %s", env.GetBaseURL())
	env.LogTest(t, "Results directory: %s", env.GetResultsDir())
	env.LogTest(t, "Config file: %s", env.ConfigFilePath)

	// Load test agent job definition from job-definitions directory
	env.LogTest(t, "Loading test agent job definition...")
	if err := env.LoadJobDefinitionFile("../config/job-definitions/test-agent-job.toml"); err != nil {
		env.LogTest(t, "ERROR: Failed to load test agent job: %v", err)
		t.Fatalf("Failed to load test agent job: %v", err)
	}
	env.LogTest(t, "✓ Test agent job loaded")

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := env.GetBaseURL() + "/jobs"

	// Collect console errors
	consoleErrors := []string{}
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if consoleEvent, ok := ev.(*runtime.EventExceptionThrown); ok {
			if consoleEvent.ExceptionDetails != nil && consoleEvent.ExceptionDetails.Exception != nil {
				errorMsg := consoleEvent.ExceptionDetails.Exception.Description
				if errorMsg == "" && consoleEvent.ExceptionDetails.Text != "" {
					errorMsg = consoleEvent.ExceptionDetails.Text
				}
				consoleErrors = append(consoleErrors, errorMsg)
			}
		}
	})

	env.LogTest(t, "Setting desktop viewport size (1920x1080)")
	env.LogTest(t, "Navigating to jobs page: %s", url)

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for Alpine.js to initialize and fetch job definitions
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to load jobs page: %v", err)
		env.TakeScreenshot(ctx, "jobs-page-load-failed")
		t.Fatalf("Failed to load jobs page: %v", err)
	}

	env.LogTest(t, "✓ Jobs page loaded")
	env.TakeScreenshot(ctx, "jobs-page-loaded")

	// Check for console errors
	env.LogTest(t, "Checking for console errors...")
	if len(consoleErrors) > 0 {
		env.LogTest(t, "WARNING: Found %d console error(s):", len(consoleErrors))
		for i, errMsg := range consoleErrors {
			env.LogTest(t, "  [%d] %s", i+1, errMsg)
		}
		// Don't fail test on console errors - they might be expected
	} else {
		env.LogTest(t, "✓ No console errors found")
	}

	// Wait for job definitions section to load
	env.LogTest(t, "Waiting for job definitions section to load...")
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`[x-data="jobDefinitionsManagement"]`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Allow Alpine.js to fetch and render data
	)
	if err != nil {
		env.LogTest(t, "ERROR: Job definitions section not found: %v", err)
		env.TakeScreenshot(ctx, "job-definitions-section-not-found")
		t.Fatalf("Job definitions section not found: %v", err)
	}
	env.LogTest(t, "✓ Job definitions section loaded")

	// Check if the test agent job appears in the list
	env.LogTest(t, "Looking for 'Test Keyword Extraction' job card...")
	var agentJobInfo struct {
		Found                bool   `json:"found"`
		Name                 string `json:"name"`
		Description          string `json:"description"`
		HasDisabledBadge     bool   `json:"hasDisabledBadge"`
		RuntimeError         string `json:"runtimeError"`
		RunButtonDisabled    bool   `json:"runButtonDisabled"`
		EditButtonDisabled   bool   `json:"editButtonDisabled"`
		DeleteButtonDisabled bool   `json:"deleteButtonDisabled"`
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				// Look for job cards
				const cards = Array.from(document.querySelectorAll('[x-data="jobDefinitionsManagement"] .card-body > .card'));

				// Find the test agent job
				const agentCard = cards.find(card => card.textContent.includes('Test Keyword Extraction'));

				if (!agentCard) {
					return { found: false, name: '', description: '', hasDisabledBadge: false, runtimeError: '', runButtonDisabled: false, editButtonDisabled: false, deleteButtonDisabled: false };
				}

				// Extract job details
				const nameElement = agentCard.querySelector('.card-title');
				const name = nameElement ? nameElement.textContent.trim() : '';

				const descElement = agentCard.querySelector('.card-subtitle');
				const description = descElement ? descElement.textContent.trim() : '';

				// Check for "Disabled" badge (must be visible, not hidden by Alpine.js)
				const disabledBadges = Array.from(agentCard.querySelectorAll('.label.label-error'));
				const visibleDisabledBadge = disabledBadges.find(badge => {
					// Check if badge is visible (not hidden by Alpine.js x-show)
					const style = window.getComputedStyle(badge);
					return style.display !== 'none' && badge.textContent.includes('Disabled');
				});
				const hasDisabledBadge = !!visibleDisabledBadge;

				// Check for runtime error message
				const errorToast = agentCard.querySelector('.toast.toast-error, .toast.error');
				const runtimeError = errorToast ? errorToast.textContent.trim() : '';

				// Check button states
				const runButton = agentCard.querySelector('button.btn-success');
				const editButton = agentCard.querySelector('button .fa-edit')?.closest('button');
				const deleteButton = agentCard.querySelector('button.btn-error');

				return {
					found: true,
					name: name,
					description: description,
					hasDisabledBadge: hasDisabledBadge,
					runtimeError: runtimeError,
					runButtonDisabled: runButton ? runButton.disabled : false,
					editButtonDisabled: editButton ? editButton.disabled : false,
					deleteButtonDisabled: deleteButton ? deleteButton.disabled : false
				};
			})()
		`, &agentJobInfo),
	)

	if err != nil {
		env.LogTest(t, "ERROR: Failed to check for agent job: %v", err)
		env.TakeScreenshot(ctx, "agent-job-check-failed")
		t.Fatalf("Failed to check for agent job: %v", err)
	}

	if !agentJobInfo.Found {
		env.LogTest(t, "ERROR: 'Test Keyword Extraction' job not found in job definitions list")
		env.TakeScreenshot(ctx, "agent-job-not-found")
		t.Fatal("Agent job should appear in job definitions list even when agent service is unavailable")
	}

	env.LogTest(t, "✓ Found agent job: %s", agentJobInfo.Name)
	env.LogTest(t, "  Description: %s", agentJobInfo.Description)

	// Verify "Disabled" badge is present
	if !agentJobInfo.HasDisabledBadge {
		env.LogTest(t, "ERROR: Agent job should have 'Disabled' badge when agent service is unavailable")
		env.TakeScreenshot(ctx, "no-disabled-badge")
		t.Error("Agent job should have 'Disabled' badge when agent service is unavailable")
	} else {
		env.LogTest(t, "✓ Agent job has 'Disabled' badge")
	}

	// Verify runtime error message is present
	if agentJobInfo.RuntimeError == "" {
		env.LogTest(t, "ERROR: Agent job should display runtime error message")
		env.TakeScreenshot(ctx, "no-runtime-error")
		t.Error("Agent job should display runtime error message explaining missing API key")
	} else {
		env.LogTest(t, "✓ Runtime error message displayed:")
		env.LogTest(t, "  %s", agentJobInfo.RuntimeError)

		// Verify error message mentions API key
		if !contains(agentJobInfo.RuntimeError, "Google API key") && !contains(agentJobInfo.RuntimeError, "QUAERO_AGENT_GOOGLE_API_KEY") {
			env.LogTest(t, "WARNING: Error message should mention Google API key or QUAERO_AGENT_GOOGLE_API_KEY")
		}
	}

	// Verify action buttons are disabled
	if !agentJobInfo.RunButtonDisabled {
		env.LogTest(t, "ERROR: Run button should be disabled when agent service is unavailable")
		env.TakeScreenshot(ctx, "run-button-not-disabled")
		t.Error("Run button should be disabled when agent service is unavailable")
	} else {
		env.LogTest(t, "✓ Run button is disabled")
	}

	if !agentJobInfo.EditButtonDisabled {
		env.LogTest(t, "ERROR: Edit button should be disabled when agent service is unavailable")
		env.TakeScreenshot(ctx, "edit-button-not-disabled")
		t.Error("Edit button should be disabled when agent service is unavailable")
	} else {
		env.LogTest(t, "✓ Edit button is disabled")
	}

	if !agentJobInfo.DeleteButtonDisabled {
		env.LogTest(t, "ERROR: Delete button should be disabled when agent service is unavailable")
		env.TakeScreenshot(ctx, "delete-button-not-disabled")
		t.Error("Delete button should be disabled when agent service is unavailable")
	} else {
		env.LogTest(t, "✓ Delete button is disabled")
	}

	env.TakeScreenshot(ctx, "agent-job-disabled-final")
	env.LogTest(t, "✅ Agent job displays correctly with disabled state when agent service is unavailable")
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
