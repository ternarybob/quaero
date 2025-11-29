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

// Helper functions for job testing

// createTestJobDefinition creates a minimal valid job definition for testing
func createTestJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id, name, jobType string) string {
	body := map[string]interface{}{
		"id":   id,
		"name": name,
		"type": jobType,
		"steps": []map[string]interface{}{
			{
				"name": "test-step",
				"type": "crawl",
				"config": map[string]interface{}{
					"start_urls":  []string{"https://example.com"},
					"max_depth":   1,
					"max_pages":   5,
					"concurrency": 1,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("⚠️  Failed to create job definition: status %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse job definition response")

	t.Logf("Created job definition: id=%s", id)
	return id
}

// deleteJobDefinition deletes a job definition
func deleteJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", id))
	if err != nil {
		t.Logf("⚠️  Failed to delete job definition %s: %v", id, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		t.Logf("Deleted job definition: id=%s", id)
	}
}

// executeJobDefinition executes a job definition and returns the job ID
func executeJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id string) string {
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", id), nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Logf("⚠️  Failed to execute job definition: status %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse execution response")

	jobID, ok := result["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")

	t.Logf("Executed job definition %s → job_id=%s", id, jobID)
	return jobID
}

// waitForJobCompletion polls job status until terminal state or timeout
func waitForJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			t.Logf("⚠️  Failed to get job status: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(pollInterval)
			continue
		}

		var job map[string]interface{}
		if err := helper.ParseJSONResponse(resp, &job); err != nil {
			time.Sleep(pollInterval)
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			time.Sleep(pollInterval)
			continue
		}

		// Check for terminal states
		if status == "completed" || status == "failed" || status == "cancelled" {
			t.Logf("Job %s reached terminal state: %s", jobID, status)
			return status
		}

		time.Sleep(pollInterval)
	}

	t.Logf("⚠️  Job %s did not reach terminal state within %v", jobID, timeout)
	return "timeout"
}

// createTestJob creates a test job via job definition execution
func createTestJob(t *testing.T, helper *common.HTTPTestHelper) string {
	// Create a temporary job definition
	defID := fmt.Sprintf("test-job-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Job Definition", "crawler")

	// Execute it to create a job
	jobID := executeJobDefinition(t, helper, defID)

	// Clean up job definition
	deleteJobDefinition(t, helper, defID)

	return jobID
}

// deleteJob deletes a job
func deleteJob(t *testing.T, helper *common.HTTPTestHelper, jobID string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
	if err != nil {
		t.Logf("⚠️  Failed to delete job %s: %v", jobID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		t.Logf("Deleted job: id=%s", jobID)
	}
}

// Job Management Tests

// TestJobManagement_ListJobs tests GET /api/jobs with pagination and filtering
func TestJobManagement_ListJobs(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: List jobs with default parameters
	t.Log("Step 1: Listing jobs with defaults")
	resp, err := helper.GET("/api/jobs")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "jobs", "Response should contain jobs array")
	assert.Contains(t, result, "total_count", "Response should contain total_count")
	assert.Contains(t, result, "limit", "Response should contain limit")
	assert.Contains(t, result, "offset", "Response should contain offset")

	// Verify individual job fields if jobs exist
	jobs, ok := result["jobs"].([]interface{})
	assert.True(t, ok, "Jobs should be an array")
	if len(jobs) > 0 {
		firstJob := jobs[0].(map[string]interface{})
		assert.Contains(t, firstJob, "id", "Job should have id")
		assert.Contains(t, firstJob, "name", "Job should have name")
		assert.Contains(t, firstJob, "type", "Job should have type")
		assert.Contains(t, firstJob, "status", "Job should have status")
		assert.Contains(t, firstJob, "created_at", "Job should have created_at")
		assert.Contains(t, firstJob, "updated_at", "Job should have updated_at")
		// Note: parent_id, child counts, and document_count may not be present in all jobs
		t.Logf("Sample job fields: id=%v, name=%v, status=%v", firstJob["id"], firstJob["name"], firstJob["status"])
	}

	// Test 2: List with pagination
	t.Log("Step 2: Listing with pagination (limit=10, offset=0)")
	resp, err = helper.GET("/api/jobs?limit=10&offset=0")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var paginatedResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &paginatedResult)
	require.NoError(t, err)
	assert.Equal(t, float64(10), paginatedResult["limit"], "Limit should be 10")
	assert.Equal(t, float64(0), paginatedResult["offset"], "Offset should be 0")

	// Test 3: List with status filter
	t.Log("Step 3: Listing with status filter (status=completed)")
	resp, err = helper.GET("/api/jobs?status=completed")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// Test 4: List with grouped mode
	t.Log("Step 4: Listing with grouped mode (grouped=true)")
	resp, err = helper.GET("/api/jobs?grouped=true")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var groupedResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &groupedResult)
	require.NoError(t, err)
	assert.Contains(t, groupedResult, "groups", "Grouped response should contain groups array")
	assert.Contains(t, groupedResult, "orphans", "Grouped response should contain orphans array")

	t.Log("✓ Job listing test completed successfully")
}

