// -----------------------------------------------------------------------
// Common test infrastructure for Portfolio worker tests
// Provides shared helpers for API validation, output assertions, and test setup
// -----------------------------------------------------------------------

package portfolio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Constants
// =============================================================================

const NavexaDefaultBaseURL = "https://api.navexa.com.au"

// =============================================================================
// Schema Definitions
// =============================================================================

// WorkerSchema defines the expected output schema for a worker
type WorkerSchema struct {
	RequiredFields []string            // Fields that must be present
	OptionalFields []string            // Fields that may be present
	FieldTypes     map[string]string   // Expected types: "string", "number", "array", "object", "boolean"
	ArraySchemas   map[string][]string // For array fields, required fields within each element
}

// NavexaPortfolioReviewSchema for navexa_portfolio_review worker
var NavexaPortfolioReviewSchema = WorkerSchema{
	RequiredFields: []string{"portfolio_id", "portfolio_name"},
	OptionalFields: []string{"holdings_count", "total_value", "review_date", "model_used"},
	FieldTypes: map[string]string{
		"portfolio_id":   "number",
		"portfolio_name": "string",
		"holdings_count": "number",
		"total_value":    "number",
		"review_date":    "string",
		"model_used":     "string",
	},
}

// NavexaPortfolioSchema for navexa_portfolio worker
// Holdings come from performance API with quantity, avgBuyPrice, currentValue, holdingWeight
var NavexaPortfolioSchema = WorkerSchema{
	RequiredFields: []string{"portfolio", "holdings"},
	OptionalFields: []string{"holding_count", "fetched_at"},
	FieldTypes: map[string]string{
		"portfolio":     "object",
		"holdings":      "array",
		"holding_count": "number",
		"fetched_at":    "string",
	},
	ArraySchemas: map[string][]string{
		"holdings": {"symbol", "quantity"},
	},
}

// =============================================================================
// API Key and Configuration Helpers
// =============================================================================

// GetNavexaBaseURL retrieves the Navexa API base URL from the KV store
func GetNavexaBaseURL(t *testing.T, helper *common.HTTPTestHelper) string {
	resp, err := helper.GET("/api/kv/navexa_base_url")
	if err != nil {
		t.Logf("Failed to get Navexa base URL from KV store: %v, using default", err)
		return NavexaDefaultBaseURL
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Navexa base URL not in KV store (status %d), using default", resp.StatusCode)
		return NavexaDefaultBaseURL
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse Navexa base URL response: %v, using default", err)
		return NavexaDefaultBaseURL
	}

	if result.Value == "" {
		return NavexaDefaultBaseURL
	}

	return result.Value
}

// GetNavexaAPIKey retrieves the Navexa API key from the KV store
func GetNavexaAPIKey(t *testing.T, helper *common.HTTPTestHelper) string {
	resp, err := helper.GET("/api/kv/navexa_api_key")
	if err != nil {
		t.Logf("Failed to get Navexa API key from KV store: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Navexa API key not found in KV store (status %d)", resp.StatusCode)
		return ""
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse Navexa API key response: %v", err)
		return ""
	}

	if result.Value == "" || strings.HasPrefix(result.Value, "fake-") {
		t.Log("Navexa API key is placeholder - skipping")
		return ""
	}

	return result.Value
}

// =============================================================================
// API Validation Helpers
// =============================================================================

// FetchAndValidateNavexaAPI makes a direct HTTP call to Navexa API and validates JSON response
func FetchAndValidateNavexaAPI(t *testing.T, resultsDir, baseURL, apiKey string) ([]map[string]interface{}, error) {
	url := baseURL + "/v1/portfolios"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Save raw API response
	apiResponsePath := filepath.Join(resultsDir, "navexa_api_response.json")
	if err := os.WriteFile(apiResponsePath, body, 0644); err != nil {
		t.Logf("Warning: failed to save API response: %v", err)
	} else {
		t.Logf("Saved Navexa API response to: %s", apiResponsePath)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var portfolios []map[string]interface{}
	if err := json.Unmarshal(body, &portfolios); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	t.Logf("Navexa API returned %d portfolios", len(portfolios))
	return portfolios, nil
}

// =============================================================================
// Test Logging Helpers
// =============================================================================

// WriteTestLog writes test progress to test.log file
func WriteTestLog(t *testing.T, resultsDir string, entries []string) {
	logPath := filepath.Join(resultsDir, "test.log")
	content := strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: failed to write test.log: %v", err)
	}
}

