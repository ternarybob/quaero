// =============================================================================
// Stock Deep Dive Tools Integration Test
// =============================================================================
// Tests the stock-deep-dive-tools.toml workflow with orchestrator tool-calling.
//
// WORKFLOW:
// 1. orchestrate_analysis - LLM plans and executes tool calls for data collection
// 2. format_output - Formats documents for email delivery (PDF attachment)
// 3. email_report - Sends the analysis report
//
// IMPORTANT: This test requires:
// - Valid EODHD API key in KV storage
// - Valid LLM API key (Gemini or Claude)
// - Extended timeout: go test -timeout 30m
//
// Run with:
//
//	go test -timeout 30m -run TestStockDeepDiveToolsOrchestration ./test/api/portfolio/...
//	go test -timeout 30m -run TestStockDeepDiveToolsFullPipeline ./test/api/portfolio/...
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

// TestStockDeepDiveToolsOrchestration tests just the orchestrator step.
// This validates:
// - LLM tool calling pattern works correctly
// - Documents are created with correct tags (stock-deep-dive-report)
// - Documents have the managerID in their Jobs field for pipeline isolation
func TestStockDeepDiveToolsOrchestration(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 30*time.Minute)
	resultsDir := env.GetResultsDir()

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockDeepDiveToolsOrchestration", time.Now().Format(time.RFC3339)))

	// Step 1: Load the job definition
	stepStart := time.Now()
	jobDefFile := "stock-deep-dive-tools-orchestration-test.toml"
	jobDefID := "stock-deep-dive-tools-orchestration-test"
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

	// Step 3: Wait for job completion with error monitoring
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

	// Step 6: Validate stock-deep-dive-report documents exist
	t.Log("Step 6: Validating stock-deep-dive-report documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating stock-deep-dive-report documents", time.Now().Format(time.RFC3339)))
	reportDocs := getDocumentsByTag(t, helper, "stock-deep-dive-report")
	filteredDocs := filterOutputDocuments(reportDocs)
	require.Greater(t, len(filteredDocs), 0, "Should have stock-deep-dive-report documents from orchestrator")
	t.Logf("PASS: Found %d stock-deep-dive-report documents", len(filteredDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d stock-deep-dive-report documents", time.Now().Format(time.RFC3339), len(filteredDocs)))

	// Step 7: Validate deep_dive_data documents exist (from tool calls)
	t.Log("Step 7: Validating deep_dive_data documents exist")
	dataDocs := getDocumentsByTag(t, helper, "deep_dive_data")
	require.Greater(t, len(dataDocs), 0, "Should have deep_dive_data documents from tool calls")
	t.Logf("PASS: Found %d deep_dive_data documents", len(dataDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d deep_dive_data documents", time.Now().Format(time.RFC3339), len(dataDocs)))

	// Step 8: Get the output document for content validation
	outputDoc := findOutputDocument(t, reportDocs)
	require.NotNil(t, outputDoc, "Should find a valid stock-deep-dive-report document")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))
	testLog = append(testLog, fmt.Sprintf("[%s] Document content: %d characters", time.Now().Format(time.RFC3339), len(content)))

	// Step 9: Save test outputs
	t.Log("Step 9: Saving test outputs to results directory")
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

	// Step 10: Validate content
	t.Log("Step 10: Validating deep dive content")
	validateDeepDiveContent(t, content)

	// Step 11: Assert result files exist
	t.Log("Step 11: Verifying result files were written")
	AssertResultFilesExist(t, resultsDir)

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
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockDeepDiveToolsOrchestration completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Deep Dive Tools Orchestration test completed successfully")
}

// TestStockDeepDiveToolsFullPipeline tests the full pipeline including format and email.
// This validates:
// - Orchestrator produces documents with correct tags
// - format_output finds documents and creates email-ready output
// - email step executes successfully
func TestStockDeepDiveToolsFullPipeline(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 30*time.Minute)
	resultsDir := env.GetResultsDir()

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockDeepDiveToolsFullPipeline", time.Now().Format(time.RFC3339)))

	// Step 1: Load the job definition
	stepStart := time.Now()
	jobDefFile := "stock-deep-dive-tools-full-test.toml"
	jobDefID := "stock-deep-dive-tools-full-test"
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

	// Step 3: Wait for job completion
	stepStart = time.Now()
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 30 minutes)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Waiting for job completion", time.Now().Format(time.RFC3339)))

	finalStatus, errorLogs := WaitForJobCompletionWithMonitoring(t, helper, jobID, 30*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job completed with status: %s", time.Now().Format(time.RFC3339), finalStatus))

	// Step 4: Handle error logs
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
			assertChildJobsFailedOrStopped(t, helper, jobID)
		}

		WriteTestLog(t, resultsDir, testLog)
		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 5: Assert job completed successfully
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Job completed successfully", time.Now().Format(time.RFC3339)))

	// Step 6: Validate stock-deep-dive-report documents exist
	t.Log("Step 6: Validating stock-deep-dive-report documents exist")
	reportDocs := getDocumentsByTag(t, helper, "stock-deep-dive-report")
	filteredDocs := filterOutputDocuments(reportDocs)
	require.Greater(t, len(filteredDocs), 0, "Should have stock-deep-dive-report documents")
	t.Logf("PASS: Found %d stock-deep-dive-report documents", len(filteredDocs))

	// Step 7: Validate email_report documents exist (from format_output)
	t.Log("Step 7: Validating email_report documents exist")
	emailDocs := getDocumentsByTag(t, helper, "email_report")
	require.Greater(t, len(emailDocs), 0, "Should have email_report documents from format_output")
	t.Logf("PASS: Found %d email_report documents", len(emailDocs))

	// Step 8: Get the email output document for validation
	outputDoc := findOutputDocument(t, emailDocs)
	require.NotNil(t, outputDoc, "Should find a valid email_report document")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found email output document: %s", docID)

	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))

	// Step 9: Save test outputs
	t.Log("Step 9: Saving test outputs to results directory")
	outputPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	}

	if metadata != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			os.WriteFile(jsonPath, data, 0644)
		}
	}

	saveOrchestratorJobConfig(t, resultsDir, jobDefFile)

	// Assert result files exist
	t.Log("Verifying result files were written")
	AssertResultFilesExist(t, resultsDir)

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
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockDeepDiveToolsFullPipeline completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Deep Dive Tools Full Pipeline test completed successfully")
}

