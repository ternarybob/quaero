package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/services/atlassian"
	bolt "go.etcd.io/bbolt"
)

func setupTestDB(t *testing.T) (*bolt.DB, func()) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestNewAtlassianAuthService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully creates auth service",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			logger := arbor.NewLogger()

			service, err := atlassian.NewAtlassianAuthService(db, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewAtlassianAuthService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && service == nil {
				t.Error("Expected non-nil service")
			}
		})
	}
}

func TestUpdateAuth(t *testing.T) {
	tests := []struct {
		name     string
		authData *interfaces.AtlassianAuthData
		wantErr  bool
	}{
		{
			name: "successfully updates auth with valid data",
			authData: &interfaces.AtlassianAuthData{
				BaseURL:   "https://test.atlassian.net",
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
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, err := atlassian.NewAtlassianAuthService(db, logger)
			if err != nil {
				t.Fatalf("Failed to create service: %v", err)
			}

			err = service.UpdateAuth(tt.authData)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAuth() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if service.GetBaseURL() != tt.authData.BaseURL {
					t.Errorf("GetBaseURL() = %v, want %v", service.GetBaseURL(), tt.authData.BaseURL)
				}

				if service.GetUserAgent() != tt.authData.UserAgent {
					t.Errorf("GetUserAgent() = %v, want %v", service.GetUserAgent(), tt.authData.UserAgent)
				}
			}
		})
	}
}

func TestIsAuthenticated(t *testing.T) {
	tests := []struct {
		name           string
		setupAuth      bool
		expectedResult bool
	}{
		{
			name:           "returns false when not authenticated",
			setupAuth:      false,
			expectedResult: false,
		},
		{
			name:           "returns true when authenticated",
			setupAuth:      true,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, _ := atlassian.NewAtlassianAuthService(db, logger)

			if tt.setupAuth {
				authData := &interfaces.AtlassianAuthData{
					BaseURL:   "https://test.atlassian.net",
					UserAgent: "TestAgent/1.0",
					Cookies:   []*interfaces.AtlassianExtensionCookie{},
					Tokens:    map[string]interface{}{},
				}
				service.UpdateAuth(authData)
			}

			result := service.IsAuthenticated()

			if result != tt.expectedResult {
				t.Errorf("IsAuthenticated() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestLoadAuth(t *testing.T) {
	tests := []struct {
		name      string
		setupAuth bool
		wantErr   bool
	}{
		{
			name:      "returns error when no auth stored",
			setupAuth: false,
			wantErr:   true,
		},
		{
			name:      "successfully loads stored auth",
			setupAuth: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := setupTestDB(t)
			defer cleanup()

			logger := arbor.NewLogger()
			service, _ := atlassian.NewAtlassianAuthService(db, logger)

			var originalAuth *interfaces.AtlassianAuthData
			if tt.setupAuth {
				originalAuth = &interfaces.AtlassianAuthData{
					BaseURL:   "https://test.atlassian.net",
					UserAgent: "TestAgent/1.0",
					Cookies:   []*interfaces.AtlassianExtensionCookie{},
					Tokens:    map[string]interface{}{"cloudId": "test-id"},
					Timestamp: 1234567890,
				}
				service.UpdateAuth(originalAuth)
			}

			loadedAuth, err := service.LoadAuth()

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadAuth() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && loadedAuth.BaseURL != originalAuth.BaseURL {
				t.Errorf("LoadAuth() BaseURL = %v, want %v", loadedAuth.BaseURL, originalAuth.BaseURL)
			}
		})
	}
}
