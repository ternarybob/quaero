// =============================================================================
// Navexa Portfolio Worker Integration Tests
// =============================================================================
// Tests the navexa_portfolio and navexa_portfolio_review workers:
// 1. navexa_portfolio: Fetches portfolio by name with all holdings
// 2. navexa_portfolio_review: Generates LLM-powered portfolio review
//
// EXECUTION ORDER: Tests run sequentially. If navexa_portfolio fails,
// navexa_portfolio_review is skipped to avoid cascading failures.
//
// IMPORTANT: Requires valid navexa_api_key in KV storage.
// LLM API key required for portfolio review test.
// =============================================================================

package portfolio

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Sequential Workflow Test
// =============================================================================

// TestNavexaPortfolioWorkflow runs both portfolio tests sequentially.
// If the first test (navexa_portfolio) fails, the second test is skipped.
func TestNavexaPortfolioWorkflow(t *testing.T) {
	// Shared state between subtests
	var portfolioTestPassed bool
	var portfolioID int
	var portfolioName string

	// Initialize shared test environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if Navexa API key is configured (required for both tests)
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("Navexa API key not configured - skipping workflow tests")
	}

	// Run portfolio fetch test first
	t.Run("1_NavexaPortfolio", func(t *testing.T) {
		portfolioTestPassed, portfolioID, portfolioName = runNavexaPortfolioTest(t, env, helper, apiKey)
	})

	// Run portfolio review test only if first test passed
	t.Run("2_NavexaPortfolioReview", func(t *testing.T) {
		if !portfolioTestPassed {
			t.Skip("Skipping portfolio review test because navexa_portfolio test failed")
		}
		runNavexaPortfolioReviewTest(t, env, helper, portfolioID, portfolioName)
	})
}

// =============================================================================
// Individual Test Functions
// =============================================================================

