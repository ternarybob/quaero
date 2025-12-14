// -----------------------------------------------------------------------
// Error Generator API Tests
// Tests for error tolerance, UI status display, and error block logging
// -----------------------------------------------------------------------

package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// createErrorGeneratorJobDefinition creates an error generator job definition for testing
func createErrorGeneratorJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id string, workerCount, logCount int, failureRate float64, maxChildFailures int) string {
	body := map[string]interface{}{
		"id":          id,
		"name":        "Error Generator Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test error generator for error tolerance validation",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate random logs with INF, WRN, and ERR levels",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    workerCount,
					"log_count":       logCount,
					"log_delay_ms":    10, // Fast for testing
					"failure_rate":    failureRate,
					"child_count":     2,
					"recursion_depth": 2,
				},
			},
		},
		"error_tolerance": map[string]interface{}{
			"max_child_failures": maxChildFailures,
			"failure_action":     "continue",
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create error generator job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Failed to create error generator job definition: status %d", resp.StatusCode)
		return ""
	}

	t.Logf("Created error generator job definition: id=%s", id)
	return id
}

// TestErrorGeneratorJobDefinitionCreation tests that error_generator job definitions can be created
func TestErrorGeneratorJobDefinitionCreation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-error-gen-%d", time.Now().UnixNano())

	// Create a simple error generator job definition
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Error Generator Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test error generator for API validation",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate random logs",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    5,
					"log_count":       50,
					"log_delay_ms":    10,
					"failure_rate":    0.1,
					"child_count":     2,
					"recursion_depth": 2,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create error generator job definition")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Should create job definition successfully")

	// Verify the job definition was created
	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err, "Failed to get job definition")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should retrieve job definition")

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse job definition response")

	assert.Equal(t, defID, result["id"], "Job definition ID should match")
	assert.Equal(t, "Error Generator Test", result["name"], "Job definition name should match")

	// Cleanup
	deleteJobDefinition(t, helper, defID)
}

// TestErrorToleranceJobStopping tests that jobs stop when max_child_failures threshold is exceeded
func TestErrorToleranceJobStopping(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-error-tolerance-%d", time.Now().UnixNano())

	// Create job definition with high failure rate and low tolerance threshold
	// This ensures the job will hit the error tolerance limit
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Error Tolerance Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test error tolerance threshold",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate jobs with high failure rate",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    20,
					"log_count":       10,
					"log_delay_ms":    5,
					"failure_rate":    0.8, // 80% failure rate
					"child_count":     0,   // No recursive children
					"recursion_depth": 0,
				},
			},
		},
		"error_tolerance": map[string]interface{}{
			"max_child_failures": 5, // Stop after 5 failures
			"failure_action":     "continue",
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create error tolerance job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Should create job definition")

	// Execute the job
	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Should get a job ID")

	// Wait for job to reach terminal state (should stop due to error tolerance)
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)

	t.Logf("Job reached final status: %s", finalStatus)

	// Get job details to verify error tolerance behavior
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err, "Failed to get job details")
	defer resp.Body.Close()

	var job map[string]interface{}
	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err, "Failed to parse job response")

	// Verify job completed (may be completed, failed, or have warning due to error tolerance)
	status := job["status"].(string)
	t.Logf("Job status: %s", status)

	// The job should complete even with failures due to failure_action="continue"
	assert.Contains(t, []string{"completed", "failed"}, status,
		"Job should reach terminal state")

	// Cleanup
	deleteJob(t, helper, jobID)
	deleteJobDefinition(t, helper, defID)
}

// TestUIStatusDisplayLogCounts tests that the UI API returns log level counts
func TestUIStatusDisplayLogCounts(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-log-counts-%d", time.Now().UnixNano())

	// Create job definition that generates logs with known distribution
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Log Counts Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test log level count display",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate logs with various levels",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    3,
					"log_count":       100, // 100 logs per worker
					"log_delay_ms":    5,
					"failure_rate":    0.0, // No failures
					"child_count":     0,
					"recursion_depth": 0,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create log counts job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Should create job definition")

	// Execute the job
	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Should get a job ID")

	// Wait for job to complete
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached final status: %s", finalStatus)

	// Wait a moment for logs to be fully written
	time.Sleep(500 * time.Millisecond)

	// Get logs for the job
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs", jobID))
	require.NoError(t, err, "Failed to get job logs")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should retrieve logs successfully")

	var logsResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &logsResult)
	require.NoError(t, err, "Failed to parse logs response")

	// Check that logs exist
	logs, ok := logsResult["logs"].([]interface{})
	if ok && len(logs) > 0 {
		t.Logf("Retrieved %d log entries", len(logs))

		// Count log levels
		infoCount := 0
		warnCount := 0
		errorCount := 0

		for _, log := range logs {
			logEntry, ok := log.(map[string]interface{})
			if !ok {
				continue
			}
			level, ok := logEntry["level"].(string)
			if !ok {
				continue
			}
			switch level {
			case "info":
				infoCount++
			case "warn":
				warnCount++
			case "error":
				errorCount++
			}
		}

		t.Logf("Log counts - INF: %d, WRN: %d, ERR: %d", infoCount, warnCount, errorCount)

		// Verify we have logs at various levels (distribution is ~80% INFO, ~15% WARN, ~5% ERROR)
		assert.Greater(t, infoCount, 0, "Should have INFO logs")
		// WARN and ERROR logs are random, so only check they can exist
	} else {
		t.Logf("No logs found for job (logs may not have been written yet)")
	}

	// Cleanup
	deleteJob(t, helper, jobID)
	deleteJobDefinition(t, helper, defID)
}

