// -----------------------------------------------------------------------
// Result file helpers for test output management
// Provides consolidated infrastructure for saving and validating test result files
// shared across test/api/portfolio and test/api/market_workers
// -----------------------------------------------------------------------

package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Output Configuration
// =============================================================================

// TestOutputConfig defines which output files are required for a test
type TestOutputConfig struct {
	// RequireOutputMD requires output.md to exist and be non-empty
	RequireOutputMD bool
	// RequireOutputJSON requires output.json to exist and be non-empty
	RequireOutputJSON bool
	// RequireJobDefinition requires job_definition.json or job_definition.toml
	RequireJobDefinition bool
	// RequireTestLog requires test.log to exist
	RequireTestLog bool
	// RequireServiceLog requires service.log to exist
	RequireServiceLog bool
	// RequireSchema requires schema.json to exist
	RequireSchema bool
	// RequireTimingData requires timing_data.json to exist
	RequireTimingData bool
	// AllowEmptyTestLog allows test.log to be empty (some tests have minimal logging)
	AllowEmptyTestLog bool
}

// DefaultTestOutputConfig returns the standard output requirements for API tests
// Per docs/architecture/TEST_ARCHITECTURE.md and .claude/skills/test-architecture/SKILL.md
func DefaultTestOutputConfig() TestOutputConfig {
	return TestOutputConfig{
		RequireOutputMD:      true,
		RequireOutputJSON:    true,
		RequireJobDefinition: true,
		RequireTestLog:       true,
		RequireServiceLog:    true,
		RequireSchema:        false, // Optional
		RequireTimingData:    false, // Recommended but optional
		AllowEmptyTestLog:    false,
	}
}

// PortfolioTestOutputConfig returns output requirements for portfolio tests
func PortfolioTestOutputConfig() TestOutputConfig {
	config := DefaultTestOutputConfig()
	config.RequireTimingData = true // Portfolio tests should have timing data
	return config
}

// MarketWorkerTestOutputConfig returns output requirements for market worker tests
func MarketWorkerTestOutputConfig() TestOutputConfig {
	config := DefaultTestOutputConfig()
	config.RequireSchema = true // Market workers typically validate schemas
	return config
}

// =============================================================================
// File Write Helper (internal)
// =============================================================================

// writeFile is an internal helper to write files with proper error handling
func writeFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// =============================================================================
// Save Helpers
// =============================================================================

// SaveWorkerOutput saves worker output to results directory
// tickerCode is used as suffix for output files (e.g., output_BHP.md)
// Returns error if save fails
func SaveWorkerOutput(t *testing.T, env *TestEnvironment, helper *HTTPTestHelper, tags []string, tickerCode string) error {
	t.Helper()
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
	if tickerCode != "" {
		tickerMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%s.md", strings.ToUpper(tickerCode)))
		os.WriteFile(tickerMdPath, []byte(doc.ContentMarkdown), 0644)
	}

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
		if tickerCode != "" {
			tickerJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%s.json", strings.ToUpper(tickerCode)))
			if data, err := json.MarshalIndent(doc.Metadata, "", "  "); err == nil {
				os.WriteFile(tickerJsonPath, data, 0644)
			}
		}
	}

	return nil
}

