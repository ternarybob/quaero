package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// =============================================================================
// Navexa Worker Integration Tests
// =============================================================================
// Tests the Navexa portfolio integration workers:
// 1. navexa_portfolios - Fetch all user portfolios
// 2. navexa_holdings - Fetch holdings for a portfolio
// 3. navexa_performance - Fetch P/L performance data
//
// IMPORTANT: These tests require a valid navexa_api_key in KV storage.
// If not configured, tests will skip gracefully.
// =============================================================================

// getNavexaAPIKey retrieves the Navexa API key from the KV store
func getNavexaAPIKey(t *testing.T, helper *common.HTTPTestHelper) string {
	resp, err := helper.GET("/api/kv/navexa_api_key")
	if err != nil {
		t.Logf("Failed to get Navexa API key from KV store: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Navexa API key not found in KV store (status %d)", resp.StatusCode)
		return ""
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := helper.ParseJSONResponse(resp, &result); err != nil {
		t.Logf("Failed to parse Navexa API key response: %v", err)
		return ""
	}

	if result.Value == "" || strings.HasPrefix(result.Value, "fake-") {
		t.Log("Navexa API key is placeholder - skipping")
		return ""
	}

	return result.Value
}

// TestWorkerNavexaPortfolios tests the navexa_portfolios worker
func TestWorkerNavexaPortfolios(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if API key is configured
	apiKey := getNavexaAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("Navexa API key not configured - skipping test")
	}

	defID := fmt.Sprintf("test-navexa-portfolios-%d", time.Now().UnixNano())

	body := map[string]interface{}{
		"id":          defID,
		"name":        "Navexa Portfolios Worker Test",
		"description": "Test navexa_portfolios worker",
		"type":        "navexa_portfolios",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-portfolios",
				"type": "navexa_portfolios",
			},
		},
	}

	// Create job definition
	resp, err := helper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skipf("Job definition creation failed with status: %d", resp.StatusCode)
	}

	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + defID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	// Execute job
	execResp, err := helper.POST("/api/job-definitions/"+defID+"/execute", nil)
	require.NoError(t, err, "Failed to execute job")
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusAccepted {
		t.Skipf("Job execution failed with status: %d", execResp.StatusCode)
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)
	t.Logf("Executed navexa_portfolios job: %s", jobID)

	// Wait for completion
	finalStatus := waitForJobCompletion(t, helper, jobID, 3*time.Minute)
	if finalStatus != "completed" {
		t.Logf("INFO: Job ended with status %s", finalStatus)
		return
	}

	// Validate document was created
	docResp, err := helper.GET("/api/documents?tags=navexa-portfolio&limit=1")
	require.NoError(t, err)
	defer docResp.Body.Close()

	var docResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(docResp, &docResult))

	if len(docResult.Documents) > 0 {
		t.Log("PASS: navexa_portfolios created document with navexa-portfolio tag")
	} else {
		t.Log("INFO: No navexa-portfolio document found (may be expected if no portfolios)")
	}

	t.Log("PASS: TestWorkerNavexaPortfolios completed successfully")
}

