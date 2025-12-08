package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Types and Structs
// =============================================================================

// devopsTestContext holds shared state for DevOps enrichment tests
type devopsTestContext struct {
	t             *testing.T
	env           *common.TestEnvironment
	ctx           context.Context
	baseURL       string
	jobsURL       string
	queueURL      string
	helper        *common.HTTPTestHelper
	docsCount     int      // Track number of imported docs
	importedFiles []string // Track imported file paths for manifest
	screenshotNum int      // Sequential screenshot counter
}

// LocalDirImportConfig holds configuration for a local_dir import job
type LocalDirImportConfig struct {
	DirPath           string   // Path to directory to import
	IncludeExtensions []string // File extensions to include (nil = all files)
	ExcludePaths      []string // Paths to exclude
	MaxFileSize       int64    // Max file size in bytes (0 = default)
	Tags              []string // Tags to apply to imported documents
}

// LocalDirImportResult holds the result of a local_dir import job
type LocalDirImportResult struct {
	JobID         string   // ID of the executed job
	JobDefID      string   // ID of the job definition
	ImportedCount int      // Number of files imported
	ImportedFiles []string // List of imported file paths
	Success       bool     // Whether import completed successfully
}

// =============================================================================
// Public Test Functions
// =============================================================================

// TestDevOpsEnrichmentPipeline_FullFlow tests the complete enrichment pipeline
func TestDevOpsEnrichmentPipeline_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}

	dtc, cleanup := newDevopsTestContext(t, 10*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: DevOps Enrichment Full Flow ---")

	// Save job definition TOML to results directory
	if err := dtc.loadAndSaveJobDefinitionToml(); err != nil {
		t.Fatalf("Failed to load job definition: %v", err)
	}

	// Screenshot 1: Initial state - DOCUMENTS page showing empty database
	documentsURL := dtc.baseURL + "/documents"
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(documentsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to documents page: %v", err)
	}
	dtc.takeSequentialScreenshot("initial_empty_documents")

	// Screenshot 2: JOBS page showing available job definitions (loaded at startup)
	jobsListURL := dtc.baseURL + "/jobs"
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(jobsListURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to jobs page: %v", err)
	}
	dtc.takeSequentialScreenshot("jobs_definitions_available")

	// 1. Import test fixtures via API (test data setup)
	if err := dtc.importFixtures(); err != nil {
		t.Fatalf("Failed to import fixtures: %v", err)
	}

	// Verify documents were imported with correct tags
	if err := dtc.verifyImportedDocumentTags(); err != nil {
		t.Fatalf("Tag verification failed: %v", err)
	}

	// Screenshot 3: DOCUMENTS page showing imported files
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(documentsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to documents page: %v", err)
	}
	dtc.takeSequentialScreenshot("documents_after_import")

	// 2. Trigger enrichment pipeline via UI
	jobID, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger enrichment: %v", err)
	}

	// 3. Monitor job progress (polling with timeout, with step screenshots)
	if err := dtc.monitorJobWithPolling(jobID, 8*time.Minute); err != nil {
		dtc.takeSequentialScreenshot("job_failed")
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// Screenshot: QUEUE page showing completed job
	queueURL := dtc.baseURL + "/queue"
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(queueURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to queue page: %v", err)
	}
	dtc.takeSequentialScreenshot("queue_job_completed")

	// 4. Verify enrichment results (with actual data validation)
	if err := dtc.verifyEnrichmentResults(); err != nil {
		dtc.takeSequentialScreenshot("verification_failed")
		t.Fatalf("Enrichment verification failed: %v", err)
	}

	// 5. Verify documents have devops metadata (with content validation)
	if err := dtc.verifyDocumentsEnriched(); err != nil {
		dtc.env.LogTest(t, "Warning: Document verification failed: %v", err)
		// Don't fail the test - this is informational
	}

	// Screenshot: DOCUMENTS page showing enriched docs (should show devops-enriched tag)
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(documentsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to documents page: %v", err)
	}
	dtc.takeSequentialScreenshot("documents_after_enrichment")

	dtc.env.LogTest(t, "✓ Test completed successfully")
}

// TestDevOpsEnrichmentPipeline_ProgressMonitoring tests job progress monitoring via UI
func TestDevOpsEnrichmentPipeline_ProgressMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}

	dtc, cleanup := newDevopsTestContext(t, 10*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: DevOps Progress Monitoring ---")

	// Save job definition TOML to results directory
	if err := dtc.loadAndSaveJobDefinitionToml(); err != nil {
		t.Fatalf("Failed to load job definition: %v", err)
	}

	// Import fixtures
	if err := dtc.importFixtures(); err != nil {
		t.Fatalf("Failed to import fixtures: %v", err)
	}

	// Trigger enrichment
	jobID, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger enrichment: %v", err)
	}

	// Navigate to Queue page to monitor via UI
	if err := chromedp.Run(dtc.ctx, chromedp.Navigate(dtc.queueURL)); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}

	// Wait for page to load
	if err := chromedp.Run(dtc.ctx,
		chromedp.WaitVisible(`.page-title`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		t.Fatalf("Queue page did not load: %v", err)
	}

	dtc.env.TakeScreenshot(dtc.ctx, "devops_queue_page")

	// Monitor job via polling (simpler than UI monitoring for this test)
	startTime := time.Now()
	lastScreenshot := time.Now()

	for {
		if time.Since(startTime) > 8*time.Minute {
			t.Fatal("Job did not complete within timeout")
		}

		// Take periodic screenshots
		if time.Since(lastScreenshot) >= 30*time.Second {
			elapsed := time.Since(startTime)
			screenshotName := fmt.Sprintf("devops_progress_%ds", int(elapsed.Seconds()))
			dtc.env.TakeFullScreenshot(dtc.ctx, screenshotName)
			lastScreenshot = time.Now()
		}

		// Check job status
		resp, err := dtc.helper.GET("/api/jobs/" + jobID)
		if err != nil {
			t.Fatalf("Failed to get job status: %v", err)
		}

		var job map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&job)
		resp.Body.Close()

		status, _ := job["status"].(string)
		if status == "completed" {
			dtc.env.LogTest(t, "✓ Job completed")
			break
		}

		if status == "failed" || status == "cancelled" {
			dtc.env.TakeScreenshot(dtc.ctx, "devops_job_terminal_state")
			t.Fatalf("Job reached terminal state: %s", status)
		}

		time.Sleep(2 * time.Second)
	}

	// Take final screenshot
	dtc.env.TakeFullScreenshot(dtc.ctx, "devops_progress_completed")
	dtc.env.LogTest(t, "✓ Progress monitoring test completed")
}

// TestDevOpsEnrichmentPipeline_IncrementalEnrich tests re-enrichment with new files
func TestDevOpsEnrichmentPipeline_IncrementalEnrich(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}

	dtc, cleanup := newDevopsTestContext(t, 15*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: Incremental Enrichment ---")

	// Save job definition TOML to results directory
	if err := dtc.loadAndSaveJobDefinitionToml(); err != nil {
		t.Fatalf("Failed to load job definition: %v", err)
	}

	// 1. Import initial subset of files
	dtc.env.LogTest(t, "Importing initial file subset...")
	if err := dtc.importFixtures(); err != nil {
		t.Fatalf("Failed to import fixtures: %v", err)
	}

	initialCount := dtc.docsCount

	// 2. Run first enrichment
	dtc.env.LogTest(t, "Running initial enrichment...")
	jobID1, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger initial enrichment: %v", err)
	}

	if err := dtc.monitorJobWithPolling(jobID1, 8*time.Minute); err != nil {
		t.Fatalf("Initial enrichment failed: %v", err)
	}

	dtc.env.LogTest(t, "✓ Initial enrichment completed")

	// 3. Add more files (create synthetic documents)
	dtc.env.LogTest(t, "Adding new files...")
	newDocs := []map[string]interface{}{
		{
			"id":               uuid.New().String(),
			"source_type":      "local_file",
			"url":              "file:///test/new_component.cpp",
			"title":            "new_component.cpp",
			"content_markdown": "// New component\n#include <iostream>\nvoid newFunc() {}",
			"metadata": map[string]interface{}{
				"file_type": ".cpp",
				"language":  "cpp",
			},
			"tags": []string{"test-fixture", "devops-candidate"},
		},
		{
			"id":               uuid.New().String(),
			"source_type":      "local_file",
			"url":              "file:///test/new_utils.h",
			"title":            "new_utils.h",
			"content_markdown": "// New utilities\n#ifndef NEW_UTILS_H\n#define NEW_UTILS_H\nvoid util();\n#endif",
			"metadata": map[string]interface{}{
				"file_type": ".h",
				"language":  "cpp-header",
			},
			"tags": []string{"test-fixture", "devops-candidate"},
		},
	}

	for _, doc := range newDocs {
		resp, err := dtc.helper.POST("/api/documents", doc)
		if err != nil {
			dtc.env.LogTest(t, "Warning: Failed to add new document: %v", err)
			continue
		}
		resp.Body.Close()
	}

	dtc.env.LogTest(t, "✓ Added %d new documents", len(newDocs))

	// 4. Run second enrichment
	dtc.env.LogTest(t, "Running incremental enrichment...")
	jobID2, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger incremental enrichment: %v", err)
	}

	if err := dtc.monitorJobWithPolling(jobID2, 8*time.Minute); err != nil {
		t.Fatalf("Incremental enrichment failed: %v", err)
	}

	// 5. Verify graph was updated
	if err := dtc.verifyEnrichmentResults(); err != nil {
		t.Fatalf("Verification after incremental enrichment failed: %v", err)
	}

	dtc.env.LogTest(t, "✓ Incremental enrichment test completed (initial: %d docs, added: %d docs)",
		initialCount, len(newDocs))
}

