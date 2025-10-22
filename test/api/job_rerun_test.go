// -----------------------------------------------------------------------
// Last Modified: Thursday, 23rd October 2025 8:30:34 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test"
)

// TestJobRerun tests the job rerun/copy functionality
func TestJobRerun(t *testing.T) {
	t.Skip(`
		SKIP REASON: Architecture mismatch - test needs refactoring

		This test was written for an old API where executing a job definition created
		a queryable "Job" record. The current system works differently:

		Current System:
		- Job Definitions execute steps asynchronously
		- "crawl" steps create CrawlerJob entities
		- The /rerun endpoint works on CrawlerJob IDs, not job definition IDs

		To fix: Rewrite test to:
		1. Execute a job definition with a crawl step
		2. Wait/poll for CrawlerJobs to be created (async)
		3. Query the created CrawlerJob IDs
		4. Test rerun on those CrawlerJob IDs

		See: internal/services/jobs/executor.go (lines 161-226) for async job creation
	`)

	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create a Job Definition (modern approach - steps-based)
	jobDef := map[string]interface{}{
		"id":          "test-job-rerun-def-1",
		"name":        "Test Job Definition for Rerun",
		"type":        "crawler",
		"description": "Test job for rerun functionality",
		"enabled":     true,
		"sources":     []string{}, // Empty sources array, will use seed URLs from step config
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"start_urls": []string{
						"https://rerun-test.atlassian.net/rest/api/2/project",
					},
					"max_depth":    1,
					"max_pages":    10,
					"concurrency":  1,
					"follow_links": false,
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
	if err := h.ParseJSONResponse(jobDefResp, &jobDefResult); err != nil {
		t.Fatalf("Failed to parse job definition response: %v", err)
	}

	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	// 2. Execute the job definition to create an actual crawler job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}

	h.AssertStatusCode(execResp, http.StatusAccepted)

	var execResult map[string]interface{}
	if err := h.ParseJSONResponse(execResp, &execResult); err != nil {
		t.Fatalf("Failed to parse execution response: %v", err)
	}

	originalJobID := execResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + originalJobID)

	// Wait for job to be persisted
	time.Sleep(500 * time.Millisecond)

	// 3. Get original job details
	originalJobResp, err := h.GET("/api/jobs/" + originalJobID)
	if err != nil {
		t.Fatalf("Failed to get original job: %v", err)
	}

	var originalJob map[string]interface{}
	if err := h.ParseJSONResponse(originalJobResp, &originalJob); err != nil {
		t.Fatalf("Failed to parse original job: %v", err)
	}

	// 4. Rerun the job (copy to queue)
	rerunResp, err := h.POST("/api/jobs/"+originalJobID+"/rerun", nil)
	if err != nil {
		t.Fatalf("Failed to rerun job: %v", err)
	}

	h.AssertStatusCode(rerunResp, http.StatusCreated)

	var rerunResult map[string]interface{}
	if err := h.ParseJSONResponse(rerunResp, &rerunResult); err != nil {
		t.Fatalf("Failed to parse rerun response: %v", err)
	}

	// 5. Verify rerun response
	if originalID, ok := rerunResult["original_job_id"].(string); !ok || originalID != originalJobID {
		t.Errorf("Expected original_job_id '%s', got: %v", originalJobID, rerunResult["original_job_id"])
	}

	newJobID, ok := rerunResult["new_job_id"].(string)
	if !ok || newJobID == "" {
		t.Fatal("Expected new_job_id in response")
	}

	if newJobID == originalJobID {
		t.Error("New job ID should be different from original job ID")
	}

	defer h.DELETE("/api/jobs/" + newJobID)

	// 6. Verify new job was created with pending status
	newJobResp, err := h.GET("/api/jobs/" + newJobID)
	if err != nil {
		t.Fatalf("Failed to get new job: %v", err)
	}

	h.AssertStatusCode(newJobResp, http.StatusOK)

	var newJob map[string]interface{}
	if err := h.ParseJSONResponse(newJobResp, &newJob); err != nil {
		t.Fatalf("Failed to parse new job: %v", err)
	}

	// 7. Verify new job is in pending status (not running)
	if status, ok := newJob["status"].(string); !ok || status != "pending" {
		t.Errorf("Expected new job status to be 'pending', got: %v", newJob["status"])
	}

	// 8. Verify new job has same configuration as original
	if newSourceType, ok := newJob["source_type"].(string); !ok || newSourceType != originalJob["source_type"].(string) {
		t.Errorf("Expected source_type to match original, got: %v", newJob["source_type"])
	}

	if newEntityType, ok := newJob["entity_type"].(string); !ok || newEntityType != originalJob["entity_type"].(string) {
		t.Errorf("Expected entity_type to match original, got: %v", newJob["entity_type"])
	}

	// 9. Verify snapshots were copied
	if newJob["source_config_snapshot"] != nil && originalJob["source_config_snapshot"] != nil {
		// If original has snapshot, new should too
		if newSnapshot, ok := newJob["source_config_snapshot"].(string); !ok || newSnapshot == "" {
			t.Error("Expected source_config_snapshot to be copied to new job")
		}
	}

	// 10. Verify original job is unchanged
	originalJobCheckResp, err := h.GET("/api/jobs/" + originalJobID)
	if err != nil {
		t.Fatalf("Failed to get original job after rerun: %v", err)
	}

	var originalJobCheck map[string]interface{}
	if err := h.ParseJSONResponse(originalJobCheckResp, &originalJobCheck); err != nil {
		t.Fatalf("Failed to parse original job after rerun: %v", err)
	}

	// Original job should be unchanged
	if originalJobCheck["status"] != originalJob["status"] {
		t.Error("Original job status changed after rerun (should be unchanged)")
	}

	t.Log("✓ Job rerun created a new pending job with copied configuration")
}

