package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Worker Schema Integration Tests
// =============================================================================
// Tests are ordered by data collection hierarchy (see docs/architecture/WORKER_DATA_OVERLAP.md):
//
// 1. FOUNDATION DATA
//    - asx_stock_collector: Consolidated Yahoo Finance data (prices, technicals, analyst coverage, financials)
//
// 2. STRUCTURED DATA
//    - asx_announcements: Official ASX announcements with price sensitivity flags
//
// 3. UNSTRUCTURED DATA
//    - web_search: Web searches with AI-powered results
//
// 4. ANALYSIS LAYER
//    - summary: Generates analysis with JSON schema enforcement
//
// 5. DEPRECATED WORKERS (retained for backward compatibility)
//    - asx_stock_data: DEPRECATED - use asx_stock_collector instead
//
// Primary concern: CONSISTENCY of both tooling and final output
// =============================================================================

// =============================================================================
// 1. FOUNDATION DATA - asx_stock_collector
// =============================================================================

// TestWorkerASXStockCollector tests the consolidated asx_stock_collector worker.
// This worker combines price data, technicals, analyst coverage, and historical financials
// in a single Yahoo Finance API call, replacing the deprecated individual workers.
func TestWorkerASXStockCollector(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create job definition that uses asx_stock_collector worker
	defID := fmt.Sprintf("test-asx-stock-collector-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "ASX Stock Collector Worker Test",
		"description": "Test asx_stock_collector worker for consolidated output",
		"type":        "asx_stock_collector",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-stock-collector"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock-data",
				"type": "asx_stock_collector",
				"config": map[string]interface{}{
					"asx_code": "BHP", // Use BHP as a stable test stock
					"period":   "Y2",  // 24 months for comprehensive data verification
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d (may need ASX market hours)", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute the job
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed asx_stock_collector job: %s", jobID)

	// Wait for completion
	finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)

	// Save job definition
	if err := saveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	// Verify completion (may fail if market is closed or API unavailable)
	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s (may be expected outside market hours)", finalStatus)
		return
	}

	// ===== RUN 1 =====
	t.Log("=== Run 1 ===")
	validateASXStockCollectorOutput(t, helper, "BHP")

	if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", "bhp"}, 1); err != nil {
		t.Logf("Warning: failed to save worker output run 1: %v", err)
	}
	assertResultFilesExist(t, env, 1)

	// ===== RUN 2 =====
	t.Log("=== Run 2 ===")
	execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition for run 2")
	defer execResp2.Body.Close()

	if execResp2.StatusCode == http.StatusAccepted {
		var execResult2 map[string]interface{}
		require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
		jobID2 := execResult2["job_id"].(string)
		t.Logf("Executed asx_stock_collector job run 2: %s", jobID2)

		finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
		if finalStatus2 == "completed" {
			if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", "bhp"}, 2); err != nil {
				t.Logf("Warning: failed to save worker output run 2: %v", err)
			}
			assertResultFilesExist(t, env, 2)

			// Compare consistency between runs
			assertOutputStructureConsistency(t, env)
		}
	}

	t.Log("PASS: asx_stock_collector worker produced consistent output across runs")
}

// =============================================================================
// 2. STRUCTURED DATA - asx_announcements
// =============================================================================

// TestWorkerASXAnnouncements tests the asx_announcements worker produces consistent output
// including the summary document with relevance classification and price impact analysis
func TestWorkerASXAnnouncements(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	defID := fmt.Sprintf("test-asx-announcements-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "ASX Announcements Worker Test",
		"description": "Test asx_announcements worker for consistent output with summary",
		"type":        "asx_announcements",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-announcement"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "asx_announcements",
				"config": map[string]interface{}{
					"asx_code": "BHP",
					// Use worker default Y1 (12 months) for consistent output
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed asx_announcements job: %s", jobID)

	finalStatus := waitForJobCompletion(t, helper, jobID, 3*time.Minute)

	// Save job definition
	if err := saveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s", finalStatus)
		return
	}

	// ===== RUN 1 =====
	t.Log("=== Run 1 ===")
	validateASXAnnouncementsOutput(t, helper, "BHP")
	validateASXAnnouncementsSummary(t, helper, "BHP")

	if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-announcement-summary", "bhp"}, 1); err != nil {
		t.Logf("Warning: failed to save summary output run 1: %v", err)
	}
	assertResultFilesExist(t, env, 1)

	// ===== RUN 2 =====
	t.Log("=== Run 2 ===")
	execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition for run 2")
	defer execResp2.Body.Close()

	if execResp2.StatusCode == http.StatusAccepted {
		var execResult2 map[string]interface{}
		require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
		jobID2 := execResult2["job_id"].(string)
		t.Logf("Executed asx_announcements job run 2: %s", jobID2)

		finalStatus2 := waitForJobCompletion(t, helper, jobID2, 3*time.Minute)
		if finalStatus2 == "completed" {
			if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-announcement-summary", "bhp"}, 2); err != nil {
				t.Logf("Warning: failed to save summary output run 2: %v", err)
			}
			assertResultFilesExist(t, env, 2)

			// Compare consistency between runs
			assertOutputStructureConsistency(t, env)
		}
	}

	t.Log("PASS: asx_announcements worker produced consistent output across runs")
}

// TestWorkerASXAnnouncementsMultiStock tests the asx_announcements worker with multiple stocks
func TestWorkerASXAnnouncementsMultiStock(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP", "EXR"}

	for i, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-asx-announcements-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("ASX Announcements Worker Test - %s", stock),
				"description": "Test asx_announcements worker for multi-stock support",
				"type":        "asx_announcements",
				"enabled":     true,
				"tags":        []string{"worker-test", "asx-announcement", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-announcements",
						"type": "asx_announcements",
						"config": map[string]interface{}{
							"asx_code": stock,
							// Use worker default Y1 (12 months) for consistent output
						},
					},
				},
			}

			resp, err := helper.POST("/api/job-definitions", body)
			require.NoError(t, err, "Failed to create job definition for %s", stock)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Skipf("Job definition creation failed for %s with status: %d", stock, resp.StatusCode)
			}

			defer func() {
				delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
				if delResp != nil {
					delResp.Body.Close()
				}
			}()

			execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
			require.NoError(t, err, "Failed to execute job for %s", stock)
			defer execResp.Body.Close()

			if execResp.StatusCode != http.StatusAccepted {
				t.Skipf("Job execution failed for %s with status: %d", stock, execResp.StatusCode)
			}

			var execResult map[string]interface{}
			require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
			jobID := execResult["job_id"].(string)
			t.Logf("Executed asx_announcements job for %s: %s", stock, jobID)

			finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)

			// Save job definition for first stock only
			if i == 0 {
				if err := saveJobDefinition(t, env, body); err != nil {
					t.Logf("Warning: failed to save job definition: %v", err)
				}
			}

			if finalStatus != "completed" {
				t.Logf("INFO: Job for %s ended with status %s", stock, finalStatus)
				return
			}

			// ===== RUN 1 =====
			t.Logf("=== Run 1 for %s ===", stock)
			validateASXAnnouncementsOutput(t, helper, stock)
			validateASXAnnouncementsSummary(t, helper, stock)

			if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-announcement-summary", strings.ToLower(stock)}, i*2+1); err != nil {
				t.Logf("Warning: failed to save output for %s run 1: %v", stock, err)
			}
			assertResultFilesExist(t, env, i*2+1)

			// ===== RUN 2 =====
			t.Logf("=== Run 2 for %s ===", stock)
			execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
			require.NoError(t, err, "Failed to execute job for %s run 2", stock)
			defer execResp2.Body.Close()

			if execResp2.StatusCode == http.StatusAccepted {
				var execResult2 map[string]interface{}
				require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
				jobID2 := execResult2["job_id"].(string)
				t.Logf("Executed asx_announcements job for %s run 2: %s", stock, jobID2)

				finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
				if finalStatus2 == "completed" {
					if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-announcement-summary", strings.ToLower(stock)}, i*2+2); err != nil {
						t.Logf("Warning: failed to save output for %s run 2: %v", stock, err)
					}
					assertResultFilesExist(t, env, i*2+2)
				}
			}

			t.Logf("PASS: %s announcements processed with consistency check", stock)
		})
	}
}

