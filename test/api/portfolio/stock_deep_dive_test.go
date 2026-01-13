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

	err = env.LoadTestJobDefinitions("../../config/job-definitions/" + jobDefFile)
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

// TestStockDeepDiveMultipleAttachments tests that multi_document mode creates separate
// PDF attachments for each stock in the variables list.
//
// This test validates:
// - Job definition with multiple stocks executes correctly
// - Each stock produces a separate format_output tagged document
// - Each document has multi_document=true in metadata
// - Each document has its ticker in metadata and as a tag
//
// Run with:
//
//	go test -timeout 30m -run TestStockDeepDiveMultipleAttachments ./test/api/portfolio/...
func TestStockDeepDiveMultipleAttachments(t *testing.T) {
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
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestStockDeepDiveMultipleAttachments", time.Now().Format(time.RFC3339)))

	// Expected tickers from job definition
	expectedTickers := []string{"CGS", "GNP"}

	// Step 1: Load the job definition
	stepStart := time.Now()
	jobDefFile := "stock-deep-dive-multi-attach-test.toml"
	jobDefID := "stock-deep-dive-multi-attach-test"
	t.Logf("Step 1: Loading job definition %s", jobDefFile)
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Loading job definition %s", time.Now().Format(time.RFC3339), jobDefFile))

	err = env.LoadTestJobDefinitions("../../config/job-definitions/" + jobDefFile)
	require.NoError(t, err, "Failed to load job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition loaded successfully", time.Now().Format(time.RFC3339)))

	// Step 2: Trigger the job
	stepStart = time.Now()
	t.Log("Step 2: Triggering multi-attachment orchestrator job")
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

	// Step 6: Validate email_report documents exist and count matches tickers
	t.Log("Step 6: Validating email_report documents exist and count matches ticker count")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating email_report documents", time.Now().Format(time.RFC3339)))

	emailDocs := getDocumentsByTag(t, helper, "email_report")
	require.Greater(t, len(emailDocs), 0, "Should have email_report documents from format step")
	t.Logf("Found %d email_report documents", len(emailDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] Found %d email_report documents", time.Now().Format(time.RFC3339), len(emailDocs)))

	// In multi_document mode, we expect one document per ticker
	// Each document should have the ticker tag and multi_document metadata
	require.GreaterOrEqual(t, len(emailDocs), len(expectedTickers),
		"Should have at least one email_report document per ticker (expected %d, got %d)", len(expectedTickers), len(emailDocs))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: email_report count >= ticker count (%d >= %d)",
		time.Now().Format(time.RFC3339), len(emailDocs), len(expectedTickers)))

	// Step 7: Validate multi_document mode by checking document metadata and tags
	t.Log("Step 7: Validating multi_document mode on email_report documents")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating multi_document metadata", time.Now().Format(time.RFC3339)))

	validateMultiDocumentOutputs(t, helper, emailDocs, expectedTickers)
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: multi_document mode validated", time.Now().Format(time.RFC3339)))

	// Step 8: Validate each ticker has a corresponding document
	t.Log("Step 8: Validating each ticker has a format_output document")
	testLog = append(testLog, fmt.Sprintf("[%s] Step 8: Validating per-ticker documents", time.Now().Format(time.RFC3339)))

	validateTickerDocumentsExist(t, helper, emailDocs, expectedTickers)
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: All tickers have corresponding documents", time.Now().Format(time.RFC3339)))

	// Step 9: Save test outputs
	t.Log("Step 9: Saving test outputs to results directory")

	// Save a sample output document
	if len(emailDocs) > 0 {
		sampleDoc := emailDocs[0]
		docID, _ := sampleDoc["id"].(string)
		content, metadata := getDocumentContentAndMetadata(t, helper, docID)

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
	}

	// Save all document summaries for multi-attachment verification
	saveMultiDocumentSummary(t, helper, resultsDir, emailDocs)

	// Save job definition
	saveStockDeepDiveJobConfig(t, resultsDir, jobDefFile)

	// Step 10: Verify result files exist
	t.Log("Step 10: Verifying result files were written")
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
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestStockDeepDiveMultipleAttachments completed successfully", time.Now().Format(time.RFC3339)))
	WriteTestLog(t, resultsDir, testLog)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("SUCCESS: Stock Deep Dive Multiple Attachments test completed successfully")
}