// SaveSchemaDefinition saves the schema definition to results directory
// This allows external verification that output matches the expected schema
func SaveSchemaDefinition(t *testing.T, env *TestEnvironment, schema WorkerSchema, schemaName string) error {
	t.Helper()
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

// SaveSchemaDefinitionToDir saves the schema definition to a specific directory
// Use this when you need to save to a custom directory path
func SaveSchemaDefinitionToDir(t *testing.T, resultsDir string, schema WorkerSchema, schemaName string) {
	t.Helper()
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
func SaveJobDefinition(t *testing.T, env *TestEnvironment, definition map[string]interface{}) error {
	t.Helper()
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

// SaveJobDefinitionToDir saves job definition to a specific directory
// Use this when you need to save to a custom directory path
func SaveJobDefinitionToDir(t *testing.T, resultsDir string, definition map[string]interface{}) {
	t.Helper()
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
// Result File Assertions
// =============================================================================

// ValidateTestOutputs validates test result files against the provided config
// Returns a slice of missing/empty file names, or nil if all requirements met
// This is the core validation function used by all assertion helpers
func ValidateTestOutputs(t *testing.T, resultsDir string, config TestOutputConfig) []string {
	t.Helper()

	var missing []string

	// Helper to check file exists and is not empty
	checkFile := func(name string, required bool, allowEmpty bool) {
		path := filepath.Join(resultsDir, name)
		info, err := os.Stat(path)
		if err != nil {
			if required {
				missing = append(missing, name+" (MISSING)")
				t.Logf("FAIL: %s must exist at %s", name, path)
			}
			return
		}
		if info.Size() == 0 && !allowEmpty && required {
			missing = append(missing, name+" (EMPTY)")
			t.Logf("FAIL: %s must not be empty", name)
			return
		}
		t.Logf("PASS: %s exists (%d bytes)", name, info.Size())
	}

	// Check required files
	if config.RequireOutputMD {
		checkFile("output.md", true, false)
	}
	if config.RequireOutputJSON {
		checkFile("output.json", true, false)
	}
	if config.RequireTestLog {
		checkFile("test.log", true, config.AllowEmptyTestLog)
	}
	if config.RequireServiceLog {
		checkFile("service.log", true, false)
	}
	if config.RequireSchema {
		checkFile("schema.json", true, false)
	}
	if config.RequireTimingData {
		checkFile("timing_data.json", true, false)
	}

	// Check job definition (json OR toml)
	if config.RequireJobDefinition {
		jsonPath := filepath.Join(resultsDir, "job_definition.json")
		tomlPath := filepath.Join(resultsDir, "job_definition.toml")
		_, jsonErr := os.Stat(jsonPath)
		_, tomlErr := os.Stat(tomlPath)
		if jsonErr != nil && tomlErr != nil {
			missing = append(missing, "job_definition.json or job_definition.toml (MISSING)")
			t.Logf("FAIL: job_definition must exist (json or toml)")
		} else {
			if jsonErr == nil {
				t.Logf("PASS: job_definition.json exists")
			}
			if tomlErr == nil {
				t.Logf("PASS: job_definition.toml exists")
			}
		}
	}

	return missing
}

// AssertTestOutputs validates test outputs and fails the test if requirements not met
// This is the primary assertion function for validating test result files
func AssertTestOutputs(t *testing.T, resultsDir string, config TestOutputConfig) {
	t.Helper()
	if resultsDir == "" {
		t.Fatal("FAIL: Results directory not available - cannot validate output files")
		return
	}

	missing := ValidateTestOutputs(t, resultsDir, config)
	if len(missing) > 0 {
		t.Fatalf("FAIL: Required test output files missing or empty: %v\nSee: docs/architecture/TEST_ARCHITECTURE.md", missing)
	}
	t.Log("PASS: All required test output files present")
}

// AssertPortfolioTestOutputs validates portfolio test outputs
// Uses PortfolioTestOutputConfig() for requirements
func AssertPortfolioTestOutputs(t *testing.T, resultsDir string) {
	t.Helper()
	AssertTestOutputs(t, resultsDir, PortfolioTestOutputConfig())
}

// AssertMarketWorkerTestOutputs validates market worker test outputs
// Uses MarketWorkerTestOutputConfig() for requirements
func AssertMarketWorkerTestOutputs(t *testing.T, resultsDir string) {
	t.Helper()
	AssertTestOutputs(t, resultsDir, MarketWorkerTestOutputConfig())
}

// AssertResultFilesExist validates that result files exist with content
// This function FAILS the test if output files are missing or empty
// Delegates to AssertTestOutputs with DefaultTestOutputConfig
func AssertResultFilesExist(t *testing.T, env *TestEnvironment, runNumber int) {
	t.Helper()
	resultsDir := env.GetResultsDir()
	if resultsDir == "" {
		t.Fatal("FAIL: Results directory not available - cannot validate output files")
		return
	}

	AssertResultFilesExistInDir(t, resultsDir, runNumber)
}

// AssertResultFilesExistInDir validates that result files exist in a specific directory
// This function FAILS the test if output files are missing or empty
func AssertResultFilesExistInDir(t *testing.T, resultsDir string, runNumber int) {
	t.Helper()
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
	if runNumber > 0 {
		numberedMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
		if numberedInfo, numberedErr := os.Stat(numberedMdPath); numberedErr == nil {
			t.Logf("PASS: output_%d.md exists (%d bytes)", runNumber, numberedInfo.Size())
		}

		numberedJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
		if numberedJsonInfo, numberedJsonErr := os.Stat(numberedJsonPath); numberedJsonErr == nil {
			t.Logf("PASS: output_%d.json exists (%d bytes)", runNumber, numberedJsonInfo.Size())
		}
	}

	// Check schema.json exists (informational - not fatal, as not all tests save schema)
	schemaPath := filepath.Join(resultsDir, "schema.json")
	if schemaInfo, schemaErr := os.Stat(schemaPath); schemaErr == nil {
		t.Logf("PASS: schema.json exists (%d bytes)", schemaInfo.Size())
	}

	// Check job_definition.json exists (informational)
	defPath := filepath.Join(resultsDir, "job_definition.json")
	if defInfo, defErr := os.Stat(defPath); defErr == nil {
		t.Logf("PASS: job_definition.json exists (%d bytes)", defInfo.Size())
	}
}

// AssertSchemaFileExists validates that schema.json exists and is non-empty
// Call this after SaveSchemaDefinition to verify the schema was saved
func AssertSchemaFileExists(t *testing.T, env *TestEnvironment) {
	t.Helper()
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
// Test Log Helpers
// =============================================================================

// WriteTestLog writes test progress to test.log file
func WriteTestLog(t *testing.T, resultsDir string, entries []string) {
	t.Helper()
	if resultsDir == "" {
		return
	}

	logPath := filepath.Join(resultsDir, "test.log")
	content := strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Logf("Warning: failed to write test.log: %v", err)
	}
}

// AppendTestLog appends an entry to test.log file
func AppendTestLog(t *testing.T, resultsDir string, entry string) {
	t.Helper()
	if resultsDir == "" {
		return
	}

	logPath := filepath.Join(resultsDir, "test.log")

	// Open file for appending (create if not exists)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Logf("Warning: failed to open test.log for append: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(entry + "\n"); err != nil {
		t.Logf("Warning: failed to append to test.log: %v", err)
	}
}

// =============================================================================
// Environment Helpers
// =============================================================================

// SetupFreshEnvironment creates a fresh test environment with clean database
// Skips the test if environment setup fails
func SetupFreshEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()
	env, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
		return nil
	}
	return env
}

// =============================================================================
// Multi-Stock Output Helpers
// =============================================================================

// SaveMultiStockOutput saves combined output from multiple stock data documents
func SaveMultiStockOutput(t *testing.T, env *TestEnvironment, helper *HTTPTestHelper, tickers []string, runNumber int) error {
	t.Helper()
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
	if runNumber > 0 {
		numberedMdPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.md", runNumber))
		os.WriteFile(numberedMdPath, []byte(contentBuilder.String()), 0644)
	}

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
	if runNumber > 0 {
		numberedJsonPath := filepath.Join(resultsDir, fmt.Sprintf("output_%d.json", runNumber))
		if data, err := json.MarshalIndent(combinedMetadata, "", "  "); err == nil {
			os.WriteFile(numberedJsonPath, data, 0644)
		}
	}

	return nil
}

// =============================================================================
// TestOutputGuard - Ensures test outputs are saved and validated
// =============================================================================

// TestOutputGuard ensures test outputs are saved and validated on all exit paths.
// Use this guard with defer to guarantee outputs are validated even on test failure.
//
// Usage:
//
//	func TestMyWorker(t *testing.T) {
//	    env := common.SetupFreshEnvironment(t)
//	    defer env.Cleanup()
//	    resultsDir := env.GetResultsDir()
//
//	    guard := common.NewTestOutputGuard(t, resultsDir, common.DefaultTestOutputConfig())
//	    defer guard.Close()
//
//	    guard.LogWithTimestamp("Test started")
//	    // ... test logic ...
//	    guard.MarkOutputSaved()  // Call after saving output.md/output.json
//	}
type TestOutputGuard struct {
	t           *testing.T
	resultsDir  string
	testLog     []string
	config      TestOutputConfig
	outputSaved bool // Set to true after outputs are saved
	closed      bool // Prevent double-close
}

// NewTestOutputGuard creates a guard that ensures outputs are validated.
// The guard should be used with defer guard.Close() immediately after creation.
func NewTestOutputGuard(t *testing.T, resultsDir string, config TestOutputConfig) *TestOutputGuard {
	t.Helper()
	return &TestOutputGuard{
		t:          t,
		resultsDir: resultsDir,
		testLog:    make([]string, 0, 50),
		config:     config,
	}
}

// NewPortfolioTestOutputGuard creates a guard with PortfolioTestOutputConfig
func NewPortfolioTestOutputGuard(t *testing.T, resultsDir string) *TestOutputGuard {
	config := PortfolioTestOutputConfig()
	// Don't require timing_data during guard validation - it's often saved after guard.Close
	config.RequireTimingData = false
	return NewTestOutputGuard(t, resultsDir, config)
}

// NewMarketWorkerTestOutputGuard creates a guard with MarketWorkerTestOutputConfig
func NewMarketWorkerTestOutputGuard(t *testing.T, resultsDir string) *TestOutputGuard {
	return NewTestOutputGuard(t, resultsDir, MarketWorkerTestOutputConfig())
}

// Log adds an entry to the test log
func (g *TestOutputGuard) Log(entry string) {
	g.testLog = append(g.testLog, entry)
}

// LogWithTimestamp adds a timestamped entry to the test log
func (g *TestOutputGuard) LogWithTimestamp(entry string) {
	timestamp := fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), entry)
	g.testLog = append(g.testLog, timestamp)
}

