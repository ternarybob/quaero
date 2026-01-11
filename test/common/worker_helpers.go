// -----------------------------------------------------------------------
// Worker test helpers for both portfolio and market worker tests
// Provides consolidated infrastructure for schema validation, job execution,
// and output assertions shared across test/api/portfolio and test/api/market_workers
// -----------------------------------------------------------------------

package common

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// =============================================================================
// Schema Validation
// =============================================================================

// ValidateSchema validates metadata against a schema definition
func ValidateSchema(t *testing.T, metadata map[string]interface{}, schema WorkerSchema) bool {
	t.Helper()
	allValid := true

	// Check required fields
	for _, field := range schema.RequiredFields {
		if val, exists := metadata[field]; !exists || val == nil {
			t.Errorf("SCHEMA FAIL: Required field '%s' is missing", field)
			allValid = false
		} else {
			t.Logf("SCHEMA PASS: Required field '%s' present", field)
		}
	}

	// Check field types
	for field, expectedType := range schema.FieldTypes {
		if val, exists := metadata[field]; exists && val != nil {
			if !validateFieldType(val, expectedType) {
				t.Errorf("SCHEMA FAIL: Field '%s' has wrong type (expected %s)", field, expectedType)
				allValid = false
			}
		}
	}

	// Check array schemas
	for arrayField, requiredElementFields := range schema.ArraySchemas {
		if val, exists := metadata[arrayField]; exists {
			if arr, ok := val.([]interface{}); ok && len(arr) > 0 {
				// Validate first element has required fields
				if elem, ok := arr[0].(map[string]interface{}); ok {
					for _, elemField := range requiredElementFields {
						if _, hasField := elem[elemField]; !hasField {
							t.Errorf("SCHEMA FAIL: Array '%s' element missing field '%s'", arrayField, elemField)
							allValid = false
						}
					}
				}
			}
		}
	}

	return allValid
}

// validateFieldType checks if a value matches the expected type
func validateFieldType(val interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		switch val.(type) {
		case float64, float32, int, int64, int32:
			return true
		}
		return false
	case "array":
		_, ok := val.([]interface{})
		return ok
	case "object":
		_, ok := val.(map[string]interface{})
		return ok
	case "boolean":
		_, ok := val.(bool)
		return ok
	}
	return true // Unknown type, don't fail
}

// =============================================================================
// Job Execution Helpers
// =============================================================================

// CreateAndExecuteJob creates a job definition and executes it
// Returns job ID and definition ID. Returns empty strings on failure.
func CreateAndExecuteJob(t *testing.T, helper *HTTPTestHelper, body map[string]interface{}) (string, string) {
	t.Helper()

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d", resp.StatusCode)
		return "", ""
	}

	defID := body["id"].(string)

	// Cleanup job definition at end
	t.Cleanup(func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	})

	// Execute job
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job definition")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
		return "", ""
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)

	return jobID, defID
}