// =============================================================================
// Output Save Helpers
// =============================================================================

// SaveNavexaWorkerOutput saves worker document output to results directory
// CRITICAL: This function FAILS the test if document is missing or empty
// Returns metadata and content for further validation
func SaveNavexaWorkerOutput(t *testing.T, helper *common.HTTPTestHelper, resultsDir, tag string) (map[string]interface{}, string) {
	resp, err := helper.GET("/api/documents?tags=" + tag + "&limit=1")
	require.NoError(t, err, "FAIL: Failed to query documents with tag %s", tag)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "FAIL: Document query must succeed")

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}

	require.NoError(t, helper.ParseJSONResponse(resp, &result), "FAIL: Failed to parse document response")
	require.NotEmpty(t, result.Documents, "FAIL: No documents found with tag %s - worker produced no output", tag)

	doc := result.Documents[0]

	// CRITICAL: Verify content is not empty or blank - FAIL test if empty
	content := strings.TrimSpace(doc.ContentMarkdown)
	require.NotEmpty(t, content, "FAIL: output.md (content_markdown) is empty or blank for tag: %s", tag)
	require.Greater(t, len(content), 100, "FAIL: output.md content too short (%d bytes) for tag: %s", len(content), tag)
	t.Logf("PASS: output.md has %d bytes", len(content))

	// CRITICAL: Verify metadata is not nil - FAIL test if nil
	require.NotNil(t, doc.Metadata, "FAIL: output.json (metadata) is nil for tag: %s", tag)
	t.Logf("PASS: output.json has %d fields", len(doc.Metadata))

	// Save output.md - FAIL if write fails
	mdPath := filepath.Join(resultsDir, "output.md")
	err = os.WriteFile(mdPath, []byte(doc.ContentMarkdown), 0644)
	require.NoError(t, err, "FAIL: Failed to write output.md to %s", mdPath)
	t.Logf("Saved output.md to: %s (%d bytes)", mdPath, len(doc.ContentMarkdown))

	// Save output.json - FAIL if write fails
	jsonPath := filepath.Join(resultsDir, "output.json")
	data, err := json.MarshalIndent(doc.Metadata, "", "  ")
	require.NoError(t, err, "FAIL: Failed to marshal metadata to JSON")
	err = os.WriteFile(jsonPath, data, 0644)
	require.NoError(t, err, "FAIL: Failed to write output.json to %s", jsonPath)
	t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))

	return doc.Metadata, doc.ContentMarkdown
}

// SavePortfolioReviewWorkerOutput saves portfolio review document output to results directory
// CRITICAL: This function FAILS the test if document is missing or empty
func SavePortfolioReviewWorkerOutput(t *testing.T, helper *common.HTTPTestHelper, resultsDir string) (map[string]interface{}, string) {
	resp, err := helper.GET("/api/documents?tags=portfolio-review&limit=1")
	require.NoError(t, err, "FAIL: Failed to query documents with tag portfolio-review")
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "FAIL: Document query must succeed")

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}

	require.NoError(t, helper.ParseJSONResponse(resp, &result), "FAIL: Failed to parse document response")
	require.NotEmpty(t, result.Documents, "FAIL: No documents found with tag portfolio-review - worker produced no output")

	doc := result.Documents[0]

	// CRITICAL: Verify content is not empty or blank - FAIL test if empty
	content := strings.TrimSpace(doc.ContentMarkdown)
	require.NotEmpty(t, content, "FAIL: output.md (content_markdown) is empty or blank - worker produced no content")
	require.Greater(t, len(content), 100, "FAIL: output.md content too short (%d bytes) - worker likely failed", len(content))
	t.Logf("PASS: output.md has %d bytes", len(content))

	// CRITICAL: Verify metadata is not nil - FAIL test if nil
	require.NotNil(t, doc.Metadata, "FAIL: output.json (metadata) is nil - worker produced no metadata")
	t.Logf("PASS: output.json has %d fields", len(doc.Metadata))

	// Save output.md - FAIL if write fails
	mdPath := filepath.Join(resultsDir, "output.md")
	err = os.WriteFile(mdPath, []byte(doc.ContentMarkdown), 0644)
	require.NoError(t, err, "FAIL: Failed to write output.md to %s", mdPath)
	t.Logf("Saved output.md to: %s (%d bytes)", mdPath, len(doc.ContentMarkdown))

	// Save output.json - FAIL if write fails
	jsonPath := filepath.Join(resultsDir, "output.json")
	data, err := json.MarshalIndent(doc.Metadata, "", "  ")
	require.NoError(t, err, "FAIL: Failed to marshal metadata to JSON")
	err = os.WriteFile(jsonPath, data, 0644)
	require.NoError(t, err, "FAIL: Failed to write output.json to %s", jsonPath)
	t.Logf("Saved output.json to: %s (%d bytes)", jsonPath, len(data))

	return doc.Metadata, doc.ContentMarkdown
}