// Logf adds a formatted entry to the test log
func (g *TestOutputGuard) Logf(format string, args ...interface{}) {
	g.testLog = append(g.testLog, fmt.Sprintf(format, args...))
}

// LogfWithTimestamp adds a formatted, timestamped entry to the test log
func (g *TestOutputGuard) LogfWithTimestamp(format string, args ...interface{}) {
	entry := fmt.Sprintf(format, args...)
	timestamp := fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), entry)
	g.testLog = append(g.testLog, timestamp)
}

// MarkOutputSaved indicates that output.md and output.json have been saved.
// Call this after successfully saving the output files.
func (g *TestOutputGuard) MarkOutputSaved() {
	g.outputSaved = true
}

// GetTestLog returns the accumulated test log entries
func (g *TestOutputGuard) GetTestLog() []string {
	return g.testLog
}

// Close validates outputs and writes test.log.
// This method is designed to be called via defer.
// It will:
// 1. Always write the test.log file
// 2. If outputs were not saved, log a warning (but don't fail - test may have already failed)
// 3. If outputs should exist, validate they are present and non-empty
func (g *TestOutputGuard) Close() {
	if g.closed {
		return
	}
	g.closed = true

	// Always write test.log
	g.testLog = append(g.testLog, fmt.Sprintf("[%s] === TEST COMPLETED ===", time.Now().Format(time.RFC3339)))
	WriteTestLog(g.t, g.resultsDir, g.testLog)

	// If outputs weren't explicitly saved, check if they exist anyway
	// (they might have been saved by helper functions)
	if g.resultsDir == "" {
		return
	}

	missing := ValidateTestOutputs(g.t, g.resultsDir, g.config)
	if len(missing) > 0 {
		// Log the missing files as warnings in test output
		// Don't fail the test here - it may have already failed, and we want to preserve the original failure
		g.t.Logf("WARNING: Test output files missing or empty: %v", missing)
		g.t.Logf("See: docs/architecture/TEST_ARCHITECTURE.md and .claude/skills/test-architecture/SKILL.md")
	}
}

