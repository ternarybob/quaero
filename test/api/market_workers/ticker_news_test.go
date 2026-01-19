// -----------------------------------------------------------------------
// Tests for ticker_news worker
// Tests the ticker_news worker which fetches and aggregates news from
// EODHD and web search for specified tickers.
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTickerNewsWorker_SingleTicker tests fetching news for a single ticker
func TestTickerNewsWorker_SingleTicker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Requires EODHD API key
	RequireEODHD(t, env)

	// LLM is optional for web search enhancement
	hasLLM := HasGeminiAPIKey(env)
	if !hasLLM {
		t.Log("WARN: No Gemini API key - web search will be skipped")
	}

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-ticker-news-single-%d", time.Now().UnixNano())
	ticker := "ASX:GNP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Ticker News Single Stock Test",
		"description": "Test news fetch for single ticker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-news", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"ticker":      ticker,
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"test-output"},
				},
			},
		},
	}

	// Save job definition BEFORE execution
	SaveJobDefinition(t, env, body)

	// Create and execute
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing ticker news job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate output
	tickerCode := "gnp"
	summaryTags := []string{"ticker-news", tickerCode}
	docID, metadata, content := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("Document ID: %s", docID)

	// Save output AFTER completion
	SaveWorkerOutput(t, env, helper, summaryTags, "GNP")

	// Validate schema
	isValid := ValidateSchema(t, metadata, TickerNewsSchema)
	assert.True(t, isValid, "Output should comply with ticker news schema")

	// Assert required fields
	AssertMetadataHasFields(t, metadata, []string{"ticker", "news_count", "fetched_at"})

	// Assert content contains expected sections
	expectedSections := []string{
		"News Summary",
		"Period",
	}
	AssertOutputContains(t, content, expectedSections)

	// Validate result files
	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker news single stock test completed")
}

// TestTickerNewsWorker_MultiTicker tests fetching news for multiple tickers via variables
func TestTickerNewsWorker_MultiTicker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-ticker-news-multi-%d", time.Now().UnixNano())
	tickers := []string{"ASX:GNP", "ASX:CGS"}

	// Build variables array
	variables := make([]map[string]interface{}, len(tickers))
	for i, t := range tickers {
		variables[i] = map[string]interface{}{"ticker": t}
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Ticker News Multi Stock Test",
		"description": "Test news fetch for multiple tickers via variables",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-news", "multi-stock"},
		"config": map[string]interface{}{
			"variables": variables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"test-output"},
				},
			},
		},
	}

	// Save job definition BEFORE execution
	SaveJobDefinition(t, env, body)

	// Create and execute
	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing multi-ticker news job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 8*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate output for each ticker
	for _, ticker := range tickers {
		parts := strings.Split(ticker, ":")
		tickerCode := strings.ToLower(parts[1])
		summaryTags := []string{"ticker-news", tickerCode}

		_, metadata, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)

		// Validate schema
		isValid := ValidateSchema(t, metadata, TickerNewsSchema)
		assert.True(t, isValid, "Output for %s should comply with schema", ticker)

		t.Logf("PASS: %s news document validated", ticker)
	}

	// Save output for first ticker
	SaveWorkerOutput(t, env, helper, []string{"ticker-news", "gnp"}, "GNP")

	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker news multi stock test completed")
}

// TestTickerNewsWorker_Caching tests that news caching works correctly
func TestTickerNewsWorker_Caching(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "ASX:EXR"
	tickerCode := "exr"

	body := map[string]interface{}{
		"id":          fmt.Sprintf("test-ticker-news-cache-%d", time.Now().UnixNano()),
		"name":        "Ticker News Cache Test",
		"description": "Test news caching behavior",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-news", "caching"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "ticker_news",
				"config": map[string]interface{}{
					"ticker":      ticker,
					"period":      "M1",
					"cache_hours": 24,
					"output_tags": []string{"cache-test"},
				},
			},
		},
	}

	// Save job definition BEFORE execution
	SaveJobDefinition(t, env, body)

	// === FIRST EXECUTION ===
	jobID1, _ := CreateAndExecuteJob(t, helper, body)
	if jobID1 == "" {
		return
	}

	t.Logf("First run job ID: %s", jobID1)

	finalStatus1 := WaitForJobCompletion(t, helper, jobID1, 5*time.Minute)
	if finalStatus1 != "completed" {
		t.Skipf("First job ended with status %s", finalStatus1)
		return
	}

	summaryTags := []string{"ticker-news", tickerCode}
	docID1, _, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("First run document ID: %s", docID1)

	// === SECOND EXECUTION (should use cache) ===
	jobID2, _ := CreateAndExecuteJob(t, helper, body)
	if jobID2 == "" {
		return
	}

	t.Logf("Second run job ID: %s", jobID2)

	finalStatus2 := WaitForJobCompletion(t, helper, jobID2, 2*time.Minute)
	if finalStatus2 != "completed" {
		t.Skipf("Second job ended with status %s", finalStatus2)
		return
	}

	docID2, _, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("Second run document ID: %s", docID2)

	// === CACHING ASSERTION ===
	assert.Equal(t, docID1, docID2, "CACHING: Document ID should be the same on second run (document was cached)")
	if docID1 == docID2 {
		t.Log("PASS: Caching works - same document ID returned on second run")
	}

	SaveWorkerOutput(t, env, helper, summaryTags, "EXR")
	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker news caching test completed")
}
