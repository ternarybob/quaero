// Package api contains API integration tests for the Quaero service.
//
// IMPORTANT: Portfolio assessment tests require extended timeout due to LLM operations:
//
//	go test -timeout 20m -run TestOrchestratorPortfolioAssessmentGoal ./test/api/...
//
// The default Go test timeout (10 minutes) is insufficient for these tests.
// Individual tests use 15-minute timeouts for job completion with error monitoring.
package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Portfolio Assessment Integration Test
// =============================================================================
// Tests the portfolio-assessment-goal template with various configurations.
//
// Template tested: job-templates/portfolio-assessment-goal.toml
// Prerequisite: job-templates/stock-analysis-goal.toml
// Output tag: portfolio-review
//
// WORKFLOW:
// 1. Stock Analysis (prerequisite - produces stock-analysis documents)
// 2. Portfolio Assessment (uses stock-analysis documents)
// 3. Email Report (sends portfolio-review content)
//
// IMPORTANT: This test requires:
// - Valid EODHD API key
// - Valid LLM API key (Gemini or Claude)
// - Extended timeout: go test -timeout 20m
// =============================================================================

// TestOrchestratorPortfolioAssessmentGoal tests the portfolio-assessment-goal template
// which provides portfolio-level analysis including:
// - Concentration analysis
// - Diversification scoring
// - Sector/industry breakdown
// - Dual-horizon portfolio recommendations (Trader vs Super Fund)
func TestOrchestratorPortfolioAssessmentGoal(t *testing.T) {
	testCases := []orchestratorTestCase{
		{
			name:            "MultiPortfolio",
			jobDefFile:      "orchestrator-portfolio-assessment-test.toml",
			jobDefID:        "orchestrator-portfolio-assessment-test",
			expectedTickers: []string{"GNP", "SKS", "WEB", "BCN"},
			outputTag:       "portfolio-review",
			schemaFile:      "portfolio-review.schema.json",
			expectedIndices: []string{"XJO"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runPortfolioAssessmentTest(t, tc)
		})
	}
}

// runPortfolioAssessmentTest executes a single portfolio assessment test scenario
func runPortfolioAssessmentTest(t *testing.T, tc orchestratorTestCase) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 20*time.Minute)

	// Step 1: Load the job definition
	stepStart := time.Now()
	t.Logf("Step 1: Loading job definition %s", tc.jobDefFile)
	err = env.LoadTestJobDefinitions("../config/job-definitions/" + tc.jobDefFile)
	require.NoError(t, err, "Failed to load job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())

	// Step 2: Verify the goal template exists
	t.Log("Step 2: Verifying portfolio-assessment-goal template exists")
	templateResp, err := helper.GET("/api/job-templates/portfolio-assessment-goal")
	require.NoError(t, err, "Failed to check template")
	if templateResp.StatusCode != http.StatusOK {
		t.Skip("portfolio-assessment-goal template not found - skipping test")
	}
	templateResp.Body.Close()

	// Step 3: Trigger the job
	stepStart = time.Now()
	t.Log("Step 3: Triggering portfolio assessment job")
	jobID := executeJobDefinition(t, helper, tc.jobDefID)
	require.NotEmpty(t, jobID, "Job execution should return job ID")
	t.Logf("Triggered job ID: %s", jobID)
	timingData.AddStepTiming("trigger_job", time.Since(stepStart).Seconds())

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 4: Wait for job completion with error monitoring (20 minute timeout for LLM operations)
	// Portfolio assessment requires stock-analysis to complete first, so needs longer timeout
	stepStart = time.Now()
	t.Log("Step 4: Waiting for job completion with error monitoring (timeout: 20 minutes)")
	finalStatus, errorLogs := waitForJobCompletionWithMonitoring(t, helper, jobID, 20*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())

	// Step 5: Handle error logs if any were found
	if len(errorLogs) > 0 {
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
		for i, log := range errorLogs {
			if i < 10 { // Limit output to first 10
				logMsg, _ := log["message"].(string)
				t.Logf("  ERROR[%d]: %s", i, logMsg)
			}
		}

		// If job failed with errors, verify children also failed
		if finalStatus == "failed" || finalStatus == "error" {
			t.Log("Job failed - verifying all children are also failed/stopped")
			assertChildJobsFailedOrStopped(t, helper, jobID)
		}

		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 6: Assert job completed successfully
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")

	// Step 7: Validate stock-analysis documents exist (prerequisite check)
	t.Log("Step 7: Validating stock-analysis documents exist (prerequisite)")
	stockAnalysisDocs := getDocumentsByTag(t, helper, "stock-analysis")
	require.Greater(t, len(stockAnalysisDocs), 0, "Should have stock-analysis documents from prerequisite step")
	t.Logf("PASS: Found %d stock-analysis documents", len(stockAnalysisDocs))

	// Step 8: Validate index data was fetched
	if len(tc.expectedIndices) > 0 {
		t.Logf("Step 8: Validating index data for %v", tc.expectedIndices)
		validateIndexDataFetched(t, helper, tc.expectedIndices)
	}

	// Step 9: Get the portfolio-review output document
	// Find the actual portfolio review document (filter out orchestrator-execution-log)
	t.Logf("Step 9: Retrieving output document with tag '%s'", tc.outputTag)
	docs := getDocumentsByTag(t, helper, tc.outputTag)
	require.Greater(t, len(docs), 0, "Should have at least one document with '%s' tag", tc.outputTag)

	// Filter out orchestrator-execution-log documents to get the actual output
	outputDoc := findOutputDocument(t, docs)
	require.NotNil(t, outputDoc, "Should find a valid output document (not orchestrator-execution-log)")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	// Get document content and metadata
	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))

	// Step 10: Save test output and logs to results directory for verification
	t.Log("Step 10: Saving test output, config, schema, and JSON to results directory")
	resultsDir := saveTestOutput(t, tc.name, jobID, content, env.GetResultsDir())
	saveOrchestratorJobConfig(t, resultsDir, tc.jobDefFile)
	saveSchemaFile(t, resultsDir, tc.schemaFile)
	saveDocumentMetadata(t, resultsDir, metadata)

	// Step 11: Validate portfolio assessment content
	t.Log("Step 11: Validating portfolio assessment content")
	validatePortfolioAssessmentContent(t, content, tc.expectedTickers)

	// Step 12: Validate schema compliance
	validatePortfolioReviewSchema(t, content)

	// Get child job timings and add to timing data
	childTimings := logChildJobTimings(t, helper, jobID)
	for _, wt := range childTimings {
		timingData.WorkerTimings = append(timingData.WorkerTimings, wt)
	}

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Portfolio assessment integration test completed successfully")
}

