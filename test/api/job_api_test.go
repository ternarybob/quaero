package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test"
)

// TestCreateJobWithSource creates a job from an existing source with authentication
func TestCreateJobWithSource(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	baseURL := test.MustGetTestServerURL()
	h := test.NewHTTPTestHelper(t, baseURL)

	// 1. Create test authentication
	authData := map[string]interface{}{
		"baseUrl":   "https://test-job.atlassian.net",
		"userAgent": "Mozilla/5.0 Test",
		"cookies": []map[string]interface{}{
			{
				"name":     "cloud.session.token",
				"value":    "test-job-token",
				"domain":   ".atlassian.net",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			},
		},
		"tokens": map[string]string{
			"cloudId":  "test-cloud-id",
			"atlToken": "test-atl-token",
		},
	}

	authJSON, err := json.Marshal(authData)
	if err != nil {
		t.Fatalf("Failed to marshal auth data: %v", err)
	}

	authResp, err := http.Post(baseURL+"/api/auth", "application/json", bytes.NewBuffer(authJSON))
	if err != nil {
		t.Fatalf("Failed to create auth: %v", err)
	}
	defer authResp.Body.Close()

	// Get auth ID
	authListResp, err := http.Get(baseURL + "/api/auth/list")
	if err != nil {
		t.Fatalf("Failed to list auths: %v", err)
	}
	defer authListResp.Body.Close()

	var auths []map[string]interface{}
	if err := json.NewDecoder(authListResp.Body).Decode(&auths); err != nil {
		t.Fatalf("Failed to decode auth list: %v", err)
	}

	var authID string
	for _, auth := range auths {
		if siteDomain, ok := auth["site_domain"].(string); ok && siteDomain == "test-job.atlassian.net" {
			authID = auth["id"].(string)
			break
		}
	}

	if authID == "" {
		t.Fatal("Could not find created authentication")
	}

	// Cleanup auth at end
	defer func() {
		req, _ := http.NewRequest("DELETE", baseURL+"/api/auth/"+authID, nil)
		if req != nil {
			client := &http.Client{}
			resp, _ := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}()

	// 2. Create test source with authentication
	source := map[string]interface{}{
		"name":     "Test Jira Source for Job",
		"type":     "jira",
		"base_url": "https://test-job.atlassian.net/jira",
		"auth_id":  authID,
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    3,
			"follow_links": true,
			"concurrency":  2,
			"rate_limit":   1,
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

	sourceID, ok := sourceResult["id"].(string)
	if !ok {
		t.Fatal("Could not extract source ID")
	}

	// Cleanup source at end
	defer func() {
		h.DELETE("/api/sources/" + sourceID)
	}()

	// 3. Create job from source
	jobReq := map[string]interface{}{
		"source_id":      sourceID,
		"refresh_source": false,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	h.AssertStatusCode(jobResp, http.StatusCreated)

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	// Verify response contains job details
	jobID, ok := jobResult["job_id"].(string)
	if !ok {
		t.Fatal("Could not extract job ID")
	}

	if sourceIDCheck, ok := jobResult["source_id"].(string); !ok || sourceIDCheck != sourceID {
		t.Errorf("Expected source_id '%s', got: %v", sourceID, jobResult["source_id"])
	}

	if sourceType, ok := jobResult["source_type"].(string); !ok || sourceType != "jira" {
		t.Errorf("Expected source_type 'jira', got: %v", jobResult["source_type"])
	}

	// 4. Fetch job details to verify snapshots
	jobDetailsResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job details: %v", err)
	}

	h.AssertStatusCode(jobDetailsResp, http.StatusOK)

	var jobDetails map[string]interface{}
	if err := h.ParseJSONResponse(jobDetailsResp, &jobDetails); err != nil {
		t.Fatalf("Failed to parse job details: %v", err)
	}

	// Verify snapshots are populated
	if sourceSnapshot, ok := jobDetails["source_config_snapshot"].(string); !ok || sourceSnapshot == "" {
		t.Error("Expected source_config_snapshot to be populated")
	}

	if authSnapshot, ok := jobDetails["auth_snapshot"].(string); !ok || authSnapshot == "" {
		t.Error("Expected auth_snapshot to be populated")
	}

	if refreshSource, ok := jobDetails["refresh_source"].(bool); !ok || refreshSource != false {
		t.Errorf("Expected refresh_source to be false, got: %v", jobDetails["refresh_source"])
	}

	// Cleanup job
	defer func() {
		h.DELETE("/api/jobs/" + jobID)
	}()

	t.Log("✓ Successfully created job with source and auth snapshots")
}

// TestCreateJobWithRefresh creates a job with refresh_source=true
func TestCreateJobWithRefresh(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create source without auth
	source := map[string]interface{}{
		"name":     "Test Source for Refresh Job",
		"type":     "jira",
		"base_url": "https://refresh-test.atlassian.net/jira",
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

	// Create job with refresh_source=true
	jobReq := map[string]interface{}{
		"source_id":      sourceID,
		"refresh_source": true,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	h.AssertStatusCode(jobResp, http.StatusCreated)

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	jobID := jobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + jobID)

	// Verify refresh_source flag
	if refreshSource, ok := jobResult["refresh_source"].(bool); !ok || refreshSource != true {
		t.Errorf("Expected refresh_source to be true, got: %v", jobResult["refresh_source"])
	}

	t.Log("✓ Successfully created job with refresh_source=true")
}

// TestCreateJobValidationFailure tests job creation with invalid source configuration
func TestCreateJobValidationFailure(t *testing.T) {
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create source with invalid configuration (negative concurrency)
	source := map[string]interface{}{
		"name":     "Invalid Source for Job",
		"type":     "jira",
		"base_url": "https://invalid.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": -1, // Invalid
		},
	}

	// This should fail during source creation validation
	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Source creation should fail with 400
	if sourceResp.StatusCode == http.StatusCreated {
		// If it was created, try to create a job and it should fail
		var sourceResult map[string]interface{}
		h.ParseJSONResponse(sourceResp, &sourceResult)
		sourceID := sourceResult["id"].(string)
		defer h.DELETE("/api/sources/" + sourceID)

		jobReq := map[string]interface{}{
			"source_id": sourceID,
		}

		jobResp, _ := h.POST("/api/jobs/create", jobReq)
		h.AssertStatusCode(jobResp, http.StatusBadRequest)
	} else {
		// Source creation correctly failed
		h.AssertStatusCode(sourceResp, http.StatusBadRequest)
	}

	t.Log("✓ Validation correctly prevented invalid job creation")
}

// TestCreateJobSourceNotFound tests job creation with non-existent source
func TestCreateJobSourceNotFound(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Attempt to create job with non-existent source ID
	jobReq := map[string]interface{}{
		"source_id": "non-existent-source-id-12345",
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job request: %v", err)
	}

	h.AssertStatusCode(jobResp, http.StatusNotFound)

	t.Log("✓ Correctly returned 404 for non-existent source")
}

// TestCreateJobWithoutAuth creates a job from a source without authentication
func TestCreateJobWithoutAuth(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create source without auth_id
	source := map[string]interface{}{
		"name":     "Source without Auth for Job",
		"type":     "jira",
		"base_url": "https://noauth.atlassian.net/jira",
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

	// Create job from source
	jobReq := map[string]interface{}{
		"source_id": sourceID,
	}

	jobResp, err := h.POST("/api/jobs/create", jobReq)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	h.AssertStatusCode(jobResp, http.StatusCreated)

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job response: %v", err)
	}

	jobID := jobResult["job_id"].(string)
	defer h.DELETE("/api/jobs/" + jobID)

	// Fetch job details
	jobDetailsResp, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job details: %v", err)
	}

	var jobDetails map[string]interface{}
	if err := h.ParseJSONResponse(jobDetailsResp, &jobDetails); err != nil {
		t.Fatalf("Failed to parse job details: %v", err)
	}

	// Verify auth_snapshot is empty or null
	if authSnapshot, ok := jobDetails["auth_snapshot"].(string); ok && authSnapshot != "" {
		t.Errorf("Expected empty auth_snapshot, got: %s", authSnapshot)
	}

	t.Log("✓ Successfully created job without authentication")
}

// TestGetJobQueue tests the job queue endpoint
func TestGetJobQueue(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// Create a source for testing
	source := map[string]interface{}{
		"name":     "Source for Queue Test",
		"type":     "jira",
		"base_url": "https://queue.atlassian.net/jira",
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

	// Create multiple jobs
	jobIDs := []string{}
	for i := 0; i < 3; i++ {
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

	// Get job queue
	queueResp, err := h.GET("/api/jobs/queue")
	if err != nil {
		t.Fatalf("Failed to get job queue: %v", err)
	}

	h.AssertStatusCode(queueResp, http.StatusOK)

	var queueResult map[string]interface{}
	if err := h.ParseJSONResponse(queueResp, &queueResult); err != nil {
		t.Fatalf("Failed to parse queue response: %v", err)
	}

	// Verify response structure
	if _, ok := queueResult["pending"]; !ok {
		t.Error("Expected 'pending' field in queue response")
	}

	if _, ok := queueResult["running"]; !ok {
		t.Error("Expected 'running' field in queue response")
	}

	if totalCount, ok := queueResult["total"].(float64); !ok || totalCount < 0 {
		t.Errorf("Expected valid total count, got: %v", queueResult["total"])
	}

	t.Log("✓ Successfully retrieved job queue")
}

// TestJobSnapshotImmutability verifies that job snapshots remain unchanged after source modification
func TestJobSnapshotImmutability(t *testing.T) {
	t.Skip("SKIP: Direct job creation endpoint is deprecated - use job definitions instead")
	h := test.NewHTTPTestHelper(t, test.MustGetTestServerURL())

	// 1. Create source
	source := map[string]interface{}{
		"name":     "Original Source Name",
		"type":     "jira",
		"base_url": "https://immutable.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": 2,
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
	originalName := sourceResult["name"].(string)
	defer h.DELETE("/api/sources/" + sourceID)

	// 2. Create job from source
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

	// Get job details before source modification
	jobDetailsResp1, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job details: %v", err)
	}

	var jobDetails1 map[string]interface{}
	if err := h.ParseJSONResponse(jobDetailsResp1, &jobDetails1); err != nil {
		t.Fatalf("Failed to parse job details: %v", err)
	}

	originalSnapshot := jobDetails1["source_config_snapshot"].(string)

	// 3. Modify source configuration
	updatedSource := map[string]interface{}{
		"name":     "Modified Source Name",
		"type":     "jira",
		"base_url": "https://immutable.atlassian.net/jira",
		"enabled":  false,
		"crawl_config": map[string]interface{}{
			"concurrency": 5, // Changed
		},
	}

	updateResp, err := h.PUT("/api/sources/"+sourceID, updatedSource)
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}

	h.AssertStatusCode(updateResp, http.StatusOK)

	// 4. Fetch job details again
	jobDetailsResp2, err := h.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Fatalf("Failed to get job details after update: %v", err)
	}

	var jobDetails2 map[string]interface{}
	if err := h.ParseJSONResponse(jobDetailsResp2, &jobDetails2); err != nil {
		t.Fatalf("Failed to parse job details after update: %v", err)
	}

	currentSnapshot := jobDetails2["source_config_snapshot"].(string)

	// 5. Verify snapshot is unchanged
	if currentSnapshot != originalSnapshot {
		t.Error("Job snapshot was modified after source update (should be immutable)")
	}

	// Verify the snapshot still contains original configuration
	var snapshotData map[string]interface{}
	if err := json.Unmarshal([]byte(currentSnapshot), &snapshotData); err != nil {
		t.Fatalf("Failed to parse snapshot: %v", err)
	}

	if snapshotName, ok := snapshotData["name"].(string); !ok || snapshotName != originalName {
		t.Errorf("Expected snapshot to contain original name '%s', got: %s", originalName, snapshotName)
	}

	t.Log("✓ Job snapshot remained immutable after source modification")
}
