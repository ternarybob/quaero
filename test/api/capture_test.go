package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/quaero/test/common"
)

// CaptureRequest represents the payload the Chrome extension sends
// This matches the expected format in internal/handlers/document_handler.go
type CaptureRequest struct {
	URL         string `json:"url"`
	HTML        string `json:"html"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

// CaptureResponse represents the expected response from the capture endpoint
type CaptureResponse struct {
	DocumentID  string `json:"document_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	ContentSize int    `json:"content_size"`
	Message     string `json:"message"`
}

// TestCaptureEndpoint tests POST /api/documents/capture
// This simulates the Chrome extension sending captured page content
func TestCaptureEndpoint(t *testing.T) {
	// Start service with Badger configuration
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	// Create HTTP helper
	helper := env.NewHTTPTestHelper(t)

	// Prepare capture request matching what the Chrome extension sends
	captureReq := CaptureRequest{
		URL:         "https://example.atlassian.net/wiki/spaces/TEST/pages/123456/Test+Page",
		HTML:        `<!DOCTYPE html><html><head><title>Test Page</title></head><body><h1>Test Content</h1><p>This is test content from the Chrome extension capture.</p></body></html>`,
		Title:       "Test Page",
		Description: "A test page captured from Confluence",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// POST to capture endpoint
	resp, err := helper.POST("/api/documents/capture", captureReq)
	require.NoError(t, err, "Failed to call capture endpoint")

	// Verify status code is 201 Created
	helper.AssertStatusCode(resp, http.StatusCreated)

	// Parse and verify response
	var result CaptureResponse
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse capture response")

	// Verify response fields
	assert.NotEmpty(t, result.DocumentID, "Document ID should not be empty")
	assert.Equal(t, captureReq.URL, result.URL, "Response URL should match request URL")
	assert.Equal(t, captureReq.Title, result.Title, "Response title should match request title")
	assert.Greater(t, result.ContentSize, 0, "Content size should be greater than 0")
	assert.Equal(t, "Page captured successfully", result.Message, "Message should indicate success")

	t.Logf("Capture test passed - DocumentID: %s, ContentSize: %d", result.DocumentID, result.ContentSize)
}

// TestCaptureEndpointMissingURL tests that capture fails when URL is missing
func TestCaptureEndpointMissingURL(t *testing.T) {
	// Start service
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Request without URL
	captureReq := CaptureRequest{
		HTML:  `<html><body>Content</body></html>`,
		Title: "Test Page",
	}

	resp, err := helper.POST("/api/documents/capture", captureReq)
	require.NoError(t, err, "Failed to call capture endpoint")

	// Should fail with 400 Bad Request
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("Missing URL validation test passed")
}

// TestCaptureEndpointMissingHTML tests that capture fails when HTML is missing
func TestCaptureEndpointMissingHTML(t *testing.T) {
	// Start service
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Request without HTML
	captureReq := CaptureRequest{
		URL:   "https://example.com/page",
		Title: "Test Page",
	}

	resp, err := helper.POST("/api/documents/capture", captureReq)
	require.NoError(t, err, "Failed to call capture endpoint")

	// Should fail with 400 Bad Request
	helper.AssertStatusCode(resp, http.StatusBadRequest)

	t.Log("Missing HTML validation test passed")
}

// TestCaptureEndpointConfluencePage tests capturing a realistic Confluence page
func TestCaptureEndpointConfluencePage(t *testing.T) {
	// Start service
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Simulate a realistic Confluence page capture
	confluenceHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Project Documentation - Confluence</title>
    <meta name="description" content="Project documentation and guidelines">
</head>
<body>
    <div id="main-content">
        <article>
            <header>
                <h1>Project Documentation</h1>
                <div class="page-metadata">
                    <span>Last updated: 2024-01-15</span>
                    <span>Author: John Doe</span>
                </div>
            </header>
            <section>
                <h2>Overview</h2>
                <p>This document describes the project architecture and implementation details.</p>
                <ul>
                    <li>Component A - handles user authentication</li>
                    <li>Component B - manages data persistence</li>
                    <li>Component C - provides API endpoints</li>
                </ul>
            </section>
            <section>
                <h2>Getting Started</h2>
                <p>Follow these steps to set up the development environment:</p>
                <ol>
                    <li>Clone the repository</li>
                    <li>Install dependencies</li>
                    <li>Run the setup script</li>
                </ol>
            </section>
        </article>
    </div>
</body>
</html>`

	captureReq := CaptureRequest{
		URL:         "https://company.atlassian.net/wiki/spaces/PROJ/pages/12345678/Project+Documentation",
		HTML:        confluenceHTML,
		Title:       "Project Documentation - Confluence",
		Description: "Project documentation and guidelines",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	resp, err := helper.POST("/api/documents/capture", captureReq)
	require.NoError(t, err, "Failed to call capture endpoint")

	helper.AssertStatusCode(resp, http.StatusCreated)

	var result CaptureResponse
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse capture response")

	assert.NotEmpty(t, result.DocumentID, "Document ID should not be empty")
	assert.Equal(t, captureReq.URL, result.URL, "URL should match")
	assert.Greater(t, result.ContentSize, 0, "Content should be processed")

	t.Logf("Confluence page capture test passed - DocumentID: %s, ContentSize: %d", result.DocumentID, result.ContentSize)
}

// TestCaptureEndpointJiraTicket tests capturing a JIRA ticket page
func TestCaptureEndpointJiraTicket(t *testing.T) {
	// Start service
	env, err := common.SetupTestEnvironment(t.Name())
	require.NoError(t, err, "Failed to setup test environment")
	defer env.Cleanup()

	helper := env.NewHTTPTestHelper(t)

	// Simulate a JIRA ticket page
	jiraHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>[PROJ-123] Implement user authentication - JIRA</title>
</head>
<body>
    <div id="jira-issue">
        <header>
            <h1>PROJ-123: Implement user authentication</h1>
            <span class="status">In Progress</span>
            <span class="priority">High</span>
        </header>
        <div class="issue-details">
            <div class="description">
                <h3>Description</h3>
                <p>Implement OAuth2 authentication flow for the application.</p>
                <h4>Acceptance Criteria:</h4>
                <ul>
                    <li>Users can sign in with Google</li>
                    <li>Users can sign in with GitHub</li>
                    <li>Session management is implemented</li>
                </ul>
            </div>
            <div class="comments">
                <h3>Comments</h3>
                <div class="comment">
                    <span class="author">Jane Smith</span>
                    <span class="date">2024-01-10</span>
                    <p>Started working on the Google OAuth integration.</p>
                </div>
            </div>
        </div>
    </div>
</body>
</html>`

	captureReq := CaptureRequest{
		URL:         "https://company.atlassian.net/browse/PROJ-123",
		HTML:        jiraHTML,
		Title:       "[PROJ-123] Implement user authentication - JIRA",
		Description: "JIRA ticket for implementing user authentication",
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	resp, err := helper.POST("/api/documents/capture", captureReq)
	require.NoError(t, err, "Failed to call capture endpoint")

	helper.AssertStatusCode(resp, http.StatusCreated)

	var result CaptureResponse
	err = helper.ParseJSONResponse(resp, &result)
	require.NoError(t, err, "Failed to parse capture response")

	assert.NotEmpty(t, result.DocumentID, "Document ID should not be empty")
	assert.Contains(t, result.Title, "PROJ-123", "Title should contain ticket ID")

	t.Logf("JIRA ticket capture test passed - DocumentID: %s", result.DocumentID)
}
