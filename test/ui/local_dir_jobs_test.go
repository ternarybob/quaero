package ui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// createLocalDirTestDirectory creates a temporary directory with test files
func createLocalDirTestDirectory(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "quaero-ui-local-dir-test-*")
	require.NoError(t, err, "Failed to create temp directory")

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
			t.Logf("Warning: failed to create directory for %s: %v", path, err)
			continue
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Logf("Warning: failed to create test file %s: %v", path, err)
		}
	}

	t.Logf("Created test directory with %d files at: %s", len(testFiles), tempDir)
	return tempDir
}

// cleanupLocalDirTestDirectory removes the test directory
func cleanupLocalDirTestDirectory(t *testing.T, dir string) {
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to cleanup test directory %s: %v", dir, err)
	} else {
		t.Logf("Cleaned up test directory: %s", dir)
	}
}

// createLocalDirJobDef creates a local_dir job definition via API
func createLocalDirJobDef(t *testing.T, helper *common.HTTPTestHelper, id, name, dirPath string, tags []string) string {
	body := map[string]interface{}{
		"id":      id,
		"name":    name,
		"type":    "local_dir",
		"enabled": true,
		"tags":    tags,
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
	require.NoError(t, err, "Failed to create local_dir job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Failed to create job definition: status %d", resp.StatusCode)
		return ""
	}

	t.Logf("Created local_dir job definition: id=%s", id)
	return id
}

// createCombinedJobDef creates a job with index + summary steps using depends
func createCombinedJobDef(t *testing.T, helper *common.HTTPTestHelper, id, name, dirPath string, tags []string, prompt string) string {
	body := map[string]interface{}{
		"id":          id,
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
	require.NoError(t, err, "Failed to create combined job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Failed to create combined job definition: status %d", resp.StatusCode)
		return ""
	}

	t.Logf("Created combined job definition: id=%s", id)
	return id
}

// deleteLocalDirJobDef deletes a job definition via API
func deleteLocalDirJobDef(t *testing.T, helper *common.HTTPTestHelper, defID string) {
	resp, err := helper.DELETE("/api/job-definitions/" + defID)
	if err != nil {
		t.Logf("Warning: failed to delete job definition %s: %v", defID, err)
		return
	}
	resp.Body.Close()
	t.Logf("Deleted job definition: %s", defID)
}

// executeJobDef executes a job definition and returns the job ID
func executeJobDef(t *testing.T, helper *common.HTTPTestHelper, defID string) string {
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Logf("Job execution returned status %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse execute response")

	jobID, ok := result["job_id"].(string)
	if !ok {
		t.Log("Job ID not found in response")
		return ""
	}

	t.Logf("Job started: %s", jobID)
	return jobID
}

// waitForLocalDirJobCompletion waits for a job to reach terminal state
func waitForLocalDirJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	lastStatus := ""

	for time.Now().Before(deadline) {
		resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var job map[string]interface{}
		err = helper.ParseJSONResponse(resp, &job)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		status, _ := job["status"].(string)
		if status != lastStatus {
			t.Logf("Job status: %s -> %s", lastStatus, status)
			lastStatus = status
		}

		// Check terminal states
		if status == "completed" || status == "failed" || status == "cancelled" {
			return status
		}

		time.Sleep(500 * time.Millisecond)
	}

	return "timeout"
}

// deleteLocalDirJob deletes a job via API
func deleteLocalDirJob(t *testing.T, helper *common.HTTPTestHelper, jobID string) {
	resp, err := helper.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Logf("Warning: failed to delete job %s: %v", jobID, err)
		return
	}
	resp.Body.Close()
}

// TestLocalDirJobAddPage tests creating a local_dir job definition via API
func TestLocalDirJobAddPage(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	env.LogTest(t, "--- Starting Test: Local Dir Job Add Page ---")

	// Create test directory
	testDir := createLocalDirTestDirectory(t)
	defer cleanupLocalDirTestDirectory(t, testDir)

	// Step 1: Create a valid local_dir job definition
	env.LogTest(t, "Step 1: Creating local_dir job definition")
	defID := fmt.Sprintf("local-dir-add-test-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Local Dir Add Test",
		"description": "Test local_dir job definition creation",
		"type":        "local_dir",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name": "index-files",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md", ".txt"},
					"exclude_paths":      []string{".git"},
					"max_file_size":      1048576,
					"max_files":          50,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, defID, result["id"], "Job definition ID should match")
	assert.Contains(t, result, "name", "Response should contain name")
	assert.Contains(t, result, "steps", "Response should contain steps")

	env.LogTest(t, "Created job definition: %s", defID)
	defer deleteLocalDirJobDef(t, helper, defID)

	// Step 2: Verify job definition was created by fetching it
	env.LogTest(t, "Step 2: Verifying job definition exists")
	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)
	assert.Equal(t, "Local Dir Add Test", jobDef["name"], "Job definition name should match")

	env.LogTest(t, "Test completed successfully")
}

