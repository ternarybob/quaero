// -----------------------------------------------------------------------
// Tests for signal_computer worker
// Computes signals using market_fundamentals and signal_computer
// -----------------------------------------------------------------------

package market_workers

import (
	"encoding/json"
	"fmt"
	"io"
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
// Signal Worker Integration Tests
// =============================================================================
// Tests the signal computation pipeline:
// 1. signal_computer - Computes PBAS, VLI, Regime, Cooked, RS signals from stock data
// 2. portfolio_rollup - Aggregates ticker signals into portfolio metrics
// 3. ai_assessor - Generates AI assessments with validation
//
// IMPORTANT: These tests require:
// - Valid eodhd_api_key in KV storage for stock data
// - LLM service for AI assessor tests
//
// Test stocks (using exchange-qualified tickers):
// - ASX:GNP - GenusPlus Group Ltd (infrastructure)
// - ASX:BCN - Beacon Minerals (mining)
// - ASX:MYG - Mayfield Childcare (services)
// =============================================================================

const (
	eodhdDefaultBaseURL = "https://eodhd.com/api"
	// Exchange-qualified tickers (ASX:XXX format)
	testStockGNP = "ASX:GNP"
	testStockBCN = "ASX:BCN"
	testStockMYG = "ASX:MYG"
	// Stock codes (for EODHD API which uses XXX.AU format)
	testCodeGNP = "GNP"
	testCodeBCN = "BCN"
	testCodeMYG = "MYG"
)

var testStocks = []string{testStockGNP, testStockBCN, testStockMYG}
var testCodes = []string{testCodeGNP, testCodeBCN, testCodeMYG}

// =============================================================================
// Public Test Functions
// =============================================================================

// TestSignalComputerWorker tests the signal_computer worker with multiple stocks
func TestSignalComputerWorker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	t.Logf("[%s] Test started: TestSignalComputerWorker", time.Now().Format(time.RFC3339))

	// Check EODHD API key
	apiKey := common.GetKVValue(t, helper, "eodhd_api_key")
	if apiKey == "" {
		t.Logf("[%s] SKIP: EODHD API key not configured", time.Now().Format(time.RFC3339))
		t.Skip("EODHD API key not configured - skipping test")
	}
	t.Logf("[%s] EODHD API key loaded from KV store", time.Now().Format(time.RFC3339))

	for i, stock := range testStocks {
		code := testCodes[i] // Use code for EODHD API, stock (ASX:XXX) for workers
		t.Run("ASX_"+code, func(t *testing.T) {
			subResultsDir := filepath.Join(resultsDir, code)
			if err := os.MkdirAll(subResultsDir, 0755); err != nil {
				t.Fatalf("Failed to create results dir: %v", err)
			}

			runSignalComputerTest(t, env, helper, stock, code, apiKey, subResultsDir)
		})
	}

	t.Logf("[%s] PASS: TestSignalComputerWorker completed", time.Now().Format(time.RFC3339))
}