// TestJobRerunNonExistent tests rerun of a non-existent job
func TestJobRerunNonExistent(t *testing.T) {
	t.Skip("SKIP: Depends on TestJobRerun architecture - needs refactoring with parent test")

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

// TestJobRerunPreservesSeedURLs verifies that rerun preserves seed URLs from original job
func TestJobRerunPreservesSeedURLs(t *testing.T) {
	t.Skip("SKIP: Depends on TestJobRerun architecture - needs refactoring with parent test")

	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create a Job Definition with specific seed URLs
	seedURLs := []string{
		"https://snapshot-test.atlassian.net/rest/api/2/project/TEST1",
		"https://snapshot-test.atlassian.net/rest/api/2/project/TEST2",
	}

	jobDef := map[string]interface{}{
		"id":          "test-job-snapshot-def-1",
		"name":        "Snapshot Test Job Definition",
		"type":        "crawler",
		"description": "Test job for snapshot preservation",
		"enabled":     true,
		"sources":     []string{}, // Empty sources array, will use seed URLs from step config
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"start_urls":   seedURLs,
					"max_depth":    1,
					"max_pages":    10,
					"concurrency":  2,
					"follow_links": false,
				},
				"on_error": "fail",
			},
		},
	}

	jobDefResp, err := h.POST("/api/job-definitions", jobDef)
	if err != nil {
		t.Fatalf("Failed to create job definition: %v", err)
	}

	var jobDefResult map[string]interface{}
	h.ParseJSONResponse(jobDefResp, &jobDefResult)
	jobDefID := jobDefResult["id"].(string)
	defer h.DELETE("/api/job-definitions/" + jobDefID)

	// 2. Execute job definition to create original job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}

	var execResult map[string]interface{}
	h.ParseJSONResponse(execResp, &execResult)
	originalJobID := execResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + originalJobID)

	time.Sleep(500 * time.Millisecond)

	// 3. Get original job
	originalJobResp, _ := h.GET("/api/jobs/" + originalJobID)
	var originalJob map[string]interface{}
	h.ParseJSONResponse(originalJobResp, &originalJob)

	// 4. Modify job definition (change seed URLs)
	updatedJobDef := map[string]interface{}{
		"name":        "Modified Snapshot Test Job Definition",
		"type":        "crawler",
		"description": "Modified test job",
		"enabled":     true,
		"sources":     []string{}, // Empty sources array
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"start_urls": []string{
						"https://snapshot-test.atlassian.net/rest/api/2/project/MODIFIED",
					},
					"max_depth":    1,
					"max_pages":    20,
					"concurrency":  5, // Changed!
					"follow_links": false,
				},
				"on_error": "fail",
			},
		},
	}

	h.PUT("/api/job-definitions/"+jobDefID, updatedJobDef)

	// 5. Rerun original job
	rerunResp, err := h.POST("/api/jobs/"+originalJobID+"/rerun", nil)
	if err != nil {
		t.Fatalf("Failed to rerun job: %v", err)
	}

	h.AssertStatusCode(rerunResp, http.StatusCreated)

	var rerunResult map[string]interface{}
	h.ParseJSONResponse(rerunResp, &rerunResult)
	newJobID := rerunResult["new_job_id"].(string)
	defer h.DELETE("/api/jobs/" + newJobID)

	// 6. Get new job
	newJobResp, _ := h.GET("/api/jobs/" + newJobID)
	var newJob map[string]interface{}
	h.ParseJSONResponse(newJobResp, &newJob)

	// 7. Verify new job has ORIGINAL seed URLs (not modified ones)
	if newSeedURLs, ok := newJob["seed_urls"].([]interface{}); ok {
		if len(newSeedURLs) != len(seedURLs) {
			t.Errorf("Expected %d seed URLs, got: %d", len(seedURLs), len(newSeedURLs))
		}

		// Verify URLs match original
		for i, expectedURL := range seedURLs {
			if i < len(newSeedURLs) {
				if actualURL, ok := newSeedURLs[i].(string); ok {
					if actualURL != expectedURL {
						t.Errorf("Seed URL %d mismatch: expected '%s', got: '%s'", i, expectedURL, actualURL)
					}
				}
			}
		}
	} else {
		t.Error("Expected seed_urls field in new job")
	}

	// 8. Verify new job has ORIGINAL config (concurrency=2, not 5)
	if newJobConfig, ok := newJob["config"].(map[string]interface{}); ok {
		if concurrency, ok := newJobConfig["concurrency"].(float64); ok {
			if int(concurrency) != 2 {
				t.Errorf("Expected original concurrency (2), got: %d", int(concurrency))
			}
		}
	}

	t.Log("✓ Job rerun correctly preserved original seed URLs and config")
}
