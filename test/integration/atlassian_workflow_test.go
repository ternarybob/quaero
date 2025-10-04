package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	bolt "go.etcd.io/bbolt"
)

// TestFullAtlassianWorkflow tests the complete workflow from auth to scraping
func TestFullAtlassianWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	logger := arbor.NewLogger()

	testServer := createMockAtlassianServer()
	defer testServer.Close()

	authService, err := atlassian.NewAtlassianAuthService(db, logger)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	authData := &interfaces.AtlassianAuthData{
		BaseURL:   testServer.URL,
		UserAgent: "TestAgent/1.0",
		Cookies: []*interfaces.AtlassianExtensionCookie{
			{
				Name:   "test-cookie",
				Value:  "test-value",
				Domain: ".atlassian.net",
				Path:   "/",
			},
		},
		Tokens: map[string]interface{}{
			"cloudId":  "test-cloud-id",
			"atlToken": "test-atl-token",
		},
		Timestamp: 1234567890,
	}

	if err := authService.UpdateAuth(authData); err != nil {
		t.Fatalf("Failed to update auth: %v", err)
	}

	if !authService.IsAuthenticated() {
		t.Error("Expected service to be authenticated")
	}

	jiraService, err := atlassian.NewJiraScraperService(db, authService, logger)
	if err != nil {
		t.Fatalf("Failed to create Jira service: %v", err)
	}
	defer jiraService.Close()

	if err := jiraService.ScrapeProjects(); err != nil {
		t.Errorf("Failed to scrape projects: %v", err)
	}

	projectCount := jiraService.GetProjectCount()
	if projectCount != 2 {
		t.Errorf("Expected 2 projects, got %d", projectCount)
	}

	confluenceService, err := atlassian.NewConfluenceScraperService(db, authService, logger)
	if err != nil {
		t.Fatalf("Failed to create Confluence service: %v", err)
	}
	defer confluenceService.Close()

	if err := confluenceService.ScrapeSpaces(); err != nil {
		t.Errorf("Failed to scrape spaces: %v", err)
	}

	spaceCount := confluenceService.GetSpaceCount()
	if spaceCount != 2 {
		t.Errorf("Expected 2 spaces, got %d", spaceCount)
	}

	jiraData, err := jiraService.GetJiraData()
	if err != nil {
		t.Errorf("Failed to get Jira data: %v", err)
	}

	projects, ok := jiraData["projects"].([]map[string]interface{})
	if !ok || len(projects) != 2 {
		t.Errorf("Expected 2 projects in data, got %v", jiraData["projects"])
	}

	confluenceData, err := confluenceService.GetConfluenceData()
	if err != nil {
		t.Errorf("Failed to get Confluence data: %v", err)
	}

	spaces, ok := confluenceData["spaces"].([]map[string]interface{})
	if !ok || len(spaces) != 2 {
		t.Errorf("Expected 2 spaces in data, got %v", confluenceData["spaces"])
	}

	if err := jiraService.ClearAllData(); err != nil {
		t.Errorf("Failed to clear Jira data: %v", err)
	}

	if jiraService.GetProjectCount() != 0 {
		t.Error("Expected projects to be cleared")
	}

	if err := confluenceService.ClearAllData(); err != nil {
		t.Errorf("Failed to clear Confluence data: %v", err)
	}

	if confluenceService.GetSpaceCount() != 0 {
		t.Error("Expected spaces to be cleared")
	}

	loadedAuth, err := authService.LoadAuth()
	if err != nil {
		t.Errorf("Failed to load auth: %v", err)
	}

	if loadedAuth.BaseURL != authData.BaseURL {
		t.Errorf("Expected BaseURL %s, got %s", authData.BaseURL, loadedAuth.BaseURL)
	}
}

func createMockAtlassianServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/project":
			projects := []map[string]interface{}{
				{"key": "TEST1", "name": "Test Project 1", "id": "10001"},
				{"key": "TEST2", "name": "Test Project 2", "id": "10002"},
			}
			json.NewEncoder(w).Encode(projects)

		case "/rest/api/3/search/jql":
			response := map[string]interface{}{
				"issues": []map[string]interface{}{},
				"isLast": true,
			}
			json.NewEncoder(w).Encode(response)

		case "/wiki/rest/api/space":
			spaces := struct {
				Results []map[string]interface{} `json:"results"`
				Size    int                      `json:"size"`
			}{
				Results: []map[string]interface{}{
					{"key": "SPACE1", "name": "Space 1", "id": "20001"},
					{"key": "SPACE2", "name": "Space 2", "id": "20002"},
				},
				Size: 2,
			}
			json.NewEncoder(w).Encode(spaces)

		case "/wiki/rest/api/content":
			response := struct {
				Size    int                      `json:"size"`
				Total   int                      `json:"total"`
				Results []map[string]interface{} `json:"results"`
			}{
				Size:    0,
				Total:   0,
				Results: []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