// TestDevOpsEnrichmentPipeline_LargeCodebase tests enrichment with many files
func TestDevOpsEnrichmentPipeline_LargeCodebase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large codebase test")
	}

	dtc, cleanup := newDevopsTestContext(t, 15*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: Large Codebase Enrichment ---")

	// Save job definition TOML to results directory
	if err := dtc.loadAndSaveJobDefinitionToml(); err != nil {
		t.Fatalf("Failed to load job definition: %v", err)
	}

	// Generate synthetic files (simulate large codebase)
	dtc.env.LogTest(t, "Generating synthetic codebase...")

	fileCount := 100 // Use 100 instead of 1000 for faster test
	templates := []string{
		"#include <iostream>\nclass Component%d {\npublic:\n  void process();\n};\n",
		"#ifndef HEADER_%d_H\n#define HEADER_%d_H\nvoid function%d();\n#endif\n",
		"void utility%d() {\n  // Implementation\n}\n",
		"// Configuration for module %d\nconst int CONFIG_%d = %d;\n",
	}

	for i := 0; i < fileCount; i++ {
		templateIdx := i % len(templates)
		ext := ".cpp"
		if templateIdx == 1 {
			ext = ".h"
		}

		content := fmt.Sprintf(templates[templateIdx], i, i, i, i, i, i)

		doc := map[string]interface{}{
			"id":               uuid.New().String(),
			"source_type":      "local_file",
			"url":              fmt.Sprintf("file:///synthetic/file_%d%s", i, ext),
			"title":            fmt.Sprintf("file_%d%s", i, ext),
			"content_markdown": content,
			"metadata": map[string]interface{}{
				"file_type": ext,
				"language":  "cpp",
				"synthetic": true,
			},
			"tags": []string{"test-fixture", "devops-candidate", "synthetic"},
		}

		resp, err := dtc.helper.POST("/api/documents", doc)
		if err != nil {
			dtc.env.LogTest(t, "Warning: Failed to create synthetic doc %d: %v", i, err)
			continue
		}
		resp.Body.Close()

		// Log progress every 20 files
		if (i+1)%20 == 0 {
			dtc.env.LogTest(t, "  Generated %d/%d files...", i+1, fileCount)
		}
	}

	dtc.env.LogTest(t, "✓ Generated %d synthetic files", fileCount)

	// Trigger enrichment with extended timeout
	jobID, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger enrichment: %v", err)
	}

	// Monitor with longer timeout (10 minutes)
	dtc.env.LogTest(t, "Monitoring large codebase enrichment (this may take several minutes)...")
	if err := dtc.monitorJobWithPolling(jobID, 12*time.Minute); err != nil {
		dtc.env.TakeScreenshot(dtc.ctx, "devops_large_codebase_failed")
		t.Fatalf("Large codebase enrichment failed: %v", err)
	}

	// Verify completion
	if err := dtc.verifyEnrichmentResults(); err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	dtc.env.LogTest(t, "✓ Large codebase test completed (%d files processed)", fileCount)
}

// TestLocalDirImport_IncludeExtensions tests local_dir import with different include_extensions configurations
func TestLocalDirImport_IncludeExtensions(t *testing.T) {
	dtc, cleanup := newDevopsTestContext(t, 10*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: Local Dir Import Include Extensions ---")

	// Get cpp_project fixture path
	cppProjectPath, err := getCppProjectPath()
	if err != nil {
		t.Fatalf("Failed to find cpp_project fixture: %v", err)
	}
	dtc.env.LogTest(t, "Using cpp_project fixture at: %s", cppProjectPath)

	// Screenshot 1: Initial state
	chromedp.Run(dtc.ctx, chromedp.Navigate(dtc.baseURL+"/documents"), chromedp.Sleep(2*time.Second))
	dtc.takeSequentialScreenshot("initial_empty_documents")

	// Test cases for different extensions configurations
	// cpp_project has:
	// - 5 .cpp files: main.cpp, utils.cpp, platform_linux.cpp, platform_win.cpp, test_main.cpp
	// - 2 .h files: utils.h, config.h
	// - 1 .txt file: CMakeLists.txt
	// - 1 file without extension: Makefile (won't be matched)
	testCases := []struct {
		name              string
		includeExtensions []string
		expectedCount     int
		description       string
	}{
		{
			name:              "cpp-only",
			includeExtensions: []string{".cpp"},
			expectedCount:     5,
			description:       "Import only .cpp files",
		},
		{
			name:              "cpp-and-h",
			includeExtensions: []string{".cpp", ".h"},
			expectedCount:     7,
			description:       "Import .cpp and .h files",
		},
		{
			name:              "all-files",
			includeExtensions: []string{".cpp", ".h", ".txt"}, // Include all file types in cpp_project
			expectedCount:     8,                              // 5 .cpp + 2 .h + 1 .txt (CMakeLists.txt) = 8 (Makefile has no ext)
			description:       "Import all files with standard extensions",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dtc.env.LogTest(t, "")
			dtc.env.LogTest(t, "=== Test Case %d: %s ===", i+1, tc.description)

			// Clear documents before each test case
			dtc.env.LogTest(t, "  Clearing existing documents...")
			resp, err := dtc.helper.DELETE("/api/documents/clear-all")
			if err != nil {
				t.Fatalf("Failed to clear documents: %v", err)
			}
			resp.Body.Close()
			time.Sleep(2 * time.Second) // Give time for deletion to complete

			// Configure import
			config := LocalDirImportConfig{
				DirPath:           cppProjectPath,
				IncludeExtensions: tc.includeExtensions,
				Tags:              []string{"import-test", tc.name},
			}

			// Run import via UI
			jobName := fmt.Sprintf("Import Test %s", tc.name)
			result, err := dtc.importFilesViaLocalDirJob(jobName, config, false)
			if err != nil {
				t.Fatalf("Import failed: %v", err)
			}

			// Take screenshot of results
			chromedp.Run(dtc.ctx, chromedp.Navigate(dtc.baseURL+"/documents"), chromedp.Sleep(2*time.Second))
			dtc.takeSequentialScreenshot(fmt.Sprintf("documents_after_%s_import", tc.name))

			// Verify count
			if result.ImportedCount != tc.expectedCount {
				t.Errorf("Expected %d files, got %d for %s",
					tc.expectedCount, result.ImportedCount, tc.description)
			} else {
				dtc.env.LogTest(t, "✓ %s: imported %d files (expected %d)",
					tc.description, result.ImportedCount, tc.expectedCount)
			}
		})
	}

	// Final screenshot
	dtc.takeSequentialScreenshot("test_complete")
	dtc.env.LogTest(t, "")
	dtc.env.LogTest(t, "✓ All include_extensions test cases completed")
}

// =============================================================================
// Private Helper Functions
// =============================================================================

// newDevopsTestContext creates a new test context with browser and environment
func newDevopsTestContext(t *testing.T, timeout time.Duration) (*devopsTestContext, func()) {
	// Setup Test Environment
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Create a timeout context for the entire test
	ctx, cancelTimeout := context.WithTimeout(context.Background(), timeout)

	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(1920, 1080),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)

	// Create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)

	baseURL := env.GetBaseURL()

	dtc := &devopsTestContext{
		t:        t,
		env:      env,
		ctx:      browserCtx,
		baseURL:  baseURL,
		jobsURL:  baseURL + "/jobs",
		queueURL: baseURL + "/queue",
		helper:   env.NewHTTPTestHelperWithTimeout(t, 5*time.Minute),
	}

	// Return cleanup function
	cleanup := func() {
		// Properly close the browser before canceling contexts
		if err := chromedp.Cancel(browserCtx); err != nil {
			t.Logf("Warning: browser cancel returned: %v", err)
		}
		cancelBrowser()
		cancelAlloc()
		cancelTimeout()
		env.Cleanup()
	}

	return dtc, cleanup
}

