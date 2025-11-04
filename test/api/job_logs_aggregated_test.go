// -----------------------------------------------------------------------
// Last Modified: Friday, 1st November 2025
// Modified By: Claude Code
// -----------------------------------------------------------------------

package api

import (
	"io"
	"net/http"
	"testing"
	"time"
)

// TestJobLogsAggregated_ParentOnly tests the aggregated logs endpoint for a parent job without children
func TestJobLogsAggregated_ParentOnly(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_ParentOnly")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create a source and job definition
	source := map[string]interface{}{
		"id":       "test-source-aggregated-logs-1",
		"name":     "Test Source for Aggregated Logs",
		"type":     "jira",
		"base_url": "https://logs-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    3,
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
		"id":          "test-job-def-aggregated-1",
		"name":        "Test Job Definition for Aggregated Logs",
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
					"max_pages":               3,
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
	var parentJobID string
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
				if jobID, ok := job["id"].(string); ok && jobID != "" {
					// Only get parent jobs (no parent_id)
					if parentID, hasParent := job["parent_id"].(string); !hasParent || parentID == "" {
						parentJobID = jobID
						t.Logf("Found parent job: %s", parentJobID)
						break
					}
				}
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Failed to find created parent job")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// 4. Wait for job to complete/fail
	t.Log("Waiting for job to finish...")
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	var parentJob map[string]interface{}

	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		h.ParseJSONResponse(jobResp, &parentJob)

		if status, ok := parentJob["status"].(string); ok {
			if terminalStates[status] {
				t.Logf("Job finished with status: %s", status)
				break
			}
		}
	}

	if status, ok := parentJob["status"].(string); !ok || !terminalStates[status] {
		t.Fatal("Job did not reach terminal state")
	}

	// Wait a moment for logs to be written
	time.Sleep(1 * time.Second)

	// 5. Test aggregated logs endpoint - parent only
	t.Log("Testing aggregated logs endpoint (parent only)...")
	aggregatedResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?include_children=false")
	if err != nil {
		t.Fatalf("Failed to get aggregated logs: %v", err)
	}

	h.AssertStatusCode(aggregatedResp, http.StatusOK)

	var aggregatedResult struct {
		JobID           string                   `json:"job_id"`
		Logs            []map[string]interface{} `json:"logs"`
		Count           int                      `json:"count"`
		Order           string                   `json:"order"`
		Level           string                   `json:"level"`
		IncludeChildren bool                     `json:"include_children"`
		Metadata        map[string]interface{}   `json:"metadata"`
	}

	if err := h.ParseJSONResponse(aggregatedResp, &aggregatedResult); err != nil {
		t.Fatalf("Failed to parse aggregated logs response: %v", err)
	}

	// Verify response structure
	if aggregatedResult.JobID != parentJobID {
		t.Errorf("Expected job_id to match: got %s, want %s", aggregatedResult.JobID, parentJobID)
	}

	if aggregatedResult.IncludeChildren != false {
		t.Errorf("Expected include_children to be false: got %v", aggregatedResult.IncludeChildren)
	}

	if aggregatedResult.Level != "all" {
		t.Errorf("Expected level to be 'all': got %s", aggregatedResult.Level)
	}

	// Should have logs (job execution generates logs)
	if aggregatedResult.Count == 0 {
		t.Log("Warning: No logs found for job (may be normal if job failed before logging)")
	} else {
		t.Logf("✓ Found %d log entries", aggregatedResult.Count)

		// Verify log structure
		if len(aggregatedResult.Logs) > 0 {
			firstLog := aggregatedResult.Logs[0]
			if _, ok := firstLog["timestamp"]; !ok {
				t.Error("Log entry missing 'timestamp' field")
			}
			if _, ok := firstLog["level"]; !ok {
				t.Error("Log entry missing 'level' field")
			}
			if _, ok := firstLog["message"]; !ok {
				t.Error("Log entry missing 'message' field")
			}
			if _, ok := firstLog["job_id"]; !ok {
				t.Error("Log entry missing 'job_id' field")
			}
		}
	}

	// Verify metadata
	if aggregatedResult.Metadata == nil {
		t.Error("Expected metadata in response")
	} else {
		if _, ok := aggregatedResult.Metadata[parentJobID]; !ok {
			t.Errorf("Expected metadata for job %s", parentJobID)
		} else {
			t.Log("✓ Metadata present in response")
		}
	}

	t.Log("✅ Parent-only aggregated logs test passed")
}

