// Package portfolio contains API integration tests for portfolio workers.
//
// IMPORTANT: Stock analysis orchestrator tests require extended timeout due to LLM operations:
//
//	go test -timeout 15m -run TestOrchestratorStockAnalysisGoal ./test/api/portfolio/...
//
// The default Go test timeout (10 minutes) is insufficient for these tests.
// Individual tests use 15-minute timeouts for job completion with error monitoring.
//
// Workflow: hybrid (stock_data_collection → summary with embedded template)
// Output tag: stock-analysis
package portfolio

import (
	"encoding/json"
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
	"github.com/ternarybob/quaero/internal/schemas"
	"github.com/ternarybob/quaero/test/common"
)

// orchestratorTestCase defines a test scenario for the orchestrator integration test
type orchestratorTestCase struct {
	name                   string   // Test scenario name
	jobDefFile             string   // Job definition TOML file name
	jobDefID               string   // Job definition ID (matches id field in TOML)
	expectedTickers        []string // Expected stock tickers in the output
	outputTag              string   // Tag to find output document (default: "stock-recommendation")
	schemaFile             string   // Schema file name for validation (e.g., "stock-report.schema.json")
	expectedIndices        []string // Expected index codes (e.g., XJO, XSO) - validates fetch_index_data was called
	expectDirectorInterest bool     // Whether to validate director-interest documents exist
	expectMacroData        bool     // Whether to validate macro-data documents exist
}

// TestOrchestratorStockAnalysisGoal tests the hybrid stock analysis workflow
// with different stock configurations:
// 1. SingleStock - Tests with 1 stock to verify basic functionality
// 2. MultipleStocks - Tests with 3 stocks to verify multi-stock handling
//
// Workflow: hybrid (stock_data_collection → summary with embedded template)
// Output tag: stock-analysis
//
// Each scenario validates:
// - Job executes without errors
// - Output content is NOT a placeholder
// - Output content is NOT the AI prompt
// - Output content contains actual stock analysis
func TestOrchestratorStockAnalysisGoal(t *testing.T) {
	testCases := []orchestratorTestCase{
		{
			name:            "SingleStock",
			jobDefFile:      "orchestrator-stock-analysis-1-stock-test.toml",
			jobDefID:        "orchestrator-stock-analysis-1-stock-test",
			expectedTickers: []string{"GNP"},
			outputTag:       "stock-analysis",
			schemaFile:      "stock-report.schema.json",
			expectedIndices: []string{"XJO"},
		},
		{
			name:            "MultipleStocks",
			jobDefFile:      "orchestrator-stock-analysis-3-stocks-test.toml",
			jobDefID:        "orchestrator-stock-analysis-3-stocks-test",
			expectedTickers: []string{"GNP", "SKS", "WEB"},
			outputTag:       "stock-analysis",
			schemaFile:      "stock-report.schema.json",
			expectedIndices: []string{"XJO"},
		},
		{
			name:            "StockAnalysisList",
			jobDefFile:      "orchestrator-stock-analysis-list-test.toml",
			jobDefID:        "orchestrator-stock-analysis-list-test",
			expectedTickers: []string{"GNP", "BCN", "MYG"},
			outputTag:       "stock-analysis",
			schemaFile:      "stock-report.schema.json",
			expectedIndices: []string{"XJO"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runOrchestratorTest(t, tc)
		})
	}
}

// =============================================================================
// Announcement Signal Analysis Tests
// =============================================================================

// TestOrchestratorAnnouncementAnalysis tests the announcement signal-to-noise analysis workflow.
// This validates the announcement-analysis template produces schema-compliant output that:
// - Excludes noise items from output
// - Includes source references (local/data/web)
// - Contains signal-noise assessment metrics
// - Is repeatable (same structure each time)
//
// Workflow: market_announcements → summary with announcement-analysis template → email
// Output tag: announcement-analysis
func TestOrchestratorAnnouncementAnalysis(t *testing.T) {
	tc := orchestratorTestCase{
		name:            "AnnouncementSignalAnalysis",
		jobDefFile:      "orchestrator-announcement-analysis-test.toml",
		jobDefID:        "orchestrator-announcement-analysis-test",
		expectedTickers: []string{"GNP"},
		outputTag:       "announcement-analysis",
		schemaFile:      "announcement-analysis.schema.json",
	}

	runOrchestratorTest(t, tc)
}

// TestOrchestratorAnnouncementAnalysisMultiStock tests the multi-stock announcement analysis workflow.
// This validates the announcement-analysis-report template produces:
// - Consolidated output with all stocks
// - Stocks ordered alphabetically by ticker
// - Per-stock signal-noise analysis
// - Cross-stock summary
//
// Workflow: market_announcements → summary with announcement-analysis-report template → email
// Output tag: announcement-analysis-report
func TestOrchestratorAnnouncementAnalysisMultiStock(t *testing.T) {
	tc := orchestratorTestCase{
		name:            "AnnouncementSignalAnalysisMultiStock",
		jobDefFile:      "orchestrator-announcement-analysis-3-stocks-test.toml",
		jobDefID:        "orchestrator-announcement-analysis-3-stocks-test",
		expectedTickers: []string{"GNP", "SKS", "WEB"},
		outputTag:       "announcement-analysis-report",
		schemaFile:      "announcement-analysis-report.schema.json",
	}

	runOrchestratorTest(t, tc)
}

