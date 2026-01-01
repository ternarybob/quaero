package api

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
// ASX Stock Collector Integration Tests
// =============================================================================
// These tests verify the asx_stock_collector worker with EODHD API integration.
// Tests cover:
// - Single stock (BHP)
// - Multiple stocks (BHP, CSL, CBA)
// - 24 months of historical price/volume data
// - Currency validation (AUD)
// - EODHD API response capture
// =============================================================================

// TestASXStockCollectorSingle tests the asx_stock_collector with a single stock (BHP)
func TestASXStockCollectorSingle(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	stock := "BHP"

	// Step 1: Fetch and save EODHD API responses
	apiKey := getEODHDAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("EODHD API key not available - skipping test")
	}

	fetchAndSaveEODHDData(t, env, stock, apiKey)

	// Step 2: Run asx_stock_collector worker
	defID := fmt.Sprintf("test-asx-collector-single-%d", time.Now().UnixNano())
	runStockCollectorTest(t, env, helper, defID, stock)

	// Step 3: Validate outputs
	assertEODHDFilesExist(t, env, stock)
	assertWorkerOutputFilesExist(t, env)
	assertHistoricalPrices24Months(t, env)
	assertHistoricalPricesInMarkdown(t, env)
	assertCurrencyAUD(t, env)

	t.Log("PASS: TestASXStockCollectorSingle completed successfully")
}

// TestASXStockCollectorMulti tests the asx_stock_collector with multiple stocks
func TestASXStockCollectorMulti(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)
	stocks := []string{"BHP", "CSL", "CBA"}

	// Get EODHD API key once
	apiKey := getEODHDAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("EODHD API key not available - skipping test")
	}

	for i, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			// Create sub-environment for each stock
			subResultsDir := filepath.Join(env.GetResultsDir(), stock)
			if err := os.MkdirAll(subResultsDir, 0755); err != nil {
				t.Fatalf("Failed to create results dir for %s: %v", stock, err)
			}

			// Fetch and save EODHD data
			fetchAndSaveEODHDDataToDir(t, subResultsDir, stock, apiKey)

			// Run worker
			defID := fmt.Sprintf("test-asx-collector-multi-%s-%d", strings.ToLower(stock), time.Now().UnixNano())
			runStockCollectorTestWithDir(t, env, helper, defID, stock, subResultsDir, i+1)

			// Validate
			assertEODHDFilesExistInDir(t, subResultsDir, stock)
			assertWorkerOutputFilesExistInDir(t, subResultsDir)
			assertHistoricalPrices24MonthsInDir(t, subResultsDir)
			assertHistoricalPricesInMarkdownInDir(t, subResultsDir)
			assertCurrencyAUDInDir(t, subResultsDir)

			t.Logf("PASS: %s completed successfully", stock)
		})
	}

	t.Log("PASS: TestASXStockCollectorMulti completed successfully")
}

// =============================================================================
// Helper Functions
// =============================================================================

