package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"
)

// TestPlacesJobDocumentCount verifies that the job's document count is updated
func TestPlacesJobDocumentCount(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestPlacesJobDocumentCount")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create and execute places job
	jobDef := map[string]interface{}{
		"id":          "test-places-count-job",
		"name":        "Test Places Count",
		"type":        "places",
		"job_type":    "user",
		"description": "Test document count tracking",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":   "search_test",
				"action": "places_search",
				"config": map[string]interface{}{
					"search_query": "cafes near Melbourne",
					"search_type":  "nearby_search",
					"max_results":  3,
					"location": map[string]interface{}{
						"latitude":  -37.8136,
						"longitude": 144.9631,
						"radius":    500,
					},
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

	// 2. Wait for job completion
	var parentJobID string
	var finalJob map[string]interface{}
	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

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
			jobType, _ := job["job_type"].(string)
			jobName, _ := job["name"].(string)

			if jobType == "places" && jobName == "Test Places Count" {
				parentJobID = job["id"].(string)
				finalJob = job

				if status, ok := job["status"].(string); ok && status == "completed" {
					goto done
				}
			}
		}
	}

done:
	if parentJobID == "" {
		t.Fatal("Job not found")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)

	if finalJob == nil {
		t.Fatal("Failed to get final job status")
	}

	// 3. Verify result_count matches expected documents (max_results or actual places found)
	// We requested max_results=3, so we expect at least 3 documents (one per place)
	expectedMinDocs := 3
	resultCount := 0
	if rc, ok := finalJob["result_count"].(float64); ok {
		resultCount = int(rc)
	}

	if resultCount < expectedMinDocs {
		t.Errorf("Job result_count should be at least %d (one document per place), got: %d", expectedMinDocs, resultCount)
	} else {
		t.Logf("✓ Job result_count: %d (expected at least %d)", resultCount, expectedMinDocs)
	}

	// 4. Verify document_count in job metadata
	// This is set by the event-driven JobMonitor when EventDocumentSaved is published
	metadataStr, ok := finalJob["metadata_json"].(string)
	if !ok || metadataStr == "" {
		t.Error("Job should have metadata_json")
	} else {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			t.Errorf("Failed to parse job metadata JSON: %v", err)
		} else {
			documentCount := 0
			if dc, ok := metadata["document_count"].(float64); ok {
				documentCount = int(dc)
			}

			if documentCount < expectedMinDocs {
				t.Errorf("Job metadata document_count should be at least %d (one per place), got: %d. This indicates EventDocumentSaved was not published for all documents", expectedMinDocs, documentCount)
			} else {
				t.Logf("✓ Job metadata document_count: %d (event-driven tracking working, expected at least %d)", documentCount, expectedMinDocs)
			}
		}
	}

	t.Log("✓ Document count tracking verified (both result_count and metadata document_count)")
}

// TestPlacesJobDocumentTags verifies that tags from job definition are saved to document
func TestPlacesJobDocumentTags(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestPlacesJobDocumentTags")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// 1. Create places job definition WITH TAGS
	expectedTags := []string{"test-tag", "places", "sydney"}
	jobDef := map[string]interface{}{
		"id":          "test-places-tags-job",
		"name":        "Test Places Tags",
		"type":        "places",
		"job_type":    "user",
		"description": "Test that tags from job definition are saved to document",
		"enabled":     true,
		"tags":        expectedTags, // IMPORTANT: Tags in job definition
		"steps": []map[string]interface{}{
			{
				"name":   "search_with_tags",
				"action": "places_search",
				"config": map[string]interface{}{
					"search_query": "parks near Sydney Opera House",
					"search_type":  "nearby_search",
					"max_results":  3,
					"location": map[string]interface{}{
						"latitude":  -33.8568,
						"longitude": 151.2153,
						"radius":    500,
					},
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
	t.Logf("✓ Created places job definition with tags: %v", expectedTags)

	// 2. Execute job definition
	execResp, err := h.POST("/api/job-definitions/"+jobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute job definition: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)
	t.Log("✓ Job execution triggered")

	// 3. Wait for job to complete
	var parentJobID string
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

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
			jobType, _ := job["job_type"].(string)
			jobName, _ := job["name"].(string)

			if jobType == "places" && jobName == "Test Places Tags" {
				parentJobID = job["id"].(string)

				if status, ok := job["status"].(string); ok && status == "completed" {
					goto done
				} else if status == "failed" {
					errorMsg := "unknown"
					if errStr, ok := job["error"].(string); ok {
						errorMsg = errStr
					}
					t.Fatalf("Job failed: %s", errorMsg)
				}
			}
		}
	}

done:
	if parentJobID == "" {
		t.Fatal("Job not found or did not complete in time")
	}

	defer h.DELETE("/api/jobs/" + parentJobID)
	t.Logf("✓ Places job completed: %s", parentJobID)

	// 4. Fetch documents created by places job
	docsResp, err := h.GET("/api/documents")
	if err != nil {
		t.Fatalf("Failed to fetch documents: %v", err)
	}

	var docsResult struct {
		Documents []map[string]interface{} `json:"documents"`
		Total     int                      `json:"total"`
	}
	if err := h.ParseJSONResponse(docsResp, &docsResult); err != nil {
		t.Fatalf("Failed to parse documents response: %v", err)
	}

	// Find ALL places documents for this job (should be one per place)
	// NOTE: source_id is now place_id, so we check job_id in metadata
	var placesDocs []map[string]interface{}
	for _, doc := range docsResult.Documents {
		sourceType, _ := doc["source_type"].(string)
		if sourceType != "places" {
			continue
		}

		// Check job_id in metadata to find documents from this job
		metadataStr, ok := doc["metadata"].(string)
		if !ok || metadataStr == "" {
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			continue
		}

		jobIDInMetadata, _ := metadata["job_id"].(string)
		if jobIDInMetadata == parentJobID {
			placesDocs = append(placesDocs, doc)
		}
	}

	if len(placesDocs) == 0 {
		t.Fatalf("No documents with source_type='places' and metadata.job_id='%s' found", parentJobID)
	}

	t.Logf("✓ Found %d place documents for job %s", len(placesDocs), parentJobID)

	// 5. VERIFY TAGS ARE PRESENT ON ALL DOCUMENTS
	for i, placesDoc := range placesDocs {
		docID := placesDoc["id"].(string)

		tagsStr, ok := placesDoc["tags"].(string)
		if !ok || tagsStr == "" {
			t.Errorf("Document %d (%s) should have tags field", i+1, docID)
			continue
		}

		var actualTags []string
		if err := json.Unmarshal([]byte(tagsStr), &actualTags); err != nil {
			t.Errorf("Document %d (%s): Failed to parse tags JSON: %v", i+1, docID, err)
			continue
		}

		// Verify tags match job definition tags
		if len(actualTags) != len(expectedTags) {
			t.Errorf("Document %d (%s): Expected %d tags, got %d. Expected: %v, Got: %v", i+1, docID, len(expectedTags), len(actualTags), expectedTags, actualTags)
			continue
		}

		tagMap := make(map[string]bool)
		for _, tag := range actualTags {
			tagMap[tag] = true
		}

		for _, expectedTag := range expectedTags {
			if !tagMap[expectedTag] {
				t.Errorf("Document %d (%s): Expected tag '%s' not found in document tags: %v", i+1, docID, expectedTag, actualTags)
			}
		}

		t.Logf("✓ Document %d (%s): Tags verified - %v", i+1, docID, actualTags)
	}

	t.Logf("✓ All %d place documents correctly inherit tags from job definition", len(placesDocs))
}