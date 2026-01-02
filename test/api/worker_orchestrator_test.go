package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// TestOrchestratorWorkerSubmission tests that orchestrator job definitions can be created
// and triggered via the API. This validates:
// 1. Job definition with type "orchestrator" is accepted
// 2. Step config with "goal" field passes validation
// 3. Job can be triggered and returns a valid job ID
func TestOrchestratorWorkerSubmission(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create job definition with orchestrator type
	defID := "orchestrator-test-" + time.Now().Format("20060102150405")
	body := map[string]interface{}{
		"id":          defID,
		"name":        "Orchestrator API Test",
		"type":        "orchestrator",
		"description": "API test for OrchestratorWorker",
		"enabled":     true,
		"steps": []map[string]interface{}{
			{
				"name":        "verify_claim",
				"type":        "orchestrator",
				"description": "Verify a test claim using LLM reasoning",
				"on_error":    "fail",
				"config": map[string]interface{}{
					"goal":           "Verify the claim: 'Water boils at 100 degrees Celsius at sea level'.",
					"thinking_level": "MEDIUM",
				},
			},
		},
	}

	t.Log("Creating orchestrator job definition")
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to POST job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResult map[string]interface{}
		helper.ParseJSONResponse(resp, &errResult)
		t.Logf("Error response: %v", errResult)
	}
	helper.AssertStatusCode(resp, http.StatusCreated)

	// Cleanup job definition after test
	defer helper.DELETE(fmt.Sprintf("/api/job-definitions/%s", defID))

	// Verify job definition was created
	t.Log("Verifying job definition exists")
	getResp, err := helper.GET("/api/job-definitions/" + defID)
	require.NoError(t, err)
	defer getResp.Body.Close()
	helper.AssertStatusCode(getResp, http.StatusOK)

	var jobDef map[string]interface{}
	err = helper.ParseJSONResponse(getResp, &jobDef)
	require.NoError(t, err)
	assert.Equal(t, defID, jobDef["id"])
	assert.Equal(t, "orchestrator", jobDef["type"])

	// Verify steps contain orchestrator config with goal
	steps, ok := jobDef["steps"].([]interface{})
	require.True(t, ok, "Steps should be array")
	require.Len(t, steps, 1, "Should have 1 step")

	firstStep := steps[0].(map[string]interface{})
	assert.Equal(t, "verify_claim", firstStep["name"])
	assert.Equal(t, "orchestrator", firstStep["type"])

	stepConfig, ok := firstStep["config"].(map[string]interface{})
	require.True(t, ok, "Step config should exist")
	assert.NotEmpty(t, stepConfig["goal"], "Goal should be set")

	// Trigger job execution
	t.Log("Triggering orchestrator job")
	triggerResp, err := helper.POST(fmt.Sprintf("/api/job-definitions/%s/execute", defID), nil)
	require.NoError(t, err)
	defer triggerResp.Body.Close()
	helper.AssertStatusCode(triggerResp, http.StatusAccepted)

	// Parse job ID from trigger response
	var triggerResult map[string]interface{}
	err = helper.ParseJSONResponse(triggerResp, &triggerResult)
	require.NoError(t, err)

	jobID, ok := triggerResult["job_id"].(string)
	require.True(t, ok, "Trigger response should contain job ID")
	require.NotEmpty(t, jobID, "Job ID should not be empty")
	t.Logf("Triggered job ID: %s", jobID)

	// Poll for job completion (with timeout)
	t.Log("Polling for job completion")
	var finalStatus string
	maxRetries := 60 // 60 * 500ms = 30 seconds max wait
	for i := 0; i < maxRetries; i++ {
		statusResp, err := helper.GET(fmt.Sprintf("/api/jobs/%s", jobID))
		require.NoError(t, err)

		var jobStatus map[string]interface{}
		err = helper.ParseJSONResponse(statusResp, &jobStatus)
		statusResp.Body.Close()
		require.NoError(t, err)

		status, _ := jobStatus["status"].(string)
		if status == "completed" || status == "failed" {
			finalStatus = status
			break
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Verify job reached a terminal state (completed or failed)
	// The orchestrator may fail if no tools are available, but the API test validates
	// that the job was successfully triggered and executed
	assert.True(t, finalStatus == "completed" || finalStatus == "failed", "Job should reach terminal state, got: "+finalStatus)
	t.Logf("Final job status: %s", finalStatus)
}
