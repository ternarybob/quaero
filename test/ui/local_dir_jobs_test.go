package ui

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pelletier/go-toml/v2"
	"github.com/ternarybob/quaero/test/common"
)

// localDirTestContext holds shared state for local_dir tests
type localDirTestContext struct {
	t               *testing.T
	env             *common.TestEnvironment
	ctx             context.Context
	helper          *common.HTTPTestHelper
	jobsURL         string
	queueURL        string
	jobAddURL       string
	screenshotCount int
}

// screenshot takes a full-page screenshot with an incremented prefix (01, 02, 03, etc.)
func (ltc *localDirTestContext) screenshot(name string) {
	ltc.screenshotCount++
	prefixedName := fmt.Sprintf("%02d_%s", ltc.screenshotCount, name)
	ltc.env.TakeFullScreenshot(ltc.ctx, prefixedName)
}

// saveJobToml saves the job definition as TOML to the results directory
// Converts the API steps array format to the correct [step.{name}] TOML format
func (ltc *localDirTestContext) saveJobToml(filename string, jobDef map[string]interface{}) {
	// Convert the steps array to the correct [step.{name}] format for TOML
	tomlDef := make(map[string]interface{})
	for k, v := range jobDef {
		if k == "steps" {
			// Convert steps array to step map: [step.{name}] format
			if stepsArray, ok := v.([]map[string]interface{}); ok {
				stepMap := make(map[string]map[string]interface{})
				for _, step := range stepsArray {
					if name, ok := step["name"].(string); ok {
						// Copy step data but exclude "name" (it becomes the key)
						stepData := make(map[string]interface{})
						for sk, sv := range step {
							if sk != "name" {
								// Flatten config into step data for TOML format
								if sk == "config" {
									if configMap, ok := sv.(map[string]interface{}); ok {
										for ck, cv := range configMap {
											stepData[ck] = cv
										}
									}
								} else if sk == "depends" {
									// Convert depends string to array for TOML format
									if depStr, ok := sv.(string); ok && depStr != "" {
										// Split comma-separated deps into array
										deps := strings.Split(depStr, ",")
										for i := range deps {
											deps[i] = strings.TrimSpace(deps[i])
										}
										stepData["depends"] = deps
									}
								} else {
									stepData[sk] = sv
								}
							}
						}
						stepMap[name] = stepData
					}
				}
				tomlDef["step"] = stepMap
			}
		} else {
			tomlDef[k] = v
		}
	}

	tomlData, err := toml.Marshal(tomlDef)
	if err != nil {
		ltc.env.LogTest(ltc.t, "Warning: failed to marshal job definition to TOML: %v", err)
		return
	}

	// Remove the redundant [step] line that go-toml generates for nested maps
	// We only want [step.{name}] sections, not a standalone [step] header
	tomlStr := string(tomlData)
	tomlStr = strings.Replace(tomlStr, "[step]\n", "", 1)

	tomlPath := filepath.Join(ltc.env.GetResultsDir(), filename)
	if err := os.WriteFile(tomlPath, []byte(tomlStr), 0644); err != nil {
		ltc.env.LogTest(ltc.t, "Warning: failed to save job TOML to %s: %v", tomlPath, err)
		return
	}
	ltc.env.LogTest(ltc.t, "Saved job definition TOML to: %s", tomlPath)
}