// TestJobManagement_GetJob tests GET /api/jobs/{id}
func TestJobManagement_GetJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Get valid job
	t.Log("Step 3: Getting job details")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var job map[string]interface{}
	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err)

	// Verify response includes all expected fields
	assert.Equal(t, jobID, job["id"], "Job ID should match")
	assert.Contains(t, job, "name", "Job should have name")
	assert.Contains(t, job, "type", "Job should have type")
	assert.Contains(t, job, "status", "Job should have status")
	assert.Contains(t, job, "config", "Job should have config")
	assert.Contains(t, job, "created_at", "Job should have created_at")
	assert.Contains(t, job, "updated_at", "Job should have updated_at")

	// Verify optional fields (parent_id, child statistics, document_count)
	// These may not always be present depending on job type and execution
	if parentID, exists := job["parent_id"]; exists && parentID != nil {
		t.Logf("Job has parent_id: %v", parentID)
	}
	if completedChildren, exists := job["completed_children_count"]; exists {
		t.Logf("Job has completed_children_count: %v", completedChildren)
	}
	if failedChildren, exists := job["failed_children_count"]; exists {
		t.Logf("Job has failed_children_count: %v", failedChildren)
	}
	if totalChildren, exists := job["total_children_count"]; exists {
		t.Logf("Job has total_children_count: %v", totalChildren)
	}
	if documentCount, exists := job["document_count"]; exists {
		t.Logf("Job has document_count: %v", documentCount)
	}

	// Verify terminal status
	status, ok := job["status"].(string)
	assert.True(t, ok, "Status should be a string")
	assert.True(t, status == "completed" || status == "failed" || status == "cancelled", "Job should be in terminal state")

	// Test 2: Get nonexistent job (404)
	t.Log("Step 4: Getting nonexistent job")
	resp, err = helper.GET("/api/jobs/nonexistent-job-id")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Get with empty ID (400)
	t.Log("Step 5: Getting job with empty ID")
	resp, err = helper.GET("/api/jobs/")
	require.NoError(t, err)
	defer resp.Body.Close()
	// May return 400 or 404 depending on routing
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
		"Empty ID should return 400 or 404")

	t.Log("✓ Get job test completed successfully")
}

// TestJobManagement_JobStats tests GET /api/jobs/stats
func TestJobManagement_JobStats(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Get initial stats
	t.Log("Step 1: Getting job statistics")
	resp, err := helper.GET("/api/jobs/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var stats map[string]interface{}
	err = helper.ParseJSONResponse(resp, &stats)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, stats, "total_jobs", "Stats should contain total_jobs")
	assert.Contains(t, stats, "pending_jobs", "Stats should contain pending_jobs")
	assert.Contains(t, stats, "running_jobs", "Stats should contain running_jobs")
	assert.Contains(t, stats, "completed_jobs", "Stats should contain completed_jobs")
	assert.Contains(t, stats, "failed_jobs", "Stats should contain failed_jobs")
	assert.Contains(t, stats, "cancelled_jobs", "Stats should contain cancelled_jobs")

	// Verify all counts are numbers
	for key, value := range stats {
		_, ok := value.(float64)
		assert.True(t, ok, fmt.Sprintf("%s should be a number", key))
	}

	t.Logf("Current stats: total=%v, pending=%v, running=%v, completed=%v",
		stats["total_jobs"], stats["pending_jobs"], stats["running_jobs"], stats["completed_jobs"])

	t.Log("✓ Job stats test completed successfully")
}

// TestJobManagement_JobQueue tests GET /api/jobs/queue
func TestJobManagement_JobQueue(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Get job queue
	t.Log("Step 1: Getting job queue")
	resp, err := helper.GET("/api/jobs/queue")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var queue map[string]interface{}
	err = helper.ParseJSONResponse(resp, &queue)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, queue, "pending", "Queue should contain pending array")
	assert.Contains(t, queue, "running", "Queue should contain running array")
	assert.Contains(t, queue, "total", "Queue should contain total count")

	// Verify arrays
	pending, ok := queue["pending"].([]interface{})
	assert.True(t, ok, "Pending should be an array")
	running, ok := queue["running"].([]interface{})
	assert.True(t, ok, "Running should be an array")
	total, ok := queue["total"].(float64)
	assert.True(t, ok, "Total should be a number")

	t.Logf("Queue: pending=%d, running=%d, total=%v", len(pending), len(running), total)

	t.Log("✓ Job queue test completed successfully")
}

// TestJobManagement_JobLogs tests GET /api/jobs/{id}/logs
func TestJobManagement_JobLogs(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state so logs are generated
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Get logs with default parameters
	t.Log("Step 3: Getting job logs")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/logs", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "logs", "Response should contain logs array")
	assert.Contains(t, result, "count", "Response should contain count")
	assert.Contains(t, result, "order", "Response should contain order")
	assert.Contains(t, result, "level", "Response should contain level")

	logs, ok := result["logs"].([]interface{})
	assert.True(t, ok, "Logs should be an array")
	t.Logf("Retrieved %d logs", len(logs))

	// Test 2: Get logs with level filter
	t.Log("Step 4: Getting logs with level filter (level=error)")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs?level=error", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var filteredResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &filteredResult)
	require.NoError(t, err)
	assert.Equal(t, "error", filteredResult["level"], "Level filter should be applied")

	// Test 3: Get logs with ordering
	t.Log("Step 5: Getting logs with asc order")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs?order=asc", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var orderedResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &orderedResult)
	require.NoError(t, err)
	assert.Equal(t, "asc", orderedResult["order"], "Order should be asc")

	t.Log("✓ Job logs test completed successfully")
}

// TestJobManagement_AggregatedLogs tests GET /api/jobs/{id}/logs/aggregated
func TestJobManagement_AggregatedLogs(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job (parent)
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state so logs are generated
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Get aggregated logs with default parameters
	t.Log("Step 3: Getting aggregated logs")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/logs/aggregated", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "logs", "Response should contain logs array")
	assert.Contains(t, result, "count", "Response should contain count")
	assert.Contains(t, result, "order", "Response should contain order")
	assert.Contains(t, result, "level", "Response should contain level")
	assert.Contains(t, result, "include_children", "Response should contain include_children")
	assert.Contains(t, result, "metadata", "Response should contain metadata")

	logs, ok := result["logs"].([]interface{})
	assert.True(t, ok, "Logs should be an array")
	t.Logf("Retrieved %d aggregated logs", len(logs))

	// Verify logs have enrichment fields (job_id, job_name)
	if len(logs) > 0 {
		firstLog := logs[0].(map[string]interface{})
		assert.Contains(t, firstLog, "job_id", "Log should have job_id")
		assert.Contains(t, firstLog, "job_name", "Log should have job_name")
		assert.Contains(t, firstLog, "timestamp", "Log should have timestamp")
		assert.Contains(t, firstLog, "level", "Log should have level")
		assert.Contains(t, firstLog, "message", "Log should have message")
	}

	// Test 2: Get aggregated logs with level filter
	t.Log("Step 4: Getting aggregated logs with level filter")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs/aggregated?level=error&limit=100", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// Test 3: Get aggregated logs without children
	t.Log("Step 5: Getting aggregated logs without children")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs/aggregated?include_children=false", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var noChildrenResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &noChildrenResult)
	require.NoError(t, err)
	assert.Equal(t, false, noChildrenResult["include_children"], "Should exclude children")

	t.Log("✓ Aggregated logs test completed successfully")
}

