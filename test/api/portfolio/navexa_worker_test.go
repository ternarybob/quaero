// =============================================================================
// Navexa Worker Integration Tests
// =============================================================================
// Tests the Navexa portfolio integration workers:
// 1. navexa_portfolios - Fetch all user portfolios
// 2. navexa_holdings - Fetch holdings for a portfolio
// 3. navexa_performance - Fetch P/L performance data
//
// IMPORTANT: These tests require a valid navexa_api_key in KV storage.
// If not configured, tests will skip gracefully.
// =============================================================================

package portfolio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestWorkerNavexaPortfolios tests the navexa_portfolios worker
func TestWorkerNavexaPortfolios(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := common.GetTestResultsDir("worker", t.Name())
	common.EnsureResultsDir(t, resultsDir)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestWorkerNavexaPortfolios", time.Now().Format(time.RFC3339)))

	// Check if API key is configured
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, resultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping test")
	}
	testLog = append(testLog, fmt.Sprintf("[%s] API key loaded from KV store", time.Now().Format(time.RFC3339)))

	// Get base URL from KV store (or use default)
	baseURL := GetNavexaBaseURL(t, helper)
	testLog = append(testLog, fmt.Sprintf("[%s] Using base URL: %s", time.Now().Format(time.RFC3339), baseURL))

	// Step 1: Validate direct API call works
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Validating direct Navexa API call", time.Now().Format(time.RFC3339)))
	portfolios, err := FetchAndValidateNavexaAPI(t, resultsDir, baseURL, apiKey)
	require.NoError(t, err, "Direct Navexa API call must succeed")
	require.NotNil(t, portfolios, "Navexa API must return valid JSON array")
	testLog = append(testLog, fmt.Sprintf("[%s] Direct API call succeeded: %d portfolios", time.Now().Format(time.RFC3339), len(portfolios)))
	timingData.AddStepTiming("navexa_api_call", time.Since(stepStart).Seconds())

	// Step 2: Run worker job
	testLog = append(testLog, fmt.Sprintf("[%s] Step 2: Creating job definition", time.Now().Format(time.RFC3339)))
	defID := fmt.Sprintf("test-navexa-portfolios-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Navexa Portfolios Worker Test",
		"description": "Test navexa_portfolios worker",
		"type":        "navexa_portfolios",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-portfolios",
				"type": "navexa_portfolios",
			},
		},
	}

	// Save job definition
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if data, err := json.MarshalIndent(body, "", "  "); err == nil {
		os.WriteFile(defPath, data, 0644)
	}

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Job definition creation must succeed")
	testLog = append(testLog, fmt.Sprintf("[%s] Job definition created: %s", time.Now().Format(time.RFC3339), defID))

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute job
	stepStart = time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 3: Executing job", time.Now().Format(time.RFC3339)))
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job")
	defer execResp.Body.Close()

	require.Equal(t, http.StatusAccepted, execResp.StatusCode, "Job execution must be accepted")

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))
	t.Logf("Executed navexa_portfolios job: %s", jobID)
	timingData.AddStepTiming("job_trigger", time.Since(stepStart).Seconds())

	// Wait for completion
	stepStart = time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 4: Waiting for job completion", time.Now().Format(time.RFC3339)))
	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	timingData.AddStepTiming("job_execution", time.Since(stepStart).Seconds())

	// CRITICAL: Job MUST complete successfully
	testLog = append(testLog, fmt.Sprintf("[%s] Job final status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "Job must complete successfully - got status: %s", finalStatus)

	// Step 5: Validate document was created
	testLog = append(testLog, fmt.Sprintf("[%s] Step 5: Validating document creation", time.Now().Format(time.RFC3339)))
	_, _ = SaveNavexaWorkerOutput(t, helper, resultsDir, "navexa-portfolio")

	docResp, err := helper.GET("/api/documents?tags=navexa-portfolio&limit=1")
	require.NoError(t, err)
	defer docResp.Body.Close()

	var docResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(docResp, &docResult))
	require.NotEmpty(t, docResult.Documents, "Worker must create document with navexa-portfolio tag")

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Document created with navexa-portfolio tag", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestWorkerNavexaPortfolios completed successfully", time.Now().Format(time.RFC3339)))

	WriteTestLog(t, resultsDir, testLog)

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("PASS: TestWorkerNavexaPortfolios completed successfully")
}

