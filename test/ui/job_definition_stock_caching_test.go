package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobDefinitionStockCaching tests stock job caching behavior via the UI.
// This test verifies that:
// 1. ASX Stocks Daily job can be triggered via the Jobs page UI
// 2. Job executes and monitors successfully via Queue page
// 3. Documents are created with "stock-data" tag
// 4. Running the job a second time uses cached documents (LastSynced unchanged)
// 5. Variables are correctly substituted (no {stock:ticker} placeholders)
//
// Related: test/api/stock_job_caching_test.go tests the same behavior via API
func TestJobDefinitionStockCaching(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: ASX Stocks Daily (Caching Behavior) ---")

	jobName := "ASX Daily Stock Analysis (Multiple Stocks)"
	jobTimeout := 8 * time.Minute // Stock jobs may take longer due to external API calls
	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	// Copy job definition to results for reference
	if err := utc.CopyJobDefinitionToResults("../config/job-definitions/asx-stocks-daily.toml"); err != nil {
		utc.Log("Warning: Could not copy job definition: %v", err)
	}

	// Step 0: Clean up existing documents with stock-data tag
	utc.Log("Step 0: Cleaning up existing stock-data documents")
	cleanupDocumentsByTagUI(t, utc, httpHelper, "stock-data")

	// Step 1: Navigate to Jobs page and trigger the first job
	utc.Log("Step 1: Triggering first job execution via UI")
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger job: %v", err)
	}
	utc.Screenshot("first_job_triggered")

	// Step 2: Navigate to Queue page for monitoring
	utc.Log("Step 2: Monitoring first job on Queue page")
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Monitor first job until completion
	firstJobID := ""
	firstFinalStatus := monitorJobToCompletion(t, utc, jobName, jobTimeout, &firstJobID)
	utc.Screenshot("first_job_completed")

	utc.Log("First job completed with status: %s (job_id: %s)", firstFinalStatus, firstJobID)

	// Step 2b: Verify variable substitution in child jobs
	utc.Log("Step 2b: Verifying variable substitution in child jobs")
	childJobs1 := getChildJobsUI(t, httpHelper, firstJobID)
	variableSubstitutionOK := true
	for _, job := range childJobs1 {
		if name, ok := job["name"].(string); ok {
			if strings.Contains(name, "{stock:") || strings.Contains(name, "{ticker}") {
				utc.Log("✗ Job name contains unsubstituted variable: %s", name)
				variableSubstitutionOK = false
			} else {
				utc.Log("✓ Job name properly substituted: %s", name)
			}
		}
	}
	assert.True(t, variableSubstitutionOK, "All job names should have variables substituted")
	utc.Screenshot("first_job_variable_check")

	// Step 3: Verify documents created with stock-data tag
	utc.Log("Step 3: Verifying documents created by first run")
	docs1 := getDocumentsByTagUI(t, httpHelper, "stock-data")
	require.Greater(t, len(docs1), 0, "First run should create stock-data documents")
	utc.Log("First run created %d documents with stock-data tag", len(docs1))

	// Record LastSynced timestamps from first run
	lastSyncedMap := make(map[string]string)
	for _, doc := range docs1 {
		docID := doc["id"].(string)
		if lastSynced, ok := doc["last_synced"].(string); ok {
			lastSyncedMap[docID] = lastSynced
			utc.Log("Document %s: last_synced=%s", docID, lastSynced)
		}
	}

	// Step 4: Trigger job a second time to test caching (NO cleanup - testing caching)
	utc.Log("Step 4: Triggering second job execution (should use cached documents, NO data cleanup)")
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger second job: %v", err)
	}
	utc.Screenshot("second_job_triggered")

	// Step 5: Monitor second job
	utc.Log("Step 5: Monitoring second job on Queue page")
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to Queue page: %v", err)
	}
	time.Sleep(2 * time.Second)

	secondJobID := ""
	secondFinalStatus := monitorJobToCompletion(t, utc, jobName, jobTimeout, &secondJobID)
	utc.Screenshot("second_job_completed")

	utc.Log("Second job completed with status: %s (job_id: %s)", secondFinalStatus, secondJobID)

	// Step 5b: Verify variable substitution on second run (critical for caching test)
	utc.Log("Step 5b: Verifying variable substitution in second run's child jobs")
	childJobs2 := getChildJobsUI(t, httpHelper, secondJobID)
	variableSubstitutionOK2 := true
	for _, job := range childJobs2 {
		if name, ok := job["name"].(string); ok {
			if strings.Contains(name, "{stock:") || strings.Contains(name, "{ticker}") {
				utc.Log("✗ Second run job name contains unsubstituted variable: %s", name)
				variableSubstitutionOK2 = false
			} else {
				utc.Log("✓ Second run job name properly substituted: %s", name)
			}
		}
	}
	assert.True(t, variableSubstitutionOK2, "Second run: All job names should have variables substituted")
	utc.Screenshot("second_job_variable_check")

	// Step 6: Verify caching - LastSynced timestamps should NOT change
	utc.Log("Step 6: Verifying caching behavior (LastSynced should be unchanged)")
	docs2 := getDocumentsByTagUI(t, httpHelper, "stock-data")
	require.Equal(t, len(docs1), len(docs2), "Document count should be same after second run")

	unchangedCount := 0
	for _, doc := range docs2 {
		docID := doc["id"].(string)
		if newLastSynced, ok := doc["last_synced"].(string); ok {
			if oldLastSynced, exists := lastSyncedMap[docID]; exists {
				if oldLastSynced == newLastSynced {
					unchangedCount++
					utc.Log("✓ Document %s: last_synced unchanged (cache hit)", docID)
				} else {
					utc.Log("✗ Document %s: last_synced changed from %s to %s", docID, oldLastSynced, newLastSynced)
				}
			}
		}
	}

	// Assert that at least one document's LastSynced was unchanged (proving cache was used)
	assert.Greater(t, unchangedCount, 0,
		"At least one document should have unchanged LastSynced (cache hit)")

	utc.Log("✓ Cache verification: %d/%d documents had unchanged LastSynced",
		unchangedCount, len(docs2))

	// Step 7: Cleanup jobs
	utc.Log("Step 7: Cleanup")
	if firstJobID != "" {
		deleteJobUI(t, httpHelper, firstJobID)
	}
	if secondJobID != "" {
		deleteJobUI(t, httpHelper, secondJobID)
	}

	// Final screenshot
	utc.RefreshAndScreenshot("final_state")

	utc.Log("✓ Stock job caching UI test completed successfully")
}