// TestJobLogsAggregated_WithChildren tests the aggregated logs endpoint for a parent job with child jobs
func TestJobLogsAggregated_WithChildren(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_WithChildren")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create a source and job definition that will create child jobs
	source := map[string]interface{}{
		"id":       "test-source-aggregated-logs-2",
		"name":     "Test Source for Child Jobs",
		"type":     "jira",
		"base_url": "https://child-logs-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    2, // This will create child jobs
			"max_pages":    10,
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
		"id":          "test-job-def-aggregated-2",
		"name":        "Test Job Definition with Children",
		"type":        "crawler",
		"description": "Test job with child jobs",
		"enabled":     true,
		"sources":     []string{sourceID},
		"steps": []map[string]interface{}{
			{
				"name":   "crawl-step",
				"action": "crawl",
				"config": map[string]interface{}{
					"max_depth":               2, // Creates child jobs
					"max_pages":               10,
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

	// 2. Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// 3. Wait for parent job to be created
	var parentJobID string
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
				if jobID, ok := job["id"].(string); ok && jobID != "" {
					// Only get parent jobs (no parent_id)
					if parentID, hasParent := job["parent_id"].(string); !hasParent || parentID == "" {
						parentJobID = jobID
						t.Logf("Found parent job: %s", parentJobID)
						break
					}
				}
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Failed to find created parent job")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// 4. Wait for jobs to complete
	t.Log("Waiting for jobs to finish...")
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}

	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(1 * time.Second)

		// Check parent job
		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var parentJob map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &parentJob); err != nil {
			continue
		}

		if status, ok := parentJob["status"].(string); ok {
			if terminalStates[status] {
				t.Logf("Parent job finished with status: %s", status)
				break
			}
		}
	}

	// Wait a moment for all logs to be written
	time.Sleep(2 * time.Second)

	// 5. Get list of jobs to find children
	allJobsResp, err := h.GET("/api/jobs?grouped=true")
	if err != nil {
		t.Fatalf("Failed to get grouped jobs: %v", err)
	}

	var groupedResponse struct {
		Groups  []map[string]interface{} `json:"groups"`
		Orphans []map[string]interface{} `json:"orphans"`
	}
	if err := h.ParseJSONResponse(allJobsResp, &groupedResponse); err != nil {
		t.Fatalf("Failed to parse grouped jobs response: %v", err)
	}

	hasChildren := false
	for _, group := range groupedResponse.Groups {
		if parent, ok := group["parent"].(map[string]interface{}); ok {
			if pid, ok := parent["id"].(string); ok && pid == parentJobID {
				if children, ok := group["children"].([]interface{}); ok && len(children) > 0 {
					hasChildren = true
					t.Logf("✓ Found %d child jobs", len(children))
					break
				}
			}
		}
	}

	// 6. Test aggregated logs endpoint with children
	t.Log("Testing aggregated logs endpoint (with children)...")
	aggregatedResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?include_children=true")
	if err != nil {
		t.Fatalf("Failed to get aggregated logs: %v", err)
	}

	h.AssertStatusCode(aggregatedResp, http.StatusOK)

	var aggregatedResult struct {
		JobID           string                   `json:"job_id"`
		Logs            []map[string]interface{} `json:"logs"`
		Count           int                      `json:"count"`
		Order           string                   `json:"order"`
		Level           string                   `json:"level"`
		IncludeChildren bool                     `json:"include_children"`
		Metadata        map[string]interface{}   `json:"metadata"`
	}

	if err := h.ParseJSONResponse(aggregatedResp, &aggregatedResult); err != nil {
		t.Fatalf("Failed to parse aggregated logs response: %v", err)
	}

	// Verify response structure
	if aggregatedResult.JobID != parentJobID {
		t.Errorf("Expected job_id to match: got %s, want %s", aggregatedResult.JobID, parentJobID)
	}

	if aggregatedResult.IncludeChildren != true {
		t.Errorf("Expected include_children to be true: got %v", aggregatedResult.IncludeChildren)
	}

	t.Logf("✓ Found %d total log entries (parent + children)", aggregatedResult.Count)

	// If we have children, verify we have logs from multiple jobs
	if hasChildren {
		if aggregatedResult.Count > 0 {
			// Check that logs have job_id field indicating which job they came from
			uniqueJobIDs := make(map[string]bool)
			for _, log := range aggregatedResult.Logs {
				if jobID, ok := log["job_id"].(string); ok {
					uniqueJobIDs[jobID] = true
				}
			}

			if len(uniqueJobIDs) > 1 {
				t.Logf("✓ Logs from %d different jobs found", len(uniqueJobIDs))
			} else {
				t.Log("Warning: Expected logs from multiple jobs, but only found one")
			}
		}
	}

	// Verify metadata has entries for parent and children
	if aggregatedResult.Metadata != nil {
		metadataCount := len(aggregatedResult.Metadata)
		t.Logf("✓ Metadata contains entries for %d jobs", metadataCount)

		if hasChildren && metadataCount < 2 {
			t.Errorf("Expected metadata for parent and children, got %d entries", metadataCount)
		}
	}

	t.Log("✅ Aggregated logs with children test passed")
}

// TestJobLogsAggregated_LevelFiltering tests level filtering in aggregated logs
func TestJobLogsAggregated_LevelFiltering(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_LevelFiltering")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create a simple source and job
	source := map[string]interface{}{
		"id":       "test-source-level-filter-1",
		"name":     "Test Source for Level Filtering",
		"type":     "confluence",
		"base_url": "https://level-filter-test.atlassian.net",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    3,
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
		"id":          "test-job-def-level-filter-1",
		"name":        "Test Job for Level Filtering",
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
					"max_pages":               3,
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

	// Execute job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Wait for job
	var parentJobID string
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
					if parentID, hasParent := job["parent_id"].(string); !hasParent || parentID == "" {
						parentJobID = jobID
						break
					}
				}
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Failed to find created job")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// Wait for job to complete
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		if status, ok := job["status"].(string); ok {
			if terminalStates[status] {
				break
			}
		}
	}

	time.Sleep(1 * time.Second)

	// Test different level filters
	t.Log("Testing level filter: error...")
	errorResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?level=error")
	if err != nil {
		t.Fatalf("Failed to get error logs: %v", err)
	}
	h.AssertStatusCode(errorResp, http.StatusOK)

	var errorResult struct {
		Level string                   `json:"level"`
		Logs  []map[string]interface{} `json:"logs"`
		Count int                      `json:"count"`
	}
	if err := h.ParseJSONResponse(errorResp, &errorResult); err != nil {
		t.Fatalf("Failed to parse error logs response: %v", err)
	}

	if errorResult.Level != "error" {
		t.Errorf("Expected level to be 'error': got %s", errorResult.Level)
	}

	t.Logf("✓ Level filter test passed - found %d error logs", errorResult.Count)

	t.Log("Testing level filter: all...")
	allResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?level=all")
	if err != nil {
		t.Fatalf("Failed to get all logs: %v", err)
	}
	h.AssertStatusCode(allResp, http.StatusOK)

	var allResult struct {
		Level string `json:"level"`
		Count int    `json:"count"`
	}
	if err := h.ParseJSONResponse(allResp, &allResult); err != nil {
		t.Fatalf("Failed to parse all logs response: %v", err)
	}

	if allResult.Level != "all" {
		t.Errorf("Expected level to be 'all': got %s", allResult.Level)
	}

	t.Logf("✓ Found %d total logs (level=all)", allResult.Count)

	t.Log("✅ Level filtering test passed")
}