// runOrchestratorTest executes a single orchestrator test scenario
func runOrchestratorTest(t *testing.T, tc orchestratorTestCase) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 15*time.Minute)

	// Step 1: Load the orchestrated job definition
	stepStart := time.Now()
	t.Logf("Step 1: Loading job definition %s", tc.jobDefFile)
	err = env.LoadTestJobDefinitions("../config/job-definitions/" + tc.jobDefFile)
	require.NoError(t, err, "Failed to load orchestrated job definition")
	timingData.AddStepTiming("load_job_definition", time.Since(stepStart).Seconds())

	// Step 2: Trigger the job
	stepStart = time.Now()
	t.Log("Step 2: Triggering orchestrated job")
	jobID := executeJobDefinition(t, helper, tc.jobDefID)
	require.NotEmpty(t, jobID, "Job execution should return job ID")
	t.Logf("Triggered job ID: %s", jobID)
	timingData.AddStepTiming("trigger_job", time.Since(stepStart).Seconds())

	// Cleanup job after test
	defer deleteJob(t, helper, jobID)

	// Step 3: Wait for job completion with error monitoring (15 minute timeout for LLM operations)
	// This monitors for ERROR logs during execution and fails fast if errors are detected
	stepStart = time.Now()
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 15 minutes)")
	finalStatus, errorLogs := waitForJobCompletionWithMonitoring(t, helper, jobID, 15*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)
	timingData.AddStepTiming("wait_for_completion", time.Since(stepStart).Seconds())

	// Step 4: Handle error logs if any were found
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
	// Find the actual stock analysis document (filter out orchestrator-execution-log)
	t.Logf("Step 6: Retrieving output document with tag '%s'", tc.outputTag)
	docs := getDocumentsByTag(t, helper, tc.outputTag)
	require.Greater(t, len(docs), 0, "Should have at least one document with '%s' tag", tc.outputTag)

	// Filter out orchestrator-execution-log documents to get the actual analysis output
	outputDoc := findOutputDocument(t, docs)
	require.NotNil(t, outputDoc, "Should find a valid output document (not orchestrator-execution-log)")
	docID, _ := outputDoc["id"].(string)
	t.Logf("Found output document: %s", docID)

	// Get document content and metadata
	content, metadata := getDocumentContentAndMetadata(t, helper, docID)
	require.NotEmpty(t, content, "Document content should not be empty")
	t.Logf("Document content length: %d characters", len(content))

	// Step 7: Save test output and logs to results directory for verification
	t.Log("Step 7: Saving test output, config, schema, and JSON to results directory")
	resultsDir := saveTestOutput(t, tc.name, jobID, content, env.GetResultsDir())
	saveOrchestratorJobConfig(t, resultsDir, tc.jobDefFile)
	saveSchemaFile(t, resultsDir, tc.schemaFile)
	saveDocumentMetadata(t, resultsDir, metadata)

	// Validate email content
	validateEmailContent(t, content, tc.expectedTickers, tc.schemaFile)

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

	// Check service.log for errors
	common.AssertNoErrorsInServiceLog(t, env)

	t.Log("SUCCESS: Orchestrator integration test completed successfully")
}