// validatePortfolioAssessmentContent validates that the output contains expected portfolio analysis
func validatePortfolioAssessmentContent(t *testing.T, content string, expectedTickers []string) {
	// Check for placeholder text
	placeholderTexts := []string{
		"Job completed. No content was specified for this email.",
		"No content was specified",
		"email body is empty",
	}
	for _, placeholder := range placeholderTexts {
		require.NotContains(t, content, placeholder,
			"Content should not contain placeholder text: %s", placeholder)
	}
	t.Log("PASS: Content is not a placeholder")

	// Check for prompt text (shouldn't be in output)
	promptIndicators := []string{
		"=== INPUT REQUIREMENTS ===",
		"=== PHASE 1: GATHER DATA ===",
		"This template requires PRIOR data collection",
	}
	for _, indicator := range promptIndicators {
		require.NotContains(t, content, indicator,
			"Content should not contain prompt text: %s", indicator)
	}
	t.Log("PASS: Content is not the AI prompt")

	// Check for at least one expected ticker
	foundTicker := false
	for _, ticker := range expectedTickers {
		if containsIgnoreCase(content, ticker) {
			foundTicker = true
			t.Logf("PASS: Found ticker '%s' in content", ticker)
			break
		}
	}
	require.True(t, foundTicker, "Content should contain at least one expected ticker: %v", expectedTickers)

	// Check for portfolio-specific content
	portfolioTerms := []string{
		"portfolio", "diversification", "concentration",
		"sector", "quality", "recommendation",
	}
	foundPortfolioTerm := false
	for _, term := range portfolioTerms {
		if containsIgnoreCase(content, term) {
			foundPortfolioTerm = true
			t.Logf("PASS: Found portfolio term '%s' in content", term)
			break
		}
	}
	require.True(t, foundPortfolioTerm, "Content should contain portfolio analysis terms")

	t.Log("PASS: Portfolio assessment content validation complete")
}

// containsIgnoreCase checks if content contains substr (case-insensitive)
func containsIgnoreCase(content, substr string) bool {
	return len(content) > 0 && len(substr) > 0 &&
		(len(content) >= len(substr)) &&
		(indexIgnoreCase(content, substr) >= 0)
}

// indexIgnoreCase returns the index of substr in s (case-insensitive), or -1 if not found
func indexIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldSubstring(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

// equalFoldSubstring checks if two strings are equal (case-insensitive)
func equalFoldSubstring(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