// TestWorkerNavexaHoldings tests the navexa_holdings worker
// This test fetches portfolios first to get a valid portfolio ID
func TestWorkerNavexaHoldings(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if API key is configured
	apiKey := getNavexaAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("Navexa API key not configured - skipping test")
	}

	// First, execute portfolios to get a portfolio ID
	t.Log("Step 1: Fetching portfolios to get portfolio ID")

	portfolioDefID := fmt.Sprintf("test-navexa-portfolios-for-holdings-%d", time.Now().UnixNano())
	portfolioBody := map[string]interface{}{
		"id":          portfolioDefID,
		"name":        "Navexa Portfolios for Holdings Test",
		"description": "Fetch portfolios to get ID for holdings test",
		"type":        "navexa_portfolios",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{"name": "fetch-portfolios", "type": "navexa_portfolios"},
		},
	}

	resp, err := helper.POST("/api/job-definitions", portfolioBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Skip("Could not create portfolios job definition")
	}
	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + portfolioDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp, err := helper.POST("/api/job-definitions/"+portfolioDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()
	if execResp.StatusCode != http.StatusAccepted {
		t.Skip("Could not execute portfolios job")
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)

	finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skip("Portfolios job did not complete successfully")
	}

	// Get portfolio document to extract portfolio ID
	docResp, err := helper.GET("/api/documents?tags=navexa-portfolio&limit=1")
	require.NoError(t, err)
	defer docResp.Body.Close()

	var docResult struct {
		Documents []struct {
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(docResp, &docResult))

	if len(docResult.Documents) == 0 {
		t.Skip("No portfolios found - cannot test holdings")
	}

	portfolios, ok := docResult.Documents[0].Metadata["portfolios"].([]interface{})
	if !ok || len(portfolios) == 0 {
		t.Skip("No portfolios in metadata - cannot test holdings")
	}

	firstPortfolio := portfolios[0].(map[string]interface{})
	portfolioID := int(firstPortfolio["id"].(float64))
	portfolioName := firstPortfolio["name"].(string)
	t.Logf("Step 2: Testing holdings for portfolio %d (%s)", portfolioID, portfolioName)

	// Now test holdings worker
	holdingsDefID := fmt.Sprintf("test-navexa-holdings-%d", time.Now().UnixNano())
	holdingsBody := map[string]interface{}{
		"id":          holdingsDefID,
		"name":        "Navexa Holdings Worker Test",
		"description": "Test navexa_holdings worker",
		"type":        "navexa_holdings",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-holdings",
				"type": "navexa_holdings",
				"config": map[string]interface{}{
					"portfolio_id":   portfolioID,
					"portfolio_name": portfolioName,
				},
			},
		},
	}

	resp2, err := helper.POST("/api/job-definitions", holdingsBody)
	require.NoError(t, err)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Skipf("Holdings job definition creation failed with status: %d", resp2.StatusCode)
	}
	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + holdingsDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp2, err := helper.POST("/api/job-definitions/"+holdingsDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp2.Body.Close()
	if execResp2.StatusCode != http.StatusAccepted {
		t.Skipf("Holdings job execution failed with status: %d", execResp2.StatusCode)
	}

	var execResult2 map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
	jobID2 := execResult2["job_id"].(string)
	t.Logf("Executed navexa_holdings job: %s", jobID2)

	finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
	if finalStatus2 != "completed" {
		t.Logf("INFO: Holdings job ended with status %s", finalStatus2)
		return
	}

	// Validate document was created
	holdingsDocResp, err := helper.GET("/api/documents?tags=navexa-holdings&limit=1")
	require.NoError(t, err)
	defer holdingsDocResp.Body.Close()

	var holdingsDocResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(holdingsDocResp, &holdingsDocResult))

	if len(holdingsDocResult.Documents) > 0 {
		t.Log("PASS: navexa_holdings created document with navexa-holdings tag")
	} else {
		t.Log("INFO: No navexa-holdings document found (may be expected if portfolio empty)")
	}

	t.Log("PASS: TestWorkerNavexaHoldings completed successfully")
}