// newLocalDirTestContext creates a new test context with browser and environment
func newLocalDirTestContext(t *testing.T, timeout time.Duration) (*localDirTestContext, func()) {
	// Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	baseURL := env.GetBaseURL()

	ltc := &localDirTestContext{
		t:         t,
		env:       env,
		ctx:       browserCtx,
		helper:    env.NewHTTPTestHelper(t),
		jobsURL:   baseURL + "/jobs",
		queueURL:  baseURL + "/queue",
		jobAddURL: baseURL + "/jobs/add",
	}

	// Return cleanup function
	cleanup := func() {
		if err := chromedp.Cancel(browserCtx); err != nil {
			t.Logf("Warning: browser cancel returned: %v", err)
		}
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return ltc, cleanup
}

// createTestDirectory creates a temporary directory with test files
func createTestDirectory(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "quaero-ui-local-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	testFiles := map[string]string{
		"README.md":          "# Test Project\n\nThis is a test project for local_dir worker UI testing.\n\n## Features\n- File indexing\n- Content extraction\n",
		"main.go":            "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n",
		"config.txt":         "# Configuration file\nkey=value\nfoo=bar\nenvironment=test\n",
		"src/utils.go":       "package src\n\n// Helper is a utility function\nfunc Helper() string {\n\treturn \"helper\"\n}\n",
		"src/models/user.go": "package models\n\n// User represents a user in the system\ntype User struct {\n\tID   int\n\tName string\n}\n",
		"docs/api.md":        "# API Documentation\n\n## Endpoints\n\n### GET /api/status\nReturns the service status.\n",
		"docs/guide.md":      "# User Guide\n\n## Getting Started\n\n1. Install the application\n2. Configure settings\n3. Run the service\n",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	t.Logf("Created test directory with %d files at: %s", len(testFiles), tempDir)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createJobDefinitionViaAPI creates a job definition via API (used to set up test data)
func (ltc *localDirTestContext) createJobDefinitionViaAPI(name, dirPath string, tags []string) (string, error) {
	defID := fmt.Sprintf("local-dir-test-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        name,
		"description": "Test local directory indexing job",
		"type":        "local_dir",
		"enabled":     true,
		"tags":        tags,
		"steps": []map[string]interface{}{
			{
				"name": "index-files",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           dirPath,
					"include_extensions": []string{".go", ".md", ".txt"},
					"exclude_paths":      []string{".git", "node_modules"},
					"max_file_size":      1048576,
					"max_files":          50,
				},
			},
		},
	}

	// Save job definition as TOML to results directory
	ltc.saveJobToml("local-dir-job-definition.toml", body)

	resp, err := ltc.helper.POST("/api/job-definitions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("job definition creation failed with status: %d", resp.StatusCode)
	}

	ltc.env.LogTest(ltc.t, "Created job definition via API: %s", defID)
	return defID, nil
}

// createCombinedJobDefinitionViaAPI creates a job with index + summary steps
func (ltc *localDirTestContext) createCombinedJobDefinitionViaAPI(name, dirPath string, tags []string, prompt string) (string, error) {
	defID := fmt.Sprintf("combined-test-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        name,
		"description": "Combined job: index files then generate summary",
		"type":        "summarizer",
		"enabled":     true,
		"tags":        tags,
		"steps": []map[string]interface{}{
			{
				"name": "index-files",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           dirPath,
					"include_extensions": []string{".go", ".md", ".txt"},
					"exclude_paths":      []string{".git", "node_modules"},
					"max_file_size":      1048576,
					"max_files":          50,
				},
			},
			{
				"name":    "generate-summary",
				"type":    "summary",
				"depends": "index-files",
				"config": map[string]interface{}{
					"prompt":      prompt,
					"filter_tags": tags,
					"api_key":     "{google_gemini_api_key}",
				},
			},
		},
	}

	// Save job definition as TOML to results directory
	ltc.saveJobToml("combined-job-definition.toml", body)

	resp, err := ltc.helper.POST("/api/job-definitions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create combined job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("combined job definition creation failed with status: %d", resp.StatusCode)
	}

	ltc.env.LogTest(ltc.t, "Created combined job definition via API: %s", defID)
	return defID, nil
}

// deleteJobDefinitionViaAPI deletes a job definition via API
func (ltc *localDirTestContext) deleteJobDefinitionViaAPI(defID string) {
	resp, err := ltc.helper.DELETE("/api/job-definitions/" + defID)
	if err != nil {
		ltc.env.LogTest(ltc.t, "Warning: failed to delete job definition %s: %v", defID, err)
		return
	}
	resp.Body.Close()
	ltc.env.LogTest(ltc.t, "Deleted job definition: %s", defID)
}

