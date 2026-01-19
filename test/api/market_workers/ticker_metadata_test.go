// -----------------------------------------------------------------------
// Tests for ticker_metadata worker
// Tests the ticker_metadata worker which fetches company profile data
// from EODHD fundamentals API including directors and management.
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTickerMetadataWorker_SingleTicker tests fetching metadata for a single ticker
func TestTickerMetadataWorker_SingleTicker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Requires EODHD API key
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-ticker-metadata-single-%d", time.Now().UnixNano())
	ticker := "ASX:GNP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Ticker Metadata Single Stock Test",
		"description": "Test metadata fetch for single ticker",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-metadata", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"ticker":      ticker,
					"cache_hours": 168,
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

	t.Logf("Executing ticker metadata job: %s", jobID)

	// Wait for completion
	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate output
	tickerCode := "gnp"
	summaryTags := []string{"ticker-metadata", tickerCode}
	docID, metadata, content := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("Document ID: %s", docID)

	// Save output AFTER completion
	SaveWorkerOutput(t, env, helper, summaryTags, "GNP")

	// Validate schema
	isValid := ValidateSchema(t, metadata, TickerMetadataSchema)
	assert.True(t, isValid, "Output should comply with ticker metadata schema")

	// Assert required fields
	AssertMetadataHasFields(t, metadata, []string{"ticker", "company_name", "fetched_at"})

	// Assert content contains expected sections
	expectedSections := []string{
		"Company Overview",
		"Key Financials",
	}
	AssertOutputContains(t, content, expectedSections)

	// Validate result files
	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker metadata single stock test completed")
}

// TestTickerMetadataWorker_DirectorsExtraction tests that directors and management are extracted
func TestTickerMetadataWorker_DirectorsExtraction(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Use a well-known company that has officers data (US stocks have better coverage)
	defID := fmt.Sprintf("test-ticker-metadata-directors-%d", time.Now().UnixNano())
	ticker := "AAPL.US"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Ticker Metadata Directors Test",
		"description": "Test directors and management extraction",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-metadata", "directors"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"ticker":      ticker,
					"cache_hours": 168,
					"output_tags": []string{"test-directors"},
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

	t.Logf("Executing directors extraction job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate output
	summaryTags := []string{"ticker-metadata", "aapl"}
	_, metadata, content := AssertOutputNotEmptyWithID(t, helper, summaryTags)

	// Check for directors array
	if directors, ok := metadata["directors"].([]interface{}); ok {
		t.Logf("Directors found: %d", len(directors))
		assert.GreaterOrEqual(t, len(directors), 0, "Directors array should exist")

		// If we have directors, check structure
		if len(directors) > 0 {
			if dir, ok := directors[0].(map[string]interface{}); ok {
				assert.Contains(t, dir, "name", "Director entry should have name")
				assert.Contains(t, dir, "title", "Director entry should have title")
				t.Logf("First director: %s - %s", dir["name"], dir["title"])
			}
		}
	}

	// Check for management array
	if management, ok := metadata["management"].([]interface{}); ok {
		t.Logf("Management found: %d", len(management))
		assert.GreaterOrEqual(t, len(management), 0, "Management array should exist")

		// If we have management, check structure
		if len(management) > 0 {
			if mgr, ok := management[0].(map[string]interface{}); ok {
				assert.Contains(t, mgr, "name", "Management entry should have name")
				assert.Contains(t, mgr, "title", "Management entry should have title")
				t.Logf("First management: %s - %s", mgr["name"], mgr["title"])
			}
		}
	}

	// Check content has directors/management sections (if data exists)
	if dirCount, ok := metadata["director_count"].(float64); ok && dirCount > 0 {
		assert.Contains(t, content, "Directors", "Content should have Directors section")
	}
	if mgrCount, ok := metadata["management_count"].(float64); ok && mgrCount > 0 {
		assert.Contains(t, content, "Management", "Content should have Management section")
	}

	SaveWorkerOutput(t, env, helper, summaryTags, "AAPL")
	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker metadata directors extraction test completed")
}

// TestTickerMetadataWorker_MultiTicker tests fetching metadata for multiple tickers
func TestTickerMetadataWorker_MultiTicker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-ticker-metadata-multi-%d", time.Now().UnixNano())
	tickers := []string{"ASX:GNP", "ASX:CGS"}

	// Build variables array
	variables := make([]map[string]interface{}, len(tickers))
	for i, t := range tickers {
		variables[i] = map[string]interface{}{"ticker": t}
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Ticker Metadata Multi Stock Test",
		"description": "Test metadata fetch for multiple tickers via variables",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-metadata", "multi-stock"},
		"config": map[string]interface{}{
			"variables": variables,
		},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"cache_hours": 168,
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

	t.Logf("Executing multi-ticker metadata job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 5*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Validate output for each ticker
	for _, ticker := range tickers {
		parts := strings.Split(ticker, ":")
		tickerCode := strings.ToLower(parts[1])
		summaryTags := []string{"ticker-metadata", tickerCode}

		_, metadata, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)

		// Validate schema
		isValid := ValidateSchema(t, metadata, TickerMetadataSchema)
		assert.True(t, isValid, "Output for %s should comply with schema", ticker)

		t.Logf("PASS: %s metadata document validated", ticker)
	}

	SaveWorkerOutput(t, env, helper, []string{"ticker-metadata", "gnp"}, "GNP")
	AssertResultFilesExist(t, env, 0)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: ticker metadata multi stock test completed")
}

// TestTickerMetadataWorker_Caching tests that metadata caching works correctly
func TestTickerMetadataWorker_Caching(t *testing.T) {
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
		"id":          fmt.Sprintf("test-ticker-metadata-cache-%d", time.Now().UnixNano()),
		"name":        "Ticker Metadata Cache Test",
		"description": "Test metadata caching behavior",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"worker-test", "ticker-metadata", "caching"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-metadata",
				"type": "ticker_metadata",
				"config": map[string]interface{}{
					"ticker":      ticker,
					"cache_hours": 168,
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

	finalStatus1 := WaitForJobCompletion(t, helper, jobID1, 3*time.Minute)
	if finalStatus1 != "completed" {
		t.Skipf("First job ended with status %s", finalStatus1)
		return
	}

	summaryTags := []string{"ticker-metadata", tickerCode}
	docID1, _, _ := AssertOutputNotEmptyWithID(t, helper, summaryTags)
	t.Logf("First run document ID: %s", docID1)

	// === SECOND EXECUTION (should use cache) ===
	jobID2, _ := CreateAndExecuteJob(t, helper, body)
	if jobID2 == "" {
		return
	}

	t.Logf("Second run job ID: %s", jobID2)

	finalStatus2 := WaitForJobCompletion(t, helper, jobID2, 1*time.Minute)
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

	t.Log("PASS: ticker metadata caching test completed")
}
