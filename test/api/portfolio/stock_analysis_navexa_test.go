// =============================================================================
// Stock Analysis Navexa Integration Test
// =============================================================================
// Tests the complete Navexa portfolio stock analysis workflow defined in:
// deployments/common/job-definitions/stock-analysis-navexa.toml
//
// WORKFLOW:
// 1. navexa_portfolio - Fetch portfolio with holdings from Navexa API
// 2. market_data_collection - Deterministic stock data collection
// 3. summary - AI analysis using stock-analysis template
// 4. portfolio_review - AI-generated portfolio review
// 5. email - Send analysis results
//
// IMPORTANT: This test requires:
// - Valid navexa_api_key in KV storage
// - Valid EODHD API key in KV storage
// - Valid LLM API key (Gemini or Claude)
// - Extended timeout: go test -timeout 20m
//
// Run with:
//
//	go test -timeout 20m -run TestStockAnalysisNavexaIntegration ./test/api/portfolio/...
//
// =============================================================================

package portfolio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestStockAnalysisNavexaIntegration tests the complete Navexa portfolio stock analysis workflow.
// This test validates:
// - Navexa API connectivity and portfolio fetching
// - Market data collection for portfolio holdings
// - AI-powered stock analysis generation
// - AI-powered portfolio review generation
// - Email delivery step execution
func TestStockAnalysisNavexaIntegration(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 20*time.Minute)
	resultsDir := common.GetTestResultsDir("orchestrator", t.Name())
	common.EnsureResultsDir(t, resultsDir)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockAnalysisNavexaIntegration", time.Now().Format(time.RFC3339)))

	// Step 0: Check if Navexa API key is configured
	t.Log("Step 0: Checking Navexa API key configuration")
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, resultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping integration test")
	}
	testLog = append(testLog, fmt.Sprintf("[%s] Navexa API key loaded from KV store", time.Now().Format(time.RFC3339)))

	// Step 1: Load the job definition
	stepStart := time.Now()
	jobDefFile := "stock-analysis-navexa.toml"
	jobDefID := "stock-analysis-navexa"
	t.Logf("Step 1: Loading job definition %s", jobDefFile)
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition %s", time.Now().Format(time.RFC3339), jobDefFile))

	err = env.LoadTestJobDefinitions("../config/job-definitions/" + jobDefFile)
	require.NoError(t, err, "Failed to load job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition loaded successfully", time.Now().Format(time.RFC3339)))

	// Step 2: Trigger the job
	stepStart = time.Now()
	t.Log("Step 2: Triggering orchestrated job")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Triggering job execution", time.Now().Format(time.RFC3339)))

	jobID := executeJobDefinition(t, helper, jobDefID)
	require.NotEmpty(t, jobID, "Job execution should return job ID")
	t.Logf("Triggered job ID: %s", jobID)
	testLog = append(testLog, fmt.Sprintf("[%s] Job triggered with ID: %s", time.Now().Format(time.RFC3339), jobID))
	timingData.AddStepTiming("trigger_job", time.Since(stepStart).Seconds())

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 3: Wait for job completion with error monitoring (20 minute timeout for LLM operations)
	// Navexa workflow has multiple LLM steps so needs extended timeout
	stepStart = time.Now()
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 20 minutes)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Waiting for job completion", time.Now().Format(time.RFC3339)))

	finalStatus, errorLogs := WaitForJobCompletionWithMonitoring(t, helper, jobID, 20*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job completed with status: %s", time.Now().Format(time.RFC3339), finalStatus))

	// Step 4: Handle error logs if any were found
	if len(errorLogs) > 0 {
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
		testLog = append(testLog, fmt.Sprintf("[%s] ERROR: Found %d error logs", time.Now().Format(time.RFC3339), len(errorLogs)))
		for i, log := range errorLogs {
			if i < 10 { // Limit output to first 10
				logMsg, _ := log["message"].(string)
				t.Logf("  ERROR[%d]: %s", i, logMsg)
				testLog = append(testLog, fmt.Sprintf("[%s]   ERROR[%d]: %s", time.Now().Format(time.RFC3339), i, logMsg))
			}
		}

		// If job failed with errors, verify children also failed
		if finalStatus == "failed" || finalStatus == "error" {
			t.Log("Job failed - verifying all children are also failed/stopped")
			assertChildJobsFailedOrStopped(t, helper, jobID)
		}

		// Save test log before failing
		WriteTestLog(t, resultsDir, testLog)
		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 5: Assert job completed successfully
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Job completed successfully", time.Now().Format(time.RFC3339)))

	// Step 6: Validate Navexa portfolio document was created
	t.Log("Step 6: Validating navexa-portfolio document exists")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating navexa-portfolio document", time.Now().Format(time.RFC3339)))
	portfolioDocs := getDocumentsByTag(t, helper, "navexa-portfolio")
	require.Greater(t, len(portfolioDocs), 0, "Should have navexa-portfolio documents from step 1")
	t.Logf("PASS: Found %d navexa-portfolio documents", len(portfolioDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d navexa-portfolio documents", time.Now().Format(time.RFC3339), len(portfolioDocs)))

	// Step 7: Validate stock-analysis documents were created
	t.Log("Step 7: Validating stock-analysis documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating stock-analysis documents", time.Now().Format(time.RFC3339)))
	analysisDocs := getDocumentsByTag(t, helper, "stock-analysis")
	// Filter out non-analysis documents (e.g., orchestrator-execution-log)
	filteredAnalysisDocs := filterOutputDocuments(analysisDocs)
	require.Greater(t, len(filteredAnalysisDocs), 0, "Should have stock-analysis documents from step 3")
	t.Logf("PASS: Found %d stock-analysis documents", len(filteredAnalysisDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d stock-analysis documents", time.Now().Format(time.RFC3339), len(filteredAnalysisDocs)))

	// Step 8: Validate portfolio-review document was created
	t.Log("Step 8: Validating portfolio-review document exists")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 8: Validating portfolio-review document", time.Now().Format(time.RFC3339)))
	reviewDocs := getDocumentsByTag(t, helper, "portfolio-review")
	// Filter out non-review documents
	filteredReviewDocs := filterOutputDocuments(reviewDocs)
	require.Greater(t, len(filteredReviewDocs), 0, "Should have portfolio-review document from step 4")
	t.Logf("PASS: Found %d portfolio-review documents", len(filteredReviewDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d portfolio-review documents", time.Now().Format(time.RFC3339), len(filteredReviewDocs)))

	// Step 9: Get the portfolio-review output document for content validation
	outputDoc := findOutputDocument(t, reviewDocs)
	require.NotNil(t, outputDoc, "Should find a valid portfolio-review document")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	// Get document content and metadata
	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))
	testLog = append(testLog, fmt.Sprintf("[%s] Document content: %d characters", time.Now().Format(time.RFC3339), len(content)))

	// Step 10: Save test outputs to results directory
	t.Log("Step 10: Saving test outputs to results directory")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 10: Saving test outputs", time.Now().Format(time.RFC3339)))

	// Save output.md
	outputPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s (%d bytes)", outputPath, len(content))
	}

	// Save output.json (document metadata)
	if metadata != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Warning: Failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))
			}
		}
	}

	// Save job definition
	saveOrchestratorJobConfig(t, resultsDir, jobDefFile)

	// Step 11: Validate portfolio review content
	t.Log("Step 11: Validating portfolio review content")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 11: Validating portfolio review content", time.Now().Format(time.RFC3339)))
	validateNavexaPortfolioReviewContent(t, content)

	// Get child job timings
	childTimings := logChildJobTimings(t, helper, jobID)
	for _, wt := range childTimings {
		timingData.WorkerTimings = append(timingData.WorkerTimings, wt)
	}

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Write test log
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockAnalysisNavexaIntegration completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Analysis Navexa integration test completed successfully")
}

