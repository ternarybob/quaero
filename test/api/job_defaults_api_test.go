// -----------------------------------------------------------------------
// API test for default job definitions created on service startup
// Tests the GET /api/job-definitions endpoint
// -----------------------------------------------------------------------

package api

import (
	"github.com/ternarybob/quaero/test/common"
	"net/http"
	"testing"
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

	// Parse response
	var jobDefs []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &jobDefs); err != nil {
		t.Fatalf("Failed to parse job definitions response: %v", err)
	}

	env.LogTest(t, "Found %d job definitions", len(jobDefs))

	// REQUIREMENT: There should be exactly 2 default job definitions
	if len(jobDefs) != 2 {
		t.Fatalf("REQUIREMENT FAILED: Expected 2 default job definitions, got %d", len(jobDefs))
	}

	// Verify job definition names
	var foundDbMaintenance bool
	var foundSystemHealth bool

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
		case "System Health Check":
			foundSystemHealth = true
			env.LogTest(t, "✓ Found 'System Health Check' job definition")
		}
	}

	// REQUIREMENT: Both default job definitions must be present
	if !foundDbMaintenance {
		t.Error("REQUIREMENT FAILED: 'Database Maintenance' job definition not found")
	}

	if !foundSystemHealth {
		t.Error("REQUIREMENT FAILED: 'System Health Check' job definition not found")
	}

	if !foundDbMaintenance || !foundSystemHealth {
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

	var jobDefs []map[string]interface{}
	if err := h.ParseJSONResponse(resp, &jobDefs); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(jobDefs) == 0 {
		t.Fatal("No job definitions returned")
	}

	// Verify first job definition has required fields
	jobDef := jobDefs[0]
	requiredFields := []string{"id", "name", "type", "enabled", "created_at"}

	for _, field := range requiredFields {
		if _, exists := jobDef[field]; !exists {
			t.Errorf("Job definition missing required field: %s", field)
		}
	}

	env.LogTest(t, "✓ Job definition response format verified")
}
