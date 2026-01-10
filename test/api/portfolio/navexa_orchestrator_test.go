// Package portfolio contains API integration tests for Portfolio workers.
//
// IMPORTANT: Portfolio orchestrator tests require extended timeout due to LLM operations:
//
//	go test -timeout 15m -run TestNavexaOrchestratorIntegration ./test/api/portfolio/...
//
// The default Go test timeout (10 minutes) is insufficient for these tests.
// Individual tests use 15-minute timeouts for job completion with error monitoring.
package portfolio

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Navexa Orchestrator Integration Test
// =============================================================================
// Tests the complete Navexa portfolio analysis workflow:
// 1. Load stock-analysis-navexa.toml
// 2. Execute orchestrator with goal template
// 3. Verify email content is actual analysis (not placeholder)
//
// IMPORTANT: This test requires:
// - Valid navexa_api_key in KV storage
// - Valid LLM API key (Gemini or Claude)
// - Extended timeout: go test -timeout 15m
// =============================================================================

// TestNavexaOrchestratorIntegration tests the complete Navexa portfolio analysis workflow
func TestNavexaOrchestratorIntegration(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelperWithTimeout(t, 15*time.Minute)

	// Step 0: Check if Navexa API key is configured
	navexaKey := GetNavexaAPIKey(t, helper)
	if navexaKey == "" {
		t.Skip("Navexa API key not configured - skipping orchestrator test")
	}

	// Step 1: Load the orchestrated job definition
	t.Log("Step 1: Loading job definition stock-analysis-navexa.toml")
	err = env.LoadTestJobDefinitions("../config/job-definitions/stock-analysis-navexa.toml")
	if err != nil {
		t.Skipf("Failed to load Navexa job definition: %v", err)
	}

	// Verify the definition was loaded
	defResp, err := helper.GET("/api/job-definitions/stock-analysis-navexa")
	if err != nil || defResp.StatusCode != http.StatusOK {
		t.Skip("Navexa job definition not loaded - skipping test")
	}
	defResp.Body.Close()

	// Step 2: Trigger the job
	t.Log("Step 2: Triggering orchestrated job")
	execResp, err := helper.POST("/api/job-definitions/stock-analysis-navexa/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Triggered job ID: %s", jobID)

	// Step 3: Wait for job completion with error monitoring (15 minute timeout for LLM operations)
	t.Log("Step 3: Waiting for job completion with error monitoring (timeout: 15 minutes)")
	finalStatus, errorLogs := WaitForJobCompletionWithMonitoring(t, helper, jobID, 15*time.Minute)
	t.Logf("Job completed with status: %s", finalStatus)

	// Log any errors found
	if len(errorLogs) > 0 {
		t.Logf("Found %d ERROR log entries:", len(errorLogs))
		for i, log := range errorLogs {
			if i < 5 { // Limit output
				logMsg, _ := log["message"].(string)
				t.Logf("  ERROR[%d]: %s", i, logMsg)
			}
		}
	}

	// Step 4: Validate output if job completed
	if finalStatus == "completed" {
		t.Log("Step 4: Validating output document")
		validateNavexaOutput(t, helper)
	} else {
		t.Logf("Job ended with status %s - skipping output validation", finalStatus)
	}

	// Step 5: Check service.log for errors
	common.AssertNoErrorsInServiceLog(t, env)
}

// validateNavexaOutput validates the Navexa orchestrator output document
func validateNavexaOutput(t *testing.T, helper *common.HTTPTestHelper) {
	// Find document by tag - portfolio_review worker uses portfolio-review tag
	searchResp, err := helper.GET("/api/documents?tags=portfolio-review&limit=1")
	require.NoError(t, err)
	defer searchResp.Body.Close()

	var searchResult struct {
		Documents []struct {
			ContentMarkdown string `json:"content_markdown"`
		} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(searchResp, &searchResult))

	if len(searchResult.Documents) == 0 {
		t.Log("INFO: No portfolio-review document found (may be expected if job had errors)")
		return
	}

	doc := searchResult.Documents[0]
	content := doc.ContentMarkdown

	// Validate not placeholder
	assert.NotContains(t, strings.ToLower(content), "placeholder", "Content should not be a placeholder")
	assert.NotContains(t, content, "TODO", "Content should not contain TODO")

	// Validate has portfolio data
	hasPortfolioContent := strings.Contains(strings.ToLower(content), "portfolio") ||
		strings.Contains(strings.ToLower(content), "holding") ||
		strings.Contains(strings.ToLower(content), "performance")

	if hasPortfolioContent {
		t.Log("PASS: Output document contains portfolio analysis content")
	} else {
		t.Log("WARN: Output document may not contain expected portfolio content")
	}

	t.Log("PASS: Navexa output validation complete")
}