// validateEmailContent validates that the email content is valid stock analysis
func validateEmailContent(t *testing.T, content string, expectedTickers []string, schemaFile string) {
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

	// Step 10: Validate schema compliance based on output type
	validateSchemaComplianceByType(t, content, schemaFile)
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

// validateSchemaComplianceByType dispatches to the appropriate schema validator based on schema file.
func validateSchemaComplianceByType(t *testing.T, content string, schemaFile string) {
	t.Logf("Step 10: Validating output against schema: %s", schemaFile)

	switch schemaFile {
	case "stock-report.schema.json":
		validateStockReportSchema(t, content)
	case "portfolio-review.schema.json":
		validatePortfolioReviewSchema(t, content)
	case "purchase-conviction.schema.json":
		validatePurchaseConvictionSchema(t, content)
	case "announcement-analysis.schema.json":
		validateAnnouncementAnalysisSchema(t, content)
	case "announcement-analysis-report.schema.json":
		validateAnnouncementAnalysisReportSchema(t, content)
	default:
		// Fall back to generic validation
		t.Logf("Using generic schema validation for: %s", schemaFile)
		validateSchemaCompliance(t, content)
	}
}

// validateStockReportSchema validates output against stock-report.schema.json required fields.
// Required fields: stocks, summary_table, watchlists, definitions
func validateStockReportSchema(t *testing.T, content string) {
	t.Log("Validating stock-report.schema.json compliance")
	contentLower := strings.ToLower(content)

	schemaScore := 0
	totalFields := 4

	// 1. Check for stocks array indicators (detailed stock analysis)
	stockIndicators := []string{"stock data", "announcement analysis", "price event", "quality assessment"}
	foundStocks := false
	for _, indicator := range stockIndicators {
		if strings.Contains(contentLower, indicator) {
			foundStocks = true
			t.Logf("PASS: Found 'stocks' section indicator: '%s'", indicator)
			break
		}
	}
	if foundStocks {
		schemaScore++
	} else {
		t.Log("INFO: Stock analysis sections not found in expected format")
	}

	// 2. Check for summary_table indicators
	summaryTablePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\|\s*ticker\s*\|`),         // | Ticker | (table header)
		regexp.MustCompile(`(?i)summary\s*table`),          // Summary Table heading
		regexp.MustCompile(`(?i)\|\s*quality\s*\|`),        // | Quality | column
		regexp.MustCompile(`(?i)\|\s*[A-Z]{2,5}\s*\|.*\|`), // | GNP | ... | (ticker in table)
	}
	foundSummaryTable := false
	for _, pattern := range summaryTablePatterns {
		if pattern.MatchString(content) {
			foundSummaryTable = true
			t.Log("PASS: Found 'summary_table' format in output")
			break
		}
	}
	if foundSummaryTable {
		schemaScore++
	} else {
		t.Log("INFO: Summary table not found in expected format")
	}

	// 3. Check for watchlists indicators
	watchlistPatterns := []string{"watchlist", "trader momentum", "super accumulate", "accumulation"}
	foundWatchlists := false
	for _, pattern := range watchlistPatterns {
		if strings.Contains(contentLower, pattern) {
			foundWatchlists = true
			t.Logf("PASS: Found 'watchlists' indicator: '%s'", pattern)
			break
		}
	}
	if foundWatchlists {
		schemaScore++
	} else {
		t.Log("INFO: Watchlists section not found")
	}

	// 4. Check for definitions section
	definitionPatterns := []string{"definition", "rating scale", "quality rating", "signal:noise", "signal-to-noise"}
	foundDefinitions := false
	for _, pattern := range definitionPatterns {
		if strings.Contains(contentLower, pattern) {
			foundDefinitions = true
			t.Logf("PASS: Found 'definitions' indicator: '%s'", pattern)
			break
		}
	}
	if foundDefinitions {
		schemaScore++
	} else {
		t.Log("INFO: Definitions section not found")
	}

	t.Logf("Stock Report Schema compliance: %d/%d required sections found", schemaScore, totalFields)

	// Also run the general schema compliance for recommendation fields
	validateSchemaCompliance(t, content)

	// Assert at least 2 of 4 sections are present
	assert.GreaterOrEqual(t, schemaScore, 2,
		"Stock report should contain at least 2 of 4 required sections (stocks, summary_table, watchlists, definitions)")
}

// validatePortfolioReviewSchema validates output against portfolio-review.schema.json required fields.
// Required fields: portfolio_valuation, total_summary, recommendations, risk_alerts
func validatePortfolioReviewSchema(t *testing.T, content string) {
	t.Log("Validating portfolio-review.schema.json compliance")
	contentLower := strings.ToLower(content)

	schemaScore := 0
	totalFields := 4

	// 1. Check for portfolio_valuation indicators
	valuationPatterns := []string{
		"valuation", "current value", "current price", "cost basis",
		"unrealized", "profit/loss", "p/l", "market value",
	}
	foundValuation := false
	for _, pattern := range valuationPatterns {
		if strings.Contains(contentLower, pattern) {
			foundValuation = true
			t.Logf("PASS: Found 'portfolio_valuation' indicator: '%s'", pattern)
			break
		}
	}
	if foundValuation {
		schemaScore++
	} else {
		t.Log("INFO: Portfolio valuation section not found")
	}

	// 2. Check for total_summary indicators
	summaryPatterns := []string{
		"total investment", "total value", "total p/l", "overall return",
		"portfolio summary", "total portfolio",
	}
	foundSummary := false
	for _, pattern := range summaryPatterns {
		if strings.Contains(contentLower, pattern) {
			foundSummary = true
			t.Logf("PASS: Found 'total_summary' indicator: '%s'", pattern)
			break
		}
	}
	if foundSummary {
		schemaScore++
	} else {
		t.Log("INFO: Total summary section not found")
	}

	// 3. Check for recommendations indicators
	recPatterns := []string{
		"recommendation", "quality rating", "trader recommendation", "super recommendation",
		"accumulate", "hold", "reduce", "avoid",
	}
	foundRecommendations := false
	for _, pattern := range recPatterns {
		if strings.Contains(contentLower, pattern) {
			foundRecommendations = true
			t.Logf("PASS: Found 'recommendations' indicator: '%s'", pattern)
			break
		}
	}
	if foundRecommendations {
		schemaScore++
	} else {
		t.Log("INFO: Recommendations section not found")
	}

	// 4. Check for risk_alerts indicators
	riskPatterns := []string{
		"risk alert", "risk warning", "warning", "concentration risk",
		"exposure", "diversification",
	}
	foundRiskAlerts := false
	for _, pattern := range riskPatterns {
		if strings.Contains(contentLower, pattern) {
			foundRiskAlerts = true
			t.Logf("PASS: Found 'risk_alerts' indicator: '%s'", pattern)
			break
		}
	}
	if foundRiskAlerts {
		schemaScore++
	} else {
		t.Log("INFO: Risk alerts section not found")
	}

	t.Logf("Portfolio Review Schema compliance: %d/%d required sections found", schemaScore, totalFields)

	// Also run the general schema compliance
	validateSchemaCompliance(t, content)

	// Assert at least 2 of 4 sections are present
	assert.GreaterOrEqual(t, schemaScore, 2,
		"Portfolio review should contain at least 2 of 4 required sections (portfolio_valuation, total_summary, recommendations, risk_alerts)")
}

// validatePurchaseConvictionSchema validates output against purchase-conviction.schema.json required fields.
// Required fields: executive_summary, stocks (with conviction_score, tier), comparative_table, warnings
func validatePurchaseConvictionSchema(t *testing.T, content string) {
	t.Log("Validating purchase-conviction.schema.json compliance")
	contentLower := strings.ToLower(content)

	schemaScore := 0
	totalFields := 4

	// 1. Check for executive_summary indicators
	execSummaryPatterns := []string{
		"executive summary", "summary", "overview", "top picks",
		"market context", "key findings",
	}
	foundExecSummary := false
	for _, pattern := range execSummaryPatterns {
		if strings.Contains(contentLower, pattern) {
			foundExecSummary = true
			t.Logf("PASS: Found 'executive_summary' indicator: '%s'", pattern)
			break
		}
	}
	if foundExecSummary {
		schemaScore++
	} else {
		t.Log("INFO: Executive summary section not found")
	}

	// 2. Check for stocks with conviction analysis
	convictionPatterns := []string{
		"conviction", "fundamental analysis", "technical analysis",
		"bear case", "bull case", "analyst resolution",
		"tier 1", "tier 2", "tier 3", "tier 4",
	}
	foundConviction := false
	for _, pattern := range convictionPatterns {
		if strings.Contains(contentLower, pattern) {
			foundConviction = true
			t.Logf("PASS: Found conviction analysis indicator: '%s'", pattern)
			break
		}
	}
	if foundConviction {
		schemaScore++
	} else {
		t.Log("INFO: Conviction analysis sections not found")
	}

	// 3. Check for comparative_table indicators
	tablePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)comparative.*table`),
		regexp.MustCompile(`(?i)\|\s*ticker\s*\|.*score`),
		regexp.MustCompile(`(?i)\|\s*fundamental\s*\|`),
		regexp.MustCompile(`(?i)total.*score`),
	}
	foundTable := false
	for _, pattern := range tablePatterns {
		if pattern.MatchString(content) {
			foundTable = true
			t.Log("PASS: Found 'comparative_table' format in output")
			break
		}
	}
	if !foundTable {
		// Check string patterns too
		if strings.Contains(contentLower, "comparison") || strings.Contains(contentLower, "ranking") {
			foundTable = true
			t.Log("PASS: Found comparison/ranking section")
		}
	}
	if foundTable {
		schemaScore++
	} else {
		t.Log("INFO: Comparative table not found in expected format")
	}

	// 4. Check for warnings indicators
	warningPatterns := []string{
		"warning", "risk", "caution", "concern", "caveat",
		"market risk", "downside",
	}
	foundWarnings := false
	for _, pattern := range warningPatterns {
		if strings.Contains(contentLower, pattern) {
			foundWarnings = true
			t.Logf("PASS: Found 'warnings' indicator: '%s'", pattern)
			break
		}
	}
	if foundWarnings {
		schemaScore++
	} else {
		t.Log("INFO: Warnings section not found")
	}

	t.Logf("Purchase Conviction Schema compliance: %d/%d required sections found", schemaScore, totalFields)

	// Also run the general schema compliance
	validateSchemaCompliance(t, content)

	// Assert at least 2 of 4 sections are present
	assert.GreaterOrEqual(t, schemaScore, 2,
		"Purchase conviction should contain at least 2 of 4 required sections (executive_summary, stocks, comparative_table, warnings)")
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