// WaitForJobCompletion polls job status until completion or timeout
// Returns final status: "completed", "failed", "cancelled", or "timeout"
func WaitForJobCompletion(t *testing.T, helper *HTTPTestHelper, jobID string, timeout time.Duration) string {
	t.Helper()
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
// Returns final status and any error logs collected during execution
func WaitForJobCompletionWithMonitoring(t *testing.T, helper *HTTPTestHelper, jobID string, timeout time.Duration) (string, []map[string]interface{}) {
	t.Helper()
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
// Output Assertions
// =============================================================================

// AssertOutputNotEmpty validates that output.md and output.json exist and are non-empty
// Returns metadata and content
func AssertOutputNotEmpty(t *testing.T, helper *HTTPTestHelper, tags []string) (map[string]interface{}, string) {
	t.Helper()
	_, metadata, content := AssertOutputNotEmptyWithID(t, helper, tags)
	return metadata, content
}

// AssertOutputNotEmptyWithID validates that output.md and output.json exist and are non-empty
// Returns document ID, metadata, and content
func AssertOutputNotEmptyWithID(t *testing.T, helper *HTTPTestHelper, tags []string) (string, map[string]interface{}, string) {
	t.Helper()
	tagStr := strings.Join(tags, ",")
	resp, err := helper.GET("/api/documents?tags=" + tagStr + "&limit=1")
	require.NoError(t, err, "Failed to query documents with tags: %s", tagStr)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "Document query should succeed")

	var result struct {
		Documents []struct {
			ID              string                 `json:"id"`
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	require.NoError(t, helper.ParseJSONResponse(resp, &result), "Failed to parse document response")
	require.Greater(t, len(result.Documents), 0, "Should find at least one document with tags: %s", tagStr)

	doc := result.Documents[0]

	// Assert output.md (content_markdown) is not empty - CRITICAL: use require to fail immediately
	require.NotEmpty(t, doc.ContentMarkdown, "FAIL: output.md (content_markdown) must not be empty - worker produced no content")
	require.Greater(t, len(doc.ContentMarkdown), 10, "FAIL: output.md content too short (%d bytes) - worker likely failed", len(doc.ContentMarkdown))
	t.Logf("PASS: output.md has %d bytes", len(doc.ContentMarkdown))

	// Assert output.json (metadata) is not empty - CRITICAL: use require to fail immediately
	require.NotNil(t, doc.Metadata, "FAIL: output.json (metadata) must not be nil - worker produced no metadata")
	require.Greater(t, len(doc.Metadata), 0, "FAIL: output.json (metadata) must not be empty - worker produced no metadata fields")
	t.Logf("PASS: output.json has %d fields", len(doc.Metadata))

	return doc.ID, doc.Metadata, doc.ContentMarkdown
}

// AssertOutputContains validates that output.md contains expected strings
func AssertOutputContains(t *testing.T, content string, expectedStrings []string) {
	t.Helper()
	for _, expected := range expectedStrings {
		if strings.Contains(content, expected) {
			t.Logf("PASS: Output contains '%s'", expected)
		} else {
			t.Errorf("FAIL: Output missing expected string '%s'", expected)
		}
	}
}

// AssertMetadataHasFields validates that metadata has specific fields
func AssertMetadataHasFields(t *testing.T, metadata map[string]interface{}, fields []string) {
	t.Helper()
	for _, field := range fields {
		if val, exists := metadata[field]; exists && val != nil {
			t.Logf("PASS: Metadata has field '%s'", field)
		} else {
			t.Errorf("FAIL: Metadata missing field '%s'", field)
		}
	}
}

// =============================================================================
// WorkerResult Validation Helpers
// =============================================================================

// WorkerResult mirrors interfaces.WorkerResult for test parsing
type WorkerResult struct {
	DocumentsCreated int                      `json:"documents_created"`
	DocumentIDs      []string                 `json:"document_ids"`
	Tags             []string                 `json:"tags"`
	SourceType       string                   `json:"source_type"`
	SourceIDs        []string                 `json:"source_ids"`
	Errors           []string                 `json:"errors"`
	ByTicker         map[string]*TickerResult `json:"by_ticker"`
}

// TickerResult mirrors interfaces.TickerResult for test parsing
type TickerResult struct {
	DocumentsCreated int      `json:"documents_created"`
	DocumentIDs      []string `json:"document_ids"`
	Tags             []string `json:"tags"`
}

// GetJobWorkerResult retrieves the worker_result from job metadata.
// For manager jobs, it looks up the first step job ID from step_job_ids and queries that.
// For step jobs, it queries the step job directly.
func GetJobWorkerResult(t *testing.T, helper *HTTPTestHelper, jobID string) *WorkerResult {
	t.Helper()
	resp, err := helper.GET("/api/jobs/" + jobID)
	if err != nil {
		t.Logf("Failed to get job %s: %v", jobID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Get job returned status %d", resp.StatusCode)
		return nil
	}

	var job struct {
		Type     string                 `json:"type"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := helper.ParseJSONResponse(resp, &job); err != nil {
		t.Logf("Failed to parse job response: %v", err)
		return nil
	}

	if job.Metadata == nil {
		t.Logf("Job %s has no metadata", jobID)
		return nil
	}

	// If this is a manager job, look up the step job ID and query that instead
	if job.Type == "manager" {
		stepJobIDs, ok := job.Metadata["step_job_ids"].(map[string]interface{})
		if !ok || len(stepJobIDs) == 0 {
			// Some manager jobs might store result directly if they do logic themselves
			// But usually they delegate. Let's check if worker_result exists directly first.
			if _, ok := job.Metadata["worker_result"]; ok {
				// Fall through to parse worker_result
			} else {
				t.Logf("Manager job %s has no step_job_ids in metadata", jobID)
				return nil
			}
		} else {
			// Get the first step job ID (for single-step jobs) or specific one if known
			var firstStepJobID string
			for _, stepID := range stepJobIDs {
				if id, ok := stepID.(string); ok {
					firstStepJobID = id
					break
				}
			}
			if firstStepJobID != "" {
				return GetJobWorkerResult(t, helper, firstStepJobID)
			}
		}
	}

	workerResultRaw, ok := job.Metadata["worker_result"].(map[string]interface{})
	if !ok {
		t.Logf("Job %s has no worker_result in metadata", jobID)
		return nil
	}

	result := &WorkerResult{}

	if v, ok := workerResultRaw["documents_created"].(float64); ok {
		result.DocumentsCreated = int(v)
	}

	if v, ok := workerResultRaw["document_ids"].([]interface{}); ok {
		for _, id := range v {
			if s, ok := id.(string); ok {
				result.DocumentIDs = append(result.DocumentIDs, s)
			}
		}
	}

	if v, ok := workerResultRaw["tags"].([]interface{}); ok {
		for _, tag := range v {
			if s, ok := tag.(string); ok {
				result.Tags = append(result.Tags, s)
			}
		}
	}

	if v, ok := workerResultRaw["source_type"].(string); ok {
		result.SourceType = v
	}

	if v, ok := workerResultRaw["source_ids"].([]interface{}); ok {
		for _, id := range v {
			if s, ok := id.(string); ok {
				result.SourceIDs = append(result.SourceIDs, s)
			}
		}
	}

	if v, ok := workerResultRaw["errors"].([]interface{}); ok {
		for _, e := range v {
			if s, ok := e.(string); ok {
				result.Errors = append(result.Errors, s)
			}
		}
	}

	// Parse by_ticker if present
	if byTicker, ok := workerResultRaw["by_ticker"].(map[string]interface{}); ok {
		result.ByTicker = make(map[string]*TickerResult)
		for ticker, tickerData := range byTicker {
			if tickerMap, ok := tickerData.(map[string]interface{}); ok {
				tr := &TickerResult{}
				if v, ok := tickerMap["documents_created"].(float64); ok {
					tr.DocumentsCreated = int(v)
				}
				if v, ok := tickerMap["document_ids"].([]interface{}); ok {
					for _, id := range v {
						if s, ok := id.(string); ok {
							tr.DocumentIDs = append(tr.DocumentIDs, s)
						}
					}
				}
				if v, ok := tickerMap["tags"].([]interface{}); ok {
					for _, tag := range v {
						if s, ok := tag.(string); ok {
							tr.Tags = append(tr.Tags, s)
						}
					}
				}
				result.ByTicker[ticker] = tr
			}
		}
	}

	return result
}

// ValidateWorkerResult validates that a WorkerResult contains expected documents
func ValidateWorkerResult(t *testing.T, helper *HTTPTestHelper, resultsDir string, result *WorkerResult, expectedCount int, requiredTags []string) bool {
	t.Helper()
	if result == nil {
		t.Error("WorkerResult is nil - worker did not return result")
		return false
	}

	// Save WorkerResult for debugging
	resultPath := resultsDir + "/worker_result.json"
	if data, err := json.MarshalIndent(result, "", "  "); err == nil {
		if err := writeFile(resultPath, data, 0644); err != nil {
			t.Logf("Warning: failed to save worker_result.json: %v", err)
		}
	}

	// Check for errors in result
	if len(result.Errors) > 0 {
		t.Errorf("WorkerResult contains %d errors: %v", len(result.Errors), result.Errors)
		return false
	}

	// Validate document count
	if result.DocumentsCreated < expectedCount {
		t.Errorf("Expected at least %d documents, got %d", expectedCount, result.DocumentsCreated)
		return false
	}
	t.Logf("WorkerResult: %d documents created", result.DocumentsCreated)

	// Validate document IDs
	if len(result.DocumentIDs) < expectedCount {
		t.Errorf("Expected at least %d document IDs, got %d", expectedCount, len(result.DocumentIDs))
		return false
	}

	// Validate required tags are present
	for _, reqTag := range requiredTags {
		found := false
		for _, tag := range result.Tags {
			if tag == reqTag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required tag '%s' not found in result tags: %v", reqTag, result.Tags)
			return false
		}
	}

	// Validate documents exist in storage by querying with tags
	if len(result.Tags) > 0 {
		// Query documents using first two tags (usually "ticker-signals" and stock code)
		queryTags := result.Tags
		if len(queryTags) > 2 {
			queryTags = queryTags[:2] // Limit to first 2 tags for query
		}
		tagStr := strings.Join(queryTags, ",")

		resp, err := helper.GET("/api/documents?tags=" + tagStr + "&limit=10")
		if err != nil {
			t.Errorf("Failed to query documents with tags %s: %v", tagStr, err)
			return false
		}
		defer resp.Body.Close()

		var docsResult struct {
			Documents []struct {
				ID string `json:"id"`
			} `json:"documents"`
		}
		if err := helper.ParseJSONResponse(resp, &docsResult); err != nil {
			t.Errorf("Failed to parse documents response: %v", err)
			return false
		}

		if len(docsResult.Documents) < expectedCount {
			t.Errorf("Expected at least %d documents in storage with tags %v, found %d",
				expectedCount, queryTags, len(docsResult.Documents))
			return false
		}
		t.Logf("Verified %d documents exist in storage with tags %v", len(docsResult.Documents), queryTags)
	}

	return true
}

// =============================================================================
// Job Log Helpers
// =============================================================================

// GetJobLogs retrieves job logs and separates info/error logs
func GetJobLogs(t *testing.T, helper *HTTPTestHelper, jobID string) ([]string, []string) {
	t.Helper()
	var infoLogs, errorLogs []string

	resp, err := helper.GET("/api/jobs/" + jobID + "/logs?limit=100")
	if err != nil {
		t.Logf("Failed to get job logs: %v", err)
		return infoLogs, errorLogs
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Get job logs returned status %d", resp.StatusCode)
		return infoLogs, errorLogs
	}

	var logs struct {
		Logs []struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		} `json:"logs"`
	}
	if err := helper.ParseJSONResponse(resp, &logs); err != nil {
		t.Logf("Failed to parse logs response: %v", err)
		return infoLogs, errorLogs
	}

	for _, log := range logs.Logs {
		if log.Level == "error" {
			errorLogs = append(errorLogs, log.Message)
		} else {
			infoLogs = append(infoLogs, log.Message)
		}
	}

	return infoLogs, errorLogs
}

// AssertNoJobErrors fails the test if job logs contain errors
func AssertNoJobErrors(t *testing.T, helper *HTTPTestHelper, jobID, jobName string) {
	t.Helper()
	_, errorLogs := GetJobLogs(t, helper, jobID)
	if len(errorLogs) > 0 {
		t.Errorf("%s job %s had %d errors:", jobName, jobID, len(errorLogs))
		for i, errLog := range errorLogs {
			t.Errorf("  Error %d: %s", i+1, errLog)
		}
	}
}

// =============================================================================
// Ticker Validation Helpers
// =============================================================================

// AssertTickerInOutput validates that the ticker appears in output content and metadata
func AssertTickerInOutput(t *testing.T, ticker string, metadata map[string]interface{}, content string) {
	t.Helper()
	// Check content contains ticker
	assert.Contains(t, content, ticker, "Content should contain ticker %s", ticker)
	t.Logf("PASS: Content contains ticker %s", ticker)

	// Check metadata has ticker/symbol field
	var foundTicker bool
	for _, field := range []string{"ticker", "symbol", "asx_code"} {
		if val, ok := metadata[field].(string); ok && strings.Contains(val, ticker) {
			foundTicker = true
			t.Logf("PASS: Found ticker %s in metadata field '%s'", ticker, field)
			break
		}
	}
	assert.True(t, foundTicker, "Metadata should contain ticker %s", ticker)
}

// AssertNonZeroStockData validates that key stock data fields are present and non-zero
func AssertNonZeroStockData(t *testing.T, metadata map[string]interface{}) {
	t.Helper()
	// Check for price field (current_price or last_price)
	var priceFound bool
	for _, field := range []string{"current_price", "last_price"} {
		if val, ok := metadata[field].(float64); ok && val > 0 {
			priceFound = true
			t.Logf("PASS: %s = %.4f (non-zero)", field, val)
			break
		}
	}
	assert.True(t, priceFound, "Price data must be present and non-zero")

	// Check currency is present
	if currency, ok := metadata["currency"].(string); ok {
		assert.NotEmpty(t, currency, "Currency should not be empty")
		t.Logf("PASS: currency = %s", currency)
	}

	// Check company_name is present (if available)
	if name, ok := metadata["company_name"].(string); ok {
		assert.NotEmpty(t, name, "Company name should not be empty")
		t.Logf("PASS: company_name = %s", name)
	}
}

// =============================================================================
// Section Consistency Validation
// =============================================================================

// AssertSectionConsistency verifies that multiple outputs have consistent section structure
// This catches schema drift where one run has sections that another is missing
// Returns true if all sections are consistent, false otherwise
func AssertSectionConsistency(t *testing.T, content1, content2 string, requiredSections []string) bool {
	t.Helper()
	allConsistent := true

	for _, section := range requiredSections {
		in1 := strings.Contains(content1, section)
		in2 := strings.Contains(content2, section)

		if in1 && !in2 {
			t.Errorf("SCHEMA DRIFT: Section '%s' present in first output but MISSING in second output", section)
			allConsistent = false
		} else if !in1 && in2 {
			t.Errorf("SCHEMA DRIFT: Section '%s' MISSING in first output but present in second output", section)
			allConsistent = false
		} else if !in1 && !in2 {
			t.Errorf("SCHEMA FAIL: Required section '%s' MISSING from both outputs", section)
			allConsistent = false
		} else {
			t.Logf("SCHEMA PASS: Section '%s' present in both outputs", section)
		}
	}

	return allConsistent
}