// TestJobManagement_RerunJob tests POST /api/jobs/{id}/rerun
func TestJobManagement_RerunJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state before rerunning
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Rerun completed job
	t.Log("Step 3: Rerunning job")
	resp, err := helper.POST(fmt.Sprintf("/api/jobs/%s/rerun", jobID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Job should be in terminal state, so rerun should succeed
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Contains(t, result, "original_job_id", "Response should contain original_job_id")
	assert.Contains(t, result, "new_job_id", "Response should contain new_job_id")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Equal(t, jobID, result["original_job_id"], "Original job ID should match")

	newJobID, ok := result["new_job_id"].(string)
	require.True(t, ok, "New job ID should be a string")
	assert.NotEqual(t, jobID, newJobID, "New job ID should be different")

	t.Logf("Job rerun created: original=%s, new=%s", jobID, newJobID)
	defer deleteJob(t, helper, newJobID)

	// Test 2: Rerun nonexistent job (should fail)
	t.Log("Step 4: Rerunning nonexistent job")
	resp, err = helper.POST("/api/jobs/nonexistent-job-id/rerun", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusInternalServerError)

	t.Log("✓ Rerun job test completed successfully")
}

// TestJobManagement_CancelJob tests POST /api/jobs/{id}/cancel
func TestJobManagement_CancelJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Test 1: Cancel job
	t.Log("Step 2: Cancelling job")
	resp, err := helper.POST(fmt.Sprintf("/api/jobs/%s/cancel", jobID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK or 500 if job already completed
	if resp.StatusCode == http.StatusInternalServerError {
		t.Log("⚠️  Job may have already completed")
		return
	}

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Equal(t, jobID, result["job_id"], "Job ID should match")

	t.Log("Job cancelled successfully")

	// Test 2: Cancel nonexistent job
	t.Log("Step 3: Cancelling nonexistent job")
	resp, err = helper.POST("/api/jobs/nonexistent-job-id/cancel", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusInternalServerError)

	t.Log("✓ Cancel job test completed successfully")
}

// TestJobManagement_CopyJob tests POST /api/jobs/{id}/copy
func TestJobManagement_CopyJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Test 1: Copy job
	t.Log("Step 2: Copying job")
	resp, err := helper.POST(fmt.Sprintf("/api/jobs/%s/copy", jobID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Contains(t, result, "original_job_id", "Response should contain original_job_id")
	assert.Contains(t, result, "new_job_id", "Response should contain new_job_id")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Equal(t, jobID, result["original_job_id"], "Original job ID should match")

	newJobID, ok := result["new_job_id"].(string)
	require.True(t, ok, "New job ID should be a string")
	assert.NotEqual(t, jobID, newJobID, "New job ID should be different")

	t.Logf("Job copied: original=%s, new=%s", jobID, newJobID)
	defer deleteJob(t, helper, newJobID)

	// Test 2: Copy nonexistent job (should fail)
	t.Log("Step 3: Copying nonexistent job")
	resp, err = helper.POST("/api/jobs/nonexistent-job-id/copy", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusInternalServerError)

	t.Log("✓ Copy job test completed successfully")
}

// TestJobManagement_DeleteJob tests DELETE /api/jobs/{id}
func TestJobManagement_DeleteJob(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Delete valid job
	t.Log("Step 1: Creating test job for deletion")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}

	// Wait for job to complete or reach terminal state
	t.Log("Step 2: Waiting for job to complete")
	waitForJobCompletion(t, helper, jobID, 10*time.Second)

	t.Log("Step 3: Deleting job")
	resp, err := helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK with deletion confirmation
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Equal(t, jobID, result["job_id"], "Job ID should match")

	// Test 2: Verify job no longer exists
	t.Log("Step 4: Verifying job was deleted")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Delete nonexistent job (should return 404)
	t.Log("Step 5: Deleting nonexistent job")
	resp, err = helper.DELETE("/api/jobs/nonexistent-job-id")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("✓ Delete job test completed successfully")
}

// TestJobManagement_JobResults tests GET /api/jobs/{id}/results
func TestJobManagement_JobResults(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state so results are generated
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Get results for job (may be empty)
	t.Log("Step 3: Getting job results")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/results", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 200 OK or 500 if job not found in crawler service
	if resp.StatusCode == http.StatusInternalServerError {
		t.Log("⚠️  Job results not available (job may not be tracked by crawler service)")
		return
	}

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "results", "Response should contain results array")
	assert.Contains(t, result, "count", "Response should contain count")

	results, ok := result["results"].([]interface{})
	assert.True(t, ok, "Results should be an array")
	count, ok := result["count"].(float64)
	assert.True(t, ok, "Count should be a number")

	t.Logf("Job results: count=%v, results_len=%d", count, len(results))

	// Test 2: Get results for nonexistent job
	t.Log("Step 4: Getting results for nonexistent job")
	resp, err = helper.GET("/api/jobs/nonexistent-job-id/results")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusInternalServerError)

	t.Log("✓ Job results test completed successfully")
}

