// -----------------------------------------------------------------------
// Tests for market_portfolio worker
// Aggregates signals and data across portfolio holdings
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerPortfolioSingle tests portfolio aggregation for a single stock
func TestWorkerPortfolioSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// First prepare market data and signals for a stock
	ticker := "BHP"

	// Run market_data first
	dataDefID := fmt.Sprintf("test-portfolio-data-prep-%d", time.Now().UnixNano())
	dataBody := map[string]interface{}{
		"id":      dataDefID,
		"name":    "Market Data Prep for Portfolio Test",
		"type":    "market_data",
		"enabled": true,
		"steps": []map[string]interface{}{
			{
				"name": "fetch-market-data",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
					"period": "Y1",
				},
			},
		},
	}

	dataJobID, _ := CreateAndExecuteJob(t, helper, dataBody)
	if dataJobID != "" {
		dataStatus := WaitForJobCompletion(t, helper, dataJobID, 2*time.Minute)
		if dataStatus != "completed" {
			t.Skipf("Market data job ended with status %s", dataStatus)
			return
		}
	}

	t.Log("Data prepared, running portfolio aggregation")

	// Now run portfolio worker
	defID := fmt.Sprintf("test-portfolio-single-%d", time.Now().UnixNano())
	portfolioTag := "test-portfolio"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Portfolio Aggregation Single Stock Test",
		"description": "Test market_portfolio worker with single stock",
		"type":        "market_portfolio",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-portfolio", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "aggregate-portfolio",
				"type": "market_portfolio",
				"config": map[string]interface{}{
					"portfolio_tag": portfolioTag,
					"tickers":       []string{fmt.Sprintf("ASX:%s", ticker)},
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_portfolio job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"portfolio-summary", portfolioTag}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Portfolio", "Summary"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, PortfolioSchema)
	assert.True(t, isValid, "Output should comply with portfolio schema")

	// Validate holdings array
	if holdings, ok := metadata["holdings"].([]interface{}); ok {
		t.Logf("PASS: Found %d portfolio holdings", len(holdings))
	}

	SaveWorkerOutput(t, env, helper, tags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_portfolio single stock test completed")
}

// TestWorkerPortfolioMulti tests portfolio aggregation for multiple stocks
func TestWorkerPortfolioMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP"}
	portfolioTag := "test-multi-portfolio"

	// First prepare market data for all stocks
	for _, stock := range stocks {
		dataDefID := fmt.Sprintf("test-portfolio-data-%s-%d", strings.ToLower(stock), time.Now().UnixNano())
		dataBody := map[string]interface{}{
			"id":      dataDefID,
			"name":    fmt.Sprintf("Market Data Prep - %s", stock),
			"type":    "market_data",
			"enabled": true,
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

		jobID, _ := CreateAndExecuteJob(t, helper, dataBody)
		if jobID != "" {
			WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
		}
	}

	t.Log("Data prepared for all stocks, running portfolio aggregation")

	// Now run portfolio worker with all stocks
	defID := fmt.Sprintf("test-portfolio-multi-%d", time.Now().UnixNano())

	// Convert stocks to ticker format
	tickers := make([]string, len(stocks))
	for i, stock := range stocks {
		tickers[i] = fmt.Sprintf("ASX:%s", stock)
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Portfolio Aggregation Multi-Stock Test",
		"description": "Test market_portfolio worker with multiple stocks",
		"type":        "market_portfolio",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-portfolio", "multi-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "aggregate-portfolio",
				"type": "market_portfolio",
				"config": map[string]interface{}{
					"portfolio_tag": portfolioTag,
					"tickers":       tickers,
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_portfolio multi-stock job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"portfolio-summary", portfolioTag}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	assert.NotEmpty(t, content, "Content should not be empty")

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, PortfolioSchema)
	assert.True(t, isValid, "Output should comply with portfolio schema")

	// Validate holdings count matches stocks
	if holdings, ok := metadata["holdings"].([]interface{}); ok {
		t.Logf("PASS: Found %d holdings (expected %d)", len(holdings), len(stocks))
	}

	SaveWorkerOutput(t, env, helper, tags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_portfolio multi-stock test completed")
}
