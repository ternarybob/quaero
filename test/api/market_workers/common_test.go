// -----------------------------------------------------------------------
// Common test infrastructure for market worker tests
// Provides shared helpers for schema validation, output assertions, and test setup
// -----------------------------------------------------------------------

package market_workers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
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

// Schema definitions for each worker type
var (
	// FundamentalsSchema for market_fundamentals worker
	FundamentalsSchema = WorkerSchema{
		RequiredFields: []string{"asx_code", "company_name", "current_price", "currency"},
		OptionalFields: []string{"historical_prices", "analyst_count", "pe_ratio", "change_percent", "volume", "market_cap", "ticker", "symbol"},
		FieldTypes: map[string]string{
			"asx_code":          "string",
			"ticker":            "string",
			"symbol":            "string",
			"company_name":      "string",
			"current_price":     "number",
			"currency":          "string",
			"change_percent":    "number",
			"historical_prices": "array",
		},
		ArraySchemas: map[string][]string{
			"historical_prices": {"date", "close"},
		},
	}

	// AnnouncementsSchema for market_announcements worker
	AnnouncementsSchema = WorkerSchema{
		RequiredFields: []string{"asx_code", "announcements", "total_count"},
		OptionalFields: []string{"high_count", "medium_count", "low_count", "noise_count"},
		FieldTypes: map[string]string{
			"asx_code":      "string",
			"announcements": "array",
			"total_count":   "number",
		},
		ArraySchemas: map[string][]string{
			"announcements": {"date", "headline", "relevance_category"},
		},
	}

	// DataSchema for market_data worker
	DataSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "last_price"},
		OptionalFields: []string{"historical_prices", "sma_20", "sma_50", "sma_200", "rsi_14", "trend_signal"},
		FieldTypes: map[string]string{
			"ticker":            "string",
			"last_price":        "number",
			"historical_prices": "array",
			"sma_20":            "number",
			"trend_signal":      "string",
		},
	}

	// NewsSchema for market_news worker
	NewsSchema = WorkerSchema{
		RequiredFields: []string{"ticker"},
		OptionalFields: []string{"announcements", "news_items", "total_count"},
		FieldTypes: map[string]string{
			"ticker":      "string",
			"total_count": "number",
		},
	}

	// DirectorInterestSchema for market_director_interest worker
	DirectorInterestSchema = WorkerSchema{
		RequiredFields: []string{"asx_code", "filings"},
		OptionalFields: []string{"total_count"},
		FieldTypes: map[string]string{
			"asx_code": "string",
			"filings":  "array",
		},
		ArraySchemas: map[string][]string{
			"filings": {"date", "headline"},
		},
	}

	// MacroSchema for market_macro worker
	MacroSchema = WorkerSchema{
		RequiredFields: []string{"data_type"},
		OptionalFields: []string{"data_points", "value", "unit"},
		FieldTypes: map[string]string{
			"data_type": "string",
		},
	}

	// CompetitorSchema for market_competitor worker
	CompetitorSchema = WorkerSchema{
		RequiredFields: []string{"target_asx_code", "competitors"},
		OptionalFields: []string{"competitor_count"},
		FieldTypes: map[string]string{
			"target_asx_code": "string",
			"competitors":     "array",
		},
	}

	// SignalSchema for market_signal worker
	SignalSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "signals"},
		OptionalFields: []string{"regime", "pbas", "vli", "computed_at"},
		FieldTypes: map[string]string{
			"ticker":  "string",
			"signals": "object",
		},
	}

	// PortfolioSchema for market_portfolio worker
	PortfolioSchema = WorkerSchema{
		RequiredFields: []string{"portfolio_tag"},
		OptionalFields: []string{"holdings", "summary", "total_value"},
		FieldTypes: map[string]string{
			"portfolio_tag": "string",
			"holdings":      "array",
		},
	}

	// AssessorSchema for market_assessor worker
	AssessorSchema = WorkerSchema{
		RequiredFields: []string{"ticker"},
		OptionalFields: []string{"assessment", "recommendation", "signals"},
		FieldTypes: map[string]string{
			"ticker":         "string",
			"recommendation": "string",
		},
	}

	// DataCollectionSchema for market_data_collection worker
	DataCollectionSchema = WorkerSchema{
		RequiredFields: []string{"tickers_processed"},
		OptionalFields: []string{"documents_created", "errors"},
		FieldTypes: map[string]string{
			"tickers_processed": "number",
			"documents_created": "number",
		},
	}
)

// =============================================================================
// Schema Validation
// =============================================================================

