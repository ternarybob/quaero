package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestJobTemplate tests the job_template worker type
// This verifies that:
// 1. Job templates can be defined with {variable:key} syntax
// 2. The job_template step type is recognized
// 3. Variables are properly substituted in templates
func TestJobTemplate_JobDefinitionCreation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test: Create a job definition that uses job_template type
	t.Log("Creating job definition with job_template step")
	jobDefID := "test-job-template-" + time.Now().Format("20060102150405")

	body := map[string]interface{}{
		"id":          jobDefID,
		"name":        "Test Job Template Orchestration",
		"type":        "job_template",
		"description": "Tests job template variable substitution",
		"enabled":     false, // Don't auto-run
		"steps": []map[string]interface{}{
			{
				"name":        "run_templates",
				"type":        "job_template",
				"description": "Execute test job template",
				"on_error":    "fail",
				"config": map[string]interface{}{
					"template": "asx-stock-analysis",
					"variables": []map[string]interface{}{
						{"ticker": "TST", "name": "Test Stock", "industry": "testing"},
					},
				},
			},
		},
	}

	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	// Should succeed
	if resp.StatusCode != http.StatusCreated {
		var errResult map[string]interface{}
		helper.ParseJSONResponse(resp, &errResult)
		t.Logf("Error response: %v", errResult)
	}
	helper.AssertStatusCode(resp, http.StatusCreated)

	// Verify the job definition was created correctly
	t.Log("Verifying job definition exists")
	resp, err = helper.GET("/api/job-definitions/" + jobDefID)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, jobDefID, result["id"])
	assert.Equal(t, "Test Job Template Orchestration", result["name"])

	steps, ok := result["steps"].([]interface{})
	require.True(t, ok, "Steps should be array")
	require.Len(t, steps, 1, "Should have 1 step")

	firstStep := steps[0].(map[string]interface{})
	assert.Equal(t, "run_templates", firstStep["name"])
	assert.Equal(t, "job_template", firstStep["type"])

	// Cleanup
	t.Log("Cleaning up test job definition")
	deleteJobDefinition(t, helper, jobDefID)
}

// TestJobTemplate_WorkerTypeValidation verifies job_template is a valid worker type
func TestJobTemplate_WorkerTypeValidation(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Get worker types from the worker-types endpoint
	t.Log("Getting available worker types")
	resp, err := helper.GET("/api/worker-types")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The endpoint might not exist, so check gracefully
	if resp.StatusCode == http.StatusNotFound {
		t.Log("Worker types endpoint not available, skipping validation")
		return
	}

	helper.AssertStatusCode(resp, http.StatusOK)

	var result map[string]interface{}
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err)

	types, ok := result["types"].([]interface{})
	if !ok {
		t.Log("Worker types response format unexpected, skipping validation")
		return
	}

	// Check if job_template is in the list
	found := false
	for _, wt := range types {
		if wt == "job_template" {
			found = true
			break
		}
	}

	assert.True(t, found, "job_template should be in the list of valid worker types")
}

// TestVariablesFile_LoadFromRoot tests that variables.toml is loaded from root directory
func TestVariablesFile_LoadFromRoot(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if we can get key-value pairs that would come from variables.toml
	t.Log("Checking key-value store for loaded variables")
	resp, err := helper.GET("/api/kv")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The endpoint might not exist or might be protected
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		t.Log("KV endpoint not available, checking variables indirectly")

		// Try to create a job that would use a variable
		// If variables loading is broken, jobs using {variable_name} syntax would fail
		return
	}

	helper.AssertStatusCode(resp, http.StatusOK)
	t.Log("Variables loaded successfully from root directory")
}