// runNavexaPortfolioTest tests the navexa_portfolio worker
// Returns: success flag, portfolio ID, portfolio name
func runNavexaPortfolioTest(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, apiKey string) (bool, int, string) {
	timingData := common.NewTestTimingData("NavexaPortfolio")

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: NavexaPortfolio", time.Now().Format(time.RFC3339)))

	// Create results subdirectory for this test
	resultsDir := filepath.Join(env.ResultsDir, "navexa_portfolio")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	// Get base URL from KV store (or use default)
	baseURL := GetNavexaBaseURL(t, helper)
	testLog = append(testLog, fmt.Sprintf("[%s] Using base URL: %s", time.Now().Format(time.RFC3339), baseURL))

	// Step 1: Validate Navexa API connectivity
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Validating Navexa API connectivity", time.Now().Format(time.RFC3339)))
	portfolios, err := FetchAndValidateNavexaAPI(t, resultsDir, baseURL, apiKey)
	require.NoError(t, err, "FAIL: Failed to fetch portfolios from Navexa API")
	require.NotEmpty(t, portfolios, "FAIL: Must have at least one portfolio")
	timingData.AddStepTiming("validate_api", time.Since(stepStart).Seconds())

	// Find SMSF portfolio or use first portfolio
	var targetPortfolioID int
	var targetPortfolioName string
	for _, p := range portfolios {
		name, _ := p["name"].(string)
		if strings.Contains(strings.ToUpper(name), "SMSF") {
			targetPortfolioID = int(p["id"].(float64))
			targetPortfolioName = name
			break
		}
	}
	if targetPortfolioID == 0 {
		targetPortfolioID = int(portfolios[0]["id"].(float64))
		targetPortfolioName = portfolios[0]["name"].(string)
	}
	testLog = append(testLog, fmt.Sprintf("[%s] Target portfolio: %d (%s)", time.Now().Format(time.RFC3339), targetPortfolioID, targetPortfolioName))

	// Step 2: Save schema definition
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Saving schema definition", time.Now().Format(time.RFC3339)))
	SaveSchemaDefinition(t, resultsDir, NavexaPortfolioSchema, "navexa_portfolio")

	// Step 3: Create job definition for portfolio fetch
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Creating job definition", time.Now().Format(time.RFC3339)))
	defID := fmt.Sprintf("test-navexa-portfolio-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Navexa Portfolio Worker Test",
		"description": "Test navexa_portfolio worker",
		"type":        "navexa_portfolio",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa", "portfolio"},
		"steps": []map[string]interface{}{
			{
				"name": "get-portfolio",
				"type": "navexa_portfolio",
				"config": map[string]interface{}{
					"name": "smsf", // Search for portfolio containing "smsf"
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, resultsDir, body)

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "FAIL: Failed to create job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "FAIL: Job definition creation must succeed")
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition created: %s", time.Now().Format(time.RFC3339), defID))

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Step 4: Execute job
	stepStart = time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 4: Executing job", time.Now().Format(time.RFC3339)))
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "FAIL: Failed to execute job")
	defer execResp.Body.Close()

	require.Equal(t, http.StatusAccepted, execResp.StatusCode, "FAIL: Job execution must be accepted")

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))
	t.Logf("Executed navexa_portfolio job: %s", jobID)
	timingData.AddStepTiming("job_trigger", time.Since(stepStart).Seconds())

	// Step 5: Wait for completion
	stepStart = time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 5: Waiting for job completion", time.Now().Format(time.RFC3339)))
	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	timingData.AddStepTiming("job_execution", time.Since(stepStart).Seconds())

	// CRITICAL: Job MUST complete successfully
	testLog = append(testLog, fmt.Sprintf("[%s] Job final status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "FAIL: Job must complete successfully - got status: %s", finalStatus)

	// Step 6: Validate document was created
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Validating document creation", time.Now().Format(time.RFC3339)))
	metadata, contentMarkdown := SaveNavexaWorkerOutput(t, helper, resultsDir, "navexa-portfolio")

	// Step 7: Validate document structure
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating document structure", time.Now().Format(time.RFC3339)))

	// Verify content has expected structure
	require.Contains(t, contentMarkdown, "# Navexa Portfolio", "FAIL: Document must have portfolio header")
	require.Contains(t, contentMarkdown, "## Holdings", "FAIL: Document must have holdings section")
	require.Greater(t, len(contentMarkdown), 200, "FAIL: Document must have substantial content (>200 chars), got %d", len(contentMarkdown))
	t.Logf("PASS: Content has expected structure and %d characters", len(contentMarkdown))

	// Verify metadata has required fields
	require.NotNil(t, metadata, "FAIL: Metadata must not be nil")
	require.NotNil(t, metadata["portfolio"], "FAIL: Metadata must have portfolio field")
	require.NotNil(t, metadata["holdings"], "FAIL: Metadata must have holdings field")
	require.NotNil(t, metadata["holding_count"], "FAIL: Metadata must have holding_count field")

	// Verify holdings is non-empty array
	holdings, ok := metadata["holdings"].([]interface{})
	require.True(t, ok, "FAIL: holdings must be an array")
	require.NotEmpty(t, holdings, "FAIL: holdings array must not be empty")
	t.Logf("PASS: Portfolio document has %d holdings", len(holdings))

	// Extract portfolio info from metadata for next test
	portfolioMeta, ok := metadata["portfolio"].(map[string]interface{})
	require.True(t, ok, "FAIL: portfolio metadata must be an object")
	resultPortfolioID := int(portfolioMeta["id"].(float64))
	resultPortfolioName := portfolioMeta["name"].(string)

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Portfolio document created with %d holdings", time.Now().Format(time.RFC3339), len(holdings)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: NavexaPortfolio test completed successfully", time.Now().Format(time.RFC3339)))

	WriteTestLog(t, resultsDir, testLog)

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	t.Log("PASS: NavexaPortfolio test completed successfully")
	return true, resultPortfolioID, resultPortfolioName
}

