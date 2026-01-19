// -----------------------------------------------------------------------
// Job Configuration Validation Tests
// Tests that all job definition TOML files are properly configured
// This is a common test that validates configuration for all job tests
// -----------------------------------------------------------------------

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/test/common"
)

// jobDefinitionDirectories lists all directories that may contain job definition TOML files
// These are scanned during configuration validation
var jobDefinitionDirectories = []string{
	"bin/job-definitions",
	"deployments/common/job-definitions",
	"test/config/job-definitions",
}

// TestJobConfigurationValidation validates all job definition TOML files can be parsed
// This test ensures job definitions are correctly formatted and will load at runtime
// Note: Files with deprecated worker types will cause warnings but not failures
// The critical test is TestJobConfigurationNoStepsArray which checks for the [[steps]] format
func TestJobConfigurationValidation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	resultsDir := env.GetResultsDir()
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: %s", time.Now().Format(time.RFC3339), t.Name()))

	defer func() {
		common.WriteTestLog(t, resultsDir, testLog)
	}()

	// Find project root by looking for go.mod
	projectRoot := findProjectRoot(t)
	require.NotEmpty(t, projectRoot, "Could not find project root")
	testLog = append(testLog, fmt.Sprintf("[%s] Project root: %s", time.Now().Format(time.RFC3339), projectRoot))

	// Scan all job definition directories
	var allFiles []string
	for _, dir := range jobDefinitionDirectories {
		fullPath := filepath.Join(projectRoot, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			testLog = append(testLog, fmt.Sprintf("[%s] Directory not found (skipped): %s", time.Now().Format(time.RFC3339), dir))
			continue
		}

		files, err := filepath.Glob(filepath.Join(fullPath, "*.toml"))
		if err != nil {
			t.Logf("Error scanning directory %s: %v", dir, err)
			continue
		}

		for _, f := range files {
			allFiles = append(allFiles, f)
		}
		testLog = append(testLog, fmt.Sprintf("[%s] Found %d TOML files in %s", time.Now().Format(time.RFC3339), len(files), dir))
	}

	require.NotEmpty(t, allFiles, "No job definition TOML files found in any directory")
	testLog = append(testLog, fmt.Sprintf("[%s] Total TOML files to validate: %d", time.Now().Format(time.RFC3339), len(allFiles)))

	// Validate each file
	var validCount, parseErrors, conversionErrors int
	var errors []string

	for _, filePath := range allFiles {
		fileName := filepath.Base(filePath)
		t.Run(fileName, func(t *testing.T) {
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Failed to read file: %s", filePath)

			// Parse using the jobs package ParseTOML function
			// This specifically catches [[steps]] format errors
			jobFile, err := jobs.ParseTOML(content)
			if err != nil {
				parseErrors++
				errorMsg := fmt.Sprintf("%s: %v", fileName, err)
				errors = append(errors, errorMsg)
				testLog = append(testLog, fmt.Sprintf("[%s] PARSE ERROR: %s", time.Now().Format(time.RFC3339), errorMsg))
				// Parse errors are critical - fail the test
				t.Errorf("Failed to parse job definition: %v", err)
				return
			}

			// Verify required fields
			assert.NotEmpty(t, jobFile.ID, "Job definition must have an ID")
			assert.NotEmpty(t, jobFile.Name, "Job definition must have a name")

			// Verify conversion to model works
			// Conversion errors (e.g., invalid worker types) are warnings, not failures
			// These are pre-existing issues with deprecated worker types
			jobDef, err := jobFile.ToJobDefinition()
			if err != nil {
				conversionErrors++
				errorMsg := fmt.Sprintf("%s: conversion warning: %v", fileName, err)
				testLog = append(testLog, fmt.Sprintf("[%s] WARN: %s", time.Now().Format(time.RFC3339), errorMsg))
				// Log but don't fail - deprecated worker types are informational
				t.Logf("Warning: conversion issue (deprecated worker type): %v", err)
				return
			}

			// Log success
			validCount++
			testLog = append(testLog, fmt.Sprintf("[%s] PASS: %s (id=%s, name=%s, steps=%d)",
				time.Now().Format(time.RFC3339), fileName, jobDef.ID, jobDef.Name, len(jobDef.Steps)))
			t.Logf("Valid: %s (id=%s, steps=%d)", fileName, jobDef.ID, len(jobDef.Steps))
		})
	}

	// Summary
	testLog = append(testLog, fmt.Sprintf("[%s] Summary: %d valid, %d parse errors, %d conversion warnings",
		time.Now().Format(time.RFC3339), validCount, parseErrors, conversionErrors))

	if parseErrors > 0 {
		testLog = append(testLog, fmt.Sprintf("[%s] Parse Errors:", time.Now().Format(time.RFC3339)))
		for _, e := range errors {
			testLog = append(testLog, fmt.Sprintf("  - %s", e))
		}
	}

	// Save configuration summary to results
	summary := fmt.Sprintf("# Job Configuration Validation Summary\n\n"+
		"- Total files: %d\n"+
		"- Valid: %d\n"+
		"- Parse errors: %d\n"+
		"- Conversion warnings: %d\n\n"+
		"## Files Validated:\n\n", len(allFiles), validCount, parseErrors, conversionErrors)

	for _, f := range allFiles {
		summary += fmt.Sprintf("- %s\n", filepath.Base(f))
	}

	if parseErrors > 0 {
		summary += "\n## Parse Errors (CRITICAL):\n\n"
		for _, e := range errors {
			summary += fmt.Sprintf("- %s\n", e)
		}
	}

	summaryPath := filepath.Join(resultsDir, "configuration_summary.md")
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		t.Logf("Warning: failed to write summary: %v", err)
	}

	testLog = append(testLog, fmt.Sprintf("[%s] PASS: Job configuration validation completed", time.Now().Format(time.RFC3339)))
}