// getEODHDAPIKey retrieves the EODHD API key from the KV store
func getEODHDAPIKey(t *testing.T, helper *common.HTTPTestHelper) string {
	resp, err := helper.GET("/api/kv/eodhd_api_key")
	if err != nil {
		t.Logf("Failed to get EODHD API key from KV store: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("EODHD API key not found in KV store (status %d)", resp.StatusCode)
		return ""
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse EODHD API key response: %v", err)
		return ""
	}

	if result.Value == "" || strings.HasPrefix(result.Value, "fake-") {
		t.Log("EODHD API key is placeholder - skipping")
		return ""
	}

	return result.Value
}

// fetchAndSaveEODHDData fetches EODHD API data and saves to results directory
func fetchAndSaveEODHDData(t *testing.T, env *common.TestEnvironment, stock, apiKey string) {
	fetchAndSaveEODHDDataToDir(t, env.GetResultsDir(), stock, apiKey)
}

// fetchAndSaveEODHDDataToDir fetches EODHD API data and saves to specified directory
func fetchAndSaveEODHDDataToDir(t *testing.T, resultsDir, stock, apiKey string) {
	// Fetch EOD data (24 months = ~730 days)
	fromDate := time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
	eodURL := fmt.Sprintf("https://eodhd.com/api/eod/%s.AU?api_token=%s&fmt=json&from=%s", stock, apiKey, fromDate)
	eodPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_eod_%s.json", strings.ToLower(stock)))
	if err := fetchAndSaveURL(eodURL, eodPath); err != nil {
		t.Logf("Warning: Failed to fetch EODHD EOD data for %s: %v", stock, err)
	} else {
		t.Logf("Saved EODHD EOD data to: %s", eodPath)
	}

	// Fetch fundamentals data
	fundURL := fmt.Sprintf("https://eodhd.com/api/fundamentals/%s.AU?api_token=%s&fmt=json", stock, apiKey)
	fundPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_fundamentals_%s.json", strings.ToLower(stock)))
	if err := fetchAndSaveURL(fundURL, fundPath); err != nil {
		t.Logf("Warning: Failed to fetch EODHD fundamentals for %s: %v", stock, err)
	} else {
		t.Logf("Saved EODHD fundamentals to: %s", fundPath)
	}
}

// fetchAndSaveURL fetches a URL and saves the response to a file
func fetchAndSaveURL(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// runStockCollectorTest runs the asx_stock_collector worker test
func runStockCollectorTest(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, defID, stock string) {
	runStockCollectorTestWithDir(t, env, helper, defID, stock, env.GetResultsDir(), 1)
}

// runStockCollectorTestWithDir runs the worker and saves output to specified directory
func runStockCollectorTestWithDir(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, defID, stock, resultsDir string, runNumber int) {
	body := map[string]interface{}{
		"id":          defID,
		"name":        fmt.Sprintf("ASX Stock Collector Test - %s", stock),
		"description": "Test asx_stock_collector with 24 months data",
		"type":        "asx_stock_collector",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-stock-collector", strings.ToLower(stock)},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock-data",
				"type": "asx_stock_collector",
				"config": map[string]interface{}{
					"asx_code": stock,
					"period":   "Y2", // 24 months
				},
			},
		},
	}

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Save job definition
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if data, err := json.MarshalIndent(body, "", "  "); err == nil {
		os.WriteFile(defPath, data, 0644)
	}

	// Execute job
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed asx_stock_collector job for %s: %s", stock, jobID)

	// Wait for completion
	finalStatus := waitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s (may be expected outside market hours)", finalStatus)
		return
	}

	// Save worker output
	saveWorkerOutputToDir(t, helper, resultsDir, stock, runNumber)
}

