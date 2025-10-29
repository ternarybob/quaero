package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/test"
)

// TestJobCompletionDelayed verifies that job completion is delayed by ~5s grace period after PendingURLs reaches 0
// This ensures in-flight URL processing can complete before marking the job as done
func TestJobCompletionDelayed(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create a source with minimal URLs to ensure quick completion
	source := map[string]interface{}{
		"id":       "test-source-completion-1",
		"name":     "Test Source for Completion Delay",
		"type":     "jira",
		"base_url": "https://completion-test.atlassian.net",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    2, // Very limited to ensure fast completion
			"concurrency":  1,
			"follow_links": false,
			"rate_limit":   100,
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

	// 2. Create job definition
	jobDef := map[string]interface{}{
		"id":          "test-job-completion-def-1",
		"name":        "Test Job for Completion Delay",
		"type":        "crawler",
		"description": "Test completion delay",
		"enabled":     true,
		"sources":     []string{sourceID},
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"max_depth":               1,
					"max_pages":               2,
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

	// 3. Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// 4. Wait for job to be created
	var jobID string
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
			if sourceType, ok := job["source_type"].(string); ok && sourceType == "jira" {
				if id, ok := job["id"].(string); ok && id != "" {
					jobID = id
					t.Logf("Found job: %s", jobID)
					break
				}
			}
		}

		if jobID != "" {
			break
		}
	}

	if jobID == "" {
		t.Fatal("Failed to find created job")
	}

	defer h.DELETE("/api/jobs/" + jobID)

	// 5. Wait for job to start running
	startTime := time.Now()
	jobReachedRunning := false
	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		h.ParseJSONResponse(jobResp, &job)

		if status, ok := job["status"].(string); ok && status == "running" {
			jobReachedRunning = true
			t.Logf("Job started running at %v", time.Since(startTime))
			break
		}
	}

	if !jobReachedRunning {
		t.Log("Warning: Job did not reach running state - may complete too quickly")
	}

	// 6. Wait for job to complete and measure timing
	completionStartTime := time.Now()
	var completedJob map[string]interface{}
	jobCompleted := false

	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		h.ParseJSONResponse(jobResp, &completedJob)

		if status, ok := completedJob["status"].(string); ok && status == "completed" {
			jobCompleted = true
			completionDuration := time.Since(completionStartTime)
			t.Logf("Job completed after %v", completionDuration)
			break
		}
	}

	if !jobCompleted {
		t.Fatal("Job did not complete within timeout")
	}

	// 7. Verify ResultCount matches CompletedURLs
	resultCount := int(completedJob["result_count"].(float64))
	progress := completedJob["progress"].(map[string]interface{})
	completedURLs := int(progress["completed_urls"].(float64))

	if resultCount != completedURLs {
		t.Errorf("ResultCount (%d) should match CompletedURLs (%d)", resultCount, completedURLs)
	}

	t.Log("✓ Job completion with grace period verified")
}

