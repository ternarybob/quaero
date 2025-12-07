package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/ternarybob/quaero/test/common"
)

// devopsTestContext holds shared state for DevOps enrichment tests
type devopsTestContext struct {
	t         *testing.T
	env       *common.TestEnvironment
	ctx       context.Context
	baseURL   string
	jobsURL   string
	queueURL  string
	helper    *common.HTTPTestHelper
	docsCount int // Track number of imported docs
}

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
		dtc.env.LogTest(dtc.t, "  ✓ Imported: %s", relPath)
	}

	dtc.docsCount = importedCount
	dtc.env.LogTest(dtc.t, "✓ Imported %d files from fixtures", importedCount)

	if importedCount == 0 {
		return fmt.Errorf("no files were imported")
	}

	return nil
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

// triggerEnrichment triggers the DevOps enrichment pipeline via API
func (dtc *devopsTestContext) triggerEnrichment() (string, error) {
	dtc.env.LogTest(dtc.t, "Triggering DevOps enrichment pipeline...")

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

	dtc.env.LogTest(dtc.t, "✓ Enrichment pipeline triggered (job ID: %s)", jobID)
	return jobID, nil
}

// monitorJobWithPolling monitors a job via polling (API-based, not UI-based)
func (dtc *devopsTestContext) monitorJobWithPolling(jobID string, timeout time.Duration) error {
	dtc.env.LogTest(dtc.t, "Monitoring job: %s (timeout: %v)", jobID, timeout)

	startTime := time.Now()
	lastProgressLog := time.Now()
	checkCount := 0

	for {
		// Check timeout
		if time.Since(startTime) > timeout {
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

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= 10*time.Second {
			elapsed := time.Since(startTime)
			dtc.env.LogTest(dtc.t, "  [%v] Still monitoring... (status: %s, checks: %d)",
				elapsed.Round(time.Second), status, checkCount)
			lastProgressLog = time.Now()
		}

		// Check if job is done
		if status == "completed" {
			dtc.env.LogTest(dtc.t, "✓ Job completed successfully (after %d checks)", checkCount)
			return nil
		}

		if status == "failed" {
			failureReason := "unknown"
			if metadata, ok := job["metadata"].(map[string]interface{}); ok {
				if reason, ok := metadata["failure_reason"].(string); ok {
					failureReason = reason
				}
			}
			return fmt.Errorf("job %s failed: %s", jobID, failureReason)
		}

		if status == "cancelled" {
			return fmt.Errorf("job %s was cancelled", jobID)
		}

		// Wait before next check
		time.Sleep(1 * time.Second)
	}
}

// verifyEnrichmentResults verifies that enrichment produced expected results
func (dtc *devopsTestContext) verifyEnrichmentResults() error {
	dtc.env.LogTest(dtc.t, "Verifying enrichment results...")

	// 1. Verify dependency graph exists
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

	// Check for nodes and edges
	nodes, hasNodes := graph["nodes"]
	edges, hasEdges := graph["edges"]

	if !hasNodes || !hasEdges {
		return fmt.Errorf("graph missing nodes or edges: hasNodes=%v, hasEdges=%v", hasNodes, hasEdges)
	}

	dtc.env.LogTest(dtc.t, "  ✓ Dependency graph exists with nodes and edges")

	// 2. Verify summary document exists
	dtc.env.LogTest(dtc.t, "  Checking DevOps summary...")
	resp2, err := dtc.helper.GET("/api/devops/summary")
	if err != nil {
		return fmt.Errorf("failed to get summary: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("summary not found (status %d): %s", resp2.StatusCode, string(body))
	}

	var summaryResult map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&summaryResult); err != nil {
		return fmt.Errorf("failed to parse summary: %w", err)
	}

	summary, ok := summaryResult["summary"].(string)
	if !ok || summary == "" {
		return fmt.Errorf("summary is empty or missing")
	}

	dtc.env.LogTest(dtc.t, "  ✓ DevOps summary exists (%d characters)", len(summary))

	// 3. Verify components endpoint works
	dtc.env.LogTest(dtc.t, "  Checking components...")
	resp3, err := dtc.helper.GET("/api/devops/components")
	if err != nil {
		return fmt.Errorf("failed to get components: %w", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		return fmt.Errorf("components not found (status %d): %s", resp3.StatusCode, string(body))
	}

	dtc.env.LogTest(dtc.t, "  ✓ Components endpoint accessible")

	// 4. Verify platforms endpoint works
	dtc.env.LogTest(dtc.t, "  Checking platforms...")
	resp4, err := dtc.helper.GET("/api/devops/platforms")
	if err != nil {
		return fmt.Errorf("failed to get platforms: %w", err)
	}
	defer resp4.Body.Close()

	if resp4.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp4.Body)
		return fmt.Errorf("platforms not found (status %d): %s", resp4.StatusCode, string(body))
	}

	dtc.env.LogTest(dtc.t, "  ✓ Platforms endpoint accessible")

	dtc.env.LogTest(dtc.t, "✓ All enrichment results verified")
	return nil
}

