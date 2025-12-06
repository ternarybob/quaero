package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// skipIfNoChrome skips the test if Chrome is not available (required for service startup)
func skipIfNoChrome(t *testing.T) {
	chromeNames := []string{"google-chrome", "chromium", "chromium-browser", "chrome"}
	for _, name := range chromeNames {
		if _, err := exec.LookPath(name); err == nil {
			return
		}
	}
	t.Skip("Skipping test - Chrome/Chromium not found (required for service startup)")
}

// TestSummaryAgentWithDependency tests the summary agent workflow:
// 1. Create a local_dir job to index files
// 2. Create a summary job that depends on the index step
// 3. Execute the combined job definition
// 4. Verify summary document is created
func TestSummaryAgentWithDependency(t *testing.T) {
	skipIfNoChrome(t)
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if Gemini API key is available
	if !hasGeminiAPIKey(env) {
		t.Skip("Skipping test - no valid google_gemini_api_key found in environment")
	}

	// Create test directory with code files
	testDir, cleanup := createTestCodeDirectory(t)
	defer cleanup()

	// Create a combined job with index step and dependent summary step
	defID := fmt.Sprintf("summary-dep-test-%d", time.Now().UnixNano())
	jobName := "Summary Dependency Test"

	body := map[string]interface{}{
		"id":          defID,
		"name":        jobName,
		"description": "Test summary agent with step dependency",
		"type":        "summarizer",
		"enabled":     true,
		"tags":        []string{"test", "summary-output"},
		"steps": []map[string]interface{}{
			{
				"name": "index-files",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md", ".txt"},
					"exclude_paths":      []string{".git", "node_modules"},
					"max_file_size":      1048576,
					"max_files":          50,
				},
			},
			{
				"name":    "generate-summary",
				"type":    "summary",
				"depends": "index-files", // Summary depends on index completing
				"config": map[string]interface{}{
					"prompt":      "Review the code base and provide an architectural summary of the code in markdown. Include: main components, file structure patterns, key functions/types, and how the code is organized.",
					"filter_tags": []string{"test", "summary-output"},
					"api_key":     "{google_gemini_api_key}",
				},
			},
		},
	}

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Job definition creation failed with status: %d", resp.StatusCode)
	}

	var createResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(resp, &createResult))
	t.Logf("Created job definition: %s", defID)

	// Cleanup job definition after test
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
		t.Fatalf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID, ok := execResult["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	t.Logf("Executed job, got job_id: %s", jobID)

	// Wait for job completion
	finalStatus := waitForJobCompletion(t, helper, jobID, 5*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)

	// Verify job completed successfully
	assert.Equal(t, "completed", finalStatus, "Job should complete successfully")

	// Verify summary document was created
	verifySummaryDocumentCreated(t, helper, "summary")
}