// monitorJobToCompletion monitors a job until it reaches a terminal state
// Returns the final status and captures the job ID
func monitorJobToCompletion(t *testing.T, utc *UITestContext, jobName string, timeout time.Duration, jobID *string) string {
	startTime := time.Now()
	lastStatus := ""
	lastProgressLog := time.Now()
	lastScreenshotTime := time.Now()

	for {
		// Check context
		if err := utc.Ctx.Err(); err != nil {
			t.Fatalf("Context cancelled: %v", err)
		}

		// Check timeout
		if time.Since(startTime) > timeout {
			utc.Screenshot("job_timeout")
			t.Fatalf("Job %s did not complete within %v", jobName, timeout)
		}

		// Log progress every 10 seconds
		if time.Since(lastProgressLog) >= ProgressLogInterval {
			elapsed := time.Since(startTime)
			utc.Log("[%v] Monitoring... (status: %s)", elapsed.Round(time.Second), lastStatus)
			lastProgressLog = time.Now()
		}

		// Take screenshot every 30 seconds
		if time.Since(lastScreenshotTime) >= ScreenshotInterval {
			elapsed := time.Since(startTime)
			utc.FullScreenshot(fmt.Sprintf("monitor_%ds", int(elapsed.Seconds())))
			lastScreenshotTime = time.Now()
		}

		// Get current job status via JavaScript
		var currentStatus string
		err := chromedp.Run(utc.Ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(() => {
					const cards = document.querySelectorAll('.card');
					for (const card of cards) {
						const titleEl = card.querySelector('.card-title');
						if (titleEl && titleEl.textContent.includes('%s')) {
							const statusBadge = card.querySelector('span.label[data-status]');
							if (statusBadge) return statusBadge.getAttribute('data-status');
						}
					}
					return '';
				})()
			`, jobName), &currentStatus),
		)
		if err != nil {
			utc.Log("Warning: failed to get status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Capture job ID once Alpine has loaded the job list
		if *jobID == "" {
			if id, err := getJobIDFromQueueUI(utc, jobName); err == nil && id != "" {
				*jobID = id
				utc.Log("Captured job_id from UI: %s", *jobID)
			}
		}

		// Log status changes
		if currentStatus != lastStatus && currentStatus != "" {
			elapsed := time.Since(startTime)
			utc.Log("Status change: %s -> %s (at %v)", lastStatus, currentStatus, elapsed.Round(time.Second))
			lastStatus = currentStatus
			utc.FullScreenshot(fmt.Sprintf("status_%s", currentStatus))
		}

		// Check for terminal status
		if currentStatus == "completed" || currentStatus == "failed" || currentStatus == "cancelled" {
			utc.Log("Job reached terminal status: %s", currentStatus)
			return currentStatus
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// getDocumentsByTagUI queries documents with a specific tag via API
func getDocumentsByTagUI(t *testing.T, helper httpGetter, tag string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/documents?tag=%s", tag))
	require.NoError(t, err, "Failed to query documents")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET /api/documents?tag=%s returned %d", tag, resp.StatusCode)
		return []map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := parseJSONResponseUI(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse documents response: %v", err)
		return []map[string]interface{}{}
	}

	documents, ok := result["documents"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var docs []map[string]interface{}
	for _, d := range documents {
		if doc, ok := d.(map[string]interface{}); ok {
			docs = append(docs, doc)
		}
	}

	return docs
}

// cleanupDocumentsByTagUI deletes all documents with a specific tag
func cleanupDocumentsByTagUI(t *testing.T, utc *UITestContext, helper httpGetter, tag string) {
	docs := getDocumentsByTagUI(t, helper, tag)
	for _, doc := range docs {
		if id, ok := doc["id"].(string); ok {
			if deleter, ok := helper.(interface {
				DELETE(string) (*http.Response, error)
			}); ok {
				resp, err := deleter.DELETE(fmt.Sprintf("/api/documents/%s", id))
				if err == nil {
					resp.Body.Close()
					utc.Log("Deleted document: %s", id)
				}
			}
		}
	}
}

// deleteJobUI deletes a job via API
func deleteJobUI(t *testing.T, helper httpGetter, jobID string) {
	if deleter, ok := helper.(interface {
		DELETE(string) (*http.Response, error)
	}); ok {
		resp, err := deleter.DELETE(fmt.Sprintf("/api/jobs/%s", jobID))
		if err == nil {
			resp.Body.Close()
			t.Logf("Deleted job: %s", jobID)
		}
	}
}

// parseJSONResponseUI parses a JSON response body
func parseJSONResponseUI(resp *http.Response, dest interface{}) error {
	return json.NewDecoder(resp.Body).Decode(dest)
}

// getChildJobsUI retrieves child jobs of a parent job via API
func getChildJobsUI(t *testing.T, helper httpGetter, parentJobID string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/jobs/%s/children", parentJobID))
	if err != nil {
		t.Logf("Warning: Failed to get child jobs: %v", err)
		return []map[string]interface{}{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Warning: GET /api/jobs/%s/children returned %d", parentJobID, resp.StatusCode)
		return []map[string]interface{}{}
	}

	var result map[string]interface{}
	if err := parseJSONResponseUI(resp, &result); err != nil {
		t.Logf("Warning: Failed to parse child jobs response: %v", err)
		return []map[string]interface{}{}
	}

	jobs, ok := result["jobs"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}

	var childJobs []map[string]interface{}
	for _, j := range jobs {
		if job, ok := j.(map[string]interface{}); ok {
			childJobs = append(childJobs, job)
		}
	}

	return childJobs
}
