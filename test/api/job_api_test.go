package api

import (
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
)

// NOTE: Direct job creation endpoints (/api/jobs/create) have been deprecated.
// All job creation now goes through Job Definitions (/api/job-definitions).
//
// For examples of current job creation patterns, see:
// - test/api/job_rerun_test.go - Shows job definition creation and execution
// - Pages can create job definitions with crawl actions
//
// Legacy tests removed as of 2025-10-24:
// - TestCreateJobWithSource
// - TestCreateJobWithRefresh
// - TestCreateJobSourceNotFound
// - TestCreateJobWithoutAuth
// - TestGetJobQueue
// - TestJobSnapshotImmutability
//
// These concepts are now handled at the Job Definition level.

// TestCreateJobValidationFailure tests source creation validation
// Note: Job validation now occurs at job definition level
func TestCreateJobValidationFailure(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestCreateJobValidationFailure")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Create source with invalid configuration (negative concurrency)
	source := map[string]interface{}{
		"name":     "Invalid Source for Job",
		"type":     "jira",
		"base_url": "https://invalid.atlassian.net/jira",
		"enabled":  true,
		"crawl_config": map[string]interface{}{
			"concurrency": -1, // Invalid
		},
	}

	// This should fail during source creation validation
	sourceResp, err := h.POST("/api/sources", source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Source creation should fail with 400
	if sourceResp.StatusCode == http.StatusCreated {
		// If it was created, cleanup
		var sourceResult map[string]interface{}
		h.ParseJSONResponse(sourceResp, &sourceResult)
		sourceID := sourceResult["id"].(string)
		defer h.DELETE("/api/sources/" + sourceID)

		t.Error("Source with invalid config should not be created")
	} else {
		// Source creation correctly failed
		h.AssertStatusCode(sourceResp, http.StatusBadRequest)
	}

	t.Log("✓ Validation correctly prevented invalid source creation")
}