// validateDeepDiveContent validates that the deep dive content is valid
func validateDeepDiveContent(t *testing.T, content string) {
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
		"=== PHASE 1: DATA COLLECTION ===",
		"=== DOCUMENT TAGGING (IMPORTANT) ===",
		"CRITICAL REQUIREMENTS FOR ANALYSIS",
	}
	for _, indicator := range promptIndicators {
		assert.NotContains(t, content, indicator,
			"Content should not contain prompt text: %s", indicator)
	}
	t.Log("PASS: Content is not the AI prompt")

	// Check for Kneppy framework content
	contentLower := strings.ToLower(content)
	kneppyTerms := []string{
		"capital efficiency", "roic", "financial robustness",
		"cash flow", "management", "moat",
		"quality", "grade", "recommendation",
	}
	foundKneppyTerm := false
	for _, term := range kneppyTerms {
		if strings.Contains(contentLower, term) {
			foundKneppyTerm = true
			t.Logf("PASS: Found Kneppy framework term '%s' in content", term)
			break
		}
	}
	assert.True(t, foundKneppyTerm, "Content should contain Kneppy framework analysis terms")

	// Check for markdown structure (headers)
	assert.Contains(t, content, "#", "Deep dive must contain markdown headers")

	t.Log("PASS: Deep dive content validation complete")
}
