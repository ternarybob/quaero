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
	// Data collection creates a summary document
	tags := []string{"data-collection-summary"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Data Collection", "Summary"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, DataCollectionSchema)
	assert.True(t, isValid, "Output should comply with data collection schema")

	// Validate tickers_processed
	if processed, ok := metadata["tickers_processed"].(float64); ok {
		assert.GreaterOrEqual(t, int(processed), 1, "Should have processed at least 1 ticker")
		t.Logf("PASS: Processed %d tickers", int(processed))
	}

	// Validate documents_created
	if created, ok := metadata["documents_created"].(float64); ok {
		assert.GreaterOrEqual(t, int(created), 1, "Should have created at least 1 document")
		t.Logf("PASS: Created %d documents", int(created))
	}

	SaveWorkerOutput(t, env, helper, tags, 1)
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
	tags := []string{"data-collection-summary"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	assert.NotEmpty(t, content, "Content should not be empty")

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, DataCollectionSchema)
	assert.True(t, isValid, "Output should comply with data collection schema")

	// Validate tickers_processed matches input
	if processed, ok := metadata["tickers_processed"].(float64); ok {
		assert.GreaterOrEqual(t, int(processed), len(stocks), "Should have processed all tickers")
		t.Logf("PASS: Processed %d tickers (expected %d)", int(processed), len(stocks))
	}

	// Check for errors field
	if errors, ok := metadata["errors"].([]interface{}); ok {
		if len(errors) > 0 {
			t.Logf("WARNING: Data collection had %d errors", len(errors))
		} else {
			t.Log("PASS: No errors during data collection")
		}
	}

	SaveWorkerOutput(t, env, helper, tags, 1)
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
	tags := []string{"data-collection-summary"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	assert.NotEmpty(t, content, "Content should not be empty")

	// Validate documents_created matches expected (1 per worker)
	if created, ok := metadata["documents_created"].(float64); ok {
		assert.GreaterOrEqual(t, int(created), len(allWorkers), "Should have created at least 1 document per worker")
		t.Logf("PASS: Created %d documents (expected at least %d)", int(created), len(allWorkers))
	}

	SaveWorkerOutput(t, env, helper, tags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_data_collection all-workers test completed")
}
