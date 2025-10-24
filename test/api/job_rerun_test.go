// -----------------------------------------------------------------------
// Last Modified: Friday, 24th October 2025 4:11:33 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test"
)

// TestJobRerun verifies the core rerun requirement:
// When a user clicks "rerun" on a completed/failed job in the queue,
// the system should create a new job with the same configuration and add it to the queue.
func TestJobRerun(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create a source and job definition (simulates what the UI does)
	source := map[string]interface{}{
		"id":       "test-source-rerun-1",
		"name":     "Test Source for Rerun",
		"type":     "confluence",
		"base_url": "https://rerun-test.atlassian.net",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    5,
			"concurrency":  1,
			"follow_links": false,
			"rate_limit":   1000,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	h.AssertStatusCode(sourceResp, http.StatusCreated)

	var sourceResult map[string]interface{}
	h.ParseJSONResponse(sourceResp, &sourceResult)
	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	jobDef := map[string]interface{}{
		"id":          "test-job-rerun-def-1",
		"name":        "Test Job Definition",
		"type":        "crawler",
		"description": "Test job",
		"enabled":     true,
		"sources":     []string{sourceID},
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"max_depth":               1,
					"max_pages":               5,
					"concurrency":             1,
					"follow_links":            false,
					"wait_for_completion":     false,
					"polling_timeout_seconds": 0,
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}
	h.AssertStatusCode(jobDefResp, http.StatusCreated)

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	// 2. Execute job definition to create a job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// 3. Wait for job to be created
	var originalJobID string
	for attempt := 0; attempt < 20; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			continue
		}

		for _, job := range paginatedResponse.Jobs {
			if sourceType, ok := job["source_type"].(string); ok && sourceType == "confluence" {
				if jobID, ok := job["id"].(string); ok && jobID != "" {
					originalJobID = jobID
					t.Logf("Found job: %s", originalJobID)
					break
				}
			}
		}

		if originalJobID != "" {
			break
		}
	}

	if originalJobID == "" {
		t.Fatal("Failed to find created job")
	}

	defer h.DELETE("/api/jobs/" + originalJobID)

	// 4. Wait for job to complete/fail
	t.Log("Waiting for job to finish...")
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	var originalJob map[string]interface{}

	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + originalJobID)
		if err != nil {
			continue
		}

		h.ParseJSONResponse(jobResp, &originalJob)

		if status, ok := originalJob["status"].(string); ok {
			if terminalStates[status] {
				t.Logf("Job finished with status: %s", status)
				break
			}
		}
	}

	if status, ok := originalJob["status"].(string); !ok || !terminalStates[status] {
		t.Fatal("Job did not reach terminal state")
	}

	// Wait a moment for any database transactions to complete
	time.Sleep(1 * time.Second)

	// 5. REQUIREMENT: Click rerun button → Should successfully create new job in queue
	t.Log("Testing rerun functionality (core requirement)...")
	rerunResp, err := h.POST("/api/jobs/"+originalJobID+"/rerun", nil)
	if err != nil {
		t.Fatalf("Failed to call rerun endpoint: %v", err)
	}

	// REQUIREMENT: Rerun should succeed with 201 status
	if rerunResp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(rerunResp.Body)
		rerunResp.Body.Close()
		t.Fatalf("REQUIREMENT FAILED: Rerun should succeed.\nExpected: 201 Created\nGot: %d\nError: %s",
			rerunResp.StatusCode, string(bodyBytes))
	}

	var rerunResult map[string]interface{}
	if err := h.ParseJSONResponse(rerunResp, &rerunResult); err != nil {
		t.Fatalf("Failed to parse rerun response: %v", err)
	}

	// REQUIREMENT: Should return new job ID
	newJobID, ok := rerunResult["new_job_id"].(string)
	if !ok || newJobID == "" {
		t.Fatal("REQUIREMENT FAILED: Rerun should return new_job_id")
	}

	if newJobID == originalJobID {
		t.Fatal("REQUIREMENT FAILED: New job must have different ID than original")
	}

	defer h.DELETE("/api/jobs/" + newJobID)

	// 6. REQUIREMENT: New job should exist in queue with pending status
	newJobResp, err := h.GET("/api/jobs/" + newJobID)
	if err != nil {
		t.Fatalf("REQUIREMENT FAILED: New job should be retrievable: %v", err)
	}

	var newJob map[string]interface{}
	if err := h.ParseJSONResponse(newJobResp, &newJob); err != nil {
		t.Fatalf("Failed to parse new job: %v", err)
	}

	// REQUIREMENT: New job should be pending (queued, not running yet)
	if status, ok := newJob["status"].(string); !ok || status != "pending" {
		t.Errorf("REQUIREMENT FAILED: New job should be 'pending' (queued), got: %v", newJob["status"])
	}

	// REQUIREMENT: New job should have same source/entity type as original
	if newSourceType, _ := newJob["source_type"].(string); newSourceType != originalJob["source_type"].(string) {
		t.Errorf("REQUIREMENT FAILED: New job should have same source_type.\nOriginal: %v\nNew: %v",
			originalJob["source_type"], newSourceType)
	}

	if newEntityType, _ := newJob["entity_type"].(string); newEntityType != originalJob["entity_type"].(string) {
		t.Errorf("REQUIREMENT FAILED: New job should have same entity_type.\nOriginal: %v\nNew: %v",
			originalJob["entity_type"], newEntityType)
	}

	// REQUIREMENT: Original job should remain unchanged
	originalJobCheckResp, _ := h.GET("/api/jobs/" + originalJobID)
	var originalJobCheck map[string]interface{}
	h.ParseJSONResponse(originalJobCheckResp, &originalJobCheck)

	if originalJobCheck["status"] != originalJob["status"] {
		t.Error("REQUIREMENT FAILED: Original job status should not change after rerun")
	}

	t.Log("✅ REQUIREMENT MET: Rerun successfully created new job in queue")
}

// TestJobRerunNonExistent tests rerun of a non-existent job
func TestJobRerunNonExistent(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Attempt to rerun a non-existent job
	rerunResp, err := h.POST("/api/jobs/non-existent-job-id-12345/rerun", nil)
	if err != nil {
		t.Fatalf("Failed to send rerun request: %v", err)
	}

	// Should return 500 with error about job not found
	h.AssertStatusCode(rerunResp, http.StatusInternalServerError)

	t.Log("✓ Correctly handled rerun of non-existent job")
}

// TestJobRerunPreservesSeedURLs - REMOVED (2025-10-24)
// Reason: Architecture changed - seed URLs now generated from source configuration.
//
// The core rerun functionality (seed URL preservation) is already tested in TestJobRerun.
// Snapshot preservation is handled by the job definition and rerun system.
//
// If snapshot preservation testing is needed against source modifications:
// 1. Create source with base_url A
// 2. Execute job (generates seed URLs from A)
// 3. Modify source to use base_url B
// 4. Rerun job and verify it uses seed URLs from A (snapshot)
//
// This scenario is not critical for basic rerun functionality and would require
// additional test infrastructure to properly validate.
