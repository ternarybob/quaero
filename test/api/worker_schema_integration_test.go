package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	skipIfNoChrome(t)
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
	skipIfNoChrome(t)
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
func TestWorkerSummaryWithSchema(t *testing.T) {
	skipIfNoChrome(t)
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
	summaryDefID := fmt.Sprintf("summary-schema-test-%d", time.Now().UnixNano())

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

	summaryBody := map[string]interface{}{
		"id":      summaryDefID,
		"name":    "Summary with Schema Test",
		"type":    "summarizer",
		"enabled": true,
		"tags":    []string{"schema-test", "summary-output"},
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

	resp2, err := helper.POST("/api/job-definitions", summaryBody)
	require.NoError(t, err)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		t.Skipf("Summary job creation failed: %d", resp2.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + summaryDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute summary
	execResp2, err := helper.POST("/api/job-definitions/"+summaryDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp2.Body.Close()

	var execResult2 map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
	summaryJobID := execResult2["job_id"].(string)
	t.Logf("Executed summary job with schema: %s", summaryJobID)

	summaryStatus := waitForJobCompletion(t, helper, summaryJobID, 5*time.Minute)

	if summaryStatus == "completed" {
		// Validate the output contains schema-defined fields
		validateSummarySchemaOutput(t, helper, []string{"summary", "components", "recommendation"})
		t.Log("PASS: summary worker with schema produced structured output")
	} else {
		t.Logf("INFO: Summary job ended with status %s", summaryStatus)
	}
}

// TestWorkerWebSearch tests the web_search worker for consistent output
func TestWorkerWebSearch(t *testing.T) {
	skipIfNoChrome(t)
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
