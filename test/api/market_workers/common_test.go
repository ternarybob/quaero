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
		OptionalFields: []string{"historical_prices", "analyst_count", "pe_ratio", "change_percent", "volume", "market_cap", "ticker", "symbol", "company_blurb"},
		FieldTypes: map[string]string{
			"asx_code":          "string",
			"ticker":            "string",
			"symbol":            "string",
			"company_name":      "string",
			"company_blurb":     "string",
			"current_price":     "number",
			"currency":          "string",
			"change_percent":    "number",
			"historical_prices": "array",
		},
		ArraySchemas: map[string][]string{
			"historical_prices": {"date", "close"},
		},
	}

	// AnnouncementsSchema for market_announcements worker (with inline classification)
	// Schema: quaero/announcements/v1
	AnnouncementsSchema = WorkerSchema{
		RequiredFields: []string{"$schema", "ticker", "summary", "announcements"},
		OptionalFields: []string{"exchange", "code", "fetched_at", "date_range_start", "date_range_end"},
		FieldTypes: map[string]string{
			"$schema":          "string",
			"ticker":           "string",
			"exchange":         "string",
			"code":             "string",
			"fetched_at":       "string",
			"date_range_start": "string",
			"date_range_end":   "string",
			"summary":          "object",
			"announcements":    "array",
		},
		ArraySchemas: map[string][]string{},
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
	// Schema: quaero/competitor/v1
	CompetitorSchema = WorkerSchema{
		RequiredFields: []string{"$schema", "target_ticker", "target_code", "analyzed_at", "gemini_prompt", "competitors"},
		OptionalFields: []string{"worker_debug"},
		FieldTypes: map[string]string{
			"$schema":       "string",
			"target_ticker": "string",
			"target_code":   "string",
			"analyzed_at":   "string",
			"gemini_prompt": "string",
			"competitors":   "array",
			"worker_debug":  "object",
		},
		ArraySchemas: map[string][]string{
			"competitors": {"code", "rationale"},
		},
	}

	// BFSSchema for rating_bfs worker - Business Foundation Score
	BFSSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "indicator_count", "components", "reasoning"},
		OptionalFields: []string{"calculated_at"},
		FieldTypes: map[string]string{
			"ticker":          "string",
			"score":           "number", // 0, 1, or 2
			"indicator_count": "number",
			"components":      "object",
			"reasoning":       "string",
		},
	}

	// CDSSchema for rating_cds worker - Capital Discipline Score
	CDSSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "components", "reasoning"},
		OptionalFields: []string{"calculated_at", "analysis_period_months"},
		FieldTypes: map[string]string{
			"ticker":     "string",
			"score":      "number", // 0, 1, or 2
			"components": "object",
			"reasoning":  "string",
		},
	}

	// NFRSchema for rating_nfr worker - Narrative-to-Fact Ratio
	NFRSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "components", "reasoning"},
		OptionalFields: []string{"calculated_at"},
		FieldTypes: map[string]string{
			"ticker":     "string",
			"score":      "number", // 0.0 to 1.0
			"components": "object",
			"reasoning":  "string",
		},
	}

	// PPSSchema for rating_pps worker - Price Progression Score
	PPSSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "reasoning"},
		OptionalFields: []string{"calculated_at", "event_details"},
		FieldTypes: map[string]string{
			"ticker":        "string",
			"score":         "number", // 0.0 to 1.0
			"event_details": "array",
			"reasoning":     "string",
		},
	}

	// VRSSchema for rating_vrs worker - Volatility Regime Stability
	VRSSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "components", "reasoning"},
		OptionalFields: []string{"calculated_at"},
		FieldTypes: map[string]string{
			"ticker":     "string",
			"score":      "number", // 0.0 to 1.0
			"components": "object",
			"reasoning":  "string",
		},
	}

	// OBSchema for rating_ob worker - Optionality Bonus
	OBSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "score", "catalyst_found", "timeframe_found", "reasoning"},
		OptionalFields: []string{"calculated_at"},
		FieldTypes: map[string]string{
			"ticker":          "string",
			"score":           "number", // 0.0, 0.5, or 1.0
			"catalyst_found":  "boolean",
			"timeframe_found": "boolean",
			"reasoning":       "string",
		},
	}

	// RatingCompositeSchema for rating_composite worker - Final investability rating
	RatingCompositeSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "label", "gate_passed", "scores"},
		OptionalFields: []string{"calculated_at", "reasoning", "investability"},
		FieldTypes: map[string]string{
			"ticker":        "string",
			"label":         "string", // SPECULATIVE|LOW_ALPHA|WATCHLIST|INVESTABLE|HIGH_CONVICTION
			"investability": "number", // 0-100 or null if gate failed
			"gate_passed":   "boolean",
			"scores":        "object", // All component scores
			"reasoning":     "string",
		},
	}

	// AnnouncementDownloadSchema for market_announcement_download worker
	// Schema: quaero/announcement_download/v1
	AnnouncementDownloadSchema = WorkerSchema{
		RequiredFields: []string{"$schema", "ticker", "fetched_at", "filter_types", "total_matched", "total_downloaded", "total_failed", "announcements"},
		OptionalFields: []string{"source_document_id"},
		FieldTypes: map[string]string{
			"$schema":            "string",
			"ticker":             "string",
			"fetched_at":         "string",
			"filter_types":       "array",
			"total_matched":      "number",
			"total_downloaded":   "number",
			"total_failed":       "number",
			"announcements":      "array",
			"source_document_id": "string",
		},
		ArraySchemas: map[string][]string{
			"announcements": {"date", "headline", "type"},
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

	// SignalAnalysisSchema for signal_analysis worker
	SignalAnalysisSchema = WorkerSchema{
		RequiredFields: []string{"ticker", "analysis_date", "summary", "classifications", "flags", "data_source"},
		OptionalFields: []string{"data_gaps", "period_start", "period_end"},
		FieldTypes: map[string]string{
			"ticker":          "string",
			"analysis_date":   "string",
			"summary":         "object",
			"classifications": "array",
			"flags":           "object",
			"data_source":     "object",
		},
		ArraySchemas: map[string][]string{
			"classifications": {"date", "title", "classification", "metrics"},
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
	_, metadata, content := AssertOutputNotEmptyWithID(t, helper, tags)
	return metadata, content
}

// AssertOutputNotEmptyWithID validates that output.md and output.json exist and are non-empty, returns document ID
func AssertOutputNotEmptyWithID(t *testing.T, helper *common.HTTPTestHelper, tags []string) (string, map[string]interface{}, string) {
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
// tickerCode is used as suffix for output files (e.g., output_BHP.md)
func SaveWorkerOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, tags []string, tickerCode string) error {
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

	// Save ticker-named output (e.g., output_BHP.md)
	tickerMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%s.md", strings.ToUpper(tickerCode)))
	os.WriteFile(tickerMdPath, []byte(doc.ContentMarkdown), 0644)

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

		// Save ticker-named JSON (e.g., output_BHP.json)
		tickerJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%s.json", strings.ToUpper(tickerCode)))
		if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
			os.WriteFile(tickerJsonPath, data, 0644)
		}
	}

	return nil
}

// SaveSchemaDefinition saves the schema definition to results directory
// This allows external verification that output matches the expected schema
func SaveSchemaDefinition(t *testing.T, env *common.TestEnvironment, schema WorkerSchema, schemaName string) error {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		return fmt.Errorf("results directory not available")
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
	if err != nil {
		return fmt.Errorf("failed to marshal schema definition: %w", err)
	}

	if err := os.WriteFile(schemaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema definition: %w", err)
	}

	t.Logf("Saved schema definition to: %s", schemaPath)
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

// AssertSectionConsistency verifies that multiple outputs have consistent section structure
// This catches schema drift where one run has sections that another is missing
// Returns true if all sections are consistent, false otherwise
func AssertSectionConsistency(t *testing.T, content1, content2 string, requiredSections []string) bool {
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

// AnnouncementsRequiredSections defines the sections that must be present in announcements output
var AnnouncementsRequiredSections = []string{
	"Summary",
	"Announcements",
}

// =============================================================================
// Rating Business Rule Validators
// =============================================================================

// ValidGateScores - BFS and CDS must be 0, 1, or 2
var ValidGateScores = []float64{0, 1, 2}

// ValidOBScores - OB must be 0.0, 0.5, or 1.0
var ValidOBScores = []float64{0.0, 0.5, 1.0}

// ValidRatingLabels - Enum values for rating label
var ValidRatingLabels = []string{
	"SPECULATIVE",
	"LOW_ALPHA",
	"WATCHLIST",
	"INVESTABLE",
	"HIGH_CONVICTION",
}

// AssertGateScore validates BFS/CDS score is 0, 1, or 2
func AssertGateScore(t *testing.T, score float64, fieldName string) {
	t.Helper()
	valid := score == 0 || score == 1 || score == 2
	assert.True(t, valid, "%s must be 0, 1, or 2, got %v", fieldName, score)
}

// AssertComponentScore validates NFR/PPS/VRS score is 0.0 to 1.0
func AssertComponentScore(t *testing.T, score float64, fieldName string) {
	t.Helper()
	assert.GreaterOrEqual(t, score, 0.0, "%s must be >= 0.0", fieldName)
	assert.LessOrEqual(t, score, 1.0, "%s must be <= 1.0", fieldName)
}

// AssertOBScore validates OB score is 0.0, 0.5, or 1.0
func AssertOBScore(t *testing.T, score float64) {
	t.Helper()
	valid := score == 0.0 || score == 0.5 || score == 1.0
	assert.True(t, valid, "OB score must be 0.0, 0.5, or 1.0, got %v", score)
}

// AssertRatingLabel validates label is valid enum value
func AssertRatingLabel(t *testing.T, label string) {
	t.Helper()
	valid := false
	for _, v := range ValidRatingLabels {
		if label == v {
			valid = true
			break
		}
	}
	assert.True(t, valid, "Invalid rating label: %s", label)
}

// AssertInvestabilityScore validates investability is 0-100 or nil (if gate failed)
func AssertInvestabilityScore(t *testing.T, score interface{}, gatePassed bool) {
	t.Helper()
	if !gatePassed {
		// Score can be nil or zero when gate fails
		if score == nil {
			return
		}
		if s, ok := score.(float64); ok && s == 0 {
			return
		}
		// Allow nil representation in JSON
		return
	}
	if s, ok := score.(float64); ok {
		assert.GreaterOrEqual(t, s, 0.0, "Investability must be >= 0")
		assert.LessOrEqual(t, s, 100.0, "Investability must be <= 100")
	} else if score != nil {
		t.Errorf("Investability must be a number, got %T", score)
	}
}

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

	// Check schema.json exists (informational - not fatal, as not all tests save schema)
	schemaPath := filepath.Join(resultsDir, "schema.json")
	if schemaInfo, schemaErr := os.Stat(schemaPath); schemaErr == nil {
		t.Logf("PASS: schema.json exists (%d bytes)", schemaInfo.Size())
	}
}

// AssertSchemaFileExists validates that schema.json exists and is non-empty
// Call this after SaveSchemaDefinition to verify the schema was saved
func AssertSchemaFileExists(t *testing.T, env *common.TestEnvironment) {
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Fatal("FAIL: Results directory not available")
		return
	}

	schemaPath := filepath.Join(resultsDir, "schema.json")
	info, err := os.Stat(schemaPath)
	require.NoError(t, err, "FAIL: schema.json must exist at %s", schemaPath)
	require.Greater(t, info.Size(), int64(0), "FAIL: schema.json must not be empty")
	t.Logf("PASS: schema.json exists (%d bytes)", info.Size())
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
func GetJobWorkerResult(t *testing.T, helper *common.HTTPTestHelper, jobID string) *WorkerResult {
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
			// This logic tries to pick a likely candidate.
			var firstStepJobID string
			for _, stepID := range stepJobIDs {
				if id, ok := stepID.(string); ok {
					firstStepJobID = id
					break
				}
			}
			if firstStepJobID != "" {
				// t.Logf("Querying step job %s for worker_result (manager job: %s)", firstStepJobID, jobID)
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
func ValidateWorkerResult(t *testing.T, helper *common.HTTPTestHelper, resultsDir string, result *WorkerResult, expectedCount int, requiredTags []string) bool {
	if result == nil {
		t.Error("WorkerResult is nil - worker did not return result")
		return false
	}

	// Save WorkerResult for debugging
	resultPath := filepath.Join(resultsDir, "worker_result.json")
	if data, err := json.MarshalIndent(result, "", "  "); err == nil {
		if err := os.WriteFile(resultPath, data, 0644); err != nil {
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

// GetJobLogs retrieves job logs and checks for errors
func GetJobLogs(t *testing.T, helper *common.HTTPTestHelper, jobID string) ([]string, []string) {
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
func AssertNoJobErrors(t *testing.T, helper *common.HTTPTestHelper, jobID, jobName string) {
	_, errorLogs := GetJobLogs(t, helper, jobID)
	if len(errorLogs) > 0 {
		t.Errorf("%s job %s had %d errors:", jobName, jobID, len(errorLogs))
		for i, errLog := range errorLogs {
			t.Errorf("  Error %d: %s", i+1, errLog)
		}
	}
}
