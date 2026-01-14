// -----------------------------------------------------------------------
// Common test infrastructure for Portfolio worker tests
// Provides Navexa-specific helpers and schema definitions
// Generic infrastructure moved to test/common package
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
// Schema Definitions (Navexa-specific)
// =============================================================================

// NavexaPortfolioReviewSchema for navexa_portfolio_review worker
var NavexaPortfolioReviewSchema = common.WorkerSchema{
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
var NavexaPortfolioSchema = common.WorkerSchema{
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
// API Key and Configuration Helpers (Navexa-specific)
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
// API Validation Helpers (Navexa-specific)
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

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
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
// Output Save Helpers (Navexa-specific)
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
// Job Execution Helpers (internal)
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

// =============================================================================
// Local Helper Aliases (for backward compatibility in test files)
// =============================================================================

// WaitForJobCompletion polls job status until completion or timeout
// Delegates to common.WaitForJobCompletion
func WaitForJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) string {
	return common.WaitForJobCompletion(t, helper, jobID, timeout)
}

// WaitForJobCompletionWithMonitoring polls job status and collects error logs
// Delegates to common.WaitForJobCompletionWithMonitoring
func WaitForJobCompletionWithMonitoring(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) (string, []map[string]interface{}) {
	return common.WaitForJobCompletionWithMonitoring(t, helper, jobID, timeout)
}

// SaveSchemaDefinition saves the schema definition to results directory
// Delegates to common.SaveSchemaDefinitionToDir
func SaveSchemaDefinition(t *testing.T, resultsDir string, schema common.WorkerSchema, schemaName string) {
	common.SaveSchemaDefinitionToDir(t, resultsDir, schema, schemaName)
}

// SaveJobDefinition saves job definition to results directory
// Delegates to common.SaveJobDefinitionToDir
func SaveJobDefinition(t *testing.T, resultsDir string, definition map[string]interface{}) {
	common.SaveJobDefinitionToDir(t, resultsDir, definition)
}

// AssertResultFilesExist validates that result files exist with content
// Delegates to common.AssertResultFilesExistInDir
func AssertResultFilesExist(t *testing.T, resultsDir string) {
	common.AssertResultFilesExistInDir(t, resultsDir, 0)
}

// SetupFreshEnvironment creates a fresh test environment with clean database
// Delegates to common.SetupFreshEnvironment
func SetupFreshEnvironment(t *testing.T) *common.TestEnvironment {
	return common.SetupFreshEnvironment(t)
}

// AssertNoServiceErrors checks service log for errors
// Delegates to common.AssertNoErrorsInServiceLog
func AssertNoServiceErrors(t *testing.T, env *common.TestEnvironment) {
	common.AssertNoErrorsInServiceLog(t, env)
}

// WriteTestLog writes test progress to test.log file
// Delegates to common.WriteTestLog
func WriteTestLog(t *testing.T, resultsDir string, entries []string) {
	common.WriteTestLog(t, resultsDir, entries)
}
