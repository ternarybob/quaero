// =============================================================================
// Stock Deep Dive Integration Test (Step-Based Workflow)
// =============================================================================
// Tests the stock-deep-dive.toml workflow using traditional step-based orchestration.
//
// WORKFLOW (Step-Based):
// 1. fetch_fundamentals - Fetches stock fundamentals
// 2. fetch_announcements - Fetches company announcements
// 3. fetch_market_data - Fetches price history and technicals
// 4. analyze_competitors - Identifies ASX-listed competitors via LLM
// 5. deep_dive_analysis - AI analysis using Kneppy framework template
// 6. format_output - Formats documents for email delivery (PDF attachment)
// 7. email_report - Sends the analysis report
//
// IMPORTANT: This test requires:
// - Valid EODHD API key in KV storage
// - Valid LLM API key (Gemini or Claude)
// - Extended timeout: go test -timeout 30m
//
// Run with:
//
//	go test -timeout 30m -run TestStockDeepDiveWorkflow ./test/api/portfolio/...
//
// NOTE: This is separate from stock_deep_dive_tools_test.go which tests
// the tool-based orchestrator version of the deep dive workflow.
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

// TestStockDeepDiveWorkflow tests the step-based stock deep dive workflow.
// This validates:
// - All data collection steps execute and create tagged documents
// - Summary step produces Kneppy framework analysis
// - Format and email steps execute successfully
// - Documents have correct tags for pipeline isolation
func TestStockDeepDiveWorkflow(t *testing.T) {
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
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockDeepDiveWorkflow", time.Now().Format(time.RFC3339)))

	// Step 1: Load the job definition
	stepStart := time.Now()
	jobDefFile := "stock-deep-dive-test.toml"
	jobDefID := "stock-deep-dive-test"
	t.Logf("Step 1: Loading job definition %s", jobDefFile)
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition %s", time.Now().Format(time.RFC3339), jobDefFile))

	err = env.LoadTestJobDefinitions("../config/job-definitions/" + jobDefFile)
	require.NoError(t, err, "Failed to load job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition loaded successfully", time.Now().Format(time.RFC3339)))

	// Step 2: Trigger the job
	stepStart = time.Now()
	t.Log("Step 2: Triggering step-based orchestrator job")
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

	// Step 6: Validate deep_dive_data documents exist (from data collection steps)
	t.Log("Step 6: Validating deep_dive_data documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating deep_dive_data documents", time.Now().Format(time.RFC3339)))
	dataDocs := getDocumentsByTag(t, helper, "deep_dive_data")
	require.Greater(t, len(dataDocs), 0, "Should have deep_dive_data documents from data collection steps")
	t.Logf("PASS: Found %d deep_dive_data documents", len(dataDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d deep_dive_data documents", time.Now().Format(time.RFC3339), len(dataDocs)))

	// Step 7: Validate format_output documents exist (from summary step)
	t.Log("Step 7: Validating format_output documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating format_output documents", time.Now().Format(time.RFC3339)))
	formatDocs := getDocumentsByTag(t, helper, "format_output")
	require.Greater(t, len(formatDocs), 0, "Should have format_output documents from summary step")
	t.Logf("PASS: Found %d format_output documents", len(formatDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d format_output documents", time.Now().Format(time.RFC3339), len(formatDocs)))

	// Step 8: Validate email_report documents exist (from format step)
	t.Log("Step 8: Validating email_report documents exist")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 8: Validating email_report documents", time.Now().Format(time.RFC3339)))
	emailDocs := getDocumentsByTag(t, helper, "email_report")
	require.Greater(t, len(emailDocs), 0, "Should have email_report documents from format step")
	t.Logf("PASS: Found %d email_report documents", len(emailDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Found %d email_report documents", time.Now().Format(time.RFC3339), len(emailDocs)))

	// Step 9: Get the output document for content validation
	t.Log("Step 9: Getting output document for content validation")
	outputDoc := findOutputDocument(t, formatDocs)
	require.NotNil(t, outputDoc, "Should find a valid format_output document")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))
	testLog = append(testLog, fmt.Sprintf("[%s] Document content: %d characters", time.Now().Format(time.RFC3339), len(content)))

	// Step 10: Validate Kneppy framework content
	t.Log("Step 10: Validating Kneppy framework content")
	validateKneppyFrameworkContent(t, content)
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Kneppy framework content validated", time.Now().Format(time.RFC3339)))

	// Step 11: Save test outputs
	t.Log("Step 11: Saving test outputs to results directory")

	// Save output.md
	outputPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s (%d bytes)", outputPath, len(content))
	}

	// Save output.json
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
	saveStockDeepDiveJobConfig(t, resultsDir, jobDefFile)

	// Step 12: Verify result files exist
	t.Log("Step 12: Verifying result files were written")
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
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockDeepDiveWorkflow completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Deep Dive Workflow test completed successfully")
}

// validateKneppyFrameworkContent validates that the content contains Kneppy framework analysis
func validateKneppyFrameworkContent(t *testing.T, content string) {
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

	// Check for Kneppy framework content (required)
	contentLower := strings.ToLower(content)

	// Kneppy framework pillars
	kneppyPillars := []string{
		"capital efficiency",
		"share allocation",
		"financial robustness",
		"cash flow",
		"management",
		"moat",
	}
	foundPillars := 0
	for _, pillar := range kneppyPillars {
		if strings.Contains(contentLower, pillar) {
			foundPillars++
			t.Logf("PASS: Found Kneppy pillar '%s' in content", pillar)
		}
	}
	assert.GreaterOrEqual(t, foundPillars, 3,
		"Content should contain at least 3 Kneppy framework pillars (found %d)", foundPillars)

	// Check for quality/rating terms
	ratingTerms := []string{
		"quality", "grade", "rating", "recommendation",
		"roic", "ebitda", "debt",
	}
	foundRating := false
	for _, term := range ratingTerms {
		if strings.Contains(contentLower, term) {
			foundRating = true
			t.Logf("PASS: Found rating term '%s' in content", term)
			break
		}
	}
	assert.True(t, foundRating, "Content should contain quality/rating terms")

	// Check for markdown structure (headers)
	assert.Contains(t, content, "#", "Deep dive must contain markdown headers")

	// Check for the ticker
	assert.True(t, strings.Contains(content, "GNP") || strings.Contains(contentLower, "genusplus"),
		"Content should reference the analyzed ticker GNP or GenusPlus")

	t.Log("PASS: Kneppy framework content validation complete")
}

// saveStockDeepDiveJobConfig saves the job definition TOML file to the results directory
func saveStockDeepDiveJobConfig(t *testing.T, resultsDir string, jobDefFile string) {
	if resultsDir == "" || jobDefFile == "" {
		return
	}

	// Job definitions are in test/config/job-definitions/
	jobDefPath := filepath.Join("..", "config", "job-definitions", jobDefFile)
	content, err := os.ReadFile(jobDefPath)
	if err != nil {
		t.Logf("Warning: Failed to read job definition %s: %v", jobDefFile, err)
		return
	}

	destPath := filepath.Join(resultsDir, "job_definition.toml")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Logf("Warning: Failed to write job definition: %v", err)
		return
	}

	t.Logf("Saved job definition to: %s (%d bytes)", destPath, len(content))
}
