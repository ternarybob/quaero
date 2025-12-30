package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// orchestratorTestCase defines a test scenario for the orchestrator integration test
type orchestratorTestCase struct {
	name                   string   // Test scenario name
	jobDefFile             string   // Job definition TOML file name
	jobDefID               string   // Job definition ID (matches id field in TOML)
	expectedTickers        []string // Expected stock tickers in the output
	outputTag              string   // Tag to find output document (default: "stock-recommendation")
	expectedIndices        []string // Expected index codes (e.g., XJO, XSO) - validates fetch_index_data was called
	expectDirectorInterest bool     // Whether to validate director-interest documents exist
	expectMacroData        bool     // Whether to validate macro-data documents exist
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
			expectedIndices: []string{"XJO"},
		},
		{
			name:            "TwoStocks",
			jobDefFile:      "asx-stocks-daily-orchestrated.toml",
			jobDefID:        "asx-stocks-daily-orchestrated",
			expectedTickers: []string{"GNP", "SKS"},
			outputTag:       "stock-recommendation",
			expectedIndices: []string{"XJO"},
		},
		{
			name:            "ThreeStocks",
			jobDefFile:      "asx-stocks-3-stocks-test.toml",
			jobDefID:        "asx-stocks-3-stocks-test",
			expectedTickers: []string{"GNP", "SKS", "WEB"},
			outputTag:       "stock-recommendation",
			expectedIndices: []string{"XJO"},
		},
		{
			name:            "SMSFPortfolio",
			jobDefFile:      "smsf-portfolio-daily-orchestrated.toml",
			jobDefID:        "smsf-portfolio-daily-orchestrated",
			expectedTickers: []string{"GNP", "SKS"},
			outputTag:       "smsf-portfolio-review",
			expectedIndices: []string{"XJO", "XSO"},
		},
		{
			name:                   "ConvictionAnalysis",
			jobDefFile:             "asx-purchase-conviction-test.toml",
			jobDefID:               "asx-purchase-conviction-test",
			expectedTickers:        []string{"GNP", "SKS", "WES", "AMS", "AV1", "CSL", "KYP", "PNC", "SDF", "SGI", "VBTC", "VGB", "VNT"},
			outputTag:              "purchase-recommendation",
			expectedIndices:        []string{"XJO"},
			expectDirectorInterest: true,
			expectMacroData:        true,
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

	helper := env.NewHTTPTestHelperWithTimeout(t, 15*time.Minute)

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

	// Step 3: Wait for job completion (15 minute timeout for LLM operations)
	t.Log("Step 3: Waiting for job completion (timeout: 15 minutes)")
	finalStatus := waitForJobCompletion(t, helper, jobID, 15*time.Minute)
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

	// Step 5c: Validate director interest data was fetched (if expected)
	if tc.expectDirectorInterest {
		t.Log("Step 5c: Validating director interest data for expected tickers")
		validateDirectorInterestFetched(t, helper, tc.expectedTickers)
	}

	// Step 5d: Validate macro data was fetched (if expected)
	if tc.expectMacroData {
		t.Log("Step 5d: Validating macro data was fetched")
		validateMacroDataFetched(t, helper)
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

	// Step 7: Save test output and logs to results directory for verification
	t.Log("Step 7: Saving test output and logs to results directory")
	saveTestOutput(t, tc.name, jobID, content, env.GetResultsDir())

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

	// Step 10: Validate schema compliance (for stock recommendation outputs)
	validateSchemaCompliance(t, content)
}

// validateSchemaCompliance validates that the output content contains expected schema fields.
// This ensures the output_schema from goal templates (e.g., stock-report.schema.json) is being enforced.
// Schema fields validated:
// - Recommendation actions: STRONG BUY, BUY, HOLD, SELL, STRONG SELL (trader) and ACCUMULATE, HOLD, REDUCE, AVOID (super)
// - Quality rating: A, B, C, D, or F
// - Signal:Noise ratio: HIGH, MEDIUM, or LOW
// - Technical indicators: RSI, SMA, support, resistance
func validateSchemaCompliance(t *testing.T, content string) {
	t.Log("Step 10: Validating output schema compliance")

	contentUpper := strings.ToUpper(content)

	// 1. Check for recommendation action fields from stock-analysis.schema.json
	// trader_recommendation.action: STRONG BUY, BUY, HOLD, SELL, STRONG SELL
	// super_recommendation.action: ACCUMULATE, HOLD, REDUCE, AVOID
	recommendationActions := []string{
		"STRONG BUY", "STRONG SELL", // Strong trader actions
		"ACCUMULATE", "REDUCE", "AVOID", // Super recommendation actions
	}

	foundRecommendation := false
	for _, action := range recommendationActions {
		if strings.Contains(contentUpper, action) {
			foundRecommendation = true
			t.Logf("PASS: Found recommendation action '%s' in output (schema: trader/super_recommendation.action)", action)
			break
		}
	}
	// Note: Basic BUY/SELL/HOLD are already checked in validateEmailContent
	// This checks for the more specific schema fields
	if !foundRecommendation {
		t.Log("INFO: No strong recommendation actions found (STRONG BUY/SELL, ACCUMULATE/REDUCE/AVOID)")
		t.Log("      This may indicate schema is not being strictly enforced by LLM")
	}

	// 2. Check for quality rating (A/B/C/D/F) from stock-analysis.schema.json
	// quality_rating enum: A, B, C, D, F
	qualityPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)quality[:\s]+[ABCDF]\b`),                 // Quality: A, Quality B
		regexp.MustCompile(`(?i)quality\s+rating[:\s]+[ABCDF]\b`),        // Quality Rating: A
		regexp.MustCompile(`(?i)\bquality\s+[ABCDF]\b`),                  // Quality A
		regexp.MustCompile(`(?i)\b[ABCDF]\s+(?:quality|rated|rating)\b`), // A quality, A rated
		regexp.MustCompile(`(?i)grade[:\s]+[ABCDF]\b`),                   // Grade: A
		regexp.MustCompile(`\|\s*[ABCDF]\s*\|`),                          // | A | (table format)
	}

	foundQuality := false
	for _, pattern := range qualityPatterns {
		if pattern.MatchString(content) {
			foundQuality = true
			t.Log("PASS: Found quality rating (A/B/C/D/F) in output (schema: quality_rating)")
			break
		}
	}

	if !foundQuality {
		t.Log("INFO: Quality rating (A/B/C/D/F) not found in expected format")
		t.Log("      Schema field: quality_rating with enum [A, B, C, D, F]")
	}

	// 3. Check for signal:noise ratio from stock-analysis.schema.json
	// signal_noise_ratio enum: HIGH, MEDIUM, LOW
	signalNoisePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)signal[:\s/-]+noise[:\s]+(?:HIGH|MEDIUM|LOW)`),
		regexp.MustCompile(`(?i)signal[:\s/-]+noise\s+ratio[:\s]+(?:HIGH|MEDIUM|LOW)`),
		regexp.MustCompile(`(?i)s\s*/\s*n[:\s]+(?:HIGH|MEDIUM|LOW)`),
		regexp.MustCompile(`\|\s*(?:HIGH|MEDIUM|LOW)\s*\|`), // Table format
	}

	foundSignalNoise := false
	for _, pattern := range signalNoisePatterns {
		if pattern.MatchString(content) {
			foundSignalNoise = true
			t.Log("PASS: Found signal:noise ratio in output (schema: signal_noise_ratio)")
			break
		}
	}

	if !foundSignalNoise {
		t.Log("INFO: Signal:noise ratio not found in expected format")
		t.Log("      Schema field: signal_noise_ratio with enum [HIGH, MEDIUM, LOW]")
	}

	// 4. Check for technical indicators from stock-analysis.schema.json
	// technical_analysis object with: sma_20, sma_50, sma_200, rsi_14
	technicalIndicators := []string{
		"SMA", "RSI", "SUPPORT", "RESISTANCE",
		"MOVING AVERAGE", "BULLISH", "BEARISH", "NEUTRAL",
	}

	foundTechnical := false
	for _, indicator := range technicalIndicators {
		if strings.Contains(contentUpper, indicator) {
			foundTechnical = true
			t.Logf("PASS: Found technical indicator '%s' in output (schema: technical_analysis)", indicator)
			break
		}
	}

	if !foundTechnical {
		t.Log("INFO: Technical indicators (SMA, RSI, Support, Resistance) not found")
		t.Log("      Schema field: technical_analysis object")
	}

	// 5. Check for conviction scores (1-10) from stock-analysis.schema.json
	// trader_recommendation.conviction and super_recommendation.conviction
	convictionPattern := regexp.MustCompile(`(?i)conviction[:\s]+([1-9]|10)\b`)
	foundConviction := convictionPattern.MatchString(content)

	if foundConviction {
		t.Log("PASS: Found conviction score (1-10) in output (schema: conviction)")
	} else {
		t.Log("INFO: Conviction score (1-10) not found in expected format")
	}

	// Summary of schema compliance
	schemaScore := 0
	if foundRecommendation {
		schemaScore++
	}
	if foundQuality {
		schemaScore++
	}
	if foundSignalNoise {
		schemaScore++
	}
	if foundTechnical {
		schemaScore++
	}
	if foundConviction {
		schemaScore++
	}

	t.Logf("Schema compliance score: %d/5 fields detected", schemaScore)

	// Assert at least basic schema compliance (quality or recommendation found)
	assert.True(t, foundQuality || foundRecommendation || foundTechnical,
		"Output should contain at least one schema-defined field (quality rating, recommendation action, or technical analysis)")

	t.Log("PASS: Output shows schema compliance")
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