// TestJobLogsAggregated_Order tests ordering parameter (asc/desc)
func TestJobLogsAggregated_Order(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_Order")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create a simple source and job
	source := map[string]interface{}{
		"id":       "test-source-order-1",
		"name":     "Test Source for Order Testing",
		"type":     "jira",
		"base_url": "https://order-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    3,
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
		"id":          "test-job-def-order-1",
		"name":        "Test Job for Order Testing",
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
					"max_pages":               3,
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

	// Execute job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Wait for job
	var parentJobID string
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
				if jobID, ok := job["id"].(string); ok && jobID != "" {
					if parentID, hasParent := job["parent_id"].(string); !hasParent || parentID == "" {
						parentJobID = jobID
						break
					}
				}
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Failed to find created job")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// Wait for job to complete
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		if status, ok := job["status"].(string); ok {
			if terminalStates[status] {
				break
			}
		}
	}

	time.Sleep(1 * time.Second)

	// Test ascending order (default)
	t.Log("Testing order: asc (oldest-first)...")
	ascResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?order=asc")
	if err != nil {
		t.Fatalf("Failed to get logs with asc order: %v", err)
	}
	h.AssertStatusCode(ascResp, http.StatusOK)

	var ascResult struct {
		Order string                   `json:"order"`
		Logs  []map[string]interface{} `json:"logs"`
		Count int                      `json:"count"`
	}
	if err := h.ParseJSONResponse(ascResp, &ascResult); err != nil {
		t.Fatalf("Failed to parse asc order response: %v", err)
	}

	if ascResult.Order != "asc" {
		t.Errorf("Expected order to be 'asc': got %s", ascResult.Order)
	}

	if ascResult.Count > 1 {
		// Verify timestamps are in ascending order
		for i := 0; i < ascResult.Count-1; i++ {
			curr := ascResult.Logs[i]["timestamp"].(string)
			next := ascResult.Logs[i+1]["timestamp"].(string)
			if curr > next {
				t.Errorf("Logs not in ascending order: %s > %s", curr, next)
				break
			}
		}
		t.Logf("✓ Ascending order verified (%d logs)", ascResult.Count)
	}

	// Test descending order
	t.Log("Testing order: desc (newest-first)...")
	descResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?order=desc")
	if err != nil {
		t.Fatalf("Failed to get logs with desc order: %v", err)
	}
	h.AssertStatusCode(descResp, http.StatusOK)

	var descResult struct {
		Order string                   `json:"order"`
		Logs  []map[string]interface{} `json:"logs"`
		Count int                      `json:"count"`
	}
	if err := h.ParseJSONResponse(descResp, &descResult); err != nil {
		t.Fatalf("Failed to parse desc order response: %v", err)
	}

	if descResult.Order != "desc" {
		t.Errorf("Expected order to be 'desc': got %s", descResult.Order)
	}

	if descResult.Count > 1 {
		// Verify timestamps are in descending order
		for i := 0; i < descResult.Count-1; i++ {
			curr := descResult.Logs[i]["timestamp"].(string)
			next := descResult.Logs[i+1]["timestamp"].(string)
			if curr < next {
				t.Errorf("Logs not in descending order: %s < %s", curr, next)
				break
			}
		}
		t.Logf("✓ Descending order verified (%d logs)", descResult.Count)
	}

	t.Log("✅ Order parameter test passed")
}