// runNavexaPortfolioReviewTest tests the navexa_portfolio_review worker
// Uses a two-step pipeline:
// Step 1: navexa_portfolio - fetches portfolio document with holdings (with document caching)
// Step 2: navexa_portfolio_review - consumes the document via filter_tags and generates LLM review
func runNavexaPortfolioReviewTest(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, portfolioID int, portfolioName string) {
	timingData := common.NewTestTimingData("NavexaPortfolioReview")

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: NavexaPortfolioReview", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] Using portfolio: %d (%s)", time.Now().Format(time.RFC3339), portfolioID, portfolioName))

	// Create results subdirectory for this test
	resultsDir := filepath.Join(env.ResultsDir, "navexa_portfolio_review")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("Failed to create results directory: %v", err)
	}

	// Step 1: Save schema definition
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Saving schema definition", time.Now().Format(time.RFC3339)))
	SaveSchemaDefinition(t, resultsDir, NavexaPortfolioReviewSchema, "navexa_portfolio_review")

	// Step 2: Create job definition for two-step portfolio review pipeline
	// Step 1: navexa_portfolio generates the portfolio document with caching
	// Step 2: navexa_portfolio_review consumes the document via filter_tags
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Creating two-step job definition", time.Now().Format(time.RFC3339)))
	defID := fmt.Sprintf("test-navexa-portfolio-review-%d", time.Now().UnixNano())

	// Create unique tag for this test run to link step 1 output to step 2 input
	pipelineTag := fmt.Sprintf("review-input-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Navexa Portfolio Review Worker Test",
		"description": "Test two-step portfolio review pipeline: fetch portfolio with caching, then generate LLM review",
		"type":        "navexa_portfolio_review",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa", "portfolio-review"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-portfolio",
				"type": "navexa_portfolio",
				"config": map[string]interface{}{
					"name":          "smsf",                                         // Search for portfolio containing "smsf"
					"cache_hours":   24,                                             // Use cached document if fresh within 24 hours
					"output_tags":   []string{pipelineTag, "portfolio-review-step"}, // Tag for step 2 to find
					"force_refresh": false,                                          // Allow using cached documents
				},
			},
			{
				"name": "generate-review",
				"type": "navexa_portfolio_review",
				"config": map[string]interface{}{
					"filter_tags": []string{pipelineTag}, // Find document from step 1
					"model":       "gemini",              // Use Gemini for faster/cheaper tests
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, resultsDir, body)

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "FAIL: Failed to create job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "FAIL: Job definition creation must succeed")
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition created: %s", time.Now().Format(time.RFC3339), defID))

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Step 3: Execute job
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Executing job", time.Now().Format(time.RFC3339)))
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "FAIL: Failed to execute job")
	defer execResp.Body.Close()

	require.Equal(t, http.StatusAccepted, execResp.StatusCode, "FAIL: Job execution must be accepted")

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))
	t.Logf("Executed navexa_portfolio_review job: %s", jobID)
	timingData.AddStepTiming("job_trigger", time.Since(stepStart).Seconds())

	// Step 4: Wait for completion (longer timeout since LLM is involved)
	stepStart = time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 4: Waiting for job completion", time.Now().Format(time.RFC3339)))
	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	timingData.AddStepTiming("job_execution", time.Since(stepStart).Seconds())

	// CRITICAL: Job MUST complete successfully
	testLog = append(testLog, fmt.Sprintf("[%s] Job final status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "FAIL: Job must complete successfully - got status: %s", finalStatus)

	// Step 5: Validate document was created and save output files
	testLog = append(testLog, fmt.Sprintf("[%s] Step 5: Validating document creation and saving output", time.Now().Format(time.RFC3339)))
	metadata, contentMarkdown := SavePortfolioReviewWorkerOutput(t, helper, resultsDir)

	// Step 6: Assert result files exist on disk
	testLog = append(testLog, fmt.Sprintf("[%s] Step 6: Asserting result files exist", time.Now().Format(time.RFC3339)))
	AssertResultFilesExist(t, resultsDir)

	// Step 7: Validate document content structure
	testLog = append(testLog, fmt.Sprintf("[%s] Step 7: Validating content structure", time.Now().Format(time.RFC3339)))

	// Verify content has expected structure (LLM should produce headers and analysis)
	require.Contains(t, contentMarkdown, "#", "FAIL: Portfolio review must contain markdown headers")
	require.Greater(t, len(contentMarkdown), 500, "FAIL: Portfolio review must have substantial content (>500 chars), got %d", len(contentMarkdown))
	t.Logf("PASS: Content has markdown headers and %d characters", len(contentMarkdown))

	// Verify metadata has portfolio info
	require.NotNil(t, metadata, "FAIL: Metadata must not be nil")
	t.Logf("PASS: Metadata has %d fields", len(metadata))

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Document created with navexa-portfolio-review tag", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Document has substantial portfolio review content (%d chars)", time.Now().Format(time.RFC3339), len(contentMarkdown)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Result files (output.md, output.json, schema.json, job_definition.json) exist", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: NavexaPortfolioReview test completed successfully", time.Now().Format(time.RFC3339)))

	WriteTestLog(t, resultsDir, testLog)

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("PASS: NavexaPortfolioReview test completed successfully")
}