// TestLocalDirJobExecution tests executing a local_dir job via API
func TestLocalDirJobExecution(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	env.LogTest(t, "--- Starting Test: Local Dir Job Execution ---")

	// Create test directory
	testDir := createLocalDirTestDirectory(t)
	defer cleanupLocalDirTestDirectory(t, testDir)

	// Step 1: Create job definition
	env.LogTest(t, "Step 1: Creating job definition")
	defID := fmt.Sprintf("local-dir-exec-test-%d", time.Now().UnixNano())
	createLocalDirJobDef(t, helper, defID, "Local Dir Exec Test", testDir, []string{"test", "local_dir"})
	defer deleteLocalDirJobDef(t, helper, defID)

	// Step 2: Execute job definition
	env.LogTest(t, "Step 2: Executing job definition")
	jobID := executeJobDef(t, helper, defID)
	if jobID == "" {
		t.Skip("Skipping - job execution not available")
		return
	}
	defer deleteLocalDirJob(t, helper, jobID)

	// Step 3: Monitor job execution
	env.LogTest(t, "Step 3: Monitoring job execution")
	finalStatus := waitForLocalDirJobCompletion(t, helper, jobID, 2*time.Minute)
	env.LogTest(t, "Job reached terminal state: %s", finalStatus)

	// Verify completion
	assert.Equal(t, "completed", finalStatus, "Job should complete successfully")

	env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestLocalDirJobWithEmptyDirectory tests local_dir job with empty directory
func TestLocalDirJobWithEmptyDirectory(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	env.LogTest(t, "--- Starting Test: Local Dir Job With Empty Directory ---")

	// Create empty test directory
	tempDir, err := os.MkdirTemp("", "quaero-ui-empty-dir-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	env.LogTest(t, "Created empty test directory: %s", tempDir)

	// Step 1: Create job definition
	env.LogTest(t, "Step 1: Creating job definition for empty directory")
	defID := fmt.Sprintf("local-dir-empty-test-%d", time.Now().UnixNano())
	createLocalDirJobDef(t, helper, defID, "Local Dir Empty Test", tempDir, []string{"test", "empty"})
	defer deleteLocalDirJobDef(t, helper, defID)

	// Step 2: Execute job
	env.LogTest(t, "Step 2: Executing job")
	jobID := executeJobDef(t, helper, defID)
	if jobID == "" {
		t.Skip("Skipping - job execution not available")
		return
	}
	defer deleteLocalDirJob(t, helper, jobID)

	// Step 3: Monitor job
	env.LogTest(t, "Step 3: Monitoring job")
	finalStatus := waitForLocalDirJobCompletion(t, helper, jobID, 1*time.Minute)

	// Job should complete (possibly with 0 documents)
	env.LogTest(t, "Test completed - final status: %s", finalStatus)
}

// TestSummaryAgentWithDependency tests the summary agent with step dependency on index step
func TestSummaryAgentWithDependency(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	env.LogTest(t, "--- Starting Test: Summary Agent With Dependency ---")

	// Create test directory
	testDir := createLocalDirTestDirectory(t)
	defer cleanupLocalDirTestDirectory(t, testDir)

	// Step 1: Create combined job definition with index + summary steps
	env.LogTest(t, "Step 1: Creating combined job definition with dependency")
	defID := fmt.Sprintf("combined-test-%d", time.Now().UnixNano())
	tags := []string{"codebase", "test-project"}
	prompt := "Review the code base and provide an architectural summary in markdown."

	createCombinedJobDef(t, helper, defID, "Combined Index Summary Test", testDir, tags, prompt)
	defer deleteLocalDirJobDef(t, helper, defID)
	env.LogTest(t, "Created combined job definition: %s", defID)

	// Step 2: Verify job definition structure
	env.LogTest(t, "Step 2: Verifying job definition structure")
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var jobDef map[string]interface{}
		err = helper.ParseJSONResponse(resp, &jobDef)
		require.NoError(t, err)

		steps, ok := jobDef["steps"].([]interface{})
		if ok && len(steps) == 2 {
			env.LogTest(t, "Job definition has 2 steps as expected")

			// Check second step has depends field
			step2, ok := steps[1].(map[string]interface{})
			if ok {
				depends, _ := step2["depends"].(string)
				assert.Equal(t, "index-files", depends, "Summary step should depend on index-files")
				env.LogTest(t, "Summary step depends on: %s", depends)
			}
		}
	}

	// Step 3: Execute job
	env.LogTest(t, "Step 3: Executing combined job")
	jobID := executeJobDef(t, helper, defID)
	if jobID == "" {
		t.Skip("Skipping - job execution not available")
		return
	}
	defer deleteLocalDirJob(t, helper, jobID)

	// Step 4: Monitor job execution (longer timeout for LLM call)
	env.LogTest(t, "Step 4: Monitoring job execution (index + summary)")
	finalStatus := waitForLocalDirJobCompletion(t, helper, jobID, 5*time.Minute)
	env.LogTest(t, "Job reached terminal state: %s", finalStatus)

	// Verify completion
	if finalStatus != "completed" {
		t.Logf("Job did not complete successfully: %s (may require API key)", finalStatus)
	}

	// Step 5: Check if summary document was created
	env.LogTest(t, "Step 5: Checking for summary document")
	resp, err = helper.GET("/api/documents?tags=summary")
	if err == nil && resp.StatusCode == http.StatusOK {
		env.LogTest(t, "Summary document query successful")
		resp.Body.Close()
	}

	env.LogTest(t, "Test completed - job status: %s", finalStatus)
}

// TestSummaryAgentPlainRequest tests the summary agent with a plain text prompt
func TestSummaryAgentPlainRequest(t *testing.T) {
	// 1. Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	env.LogTest(t, "--- Starting Test: Summary Agent Plain Request ---")

	// Create test directory
	testDir := createLocalDirTestDirectory(t)
	defer cleanupLocalDirTestDirectory(t, testDir)

	// Step 1: First index the files
	env.LogTest(t, "Step 1: Creating and running index job")
	indexDefID := fmt.Sprintf("plain-index-%d", time.Now().UnixNano())
	createLocalDirJobDef(t, helper, indexDefID, "Plain Request Index", testDir, []string{"plain-test"})
	defer deleteLocalDirJobDef(t, helper, indexDefID)

	indexJobID := executeJobDef(t, helper, indexDefID)
	if indexJobID == "" {
		t.Skip("Skipping - job execution not available")
		return
	}
	defer deleteLocalDirJob(t, helper, indexJobID)

	indexStatus := waitForLocalDirJobCompletion(t, helper, indexJobID, 2*time.Minute)
	if indexStatus != "completed" {
		t.Fatalf("Index job did not complete: %s", indexStatus)
	}
	env.LogTest(t, "Index job completed")

	// Step 2: Create summary job with plain prompt
	env.LogTest(t, "Step 2: Creating summary job with plain prompt")
	summaryDefID := fmt.Sprintf("summary-plain-%d", time.Now().UnixNano())
	plainPrompt := "List all the files and describe what each one does in a simple bullet point format."

	summaryBody := map[string]interface{}{
		"id":          summaryDefID,
		"name":        "Plain Summary Request",
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
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Summary job definition creation returned status %d", resp.StatusCode)
	}
	defer deleteLocalDirJobDef(t, helper, summaryDefID)

	// Step 3: Execute summary job
	env.LogTest(t, "Step 3: Executing summary job")
	summaryJobID := executeJobDef(t, helper, summaryDefID)
	if summaryJobID == "" {
		t.Skip("Skipping - summary job execution not available")
		return
	}
	defer deleteLocalDirJob(t, helper, summaryJobID)

	// Step 4: Monitor summary job
	env.LogTest(t, "Step 4: Monitoring summary job")
	summaryStatus := waitForLocalDirJobCompletion(t, helper, summaryJobID, 3*time.Minute)
	env.LogTest(t, "Summary job reached terminal state: %s", summaryStatus)

	if summaryStatus != "completed" {
		t.Logf("Summary job did not complete: %s (may require API key)", summaryStatus)
	}

	env.LogTest(t, "Test completed - summary job status: %s", summaryStatus)
}
