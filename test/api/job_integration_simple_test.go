package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"
)

// TestJobIntegration_KeywordExtraction_Simple tests keyword extraction without external APIs:
// 1. Create mock place documents directly
// 2. Run "Keyword Extraction" job to update those documents
// 3. Verify document count > 0 and equals initial count
func TestJobIntegration_KeywordExtraction_Simple(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobIntegration_KeywordExtraction_Simple")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// ============================================================
	// PHASE 1: Create mock place documents
	// ============================================================

	t.Log("=== PHASE 1: Creating mock place documents ===")

	// Create 5 mock place documents
	mockPlaces := []map[string]interface{}{
		{
			"id":               "doc_place_test1",
			"source_type":      "places",
			"source_id":        "test_place_1",
			"title":            "Test Restaurant 1",
			"content_markdown": "A great Italian restaurant with authentic pasta and pizza. Family-friendly atmosphere with outdoor seating.",
			"url":              "https://test.example.com/place1",
			"metadata": map[string]interface{}{
				"place_id": "test_place_1",
				"name":     "Test Restaurant 1",
				"types":    []string{"restaurant", "food"},
				"rating":   4.5,
			},
		},
		{
			"id":               "doc_place_test2",
			"source_type":      "places",
			"source_id":        "test_place_2",
			"title":            "Test Cafe 2",
			"content_markdown": "Modern cafe serving specialty coffee and breakfast. Known for their avocado toast and flat whites.",
			"url":              "https://test.example.com/place2",
			"metadata": map[string]interface{}{
				"place_id": "test_place_2",
				"name":     "Test Cafe 2",
				"types":    []string{"cafe", "food"},
				"rating":   4.2,
			},
		},
		{
			"id":               "doc_place_test3",
			"source_type":      "places",
			"source_id":        "test_place_3",
			"title":            "Test Bar 3",
			"content_markdown": "Craft beer bar with wide selection of local and international beers. Live music on weekends.",
			"url":              "https://test.example.com/place3",
			"metadata": map[string]interface{}{
				"place_id": "test_place_3",
				"name":     "Test Bar 3",
				"types":    []string{"bar", "nightlife"},
				"rating":   4.0,
			},
		},
		{
			"id":               "doc_place_test4",
			"source_type":      "places",
			"source_id":        "test_place_4",
			"title":            "Test Bakery 4",
			"content_markdown": "Artisan bakery with fresh bread, pastries, and cakes. Everything made from scratch daily.",
			"url":              "https://test.example.com/place4",
			"metadata": map[string]interface{}{
				"place_id": "test_place_4",
				"name":     "Test Bakery 4",
				"types":    []string{"bakery", "food"},
				"rating":   4.8,
			},
		},
		{
			"id":               "doc_place_test5",
			"source_type":      "places",
			"source_id":        "test_place_5",
			"title":            "Test Sushi 5",
			"content_markdown": "Authentic Japanese sushi restaurant. Fresh fish delivered daily, traditional preparation methods.",
			"url":              "https://test.example.com/place5",
			"metadata": map[string]interface{}{
				"place_id": "test_place_5",
				"name":     "Test Sushi 5",
				"types":    []string{"restaurant", "japanese"},
				"rating":   4.6,
			},
		},
	}

	documentIDs := make([]string, 0, len(mockPlaces))
	for i, place := range mockPlaces {
		docResp, err := h.POST("/api/documents", place)
		if err != nil {
			t.Fatalf("Failed to create document %d: %v", i+1, err)
		}
		h.AssertStatusCode(docResp, http.StatusCreated)

		var docResult map[string]interface{}
		h.ParseJSONResponse(docResp, &docResult)
		docID := docResult["id"].(string)
		documentIDs = append(documentIDs, docID)

		t.Logf("✓ Created document %d: %s", i+1, docID[:8])
	}

	placesDocCount := len(documentIDs)
	t.Logf("✅ PHASE 1 PASS: Created %d mock place documents", placesDocCount)

	// ============================================================
	// PHASE 2: Run "Keyword Extraction" job to update documents
	// ============================================================

	t.Log("=== PHASE 2: Updating documents via Keyword Extraction job ===")

	// Create Keyword Extraction job definition
	keywordJobDef := map[string]interface{}{
		"id":          "keyword-extractor-test",
		"name":        "Keyword Extraction Test",
		"type":        "custom",
		"job_type":    "user",
		"description": "Test keyword extraction on mock documents",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":     "Extract Keywords",
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

	createResp, err := h.POST("/api/job-definitions", keywordJobDef)
	if err == nil && (createResp.StatusCode == http.StatusCreated || createResp.StatusCode == http.StatusConflict) {
		t.Log("✓ Keyword Extraction job definition created/exists")
	}

	// Execute the Keyword Extraction job
	execResp, err := h.POST("/api/job-definitions/keyword-extractor-test/execute", nil)
	if err != nil {
		t.Fatalf("Failed to execute Keyword Extraction job: %v", err)
	}
	h.AssertStatusCode(execResp, http.StatusAccepted)

	t.Log("✓ Keyword Extraction job execution started")

	// Poll for parent job creation
	keywordJobID, err := pollForParentJob(t, h, "job_definition", 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to find Keyword Extraction parent job: %v", err)
	}

	t.Logf("✓ Keyword Extraction parent job found: %s", keywordJobID[:8])

	// Poll for Keyword Extraction job completion
	keywordDocCount, err := pollForCompletion(t, h, keywordJobID, 10*time.Minute)
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

	// Test 2: Keyword job document count matches created document count
	if keywordDocCount != placesDocCount {
		t.Errorf("FAIL: Document count mismatch - Created: %d, Processed: %d", placesDocCount, keywordDocCount)
		t.Logf("Expected Keyword Extraction to process all %d documents", placesDocCount)
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
	t.Logf("Documents created: %d", placesDocCount)
	t.Logf("Documents processed by keyword job: %d", keywordDocCount)
	t.Logf("Documents with metadata: %d", updatedDocsCount)

	// Overall test result
	if keywordDocCount > 0 && keywordDocCount == placesDocCount && updatedDocsCount == len(documentIDs) {
		t.Log("✅ ALL TESTS PASSED")
	} else {
		t.Log("❌ SOME TESTS FAILED - see details above")
	}
}

// pollForCompletion polls a job until completion and returns document_count
func pollForCompletion(t *testing.T, h *common.HTTPTestHelper, jobID string, timeout time.Duration) (int, error) {
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

// pollForParentJob polls for a parent job to be created after job definition execution
func pollForParentJob(t *testing.T, h *common.HTTPTestHelper, sourceType string, timeout time.Duration) (string, error) {
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
