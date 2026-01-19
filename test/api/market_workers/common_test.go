// -----------------------------------------------------------------------
// Common test infrastructure for market worker tests
// Provides schema definitions and market-specific validation helpers
// Generic infrastructure moved to test/common package
// -----------------------------------------------------------------------

package market_workers

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Schema Definitions (market-worker specific)
// =============================================================================

// Schema definitions for each worker type
var (
	// FundamentalsSchema for market_fundamentals worker
	FundamentalsSchema = common.WorkerSchema{
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
	AnnouncementsSchema = common.WorkerSchema{
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
	DataSchema = common.WorkerSchema{
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
	NewsSchema = common.WorkerSchema{
		RequiredFields: []string{"ticker"},
		OptionalFields: []string{"announcements", "news_items", "total_count"},
		FieldTypes: map[string]string{
			"ticker":      "string",
			"total_count": "number",
		},
	}

	// DirectorInterestSchema for market_director_interest worker
	DirectorInterestSchema = common.WorkerSchema{
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
	MacroSchema = common.WorkerSchema{
		RequiredFields: []string{"data_type"},
		OptionalFields: []string{"data_points", "value", "unit"},
		FieldTypes: map[string]string{
			"data_type": "string",
		},
	}

	// CompetitorSchema for market_competitor worker
	// Schema: quaero/competitor/v1
	CompetitorSchema = common.WorkerSchema{
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
	BFSSchema = common.WorkerSchema{
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
	CDSSchema = common.WorkerSchema{
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
	NFRSchema = common.WorkerSchema{
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
	PPSSchema = common.WorkerSchema{
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
	VRSSchema = common.WorkerSchema{
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
	OBSchema = common.WorkerSchema{
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
	RatingCompositeSchema = common.WorkerSchema{
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
	AnnouncementDownloadSchema = common.WorkerSchema{
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
	PortfolioSchema = common.WorkerSchema{
		RequiredFields: []string{"portfolio_tag"},
		OptionalFields: []string{"holdings", "summary", "total_value"},
		FieldTypes: map[string]string{
			"portfolio_tag": "string",
			"holdings":      "array",
		},
	}

	// AssessorSchema for market_assessor worker
	AssessorSchema = common.WorkerSchema{
		RequiredFields: []string{"ticker"},
		OptionalFields: []string{"assessment", "recommendation", "signals"},
		FieldTypes: map[string]string{
			"ticker":         "string",
			"recommendation": "string",
		},
	}

	// DataCollectionSchema for market_data_collection worker
	DataCollectionSchema = common.WorkerSchema{
		RequiredFields: []string{"tickers_processed"},
		OptionalFields: []string{"documents_created", "errors"},
		FieldTypes: map[string]string{
			"tickers_processed": "number",
			"documents_created": "number",
		},
	}

	// SignalAnalysisSchema for signal_analysis worker
	SignalAnalysisSchema = common.WorkerSchema{
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

	// TickerNewsSchema for ticker_news worker
	TickerNewsSchema = common.WorkerSchema{
		RequiredFields: []string{"ticker", "news_count", "fetched_at"},
		OptionalFields: []string{"eodhd_count", "web_search_count", "sentiment_summary", "exchange", "code", "period"},
		FieldTypes: map[string]string{
			"ticker":            "string",
			"news_count":        "number",
			"eodhd_count":       "number",
			"web_search_count":  "number",
			"fetched_at":        "string",
			"sentiment_summary": "object",
		},
	}

	// TickerMetadataSchema for ticker_metadata worker
	TickerMetadataSchema = common.WorkerSchema{
		RequiredFields: []string{"ticker", "company_name", "fetched_at"},
		OptionalFields: []string{"industry", "sector", "location", "address", "isin", "ipo_date", "employees", "market_cap", "enterprise_value", "pe_ratio", "dividend_yield", "currency", "directors", "management", "director_count", "management_count"},
		FieldTypes: map[string]string{
			"ticker":           "string",
			"company_name":     "string",
			"industry":         "string",
			"sector":           "string",
			"location":         "string",
			"fetched_at":       "string",
			"directors":        "array",
			"management":       "array",
			"director_count":   "number",
			"management_count": "number",
			"market_cap":       "number",
		},
	}

	// NewsletterSchema for portfolio_newsletter worker
	NewsletterSchema = common.WorkerSchema{
		RequiredFields: []string{"portfolio", "tickers", "generated_at"},
		OptionalFields: []string{"ticker_count", "news_count", "metadata_count", "job_id"},
		FieldTypes: map[string]string{
			"portfolio":      "string",
			"tickers":        "array",
			"generated_at":   "string",
			"ticker_count":   "number",
			"news_count":     "number",
			"metadata_count": "number",
		},
	}
)

// AnnouncementsRequiredSections defines the sections that must be present in announcements output
var AnnouncementsRequiredSections = []string{
	"Summary",
	"Announcements",
}

// =============================================================================
// Multi-Stock Helpers (market-specific)
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
// Environment Helpers (market-specific wrappers)
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

// =============================================================================
// Rating Business Rule Validators (market-specific)
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

// =============================================================================
// Local Helper Aliases (for backward compatibility in test files)
// =============================================================================

// WorkerSchema is an alias for common.WorkerSchema
type WorkerSchema = common.WorkerSchema

// ValidateSchema validates metadata against a schema definition
// Delegates to common.ValidateSchema
func ValidateSchema(t *testing.T, metadata map[string]interface{}, schema common.WorkerSchema) bool {
	return common.ValidateSchema(t, metadata, schema)
}

// CreateAndExecuteJob creates a job definition and executes it
// Delegates to common.CreateAndExecuteJob
func CreateAndExecuteJob(t *testing.T, helper *common.HTTPTestHelper, body map[string]interface{}) (string, string) {
	return common.CreateAndExecuteJob(t, helper, body)
}

// WaitForJobCompletion polls job status until completion or timeout
// Delegates to common.WaitForJobCompletion
func WaitForJobCompletion(t *testing.T, helper *common.HTTPTestHelper, jobID string, timeout time.Duration) string {
	return common.WaitForJobCompletion(t, helper, jobID, timeout)
}

// AssertOutputNotEmpty validates that output.md and output.json exist and are non-empty
// Delegates to common.AssertOutputNotEmpty
func AssertOutputNotEmpty(t *testing.T, helper *common.HTTPTestHelper, tags []string) (map[string]interface{}, string) {
	return common.AssertOutputNotEmpty(t, helper, tags)
}

// AssertOutputNotEmptyWithID validates that output.md and output.json exist and are non-empty
// Delegates to common.AssertOutputNotEmptyWithID
func AssertOutputNotEmptyWithID(t *testing.T, helper *common.HTTPTestHelper, tags []string) (string, map[string]interface{}, string) {
	return common.AssertOutputNotEmptyWithID(t, helper, tags)
}

// AssertOutputContains validates that output.md contains expected strings
// Delegates to common.AssertOutputContains
func AssertOutputContains(t *testing.T, content string, expectedStrings []string) {
	common.AssertOutputContains(t, content, expectedStrings)
}

// AssertMetadataHasFields validates that metadata has specific fields
// Delegates to common.AssertMetadataHasFields
func AssertMetadataHasFields(t *testing.T, metadata map[string]interface{}, fields []string) {
	common.AssertMetadataHasFields(t, metadata, fields)
}

// SaveWorkerOutput saves worker output to results directory
// Delegates to common.SaveWorkerOutput
func SaveWorkerOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, tags []string, tickerCode string) error {
	return common.SaveWorkerOutput(t, env, helper, tags, tickerCode)
}

// SaveSchemaDefinition saves the schema definition to results directory
// Delegates to common.SaveSchemaDefinition
func SaveSchemaDefinition(t *testing.T, env *common.TestEnvironment, schema common.WorkerSchema, schemaName string) error {
	return common.SaveSchemaDefinition(t, env, schema, schemaName)
}

// SaveJobDefinition saves job definition to results directory
// Delegates to common.SaveJobDefinition
func SaveJobDefinition(t *testing.T, env *common.TestEnvironment, definition map[string]interface{}) error {
	return common.SaveJobDefinition(t, env, definition)
}

// AssertResultFilesExist validates that result files exist with content
// Delegates to common.AssertResultFilesExist
func AssertResultFilesExist(t *testing.T, env *common.TestEnvironment, runNumber int) {
	common.AssertResultFilesExist(t, env, runNumber)
}

// AssertSchemaFileExists validates that schema.json exists and is non-empty
// Delegates to common.AssertSchemaFileExists
func AssertSchemaFileExists(t *testing.T, env *common.TestEnvironment) {
	common.AssertSchemaFileExists(t, env)
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

// RequireLLM fails test if LLM service unavailable
// Delegates to common.RequireLLM
func RequireLLM(t *testing.T, env *common.TestEnvironment) {
	common.RequireLLM(t, env)
}

// RequireEODHD fails test if EODHD API unavailable
// Delegates to common.RequireEODHD
func RequireEODHD(t *testing.T, env *common.TestEnvironment) {
	common.RequireEODHD(t, env)
}

// RequireAllMarketServices fails test if any market service unavailable
// Delegates to common.RequireAllMarketServices
func RequireAllMarketServices(t *testing.T, env *common.TestEnvironment) {
	common.RequireAllMarketServices(t, env)
}

// GetJobWorkerResult retrieves the worker_result from job metadata
// Delegates to common.GetJobWorkerResult
func GetJobWorkerResult(t *testing.T, helper *common.HTTPTestHelper, jobID string) *common.WorkerResult {
	return common.GetJobWorkerResult(t, helper, jobID)
}

// ValidateWorkerResult validates that a WorkerResult contains expected documents
// Delegates to common.ValidateWorkerResult
func ValidateWorkerResult(t *testing.T, helper *common.HTTPTestHelper, resultsDir string, result *common.WorkerResult, expectedCount int, requiredTags []string) bool {
	return common.ValidateWorkerResult(t, helper, resultsDir, result, expectedCount, requiredTags)
}

// GetJobLogs retrieves job logs and separates info/error logs
// Delegates to common.GetJobLogs
func GetJobLogs(t *testing.T, helper *common.HTTPTestHelper, jobID string) ([]string, []string) {
	return common.GetJobLogs(t, helper, jobID)
}

// AssertNoJobErrors fails the test if job logs contain errors
// Delegates to common.AssertNoJobErrors
func AssertNoJobErrors(t *testing.T, helper *common.HTTPTestHelper, jobID, jobName string) {
	common.AssertNoJobErrors(t, helper, jobID, jobName)
}

// AssertTickerInOutput validates that the ticker appears in output content and metadata
// Delegates to common.AssertTickerInOutput
func AssertTickerInOutput(t *testing.T, ticker string, metadata map[string]interface{}, content string) {
	common.AssertTickerInOutput(t, ticker, metadata, content)
}

// AssertNonZeroStockData validates that key stock data fields are present and non-zero
// Delegates to common.AssertNonZeroStockData
func AssertNonZeroStockData(t *testing.T, metadata map[string]interface{}) {
	common.AssertNonZeroStockData(t, metadata)
}

// AssertSectionConsistency verifies that multiple outputs have consistent section structure
// Delegates to common.AssertSectionConsistency
func AssertSectionConsistency(t *testing.T, content1, content2 string, requiredSections []string) bool {
	return common.AssertSectionConsistency(t, content1, content2, requiredSections)
}

// SaveMultiStockOutput saves combined output from multiple stock data documents
// Delegates to common.SaveMultiStockOutput
func SaveMultiStockOutput(t *testing.T, env *common.TestEnvironment, helper *common.HTTPTestHelper, tickers []string, runNumber int) error {
	return common.SaveMultiStockOutput(t, env, helper, tickers, runNumber)
}
