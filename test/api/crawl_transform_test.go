package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test"
)

// TestCrawlToDocumentTransformation tests the complete flow from crawl to document transformation
// This test verifies that completed crawl jobs produce documents via the transformer
func TestCrawlToDocumentTransformation(t *testing.T) {
	baseURL := test.MustGetTestServerURL()
	h := test.NewHTTPTestHelper(t, baseURL)

	// 1. Create a minimal test source pointing to local mock endpoint
	source := map[string]interface{}{
		"name":     "Test Source for Crawl-Transform Flow",
		"type":     "jira",
		"base_url": "http://localhost:3333/rest/api/3/project",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"max_depth":    1,
			"max_pages":    5,
			"follow_links": false,
			"concurrency":  1,
			"rate_limit":   100,
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

	// 2. Get initial document count
	initialDocsResp, err := h.GET("/api/documents")
	if err != nil {
		t.Fatalf("Failed to get initial documents: %v", err)
	}

	var initialDocs map[string]interface{}
	if err := h.ParseJSONResponse(initialDocsResp, &initialDocs); err != nil {
		t.Fatalf("Failed to parse initial documents response: %v", err)
	}

	var initialCount int
	if total, ok := initialDocs["total"].(float64); ok {
		initialCount = int(total)
	}

	t.Logf("Initial document count: %d", initialCount)

	// 3. Trigger crawl_and_collect by creating a job
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

	jobID, ok := jobResult["job_id"].(string)
	if !ok {
		t.Fatal("Could not extract job ID")
	}

	// Cleanup job at end
	defer func() {
		h.DELETE("/api/jobs/" + jobID)
	}()

	t.Logf("Created job: %s", jobID)

	// 4. Wait for job completion (with timeout)
	// The job should complete successfully using the local mock Jira endpoint
	maxWait := 30 * time.Second
	pollInterval := 500 * time.Millisecond
	deadline := time.Now().Add(maxWait)

	var jobStatus string
	for time.Now().Before(deadline) {
		jobDetailsResp, err := h.GET("/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("Failed to get job details: %v", err)
		}

		var jobDetails map[string]interface{}
		if err := h.ParseJSONResponse(jobDetailsResp, &jobDetails); err != nil {
			t.Fatalf("Failed to parse job details: %v", err)
		}

		if status, ok := jobDetails["status"].(string); ok {
			jobStatus = status
			t.Logf("Job status: %s", jobStatus)

			// Job completed (successfully or with errors)
			if jobStatus == "completed" || jobStatus == "failed" || jobStatus == "cancelled" {
				break
			}
		}

		time.Sleep(pollInterval)
	}

	// 5. If job completed successfully, verify documents were created
	if jobStatus == "completed" {
		// Wait for transformer to process and persist documents
		time.Sleep(3 * time.Second)

		// Get final document count
		finalDocsResp, err := h.GET("/api/documents")
		if err != nil {
			t.Fatalf("Failed to get final documents: %v", err)
		}

		var finalDocs map[string]interface{}
		if err := h.ParseJSONResponse(finalDocsResp, &finalDocs); err != nil {
			t.Fatalf("Failed to parse final documents response: %v", err)
		}

		var finalCount int
		if total, ok := finalDocs["total"].(float64); ok {
			finalCount = int(total)
		}

		t.Logf("Final document count: %d (increase: %d)", finalCount, finalCount-initialCount)

		// Assert document count increased
		if finalCount <= initialCount {
			t.Errorf("Expected document count to increase, but initial=%d, final=%d", initialCount, finalCount)
		}

		// 6. Verify at least one document has expected source_type and non-empty ContentMarkdown
		if docs, ok := finalDocs["documents"].([]interface{}); ok && len(docs) > 0 {
			foundValidDoc := false
			for _, doc := range docs {
				if docMap, ok := doc.(map[string]interface{}); ok {
					sourceType, hasSourceType := docMap["source_type"].(string)
					content, hasContent := docMap["content_markdown"].(string)

					// Check if this document matches our source type and has content
					if hasSourceType && sourceType == "jira" && hasContent && content != "" {
						foundValidDoc = true
						t.Logf("✓ Found valid transformed document: source_type=%s, content_length=%d",
							sourceType, len(content))
						break
					}
				}
			}

			if !foundValidDoc {
				t.Error("Expected to find at least one document with source_type='jira' and non-empty content")
			}
		}

		t.Log("✓ Crawl to document transformation flow validated")
	} else {
		t.Errorf("Job did not complete successfully (status: %s)", jobStatus)
	}
}