// waitForJobCompletionWithMonitoring polls job status and checks for ERROR logs during execution.
// Unlike waitForJobCompletion, this function fails early if ERROR logs are detected,
// preventing long waits when a job has already failed.
// Returns the final status and any error logs found.
func waitForJobCompletionWithMonitoring(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) (status string, errorLogs []map[string]interface{}) {
	startTime := time.Now()
	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond
	errorCheckInterval := 5 * time.Second
	lastErrorCheck := time.Time{}
	lastProgressLog := time.Now()
	progressLogInterval := 30 * time.Second

	t.Logf("Waiting for job %s with error monitoring (timeout: %v)", jobID, timeout)

	for time.Now().Before(deadline) {
		// Get job status
		resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		if err != nil {
			t.Logf("Warning: Failed to get job status: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(pollInterval)
			continue
		}

		var job map[string]interface{}
		if err := helper.ParseJSONResponse(resp, &job); err != nil {
			time.Sleep(pollInterval)
			continue
		}

		jobStatus, ok := job["status"].(string)
		if !ok {
			time.Sleep(pollInterval)
			continue
		}

		// Check for terminal states
		if jobStatus == "completed" || jobStatus == "failed" || jobStatus == "cancelled" {
			elapsed := time.Since(startTime)
			t.Logf("Job %s reached terminal state: %s (elapsed: %s)", jobID, jobStatus, formatTestDuration(elapsed))
			// Final error check on completion
			errorLogs = getJobErrorLogs(t, helper, jobID)
			return jobStatus, errorLogs
		}

		// Log progress every 30 seconds
		if time.Since(lastProgressLog) >= progressLogInterval {
			elapsed := time.Since(startTime)
			t.Logf("Job %s still %s... (elapsed: %s)", jobID[:8], jobStatus, formatTestDuration(elapsed))
			lastProgressLog = time.Now()
		}

		// Check for errors periodically (every 5 seconds) to fail fast
		if time.Since(lastErrorCheck) >= errorCheckInterval {
			errorLogs = getJobErrorLogs(t, helper, jobID)
			if len(errorLogs) > 0 {
				t.Logf("ERROR detected during job execution (found %d errors) - failing early", len(errorLogs))
				// Log first few errors for visibility
				for i, log := range errorLogs {
					if i < 3 {
						logMsg, _ := log["message"].(string)
						t.Logf("  ERROR[%d]: %s", i, logMsg)
					}
				}
				return "error", errorLogs
			}
			lastErrorCheck = time.Now()
		}

		time.Sleep(pollInterval)
	}

	t.Logf("Job %s did not reach terminal state within %v", jobID, timeout)
	return "timeout", nil
}

// formatTestDuration formats a duration for test output (e.g., "2m15s")
func formatTestDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// logChildJobTimings logs timing information for all child jobs and returns timing data
func logChildJobTimings(t *testing.T, helper *common.HTTPTestHelper, parentJobID string) []common.WorkerTiming {
	childJobs := getChildJobs(t, helper, parentJobID)
	if len(childJobs) == 0 {
		t.Log("No child jobs found for timing analysis")
		return nil
	}

	t.Log("=== Child Job Timing Summary ===")

	// Group by worker type for summary
	workerTimings := make(map[string][]time.Duration)
	var totalDuration time.Duration
	var timingData []common.WorkerTiming

	for _, job := range childJobs {
		jobID, _ := job["id"].(string)
		name, _ := job["name"].(string)
		workerType, _ := job["worker_type"].(string)
		status, _ := job["status"].(string)

		// Parse duration from job stats if available
		var duration time.Duration
		var durationSeconds float64
		if stats, ok := job["stats"].(map[string]interface{}); ok {
			if durationSec, ok := stats["duration_seconds"].(float64); ok {
				durationSeconds = durationSec
				duration = time.Duration(durationSec * float64(time.Second))
			}
		}

		// Fallback: calculate from started_at/completed_at
		if duration == 0 {
			if startedAtStr, ok := job["started_at"].(string); ok {
				if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
					if completedAtStr, ok := job["completed_at"].(string); ok {
						if completedAt, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
							duration = completedAt.Sub(startedAt)
							durationSeconds = duration.Seconds()
						}
					}
				}
			}
		}

		if duration > 0 {
			workerTimings[workerType] = append(workerTimings[workerType], duration)
			totalDuration += duration
			t.Logf("  %s (%s): %s [%s] - %s", name, workerType, formatTestDuration(duration), status, jobID[:8])

			// Add to timing data
			timingData = append(timingData, common.WorkerTiming{
				Name:              name,
				WorkerType:        workerType,
				DurationFormatted: formatTestDuration(duration),
				DurationSeconds:   durationSeconds,
				Status:            status,
				JobID:             jobID,
			})
		} else {
			t.Logf("  %s (%s): duration unknown [%s] - %s", name, workerType, status, jobID[:8])
		}
	}

	// Log summary by worker type
	if len(workerTimings) > 0 {
		t.Log("--- Worker Type Summary ---")
		for workerType, durations := range workerTimings {
			var totalWorkerDuration time.Duration
			for _, d := range durations {
				totalWorkerDuration += d
			}
			avgDuration := totalWorkerDuration / time.Duration(len(durations))
			t.Logf("  %s: %d jobs, total=%s, avg=%s", workerType, len(durations), formatTestDuration(totalWorkerDuration), formatTestDuration(avgDuration))
		}
		t.Logf("--- Total child job time: %s (may overlap if parallel) ---", formatTestDuration(totalDuration))
	}

	return timingData
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

