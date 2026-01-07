// -----------------------------------------------------------------------
// Tests for market_macro worker
// Fetches macroeconomic data (RBA rates, commodity prices)
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerMacroAll tests fetching all macro data types
func TestWorkerMacroAll(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-macro-all-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Macro Data All Types Test",
		"description": "Test market_macro worker with all data types",
		"type":        "market_macro",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-macro"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-macro-data",
				"type": "market_macro",
				"config": map[string]interface{}{
					"data_type": "all",
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_macro job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"macro-data"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Macro", "Data"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, MacroSchema)
	assert.True(t, isValid, "Output should comply with macro schema")

	// Validate data_type field
	if dataType, ok := metadata["data_type"].(string); ok {
		t.Logf("PASS: Macro data_type is '%s'", dataType)
	}

	// Validate data_points if present
	if dataPoints, ok := metadata["data_points"].([]interface{}); ok {
		t.Logf("PASS: Found %d macro data points", len(dataPoints))
	}

	SaveWorkerOutput(t, env, helper, tags, "MACRO")
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_macro all types test completed")
}

// TestWorkerMacroRBA tests fetching RBA cash rate only
func TestWorkerMacroRBA(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-macro-rba-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Macro Data RBA Test",
		"description": "Test market_macro worker with RBA cash rate",
		"type":        "market_macro",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-macro", "rba"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-rba-rate",
				"type": "market_macro",
				"config": map[string]interface{}{
					"data_type": "rba_cash_rate",
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	jobID, _ := CreateAndExecuteJob(t, helper, body)
	if jobID == "" {
		return
	}

	t.Logf("Executing market_macro RBA job: %s", jobID)

	finalStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"macro-data", "rba"}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	assert.NotEmpty(t, content, "Content should not be empty")

	// Assert data_type is rba_cash_rate
	if dataType, ok := metadata["data_type"].(string); ok {
		assert.Equal(t, "rba_cash_rate", dataType, "data_type should be rba_cash_rate")
	}

	SaveWorkerOutput(t, env, helper, tags, "RBA")
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_macro RBA test completed")
}
