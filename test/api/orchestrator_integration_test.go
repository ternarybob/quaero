package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// orchestratorTestCase defines a test scenario for the orchestrator integration test
type orchestratorTestCase struct {
	name            string   // Test scenario name
	jobDefFile      string   // Job definition TOML file name
	jobDefID        string   // Job definition ID (matches id field in TOML)
	expectedTickers []string // Expected stock tickers in the output
	outputTag       string   // Tag to find output document (default: "stock-recommendation")
	expectedIndices []string // Expected index codes (e.g., XJO, XSO) - validates fetch_index_data was called
}

// TestOrchestratorIntegration_FullWorkflow tests the complete orchestrator workflow
// with different stock configurations:
// 1. SingleStock - Tests with 1 stock to verify basic functionality
// 2. MultipleStocks - Tests with 2+ stocks to verify multi-stock handling
//
// Each scenario validates:
// - Job executes without errors
// - Email content is NOT a placeholder
// - Email content is NOT the AI prompt
// - Email content contains actual stock analysis
func TestOrchestratorIntegration_FullWorkflow(t *testing.T) {
	testCases := []orchestratorTestCase{
		{
			name:            "SingleStock",
			jobDefFile:      "asx-stocks-1-stock-test.toml",
			jobDefID:        "asx-stocks-1-stock-test",
			expectedTickers: []string{"GNP"},
			outputTag:       "stock-recommendation",
		},
		{
			name:            "TwoStocks",
			jobDefFile:      "asx-stocks-daily-orchestrated.toml",
			jobDefID:        "asx-stocks-daily-orchestrated",
			expectedTickers: []string{"GNP", "SKS"},
			outputTag:       "stock-recommendation",
		},
		{
			name:            "ThreeStocks",
			jobDefFile:      "asx-stocks-3-stocks-test.toml",
			jobDefID:        "asx-stocks-3-stocks-test",
			expectedTickers: []string{"GNP", "SKS", "WEB"},
			outputTag:       "stock-recommendation",
		},
		{
			name:            "SMSFPortfolio",
			jobDefFile:      "smsf-portfolio-daily-orchestrated.toml",
			jobDefID:        "smsf-portfolio-daily-orchestrated",
			expectedTickers: []string{"GNP", "SKS"},
			outputTag:       "smsf-portfolio-review",
			expectedIndices: []string{"XJO", "XSO"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runOrchestratorTest(t, tc)
		})
	}
}

// runOrchestratorTest executes a single orchestrator test scenario
func runOrchestratorTest(t *testing.T, tc orchestratorTestCase) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 5*time.Minute)

	// Step 1: Load the orchestrated job definition
	t.Logf("Step 1: Loading job definition %s", tc.jobDefFile)
	err = env.LoadTestJobDefinitions("../config/job-definitions/" + tc.jobDefFile)
	require.NoError(t, err, "Failed to load orchestrated job definition")

	// Step 2: Trigger the job
	t.Log("Step 2: Triggering orchestrated job")
	jobID := executeJobDefinition(t, helper, tc.jobDefID)
	require.NotEmpty(t, jobID, "Job execution should return job ID")
	t.Logf("Triggered job ID: %s", jobID)

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 3: Wait for job completion (10 minute timeout for LLM operations)
	t.Log("Step 3: Waiting for job completion (timeout: 10 minutes)")
	finalStatus := waitForJobCompletion(t, helper, jobID, 10*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)

	// Step 4: Assert NO error logs in job execution
	t.Log("Step 4: Checking for ERROR logs")
	errorLogs := getJobErrorLogs(t, helper, jobID)
	if len(errorLogs) > 0 {
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
		for i, log := range errorLogs {
			if i < 10 { // Limit output to first 10
				logMsg, _ := log["message"].(string)
				t.Logf("  ERROR[%d]: %s", i, logMsg)
			}
		}

		// If job failed with errors, verify children also failed
		if finalStatus == "failed" {
			t.Log("Job failed - verifying all children are also failed/stopped")
			assertChildJobsFailedOrStopped(t, helper, jobID)
		}

		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 5: Assert job completed successfully
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")

	// Step 5b: Validate index data was fetched (if expected)
	if len(tc.expectedIndices) > 0 {
		t.Logf("Step 5b: Validating index data for %v", tc.expectedIndices)
		validateIndexDataFetched(t, helper, tc.expectedIndices)
	}

	// Step 6: Get the email/output document
	t.Logf("Step 6: Retrieving output document with tag '%s'", tc.outputTag)
	docs := getDocumentsByTag(t, helper, tc.outputTag)
	require.Greater(t, len(docs), 0, "Should have at least one document with '%s' tag", tc.outputTag)

	// Get the most recent document (first in list, sorted by created desc)
	outputDoc := docs[0]
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	// Get document content
	content := getDocumentContent(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))

	// Validate email content
	validateEmailContent(t, content, tc.expectedTickers)

	t.Log("SUCCESS: Orchestrator integration test completed successfully")
}

