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

// Codebase Classify job definition ID (from test/config/job-definitions/codebase_classify.toml)
const codebaseClassifyJobID = "codebase_classify"

// TestLogsAPIJobScope tests the /api/logs endpoint with scope=job
// This test creates a job, waits for logs to be generated, and verifies retrieval timing.
func TestLogsAPIJobScope(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Create and execute a test job
	defID := fmt.Sprintf("test-logs-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Log Test Job", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Job ID should not be empty")
	defer deleteJob(t, helper, jobID)

	// Wait for job to generate some logs (short timeout as we just need some activity)
	time.Sleep(2 * time.Second)

	// Test 1: Get job logs with timing
	t.Run("GetJobLogsWithTiming", func(t *testing.T) {
		start := time.Now()
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=all&limit=100", jobID))
		elapsed := time.Since(start)

		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		// Verify response structure
		assert.Equal(t, "job", result["scope"], "Scope should be 'job'")
		assert.Equal(t, jobID, result["job_id"], "Job ID should match")

		logs, ok := result["logs"].([]interface{})
		require.True(t, ok, "Response should contain logs array")

		count, ok := result["count"].(float64)
		require.True(t, ok, "Response should contain count")

		t.Logf("Retrieved %d logs in %v (count: %.0f)", len(logs), elapsed, count)
		t.Logf("API response time: %v", elapsed)

		// Performance check: API should respond within 1 second for reasonable log counts
		assert.Less(t, elapsed.Milliseconds(), int64(1000), "API should respond within 1 second")
	})

	// Test 2: Get job logs with level filter
	t.Run("GetJobLogsWithLevelFilter", func(t *testing.T) {
		start := time.Now()
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=info&limit=50", jobID))
		elapsed := time.Since(start)

		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		assert.Equal(t, "info", result["level"], "Level filter should be 'info'")
		t.Logf("Retrieved info-level logs in %v", elapsed)
	})

	// Test 3: Get job logs with ascending order
	t.Run("GetJobLogsAscendingOrder", func(t *testing.T) {
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=all&order=asc", jobID))
		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		assert.Equal(t, "asc", result["order"], "Order should be 'asc'")

		logs, ok := result["logs"].([]interface{})
		if ok && len(logs) >= 2 {
			// Verify chronological order (oldest first)
			first := logs[0].(map[string]interface{})
			last := logs[len(logs)-1].(map[string]interface{})

			firstTS, _ := first["full_timestamp"].(string)
			lastTS, _ := last["full_timestamp"].(string)

			if firstTS != "" && lastTS != "" {
				assert.LessOrEqual(t, firstTS, lastTS, "Logs should be in ascending chronological order")
				t.Logf("Log order verified: first=%s, last=%s", firstTS, lastTS)
			}
		}
	})

	// Test 4: Include children option
	t.Run("GetJobLogsWithChildren", func(t *testing.T) {
		start := time.Now()
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&include_children=true&level=all", jobID))
		elapsed := time.Since(start)

		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		includeChildren, ok := result["include_children"].(bool)
		require.True(t, ok, "Response should contain include_children")
		assert.True(t, includeChildren, "include_children should be true")

		t.Logf("Retrieved logs with children in %v", elapsed)
	})

	// Wait for job completion before cleanup
	waitForJobCompletion(t, helper, jobID, 30*time.Second)
}

// TestLogsAPIServiceScope tests the /api/logs endpoint with scope=service
func TestLogsAPIServiceScope(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Test service logs retrieval
	t.Run("GetServiceLogs", func(t *testing.T) {
		start := time.Now()
		resp, err := helper.GET("/api/logs?scope=service&level=all&limit=100")
		elapsed := time.Since(start)

		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		// Verify response structure
		assert.Equal(t, "service", result["scope"], "Scope should be 'service'")

		logs, ok := result["logs"].([]interface{})
		require.True(t, ok, "Response should contain logs array")

		t.Logf("Retrieved %d service logs in %v", len(logs), elapsed)

		// Performance check
		assert.Less(t, elapsed.Milliseconds(), int64(500), "Service logs should be fast (in-memory)")
	})

	// Test default scope (should be service)
	t.Run("DefaultScopeIsService", func(t *testing.T) {
		resp, err := helper.GET("/api/logs")
		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		assert.Equal(t, "service", result["scope"], "Default scope should be 'service'")
	})
}

// TestLogsAPIValidation tests validation and error handling
func TestLogsAPIValidation(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Test missing job_id for job scope
	t.Run("MissingJobIDForJobScope", func(t *testing.T) {
		resp, err := helper.GET("/api/logs?scope=job")
		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusBadRequest)
		t.Log("Correctly rejected request without job_id")
	})

	// Test invalid scope
	t.Run("InvalidScope", func(t *testing.T) {
		resp, err := helper.GET("/api/logs?scope=invalid")
		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusBadRequest)
		t.Log("Correctly rejected invalid scope")
	})

	// Test non-existent job
	t.Run("NonExistentJob", func(t *testing.T) {
		resp, err := helper.GET("/api/logs?scope=job&job_id=non-existent-job-id")
		require.NoError(t, err, "Failed to call logs endpoint")
		// Should return 404 for non-existent job
		assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusOK,
			"Should return 404 for non-existent job or empty logs")
		t.Logf("Response for non-existent job: %d", resp.StatusCode)
	})
}