// validateNavexaPortfolioReviewContent validates that the portfolio review content is valid
func validateNavexaPortfolioReviewContent(t *testing.T, content string) {
	t.Helper()

	// Check for placeholder text
	placeholderTexts := []string{
		"Job completed. No content was specified for this email.",
		"No content was specified",
		"email body is empty",
	}
	for _, placeholder := range placeholderTexts {
		assert.NotContains(t, content, placeholder,
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
		assert.NotContains(t, content, indicator,
			"Content should not contain prompt text: %s", indicator)
	}
	t.Log("PASS: Content is not the AI prompt")

	// Check for portfolio-specific content
	contentLower := strings.ToLower(content)
	portfolioTerms := []string{
		"portfolio", "holding", "position",
		"analysis", "review", "recommendation",
	}
	foundPortfolioTerm := false
	for _, term := range portfolioTerms {
		if strings.Contains(contentLower, term) {
			foundPortfolioTerm = true
			t.Logf("PASS: Found portfolio term '%s' in content", term)
			break
		}
	}
	assert.True(t, foundPortfolioTerm, "Content should contain portfolio analysis terms")

	// Check for markdown structure (headers)
	assert.Contains(t, content, "#", "Portfolio review must contain markdown headers")

	t.Log("PASS: Portfolio review content validation complete")
}

// filterOutputDocuments filters out non-output documents like orchestrator-execution-log
func filterOutputDocuments(docs []map[string]interface{}) []map[string]interface{} {
	var filtered []map[string]interface{}
	for _, doc := range docs {
		sourceType, _ := doc["source_type"].(string)
		if sourceType != "orchestrator-execution-log" && sourceType != "orchestrator_execution_log" {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// =============================================================================
// TestStockAnalysisNavexaWithFormatAndEmail - Full Pipeline with Format Step
// =============================================================================
// This test validates the complete Navexa workflow including:
// - Navexa portfolio fetching
// - Market data collection
// - AI stock analysis
// - Portfolio review generation
// - Output formatting (PDF attachment)
// - Email delivery
//
// Run with:
//
//	go test -timeout 30m -run TestStockAnalysisNavexaWithFormatAndEmail ./test/api/portfolio/...
//
// =============================================================================

// TestStockAnalysisNavexaWithFormatAndEmail tests the complete pipeline with format and email steps.
// This is a sequential test that validates the full workflow including document formatting.
func TestStockAnalysisNavexaWithFormatAndEmail(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 30*time.Minute)
	resultsDir := common.GetTestResultsDir("orchestrator", t.Name())
	common.EnsureResultsDir(t, resultsDir)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockAnalysisNavexaWithFormatAndEmail", time.Now().Format(time.RFC3339)))

	// Step 0: Check if Navexa API key is configured
	t.Log("Step 0: Checking Navexa API key configuration")
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, resultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping integration test")
	}
	testLog = append(testLog, fmt.Sprintf("[%s] Navexa API key loaded from KV store", time.Now().Format(time.RFC3339)))

	// Step 1: Load the job definition with format step
	stepStart := time.Now()
	jobDefFile := "stock-analysis-navexa-with-format.toml"
	jobDefID := "stock-analysis-navexa-with-format"
	t.Logf("Step 1: Loading job definition %s", jobDefFile)
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition %s", time.Now().Format(time.RFC3339), jobDefFile))

	err = env.LoadTestJobDefinitions("../config/job-definitions/" + jobDefFile)
	require.NoError(t, err, "Failed to load job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition loaded successfully", time.Now().Format(time.RFC3339)))

	// Step 2: Trigger the job
	stepStart = time.Now()
	t.Log("Step 2: Triggering orchestrated job")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Triggering job execution", time.Now().Format(time.RFC3339)))

	jobID := executeJobDefinition(t, helper, jobDefID)
	require.NotEmpty(t, jobID, "Job execution should return job ID")
	t.Logf("Triggered job ID: %s", jobID)
	testLog = append(testLog, fmt.Sprintf("[%s] Job triggered with ID: %s", time.Now().Format(time.RFC3339), jobID))
	timingData.AddStepTiming("trigger_job", time.Since(stepStart).Seconds())

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 3: Wait for job completion with error monitoring (30 minute timeout)
	stepStart = time.Now()
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 30 minutes)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Waiting for job completion", time.Now().Format(time.RFC3339)))

	finalStatus, errorLogs := WaitForJobCompletionWithMonitoring(t, helper, jobID, 30*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job completed with status: %s", time.Now().Format(time.RFC3339), finalStatus))

	// Step 4: Handle error logs if any were found
	if len(errorLogs) > 0 {
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
		testLog = append(testLog, fmt.Sprintf("[%s] ERROR: Found %d error logs", time.Now().Format(time.RFC3339), len(errorLogs)))
		for i, log := range errorLogs {
			if i < 10 {
				logMsg, _ := log["message"].(string)
				t.Logf("  ERROR[%d]: %s", i, logMsg)
				testLog = append(testLog, fmt.Sprintf("[%s]   ERROR[%d]: %s", time.Now().Format(time.RFC3339), i, logMsg))
			}
		}

		if finalStatus == "failed" || finalStatus == "error" {
			t.Log("Job failed - verifying all children are also failed/stopped")
			assertChildJobsFailedOrStopped(t, helper, jobID)
		}

		WriteTestLog(t, resultsDir, testLog)
		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 5: Assert job completed successfully
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Job completed successfully", time.Now().Format(time.RFC3339)))

	// Step 6: Validate Navexa portfolio document was created
	t.Log("Step 6: Validating navexa-portfolio document exists")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating navexa-portfolio document", time.Now().Format(time.RFC3339)))
	portfolioDocs := getDocumentsByTag(t, helper, "navexa-portfolio")
	require.Greater(t, len(portfolioDocs), 0, "Should have navexa-portfolio documents")
	t.Logf("PASS: Found %d navexa-portfolio documents", len(portfolioDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d navexa-portfolio documents", time.Now().Format(time.RFC3339), len(portfolioDocs)))

	// Step 7: Validate stock-analysis documents were created
	t.Log("Step 7: Validating stock-analysis documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating stock-analysis documents", time.Now().Format(time.RFC3339)))
	analysisDocs := getDocumentsByTag(t, helper, "stock-analysis")
	filteredAnalysisDocs := filterOutputDocuments(analysisDocs)
	require.Greater(t, len(filteredAnalysisDocs), 0, "Should have stock-analysis documents")
	t.Logf("PASS: Found %d stock-analysis documents", len(filteredAnalysisDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d stock-analysis documents", time.Now().Format(time.RFC3339), len(filteredAnalysisDocs)))

	// Step 8: Validate portfolio-review document was created
	t.Log("Step 8: Validating portfolio-review document exists")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 8: Validating portfolio-review document", time.Now().Format(time.RFC3339)))
	reviewDocs := getDocumentsByTag(t, helper, "portfolio-review")
	filteredReviewDocs := filterOutputDocuments(reviewDocs)
	require.Greater(t, len(filteredReviewDocs), 0, "Should have portfolio-review document")
	t.Logf("PASS: Found %d portfolio-review documents", len(filteredReviewDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d portfolio-review documents", time.Now().Format(time.RFC3339), len(filteredReviewDocs)))

	// Step 9: Validate format_output (email_report) document was created
	t.Log("Step 9: Validating email_report document exists (from format_output)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 9: Validating email_report document", time.Now().Format(time.RFC3339)))
	emailDocs := getDocumentsByTag(t, helper, "email_report")
	require.Greater(t, len(emailDocs), 0, "Should have email_report document from format_output step")
	t.Logf("PASS: Found %d email_report documents", len(emailDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d email_report documents", time.Now().Format(time.RFC3339), len(emailDocs)))

	// Step 10: Get the email_report document for content validation
	outputDoc := findOutputDocument(t, emailDocs)
	require.NotNil(t, outputDoc, "Should find a valid email_report document")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))
	testLog = append(testLog, fmt.Sprintf("[%s] Document content: %d characters", time.Now().Format(time.RFC3339), len(content)))

	// Step 11: Save test outputs to results directory
	t.Log("Step 11: Saving test outputs to results directory")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 11: Saving test outputs", time.Now().Format(time.RFC3339)))

	outputPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s (%d bytes)", outputPath, len(content))
	}

	if metadata != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Warning: Failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))
			}
		}
	}

	saveOrchestratorJobConfig(t, resultsDir, jobDefFile)

	// Step 12: Validate formatted output has email instructions
	t.Log("Step 12: Validating formatted output content")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 12: Validating formatted output content", time.Now().Format(time.RFC3339)))

	// Check for email frontmatter (format_output adds YAML-like instructions)
	assert.Contains(t, content, "format:", "Formatted output should contain email format instructions")
	t.Log("PASS: Found email format instructions in output")

	// Get child job timings
	childTimings := logChildJobTimings(t, helper, jobID)
	for _, wt := range childTimings {
		timingData.WorkerTimings = append(timingData.WorkerTimings, wt)
	}

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Write test log
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockAnalysisNavexaWithFormatAndEmail completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Analysis Navexa with Format and Email test completed successfully")
}