// =============================================================================
// Job Execution Helpers
// =============================================================================

// WaitForJobCompletion polls job status until completion or timeout
func WaitForJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	pollInterval := 1 * time.Second

	for time.Now().Before(deadline) {
		resp, err := helper.GET("/api/jobs/" + jobID)
		if err != nil {
			t.Logf("Warning: Failed to get job status: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		var job struct {
			Status string `json:"status"`
		}
		if err := helper.ParseJSONResponse(resp, &job); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		// Check for terminal states
		switch job.Status {
		case "completed", "failed", "cancelled":
			t.Logf("Job %s reached terminal state: %s", jobID, job.Status)
			return job.Status
		}

		time.Sleep(pollInterval)
	}

	t.Logf("Job %s timed out after %v", jobID, timeout)
	return "timeout"
}

// WaitForJobCompletionWithMonitoring polls job status and collects error logs
func WaitForJobCompletionWithMonitoring(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) (string, []map[string]interface{}) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second
	var errorLogs []map[string]interface{}

	for time.Now().Before(deadline) {
		resp, err := helper.GET("/api/jobs/" + jobID)
		if err != nil {
			t.Logf("Warning: Failed to get job status: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		var job struct {
			Status string `json:"status"`
		}
		if err := helper.ParseJSONResponse(resp, &job); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		// Check job logs for errors
		logsResp, err := helper.GET("/api/jobs/" + jobID + "/logs?level=error&limit=50")
		if err == nil {
			var logsResult struct {
				Logs []map[string]interface{} `json:"logs"`
			}
			if helper.ParseJSONResponse(logsResp, &logsResult) == nil {
				errorLogs = logsResult.Logs
			}
			logsResp.Body.Close()
		}

		// Check for terminal states
		switch job.Status {
		case "completed", "failed", "cancelled":
			t.Logf("Job %s reached terminal state: %s", jobID, job.Status)
			return job.Status, errorLogs
		}

		time.Sleep(pollInterval)
	}

	t.Logf("Job %s timed out after %v", jobID, timeout)
	return "timeout", errorLogs
}

// =============================================================================
// Environment Helpers
// =============================================================================

// SetupFreshEnvironment creates a fresh test environment with clean database
func SetupFreshEnvironment(t *testing.T) *common.TestEnvironment {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
		return nil
	}
	return env
}

// AssertNoServiceErrors checks service log for errors
func AssertNoServiceErrors(t *testing.T, env *common.TestEnvironment) {
	common.AssertNoErrorsInServiceLog(t, env)
}

// =============================================================================
// Result File Assertions
// =============================================================================

// AssertResultFilesExist validates that result files exist with content
// This function FAILS the test if output files are missing or empty
func AssertResultFilesExist(t *testing.T, resultsDir string) {
	if resultsDir == "" {
		t.Fatal("FAIL: Results directory not available - cannot validate output files")
		return
	}

	// Check output.md exists and has content - FATAL if missing or empty
	mdPath := filepath.Join(resultsDir, "output.md")
	info, err := os.Stat(mdPath)
	require.NoError(t, err, "FAIL: output.md must exist at %s", mdPath)
	require.Greater(t, info.Size(), int64(0), "FAIL: output.md must not be empty")
	t.Logf("PASS: output.md exists (%d bytes)", info.Size())

	// Check output.json exists and has content - FATAL if missing or empty
	jsonPath := filepath.Join(resultsDir, "output.json")
	jsonInfo, jsonErr := os.Stat(jsonPath)
	require.NoError(t, jsonErr, "FAIL: output.json must exist at %s", jsonPath)
	require.Greater(t, jsonInfo.Size(), int64(0), "FAIL: output.json must not be empty")
	t.Logf("PASS: output.json exists (%d bytes)", jsonInfo.Size())

	// Check job_definition.json exists (informational)
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if defInfo, defErr := os.Stat(defPath); defErr == nil {
		t.Logf("PASS: job_definition.json exists (%d bytes)", defInfo.Size())
	} else {
		t.Logf("INFO: job_definition.json not found (optional)")
	}

	// Check schema.json exists (informational)
	schemaPath := filepath.Join(resultsDir, "schema.json")
	if schemaInfo, schemaErr := os.Stat(schemaPath); schemaErr == nil {
		t.Logf("PASS: schema.json exists (%d bytes)", schemaInfo.Size())
	} else {
		t.Logf("INFO: schema.json not found (optional)")
	}
}

