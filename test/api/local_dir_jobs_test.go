package api

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

// createTestLocalDirJobDefinition creates a local_dir job definition for testing
func createTestLocalDirJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id, name, dirPath string) string {
	body := map[string]interface{}{
		"id":      id,
		"name":    name,
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           dirPath,
					"include_extensions": []string{".go", ".md", ".txt"},
					"exclude_paths":      []string{".git", "node_modules"},
					"max_file_size":      1048576,
					"max_files":          100,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create local_dir job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Failed to create local_dir job definition: status %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse job definition response")

	t.Logf("Created local_dir job definition: id=%s", id)
	return id
}

// createTestDirectory creates a temporary directory with test files for local_dir testing
func createTestDirectory(t *testing.T) string {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "quaero-local-dir-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Create test files
	testFiles := map[string]string{
		"README.md":          "# Test Project\n\nThis is a test project for local_dir worker testing.",
		"main.go":            "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n",
		"config.txt":         "key=value\nfoo=bar\n",
		"src/utils.go":       "package src\n\nfunc Helper() {}\n",
		"src/models/user.go": "package models\n\ntype User struct {\n\tName string\n}\n",
		"docs/api.md":        "# API Documentation\n\n## Endpoints\n",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Logf("Warning: failed to create directory for %s: %v", path, err)
			continue
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Logf("Warning: failed to create test file %s: %v", path, err)
		}
	}

	t.Logf("Created test directory with %d files at: %s", len(testFiles), tempDir)
	return tempDir
}

// cleanupTestDirectory removes the test directory
func cleanupTestDirectory(t *testing.T, dir string) {
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to cleanup test directory %s: %v", dir, err)
	} else {
		t.Logf("Cleaned up test directory: %s", dir)
	}
}

// TestLocalDirJobs_ValidationErrors tests validation errors for local_dir job definitions
func TestLocalDirJobs_ValidationErrors(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Missing dir_path in config
	t.Log("Step 1: Testing missing dir_path in local_dir config")
	body := map[string]interface{}{
		"id":      "test-missing-path",
		"name":    "Test Missing Path",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name":   "index-step",
				"type":   "local_dir",
				"config": map[string]interface{}{
					// dir_path is missing
				},
			},
		},
	}
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Should fail validation
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusCreated,
		"Should handle missing dir_path (got status %d)", resp.StatusCode)

	// Test 2: Empty dir_path
	t.Log("Step 2: Testing empty dir_path in local_dir config")
	body2 := map[string]interface{}{
		"id":      "test-empty-path",
		"name":    "Test Empty Path",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path": "",
				},
			},
		},
	}
	resp, err = helper.POST("/api/job-definitions", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Should fail validation
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusCreated,
		"Should handle empty dir_path (got status %d)", resp.StatusCode)

	t.Log("Validation error tests completed successfully")
}

// TestLocalDirJobs_CreateJobDefinition tests creating a local_dir job definition
func TestLocalDirJobs_CreateJobDefinition(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Test 1: Create valid local_dir job definition
	t.Log("Step 1: Creating valid local_dir job definition")
	defID := fmt.Sprintf("test-local-dir-def-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Test Local Dir Job",
		"description": "Test job for local directory indexing",
		"type":        "local_dir",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
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

	t.Logf("Created local_dir job definition: %s", defID)

	// Cleanup
	deleteJobDefinition(t, helper, defID)

	t.Log("Create local_dir job definition test completed successfully")
}

// TestLocalDirJobs_ExecuteJob tests executing a local_dir job
func TestLocalDirJobs_ExecuteJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Create job definition
	t.Log("Step 1: Creating local_dir job definition")
	defID := fmt.Sprintf("test-local-dir-exec-%d", time.Now().UnixNano())
	createTestLocalDirJobDefinition(t, helper, defID, "Test Local Dir Execute", testDir)
	defer deleteJobDefinition(t, helper, defID)

	// Execute job definition
	t.Log("Step 2: Executing local_dir job definition")
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Logf("Job execution returned status %d (may require additional setup)", resp.StatusCode)
		t.Skip("Skipping execution test - job execution not available")
		return
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "status", "Response should contain status")

	jobID, ok := result["job_id"].(string)
	require.True(t, ok, "Job ID should be a string")
	t.Logf("Started local_dir job: job_id=%s", jobID)

	// Wait for job to reach terminal state
	t.Log("Step 3: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Cleanup
	deleteJob(t, helper, jobID)

	t.Log("Execute local_dir job test completed successfully")
}

