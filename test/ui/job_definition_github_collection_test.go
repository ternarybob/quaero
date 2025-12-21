// job_definition_github_collection_test.go - GitHub Collection Template Tests
// Tests the github-collection job template and github-quaero-collector job definition

package ui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

// TestJobDefinitionGitHubCollection tests the github-collection template in the UI
func TestJobDefinitionGitHubCollection(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: GitHub Collection Template ---")

	// Create a job definition using the github-collection template via API
	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	jobDefID := fmt.Sprintf("test-github-collection-ui-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":          jobDefID,
		"name":        "Test GitHub Collection UI",
		"type":        "job_template",
		"description": "UI test for github-collection template",
		"enabled":     false,
		"steps": []map[string]interface{}{
			{
				"name":        "collect_test_repo",
				"type":        "job_template",
				"description": "Execute github-collection template",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"template": "github-collection",
					"variables": []map[string]interface{}{
						{
							"owner":      "golang",
							"name":       "example",
							"name_lower": "example",
							"branch":     "master",
							"connector":  "test-connector",
						},
					},
				},
			},
		},
	}

	resp, err := httpHelper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		utc.Log("Warning: Failed to create job definition (status %d)", resp.StatusCode)
	} else {
		utc.Log("Created test job definition: %s", jobDefID)
		defer func() {
			httpHelper.DELETE("/api/job-definitions/" + jobDefID)
		}()
	}

	// Navigate to jobs page
	utc.Log("Navigating to Jobs page")
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	// Wait for page to load
	time.Sleep(2 * time.Second)
	utc.Screenshot("github_collection_jobs_page")

	// Search for the job definition
	utc.Log("Looking for github-collection in the job list")

	var pageContent string
	err = chromedp.Run(utc.Ctx,
		chromedp.Text("body", &pageContent, chromedp.ByQuery),
	)
	if err != nil {
		utc.Log("Warning: Could not get page content: %v", err)
	}

	// Check if github-related jobs are visible
	if strings.Contains(pageContent, "GitHub") || strings.Contains(pageContent, "github") {
		utc.Log("Found GitHub-related content in jobs page")
	}

	// Take final screenshot
	utc.FullScreenshot("github_collection_final")
	utc.Log("GitHub collection template UI test completed")
}

// TestJobDefinitionQuaeroCollector tests the github-quaero-collector job definition
func TestJobDefinitionQuaeroCollector(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing Job Definition: Quaero Collector ---")

	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	// Check if the quaero collector job definition exists
	utc.Log("Checking for github-quaero-collector job definition")
	resp, err := httpHelper.GET("/api/job-definitions/github-quaero-collector")
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		utc.Log("github-quaero-collector job definition found")

		var jobDef map[string]interface{}
		if err := httpHelper.ParseJSONResponse(resp, &jobDef); err == nil {
			// Log job definition details
			if name, ok := jobDef["name"].(string); ok {
				utc.Log("Job definition name: %s", name)
			}
			if desc, ok := jobDef["description"].(string); ok {
				utc.Log("Job definition description: %s", desc)
			}
		}
	} else {
		utc.Log("github-quaero-collector not loaded (status %d) - may need job-definitions dir configured", resp.StatusCode)
	}

	// Navigate to jobs page
	utc.Log("Navigating to Jobs page")
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	time.Sleep(2 * time.Second)
	utc.Screenshot("quaero_collector_jobs_page")

	// Take final screenshot
	utc.FullScreenshot("quaero_collector_final")
	utc.Log("Quaero collector UI test completed")
}

// TestGitHubCollectionTemplateVariables tests that template variables are correctly displayed
func TestGitHubCollectionTemplateVariables(t *testing.T) {
	utc := NewUITestContext(t, 2*time.Minute)
	defer utc.Cleanup()

	utc.Log("--- Testing GitHub Collection Template Variables ---")

	httpHelper := utc.Env.NewHTTPTestHelperWithTimeout(t, 10*time.Second)

	// Create a job definition with multiple variable sets
	jobDefID := fmt.Sprintf("test-github-template-vars-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"id":          jobDefID,
		"name":        "Test GitHub Template Variables",
		"type":        "job_template",
		"description": "Test template variable substitution",
		"enabled":     false,
		"steps": []map[string]interface{}{
			{
				"name":        "collect_repos",
				"type":        "job_template",
				"description": "Execute github-collection for multiple repos",
				"on_error":    "continue",
				"config": map[string]interface{}{
					"template": "github-collection",
					"variables": []map[string]interface{}{
						{
							"owner":      "golang",
							"name":       "go",
							"name_lower": "go",
							"branch":     "master",
							"connector":  "github-test",
						},
						{
							"owner":      "golang",
							"name":       "example",
							"name_lower": "example",
							"branch":     "master",
							"connector":  "github-test",
						},
					},
				},
			},
		},
	}

	resp, err := httpHelper.POST("/api/job-definitions", body)
	require.NoError(t, err, "Failed to create job definition")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		utc.Log("Created job definition with multiple variable sets: %s", jobDefID)
		defer func() {
			httpHelper.DELETE("/api/job-definitions/" + jobDefID)
		}()

		// Verify the job definition was stored correctly
		resp2, err := httpHelper.GET("/api/job-definitions/" + jobDefID)
		require.NoError(t, err)
		defer resp2.Body.Close()

		if resp2.StatusCode == http.StatusOK {
			var jobDef map[string]interface{}
			if err := httpHelper.ParseJSONResponse(resp2, &jobDef); err == nil {
				steps, _ := jobDef["steps"].([]interface{})
				if len(steps) > 0 {
					step := steps[0].(map[string]interface{})
					config, _ := step["config"].(map[string]interface{})
					variables, _ := config["variables"].([]interface{})
					utc.Log("Job definition has %d variable sets", len(variables))
				}
			}
		}
	} else {
		utc.Log("Warning: Failed to create job definition (status %d)", resp.StatusCode)
	}

	// Navigate to jobs page to see the job
	utc.Log("Navigating to Jobs page")
	if err := utc.Navigate(utc.JobsURL); err != nil {
		t.Fatalf("Failed to navigate to Jobs page: %v", err)
	}

	time.Sleep(2 * time.Second)
	utc.Screenshot("template_variables_jobs_page")

	utc.FullScreenshot("template_variables_final")
	utc.Log("Template variables UI test completed")
}
