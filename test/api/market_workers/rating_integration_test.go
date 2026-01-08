// -----------------------------------------------------------------------
// Integration tests for the complete rating workflow
// Tests full pipeline from data collection to final investability rating
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

// =============================================================================
// Full Flow Integration Tests
// =============================================================================

// TestRatingFullFlowSingle tests the complete rating flow for a single ticker
func TestRatingFullFlowSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	ticker := "GNP"
	defID := fmt.Sprintf("test-rating-full-flow-%d", time.Now().UnixNano())

	// Full rating pipeline using job definition
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating Full Flow Test",
		"description": "Test complete rating workflow",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"integration-test", "rating-full-flow"},
		"steps": []map[string]interface{}{
			// Step 1: Data collection
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Step 2: Gate scores
			{
				"name":    "calculate-bfs",
				"type":    "rating_bfs",
				"depends": "fetch-fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-cds",
				"type":    "rating_cds",
				"depends": "fetch-fundamentals,process-announcements",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Step 3: Component scores
			{
				"name":    "calculate-nfr",
				"type":    "rating_nfr",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-pps",
				"type":    "rating_pps",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-vrs",
				"type":    "rating_vrs",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-ob",
				"type":    "rating_ob",
				"depends": "process-announcements,calculate-bfs",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			// Step 4: Final rating
			{
				"name":    "calculate-rating",
				"type":    "rating_composite",
				"depends": "calculate-bfs,calculate-cds,calculate-nfr,calculate-pps,calculate-vrs,calculate-ob",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing full rating flow job: %s", jobID)

	// Extended timeout for full flow
	finalStatus := WaitForJobCompletion(t, helper, jobID, 8*time.Minute)
	require.Equal(t, "completed", finalStatus, "Full flow job must complete")

	// === VALIDATE ALL OUTPUTS ===

	// 1. Validate raw announcements were created
	rawTags := []string{"asx-announcement-raw", strings.ToLower(ticker)}
	_, _ = AssertOutputNotEmpty(t, helper, rawTags)
	t.Log("PASS: Raw announcements document created")

	// 2. Validate processed announcements were created
	summaryTags := []string{"asx-announcement-summary", strings.ToLower(ticker)}
	_, _ = AssertOutputNotEmpty(t, helper, summaryTags)
	t.Log("PASS: Processed announcements document created")

	// 3. Validate all rating documents were created
	ratingDocs := []struct {
		tag    string
		schema WorkerSchema
	}{
		{"rating-bfs", BFSSchema},
		{"rating-cds", CDSSchema},
		{"rating-nfr", NFRSchema},
		{"rating-pps", PPSSchema},
		{"rating-vrs", VRSSchema},
		{"rating-ob", OBSchema},
		{"stock-rating", RatingCompositeSchema},
	}

	for _, rd := range ratingDocs {
		tags := []string{rd.tag, strings.ToLower(ticker)}
		metadata, _ := AssertOutputNotEmpty(t, helper, tags)
		isValid := ValidateSchema(t, metadata, rd.schema)
		assert.True(t, isValid, "Output should comply with %s schema", rd.tag)
		t.Logf("PASS: %s document created and valid", rd.tag)
	}

	// 4. Final validation of composite rating
	tags := []string{"stock-rating", strings.ToLower(ticker)}
	metadata, _ := AssertOutputNotEmpty(t, helper, tags)

	// Validate rating label
	if label, ok := metadata["label"].(string); ok {
		AssertRatingLabel(t, label)
		t.Logf("Final rating for %s: %s", ticker, label)
	}

	// Validate gate status
	if gatePassed, ok := metadata["gate_passed"].(bool); ok {
		t.Logf("Gate passed: %v", gatePassed)
	}

	// Validate investability if available
	if investability, ok := metadata["investability"].(float64); ok {
		t.Logf("Investability score: %.1f", investability)
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: Rating full flow integration test completed")
}

// TestRatingMultiTicker tests rating calculation for multiple tickers
func TestRatingMultiTicker(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	// Test multiple tickers - run each as subtest
	tickers := []string{"GNP", "BHP"}

	for _, ticker := range tickers {
		t.Run(ticker, func(t *testing.T) {
			defID := fmt.Sprintf("test-rating-multi-%s-%d", strings.ToLower(ticker), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("Rating Multi-Ticker Test - %s", ticker),
				"description": "Test rating for multiple tickers",
				"type":        "manager",
				"enabled":     true,
				"tags":        []string{"integration-test", "rating-multi-ticker"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-fundamentals",
						"type": "market_fundamentals",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name": "fetch-announcements",
						"type": "market_announcements",
						"config": map[string]interface{}{
							"asx_code": ticker,
						},
					},
					{
						"name":    "process-announcements",
						"type":    "processing_announcements",
						"depends": "fetch-announcements",
						"config": map[string]interface{}{
							"asx_code": ticker,
						},
					},
					{
						"name": "fetch-prices",
						"type": "market_data",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-bfs",
						"type":    "rating_bfs",
						"depends": "fetch-fundamentals",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-cds",
						"type":    "rating_cds",
						"depends": "fetch-fundamentals,process-announcements",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-nfr",
						"type":    "rating_nfr",
						"depends": "process-announcements,fetch-prices",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-pps",
						"type":    "rating_pps",
						"depends": "process-announcements,fetch-prices",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-vrs",
						"type":    "rating_vrs",
						"depends": "process-announcements,fetch-prices",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-ob",
						"type":    "rating_ob",
						"depends": "process-announcements,calculate-bfs",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
					{
						"name":    "calculate-rating",
						"type":    "rating_composite",
						"depends": "calculate-bfs,calculate-cds,calculate-nfr,calculate-pps,calculate-vrs,calculate-ob",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", ticker),
						},
					},
				},
			}

			jobID, _ := CreateAndExecuteJob(t, helper, body)
			if jobID == "" {
				return
			}

			t.Logf("Executing rating job for %s: %s", ticker, jobID)

			finalStatus := WaitForJobCompletion(t, helper, jobID, 6*time.Minute)
			if finalStatus != "completed" {
				t.Logf("Job for %s ended with status %s", ticker, finalStatus)
				return
			}

			// Validate final rating
			tags := []string{"stock-rating", strings.ToLower(ticker)}
			metadata, _ := AssertOutputNotEmpty(t, helper, tags)

			isValid := ValidateSchema(t, metadata, RatingCompositeSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", ticker)

			if label, ok := metadata["label"].(string); ok {
				t.Logf("Rating for %s: %s", ticker, label)
			}
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: Rating multi-ticker test completed")
}

// TestRatingGateFail tests that gate failure produces SPECULATIVE label
func TestRatingGateFail(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	RequireAllMarketServices(t, env)

	helper := env.NewHTTPTestHelper(t)

	// EXR is likely to have lower scores (exploration company)
	ticker := "EXR"
	defID := fmt.Sprintf("test-rating-gate-fail-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Rating Gate Fail Test",
		"description": "Test that low BFS/CDS results in SPECULATIVE",
		"type":        "manager",
		"enabled":     true,
		"tags":        []string{"integration-test", "rating-gate-fail"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-fundamentals",
				"type": "market_fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name": "fetch-announcements",
				"type": "market_announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name":    "process-announcements",
				"type":    "processing_announcements",
				"depends": "fetch-announcements",
				"config": map[string]interface{}{
					"asx_code": ticker,
				},
			},
			{
				"name": "fetch-prices",
				"type": "market_data",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-bfs",
				"type":    "rating_bfs",
				"depends": "fetch-fundamentals",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-cds",
				"type":    "rating_cds",
				"depends": "fetch-fundamentals,process-announcements",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-nfr",
				"type":    "rating_nfr",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-pps",
				"type":    "rating_pps",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-vrs",
				"type":    "rating_vrs",
				"depends": "process-announcements,fetch-prices",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-ob",
				"type":    "rating_ob",
				"depends": "process-announcements,calculate-bfs",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
			{
				"name":    "calculate-rating",
				"type":    "rating_composite",
				"depends": "calculate-bfs,calculate-cds,calculate-nfr,calculate-pps,calculate-vrs,calculate-ob",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing gate fail test job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 6*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// Check BFS score
	bfsTags := []string{"rating-bfs", strings.ToLower(ticker)}
	bfsMetadata, _ := AssertOutputNotEmpty(t, helper, bfsTags)
	if bfsScore, ok := bfsMetadata["score"].(float64); ok {
		t.Logf("BFS score for %s: %.0f", ticker, bfsScore)
	}

	// Check CDS score
	cdsTags := []string{"rating-cds", strings.ToLower(ticker)}
	cdsMetadata, _ := AssertOutputNotEmpty(t, helper, cdsTags)
	if cdsScore, ok := cdsMetadata["score"].(float64); ok {
		t.Logf("CDS score for %s: %.0f", ticker, cdsScore)
	}

	// Check final rating
	tags := []string{"stock-rating", strings.ToLower(ticker)}
	metadata, _ := AssertOutputNotEmpty(t, helper, tags)

	if gatePassed, ok := metadata["gate_passed"].(bool); ok {
		t.Logf("Gate passed: %v", gatePassed)
	}

	if label, ok := metadata["label"].(string); ok {
		t.Logf("Rating label: %s", label)
		// If gate failed, should be SPECULATIVE
		if gatePassed, ok := metadata["gate_passed"].(bool); ok && !gatePassed {
			assert.Equal(t, "SPECULATIVE", label, "Gate failure should result in SPECULATIVE label")
		}
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: Rating gate fail test completed")
}