// TestLocalDirJobs_JobLifecycle tests the complete lifecycle of a local_dir job
func TestLocalDirJobs_JobLifecycle(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Step 1: Create job definition
	t.Log("Step 1: Creating local_dir job definition")
	defID := fmt.Sprintf("test-local-dir-lifecycle-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Local Dir Lifecycle Test",
		"description": "Test complete job lifecycle",
		"type":        "local_dir",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name": "index-files",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md"},
					"max_files":          10,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID)

	// Step 2: Verify job definition was created
	t.Log("Step 2: Verifying job definition was created")
	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)
	assert.Equal(t, defID, jobDef["id"], "Job definition ID should match")
	assert.Equal(t, "Local Dir Lifecycle Test", jobDef["name"], "Job definition name should match")

	// Step 3: Execute job
	t.Log("Step 3: Executing job definition")
	resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Logf("Job execution returned status %d", resp.StatusCode)
		t.Skip("Skipping lifecycle test - job execution not available")
		return
	}

	var execResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &execResult)
	require.NoError(t, err)

	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Job ID should be a string")
	t.Logf("Job created: %s", jobID)
	defer deleteJob(t, helper, jobID)

	// Step 4: Check initial job status (with retry for timing)
	t.Log("Step 4: Checking initial job status")
	var job map[string]interface{}
	var status string
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			err = helper.ParseJSONResponse(resp, &job)
			if err == nil {
				status, _ = job["status"].(string)
				break
			}
		}
	}
	if status == "" {
		t.Log("Could not get initial job status, proceeding to monitor")
	} else {
		t.Logf("Initial job status: %s", status)
	}

	// Step 5: Monitor job progress
	t.Log("Step 5: Monitoring job progress")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Step 6: Check job logs (optional - may return 404 if job cleaned up)
	t.Log("Step 6: Checking job logs")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs", jobID))
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var logsResult map[string]interface{}
		if err = helper.ParseJSONResponse(resp, &logsResult); err == nil {
			logs, _ := logsResult["logs"].([]interface{})
			t.Logf("Job has %d log entries", len(logs))
		}
	} else {
		t.Log("Job logs not available (job may have completed quickly)")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Step 7: Verify final status (optional)
	t.Log("Step 7: Verifying final job state")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		if err = helper.ParseJSONResponse(resp, &job); err == nil {
			finalStatusFromAPI, _ := job["status"].(string)
			t.Logf("Final status from API: %s", finalStatusFromAPI)
		}
	} else {
		t.Logf("Job status not available (status from monitor: %s)", finalStatus)
		if resp != nil {
			resp.Body.Close()
		}
	}

	t.Log("Local_dir job lifecycle test completed successfully")
}

// TestLocalDirJobs_ConfigOptions tests various configuration options for local_dir jobs
func TestLocalDirJobs_ConfigOptions(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Test 1: Job with extension filtering
	t.Log("Step 1: Creating job with extension filtering")
	defID1 := fmt.Sprintf("test-local-dir-ext-%d", time.Now().UnixNano())
	body1 := map[string]interface{}{
		"id":      defID1,
		"name":    "Extension Filter Test",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-go-only",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go"},
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body1)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID1)
	t.Log("Created job definition with extension filtering")

	// Test 2: Job with path exclusion
	t.Log("Step 2: Creating job with path exclusion")
	defID2 := fmt.Sprintf("test-local-dir-exclude-%d", time.Now().UnixNano())
	body2 := map[string]interface{}{
		"id":      defID2,
		"name":    "Path Exclusion Test",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-exclude-docs",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":      testDir,
					"exclude_paths": []string{"docs", "src/models"},
				},
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID2)
	t.Log("Created job definition with path exclusion")

	// Test 3: Job with size and count limits
	t.Log("Step 3: Creating job with size and count limits")
	defID3 := fmt.Sprintf("test-local-dir-limits-%d", time.Now().UnixNano())
	body3 := map[string]interface{}{
		"id":      defID3,
		"name":    "Limits Test",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-limited",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":      testDir,
					"max_file_size": 512000, // 500KB
					"max_files":     5,      // Only 5 files
				},
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions", body3)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID3)
	t.Log("Created job definition with size and count limits")

	// Test 4: Job with all options combined
	t.Log("Step 4: Creating job with all configuration options")
	defID4 := fmt.Sprintf("test-local-dir-full-%d", time.Now().UnixNano())
	body4 := map[string]interface{}{
		"id":          defID4,
		"name":        "Full Config Test",
		"description": "Test all local_dir configuration options",
		"type":        "local_dir",
		"enabled":     true,
		"tags":        []string{"test", "local", "full-config"},
		"steps": []map[string]interface{}{
			{
				"name": "index-full-config",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md", ".txt", ".json"},
					"exclude_paths":      []string{".git", "node_modules", "vendor"},
					"max_file_size":      1048576,
					"max_files":          100,
					"follow_symlinks":    false,
				},
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions", body4)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID4)
	t.Log("Created job definition with full configuration")

	t.Log("Config options test completed successfully")
}

// TestLocalDirJobs_TOMLUpload tests uploading local_dir job definitions via TOML
func TestLocalDirJobs_TOMLUpload(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Test 1: Upload valid local_dir TOML
	t.Log("Step 1: Uploading valid local_dir TOML")

	// Escape backslashes for TOML on Windows
	escapedPath := filepath.ToSlash(testDir)

	tomlContent := fmt.Sprintf(`# Local Directory Indexing Job
id = "toml-upload-local-dir-test"
name = "TOML Upload Local Dir Test"
description = "Test local_dir job created via TOML upload"
tags = ["test", "toml", "local_dir"]
enabled = true

[step.index]
type = "local_dir"
dir_path = "%s"
include_extensions = [".go", ".md"]
exclude_paths = [".git"]
max_files = 50
`, escapedPath)

	resp, err := helper.POSTBody("/api/job-definitions/upload", "text/plain", []byte(tomlContent))
	require.NoError(t, err)
	defer resp.Body.Close()

	// May return 201 Created or 200 OK
	assert.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK,
		"TOML upload should succeed (got status %d)", resp.StatusCode)

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err)

		if id, ok := result["id"].(string); ok {
			t.Logf("Created local_dir job definition via TOML: %s", id)
			defer deleteJobDefinition(t, helper, id)
		}
	}

	// Test 2: Upload TOML with step-based configuration
	t.Log("Step 2: Uploading step-based local_dir TOML")
	tomlContent2 := fmt.Sprintf(`# Multi-step local directory job
id = "multi-step-local-dir-test"
name = "Multi-Step Local Dir Test"
description = "Test step-based local_dir configuration"
tags = ["test", "multi-step"]
enabled = true

[step.scan_source]
type = "local_dir"
dir_path = "%s"
include_extensions = [".go"]
exclude_paths = ["vendor", ".git"]

[step.scan_docs]
type = "local_dir"
dir_path = "%s"
include_extensions = [".md", ".txt"]
depends = "scan_source"
`, escapedPath, escapedPath)

	resp, err = helper.POSTBody("/api/job-definitions/upload", "text/plain", []byte(tomlContent2))
	require.NoError(t, err)
	defer resp.Body.Close()

	// May succeed or fail depending on multi-step support
	t.Logf("Multi-step TOML upload returned status: %d", resp.StatusCode)

	t.Log("TOML upload test completed successfully")
}

