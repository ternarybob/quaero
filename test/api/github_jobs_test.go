package api

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// createTestGitHubConnector creates a GitHub connector for testing
// Uses skip_validation_token for unit tests or GITHUB_TOKEN for integration tests
func createTestGitHubConnector(t *testing.T, helper *common.HTTPTestHelper) string {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = "skip_validation_token" // For unit tests
	}

	body := map[string]interface{}{
		"name": "Test GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": token,
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Failed to create connector, status: %d", resp.StatusCode)
		t.Skip("GitHub connector creation failed - skipping test")
	}

	var connector map[string]interface{}
	err = helper.ParseJSONResponse(resp, &connector)
	require.NoError(t, err)

	connectorID, ok := connector["id"].(string)
	require.True(t, ok, "connector ID should be a string")

	return connectorID
}

// skipIfNoGitHubToken skips the test if no GitHub token is available for integration tests
func skipIfNoGitHubToken(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test - GITHUB_TOKEN not set")
	}
}

// TestGitHubJobs_ValidationErrors tests that the API returns proper validation errors
func TestGitHubJobs_ValidationErrors(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Missing connector_id and connector_name for repo preview
	t.Log("Step 1: Testing missing connector_id/connector_name for repo preview")
	body := map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	}
	resp, err := helper.POST("/api/github/repo/preview", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 2: Missing owner for repo preview
	t.Log("Step 2: Testing missing owner for repo preview")
	body2 := map[string]interface{}{
		"connector_id": "test-connector",
		"repo":         "test-repo",
	}
	resp, err = helper.POST("/api/github/repo/preview", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 3: Missing repo for repo preview
	t.Log("Step 3: Testing missing repo for repo preview")
	body3 := map[string]interface{}{
		"connector_id": "test-connector",
		"owner":        "test-owner",
	}
	resp, err = helper.POST("/api/github/repo/preview", body3)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	// Test 4: Missing connector_id and connector_name for actions preview
	t.Log("Step 4: Testing missing connector_id/connector_name for actions preview")
	body4 := map[string]interface{}{
		"owner": "test-owner",
		"repo":  "test-repo",
	}
	resp, err = helper.POST("/api/github/actions/preview", body4)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("Validation error tests completed successfully")
}

// TestGitHubJobs_MissingConnector tests that the API returns 404 for non-existent connectors
func TestGitHubJobs_MissingConnector(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Test 1: Non-existent connector for repo preview
	t.Log("Step 1: Testing non-existent connector for repo preview")
	body := map[string]interface{}{
		"connector_id": "non-existent-connector-id",
		"owner":        "test-owner",
		"repo":         "test-repo",
	}
	resp, err := helper.POST("/api/github/repo/preview", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 2: Non-existent connector for actions preview
	t.Log("Step 2: Testing non-existent connector for actions preview")
	body2 := map[string]interface{}{
		"connector_id": "non-existent-connector-id",
		"owner":        "test-owner",
		"repo":         "test-repo",
	}
	resp, err = helper.POST("/api/github/actions/preview", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 3: Non-existent connector for repo start
	t.Log("Step 3: Testing non-existent connector for repo start")
	body3 := map[string]interface{}{
		"connector_id": "non-existent-connector-id",
		"owner":        "test-owner",
		"repo":         "test-repo",
	}
	resp, err = helper.POST("/api/github/repo/start", body3)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	// Test 4: Non-existent connector for actions start
	t.Log("Step 4: Testing non-existent connector for actions start")
	body4 := map[string]interface{}{
		"connector_id": "non-existent-connector-id",
		"owner":        "test-owner",
		"repo":         "test-repo",
	}
	resp, err = helper.POST("/api/github/actions/start", body4)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("Missing connector tests completed successfully")
}

// TestGitHubJobs_RepoPreview tests the repo preview endpoint with a real connector
func TestGitHubJobs_RepoPreview(t *testing.T) {
	skipIfNoGitHubToken(t)

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector first
	connectorID := createTestGitHubConnector(t, helper)
	t.Logf("Created connector: %s", connectorID)

	// Test repo preview with a public repository
	t.Log("Testing repo preview with golang/go repository")
	body := map[string]interface{}{
		"connector_id":  connectorID,
		"owner":         "golang",
		"repo":          "go",
		"branches":      []string{"master"},
		"extensions":    []string{".go", ".md"},
		"exclude_paths": []string{"src/", "test/"},
	}

	resp, err := helper.POST("/api/github/repo/preview", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var previewResp map[string]interface{}
	err = helper.ParseJSONResponse(resp, &previewResp)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, previewResp, "files", "Response should contain files")
	assert.Contains(t, previewResp, "total_count", "Response should contain total_count")
	assert.Contains(t, previewResp, "branches", "Response should contain branches")

	files, ok := previewResp["files"].([]interface{})
	require.True(t, ok, "files should be an array")
	t.Logf("Preview returned %d files", len(files))

	// Verify file structure if we got any files
	if len(files) > 0 {
		firstFile, ok := files[0].(map[string]interface{})
		require.True(t, ok, "file entry should be an object")
		assert.Contains(t, firstFile, "path", "File should have path")
		assert.Contains(t, firstFile, "folder", "File should have folder")
		assert.Contains(t, firstFile, "branch", "File should have branch")
	}

	t.Log("Repo preview test completed successfully")
}

// TestGitHubJobs_ActionsPreview tests the actions preview endpoint with a real connector
func TestGitHubJobs_ActionsPreview(t *testing.T) {
	skipIfNoGitHubToken(t)

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector first
	connectorID := createTestGitHubConnector(t, helper)
	t.Logf("Created connector: %s", connectorID)

	// Test actions preview with a public repository that has actions
	t.Log("Testing actions preview with golang/go repository")
	body := map[string]interface{}{
		"connector_id": connectorID,
		"owner":        "golang",
		"repo":         "go",
		"limit":        5,
	}

	resp, err := helper.POST("/api/github/actions/preview", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var previewResp map[string]interface{}
	err = helper.ParseJSONResponse(resp, &previewResp)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, previewResp, "runs", "Response should contain runs")
	assert.Contains(t, previewResp, "total_count", "Response should contain total_count")

	runs, ok := previewResp["runs"].([]interface{})
	require.True(t, ok, "runs should be an array")
	t.Logf("Preview returned %d workflow runs", len(runs))

	// Verify run structure if we got any runs
	if len(runs) > 0 {
		firstRun, ok := runs[0].(map[string]interface{})
		require.True(t, ok, "run entry should be an object")
		assert.Contains(t, firstRun, "id", "Run should have id")
		assert.Contains(t, firstRun, "workflow_name", "Run should have workflow_name")
		assert.Contains(t, firstRun, "status", "Run should have status")
		assert.Contains(t, firstRun, "branch", "Run should have branch")
		assert.Contains(t, firstRun, "started_at", "Run should have started_at")
	}

	t.Log("Actions preview test completed successfully")
}

// TestGitHubJobs_RepoCollectorStart tests starting a repo collector job
func TestGitHubJobs_RepoCollectorStart(t *testing.T) {
	skipIfNoGitHubToken(t)

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector first
	connectorID := createTestGitHubConnector(t, helper)
	t.Logf("Created connector: %s", connectorID)

	// Start a repo collector job with a small limit
	t.Log("Starting repo collector job")
	body := map[string]interface{}{
		"connector_id":  connectorID,
		"owner":         "golang",
		"repo":          "go",
		"branches":      []string{"master"},
		"extensions":    []string{".md"},
		"exclude_paths": []string{"src/", "test/"},
		"max_files":     5,
		"tags":          []string{"test", "repo-collector"},
	}

	resp, err := helper.POST("/api/github/repo/start", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var startResp map[string]interface{}
	err = helper.ParseJSONResponse(resp, &startResp)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, startResp, "job_id", "Response should contain job_id")
	assert.Contains(t, startResp, "message", "Response should contain message")

	jobID, ok := startResp["job_id"].(string)
	require.True(t, ok, "job_id should be a string")
	require.NotEmpty(t, jobID, "job_id should not be empty")
	t.Logf("Started job: %s", jobID)

	// Wait a bit and check job status
	time.Sleep(2 * time.Second)

	resp, err = helper.GET("/api/jobs/" + jobID)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var job map[string]interface{}
	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err)

	t.Logf("Job status: %v", job["status"])

	t.Log("Repo collector start test completed successfully")
}

// TestGitHubJobs_ActionsCollectorStart tests starting an actions collector job
func TestGitHubJobs_ActionsCollectorStart(t *testing.T) {
	skipIfNoGitHubToken(t)

	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector first
	connectorID := createTestGitHubConnector(t, helper)
	t.Logf("Created connector: %s", connectorID)

	// Start an actions collector job with a small limit
	t.Log("Starting actions collector job")
	body := map[string]interface{}{
		"connector_id": connectorID,
		"owner":        "golang",
		"repo":         "go",
		"limit":        3,
		"tags":         []string{"test", "actions-collector"},
	}

	resp, err := helper.POST("/api/github/actions/start", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var startResp map[string]interface{}
	err = helper.ParseJSONResponse(resp, &startResp)
	require.NoError(t, err)

	// Verify response structure
	assert.Contains(t, startResp, "job_id", "Response should contain job_id")
	assert.Contains(t, startResp, "message", "Response should contain message")

	jobID, ok := startResp["job_id"].(string)
	require.True(t, ok, "job_id should be a string")
	require.NotEmpty(t, jobID, "job_id should not be empty")
	t.Logf("Started job: %s", jobID)

	// Wait a bit and check job status
	time.Sleep(2 * time.Second)

	resp, err = helper.GET("/api/jobs/" + jobID)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusOK)

	var job map[string]interface{}
	err = helper.ParseJSONResponse(resp, &job)
	require.NoError(t, err)

	t.Logf("Job status: %v", job["status"])

	t.Log("Actions collector start test completed successfully")
}

// TestGitHubJobs_ConnectorWithSkipToken tests using a connector with skip_validation_token
func TestGitHubJobs_ConnectorWithSkipToken(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector with skip_validation_token
	body := map[string]interface{}{
		"name": "Test GitHub Connector",
		"type": "github",
		"config": map[string]interface{}{
			"token": "skip_validation_token",
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var connector map[string]interface{}
	err = helper.ParseJSONResponse(resp, &connector)
	require.NoError(t, err)

	connectorID, ok := connector["id"].(string)
	require.True(t, ok, "connector ID should be a string")
	require.NotEmpty(t, connectorID, "connector ID should not be empty")

	t.Logf("Created connector with skip_validation_token: %s", connectorID)

	// Verify the connector is usable for preview (will fail when actually calling GitHub API)
	// This tests the API routing and validation, not the actual GitHub integration
	body2 := map[string]interface{}{
		"connector_id": connectorID,
		"owner":        "test-owner",
		"repo":         "test-repo",
	}

	resp, err = helper.POST("/api/github/repo/preview", body2)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should reach the GitHub API call (not validation errors)
	// Will fail with auth error since skip_validation_token isn't a real token
	// But this proves the routing and handler work correctly
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"Should pass validation but may fail on GitHub API call, got status %d", resp.StatusCode)

	t.Log("Skip validation token test completed successfully")
}

// TestGitHubJobs_ConnectorByName tests using connector_name instead of connector_id
func TestGitHubJobs_ConnectorByName(t *testing.T) {
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Create a connector with a specific name
	connectorName := "test-github-connector"
	body := map[string]interface{}{
		"name": connectorName,
		"type": "github",
		"config": map[string]interface{}{
			"token": "skip_validation_token",
		},
	}

	resp, err := helper.POST("/api/connectors", body)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusCreated)

	var connector map[string]interface{}
	err = helper.ParseJSONResponse(resp, &connector)
	require.NoError(t, err)

	connectorID, ok := connector["id"].(string)
	require.True(t, ok, "connector ID should be a string")
	t.Logf("Created connector '%s' with ID: %s", connectorName, connectorID)

	// Test 1: Use connector_name for repo preview
	t.Log("Step 1: Testing connector_name for repo preview")
	body2 := map[string]interface{}{
		"connector_name": connectorName,
		"owner":          "test-owner",
		"repo":           "test-repo",
	}

	resp, err = helper.POST("/api/github/repo/preview", body2)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Should pass validation and reach the GitHub API
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"Should pass validation with connector_name, got status %d", resp.StatusCode)

	// Test 2: Use connector_name for actions preview
	t.Log("Step 2: Testing connector_name for actions preview")
	body3 := map[string]interface{}{
		"connector_name": connectorName,
		"owner":          "test-owner",
		"repo":           "test-repo",
	}

	resp, err = helper.POST("/api/github/actions/preview", body3)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"Should pass validation with connector_name, got status %d", resp.StatusCode)

	// Test 3: connector_id takes precedence over connector_name
	t.Log("Step 3: Testing connector_id precedence over connector_name")
	body4 := map[string]interface{}{
		"connector_id":   connectorID,
		"connector_name": "non-existent-name", // Should be ignored
		"owner":          "test-owner",
		"repo":           "test-repo",
	}

	resp, err = helper.POST("/api/github/repo/preview", body4)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Should succeed because connector_id takes precedence
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError,
		"connector_id should take precedence, got status %d", resp.StatusCode)

	// Test 4: Non-existent connector_name returns 404
	t.Log("Step 4: Testing non-existent connector_name returns 404")
	body5 := map[string]interface{}{
		"connector_name": "this-connector-does-not-exist",
		"owner":          "test-owner",
		"repo":           "test-repo",
	}

	resp, err = helper.POST("/api/github/repo/preview", body5)
	require.NoError(t, err)
	defer resp.Body.Close()
	helper.AssertStatusCode(resp, http.StatusNotFound)

	t.Log("Connector by name tests completed successfully")
}