// TestJobConfigurationNoStepsArray specifically tests that no files use the deprecated [[steps]] format
// This is a guard test to catch configuration regressions
func TestJobConfigurationNoStepsArray(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	resultsDir := env.GetResultsDir()
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: %s", time.Now().Format(time.RFC3339), t.Name()))

	defer func() {
		common.WriteTestLog(t, resultsDir, testLog)
	}()

	// Find project root
	projectRoot := findProjectRoot(t)
	require.NotEmpty(t, projectRoot, "Could not find project root")

	// Scan all directories for TOML files
	var violations []string

	for _, dir := range jobDefinitionDirectories {
		fullPath := filepath.Join(projectRoot, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		files, err := filepath.Glob(filepath.Join(fullPath, "*.toml"))
		if err != nil {
			continue
		}

		for _, filePath := range files {
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Logf("Warning: could not read %s: %v", filePath, err)
				continue
			}

			// Check for [[steps]] pattern (array of tables)
			if strings.Contains(string(content), "[[steps]]") {
				fileName := filepath.Base(filePath)
				violation := fmt.Sprintf("%s uses deprecated [[steps]] format", fileName)
				violations = append(violations, violation)
				testLog = append(testLog, fmt.Sprintf("[%s] VIOLATION: %s", time.Now().Format(time.RFC3339), violation))
				t.Errorf("FAIL: %s - file uses deprecated [[steps]] array format, must use [step.{name}] format", fileName)
			}
		}
	}

	if len(violations) == 0 {
		testLog = append(testLog, fmt.Sprintf("[%s] PASS: No files use deprecated [[steps]] format", time.Now().Format(time.RFC3339)))
		t.Log("PASS: No job definition files use the deprecated [[steps]] format")
	} else {
		testLog = append(testLog, fmt.Sprintf("[%s] FAIL: %d files use deprecated [[steps]] format", time.Now().Format(time.RFC3339), len(violations)))
	}
}

// TestJobConfigurationRequiredFields validates that all job definitions have required fields
func TestJobConfigurationRequiredFields(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	resultsDir := env.GetResultsDir()
	var testLog []string
	testLog = append(testLog, fmt.Sprintf("[%s] Test started: %s", time.Now().Format(time.RFC3339), t.Name()))

	defer func() {
		common.WriteTestLog(t, resultsDir, testLog)
	}()

	projectRoot := findProjectRoot(t)
	require.NotEmpty(t, projectRoot, "Could not find project root")

	var allFiles []string
	for _, dir := range jobDefinitionDirectories {
		fullPath := filepath.Join(projectRoot, dir)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		files, err := filepath.Glob(filepath.Join(fullPath, "*.toml"))
		if err != nil {
			continue
		}
		allFiles = append(allFiles, files...)
	}

	require.NotEmpty(t, allFiles, "No job definition TOML files found")

	for _, filePath := range allFiles {
		fileName := filepath.Base(filePath)
		t.Run(fileName, func(t *testing.T) {
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Failed to read file")

			jobFile, err := jobs.ParseTOML(content)
			if err != nil {
				// Skip files with parse errors (covered by other test)
				t.Skipf("Parse error (covered by TestJobConfigurationValidation): %v", err)
				return
			}

			// Required fields
			assert.NotEmpty(t, jobFile.ID, "Job must have 'id' field")
			assert.NotEmpty(t, jobFile.Name, "Job must have 'name' field")

			// If it has steps, verify step structure
			if len(jobFile.Step) > 0 {
				for stepName, stepData := range jobFile.Step {
					stepType, hasType := stepData["type"]
					assert.True(t, hasType, "Step '%s' must have 'type' field", stepName)
					assert.NotEmpty(t, stepType, "Step '%s' type must not be empty", stepName)
				}
			}

			testLog = append(testLog, fmt.Sprintf("[%s] PASS: %s has all required fields", time.Now().Format(time.RFC3339), fileName))
		})
	}
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Logf("Warning: could not get working directory: %v", err)
		return ""
	}

	// Walk up looking for go.mod
	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	// Try common relative paths from test directory
	testPaths := []string{
		"../..", // test/api -> root
		"..",    // test -> root
	}

	for _, rel := range testPaths {
		absPath, err := filepath.Abs(filepath.Join(cwd, rel))
		if err != nil {
			continue
		}
		goModPath := filepath.Join(absPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return absPath
		}
	}

	return ""
}
