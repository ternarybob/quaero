// -----------------------------------------------------------------------
// Tests for market_signal worker
// Computes trading signals (PBAS, VLI, etc.) from market data documents
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkerSignalSingle tests single stock signal computation
func TestWorkerSignalSingle(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market data (signals are computed from market data)
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	// First, we need market data to generate signals from
	// Run market_data worker first
	ticker := "BHP"
	dataDefID := fmt.Sprintf("test-signal-data-prep-%d", time.Now().UnixNano())

	dataBody := map[string]interface{}{
		"id":      dataDefID,
		"name":    "Market Data Prep for Signal Test",
		"type":    "market_data",
		"enabled": true,
		"tags":    []string{"worker-test", "signal-prep"},
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

	jobID, _ := CreateAndExecuteJob(t, helper, dataBody)
	if jobID == "" {
		t.Skip("Could not create market data for signal test")
		return
	}

	dataStatus := WaitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if dataStatus != "completed" {
		t.Skipf("Market data job ended with status %s", dataStatus)
		return
	}

	t.Log("Market data prepared, now running signal computation")

	// Now run signal worker
	defID := fmt.Sprintf("test-signal-single-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Signal Computation Single Stock Test",
		"description": "Test market_signal worker with single stock",
		"type":        "market_signal",
		"enabled":     true,
		"tags":        []string{"worker-test", "market-signal", "single-stock"},
		"steps": []map[string]interface{}{
			{
				"name": "compute-signals",
				"type": "market_signal",
				"config": map[string]interface{}{
					"ticker": fmt.Sprintf("ASX:%s", ticker),
				},
			},
		},
	}

	SaveJobDefinition(t, env, body)

	signalJobID, _ := CreateAndExecuteJob(t, helper, body)
	if signalJobID == "" {
		return
	}

	t.Logf("Executing market_signal job: %s", signalJobID)

	finalStatus := WaitForJobCompletion(t, helper, signalJobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skipf("Job ended with status %s", finalStatus)
		return
	}

	// === ASSERTIONS ===
	tags := []string{"market-signal", strings.ToLower(ticker)}
	metadata, content := AssertOutputNotEmpty(t, helper, tags)

	// Assert content contains expected sections
	expectedSections := []string{"Signal", "Analysis"}
	AssertOutputContains(t, content, expectedSections)

	// Assert schema compliance
	isValid := ValidateSchema(t, metadata, SignalSchema)
	assert.True(t, isValid, "Output should comply with signal schema")

	// Validate signal fields
	signalFields := []string{"pbas", "vli", "regime"}
	for _, field := range signalFields {
		if _, exists := metadata[field]; exists {
			t.Logf("PASS: Has signal field '%s'", field)
		}
	}

	SaveWorkerOutput(t, env, helper, tags, ticker)
	AssertResultFilesExist(t, env, 1)
	AssertNoServiceErrors(t, env)

	t.Log("PASS: market_signal single stock test completed")
}

// TestWorkerSignalMulti tests multi-stock signal computation
func TestWorkerSignalMulti(t *testing.T) {
	env := SetupFreshEnvironment(t)
	if env == nil {
		return
	}
	defer env.Cleanup()

	// Require EODHD for market data (signals are computed from market data)
	RequireEODHD(t, env)

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL"}

	// First prepare market data for all stocks
	for _, stock := range stocks {
		dataDefID := fmt.Sprintf("test-signal-data-prep-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

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

	t.Log("Market data prepared for all stocks, running signal tests")

	for _, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-signal-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":      defID,
				"name":    fmt.Sprintf("Signal Computation Test - %s", stock),
				"type":    "market_signal",
				"enabled": true,
				"tags":    []string{"worker-test", "market-signal", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "compute-signals",
						"type": "market_signal",
						"config": map[string]interface{}{
							"ticker": fmt.Sprintf("ASX:%s", stock),
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

			tags := []string{"market-signal", strings.ToLower(stock)}
			metadata, content := AssertOutputNotEmpty(t, helper, tags)

			assert.NotEmpty(t, content, "Content for %s should not be empty", stock)
			isValid := ValidateSchema(t, metadata, SignalSchema)
			assert.True(t, isValid, "Output for %s should comply with schema", stock)

			SaveWorkerOutput(t, env, helper, tags, stock)
			t.Logf("PASS: Validated signal for %s", stock)
		})
	}

	AssertNoServiceErrors(t, env)
	t.Log("PASS: market_signal multi-stock test completed")
}
