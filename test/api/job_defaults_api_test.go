// -----------------------------------------------------------------------
// API test for default job definitions created on service startup
// Tests the GET /api/job-definitions endpoint
// -----------------------------------------------------------------------

package api

import (
	"net/http"
	"testing"

	"github.com/ternarybob/quaero/test/common"
)

// TestJobDefaultDefinitionsAPI verifies that 2 default job definitions are returned by the API
func TestJobDefaultDefinitionsAPI(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefaultDefinitionsAPI")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Step 1: Get list of job definitions
	env.LogTest(t, "Fetching job definitions from API")
	resp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to get job definitions: %v", err)
	}

	// Verify response status
	h.AssertStatusCode(resp, http.StatusOK)

	// Parse response - API now returns object with job_definitions array
	var response map[string]interface{}
	if err := h.ParseJSONResponse(resp, &response); err != nil {
		t.Fatalf("Failed to parse job definitions response: %v", err)
	}

	// Extract job_definitions array from response
	jobDefsInterface, ok := response["job_definitions"]
	if !ok {
		t.Fatalf("Response missing 'job_definitions' field")
	}

	jobDefsArray, ok := jobDefsInterface.([]interface{})
	if !ok {
		t.Fatalf("'job_definitions' is not an array")
	}

	// Convert to []map[string]interface{}
	jobDefs := make([]map[string]interface{}, len(jobDefsArray))
	for i, item := range jobDefsArray {
		jobDef, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("Job definition at index %d is not an object", i)
		}
		jobDefs[i] = jobDef
	}

	env.LogTest(t, "Found %d job definitions", len(jobDefs))

	// REQUIREMENT: There should be exactly 2 default job definitions
	if len(jobDefs) != 2 {
		t.Fatalf("REQUIREMENT FAILED: Expected 2 default job definitions, got %d", len(jobDefs))
	}

	// Verify job definition names
	var foundDbMaintenance bool
	var foundCorpusSummary bool

	for _, jobDef := range jobDefs {
		name, ok := jobDef["name"].(string)
		if !ok {
			t.Errorf("Job definition missing 'name' field: %v", jobDef)
			continue
		}

		env.LogTest(t, "Found job definition: %s", name)

		switch name {
		case "Database Maintenance":
			foundDbMaintenance = true
			env.LogTest(t, "✓ Found 'Database Maintenance' job definition")
		case "Corpus Summary Generation":
			foundCorpusSummary = true
			env.LogTest(t, "✓ Found 'Corpus Summary Generation' job definition")
		}
	}

	// REQUIREMENT: Both default job definitions must be present
	if !foundDbMaintenance {
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job definition not found")
	}

	if !foundCorpusSummary {
		t.Error("REQUIREMENT FAILED: 'Corpus Summary Generation' job definition not found")
	}

	if !foundDbMaintenance || !foundCorpusSummary {
		t.Fatal("REQUIREMENT FAILED: Not all default job definitions are present")
	}

	env.LogTest(t, "✅ All default job definitions verified successfully via API")
}

// TestJobDefinitionsResponseFormat verifies the structure of job definition responses
func TestJobDefinitionsResponseFormat(t *testing.T) {
	env, err := common.SetupTestEnvironment("TestJobDefinitionsResponseFormat")
	if err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	h := env.NewHTTPTestHelper(t)

	// Get job definitions
	resp, err := h.GET("/api/job-definitions")
	if err != nil {
		t.Fatalf("Failed to get job definitions: %v", err)
	}

	h.AssertStatusCode(resp, http.StatusOK)

	// Parse response - API now returns object with job_definitions array
	var response map[string]interface{}
	if err := h.ParseJSONResponse(resp, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Extract job_definitions array from response
	jobDefsInterface, ok := response["job_definitions"]
	if !ok {
		t.Fatalf("Response missing 'job_definitions' field")
	}

	jobDefsArray, ok := jobDefsInterface.([]interface{})
	if !ok {
		t.Fatalf("'job_definitions' is not an array")
	}

	if len(jobDefsArray) == 0 {
		t.Fatal("No job definitions returned")
	}

	// Verify first job definition has required fields
	jobDefInterface := jobDefsArray[0]
	jobDef, ok := jobDefInterface.(map[string]interface{})
	if !ok {
		t.Fatalf("First job definition is not an object")
	}
	requiredFields := []string{"id", "name", "type", "enabled", "created_at"}

	for _, field := range requiredFields {
		if _, exists := jobDef[field]; !exists {
			t.Errorf("Job definition missing required field: %s", field)
		}
	}

	env.LogTest(t, "✓ Job definition response format verified")
}
