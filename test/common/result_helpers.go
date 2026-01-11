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

	"github.com/stretchr/testify/require"
)

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

// AssertResultFilesExist validates that result files exist with content
// This function FAILS the test if output files are missing or empty
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