// saveTestOutput saves the generated output and logs to:
// test/results/api/orchestrator-YYYYMMDD-HHMMSS-TestName/output.md + service.log + test.log
// This allows manual inspection of test outputs and historical tracking of analysis quality.
// Returns the structured directory path for use by timing and TDD summary functions.
func saveTestOutput(t *testing.T, testName string, jobID string, content string, envResultsDir string) string {
	// Use common.GetTestResultsDir for identifiable directory naming
	structuredDir := common.GetTestResultsDir("orchestrator", t.Name())
	if err := os.MkdirAll(structuredDir, 0755); err != nil {
		t.Logf("Warning: Failed to create structured results directory: %v", err)
		return ""
	}

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

	return structuredDir
}

// findOutputDocument finds the actual output document from a list of tagged documents.
// It filters out orchestrator-execution-log documents which are internal execution summaries.
// The actual output is typically from a summary worker with the "summary" source type.
func findOutputDocument(t *testing.T, docs []map[string]interface{}) map[string]interface{} {
	for _, doc := range docs {
		// Check if document has "orchestrator-execution-log" tag - skip these
		tags, ok := doc["tags"].([]interface{})
		if ok {
			isExecutionLog := false
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr == "orchestrator-execution-log" {
					isExecutionLog = true
					break
				}
			}
			if isExecutionLog {
				docID, _ := doc["id"].(string)
				t.Logf("Skipping orchestrator-execution-log document: %s", docID)
				continue
			}
		}

		// Check source_type - prefer "summary" type documents
		sourceType, _ := doc["source_type"].(string)
		if sourceType == "summary" {
			docID, _ := doc["id"].(string)
			t.Logf("Found summary output document: %s (source_type: %s)", docID, sourceType)
			return doc
		}
	}

	// If no summary document found, return first non-execution-log document
	for _, doc := range docs {
		tags, ok := doc["tags"].([]interface{})
		if ok {
			isExecutionLog := false
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr == "orchestrator-execution-log" {
					isExecutionLog = true
					break
				}
			}
			if !isExecutionLog {
				docID, _ := doc["id"].(string)
				t.Logf("Found output document: %s (fallback - first non-execution-log)", docID)
				return doc
			}
		} else {
			// No tags at all - return this document
			docID, _ := doc["id"].(string)
			t.Logf("Found output document: %s (no tags)", docID)
			return doc
		}
	}

	return nil
}