// TestSignalComputerMultipleStocks tests signal computation for all test stocks in a single batch job.
// This test uses a multi-step job with market_fundamentals steps followed by a signal_computer step
// that processes all tickers together. It validates that the output format is the same as
// processing each ticker individually.
func TestSignalComputerMultipleStocks(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	resultsDir := env.GetResultsDir()

	t.Logf("[%s] Test started: TestSignalComputerMultipleStocks", time.Now().Format(time.RFC3339))

	// Check EODHD API key
	apiKey := common.GetKVValue(t, helper, "eodhd_api_key")
	if apiKey == "" {
		t.Logf("[%s] SKIP: EODHD API key not configured", time.Now().Format(time.RFC3339))
		t.Skip("EODHD API key not configured - skipping test")
	}
	t.Logf("[%s] EODHD API key loaded from KV store", time.Now().Format(time.RFC3339))

	// Step 1: Fetch EODHD data directly for validation baseline (same as single-stock test)
	t.Logf("[%s] Step 1: Fetching EODHD data for all stocks", time.Now().Format(time.RFC3339))
	for i, code := range testCodes {
		subDir := filepath.Join(resultsDir, code)
		os.MkdirAll(subDir, 0755)

		_, err := fetchEODHDFundamentals(t, subDir, code, apiKey)
		if err != nil {
			t.Logf("Warning: Failed to fetch EODHD fundamentals for %s: %v", code, err)
		}
		_, err = fetchEODHDHistorical(t, subDir, code, apiKey)
		if err != nil {
			t.Logf("Warning: Failed to fetch EODHD historical prices for %s: %v", code, err)
		}
		t.Logf("[%s] EODHD data fetched for %s", time.Now().Format(time.RFC3339), testStocks[i])
	}

	// Create job with multiple exchange-qualified tickers
	t.Logf("[%s] Step 2: Creating multi-stock job definition", time.Now().Format(time.RFC3339))
	defID := fmt.Sprintf("test-signal-multi-%d", time.Now().UnixNano())

	// Create interface slice for tickers to satisfy JSON/map types
	tickersInterface := make([]interface{}, len(testStocks))
	for i, v := range testStocks {
		tickersInterface[i] = v
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Signal Computer Multi-Stock Test",
		"description": "Compute signals for multiple ASX stocks",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"signal-multi-test"},
		"steps": []map[string]interface{}{
			{
				"name": "collect-all",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"tickers": tickersInterface,
					"period":  "Y2",
				},
			},
			{
				"name": "collect-bcn",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": testStockBCN, // Exchange-qualified ticker
					"period": "Y2",
				},
			},
			{
				"name": "collect-myg",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": testStockMYG, // Exchange-qualified ticker
					"period": "Y2",
				},
			},
			{
				"name":    "compute-signals",
				"type":    "signal_computer",
				"depends": "collect-all,collect-bcn,collect-myg",
				"config": map[string]interface{}{
					"tickers":     tickersInterface, // Exchange-qualified tickers
					"output_tags": []string{"signal-multi-test"},
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Multi-stock job started: %s", jobID)

	// Wait for completion with extended timeout
	finalStatus := WaitForJobCompletion(t, helper, jobID, 10*time.Minute)
	require.Equal(t, "completed", finalStatus, "Multi-stock job must complete successfully")

	// Get the signal_computer step's WorkerResult to validate by_ticker format
	// First find the compute-signals step job ID
	signalStepResult := getSignalComputerWorkerResult(t, helper, jobID)
	if signalStepResult != nil {
		// Validate by_ticker field exists and contains all test stocks
		require.NotNil(t, signalStepResult.ByTicker, "WorkerResult must have by_ticker field for multi-stock processing")
		require.Equal(t, len(testStocks), len(signalStepResult.ByTicker),
			"by_ticker should have entries for all %d stocks", len(testStocks))

		// Validate each stock has correct per-ticker result
		for _, stock := range testStocks {
			tickerResult, exists := signalStepResult.ByTicker[stock]
			require.True(t, exists, "by_ticker must contain entry for %s", stock)
			require.NotNil(t, tickerResult, "TickerResult for %s must not be nil", stock)
			assert.Equal(t, 1, tickerResult.DocumentsCreated,
				"Each stock should have exactly 1 document created, got %d for %s",
				tickerResult.DocumentsCreated, stock)
			assert.Len(t, tickerResult.DocumentIDs, 1,
				"Each stock should have exactly 1 document ID for %s", stock)
			assert.NotEmpty(t, tickerResult.Tags,
				"TickerResult for %s must have tags", stock)
			t.Logf("by_ticker[%s]: docs=%d, ids=%v, tags=%v",
				stock, tickerResult.DocumentsCreated, tickerResult.DocumentIDs, tickerResult.Tags)
		}

		// Validate totals match per-ticker sum
		totalDocs := 0
		for _, tr := range signalStepResult.ByTicker {
			totalDocs += tr.DocumentsCreated
		}
		assert.Equal(t, signalStepResult.DocumentsCreated, totalDocs,
			"Total documents_created (%d) should equal sum of per-ticker counts (%d)",
			signalStepResult.DocumentsCreated, totalDocs)

		// Save the WorkerResult with by_ticker for inspection
		resultPath := filepath.Join(resultsDir, "worker_result.json")
		if data, err := json.MarshalIndent(signalStepResult, "", "  "); err == nil {
			os.WriteFile(resultPath, data, 0644)
			t.Logf("Saved worker_result.json with by_ticker to: %s", resultPath)
		}
	} else {
		t.Error("Failed to get signal_computer WorkerResult from job metadata")
	}

	// Step 4: Save collector and signal output for each stock (same as single-stock test)
	t.Logf("[%s] Step 4: Saving output for each stock", time.Now().Format(time.RFC3339))
	for i, stock := range testStocks {
		code := testCodes[i]
		subDir := filepath.Join(resultsDir, code)
		os.MkdirAll(subDir, 0755)

		// Save collector output
		collectorDir := filepath.Join(subDir, "collector")
		os.MkdirAll(collectorDir, 0755)
		saveASXStockCollectorOutput(t, helper, collectorDir, code)

		// Save signal output
		signalsDir := filepath.Join(subDir, "signals")
		os.MkdirAll(signalsDir, 0755)
		saveSignalWorkerOutput(t, helper, subDir, code)

		jsonPath := filepath.Join(subDir, "output.json")
		if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
			t.Errorf("Signal document not created for %s", stock)
			t.Logf("[%s] ERROR: Signal document not created for %s", time.Now().Format(time.RFC3339), stock)
		} else {
			t.Logf("Signal document verified for %s", stock)
			t.Logf("[%s] Signal document verified for %s", time.Now().Format(time.RFC3339), stock)
		}
	}

	// Check for errors in service log
	AssertNoServiceErrors(t, env)
	t.Logf("[%s] PASS: TestSignalComputerMultipleStocks completed", time.Now().Format(time.RFC3339))
}