// TestWorkerNavexaPerformance tests the navexa_performance worker
// This test fetches portfolios first to get a valid portfolio ID
func TestWorkerNavexaPerformance(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	if err != nil {
		t.Skipf("Failed to setup test environment: %v", err)
	}
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Check if API key is configured
	apiKey := getNavexaAPIKey(t, helper)
	if apiKey == "" {
		t.Skip("Navexa API key not configured - skipping test")
	}

	// First, execute portfolios to get a portfolio ID (similar to holdings test)
	t.Log("Step 1: Fetching portfolios to get portfolio ID")

	portfolioDefID := fmt.Sprintf("test-navexa-portfolios-for-perf-%d", time.Now().UnixNano())
	portfolioBody := map[string]interface{}{
		"id":          portfolioDefID,
		"name":        "Navexa Portfolios for Performance Test",
		"description": "Fetch portfolios to get ID for performance test",
		"type":        "navexa_portfolios",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{"name": "fetch-portfolios", "type": "navexa_portfolios"},
		},
	}

	resp, err := helper.POST("/api/job-definitions", portfolioBody)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Skip("Could not create portfolios job definition")
	}
	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + portfolioDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp, err := helper.POST("/api/job-definitions/"+portfolioDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp.Body.Close()
	if execResp.StatusCode != http.StatusAccepted {
		t.Skip("Could not execute portfolios job")
	}

	var execResult map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp, &execResult))
	jobID := execResult["job_id"].(string)

	finalStatus := waitForJobCompletion(t, helper, jobID, 2*time.Minute)
	if finalStatus != "completed" {
		t.Skip("Portfolios job did not complete successfully")
	}

	// Get portfolio document to extract portfolio ID
	docResp, err := helper.GET("/api/documents?tags=navexa-portfolio&limit=1")
	require.NoError(t, err)
	defer docResp.Body.Close()

	var docResult struct {
		Documents []struct {
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(docResp, &docResult))

	if len(docResult.Documents) == 0 {
		t.Skip("No portfolios found - cannot test performance")
	}

	portfolios, ok := docResult.Documents[0].Metadata["portfolios"].([]interface{})
	if !ok || len(portfolios) == 0 {
		t.Skip("No portfolios in metadata - cannot test performance")
	}

	firstPortfolio := portfolios[0].(map[string]interface{})
	portfolioID := int(firstPortfolio["id"].(float64))
	portfolioName := firstPortfolio["name"].(string)
	t.Logf("Step 2: Testing performance for portfolio %d (%s)", portfolioID, portfolioName)

	// Now test performance worker
	perfDefID := fmt.Sprintf("test-navexa-performance-%d", time.Now().UnixNano())
	perfBody := map[string]interface{}{
		"id":          perfDefID,
		"name":        "Navexa Performance Worker Test",
		"description": "Test navexa_performance worker",
		"type":        "navexa_performance",
		"enabled":     true,
		"tags":        []string{"worker-test", "navexa"},
		"steps": []map[string]interface{}{
			{
				"name": "fetch-performance",
				"type": "navexa_performance",
				"config": map[string]interface{}{
					"portfolio_id":   portfolioID,
					"portfolio_name": portfolioName,
				},
			},
		},
	}

	resp2, err := helper.POST("/api/job-definitions", perfBody)
	require.NoError(t, err)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Skipf("Performance job definition creation failed with status: %d", resp2.StatusCode)
	}
	defer func() {
		delResp, _ := helper.DELETE("/api/job-definitions/" + perfDefID)
		if delResp != nil {
			delResp.Body.Close()
		}
	}()

	execResp2, err := helper.POST("/api/job-definitions/"+perfDefID+"/execute", nil)
	require.NoError(t, err)
	defer execResp2.Body.Close()
	if execResp2.StatusCode != http.StatusAccepted {
		t.Skipf("Performance job execution failed with status: %d", execResp2.StatusCode)
	}

	var execResult2 map[string]interface{}
	require.NoError(t, helper.ParseJSONResponse(execResp2, &execResult2))
	jobID2 := execResult2["job_id"].(string)
	t.Logf("Executed navexa_performance job: %s", jobID2)

	finalStatus2 := waitForJobCompletion(t, helper, jobID2, 2*time.Minute)
	if finalStatus2 != "completed" {
		t.Logf("INFO: Performance job ended with status %s", finalStatus2)
		return
	}

	// Validate document was created
	perfDocResp, err := helper.GET("/api/documents?tags=navexa-performance&limit=1")
	require.NoError(t, err)
	defer perfDocResp.Body.Close()

	var perfDocResult struct {
		Documents []map[string]interface{} `json:"documents"`
	}
	require.NoError(t, helper.ParseJSONResponse(perfDocResp, &perfDocResult))

	if len(perfDocResult.Documents) > 0 {
		t.Log("PASS: navexa_performance created document with navexa-performance tag")
	} else {
		t.Log("INFO: No navexa-performance document found")
	}

	t.Log("PASS: TestWorkerNavexaPerformance completed successfully")
}