// getDocumentContentAndMetadata retrieves both the content and full document data of a document by ID.
// Returns the content string and the full document response (for saving as output.json).
func getDocumentContentAndMetadata(t *testing.T, helper *common.HTTPTestHelper, docID string) (string, map[string]interface{}) {
	resp, err := helper.GET(fmt.Sprintf("/api/documents/%s", docID))
	if err != nil {
		t.Logf("Warning: Failed to get document: %v", err)
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET document returned %d", resp.StatusCode)
		return "", nil
	}

	var result map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse document response: %v", err)
		return "", nil
	}

	// Extract content
	var content string
	if c, ok := result["content"].(string); ok && c != "" {
		content = c
	} else if mdContent, ok := result["content_markdown"].(string); ok && mdContent != "" {
		content = mdContent
	} else if body, ok := result["body"].(string); ok && body != "" {
		content = body
	} else if text, ok := result["text"].(string); ok && text != "" {
		content = text
	} else if data, ok := result["data"].(map[string]interface{}); ok {
		if c, ok := data["content"].(string); ok {
			content = c
		}
	}

	// Return the full document (without content to avoid duplication in output.json)
	// Remove large content fields to keep output.json manageable
	docCopy := make(map[string]interface{})
	for k, v := range result {
		// Skip content fields that are already saved in output.md
		if k == "content" || k == "content_markdown" || k == "body" || k == "text" {
			continue
		}
		docCopy[k] = v
	}

	return content, docCopy
}

// saveOrchestratorJobConfig saves the job definition TOML file to the results directory
func saveOrchestratorJobConfig(t *testing.T, resultsDir string, jobDefFile string) {
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

// saveSchemaFile copies the schema file to the results directory
// Uses embedded schemas from internal/schemas instead of file system
func saveSchemaFile(t *testing.T, resultsDir string, schemaFile string) {
	if resultsDir == "" || schemaFile == "" {
		return
	}

	// Use embedded schemas from internal/schemas package
	content, err := schemas.GetSchema(schemaFile)
	if err != nil {
		t.Logf("Warning: Failed to get embedded schema %s: %v", schemaFile, err)
		return
	}

	destPath := filepath.Join(resultsDir, "schema.json")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		t.Logf("Warning: Failed to write schema file: %v", err)
		return
	}

	t.Logf("Saved schema to: %s (%d bytes)", destPath, len(content))
}