// =============================================================================
// EODHD API Direct Validation Helpers
// =============================================================================

// fetchEODHDFundamentals fetches fundamentals directly from EODHD API for validation
func fetchEODHDFundamentals(t *testing.T, resultsDir, stock, apiKey string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/fundamentals/%s.AU?api_token=%s&fmt=json", eodhdDefaultBaseURL, stock, apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Save raw response
	apiPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_fundamentals_%s.json", strings.ToLower(stock)))
	if err := os.WriteFile(apiPath, body, 0644); err != nil {
		t.Logf("Warning: failed to save EODHD response: %v", err)
	} else {
		t.Logf("Saved EODHD fundamentals to: %s", apiPath)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EODHD API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

// fetchEODHDHistorical fetches historical prices from EODHD API
func fetchEODHDHistorical(t *testing.T, resultsDir, stock, apiKey string) ([]map[string]interface{}, error) {
	fromDate := time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	url := fmt.Sprintf("%s/eod/%s.AU?api_token=%s&fmt=json&from=%s", eodhdDefaultBaseURL, stock, apiKey, fromDate)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Save raw response
	apiPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_eod_%s.json", strings.ToLower(stock)))
	if err := os.WriteFile(apiPath, body, 0644); err != nil {
		t.Logf("Warning: failed to save EODHD response: %v", err)
	} else {
		t.Logf("Saved EODHD historical prices to: %s", apiPath)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EODHD API returned status %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

// extractEODHDCompanyInfo extracts company info from EODHD fundamentals response
func extractEODHDCompanyInfo(fundamentals map[string]interface{}) (name, sector, industry string) {
	if general, ok := fundamentals["General"].(map[string]interface{}); ok {
		name, _ = general["Name"].(string)
		sector, _ = general["Sector"].(string)
		industry, _ = general["Industry"].(string)
	}
	return
}

// =============================================================================
// Test Signal Computer Worker Helpers
// =============================================================================

// runSignalComputerTest runs signal computer test for a single stock
// stock is the exchange-qualified ticker (e.g., "ASX:GNP")
// code is the stock code for EODHD API (e.g., "GNP")
func runSignalComputerTest(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, stock, code, apiKey, resultsDir string) {
	t.Logf("[%s] Testing %s", time.Now().Format(time.RFC3339), stock)

	// Step 1: Fetch EODHD data directly for validation baseline
	t.Logf("[%s] Step 1: Fetching EODHD data for validation", time.Now().Format(time.RFC3339))
	fundamentals, err := fetchEODHDFundamentals(t, resultsDir, code, apiKey)
	if err != nil {
		t.Logf("Warning: Failed to fetch EODHD fundamentals for %s: %v", code, err)
	}

	historicalPrices, err := fetchEODHDHistorical(t, resultsDir, code, apiKey)
	if err != nil {
		t.Logf("Warning: Failed to fetch EODHD historical prices for %s: %v", code, err)
	}

	// Extract company info from EODHD
	companyName, sector, industry := extractEODHDCompanyInfo(fundamentals)
	t.Logf("[%s] Company: %s, Sector: %s, Industry: %s",
		time.Now().Format(time.RFC3339), companyName, sector, industry)

	// Step 2: Run market_fundamentals first to get stock data
	t.Logf("[%s] Step 2: Running market_fundamentals worker", time.Now().Format(time.RFC3339))
	collectorDefID := fmt.Sprintf("test-signal-collector-%s-%d", strings.ToLower(code), time.Now().UnixNano())

	collectorBody := map[string]interface{}{
		"id":          collectorDefID,
		"name":        fmt.Sprintf("Stock Collector for Signal Test - %s", stock),
		"description": "Collect stock data for signal computation",
		"type":        "market_fundamentals",
		"enabled":     true,
		"tags":        []string{"signal-test", strings.ToLower(code)},
		"steps": []map[string]interface{}{
			{
				"name": "collect-data",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": stock, // Use exchange-qualified ticker
					"period": "Y2",
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", collectorBody)
	require.NoError(t, err, "Failed to create collector job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Collector job creation failed with status: %d", resp.StatusCode)
		return
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + collectorDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute collector job
	execResp, err := helper.POST("/api/job-definitions/"+collectorDefID+"/execute", nil)
	require.NoError(t, err, "Failed to execute collector job")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Logf("Collector job execution failed with status: %d", execResp.StatusCode)
		return
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	collectorJobID := execResult["job_id"].(string)
	t.Logf("Collector job started: %s", collectorJobID)

	// Wait for collector completion
	collectorStatus := WaitForJobCompletion(t, helper, collectorJobID, 3*time.Minute)
	t.Logf("Collector job status: %s", collectorStatus)

	if collectorStatus != "completed" {
		t.Logf("Collector job did not complete successfully: %s", collectorStatus)
		// Check for errors in job logs
		AssertNoJobErrors(t, helper, collectorJobID, "Collector")
		t.FailNow()
		return
	}

	// Validate collector WorkerResult
	collectorResult := GetJobWorkerResult(t, helper, collectorJobID)
	if collectorResult != nil {
		collectorResultDir := filepath.Join(resultsDir, "collector")
		if err := os.MkdirAll(collectorResultDir, 0755); err == nil {
			if !ValidateWorkerResult(t, helper, collectorResultDir, collectorResult, 1, nil) {
				t.Logf("Warning: Collector WorkerResult validation failed")
			}
		}
		t.Logf("Collector created %d documents", collectorResult.DocumentsCreated)
	} else {
		t.Logf("Warning: Collector did not return WorkerResult in job metadata")
	}

	// Check for errors in collector job logs
	AssertNoJobErrors(t, helper, collectorJobID, "Collector")

	// Step 3: Run signal_computer worker
	t.Logf("[%s] Step 3: Running signal_computer worker", time.Now().Format(time.RFC3339))
	signalDefID := fmt.Sprintf("test-signal-computer-%s-%d", strings.ToLower(code), time.Now().UnixNano())

	signalBody := map[string]interface{}{
		"id":          signalDefID,
		"name":        fmt.Sprintf("Signal Computer Test - %s", stock),
		"description": "Compute signals from stock data",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"signal-test", strings.ToLower(code)},
		"steps": []map[string]interface{}{
			{
				"name": "compute-signals",
				"type": "signal_computer",
				"config": map[string]interface{}{
					"ticker":      stock, // Use exchange-qualified ticker
					"output_tags": []string{"signal-test"},
				},
			},
		},
	}

	// Save job definition
	SaveJobDefinition(t, env, signalBody)

	signalResp, err := helper.POST("/api/job-definitions", signalBody)
	require.NoError(t, err, "Failed to create signal job definition")
	defer signalResp.Body.Close()

	if signalResp.StatusCode != http.StatusCreated {
		t.Logf("Signal job creation failed with status: %d", signalResp.StatusCode)
		return
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + signalDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute signal job
	signalExecResp, err := helper.POST("/api/job-definitions/"+signalDefID+"/execute", nil)
	require.NoError(t, err, "Failed to execute signal job")
	defer signalExecResp.Body.Close()

	if signalExecResp.StatusCode != http.StatusAccepted {
		t.Logf("Signal job execution failed with status: %d", signalExecResp.StatusCode)
		return
	}

	var signalExecResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(signalExecResp, &signalExecResult))
	signalJobID := signalExecResult["job_id"].(string)
	t.Logf("Signal job started: %s", signalJobID)

	// Wait for completion
	signalStatus := WaitForJobCompletion(t, helper, signalJobID, 2*time.Minute)
	t.Logf("Signal job status: %s", signalStatus)

	// Check for errors in signal job logs first
	AssertNoJobErrors(t, helper, signalJobID, "Signal")

	require.Equal(t, "completed", signalStatus, "Signal job must complete successfully")

	// Step 4: Validate WorkerResult from signal_computer
	t.Logf("[%s] Step 4: Validating signal WorkerResult", time.Now().Format(time.RFC3339))

	signalResult := GetJobWorkerResult(t, helper, signalJobID)
	requiredTags := []string{"ticker-signals", strings.ToLower(code)}
	if signalResult != nil {
		signalResultDir := filepath.Join(resultsDir, "signals")
		if err := os.MkdirAll(signalResultDir, 0755); err == nil {
			if !ValidateWorkerResult(t, helper, signalResultDir, signalResult, 1, requiredTags) {
				t.Errorf("Signal WorkerResult validation failed for %s", stock)
			}
		}
		t.Logf("Signal worker created %d documents with tags %v", signalResult.DocumentsCreated, signalResult.Tags)
	} else {
		t.Errorf("Signal worker did not return WorkerResult in job metadata for %s", stock)
	}

	// Step 5: Validate signal document content
	t.Logf("[%s] Step 5: Validating signal document content", time.Now().Format(time.RFC3339))
	saveSignalWorkerOutput(t, helper, resultsDir, code)
	validateSignalOutput(t, resultsDir, code, fundamentals, historicalPrices)

	// Check for errors in service log
	AssertNoServiceErrors(t, env)

	t.Logf("[%s] PASS: %s signal computation completed", time.Now().Format(time.RFC3339), stock)
}

// saveASXStockCollectorOutput saves market_fundamentals document output for a stock
func saveASXStockCollectorOutput(t *testing.T, helper *common.HTTPTestHelper, resultsDir, stock string) {
	tags := []string{"asx-stock-data", strings.ToLower(stock)}
	tagStr := strings.Join(tags, ",")

	resp, err := helper.GET("/api/documents?tags=" + tagStr + "&limit=1")
	if err != nil {
		t.Logf("Failed to query collector documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Collector document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse collector document response: %v", err)
		return
	}

	if len(result.Documents) == 0 {
		t.Logf("No collector documents found for %s with tags %v", stock, tags)
		return
	}

	doc := result.Documents[0]

	// Save markdown content
	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(doc.ContentMarkdown), 0644); err != nil {
		t.Logf("Failed to save collector markdown: %v", err)
	} else {
		t.Logf("Saved collector output.md to: %s", mdPath)
	}

	// Save metadata as JSON
	jsonPath := filepath.Join(resultsDir, "output.json")
	if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
		if err := os.WriteFile(jsonPath, data, 0644); err != nil {
			t.Logf("Failed to save collector JSON: %v", err)
		} else {
			t.Logf("Saved collector output.json to: %s", jsonPath)
		}
	}
}

// saveSignalWorkerOutput saves signal worker document output
func saveSignalWorkerOutput(t *testing.T, helper *common.HTTPTestHelper, resultsDir, stock string) {
	tags := []string{"ticker-signals", strings.ToLower(stock)}
	tagStr := strings.Join(tags, ",")

	resp, err := helper.GET("/api/documents?tags=" + tagStr + "&limit=1")
	if err != nil {
		t.Logf("Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse document response: %v", err)
		return
	}

	if len(result.Documents) == 0 {
		t.Logf("No signal documents found for %s", stock)
		return
	}

	doc := result.Documents[0]

	// Save output.md
	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(doc.ContentMarkdown), 0644); err != nil {
		t.Logf("Failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s", mdPath)
	}

	// Save output.json
	if doc.Metadata != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s", jsonPath)
			}
		}
	}
}

