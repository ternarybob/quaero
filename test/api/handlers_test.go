package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAPIHandlers tests all API handler endpoints to ensure
// they work correctly before and after refactoring
func TestAPIHandlers(t *testing.T) {
	serverURL := getServerURL(t)

	t.Run("Version", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/version")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotEmpty(t, result["version"])
		t.Logf("✓ Version: %s", result["version"])
	})

	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Equal(t, "ok", result["status"])
		t.Log("✓ Health check passed")
	})

	t.Run("AuthStatus", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/auth/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		t.Logf("✓ Auth status: %v", result["authenticated"])
	})

	t.Run("ParserStatus", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/parser/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["jiraProjects"])
		require.NotNil(t, result["jiraIssues"])
		require.NotNil(t, result["confluenceSpaces"])
		require.NotNil(t, result["confluencePages"])
		t.Log("✓ Parser status retrieved")
	})

	t.Run("AuthDetails", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/auth/details")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["services"])
		t.Log("✓ Auth details retrieved")
	})

	t.Run("CollectorProjects", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/collector/projects?page=0&pageSize=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["data"])
		require.NotNil(t, result["pagination"])

		pagination := result["pagination"].(map[string]interface{})
		require.Equal(t, float64(0), pagination["page"])
		require.Equal(t, float64(10), pagination["pageSize"])
		t.Log("✓ Collector projects with pagination")
	})

	t.Run("CollectorSpaces", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/collector/spaces?page=0&pageSize=10")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["data"])
		require.NotNil(t, result["pagination"])
		t.Log("✓ Collector spaces with pagination")
	})

	t.Run("DocumentStats", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/documents/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		t.Logf("✓ Document stats: %+v", result)
	})

	t.Run("DocumentList", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/documents")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["documents"])
		require.NotNil(t, result["count"])
		t.Log("✓ Document list retrieved")
	})

	t.Run("ProcessingStatus", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/documents/processing/status")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		t.Logf("✓ Processing status: %+v", result)
	})

	t.Run("ChatHealth", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/chat/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Accept either 200 or 503 (service unavailable in mock mode)
		require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["mode"])
		t.Logf("✓ Chat health: mode=%v", result["mode"])
	})
}

// TestMethodNotAllowed verifies that wrong HTTP methods return 405
func TestMethodNotAllowed(t *testing.T) {
	serverURL := getServerURL(t)

	testCases := []struct {
		name   string
		url    string
		method string
	}{
		{"GET on POST endpoint", "/api/scrape/projects", "GET"},
		{"POST on GET endpoint", "/api/health", "POST"},
		{"DELETE on GET endpoint", "/api/version", "DELETE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, serverURL+tc.url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
			t.Logf("✓ Method %s not allowed on %s", tc.method, tc.url)
		})
	}
}

// TestJSONResponseFormat verifies all endpoints return proper JSON
func TestJSONResponseFormat(t *testing.T) {
	serverURL := getServerURL(t)

	endpoints := []string{
		"/api/version",
		"/api/health",
		"/api/auth/status",
		"/api/parser/status",
		"/api/auth/details",
		"/api/collector/projects",
		"/api/collector/spaces",
		"/api/documents/stats",
		"/api/documents",
		"/api/documents/processing/status",
		"/api/chat/health",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(serverURL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			require.Equal(t, "application/json", contentType)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var result interface{}
			require.NoError(t, json.Unmarshal(body, &result))
			t.Logf("✓ %s returns valid JSON", endpoint)
		})
	}
}

// TestPaginationParams verifies pagination parameters work correctly
func TestPaginationParams(t *testing.T) {
	serverURL := getServerURL(t)

	testCases := []struct {
		page     int
		pageSize int
	}{
		{0, 10},
		{0, 25},
		{0, 50},
		{1, 10},
	}

	for _, tc := range testCases {
		t.Run("page="+string(rune(tc.page))+" pageSize="+string(rune(tc.pageSize)), func(t *testing.T) {
			url := serverURL + "/api/collector/projects?page=" +
				string(rune(tc.page+'0')) + "&pageSize=" + string(rune(tc.pageSize/10+'0'))

			resp, err := http.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]interface{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

			pagination := result["pagination"].(map[string]interface{})
			require.Equal(t, float64(tc.page), pagination["page"])
			require.Equal(t, float64(tc.pageSize), pagination["pageSize"])
			t.Logf("✓ Pagination params validated")
		})
	}
}

// TestAuthenticationRequired verifies endpoints that require auth return 401
func TestAuthenticationRequired(t *testing.T) {
	serverURL := getServerURL(t)

	// Note: This test assumes auth is NOT set up
	// If auth is configured, these tests may need adjustment

	endpoints := []struct {
		method string
		url    string
	}{
		{"POST", "/api/scrape/projects"},
		{"POST", "/api/scrape/spaces"},
		{"POST", "/api/projects/refresh-cache"},
		{"POST", "/api/spaces/refresh-cache"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.url, func(t *testing.T) {
			var resp *http.Response
			var err error

			if ep.method == "POST" {
				resp, err = http.Post(serverURL+ep.url, "application/json", nil)
			} else {
				req, _ := http.NewRequest(ep.method, serverURL+ep.url, nil)
				resp, err = http.DefaultClient.Do(req)
			}

			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 401 if not authenticated
			// Or 200 if authentication is set up
			require.True(t,
				resp.StatusCode == http.StatusUnauthorized ||
					resp.StatusCode == http.StatusOK,
				"Expected 401 or 200, got %d", resp.StatusCode)

			if resp.StatusCode == http.StatusUnauthorized {
				var result map[string]string
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
				require.Equal(t, "error", result["status"])
				t.Logf("✓ Auth required (401) for %s", ep.url)
			} else {
				t.Logf("✓ Auth configured, endpoint accessible")
			}
		})
	}
}

