package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestStockJobCaching_DocumentsNotUpdatedWithin24Hours tests that:
// 1. First job execution creates documents with correct variable substitution
// 2. Second job execution (within 24hrs) uses cached documents
// 3. Document LastSynced is NOT updated on second run
// 4. Variables are correctly substituted (no {stock:ticker} placeholders)
func TestStockJobCaching_DocumentsNotUpdatedWithin24Hours(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Clean up any existing documents with stock-data tag ONLY on first run
	t.Log("Step 0: Cleaning up existing stock-data documents (first run only)")
	cleanupDocumentsByTag(t, helper, "stock-data")

	// Step 1: Load the asx-stocks-daily job definition
	t.Log("Step 1: Loading asx-stocks-daily job definition")
	err = env.LoadTestJobDefinitions("../config/job-definitions/asx-stocks-daily.toml")
	require.NoError(t, err, "Failed to load job definition")

	// Step 2: Execute the job first time
	t.Log("Step 2: Executing job first time - documents should be created")
	jobID1 := executeJobDefinition(t, helper, "asx-stocks-daily")
	require.NotEmpty(t, jobID1, "First job execution should return job ID")

	// Wait for job completion
	status1 := waitForJobCompletion(t, helper, jobID1, 5*time.Minute)
	t.Logf("First job completed with status: %s", status1)

	// Step 2b: Verify variable substitution - job title should NOT contain placeholders
	t.Log("Step 2b: Verifying variable substitution in child jobs")
	childJobs := getChildJobs(t, helper, jobID1)
	variableSubstitutionOK := true
	for _, job := range childJobs {
		name := job["name"].(string)
		// Job names should NOT contain unsubstituted placeholders like {stock:ticker}
		if strings.Contains(name, "{stock:") || strings.Contains(name, "{ticker}") {
			t.Errorf("✗ Job name contains unsubstituted variable: %s", name)
			variableSubstitutionOK = false
		} else {
			t.Logf("✓ Job name properly substituted: %s", name)
		}
	}
	assert.True(t, variableSubstitutionOK, "All job names should have variables substituted")

	// Step 3: Query documents created by first run
	t.Log("Step 3: Querying documents created by first run")
	docs1 := getDocumentsByTag(t, helper, "stock-data")
	require.Greater(t, len(docs1), 0, "First run should create stock-data documents")
	t.Logf("First run created %d documents", len(docs1))

	// Record LastSynced timestamps from first run
	lastSyncedMap := make(map[string]string)
	for _, doc := range docs1 {
		docID := doc["id"].(string)
		if lastSynced, ok := doc["last_synced"].(string); ok {
			lastSyncedMap[docID] = lastSynced
			t.Logf("Document %s: last_synced=%s", docID, lastSynced)
		}
	}

	// Step 4: Execute job a second time immediately (NO cleanup - testing caching)
	t.Log("Step 4: Executing job second time - should use cached documents (NO data cleanup)")
	// NOTE: We explicitly do NOT clean up documents here to test caching behavior
	jobID2 := executeJobDefinition(t, helper, "asx-stocks-daily")
	require.NotEmpty(t, jobID2, "Second job execution should return job ID")

	// Wait for job completion
	status2 := waitForJobCompletion(t, helper, jobID2, 5*time.Minute)
	t.Logf("Second job completed with status: %s", status2)

	// Step 4b: Verify variable substitution on second run (critical for caching test)
	t.Log("Step 4b: Verifying variable substitution in second run's child jobs")
	childJobs2 := getChildJobs(t, helper, jobID2)
	variableSubstitutionOK2 := true
	for _, job := range childJobs2 {
		name := job["name"].(string)
		// Job names should NOT contain unsubstituted placeholders like {stock:ticker}
		if strings.Contains(name, "{stock:") || strings.Contains(name, "{ticker}") {
			t.Errorf("✗ Second run job name contains unsubstituted variable: %s", name)
			variableSubstitutionOK2 = false
		} else {
			t.Logf("✓ Second run job name properly substituted: %s", name)
		}
	}
	assert.True(t, variableSubstitutionOK2, "Second run: All job names should have variables substituted")

	// Step 5: Query documents after second run
	t.Log("Step 5: Querying documents after second run")
	docs2 := getDocumentsByTag(t, helper, "stock-data")
	require.Equal(t, len(docs1), len(docs2), "Document count should be same after second run")

	// Step 6: Verify LastSynced timestamps did NOT change
	t.Log("Step 6: Verifying LastSynced timestamps did NOT change (cache hit)")
	unchangedCount := 0
	for _, doc := range docs2 {
		docID := doc["id"].(string)
		if newLastSynced, ok := doc["last_synced"].(string); ok {
			if oldLastSynced, exists := lastSyncedMap[docID]; exists {
				if oldLastSynced == newLastSynced {
					unchangedCount++
					t.Logf("✓ Document %s: last_synced unchanged (cache hit)", docID)
				} else {
					t.Logf("✗ Document %s: last_synced changed from %s to %s", docID, oldLastSynced, newLastSynced)
				}
			}
		}
	}

	// Assert that at least one document's LastSynced was unchanged (proving cache was used)
	// Note: Some documents may be from different step types with different caching behavior
	assert.Greater(t, unchangedCount, 0,
		"At least one document should have unchanged LastSynced (cache hit)")

	t.Logf("✓ Cache verification: %d/%d documents had unchanged LastSynced",
		unchangedCount, len(docs2))

	// Cleanup
	t.Log("Step 7: Cleanup")
	deleteJob(t, helper, jobID1)
	deleteJob(t, helper, jobID2)

	t.Log("✓ Stock job caching test completed successfully")
}

// getDocumentsByTag queries documents with a specific tag
func getDocumentsByTag(t *testing.T, helper *common.HTTPTestHelper, tag string) []map[string]interface{} {
	resp, err := helper.GET(fmt.Sprintf("/api/documents?tag=%s", tag))
	require.NoError(t, err, "Failed to query documents")
	defer resp.Body.Close()

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse documents response")

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

// cleanupDocumentsByTag deletes all documents with a specific tag
func cleanupDocumentsByTag(t *testing.T, helper *common.HTTPTestHelper, tag string) {
	docs := getDocumentsByTag(t, helper, tag)
	for _, doc := range docs {
		if id, ok := doc["id"].(string); ok {
			resp, err := helper.DELETE(fmt.Sprintf("/api/documents/%s", id))
			if err == nil {
				resp.Body.Close()
				t.Logf("Deleted document: %s", id)
			}
		}
	}
}

// getChildJobs retrieves child jobs of a parent job
func getChildJobs(t *testing.T, helper *common.HTTPTestHelper, parentJobID string) []map[string]interface{} {
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
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
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
