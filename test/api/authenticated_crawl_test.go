// Package api contains API integration tests.
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/quaero/test/common"
)

// startMockAuthServer creates a test server that simulates an authenticated website
// Returns the server, expected cookie name, and expected cookie value
func startMockAuthServer() (*httptest.Server, string, string) {
	expectedCookieName := "auth_token"
	expectedCookieValue := "secret123"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for the expected cookie
		cookie, err := r.Cookie(expectedCookieName)
		if err != nil || cookie.Value != expectedCookieValue {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error": "authentication required"}`))
			return
		}

		// Return authenticated content
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message": "authenticated content", "page": "test-page"}`))
	})

	server := httptest.NewServer(handler)
	return server, expectedCookieName, expectedCookieValue
}

// waitForJobCompletion polls the job status until completion or timeout
func waitForJobCompletion(t *testing.T, h *common.HTTPTestHelper, jobID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		resp, err := h.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			return "", fmt.Errorf("failed to get job status: %w", err)
		}

		var job map[string]interface{}
		if err := h.ParseJSONResponse(resp, &job); err != nil {
			return "", fmt.Errorf("failed to parse job response: %w", err)
		}

		status, ok := job["status"].(string)
		if !ok {
			return "", fmt.Errorf("job status field missing or not a string")
		}

		t.Logf("Job %s status: %s", jobID, status)

		if status == "completed" || status == "failed" || status == "cancelled" {
			return status, nil
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("job did not complete within %v", timeout)
}

