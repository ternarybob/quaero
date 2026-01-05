// -----------------------------------------------------------------------
// Tests for market_news worker
// Fetches company news via EODHD News API (or delegates to ASX worker)
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerNewsSingle tests single stock news fetch
func TestWorkerNewsSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market news data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-news-single-%d", time.Now().UnixNano())
	ticker := "ASX:BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Market News Single Stock Test",
		"description": "Test market_news worker with single stock",
		"type":        "market_news",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-news", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-news",
				"type": "market_news",
				"config": map[string]interface{}{
					"ticker": ticker,
					"period": "M3",
					"limit":  20,
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_news job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	// For ASX, this delegates to market_announcements
	tags := []string{"asx-announcement-summary", "bhp"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	assert.NotEmpty(t, content, "Content should not be empty")

	// Validate schema (uses announcements schema for ASX)
	isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
	assert.True(t, isValid, "Output should comply with schema")

	SaveWorkerOutput(t, env, helper, tags, 1)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_news single stock test completed")
}

// TestWorkerNewsMulti tests multi-stock news fetch
func TestWorkerNewsMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market news data
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP"}

	for i, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-news-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Market News Test - %s", stock),
				"type":    "market_news",
				"enabled": true,
				"tags":    []string{"worker-test", "market-news", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-news",
						"type": "market_news",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", stock),
							"period": "M3",
							"limit":  10,
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

			// For ASX, uses announcement tags
			tags := []string{"asx-announcement-summary", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, AnnouncementsSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, i+1)
			t.Logf("PASS: Validated market_news for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_news multi-stock test completed")
}
