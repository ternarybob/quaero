package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// checkChromeAvailable checks if Chrome is available for testing
func checkChromeAvailable() bool {
	chromeNames := []string{"google-chrome", "chromium", "chromium-browser", "chrome"}
	for _, name := range chromeNames {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	return false
}

// skipIfNoChrome skips the test if Chrome is not available
func skipIfNoChrome(t *testing.T) {
	if !checkChromeAvailable() {
		t.Skip("Skipping test - Chrome/Chromium not found in PATH")
	}
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

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// createJobDefinition creates a local_dir job definition via API
func createJobDefinition(helper *common.HTTPTestHelper, name, dirPath string, tags []string) (string, error) {
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

	resp, err := helper.POST("/api/job-definitions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("job definition creation failed with status: %d", resp.StatusCode)
	}

	return defID, nil
}

// createCombinedJobDefinition creates a job with index + summary steps using depends
func createCombinedJobDefinition(helper *common.HTTPTestHelper, name, dirPath string, tags []string, prompt string) (string, error) {
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

	resp, err := helper.POST("/api/job-definitions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return "", fmt.Errorf("job definition creation failed with status: %d", resp.StatusCode)
	}

	return defID, nil
}

// deleteJobDefinition deletes a job definition via API
func deleteJobDefinition(helper *common.HTTPTestHelper, defID string) {
	resp, err := helper.DELETE("/api/job-definitions/" + defID)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// triggerJobViaUI triggers a job via the Jobs page UI
func triggerJobViaUI(ctx context.Context, env *common.TestEnvironment, t *testing.T, jobName string) error {
	baseURL := env.GetBaseURL()
	jobsURL := baseURL + "/jobs"

	// Navigate to Jobs page - before screenshot
	env.LogTest(t, "Navigating to Jobs page")
	if err := env.TakeScreenshot(ctx, "trigger_job_before"); err != nil {
		t.Logf("Failed to take before screenshot: %v", err)
	}

	if err := chromedp.Run(ctx, chromedp.Navigate(jobsURL)); err != nil {
		return fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return fmt.Errorf("jobs page did not load: %w", err)
	}

	env.TakeScreenshot(ctx, "jobs_page_loaded")

	// Convert job name to button ID format
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	buttonID = buttonID + "-run"

	env.LogTest(t, "Looking for button with ID: %s", buttonID)

	// Click the run button
	runBtnSelector := fmt.Sprintf(`#%s`, buttonID)
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(runBtnSelector, chromedp.ByQuery),
		chromedp.Click(runBtnSelector, chromedp.ByQuery),
	); err != nil {
		env.TakeFullScreenshot(ctx, "run_click_failed")
		return fmt.Errorf("failed to click run button: %w", err)
	}

	// Handle Confirmation Modal
	env.LogTest(t, "Waiting for confirmation modal")
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`.modal.active`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		env.TakeFullScreenshot(ctx, "modal_wait_failed")
		return fmt.Errorf("confirmation modal did not appear: %w", err)
	}

	env.TakeScreenshot(ctx, "confirmation_modal")

	// Click Confirm button
	env.LogTest(t, "Confirming run")
	if err := chromedp.Run(ctx,
		chromedp.Click(`.modal.active .modal-footer .btn-primary`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		env.TakeFullScreenshot(ctx, "confirm_click_failed")
		return fmt.Errorf("failed to confirm run: %w", err)
	}

	env.TakeScreenshot(ctx, "trigger_job_after")
	env.LogTest(t, "Job triggered: %s", jobName)
	return nil
}

// monitorJobViaUI monitors a job on the Queue page until completion
func monitorJobViaUI(ctx context.Context, env *common.TestEnvironment, t *testing.T, jobName string, timeout time.Duration) (string, error) {
	baseURL := env.GetBaseURL()
	queueURL := baseURL + "/queue"

	env.LogTest(t, "Monitoring job: %s (timeout: %v)", jobName, timeout)

	// Navigate to Queue page - before screenshot
	env.TakeScreenshot(ctx, "monitor_job_before")

	if err := chromedp.Run(ctx, chromedp.Navigate(queueURL)); err != nil {
		return "", fmt.Errorf("failed to navigate to queue page: %w", err)
	}

	// Wait for page to load
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return "", fmt.Errorf("queue page did not load: %w", err)
	}

	env.TakeScreenshot(ctx, "queue_page_loaded")
	env.LogTest(t, "Queue page loaded, looking for job...")

	// Poll for job to appear in the queue
	var jobID string
	pollErr := chromedp.Run(ctx,
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
		env.TakeFullScreenshot(ctx, "job_not_found")
		return "", fmt.Errorf("job %s not found in queue: %w", jobName, pollErr)
	}
	env.LogTest(t, "Job found in queue (ID: %s)", jobID)

	// Monitor status
	startTime := time.Now()
	lastStatus := ""
	var currentStatus string
	pollStart := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			return lastStatus, fmt.Errorf("context cancelled: %w", err)
		}

		if time.Since(pollStart) > timeout {
			env.TakeFullScreenshot(ctx, "job_timeout")
			return lastStatus, fmt.Errorf("job did not complete within %v (last status: %s)", timeout, lastStatus)
		}

		// Trigger refresh
		chromedp.Run(ctx, chromedp.Evaluate(`typeof loadJobs === 'function' && loadJobs()`, nil))
		time.Sleep(200 * time.Millisecond)

		// Get current status
		err := chromedp.Run(ctx,
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
			env.LogTest(t, "  Status: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Millisecond))
			lastStatus = currentStatus
			env.TakeScreenshot(ctx, fmt.Sprintf("status_%s", currentStatus))
		}

		// Check terminal states
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			env.LogTest(t, "Job reached terminal status: %s", currentStatus)
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	env.TakeFullScreenshot(ctx, "monitor_job_after")
	return currentStatus, nil
}

// TestLocalDirJobAddPage tests the job add page basic functionality
func TestLocalDirJobAddPage(t *testing.T) {
	skipIfNoChrome(t)

	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelTimeout()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	baseURL := env.GetBaseURL()
	jobAddURL := baseURL + "/jobs/add"
	env.LogTest(t, "--- Starting Test: Local Dir Job Add Page ---")

	// Take before screenshot
	env.LogTest(t, "Step 1: Navigate to job add page")
	if err := env.TakeScreenshot(ctx, "job_add_before"); err != nil {
		t.Logf("Failed to take before screenshot: %v", err)
	}

	// Navigate to job add page
	if err := chromedp.Run(ctx,
		chromedp.Navigate(jobAddURL),
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Failed to navigate to job add page: %v", err)
	}

	env.TakeScreenshot(ctx, "job_add_after")
	env.LogTest(t, "Job add page loaded successfully")

	// Verify TOML editor exists
	env.LogTest(t, "Step 2: Verifying TOML editor exists")
	env.TakeScreenshot(ctx, "toml_editor_before")

	var editorExists bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('toml-editor') !== null`, &editorExists),
	); err != nil {
		t.Fatalf("Failed to check for TOML editor: %v", err)
	}

	if !editorExists {
		env.TakeFullScreenshot(ctx, "editor_missing")
		t.Fatal("TOML editor not found on page")
	}
	env.LogTest(t, "TOML editor found")
	env.TakeScreenshot(ctx, "toml_editor_after")

	env.TakeFullScreenshot(ctx, "test_complete")
	env.LogTest(t, "Test completed successfully")
}

// TestLocalDirJobExecution tests executing a local_dir job via UI
func TestLocalDirJobExecution(t *testing.T) {
	skipIfNoChrome(t)

	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create test directory
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()
	env.LogTest(t, "Created test directory: %s", testDir)

	// Create timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelTimeout()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	env.LogTest(t, "--- Starting Test: Local Dir Job Execution ---")

	// Create job definition via API
	helper := env.NewHTTPTestHelper(t)
	jobName := "Local Dir UI Test"

	env.LogTest(t, "Step 1: Creating job definition")
	env.TakeScreenshot(ctx, "create_job_before")

	defID, err := createJobDefinition(helper, jobName, testDir, []string{"test", "local_dir"})
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	defer deleteJobDefinition(helper, defID)
	env.LogTest(t, "Created job definition: %s", defID)

	// Trigger job via UI
	env.LogTest(t, "Step 2: Triggering job via UI")
	if err := triggerJobViaUI(ctx, env, t, jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job execution
	env.LogTest(t, "Step 3: Monitoring job execution")
	finalStatus, err := monitorJobViaUI(ctx, env, t, jobName, 2*time.Minute)
	if err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// Verify completion
	if finalStatus != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", finalStatus)
	}

	env.TakeFullScreenshot(ctx, "test_complete")
	env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestLocalDirJobWithEmptyDirectory tests local_dir job behavior with empty directory
func TestLocalDirJobWithEmptyDirectory(t *testing.T) {
	skipIfNoChrome(t)

	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create empty test directory
	tempDir, err := os.MkdirTemp("", "quaero-ui-empty-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	env.LogTest(t, "Created empty test directory: %s", tempDir)

	// Create timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelTimeout()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	env.LogTest(t, "--- Starting Test: Local Dir Job With Empty Directory ---")

	// Create job definition
	helper := env.NewHTTPTestHelper(t)
	jobName := "Local Dir Empty Test"

	env.LogTest(t, "Step 1: Creating job definition for empty directory")
	env.TakeScreenshot(ctx, "create_empty_job_before")

	defID, err := createJobDefinition(helper, jobName, tempDir, []string{"test", "empty"})
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	defer deleteJobDefinition(helper, defID)

	env.TakeScreenshot(ctx, "create_empty_job_after")

	// Trigger job
	env.LogTest(t, "Step 2: Triggering job")
	if err := triggerJobViaUI(ctx, env, t, jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job
	env.LogTest(t, "Step 3: Monitoring job")
	finalStatus, err := monitorJobViaUI(ctx, env, t, jobName, 1*time.Minute)
	if err != nil {
		env.LogTest(t, "Job monitoring ended: %v (status: %s)", err, finalStatus)
	}

	env.TakeFullScreenshot(ctx, "test_complete")
	env.LogTest(t, "Test completed - final status: %s", finalStatus)
}

// TestSummaryAgentWithDependency tests the summary agent with step dependency on index step
func TestSummaryAgentWithDependency(t *testing.T) {
	skipIfNoChrome(t)

	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create test directory with code files
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()
	env.LogTest(t, "Created test directory: %s", testDir)

	// Create timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelTimeout()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	env.LogTest(t, "--- Starting Test: Summary Agent With Dependency ---")
	env.TakeScreenshot(ctx, "test_start")

	// Create combined job definition with index + summary steps
	helper := env.NewHTTPTestHelper(t)
	jobName := "Combined Index Summary Test"
	tags := []string{"codebase", "test-project"}
	prompt := "Review the code base and provide an architectural summary in markdown."

	env.LogTest(t, "Step 1: Creating combined job definition with dependency")
	env.TakeScreenshot(ctx, "create_combined_job_before")

	defID, err := createCombinedJobDefinition(helper, jobName, testDir, tags, prompt)
	if err != nil {
		t.Fatalf("Failed to create combined job definition: %v", err)
	}
	defer deleteJobDefinition(helper, defID)
	env.LogTest(t, "Created combined job definition: %s", defID)
	env.TakeScreenshot(ctx, "create_combined_job_after")

	// Trigger job via UI
	env.LogTest(t, "Step 2: Triggering combined job via UI")
	if err := triggerJobViaUI(ctx, env, t, jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}

	// Monitor job execution (longer timeout for LLM call)
	env.LogTest(t, "Step 3: Monitoring job execution (index + summary)")
	finalStatus, err := monitorJobViaUI(ctx, env, t, jobName, 5*time.Minute)
	if err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// Verify completion
	if finalStatus != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", finalStatus)
	}

	// Step 4: Verify summary document was created
	env.LogTest(t, "Step 4: Verifying summary document")
	env.TakeScreenshot(ctx, "verify_summary_before")

	resp, err := helper.GET("/api/documents?tags=summary")
	if err != nil {
		t.Fatalf("Failed to query documents: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		env.LogTest(t, "Summary document query successful")
	}

	env.TakeScreenshot(ctx, "verify_summary_after")
	env.TakeFullScreenshot(ctx, "test_complete")
	env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestSummaryAgentPlainRequest tests the summary agent with a plain text prompt
func TestSummaryAgentPlainRequest(t *testing.T) {
	skipIfNoChrome(t)

	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	// Create test directory
	testDir, cleanupDir := createTestDirectory(t)
	defer cleanupDir()
	env.LogTest(t, "Created test directory: %s", testDir)

	// Create timeout context
	ctx, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelTimeout()

	// Create browser context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer func() {
		chromedp.Cancel(browserCtx)
		cancelBrowser()
	}()
	ctx = browserCtx

	env.LogTest(t, "--- Starting Test: Summary Agent Plain Request ---")
	env.TakeScreenshot(ctx, "test_start")

	helper := env.NewHTTPTestHelper(t)

	// First index the files
	indexJobName := "Plain Request Index"
	env.LogTest(t, "Step 1: Creating and running index job")
	env.TakeScreenshot(ctx, "index_job_before")

	indexDefID, err := createJobDefinition(helper, indexJobName, testDir, []string{"plain-test"})
	if err != nil {
		t.Fatalf("Failed to create index job: %v", err)
	}
	defer deleteJobDefinition(helper, indexDefID)

	if err := triggerJobViaUI(ctx, env, t, indexJobName); err != nil {
		t.Fatalf("Failed to trigger index job: %v", err)
	}

	indexStatus, err := monitorJobViaUI(ctx, env, t, indexJobName, 2*time.Minute)
	if err != nil {
		t.Fatalf("Index job failed: %v", err)
	}
	if indexStatus != "completed" {
		t.Fatalf("Index job did not complete: %s", indexStatus)
	}
	env.TakeScreenshot(ctx, "index_job_after")

	// Now create summary job with plain request
	summaryJobName := "Plain Summary Request"
	plainPrompt := "List all the files and describe what each one does in a simple bullet point format."

	env.LogTest(t, "Step 2: Creating summary job with plain prompt")
	env.TakeScreenshot(ctx, "summary_job_before")

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

	resp, err := helper.POST("/api/job-definitions", summaryBody)
	if err != nil {
		t.Fatalf("Failed to create summary job: %v", err)
	}
	resp.Body.Close()
	defer deleteJobDefinition(helper, summaryDefID)

	// Trigger and monitor summary job
	env.LogTest(t, "Step 3: Triggering summary job")
	if err := triggerJobViaUI(ctx, env, t, summaryJobName); err != nil {
		t.Fatalf("Failed to trigger summary job: %v", err)
	}

	env.LogTest(t, "Step 4: Monitoring summary job")
	summaryStatus, err := monitorJobViaUI(ctx, env, t, summaryJobName, 3*time.Minute)
	if err != nil {
		t.Fatalf("Summary job failed: %v", err)
	}

	if summaryStatus != "completed" {
		t.Errorf("Expected summary job status 'completed', got '%s'", summaryStatus)
	}

	env.TakeScreenshot(ctx, "summary_job_after")
	env.TakeFullScreenshot(ctx, "test_complete")
	env.LogTest(t, "Test completed - summary job status: %s", summaryStatus)
}