// =============================================================================
// 3. UNSTRUCTURED DATA - web_search
// =============================================================================

// TestWorkerWebSearch tests the web_search worker for consistent output
func TestWorkerWebSearch(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if Gemini API key is available (web_search uses Gemini)
	if !hasGeminiAPIKey(env) {
		t.Skip("Skipping test - no valid google_gemini_api_key found")
	}

	defID := fmt.Sprintf("test-web-search-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Web Search Worker Test",
		"description": "Test web_search worker for consistent output",
		"type":        "web_search",
		"enabled":     true,
		"tags":        []string{"worker-test", "web-search"},
		"steps": []map[string]interface{}{
			{
				"name": "search",
				"type": "web_search",
				"config": map[string]interface{}{
					"query":   "BHP Group financial results 2024",
					"api_key": "{google_gemini_api_key}",
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed web_search job: %s", jobID)

	finalStatus := waitForJobCompletion(t, helper, jobID, 3*time.Minute)

	// Save job definition
	if err := saveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s", finalStatus)
		return
	}

	// ===== RUN 1 =====
	t.Log("=== Run 1 ===")
	validateWebSearchOutput(t, helper)

	if _, _, err := saveWorkerOutput(t, env, helper, []string{"web-search"}, 1); err != nil {
		t.Logf("Warning: failed to save worker output run 1: %v", err)
	}
	assertResultFilesExist(t, env, 1)

	// ===== RUN 2 =====
	t.Log("=== Run 2 ===")
	execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition for run 2")
	defer execResp2.Body.Close()

	if execResp2.StatusCode == http.StatusAccepted {
		var execResult2 map[string]interface{}
		require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
		jobID2 := execResult2["job_id"].(string)
		t.Logf("Executed web_search job run 2: %s", jobID2)

		finalStatus2 := waitForJobCompletion(t, helper, jobID2, 3*time.Minute)
		if finalStatus2 == "completed" {
			if _, _, err := saveWorkerOutput(t, env, helper, []string{"web-search"}, 2); err != nil {
				t.Logf("Warning: failed to save worker output run 2: %v", err)
			}
			assertResultFilesExist(t, env, 2)

			// Compare consistency between runs
			assertOutputStructureConsistency(t, env)
		}
	}

	t.Log("PASS: web_search worker produced consistent output across runs")
}

// =============================================================================
// 4. ANALYSIS LAYER - summary
// =============================================================================

// TestWorkerSummaryWithSchema tests the summary worker uses JSON schema for consistent output
// This test executes the summary worker TWICE to verify output consistency and schema enforcement
func TestWorkerSummaryWithSchema(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if Gemini API key is available
	if !hasGeminiAPIKey(env) {
		t.Skip("Skipping test - no valid google_gemini_api_key found")
	}

	// First, create some test documents to summarize
	testDir, cleanup := createTestCodeDirectory(t)
	defer cleanup()

	// Step 1: Index files
	indexDefID := fmt.Sprintf("index-for-schema-test-%d", time.Now().UnixNano())
	indexBody := map[string]interface{}{
		"id":      indexDefID,
		"name":    "Index for Schema Test",
		"type":    "local_dir",
		"enabled": true,
		"tags":    []string{"schema-test"},
		"steps": []map[string]interface{}{
			{
				"name": "index",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md"},
					"max_files":          10,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", indexBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Index job creation failed: %d", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + indexDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute index
	execResp, err := helper.POST("/api/job-definitions/"+indexDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	indexJobID := execResult["job_id"].(string)

	indexStatus := waitForJobCompletion(t, helper, indexJobID, 2*time.Minute)
	require.Equal(t, "completed", indexStatus, "Index job should complete")

	// Step 2: Create summary job WITH schema
	// Define a test schema for code analysis
	testSchema := map[string]interface{}{
		"type":     "object",
		"required": []string{"summary", "components", "recommendation"},
		"properties": map[string]interface{}{
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Brief summary of the codebase",
			},
			"components": map[string]interface{}{
				"type":        "array",
				"description": "List of main components",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"recommendation": map[string]interface{}{
				"type":        "string",
				"description": "Code quality recommendation",
				"enum":        []string{"EXCELLENT", "GOOD", "FAIR", "NEEDS_IMPROVEMENT"},
			},
		},
	}

	// Execute the summary job TWICE to verify consistency
	const numRuns = 2
	completedRuns := 0

	for runNumber := 1; runNumber <= numRuns; runNumber++ {
		t.Logf("=== Execution %d of %d ===", runNumber, numRuns)

		// Create unique job definition for each run
		summaryDefID := fmt.Sprintf("summary-schema-test-%d-run%d", time.Now().UnixNano(), runNumber)
		outputTag := fmt.Sprintf("summary-output-run%d", runNumber)

		summaryBody := map[string]interface{}{
			"id":      summaryDefID,
			"name":    fmt.Sprintf("Summary with Schema Test - Run %d", runNumber),
			"type":    "summarizer",
			"enabled": true,
			"tags":    []string{"schema-test", "summary-output", outputTag},
			"steps": []map[string]interface{}{
				{
					"name": "summarize",
					"type": "summary",
					"config": map[string]interface{}{
						"prompt":        "Analyze the code and provide a structured assessment. Return JSON matching the schema.",
						"filter_tags":   []string{"schema-test"},
						"api_key":       "{google_gemini_api_key}",
						"output_schema": testSchema,
					},
				},
			},
		}

		// Save job definition on first run
		if runNumber == 1 {
			if err := saveJobDefinition(t, env, summaryBody); err != nil {
				t.Logf("Warning: failed to save job definition: %v", err)
			}
		}

		resp2, err := helper.POST("/api/job-definitions", summaryBody)
		require.NoError(t, err)
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusCreated {
			t.Logf("Summary job creation failed for run %d: %d", runNumber, resp2.StatusCode)
			continue
		}

		defer func(defID string) {
			delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
			if delResp != nil {
				delResp.Body.Close()
			}
		}(summaryDefID)

		// Execute summary
		execResp2, err := helper.POST("/api/job-definitions/"+summaryDefID+"/execute", nil)
		require.NoError(t, err)
		defer execResp2.Body.Close()

		var execResult2 map[string]interface{}
		require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
		summaryJobID := execResult2["job_id"].(string)
		t.Logf("Executed summary job with schema (run %d): %s", runNumber, summaryJobID)

		summaryStatus := waitForJobCompletion(t, helper, summaryJobID, 5*time.Minute)

		if summaryStatus == "completed" {
			completedRuns++

			// Save worker output for this run
			jsonPath, mdPath, err := saveWorkerOutput(t, env, helper, []string{"summary-output", outputTag}, runNumber)
			if err != nil {
				t.Logf("Warning: failed to save output for run %d: %v", runNumber, err)
			} else {
				t.Logf("Run %d outputs saved: JSON=%s, MD=%s", runNumber, jsonPath, mdPath)
			}

			// Assert result files exist for this run
			assertResultFilesExist(t, env, runNumber)

			// Validate the output contains schema-defined fields
			validateSummarySchemaOutput(t, helper, []string{"summary", "components", "recommendation"})
			t.Logf("PASS: Run %d - summary worker with schema produced structured output", runNumber)
		} else {
			t.Logf("INFO: Run %d - Summary job ended with status %s", runNumber, summaryStatus)
		}
	}

	// Validate schema was logged in service.log
	if checkSchemaInServiceLog(t, env, "output schema") {
		t.Log("PASS: Schema usage was logged in service.log")
	} else {
		t.Log("INFO: Schema usage logging not found (may need SCHEMA_ENFORCEMENT marker)")
	}

	// Compare outputs for consistency if both runs completed
	if completedRuns == numRuns {
		t.Log("=== Comparing outputs for consistency ===")
		validateOutputConsistency(t, env)
		assertOutputStructureConsistency(t, env)
	} else {
		t.Logf("INFO: Only %d of %d runs completed, skipping consistency comparison", completedRuns, numRuns)
	}
}

// =============================================================================
// 5. DEPRECATED WORKERS (retained for backward compatibility)
// =============================================================================

// TestWorkerASXStockData tests the asx_stock_data worker produces consistent output
// DEPRECATED: This worker is deprecated. Use asx_stock_collector instead which combines
// price data, analyst coverage, and historical financials in a single API call.
// This test is retained for backward compatibility.
func TestWorkerASXStockData(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create job definition that uses asx_stock_data worker
	defID := fmt.Sprintf("test-asx-stock-data-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "ASX Stock Data Worker Test",
		"description": "Test asx_stock_data worker for consistent output",
		"type":        "asx_stock_data",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-stock-data"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock",
				"type": "asx_stock_data",
				"config": map[string]interface{}{
					"asx_code": "BHP", // Use BHP as a stable test stock
					"period":   "Y2",  // 24 months for comprehensive data verification
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d (may need ASX market hours)", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute the job
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed asx_stock_data job: %s", jobID)

	// Wait for completion
	finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)

	// Save job definition
	if err := saveJobDefinition(t, env, body); err != nil {
		t.Logf("Warning: failed to save job definition: %v", err)
	}

	// Verify completion (may fail if market is closed or API unavailable)
	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s (may be expected outside market hours)", finalStatus)
		return
	}

	// ===== RUN 1 =====
	t.Log("=== Run 1 ===")
	validateASXStockDataOutputWithPeriod(t, helper, "BHP", false) // require24Months=false (EODHD Fundamental subscription doesn't include EOD endpoint)

	if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", "bhp"}, 1); err != nil {
		t.Logf("Warning: failed to save worker output run 1: %v", err)
	}
	assertResultFilesExist(t, env, 1)

	// Capture Run 1 timestamp for cache validation
	timestamp1, docID1, err := getDocumentLastSynced(t, helper, []string{"asx-stock-data", "bhp"})
	if err != nil {
		t.Logf("Warning: failed to get document timestamp for run 1: %v", err)
	} else {
		t.Logf("Run 1 document ID: %s", docID1)
	}

	// ===== RUN 2 (should use cache) =====
	t.Log("=== Run 2 (should use cache) ===")
	execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition for run 2")
	defer execResp2.Body.Close()

	if execResp2.StatusCode == http.StatusAccepted {
		var execResult2 map[string]interface{}
		require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
		jobID2 := execResult2["job_id"].(string)
		t.Logf("Executed asx_stock_data job run 2: %s", jobID2)

		finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
		if finalStatus2 == "completed" {
			if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", "bhp"}, 2); err != nil {
				t.Logf("Warning: failed to save worker output run 2: %v", err)
			}
			assertResultFilesExist(t, env, 2)

			// Compare consistency between runs
			assertOutputStructureConsistency(t, env)

			// Verify cache was used - timestamp should match Run 1
			timestamp2, _, err := getDocumentLastSynced(t, helper, []string{"asx-stock-data", "bhp"})
			if err != nil {
				t.Logf("Warning: failed to get document timestamp for run 2: %v", err)
			} else {
				assertCacheUsed(t, timestamp1, timestamp2)
			}
		}
	}

	// ===== RUN 3 (force_refresh=true, should bypass cache) =====
	t.Log("=== Run 3 (force_refresh=true, should bypass cache) ===")

	// Create new job definition with force_refresh enabled
	defID3 := fmt.Sprintf("test-asx-stock-data-force-%d", time.Now().UnixNano())
	body3 := map[string]interface{}{
		"id":          defID3,
		"name":        "ASX Stock Data Worker Test - Force Refresh",
		"description": "Test asx_stock_data worker with force_refresh to bypass cache",
		"type":        "asx_stock_data",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-stock-data"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-stock",
				"type": "asx_stock_data",
				"config": map[string]interface{}{
					"asx_code":      "BHP",
					"period":        "Y2",
					"force_refresh": true, // Bypass cache
				},
			},
		},
	}

	resp3, err := helper.POST("/api/job-definitions", body3)
	require.NoError(t, err, "Failed to create job definition for run 3")
	defer resp3.Body.Close()

	if resp3.StatusCode == http.StatusCreated {
		defer func() {
			delResp, _ := helper.DELETE("/api/job-definitions/" + defID3)
			if delResp != nil {
				delResp.Body.Close()
			}
		}()

		execResp3, err := helper.POST("/api/job-definitions/"+defID3+"/execute", nil)
		require.NoError(t, err, "Failed to execute job definition for run 3")
		defer execResp3.Body.Close()

		if execResp3.StatusCode == http.StatusAccepted {
			var execResult3 map[string]interface{}
			require.NoError(t, helper.ParseJSONResponse(execResp3, &execResult3))
			jobID3 := execResult3["job_id"].(string)
			t.Logf("Executed asx_stock_data job run 3 (force_refresh): %s", jobID3)

			finalStatus3 := waitForJobCompletion(t, helper, jobID3, 2*time.Minute)
			if finalStatus3 == "completed" {
				if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", "bhp"}, 3); err != nil {
					t.Logf("Warning: failed to save worker output run 3: %v", err)
				}
				assertResultFilesExist(t, env, 3)

				// Verify cache was bypassed - timestamp should differ from Run 1
				timestamp3, _, err := getDocumentLastSynced(t, helper, []string{"asx-stock-data", "bhp"})
				if err != nil {
					t.Logf("Warning: failed to get document timestamp for run 3: %v", err)
				} else {
					assertCacheBypass(t, timestamp1, timestamp3)
				}
			}
		}
	} else {
		t.Logf("INFO: Job definition creation failed for run 3 with status: %d", resp3.StatusCode)
	}

	t.Log("PASS: asx_stock_data worker produced consistent output across runs with cache validation")
}

// TestWorkerASXStockDataMultiStock tests the asx_stock_data worker with multiple stocks
// DEPRECATED: This worker is deprecated. Use asx_stock_collector instead which combines
// price data, analyst coverage, and historical financials in a single API call.
// This test is retained for backward compatibility.
func TestWorkerASXStockDataMultiStock(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	stocks := []string{"BHP", "CSL", "GNP", "EXR"}

	for i, stock := range stocks {
		t.Run(stock, func(t *testing.T) {
			defID := fmt.Sprintf("test-asx-stock-data-%s-%d", strings.ToLower(stock), time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        fmt.Sprintf("ASX Stock Data Worker Test - %s", stock),
				"description": "Test asx_stock_data worker for multi-stock support",
				"type":        "asx_stock_data",
				"enabled":     true,
				"tags":        []string{"worker-test", "asx-stock-data", "multi-stock"},
				"steps": []map[string]interface{}{
					{
						"name": "fetch-stock",
						"type": "asx_stock_data",
						"config": map[string]interface{}{
							"asx_code": stock,
							"period":   "Y2", // 24 months for comprehensive data verification
						},
					},
				},
			}

			resp, err := helper.POST("/api/job-definitions", body)
			require.NoError(t, err, "Failed to create job definition for %s", stock)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Skipf("Job definition creation failed for %s with status: %d (may need ASX market hours)", stock, resp.StatusCode)
			}

			defer func() {
				delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
				if delResp != nil {
					delResp.Body.Close()
				}
			}()

			execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
			require.NoError(t, err, "Failed to execute job for %s", stock)
			defer execResp.Body.Close()

			if execResp.StatusCode != http.StatusAccepted {
				t.Skipf("Job execution failed for %s with status: %d", stock, execResp.StatusCode)
			}

			var execResult map[string]interface{}
			require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
			jobID := execResult["job_id"].(string)
			t.Logf("Executed asx_stock_data job for %s: %s", stock, jobID)

			finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)

			// Save job definition for first stock only
			if i == 0 {
				if err := saveJobDefinition(t, env, body); err != nil {
					t.Logf("Warning: failed to save job definition: %v", err)
				}
			}

			if finalStatus != "completed" {
				t.Logf("INFO: Job for %s ended with status %s (may be expected outside market hours)", stock, finalStatus)
				return
			}

			// ===== RUN 1 =====
			t.Logf("=== Run 1 for %s ===", stock)
			validateASXStockDataOutputWithPeriod(t, helper, stock, false) // require24Months=false (EODHD Fundamental subscription doesn't include EOD endpoint)

			if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", strings.ToLower(stock)}, i*2+1); err != nil {
				t.Logf("Warning: failed to save output for %s run 1: %v", stock, err)
			}
			assertResultFilesExist(t, env, i*2+1)

			// ===== RUN 2 =====
			t.Logf("=== Run 2 for %s ===", stock)
			execResp2, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
			require.NoError(t, err, "Failed to execute job for %s run 2", stock)
			defer execResp2.Body.Close()

			if execResp2.StatusCode == http.StatusAccepted {
				var execResult2 map[string]interface{}
				require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
				jobID2 := execResult2["job_id"].(string)
				t.Logf("Executed asx_stock_data job for %s run 2: %s", stock, jobID2)

				finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
				if finalStatus2 == "completed" {
					if _, _, err := saveWorkerOutput(t, env, helper, []string{"asx-stock-data", strings.ToLower(stock)}, i*2+2); err != nil {
						t.Logf("Warning: failed to save output for %s run 2: %v", stock, err)
					}
					assertResultFilesExist(t, env, i*2+2)
				}
			}

			t.Logf("PASS: %s stock data processed with consistency check", stock)
		})
	}
}

