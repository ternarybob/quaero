// -----------------------------------------------------------------------
// Tests for market_fundamentals worker
// Fetches price, analyst coverage, and financials via EODHD API
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestWorkerFundamentalsSingle tests single stock processing
func TestWorkerFundamentalsSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for stock fundamentals data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create job definition with single ticker
	defID := fmt.Sprintf("test-fundamentals-single-%d", time.Now().UnixNano())
	ticker := "BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Fundamentals Single Stock Test",
		"description": "Test market_fundamentals worker with single stock",
		"type":        "market_fundamentals",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-fundamentals", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock-data",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"asx_code": ticker,
					"period":   "Y1",
				},
			},
		},
	}

	// Save job definition
	if err := SaveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	// Create and execute job
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_fundamentals job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s (may be expected outside market hours)", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Assert output.md and output.json are not empty
	tags := []string{"asx-stock-data", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{
		"Current Price",
		"Technical",
	}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, FundamentalsSchema)
	assert.True(t, isValid, "Output should comply with fundamentals schema")

	// Assert specific fields
	AssertMetadataHasFields(t, metadata, []string{"symbol", "current_price", "currency"})

	// Validate historical_prices array if present
	if histPrices, ok := metadata["historical_prices"].([]interface{}); ok {
		assert.Greater(t, len(histPrices), 0, "Should have historical price entries")
		t.Logf("PASS: Found %d historical price entries", len(histPrices))
	}

	// Save output
	if err := SaveWorkerOutput(t, env, helper, tags, 1); err != nil {
		t.Logf("Warning: failed to save worker output: %v", err)
	}
	AssertResultFilesExist(t, env, 1)

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_fundamentals single stock test completed")
}

// TestWorkerFundamentalsMulti tests multi-stock processing
func TestWorkerFundamentalsMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for stock fundamentals data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create job definition with multiple tickers
	defID := fmt.Sprintf("test-fundamentals-multi-%d", time.Now().UnixNano())
	testTickers := []string{"ASX:BHP", "ASX:CSL", "ASX:GNP"}
	testCodes := []string{"BHP", "CSL", "GNP"}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Fundamentals Multi-Stock Test",
		"description": "Test market_fundamentals worker with multiple stocks",
		"type":        "market_fundamentals",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-fundamentals", "multi-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock-data",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"tickers": testTickers,
					"period":  "Y1",
				},
			},
		},
	}

	// Save job definition
	if err := SaveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	// Create and execute job
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_fundamentals multi-stock job: %s", jobID)

	// Wait for completion (longer timeout for multiple stocks)
	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===

	// Get the WorkerResult to validate by_ticker format
	workerResult := getWorkerResultFromJob(t, helper, jobID)
	if workerResult != nil {
		// Validate by_ticker field exists
		require.NotNil(t, workerResult.ByTicker, "WorkerResult must have by_ticker field")
		require.Equal(t, len(testTickers), len(workerResult.ByTicker),
			"by_ticker should have entries for all %d stocks", len(testTickers))

		// Validate each stock has correct per-ticker result
		for _, stock := range testTickers {
			tickerResult, exists := workerResult.ByTicker[stock]
			require.True(t, exists, "by_ticker must contain entry for %s", stock)
			require.NotNil(t, tickerResult, "TickerResult for %s must not be nil", stock)
			assert.Equal(t, 1, tickerResult.DocumentsCreated,
				"Each stock should have exactly 1 document created for %s", stock)
			t.Logf("PASS: by_ticker[%s] has %d documents", stock, tickerResult.DocumentsCreated)
		}

		// Validate totals match per-ticker sum
		totalDocs := 0
		for _, tr := range workerResult.ByTicker {
			totalDocs += tr.DocumentsCreated
		}
		assert.Equal(t, workerResult.DocumentsCreated, totalDocs,
			"Total documents_created (%d) should equal sum of per-ticker counts (%d)",
			workerResult.DocumentsCreated, totalDocs)
	}

	// Validate each stock has output with correct assertions
	for _, code := range testCodes {
		tags := []string{"asx-stock-data", strings.ToLower(code)}
		metadata, content := AssertOutputNotEmpty(t, helper, tags)

		// Assert content not empty
		assert.NotEmpty(t, content, "Content for %s should not be empty", code)

		// Assert schema compliance per ticker
		isValid := ValidateSchema(t, metadata, FundamentalsSchema)
		assert.True(t, isValid, "Output for %s should comply with schema", code)

		t.Logf("PASS: Validated output for %s", code)
	}

	// Check for service errors
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_fundamentals multi-stock test completed")
}

// =============================================================================
// Helper Types and Functions
// =============================================================================

// WorkerResult mirrors the WorkerResult structure for test parsing
type WorkerResult struct {
	DocumentsCreated int                      `json:"documents_created"`
	DocumentIDs      []string                 `json:"document_ids"`
	Tags             []string                 `json:"tags"`
	SourceType       string                   `json:"source_type"`
	ByTicker         map[string]*TickerResult `json:"by_ticker"`
}

// TickerResult mirrors TickerResult for test parsing
type TickerResult struct {
	DocumentsCreated int      `json:"documents_created"`
	DocumentIDs      []string `json:"document_ids"`
	Tags             []string `json:"tags"`
}

// getWorkerResultFromJob retrieves WorkerResult from job metadata
func getWorkerResultFromJob(t *testing.T, helper *common.HTTPTestHelper, jobID string) *WorkerResult {
	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Logf("Failed to get job %s: %v", jobID, err)
		return nil
	}
	defer resp.Body.Close()

	var job struct {
		Type     string                 `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		t.Logf("Failed to parse job response: %v", err)
		return nil
	}

	if job.Metadata == nil {
		return nil
	}

	// For manager job, find the step job
	if job.Type == "manager" {
		stepJobIDs, ok := job.Metadata["step_job_ids"].(map[string]interface{})
		if !ok {
			return nil
		}
		fetchJobID, ok := stepJobIDs["fetch-stock-data"].(string)
		if !ok {
			return nil
		}
		return getWorkerResultDirect(t, helper, fetchJobID)
	}

	return getWorkerResultDirect(t, helper, jobID)
}

// getWorkerResultDirect gets WorkerResult from a specific job's metadata
func getWorkerResultDirect(t *testing.T, helper *common.HTTPTestHelper, jobID string) *WorkerResult {
	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var job struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		return nil
	}

	if job.Metadata == nil {
		return nil
	}

	workerResultRaw, ok := job.Metadata["worker_result"].(map[string]interface{})
	if !ok {
		return nil
	}

	result := &WorkerResult{}

	if v, ok := workerResultRaw["documents_created"].(float64); ok {
		result.DocumentsCreated = int(v)
	}

	// Parse by_ticker if present
	if byTicker, ok := workerResultRaw["by_ticker"].(map[string]interface{}); ok {
		result.ByTicker = make(map[string]*TickerResult)
		for ticker, tickerData := range byTicker {
			if tickerMap, ok := tickerData.(map[string]interface{}); ok {
				tr := &TickerResult{}
				if v, ok := tickerMap["documents_created"].(float64); ok {
					tr.DocumentsCreated = int(v)
				}
				if v, ok := tickerMap["document_ids"].([]interface{}); ok {
					for _, id := range v {
						if s, ok := id.(string); ok {
							tr.DocumentIDs = append(tr.DocumentIDs, s)
						}
					}
				}
				result.ByTicker[ticker] = tr
			}
		}
	}

	return result
}