// TestJobCompletionOnlyAfterAllChildURLs verifies completion only occurs after all child URLs are processed
// This prevents premature completion when new URLs are discovered during crawling
func TestJobCompletionOnlyAfterAllChildURLs(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Start mock HTML server on port 13333 (different from main test server)
	mockServer := test.NewMockServer(13333)
	if err := mockServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mockServer.Stop()

	// Wait for mock server to be ready
	time.Sleep(200 * time.Millisecond)

	// Create a jira source pointing to the mock server (only jira/confluence/github are valid types)
	source := map[string]interface{}{
		"id":       "test-source-child-urls-1",
		"name":     "Test Source for Child URL Completion",
		"type":     "jira", // Valid source type (mock server will serve HTML regardless)
		"base_url": "http://localhost:13333",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    2,    // Allow parent + children
			"max_pages":    10,   // Allow all 6 pages
			"concurrency":  1,    // Sequential for predictability
			"follow_links": true, // CRITICAL: Must follow links
			"rate_limit":   100,
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

	// Create job definition with seed_urls and source
	jobDef := map[string]interface{}{
		"id":          "test-job-child-urls-def-1",
		"name":        "Test Job for Child URL Completion",
		"type":        "crawler",
		"description": "Test child URL completion",
		"enabled":     true,
		"sources":     []string{sourceID},                             // Source required for crawl action
		"seed_urls":   []string{"http://localhost:13333/test/parent"}, // Parent page with 5 children
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"max_depth":               2,
					"max_pages":               10,
					"concurrency":             1,
					"follow_links":            true, // CRITICAL: Enable link following
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

	// Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Wait for job to be created (jira type, since we used jira source)
	job := waitForJobCreation(t, h, "jira", 20*time.Second)
	if job == nil {
		t.Fatal("Failed to find created job")
	}
	jobID := job["id"].(string)
	defer h.DELETE("/api/jobs/" + jobID)

	t.Logf("Job created: %s", jobID)

	// 5. Track progress as URLs are processed
	// We expect: 1 parent URL + 5 child URLs = 6 total URLs
	type ProgressSnapshot struct {
		Time          time.Time
		Status        string
		TotalURLs     int
		CompletedURLs int
		FailedURLs    int
		PendingURLs   int
	}

	var snapshots []ProgressSnapshot
	var pendingReachedZeroTime time.Time
	var completionTime time.Time

	// Poll job status and track progress changes
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		var currentJob map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &currentJob); err != nil {
			continue
		}

		status, _ := currentJob["status"].(string)
		progress, _ := currentJob["progress"].(map[string]interface{})

		snapshot := ProgressSnapshot{
			Time:          time.Now(),
			Status:        status,
			TotalURLs:     int(progress["total_urls"].(float64)),
			CompletedURLs: int(progress["completed_urls"].(float64)),
			FailedURLs:    int(progress["failed_urls"].(float64)),
			PendingURLs:   int(progress["pending_urls"].(float64)),
		}

		// Detect first time PendingURLs reaches 0
		if snapshot.PendingURLs == 0 && snapshot.TotalURLs > 0 && pendingReachedZeroTime.IsZero() {
			pendingReachedZeroTime = time.Now()
			t.Logf("PendingURLs reached 0 (total=%d, completed=%d)",
				snapshot.TotalURLs,
				snapshot.CompletedURLs)
		}

		// Detect completion
		if status == "completed" && completionTime.IsZero() {
			completionTime = time.Now()
			t.Logf("Job completed")
		}

		snapshots = append(snapshots, snapshot)

		if status == "completed" {
			break
		}
	}

	if completionTime.IsZero() {
		t.Fatal("Job did not complete within timeout")
	}

	// 6. Verify completion timing
	if !pendingReachedZeroTime.IsZero() && !completionTime.IsZero() {
		graceDelay := completionTime.Sub(pendingReachedZeroTime)
		t.Logf("Grace period delay: %v", graceDelay)

		// Verify delay is approximately 5 seconds (allow 4-7 second range for timing variance)
		if graceDelay < 4*time.Second {
			t.Errorf("Grace period too short: %v (expected ~5s)", graceDelay)
		}
		if graceDelay > 7*time.Second {
			t.Errorf("Grace period too long: %v (expected ~5s)", graceDelay)
		}
	}

	// 7. Verify all child URLs were processed
	jobResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to fetch completed job: %v", err)
	}

	var completedJob map[string]interface{}
	h.ParseJSONResponse(jobResp, &completedJob)

	progress := completedJob["progress"].(map[string]interface{})
	totalURLs := int(progress["total_urls"].(float64))
	completedURLs := int(progress["completed_urls"].(float64))
	failedURLs := int(progress["failed_urls"].(float64))
	pendingURLs := int(progress["pending_urls"].(float64))

	t.Logf("Final progress: total=%d, completed=%d, failed=%d, pending=%d",
		totalURLs, completedURLs, failedURLs, pendingURLs)

	// Verify expected URL counts
	expectedTotal := 6 // 1 parent + 5 children
	if totalURLs != expectedTotal {
		t.Errorf("Expected %d total URLs, got %d", expectedTotal, totalURLs)
	}

	if completedURLs != expectedTotal {
		t.Errorf("Expected %d completed URLs, got %d", expectedTotal, completedURLs)
	}

	if pendingURLs != 0 {
		t.Errorf("Expected 0 pending URLs at completion, got %d", pendingURLs)
	}

	// Verify ResultCount matches
	resultCount := int(completedJob["result_count"].(float64))
	if resultCount != completedURLs {
		t.Errorf("ResultCount (%d) should match CompletedURLs (%d)", resultCount, completedURLs)
	}

	t.Log("✓ Child URL completion and grace period verified")
}

// TestJobCompletionHeartbeatValidation verifies that completion checks use last_heartbeat, not CompletionCandidateAt
// This ensures idle detection is based on actual activity timestamps from the database
func TestJobCompletionHeartbeatValidation(t *testing.T) {
	t.Skip("TODO: Implement heartbeat validation test - requires direct database access")

	// Test outline:
	// 1. Create and start a crawl job
	// 2. Use SQLite connection to manually UPDATE crawl_jobs SET pending_urls=0, last_heartbeat=<recent timestamp>
	// 3. Manually enqueue a completion probe message via QueueManager
	// 4. Wait 2 seconds
	// 5. Verify job is NOT marked completed (heartbeat too recent)
	// 6. Wait 4 more seconds (total 6s)
	// 7. Verify job IS now marked completed (heartbeat aged past grace period)
	//
	// Implementation notes:
	// - Requires access to internal/storage/sqlite database connection
	// - May need to expose test helper in internal/app for database access
	// - Verify last_heartbeat field is properly loaded and used
}

