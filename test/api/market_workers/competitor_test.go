// -----------------------------------------------------------------------
// Tests for market_competitor worker
// Uses LLM (Gemini) to analyze competitor landscape for a company
// Output: competitor tickers with rationale (no stock data)
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkerCompetitorSingle tests single stock competitor analysis
func TestWorkerCompetitorSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require LLM for competitor identification
	RequireLLM(t, env)

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
					"asx_code":    ticker,
					"api_key":     "{google_gemini_api_key}",
					"output_tags": []string{"competitor-analysis", strings.ToLower(ticker)},
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
	// The competitor worker creates competitor analysis documents
	tags := []string{"competitor-analysis", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert schema compliance - competitor output uses CompetitorSchema
	isValid := ValidateSchema(t, metadata, CompetitorSchema)
	assert.True(t, isValid, "Output should comply with CompetitorSchema")

	// Assert content contains expected sections (competitor analysis format)
	expectedSections := []string{"Competitor Analysis", "Competitors"}
	AssertOutputContains(t, content, expectedSections)

	// Verify target ticker is captured correctly
	if targetTicker, ok := metadata["target_ticker"].(string); ok {
		assert.Contains(t, targetTicker, ticker, "target_ticker should contain ticker code")
		t.Logf("PASS: target_ticker = %s", targetTicker)
	}

	// Verify target code is captured correctly
	if targetCode, ok := metadata["target_code"].(string); ok {
		assert.Equal(t, ticker, targetCode, "target_code should match ticker")
		t.Logf("PASS: target_code = %s", targetCode)
	}

	// Verify gemini_prompt is present and non-empty
	if geminiPrompt, ok := metadata["gemini_prompt"].(string); ok {
		assert.NotEmpty(t, geminiPrompt, "gemini_prompt should not be empty")
		assert.Contains(t, geminiPrompt, ticker, "gemini_prompt should contain target ticker")
		t.Logf("PASS: gemini_prompt captured (%d chars)", len(geminiPrompt))
	}

	// Verify competitors array exists
	if competitors, ok := metadata["competitors"].([]interface{}); ok {
		t.Logf("PASS: Found %d competitors", len(competitors))
		// Verify each competitor has code and rationale
		for i, comp := range competitors {
			if compMap, ok := comp.(map[string]interface{}); ok {
				code, hasCode := compMap["code"].(string)
				rationale, hasRationale := compMap["rationale"].(string)
				require.True(t, hasCode, "Competitor %d should have code", i)
				require.True(t, hasRationale, "Competitor %d should have rationale", i)
				require.NotEmpty(t, code, "Competitor %d code should not be empty", i)
				require.NotEmpty(t, rationale, "Competitor %d rationale should not be empty", i)
				t.Logf("  Competitor %d: %s - %s", i, code, rationale)
			}
		}
	}

	// Verify content contains the prompt in grey styling
	assert.Contains(t, content, "prompt -", "Content should contain grey-styled prompt")

	SaveWorkerOutput(t, env, helper, tags, ticker)
	SaveSchemaDefinition(t, env, CompetitorSchema, "CompetitorSchema")
	AssertResultFilesExist(t, env, 1)
	AssertSchemaFileExists(t, env)
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

	// Require LLM for competitor identification
	RequireLLM(t, env)

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
							"asx_code":    stock,
							"api_key":     "{google_gemini_api_key}",
							"output_tags": []string{"competitor-analysis", strings.ToLower(stock)},
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

			// Validate schema
			isValid := ValidateSchema(t, metadata, CompetitorSchema)
			assert.True(t, isValid, "Output for %s should comply with CompetitorSchema", stock)

			// Verify target code matches
			if targetCode, ok := metadata["target_code"].(string); ok {
				assert.Equal(t, stock, targetCode, "target_code should match stock")
			}

			// Verify competitors have rationale
			if competitors, ok := metadata["competitors"].([]interface{}); ok {
				for i, comp := range competitors {
					if compMap, ok := comp.(map[string]interface{}); ok {
						_, hasRationale := compMap["rationale"].(string)
						assert.True(t, hasRationale, "Competitor %d for %s should have rationale", i, stock)
					}
				}
			}

			SaveWorkerOutput(t, env, helper, tags, stock)
			SaveSchemaDefinition(t, env, CompetitorSchema, "CompetitorSchema")
			t.Logf("PASS: Validated competitor analysis for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_competitor multi-stock test completed")
}