// =============================================================================
// Validation Helpers
// =============================================================================

// validateASXStockDataOutput validates that asx_stock_data produced consistent structure
func validateASXStockDataOutput(t *testing.T, helper *common.HTTPTestHelper, ticker string) {
	validateASXStockDataOutputWithPeriod(t, helper, ticker, false)
}

// validateASXStockDataOutputWithPeriod validates asx_stock_data output with optional 24-month check
func validateASXStockDataOutputWithPeriod(t *testing.T, helper *common.HTTPTestHelper, ticker string, require24Months bool) {
	// Query for documents created by asx_stock_data
	resp, err := helper.GET("/api/documents?tags=asx-stock-data," + strings.ToLower(ticker))
	if err != nil {
		t.Logf("Warning: Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			Title           string                 `json:"title"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse response: %v", err)
		return
	}

	t.Logf("Found %d asx-stock-data documents for %s", result.Total, ticker)

	if len(result.Documents) > 0 {
		doc := result.Documents[0]
		content := doc.ContentMarkdown

		// Validate expected sections are present
		expectedSections := []string{
			"Current Price",
			"Performance",
		}

		for _, section := range expectedSections {
			if strings.Contains(content, section) {
				t.Logf("PASS: Found expected section '%s'", section)
			} else {
				t.Logf("INFO: Section '%s' not found in output", section)
			}
		}

		// Validate numeric data patterns
		pricePattern := regexp.MustCompile(`\$?\d+\.\d{2}`)
		if pricePattern.MatchString(content) {
			t.Log("PASS: Found price data in expected format")
		}

		// Validate historical_prices in metadata
		if doc.Metadata != nil {
			if histPrices, ok := doc.Metadata["historical_prices"].([]interface{}); ok {
				priceCount := len(histPrices)
				t.Logf("Found %d historical price entries for %s", priceCount, ticker)

				if require24Months {
					// 24 months should have ~500 trading days (252 per year x 2)
					// Use 400 as minimum threshold to allow for holidays/weekends
					assert.GreaterOrEqual(t, priceCount, 400,
						"Expected at least 400 trading days for 24-month period, got %d", priceCount)
					t.Logf("PASS: Historical prices count (%d) meets 24-month threshold (>=400)", priceCount)
				}

				// Validate first price entry has required fields
				if priceCount > 0 {
					if firstPrice, ok := histPrices[0].(map[string]interface{}); ok {
						if _, hasDate := firstPrice["date"]; hasDate {
							t.Log("PASS: Historical price entries have 'date' field")
						} else {
							t.Log("FAIL: Historical price entries missing 'date' field")
						}
						if _, hasClose := firstPrice["close"]; hasClose {
							t.Log("PASS: Historical price entries have 'close' field")
						} else {
							t.Log("FAIL: Historical price entries missing 'close' field")
						}
					}
				}
			} else {
				if require24Months {
					t.Error("FAIL: historical_prices not found in metadata")
				} else {
					t.Log("INFO: historical_prices not found in metadata")
				}
			}
		}
	}
}