// TestWorkerNavexaHoldings tests the navexa_holdings worker
// This test fetches portfolios first to get a valid portfolio ID
func TestWorkerNavexaHoldings(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := common.GetTestResultsDir("worker", t.Name())
	common.EnsureResultsDir(t, resultsDir)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestWorkerNavexaHoldings", time.Now().Format(time.RFC3339)))

	// Check if API key is configured
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, resultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping test")
	}

	// Get base URL from KV store (or use default)
	baseURL := GetNavexaBaseURL(t, helper)

	// First, get portfolios directly from API to get a valid portfolio ID
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Fetching portfolios from Navexa API", time.Now().Format(time.RFC3339)))
	portfolios, err := FetchAndValidateNavexaAPI(t, resultsDir, baseURL, apiKey)
	require.NoError(t, err, "Failed to fetch portfolios from Navexa API")
	require.NotEmpty(t, portfolios, "Must have at least one portfolio to test holdings")
	timingData.AddStepTiming("fetch_portfolios", time.Since(stepStart).Seconds())

	firstPortfolio := portfolios[0]
	portfolioID := int(firstPortfolio["id"].(float64))
	portfolioName := firstPortfolio["name"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Using portfolio: %d (%s)", time.Now().Format(time.RFC3339), portfolioID, portfolioName))
	t.Logf("Step 2: Testing holdings for portfolio %d (%s)", portfolioID, portfolioName)

	// Test holdings worker
	holdingsDefID := fmt.Sprintf("test-navexa-holdings-%d", time.Now().UnixNano())
	holdingsBody := map[string]interface{}{
		"id":          holdingsDefID,
		"name":        "Navexa Holdings Worker Test",
		"description": "Test navexa_holdings worker",
		"type":        "navexa_holdings",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-holdings",
				"type": "navexa_holdings",
				"config": map[string]interface{}{
					"portfolio_id":   portfolioID,
					"portfolio_name": portfolioName,
				},
			},
		},
	}

	// Save job definition
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if data, err := json.MarshalIndent(holdingsBody, "", "  "); err == nil {
		os.WriteFile(defPath, data, 0644)
	}

	resp, err := helper.POST("/api/job-definitions", holdingsBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Holdings job definition creation must succeed")

	testLog = append(testLog, fmt.Sprintf("[%s] Job definition created: %s", time.Now().Format(time.RFC3339), holdingsDefID))

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + holdingsDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	stepStart = time.Now()
	execResp, err := helper.POST("/api/job-definitions/"+holdingsDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()
	require.Equal(t, http.StatusAccepted, execResp.StatusCode, "Holdings job execution must be accepted")

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))
	t.Logf("Executed navexa_holdings job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	timingData.AddStepTiming("job_execution", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job final status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "Holdings job must complete successfully - got status: %s", finalStatus)

	// Validate document was created
	_, _ = SaveNavexaWorkerOutput(t, helper, resultsDir, "navexa-holdings")

	holdingsDocResp, err := helper.GET("/api/documents?tags=navexa-holdings&limit=1")
	require.NoError(t, err)
	defer holdingsDocResp.Body.Close()

	var holdingsDocResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(holdingsDocResp, &holdingsDocResult))
	require.NotEmpty(t, holdingsDocResult.Documents, "Worker must create document with navexa-holdings tag")

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Document created with navexa-holdings tag", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestWorkerNavexaHoldings completed successfully", time.Now().Format(time.RFC3339)))

	WriteTestLog(t, resultsDir, testLog)

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("PASS: TestWorkerNavexaHoldings completed successfully")
}