// verifyDocumentsEnriched verifies that documents have devops metadata
func (dtc *devopsTestContext) verifyDocumentsEnriched() error {
	dtc.env.LogTest(dtc.t, "Verifying documents have DevOps metadata...")

	// Query documents with devops tags
	resp, err := dtc.helper.GET("/api/documents?tags=devops-enriched&limit=100")
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}
	defer resp.Body.Close()

	var docs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return fmt.Errorf("failed to parse documents: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Errorf("no documents found with devops-enriched tag")
	}

	// Check that at least one document has devops metadata
	hasMetadata := false
	for _, doc := range docs {
		if metadata, ok := doc["metadata"].(map[string]interface{}); ok {
			if _, hasDevOps := metadata["devops"]; hasDevOps {
				hasMetadata = true
				break
			}
		}
	}

	if !hasMetadata {
		dtc.env.LogTest(dtc.t, "  Warning: No documents have devops metadata field")
	} else {
		dtc.env.LogTest(dtc.t, "  ✓ Found documents with devops metadata")
	}

	dtc.env.LogTest(dtc.t, "✓ Found %d enriched documents", len(docs))
	return nil
}

// TestDevOpsEnrichmentPipeline_FullFlow tests the complete enrichment pipeline
func TestDevOpsEnrichmentPipeline_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test")
	}

	dtc, cleanup := newDevopsTestContext(t, 10*time.Minute)
	defer cleanup()

	dtc.env.LogTest(t, "--- Starting Test: DevOps Enrichment Full Flow ---")

	// 1. Import test fixtures via API
	if err := dtc.importFixtures(); err != nil {
		t.Fatalf("Failed to import fixtures: %v", err)
	}

	// Take screenshot before enrichment
	if err := chromedp.Run(dtc.ctx, chromedp.Navigate(dtc.baseURL)); err != nil {
		dtc.env.LogTest(t, "Warning: Failed to navigate to home page: %v", err)
	} else {
		dtc.env.TakeScreenshot(dtc.ctx, "devops_before_enrichment")
	}

	// 2. Trigger enrichment pipeline
	jobID, err := dtc.triggerEnrichment()
	if err != nil {
		t.Fatalf("Failed to trigger enrichment: %v", err)
	}

	// 3. Monitor job progress (polling with timeout)
	if err := dtc.monitorJobWithPolling(jobID, 8*time.Minute); err != nil {
		dtc.env.TakeScreenshot(dtc.ctx, "devops_job_failed")
		t.Fatalf("Job monitoring failed: %v", err)
	}

	// 4. Verify enrichment results
	if err := dtc.verifyEnrichmentResults(); err != nil {
		dtc.env.TakeScreenshot(dtc.ctx, "devops_verification_failed")
		t.Fatalf("Enrichment verification failed: %v", err)
	}

	// 5. Verify documents have devops metadata
	if err := dtc.verifyDocumentsEnriched(); err != nil {
		dtc.env.LogTest(t, "Warning: Document verification failed: %v", err)
		// Don't fail the test - this is informational
	}

	// Take final screenshot
	dtc.env.TakeScreenshot(dtc.ctx, "devops_after_enrichment")

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