// validateSignalOutput validates signal output against EODHD data
func validateSignalOutput(t *testing.T, resultsDir, stock string, fundamentals map[string]interface{}, historicalPrices []map[string]interface{}) {
	jsonPath := filepath.Join(resultsDir, "output.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Errorf("Failed to read output.json: %v", err)
		return
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Errorf("Failed to parse output.json: %v", err)
		return
	}

	// ==========================================================================
	// SECTION 1: Validate required top-level fields exist
	// ==========================================================================
	assertFieldExists(t, metadata, "ticker")
	assertFieldExists(t, metadata, "computed_at")
	assertFieldExists(t, metadata, "pbas_score")
	assertFieldExists(t, metadata, "pbas_interpretation")
	assertFieldExists(t, metadata, "vli_score")
	assertFieldExists(t, metadata, "vli_label")
	assertFieldExists(t, metadata, "regime")
	assertFieldExists(t, metadata, "regime_confidence")
	assertFieldExists(t, metadata, "is_cooked")
	assertFieldExists(t, metadata, "cooked_score")
	assertFieldExists(t, metadata, "rs_rank")
	assertFieldExists(t, metadata, "quality_overall")
	assertFieldExists(t, metadata, "justified_expected")
	assertFieldExists(t, metadata, "justified_actual")
	assertFieldExists(t, metadata, "justified_diverge")
	assertFieldExists(t, metadata, "risk_flags")
	assertFieldExists(t, metadata, "signals")

	// ==========================================================================
	// SECTION 2: Validate PBAS signal schema
	// ==========================================================================
	if pbasScore, ok := metadata["pbas_score"].(float64); ok {
		assert.GreaterOrEqual(t, pbasScore, 0.0, "PBAS score must be >= 0")
		assert.LessOrEqual(t, pbasScore, 1.0, "PBAS score must be <= 1")
		t.Logf("PBAS score: %.2f", pbasScore)
	} else {
		t.Errorf("pbas_score must be a float64")
	}

	if pbasInterp, ok := metadata["pbas_interpretation"].(string); ok {
		validInterpretations := []string{"underpriced", "neutral", "overpriced"}
		assert.Contains(t, validInterpretations, pbasInterp, "PBAS interpretation must be valid")
		t.Logf("PBAS interpretation: %s", pbasInterp)
	} else {
		t.Errorf("pbas_interpretation must be a string")
	}

	// ==========================================================================
	// SECTION 3: Validate VLI signal schema
	// ==========================================================================
	if vliScore, ok := metadata["vli_score"].(float64); ok {
		assert.GreaterOrEqual(t, vliScore, -1.0, "VLI score must be >= -1")
		assert.LessOrEqual(t, vliScore, 1.0, "VLI score must be <= 1")
		t.Logf("VLI score: %.2f", vliScore)
	} else {
		t.Errorf("vli_score must be a float64")
	}

	if vliLabel, ok := metadata["vli_label"].(string); ok {
		validLabels := []string{"accumulating", "distributing", "neutral"}
		assert.Contains(t, validLabels, vliLabel, "VLI label must be valid")
		t.Logf("VLI label: %s", vliLabel)
	} else {
		t.Errorf("vli_label must be a string")
	}

	// ==========================================================================
	// SECTION 4: Validate Regime signal schema
	// ==========================================================================
	if regime, ok := metadata["regime"].(string); ok {
		validRegimes := []string{"breakout", "trend_up", "trend_down", "accumulation", "distribution", "range", "decay", "undefined"}
		assert.Contains(t, validRegimes, regime, "Regime must be valid")
		t.Logf("Regime: %s", regime)
	} else {
		t.Errorf("regime must be a string")
	}

	if regimeConf, ok := metadata["regime_confidence"].(float64); ok {
		assert.GreaterOrEqual(t, regimeConf, 0.0, "Regime confidence must be >= 0")
		assert.LessOrEqual(t, regimeConf, 1.0, "Regime confidence must be <= 1")
		t.Logf("Regime confidence: %.2f", regimeConf)
	} else {
		t.Errorf("regime_confidence must be a float64")
	}

	// ==========================================================================
	// SECTION 5: Validate Cooked signal schema
	// ==========================================================================
	if cookedScore, ok := metadata["cooked_score"].(float64); ok {
		assert.GreaterOrEqual(t, int(cookedScore), 0, "Cooked score must be >= 0")
		assert.LessOrEqual(t, int(cookedScore), 5, "Cooked score must be <= 5")
		t.Logf("Cooked score: %d", int(cookedScore))
	} else {
		t.Errorf("cooked_score must be a float64")
	}

	// is_cooked must be a boolean
	if _, ok := metadata["is_cooked"].(bool); !ok {
		t.Errorf("is_cooked must be a boolean")
	}

	// ==========================================================================
	// SECTION 6: Validate RS signal schema
	// ==========================================================================
	if rsRank, ok := metadata["rs_rank"].(float64); ok {
		assert.GreaterOrEqual(t, int(rsRank), 0, "RS rank must be >= 0")
		assert.LessOrEqual(t, int(rsRank), 100, "RS rank must be <= 100")
		t.Logf("RS rank: %d percentile", int(rsRank))
	} else {
		t.Errorf("rs_rank must be a float64")
	}

	// ==========================================================================
	// SECTION 7: Validate Quality signal schema
	// ==========================================================================
	if quality, ok := metadata["quality_overall"].(string); ok {
		validQualities := []string{"good", "fair", "poor"}
		assert.Contains(t, validQualities, quality, "Quality must be valid label (good/fair/poor)")
		t.Logf("Quality overall: %s", quality)
	} else {
		t.Errorf("quality_overall must be a string")
	}

	// ==========================================================================
	// SECTION 8: Validate Justified Return signal schema
	// ==========================================================================
	if _, ok := metadata["justified_expected"].(float64); !ok {
		t.Errorf("justified_expected must be a float64")
	}
	if _, ok := metadata["justified_actual"].(float64); !ok {
		t.Errorf("justified_actual must be a float64")
	}
	if _, ok := metadata["justified_diverge"].(float64); !ok {
		t.Errorf("justified_diverge must be a float64")
	}

	// ==========================================================================
	// SECTION 9: Validate the full signals object with nested signal structs
	// ==========================================================================
	if signals, ok := metadata["signals"].(map[string]interface{}); ok {
		validateSignalsObject(t, signals)
	} else {
		t.Errorf("signals must be a map containing all computed signals")
	}

	// ==========================================================================
	// SECTION 10: Validate output.md has expected sections and new format
	// ==========================================================================
	mdPath := filepath.Join(resultsDir, "output.md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Errorf("Failed to read output.md: %v", err)
		return
	}

	mdContent := string(mdData)

	// Validate section headers exist
	assert.Contains(t, mdContent, "# Signal Analysis:", "output.md must have Signal Analysis header")
	assert.Contains(t, mdContent, "## PBAS", "output.md must have PBAS section")
	assert.Contains(t, mdContent, "## VLI", "output.md must have VLI section")
	assert.Contains(t, mdContent, "## Regime", "output.md must have Regime section")
	assert.Contains(t, mdContent, "## Cooked Status", "output.md must have Cooked Status section")
	assert.Contains(t, mdContent, "## Relative Strength", "output.md must have Relative Strength section")
	assert.Contains(t, mdContent, "## Quality Assessment", "output.md must have Quality Assessment section")
	assert.Contains(t, mdContent, "## Justified Return", "output.md must have Justified Return section")
	assert.Contains(t, mdContent, "## Risk Flags", "output.md must have Risk Flags section")

	// Validate descriptions are present (italicized text under section headers)
	assert.Contains(t, mdContent, "*Measures alignment between business fundamentals", "PBAS section must have description")
	assert.Contains(t, mdContent, "*Detects institutional accumulation or distribution", "VLI section must have description")
	assert.Contains(t, mdContent, "*Classifies the current price action phase", "Regime section must have description")
	assert.Contains(t, mdContent, "*Identifies stocks that may be overvalued", "Cooked section must have description")
	assert.Contains(t, mdContent, "*Measures price performance relative to the ASX 200", "RS section must have description")
	assert.Contains(t, mdContent, "*Assesses business quality", "Quality section must have description")
	assert.Contains(t, mdContent, "*Compares expected return", "Justified Return section must have description")
	assert.Contains(t, mdContent, "*Aggregated list of risk indicators", "Risk Flags section must have description")

	// Validate AI Review comments are present (blockquote format)
	assert.Contains(t, mdContent, "> **AI Review**:", "output.md must have AI Review blockquotes")

	t.Logf("PASS: Signal output validation completed for %s", stock)
}