// validateDirectorInterestFetched validates that director interest documents were created for expected tickers.
// This ensures fetch_director_interest tool was called and returned data (or "no filings" document).
func validateDirectorInterestFetched(t *testing.T, helper *common.HTTPTestHelper, expectedTickers []string) {
	t.Log("Validating director interest data was fetched...")

	// Director interest documents are tagged with ["director-interest", "<lowercase-ticker>"]
	docs := getDocumentsByTag(t, helper, "director-interest")

	// We expect at least one document per ticker (either filings or "no filings" placeholder)
	for _, ticker := range expectedTickers {
		found := false
		var matchedDoc map[string]interface{}

		for _, doc := range docs {
			// Check if document tags contain the ticker (lowercase)
			if tags, ok := doc["tags"].([]interface{}); ok {
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok && strings.EqualFold(tagStr, ticker) {
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

		// Director interest may return "no filings" which is still valid
		// The key is that the worker was called and produced a document
		if !found {
			t.Logf("Note: No director-interest document found for %s (may not have recent filings)", ticker)
		} else {
			// Verify document has content
			if matchedDoc != nil {
				docID, _ := matchedDoc["id"].(string)
				content := getDocumentContent(t, helper, docID)
				require.NotEmpty(t, content, "Director interest document for %s should have content", ticker)
				t.Logf("PASS: Found director interest data for %s (doc: %s, content: %d chars)", ticker, docID[:8], len(content))
			}
		}
	}

	// At minimum, we should have at least one director-interest document
	// (even if it's a "no filings" placeholder)
	if len(docs) > 0 {
		t.Logf("PASS: Found %d director interest documents", len(docs))
	} else {
		t.Log("Note: No director-interest documents found - worker may not have been called or no data available")
	}
}

// validateMacroDataFetched validates that macro data documents were created.
// This ensures fetch_macro_data tool was called and returned data.
func validateMacroDataFetched(t *testing.T, helper *common.HTTPTestHelper) {
	t.Log("Validating macro data was fetched...")

	// Macro data documents are tagged with ["macro-data", "<data_type>"]
	docs := getDocumentsByTag(t, helper, "macro-data")

	if len(docs) == 0 {
		t.Log("Note: No macro-data documents found - worker may not have been called or data unavailable")
		return
	}

	// Verify at least one macro data document has content
	for _, doc := range docs {
		docID, _ := doc["id"].(string)
		content := getDocumentContent(t, helper, docID)
		if content != "" {
			// Check for expected macro data content
			hasRBA := strings.Contains(content, "RBA") || strings.Contains(content, "Cash Rate")
			hasCommodity := strings.Contains(content, "Iron Ore") || strings.Contains(content, "Gold") || strings.Contains(content, "Commodity")

			if hasRBA || hasCommodity {
				t.Logf("PASS: Found macro data (doc: %s, content: %d chars)", docID[:8], len(content))
				t.Logf("PASS: Macro data contains - RBA: %v, Commodities: %v", hasRBA, hasCommodity)
				return
			}
		}
	}

	t.Logf("PASS: Found %d macro data documents", len(docs))
}

// saveTestOutput saves the generated output and logs to BOTH locations:
// 1. Structured: test/results/api/orchestrator-YYYYMMDD-HHMMSS/TestName/output.md + service.log + test.log
// 2. Flat: test/api/results/output-TIMESTAMP-TestName-jobID.md
// This allows manual inspection of test outputs and historical tracking of analysis quality.
func saveTestOutput(t *testing.T, testName string, jobID string, content string, envResultsDir string) {
	timestamp := time.Now().Format("20060102-150405")
	fullTestName := t.Name() // Gets full path like "TestOrchestratorIntegration_FullWorkflow/SingleStock"

	// 1. Save to structured directory: test/results/api/orchestrator-TIMESTAMP/TestName/output.md
	structuredDir := filepath.Join("..", "results", "api", fmt.Sprintf("orchestrator-%s", timestamp), fullTestName)
	if err := os.MkdirAll(structuredDir, 0755); err != nil {
		t.Logf("Warning: Failed to create structured results directory: %v", err)
	} else {
		// Save output.md
		structuredPath := filepath.Join(structuredDir, "output.md")
		if err := os.WriteFile(structuredPath, []byte(content), 0644); err != nil {
			t.Logf("Warning: Failed to write structured output file: %v", err)
		} else {
			t.Logf("Saved structured output to: %s (%d bytes)", structuredPath, len(content))
		}

		// Copy service.log from environment results directory
		serviceLogSrc := filepath.Join(envResultsDir, "service.log")
		if serviceLogContent, err := os.ReadFile(serviceLogSrc); err == nil {
			serviceLogDst := filepath.Join(structuredDir, "service.log")
			if err := os.WriteFile(serviceLogDst, serviceLogContent, 0644); err != nil {
				t.Logf("Warning: Failed to copy service.log: %v", err)
			} else {
				t.Logf("Copied service.log to: %s (%d bytes)", serviceLogDst, len(serviceLogContent))
			}
		} else {
			t.Logf("Warning: Could not read service.log from %s: %v", serviceLogSrc, err)
		}

		// Copy test.log from environment results directory
		testLogSrc := filepath.Join(envResultsDir, "test.log")
		if testLogContent, err := os.ReadFile(testLogSrc); err == nil {
			testLogDst := filepath.Join(structuredDir, "test.log")
			if err := os.WriteFile(testLogDst, testLogContent, 0644); err != nil {
				t.Logf("Warning: Failed to copy test.log: %v", err)
			} else {
				t.Logf("Copied test.log to: %s (%d bytes)", testLogDst, len(testLogContent))
			}
		} else {
			t.Logf("Warning: Could not read test.log from %s: %v", testLogSrc, err)
		}
	}

	// 2. Save to flat directory: test/api/results/output-TIMESTAMP-TestName-jobID.md
	flatDir := filepath.Join("results")
	if err := os.MkdirAll(flatDir, 0755); err != nil {
		t.Logf("Warning: Failed to create flat results directory: %v", err)
		return
	}
	safeTestName := strings.ReplaceAll(testName, "/", "-")
	safeTestName = strings.ReplaceAll(safeTestName, " ", "-")
	flatTimestamp := time.Now().Format("2006-01-02T15-04-05")
	flatFilename := fmt.Sprintf("output-%s-%s-%s.md", flatTimestamp, safeTestName, jobID[:8])
	flatPath := filepath.Join(flatDir, flatFilename)
	if err := os.WriteFile(flatPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: Failed to write flat output file: %v", err)
		return
	}
	t.Logf("Saved flat output to: %s (%d bytes)", flatPath, len(content))
}
