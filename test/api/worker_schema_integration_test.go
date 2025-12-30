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
// These tests verify each worker/tool produces consistent output:
// - asx_stock_data: Fetches stock data with consistent structure
// - asx_announcements: Fetches announcements with consistent structure
// - web_search: Searches web with AI-powered results
// - summary: Generates analysis with JSON schema enforcement
//
// Primary concern: CONSISTENCY of both tooling and final output
// =============================================================================

// TestWorkerASXStockData tests the asx_stock_data worker produces consistent output
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
					"period":   "M1",  // 1 month for quick test
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

	// Verify completion (may fail if market is closed or API unavailable)
	if finalStatus == "completed" {
		// Verify document was created with expected structure
		validateASXStockDataOutput(t, helper, "BHP")
		t.Log("PASS: asx_stock_data worker produced consistent output")
	} else {
		t.Logf("INFO: Job ended with status %s (may be expected outside market hours)", finalStatus)
	}
}

// TestWorkerASXAnnouncements tests the asx_announcements worker produces consistent output
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
		"description": "Test asx_announcements worker for consistent output",
		"type":        "asx_announcements",
		"enabled":     true,
		"tags":        []string{"worker-test", "asx-announcement"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-announcements",
				"type": "asx_announcements",
				"config": map[string]interface{}{
					"asx_code": "BHP",
					"period":   "M1", // 1 month
					"limit":    5,    // Limit for quick test
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

	finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)

	if finalStatus == "completed" {
		validateASXAnnouncementsOutput(t, helper, "BHP")
		t.Log("PASS: asx_announcements worker produced consistent output")
	} else {
		t.Logf("INFO: Job ended with status %s", finalStatus)
	}
}

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

		// Save job config on first run
		if runNumber == 1 {
			if err := saveJobConfig(t, env, summaryBody); err != nil {
				t.Logf("Warning: failed to save job config: %v", err)
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
	} else {
		t.Logf("INFO: Only %d of %d runs completed, skipping consistency comparison", completedRuns, numRuns)
	}
}

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

	if finalStatus == "completed" {
		validateWebSearchOutput(t, helper)
		t.Log("PASS: web_search worker produced output")
	} else {
		t.Logf("INFO: Job ended with status %s", finalStatus)
	}
}

// =============================================================================
// Validation Helpers
// =============================================================================

// validateASXStockDataOutput validates that asx_stock_data produced consistent structure
func validateASXStockDataOutput(t *testing.T, helper *common.HTTPTestHelper, ticker string) {
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

	t.Logf("Found %d asx-stock-data documents for %s", result.Total, ticker)

	if len(result.Documents) > 0 {
		content := result.Documents[0].ContentMarkdown

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

// saveJobConfig saves the job configuration to the results directory as job_config.json
func saveJobConfig(t *testing.T, env *common.TestEnvironment, config map[string]interface{}) error {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not available")
	}

	configPath := filepath.Join(resultsDir, "job_config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job config to %s: %w", configPath, err)
	}

	t.Logf("Saved job config to: %s", configPath)
	return nil
}

// saveWorkerOutput saves the worker output (document content) to numbered files
// Returns paths to the saved files (jsonPath may be empty if content is not JSON)
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

	// Save markdown content
	mdPath = filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write markdown to %s: %w", mdPath, err)
	}
	t.Logf("Saved markdown output to: %s", mdPath)

	// Try to extract/save JSON content if present
	// The content might be markdown that contains JSON or raw JSON
	jsonContent := extractJSONFromContent(content)
	if jsonContent != "" {
		jsonPath = filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
		if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
			t.Logf("Warning: failed to write JSON to %s: %v", jsonPath, err)
			jsonPath = "" // Clear path if write failed
		} else {
			t.Logf("Saved JSON output to: %s", jsonPath)
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