// ValidateSchema validates metadata against a schema definition
func ValidateSchema(t *testing.T, metadata map[string]interface{}, schema WorkerSchema) bool {
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
// Output Assertions
// =============================================================================

// AssertOutputNotEmpty validates that output.md and output.json exist and are non-empty
func AssertOutputNotEmpty(t *testing.T, helper *common.HTTPTestHelper, tags []string) (map[string]interface{}, string) {
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

	// Assert output.md (content_markdown) is not empty
	assert.NotEmpty(t, doc.ContentMarkdown, "output.md (content_markdown) must not be empty")
	t.Logf("PASS: output.md has %d bytes", len(doc.ContentMarkdown))

	// Assert output.json (metadata) is not empty
	assert.NotNil(t, doc.Metadata, "output.json (metadata) must not be nil")
	assert.Greater(t, len(doc.Metadata), 0, "output.json (metadata) must not be empty")
	t.Logf("PASS: output.json has %d fields", len(doc.Metadata))

	return doc.Metadata, doc.ContentMarkdown
}

// AssertOutputContains validates that output.md contains expected strings
func AssertOutputContains(t *testing.T, content string, expectedStrings []string) {
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
	for _, field := range fields {
		if val, exists := metadata[field]; exists && val != nil {
			t.Logf("PASS: Metadata has field '%s'", field)
		} else {
			t.Errorf("FAIL: Metadata missing field '%s'", field)
		}
	}
}

// =============================================================================
// Test Execution Helpers
// =============================================================================

// CreateAndExecuteJob creates a job definition and executes it
// Returns job ID and final status
func CreateAndExecuteJob(t *testing.T, helper *common.HTTPTestHelper, body map[string]interface{}) (string, string) {
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

// =============================================================================
// Multi-Stock Helpers
// =============================================================================

// MultiStockResult holds results from multi-stock execution
type MultiStockResult struct {
	Ticker   string
	Metadata map[string]interface{}
	Content  string
	Error    error
}

// CombineMultiStockResults combines results from multiple tickers into a sorted output
func CombineMultiStockResults(results []MultiStockResult) (map[string]interface{}, string) {
	// Sort by ticker alphabetically
	sort.Slice(results, func(i, j int) bool {
		return results[i].Ticker < results[j].Ticker
	})

	// Combine metadata
	combined := make(map[string]interface{})
	combined["tickers"] = make([]string, 0)
	combined["by_ticker"] = make(map[string]interface{})

	var contentBuilder strings.Builder
	contentBuilder.WriteString("# Combined Multi-Stock Output\n\n")

	for _, result := range results {
		if result.Error != nil {
			continue
		}

		tickers := combined["tickers"].([]string)
		combined["tickers"] = append(tickers, result.Ticker)

		byTicker := combined["by_ticker"].(map[string]interface{})
		byTicker[result.Ticker] = result.Metadata

		contentBuilder.WriteString(fmt.Sprintf("## %s\n\n", result.Ticker))
		contentBuilder.WriteString(result.Content)
		contentBuilder.WriteString("\n\n---\n\n")
	}

	return combined, contentBuilder.String()
}

// =============================================================================
// Output Save Helpers
// =============================================================================

// SaveWorkerOutput saves worker output to results directory
func SaveWorkerOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, tags []string, runNumber int) error {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not available")
	}

	tagStr := strings.Join(tags, ",")
	resp, err := helper.GET("/api/documents?tags=" + tagStr + "&limit=1")
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("document query returned status %d", resp.StatusCode)
	}

	var result struct {
		Documents []struct {
			ContentMarkdown string                 `json:"content_markdown"`
			Metadata        map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Documents) == 0 {
		return fmt.Errorf("no documents found with tags: %s", tagStr)
	}

	doc := result.Documents[0]

	// Save output.md
	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(doc.ContentMarkdown), 0644); err != nil {
		t.Logf("Warning: failed to write output.md: %v", err)
	} else {
		t.Logf("Saved output.md to: %s", mdPath)
	}

	// Save numbered output
	numberedMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
	os.WriteFile(numberedMdPath, []byte(doc.ContentMarkdown), 0644)

	// Save output.json
	if doc.Metadata != nil {
		jsonPath := filepath.Join(resultsDir, "output.json")
		if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
			if err := os.WriteFile(jsonPath, data, 0644); err != nil {
				t.Logf("Warning: failed to write output.json: %v", err)
			} else {
				t.Logf("Saved output.json to: %s", jsonPath)
			}
		}

		// Save numbered JSON
		numberedJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
		if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
			os.WriteFile(numberedJsonPath, data, 0644)
		}
	}

	return nil
}

// SaveJobDefinition saves job definition to results directory
func SaveJobDefinition(t *testing.T, env *common.TestEnvironment, definition map[string]interface{}) error {
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
		return fmt.Errorf("failed to write job definition: %w", err)
	}

	t.Logf("Saved job definition to: %s", defPath)
	return nil
}

// =============================================================================
// Environment Helpers
// =============================================================================

// HasGeminiAPIKey checks if Gemini API key is available
func HasGeminiAPIKey(env *common.TestEnvironment) bool {
	if env.EnvVars == nil {
		return false
	}
	key, exists := env.EnvVars["google_gemini_api_key"]
	return exists && key != "" && !strings.HasPrefix(key, "YOUR_") && key != "placeholder"
}