// TestWorkerNavexaPerformance tests the navexa_performance worker
// This test fetches portfolios first to get a valid portfolio ID
func TestWorkerNavexaPerformance(t *testing.T) {
	// Initialize timing data
	timingData := common.NewTestTimingData(t.Name())

	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := common.GetTestResultsDir("worker", t.Name())
	common.EnsureResultsDir(t, resultsDir)

	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: TestWorkerNavexaPerformance", time.Now().Format(time.RFC3339)))

	// Check if API key is configured
	apiKey := GetNavexaAPIKey(t, helper)
	if apiKey == "" {
		testLog = append(testLog, fmt.Sprintf("[%s] SKIP: Navexa API key not configured", time.Now().Format(time.RFC3339)))
		WriteTestLog(t, resultsDir, testLog)
		t.Skip("Navexa API key not configured - skipping test")
	}

	// Get base URL from KV store (or use default)
	baseURL := GetNavexaBaseURL(t, helper)

	// First, get portfolios directly from API
	stepStart := time.Now()
	testLog = append(testLog, fmt.Sprintf("[%s] Step 1: Fetching portfolios from Navexa API", time.Now().Format(time.RFC3339)))
	portfolios, err := FetchAndValidateNavexaAPI(t, resultsDir, baseURL, apiKey)
	require.NoError(t, err, "Failed to fetch portfolios from Navexa API")
	require.NotEmpty(t, portfolios, "Must have at least one portfolio to test performance")
	timingData.AddStepTiming("fetch_portfolios", time.Since(stepStart).Seconds())

	firstPortfolio := portfolios[0]
	portfolioID := int(firstPortfolio["id"].(float64))
	portfolioName := firstPortfolio["name"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Using portfolio: %d (%s)", time.Now().Format(time.RFC3339), portfolioID, portfolioName))
	t.Logf("Step 2: Testing performance for portfolio %d (%s)", portfolioID, portfolioName)

	// Test performance worker
	perfDefID := fmt.Sprintf("test-navexa-performance-%d", time.Now().UnixNano())
	perfBody := map[string]interface{}{
		"id":          perfDefID,
		"name":        "Navexa Performance Worker Test",
		"description": "Test navexa_performance worker",
		"type":        "navexa_performance",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-performance",
				"type": "navexa_performance",
				"config": map[string]interface{}{
					"portfolio_id":   portfolioID,
					"portfolio_name": portfolioName,
				},
			},
		},
	}

	// Save job definition
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if data, err := json.MarshalIndent(perfBody, "", "  "); err == nil {
		os.WriteFile(defPath, data, 0644)
	}

	resp, err := helper.POST("/api/job-definitions", perfBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Performance job definition creation must succeed")

	testLog = append(testLog, fmt.Sprintf("[%s] Job definition created: %s", time.Now().Format(time.RFC3339), perfDefID))

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + perfDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	stepStart = time.Now()
	execResp, err := helper.POST("/api/job-definitions/"+perfDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()
	require.Equal(t, http.StatusAccepted, execResp.StatusCode, "Performance job execution must be accepted")

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	testLog = append(testLog, fmt.Sprintf("[%s] Job started: %s", time.Now().Format(time.RFC3339), jobID))
	t.Logf("Executed navexa_performance job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	timingData.AddStepTiming("job_execution", time.Since(stepStart).Seconds())
	testLog = append(testLog, fmt.Sprintf("[%s] Job final status: %s", time.Now().Format(time.RFC3339), finalStatus))
	require.Equal(t, "completed", finalStatus, "Performance job must complete successfully - got status: %s", finalStatus)

	// Validate document was created
	_, _ = SaveNavexaWorkerOutput(t, helper, resultsDir, "navexa-performance")

	perfDocResp, err := helper.GET("/api/documents?tags=navexa-performance&limit=1")
	require.NoError(t, err)
	defer perfDocResp.Body.Close()

	var perfDocResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(perfDocResp, &perfDocResult))
	require.NotEmpty(t, perfDocResult.Documents, "Worker must create document with navexa-performance tag")

	// Validate markdown content has real data (not formatting bugs or all zeros)
	perfDoc := perfDocResult.Documents[0]
	markdown, ok := perfDoc["content_markdown"].(string)
	require.True(t, ok, "Document must have content_markdown field")
	require.NotEmpty(t, markdown, "content_markdown must not be empty")

	// Assert no formatting bugs like "$%!,(float64=0).2f"
	require.NotContains(t, markdown, "%!", "Markdown must not contain Go format errors")
	require.NotContains(t, markdown, "float64", "Markdown must not contain raw type names")

	// Assert portfolio summary contains real non-zero values
	require.Contains(t, markdown, "Total Value", "Markdown must have Total Value row")
	require.Regexp(t, `Total Value \| \$[1-9][0-9,]*`, markdown, "Total Value must be a non-zero dollar amount")
	require.Regexp(t, `Cost Basis \| \$[1-9][0-9,]*`, markdown, "Cost Basis must be a non-zero dollar amount")

	// Assert holdings have real values (at least one holding with value > $0)
	require.Regexp(t, `\| [A-Z]+ \| .+ \| \$[1-9][0-9,]* \|`, markdown, "At least one holding must have non-zero value")

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Document created with navexa-performance tag", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Markdown contains real non-zero values (no formatting bugs)", time.Now().Format(time.RFC3339)))
	testLog = append(testLog, fmt.Sprintf("[%s] PASS: TestWorkerNavexaPerformance completed successfully", time.Now().Format(time.RFC3339)))

	WriteTestLog(t, resultsDir, testLog)

	// Complete timing and save
	timingData.Complete()
	common.SaveTimingData(t, resultsDir, timingData)

	// Check for errors in service log
	common.AssertNoErrorsInServiceLog(t, env)

	// Copy TDD summary if running from /3agents-tdd
	common.CopyTDDSummary(t, resultsDir)

	t.Log("PASS: TestWorkerNavexaPerformance completed successfully")
}