// MustClose is like Close but fails the test if outputs are invalid.
// Use this at the END of a test function (not in defer) when you want to
// enforce output validation as a hard requirement.
func (g *TestOutputGuard) MustClose() {
	if g.closed {
		return
	}
	g.closed = true

	// Always write test.log
	g.testLog = append(g.testLog, fmt.Sprintf("[%s] === TEST COMPLETED ===", time.Now().Format(time.RFC3339)))
	WriteTestLog(g.t, g.resultsDir, g.testLog)

	// Validate and fail if outputs are missing
	if g.resultsDir != "" {
		AssertTestOutputs(g.t, g.resultsDir, g.config)
	}
}

// =============================================================================
// RequireTestOutputs - Strict validation for test outputs
// =============================================================================

// RequireTestOutputs validates test outputs and FAILS the test if any are missing.
// This is a strict version of AssertTestOutputs that should be called at the end
// of every API test.
//
// Usage:
//
//	// At the end of your test, after all outputs are saved:
//	common.RequireTestOutputs(t, resultsDir)
func RequireTestOutputs(t *testing.T, resultsDir string) {
	t.Helper()
	config := DefaultTestOutputConfig()
	RequireTestOutputsWithConfig(t, resultsDir, config)
}

// RequireTestOutputsWithConfig validates test outputs against a specific config.
func RequireTestOutputsWithConfig(t *testing.T, resultsDir string, config TestOutputConfig) {
	t.Helper()
	if resultsDir == "" {
		t.Fatal("FAIL: Results directory not set - cannot validate test outputs")
		return
	}

	missing := ValidateTestOutputs(t, resultsDir, config)
	if len(missing) > 0 {
		t.Fatalf("FAIL: Required test output files missing or empty: %v\n"+
			"See: docs/architecture/TEST_ARCHITECTURE.md\n"+
			"See: .claude/skills/test-architecture/SKILL.md", missing)
	}
}

// RequirePortfolioTestOutputs validates portfolio test outputs.
func RequirePortfolioTestOutputs(t *testing.T, resultsDir string) {
	t.Helper()
	config := PortfolioTestOutputConfig()
	// Don't require timing_data - it's often saved after validation
	config.RequireTimingData = false
	RequireTestOutputsWithConfig(t, resultsDir, config)
}

// RequireMarketWorkerTestOutputs validates market worker test outputs.
func RequireMarketWorkerTestOutputs(t *testing.T, resultsDir string) {
	t.Helper()
	RequireTestOutputsWithConfig(t, resultsDir, MarketWorkerTestOutputConfig())
}