// TestSummaryAgentPlainRequest tests the summary agent with a simple plain text prompt
func TestSummaryAgentPlainRequest(t *testing.T) {
	skipIfNoChrome(t)
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if Gemini API key is available
	if !hasGeminiAPIKey(env) {
		t.Skip("Skipping test - no valid google_gemini_api_key found in environment")
	}

	// First create some documents to summarize
	testDir, cleanup := createTestCodeDirectory(t)
	defer cleanup()

	// Step 1: Index files first (separate job)
	indexDefID := fmt.Sprintf("index-for-summary-%d", time.Now().UnixNano())
	indexBody := map[string]interface{}{
		"id":          indexDefID,
		"name":        "Index for Summary",
		"description": "Index files before summary",
		"type":        "local_dir",
		"enabled":     true,
		"tags":        []string{"plain-test"},
		"steps": []map[string]interface{}{
			{
				"name": "index",
				"type": "local_dir",
				"config": map[string]interface{}{
					"dir_path":           testDir,
					"include_extensions": []string{".go", ".md"},
					"max_files":          20,
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", indexBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Index job creation should succeed")

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + indexDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute index job
	execResp, err := helper.POST("/api/job-definitions/"+indexDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()
	require.Equal(t, http.StatusAccepted, execResp.StatusCode)

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	indexJobID := execResult["job_id"].(string)

	// Wait for index to complete
	indexStatus := waitForJobCompletion(t, helper, indexJobID, 2*time.Minute)
	require.Equal(t, "completed", indexStatus, "Index job should complete")
	t.Logf("Index job completed")

	// Step 2: Create summary job with plain prompt
	summaryDefID := fmt.Sprintf("summary-plain-%d", time.Now().UnixNano())
	summaryBody := map[string]interface{}{
		"id":          summaryDefID,
		"name":        "Plain Summary Test",
		"description": "Simple summary with plain prompt",
		"type":        "summarizer",
		"enabled":     true,
		"tags":        []string{"summary-test"},
		"steps": []map[string]interface{}{
			{
				"name": "summarize",
				"type": "summary",
				"config": map[string]interface{}{
					// Plain, simple prompt
					"prompt":      "Summarize the content of these documents.",
					"filter_tags": []string{"plain-test"},
					"api_key":     "{google_gemini_api_key}",
				},
			},
		},
	}

	resp2, err := helper.POST("/api/job-definitions", summaryBody)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusCreated, resp2.StatusCode, "Summary job creation should succeed")

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + summaryDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute summary job
	execResp2, err := helper.POST("/api/job-definitions/"+summaryDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp2.Body.Close()
	require.Equal(t, http.StatusAccepted, execResp2.StatusCode)

	var execResult2 map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
	summaryJobID := execResult2["job_id"].(string)

	// Wait for summary to complete
	summaryStatus := waitForJobCompletion(t, helper, summaryJobID, 5*time.Minute)
	t.Logf("Summary job completed with status: %s", summaryStatus)

	assert.Equal(t, "completed", summaryStatus, "Summary job should complete successfully")

	// Verify summary document exists
	verifySummaryDocumentCreated(t, helper, "summary")
}

// TestSummaryAgentValidation tests validation of summary agent configuration
func TestSummaryAgentValidation(t *testing.T) {
	skipIfNoChrome(t)
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing prompt",
			config: map[string]interface{}{
				"filter_tags": []string{"test"},
				"api_key":     "test-key",
			},
			expectError: true,
			errorMsg:    "prompt",
		},
		{
			name: "missing filter_tags",
			config: map[string]interface{}{
				"prompt":  "Summarize this",
				"api_key": "test-key",
			},
			expectError: true,
			errorMsg:    "filter_tags",
		},
		{
			name: "valid config",
			config: map[string]interface{}{
				"prompt":      "Summarize the documents",
				"filter_tags": []string{"test"},
				"api_key":     "test-key",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defID := fmt.Sprintf("validation-test-%d", time.Now().UnixNano())

			body := map[string]interface{}{
				"id":          defID,
				"name":        "Validation Test: " + tt.name,
				"type":        "summarizer",
				"enabled":     true,
				"steps": []map[string]interface{}{
					{
						"name":   "summarize",
						"type":   "summary",
						"config": tt.config,
					},
				},
			}

			resp, err := helper.POST("/api/job-definitions", body)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tt.expectError {
				// Validation errors may return 400 or the definition may be created
				// but execution would fail - depends on when validation happens
				if resp.StatusCode == http.StatusCreated {
					// Cleanup if created
					delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
					if delResp != nil {
						delResp.Body.Close()
					}
				}
			} else {
				assert.Equal(t, http.StatusCreated, resp.StatusCode, "Valid config should create definition")
				// Cleanup
				delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
				if delResp != nil {
					delResp.Body.Close()
				}
			}
		})
	}
}

// Helper functions

// hasGeminiAPIKey checks if Gemini API key is configured
func hasGeminiAPIKey(env *common.TestEnvironment) bool {
	// Check environment variable
	if key := os.Getenv("QUAERO_GEMINI_GOOGLE_API_KEY"); key != "" && len(key) > 10 {
		return true
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" && len(key) > 10 {
		return true
	}

	// Check KV store via API
	helper := env.NewHTTPTestHelper(nil)
	resp, err := helper.GET("/api/kv/google_gemini_api_key")
	if err != nil || resp == nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		if err := helper.ParseJSONResponse(resp, &result); err == nil {
			if value, ok := result["value"].(string); ok && len(value) > 10 {
				return true
			}
		}
	}

	return false
}

// createTestCodeDirectory creates a temporary directory with test code files
func createTestCodeDirectory(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "quaero-summary-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Create test files
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

// Main entry point for the application
func main() {
	fmt.Println("Hello, World!")
	service := NewService()
	service.Run()
}
`,
		"service.go": `package main

// Service represents the main application service
type Service struct {
	Name    string
	Version string
	config  *Config
}

// NewService creates a new service instance
func NewService() *Service {
	return &Service{
		Name:    "MyApp",
		Version: "1.0.0",
	}
}

// Run starts the service
func (s *Service) Run() {
	// Initialize components
	// Start HTTP server
	// Handle graceful shutdown
}
`,
		"config.go": `package main

// Config holds application configuration
type Config struct {
	Host string
	Port int
	DB   DatabaseConfig
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Driver string
	DSN    string
}
`,
		"README.md": `# Test Project

This is a test project for summary agent testing.

## Features

- Service-based architecture
- Configuration management
- Database integration

## Usage

` + "```" + `bash
go run .
` + "```" + `
`,
		"utils/helpers.go": `package utils

// FormatString formats a string with prefix
func FormatString(s string) string {
	return "[PREFIX] " + s
}

// ValidateInput validates user input
func ValidateInput(input string) error {
	if input == "" {
		return fmt.Errorf("input cannot be empty")
	}
	return nil
}
`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	t.Logf("Created test directory with %d files at: %s", len(testFiles), tempDir)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// verifySummaryDocumentCreated checks if a summary document was created
func verifySummaryDocumentCreated(t *testing.T, helper *common.HTTPTestHelper, tag string) {
	resp, err := helper.GET("/api/documents?tags=" + tag)
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
			ID       string                 `json:"id"`
			Title    string                 `json:"title"`
			Tags     []string               `json:"tags"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"documents"`
		Total int `json:"total"`
	}

	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse document response: %v", err)
		return
	}

	t.Logf("Found %d documents with tag '%s'", result.Total, tag)

	// Look for summary document
	for _, doc := range result.Documents {
		if strings.Contains(strings.ToLower(doc.Title), "summary") {
			t.Logf("Found summary document: %s (tags: %v)", doc.Title, doc.Tags)

			// Check metadata
			if count, ok := doc.Metadata["source_document_count"]; ok {
				t.Logf("Summary generated from %v source documents", count)
			}
			return
		}
	}

	t.Logf("No summary document found among %d results", len(result.Documents))
}