// TestJobLogsAggregated_Limit tests the limit parameter
func TestJobLogsAggregated_Limit(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_Limit")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create a simple source and job
	source := map[string]interface{}{
		"id":       "test-source-limit-1",
		"name":     "Test Source for Limit Testing",
		"type":     "confluence",
		"base_url": "https://limit-test.atlassian.net",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    3,
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
		"id":          "test-job-def-limit-1",
		"name":        "Test Job for Limit Testing",
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
					"max_pages":               3,
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

	// Execute job
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	// Wait for job
	var parentJobID string
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
					if parentID, hasParent := job["parent_id"].(string); !hasParent || parentID == "" {
						parentJobID = jobID
						break
					}
				}
			}
		}

		if parentJobID != "" {
			break
		}
	}

	if parentJobID == "" {
		t.Fatal("Failed to find created job")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	// Wait for job to complete
	terminalStates := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	for attempt := 0; attempt < 40; attempt++ {
		time.Sleep(500 * time.Millisecond)

		jobResp, err := h.GET("/api/jobs/" + parentJobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		if status, ok := job["status"].(string); ok {
			if terminalStates[status] {
				break
			}
		}
	}

	time.Sleep(1 * time.Second)

	// Test with limit
	t.Log("Testing limit parameter...")
	limitedResp, err := h.GET("/api/jobs/" + parentJobID + "/logs/aggregated?limit=5")
	if err != nil {
		t.Fatalf("Failed to get logs with limit: %v", err)
	}
	h.AssertStatusCode(limitedResp, http.StatusOK)

	var limitedResult struct {
		Count int                      `json:"count"`
		Logs  []map[string]interface{} `json:"logs"`
	}
	if err := h.ParseJSONResponse(limitedResp, &limitedResult); err != nil {
		t.Fatalf("Failed to parse limited logs response: %v", err)
	}

	if limitedResult.Count > 5 {
		t.Errorf("Expected count to be at most 5, got %d", limitedResult.Count)
	}

	if len(limitedResult.Logs) > 5 {
		t.Errorf("Expected logs length to be at most 5, got %d", len(limitedResult.Logs))
	}

	t.Logf("✓ Limit parameter working correctly (requested: 5, got: %d)", limitedResult.Count)

	t.Log("✅ Limit parameter test passed")
}

// TestJobLogsAggregated_NonExistentJob tests error handling for non-existent job
func TestJobLogsAggregated_NonExistentJob(t *testing.T) {
	env, err := SetupTestEnvironment("TestJobLogsAggregated_NonExistentJob")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Attempt to get aggregated logs for a non-existent job
	aggregatedResp, err := h.GET("/api/jobs/non-existent-job-12345/logs/aggregated")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Should return 404 or 500
	if aggregatedResp.StatusCode != http.StatusNotFound && aggregatedResp.StatusCode != http.StatusInternalServerError {
		bodyBytes, _ := io.ReadAll(aggregatedResp.Body)
		aggregatedResp.Body.Close()
		t.Errorf("Expected 404 or 500 status for non-existent job, got: %d\nBody: %s",
			aggregatedResp.StatusCode, string(bodyBytes))
	}

	t.Log("✓ Correctly handled non-existent job")
}
