package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"
)

// TestJobIntegration_PlacesAndKeywordExtraction tests the complete flow:
// 1. Run "Nearby Restaurants" job to create documents
// 2. Verify document count > 0
// 3. Run "Keyword Extraction" job to update those documents
// 4. Verify document count > 0 and equals initial count
func TestJobIntegration_PlacesAndKeywordExtraction(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobIntegration_PlacesAndKeywordExtraction")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// ============================================================
	// PHASE 1: Run "Nearby Restaurants" job to create documents
	// ============================================================

	t.Log("=== PHASE 1: Creating documents via Nearby Restaurants job ===")

	// Create Places job definition if it doesn't exist
	placesJobDef := map[string]interface{}{
		"id":          "places-nearby-restaurants",
		"name":        "Nearby Restaurants (Wheelers Hill)",
		"type":        "places",
		"job_type":    "user",
		"description": "Search for restaurants near Wheelers Hill using Google Places Nearby Search API",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":     "search_nearby_restaurants",
				"action":   "places_search",
				"on_error": "fail",
				"config": map[string]interface{}{
					"search_query": "restaurants near Wheelers Hill",
					"search_type":  "nearby_search",
					"max_results":  20,
					"list_name":    "Wheelers Hill Restaurants",
					"location": map[string]interface{}{
						"latitude":  -37.9167,
						"longitude": 145.1833,
						"radius":    2000,
					},
					"filters": map[string]interface{}{
						"type":       "restaurant",
						"min_rating": 3.5,
					},
				},
			},
		},
	}

	// Try to create job definition (ignore error if it already exists)
	createResp, err := h.POST("/api/job-definitions", placesJobDef)
	if err == nil && (createResp.StatusCode == http.StatusCreated || createResp.StatusCode == http.StatusConflict) {
		t.Log("✓ Places job definition created/exists")
	}

	placesJobDefID := "places-nearby-restaurants"
	t.Logf("✓ Using Places job definition: %s", placesJobDefID)

	// Execute the Places job
	execResp, err := h.POST("/api/job-definitions/"+placesJobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute Places job: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	t.Log("✓ Places job execution started")

	// Poll for parent job creation (job gets created asynchronously)
	placesJobID, err := pollForParentJobCreation(t, h, "places", 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to find Places parent job: %v", err)
	}

	t.Logf("✓ Places parent job found: %s", placesJobID)

	// Poll for Places job completion
	placesDocCount, err := pollForJobCompletion(t, h, placesJobID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Places job failed: %v", err)
	}

	t.Logf("✓ Places job completed with %d documents", placesDocCount)

	// Verify document count > 0
	if placesDocCount == 0 {
		t.Fatal("FAIL: Places job created 0 documents - expected > 0")
	}

	t.Logf("✅ PHASE 1 PASS: Places job created %d documents", placesDocCount)

	// Get actual document IDs created by Places job
	docsResp, err := h.GET("/api/documents?source_type=places")
	if err != nil {
		t.Fatalf("Failed to get documents: %v", err)
	}

	var docsResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	h.ParseJSONResponse(docsResp, &docsResult)

	t.Logf("✓ Found %d places documents in database", len(docsResult.Documents))

	// Store document IDs for later verification
	documentIDs := make([]string, 0, len(docsResult.Documents))
	for _, doc := range docsResult.Documents {
		if docID, ok := doc["id"].(string); ok {
			documentIDs = append(documentIDs, docID)
		}
	}

	t.Logf("✓ Stored %d document IDs for verification", len(documentIDs))

	// ============================================================
	// PHASE 2: Run "Keyword Extraction" job to update documents
	// ============================================================

	t.Log("=== PHASE 2: Updating documents via Keyword Extraction job ===")

	// Create Keyword Extraction job definition if it doesn't exist
	keywordJobDef := map[string]interface{}{
		"id":          "keyword-extractor-agent",
		"name":        "Keyword Extraction Demo",
		"type":        "custom",
		"job_type":    "user",
		"description": "Extracts keywords from crawled documents using Google Gemini",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":     "Extract Keywords from Documents",
				"action":   "agent",
				"on_error": "fail",
				"config": map[string]interface{}{
					"agent_type": "keyword_extractor",
					"document_filter": map[string]interface{}{
						"limit": 100,
					},
				},
			},
		},
	}

	// Try to create job definition (ignore error if it already exists)
	createResp2, err := h.POST("/api/job-definitions", keywordJobDef)
	if err == nil && (createResp2.StatusCode == http.StatusCreated || createResp2.StatusCode == http.StatusConflict) {
		t.Log("✓ Keyword Extraction job definition created/exists")
	}

	keywordJobDefID := "keyword-extractor-agent"
	t.Logf("✓ Using Keyword Extraction job definition: %s", keywordJobDefID)

	// Execute the Keyword Extraction job
	execResp2, err := h.POST("/api/job-definitions/"+keywordJobDefID+"/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute Keyword Extraction job: %v", err)
	}
	h.AssertStatusCode(execResp2, http.StatusAccepted)

	t.Log("✓ Keyword Extraction job execution started")

	// Poll for parent job creation (job gets created asynchronously)
	keywordJobID, err := pollForParentJobCreation(t, h, "agent", 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to find Keyword Extraction parent job: %v", err)
	}

	t.Logf("✓ Keyword Extraction parent job found: %s", keywordJobID)

	// Poll for Keyword Extraction job completion
	keywordDocCount, err := pollForJobCompletion(t, h, keywordJobID, 10*time.Minute)
	if err != nil {
		t.Fatalf("Keyword Extraction job failed: %v", err)
	}

	t.Logf("✓ Keyword Extraction job completed with %d documents", keywordDocCount)

	// ============================================================
	// PHASE 3: Verify document counts
	// ============================================================

	t.Log("=== PHASE 3: Verifying document counts ===")

	// Test 1: Keyword job document count > 0
	if keywordDocCount == 0 {
		t.Error("FAIL: Keyword Extraction job processed 0 documents - expected > 0")
		t.Logf("This indicates EventDocumentUpdated is not being published or counted")
	} else {
		t.Logf("✅ TEST 1 PASS: Keyword job processed %d documents (> 0)", keywordDocCount)
	}

	// Test 2: Keyword job document count matches Places job count
	if keywordDocCount != placesDocCount {
		t.Errorf("FAIL: Document count mismatch - Places: %d, Keyword: %d", placesDocCount, keywordDocCount)
		t.Logf("Expected Keyword Extraction to process all %d documents created by Places job", placesDocCount)
	} else {
		t.Logf("✅ TEST 2 PASS: Document counts match (%d documents)", keywordDocCount)
	}

	// ============================================================
	// PHASE 4: Verify document metadata updates
	// ============================================================

	t.Log("=== PHASE 4: Verifying document metadata updates ===")

	updatedDocsCount := 0
	for i, docID := range documentIDs {
		docResp, err := h.GET("/api/documents/" + docID)
		if err != nil {
			t.Logf("Warning: Failed to fetch document %s: %v", docID, err)
			continue
		}

		var doc map[string]interface{}
		if err := h.ParseJSONResponse(docResp, &doc); err != nil {
			t.Logf("Warning: Failed to parse document %s: %v", docID, err)
			continue
		}

		// Check for keyword_extractor metadata
		metadataStr, ok := doc["metadata"].(string)
		if !ok || metadataStr == "" {
			t.Logf("Document %d (%s): No metadata", i+1, docID[:8])
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			t.Logf("Document %d (%s): Invalid metadata JSON", i+1, docID[:8])
			continue
		}

		if keywordData, ok := metadata["keyword_extractor"]; ok && keywordData != nil {
			updatedDocsCount++
			t.Logf("Document %d (%s): ✓ Has keyword_extractor metadata", i+1, docID[:8])
		} else {
			t.Logf("Document %d (%s): ✗ Missing keyword_extractor metadata", i+1, docID[:8])
		}
	}

	t.Logf("✓ Documents with keyword metadata: %d / %d", updatedDocsCount, len(documentIDs))

	// Test 3: All documents should have keyword metadata
	if updatedDocsCount == 0 {
		t.Error("FAIL: No documents have keyword_extractor metadata")
		t.Logf("This indicates agent jobs are not updating document metadata")
	} else if updatedDocsCount < len(documentIDs) {
		t.Logf("⚠️  WARNING: Only %d/%d documents have keyword metadata", updatedDocsCount, len(documentIDs))
	} else {
		t.Logf("✅ TEST 3 PASS: All %d documents have keyword metadata", updatedDocsCount)
	}

	// ============================================================
	// FINAL SUMMARY
	// ============================================================

	t.Log("=== FINAL SUMMARY ===")
	t.Logf("Places job created: %d documents", placesDocCount)
	t.Logf("Keyword job processed: %d documents", keywordDocCount)
	t.Logf("Documents with metadata: %d documents", updatedDocsCount)

	// Overall test result
	if keywordDocCount > 0 && keywordDocCount == placesDocCount && updatedDocsCount == len(documentIDs) {
		t.Log("✅ ALL TESTS PASSED")
	} else {
		t.Log("❌ SOME TESTS FAILED - see details above")
	}
}

