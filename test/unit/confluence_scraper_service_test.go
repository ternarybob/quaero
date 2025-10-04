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

func setupConfluenceTestEnv(t *testing.T) (*bolt.DB, interfaces.AtlassianAuthService, *httptest.Server, func()) {
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
		if r.URL.Path == "/wiki/rest/api/space" {
			spaces := struct {
				Results []map[string]interface{} `json:"results"`
				Size    int                      `json:"size"`
			}{
				Results: []map[string]interface{}{
					{
						"key":  "TEST",
						"name": "Test Space",
						"id":   "10001",
					},
				},
				Size: 1,
			}
			json.NewEncoder(w).Encode(spaces)
		} else if r.URL.Path == "/wiki/rest/api/content" {
			response := struct {
				Size  int `json:"size"`
				Total int `json:"total"`
			}{
				Size:  0,
				Total: 0,
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

func TestNewConfluenceScraperService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully creates confluence scraper",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, authService, _, cleanup := setupConfluenceTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, err := atlassian.NewConfluenceScraperService(db, authService, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfluenceScraperService() error = %v, wantErr %v", err, tt.wantErr)
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

func TestScrapeSpaces(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully scrapes spaces",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, authService, _, cleanup := setupConfluenceTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, err := atlassian.NewConfluenceScraperService(db, authService, logger)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}
			defer service.Close()

			err = service.ScrapeSpaces()

			if (err != nil) != tt.wantErr {
				t.Errorf("ScrapeSpaces() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				count := service.GetSpaceCount()
				if count == 0 {
					t.Error("Expected spaces to be scraped")
				}
			}
		})
	}
}

func TestGetSpaceCount(t *testing.T) {
	tests := []struct {
		name          string
		scrapeFirst   bool
		expectedCount int
	}{
		{
			name:          "returns 0 when no spaces scraped",
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
			db, authService, _, cleanup := setupConfluenceTestEnv(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, _ := atlassian.NewConfluenceScraperService(db, authService, logger)
			defer service.Close()

			if tt.scrapeFirst {
				service.ScrapeSpaces()
			}

			count := service.GetSpaceCount()

			if count != tt.expectedCount {
				t.Errorf("GetSpaceCount() = %v, want %v", count, tt.expectedCount)
			}
		})
	}
}

func TestClearSpacesCache(t *testing.T) {
	db, authService, _, cleanup := setupConfluenceTestEnv(t)
	defer cleanup()

	logger := arbor.NewLogger()
	service, _ := atlassian.NewConfluenceScraperService(db, authService, logger)
	defer service.Close()

	service.ScrapeSpaces()

	initialCount := service.GetSpaceCount()
	if initialCount == 0 {
		t.Fatal("Expected spaces to be scraped")
	}

	err := service.ClearSpacesCache()
	if err != nil {
		t.Errorf("ClearSpacesCache() error = %v", err)
	}

	finalCount := service.GetSpaceCount()
	if finalCount != 0 {
		t.Errorf("GetSpaceCount() after clear = %v, want 0", finalCount)
	}
}