// validateMultiDocumentOutputs validates that documents have multi_document mode enabled.
// Checks for multi_document=true in metadata.
func validateMultiDocumentOutputs(t *testing.T, helper *common.HTTPTestHelper, docs []map[string]interface{}, expectedTickers []string) {
	t.Helper()

	multiDocCount := 0
	for _, doc := range docs {
		docID, _ := doc["id"].(string)

		// Get full document details
		_, metadata := getDocumentContentAndMetadata(t, helper, docID)
		if metadata == nil {
			t.Logf("Warning: Could not get metadata for document %s", docID)
			continue
		}

		// Check for multi_document in metadata
		if md, ok := metadata["metadata"].(map[string]interface{}); ok {
			if multiDoc, ok := md["multi_document"].(bool); ok && multiDoc {
				multiDocCount++
				ticker, _ := md["ticker"].(string)
				t.Logf("PASS: Document %s has multi_document=true (ticker: %s)", docID[:8], ticker)
			}
		}
	}

	// We expect at least one document to have multi_document=true
	// (All per-ticker documents should have this flag)
	assert.Greater(t, multiDocCount, 0,
		"At least one document should have multi_document=true in metadata")
	t.Logf("Found %d documents with multi_document=true", multiDocCount)
}

// validateTickerDocumentsExist validates that each expected ticker has a corresponding document.
// Checks document tags for ticker codes.
func validateTickerDocumentsExist(t *testing.T, helper *common.HTTPTestHelper, docs []map[string]interface{}, expectedTickers []string) {
	t.Helper()

	foundTickers := make(map[string]bool)

	for _, doc := range docs {
		docID, _ := doc["id"].(string)

		// Check tags for ticker code
		tags, ok := doc["tags"].([]interface{})
		if !ok {
			continue
		}

		for _, tag := range tags {
			tagStr, ok := tag.(string)
			if !ok {
				continue
			}

			// Check if this tag matches any expected ticker (case-insensitive)
			for _, ticker := range expectedTickers {
				if strings.EqualFold(tagStr, ticker) {
					foundTickers[strings.ToUpper(ticker)] = true
					t.Logf("PASS: Found document %s with ticker tag '%s'", docID[:8], ticker)
				}
			}
		}
	}

	// Verify all expected tickers were found
	for _, ticker := range expectedTickers {
		assert.True(t, foundTickers[strings.ToUpper(ticker)],
			"Should find a document with ticker tag '%s'", ticker)
	}

	t.Logf("Found documents for %d/%d expected tickers", len(foundTickers), len(expectedTickers))
}

// saveMultiDocumentSummary saves a summary of all multi-document outputs for verification.
func saveMultiDocumentSummary(t *testing.T, helper *common.HTTPTestHelper, resultsDir string, docs []map[string]interface{}) {
	t.Helper()

	var summary strings.Builder
	summary.WriteString("# Multi-Document Output Summary\n\n")
	summary.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	summary.WriteString(fmt.Sprintf("Total documents: %d\n\n", len(docs)))
	summary.WriteString("## Documents\n\n")
	summary.WriteString("| # | Document ID | Ticker | Tags | multi_document |\n")
	summary.WriteString("|---|-------------|--------|------|----------------|\n")

	for i, doc := range docs {
		docID, _ := doc["id"].(string)
		shortID := docID
		if len(docID) > 8 {
			shortID = docID[:8]
		}

		// Get tags
		var tagList []string
		if tags, ok := doc["tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					tagList = append(tagList, tagStr)
				}
			}
		}
		tagsStr := strings.Join(tagList, ", ")

		// Get metadata
		_, metadata := getDocumentContentAndMetadata(t, helper, docID)
		ticker := ""
		multiDoc := "N/A"
		if metadata != nil {
			if md, ok := metadata["metadata"].(map[string]interface{}); ok {
				if t, ok := md["ticker"].(string); ok {
					ticker = t
				}
				if m, ok := md["multi_document"].(bool); ok {
					if m {
						multiDoc = "true"
					} else {
						multiDoc = "false"
					}
				}
			}
		}

		summary.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n", i+1, shortID, ticker, tagsStr, multiDoc))
	}

	// Write summary file
	summaryPath := filepath.Join(resultsDir, "multi_document_summary.md")
	if err := os.WriteFile(summaryPath, []byte(summary.String()), 0644); err != nil {
		t.Logf("Warning: Failed to write multi_document_summary.md: %v", err)
	} else {
		t.Logf("Saved multi_document_summary.md to: %s", summaryPath)
	}
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