// triggerJobViaUI triggers a job by clicking the run button on the Jobs page
func (ltc *localDirTestContext) triggerJobViaUI(jobName string) error {
	ltc.env.LogTest(ltc.t, "Triggering job via UI: %s", jobName)

	// Take screenshot before navigation
	ltc.screenshot("trigger_job_before")

	// Navigate to Jobs page
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.jobsURL)); err != nil {
		return fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return fmt.Errorf("jobs page did not load: %w", err)
	}

	ltc.screenshot("jobs_page_loaded")

	// Convert job name to button ID format
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	buttonID = buttonID + "-run"

	ltc.env.LogTest(ltc.t, "Looking for run button: #%s", buttonID)

	// Click the run button
	runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
		chromedp.Click(runBtnSelector, chromedp.ByQuery),
	); err != nil {
		ltc.screenshot("run_button_not_found")
		return fmt.Errorf("failed to click run button: %w", err)
	}

	// Wait for confirmation modal
	ltc.env.LogTest(ltc.t, "Waiting for confirmation modal")
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		ltc.screenshot("modal_not_found")
		return fmt.Errorf("confirmation modal did not appear: %w", err)
	}

	ltc.screenshot("confirmation_modal")

	// Click Confirm button
	ltc.env.LogTest(ltc.t, "Clicking confirm button")
	if err := chromedp.Run(ltc.ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		ltc.screenshot("confirm_failed")
		return fmt.Errorf("failed to click confirm: %w", err)
	}

	ltc.screenshot("trigger_job_after")
	ltc.env.LogTest(ltc.t, "Job triggered successfully via UI")
	return nil
}