// =============================================================================
// Standalone Tests (for individual test runs)
// =============================================================================

// TestWorkerNavexaPortfolio tests the navexa_portfolio worker independently
func TestWorkerNavexaPortfolio(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("Navexa API key not configured - skipping test")
	}

	success, _, _ := runNavexaPortfolioTest(t, env, helper, apiKey)
	require.True(t, success, "FAIL: NavexaPortfolio test must pass")
}

// TestWorkerNavexaPortfolioReview tests the navexa_portfolio_review worker independently
// Note: This test finds the portfolio directly, not depending on NavexaPortfolio test
func TestWorkerNavexaPortfolioReview(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestWorkerNavexaPortfolioReview", time.Now().Format(time.RFC3339)))

	// Check if Navexa API key is configured
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, env.ResultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping test")
	}
	testLog = append(testLog, fmt.Sprintf("[%s] Navexa API key loaded from KV store", time.Now().Format(time.RFC3339)))

	// Get base URL from KV store (or use default)
	baseURL := GetNavexaBaseURL(t, helper)
	testLog = append(testLog, fmt.Sprintf("[%s] Using base URL: %s", time.Now().Format(time.RFC3339), baseURL))

	// Step 1: Fetch portfolios to get the SMSF portfolio ID
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Fetching portfolios from Navexa API", time.Now().Format(time.RFC3339)))
	portfolios, err := FetchAndValidateNavexaAPI(t, env.ResultsDir, baseURL, apiKey)
	require.NoError(t, err, "FAIL: Failed to fetch portfolios from Navexa API")
	require.NotEmpty(t, portfolios, "FAIL: Must have at least one portfolio to test portfolio review")
	timingData.AddStepTiming("fetch_portfolios", time.Since(stepStart).Seconds())

	// Find SMSF portfolio or use first portfolio
	var portfolioID int
	var portfolioName string
	for _, p := range portfolios {
		name, _ := p["name"].(string)
		if strings.Contains(strings.ToUpper(name), "SMSF") {
			portfolioID = int(p["id"].(float64))
			portfolioName = name
			break
		}
	}
	if portfolioID == 0 {
		// Fall back to first portfolio
		portfolioID = int(portfolios[0]["id"].(float64))
		portfolioName = portfolios[0]["name"].(string)
	}

	testLog = append(testLog, fmt.Sprintf("[%s] Using portfolio: %d (%s)", time.Now().Format(time.RFC3339), portfolioID, portfolioName))
	t.Logf("Testing portfolio review for portfolio %d (%s)", portfolioID, portfolioName)

	// Run the review test
	runNavexaPortfolioReviewTest(t, env, helper, portfolioID, portfolioName)
}