// validateSignalsObject validates the nested signals structure from metadata
func validateSignalsObject(t *testing.T, signals map[string]interface{}) {
	// Validate PBAS signal struct
	if pbas, ok := signals["pbas"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, pbas, "score", "pbas")
		assertFieldExistsInMap(t, pbas, "business_momentum", "pbas")
		assertFieldExistsInMap(t, pbas, "price_momentum", "pbas")
		assertFieldExistsInMap(t, pbas, "divergence", "pbas")
		assertFieldExistsInMap(t, pbas, "interpretation", "pbas")
		assertFieldExistsInMap(t, pbas, "description", "pbas")
		assertFieldExistsInMap(t, pbas, "comment", "pbas")

		// Validate description is non-empty
		if desc, ok := pbas["description"].(string); ok {
			assert.NotEmpty(t, desc, "PBAS description must not be empty")
		}
		// Validate comment is non-empty
		if comment, ok := pbas["comment"].(string); ok {
			assert.NotEmpty(t, comment, "PBAS comment must not be empty")
		}
	} else {
		t.Errorf("signals.pbas must be a map")
	}

	// Validate VLI signal struct
	if vli, ok := signals["vli"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, vli, "score", "vli")
		assertFieldExistsInMap(t, vli, "label", "vli")
		assertFieldExistsInMap(t, vli, "vol_zscore", "vli")
		assertFieldExistsInMap(t, vli, "price_vs_vwap", "vli")
		assertFieldExistsInMap(t, vli, "description", "vli")
		assertFieldExistsInMap(t, vli, "comment", "vli")

		if desc, ok := vli["description"].(string); ok {
			assert.NotEmpty(t, desc, "VLI description must not be empty")
		}
		if comment, ok := vli["comment"].(string); ok {
			assert.NotEmpty(t, comment, "VLI comment must not be empty")
		}
	} else {
		t.Errorf("signals.vli must be a map")
	}

	// Validate Regime signal struct
	if regime, ok := signals["regime"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, regime, "classification", "regime")
		assertFieldExistsInMap(t, regime, "confidence", "regime")
		assertFieldExistsInMap(t, regime, "trend_bias", "regime")
		assertFieldExistsInMap(t, regime, "ema_stack", "regime")
		assertFieldExistsInMap(t, regime, "description", "regime")
		assertFieldExistsInMap(t, regime, "comment", "regime")

		if desc, ok := regime["description"].(string); ok {
			assert.NotEmpty(t, desc, "Regime description must not be empty")
		}
		if comment, ok := regime["comment"].(string); ok {
			assert.NotEmpty(t, comment, "Regime comment must not be empty")
		}
	} else {
		t.Errorf("signals.regime must be a map")
	}

	// Validate Cooked signal struct
	if cooked, ok := signals["cooked"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, cooked, "is_cooked", "cooked")
		assertFieldExistsInMap(t, cooked, "score", "cooked")
		assertFieldExistsInMap(t, cooked, "description", "cooked")
		assertFieldExistsInMap(t, cooked, "comment", "cooked")

		if desc, ok := cooked["description"].(string); ok {
			assert.NotEmpty(t, desc, "Cooked description must not be empty")
		}
		if comment, ok := cooked["comment"].(string); ok {
			assert.NotEmpty(t, comment, "Cooked comment must not be empty")
		}
	} else {
		t.Errorf("signals.cooked must be a map")
	}

	// Validate RS signal struct
	if rs, ok := signals["relative_strength"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, rs, "vs_xjo_3m", "relative_strength")
		assertFieldExistsInMap(t, rs, "vs_xjo_6m", "relative_strength")
		assertFieldExistsInMap(t, rs, "rs_rank_percentile", "relative_strength")
		assertFieldExistsInMap(t, rs, "description", "relative_strength")
		assertFieldExistsInMap(t, rs, "comment", "relative_strength")

		if desc, ok := rs["description"].(string); ok {
			assert.NotEmpty(t, desc, "RS description must not be empty")
		}
		if comment, ok := rs["comment"].(string); ok {
			assert.NotEmpty(t, comment, "RS comment must not be empty")
		}
	} else {
		t.Errorf("signals.relative_strength must be a map")
	}

	// Validate Quality signal struct
	if quality, ok := signals["quality"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, quality, "overall", "quality")
		assertFieldExistsInMap(t, quality, "cash_conversion", "quality")
		assertFieldExistsInMap(t, quality, "balance_sheet_risk", "quality")
		assertFieldExistsInMap(t, quality, "margin_trend", "quality")
		assertFieldExistsInMap(t, quality, "description", "quality")
		assertFieldExistsInMap(t, quality, "comment", "quality")

		if desc, ok := quality["description"].(string); ok {
			assert.NotEmpty(t, desc, "Quality description must not be empty")
		}
		if comment, ok := quality["comment"].(string); ok {
			assert.NotEmpty(t, comment, "Quality comment must not be empty")
		}
	} else {
		t.Errorf("signals.quality must be a map")
	}

	// Validate JustifiedReturn signal struct
	if justified, ok := signals["justified_return"].(map[string]interface{}); ok {
		assertFieldExistsInMap(t, justified, "expected_12m_pct", "justified_return")
		assertFieldExistsInMap(t, justified, "actual_12m_pct", "justified_return")
		assertFieldExistsInMap(t, justified, "divergence_pct", "justified_return")
		assertFieldExistsInMap(t, justified, "interpretation", "justified_return")
		assertFieldExistsInMap(t, justified, "description", "justified_return")
		assertFieldExistsInMap(t, justified, "comment", "justified_return")

		if desc, ok := justified["description"].(string); ok {
			assert.NotEmpty(t, desc, "JustifiedReturn description must not be empty")
		}
		if comment, ok := justified["comment"].(string); ok {
			assert.NotEmpty(t, comment, "JustifiedReturn comment must not be empty")
		}
	} else {
		t.Errorf("signals.justified_return must be a map")
	}

	// Validate risk_flags exists (can be empty array)
	if riskFlags, ok := signals["risk_flags"]; ok {
		if _, isSlice := riskFlags.([]interface{}); !isSlice {
			// May be nil which is acceptable
			if riskFlags != nil {
				t.Logf("Warning: signals.risk_flags is not an array (got %T)", riskFlags)
			}
		}
	}

	// Validate risk_flags_description exists
	if desc, ok := signals["risk_flags_description"].(string); ok {
		assert.NotEmpty(t, desc, "risk_flags_description must not be empty")
	} else {
		t.Errorf("signals.risk_flags_description must be a string")
	}
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// assertFieldExistsInMap asserts that a field exists in a nested map
func assertFieldExistsInMap(t *testing.T, m map[string]interface{}, field, parent string) {
	_, ok := m[field]
	assert.True(t, ok, "Field '%s.%s' must exist in metadata", parent, field)
}