// SaveSchemaDefinition saves the schema definition to results directory
func SaveSchemaDefinition(t *testing.T, resultsDir string, schema WorkerSchema, schemaName string) {
	if resultsDir == "" {
		t.Logf("Warning: results directory not available for schema save")
		return
	}

	// Convert schema to JSON-serializable format
	schemaDoc := map[string]interface{}{
		"schema_name":     schemaName,
		"required_fields": schema.RequiredFields,
		"optional_fields": schema.OptionalFields,
		"field_types":     schema.FieldTypes,
		"array_schemas":   schema.ArraySchemas,
	}

	schemaPath := filepath.Join(resultsDir, "schema.json")
	data, err := json.MarshalIndent(schemaDoc, "", "  ")
	require.NoError(t, err, "FAIL: Failed to marshal schema definition")

	err = os.WriteFile(schemaPath, data, 0644)
	require.NoError(t, err, "FAIL: Failed to write schema.json to %s", schemaPath)
	t.Logf("Saved schema.json to: %s (%d bytes)", schemaPath, len(data))
}

// SaveJobDefinition saves job definition to results directory
func SaveJobDefinition(t *testing.T, resultsDir string, definition map[string]interface{}) {
	if resultsDir == "" {
		t.Logf("Warning: results directory not available for job definition save")
		return
	}

	defPath := filepath.Join(resultsDir, "job_definition.json")
	data, err := json.MarshalIndent(definition, "", "  ")
	require.NoError(t, err, "FAIL: Failed to marshal job definition")

	err = os.WriteFile(defPath, data, 0644)
	require.NoError(t, err, "FAIL: Failed to write job_definition.json to %s", defPath)
	t.Logf("Saved job_definition.json to: %s (%d bytes)", defPath, len(data))
}

// =============================================================================
// Job Definition and Document Helpers
// =============================================================================

// executeJobDefinition executes a job definition and returns the job ID
func executeJobDefinition(t *testing.T, helper *common.HTTPTestHelper, id string) string {
	resp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", id), nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Logf("Failed to execute job definition: status %d", resp.StatusCode)
		return ""
	}

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse execution response")

	jobID, ok := result["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")

	t.Logf("Executed job definition %s -> job_id=%s", id, jobID)
	return jobID
}

// deleteJob deletes a job
func deleteJob(t *testing.T, helper *common.HTTPTestHelper, jobID string) {
	resp, err := helper.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
	if err != nil {
		t.Logf("Failed to delete job %s: %v", jobID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		t.Logf("Deleted job: id=%s", jobID)
	}
}

// getDocumentsByTag retrieves documents with a specific tag
func getDocumentsByTag(t *testing.T, helper *common.HTTPTestHelper, tag string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/documents?tag=%s", tag))
	require.NoError(t, err, "Failed to query documents")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse documents response")

	documents, ok := result["documents"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var docs []map[string]interface{}
	for _, d := range documents {
		if doc, ok := d.(map[string]interface{}); ok {
			docs = append(docs, doc)
		}
	}

	return docs
}

// getChildJobs retrieves child jobs of a parent job
func getChildJobs(t *testing.T, helper *common.HTTPTestHelper, parentJobID string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/children", parentJobID))
	if err != nil {
		t.Logf("Warning: Failed to get child jobs: %v", err)
		return []map[string]interface{}{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET /api/jobs/%s/children returned %d", parentJobID, resp.StatusCode)
		return []map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse child jobs response: %v", err)
		return []map[string]interface{}{}
	}

	jobs, ok := result["jobs"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var childJobs []map[string]interface{}
	for _, j := range jobs {
		if job, ok := j.(map[string]interface{}); ok {
			childJobs = append(childJobs, job)
		}
	}

	return childJobs
}