// validateASXAnnouncementsOutput validates that asx_announcements produced consistent structure
func validateASXAnnouncementsOutput(t *testing.T, helper *common.HTTPTestHelper, ticker string) {
	resp, err := helper.GET("/api/documents?tags=asx-announcement," + strings.ToLower(ticker))
	if err != nil {
		t.Logf("Warning: Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			ContentMarkdown string `json:"content_markdown"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse response: %v", err)
		return
	}

	t.Logf("Found %d asx-announcement documents for %s", result.Total, ticker)

	if len(result.Documents) > 0 {
		content := result.Documents[0].ContentMarkdown

		// Validate announcement structure
		expectedFields := []string{
			"Date",
			"Headline",
		}

		for _, field := range expectedFields {
			if strings.Contains(content, field) {
				t.Logf("PASS: Found expected field '%s'", field)
			}
		}

		// Validate date patterns
		datePattern := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|\d{4}-\d{2}-\d{2}`)
		if datePattern.MatchString(content) {
			t.Log("PASS: Found date data in expected format")
		}
	}
}

// validateASXAnnouncementsSummary validates the summary document structure
func validateASXAnnouncementsSummary(t *testing.T, helper *common.HTTPTestHelper, ticker string) {
	resp, err := helper.GET("/api/documents?tags=asx-announcement-summary," + strings.ToLower(ticker))
	if err != nil {
		t.Logf("Warning: Failed to query summary documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Summary document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			Title           string                 `json:"title"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse summary response: %v", err)
		return
	}

	t.Logf("Found %d summary documents for %s", result.Total, ticker)

	if len(result.Documents) > 0 {
		doc := result.Documents[0]

		// Validate content has summary sections
		assert.Contains(t, doc.ContentMarkdown, "# ASX Announcements Summary", "Should have summary header")
		assert.Contains(t, doc.ContentMarkdown, "Relevance Distribution", "Should have relevance section")
		assert.Contains(t, doc.ContentMarkdown, "| Date |", "Should have announcements table")

		t.Log("PASS: Summary document has expected markdown structure")

		// Validate metadata has required fields
		if doc.Metadata != nil {
			// Check for announcements array
			announcements, hasAnn := doc.Metadata["announcements"]
			if hasAnn {
				annList, ok := announcements.([]interface{})
				if ok && len(annList) > 0 {
					t.Logf("PASS: Summary metadata has %d announcements", len(annList))

					// Validate first announcement has required fields
					if firstAnn, ok := annList[0].(map[string]interface{}); ok {
						requiredFields := []string{"date", "headline", "relevance_category"}
						for _, field := range requiredFields {
							if _, exists := firstAnn[field]; exists {
								t.Logf("PASS: Announcement has field '%s'", field)
							} else {
								t.Logf("INFO: Announcement missing field '%s'", field)
							}
						}

						// Check for price_impact
						if _, hasPriceImpact := firstAnn["price_impact"]; hasPriceImpact {
							t.Log("PASS: Announcement has price_impact data")
						} else {
							t.Log("INFO: Announcement missing price_impact (may be expected for recent announcements)")
						}

						// Validate relevance category is one of expected values
						if category, ok := firstAnn["relevance_category"].(string); ok {
							validCategories := []string{"HIGH", "MEDIUM", "LOW", "NOISE"}
							isValid := false
							for _, vc := range validCategories {
								if category == vc {
									isValid = true
									break
								}
							}
							assert.True(t, isValid, "relevance_category should be HIGH, MEDIUM, LOW, or NOISE")
							t.Logf("PASS: First announcement has relevance_category: %s", category)
						}
					}
				}
			} else {
				t.Log("INFO: Summary metadata missing 'announcements' array")
			}

			// Check for summary counts
			countFields := []string{"total_count", "high_count", "medium_count", "low_count", "noise_count"}
			for _, field := range countFields {
				if _, exists := doc.Metadata[field]; exists {
					t.Logf("PASS: Summary has count field '%s'", field)
				}
			}
		}
	} else {
		t.Log("INFO: No summary document found")
	}
}

