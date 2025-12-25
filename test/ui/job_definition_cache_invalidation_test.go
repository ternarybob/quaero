package ui

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobDefinitionCacheInvalidation tests that cache is invalidated when job variables change.
// This test uses test_job_generator (NO LLM) to create fast, deterministic jobs.
//
// The test verifies that:
// 1. First job run with 3 workers produces specific log/child count
// 2. Job config is modified to use 5 workers
// 3. Second job run produces DIFFERENT output (different child count)
//
// This proves cache invalidation works when job configuration changes.
func TestJobDefinitionCacheInvalidation(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Cache Invalidation When Job Config Changes ---")

	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)
	jobTimeout := 2 * time.Minute

	// Step 1: Create job definition with test_job_generator (3 workers)
	defID := fmt.Sprintf("cache-invalidation-test-%d", time.Now().UnixNano())
	jobName := "Cache Invalidation Test"

	workerCountV1 := 3
	jobDef := createTestJobGeneratorDef(defID, jobName, workerCountV1)

	resp, err := httpHelper.POST("/api/job-definitions", jobDef)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create job definition")
	defer httpHelper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))

	// Save job definition V1 to results
	jobDefJSON, _ := json.MarshalIndent(jobDef, "", "  ")
	utc.SaveToResults("job_definition_v1.json", string(jobDefJSON))
	utc.Log("Created job definition V1: %s (worker_count=%d)", defID, workerCountV1)

	// Step 2: Run job first time
	utc.Log("Step 2: Running job first time (worker_count=%d)", workerCountV1)
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger first job: %v", err)
	}
	utc.Screenshot("first_job_triggered")

	// Navigate to Queue and monitor
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}
	time.Sleep(2 * time.Second)

	firstJobID := ""
	firstStatus := monitorJobToCompletion(t, utc, jobName, jobTimeout, &firstJobID)
	utc.Screenshot("first_job_completed")
	utc.Log("First job completed with status: %s (job_id: %s)", firstStatus, firstJobID)

	// Step 3: Get first job grandchild count (workers are grandchildren of manager)
	// Hierarchy: manager -> step -> worker jobs (test_job_generator)
	utc.Log("Step 3: Capturing first job grandchild count (worker count)")
	firstChildJobs := getChildJobsUI(t, httpHelper, firstJobID)
	firstGrandchildCount := 0
	for _, child := range firstChildJobs {
		if childID, ok := child["id"].(string); ok {
			grandchildren := getChildJobsUI(t, httpHelper, childID)
			firstGrandchildCount += len(grandchildren)
			utc.Log("  Step job %s has %d worker jobs", childID[:8], len(grandchildren))
		}
	}
	utc.Log("First job grandchild count: %d", firstGrandchildCount)

	// Save first job info
	firstJobInfo := fmt.Sprintf("First Job ID: %s\nStatus: %s\nWorker Count: %d\nWorker Count Config: %d\n",
		firstJobID, firstStatus, firstGrandchildCount, workerCountV1)
	utc.SaveToResults("first_job_info.txt", firstJobInfo)

	// Step 4: Delete first job so second job can be detected properly
	// (monitorJobToCompletion finds first matching job by name)
	utc.Log("Step 4: Deleting first job to allow second job detection")
	if firstJobID != "" {
		deleteJobUI(t, httpHelper, firstJobID)
	}
	time.Sleep(1 * time.Second)

	// Step 5: Update job definition with different worker count
	workerCountV2 := 5
	utc.Log("Step 5: Updating job definition (worker_count=%d -> %d)", workerCountV1, workerCountV2)

	updatedJobDef := createTestJobGeneratorDef(defID, jobName, workerCountV2)
	updateResp, err := httpHelper.PUT(fmt.Sprintf("/api/job-definitions/%s", defID), updatedJobDef)
	require.NoError(t, err, "Failed to update job definition")
	defer updateResp.Body.Close()
	require.Equal(t, 200, updateResp.StatusCode, "Failed to update job definition")

	// Save job definition V2 to results
	updatedJobDefJSON, _ := json.MarshalIndent(updatedJobDef, "", "  ")
	utc.SaveToResults("job_definition_v2.json", string(updatedJobDefJSON))

	// Step 6: Refresh job definitions
	utc.Log("Step 6: Refreshing job definitions")
	refreshResp, err := httpHelper.POST("/api/job-definitions/refresh", nil)
	if err == nil {
		refreshResp.Body.Close()
	}
	time.Sleep(1 * time.Second)

	// Step 7: Run job second time
	utc.Log("Step 7: Running job second time (worker_count=%d)", workerCountV2)
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger second job: %v", err)
	}
	utc.Screenshot("second_job_triggered")

	// Navigate to Queue and monitor
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}
	time.Sleep(2 * time.Second)

	secondJobID := ""
	secondStatus := monitorJobToCompletion(t, utc, jobName, jobTimeout, &secondJobID)
	utc.Screenshot("second_job_completed")
	utc.Log("Second job completed with status: %s (job_id: %s)", secondStatus, secondJobID)

	// Step 8: Get second job grandchild count (workers are grandchildren of manager)
	utc.Log("Step 8: Capturing second job grandchild count (worker count)")
	secondChildJobs := getChildJobsUI(t, httpHelper, secondJobID)
	secondGrandchildCount := 0
	for _, child := range secondChildJobs {
		if childID, ok := child["id"].(string); ok {
			grandchildren := getChildJobsUI(t, httpHelper, childID)
			secondGrandchildCount += len(grandchildren)
			utc.Log("  Step job %s has %d worker jobs", childID[:8], len(grandchildren))
		}
	}
	utc.Log("Second job grandchild count: %d", secondGrandchildCount)

	// Save second job info
	secondJobInfo := fmt.Sprintf("Second Job ID: %s\nStatus: %s\nWorker Count: %d\nWorker Count Config: %d\n",
		secondJobID, secondStatus, secondGrandchildCount, workerCountV2)
	utc.SaveToResults("second_job_info.txt", secondJobInfo)

	// Step 9: Assert child counts are DIFFERENT
	utc.Log("Step 9: Asserting child counts are different (cache invalidation)")

	// The critical assertion: worker counts (grandchildren) should match their respective worker_count configs
	assert.Equal(t, workerCountV1, firstGrandchildCount,
		"First job should have %d workers (matching worker_count=%d)", workerCountV1, workerCountV1)

	assert.Equal(t, workerCountV2, secondGrandchildCount,
		"Second job should have %d workers (matching worker_count=%d)", workerCountV2, workerCountV2)

	assert.NotEqual(t, firstGrandchildCount, secondGrandchildCount,
		"CACHE BUG: First and second worker counts are identical! Cache was NOT invalidated when config changed.")

	utc.Log("✓ Cache invalidation verified:")
	utc.Log("  First job:  %d workers (config: worker_count=%d)", firstGrandchildCount, workerCountV1)
	utc.Log("  Second job: %d workers (config: worker_count=%d)", secondGrandchildCount, workerCountV2)

	// Step 10: Cleanup second job (first already deleted in step 4)
	utc.Log("Step 10: Cleanup")
	if secondJobID != "" {
		deleteJobUI(t, httpHelper, secondJobID)
	}

	utc.RefreshAndScreenshot("final_state")
	utc.Log("✓ Cache invalidation test completed")
}

// createTestJobGeneratorDef creates a test job generator job definition
func createTestJobGeneratorDef(id, name string, workerCount int) map[string]interface{} {
	return map[string]interface{}{
		"id":          id,
		"name":        name,
		"type":        "custom",
		"enabled":     true,
		"description": fmt.Sprintf("Cache invalidation test with %d workers", workerCount),
		"steps": []map[string]interface{}{
			{
				"name":        "generate_jobs",
				"type":        "test_job_generator",
				"description": fmt.Sprintf("Generate %d test worker jobs", workerCount),
				"on_error":    "continue",
				"config": map[string]interface{}{
					"worker_count":    workerCount,
					"log_count":       5,
					"log_delay_ms":    10,
					"failure_rate":    0.0, // No failures for predictable test
					"child_count":     0,
					"recursion_depth": 0,
				},
			},
		},
	}
}

// Note: httpGetter interface is defined in job_definition_stock_caching_test.go