// TestLogsAPICodebaseClassify tests logging with the Codebase Classify job
// This uses the pre-configured job definition from test/config/job-definitions/codebase_classify.toml
func TestLogsAPICodebaseClassify(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Check if the codebase_classify job definition exists
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s", codebaseClassifyJobID))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("codebase_classify job definition not available")
	}
	resp.Body.Close()

	t.Logf("Found codebase_classify job definition")

	// Execute the job
	execResp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", codebaseClassifyJobID), nil)
	require.NoError(t, err, "Failed to execute codebase_classify")

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Could not execute codebase_classify: status %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	err = helper.ParseJSONResponse(execResp, &execResult)
	require.NoError(t, err, "Failed to parse execution response")

	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	t.Logf("Executed codebase_classify -> job_id=%s", jobID)

	defer func() {
		// Cancel the job on cleanup to avoid long-running jobs
		helper.POST(fmt.Sprintf("/api/jobs/%s/cancel", jobID), nil)
		deleteJob(t, helper, jobID)
	}()

	// Wait for initial logs to be generated
	t.Log("Waiting for job to generate logs...")
	time.Sleep(5 * time.Second)

	// Test log retrieval with timing
	t.Run("RetrieveCodebaseClassifyLogs", func(t *testing.T) {
		start := time.Now()
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=all&limit=200&include_children=true", jobID))
		elapsed := time.Since(start)

		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		logs, ok := result["logs"].([]interface{})
		require.True(t, ok, "Response should contain logs array")

		t.Logf("Retrieved %d logs in %v", len(logs), elapsed)

		// Log some sample entries
		if len(logs) > 0 {
			t.Log("Sample log entries:")
			for i := 0; i < min(5, len(logs)); i++ {
				entry := logs[i].(map[string]interface{})
				t.Logf("  [%s] %s: %s",
					entry["timestamp"],
					entry["level"],
					truncateString(entry["message"].(string), 80))
			}
		}

		// Check metadata for child jobs
		if metadata, ok := result["metadata"].(map[string]interface{}); ok {
			t.Logf("Metadata contains %d job entries", len(metadata))
		}

		// Performance: should respond in reasonable time even with child logs
		assert.Less(t, elapsed.Milliseconds(), int64(2000),
			"Log retrieval with children should complete within 2 seconds")
	})

	// Test level filtering
	t.Run("FilterByLogLevel", func(t *testing.T) {
		// Get only INFO level logs
		resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=info&limit=100", jobID))
		require.NoError(t, err, "Failed to call logs endpoint")
		helper.AssertStatusCode(resp, http.StatusOK)

		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		require.NoError(t, err, "Failed to parse logs response")

		assert.Equal(t, "info", result["level"], "Level filter should be applied")
		t.Logf("Info-level logs: %v entries", result["count"])
	})
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// min returns the smaller of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestLogsAPIPerformance measures log retrieval performance under load
func TestLogsAPIPerformance(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Create a job that generates logs
	defID := fmt.Sprintf("test-perf-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Performance Test Job", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Job ID should not be empty")
	defer deleteJob(t, helper, jobID)

	// Wait for job to generate logs
	time.Sleep(3 * time.Second)

	// Run multiple retrieval requests and measure timing
	t.Run("RepeatedLogRetrieval", func(t *testing.T) {
		const iterations = 5
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			start := time.Now()
			resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=all", jobID))
			elapsed := time.Since(start)
			totalDuration += elapsed

			require.NoError(t, err, "Failed to call logs endpoint")
			helper.AssertStatusCode(resp, http.StatusOK)
			resp.Body.Close()

			t.Logf("Iteration %d: %v", i+1, elapsed)
		}

		avgDuration := totalDuration / iterations
		t.Logf("Average retrieval time over %d iterations: %v", iterations, avgDuration)

		// Average should be under 500ms for acceptable performance
		assert.Less(t, avgDuration.Milliseconds(), int64(500),
			"Average log retrieval should be under 500ms")
	})

	// Wait for job completion before cleanup
	waitForJobCompletion(t, helper, jobID, 30*time.Second)
}
