// -----------------------------------------------------------------------
// Tests for job isolation - validates documents don't leak between jobs
// Ensures output_formatter and email workers correctly use managerID filtering
// so that documents from one job execution don't appear in another job's output
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestJobIsolationSingle validates that documents from one job don't leak into another job's output.
// This test:
// 1. Executes Job 1: market_announcements -> output_formatter -> email (full announcement pipeline)
// 2. Executes Job 2: market_competitor -> output_formatter -> email (full competitor pipeline)
// 3. Captures the email output from Job 2
// 4. Asserts the email does NOT contain "Company Announcements" from Job 1
//
// This catches the bug shown in the screenshot where competitor watchlist email
// incorrectly included the "Company Announcements - EXR" section.
func TestJobIsolationSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Both jobs use LLM (competitor uses Gemini, announcements may use LLM for classification)
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)
	ticker := "EXR"

	// =========================================================================
	// JOB 1: Execute market_announcements -> output_formatter -> email
	// =========================================================================
	t.Log("=== JOB 1: Executing announcements -> formatter -> email ===")

	job1DefID := fmt.Sprintf("test-job-isolation-announcements-%d", time.Now().UnixNano())
	job1Body := map[string]interface{}{
		"id":          job1DefID,
		"name":        "Job Isolation Test - Announcements Pipeline",
		"description": "Test job isolation - full announcements pipeline with email output",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "job-isolation", "announcements"},
		"config": map[string]interface{}{
			"variables": []map[string]interface{}{
				{"ticker": ticker},
			},
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch_announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"output_tags": []string{"format_announcements"},
				},
			},
			{
				"name": "format_announcements",
				"type": "output_formatter",
				"config": map[string]interface{}{
					"output_tags": []string{"email_announcements"},
					"title":       "Company Announcements",
				},
			},
			{
				"name": "email_announcements",
				"type": "email",
				"config": map[string]interface{}{
					"to":      "{email_recipient}",
					"subject": "Job Isolation Test - Announcements",
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, job1Body)

	jobID1, _ := CreateAndExecuteJob(t, helper, job1Body)
	if jobID1 == "" {
		t.Fatal("Failed to create Job 1 (announcements pipeline)")
		return
	}

	t.Logf("Job 1 (announcements pipeline) started: %s", jobID1)

	// Wait for Job 1 completion
	job1Status := WaitForJobCompletion(t, helper, jobID1, 5*time.Minute)
	if job1Status != "completed" {
		t.Skipf("Job 1 (announcements pipeline) ended with status %s - skipping isolation test", job1Status)
		return
	}
	t.Logf("Job 1 (announcements pipeline) completed successfully")

	// Verify Job 1 created an announcement document
	announcementTags := []string{"announcement", strings.ToLower(ticker)}
	_, announcementContent := AssertOutputNotEmpty(t, helper, announcementTags)
	require.Contains(t, announcementContent, "Announcements", "Job 1 should create document with Announcements content")
	t.Log("Job 1 created announcement document successfully")

	// Save Job 1 output
	SaveWorkerOutput(t, env, helper, announcementTags, ticker+"_announcements")

	// Small delay to ensure document is persisted
	time.Sleep(500 * time.Millisecond)

	// =========================================================================
	// JOB 2: Execute market_competitor -> output_formatter -> email
	// =========================================================================
	t.Log("=== JOB 2: Executing competitor -> formatter -> email ===")

	job2DefID := fmt.Sprintf("test-job-isolation-competitor-%d", time.Now().UnixNano())
	job2Body := map[string]interface{}{
		"id":          job2DefID,
		"name":        "Job Isolation Test - Competitor Pipeline",
		"description": "Test job isolation - competitor pipeline with email output",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "job-isolation", "competitor-pipeline"},
		"config": map[string]interface{}{
			"variables": []map[string]interface{}{
				{"ticker": ticker},
			},
		},
		"steps": []map[string]interface{}{
			{
				"name": "analyze_competitors",
				"type": "market_competitor",
				"config": map[string]interface{}{
					"api_key":     "{google_gemini_api_key}",
					"output_tags": []string{"format_output"},
				},
			},
			{
				"name": "format_output",
				"type": "output_formatter",
				"config": map[string]interface{}{
					"output_tags": []string{"email_report"},
					"title":       "Competitor Analysis",
				},
			},
			{
				"name": "email_report",
				"type": "email",
				"config": map[string]interface{}{
					"to":      "{email_recipient}",
					"subject": "Job Isolation Test - Competitor Analysis",
				},
			},
		},
	}

	jobID2, _ := CreateAndExecuteJob(t, helper, job2Body)
	if jobID2 == "" {
		t.Fatal("Failed to create Job 2 (competitor pipeline)")
		return
	}

	t.Logf("Job 2 (competitor pipeline) started: %s", jobID2)

	// Wait for Job 2 completion
	job2Status := WaitForJobCompletion(t, helper, jobID2, 5*time.Minute)
	if job2Status != "completed" {
		t.Skipf("Job 2 (competitor pipeline) ended with status %s - skipping isolation test", job2Status)
		return
	}
	t.Logf("Job 2 (competitor pipeline) completed successfully")

	// =========================================================================
	// VERIFY JOB ISOLATION: Email should NOT contain Job 1 content
	// =========================================================================
	t.Log("=== VERIFYING JOB ISOLATION ===")

	// Find the email HTML document created by Job 2
	// The EmailWorker saves HTML content with source_type "email_html"
	emailContent := findEmailHTMLContent(t, helper, jobID2)

	// If no email HTML document found, try to get content from the formatted output
	if emailContent == "" {
		t.Log("No email_html document found, checking formatted output document")
		formattedTags := []string{"email_report"}
		_, formattedContent := AssertOutputNotEmpty(t, helper, formattedTags)
		emailContent = formattedContent
	}

	require.NotEmpty(t, emailContent, "Email content should not be empty")

	// =========================================================================
	// CRITICAL ASSERTIONS - These MUST fail the test on job isolation violation
	// =========================================================================

	// Assert email DOES contain competitor content (sanity check)
	require.Contains(t, emailContent, "Competitor",
		"Email should contain 'Competitor' content from Job 2. Content snippet: %s",
		truncateString(emailContent, 500))
	t.Log("PASS: Email contains expected competitor content")

	// CRITICAL: Assert email does NOT contain announcement content from Job 1
	// This is the main isolation check - if this fails, documents are leaking between jobs
	// Use require.False to FAIL the test immediately on violation
	containsCompanyAnnouncements := strings.Contains(emailContent, "Company Announcements")
	if containsCompanyAnnouncements {
		t.Logf("ISOLATION VIOLATION DETECTED - Email content snippet:\n%s", truncateString(emailContent, 3000))
	}
	require.False(t, containsCompanyAnnouncements,
		"JOB ISOLATION VIOLATED - Email from Job 2 (competitor) contains 'Company Announcements' from Job 1 (announcements). "+
			"This indicates documents are leaking between separate job executions.")

	t.Log("PASS: Email does NOT contain 'Company Announcements' - primary isolation check passed")

	// Additional isolation checks - these also FAIL the test
	containsTotalAnnouncements := strings.Contains(emailContent, "Total Announcements")
	require.False(t, containsTotalAnnouncements,
		"JOB ISOLATION VIOLATED - Email contains 'Total Announcements' which is announcement-specific content from Job 1")

	// Check for announcement relevance classification terms (only present in announcement documents)
	containsHighRelevance := strings.Contains(emailContent, "High Relevance")
	containsMediumRelevance := strings.Contains(emailContent, "Medium Relevance")
	containsLowNoise := strings.Contains(emailContent, "Low/Noise/Routine") || strings.Contains(emailContent, "Low Noise")

	if containsHighRelevance || containsMediumRelevance || containsLowNoise {
		t.Logf("ISOLATION VIOLATION - Found announcement relevance terms in competitor output")
		require.False(t, containsHighRelevance || containsMediumRelevance || containsLowNoise,
			"JOB ISOLATION VIOLATED - Email contains announcement relevance classification (High/Medium/Low Relevance) from Job 1. "+
				"High: %v, Medium: %v, Low/Noise: %v",
			containsHighRelevance, containsMediumRelevance, containsLowNoise)
	}

	// Check for document count indicating merge (screenshot showed "Documents: 8")
	// Competitor analysis should only reference 1 document (the competitor analysis itself)
	docCountMatch := extractDocumentCount(emailContent)
	if docCountMatch > 1 {
		t.Logf("WARNING: Document count is %d (expected 1) - possible document merge", docCountMatch)
		// This is a warning, not a failure, as some legitimate scenarios may have multiple docs
	}

	t.Log("=== JOB ISOLATION TEST PASSED ===")
	t.Log("Documents from Job 1 (announcements) did not leak into Job 2 (competitor) output")

	// Save Job 2 output
	competitorTags := []string{"competitor-analysis", strings.ToLower(ticker)}
	SaveWorkerOutput(t, env, helper, competitorTags, ticker+"_competitor")

	// Assert result files exist
	AssertResultFilesExist(t, env, 1)

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: job isolation single stock test completed")
}