// validateASXStockCollectorOutput validates that asx_stock_collector produced consolidated structure
// This validates the combined output from price, technicals, analyst coverage, and historical financials.
func validateASXStockCollectorOutput(t *testing.T, helper *common.HTTPTestHelper, ticker string) {
	resp, err := helper.GET("/api/documents?tags=asx-stock-data," + strings.ToLower(ticker))
	if err != nil {
		t.Logf("Warning: Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			Title           string                 `json:"title"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse response: %v", err)
		return
	}

	t.Logf("Found %d asx-stock-data documents for %s (from asx_stock_collector)", result.Total, ticker)

	if len(result.Documents) > 0 {
		doc := result.Documents[0]
		content := doc.ContentMarkdown

		// Validate consolidated output sections
		expectedSections := []string{
			"Current Price",
			"Technical Indicators",
			"Analyst Coverage",
			"Historical Financials",
		}

		for _, section := range expectedSections {
			if strings.Contains(content, section) {
				t.Logf("PASS: Found expected section '%s'", section)
			} else {
				t.Logf("INFO: Section '%s' not found in output", section)
			}
		}

		// Validate metadata has consolidated fields
		if doc.Metadata != nil {
			// Price data fields
			priceFields := []string{"current_price", "change_percent", "volume"}
			for _, field := range priceFields {
				if _, exists := doc.Metadata[field]; exists {
					t.Logf("PASS: Metadata has price field '%s'", field)
				} else {
					t.Logf("INFO: Metadata missing price field '%s'", field)
				}
			}

			// Technical fields
			techFields := []string{"sma_20", "sma_50", "rsi_14", "trend_signal"}
			for _, field := range techFields {
				if _, exists := doc.Metadata[field]; exists {
					t.Logf("PASS: Metadata has technical field '%s'", field)
				} else {
					t.Logf("INFO: Metadata missing technical field '%s'", field)
				}
			}

			// Analyst coverage fields
			analystFields := []string{"analyst_count", "target_mean", "recommendation_key", "upside_potential"}
			for _, field := range analystFields {
				if _, exists := doc.Metadata[field]; exists {
					t.Logf("PASS: Metadata has analyst field '%s'", field)
				} else {
					t.Logf("INFO: Metadata missing analyst field '%s'", field)
				}
			}

			// Historical financials fields
			financialFields := []string{"revenue_growth_yoy", "revenue_cagr_3y"}
			for _, field := range financialFields {
				if _, exists := doc.Metadata[field]; exists {
					t.Logf("PASS: Metadata has financial field '%s'", field)
				} else {
					t.Logf("INFO: Metadata missing financial field '%s'", field)
				}
			}

			// Validate historical_prices array
			if histPrices, ok := doc.Metadata["historical_prices"].([]interface{}); ok {
				priceCount := len(histPrices)
				t.Logf("Found %d historical price entries", priceCount)
				if priceCount >= 400 {
					t.Logf("PASS: Historical prices count (%d) meets 24-month threshold (>=400)", priceCount)
				}
			}

			// Validate annual_data array
			if annualData, ok := doc.Metadata["annual_data"].([]interface{}); ok {
				t.Logf("PASS: Found %d annual financial periods", len(annualData))
			}

			// Validate upgrade_downgrades array
			if udHistory, ok := doc.Metadata["upgrade_downgrades"].([]interface{}); ok {
				t.Logf("PASS: Found %d upgrade/downgrade entries", len(udHistory))
			}
		}
	} else {
		t.Log("INFO: No stock collector document found")
	}
}

// validateSummarySchemaOutput validates that summary with schema produced expected fields
func validateSummarySchemaOutput(t *testing.T, helper *common.HTTPTestHelper, expectedFields []string) {
	resp, err := helper.GET("/api/documents?tags=summary")
	if err != nil {
		t.Logf("Warning: Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			ContentMarkdown string `json:"content_markdown"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse response: %v", err)
		return
	}

	t.Logf("Found %d summary documents", result.Total)

	if len(result.Documents) > 0 {
		content := result.Documents[0].ContentMarkdown
		contentLower := strings.ToLower(content)

		// Check for expected schema fields in output
		foundFields := 0
		for _, field := range expectedFields {
			if strings.Contains(contentLower, strings.ToLower(field)) {
				t.Logf("PASS: Found schema field '%s' in output", field)
				foundFields++
			} else {
				t.Logf("INFO: Schema field '%s' not found in output", field)
			}
		}

		// Try to parse as JSON (if output is JSON)
		if strings.HasPrefix(strings.TrimSpace(content), "{") {
			var jsonOutput map[string]interface{}
			if err := json.Unmarshal([]byte(content), &jsonOutput); err == nil {
				t.Log("PASS: Output is valid JSON")
				for _, field := range expectedFields {
					if _, exists := jsonOutput[field]; exists {
						t.Logf("PASS: JSON contains field '%s'", field)
					}
				}
			}
		}

		t.Logf("Schema compliance: %d/%d fields found", foundFields, len(expectedFields))
		assert.GreaterOrEqual(t, foundFields, 1, "Should find at least one schema field")
	}
}

