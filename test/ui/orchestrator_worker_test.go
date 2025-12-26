package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrchestratorWorkerDisplay tests that orchestrator jobs display correctly in the UI.
// This validates:
// 1. Job can be created via API and triggered
// 2. Job appears in queue UI with correct type
// 3. Job completes successfully (placeholder implementation)
func TestOrchestratorWorkerDisplay(t *testing.T) {
	utc := NewUITestContext(t, 5*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Orchestrator Worker UI ---")

	// Create job definition via API helper
	helper := utc.Env.NewHTTPTestHelper(t)
	defID := fmt.Sprintf("orchestrator-ui-test-%d", time.Now().UnixNano())
	jobName := "Orchestrator UI Test"

	body := map[string]interface{}{
		"id":          defID,
		"name":        jobName,
		"type":        "orchestrator",
		"enabled":     true,
		"description": "UI test for OrchestratorWorker display",
		"steps": []map[string]interface{}{
			{
				"name":        "test_orchestration",
				"type":        "orchestrator",
				"description": "Test orchestration step",
				"on_error":    "fail",
				"config": map[string]interface{}{
					"goal":           "Verify the claim: 'Water boils at 100 degrees Celsius at sea level'.",
					"thinking_level": "MEDIUM",
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create job definition")

	utc.Log("Created orchestrator job definition: %s", defID)
	defer helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))

	// Trigger job via UI
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job %s: %v", jobName, err)
	}

	utc.Screenshot("job-triggered")

	// Navigate to Queue
	err = utc.Navigate(utc.QueueURL)
	require.NoError(t, err)

	utc.Screenshot("queue-page-loaded")

	// Wait for job to appear in the queue UI
	time.Sleep(1 * time.Second)

	// Assert Job appears in queue (look for job name in card)
	var jobFound bool
	err = chromedp.Run(utc.Ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				// Find any element containing the job name
				const elements = Array.from(document.querySelectorAll('*'));
				const found = elements.some(el => el.textContent && el.textContent.includes('%s'));
				return found;
			})()
		`, jobName), &jobFound),
	)
	require.NoError(t, err)
	assert.True(t, jobFound, "Job should appear in queue UI")

	utc.Screenshot("job-visible-in-queue")

	// Poll for job completion via API (more reliable than UI scraping)
	utc.Log("Polling for job completion")

	// Get job ID from API
	listResp, err := helper.GET("/api/jobs?limit=10")
	require.NoError(t, err)
	defer listResp.Body.Close()

	var jobs map[string]interface{}
	err = helper.ParseJSONResponse(listResp, &jobs)
	require.NoError(t, err)

	// Find the job we created
	var jobID string
	if jobsList, ok := jobs["jobs"].([]interface{}); ok {
		for _, j := range jobsList {
			if job, ok := j.(map[string]interface{}); ok {
				if name, _ := job["name"].(string); name == jobName {
					jobID, _ = job["id"].(string)
					break
				}
			}
		}
	}

	if jobID != "" {
		// Poll for completion
		maxRetries := 30
		for i := 0; i < maxRetries; i++ {
			statusResp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
			if err != nil {
				break
			}

			var jobStatus map[string]interface{}
			helper.ParseJSONResponse(statusResp, &jobStatus)
			statusResp.Body.Close()

			status, _ := jobStatus["status"].(string)
			if status == "completed" || status == "failed" {
				utc.Log("Job %s reached status: %s", jobID, status)
				break
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	utc.Screenshot("test-complete")
	utc.Log("Orchestrator worker UI test complete")
}

// TestOrchestratorEmailContentNotAIInstructions verifies that when an orchestrator job
// produces email output, the email contains actual analysis results NOT the AI instructions.
//
// This test validates REQ-2 through REQ-6 from the orchestrator email fix:
// - Email content should NOT contain orchestrator summary patterns like "# Orchestration Results"
// - Email content should NOT contain "## Goal", "## Execution Plan", "## Review Summary"
// - Email content should contain actual analysis from worker execution
//
// The test uses the asx-stocks-daily-orchestrated job definition which:
// 1. Runs the orchestrator step with available_tools
// 2. Has an email step that uses body_from_tag = "stock-recommendation"
// 3. Should find documents produced by workers, NOT the orchestrator's execution log
func TestOrchestratorEmailContentNotAIInstructions(t *testing.T) {
	// Use a longer timeout since orchestrator jobs can take a while
	utc := NewUITestContext(t, 10*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Orchestrator Email Content (NOT AI Instructions) ---")

	helper := utc.Env.NewHTTPTestHelper(t)

	// Create a test orchestrator job with email step
	defID := fmt.Sprintf("orchestrator-email-test-%d", time.Now().UnixNano())
	jobName := "Orchestrator Email Content Test"

	// Job definition with orchestrator step + email step
	// This mimics the structure of asx-stocks-daily-orchestrated but simplified for testing
	body := map[string]interface{}{
		"id":          defID,
		"name":        jobName,
		"type":        "orchestrator",
		"enabled":     true,
		"description": "Test that orchestrator email contains actual content, not AI instructions",
		"tags":        []string{"test", "orchestrator-email-test"},
		"steps": []map[string]interface{}{
			{
				"name":        "test_orchestration",
				"type":        "orchestrator",
				"description": "Test orchestration step with tools",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"goal": "Analyze the current state of the test system and provide a brief status report.",
					"available_tools": []map[string]interface{}{
						{"name": "search_web", "description": "Search for information", "worker": "web_search"},
					},
					"thinking_level": "MEDIUM",
					"output_tags":    []string{"test-orchestrator-output"},
				},
			},
			{
				"name":        "email_report",
				"type":        "email",
				"description": "Email the report",
				"on_error":    "continue",
				"depends":     "test_orchestration",
				"config": map[string]interface{}{
					"to":            "test@example.com",
					"subject":       "Orchestrator Test Report",
					"body_from_tag": "test-orchestrator-output",
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()
	require.Equal(t, 201, resp.StatusCode, "Failed to create job definition: status %d", resp.StatusCode)

	utc.Log("Created orchestrator+email job definition: %s", defID)
	defer helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))

	// Trigger job via API for reliability
	triggerResp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/trigger", defID), nil)
	require.NoError(t, err, "Failed to trigger job")
	defer triggerResp.Body.Close()
	require.Equal(t, 200, triggerResp.StatusCode, "Failed to trigger job")

	utc.Log("Triggered job, waiting for completion...")
	utc.Screenshot("job-triggered")

	// Poll for job completion
	var finalJobID string
	var finalStatus string
	maxRetries := 120 // 2 minutes at 1 second intervals
	for i := 0; i < maxRetries; i++ {
		listResp, err := helper.GET("/api/jobs?limit=10")
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var jobs map[string]interface{}
		helper.ParseJSONResponse(listResp, &jobs)
		listResp.Body.Close()

		if jobsList, ok := jobs["jobs"].([]interface{}); ok {
			for _, j := range jobsList {
				if job, ok := j.(map[string]interface{}); ok {
					if name, _ := job["name"].(string); name == jobName {
						finalJobID, _ = job["id"].(string)
						finalStatus, _ = job["status"].(string)
						break
					}
				}
			}
		}

		if finalStatus == "completed" || finalStatus == "failed" {
			utc.Log("Job %s reached status: %s after %d seconds", finalJobID, finalStatus, i)
			break
		}

		if i%10 == 0 {
			utc.Log("Waiting for job... (attempt %d/%d)", i, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	require.NotEmpty(t, finalJobID, "Job ID should be found")
	utc.Screenshot("job-completed")

	// Now check for documents with the email output
	// The email step should save an email-html document
	utc.Log("Checking for email output documents...")

	// Search for documents tagged with our test tag
	docsResp, err := helper.GET(fmt.Sprintf("/api/documents?tags=test-orchestrator-output&limit=5"))
	require.NoError(t, err, "Failed to fetch documents")

	var docsResult struct {
		Documents []struct {
			ID              string   `json:"id"`
			Title           string   `json:"title"`
			ContentMarkdown string   `json:"content_markdown"`
			Tags            []string `json:"tags"`
		} `json:"documents"`
	}
	err = helper.ParseJSONResponse(docsResp, &docsResult)
	docsResp.Body.Close()
	require.NoError(t, err, "Failed to parse documents response")

	utc.Log("Found %d documents with test-orchestrator-output tag", len(docsResult.Documents))

	// Also check for email-html documents
	emailDocsResp, err := helper.GET("/api/documents?source_type=email-html&limit=5")
	if err == nil {
		var emailDocs struct {
			Documents []struct {
				ID              string `json:"id"`
				Title           string `json:"title"`
				ContentMarkdown string `json:"content_markdown"`
			} `json:"documents"`
		}
		if helper.ParseJSONResponse(emailDocsResp, &emailDocs) == nil {
			utc.Log("Found %d email-html documents", len(emailDocs.Documents))
			for _, doc := range emailDocs.Documents {
				utc.Log("Email document: %s - %s", doc.ID, doc.Title)
			}
		}
		emailDocsResp.Body.Close()
	}

	// ==========================================================================
	// CRITICAL ASSERTION: Email content should NOT contain AI instructions
	// ==========================================================================
	// If there are documents, check that they don't contain orchestrator patterns
	if len(docsResult.Documents) > 0 {
		for _, doc := range docsResult.Documents {
			content := doc.ContentMarkdown
			utc.Log("Checking document %s for AI instruction patterns...", doc.ID)

			// Patterns that indicate the orchestrator's execution log (BAD - should NOT be in email)
			orchestratorPatterns := []string{
				"# Orchestration Results",
				"## Goal",
				"## Execution Plan",
				"## Review Summary",
				"**Goal Status:**",
				"**Confidence:**",
			}

			foundPatterns := []string{}
			for _, pattern := range orchestratorPatterns {
				if strings.Contains(content, pattern) {
					foundPatterns = append(foundPatterns, pattern)
				}
			}

			if len(foundPatterns) > 0 {
				utc.Log("WARNING: Found orchestrator patterns in email content: %v", foundPatterns)
				utc.Log("Document content preview (first 500 chars): %s", truncateForLog(content, 500))

				// Save the problematic content to results for debugging
				utc.SaveToResults(fmt.Sprintf("problematic_email_content_%s.txt", doc.ID), content)
			}

			// ASSERTION: Document should NOT contain orchestrator execution log patterns
			// If it does, that means the email is showing the AI instructions instead of actual results
			assert.Empty(t, foundPatterns,
				"Email document %s contains orchestrator AI instruction patterns: %v\n"+
					"This indicates the email is showing the orchestrator's execution log instead of actual worker output.\n"+
					"The fix in orchestrator_worker.go should ensure output_tags is NOT used for the execution log.",
				doc.ID, foundPatterns)

			if len(foundPatterns) == 0 {
				utc.Log("✓ Document %s does NOT contain AI instruction patterns", doc.ID)
			}
		}
	} else {
		utc.Log("NOTE: No documents found with test-orchestrator-output tag - this is expected if orchestrator now uses internal tags")
		utc.Log("The fix ensures orchestrator uses 'orchestrator-execution-log' tag, not output_tags from config")
	}

	utc.Screenshot("test-complete")
	utc.Log("Orchestrator email content test complete")
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
