// job_definition_github_test.go - GitHub job execution tests
// Tests GitHub Repository Collector and GitHub Actions Log Collector jobs

package ui

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// createGitHubConnector creates a GitHub connector for tests using the token from .env
// Returns the connector ID on success
func createGitHubConnector(utc *UITestContext, storeInKV bool) (string, error) {
	token := utc.Env.EnvVars["github_token"]
	if token == "" {
		return "", fmt.Errorf("github_token not found in test/config/.env")
	}

	utc.Log("Creating GitHub connector...")

	// Create connector via API
	helper := utc.Env.NewHTTPTestHelper(utc.T)

	body := map[string]interface{}{
		"name": "Test GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": token,
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	if err != nil {
		return "", fmt.Errorf("failed to create connector: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("connector creation failed with status: %d", resp.StatusCode)
	}

	var connector map[string]interface{}
	if err := helper.ParseJSONResponse(resp, &connector); err != nil {
		return "", fmt.Errorf("failed to parse connector response: %w", err)
	}

	connectorID, ok := connector["id"].(string)
	if !ok {
		return "", fmt.Errorf("connector ID not found in response")
	}

	utc.Log("Created GitHub connector: %s", connectorID)

	if storeInKV {
		// Store connector ID in KV store for job definitions to use
		kvBody := map[string]string{
			"value":       connectorID,
			"description": "GitHub connector for tests",
		}
		resp, err = helper.PUT("/api/kv/github_connector_id", kvBody)
		if err != nil {
			return "", fmt.Errorf("failed to store connector ID in KV: %w", err)
		}
		defer resp.Body.Close()

		utc.Log("Stored connector ID in KV store as github_connector_id")
	} else {
		utc.Log("Note: Not storing in KV - job will resolve by name")
	}

	return connectorID, nil
}

// TestGitHubRepoCollector tests the GitHub Repository Collector job via UI
func TestGitHubRepoCollector(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Starting Test: GitHub Repository Collector ---")

	// Create GitHub connector (store in KV for ID-based lookup)
	_, err := createGitHubConnector(utc, true)
	if err != nil {
		t.Fatalf("Failed to create GitHub connector: %v", err)
	}

	jobName := "GitHub Repository Collector"

	// Take screenshot before triggering job
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	utc.Screenshot("github_repo_before")

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job (3 minute timeout for repo fetching)
	opts := MonitorJobOptions{
		Timeout:         3 * time.Minute,
		ExpectDocuments: true,
		AllowFailure:    false,
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("Test completed successfully")
}

// TestGitHubActionsCollector tests the GitHub Actions Log Collector job via UI
func TestGitHubActionsCollector(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Starting Test: GitHub Actions Log Collector ---")

	// Create GitHub connector (store in KV for ID-based lookup)
	_, err := createGitHubConnector(utc, true)
	if err != nil {
		t.Fatalf("Failed to create GitHub connector: %v", err)
	}

	jobName := "GitHub Actions Log Collector"

	// Take screenshot before triggering job
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	utc.Screenshot("github_actions_before")

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job (3 minute timeout for actions fetching)
	opts := MonitorJobOptions{
		Timeout:         3 * time.Minute,
		ExpectDocuments: true,
		AllowFailure:    false,
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("Test completed successfully")
}

// TestGitHubRepoCollectorByName tests the GitHub Repository Collector using connector_name
// This validates that jobs can resolve connectors by name instead of ID
func TestGitHubRepoCollectorByName(t *testing.T) {
	utc := NewUITestContext(t, MaxJobTestTimeout)
	defer utc.Cleanup()

	utc.Log("--- Starting Test: GitHub Repository Collector (By Name) ---")
	utc.Log("This test validates connector_name resolution (instead of connector_id)")

	// Create GitHub connector WITHOUT storing in KV
	// The job definition uses connector_name = "Test GitHub Connector" instead of {github_connector_id}
	_, err := createGitHubConnector(utc, false)
	if err != nil {
		t.Fatalf("Failed to create GitHub connector: %v", err)
	}

	jobName := "GitHub Repository Collector (By Name)"

	// Take screenshot before triggering job
	if err := utc.Navigate(utc.QueueURL); err != nil {
		t.Fatalf("Failed to navigate to queue page: %v", err)
	}
	utc.Screenshot("github_repo_by_name_before")

	// Trigger the job
	if err := utc.TriggerJob(jobName); err != nil {
		t.Fatalf("Failed to trigger %s: %v", jobName, err)
	}

	// Monitor the job (5 minute timeout for repo fetching - may have many child jobs)
	opts := MonitorJobOptions{
		Timeout:         5 * time.Minute,
		ExpectDocuments: true,
		AllowFailure:    false,
	}
	if err := utc.MonitorJob(jobName, opts); err != nil {
		t.Fatalf("Job monitoring failed: %v", err)
	}

	utc.Log("Test completed successfully - connector_name resolution works!")
}