// TestAuthenticatedCrawlWithAuthID tests the complete auth_id propagation and cookie injection flow
func TestAuthenticatedCrawlWithAuthID(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestAuthenticatedCrawlWithAuthID")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Step 1: Start Mock Authenticated Server
	mockServer, expectedCookieName, expectedCookieValue := startMockAuthServer()
	defer mockServer.Close()
	mockServerURL := mockServer.URL
	env.LogTest(t, "Started mock auth server at: %s", mockServerURL)

	// Step 2: Store Authentication Credentials
	authData := map[string]interface{}{
		"baseUrl":   mockServerURL,
		"userAgent": "Mozilla/5.0 Test",
		"cookies": []map[string]interface{}{
			{
				"name":     expectedCookieName,
				"value":    expectedCookieValue,
				"domain":   "localhost",
				"path":     "/",
				"secure":   false,
				"httpOnly": true,
			},
		},
		"tokens": map[string]string{},
	}

	resp, err := h.POST("/api/auth", authData)
	if err != nil {
		t.Fatalf("Failed to store auth credentials: %v", err)
	}
	h.AssertStatusCode(resp, http.StatusOK)

	// Get auth_id from response or list
	var authResponse map[string]interface{}
	if err := h.ParseJSONResponse(resp, &authResponse); err != nil {
		t.Fatalf("Failed to parse auth response: %v", err)
	}

	var authID string
	if id, ok := authResponse["auth_id"].(string); ok {
		authID = id
	} else {
		// Query /api/auth/list to get the ID
		listResp, err := h.GET("/api/auth/list")
		if err != nil {
			t.Fatalf("Failed to list auth credentials: %v", err)
		}
		h.AssertStatusCode(listResp, http.StatusOK)

		var auths []map[string]interface{}
		if err := h.ParseJSONResponse(listResp, &auths); err != nil {
			t.Fatalf("Failed to parse auth list: %v", err)
		}

		if len(auths) == 0 {
			t.Fatalf("No auth credentials found after storing")
		}

		// Use the first (most recent) auth entry
		if id, ok := auths[0]["id"].(string); ok {
			authID = id
		} else {
			t.Fatalf("Auth entry missing id field")
		}
	}

	env.LogTest(t, "Stored auth credentials with ID: %s", authID)

	// Step 3: Create Quick Crawl Job with auth_id
	jobPayload := map[string]interface{}{
		"url":       mockServerURL,
		"name":      "Authenticated Crawl Test",
		"auth_id":   authID,
		"max_depth": 1,
		"max_pages": 5,
	}

	jobResp, err := h.POST("/api/job-definitions/quick-crawl", jobPayload)
	if err != nil {
		t.Fatalf("Failed to create quick crawl job: %v", err)
	}
	h.AssertStatusCode(jobResp, http.StatusAccepted)

	var jobResult map[string]interface{}
	if err := h.ParseJSONResponse(jobResp, &jobResult); err != nil {
		t.Fatalf("Failed to parse job creation response: %v", err)
	}

	parentJobID, ok := jobResult["job_id"].(string)
	if !ok || parentJobID == "" {
		t.Fatalf("Job creation response missing job_id: %v", jobResult)
	}

	env.LogTest(t, "Created quick crawl job with ID: %s", parentJobID)

	// Step 4: Verify auth_id in Parent Job Metadata
	jobStatusResp, err := h.GET(fmt.Sprintf("/api/jobs/%s", parentJobID))
	if err != nil {
		t.Fatalf("Failed to get job status: %v", err)
	}
	h.AssertStatusCode(jobStatusResp, http.StatusOK)

	var jobStatus map[string]interface{}
	if err := h.ParseJSONResponse(jobStatusResp, &jobStatus); err != nil {
		t.Fatalf("Failed to parse job status: %v", err)
	}

	metadata, ok := jobStatus["metadata"].(map[string]interface{})
	if !ok {
		t.Errorf("Job metadata field missing or not an object")
	} else {
		actualAuthID, ok := metadata["auth_id"].(string)
		if !ok {
			t.Errorf("auth_id not found in job metadata")
		} else if actualAuthID != authID {
			t.Errorf("auth_id mismatch: expected %s, got %s", authID, actualAuthID)
		} else {
			env.LogTest(t, "auth_id found in parent job metadata: %s", actualAuthID)
		}
	}

	// Step 5: Wait for Job Completion
	finalStatus, err := waitForJobCompletion(t, h, parentJobID, 60*time.Second)
	if err != nil {
		t.Fatalf("Job completion wait failed: %v", err)
	}

	if finalStatus != "completed" {
		// Fetch logs for error context
		logResp, err := h.GET(fmt.Sprintf("/api/jobs/%s/logs/aggregated?include_children=true&level=all", parentJobID))
		logMsg := ""
		if err == nil {
			var logResult map[string]interface{}
			h.ParseJSONResponse(logResp, &logResult)
			if logs, ok := logResult["logs"].([]interface{}); ok {
				logMsg = fmt.Sprintf(" (logs available: %d entries)", len(logs))
			}
		}
		t.Fatalf("Job failed with status: %s%s", finalStatus, logMsg)
	}

	env.LogTest(t, "Job completed successfully with status: %s", finalStatus)

	// Step 6: Verify Cookie Injection in Logs
	logResp, err := h.GET(fmt.Sprintf("/api/jobs/%s/logs/aggregated?include_children=true&level=all", parentJobID))
	if err != nil {
		t.Fatalf("Failed to get job logs: %v", err)
	}
	h.AssertStatusCode(logResp, http.StatusOK)

	var logResult map[string]interface{}
	if err := h.ParseJSONResponse(logResp, &logResult); err != nil {
		t.Fatalf("Failed to parse job logs: %v", err)
	}

	logs, ok := logResult["logs"].([]interface{})
	if !ok {
		t.Fatalf("Logs field missing or not an array")
	}

	env.LogTest(t, "Retrieved %d log entries", len(logs))

	foundAuthLogs := false
	foundCookieInjection := false
	foundSkipWarnings := false

	for _, logEntry := range logs {
		logMap, ok := logEntry.(map[string]interface{})
		if !ok {
			continue
		}

		message, ok := logMap["message"].(string)
		if !ok {
			continue
		}

		// Check for auth-related logs
		if strings.Contains(message, "🔐") ||
		   strings.Contains(message, "auth_id found in job metadata") ||
		   strings.Contains(message, "Cookies injected successfully") {
			foundAuthLogs = true
			t.Logf("Found auth log: %s", message)
		}

		// Check for cookie injection success
		if strings.Contains(message, "Cookies injected successfully") {
			foundCookieInjection = true
		}

		// Check for skip warnings (should not be present)
		if strings.Contains(message, "auth_id NOT found") ||
		   strings.Contains(message, "SKIP: No auth_id") {
			foundSkipWarnings = true
			t.Logf("Found skip warning: %s", message)
		}
	}

	if !foundAuthLogs {
		t.Errorf("No auth-related logs found")
	}

	if !foundCookieInjection {
		t.Errorf("Cookie injection success message not found")
	}

	if foundSkipWarnings {
		t.Errorf("Found skip warnings - auth_id propagation failed")
	}

	if foundAuthLogs && foundCookieInjection && !foundSkipWarnings {
		env.LogTest(t, "Cookie injection verified in logs")
	}

	// Step 7: Verify Authenticated Content in Documents
	docsResp, err := h.GET("/api/documents?limit=50")
	if err != nil {
		t.Fatalf("Failed to get documents: %v", err)
	}
	h.AssertStatusCode(docsResp, http.StatusOK)

	var docsResult map[string]interface{}
	if err := h.ParseJSONResponse(docsResp, &docsResult); err != nil {
		t.Fatalf("Failed to parse documents response: %v", err)
	}

	documents, ok := docsResult["documents"].([]interface{})
	if !ok {
		t.Fatalf("Documents field missing or not an array")
	}

	env.LogTest(t, "Retrieved %d documents", len(documents))

	foundAuthenticatedContent := false
	for _, docEntry := range documents {
		docMap, ok := docEntry.(map[string]interface{})
		if !ok {
			continue
		}

		sourceID, _ := docMap["source_id"].(string)
		url, _ := docMap["url"].(string)
		content, _ := docMap["content"].(string)

		// Check if this document matches our mock server
		if strings.Contains(sourceID, mockServerURL) || strings.Contains(url, mockServerURL) {
			if strings.Contains(content, "authenticated content") {
				foundAuthenticatedContent = true
				t.Logf("Found authenticated document: %s", sourceID)
				break
			}
		}
	}

	if !foundAuthenticatedContent {
		// Log all document info for debugging
		for _, docEntry := range documents {
			docMap, ok := docEntry.(map[string]interface{})
			if !ok {
				continue
			}
			title, _ := docMap["title"].(string)
			url, _ := docMap["url"].(string)
			t.Logf("Document: title=%s, url=%s", title, url)
		}
		t.Errorf("Authenticated content not found in documents - cookies may not have been injected")
	} else {
		env.LogTest(t, "Authenticated content found in documents")
	}

	// Step 8: Cleanup (optional - endpoints may not exist)
	// Try to delete auth credentials
	if deleteResp, err := h.DELETE(fmt.Sprintf("/api/auth/%s", authID)); err == nil {
		h.AssertStatusCode(deleteResp, http.StatusOK)
		env.LogTest(t, "Cleaned up auth credentials: %s", authID)
	}

	// Try to delete job (if endpoint exists)
	if deleteJobResp, err := h.DELETE(fmt.Sprintf("/api/jobs/%s", parentJobID)); err == nil {
		if deleteJobResp.StatusCode == http.StatusOK || deleteJobResp.StatusCode == http.StatusNoContent {
			env.LogTest(t, "Cleaned up test job: %s", parentJobID)
		}
	}

	env.LogTest(t, "Test completed successfully")
}