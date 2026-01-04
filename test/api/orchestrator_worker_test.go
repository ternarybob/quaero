package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// TestOrchestratorWorkerSubmission - Integration test for orchestrator worker
// =============================================================================
// This test validates the full orchestrator workflow:
// 1. Load job definition that references a goal template
// 2. Execute the job via API
// 3. Wait for completion with error monitoring
// 4. Verify output document was created with actual content
//
// IMPORTANT: Requires 15+ minute timeout due to LLM operations:
//
//	go test -timeout 20m -run "^TestOrchestratorWorkerSubmission$" ./test/api/...
//
// Job definition: test/config/job-definitions/orchestrator-worker-test.toml
// Workflow: hybrid (stock_data_collection → summary with embedded template)
// Output tag: stock-analysis
// =============================================================================

func TestOrchestratorWorkerSubmission(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 15*time.Minute)
	resultsDir := env.GetResultsDir()

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestOrchestratorWorkerSubmission", time.Now().Format(time.RFC3339)))

	// Step 1: Load the orchestrated job definition
	stepStart := time.Now()
	jobDefFile := "orchestrator-worker-test.toml"
	jobDefID := "orchestrator-worker-test"
	t.Logf("Step 1: Loading job definition %s", jobDefFile)
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition %s", time.Now().Format(time.RFC3339), jobDefFile))

	err = env.LoadTestJobDefinitions("../config/job-definitions/" + jobDefFile)
	require.NoError(t, err, "Failed to load orchestrated job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())

	// Cleanup job definition after test
	defer func() {
		resp, _ := helper.DELETE("/api/job-definitions/" + jobDefID)
		if resp != nil {
			resp.Body.Close()
		}
	}()

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

	// Step 3: Wait for job completion with error monitoring (15 minute timeout for LLM operations)
	stepStart = time.Now()
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 15 minutes)")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Waiting for job completion", time.Now().Format(time.RFC3339)))

	finalStatus, errorLogs := waitForJobCompletionWithMonitoring(t, helper, jobID, 15*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())

	// Step 4: Handle error logs if any were found
	if len(errorLogs) > 0 {
		testLog = append(testLog, fmt.Sprintf("[%s] ERROR: Found %d error logs", time.Now().Format(time.RFC3339), len(errorLogs)))
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
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
		writeOrchestratorWorkerTestLog(t, resultsDir, testLog)
		t.Fatalf("FAIL: Job execution produced %d ERROR logs. Job status: %s", len(errorLogs), finalStatus)
	}
	t.Log("PASS: No ERROR logs found in job execution")

	// Step 5: Assert job completed successfully
	testLog = append(testLog, fmt.Sprintf("[%s] Job reached terminal status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "Job should complete successfully")

	// Step 6: Get the output document
	stepStart = time.Now()
	t.Log("Step 6: Retrieving output document with tag 'stock-analysis'")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Retrieving output documents", time.Now().Format(time.RFC3339)))

	docs := getDocumentsByTag(t, helper, "stock-analysis")
	require.Greater(t, len(docs), 0, "Should have at least one document with 'stock-analysis' tag")
	t.Logf("Found %d documents with 'stock-analysis' tag", len(docs))

	// Filter out orchestrator-execution-log documents to get the actual analysis output
	outputDoc := findOutputDocument(t, docs)
	require.NotNil(t, outputDoc, "Should find a valid output document (not orchestrator-execution-log)")

	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)
	testLog = append(testLog, fmt.Sprintf("[%s] Found output document: %s", time.Now().Format(time.RFC3339), docID))
	timingData.AddStepTiming("get_output", time.Since(stepStart).Seconds())

	// Step 7: Get document content and validate
	t.Log("Step 7: Getting document content")
	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))
	testLog = append(testLog, fmt.Sprintf("[%s] Document content: %d characters", time.Now().Format(time.RFC3339), len(content)))

	// Step 7a: Verify source document count includes stock data (not just announcements)
	// This is a dead-man check to ensure the stock collector document is properly tagged
	if sourceDocCount, ok := metadata["source_document_count"].(float64); ok {
		t.Logf("Source document count: %.0f", sourceDocCount)
		testLog = append(testLog, fmt.Sprintf("[%s] Source document count: %.0f", time.Now().Format(time.RFC3339), sourceDocCount))
		assert.GreaterOrEqual(t, int(sourceDocCount), 5,
			"source_document_count should include index (1) + stock_collector (1) + announcements (3+); got %.0f - this indicates stock data tagging may be broken", sourceDocCount)
	} else {
		t.Log("Warning: source_document_count not found in metadata")
	}

	// Validate content is NOT a placeholder or error
	validateOrchestratorOutputContent(t, content)

	// Step 8: Save all results
	stepStart = time.Now()
	t.Log("Step 8: Saving results to results directory")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 8: Saving results", time.Now().Format(time.RFC3339)))

	// Get structured results directory
	structuredDir := common.GetTestResultsDir("orchestrator", t.Name())
	if err := os.MkdirAll(structuredDir, 0755); err != nil {
		t.Logf("Warning: Failed to create structured results directory: %v", err)
		structuredDir = resultsDir // Fallback
	}

	// Save output.md
	outputPath := filepath.Join(structuredDir, "output.md")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s (%d bytes)", outputPath, len(content))
	}

	// Save output.json (document metadata)
	if metadata != nil {
		jsonPath := filepath.Join(structuredDir, "output.json")
		if data, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Warning: Failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))
			}
		}
	}

	// Save job output
	saveOrchestratorWorkerJobOutput(t, helper, structuredDir, jobID)

	// Save job definition used
	saveOrchestratorWorkerJobDefinition(t, structuredDir, jobDefFile)

	// Copy logs from env results directory
	copyLogsToResultsDir(t, resultsDir, structuredDir)

	timingData.AddStepTiming("save_results", time.Since(stepStart).Seconds())

	// Get child job timings
	childTimings := logChildJobTimings(t, helper, jobID)
	for _, wt := range childTimings {
		timingData.WorkerTimings = append(timingData.WorkerTimings, wt)
	}

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, structuredDir, timingData)

	// Save timing summary as markdown
	saveOrchestratorWorkerTimingSummary(t, structuredDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Write test log
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestOrchestratorWorkerSubmission completed successfully", time.Now().Format(time.RFC3339)))
	writeOrchestratorWorkerTestLog(t, structuredDir, testLog)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, structuredDir)

	t.Log("SUCCESS: Orchestrator worker test completed successfully")
}

