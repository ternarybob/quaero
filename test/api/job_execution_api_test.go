// -----------------------------------------------------------------------
// API test for basic job execution workflow
// Tests POST /api/job-definitions/{id}/execute and GET /api/jobs endpoints
// NOTE: This test is EXPECTED TO FAIL as job execution is not yet implemented
// -----------------------------------------------------------------------

package api

import (
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
	"time"
)

// TestJobBasicExecutionAPI verifies job execution via API
// NOTE: This test is EXPECTED TO FAIL - job execution is not yet implemented
func TestJobBasicExecutionAPI(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobBasicExecutionAPI")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Step 1: Get list of job definitions to find "Database Maintenance"
	env.LogTest(t, "Step 1: Fetching job definitions")
	resp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to get job definitions: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	var jobDefs []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &jobDefs); err != nil {
		t.Fatalf("Failed to parse job definitions: %v", err)
	}

	// Step 2: Find "Database Maintenance" job definition
	env.LogTest(t, "Step 2: Finding 'Database Maintenance' job definition")
	var dbMaintenanceID string
	for _, jobDef := range jobDefs {
		if name, ok := jobDef["name"].(string); ok && name == "Database Maintenance" {
			dbMaintenanceID = jobDef["id"].(string)
			env.LogTest(t, "✓ Found 'Database Maintenance' with ID: %s", dbMaintenanceID)
			break
		}
	}

	if dbMaintenanceID == "" {
		t.Fatal("REQUIREMENT FAILED: 'Database Maintenance' job definition not found")
	}

	// Step 3: Execute the job definition
	env.LogTest(t, "Step 3: Executing 'Database Maintenance' job")
	executeResp, err := h.POST("/api/job-definitions/"+dbMaintenanceID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}

	// REQUIREMENT: Execute should return success status (201 or 202)
	if executeResp.StatusCode != http.StatusCreated && executeResp.StatusCode != http.StatusAccepted {
		t.Errorf("REQUIREMENT FAILED: Expected 201/202 status for job execution, got: %d", executeResp.StatusCode)
	} else {
		env.LogTest(t, "✓ Job execution request accepted (status: %d)", executeResp.StatusCode)
	}

	var executeResult map[string]interface{}
	if err := h.ParseJSONResponse(executeResp, &executeResult); err != nil {
		t.Logf("Warning: Failed to parse execute response: %v", err)
	} else {
		env.LogTest(t, "Execute response: %v", executeResult)
	}

	// Step 4: Wait a moment for job to be created
	env.LogTest(t, "Step 4: Waiting for job to be created...")
	time.Sleep(2 * time.Second)

	// Step 5: Get list of jobs to verify job was created
	env.LogTest(t, "Step 5: Fetching jobs list to verify job creation")
	jobsResp, err := h.GET("/api/jobs")
	if err != nil {
		t.Fatalf("Failed to get jobs list: %v", err)
	}

	h.AssertStatusCode(jobsResp, http.StatusOK)

	var jobsResult struct {
		Jobs  []map[string]interface{} `json:"jobs"`
		Total int                      `json:"total"`
	}
	if err := h.ParseJSONResponse(jobsResp, &jobsResult); err != nil {
		// Try parsing as plain array if paginated response fails
		var jobsArray []map[string]interface{}
		if err2 := h.ParseJSONResponse(jobsResp, &jobsArray); err2 != nil {
			t.Fatalf("Failed to parse jobs response: %v (also tried array: %v)", err, err2)
		}
		jobsResult.Jobs = jobsArray
	}

	env.LogTest(t, "Found %d jobs in queue", len(jobsResult.Jobs))

	// Step 6: Search for "Database Maintenance" job in the queue
	env.LogTest(t, "Step 6: Searching for 'Database Maintenance' job in queue")
	var foundInQueue bool
	var jobID string

	for _, job := range jobsResult.Jobs {
		// Check if this job belongs to the Database Maintenance job definition
		if jobDefID, ok := job["job_definition_id"].(string); ok && jobDefID == dbMaintenanceID {
			foundInQueue = true
			jobID = job["id"].(string)
			status := job["status"].(string)
			env.LogTest(t, "✓ Found 'Database Maintenance' job in queue (ID: %s, Status: %s)", jobID, status)
			break
		}

		// Also check by name if job_definition_name field exists
		if name, ok := job["job_definition_name"].(string); ok && name == "Database Maintenance" {
			foundInQueue = true
			jobID = job["id"].(string)
			status := job["status"].(string)
			env.LogTest(t, "✓ Found 'Database Maintenance' job in queue (ID: %s, Status: %s)", jobID, status)
			break
		}
	}

	// REQUIREMENT: Job should be in the queue
	if !foundInQueue {
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job not found in job queue after execution")

		// Log all jobs for debugging
		env.LogTest(t, "Jobs currently in queue:")
		for i, job := range jobsResult.Jobs {
			env.LogTest(t, "  Job %d: %v", i+1, job)
		}

		env.LogTest(t, "⚠️  TEST FAILED AS EXPECTED: Job execution functionality not yet implemented")
	} else {
		// Verify job status is pending/queued/running
		for _, job := range jobsResult.Jobs {
			if id, ok := job["id"].(string); ok && id == jobID {
				status, _ := job["status"].(string)
				validStatuses := []string{"pending", "queued", "running"}
				isValidStatus := false
				for _, validStatus := range validStatuses {
					if status == validStatus {
						isValidStatus = true
						break
					}
				}
				if !isValidStatus {
					t.Errorf("Job has unexpected status: %s (expected pending/queued/running)", status)
				} else {
					env.LogTest(t, "✓ Job has valid queue status: %s", status)
				}
				break
			}
		}

		env.LogTest(t, "✅ All job execution requirements verified successfully via API")
	}
}

// TestJobExecuteNonExistent verifies error handling when executing non-existent job definition
func TestJobExecuteNonExistent(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobExecuteNonExistent")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Attempt to execute non-existent job definition
	executeResp, err := h.POST("/api/job-definitions/non-existent-id-12345/execute", nil)
	if err != nil {
		t.Fatalf("Failed to make execute request: %v", err)
	}

	// Should return 404 Not Found
	if executeResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent job definition, got: %d", executeResp.StatusCode)
	}

	var errorResult map[string]interface{}
	if err := h.ParseJSONResponse(executeResp, &errorResult); err != nil {
		t.Logf("Warning: Failed to parse error response: %v", err)
	} else {
		if _, hasError := errorResult["error"]; !hasError {
			t.Error("Error response should contain 'error' field")
		}
	}

	env.LogTest(t, "✓ Correctly handled execution of non-existent job definition")
}