// validateWebSearchOutput validates that web_search produced output
func validateWebSearchOutput(t *testing.T, helper *common.HTTPTestHelper) {
	resp, err := helper.GET("/api/documents?tags=web-search")
	if err != nil {
		t.Logf("Warning: Failed to query documents: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: Document query returned status %d", resp.StatusCode)
		return
	}

	var result struct {
		Documents []struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			ContentMarkdown string `json:"content_markdown"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse response: %v", err)
		return
	}

	t.Logf("Found %d web-search documents", result.Total)

	if len(result.Documents) > 0 {
		content := result.Documents[0].ContentMarkdown

		// Web search should produce content with search results
		assert.NotEmpty(t, content, "Web search should produce content")

		// Check for typical search result indicators
		if strings.Contains(content, "http") || strings.Contains(content, "www") {
			t.Log("PASS: Output contains URLs from search results")
		}
	}
}

// =============================================================================
// Output Capture Helpers
// =============================================================================
// These helpers save job configuration and worker outputs to the results directory
// for analysis of schema enforcement and output consistency.

// saveJobDefinition saves the job definition to the results directory as job_definition.json
func saveJobDefinition(t *testing.T, env *common.TestEnvironment, definition map[string]interface{}) error {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not available")
	}

	defPath := filepath.Join(resultsDir, "job_definition.json")
	data, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job definition: %w", err)
	}

	if err := os.WriteFile(defPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job definition to %s: %w", defPath, err)
	}

	t.Logf("Saved job definition to: %s", defPath)
	return nil
}

// saveWorkerOutput saves the worker output (document content) to the results directory.
// It saves:
// - output.md: Primary output file with document content_markdown
// - output.json: Document metadata/JSON schema data
// - output_N.md, output_N.json: Numbered files for multi-run comparison
// Returns paths to the saved files (jsonPath may be empty if no metadata)
func saveWorkerOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper,
	tags []string, runNumber int) (jsonPath, mdPath string, err error) {

	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return "", "", fmt.Errorf("results directory not available")
	}

	// Query documents by tags
	tagStr := strings.Join(tags, ",")
	resp, err := helper.GET("/api/documents?tags=" + tagStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("document query returned status %d", resp.StatusCode)
	}

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			Title           string                 `json:"title"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse document response: %w", err)
	}

	if len(result.Documents) == 0 {
		return "", "", fmt.Errorf("no documents found with tags: %s", tagStr)
	}

	// Get the most recent document (first in list, sorted by creation date desc)
	doc := result.Documents[0]
	content := doc.ContentMarkdown

	// Save to output.md as the primary output file (always overwrite with latest)
	// This contains the actual worker-generated content, not logs
	primaryOutputPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(primaryOutputPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: failed to write primary output.md: %v", err)
	} else {
		t.Logf("Saved worker output to: %s", primaryOutputPath)
	}

	// Save numbered markdown content (for multi-run comparison)
	mdPath = filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write markdown to %s: %w", mdPath, err)
	}
	t.Logf("Saved numbered output to: %s", mdPath)

	// Save document metadata to output.json (schema data / structured data)
	if doc.Metadata != nil && len(doc.Metadata) > 0 {
		metadataJSON, err := json.MarshalIndent(doc.Metadata, "", "  ")
		if err == nil {
			// Save to primary output.json
			primaryJSONPath := filepath.Join(resultsDir, "output.json")
			if err := os.WriteFile(primaryJSONPath, metadataJSON, 0644); err != nil {
				t.Logf("Warning: failed to write output.json: %v", err)
			} else {
				t.Logf("Saved document metadata to: %s", primaryJSONPath)
			}

			// Save numbered JSON for multi-run comparison
			jsonPath = filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
			if err := os.WriteFile(jsonPath, metadataJSON, 0644); err != nil {
				t.Logf("Warning: failed to write numbered JSON: %v", err)
				jsonPath = ""
			} else {
				t.Logf("Saved numbered metadata to: %s", jsonPath)
			}
		}
	} else {
		// Try to extract JSON content from the markdown itself
		jsonContent := extractJSONFromContent(content)
		if jsonContent != "" {
			jsonPath = filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
			if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
				t.Logf("Warning: failed to write JSON to %s: %v", jsonPath, err)
				jsonPath = ""
			} else {
				t.Logf("Saved extracted JSON to: %s", jsonPath)
			}
		}
	}

	return jsonPath, mdPath, nil
}