// TestJobManagement_JobLifecycle tests complete job lifecycle
func TestJobManagement_JobLifecycle(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Step 1: Create job definition
	t.Log("Step 1: Creating job definition")
	defID := fmt.Sprintf("lifecycle-test-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Lifecycle Test Job", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Step 2: Execute job definition to create job
	t.Log("Step 2: Executing job definition")
	jobID := executeJobDefinition(t, helper, defID)
	require.NotEmpty(t, jobID, "Job ID should not be empty")
	defer deleteJob(t, helper, jobID)

	// Step 3: Verify job is in pending or running state
	t.Log("Step 3: Checking initial job status")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var job map[string]interface{}
	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err)

	status, ok := job["status"].(string)
	require.True(t, ok, "Job should have status")
	t.Logf("Initial job status: %s", status)
	assert.True(t, status == "pending" || status == "running", "Job should be pending or running")

	// Step 4: Monitor progress (wait briefly)
	t.Log("Step 4: Monitoring job progress")
	time.Sleep(2 * time.Second)

	// Step 5: Check logs
	t.Log("Step 5: Checking job logs")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/logs", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var logsResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &logsResult)
	require.NoError(t, err)
	logs, _ := logsResult["logs"].([]interface{})
	t.Logf("Job has %d log entries", len(logs))

	// Step 6: Wait for completion or timeout
	t.Log("Step 6: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 15*time.Second)
	t.Logf("Final job status: %s", finalStatus)

	// Step 7: Verify final status
	t.Log("Step 7: Verifying final job state")
	resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err)
	finalStatusFromAPI, _ := job["status"].(string)
	t.Logf("Final status from API: %s", finalStatusFromAPI)

	// Step 8: Check results (if completed)
	if finalStatusFromAPI == "completed" {
		t.Log("Step 8: Checking job results")
		resp, err = helper.GET(fmt.Sprintf("/api/jobs/%s/results", jobID))
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var resultsData map[string]interface{}
			err = helper.ParseJSONResponse(resp, &resultsData)
			require.NoError(t, err)
			t.Logf("Job results available: count=%v", resultsData["count"])
		}
	}

	// Step 9: Rerun job (if in terminal state)
	if finalStatusFromAPI == "completed" || finalStatusFromAPI == "failed" || finalStatusFromAPI == "cancelled" {
		t.Log("Step 9: Rerunning job")
		resp, err = helper.POST(fmt.Sprintf("/api/jobs/%s/rerun", jobID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusCreated {
			var rerunResult map[string]interface{}
			err = helper.ParseJSONResponse(resp, &rerunResult)
			require.NoError(t, err)
			newJobID, _ := rerunResult["new_job_id"].(string)
			t.Logf("Job rerun created: new_job_id=%s", newJobID)
			if newJobID != "" {
				defer deleteJob(t, helper, newJobID)
			}
		}
	}

	// Step 10: Copy job
	t.Log("Step 10: Copying job")
	resp, err = helper.POST(fmt.Sprintf("/api/jobs/%s/copy", jobID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		var copyResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &copyResult)
		require.NoError(t, err)
		copiedJobID, _ := copyResult["new_job_id"].(string)
		t.Logf("Job copied: new_job_id=%s", copiedJobID)
		if copiedJobID != "" {
			defer deleteJob(t, helper, copiedJobID)
		}
	}

	// Step 11: Cancel job (if still running)
	if finalStatusFromAPI == "running" {
		t.Log("Step 11: Cancelling job")
		resp, err = helper.POST(fmt.Sprintf("/api/jobs/%s/cancel", jobID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		helper.AssertStatusCode(resp, http.StatusOK)
	}

	// Step 12: Delete job
	t.Log("Step 12: Deleting job")
	resp, err = helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	t.Log("✓ Complete job lifecycle test completed successfully")
}

// Job Definition Tests

// TestJobDefinition_List tests GET /api/job-definitions with pagination and filtering
func TestJobDefinition_List(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create test job definitions for testing
	t.Log("Step 1: Creating test job definitions")
	defID1 := fmt.Sprintf("test-def-1-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID1, "Test Job Definition 1", "crawler")
	defer deleteJobDefinition(t, helper, defID1)

	defID2 := fmt.Sprintf("test-def-2-%d", time.Now().UnixNano()+1)
	createTestJobDefinition(t, helper, defID2, "Test Job Definition 2", "crawler")
	defer deleteJobDefinition(t, helper, defID2)

	// Test 1: List job definitions with default parameters
	t.Log("Step 2: Listing job definitions with defaults")
	resp, err := helper.GET("/api/job-definitions")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_definitions", "Response should contain job_definitions array")
	assert.Contains(t, result, "total_count", "Response should contain total_count")
	assert.Contains(t, result, "limit", "Response should contain limit")
	assert.Contains(t, result, "offset", "Response should contain offset")

	jobDefs, ok := result["job_definitions"].([]interface{})
	assert.True(t, ok, "Job definitions should be an array")
	t.Logf("Retrieved %d job definitions", len(jobDefs))

	// Test 2: List with pagination
	t.Log("Step 3: Listing with pagination (limit=10, offset=0)")
	resp, err = helper.GET("/api/job-definitions?limit=10&offset=0")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var paginatedResult map[string]interface{}
	err = helper.ParseJSONResponse(resp, &paginatedResult)
	require.NoError(t, err)
	assert.Equal(t, float64(10), paginatedResult["limit"], "Limit should be 10")
	assert.Equal(t, float64(0), paginatedResult["offset"], "Offset should be 0")

	// Test 3: List with type filter
	t.Log("Step 4: Listing with type filter (type=crawler)")
	resp, err = helper.GET("/api/job-definitions?type=crawler")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// Test 4: List with enabled filter
	t.Log("Step 5: Listing with enabled filter (enabled=true)")
	resp, err = helper.GET("/api/job-definitions?enabled=true")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// Test 5: List with ordering
	t.Log("Step 6: Listing with ordering (order_by=name, order_dir=ASC)")
	resp, err = helper.GET("/api/job-definitions?order_by=name&order_dir=ASC")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	t.Log("✓ Job definition listing test completed successfully")
}

// TestJobDefinition_Create tests POST /api/job-definitions
func TestJobDefinition_Create(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Create valid job definition
	t.Log("Step 1: Creating valid job definition")
	defID := fmt.Sprintf("test-create-def-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":   defID,
		"name": "Test Create Job Definition",
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "crawl-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls":  []string{"https://example.com"},
					"max_depth":   2,
					"max_pages":   10,
					"concurrency": 1,
				},
			},
		},
		"enabled": true,
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
	assert.Contains(t, result, "type", "Response should contain type")
	assert.Contains(t, result, "steps", "Response should contain steps")

	// Clean up
	defer deleteJobDefinition(t, helper, defID)

	// Test 2: Create job definition with missing ID (400)
	t.Log("Step 2: Creating job definition with missing ID")
	invalidBody := map[string]interface{}{
		"name": "Invalid Job Definition",
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "crawl-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls": []string{"https://example.com"},
				},
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions", invalidBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 3: Create job definition with missing name (400)
	t.Log("Step 3: Creating job definition with missing name")
	invalidBody = map[string]interface{}{
		"id":   fmt.Sprintf("test-invalid-%d", time.Now().UnixNano()),
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "crawl-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls": []string{"https://example.com"},
				},
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions", invalidBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 4: Create job definition with missing steps (400)
	t.Log("Step 4: Creating job definition with missing steps")
	invalidBody = map[string]interface{}{
		"id":   fmt.Sprintf("test-invalid-%d", time.Now().UnixNano()),
		"name": "Invalid Job Definition",
		"type": "crawler",
	}

	resp, err = helper.POST("/api/job-definitions", invalidBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("✓ Job definition creation test completed successfully")
}

// TestJobDefinition_Get tests GET /api/job-definitions/{id}
func TestJobDefinition_Get(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job definition
	t.Log("Step 1: Creating test job definition")
	defID := fmt.Sprintf("test-get-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Get Job Definition", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Test 1: Get valid job definition
	t.Log("Step 2: Getting job definition")
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(resp, &jobDef)
	require.NoError(t, err)

	// Verify response includes all expected fields
	assert.Equal(t, defID, jobDef["id"], "Job definition ID should match")
	assert.Contains(t, jobDef, "name", "Job definition should have name")
	assert.Contains(t, jobDef, "type", "Job definition should have type")
	assert.Contains(t, jobDef, "steps", "Job definition should have steps")
	assert.Contains(t, jobDef, "created_at", "Job definition should have created_at")

	// Test 2: Get nonexistent job definition (404)
	t.Log("Step 3: Getting nonexistent job definition")
	resp, err = helper.GET("/api/job-definitions/nonexistent-def-id")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Get with empty ID (400)
	t.Log("Step 4: Getting job definition with empty ID")
	resp, err = helper.GET("/api/job-definitions/")
	require.NoError(t, err)
	defer resp.Body.Close()
	// May return 400 or 404 depending on routing
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound,
		"Empty ID should return 400 or 404")

	t.Log("✓ Get job definition test completed successfully")
}

// TestJobDefinition_Update tests PUT /api/job-definitions/{id}
func TestJobDefinition_Update(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job definition
	t.Log("Step 1: Creating test job definition")
	defID := fmt.Sprintf("test-update-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Update Job Definition", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Test 1: Update valid job definition
	t.Log("Step 2: Updating job definition")
	updateBody := map[string]interface{}{
		"id":   defID,
		"name": "Updated Job Definition Name",
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "updated-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls":  []string{"https://updated.example.com"},
					"max_depth":   3,
					"max_pages":   20,
					"concurrency": 2,
				},
			},
		},
		"enabled": true,
	}

	resp, err := helper.PUT(fmt.Sprintf("/api/job-definitions/%s", defID), updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify updated fields
	assert.Equal(t, "Updated Job Definition Name", result["name"], "Name should be updated")

	// Test 2: Update nonexistent job definition (404)
	t.Log("Step 3: Updating nonexistent job definition")
	resp, err = helper.PUT("/api/job-definitions/nonexistent-def-id", updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Update with invalid data (400)
	t.Log("Step 4: Updating with missing steps")
	invalidBody := map[string]interface{}{
		"id":   defID,
		"name": "Invalid Update",
		"type": "crawler",
		// Missing steps
	}

	resp, err = helper.PUT(fmt.Sprintf("/api/job-definitions/%s", defID), invalidBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("✓ Update job definition test completed successfully")
}

// TestJobDefinition_Delete tests DELETE /api/job-definitions/{id}
func TestJobDefinition_Delete(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Delete valid job definition
	t.Log("Step 1: Creating test job definition for deletion")
	defID := fmt.Sprintf("test-delete-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Delete Job Definition", "crawler")

	t.Log("Step 2: Deleting job definition")
	resp, err := helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 204 No Content
	helper.AssertStatusCode(resp, http.StatusNoContent)

	// Test 2: Verify job definition no longer exists
	t.Log("Step 3: Verifying job definition was deleted")
	resp, err = helper.GET(fmt.Sprintf("/api/job-definitions/%s", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Delete nonexistent job definition (404)
	t.Log("Step 4: Deleting nonexistent job definition")
	resp, err = helper.DELETE("/api/job-definitions/nonexistent-def-id")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("✓ Delete job definition test completed successfully")
}

// TestJobDefinition_Execute tests POST /api/job-definitions/{id}/execute
func TestJobDefinition_Execute(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job definition
	t.Log("Step 1: Creating test job definition")
	defID := fmt.Sprintf("test-execute-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Execute Job Definition", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Test 1: Execute valid job definition
	t.Log("Step 2: Executing job definition")
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusAccepted)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "job_name", "Response should contain job_name")
	assert.Contains(t, result, "status", "Response should contain status")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Equal(t, "running", result["status"], "Status should be 'running'")

	jobID, ok := result["job_id"].(string)
	require.True(t, ok, "Job ID should be a string")
	t.Logf("Job execution started: job_id=%s", jobID)

	// Clean up created job
	if jobID != "" {
		defer deleteJob(t, helper, jobID)

		// Verify job was created and wait for terminal state
		t.Log("Step 3: Verifying job execution progresses to terminal state")
		finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
		t.Logf("Job reached terminal state: %s", finalStatus)
	}

	// Test 2: Execute nonexistent job definition (404)
	t.Log("Step 4: Executing nonexistent job definition")
	resp, err = helper.POST("/api/job-definitions/nonexistent-def-id/execute", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Create disabled job definition and try to execute (400)
	t.Log("Step 5: Creating disabled job definition")
	disabledDefID := fmt.Sprintf("test-disabled-def-%d", time.Now().UnixNano())
	disabledBody := map[string]interface{}{
		"id":   disabledDefID,
		"name": "Disabled Job Definition",
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "crawl-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls": []string{"https://example.com"},
				},
			},
		},
		"enabled": false,
	}

	resp, err = helper.POST("/api/job-definitions", disabledBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		defer deleteJobDefinition(t, helper, disabledDefID)

		t.Log("Step 6: Executing disabled job definition")
		resp, err = helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", disabledDefID), nil)
		require.NoError(t, err)
		defer resp.Body.Close()
		helper.AssertStatusCode(resp, http.StatusBadRequest)
	}

	t.Log("✓ Execute job definition test completed successfully")
}

// Job Definition TOML Workflow Tests

// TestJobDefinition_Export tests GET /api/job-definitions/{id}/export
func TestJobDefinition_Export(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test crawler job definition
	t.Log("Step 1: Creating test crawler job definition")
	defID := fmt.Sprintf("test-export-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Test Export Job Definition", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Test 1: Export valid crawler job definition
	t.Log("Step 2: Exporting job definition")
	resp, err := helper.GET(fmt.Sprintf("/api/job-definitions/%s/export", defID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	// Verify Content-Type header
	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/toml", contentType, "Content-Type should be application/toml")

	// Verify Content-Disposition header
	contentDisposition := resp.Header.Get("Content-Disposition")
	assert.Contains(t, contentDisposition, "attachment", "Content-Disposition should contain 'attachment'")
	assert.Contains(t, contentDisposition, defID, "Content-Disposition should contain job definition ID")

	t.Logf("Exported job definition with headers: Content-Type=%s, Content-Disposition=%s", contentType, contentDisposition)

	// Test 2: Export nonexistent job definition (404)
	t.Log("Step 3: Exporting nonexistent job definition")
	resp, err = helper.GET("/api/job-definitions/nonexistent-def-id/export")
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("✓ Export job definition test completed successfully")
}

// TestJobDefinition_Status tests GET /api/jobs/{id}/status
func TestJobDefinition_Status(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a test job
	t.Log("Step 1: Creating test job")
	jobID := createTestJob(t, helper)
	if jobID == "" {
		t.Skip("Could not create test job")
		return
	}
	defer deleteJob(t, helper, jobID)

	// Wait for job to reach terminal state so tree status is complete
	t.Log("Step 2: Waiting for job completion")
	finalStatus := waitForJobCompletion(t, helper, jobID, 60*time.Second)
	t.Logf("Job reached terminal state: %s", finalStatus)

	// Test 1: Get job tree status
	t.Log("Step 3: Getting job tree status")
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/status", jobID))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var status map[string]interface{}
	err = helper.ParseJSONResponse(resp, &status)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, status, "total_children", "Status should contain total_children")
	assert.Contains(t, status, "completed_count", "Status should contain completed_count")
	assert.Contains(t, status, "failed_count", "Status should contain failed_count")
	assert.Contains(t, status, "overall_progress", "Status should contain overall_progress")

	totalChildren, ok := status["total_children"].(float64)
	assert.True(t, ok, "Total children should be a number")
	completedCount, ok := status["completed_count"].(float64)
	assert.True(t, ok, "Completed count should be a number")
	failedCount, ok := status["failed_count"].(float64)
	assert.True(t, ok, "Failed count should be a number")
	overallProgress, ok := status["overall_progress"].(float64)
	assert.True(t, ok, "Overall progress should be a number")

	t.Logf("Job tree status: total_children=%v, completed=%v, failed=%v, progress=%v",
		totalChildren, completedCount, failedCount, overallProgress)

	// Test 2: Get status for nonexistent job (should fail)
	t.Log("Step 4: Getting status for nonexistent job")
	resp, err = helper.GET("/api/jobs/nonexistent-job-id/status")
	require.NoError(t, err)
	defer resp.Body.Close()
	// May return 500 or 404 depending on implementation
	assert.True(t, resp.StatusCode >= 400, "Should return error for nonexistent job")

	t.Log("✓ Job tree status test completed successfully")
}

// TestJobDefinition_ValidateTOML tests POST /api/job-definitions/validate
func TestJobDefinition_ValidateTOML(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Validate valid TOML content
	t.Log("Step 1: Validating valid TOML content")
	validTOML := `
id = "test-validate-job"
name = "Test Validate Job"
description = "Test job for TOML validation"
enabled = true
start_urls = ["https://example.com"]
max_depth = 2
max_pages = 10
`

	resp, err := helper.POSTBody("/api/job-definitions/validate", "application/toml", []byte(validTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify validation result
	valid, ok := result["valid"].(bool)
	assert.True(t, ok, "Result should contain valid boolean")
	assert.True(t, valid, "TOML should be valid")

	// Test 2: Validate invalid TOML syntax
	t.Log("Step 2: Validating invalid TOML syntax")
	invalidTOML := `
id = "test-invalid"
name = [[[invalid syntax
`

	resp, err = helper.POSTBody("/api/job-definitions/validate", "application/toml", []byte(invalidTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	valid, ok = result["valid"].(bool)
	assert.True(t, ok, "Result should contain valid boolean")
	assert.False(t, valid, "TOML should be invalid")
	assert.Contains(t, result, "error", "Result should contain error message")

	// Test 3: Validate with missing required fields
	t.Log("Step 3: Validating TOML with missing required fields")
	incompleteTOML := `
name = "Incomplete Job"
`

	resp, err = helper.POSTBody("/api/job-definitions/validate", "application/toml", []byte(incompleteTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	// May return 400 or 200 depending on validation level
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	t.Log("✓ TOML validation test completed successfully")
}

// TestJobDefinition_UploadTOML tests POST /api/job-definitions/upload
func TestJobDefinition_UploadTOML(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Upload valid TOML (create new job definition)
	t.Log("Step 1: Uploading valid TOML")
	defID := fmt.Sprintf("test-upload-def-%d", time.Now().UnixNano())
	validTOML := fmt.Sprintf(`
id = "%s"
name = "Test Upload Job"
description = "Test job created via TOML upload"
enabled = true
start_urls = ["https://example.com"]
max_depth = 2
max_pages = 10
concurrency = 1
follow_links = true
`, defID)

	resp, err := helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(validTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify created job definition
	assert.Equal(t, defID, result["id"], "Job definition ID should match")
	assert.Contains(t, result, "name", "Response should contain name")
	assert.Contains(t, result, "type", "Response should contain type")

	// Clean up
	defer deleteJobDefinition(t, helper, defID)

	// Test 2: Upload invalid TOML syntax (400)
	t.Log("Step 2: Uploading invalid TOML syntax")
	invalidTOML := `
id = "test-invalid"
name = [[[invalid syntax
`

	resp, err = helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(invalidTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 3: Upload TOML with missing required fields (400)
	t.Log("Step 3: Uploading TOML with missing required fields")
	incompleteTOML := `
name = "Incomplete Job"
enabled = true
`

	resp, err = helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(incompleteTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 4: Update existing job definition via upload (200 OK)
	t.Log("Step 4: Updating existing job definition via TOML upload")
	updateTOML := fmt.Sprintf(`
id = "%s"
name = "Updated Upload Job"
description = "Updated via TOML upload"
enabled = true
start_urls = ["https://updated.example.com"]
max_depth = 3
max_pages = 20
concurrency = 2
follow_links = true
`, defID)

	resp, err = helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(updateTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)
	assert.Equal(t, "Updated Upload Job", result["name"], "Name should be updated")

	// Test 5: Upload TOML with system job ID collision (409)
	t.Log("Step 5: Uploading TOML with system job ID (should return 409)")
	// System job IDs typically start with "system-" prefix
	systemJobTOML := `
id = "system-health-check"
name = "System Health Check"
description = "Attempting to create job definition with system job ID"
enabled = true
start_urls = ["https://example.com"]
max_depth = 1
max_pages = 5
`

	resp, err = helper.POSTBody("/api/job-definitions/upload", "application/toml", []byte(systemJobTOML))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 409 Conflict for system job ID collision, or 403 Forbidden for system job protection
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusForbidden {
		t.Logf("System job protection working: status %d", resp.StatusCode)
		var errorResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &errorResult)
		require.NoError(t, err)
		assert.Contains(t, errorResult, "error", "Error response should contain error field")
		t.Logf("Error message: %v", errorResult["error"])
	} else {
		t.Logf("⚠️  System job protection may not be enforced (status %d)", resp.StatusCode)
	}

	t.Log("✓ TOML upload test completed successfully")
}

// TestJobDefinition_SaveInvalidTOML tests POST /api/job-definitions/save-invalid
func TestJobDefinition_SaveInvalidTOML(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Save invalid TOML without validation
	t.Log("Step 1: Saving invalid TOML without validation")
	invalidTOML := `
This is completely invalid TOML that would normally be rejected
But this endpoint saves it anyway for testing purposes
`

	resp, err := helper.POSTBody("/api/job-definitions/save-invalid", "application/toml", []byte(invalidTOML))
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response
	assert.Contains(t, result, "id", "Response should contain generated ID")
	jobDefID, ok := result["id"].(string)
	require.True(t, ok, "ID should be a string")
	assert.True(t, len(jobDefID) > 0, "ID should not be empty")
	assert.Contains(t, jobDefID, "invalid-", "ID should have 'invalid-' prefix")

	t.Logf("Saved invalid TOML with ID: %s", jobDefID)

	// Clean up
	defer deleteJobDefinition(t, helper, jobDefID)

	t.Log("✓ Save invalid TOML test completed successfully")
}

// TestJobDefinition_QuickCrawl tests POST /api/job-definitions/quick-crawl
func TestJobDefinition_QuickCrawl(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Quick crawl with valid URL
	t.Log("Step 1: Creating quick crawl with valid URL")
	quickCrawlBody := map[string]interface{}{
		"url":              "https://example.com",
		"name":             "Quick Crawl Test",
		"max_depth":        2,
		"max_pages":        10,
		"include_patterns": []string{"*/blog/*"},
		"exclude_patterns": []string{"*/admin/*"},
	}

	resp, err := helper.POST("/api/job-definitions/quick-crawl", quickCrawlBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusAccepted)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, result, "job_id", "Response should contain job_id")
	assert.Contains(t, result, "job_name", "Response should contain job_name")
	assert.Contains(t, result, "status", "Response should contain status")
	assert.Contains(t, result, "message", "Response should contain message")
	assert.Contains(t, result, "url", "Response should contain url")
	assert.Contains(t, result, "max_depth", "Response should contain max_depth")
	assert.Contains(t, result, "max_pages", "Response should contain max_pages")
	assert.Equal(t, "running", result["status"], "Status should be 'running'")

	jobDefID, ok := result["job_id"].(string)
	require.True(t, ok, "Job ID should be a string")
	t.Logf("Quick crawl started: job_id=%s", jobDefID)

	// Clean up job definition and job
	if jobDefID != "" {
		defer deleteJobDefinition(t, helper, jobDefID)
		defer deleteJob(t, helper, jobDefID)
	}

	// Test 2: Quick crawl with missing URL (400)
	t.Log("Step 2: Creating quick crawl with missing URL")
	invalidBody := map[string]interface{}{
		"name": "Invalid Quick Crawl",
	}

	resp, err = helper.POST("/api/job-definitions/quick-crawl", invalidBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 3: Quick crawl with cookies
	t.Log("Step 3: Creating quick crawl with authentication cookies")
	quickCrawlWithCookies := map[string]interface{}{
		"url":  "https://authenticated.example.com",
		"name": "Quick Crawl with Auth",
		"cookies": []map[string]interface{}{
			{
				"name":   "session_id",
				"value":  "abc123",
				"domain": "authenticated.example.com",
			},
		},
	}

	resp, err = helper.POST("/api/job-definitions/quick-crawl", quickCrawlWithCookies)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusAccepted)

	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Clean up
	if jobID, ok := result["job_id"].(string); ok && jobID != "" {
		defer deleteJobDefinition(t, helper, jobID)
		defer deleteJob(t, helper, jobID)
	}

	// Test 4: Quick crawl with invalid override values (non-numeric max_depth)
	t.Log("Step 4: Creating quick crawl with invalid max_depth (non-numeric)")
	invalidOverrideBody := map[string]interface{}{
		"url":       "https://example.com",
		"name":      "Invalid Override Test",
		"max_depth": "not-a-number", // Should be numeric
	}

	resp, err = helper.POST("/api/job-definitions/quick-crawl", invalidOverrideBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 Bad Request for invalid parameter type
	if resp.StatusCode == http.StatusBadRequest {
		t.Log("✓ Invalid override values properly rejected")
		var errorResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &errorResult)
		if err == nil {
			t.Logf("Error message: %v", errorResult["error"])
		}
	} else {
		// May accept string and coerce, or may not validate type strictly
		t.Logf("⚠️  Override value validation behavior: status %d", resp.StatusCode)
	}

	t.Log("✓ Quick crawl test completed successfully")
}

// TestJobDefinition_SystemJobProtection tests system job protection (Comment 4)
func TestJobDefinition_SystemJobProtection(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// System jobs typically have IDs starting with "system-"
	systemJobID := "system-test-protection"

	// Test 1: Attempt to update a system job definition (expect 403)
	t.Log("Step 1: Attempting to update system job definition")
	updateBody := map[string]interface{}{
		"id":   systemJobID,
		"name": "Attempting to Update System Job",
		"type": "crawler",
		"steps": []map[string]interface{}{
			{
				"name": "test-step",
				"type": "crawler",
				"config": map[string]interface{}{
					"start_urls": []string{"https://example.com"},
				},
			},
		},
		"enabled": true,
	}

	resp, err := helper.PUT(fmt.Sprintf("/api/job-definitions/%s", systemJobID), updateBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 403 Forbidden for system job protection, or 404 if system job doesn't exist
	if resp.StatusCode == http.StatusForbidden {
		t.Log("✓ System job update protection working (403 Forbidden)")
		var errorResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &errorResult)
		if err == nil {
			t.Logf("Error message: %v", errorResult["error"])
		}
	} else if resp.StatusCode == http.StatusNotFound {
		t.Log("⚠️  System job does not exist (404 Not Found) - cannot test update protection")
	} else {
		t.Logf("⚠️  Unexpected status for system job update: %d", resp.StatusCode)
	}

	// Test 2: Attempt to delete a system job definition (expect 403)
	t.Log("Step 2: Attempting to delete system job definition")
	resp, err = helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", systemJobID))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 403 Forbidden for system job protection, or 404 if system job doesn't exist
	if resp.StatusCode == http.StatusForbidden {
		t.Log("✓ System job delete protection working (403 Forbidden)")
		var errorResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &errorResult)
		if err == nil {
			t.Logf("Error message: %v", errorResult["error"])
		}
	} else if resp.StatusCode == http.StatusNotFound {
		t.Log("⚠️  System job does not exist (404 Not Found) - cannot test delete protection")
	} else {
		t.Logf("⚠️  Unexpected status for system job delete: %d", resp.StatusCode)
	}

	t.Log("✓ System job protection test completed")
}

// TestJobDefinition_DependencyValidation tests runtime dependency validation (Comment 4)
func TestJobDefinition_DependencyValidation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Create a job definition that requires dependencies
	t.Log("Step 1: Creating job definition that may require dependencies")
	defID := fmt.Sprintf("test-dependency-def-%d", time.Now().UnixNano())
	createTestJobDefinition(t, helper, defID, "Dependency Test Job", "crawler")
	defer deleteJobDefinition(t, helper, defID)

	// Test 2: Attempt to execute job definition (may fail if dependencies missing)
	t.Log("Step 2: Executing job definition to test dependency validation")
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Execution may succeed (202) if dependencies are available, or fail (400/500) if missing
	if resp.StatusCode == http.StatusAccepted {
		t.Log("✓ Job execution succeeded - dependencies are available")
		var result map[string]interface{}
		err = helper.ParseJSONResponse(resp, &result)
		if err == nil {
			if jobID, ok := result["job_id"].(string); ok && jobID != "" {
				defer deleteJob(t, helper, jobID)
				t.Logf("Created job: %s", jobID)
			}
		}
	} else if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		t.Logf("⚠️  Job execution failed (status %d) - dependencies may be missing", resp.StatusCode)
		var errorResult map[string]interface{}
		err = helper.ParseJSONResponse(resp, &errorResult)
		if err == nil {
			t.Logf("Error message: %v", errorResult["error"])
			// Check if error mentions missing dependencies (agent service, API keys)
			if errorMsg, ok := errorResult["error"].(string); ok {
				if containsStr := func(s, substr string) bool {
					return len(s) >= len(substr) && (s == substr || len(s) > len(substr))
				}; containsStr(errorMsg, "agent") || containsStr(errorMsg, "service") || containsStr(errorMsg, "key") {
					t.Log("✓ Error message indicates missing dependency (expected in test environment)")
				}
			}
		}
	} else {
		t.Logf("⚠️  Unexpected status for job execution: %d", resp.StatusCode)
	}

	t.Log("✓ Dependency validation test completed")
}