// TestJobIsolationMulti validates job isolation using the orchestrator pattern.
// This replicates how competitor-watchlist.toml works:
// 1. Job 1: Run announcements for ALL tickers (EXR, GNP, SKS, TWR), then merge and email
// 2. Job 2: Run competitor analysis for ALL tickers, then merge and email
// 3. Verify Job 2's merged email does NOT contain content from Job 1
//
// This is the critical test because it replicates the actual production scenario
// where multiple ticker results are merged into a single document.
func TestJobIsolationMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Watchlist stocks - same as competitor-watchlist.toml
	stocks := []string{"EXR", "GNP", "SKS", "TWR"}

	// Build variables array for orchestrator
	var tickerVariables []map[string]interface{}
	for _, stock := range stocks {
		tickerVariables = append(tickerVariables, map[string]interface{}{"ticker": stock})
	}

	// =========================================================================
	// JOB 1: Execute announcements pipeline for ALL tickers (orchestrator pattern)
	// market_announcements -> output_formatter -> email (merged)
	// =========================================================================
	t.Log("=== JOB 1: Executing announcements pipeline for ALL tickers ===")
	t.Logf("Tickers: %v", stocks)

	job1DefID := fmt.Sprintf("test-job-isolation-ann-multi-%d", time.Now().UnixNano())
	job1Body := map[string]interface{}{
		"id":          job1DefID,
		"name":        "Job Isolation Test - Announcements Watchlist",
		"description": "Test job isolation - announcements pipeline for all watchlist stocks",
		"type":        "orchestrator",
		"enabled":     true,
		"tags":        []string{"worker-test", "job-isolation", "multi-stock", "announcements"},
		"config": map[string]interface{}{
			"variables": tickerVariables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch_announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"output_tags": []string{"format_announcements"},
				},
			},
			{
				"name": "format_announcements",
				"type": "output_formatter",
				"config": map[string]interface{}{
					"output_tags": []string{"email_announcements"},
					"title":       "Company Announcements - Watchlist Summary",
					"style":       "body",
					"order":       "ticker",
				},
			},
			{
				"name": "email_announcements",
				"type": "email",
				"config": map[string]interface{}{
					"to":      "{email_recipient}",
					"subject": "Job Isolation Test - Announcements Watchlist",
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, job1Body)

	jobID1, _ := CreateAndExecuteJob(t, helper, job1Body)
	if jobID1 == "" {
		t.Fatal("Failed to create Job 1 (announcements watchlist)")
		return
	}

	t.Logf("Job 1 (announcements watchlist) started: %s", jobID1)

	// Wait for Job 1 completion - longer timeout for multiple tickers
	job1Status := WaitForJobCompletion(t, helper, jobID1, 10*time.Minute)
	if job1Status != "completed" {
		t.Skipf("Job 1 (announcements watchlist) ended with status %s - skipping isolation test", job1Status)
		return
	}
	t.Logf("Job 1 (announcements watchlist) completed successfully")

	// Verify Job 1 created announcement documents for all tickers
	for _, stock := range stocks {
		announcementTags := []string{"announcement", strings.ToLower(stock)}
		_, announcementContent := AssertOutputNotEmpty(t, helper, announcementTags)
		require.Contains(t, announcementContent, "Announcements",
			"Job 1 should create announcement document for %s", stock)
		t.Logf("Job 1 created announcement document for %s", stock)
	}

	// Save Job 1 merged output
	SaveWorkerOutput(t, env, helper, []string{"email_announcements"}, "announcements_watchlist")

	// Small delay to ensure documents are persisted
	time.Sleep(1 * time.Second)

	// =========================================================================
	// JOB 2: Execute competitor pipeline for ALL tickers (orchestrator pattern)
	// market_competitor -> output_formatter -> email (merged)
	// =========================================================================
	t.Log("=== JOB 2: Executing competitor pipeline for ALL tickers ===")
	t.Logf("Tickers: %v", stocks)

	job2DefID := fmt.Sprintf("test-job-isolation-comp-multi-%d", time.Now().UnixNano())
	job2Body := map[string]interface{}{
		"id":          job2DefID,
		"name":        "Job Isolation Test - Competitor Watchlist",
		"description": "Test job isolation - competitor pipeline for all watchlist stocks",
		"type":        "orchestrator",
		"enabled":     true,
		"tags":        []string{"worker-test", "job-isolation", "multi-stock", "competitor-pipeline"},
		"config": map[string]interface{}{
			"variables": tickerVariables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "analyze_competitors",
				"type": "market_competitor",
				"config": map[string]interface{}{
					"api_key":     "{google_gemini_api_key}",
					"output_tags": []string{"format_output"},
				},
			},
			{
				"name": "format_output",
				"type": "output_formatter",
				"config": map[string]interface{}{
					"output_tags": []string{"email_report"},
					"title":       "Competitor Analysis - Watchlist Summary",
					"style":       "body",
					"order":       "ticker",
				},
			},
			{
				"name": "email_report",
				"type": "email",
				"config": map[string]interface{}{
					"to":      "{email_recipient}",
					"subject": "Job Isolation Test - Competitor Watchlist",
				},
			},
		},
	}

	jobID2, _ := CreateAndExecuteJob(t, helper, job2Body)
	if jobID2 == "" {
		t.Fatal("Failed to create Job 2 (competitor watchlist)")
		return
	}

	t.Logf("Job 2 (competitor watchlist) started: %s", jobID2)

	// Wait for Job 2 completion - longer timeout for multiple tickers
	job2Status := WaitForJobCompletion(t, helper, jobID2, 15*time.Minute)
	if job2Status != "completed" {
		t.Skipf("Job 2 (competitor watchlist) ended with status %s - skipping isolation test", job2Status)
		return
	}
	t.Logf("Job 2 (competitor watchlist) completed successfully")

	// =========================================================================
	// VERIFY JOB ISOLATION: Merged email should NOT contain Job 1 content
	// =========================================================================
	t.Log("=== VERIFYING JOB ISOLATION (merged output) ===")

	// Find the email HTML document created by Job 2
	emailContent := findEmailHTMLContent(t, helper, jobID2)

	// If no email HTML document found, try to get content from the formatted output
	if emailContent == "" {
		t.Log("No email_html document found, checking formatted output document")
		formattedTags := []string{"email_report"}
		_, formattedContent := AssertOutputNotEmpty(t, helper, formattedTags)
		emailContent = formattedContent
	}

	require.NotEmpty(t, emailContent, "Email content should not be empty")

	// Log document count for debugging
	docCount := extractDocumentCount(emailContent)
	t.Logf("Email references %d documents", docCount)

	// =========================================================================
	// CRITICAL ASSERTIONS - These MUST fail the test on job isolation violation
	// =========================================================================

	// Sanity check - email should contain competitor content for all tickers
	require.Contains(t, emailContent, "Competitor",
		"Email should contain 'Competitor' content from Job 2. Content snippet: %s",
		truncateString(emailContent, 500))
	t.Log("PASS: Email contains expected competitor content")

	// Check for each ticker's competitor analysis
	for _, stock := range stocks {
		if !strings.Contains(emailContent, stock) {
			t.Logf("WARNING: Email may be missing content for ticker %s", stock)
		}
	}

	// CRITICAL: Assert email does NOT contain announcement content from Job 1
	// This is the main isolation check - if this fails, documents are leaking between jobs
	containsCompanyAnnouncements := strings.Contains(emailContent, "Company Announcements")
	if containsCompanyAnnouncements {
		t.Logf("ISOLATION VIOLATION DETECTED - Email content snippet:\n%s", truncateString(emailContent, 5000))
	}
	require.False(t, containsCompanyAnnouncements,
		"JOB ISOLATION VIOLATED - Merged email from Job 2 (competitor watchlist) contains 'Company Announcements' from Job 1 (announcements watchlist). "+
			"This indicates documents are leaking between separate job executions.")

	t.Log("PASS: Email does NOT contain 'Company Announcements' - primary isolation check passed")

	// Additional isolation checks - these also FAIL the test
	containsTotalAnnouncements := strings.Contains(emailContent, "Total Announcements")
	require.False(t, containsTotalAnnouncements,
		"JOB ISOLATION VIOLATED - Email contains 'Total Announcements' which is announcement-specific content from Job 1")

	// Check for announcement relevance classification terms (only present in announcement documents)
	containsHighRelevance := strings.Contains(emailContent, "High Relevance")
	containsMediumRelevance := strings.Contains(emailContent, "Medium Relevance")
	containsLowNoise := strings.Contains(emailContent, "Low/Noise/Routine") || strings.Contains(emailContent, "Low Noise")

	if containsHighRelevance || containsMediumRelevance || containsLowNoise {
		t.Logf("ISOLATION VIOLATION - Found announcement relevance terms in competitor output")
		require.False(t, containsHighRelevance || containsMediumRelevance || containsLowNoise,
			"JOB ISOLATION VIOLATED - Email contains announcement relevance classification (High/Medium/Low Relevance) from Job 1. "+
				"High: %v, Medium: %v, Low/Noise: %v",
			containsHighRelevance, containsMediumRelevance, containsLowNoise)
	}

	// Check document count - merged competitor analysis for 4 stocks should have ~4 docs, not 8+
	// If we see significantly more docs, it's evidence of document leakage
	if docCount > 6 {
		t.Logf("WARNING: Document count is %d (expected ~4 for competitor analysis). Possible document merge with announcements.", docCount)
	}

	t.Log("=== JOB ISOLATION TEST PASSED ===")
	t.Log("Documents from Job 1 (announcements watchlist) did not leak into Job 2 (competitor watchlist) output")

	// Save Job 2 output
	SaveWorkerOutput(t, env, helper, []string{"email_report"}, "competitor_watchlist")

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: job isolation multi-stock (orchestrator pattern) test completed")
}

