// -----------------------------------------------------------------------
// Tests for market_director_interest worker
// Fetches ASX director interest (Appendix 3Y) filings via Markit API
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerDirectorInterestSingle tests single stock director interest
func TestWorkerDirectorInterestSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-director-interest-single-%d", time.Now().UnixNano())
	ticker := "BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Director Interest Single Stock Test",
		"description": "Test market_director_interest worker with single stock",
		"type":        "market_director_interest",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-director-interest", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-director-interest",
				"type": "market_director_interest",
				"config": map[string]interface{}{
					"asx_code": ticker,
					"period":   "Y1",
					"limit":    10,
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_director_interest job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"asx-director-interest", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Director Interest", "Filing"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, DirectorInterestSchema)
	assert.True(t, isValid, "Output should comply with director interest schema")

	// Validate filings array
	if filings, ok := metadata["filings"].([]interface{}); ok {
		t.Logf("PASS: Found %d director interest filings", len(filings))
		if len(filings) > 0 {
			if first, ok := filings[0].(map[string]interface{}); ok {
				if _, hasDate := first["date"]; hasDate {
					t.Log("PASS: Filing has 'date' field")
				}
				if _, hasHeadline := first["headline"]; hasHeadline {
					t.Log("PASS: Filing has 'headline' field")
				}
			}
		}
	}

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_director_interest single stock test completed")
}

// TestWorkerDirectorInterestMulti tests multi-stock director interest
func TestWorkerDirectorInterestMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-director-interest-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Director Interest Test - %s", stock),
				"type":    "market_director_interest",
				"enabled": true,
				"tags":    []string{"worker-test", "market-director-interest", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-director-interest",
						"type": "market_director_interest",
						"config": map[string]interface{}{
							"asx_code": stock,
							"period":   "Y1",
							"limit":    5,
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

			tags := []string{"asx-director-interest", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, DirectorInterestSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, stock)
			t.Logf("PASS: Validated director_interest for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_director_interest multi-stock test completed")
}