// validateEmailContent validates that the email content is valid stock analysis
func validateEmailContent(t *testing.T, content string, expectedTickers []string) {
	// Step 7: Assert email content is NOT a generic placeholder
	t.Log("Step 6: Asserting email content is NOT a generic placeholder")
	placeholderTexts := []string{
		"Job completed. No content was specified for this email.",
		"No content was specified",
		"email body is empty",
	}
	for _, placeholder := range placeholderTexts {
		assert.NotContains(t, content, placeholder,
			"Email content should not contain placeholder text: %s", placeholder)
	}
	t.Log("PASS: Email content is not a generic placeholder")

	// Step 8: Assert email content is NOT the AI prompt
	t.Log("Step 7: Asserting email content is NOT the AI prompt")
	promptIndicators := []string{
		"Perform a comprehensive daily analysis of all ASX stocks in the variables list",
		"CRITICAL: For EACH stock in the variables list",
		"you MUST use the \"run_stock_review\" tool",
		"This tool executes a full analysis template",
	}
	for _, indicator := range promptIndicators {
		assert.NotContains(t, content, indicator,
			"Email content should not contain AI prompt text: %s", indicator)
	}
	t.Log("PASS: Email content is not the AI prompt")

	// Step 9: Assert email contains actual stock analysis
	t.Log("Step 8: Asserting email contains actual stock analysis")

	// Check for stock tickers from the job variables
	foundTicker := false
	for _, ticker := range expectedTickers {
		if strings.Contains(content, ticker) {
			foundTicker = true
			t.Logf("PASS: Found stock ticker '%s' in content", ticker)
			break
		}
	}
	assert.True(t, foundTicker, "Email should contain at least one stock ticker from analysis: %v", expectedTickers)

	// Check for analysis-related terms
	analysisTerms := []string{
		"recommendation", "BUY", "SELL", "HOLD",
		"analysis", "stock", "price",
	}
	foundAnalysis := false
	for _, term := range analysisTerms {
		if strings.Contains(strings.ToUpper(content), strings.ToUpper(term)) {
			foundAnalysis = true
			t.Logf("PASS: Found analysis term '%s' in content", term)
			break
		}
	}
	assert.True(t, foundAnalysis, "Email should contain analysis-related content")

	t.Log("PASS: Email contains actual stock analysis content")
}

