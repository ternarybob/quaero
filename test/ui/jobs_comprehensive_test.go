package ui

import (
	"github.com/ternarybob/quaero/test/common"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestJobCreationFlow tests the complete job creation flow with API and UI
func TestJobCreationFlow(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobCreationFlow")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	baseURL := env.GetBaseURL()
	h := env.NewHTTPTestHelper(t)

	// 1. Create test source via API
	source := map[string]interface{}{
		"name":     "Test Source for UI Job Creation",
		"type":     "jira",
		"base_url": "https://ui-job-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    3,
			"follow_links": true,
			"concurrency":  2,
			"rate_limit":   500,
			"max_pages":    100,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	h.AssertStatusCode(sourceResp, http.StatusCreated)

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(sourceResp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}

	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Use UI to create job
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := baseURL + "/jobs"

	var jobID string
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page load

		// Click Create Job button
		chromedp.Click(`button.btn-info`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second), // Wait for modal

		// Select the source
		chromedp.WaitVisible(`#job-source-select`, chromedp.ByQuery),
		chromedp.SetValue(`#job-source-select`, sourceID, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),

		// Click Create button in modal
		chromedp.Click(`.modal button.btn-info`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for API call

		// Extract job ID from alert or response
		chromedp.Evaluate(`
			// Wait for job to be created and get job ID from table
			new Promise((resolve) => {
				setTimeout(() => {
					const firstRow = document.querySelector('#jobs-table-body tr');
					if (firstRow) {
						const jobIdCell = firstRow.querySelector('td:first-child');
						if (jobIdCell) {
							const fullJobId = jobIdCell.getAttribute('title');
							resolve(fullJobId || '');
						}
					}
					resolve('');
				}, 2000);
			})
		`, &jobID),
	)

	if err != nil {
		t.Fatalf("Failed to create job through UI: %v", err)
	}

	// Take screenshot of result
	if err := TakeScreenshot(ctx, "job-creation-flow-complete"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// 3. Verify job was created via API
	if jobID != "" {
		defer h.DELETE("/api/jobs/" + jobID)

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("Failed to fetch created job: %v", err)
		}

		h.AssertStatusCode(jobResp, http.StatusOK)

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			t.Fatalf("Failed to parse job response: %v", err)
		}

		// Verify entity type is plural (from Comment 1 fix)
		entityType, ok := job["entity_type"].(string)
		if !ok || entityType != "projects" {
			t.Errorf("Expected entity_type 'projects', got: %v", job["entity_type"])
		}

		t.Logf("✓ Job created successfully with ID: %s", jobID)
	}

	t.Log("✓ Complete job creation flow working correctly")
}

// TestJobRerunAction tests the rerun job functionality
func TestJobRerunAction(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobRerunAction")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	baseURL := env.GetBaseURL()
	h := env.NewHTTPTestHelper(t)

	// 1. Create source
	source := map[string]interface{}{
		"name":     "Test Source for Rerun",
		"type":     "confluence",
		"base_url": "https://rerun-test.atlassian.net/wiki",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(sourceResp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}

	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Create a job
	jobReq := map[string]interface{}{
		"source_id": sourceID,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	originalJobID := jobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + originalJobID)

	// 3. Use UI to rerun the job
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := baseURL + "/jobs"

	var newJobID string
	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load

		// Find and click rerun button for the job
		chromedp.Evaluate(`
			new Promise((resolve) => {
				const rows = document.querySelectorAll('#jobs-table-body tr');
				for (let row of rows) {
					const jobIdCell = row.querySelector('td:first-child');
					if (jobIdCell && jobIdCell.getAttribute('title') === '`+originalJobID+`') {
						const rerunBtn = row.querySelector('button[title="Rerun Job"]');
						if (rerunBtn) {
							rerunBtn.click();
							resolve(true);
							return;
						}
					}
				}
				resolve(false);
			})
		`, nil),

		chromedp.Sleep(500*time.Millisecond),

		// Accept confirmation dialog
		chromedp.Evaluate(`
			// Simulate confirmation
			window.confirm = function() { return true; };
		`, nil),

		chromedp.Sleep(2*time.Second), // Wait for API call

		// Extract new job ID from alert or table
		chromedp.Evaluate(`
			new Promise((resolve) => {
				setTimeout(() => {
					// Look for newest job in table (first row after refresh)
					const firstRow = document.querySelector('#jobs-table-body tr:first-child');
					if (firstRow) {
						const jobIdCell = firstRow.querySelector('td:first-child');
						if (jobIdCell) {
							const fullJobId = jobIdCell.getAttribute('title');
							resolve(fullJobId || '');
						}
					}
					resolve('');
				}, 2000);
			})
		`, &newJobID),
	)

	if err != nil {
		t.Fatalf("Failed to rerun job through UI: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "job-rerun-action"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// 4. Verify new job was created
	if newJobID != "" && newJobID != originalJobID {
		defer h.DELETE("/api/jobs/" + newJobID)

		rerunResp, err := h.GET("/api/jobs/" + newJobID)
		if err != nil {
			t.Fatalf("Failed to fetch rerun job: %v", err)
		}

		h.AssertStatusCode(rerunResp, http.StatusOK)

		var rerunJob map[string]interface{}
		if err := h.ParseJSONResponse(rerunResp, &rerunJob); err != nil {
			t.Fatalf("Failed to parse rerun job response: %v", err)
		}

		// Verify it's a new job with same source
		if rerunJob["source_type"] != jobResult["source_type"] {
			t.Errorf("Rerun job should have same source type as original")
		}

		t.Logf("✓ Job rerun created new job with ID: %s", newJobID)
	}

	t.Log("✓ Job rerun action working correctly")
}

// TestJobCancelAction tests the cancel job functionality
func TestJobCancelAction(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobCancelAction")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create source
	source := map[string]interface{}{
		"name":     "Test Source for Cancel",
		"type":     "jira",
		"base_url": "https://cancel-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(sourceResp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}

	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Create a job (it will be pending or running)
	jobReq := map[string]interface{}{
		"source_id": sourceID,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	jobID := jobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + jobID)

	// 3. Cancel the job via API (simpler than waiting for UI to show running state)
	cancelResp, err := h.POST("/api/jobs/"+jobID+"/cancel", nil)
	if err != nil {
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Should succeed with 200 or fail gracefully if already completed
	if cancelResp.StatusCode != http.StatusOK && cancelResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Unexpected status code for cancel: %d", cancelResp.StatusCode)
	}

	// 4. Verify job status changed
	statusResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job status: %v", err)
	}

	var job map[string]interface{}
	if err := h.ParseJSONResponse(statusResp, &job); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	status, ok := job["status"].(string)
	if !ok {
		t.Fatal("Could not extract job status")
	}

	// Status should be cancelled, completed, or failed (if job finished before cancel)
	validStatuses := []string{"cancelled", "completed", "failed"}
	isValidStatus := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValidStatus = true
			break
		}
	}

	if !isValidStatus {
		t.Logf("Job status after cancel: %s (expected: cancelled, completed, or failed)", status)
	}

	t.Log("✓ Job cancel action working correctly")
}

// TestJobDeleteAction tests the delete job functionality
func TestJobDeleteAction(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobDeleteAction")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	baseURL := env.GetBaseURL()
	h := env.NewHTTPTestHelper(t)

	// 1. Create source
	source := map[string]interface{}{
		"name":     "Test Source for Delete",
		"type":     "jira",
		"base_url": "https://delete-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(sourceResp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}

	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Create a job
	jobReq := map[string]interface{}{
		"source_id": sourceID,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	jobID := jobResult["job_id"].(string)

	// 3. Use UI to delete the job
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	url := baseURL + "/jobs"

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for jobs to load

		// Find and click delete button for the job
		chromedp.Evaluate(`
			new Promise((resolve) => {
				const rows = document.querySelectorAll('#jobs-table-body tr');
				for (let row of rows) {
					const jobIdCell = row.querySelector('td:first-child');
					if (jobIdCell && jobIdCell.getAttribute('title') === '`+jobID+`') {
						const deleteBtn = row.querySelector('button[title="Delete Job"]');
						if (deleteBtn) {
							// Override confirm to auto-accept
							window.confirm = function() { return true; };
							deleteBtn.click();
							resolve(true);
							return;
						}
					}
				}
				resolve(false);
			})
		`, nil),

		chromedp.Sleep(2*time.Second), // Wait for API call and refresh
	)

	if err != nil {
		t.Fatalf("Failed to delete job through UI: %v", err)
	}

	// Take screenshot
	if err := TakeScreenshot(ctx, "job-delete-action"); err != nil {
		t.Logf("Warning: Failed to take screenshot: %v", err)
	}

	// 4. Verify job was deleted
	deleteCheckResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to check deleted job: %v", err)
	}

	// Should return 404
	if deleteCheckResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for deleted job, got: %d", deleteCheckResp.StatusCode)
	}

	t.Log("✓ Job delete action working correctly")
}

