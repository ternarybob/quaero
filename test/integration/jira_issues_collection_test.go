package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJiraIssuesCollection verifies the complete workflow of collecting issues
// This test ensures that when issues are scraped, they are actually stored in the database
func TestJiraIssuesCollection(t *testing.T) {
	serverURL := getServerURL()

	t.Log("=== Testing Jira Issues Collection Workflow ===")

	// Step 1: Get projects to find one with issues
	t.Log("Step 1: Getting projects...")
	projectsResp, err := http.Get(serverURL + "/api/collector/projects")
	require.NoError(t, err, "Should be able to get projects")
	defer projectsResp.Body.Close()

	require.Equal(t, http.StatusOK, projectsResp.StatusCode, "Projects endpoint should return 200")

	var projectsData struct {
		Data []struct {
			Key        string `json:"key"`
			Name       string `json:"name"`
			IssueCount int    `json:"issueCount"`
		} `json:"data"`
	}

	err = json.NewDecoder(projectsResp.Body).Decode(&projectsData)
	require.NoError(t, err, "Should be able to parse projects response")

	// Find a project with issues (prefer smaller projects for faster testing)
	var testProject struct {
		Key        string
		IssueCount int
	}

	for _, project := range projectsData.Data {
		if project.IssueCount > 0 && project.IssueCount <= 100 {
			testProject.Key = project.Key
			testProject.IssueCount = project.IssueCount
			break
		}
	}

	if testProject.Key == "" {
		t.Skip("No projects with issues found - run SYNC PROJECTS first")
	}

	t.Logf("✓ Found test project: %s with %d issues", testProject.Key, testProject.IssueCount)

	// Step 2: Get current issue count for this project (before scraping)
	t.Log("Step 2: Checking current issues in database...")
	issuesURL := fmt.Sprintf("%s/api/data/jira/issues?projectKey=%s", serverURL, testProject.Key)
	initialResp, err := http.Get(issuesURL)
	require.NoError(t, err, "Should be able to get issues")
	defer initialResp.Body.Close()

	var initialIssuesData struct {
		Issues []map[string]interface{} `json:"issues"`
	}

	err = json.NewDecoder(initialResp.Body).Decode(&initialIssuesData)
	require.NoError(t, err, "Should be able to parse issues response")

	initialCount := len(initialIssuesData.Issues)
	t.Logf("  Current issues in DB for %s: %d", testProject.Key, initialCount)

	// Step 3: Trigger issue collection for this project
	t.Log("Step 3: Triggering issue collection via API...")
	requestBody := map[string]interface{}{
		"projectKeys": []string{testProject.Key},
	}

	requestJSON, err := json.Marshal(requestBody)
	require.NoError(t, err, "Should be able to marshal request")

	collectResp, err := http.Post(
		serverURL+"/api/projects/get-issues",
		"application/json",
		bytes.NewBuffer(requestJSON),
	)
	require.NoError(t, err, "Should be able to call get-issues endpoint")
	defer collectResp.Body.Close()

	require.Equal(t, http.StatusOK, collectResp.StatusCode, "Get-issues should return 200")

	var collectResult map[string]string
	err = json.NewDecoder(collectResp.Body).Decode(&collectResult)
	require.NoError(t, err, "Should be able to parse collect response")

	t.Logf("✓ Collection started: %s", collectResult["message"])

	// Step 4: Wait for issues to be collected and stored
	t.Log("Step 4: Waiting for issues to be stored in database...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var finalCount int
	var issuesStored bool

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("❌ Timeout: Issues were not stored within 60 seconds")
		case <-ticker.C:
			// Check current count in database
			checkResp, err := http.Get(issuesURL)
			if err != nil {
				t.Logf("  Error checking issues: %v", err)
				continue
			}

			checkBody, err := io.ReadAll(checkResp.Body)
			checkResp.Body.Close()
			if err != nil {
				t.Logf("  Error reading response: %v", err)
				continue
			}

			var checkData struct {
				Issues []map[string]interface{} `json:"issues"`
			}

			if err := json.Unmarshal(checkBody, &checkData); err != nil {
				t.Logf("  Error parsing response: %v", err)
				continue
			}

			finalCount = len(checkData.Issues)
			elapsed := time.Since(startTime).Round(time.Second)

			t.Logf("  Checking... issues in DB: %d/%d (elapsed: %v)",
				finalCount, testProject.IssueCount, elapsed)

			// Success condition: we have the expected number of issues
			if finalCount == testProject.IssueCount {
				issuesStored = true
				t.Logf("✓ All %d issues stored after %v", finalCount, elapsed)
				goto done
			}

			// Also check if we've waited long enough and have some issues
			// (maybe the count was wrong)
			if elapsed > 30*time.Second && finalCount > 0 {
				t.Logf("⚠ After 30s, have %d issues (expected %d)", finalCount, testProject.IssueCount)
				goto done
			}
		}
	}

done:
	if !issuesStored && finalCount == 0 {
		t.Fatalf("❌ FAIL: No issues were stored in database after collection")
	}

	// Step 5: Verify the stored issues
	t.Log("Step 5: Verifying stored issues...")

	verifyResp, err := http.Get(issuesURL)
	require.NoError(t, err, "Should be able to get issues for verification")
	defer verifyResp.Body.Close()

	var verifyData struct {
		Issues []map[string]interface{} `json:"issues"`
	}

	err = json.NewDecoder(verifyResp.Body).Decode(&verifyData)
	require.NoError(t, err, "Should be able to parse verification response")

	actualCount := len(verifyData.Issues)
	t.Logf("\n=== Verification Results ===")
	t.Logf("  Project: %s", testProject.Key)
	t.Logf("  Expected Issues: %d", testProject.IssueCount)
	t.Logf("  Actual Issues in DB: %d", actualCount)
	t.Logf("  Initial Count: %d", initialCount)

	// Main assertion: we should have the expected number of issues
	assert.Equal(t, testProject.IssueCount, actualCount,
		"Database should contain all scraped issues")

	// Verify each issue belongs to the correct project
	wrongProjectCount := 0
	for i, issue := range verifyData.Issues {
		key, hasKey := issue["key"].(string)
		if !hasKey {
			t.Errorf("Issue %d missing key field", i)
			continue
		}

		fields, hasFields := issue["fields"].(map[string]interface{})
		if !hasFields {
			t.Errorf("Issue %s missing fields", key)
			continue
		}

		project, hasProject := fields["project"].(map[string]interface{})
		if !hasProject {
			t.Errorf("Issue %s missing project in fields", key)
			continue
		}

		projectKey, hasProjectKey := project["key"].(string)
		if !hasProjectKey {
			t.Errorf("Issue %s project missing key", key)
			continue
		}

		if projectKey != testProject.Key {
			wrongProjectCount++
			t.Errorf("Issue %s belongs to project %s, expected %s",
				key, projectKey, testProject.Key)
		}
	}

	if wrongProjectCount == 0 && actualCount > 0 {
		t.Logf("✓ All %d issues belong to project %s", actualCount, testProject.Key)
	}

	if actualCount == testProject.IssueCount && wrongProjectCount == 0 {
		t.Log("\n✅ SUCCESS: Issue collection workflow verified")
	} else {
		t.Log("\n❌ FAIL: Issue collection has problems")
		t.Fail()
	}
}

func getServerURL() string {
	// Default to local server
	return "http://localhost:8085"
}
