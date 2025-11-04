// -----------------------------------------------------------------------
// Last Modified: Monday, 3rd November 2025
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"net/http"
	"testing"
)

// TestJobDeletion_SingleJob tests deleting a single job via HTTP API
func TestJobDeletion_SingleJob(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobDeletion_SingleJob")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create a source first
	resp, err := h.POST("/api/sources", map[string]interface{}{
		"name": "Test Source - Job Deletion",
		"url":  "https://deletion-test.example.com",
		"type": "html",
	})
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	h.AssertStatusCode(resp, http.StatusOK)

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(resp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}
	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// Create a job
	jobResp, err := h.POST("/api/jobs/create", map[string]interface{}{
		"source_id": sourceID,
		"url":       "https://deletion-test.example.com/page",
		"type":      "crawl",
	})
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}
	h.AssertStatusCode(jobResp, http.StatusOK)

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}
	jobID := jobResult["job_id"].(string)
	t.Logf("Created job: %s", jobID)

	// Verify job exists before deletion
	getResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Job should exist before deletion, got status: %d", getResp.StatusCode)
	}

	// Delete the job
	deleteResp, err := h.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}
	h.AssertStatusCode(deleteResp, http.StatusOK)

	var deleteResult map[string]interface{}
	if err := h.ParseJSONResponse(deleteResp, &deleteResult); err != nil {
		t.Fatalf("Failed to parse delete response: %v", err)
	}

	// Verify response
	if deleteResult["job_id"] != jobID {
		t.Errorf("Expected deleted job_id %s, got: %v", jobID, deleteResult["job_id"])
	}
	if deleteResult["message"] != "Job deleted successfully" {
		t.Errorf("Expected success message, got: %v", deleteResult["message"])
	}
	t.Logf("✓ Job deleted successfully: %s", jobID)

	// Verify job no longer exists
	verifyResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to verify deletion: %v", err)
	}
	if verifyResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 after deletion, got: %d", verifyResp.StatusCode)
	}

	t.Log("✓ Job deletion verified")
}

// TestJobDeletion_NonExistentJob tests deleting a job that doesn't exist
func TestJobDeletion_NonExistentJob(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobDeletion_NonExistentJob")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	nonExistentID := "job-does-not-exist-12345"
	deleteResp, err := h.DELETE("/api/jobs/" + nonExistentID)
	if err != nil {
		t.Fatalf("Failed to delete non-existent job: %v", err)
	}

	if deleteResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent job, got: %d", deleteResp.StatusCode)
	}

	var errorResult map[string]interface{}
	if err := h.ParseJSONResponse(deleteResp, &errorResult); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResult["error"] == nil {
		t.Error("Expected error field in response")
	}

	t.Log("✓ Non-existent job deletion handled correctly")
}

// TestJobDeletion_IdempotentDelete tests that deleting the same job multiple times is safe
func TestJobDeletion_IdempotentDelete(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobDeletion_IdempotentDelete")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	resp, err := h.POST("/api/sources", map[string]interface{}{
		"name": "Idempotent Deletion Test",
		"url":  "https://idempotent.example.com",
		"type": "html",
	})
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	h.AssertStatusCode(resp, http.StatusOK)

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(resp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}
	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	jobResp, err := h.POST("/api/jobs/create", map[string]interface{}{
		"source_id": sourceID,
		"url":       "https://idempotent.example.com/test",
		"type":      "crawl",
	})
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}
	jobID := jobResult["job_id"].(string)
	t.Logf("Created job: %s", jobID)

	// First deletion
	deleteResp1, err := h.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed first deletion: %v", err)
	}
	h.AssertStatusCode(deleteResp1, http.StatusOK)
	t.Log("✓ First deletion successful")

	// Second deletion (should return 404)
	deleteResp2, err := h.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed second deletion: %v", err)
	}
	if deleteResp2.StatusCode != http.StatusNotFound {
		t.Errorf("Second deletion should return 404, got: %d", deleteResp2.StatusCode)
	}
	t.Log("✓ Second deletion correctly returned 404")

	// Third deletion (should also return 404)
	deleteResp3, err := h.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed third deletion: %v", err)
	}
	if deleteResp3.StatusCode != http.StatusNotFound {
		t.Errorf("Third deletion should return 404, got: %d", deleteResp3.StatusCode)
	}
	t.Log("✓ Idempotent deletion behavior verified")
}

// TestJobDeletion_ResponseFormat verifies the structure of the deletion response
func TestJobDeletion_ResponseFormat(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobDeletion_ResponseFormat")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	resp, err := h.POST("/api/sources", map[string]interface{}{
		"name": "Response Format Test",
		"url":  "https://response-format.example.com",
		"type": "html",
	})
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var sourceResult map[string]interface{}
	h.ParseJSONResponse(resp, &sourceResult)
	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	jobResp, err := h.POST("/api/jobs/create", map[string]interface{}{
		"source_id": sourceID,
		"url":       "https://response-format.example.com/test",
		"type":      "crawl",
	})
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	var jobResult map[string]interface{}
	h.ParseJSONResponse(jobResp, &jobResult)
	jobID := jobResult["job_id"].(string)

	deleteResp, err := h.DELETE("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}
	h.AssertStatusCode(deleteResp, http.StatusOK)

	var result map[string]interface{}
	if err := h.ParseJSONResponse(deleteResp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{"job_id", "message", "cascade_deleted", "child_count"}
	for _, field := range requiredFields {
		if _, exists := result[field]; !exists {
			t.Errorf("Response missing required field: %s", field)
		}
	}

	if result["job_id"] != jobID {
		t.Errorf("Expected job_id %s, got: %v", jobID, result["job_id"])
	}

	if result["message"] != "Job deleted successfully" {
		t.Errorf("Expected success message, got: %v", result["message"])
	}

	if cascadeDeleted, ok := result["cascade_deleted"].(float64); !ok || cascadeDeleted < 0 {
		t.Errorf("Invalid cascade_deleted value: %v", result["cascade_deleted"])
	}

	if childCount, ok := result["child_count"].(float64); !ok || childCount < 0 {
		t.Errorf("Invalid child_count value: %v", result["child_count"])
	}

	t.Log("✓ Response format verified")
}

// TestJobDeletion_ErrorResponseFormat verifies error response structure
func TestJobDeletion_ErrorResponseFormat(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobDeletion_ErrorResponseFormat")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	nonExistentID := "job-error-format-test"
	deleteResp, err := h.DELETE("/api/jobs/" + nonExistentID)
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	if deleteResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got: %d", deleteResp.StatusCode)
	}

	var result map[string]interface{}
	if err := h.ParseJSONResponse(deleteResp, &result); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	requiredErrorFields := []string{"error", "details", "job_id"}
	for _, field := range requiredErrorFields {
		if _, exists := result[field]; !exists {
			t.Errorf("Error response missing required field: %s", field)
		}
	}

	if result["job_id"] != nonExistentID {
		t.Errorf("Expected job_id %s in error response, got: %v", nonExistentID, result["job_id"])
	}

	t.Log("✓ Error response format verified")
}

// TestJobDeletion_ParentWithChildren tests cascade deletion with real job hierarchy
func TestJobDeletion_ParentWithChildren(t *testing.T) {
	t.Skip("Requires job definition execution - use integration environment")
}