// =============================================================================
// Helper Functions
// =============================================================================

// validateOrchestratorOutputContent validates that the output content is actual analysis, not an error or placeholder
func validateOrchestratorOutputContent(t *testing.T, content string) {
	t.Helper()

	// First check: content must not be empty or blank
	trimmedContent := strings.TrimSpace(content)
	require.NotEmpty(t, trimmedContent, "Output content is empty or blank")

	// Check for error indicators
	errorIndicators := []string{
		"no tools are available",
		"planning failed",
		"Cannot achieve goal",
		"ERROR:",
		"Job completed. No content was specified",
	}

	contentLower := strings.ToLower(content)
	for _, indicator := range errorIndicators {
		require.False(t, strings.Contains(contentLower, strings.ToLower(indicator)),
			"Output content contains error indicator: %s", indicator)
	}

	// Check for analysis content indicators
	analysisIndicators := []string{
		"stock", "analysis", "recommendation", "price",
		"quality", "buy", "sell", "hold",
	}

	foundAnalysis := false
	for _, indicator := range analysisIndicators {
		if strings.Contains(contentLower, indicator) {
			foundAnalysis = true
			break
		}
	}

	require.True(t, foundAnalysis, "Output should contain analysis-related content")
	t.Logf("PASS: Output content contains valid analysis (%d bytes)", len(content))
}

// saveOrchestratorWorkerJobOutput saves job output JSON to results directory
func saveOrchestratorWorkerJobOutput(t *testing.T, helper *common.HTTPTestHelper, resultsDir, jobID string) {
	if resultsDir == "" {
		return
	}

	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Logf("Warning: Failed to get job %s: %v", jobID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var job map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		return
	}

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(resultsDir, "job_output.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Logf("Warning: Failed to save job_output.json: %v", err)
	} else {
		t.Logf("Saved job output to: %s", path)
	}

	// Also save metadata separately
	if metadata, ok := job["metadata"].(map[string]interface{}); ok {
		if metaData, err := json.MarshalIndent(metadata, "", "  "); err == nil {
			metaPath := filepath.Join(resultsDir, "job_metadata.json")
			os.WriteFile(metaPath, metaData, 0644)
		}
	}
}

// saveOrchestratorWorkerJobDefinition saves the job definition TOML to results directory
func saveOrchestratorWorkerJobDefinition(t *testing.T, resultsDir, jobDefFile string) {
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

// copyLogsToResultsDir copies service.log and test.log to results directory
func copyLogsToResultsDir(t *testing.T, srcDir, destDir string) {
	logs := []string{"service.log", "test.log"}
	for _, logFile := range logs {
		srcPath := filepath.Join(srcDir, logFile)
		content, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}
		destPath := filepath.Join(destDir, logFile)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			t.Logf("Warning: Failed to copy %s: %v", logFile, err)
		}
	}
}

// saveOrchestratorWorkerTimingSummary saves timing data as markdown summary
func saveOrchestratorWorkerTimingSummary(t *testing.T, resultsDir string, timing *common.TestTimingData) {
	if resultsDir == "" || timing == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("# Test Timing Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Test:** %s\n\n", timing.TestName))
	sb.WriteString(fmt.Sprintf("**Start:** %s\n\n", timing.StartTime))
	sb.WriteString(fmt.Sprintf("**End:** %s\n\n", timing.EndTime))
	sb.WriteString(fmt.Sprintf("**Total Duration:** %s (%.2f seconds)\n\n", timing.TotalDuration, timing.TotalSeconds))

	if len(timing.StepTimings) > 0 {
		sb.WriteString("## Step Timings\n\n")
		sb.WriteString("| Step | Duration |\n")
		sb.WriteString("|------|----------|\n")
		for _, step := range timing.StepTimings {
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", step.StepName, step.DurationFormatted))
		}
		sb.WriteString("\n")
	}

	if len(timing.WorkerTimings) > 0 {
		sb.WriteString("## Worker Timings\n\n")
		sb.WriteString("| Worker | Type | Duration | Status |\n")
		sb.WriteString("|--------|------|----------|--------|\n")
		for _, worker := range timing.WorkerTimings {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				worker.Name, worker.WorkerType, worker.DurationFormatted, worker.Status))
		}
		sb.WriteString("\n")
	}

	path := filepath.Join(resultsDir, "timing_summary.md")
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Logf("Warning: Failed to save timing_summary.md: %v", err)
	} else {
		t.Logf("Saved timing summary to: %s", path)
	}
}

// writeOrchestratorWorkerTestLog writes test progress to test_log.md file
func writeOrchestratorWorkerTestLog(t *testing.T, resultsDir string, entries []string) {
	if resultsDir == "" {
		return
	}

	logPath := filepath.Join(resultsDir, "test_log.md")
	content := "# Test Execution Log\n\n" + strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write test_log.md: %v", err)
	}
}