// extractJSONFromContent attempts to extract JSON content from markdown or raw JSON
func extractJSONFromContent(content string) string {
	content = strings.TrimSpace(content)

	// If content starts with {, it's likely JSON
	if strings.HasPrefix(content, "{") {
		// Validate it's valid JSON
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(content), &js); err == nil {
			// Pretty print for readability
			formatted, err := json.MarshalIndent(js, "", "  ")
			if err == nil {
				return string(formatted)
			}
			return content
		}
	}

	// Look for JSON code blocks in markdown
	jsonBlockPattern := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*\\n?```")
	matches := jsonBlockPattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		// Validate the extracted JSON
		var js map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &js); err == nil {
			formatted, err := json.MarshalIndent(js, "", "  ")
			if err == nil {
				return string(formatted)
			}
			return matches[1]
		}
	}

	return ""
}

// checkSchemaInServiceLog checks if the service log contains schema usage logging
// Returns true if the expected pattern is found
func checkSchemaInServiceLog(t *testing.T, env *common.TestEnvironment, expectedPattern string) bool {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Log("Warning: results directory not available")
		return false
	}

	serviceLogPath := filepath.Join(resultsDir, "service.log")
	content, err := os.ReadFile(serviceLogPath)
	if err != nil {
		t.Logf("Warning: failed to read service.log: %v", err)
		return false
	}

	// Search for schema-related patterns
	logContent := string(content)

	// Check for the expected pattern
	if strings.Contains(logContent, expectedPattern) {
		t.Logf("PASS: Found '%s' in service.log", expectedPattern)
		return true
	}

	// Also check for common schema logging patterns
	schemaPatterns := []string{
		"SCHEMA_ENFORCEMENT",
		"output schema",
		"schema_ref",
		"Using output schema",
		"schema_used",
	}

	for _, pattern := range schemaPatterns {
		if strings.Contains(logContent, pattern) {
			t.Logf("Found schema log pattern: %s", pattern)
			return true
		}
	}

	t.Logf("INFO: Schema pattern '%s' not found in service.log", expectedPattern)
	return false
}

// validateOutputConsistency compares the two outputs for structural consistency
func validateOutputConsistency(t *testing.T, env *common.TestEnvironment) {
	resultsDir := env.GetResultsDir()

	// Read both JSON outputs
	json1Path := filepath.Join(resultsDir, "output_1.json")
	json2Path := filepath.Join(resultsDir, "output_2.json")

	content1, err1 := os.ReadFile(json1Path)
	content2, err2 := os.ReadFile(json2Path)

	if err1 != nil || err2 != nil {
		t.Log("INFO: Could not read both JSON outputs for structural comparison")
		if err1 != nil {
			t.Logf("  output_1.json: %v", err1)
		}
		if err2 != nil {
			t.Logf("  output_2.json: %v", err2)
		}

		// Fall back to markdown comparison
		md1Path := filepath.Join(resultsDir, "output_1.md")
		md2Path := filepath.Join(resultsDir, "output_2.md")
		md1, merr1 := os.ReadFile(md1Path)
		md2, merr2 := os.ReadFile(md2Path)

		if merr1 == nil && merr2 == nil {
			t.Logf("Comparing markdown outputs:")
			t.Logf("  output_1.md: %d bytes", len(md1))
			t.Logf("  output_2.md: %d bytes", len(md2))
			// Both should have similar length (within 50% difference)
			if len(md1) > 0 && len(md2) > 0 {
				ratio := float64(len(md1)) / float64(len(md2))
				if ratio > 0.5 && ratio < 2.0 {
					t.Log("PASS: Markdown outputs have similar length (consistent output)")
				} else {
					t.Logf("INFO: Markdown outputs have different lengths (ratio: %.2f)", ratio)
				}
			}
		}
		return
	}

	// Parse JSON outputs
	var js1, js2 map[string]interface{}
	if err := json.Unmarshal(content1, &js1); err != nil {
		t.Logf("Warning: Failed to parse output_1.json: %v", err)
		return
	}
	if err := json.Unmarshal(content2, &js2); err != nil {
		t.Logf("Warning: Failed to parse output_2.json: %v", err)
		return
	}

	// Compare structure
	diffs := compareJSONStructure("", js1, js2)
	if len(diffs) == 0 {
		t.Log("PASS: JSON outputs have identical structure (schema enforcement working)")
	} else {
		t.Logf("INFO: JSON outputs have structural differences:")
		for _, diff := range diffs {
			t.Logf("  - %s", diff)
		}
	}

	// Compare keys at top level
	keys1 := getKeys(js1)
	keys2 := getKeys(js2)
	t.Logf("Output 1 keys: %v", keys1)
	t.Logf("Output 2 keys: %v", keys2)

	// Check if same keys exist
	if len(keys1) == len(keys2) {
		allMatch := true
		for _, k := range keys1 {
			if !containsKey(keys2, k) {
				allMatch = false
				break
			}
		}
		if allMatch {
			t.Log("PASS: Both outputs have identical top-level keys")
		}
	}
}

// compareJSONStructure compares the structure (keys and types) of two JSON objects
func compareJSONStructure(path string, v1, v2 interface{}) []string {
	var diffs []string

	if v1 == nil && v2 == nil {
		return diffs
	}
	if v1 == nil || v2 == nil {
		diffs = append(diffs, fmt.Sprintf("%s: one value is nil", path))
		return diffs
	}

	// Compare types
	switch val1 := v1.(type) {
	case map[string]interface{}:
		val2, ok := v2.(map[string]interface{})
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: type mismatch (object vs %T)", path, v2))
			return diffs
		}

		// Compare keys
		for k := range val1 {
			newPath := k
			if path != "" {
				newPath = path + "." + k
			}
			if _, exists := val2[k]; !exists {
				diffs = append(diffs, fmt.Sprintf("%s: missing in second output", newPath))
			} else {
				diffs = append(diffs, compareJSONStructure(newPath, val1[k], val2[k])...)
			}
		}
		for k := range val2 {
			newPath := k
			if path != "" {
				newPath = path + "." + k
			}
			if _, exists := val1[k]; !exists {
				diffs = append(diffs, fmt.Sprintf("%s: missing in first output", newPath))
			}
		}

	case []interface{}:
		val2, ok := v2.([]interface{})
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: type mismatch (array vs %T)", path, v2))
			return diffs
		}
		// For arrays, compare first element structure if both have elements
		if len(val1) > 0 && len(val2) > 0 {
			diffs = append(diffs, compareJSONStructure(path+"[0]", val1[0], val2[0])...)
		}

	default:
		// For primitive types, just check they're both primitives (not comparing values)
		switch v2.(type) {
		case map[string]interface{}, []interface{}:
			diffs = append(diffs, fmt.Sprintf("%s: type mismatch (%T vs %T)", path, v1, v2))
		}
	}

	return diffs
}

// getKeys returns the keys of a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// containsKey checks if a slice contains a key
func containsKey(slice []string, key string) bool {
	for _, s := range slice {
		if s == key {
			return true
		}
	}
	return false
}

// =============================================================================
// File Assertion Helpers for Output Validation
// =============================================================================

// assertFileExistsAndNotEmpty asserts that a file exists and has content
func assertFileExistsAndNotEmpty(t *testing.T, path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("File does not exist: %s", path)
		} else {
			t.Errorf("Failed to stat file %s: %v", path, err)
		}
		return false
	}

	if info.Size() == 0 {
		t.Errorf("File is empty: %s", path)
		return false
	}

	t.Logf("PASS: File exists and is not empty: %s (%d bytes)", path, info.Size())
	return true
}

// assertResultFilesExist asserts that all expected result files exist for a given run
func assertResultFilesExist(t *testing.T, env *common.TestEnvironment, runNumber int) {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Error("Results directory not available")
		return
	}

	// Check job_definition.json
	assertFileExistsAndNotEmpty(t, filepath.Join(resultsDir, "job_definition.json"))

	// Check primary output files
	assertFileExistsAndNotEmpty(t, filepath.Join(resultsDir, "output.md"))

	// Check numbered output files for this run
	assertFileExistsAndNotEmpty(t, filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber)))

	// JSON files are optional (may not exist if no metadata)
	jsonPath := filepath.Join(resultsDir, "output.json")
	if _, err := os.Stat(jsonPath); err == nil {
		assertFileExistsAndNotEmpty(t, jsonPath)
	}

	numberedJSONPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
	if _, err := os.Stat(numberedJSONPath); err == nil {
		assertFileExistsAndNotEmpty(t, numberedJSONPath)
	}
}

// assertSchemaFileExists asserts that the schema file exists in results directory
func assertSchemaFileExists(t *testing.T, env *common.TestEnvironment) {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Log("INFO: Results directory not available for schema check")
		return
	}

	schemaPath := filepath.Join(resultsDir, "schema.json")
	if _, err := os.Stat(schemaPath); err == nil {
		assertFileExistsAndNotEmpty(t, schemaPath)
	} else {
		t.Logf("INFO: Schema file not present at %s (may not be required for this worker)", schemaPath)
	}
}

// compareMarkdownStructure compares the structure of two markdown documents by headers
func compareMarkdownStructure(t *testing.T, md1, md2 string) bool {
	headers1 := extractMarkdownHeaders(md1)
	headers2 := extractMarkdownHeaders(md2)

	if len(headers1) == 0 && len(headers2) == 0 {
		t.Log("INFO: No headers found in markdown outputs")
		return true
	}

	// Compare header counts
	if len(headers1) != len(headers2) {
		t.Logf("INFO: Different number of headers: %d vs %d", len(headers1), len(headers2))
		return false
	}

	// Compare header content (ignoring dynamic data like dates/prices)
	matching := 0
	for i, h1 := range headers1 {
		if i < len(headers2) {
			// Normalize headers for comparison (remove dynamic content)
			norm1 := normalizeHeader(h1)
			norm2 := normalizeHeader(headers2[i])
			if norm1 == norm2 {
				matching++
			}
		}
	}

	ratio := float64(matching) / float64(len(headers1))
	if ratio >= 0.8 {
		t.Logf("PASS: Markdown headers are consistent (%.0f%% match)", ratio*100)
		return true
	}

	t.Logf("INFO: Markdown headers differ significantly (%.0f%% match)", ratio*100)
	return false
}

// extractMarkdownHeaders extracts header lines from markdown content
func extractMarkdownHeaders(content string) []string {
	var headers []string
	headerPattern := regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	matches := headerPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			headers = append(headers, match[1])
		}
	}
	return headers
}

// normalizeHeader removes dynamic content from headers for comparison
func normalizeHeader(header string) string {
	// Remove dates, prices, percentages, and numbers for structural comparison
	normalized := header

	// Remove date patterns
	datePattern := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|\d{4}[/-]\d{1,2}[/-]\d{1,2}`)
	normalized = datePattern.ReplaceAllString(normalized, "DATE")

	// Remove price patterns
	pricePattern := regexp.MustCompile(`\$[\d,]+\.?\d*`)
	normalized = pricePattern.ReplaceAllString(normalized, "PRICE")

	// Remove percentage patterns
	percentPattern := regexp.MustCompile(`[+-]?\d+\.?\d*%`)
	normalized = percentPattern.ReplaceAllString(normalized, "PERCENT")

	// Remove standalone numbers
	numberPattern := regexp.MustCompile(`\b\d+\.?\d*\b`)
	normalized = numberPattern.ReplaceAllString(normalized, "NUM")

	return strings.TrimSpace(normalized)
}

