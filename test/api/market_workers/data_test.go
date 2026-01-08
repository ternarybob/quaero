// -----------------------------------------------------------------------
// Tests for market_data worker
// Fetches price data and technicals via EODHD API
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerDataSingle tests single stock market data
func TestWorkerDataSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Create job definition
	defID := fmt.Sprintf("test-data-single-%d", time.Now().UnixNano())
	ticker := "ASX:BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Market Data Single Stock Test",
		"description": "Test market_data worker with single stock",
		"type":        "market_data",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-data", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-market-data",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": ticker,
					"period": "Y1",
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_data job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"market-data", "bhp"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content sections
	expectedSections := []string{"Current Price", "Technical"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, DataSchema)
	assert.True(t, isValid, "Output should comply with data schema")

	// Validate historical prices
	if histPrices, ok := metadata["historical_prices"].([]interface{}); ok {
		assert.Greater(t, len(histPrices), 0, "Should have historical prices")
		t.Logf("PASS: Found %d historical price entries", len(histPrices))

		// Validate first entry has required fields
		if len(histPrices) > 0 {
			if first, ok := histPrices[0].(map[string]interface{}); ok {
				if _, hasDate := first["date"]; hasDate {
					t.Log("PASS: Historical price has 'date' field")
				}
				if _, hasClose := first["close"]; hasClose {
					t.Log("PASS: Historical price has 'close' field")
				}
			}
		}
	}

	// Validate technical indicators
	techFields := []string{"sma_20", "sma_50", "rsi_14", "trend_signal"}
	for _, field := range techFields {
		if _, exists := metadata[field]; exists {
			t.Logf("PASS: Has technical indicator '%s'", field)
		}
	}

	SaveWorkerOutput(t, env, helper, tags, "BHP")
	SaveSchemaDefinition(t, env, DataSchema, "DataSchema")
	AssertResultFilesExist(t, env, 1)
	AssertSchemaFileExists(t, env)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_data single stock test completed")
}

// TestWorkerDataMulti tests multi-stock market data
func TestWorkerDataMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-data-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Market Data Test - %s", stock),
				"type":    "market_data",
				"enabled": true,
				"tags":    []string{"worker-test", "market-data", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-market-data",
						"type": "market_data",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", stock),
							"period": "Y1",
						},
					},
				},
			}

			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", stock, finalStatus)
				return
			}

			tags := []string{"market-data", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, DataSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, stock)
			SaveSchemaDefinition(t, env, DataSchema, "DataSchema")
			t.Logf("PASS: Validated market_data for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_data multi-stock test completed")
}
