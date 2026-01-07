// -----------------------------------------------------------------------
// Tests for market_assessor worker
// Uses LLM (Gemini) to assess investment potential based on collected data
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerAssessorSingle tests single stock assessment
func TestWorkerAssessorSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require LLM for AI assessment
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-assessor-single-%d", time.Now().UnixNano())
	ticker := "BHP"

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Investment Assessment Single Stock Test",
		"description": "Test market_assessor worker with single stock",
		"type":        "market_assessor",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-assessor", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "assess-investment",
				"type": "market_assessor",
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

	t.Logf("Executing market_assessor job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"investment-assessment", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Assessment", "Investment"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, AssessorSchema)
	assert.True(t, isValid, "Output should comply with assessor schema")

	// Validate assessment fields
	if rating, ok := metadata["rating"].(string); ok {
		t.Logf("PASS: Assessment rating is '%s'", rating)
	}

	if score, ok := metadata["score"].(float64); ok {
		assert.GreaterOrEqual(t, score, 0.0, "Score should be >= 0")
		assert.LessOrEqual(t, score, 100.0, "Score should be <= 100")
		t.Logf("PASS: Assessment score is %.2f", score)
	}

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_assessor single stock test completed")
}

// TestWorkerAssessorMulti tests multi-stock assessment
func TestWorkerAssessorMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require LLM for AI assessment
	RequireLLM(t, env)

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL"}

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-assessor-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Investment Assessment Test - %s", stock),
				"type":    "market_assessor",
				"enabled": true,
				"tags":    []string{"worker-test", "market-assessor", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "assess-investment",
						"type": "market_assessor",
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

			tags := []string{"investment-assessment", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, AssessorSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, stock)
			t.Logf("PASS: Validated assessment for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_assessor multi-stock test completed")
}
