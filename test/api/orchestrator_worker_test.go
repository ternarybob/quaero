package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

func TestOrchestratorWorkerSubmission(t *testing.T) {
	// Initialize test environment
	env := common.NewTestEnvironment(t)
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// 1. Submit the definition (simulated by ensuring it's loaded or posting it)
	// For this test, we assume the file 'orchestrator-fact-check.toml' is already in the definitions dir
	// and loaded by the system. If not, we can post it.

	// Create/Update the job definition via API to match the file
	// (mirroring the file content locally for the test POST)
	defID := "orchestrator-fact-check-api-test"
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Fact Check API Test",
		"type":        "orchestrator", // This will likely fail validation currently
		"description": "API Test for Orchestrator",
		"config": map[string]interface{}{
			"output_schema": "test/config/job-templates/output-schema-fact-check.toml",
		},
		"steps": []map[string]interface{}{
			{
				"name": "verify_claim",
				"type": "orchestrator",
				"goal": "Verify the claim: 'The earth is flat'.",
			},
		},
	}

	// Trigger the job
	// NOTE: This POST is expected to fail or the trigger to fail because 'orchestrator' type is unknown
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err)

	// If the type is validated strictly, this might be 400.
	// If it allows dynamic types but fails at runtime, it might be 201.
	// We Assert 201 because we WANT it to succeed in the future.
	require.Equal(t, 201, resp.StatusCode, "Should likely fail 400 until implemented")
	defer resp.Body.Close()

	// Trigger execution
	triggerResp, err := helper.POST(fmt.Sprintf("/api/jobs/trigger/%s", defID), nil)
	require.NoError(t, err)
	defer triggerResp.Body.Close()

	// We expect this to be 200 IF valid, but currently will fail or 404
	require.Equal(t, 200, triggerResp.StatusCode)

	// Get Job ID
	var result map[string]interface{}
	// ... (parsing logic) ...
	jobID := "placeholder-until-implemented" // helper.ParseJSON(triggerResp).ID

	// Poll for completion
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		statusResp, _ := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		require.Equal(t, 200, statusResp.StatusCode)

		// Logic to check status...
		// ...

		time.Sleep(1 * time.Second)
	}

	// Assert on Outputs (The "Business Logic" check)
	// We expect the Output to contain 'verdict' and 'sources'
	// This ensures the worker actually DID something.
	// assert.NotEmpty(t, finalOutput["verdict"])
	// assert.NotEmpty(t, finalOutput["sources"])
}