// assertFieldExists asserts that a field exists in metadata
func assertFieldExists(t *testing.T, metadata map[string]interface{}, field string) {
	_, ok := metadata[field]
	assert.True(t, ok, "Field '%s' must exist in metadata", field)
}

// assertFileExistsAndNotEmpty verifies file exists and has content
func assertFileExistsAndNotEmpty(t *testing.T, path string) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("File does not exist: %s", path)
		return
	}
	require.NoError(t, err, "Failed to stat file: %s", path)
	assert.Greater(t, info.Size(), int64(0), "File should not be empty: %s", path)
}

// =============================================================================
// WorkerResult Validation Helpers
// =============================================================================

// getSignalComputerWorkerResult retrieves the signal_computer step's WorkerResult from a multi-step job.
// For manager jobs, it looks up the "compute-signals" step job ID and queries that.
func getSignalComputerWorkerResult(t *testing.T, helper *common.HTTPTestHelper, jobID string) *WorkerResult {
	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Logf("Failed to get job %s: %v", jobID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Get job returned status %d", resp.StatusCode)
		return nil
	}

	var job struct {
		Type     string                 `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		t.Logf("Failed to parse job response: %v", err)
		return nil
	}

	if job.Metadata == nil {
		t.Logf("Job %s has no metadata", jobID)
		return nil
	}

	// For manager job, find the compute-signals step
	if job.Type == "manager" {
		stepJobIDs, ok := job.Metadata["step_job_ids"].(map[string]interface{})
		if !ok || len(stepJobIDs) == 0 {
			t.Logf("Manager job %s has no step_job_ids in metadata", jobID)
			return nil
		}

		// Look for compute-signals step
		computeSignalsJobID, ok := stepJobIDs["compute-signals"].(string)
		if !ok {
			t.Logf("Manager job %s has no compute-signals step in step_job_ids", jobID)
			return nil
		}

		// t.Logf("Querying compute-signals step job %s", computeSignalsJobID)
		return GetJobWorkerResult(t, helper, computeSignalsJobID)
	}

	// Not a manager job, try to get worker_result directly
	return GetJobWorkerResult(t, helper, jobID)
}