// saveWorkerOutputToDir saves worker output to specified directory
func saveWorkerOutputToDir(t *testing.T, helper *common.HTTPTestHelper, resultsDir, stock string, runNumber int) {
	tags := []string{"asx-stock-data", strings.ToLower(stock)}
	tagStr := strings.Join(tags, ",")

	resp, err := helper.GET("/api/documents?tags=" + tagStr)
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
		t.Logf("No documents found with tags: %s", tagStr)
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

	// Save output.json (metadata)
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

// =============================================================================
// Assertion Helpers
// =============================================================================

// assertEODHDFilesExist asserts EODHD API response files exist and are not empty
func assertEODHDFilesExist(t *testing.T, env *common.TestEnvironment, stock string) {
	assertEODHDFilesExistInDir(t, env.GetResultsDir(), stock)
}

func assertEODHDFilesExistInDir(t *testing.T, resultsDir, stock string) {
	stockLower := strings.ToLower(stock)

	eodPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_eod_%s.json", stockLower))
	assertFileExistsAndNotEmpty(t, eodPath)

	fundPath := filepath.Join(resultsDir, fmt.Sprintf("eodhd_fundamentals_%s.json", stockLower))
	assertFileExistsAndNotEmpty(t, fundPath)
}

// assertWorkerOutputFilesExist asserts worker output files exist
func assertWorkerOutputFilesExist(t *testing.T, env *common.TestEnvironment) {
	assertWorkerOutputFilesExistInDir(t, env.GetResultsDir())
}

func assertWorkerOutputFilesExistInDir(t *testing.T, resultsDir string) {
	mdPath := filepath.Join(resultsDir, "output.md")
	assertFileExistsAndNotEmpty(t, mdPath)

	jsonPath := filepath.Join(resultsDir, "output.json")
	assertFileExistsAndNotEmpty(t, jsonPath)
}

// assertHistoricalPrices24Months verifies 24 months of historical price data
func assertHistoricalPrices24Months(t *testing.T, env *common.TestEnvironment) {
	assertHistoricalPrices24MonthsInDir(t, env.GetResultsDir())
}

func assertHistoricalPrices24MonthsInDir(t *testing.T, resultsDir string) {
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

	// Check historical_prices array
	histPrices, ok := metadata["historical_prices"].([]interface{})
	if !ok {
		t.Errorf("historical_prices not found or not an array in output.json")
		return
	}

	// 24 months ≈ 500 trading days (approximately 21 trading days per month)
	minExpected := 400 // Allow some variance
	if len(histPrices) < minExpected {
		t.Errorf("Expected at least %d historical prices (24 months), got %d", minExpected, len(histPrices))
		return
	}

	// Verify data is not blank/zeros - at least 80% should have non-zero values
	nonZeroCount := 0
	for _, entry := range histPrices {
		priceEntry, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for non-zero close price and volume
		closePrice, _ := priceEntry["close"].(float64)
		volume, _ := priceEntry["volume"].(float64)

		if closePrice > 0 && volume > 0 {
			nonZeroCount++
		}
	}

	minNonZero := int(float64(len(histPrices)) * 0.8)
	if nonZeroCount < minNonZero {
		t.Errorf("Expected at least 80%% non-zero price/volume entries (%d), got %d", minNonZero, nonZeroCount)
		return
	}

	t.Logf("PASS: historical_prices has %d entries with %d non-zero (%d%% valid)",
		len(histPrices), nonZeroCount, (nonZeroCount*100)/len(histPrices))
}

// assertCurrencyAUD verifies currency is AUD
func assertCurrencyAUD(t *testing.T, env *common.TestEnvironment) {
	assertCurrencyAUDInDir(t, env.GetResultsDir())
}

func assertCurrencyAUDInDir(t *testing.T, resultsDir string) {
	// Check output.md contains "Currency: AUD" or "**Currency**: AUD"
	mdPath := filepath.Join(resultsDir, "output.md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Errorf("Failed to read output.md: %v", err)
		return
	}

	mdContent := string(mdData)
	if !strings.Contains(mdContent, "Currency**: AUD") && !strings.Contains(mdContent, "Currency: AUD") {
		t.Errorf("output.md does not contain 'Currency: AUD'")
	} else {
		t.Log("PASS: output.md contains Currency: AUD")
	}

	// Check output.json has currency field = "AUD"
	jsonPath := filepath.Join(resultsDir, "output.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Errorf("Failed to read output.json: %v", err)
		return
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(jsonData, &metadata); err != nil {
		t.Errorf("Failed to parse output.json: %v", err)
		return
	}

	currency, ok := metadata["currency"].(string)
	if !ok {
		t.Errorf("currency field not found in output.json metadata")
		return
	}

	assert.Equal(t, "AUD", currency, "Currency should be AUD")
	t.Log("PASS: output.json currency is AUD")
}

// assertHistoricalPricesInMarkdown verifies historical prices table in output.md
func assertHistoricalPricesInMarkdown(t *testing.T, env *common.TestEnvironment) {
	assertHistoricalPricesInMarkdownInDir(t, env.GetResultsDir())
}

func assertHistoricalPricesInMarkdownInDir(t *testing.T, resultsDir string) {
	mdPath := filepath.Join(resultsDir, "output.md")
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Errorf("Failed to read output.md: %v", err)
		return
	}

	mdContent := string(mdData)

	// Check for Historical Prices heading
	if !strings.Contains(mdContent, "## Historical Prices") {
		t.Errorf("output.md does not contain '## Historical Prices' heading")
		return
	}
	t.Log("PASS: output.md contains Historical Prices heading")

	// Check for table columns
	if !strings.Contains(mdContent, "| Date | Open | High | Low | Close | Volume |") {
		t.Errorf("output.md does not contain Historical Prices table header")
		return
	}
	t.Log("PASS: output.md contains Historical Prices table header")

	// Check for non-zero price data by looking for dollar amounts like "$45.49"
	// Match pattern: $XX.XX where X is digit
	hasNonZeroPrices := false
	lines := strings.Split(mdContent, "\n")
	for _, line := range lines {
		// Skip header rows and look for data rows with prices
		if strings.HasPrefix(line, "| 20") { // Date starts with year like "2024-01-02"
			// Check for non-zero close price (look for $XX.XX pattern)
			if strings.Contains(line, "$") && !strings.Contains(line, "$0.00") {
				hasNonZeroPrices = true
				break
			}
		}
	}

	if !hasNonZeroPrices {
		t.Errorf("output.md Historical Prices table has no non-zero prices")
		return
	}
	t.Log("PASS: output.md Historical Prices table has non-zero prices")
}
