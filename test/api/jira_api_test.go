package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Helper functions

func getServerURL() string {
	return "http://localhost:8086" // Use port 8086 for tests (avoids conflicts with dev server on 8085)
}

type projectData struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	IssueCount int    `json:"issueCount"`
}

// clearJiraData clears all Jira data and verifies it's cleared
func clearJiraData(t *testing.T, serverURL string) {
	t.Helper()
	clearResp, err := http.Post(serverURL+"/api/data/jira/clear", "application/json", nil)
	require.NoError(t, err)
	defer clearResp.Body.Close()

	require.Equal(t, http.StatusOK, clearResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(clearResp.Body).Decode(&result))
	require.Equal(t, "success", result["status"])
	t.Logf("✓ Jira data cleared: %s", result["message"])
}

// scrapeAndWaitForProjects scrapes projects and waits for them to be available
func scrapeAndWaitForProjects(t *testing.T, serverURL string, timeout time.Duration) []projectData {
	t.Helper()

	// Trigger project scraping
	scrapeResp, err := http.Post(serverURL+"/api/scrape/projects", "application/json", nil)
	require.NoError(t, err)
	defer scrapeResp.Body.Close()

	require.Equal(t, http.StatusOK, scrapeResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(scrapeResp.Body).Decode(&result))
	t.Logf("✓ Project scraping started: %s", result["message"])

	// Wait for projects to be available
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("❌ Timeout: Projects were not scraped within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(serverURL + "/api/collector/projects")
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var data struct {
				Data []projectData `json:"data"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				continue
			}

			if len(data.Data) > 0 {
				t.Logf("✓ Projects available: %d projects", len(data.Data))
				return data.Data
			}
		}
	}
}

// collectAndWaitForIssues collects issues for a project and waits for them
func collectAndWaitForIssues(t *testing.T, serverURL string, projectKey string, timeout time.Duration) int {
	t.Helper()

	// Trigger issue collection
	requestBody := map[string]interface{}{
		"projectKeys": []string{projectKey},
	}

	requestJSON, err := json.Marshal(requestBody)
	require.NoError(t, err)

	collectResp, err := http.Post(
		serverURL+"/api/projects/get-issues",
		"application/json",
		bytes.NewBuffer(requestJSON),
	)
	require.NoError(t, err)
	defer collectResp.Body.Close()

	require.Equal(t, http.StatusOK, collectResp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(collectResp.Body).Decode(&result))
	t.Logf("✓ Issue collection started: %s", result["message"])

	// Wait for issues to be available
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	issuesURL := fmt.Sprintf("%s/api/data/jira/issues?projectKey=%s", serverURL, projectKey)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("❌ Timeout: Issues were not collected within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(issuesURL)
			if err != nil {
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var data struct {
				Issues []map[string]interface{} `json:"issues"`
			}

			if err := json.Unmarshal(body, &data); err != nil {
				continue
			}

			count := len(data.Issues)
			if count > 0 {
				t.Logf("✓ Issues available: %d issues", count)
				return count
			}
		}
	}
}

// TestJiraIssuesCollection verifies the complete workflow of collecting issues
// This test follows the required workflow:
// 1. Clear all Jira data - test passes if all Jira data is deleted
// 2. Get projects - test passes if project count > 0
// 3. Select project and get issues - test passes if issue count > 0
func TestJiraIssuesCollection(t *testing.T) {
	serverURL := getServerURL()

	t.Log("=== Testing Jira Issues Collection Workflow ===")

	// Step 1: Clear all Jira data
	clearJiraData(t, serverURL)

	// Step 2: Scrape projects and verify count > 0
	projects := scrapeAndWaitForProjects(t, serverURL, 30*time.Second)
	require.Greater(t, len(projects), 0, "Project count should be > 0")

	// Step 3: Select a project and get issues - verify count > 0
	project := projects[0]
	t.Logf("✓ Selected project: %s", project.Key)

	issueCount := collectAndWaitForIssues(t, serverURL, project.Key, 60*time.Second)
	require.Greater(t, issueCount, 0, "Issue count should be > 0")

	t.Log("\n✅ SUCCESS: Complete Jira workflow verified")
}