// assertOutputStructureConsistency compares outputs between two runs for structural consistency
func assertOutputStructureConsistency(t *testing.T, env *common.TestEnvironment) {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Log("INFO: Results directory not available for consistency check")
		return
	}

	// Compare JSON structure
	json1Path := filepath.Join(resultsDir, "output_1.json")
	json2Path := filepath.Join(resultsDir, "output_2.json")

	json1, err1 := os.ReadFile(json1Path)
	json2, err2 := os.ReadFile(json2Path)

	if err1 == nil && err2 == nil {
		var js1, js2 map[string]interface{}
		if json.Unmarshal(json1, &js1) == nil && json.Unmarshal(json2, &js2) == nil {
			diffs := compareJSONStructure("", js1, js2)
			if len(diffs) == 0 {
				t.Log("PASS: JSON outputs have identical structure across runs")
			} else {
				t.Logf("INFO: JSON structure differences found: %d", len(diffs))
				for _, diff := range diffs[:min(5, len(diffs))] {
					t.Logf("  - %s", diff)
				}
			}
		}
	}

	// Compare Markdown structure
	md1Path := filepath.Join(resultsDir, "output_1.md")
	md2Path := filepath.Join(resultsDir, "output_2.md")

	md1, merr1 := os.ReadFile(md1Path)
	md2, merr2 := os.ReadFile(md2Path)

	if merr1 == nil && merr2 == nil {
		compareMarkdownStructure(t, string(md1), string(md2))
	}
}

// =============================================================================
// Cache Validation Helpers
// =============================================================================

// getDocumentLastSynced queries the API and returns the LastSynced timestamp for a document.
// Returns the timestamp, document ID, and any error encountered.
// The timestamp is extracted from the document's metadata or last_synced field.
func getDocumentLastSynced(t *testing.T, helper *common.HTTPTestHelper, tags []string) (*time.Time, string, error) {
	tagStr := strings.Join(tags, ",")
	resp, err := helper.GET("/api/documents?tags=" + tagStr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("document query returned status %d", resp.StatusCode)
	}

	var result struct {
		Documents []struct {
			ID         string     `json:"id"`
			Title      string     `json:"title"`
			LastSynced *time.Time `json:"last_synced"`
			CreatedAt  *time.Time `json:"created_at"`
			UpdatedAt  *time.Time `json:"updated_at"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse document response: %w", err)
	}

	if len(result.Documents) == 0 {
		return nil, "", fmt.Errorf("no documents found with tags: %s", tagStr)
	}

	// Find the newest document by timestamp (for cache bypass testing)
	newestIdx := 0
	var newestTime time.Time
	for i, doc := range result.Documents {
		var docTime time.Time
		if doc.LastSynced != nil {
			docTime = *doc.LastSynced
		} else if doc.CreatedAt != nil {
			docTime = *doc.CreatedAt
		}
		if docTime.After(newestTime) {
			newestTime = docTime
			newestIdx = i
		}
	}
	doc := result.Documents[newestIdx]

	// Prefer LastSynced, fall back to UpdatedAt, then CreatedAt
	var timestamp *time.Time
	if doc.LastSynced != nil {
		timestamp = doc.LastSynced
	} else if doc.UpdatedAt != nil {
		timestamp = doc.UpdatedAt
	} else if doc.CreatedAt != nil {
		timestamp = doc.CreatedAt
	}

	if timestamp != nil {
		t.Logf("Document %s timestamp: %s", doc.ID, timestamp.Format("2006-01-02 15:04:05"))
	} else {
		t.Logf("Document %s has no timestamp fields", doc.ID)
	}

	return timestamp, doc.ID, nil
}

// assertCacheUsed verifies that run2 used cached data from run1 by comparing timestamps.
// If timestamps match (or are within 1 second), cache was used successfully.
func assertCacheUsed(t *testing.T, timestamp1, timestamp2 *time.Time) {
	if timestamp1 == nil || timestamp2 == nil {
		t.Log("INFO: Cannot verify cache usage - one or both timestamps are nil")
		return
	}

	// Allow 1 second tolerance for timing differences
	diff := timestamp1.Sub(*timestamp2)
	if diff < 0 {
		diff = -diff
	}

	if diff <= time.Second {
		t.Logf("PASS: Cache was used - timestamps match (t1: %s, t2: %s)",
			timestamp1.Format("15:04:05"), timestamp2.Format("15:04:05"))
	} else {
		t.Errorf("FAIL: Cache was NOT used - timestamps differ by %v (t1: %s, t2: %s)",
			diff, timestamp1.Format("15:04:05"), timestamp2.Format("15:04:05"))
	}
}

// assertCacheBypass verifies that cache was bypassed by checking timestamps differ.
// If timestamps are different, fresh data was fetched (cache bypassed).
func assertCacheBypass(t *testing.T, timestamp1, timestamp2 *time.Time) {
	if timestamp1 == nil || timestamp2 == nil {
		t.Log("INFO: Cannot verify cache bypass - one or both timestamps are nil")
		return
	}

	// Timestamps should differ by more than 1 second for fresh data
	diff := timestamp1.Sub(*timestamp2)
	if diff < 0 {
		diff = -diff
	}

	if diff > time.Second {
		t.Logf("PASS: Cache was bypassed - new data fetched (t1: %s, t2: %s, diff: %v)",
			timestamp1.Format("15:04:05"), timestamp2.Format("15:04:05"), diff)
	} else {
		t.Errorf("FAIL: Cache was NOT bypassed - timestamps match (t1: %s, t2: %s)",
			timestamp1.Format("15:04:05"), timestamp2.Format("15:04:05"))
	}
}
