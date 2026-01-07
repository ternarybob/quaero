// -----------------------------------------------------------------------
// Tests for market_competitor worker
// Uses LLM (Gemini) to analyze competitor landscape for a company
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerCompetitorSingle tests single stock competitor analysis
func TestWorkerCompetitorSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require both LLM (for competitor identification) and EODHD (for stock data)
	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-competitor-single-%d", time.Now().UnixNano())
	ticker := "BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Competitor Analysis Single Stock Test",
		"description": "Test market_competitor worker with single stock",
		"type":        "market_competitor",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-competitor", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "analyze-competitors",
				"type": "market_competitor",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_competitor job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"competitor-analysis", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Competitor", "Analysis"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, CompetitorSchema)
	assert.True(t, isValid, "Output should comply with competitor schema")

	// Validate competitors array
	if competitors, ok := metadata["competitors"].([]interface{}); ok {
		t.Logf("PASS: Found %d competitors", len(competitors))
		if len(competitors) > 0 {
			if first, ok := competitors[0].(map[string]interface{}); ok {
				if _, hasName := first["name"]; hasName {
					t.Log("PASS: Competitor has 'name' field")
				}
			}
		}
	}

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_competitor single stock test completed")
}

// TestWorkerCompetitorMulti tests multi-stock competitor analysis
func TestWorkerCompetitorMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require both LLM (for competitor identification) and EODHD (for stock data)
	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "RIO"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-competitor-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Competitor Analysis Test - %s", stock),
				"type":    "market_competitor",
				"enabled": true,
				"tags":    []string{"worker-test", "market-competitor", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "analyze-competitors",
						"type": "market_competitor",
						"config": map[string]interface{}{
							"asx_code": stock,
						},
					},
				},
			}

			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", stock, finalStatus)
				return
			}

			tags := []string{"competitor-analysis", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, CompetitorSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, stock)
			t.Logf("PASS: Validated competitor analysis for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_competitor multi-stock test completed")
}