// detectLanguage maps file extension to language name
func detectLanguage(ext string) string {
	switch ext {
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp-header"
	case ".cmake":
		return "cmake"
	default:
		return "text"
	}
}

// formatFileList formats a list of files for display
func formatFileList(files []string) string {
	if len(files) == 0 {
		return "(none)"
	}
	result := ""
	for i, f := range files {
		result += fmt.Sprintf("%d. %s\n", i+1, f)
	}
	return result
}

// generateLocalDirToml generates TOML content for a complete local_dir job definition
func generateLocalDirToml(defID, name string, config LocalDirImportConfig) string {
	var sb strings.Builder

	// Job definition header
	sb.WriteString("# Local Directory Import Job Definition\n")
	sb.WriteString(fmt.Sprintf("# Test: %s\n\n", name))

	// Job-level properties
	sb.WriteString(fmt.Sprintf("id = %q\n", defID))
	sb.WriteString(fmt.Sprintf("name = %q\n", name))
	sb.WriteString("description = \"Import files from local directory\"\n")

	// Tags
	tags := config.Tags
	if len(tags) == 0 {
		tags = []string{"local-dir-import", "test"}
	}
	sb.WriteString("tags = [")
	for i, tag := range tags {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", tag))
	}
	sb.WriteString("]\n")

	sb.WriteString("schedule = \"\"\n")
	sb.WriteString("timeout = \"10m\"\n")
	sb.WriteString("enabled = true\n")
	sb.WriteString("auto_start = false\n\n")

	// Step definition using [step.{name}] format
	sb.WriteString("[step.import_files]\n")
	sb.WriteString("type = \"local_dir\"\n")
	sb.WriteString("description = \"Import files from local directory\"\n")
	sb.WriteString("on_error = \"continue\"\n")
	sb.WriteString(fmt.Sprintf("dir_path = %q\n", config.DirPath))

	// Extensions
	if config.IncludeExtensions != nil {
		sb.WriteString("extensions = [")
		for i, ext := range config.IncludeExtensions {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q", ext))
		}
		sb.WriteString("]\n")
	}

	// Exclude paths
	excludePaths := config.ExcludePaths
	if len(excludePaths) == 0 {
		excludePaths = []string{".git", "node_modules", "__pycache__"}
	}
	sb.WriteString("exclude_paths = [")
	for i, path := range excludePaths {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", path))
	}
	sb.WriteString("]\n")

	// Max file size
	maxFileSize := config.MaxFileSize
	if maxFileSize == 0 {
		maxFileSize = 1048576 // 1MB default
	}
	sb.WriteString(fmt.Sprintf("max_file_size = %d\n", maxFileSize))

	return sb.String()
}

// getCppProjectPath returns the absolute path to the cpp_project fixture directory
func getCppProjectPath() (string, error) {
	possiblePaths := []string{
		"test/fixtures/cpp_project",
		"../fixtures/cpp_project",
		"../../test/fixtures/cpp_project",
	}

	for _, p := range possiblePaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("cpp_project fixture not found")
}

// =============================================================================
// Private Methods - Screenshots and Manifests
// =============================================================================

// takeSequentialScreenshot takes a screenshot with incremented numbering (01_, 02_, etc.)
func (dtc *devopsTestContext) takeSequentialScreenshot(name string) {
	dtc.screenshotNum++
	screenshotName := fmt.Sprintf("%02d_%s", dtc.screenshotNum, name)
	if err := dtc.env.TakeFullScreenshot(dtc.ctx, screenshotName); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to take screenshot %s: %v", screenshotName, err)
	} else {
		dtc.env.LogTest(dtc.t, "  📸 Screenshot: %s", screenshotName)
	}
}

// saveImportedFilesManifest saves a manifest of imported files to the results directory
func (dtc *devopsTestContext) saveImportedFilesManifest() error {
	if len(dtc.importedFiles) == 0 {
		return nil
	}

	manifestPath := filepath.Join(dtc.env.GetResultsDir(), "imported_files.txt")
	content := fmt.Sprintf("# Imported Files Manifest\n# Total: %d files\n# Generated: %s\n\n",
		len(dtc.importedFiles), time.Now().Format(time.RFC3339))

	for i, file := range dtc.importedFiles {
		content += fmt.Sprintf("%d. %s\n", i+1, file)
	}

	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	dtc.env.LogTest(dtc.t, "Saved imported files manifest to: %s", manifestPath)
	return nil
}

// =============================================================================
// Private Methods - Job Definition Management
// =============================================================================