// monitorJobViaUI monitors job progress on the Queue page
func (ltc *localDirTestContext) monitorJobViaUI(jobName string, timeout time.Duration) (string, error) {
	ltc.env.LogTest(ltc.t, "Monitoring job via UI: %s (timeout: %v)", jobName, timeout)

	// Take screenshot before navigation
	ltc.screenshot("monitor_before")

	// Navigate to Queue page
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.queueURL)); err != nil {
		return "", fmt.Errorf("failed to navigate to queue page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return "", fmt.Errorf("queue page did not load: %w", err)
	}

	ltc.screenshot("queue_page_loaded")
	ltc.env.LogTest(ltc.t, "Queue page loaded, looking for job...")

	// Poll for job to appear
	var jobID string
	pollErr := chromedp.Run(ltc.ctx,
		chromedp.Poll(
			fmt.Sprintf(`
				(() => {
					const element = document.querySelector('[x-data="jobList"]');
					if (!element) return null;
					const component = Alpine.$data(element);
					if (!component || !component.allJobs) return null;
					const job = component.allJobs.find(j => j.name && j.name.includes('%s'));
					return job ? job.id : null;
				})()
			`, jobName),
			&jobID,
			chromedp.WithPollingTimeout(30*time.Second),
			chromedp.WithPollingInterval(1*time.Second),
		),
	)
	if pollErr != nil || jobID == "" {
		ltc.screenshot("job_not_found_in_queue")
		return "", fmt.Errorf("job not found in queue: %w", pollErr)
	}

	ltc.env.LogTest(ltc.t, "Job found in queue: %s", jobID)
	ltc.screenshot("job_found")

	// Monitor status until terminal state
	startTime := time.Now()
	lastStatus := ""
	var currentStatus string
	pollStart := time.Now()

	for {
		if err := ltc.ctx.Err(); err != nil {
			return lastStatus, fmt.Errorf("context cancelled: %w", err)
		}

		if time.Since(pollStart) > timeout {
			ltc.screenshot("job_timeout")
			return lastStatus, fmt.Errorf("job did not complete within %v (last status: %s)", timeout, lastStatus)
		}

		// Refresh the page data
		chromedp.Run(ltc.ctx, chromedp.Evaluate(`typeof loadJobs === 'function' && loadJobs()`, nil))
		time.Sleep(200 * time.Millisecond)

		// Get current status from UI
		err := chromedp.Run(ltc.ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge) {
								return statusBadge.getAttribute('data-status');
							}
						}
					}
					return null;
				})()
			`, jobName), &currentStatus),
		)

		if err != nil {
			continue
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			ltc.env.LogTest(ltc.t, "Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Millisecond))
			lastStatus = currentStatus
			ltc.screenshot(fmt.Sprintf("status_%s", currentStatus))
		}

		// Check for terminal states
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			ltc.env.LogTest(ltc.t, "Job reached terminal state: %s", currentStatus)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	ltc.screenshot("monitor_after")
	return currentStatus, nil
}

// verifyJobAddPage verifies the job add page loads and has required elements
func (ltc *localDirTestContext) verifyJobAddPage() error {
	ltc.env.LogTest(ltc.t, "Navigating to job add page")
	ltc.screenshot("job_add_before")

	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.jobAddURL)); err != nil {
		return fmt.Errorf("failed to navigate to job add page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ltc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return fmt.Errorf("job add page did not load: %w", err)
	}

	ltc.screenshot("job_add_loaded")

	// Verify TOML editor exists
	var editorExists bool
	if err := chromedp.Run(ltc.ctx,
		chromedp.Evaluate(`document.getElementById('toml-editor') !== null`, &editorExists),
	); err != nil {
		return fmt.Errorf("failed to check for TOML editor: %w", err)
	}

	if !editorExists {
		ltc.screenshot("editor_missing")
		return fmt.Errorf("TOML editor not found on page")
	}

	ltc.env.LogTest(ltc.t, "TOML editor found on job add page")
	ltc.screenshot("job_add_after")
	return nil
}

// TestLocalDirJobAddPage tests the job add page UI
func TestLocalDirJobAddPage(t *testing.T) {
	ltc, cleanup := newLocalDirTestContext(t, 3*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "--- Starting Test: Local Dir Job Add Page ---")

	// Verify job add page
	if err := ltc.verifyJobAddPage(); err != nil {
		t.Fatalf("Job add page verification failed: %v", err)
	}

	ltc.screenshot("test_complete")
	ltc.env.LogTest(t, "Test completed successfully")
}

// TestLocalDirJobExecution tests triggering and monitoring a local_dir job via UI
func TestLocalDirJobExecution(t *testing.T) {
	ltc, cleanup := newLocalDirTestContext(t, 5*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "--- Starting Test: Local Dir Job Execution ---")

	// Create test directory
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()

	// Create job definition via API
	jobName := "Local Dir UI Test"
	defID, err := ltc.createJobDefinitionViaAPI(jobName, testDir, []string{"test", "local_dir"})
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	defer ltc.deleteJobDefinitionViaAPI(defID)

	// Trigger job via UI
	ltc.env.LogTest(t, "Step 1: Triggering job via UI")
	if err := ltc.triggerJobViaUI(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job via UI
	ltc.env.LogTest(t, "Step 2: Monitoring job via UI")
	finalStatus, err := ltc.monitorJobViaUI(jobName, 2*time.Minute)
	if err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// Verify completion
	if finalStatus != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", finalStatus)
	}

	ltc.screenshot("test_complete")
	ltc.env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestLocalDirJobWithEmptyDirectory tests job behavior with empty directory via UI
func TestLocalDirJobWithEmptyDirectory(t *testing.T) {
	ltc, cleanup := newLocalDirTestContext(t, 3*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "--- Starting Test: Local Dir Job With Empty Directory ---")

	// Create empty test directory
	tempDir, err := os.MkdirTemp("", "quaero-ui-empty-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	ltc.env.LogTest(t, "Created empty test directory: %s", tempDir)

	// Create job definition via API
	jobName := "Local Dir Empty Test"
	defID, err := ltc.createJobDefinitionViaAPI(jobName, tempDir, []string{"test", "empty"})
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	defer ltc.deleteJobDefinitionViaAPI(defID)

	// Trigger job via UI
	ltc.env.LogTest(t, "Step 1: Triggering job via UI")
	if err := ltc.triggerJobViaUI(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job via UI
	ltc.env.LogTest(t, "Step 2: Monitoring job via UI")
	finalStatus, err := ltc.monitorJobViaUI(jobName, 1*time.Minute)
	if err != nil {
		ltc.env.LogTest(t, "Job monitoring ended: %v (status: %s)", err, finalStatus)
	}

	ltc.screenshot("test_complete")
	ltc.env.LogTest(t, "Test completed - final status: %s", finalStatus)
}

// verifyTomlStepFormat reads the saved TOML file and verifies:
// 1. No standalone [step] line exists (only [step.{name}] sections)
// 2. The depends field is present in the generate-summary step
func (ltc *localDirTestContext) verifyTomlStepFormat(filename string) error {
	tomlPath := filepath.Join(ltc.env.GetResultsDir(), filename)
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return fmt.Errorf("failed to read TOML file: %w", err)
	}

	content := string(data)

	// Check 1: No standalone [step] line (only [step.{name}] sections allowed)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[step]" {
			return fmt.Errorf("found standalone [step] line - should only have [step.{name}] sections")
		}
	}

	// Check 2: Verify [step.generate-summary] section exists
	if !strings.Contains(content, "[step.generate-summary]") {
		return fmt.Errorf("missing [step.generate-summary] section")
	}

	// Check 3: Verify [step.index-files] section exists
	if !strings.Contains(content, "[step.index-files]") {
		return fmt.Errorf("missing [step.index-files] section")
	}

	// Check 4: Verify depends field is present as array in generate-summary step
	if !strings.Contains(content, "depends = ['index-files']") {
		return fmt.Errorf("missing depends = ['index-files'] (array format) in generate-summary step")
	}

	ltc.env.LogTest(ltc.t, "TOML format verification passed: no [step], has [step.{name}] sections, depends field present")
	return nil
}

// TestSummaryAgentWithDependency tests summary agent with step dependency via UI
func TestSummaryAgentWithDependency(t *testing.T) {
	ltc, cleanup := newLocalDirTestContext(t, 10*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "--- Starting Test: Summary Agent With Dependency ---")

	// Navigate to Jobs page first before taking initial screenshot
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.jobsURL)); err != nil {
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}
	chromedp.Run(ltc.ctx, chromedp.Sleep(2*time.Second))
	ltc.screenshot("test_start")

	// Create test directory
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()

	// Create combined job definition with index + summary steps
	jobName := "Combined Index Summary Test"
	tags := []string{"codebase", "test-project"}
	prompt := "Review the code base and provide an architectural summary in markdown."

	ltc.env.LogTest(t, "Step 1: Creating combined job definition with dependency")
	defID, err := ltc.createCombinedJobDefinitionViaAPI(jobName, testDir, tags, prompt)
	if err != nil {
		t.Fatalf("Failed to create combined job definition: %v", err)
	}
	defer ltc.deleteJobDefinitionViaAPI(defID)

	// Verify TOML format is correct (no [step], has depends field)
	ltc.env.LogTest(t, "Step 1b: Verifying TOML step format")
	if err := ltc.verifyTomlStepFormat("combined-job-definition.toml"); err != nil {
		t.Errorf("TOML format verification failed: %v", err)
	}

	// Trigger job via UI
	ltc.env.LogTest(t, "Step 2: Triggering combined job via UI")
	if err := ltc.triggerJobViaUI(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job via UI (longer timeout for LLM call)
	ltc.env.LogTest(t, "Step 3: Monitoring job execution (index + summary)")
	finalStatus, err := ltc.monitorJobViaUI(jobName, 5*time.Minute)
	if err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// Verify completion
	if finalStatus != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", finalStatus)
	}

	ltc.screenshot("test_complete")
	ltc.env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestSummaryAgentPlainRequest tests summary agent with plain text prompt via UI
func TestSummaryAgentPlainRequest(t *testing.T) {
	ltc, cleanup := newLocalDirTestContext(t, 10*time.Minute)
	defer cleanup()

	ltc.env.LogTest(t, "--- Starting Test: Summary Agent Plain Request ---")

	// Navigate to Jobs page first before taking initial screenshot
	if err := chromedp.Run(ltc.ctx, chromedp.Navigate(ltc.jobsURL)); err != nil {
		t.Fatalf("Failed to navigate to jobs page: %v", err)
	}
	chromedp.Run(ltc.ctx, chromedp.Sleep(2*time.Second))
	ltc.screenshot("test_start")

	// Create test directory
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()

	// Step 1: First run an index job
	indexJobName := "Plain Request Index"
	ltc.env.LogTest(t, "Step 1: Creating and running index job")

	indexDefID, err := ltc.createJobDefinitionViaAPI(indexJobName, testDir, []string{"plain-test"})
	if err != nil {
		t.Fatalf("Failed to create index job: %v", err)
	}
	defer ltc.deleteJobDefinitionViaAPI(indexDefID)

	if err := ltc.triggerJobViaUI(indexJobName); err != nil {
		t.Fatalf("Failed to trigger index job: %v", err)
	}

	indexStatus, err := ltc.monitorJobViaUI(indexJobName, 2*time.Minute)
	if err != nil {
		t.Fatalf("Index job failed: %v", err)
	}
	if indexStatus != "completed" {
		t.Fatalf("Index job did not complete: %s", indexStatus)
	}
	ltc.env.LogTest(t, "Index job completed")

	// Step 2: Create and run summary job with plain prompt
	summaryJobName := "Plain Summary Request"
	plainPrompt := "List all the files and describe what each one does in a simple bullet point format."

	ltc.env.LogTest(t, "Step 2: Creating summary job with plain prompt")

	summaryDefID := fmt.Sprintf("summary-plain-%d", time.Now().UnixNano())
	summaryBody := map[string]interface{}{
		"id":          summaryDefID,
		"name":        summaryJobName,
		"description": "Plain text summary request test",
		"type":        "summarizer",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name": "generate-summary",
				"type": "summary",
				"config": map[string]interface{}{
					"prompt":      plainPrompt,
					"filter_tags": []string{"plain-test"},
					"api_key":     "{google_gemini_api_key}",
				},
			},
		},
	}

	// Save job definition as TOML to results directory
	ltc.saveJobToml("summary-plain-job-definition.toml", summaryBody)

	resp, err := ltc.helper.POST("/api/job-definitions", summaryBody)
	if err != nil {
		t.Fatalf("Failed to create summary job: %v", err)
	}
	resp.Body.Close()
	defer ltc.deleteJobDefinitionViaAPI(summaryDefID)

	// Trigger summary job via UI
	ltc.env.LogTest(t, "Step 3: Triggering summary job via UI")
	if err := ltc.triggerJobViaUI(summaryJobName); err != nil {
		t.Fatalf("Failed to trigger summary job: %v", err)
	}

	// Monitor summary job via UI
	ltc.env.LogTest(t, "Step 4: Monitoring summary job via UI")
	summaryStatus, err := ltc.monitorJobViaUI(summaryJobName, 3*time.Minute)
	if err != nil {
		t.Fatalf("Summary job failed: %v", err)
	}

	if summaryStatus != "completed" {
		t.Errorf("Expected summary job status 'completed', got '%s'", summaryStatus)
	}

	ltc.screenshot("test_complete")
	ltc.env.LogTest(t, "Test completed - summary job status: %s", summaryStatus)
}