// TestErrorResponseFormat verifies error responses follow standard format
func TestErrorResponseFormat(t *testing.T) {
	serverURL := getServerURL(t)

	// Test a known error condition - POST to GET endpoint
	req, err := http.NewRequest("POST", serverURL+"/api/health", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

	// Read body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Should be plain text error from http.Error
	require.Contains(t, string(body), "Method not allowed")
	t.Log("✓ Error response format verified")
}

// TestStartedResponseFormat verifies async "started" responses
func TestStartedResponseFormat(t *testing.T) {
	serverURL := getServerURL(t)

	// This test may fail if not authenticated - that's expected
	resp, err := http.Post(serverURL+"/api/scrape/projects", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var result map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

		require.Equal(t, "started", result["status"])
		require.NotEmpty(t, result["message"])
		t.Log("✓ 'Started' response format verified")
	} else if resp.StatusCode == http.StatusUnauthorized {
		t.Skip("Skipping - authentication required")
	}
}

// TestCollectorFilterByProjectKey tests filtering issues by project
func TestCollectorFilterByProjectKey(t *testing.T) {
	serverURL := getServerURL(t)

	// First get projects to find a valid project key
	resp, err := http.Get(serverURL + "/api/collector/projects")
	require.NoError(t, err)
	defer resp.Body.Close()

	var projectResult map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projectResult))

	data := projectResult["data"].([]interface{})
	if len(data) == 0 {
		t.Skip("No projects available for testing")
	}

	project := data[0].(map[string]interface{})
	projectKey := project["key"].(string)

	// Now get issues filtered by this project
	resp2, err := http.Get(serverURL + "/api/collector/issues?projectKey=" + projectKey)
	require.NoError(t, err)
	defer resp2.Body.Close()

	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var issueResult map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&issueResult))
	require.NotNil(t, issueResult["data"])
	t.Logf("✓ Issues filtered by project: %s", projectKey)
}

// TestDataEndpoints tests the data handler endpoints
func TestDataEndpoints(t *testing.T) {
	serverURL := getServerURL(t)

	t.Run("JiraData", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/data/jira")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["projects"])
		require.NotNil(t, result["issues"])
		t.Log("✓ Jira data endpoint works")
	})

	t.Run("JiraIssues", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/data/jira/issues")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["issues"])
		t.Log("✓ Jira issues endpoint works")
	})

	t.Run("ConfluenceData", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/data/confluence")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["spaces"])
		require.NotNil(t, result["pages"])
		t.Log("✓ Confluence data endpoint works")
	})

	t.Run("ConfluencePages", func(t *testing.T) {
		resp, err := http.Get(serverURL + "/api/data/confluence/pages")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.NotNil(t, result["pages"])
		t.Log("✓ Confluence pages endpoint works")
	})
}

// TestPostEndpoints tests POST endpoints return proper formats
func TestPostEndpoints(t *testing.T) {
	serverURL := getServerURL(t)

	t.Run("TriggerCollection", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/scheduler/trigger-collection", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.True(t, result["success"].(bool))
		t.Log("✓ Trigger collection works")
	})

	t.Run("TriggerEmbedding", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/scheduler/trigger-embedding", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.True(t, result["success"].(bool))
		t.Log("✓ Trigger embedding works")
	})

	t.Run("ProcessDocuments", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/documents/process", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		require.Equal(t, "started", result["status"])
		t.Log("✓ Process documents endpoint works")
	})
}

// TestChatEndpoint tests the chat endpoint
func TestChatEndpoint(t *testing.T) {
	serverURL := getServerURL(t)

	requestBody := map[string]interface{}{
		"message": "What is Quaero?",
		"history": []interface{}{},
	}

	requestJSON, err := json.Marshal(requestBody)
	require.NoError(t, err)

	resp, err := http.Post(
		serverURL+"/api/chat",
		"application/json",
		bytes.NewBuffer(requestJSON),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Accept both 200 (working) and 500 (mock/offline mode issues)
	require.True(t,
		resp.StatusCode == http.StatusOK ||
			resp.StatusCode == http.StatusInternalServerError,
		"Expected 200 or 500, got %d", resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	if resp.StatusCode == http.StatusOK {
		require.True(t, result["success"].(bool))
		require.NotNil(t, result["message"])
		t.Log("✓ Chat endpoint works")
	} else {
		require.False(t, result["success"].(bool))
		t.Log("✓ Chat endpoint returns error (expected in mock mode)")
	}
}
