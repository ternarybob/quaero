package unit

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

func setupJiraTestEnv(t *testing.T) (*bolt.DB, interfaces.AtlassianAuthService, *httptest.Server, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	logger := arbor.NewLogger()
	authService, err := atlassian.NewAtlassianAuthService(db, logger)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/project" {
			projects := []map[string]interface{}{
				{
					"key":  "TEST",
					"name": "Test Project",
					"id":   "10001",
				},
			}
			json.NewEncoder(w).Encode(projects)
		} else if r.URL.Path == "/rest/api/3/search/jql" {
			response := map[string]interface{}{
				"issues": []map[string]interface{}{},
				"isLast": true,
			}
			json.NewEncoder(w).Encode(response)
		}
	}))

	authData := &interfaces.AtlassianAuthData{
		BaseURL:   testServer.URL,
		UserAgent: "TestAgent/1.0",
		Cookies:   []*interfaces.AtlassianExtensionCookie{},
		Tokens:    map[string]interface{}{},
	}
	authService.UpdateAuth(authData)

	cleanup := func() {
		testServer.Close()
		db.Close()
	}

	return db, authService, testServer, cleanup
}

func TestNewJiraScraperService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully creates jira scraper",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, authService, _, cleanup := setupJiraTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, err := atlassian.NewJiraScraperService(db, authService, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewJiraScraperService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && service == nil {
				t.Error("Expected non-nil service")
			}

			if !tt.wantErr {
				service.Close()
			}
		})
	}
}

func TestScrapeProjects(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully scrapes projects",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, authService, _, cleanup := setupJiraTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, err := atlassian.NewJiraScraperService(db, authService, logger)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}
			defer service.Close()

			err = service.ScrapeProjects()

			if (err != nil) != tt.wantErr {
				t.Errorf("ScrapeProjects() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				count := service.GetProjectCount()
				if count == 0 {
					t.Error("Expected projects to be scraped")
				}
			}
		})
	}
}

func TestGetProjectCount(t *testing.T) {
	tests := []struct {
		name          string
		scrapeFirst   bool
		expectedCount int
	}{
		{
			name:          "returns 0 when no projects scraped",
			scrapeFirst:   false,
			expectedCount: 0,
		},
		{
			name:          "returns count after scraping",
			scrapeFirst:   true,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, authService, _, cleanup := setupJiraTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, _ := atlassian.NewJiraScraperService(db, authService, logger)
			defer service.Close()

			if tt.scrapeFirst {
				service.ScrapeProjects()
			}

			count := service.GetProjectCount()

			if count != tt.expectedCount {
				t.Errorf("GetProjectCount() = %v, want %v", count, tt.expectedCount)
			}
		})
	}
}

func TestClearProjectsCache(t *testing.T) {
	db, authService, _, cleanup := setupJiraTestEnv(t)
	defer cleanup()

	logger := arbor.NewLogger()
	service, _ := atlassian.NewJiraScraperService(db, authService, logger)
	defer service.Close()

	service.ScrapeProjects()

	initialCount := service.GetProjectCount()
	if initialCount == 0 {
		t.Fatal("Expected projects to be scraped")
	}

	err := service.ClearProjectsCache()
	if err != nil {
		t.Errorf("ClearProjectsCache() error = %v", err)
	}

	finalCount := service.GetProjectCount()
	if finalCount != 0 {
		t.Errorf("GetProjectCount() after clear = %v, want 0", finalCount)
	}
}
