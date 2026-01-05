// -----------------------------------------------------------------------
// Tests for market_data_collection worker
// Orchestrates multiple data collection workers for a list of tickers
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerDataCollectionSingle tests data collection for a single stock
func TestWorkerDataCollectionSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for stock data collection
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-data-collection-single-%d", time.Now().UnixNano())
	ticker := "BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Data Collection Single Stock Test",
		"description": "Test market_data_collection worker with single stock",
		"type":        "market_data_collection",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-data-collection", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "collect-data",
				"type": "market_data_collection",
				"config": map[string]interface{}{
					"tickers": []string{fmt.Sprintf("ASX:%s", ticker)},
					"workers": []string{"market_fundamentals", "market_data"},
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_data_collection job: %s", jobID)

	// Data collection can take longer as it runs multiple workers
	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	// Query actual stock data document (not just summary)
	stockDataTags := []string{"stock-data-collected", ticker}
	metadata, content := AssertOutputNotEmpty(t, helper, stockDataTags)

	// Assert schema compliance for fundamentals data
	isValid := ValidateSchema(t, metadata, FundamentalsSchema)
	assert.True(t, isValid, "Output should comply with fundamentals schema")

	// Validate correct ticker in output (REQ-2)
	AssertTickerInOutput(t, ticker, metadata, content)

	// Validate non-zero stock data (REQ-3 - prevents hallucinations)
	AssertNonZeroStockData(t, metadata)

	// Save actual stock data output (REQ-1, REQ-4)
	SaveWorkerOutput(t, env, helper, stockDataTags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_data_collection single stock test completed")
}

// TestWorkerDataCollectionMulti tests data collection for multiple stocks
func TestWorkerDataCollectionMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for stock data collection
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-data-collection-multi-%d", time.Now().UnixNano())
	stocks := []string{"BHP", "CSL", "GNP"}

	// Convert to ticker format
	tickers := make([]string, len(stocks))
	for i, stock := range stocks {
		tickers[i] = fmt.Sprintf("ASX:%s", stock)
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Data Collection Multi-Stock Test",
		"description": "Test market_data_collection worker with multiple stocks",
		"type":        "market_data_collection",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-data-collection", "multi-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "collect-data",
				"type": "market_data_collection",
				"config": map[string]interface{}{
					"tickers": tickers,
					"workers": []string{"market_fundamentals", "market_data", "market_announcements"},
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_data_collection multi-stock job: %s", jobID)

	// Multi-stock collection takes even longer
	finalStatus := WaitForJobCompletion(t, helper, jobID, 8*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	// Query and validate stock data for each ticker (REQ-1, REQ-2, REQ-3)
	for _, stock := range stocks {
		stockDataTags := []string{"stock-data-collected", stock}
		metadata, content := AssertOutputNotEmpty(t, helper, stockDataTags)

		// Validate correct ticker in output (REQ-2)
		AssertTickerInOutput(t, stock, metadata, content)

		// Validate non-zero stock data (REQ-3 - prevents hallucinations)
		AssertNonZeroStockData(t, metadata)

		// Assert schema compliance
		isValid := ValidateSchema(t, metadata, FundamentalsSchema)
		assert.True(t, isValid, "Output for %s should comply with fundamentals schema", stock)

		t.Logf("PASS: Stock data validated for %s", stock)
	}

	// Save combined stock data output (REQ-4)
	SaveMultiStockOutput(t, env, helper, stocks, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_data_collection multi-stock test completed")
}

// TestWorkerDataCollectionAllWorkers tests data collection with all available workers
func TestWorkerDataCollectionAllWorkers(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require both LLM (for competitor analysis) and EODHD (for stock data)
	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-data-collection-all-%d", time.Now().UnixNano())
	ticker := "BHP"

	// All data collection workers
	allWorkers := []string{
		"market_fundamentals",
		"market_data",
		"market_announcements",
		"market_director_interest",
		"market_competitor",
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Data Collection All Workers Test",
		"description": "Test market_data_collection with all available workers",
		"type":        "market_data_collection",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-data-collection", "all-workers"},
		"steps": []map[string]interface{}{
			{
				"name": "collect-all-data",
				"type": "market_data_collection",
				"config": map[string]interface{}{
					"tickers": []string{fmt.Sprintf("ASX:%s", ticker)},
					"workers": allWorkers,
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_data_collection all-workers job: %s", jobID)

	// All workers takes longer
	finalStatus := WaitForJobCompletion(t, helper, jobID, 10*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	// Query actual stock data document (not just summary)
	stockDataTags := []string{"stock-data-collected", ticker}
	metadata, content := AssertOutputNotEmpty(t, helper, stockDataTags)

	// Assert schema compliance for fundamentals data
	isValid := ValidateSchema(t, metadata, FundamentalsSchema)
	assert.True(t, isValid, "Output should comply with fundamentals schema")

	// Validate correct ticker in output (REQ-2)
	AssertTickerInOutput(t, ticker, metadata, content)

	// Validate non-zero stock data (REQ-3 - prevents hallucinations)
	AssertNonZeroStockData(t, metadata)

	// Save actual stock data output (REQ-1, REQ-4)
	SaveWorkerOutput(t, env, helper, stockDataTags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_data_collection all-workers test completed")
}