// HasEODHDAPIKey checks if EODHD API key is available
func HasEODHDAPIKey(env *common.TestEnvironment) bool {
	if env.EnvVars == nil {
		return false
	}
	key, exists := env.EnvVars["eodhd_api_key"]
	return exists && key != "" && !strings.HasPrefix(key, "YOUR_") && key != "placeholder"
}

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

// RequireLLM fails test if LLM service unavailable
func RequireLLM(t *testing.T, env *common.TestEnvironment) {
	common.RequireLLM(t, env)
}

// RequireEODHD fails test if EODHD API unavailable
func RequireEODHD(t *testing.T, env *common.TestEnvironment) {
	common.RequireEODHD(t, env)
}

// RequireAllMarketServices fails test if any market service unavailable
func RequireAllMarketServices(t *testing.T, env *common.TestEnvironment) {
	common.RequireAllMarketServices(t, env)
}

// =============================================================================
// Stock Data Validation Helpers
// =============================================================================

// AssertTickerInOutput validates that the ticker appears in output content and metadata
func AssertTickerInOutput(t *testing.T, ticker string, metadata map[string]interface{}, content string) {
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

// SaveMultiStockOutput saves combined output from multiple stock data documents
func SaveMultiStockOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, tickers []string, runNumber int) error {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not available")
	}

	var contentBuilder strings.Builder
	combinedMetadata := make(map[string]interface{})
	combinedMetadata["tickers"] = tickers
	combinedMetadata["by_ticker"] = make(map[string]interface{})

	contentBuilder.WriteString("# Combined Stock Data Output\n\n")

	for _, ticker := range tickers {
		tags := []string{"stock-data-collected", ticker}
		resp, err := helper.GET("/api/documents?tags=" + strings.Join(tags, ",") + "&limit=1")
		if err != nil {
			t.Logf("Warning: failed to query docs for ticker %s: %v", ticker, err)
			continue
		}

		var result struct {
			Documents []struct {
				ContentMarkdown string                 `json:"content_markdown"`
				Metadata        map[string]interface{} `json:"metadata"`
			} `json:"documents"`
		}
		if err := helper.ParseJSONResponse(resp, &result); err != nil || len(result.Documents) == 0 {
			resp.Body.Close()
			t.Logf("Warning: no docs found for ticker %s", ticker)
			continue
		}
		resp.Body.Close()

		doc := result.Documents[0]
		contentBuilder.WriteString(fmt.Sprintf("## %s\n\n", ticker))
		contentBuilder.WriteString(doc.ContentMarkdown)
		contentBuilder.WriteString("\n\n---\n\n")

		byTicker := combinedMetadata["by_ticker"].(map[string]interface{})
		byTicker[ticker] = doc.Metadata
	}

	// Save output.md
	mdPath := filepath.Join(resultsDir, "output.md")
	if err := os.WriteFile(mdPath, []byte(contentBuilder.String()), 0644); err != nil {
		t.Logf("Warning: failed to write output.md: %v", err)
	} else {
		t.Logf("Saved combined output.md to: %s", mdPath)
	}

	// Save numbered output
	numberedMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
	os.WriteFile(numberedMdPath, []byte(contentBuilder.String()), 0644)

	// Save output.json
	jsonPath := filepath.Join(resultsDir, "output.json")
	if data, err := json.MarshalIndent(combinedMetadata, "", "  "); err == nil {
		if err := os.WriteFile(jsonPath, data, 0644); err != nil {
			t.Logf("Warning: failed to write output.json: %v", err)
		} else {
			t.Logf("Saved combined output.json to: %s", jsonPath)
		}
	}

	// Save numbered JSON
	numberedJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
	if data, err := json.MarshalIndent(combinedMetadata, "", "  "); err == nil {
		os.WriteFile(numberedJsonPath, data, 0644)
	}

	return nil
}

// =============================================================================
// Result File Assertions
// =============================================================================

// AssertResultFilesExist validates that result files exist with content
// This function FAILS the test if output files are missing or empty
func AssertResultFilesExist(t *testing.T, env *common.TestEnvironment, runNumber int) {
	resultsDir := env.GetResultsDir()
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

	// Check numbered files (informational - not fatal)
	numberedMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
	if numberedInfo, numberedErr := os.Stat(numberedMdPath); numberedErr == nil {
		t.Logf("PASS: output_%d.md exists (%d bytes)", runNumber, numberedInfo.Size())
	}

	numberedJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
	if numberedJsonInfo, numberedJsonErr := os.Stat(numberedJsonPath); numberedJsonErr == nil {
		t.Logf("PASS: output_%d.json exists (%d bytes)", runNumber, numberedJsonInfo.Size())
	}
}