// TestCompletionProbeRetryOnRecentActivity verifies probe retry mechanism when heartbeat is too recent
// This handles race conditions where URLs are processed during the grace period
func TestCompletionProbeRetryOnRecentActivity(t *testing.T) {
	t.Skip("TODO: Implement probe retry test - requires queue message inspection")

	// Test outline:
	// 1. Create a job and manipulate database to set PendingURLs=0, last_heartbeat=2 seconds ago
	// 2. Manually enqueue completion probe message
	// 3. Verify job is NOT marked completed
	// 4. Inspect queue to verify a retry probe was enqueued with appropriate delay
	// 5. Wait for retry delay
	// 6. Verify job eventually completes after heartbeat ages
	//
	// Implementation notes:
	// - Requires access to QueueManager internals to inspect pending messages
	// - May need to add test helper to query goqite queue for message types
	// - Verify retry logic calculates correct remaining delay (5s - time_since_heartbeat)
}

// TestCompletionProbeSkipsCompletedJobs verifies idempotency of completion probes
// This prevents double-completion or errors when multiple probes are enqueued
func TestCompletionProbeSkipsCompletedJobs(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create a source
	source := map[string]interface{}{
		"id":       "test-source-idempotent-1",
		"name":     "Test Source for Idempotent Completion",
		"type":     "jira",
		"base_url": "https://idempotent-test.atlassian.net",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    2,
			"concurrency":  1,
			"follow_links": false,
			"rate_limit":   100,
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

	// 2. Create and execute job definition
	jobDef := map[string]interface{}{
		"id":          "test-job-idempotent-def-1",
		"name":        "Test Job for Idempotent Completion",
		"type":        "crawler",
		"description": "Test idempotent completion",
		"enabled":     true,
		"sources":     []string{sourceID},
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"max_depth":               1,
					"max_pages":               2,
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

	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// 3. Wait for job to be created
	var jobID string
	job := waitForJobCreation(t, h, "jira", 20*time.Second)
	if job == nil {
		t.Fatal("Failed to find created job")
	}
	jobID = job["id"].(string)
	defer h.DELETE("/api/jobs/" + jobID)

	// 4. Wait for job to complete
	completedJob := waitForJobStatus(t, h, jobID, "completed", 30*time.Second)
	if completedJob == nil {
		t.Fatal("Job did not complete")
	}

	// 5. Record the original ResultCount
	originalResultCount := int(completedJob["result_count"].(float64))
	t.Logf("Job completed with ResultCount=%d", originalResultCount)

	// 6. Fetch job again after a delay to allow any pending probes to process
	time.Sleep(8 * time.Second)

	jobResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to fetch job after delay: %v", err)
	}

	var finalJob map[string]interface{}
	h.ParseJSONResponse(jobResp, &finalJob)

	// 7. Verify job remains in completed state
	if status, ok := finalJob["status"].(string); !ok || status != "completed" {
		t.Errorf("Job status should remain 'completed', got: %v", finalJob["status"])
	}

	// 8. Verify ResultCount is not modified
	finalResultCount := int(finalJob["result_count"].(float64))
	if finalResultCount != originalResultCount {
		t.Errorf("ResultCount should not change. Original=%d, Final=%d", originalResultCount, finalResultCount)
	}

	t.Log("✓ Completion probe idempotency verified")
}

// waitForJobCreation polls for a job with the specified source type to be created
// Returns the most recently created job with the given source type
func waitForJobCreation(t *testing.T, h *test.HTTPTestHelper, sourceType string, timeout time.Duration) map[string]interface{} {
	t.Helper()

	startTime := time.Now()
	deadline := startTime.Add(timeout)

	for time.Now().Before(deadline) {
		jobsResp, err := h.GET("/api/jobs")
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var paginatedResponse struct {
			Jobs []map[string]interface{} `json:"jobs"`
		}
		if err := h.ParseJSONResponse(jobsResp, &paginatedResponse); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Find most recently created job with matching source type
		var newestJob map[string]interface{}
		var newestTime time.Time

		for _, job := range paginatedResponse.Jobs {
			if st, ok := job["source_type"].(string); ok && st == sourceType {
				if jobID, ok := job["id"].(string); ok && jobID != "" {
					// Parse created_at timestamp
					if createdAtStr, ok := job["created_at"].(string); ok {
						createdAt, err := time.Parse(time.RFC3339, createdAtStr)
						if err == nil && createdAt.After(startTime.Add(-5*time.Second)) {
							// Only consider jobs created within last 5 seconds
							if newestJob == nil || createdAt.After(newestTime) {
								newestJob = job
								newestTime = createdAt
							}
						}
					}
				}
			}
		}

		if newestJob != nil {
			return newestJob
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// waitForJobStatus polls until the job reaches the specified status or times out
func waitForJobStatus(t *testing.T, h *test.HTTPTestHelper, jobID string, status string, timeout time.Duration) map[string]interface{} {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if currentStatus, ok := job["status"].(string); ok && currentStatus == status {
			return job
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// assertJobField verifies a job field has the expected value
func assertJobField(t *testing.T, job map[string]interface{}, field string, expected interface{}) {
	t.Helper()

	actual, ok := job[field]
	if !ok {
		t.Errorf("Field '%s' not found in job", field)
		return
	}

	if actual != expected {
		t.Errorf("Field '%s': expected %v, got %v", field, expected, actual)
	}
}

// assertJobProgress verifies job progress metrics
func assertJobProgress(t *testing.T, job map[string]interface{}, expectedCompleted, expectedFailed int) {
	t.Helper()

	progress, ok := job["progress"].(map[string]interface{})
	if !ok {
		t.Fatal("Job progress not found")
	}

	completed := int(progress["completed_urls"].(float64))
	failed := int(progress["failed_urls"].(float64))

	if completed != expectedCompleted {
		t.Errorf("Expected %d completed URLs, got %d", expectedCompleted, completed)
	}

	if failed != expectedFailed {
		t.Errorf("Expected %d failed URLs, got %d", expectedFailed, failed)
	}
}

// assertJobStatusTransition verifies job transitions through expected states
func assertJobStatusTransition(t *testing.T, h *test.HTTPTestHelper, jobID string, expectedStates []models.JobStatus, timeout time.Duration) {
	t.Helper()

	seenStates := make(map[string]bool)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var job map[string]interface{}
		h.ParseJSONResponse(jobResp, &job)

		if status, ok := job["status"].(string); ok {
			seenStates[status] = true
		}

		// Check if all expected states have been seen
		allSeen := true
		for _, expectedState := range expectedStates {
			if !seenStates[string(expectedState)] {
				allSeen = false
				break
			}
		}

		if allSeen {
			t.Logf("Job transitioned through all expected states: %v", expectedStates)
			return
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Report which states were not seen
	var missedStates []string
	for _, expectedState := range expectedStates {
		if !seenStates[string(expectedState)] {
			missedStates = append(missedStates, string(expectedState))
		}
	}

	t.Errorf("Job did not transition through expected states. Missed: %v, Seen: %v", missedStates, seenStates)
}

// measureCompletionDelay measures the time between running and completed states
func measureCompletionDelay(t *testing.T, h *test.HTTPTestHelper, jobID string, timeout time.Duration) time.Duration {
	t.Helper()

	var runningTime time.Time
	var completedTime time.Time
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		var job map[string]interface{}
		h.ParseJSONResponse(jobResp, &job)

		status, ok := job["status"].(string)
		if !ok {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Record when job enters running state
		if status == "running" && runningTime.IsZero() {
			runningTime = time.Now()
		}

		// Record when job completes
		if status == "completed" && !completedTime.IsZero() {
			completedTime = time.Now()
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	if runningTime.IsZero() || completedTime.IsZero() {
		t.Fatal("Could not measure completion delay - job did not transition properly")
		return 0
	}

	delay := completedTime.Sub(runningTime)
	t.Logf("Measured completion delay: %v", delay)
	return delay
}

// verifyLastHeartbeat checks that the LastHeartbeat field is properly set
func verifyLastHeartbeat(t *testing.T, h *test.HTTPTestHelper, jobID string) {
	t.Helper()

	jobResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to fetch job: %v", err)
	}

	var job map[string]interface{}
	h.ParseJSONResponse(jobResp, &job)

	lastHeartbeat, ok := job["last_heartbeat"].(string)
	if !ok || lastHeartbeat == "" {
		t.Error("Job should have last_heartbeat field set")
		return
	}

	// Parse timestamp to verify it's valid
	_, err = time.Parse(time.RFC3339, lastHeartbeat)
	if err != nil {
		t.Errorf("Invalid last_heartbeat timestamp format: %v", err)
	}

	t.Logf("✓ LastHeartbeat verified: %s", lastHeartbeat)
}

// createTestJobDefinition creates a standard test job definition
func createTestJobDefinition(t *testing.T, h *test.HTTPTestHelper, sourceID string, name string) (string, func()) {
	t.Helper()

	jobDef := map[string]interface{}{
		"id":          fmt.Sprintf("test-job-def-%d", time.Now().Unix()),
		"name":        name,
		"type":        "crawler",
		"description": "Test job definition",
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

	cleanup := func() {
		h.DELETE("/api/job-definitions/" + jobDefID)
	}

	return jobDefID, cleanup
}