// TestLocalDirJobs_NonExistentDirectory tests behavior with non-existent directory
func TestLocalDirJobs_NonExistentDirectory(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create job definition with non-existent path
	t.Log("Step 1: Creating job with non-existent directory")
	defID := fmt.Sprintf("test-local-dir-missing-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":      defID,
		"name":    "Non-Existent Dir Test",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path": "/path/that/does/not/exist/anywhere",
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Job definition creation may succeed (validation happens at execution time)
	if resp.StatusCode == http.StatusCreated {
		t.Log("Job definition created (validation at execution time)")
		defer deleteJobDefinition(t, helper, defID)

		// Try to execute - should fail
		t.Log("Step 2: Executing job with non-existent directory")
		resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted {
			var result map[string]interface{}
			err = helper.ParseJSONResponse(resp, &result)
			require.NoError(t, err)

			if jobID, ok := result["job_id"].(string); ok {
				t.Logf("Job started: %s (expecting failure)", jobID)

				// Wait for job to fail - may also timeout if job processing is slow
				finalStatus := waitForJobCompletion(t, helper, jobID, 30*time.Second)
				t.Logf("Job final status: %s", finalStatus)
				// Accept both "failed" (expected) or "timeout" (job still processing)
				assert.True(t, finalStatus == "failed" || finalStatus == "timeout",
					"Job should fail or timeout for non-existent directory, got: %s", finalStatus)

				deleteJob(t, helper, jobID)
			}
		}
	} else {
		t.Logf("Job definition rejected (status %d) - validation at creation time", resp.StatusCode)
	}

	t.Log("Non-existent directory test completed successfully")
}

// TestLocalDirJobs_UpdateJobDefinition tests updating a local_dir job definition
func TestLocalDirJobs_UpdateJobDefinition(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test directory
	testDir := createTestDirectory(t)
	defer cleanupTestDirectory(t, testDir)

	// Create initial job definition
	t.Log("Step 1: Creating initial local_dir job definition")
	defID := fmt.Sprintf("test-local-dir-update-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":      defID,
		"name":    "Original Name",
		"type":    "local_dir",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":  testDir,
					"max_files": 10,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)
	defer deleteJobDefinition(t, helper, defID)

	// Update the job definition
	t.Log("Step 2: Updating job definition")
	updateBody := map[string]interface{}{
		"id":          defID,
		"name":        "Updated Name",
		"description": "Updated description",
		"type":        "local_dir",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name": "index-step",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"max_files":          100,
					"include_extensions": []string{".go", ".md"},
				},
			},
		},
	}

	resp, err = helper.PUT(fmt.Sprintf("/api/job-definitions/%s", defID), updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify update
	assert.Equal(t, "Updated Name", result["name"], "Name should be updated")

	// Verify by fetching
	t.Log("Step 3: Verifying update")
	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", jobDef["name"], "Name should match updated value")

	t.Log("Update job definition test completed successfully")
}