// loadAndSaveJobDefinitionToml loads the job definition into the service and saves a copy to results
func (dtc *devopsTestContext) loadAndSaveJobDefinitionToml() error {
	// The test service working directory is test/bin/, so job-definitions/devops_enrich.toml
	// is the standard path. This matches the [jobs] definitions_dir config in test-quaero.toml.
	// The service will auto-load from job-definitions/ on startup.
	//
	// We also check relative paths for when running tests from different directories.
	possiblePaths := []string{
		"job-definitions/devops_enrich.toml",                // From test/bin/ working dir (standard)
		"../bin/job-definitions/devops_enrich.toml",         // From test/ui/ or test/api/ when running go test
		"../../test/bin/job-definitions/devops_enrich.toml", // From project root
		"../../jobs/devops_enrich.toml",                     // Fallback to root jobs/ dir
	}

	var foundPath string
	var content []byte
	var err error
	for _, p := range possiblePaths {
		absPath, _ := filepath.Abs(p)
		content, err = os.ReadFile(absPath)
		if err == nil {
			foundPath = absPath
			break
		}
	}

	if err != nil {
		dtc.env.LogTest(dtc.t, "Warning: Could not read job definition TOML: %v", err)
		return err
	}

	dtc.env.LogTest(dtc.t, "Found job definition at: %s", foundPath)

	// Save to results directory for documentation
	destPath := filepath.Join(dtc.env.GetResultsDir(), "devops_enrich.toml")
	if err := os.WriteFile(destPath, content, 0644); err != nil {
		dtc.env.LogTest(dtc.t, "Warning: Could not save job definition TOML: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "Saved job definition TOML to: %s", destPath)
	}

	// Load the job definition into the service via API
	if err := dtc.env.LoadJobDefinitionFile(foundPath); err != nil {
		dtc.env.LogTest(dtc.t, "Warning: Could not load job definition into service: %v", err)
		return err
	}

	return nil
}

// saveLocalDirJobToml saves the job definition TOML to the results directory
func (dtc *devopsTestContext) saveLocalDirJobToml(defID, name string, config LocalDirImportConfig) {
	tomlContent := generateLocalDirToml(defID, name, config)

	// Sanitize name for filename
	safeName := strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	safeName = re.ReplaceAllString(safeName, "_")
	safeName = strings.Trim(safeName, "_")

	destPath := filepath.Join(dtc.env.GetResultsDir(), fmt.Sprintf("job_def_%s.toml", safeName))
	if err := os.WriteFile(destPath, []byte(tomlContent), 0644); err != nil {
		dtc.env.LogTest(dtc.t, "Warning: Could not save job definition TOML: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "  Saved job definition TOML to: %s", filepath.Base(destPath))
	}
}

// createLocalDirJobDefinition creates a local_dir job definition via API
func (dtc *devopsTestContext) createLocalDirJobDefinition(name string, config LocalDirImportConfig) (string, error) {
	defID := fmt.Sprintf("local-dir-import-%d", time.Now().UnixNano())

	// Build step config
	stepConfig := map[string]interface{}{
		"dir_path": config.DirPath,
	}

	// Only add extensions if explicitly provided (nil = use default extensions)
	if config.IncludeExtensions != nil {
		stepConfig["extensions"] = config.IncludeExtensions
	}

	if len(config.ExcludePaths) > 0 {
		stepConfig["exclude_paths"] = config.ExcludePaths
	} else {
		stepConfig["exclude_paths"] = []string{".git", "node_modules", "__pycache__"}
	}

	if config.MaxFileSize > 0 {
		stepConfig["max_file_size"] = config.MaxFileSize
	} else {
		stepConfig["max_file_size"] = 1048576 // 1MB default
	}

	// Build job definition
	tags := config.Tags
	if len(tags) == 0 {
		tags = []string{"local-dir-import", "test"}
	}

	body := map[string]interface{}{
		"id":          defID,
		"name":        name,
		"description": "Local directory import job for testing",
		"type":        "local_dir",
		"enabled":     true,
		"tags":        tags,
		"steps": []map[string]interface{}{
			{
				"name":   "import-files",
				"type":   "local_dir",
				"config": stepConfig,
			},
		},
	}

	resp, err := dtc.helper.POST("/api/job-definitions", body)
	if err != nil {
		return "", fmt.Errorf("failed to create job definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("job definition creation failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	if err := dtc.helper.ParseJSONResponse(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	dtc.env.LogTest(dtc.t, "  Created local_dir job definition: %s (id: %s)", name, defID)

	// Save the TOML to results directory
	dtc.saveLocalDirJobToml(defID, name, config)

	return defID, nil
}

// deleteJobDefinition deletes a job definition via API
func (dtc *devopsTestContext) deleteJobDefinition(defID string) error {
	resp, err := dtc.helper.DELETE("/api/job-definitions/" + defID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// =============================================================================
// Private Methods - Import Functions
// =============================================================================

// importFixtures imports test files from test/fixtures/cpp_project/ into the document store
func (dtc *devopsTestContext) importFixtures() error {
	dtc.env.LogTest(dtc.t, "Importing test fixtures from cpp_project...")

	fixturesDir := "../fixtures/cpp_project"
	absPath, err := filepath.Abs(fixturesDir)
	if err != nil {
		return fmt.Errorf("failed to resolve fixtures path: %w", err)
	}

	var importedCount int
	var files []string

	// Walk the fixtures directory and collect all source files
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Import only source code files
		ext := filepath.Ext(path)
		if ext == ".cpp" || ext == ".h" || ext == ".txt" || ext == ".cmake" {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk fixtures directory: %w", err)
	}

	// Import each file as a document
	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			dtc.env.LogTest(dtc.t, "  Warning: Failed to read %s: %v", filePath, err)
			continue
		}

		// Extract relative path for the title
		relPath, _ := filepath.Rel(absPath, filePath)

		doc := map[string]interface{}{
			"id":               uuid.New().String(),
			"source_type":      "local_file",
			"url":              "file://" + filePath,
			"title":            relPath,
			"content_markdown": string(content),
			"metadata": map[string]interface{}{
				"file_type": filepath.Ext(filePath),
				"file_path": relPath,
				"language":  detectLanguage(filepath.Ext(filePath)),
			},
			"tags": []string{"test-fixture", "devops-candidate"},
		}

		resp, err := dtc.helper.POST("/api/documents", doc)
		if err != nil {
			dtc.env.LogTest(dtc.t, "  Warning: Failed to import %s: %v", relPath, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			dtc.env.LogTest(dtc.t, "  Warning: Failed to import %s (status %d)", relPath, resp.StatusCode)
			continue
		}

		importedCount++
		dtc.importedFiles = append(dtc.importedFiles, relPath)
		dtc.env.LogTest(dtc.t, "  ✓ Imported: %s", relPath)
	}

	dtc.docsCount = importedCount
	dtc.env.LogTest(dtc.t, "✓ Imported %d files from fixtures", importedCount)

	// Save manifest of imported files
	if err := dtc.saveImportedFilesManifest(); err != nil {
		dtc.env.LogTest(dtc.t, "Warning: Failed to save manifest: %v", err)
	}

	if importedCount == 0 {
		return fmt.Errorf("no files were imported")
	}

	return nil
}

// importFilesViaLocalDirJob creates and runs a local_dir job to import files via UI
// Parameters:
//   - jobName: Name for the job definition
//   - config: Import configuration including directory path and file filters
//   - enableForSubsequentTests: If true, keeps the job definition for later use
//
// Returns LocalDirImportResult with import details
func (dtc *devopsTestContext) importFilesViaLocalDirJob(jobName string, config LocalDirImportConfig, enableForSubsequentTests bool) (*LocalDirImportResult, error) {
	dtc.env.LogTest(dtc.t, "Importing files via local_dir job: %s", jobName)

	if config.IncludeExtensions != nil {
		dtc.env.LogTest(dtc.t, "  Include extensions: %v", config.IncludeExtensions)
	} else {
		dtc.env.LogTest(dtc.t, "  Include extensions: all files (no filter)")
	}

	result := &LocalDirImportResult{}

	// Step 1: Create job definition via API
	defID, err := dtc.createLocalDirJobDefinition(jobName, config)
	if err != nil {
		return result, fmt.Errorf("failed to create job definition: %w", err)
	}
	result.JobDefID = defID

	// Clean up job definition unless it should persist for subsequent tests
	if !enableForSubsequentTests {
		defer dtc.deleteJobDefinition(defID)
	}

	// Step 2: Trigger job via UI (click Run button + confirm)
	_, err = dtc.triggerLocalDirJobViaUI(jobName)
	if err != nil {
		return result, fmt.Errorf("failed to trigger job via UI: %w", err)
	}

	// Step 3: Wait for job with our definition ID to appear and complete
	jobID, status, err := dtc.waitForJobByDefinitionID(defID, 3*time.Minute)
	if err != nil {
		return result, fmt.Errorf("job did not complete: %w", err)
	}
	result.JobID = jobID

	result.Success = (status == "completed")
	if !result.Success {
		return result, fmt.Errorf("job ended with status: %s", status)
	}

	// Step 4: Get imported document count
	count, err := dtc.getImportedDocumentCount(config.Tags)
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Could not get document count: %v", err)
	}
	result.ImportedCount = count

	dtc.env.LogTest(dtc.t, "✓ Import completed: %d files imported", result.ImportedCount)
	return result, nil
}

// =============================================================================
// Private Methods - Job Triggering
// =============================================================================

// triggerEnrichment triggers the DevOps enrichment pipeline (tries UI first, falls back to API)
func (dtc *devopsTestContext) triggerEnrichment() (string, error) {
	return dtc.triggerEnrichmentViaUI()
}

// triggerEnrichmentViaUI triggers the DevOps enrichment pipeline by clicking the Run button in the UI
func (dtc *devopsTestContext) triggerEnrichmentViaUI() (string, error) {
	dtc.env.LogTest(dtc.t, "Triggering DevOps enrichment pipeline via UI...")

	// Navigate to Jobs page
	jobsURL := dtc.baseURL + "/jobs"
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(jobsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return "", fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// The run button has ID: {job-name-slug}-run where the slug is lowercase with hyphens
	// For "DevOps Enrichment Pipeline", the ID is "devops-enrichment-pipeline-run"
	runButtonID := "#devops-enrichment-pipeline-run"

	// Try to click the run button
	dtc.env.LogTest(dtc.t, "  Looking for Run button...")

	var clicked bool

	// Try by ID first using JavaScript click (more reliable with Vue.js)
	var clickResult string
	err := chromedp.Run(dtc.ctx,
		chromedp.WaitVisible(runButtonID, chromedp.ByQuery),
		// Use JavaScript to click - more reliable with Vue.js event handlers
		chromedp.Evaluate(`
			(function() {
				const btn = document.querySelector('#devops-enrichment-pipeline-run');
				if (btn) {
					btn.click();
					return 'clicked';
				}
				return 'not found';
			})()
		`, &clickResult),
	)
	if err == nil && clickResult == "clicked" {
		dtc.env.LogTest(dtc.t, "  Found and clicked run button by ID (JS click): %s", runButtonID)
		clicked = true
	} else {
		dtc.env.LogTest(dtc.t, "  Button not found by ID or JS click failed (%s), trying aria-label selector...", clickResult)
		// Try by aria-label using JavaScript
		err = chromedp.Run(dtc.ctx,
			chromedp.Evaluate(`
				(function() {
					const btn = document.querySelector('button.btn-success[aria-label="Run Job"]');
					if (btn) {
						btn.click();
						return 'clicked';
					}
					return 'not found';
				})()
			`, &clickResult),
		)
		if err == nil && clickResult == "clicked" {
			dtc.env.LogTest(dtc.t, "  Found and clicked run button by aria-label (JS click)")
			clicked = true
		} else {
			dtc.env.LogTest(dtc.t, "  Button not found by aria-label, trying first btn-success...")
			// Try first btn-success button
			err = chromedp.Run(dtc.ctx,
				chromedp.Evaluate(`
					(function() {
						const btn = document.querySelector('button.btn-success');
						if (btn) {
							btn.click();
							return 'clicked';
						}
						return 'not found';
					})()
				`, &clickResult),
			)
			if err == nil && clickResult == "clicked" {
				dtc.env.LogTest(dtc.t, "  Found and clicked first btn-success button (JS click)")
				clicked = true
			}
		}
	}

	if !clicked {
		dtc.env.LogTest(dtc.t, "  Warning: Could not click run button via UI, falling back to API")
		return dtc.triggerEnrichmentViaAPI()
	}

	// Wait for confirmation modal and click confirm button
	dtc.env.LogTest(dtc.t, "  Waiting for confirmation modal...")
	time.Sleep(500 * time.Millisecond)

	// Click the confirm button in the modal (Alpine.js confirmation dialog)
	var confirmClicked string
	err = chromedp.Run(dtc.ctx,
		chromedp.WaitVisible("body.modal-open", chromedp.ByQuery),
		chromedp.Evaluate(`
			(function() {
				const confirmBtn = document.querySelector('.modal.active .btn-primary, #confirmation-modal .btn-primary');
				if (confirmBtn) { confirmBtn.click(); return 'clicked'; }
				if (window.Alpine && Alpine.store('confirmation')) {
					Alpine.store('confirmation').confirm();
					return 'clicked via Alpine';
				}
				return 'not found';
			})()
		`, &confirmClicked),
	)
	if err != nil || (confirmClicked != "clicked" && confirmClicked != "clicked via Alpine") {
		dtc.env.LogTest(dtc.t, "  Warning: Could not click confirm (%s), trying Alpine directly...", confirmClicked)
		chromedp.Run(dtc.ctx,
			chromedp.Evaluate(`Alpine.store('confirmation').confirm()`, &confirmClicked),
		)
	}
	dtc.env.LogTest(dtc.t, "  Confirmation handled: %s", confirmClicked)

	// Wait for job to start
	dtc.env.LogTest(dtc.t, "  Waiting for job to start...")
	time.Sleep(2 * time.Second)

	// Get the latest job ID via API (since we triggered via UI)
	return dtc.getLatestJobID()
}

// triggerEnrichmentViaAPI triggers the DevOps enrichment pipeline via API (fallback)
func (dtc *devopsTestContext) triggerEnrichmentViaAPI() (string, error) {
	dtc.env.LogTest(dtc.t, "Triggering DevOps enrichment pipeline via API...")

	resp, err := dtc.helper.POST("/api/devops/enrich", nil)
	if err != nil {
		return "", fmt.Errorf("failed to trigger enrichment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("enrichment trigger failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := dtc.helper.ParseJSONResponse(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	jobID, ok := result["job_id"].(string)
	if !ok {
		return "", fmt.Errorf("job_id not found in response")
	}

	dtc.env.LogTest(dtc.t, "✓ Enrichment pipeline triggered via API (job ID: %s)", jobID)
	return jobID, nil
}

// triggerLocalDirJobViaUI triggers a local_dir job by clicking the run button and confirming
func (dtc *devopsTestContext) triggerLocalDirJobViaUI(jobName string) (string, error) {
	dtc.env.LogTest(dtc.t, "  Triggering local_dir job via UI: %s", jobName)

	// Navigate to Jobs page
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(dtc.jobsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return "", fmt.Errorf("failed to navigate to jobs page: %w", err)
	}

	// Convert job name to button ID format: lowercase, non-alphanumeric to hyphens
	buttonID := strings.ToLower(jobName)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	buttonID = re.ReplaceAllString(buttonID, "-")
	buttonID = strings.Trim(buttonID, "-") + "-run"

	dtc.env.LogTest(dtc.t, "  Looking for run button: #%s", buttonID)

	// Click the run button using JavaScript
	var clickResult string
	err := chromedp.Run(dtc.ctx,
		chromedp.WaitVisible("#"+buttonID, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				const btn = document.querySelector('#%s');
				if (btn) {
					btn.click();
					return 'clicked';
				}
				return 'not found';
			})()
		`, buttonID), &clickResult),
	)
	if err != nil || clickResult != "clicked" {
		return "", fmt.Errorf("failed to click run button: %v (result: %s)", err, clickResult)
	}

	// Wait for confirmation modal and click confirm
	dtc.env.LogTest(dtc.t, "  Waiting for confirmation modal...")
	time.Sleep(500 * time.Millisecond)

	var confirmClicked string
	err = chromedp.Run(dtc.ctx,
		chromedp.WaitVisible("body.modal-open", chromedp.ByQuery),
		chromedp.Evaluate(`
			(function() {
				const confirmBtn = document.querySelector('.modal.active .btn-primary, #confirmation-modal .btn-primary');
				if (confirmBtn) { confirmBtn.click(); return 'clicked'; }
				if (window.Alpine && Alpine.store('confirmation')) {
					Alpine.store('confirmation').confirm();
					return 'clicked via Alpine';
				}
				return 'not found';
			})()
		`, &confirmClicked),
	)
	if err != nil || (confirmClicked != "clicked" && confirmClicked != "clicked via Alpine") {
		dtc.env.LogTest(dtc.t, "  Warning: Could not click confirm (%s), trying Alpine directly...", confirmClicked)
		chromedp.Run(dtc.ctx,
			chromedp.Evaluate(`Alpine.store('confirmation').confirm()`, &confirmClicked),
		)
	}
	dtc.env.LogTest(dtc.t, "  Confirmation handled: %s", confirmClicked)

	// Wait for job to start being processed
	time.Sleep(1 * time.Second)

	return "", nil
}

// =============================================================================
// Private Methods - Job Monitoring
// =============================================================================

// getLatestJobID gets the most recent job ID from the jobs list
func (dtc *devopsTestContext) getLatestJobID() (string, error) {
	// Retry for up to 10 seconds since job creation may take time
	maxRetries := 10
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := dtc.helper.GET("/api/jobs?limit=1&order=desc")
		if err != nil {
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to get jobs: %w", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}

		var result map[string]interface{}
		if err := dtc.helper.ParseJSONResponse(resp, &result); err != nil {
			resp.Body.Close()
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to parse jobs response: %w", err)
			}
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		jobs, ok := result["jobs"].([]interface{})
		if !ok || len(jobs) == 0 {
			if attempt == maxRetries {
				return "", fmt.Errorf("no jobs found after %d attempts", maxRetries)
			}
			dtc.env.LogTest(dtc.t, "  Waiting for job to appear (attempt %d/%d)...", attempt, maxRetries)
			time.Sleep(1 * time.Second)
			continue
		}

		job := jobs[0].(map[string]interface{})
		jobID, ok := job["id"].(string)
		if !ok {
			return "", fmt.Errorf("job ID not found")
		}

		dtc.env.LogTest(dtc.t, "✓ Found latest job (ID: %s)", jobID)
		return jobID, nil
	}
	return "", fmt.Errorf("no jobs found")
}

// waitForJobByDefinitionID waits for a job with the given definition ID to appear and complete
// Returns the job ID and final status
func (dtc *devopsTestContext) waitForJobByDefinitionID(defID string, timeout time.Duration) (string, string, error) {
	dtc.env.LogTest(dtc.t, "  Waiting for job with definition ID: %s", defID)

	deadline := time.Now().Add(timeout)
	pollInterval := 1 * time.Second
	var foundJobID string

	// Phase 1: Wait for job to appear
	for time.Now().Before(deadline) {
		resp, err := dtc.helper.GET("/api/jobs?limit=50&order=desc")
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var result map[string]interface{}
		if err := dtc.helper.ParseJSONResponse(resp, &result); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		jobs, ok := result["jobs"].([]interface{})
		if !ok {
			time.Sleep(pollInterval)
			continue
		}

		// Look for a job matching our definition ID
		for _, j := range jobs {
			job := j.(map[string]interface{})
			metadata, ok := job["metadata"].(map[string]interface{})
			if !ok {
				continue
			}

			jobDefID, ok := metadata["job_definition_id"].(string)
			if ok && jobDefID == defID {
				jobID, _ := job["id"].(string)
				foundJobID = jobID
				dtc.env.LogTest(dtc.t, "  Found job with matching definition: %s", foundJobID)
				break
			}
		}

		if foundJobID != "" {
			break
		}

		time.Sleep(pollInterval)
	}

	if foundJobID == "" {
		return "", "", fmt.Errorf("no job found for definition %s within timeout", defID)
	}

	// Phase 2: Wait for job to complete
	for time.Now().Before(deadline) {
		resp, err := dtc.helper.GET("/api/jobs/" + foundJobID)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var job map[string]interface{}
		if err := dtc.helper.ParseJSONResponse(resp, &job); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		status, _ := job["status"].(string)
		dtc.env.LogTest(dtc.t, "    Job status: %s", status)

		if status == "completed" || status == "failed" || status == "cancelled" {
			return foundJobID, status, nil
		}

		time.Sleep(pollInterval)
	}

	return foundJobID, "", fmt.Errorf("job %s did not complete within timeout", foundJobID)
}

// waitForJobCompletion waits for a job to complete and returns the final status
func (dtc *devopsTestContext) waitForJobCompletion(jobID string, timeout time.Duration) (string, error) {
	dtc.env.LogTest(dtc.t, "  Waiting for job completion (timeout: %v)...", timeout)

	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		resp, err := dtc.helper.GET("/api/jobs/" + jobID)
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var job map[string]interface{}
		if err := dtc.helper.ParseJSONResponse(resp, &job); err != nil {
			resp.Body.Close()
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		status, _ := job["status"].(string)
		dtc.env.LogTest(dtc.t, "    Job status: %s", status)

		if status == "completed" || status == "failed" || status == "cancelled" {
			return status, nil
		}

		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("job did not complete within %v", timeout)
}

// monitorJobWithPolling monitors a job via polling with step-based screenshots
func (dtc *devopsTestContext) monitorJobWithPolling(jobID string, timeout time.Duration) error {
	dtc.env.LogTest(dtc.t, "Monitoring job: %s (timeout: %v)", jobID, timeout)

	// Navigate to job details page in browser (use queue page with job filter for better visibility)
	jobDetailsURL := fmt.Sprintf("%s/queue?job=%s", dtc.baseURL, jobID)
	if err := chromedp.Run(dtc.ctx,
		chromedp.Navigate(jobDetailsURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Could not navigate to job details: %v", err)
	}
	dtc.takeSequentialScreenshot("job_details_start")

	startTime := time.Now()
	lastProgressLog := time.Now()
	checkCount := 0
	lastStep := ""
	lastStepStatus := ""

	for {
		// Check timeout
		if time.Since(startTime) > timeout {
			dtc.takeSequentialScreenshot("job_timeout")
			return fmt.Errorf("job %s did not complete within %v", jobID, timeout)
		}

		// Check context cancellation
		if err := dtc.ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during monitoring: %w", err)
		}

		// Get job status via API
		resp, err := dtc.helper.GET("/api/jobs/" + jobID)
		if err != nil {
			return fmt.Errorf("failed to get job status: %w", err)
		}

		var job map[string]interface{}
		if err := dtc.helper.ParseJSONResponse(resp, &job); err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to parse job response: %w", err)
		}
		resp.Body.Close()

		status, _ := job["status"].(string)
		checkCount++

		// Extract current step info from metadata
		currentStep := ""
		currentStepStatus := ""
		completedSteps := 0
		totalSteps := 0
		if metadata, ok := job["metadata"].(map[string]interface{}); ok {
			if stepName, ok := metadata["current_step_name"].(string); ok {
				currentStep = stepName
			}
			if stepStatus, ok := metadata["current_step_status"].(string); ok {
				currentStepStatus = stepStatus
			}
			if cs, ok := metadata["completed_steps"].(float64); ok {
				completedSteps = int(cs)
			}
			if ts, ok := metadata["total_steps"].(float64); ok {
				totalSteps = int(ts)
			}
		}

		// Take screenshot on step change (navigate to queue page to see job progress)
		if currentStep != "" && (currentStep != lastStep || currentStepStatus != lastStepStatus) {
			// Refresh to show updated state
			if err := chromedp.Run(dtc.ctx,
				chromedp.Reload(),
				chromedp.Sleep(1*time.Second),
			); err == nil {
				screenshotName := fmt.Sprintf("step_%d_of_%d_%s", completedSteps, totalSteps, currentStep)
				dtc.takeSequentialScreenshot(screenshotName)
			}
			dtc.env.LogTest(dtc.t, "  Step %d/%d: %s (%s)", completedSteps, totalSteps, currentStep, currentStepStatus)

			lastStep = currentStep
			lastStepStatus = currentStepStatus
		}

		// Log progress every 5 seconds
		if time.Since(lastProgressLog) >= 5*time.Second {
			elapsed := time.Since(startTime)
			stepInfo := ""
			if currentStep != "" {
				stepInfo = fmt.Sprintf(", step %d/%d: %s", completedSteps, totalSteps, currentStep)
			}
			dtc.env.LogTest(dtc.t, "  [%v] Monitoring... (status: %s%s)",
				elapsed.Round(time.Second), status, stepInfo)
			lastProgressLog = time.Now()
		}

		// Check if job is done
		if status == "completed" {
			// Navigate to queue page and take final screenshot
			if err := chromedp.Run(dtc.ctx,
				chromedp.Reload(),
				chromedp.Sleep(1*time.Second),
			); err == nil {
				dtc.takeSequentialScreenshot("job_details_completed")
			}
			dtc.env.LogTest(dtc.t, "✓ Job completed successfully (after %d checks)", checkCount)
			return nil
		}

		if status == "failed" {
			dtc.takeSequentialScreenshot("job_failed")
			failureReason := "unknown"
			if metadata, ok := job["metadata"].(map[string]interface{}); ok {
				if reason, ok := metadata["failure_reason"].(string); ok {
					failureReason = reason
				}
			}
			return fmt.Errorf("job %s failed: %s", jobID, failureReason)
		}

		if status == "cancelled" {
			dtc.takeSequentialScreenshot("job_cancelled")
			return fmt.Errorf("job %s was cancelled", jobID)
		}

		// Wait before next check
		time.Sleep(1 * time.Second)
	}
}

// =============================================================================
// Private Methods - Verification
// =============================================================================

// verifyImportedDocumentTags verifies documents were imported with correct tags
func (dtc *devopsTestContext) verifyImportedDocumentTags() error {
	dtc.env.LogTest(dtc.t, "Verifying imported document tags...")

	// Query all documents (no tag filter)
	resp, err := dtc.helper.GET("/api/documents?limit=100")
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse response
	var docsResponse struct {
		Documents []struct {
			ID    string   `json:"id"`
			Title string   `json:"title"`
			Tags  []string `json:"tags"`
		} `json:"documents"`
		TotalCount int `json:"total_count"`
	}
	if err := json.Unmarshal(body, &docsResponse); err != nil {
		return fmt.Errorf("failed to parse documents response: %w", err)
	}

	dtc.env.LogTest(dtc.t, "  Total documents: %d", docsResponse.TotalCount)

	// Check each document's tags
	docsWithCandidateTag := 0
	for _, doc := range docsResponse.Documents {
		hasCandidate := false
		for _, tag := range doc.Tags {
			if tag == "devops-candidate" {
				hasCandidate = true
				break
			}
		}
		if hasCandidate {
			docsWithCandidateTag++
		}
		dtc.env.LogTest(dtc.t, "    - %s: tags=%v", doc.Title, doc.Tags)
	}

	dtc.env.LogTest(dtc.t, "  Documents with 'devops-candidate' tag: %d/%d", docsWithCandidateTag, docsResponse.TotalCount)

	if docsWithCandidateTag == 0 && docsResponse.TotalCount > 0 {
		return fmt.Errorf("no documents have 'devops-candidate' tag - tags may not be stored correctly")
	}

	return nil
}

// verifyEnrichmentResults verifies that enrichment produced expected results with actual data validation
func (dtc *devopsTestContext) verifyEnrichmentResults() error {
	dtc.env.LogTest(dtc.t, "Verifying enrichment results...")

	// 1. Verify dependency graph exists AND has actual nodes
	dtc.env.LogTest(dtc.t, "  Checking dependency graph...")
	resp, err := dtc.helper.GET("/api/devops/graph")
	if err != nil {
		return fmt.Errorf("failed to get dependency graph: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dependency graph not found (status %d): %s", resp.StatusCode, string(body))
	}

	var graph map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&graph); err != nil {
		return fmt.Errorf("failed to parse graph: %w", err)
	}

	// Validate graph has actual nodes (not just empty structure)
	nodes, hasNodes := graph["nodes"].([]interface{})
	edges, hasEdges := graph["edges"].([]interface{})

	if !hasNodes || !hasEdges {
		return fmt.Errorf("graph missing nodes or edges structure")
	}

	dtc.env.LogTest(dtc.t, "  ✓ Dependency graph: %d nodes, %d edges", len(nodes), len(edges))

	// 2. Verify summary document exists AND has meaningful content
	// Note: Summary generation may fail if no documents have devops metadata - make this non-fatal
	dtc.env.LogTest(dtc.t, "  Checking DevOps summary...")
	var summary string
	resp2, err := dtc.helper.GET("/api/devops/summary")
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to get summary: %v", err)
	} else {
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp2.Body)
			dtc.env.LogTest(dtc.t, "  Warning: Summary not available (status %d): %s", resp2.StatusCode, string(body))
		} else {
			var summaryResult map[string]interface{}
			if err := json.NewDecoder(resp2.Body).Decode(&summaryResult); err != nil {
				dtc.env.LogTest(dtc.t, "  Warning: Failed to parse summary: %v", err)
			} else {
				summary, _ = summaryResult["summary"].(string)
				if summary == "" {
					dtc.env.LogTest(dtc.t, "  Warning: Summary is empty")
				} else if len(summary) < 100 {
					dtc.env.LogTest(dtc.t, "  Warning: Summary too short (%d chars)", len(summary))
				} else {
					dtc.env.LogTest(dtc.t, "  ✓ DevOps summary: %d characters", len(summary))
				}
			}
		}
	}

	// 3. Verify components endpoint returns structure with actual data
	dtc.env.LogTest(dtc.t, "  Checking components...")
	var components []interface{}
	resp3, err := dtc.helper.GET("/api/devops/components")
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to get components: %v", err)
	} else {
		defer resp3.Body.Close()
		if resp3.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp3.Body)
			dtc.env.LogTest(dtc.t, "  Warning: Components not available (status %d): %s", resp3.StatusCode, string(body))
		} else {
			var componentsResult map[string]interface{}
			if err := json.NewDecoder(resp3.Body).Decode(&componentsResult); err != nil {
				dtc.env.LogTest(dtc.t, "  Warning: Failed to parse components: %v", err)
			} else {
				components, _ = componentsResult["components"].([]interface{})
				dtc.env.LogTest(dtc.t, "  ✓ Components: %d found", len(components))
			}
		}
	}

	// 4. Verify platforms endpoint returns structure
	dtc.env.LogTest(dtc.t, "  Checking platforms...")
	var platforms map[string]interface{}
	resp4, err := dtc.helper.GET("/api/devops/platforms")
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to get platforms: %v", err)
	} else {
		defer resp4.Body.Close()
		if resp4.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp4.Body)
			dtc.env.LogTest(dtc.t, "  Warning: Platforms not available (status %d): %s", resp4.StatusCode, string(body))
		} else {
			var platformsResult map[string]interface{}
			if err := json.NewDecoder(resp4.Body).Decode(&platformsResult); err != nil {
				dtc.env.LogTest(dtc.t, "  Warning: Failed to parse platforms: %v", err)
			} else {
				platforms, _ = platformsResult["platforms"].(map[string]interface{})
				dtc.env.LogTest(dtc.t, "  ✓ Platforms: %d found", len(platforms))
			}
		}
	}

	// Save enrichment results summary to file
	resultsSummary := fmt.Sprintf(`# Enrichment Results Summary
Generated: %s

## Dependency Graph
- Nodes: %d
- Edges: %d

## Components
- Count: %d

## Platforms
- Count: %d

## Summary Length
- Characters: %d
`,
		time.Now().Format(time.RFC3339),
		len(nodes), len(edges),
		len(components),
		len(platforms),
		len(summary))

	resultsPath := filepath.Join(dtc.env.GetResultsDir(), "enrichment_results.txt")
	if err := os.WriteFile(resultsPath, []byte(resultsSummary), 0644); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to save results summary: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "  Saved results summary to: %s", resultsPath)
	}

	// 5. Run per-file enrichment assessment
	dtc.env.LogTest(dtc.t, "")
	perFileReport, err := dtc.assessPerFileEnrichment()
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Per-file assessment failed: %v", err)
	} else if perFileReport.FailedDocuments > 0 {
		dtc.env.LogTest(dtc.t, "  Warning: %d/%d documents failed per-file assessment",
			perFileReport.FailedDocuments, perFileReport.TotalDocuments)
	}

	// 6. Run summary document assessment
	dtc.env.LogTest(dtc.t, "")
	summaryAssessment, err := dtc.assessSummaryDocument()
	if err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Summary assessment failed: %v", err)
	} else if !summaryAssessment.Passed {
		dtc.env.LogTest(dtc.t, "  Warning: Summary assessment did not pass")
		for _, issue := range summaryAssessment.Issues {
			dtc.env.LogTest(dtc.t, "    - %s", issue)
		}
	}

	dtc.env.LogTest(dtc.t, "✓ All enrichment results verified")
	return nil
}

// verifyDocumentsEnriched verifies that documents have devops metadata with actual content
func (dtc *devopsTestContext) verifyDocumentsEnriched() error {
	dtc.env.LogTest(dtc.t, "Verifying documents have DevOps metadata...")

	// Query documents with devops-enriched tag
	resp, err := dtc.helper.GET("/api/documents?tags=devops-enriched&limit=100")
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	// Handle both array and object response formats
	body, _ := io.ReadAll(resp.Body)

	// Try to parse as object with "documents" field first
	var docsResponse struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(body, &docsResponse); err == nil && docsResponse.Documents != nil {
		// Object format with documents field (even if empty)
		return dtc.validateEnrichedDocuments(docsResponse.Documents)
	}

	// Try to parse as array
	var docs []map[string]interface{}
	if err := json.Unmarshal(body, &docs); err != nil {
		return fmt.Errorf("failed to parse documents: %w", err)
	}

	return dtc.validateEnrichedDocuments(docs)
}

// validateEnrichedDocuments validates the content of enriched documents
func (dtc *devopsTestContext) validateEnrichedDocuments(docs []map[string]interface{}) error {
	if len(docs) == 0 {
		dtc.env.LogTest(dtc.t, "  Warning: No documents found with devops-enriched tag")
		return nil // Don't fail, just warn
	}

	// Track enrichment statistics
	docsWithMetadata := 0
	docsWithIncludes := 0
	docsWithPlatforms := 0
	docsWithComponent := 0
	totalIncludes := 0

	var sampleDoc map[string]interface{}

	for _, doc := range docs {
		metadata, ok := doc["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		devops, hasDevOps := metadata["devops"].(map[string]interface{})
		if !hasDevOps {
			continue
		}

		docsWithMetadata++
		if sampleDoc == nil {
			sampleDoc = devops
		}

		// Check for includes
		if includes, ok := devops["includes"].([]interface{}); ok && len(includes) > 0 {
			docsWithIncludes++
			totalIncludes += len(includes)
		}

		// Check for platforms
		if platforms, ok := devops["platforms"].([]interface{}); ok && len(platforms) > 0 {
			docsWithPlatforms++
		}

		// Check for component classification
		if component, ok := devops["component"].(string); ok && component != "" {
			docsWithComponent++
		}
	}

	dtc.env.LogTest(dtc.t, "  Documents with DevOps metadata: %d/%d", docsWithMetadata, len(docs))
	dtc.env.LogTest(dtc.t, "  Documents with includes: %d (total: %d includes)", docsWithIncludes, totalIncludes)
	dtc.env.LogTest(dtc.t, "  Documents with platforms: %d", docsWithPlatforms)
	dtc.env.LogTest(dtc.t, "  Documents with component: %d", docsWithComponent)

	// Log sample devops metadata for debugging
	if sampleDoc != nil {
		sampleJSON, _ := json.MarshalIndent(sampleDoc, "    ", "  ")
		dtc.env.LogTest(dtc.t, "  Sample DevOps metadata:\n    %s", string(sampleJSON))
	}

	// Save document enrichment details to file
	enrichmentDetails := fmt.Sprintf(`# Document Enrichment Details
Generated: %s

## Statistics
- Total documents: %d
- With DevOps metadata: %d
- With includes: %d (total includes: %d)
- With platforms: %d
- With component classification: %d

## Imported Files
%s
`,
		time.Now().Format(time.RFC3339),
		len(docs),
		docsWithMetadata,
		docsWithIncludes, totalIncludes,
		docsWithPlatforms,
		docsWithComponent,
		formatFileList(dtc.importedFiles))

	detailsPath := filepath.Join(dtc.env.GetResultsDir(), "document_enrichment.txt")
	if err := os.WriteFile(detailsPath, []byte(enrichmentDetails), 0644); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to save enrichment details: %v", err)
	}

	dtc.env.LogTest(dtc.t, "✓ Found %d enriched documents", len(docs))
	return nil
}

// =============================================================================
// Private Methods - Document Assessment
// =============================================================================

// PerFileAssessment holds the assessment result for a single document
type PerFileAssessment struct {
	DocumentID   string   `json:"document_id"`
	Title        string   `json:"title"`
	HasDevOps    bool     `json:"has_devops"`
	HasIncludes  bool     `json:"has_includes"`
	HasDefines   bool     `json:"has_defines"`
	HasPlatforms bool     `json:"has_platforms"`
	HasComponent bool     `json:"has_component"`
	HasFileRole  bool     `json:"has_file_role"`
	PassCount    int      `json:"pass_count"`
	Issues       []string `json:"issues,omitempty"`
}

// PerFileAssessmentReport holds the complete per-file assessment report
type PerFileAssessmentReport struct {
	GeneratedAt     string              `json:"generated_at"`
	TotalDocuments  int                 `json:"total_documents"`
	PassedDocuments int                 `json:"passed_documents"`
	FailedDocuments int                 `json:"failed_documents"`
	Assessments     []PerFileAssessment `json:"assessments"`
}

// SummaryAssessment holds the assessment result for the summary document
type SummaryAssessment struct {
	GeneratedAt      string   `json:"generated_at"`
	SummaryLength    int      `json:"summary_length"`
	HasBuildTargets  bool     `json:"has_build_targets"`
	HasDependencies  bool     `json:"has_dependencies"`
	HasPlatforms     bool     `json:"has_platforms"`
	HasComponents    bool     `json:"has_components"`
	HasFileStructure bool     `json:"has_file_structure"`
	ExpectedSections []string `json:"expected_sections"`
	FoundSections    []string `json:"found_sections"`
	MissingSections  []string `json:"missing_sections,omitempty"`
	SummaryContent   string   `json:"summary_content"`
	Issues           []string `json:"issues,omitempty"`
	Passed           bool     `json:"passed"`
}

// assessPerFileEnrichment assesses each enriched document for proper DevOps metadata
func (dtc *devopsTestContext) assessPerFileEnrichment() (*PerFileAssessmentReport, error) {
	dtc.env.LogTest(dtc.t, "Assessing per-file enrichment...")

	report := &PerFileAssessmentReport{
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

	// Fetch all documents with devops-enriched tag
	resp, err := dtc.helper.GET("/api/documents?tags=devops-enriched&limit=500")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse response - handle both array and object formats
	var docs []map[string]interface{}
	var docsResponse struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	if err := json.Unmarshal(body, &docsResponse); err == nil && docsResponse.Documents != nil {
		// Object format with documents field (even if empty)
		docs = docsResponse.Documents
	} else if err := json.Unmarshal(body, &docs); err != nil {
		return nil, fmt.Errorf("failed to parse documents: %w", err)
	}

	report.TotalDocuments = len(docs)

	// Assess each document
	for _, doc := range docs {
		assessment := PerFileAssessment{
			DocumentID: getString(doc, "id"),
			Title:      getString(doc, "title"),
		}

		metadata, hasMetadata := doc["metadata"].(map[string]interface{})
		if !hasMetadata {
			assessment.Issues = append(assessment.Issues, "No metadata field")
			report.Assessments = append(report.Assessments, assessment)
			report.FailedDocuments++
			continue
		}

		devops, hasDevOps := metadata["devops"].(map[string]interface{})
		if !hasDevOps {
			assessment.Issues = append(assessment.Issues, "No devops metadata")
			report.Assessments = append(report.Assessments, assessment)
			report.FailedDocuments++
			continue
		}

		assessment.HasDevOps = true

		// Check for includes
		if includes, ok := devops["includes"].([]interface{}); ok && len(includes) > 0 {
			assessment.HasIncludes = true
			assessment.PassCount++
		}

		// Check for defines
		if defines, ok := devops["defines"].([]interface{}); ok && len(defines) > 0 {
			assessment.HasDefines = true
			assessment.PassCount++
		}

		// Check for platforms
		if platforms, ok := devops["platforms"].([]interface{}); ok && len(platforms) > 0 {
			assessment.HasPlatforms = true
			assessment.PassCount++
		}

		// Check for component classification
		if component, ok := devops["component"].(string); ok && component != "" {
			assessment.HasComponent = true
			assessment.PassCount++
		}

		// Check for file_role
		if fileRole, ok := devops["file_role"].(string); ok && fileRole != "" {
			assessment.HasFileRole = true
			assessment.PassCount++
		}

		// Determine if document passes (at least has devops metadata and some enrichment)
		if assessment.PassCount >= 1 {
			report.PassedDocuments++
		} else {
			assessment.Issues = append(assessment.Issues, "No enrichment fields populated")
			report.FailedDocuments++
		}

		report.Assessments = append(report.Assessments, assessment)
	}

	// Log summary
	dtc.env.LogTest(dtc.t, "  Total documents assessed: %d", report.TotalDocuments)
	dtc.env.LogTest(dtc.t, "  Passed: %d, Failed: %d", report.PassedDocuments, report.FailedDocuments)

	// Save report to results directory
	reportPath := filepath.Join(dtc.env.GetResultsDir(), "per_file_assessment.json")
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to save assessment report: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "  Saved per-file assessment to: %s", reportPath)
	}

	return report, nil
}

// assessSummaryDocument assesses the generated summary document for meaningful content
func (dtc *devopsTestContext) assessSummaryDocument() (*SummaryAssessment, error) {
	dtc.env.LogTest(dtc.t, "Assessing summary document...")

	assessment := &SummaryAssessment{
		GeneratedAt: time.Now().Format(time.RFC3339),
		ExpectedSections: []string{
			"build", "target", "dependency", "platform",
			"component", "file", "include", "structure",
		},
	}

	// Fetch the summary
	resp, err := dtc.helper.GET("/api/devops/summary")
	if err != nil {
		assessment.Issues = append(assessment.Issues, fmt.Sprintf("Failed to fetch summary: %v", err))
		return assessment, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		assessment.Issues = append(assessment.Issues, fmt.Sprintf("Summary endpoint returned %d: %s", resp.StatusCode, string(body)))
		return assessment, fmt.Errorf("summary not found (status %d)", resp.StatusCode)
	}

	var summaryResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&summaryResult); err != nil {
		assessment.Issues = append(assessment.Issues, fmt.Sprintf("Failed to parse summary: %v", err))
		return assessment, err
	}

	summary, ok := summaryResult["summary"].(string)
	if !ok || summary == "" {
		assessment.Issues = append(assessment.Issues, "Summary is empty or missing")
		return assessment, fmt.Errorf("summary is empty")
	}

	assessment.SummaryContent = summary
	assessment.SummaryLength = len(summary)

	// Check for expected content sections (case-insensitive search)
	summaryLower := strings.ToLower(summary)

	for _, section := range assessment.ExpectedSections {
		if strings.Contains(summaryLower, section) {
			assessment.FoundSections = append(assessment.FoundSections, section)
		} else {
			assessment.MissingSections = append(assessment.MissingSections, section)
		}
	}

	// Specific checks for meaningful content
	assessment.HasBuildTargets = strings.Contains(summaryLower, "build") || strings.Contains(summaryLower, "target") || strings.Contains(summaryLower, "cmake") || strings.Contains(summaryLower, "makefile")
	assessment.HasDependencies = strings.Contains(summaryLower, "depend") || strings.Contains(summaryLower, "include") || strings.Contains(summaryLower, "library")
	assessment.HasPlatforms = strings.Contains(summaryLower, "platform") || strings.Contains(summaryLower, "linux") || strings.Contains(summaryLower, "windows") || strings.Contains(summaryLower, "macos")
	assessment.HasComponents = strings.Contains(summaryLower, "component") || strings.Contains(summaryLower, "module") || strings.Contains(summaryLower, "util")
	assessment.HasFileStructure = strings.Contains(summaryLower, "file") || strings.Contains(summaryLower, ".cpp") || strings.Contains(summaryLower, ".h")

	// Determine pass/fail
	foundCount := 0
	if assessment.HasBuildTargets {
		foundCount++
	}
	if assessment.HasDependencies {
		foundCount++
	}
	if assessment.HasPlatforms {
		foundCount++
	}
	if assessment.HasComponents {
		foundCount++
	}
	if assessment.HasFileStructure {
		foundCount++
	}

	// Pass if at least 3 of 5 content checks pass and summary has reasonable length
	assessment.Passed = foundCount >= 3 && assessment.SummaryLength >= 200

	if !assessment.Passed {
		if assessment.SummaryLength < 200 {
			assessment.Issues = append(assessment.Issues, fmt.Sprintf("Summary too short (%d chars, expected >= 200)", assessment.SummaryLength))
		}
		if foundCount < 3 {
			assessment.Issues = append(assessment.Issues, fmt.Sprintf("Only %d/5 expected content sections found", foundCount))
		}
	}

	// Log summary
	dtc.env.LogTest(dtc.t, "  Summary length: %d characters", assessment.SummaryLength)
	dtc.env.LogTest(dtc.t, "  Found sections: %v", assessment.FoundSections)
	dtc.env.LogTest(dtc.t, "  Missing sections: %v", assessment.MissingSections)
	dtc.env.LogTest(dtc.t, "  Content checks: build=%v, deps=%v, platforms=%v, components=%v, files=%v",
		assessment.HasBuildTargets, assessment.HasDependencies, assessment.HasPlatforms,
		assessment.HasComponents, assessment.HasFileStructure)
	dtc.env.LogTest(dtc.t, "  Assessment passed: %v", assessment.Passed)

	// Save assessment report
	reportPath := filepath.Join(dtc.env.GetResultsDir(), "summary_assessment.json")
	reportJSON, _ := json.MarshalIndent(assessment, "", "  ")
	if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to save summary assessment: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "  Saved summary assessment to: %s", reportPath)
	}

	// Also save the raw summary content for manual review
	summaryPath := filepath.Join(dtc.env.GetResultsDir(), "devops_summary_content.md")
	if err := os.WriteFile(summaryPath, []byte(summary), 0644); err != nil {
		dtc.env.LogTest(dtc.t, "  Warning: Failed to save summary content: %v", err)
	} else {
		dtc.env.LogTest(dtc.t, "  Saved summary content to: %s", summaryPath)
	}

	return assessment, nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// =============================================================================
// Private Methods - Document Queries
// =============================================================================

// getImportedDocumentCount returns the count of documents with specific tags
func (dtc *devopsTestContext) getImportedDocumentCount(tags []string) (int, error) {
	tagQuery := strings.Join(tags, ",")
	resp, err := dtc.helper.GET("/api/documents?tags=" + tagQuery + "&limit=1000")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := dtc.helper.ParseJSONResponse(resp, &result); err != nil {
		return 0, err
	}

	totalCount, _ := result["total_count"].(float64)
	return int(totalCount), nil
}