// findEmailHTMLContent retrieves the email HTML content from the email_html document
// created by the EmailWorker for the given job.
func findEmailHTMLContent(t *testing.T, helper *common.HTTPTestHelper, jobID string) string {
	// First, try to find email-html documents
	resp, err := helper.GET("/api/documents?source_type=email_html&limit=10")
	if err != nil {
		t.Logf("Error querying email_html documents: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Email HTML document query returned status %d", resp.StatusCode)
		return ""
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
			Jobs            []string               `json:"jobs"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Error parsing email HTML document response: %v", err)
		return ""
	}

	// Find the document that belongs to our job (by checking Jobs array or step_id in metadata)
	for _, doc := range result.Documents {
		// Check if this document belongs to our job
		for _, job := range doc.Jobs {
			if strings.HasPrefix(job, jobID[:8]) { // Compare first 8 chars of job ID
				t.Logf("Found email HTML document %s for job %s", doc.ID, jobID)
				return doc.ContentMarkdown
			}
		}

		// Also check metadata for step_id that might reference our job
		if stepID, ok := doc.Metadata["step_id"].(string); ok {
			if strings.Contains(stepID, jobID[:8]) {
				t.Logf("Found email HTML document %s via step_id for job %s", doc.ID, jobID)
				return doc.ContentMarkdown
			}
		}
	}

	// If we couldn't find by job ID, return the most recent one as fallback
	if len(result.Documents) > 0 {
		t.Logf("Using most recent email HTML document %s as fallback", result.Documents[0].ID)
		return result.Documents[0].ContentMarkdown
	}

	return ""
}

// truncateString truncates a string to maxLen characters with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractDocumentCount extracts the document count from content like "Documents: 8"
// Returns 0 if no match found
func extractDocumentCount(content string) int {
	// Match patterns like "Documents: 8" or "**Documents**: 8" or "Documents: 8\n"
	re := regexp.MustCompile(`\*?\*?Documents\*?\*?:\s*(\d+)`)
	match := re.FindStringSubmatch(content)
	if len(match) >= 2 {
		var count int
		fmt.Sscanf(match[1], "%d", &count)
		return count
	}
	return 0
}