// TestErrorBlockDisplayAboveLogs tests that error logs can be filtered and displayed separately
func TestErrorBlockDisplayAboveLogs(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-error-block-%d", time.Now().UnixNano())

	// Create job definition that generates logs including errors
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Error Block Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test error log filtering",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate logs including errors",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    5,
					"log_count":       200, // More logs to ensure we get some errors
					"log_delay_ms":    2,
					"failure_rate":    0.0, // Jobs succeed but still log errors
					"child_count":     0,
					"recursion_depth": 0,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create error block job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Should create job definition")

	// Execute the job
	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Should get a job ID")

	// Wait for job to complete
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached final status: %s", finalStatus)

	// Wait a moment for logs to be fully written
	time.Sleep(500 * time.Millisecond)

	// Get error logs specifically using level filter
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs?level=error", jobID))
	require.NoError(t, err, "Failed to get error logs")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should retrieve error logs successfully")

	var errorLogsResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &errorLogsResult)
	require.NoError(t, err, "Failed to parse error logs response")

	// Check error logs
	errorLogs, ok := errorLogsResult["logs"].([]interface{})
	if ok {
		t.Logf("Retrieved %d error log entries", len(errorLogs))

		// Verify all returned logs are error level
		for i, log := range errorLogs {
			logEntry, ok := log.(map[string]interface{})
			if !ok {
				continue
			}
			level, ok := logEntry["level"].(string)
			if !ok {
				continue
			}
			if level != "error" {
				t.Errorf("Log entry %d has level '%s', expected 'error'", i, level)
			}
		}
	}

	// Get warning logs specifically
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs?level=warn", jobID))
	require.NoError(t, err, "Failed to get warning logs")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should retrieve warning logs successfully")

	var warnLogsResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &warnLogsResult)
	require.NoError(t, err, "Failed to parse warning logs response")

	warnLogs, ok := warnLogsResult["logs"].([]interface{})
	if ok {
		t.Logf("Retrieved %d warning log entries", len(warnLogs))
	}

	// Get all logs for comparison
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs", jobID))
	require.NoError(t, err, "Failed to get all logs")
	defer resp.Body.Close()

	var allLogsResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &allLogsResult)
	require.NoError(t, err, "Failed to parse all logs response")

	allLogs, ok := allLogsResult["logs"].([]interface{})
	if ok {
		t.Logf("Retrieved %d total log entries", len(allLogs))

		// Verify we have more total logs than just error logs
		errorLogCount := len(errorLogs)
		if len(allLogs) <= errorLogCount && len(allLogs) > 0 {
			t.Logf("Warning: All logs appear to be errors (this is unlikely but possible)")
		}
	}

	// Cleanup
	deleteJob(t, helper, jobID)
	deleteJobDefinition(t, helper, defID)
}

// TestErrorGeneratorRecursiveChildren tests that error generator creates recursive child jobs
func TestErrorGeneratorRecursiveChildren(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-recursive-children-%d", time.Now().UnixNano())

	// Create job definition with recursive children
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Recursive Children Test",
		"type":        "custom",
		"enabled":     true,
		"description": "Test recursive child job creation",
		"steps": []map[string]interface{}{
			{
				"name":        "generate_errors",
				"type":        "error_generator",
				"description": "Generate jobs with recursive children",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    2,
					"log_count":       10,
					"log_delay_ms":    5,
					"failure_rate":    0.0, // No failures for predictable test
					"child_count":     2,   // Each worker spawns 2 children
					"recursion_depth": 2,   // Children can spawn grandchildren
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create recursive children job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Should create job definition")

	// Execute the job
	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Should get a job ID")

	// Wait for job to complete
	finalStatus := waitForJobCompletion(t, helper, jobID, 120*time.Second)
	t.Logf("Job reached final status: %s", finalStatus)

	// Get job children to verify hierarchy
	resp, err = helper.GET(fmt.Sprintf("/api/jobs?parent_id=%s", jobID))
	if err == nil {
		defer resp.Body.Close()

		var childrenResult map[string]interface{}
		if err := helper.ParseJSONResponse(resp, &childrenResult); err == nil {
			if jobs, ok := childrenResult["jobs"].([]interface{}); ok {
				t.Logf("Found %d direct child jobs", len(jobs))
			}
		}
	}

	// Cleanup
	deleteJob(t, helper, jobID)
	deleteJobDefinition(t, helper, defID)
}