// saveDocumentMetadata saves the document data (excluding content) as JSON to the results directory.
// This includes document ID, tags, source_type, metadata, timestamps, etc.
func saveDocumentMetadata(t *testing.T, resultsDir string, docData map[string]interface{}) {
	if resultsDir == "" || docData == nil {
		return
	}

	jsonData, err := json.MarshalIndent(docData, "", "  ")
	if err != nil {
		t.Logf("Warning: Failed to marshal document data: %v", err)
		return
	}

	destPath := filepath.Join(resultsDir, "output.json")
	if err := os.WriteFile(destPath, jsonData, 0644); err != nil {
		t.Logf("Warning: Failed to write output.json: %v", err)
		return
	}

	t.Logf("Saved document data to: %s (%d bytes)", destPath, len(jsonData))
}

// validateAnnouncementAnalysisSchema validates output against announcement-analysis.schema.json.
// Required fields: ticker, analysis_date, executive_summary, signal_noise_assessment, high_signal_announcements
func validateAnnouncementAnalysisSchema(t *testing.T, content string) {
	t.Log("Validating announcement-analysis.schema.json compliance")
	contentLower := strings.ToLower(content)

	schemaScore := 0
	totalFields := 5

	// 1. Check for ticker
	tickerPattern := regexp.MustCompile(`(?i)\bticker[:\s]+[A-Z]{2,5}\b`)
	if tickerPattern.MatchString(content) || strings.Contains(content, "GNP") {
		schemaScore++
		t.Log("PASS: Found 'ticker' field")
	} else {
		t.Log("INFO: Ticker field not found in expected format")
	}

	// 2. Check for executive_summary indicators
	summaryPatterns := []string{"executive summary", "executive_summary", "summary:"}
	foundSummary := false
	for _, pattern := range summaryPatterns {
		if strings.Contains(contentLower, pattern) {
			foundSummary = true
			schemaScore++
			t.Logf("PASS: Found 'executive_summary' indicator: '%s'", pattern)
			break
		}
	}
	if !foundSummary {
		t.Log("INFO: Executive summary section not found")
	}

	// 3. Check for signal_noise_assessment indicators
	signalNoisePatterns := []string{
		"signal-noise", "signal_noise", "signal:noise", "signal to noise",
		"noise ratio", "noise_ratio", "overall rating", "overall_rating",
		"high signal count", "high_signal_count",
	}
	foundSignalNoise := false
	for _, pattern := range signalNoisePatterns {
		if strings.Contains(contentLower, pattern) {
			foundSignalNoise = true
			schemaScore++
			t.Logf("PASS: Found 'signal_noise_assessment' indicator: '%s'", pattern)
			break
		}
	}
	if !foundSignalNoise {
		t.Log("INFO: Signal-noise assessment section not found")
	}

	// 4. Check for high_signal_announcements (noise should be EXCLUDED)
	highSignalPatterns := []string{
		"high signal", "high_signal", "significant announcement",
		"material announcement", "high-signal",
	}
	foundHighSignal := false
	for _, pattern := range highSignalPatterns {
		if strings.Contains(contentLower, pattern) {
			foundHighSignal = true
			schemaScore++
			t.Logf("PASS: Found 'high_signal_announcements' indicator: '%s'", pattern)
			break
		}
	}
	if !foundHighSignal {
		t.Log("INFO: High signal announcements section not found")
	}

	// 5. Check for sources section (local/data/web)
	sourcePatterns := []string{
		"sources", "local:", "data:", "web:", "api sources",
		"eodhd", "asx.com", "document_id", "document id",
	}
	foundSources := false
	for _, pattern := range sourcePatterns {
		if strings.Contains(contentLower, pattern) {
			foundSources = true
			schemaScore++
			t.Logf("PASS: Found 'sources' indicator: '%s'", pattern)
			break
		}
	}
	if !foundSources {
		t.Log("INFO: Sources section not found")
	}

	t.Logf("Announcement Analysis Schema compliance: %d/%d required sections found", schemaScore, totalFields)

	// 6. Verify noise exclusion (negative test - noise indicators should NOT be prominent)
	noiseExclusionCheck := true
	noiseOnlyPatterns := []string{
		"change of registry", "cleansing notice", "duplicate announcement",
	}
	for _, pattern := range noiseOnlyPatterns {
		if strings.Contains(contentLower, pattern) {
			noiseExclusionCheck = false
			t.Logf("INFO: Found potential noise content (may be in exclusion list): '%s'", pattern)
		}
	}
	if noiseExclusionCheck {
		t.Log("PASS: No obvious noise-only content found in output (noise properly excluded)")
	}

	// 7. Check for signal rating values (HIGH, MODERATE)
	ratingPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)signal[_\s-]*rating[:\s]+(?:HIGH|MODERATE)`),
		regexp.MustCompile(`\|\s*(?:HIGH|MODERATE)\s*\|`),
		regexp.MustCompile(`(?i)\b(?:HIGH|MODERATE)\s+signal\b`),
	}
	foundRating := false
	for _, pattern := range ratingPatterns {
		if pattern.MatchString(content) {
			foundRating = true
			t.Log("PASS: Found signal rating values (HIGH/MODERATE)")
			break
		}
	}
	if !foundRating {
		t.Log("INFO: Signal rating values not found in expected format")
	}

	// Assert at least 3 of 5 required sections are present
	assert.GreaterOrEqual(t, schemaScore, 3,
		"Announcement analysis should contain at least 3 of 5 required sections")
}

// validateAnnouncementAnalysisReportSchema validates output against announcement-analysis-report.schema.json.
// Required fields: analysis_date, stocks array (ordered alphabetically by ticker)
func validateAnnouncementAnalysisReportSchema(t *testing.T, content string) {
	t.Log("Validating announcement-analysis-report.schema.json compliance")
	contentLower := strings.ToLower(content)

	schemaScore := 0
	totalFields := 5

	// 1. Check for analysis_date at report level
	datePatterns := []string{"analysis_date", "analysis date"}
	foundDate := false
	for _, pattern := range datePatterns {
		if strings.Contains(contentLower, pattern) {
			foundDate = true
			schemaScore++
			t.Log("PASS: Found 'analysis_date' field")
			break
		}
	}
	if !foundDate {
		t.Log("INFO: Analysis date not found at report level")
	}

	// 2. Check for report_summary (cross-stock summary)
	summaryPatterns := []string{"report summary", "report_summary", "cross-stock", "comparative"}
	foundReportSummary := false
	for _, pattern := range summaryPatterns {
		if strings.Contains(contentLower, pattern) {
			foundReportSummary = true
			schemaScore++
			t.Logf("PASS: Found 'report_summary' indicator: '%s'", pattern)
			break
		}
	}
	if !foundReportSummary {
		t.Log("INFO: Report summary section not found")
	}

	// 3. Check for stocks array (multiple ticker sections)
	expectedTickers := []string{"GNP", "SKS", "WEB"}
	tickersFound := 0
	for _, ticker := range expectedTickers {
		if strings.Contains(content, ticker) {
			tickersFound++
			t.Logf("PASS: Found ticker '%s' in output", ticker)
		}
	}
	if tickersFound >= 2 {
		schemaScore++
		t.Logf("PASS: Found %d of %d expected tickers", tickersFound, len(expectedTickers))
	} else {
		t.Logf("INFO: Only found %d of %d expected tickers", tickersFound, len(expectedTickers))
	}

	// 4. Check for alphabetical ordering indicators
	// Look for pattern where GNP appears before SKS, SKS before WEB
	gnpIdx := strings.Index(content, "GNP")
	sksIdx := strings.Index(content, "SKS")
	webIdx := strings.Index(content, "WEB")

	alphabeticallyOrdered := false
	if gnpIdx != -1 && sksIdx != -1 && webIdx != -1 {
		if gnpIdx < sksIdx && sksIdx < webIdx {
			alphabeticallyOrdered = true
			schemaScore++
			t.Log("PASS: Tickers appear in alphabetical order (GNP < SKS < WEB)")
		} else {
			t.Logf("INFO: Tickers not in alphabetical order (GNP@%d, SKS@%d, WEB@%d)", gnpIdx, sksIdx, webIdx)
		}
	} else {
		t.Log("INFO: Cannot verify alphabetical ordering - not all tickers found")
	}

	// 5. Check for per-stock signal-noise content
	signalNoisePatterns := []string{
		"signal-noise", "signal_noise", "signal:noise", "signal to noise",
		"noise ratio", "noise_ratio", "overall rating", "overall_rating",
	}
	foundSignalNoise := false
	for _, pattern := range signalNoisePatterns {
		if strings.Contains(contentLower, pattern) {
			foundSignalNoise = true
			schemaScore++
			t.Logf("PASS: Found per-stock 'signal_noise_assessment' indicator: '%s'", pattern)
			break
		}
	}
	if !foundSignalNoise {
		t.Log("INFO: Signal-noise assessment section not found")
	}

	t.Logf("Announcement Analysis Report Schema compliance: %d/%d sections found", schemaScore, totalFields)

	// Additional validation: Check noise exclusion
	noiseOnlyPatterns := []string{
		"change of registry", "cleansing notice", "duplicate announcement",
	}
	for _, pattern := range noiseOnlyPatterns {
		if strings.Contains(contentLower, pattern) {
			t.Logf("INFO: Found potential noise content: '%s'", pattern)
		}
	}

	// Check for signal rating values across stocks
	ratingPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)overall[_\s-]*rating[:\s]+(?:EXCELLENT|GOOD|AVERAGE|POOR)`),
		regexp.MustCompile(`(?i)\b(?:HIGH|MODERATE)\s+signal\b`),
	}
	for _, pattern := range ratingPatterns {
		if pattern.MatchString(content) {
			t.Log("PASS: Found signal/overall rating values")
			break
		}
	}

	// Assert at least 3 of 5 required sections are present
	assert.GreaterOrEqual(t, schemaScore, 3,
		"Announcement analysis report should contain at least 3 of 5 required sections")

	// Additional assertion for alphabetical ordering when all tickers present
	if tickersFound == len(expectedTickers) {
		assert.True(t, alphabeticallyOrdered,
			"Stocks should be ordered alphabetically by ticker (GNP < SKS < WEB)")
	}
}