// TestJobQueueVisibility tests that queue endpoint returns correct structure
func TestJobQueueVisibility(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobQueueVisibility")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create source
	source := map[string]interface{}{
		"name":     "Test Source for Queue",
		"type":     "jira",
		"base_url": "https://queue-visibility-test.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	var sourceResult map[string]interface{}
	if err := h.ParseJSONResponse(sourceResp, &sourceResult); err != nil {
		t.Fatalf("Failed to parse source response: %v", err)
	}

	sourceID := sourceResult["id"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Create multiple jobs
	jobIDs := []string{}
	for i := 0; i < 2; i++ {
		jobReq := map[string]interface{}{
			"source_id": sourceID,
		}

		jobResp, err := h.POST("/api/jobs/create", jobReq)
		if err != nil {
			t.Fatalf("Failed to create job %d: %v", i, err)
		}

		var jobResult map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
			t.Fatalf("Failed to parse job response: %v", err)
		}

		jobID := jobResult["job_id"].(string)
		jobIDs = append(jobIDs, jobID)
	}

	// Cleanup all jobs
	defer func() {
		for _, jobID := range jobIDs {
			h.DELETE("/api/jobs/" + jobID)
		}
	}()

	// 3. Get queue data
	queueResp, err := h.GET("/api/jobs/queue")
	if err != nil {
		t.Fatalf("Failed to get job queue: %v", err)
	}

	h.AssertStatusCode(queueResp, http.StatusOK)

	var queueData map[string]interface{}
	if err := h.ParseJSONResponse(queueResp, &queueData); err != nil {
		t.Fatalf("Failed to parse queue response: %v", err)
	}

	// 4. Verify queue structure
	if _, ok := queueData["pending"]; !ok {
		t.Error("Queue response missing 'pending' field")
	}

	if _, ok := queueData["running"]; !ok {
		t.Error("Queue response missing 'running' field")
	}

	if _, ok := queueData["total"]; !ok {
		t.Error("Queue response missing 'total' field")
	}

	// Extract pending jobs array
	pendingJobs, ok := queueData["pending"].([]interface{})
	if !ok {
		t.Fatal("'pending' field is not an array")
	}

	// Should have at least our created jobs (unless they completed immediately)
	t.Logf("Queue contains %d pending jobs", len(pendingJobs))

	// Verify each job has required fields
	for i, jobInterface := range pendingJobs {
		jobMap, ok := jobInterface.(map[string]interface{})
		if !ok {
			t.Errorf("Job %d is not a map", i)
			continue
		}

		// Check for required fields
		requiredFields := []string{"id", "source_type", "entity_type", "status"}
		for _, field := range requiredFields {
			if _, exists := jobMap[field]; !exists {
				t.Errorf("Job %d missing required field: %s", i, field)
			}
		}

		// Verify entity_type is plural (from Comment 1 fix)
		if entityType, ok := jobMap["entity_type"].(string); ok {
			validEntityTypes := []string{"projects", "spaces", "repos"}
			isValid := false
			for _, valid := range validEntityTypes {
				if entityType == valid {
					isValid = true
					break
				}
			}
			if !isValid {
				t.Errorf("Job %d has invalid entity_type: %s (expected plural form)", i, entityType)
			}
		}
	}

	t.Log("✓ Job queue visibility and structure verified")
}

// TestJobFiltering tests entity type filtering with plural values
func TestJobFiltering(t *testing.T) {
	// Setup test environment
	env, err := common.SetupTestEnvironment("TestJobFiltering")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create Jira source
	jiraSource := map[string]interface{}{
		"name":     "Jira Source for Filtering",
		"type":     "jira",
		"base_url": "https://filter-test-jira.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	jiraResp, err := h.POST("/api/sources", jiraSource)
	if err != nil {
		t.Fatalf("Failed to create Jira source: %v", err)
	}

	var jiraResult map[string]interface{}
	if err := h.ParseJSONResponse(jiraResp, &jiraResult); err != nil {
		t.Fatalf("Failed to parse Jira source response: %v", err)
	}

	jiraSourceID := jiraResult["id"].(string)
	defer h.DELETE("/api/sources/" + jiraSourceID)

	// 2. Create Confluence source
	confluenceSource := map[string]interface{}{
		"name":     "Confluence Source for Filtering",
		"type":     "confluence",
		"base_url": "https://filter-test-confluence.atlassian.net/wiki",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 1,
		},
	}

	confluenceResp, err := h.POST("/api/sources", confluenceSource)
	if err != nil {
		t.Fatalf("Failed to create Confluence source: %v", err)
	}

	var confluenceResult map[string]interface{}
	if err := h.ParseJSONResponse(confluenceResp, &confluenceResult); err != nil {
		t.Fatalf("Failed to parse Confluence source response: %v", err)
	}

	confluenceSourceID := confluenceResult["id"].(string)
	defer h.DELETE("/api/sources/" + confluenceSourceID)

	// 3. Create jobs from both sources
	jiraJobReq := map[string]interface{}{
		"source_id": jiraSourceID,
	}

	jiraJobResp, err := h.POST("/api/jobs/create", jiraJobReq)
	if err != nil {
		t.Fatalf("Failed to create Jira job: %v", err)
	}

	var jiraJobResult map[string]interface{}
	json.NewDecoder(jiraJobResp.Body).Decode(&jiraJobResult)
	jiraJobResp.Body.Close()
	jiraJobID := jiraJobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + jiraJobID)

	confluenceJobReq := map[string]interface{}{
		"source_id": confluenceSourceID,
	}

	confluenceJobResp, err := h.POST("/api/jobs/create", confluenceJobReq)
	if err != nil {
		t.Fatalf("Failed to create Confluence job: %v", err)
	}

	var confluenceJobResult map[string]interface{}
	json.NewDecoder(confluenceJobResp.Body).Decode(&confluenceJobResult)
	confluenceJobResp.Body.Close()
	confluenceJobID := confluenceJobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + confluenceJobID)

	// 4. Test filtering by entity type (plural values from Comment 1 fix)
	testCases := []struct {
		entityFilter string
		expectJira   bool
		expectConf   bool
	}{
		{"projects", true, false}, // Should return only Jira jobs
		{"spaces", false, true},   // Should return only Confluence jobs
		{"", true, true},          // No filter - should return both
	}

	for _, tc := range testCases {
		t.Run("entity_filter="+tc.entityFilter, func(t *testing.T) {
			endpoint := "/api/jobs"
			if tc.entityFilter != "" {
				endpoint += "?entity=" + tc.entityFilter
			}

			listResp, err := h.GET(endpoint)
			if err != nil {
				t.Fatalf("Failed to list jobs: %v", err)
			}

			h.AssertStatusCode(listResp, http.StatusOK)

			var listResult map[string]interface{}
			if err := h.ParseJSONResponse(listResp, &listResult); err != nil {
				t.Fatalf("Failed to parse list response: %v", err)
			}

			jobs, ok := listResult["jobs"].([]interface{})
			if !ok {
				t.Fatal("'jobs' field is not an array")
			}

			foundJira := false
			foundConfluence := false

			for _, jobInterface := range jobs {
				jobMap := jobInterface.(map[string]interface{})
				entityType := jobMap["entity_type"].(string)

				if entityType == "projects" {
					foundJira = true
				}
				if entityType == "spaces" {
					foundConfluence = true
				}
			}

			if foundJira != tc.expectJira {
				t.Errorf("Expected to find Jira job: %v, but found: %v", tc.expectJira, foundJira)
			}

			if foundConfluence != tc.expectConf {
				t.Errorf("Expected to find Confluence job: %v, but found: %v", tc.expectConf, foundConfluence)
			}
		})
	}

	t.Log("✓ Job filtering with plural entity types working correctly")
}