// pollForJobCompletion polls a job until completion and returns document_count
func pollForJobCompletion(t *testing.T, h *common.HTTPTestHelper, jobID string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

		jobResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			continue
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(jobResp, &job); err != nil {
			continue
		}

		status, ok := job["status"].(string)
		if !ok {
			continue
		}

		// Get document count from job
		docCount := 0
		if count, ok := job["document_count"].(float64); ok {
			docCount = int(count)
		}

		t.Logf("  Job %s status: %s (document_count: %d)", jobID[:8], status, docCount)

		if status == "completed" {
			return docCount, nil
		}

		if status == "failed" {
			errorMsg := "unknown error"
			if errField, ok := job["error"].(string); ok {
				errorMsg = errField
			}
			return 0, fmt.Errorf("job failed: %s", errorMsg)
		}
	}

	return 0, fmt.Errorf("timeout waiting for job completion after %v", timeout)
}

// pollForParentJobCreation polls for a parent job to be created after job definition execution
func pollForParentJobCreation(t *testing.T, h *common.HTTPTestHelper, sourceType string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		<-ticker.C

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

		// Look for parent job with matching source_type
		for _, job := range paginatedResponse.Jobs {
			jobType, _ := job["job_type"].(string)
			srcType, _ := job["source_type"].(string)

			// Match parent jobs by source_type
			if jobType == "parent" && srcType == sourceType {
				jobID := job["id"].(string)
				t.Logf("  Found parent job: %s (source_type: %s)", jobID[:8], srcType)
				return jobID, nil
			}
		}
	}

	return "", fmt.Errorf("timeout waiting for parent job creation after %v", timeout)
}