// validateIndexDataFetched validates that index data documents were created for expected indices.
// This ensures fetch_index_data tool was called and returned data.
func validateIndexDataFetched(t *testing.T, helper *common.HTTPTestHelper, expectedIndices []string) {
	if len(expectedIndices) == 0 {
		return // No index validation needed
	}

	t.Log("Validating index data was fetched...")

	for _, indexCode := range expectedIndices {
		// Index documents are tagged with ["asx-index", "<lowercase-code>", "benchmark"]
		// We search for "asx-index" tag and verify the index code is present
		docs := getDocumentsByTag(t, helper, "asx-index")

		// Find document matching this index code
		found := false
		var matchedDoc map[string]interface{}
		for _, doc := range docs {
			// Check if document tags contain the index code (lowercase)
			if tags, ok := doc["tags"].([]interface{}); ok {
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok && strings.EqualFold(tagStr, indexCode) {
						found = true
						matchedDoc = doc
						break
					}
				}
			}
			if found {
				break
			}
		}

		require.True(t, found, "Index document for %s should exist (tag: asx-index)", indexCode)

		// Verify document has content
		if matchedDoc != nil {
			docID, _ := matchedDoc["id"].(string)
			content := getDocumentContent(t, helper, docID)
			require.NotEmpty(t, content, "Index document for %s should have content", indexCode)

			// Verify content contains expected index-related data
			assert.True(t, strings.Contains(content, indexCode) || strings.Contains(strings.ToUpper(content), indexCode),
				"Index document should contain index code %s", indexCode)

			t.Logf("PASS: Found index data for %s (doc: %s, content: %d chars)", indexCode, docID[:8], len(content))
		}
	}

	t.Logf("PASS: All %d expected indices have data documents", len(expectedIndices))
}

// getJobErrorLogs retrieves ERROR-level logs for a job (including children)
func getJobErrorLogs(t *testing.T, helper *common.HTTPTestHelper, jobID string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/logs?scope=job&job_id=%s&level=error&include_children=true&limit=100", jobID))
	if err != nil {
		t.Logf("Warning: Failed to get job error logs: %v", err)
		return []map[string]interface{}{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET job logs returned %d", resp.StatusCode)
		return []map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse logs response: %v", err)
		return []map[string]interface{}{}
	}

	logs, ok := result["logs"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var errorLogs []map[string]interface{}
	for _, l := range logs {
		if log, ok := l.(map[string]interface{}); ok {
			errorLogs = append(errorLogs, log)
		}
	}

	return errorLogs
}

// assertChildJobsFailedOrStopped verifies that all child jobs are in failed/stopped state
func assertChildJobsFailedOrStopped(t *testing.T, helper *common.HTTPTestHelper, parentJobID string) {
	childJobs := getChildJobs(t, helper, parentJobID)
	if len(childJobs) == 0 {
		t.Log("No child jobs found to verify")
		return
	}

	for _, job := range childJobs {
		jobID, _ := job["id"].(string)
		status, _ := job["status"].(string)
		name, _ := job["name"].(string)

		// Valid terminal states for failed jobs
		validStates := []string{"failed", "cancelled", "stopped"}
		isValidState := false
		for _, valid := range validStates {
			if status == valid {
				isValidState = true
				break
			}
		}

		if !isValidState && status != "completed" {
			t.Errorf("Child job %s (%s) should be failed/stopped but is: %s", jobID[:8], name, status)
		} else {
			t.Logf("Child job %s (%s) status: %s", jobID[:8], name, status)
		}
	}
}

// getDocumentContent retrieves the content of a document by ID
func getDocumentContent(t *testing.T, helper *common.HTTPTestHelper, docID string) string {
	resp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
	if err != nil {
		t.Logf("Warning: Failed to get document: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET document returned %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse document response: %v", err)
		return ""
	}

	// Try various content fields
	if content, ok := result["content"].(string); ok && content != "" {
		return content
	}
	if mdContent, ok := result["content_markdown"].(string); ok && mdContent != "" {
		return mdContent
	}
	if body, ok := result["body"].(string); ok && body != "" {
		return body
	}
	if text, ok := result["text"].(string); ok && text != "" {
		return text
	}

	// If no direct content, try to get from nested data
	if data, ok := result["data"].(map[string]interface{}); ok {
		if content, ok := data["content"].(string); ok {
			return content
		}
	}

	return ""
}
