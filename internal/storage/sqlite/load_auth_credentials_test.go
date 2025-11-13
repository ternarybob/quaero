package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
)

func TestLoadAuthCredsFromTOML_WithSections(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with sections
	testTOML := `[google-places-key]
api_key = "AIzaTest_FakeKey1234567890ABCDEFGHIJKLMNO"
service_type = "google-places"
description = "Google Places API key for location search functionality"

[gemini-llm-key]
api_key = "AIzaTest_FakeKey0987654321ZYXWVUTSRQPONML"
service_type = "gemini-llm"
description = "Google Gemini API key for LLM features"
`

	testFile := filepath.Join(tmpDir, "test-keys.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		logger: logger,
	}

	// Load TOML file
	sections, err := mgr.loadAuthCredsFromTOML(testFile)
	require.NoError(t, err)
	assert.Len(t, sections, 2)

	// Verify google-places-key section
	googlePlaces, ok := sections["google-places-key"]
	require.True(t, ok, "google-places-key section should exist")
	assert.Equal(t, "AIzaTest_FakeKey1234567890ABCDEFGHIJKLMNO", googlePlaces.APIKey)
	assert.Equal(t, "google-places", googlePlaces.ServiceType)
	assert.Equal(t, "Google Places API key for location search functionality", googlePlaces.Description)

	// Verify gemini-llm-key section
	geminiLLM, ok := sections["gemini-llm-key"]
	require.True(t, ok, "gemini-llm-key section should exist")
	assert.Equal(t, "AIzaTest_FakeKey0987654321ZYXWVUTSRQPONML", geminiLLM.APIKey)
	assert.Equal(t, "gemini-llm", geminiLLM.ServiceType)
	assert.Equal(t, "Google Gemini API key for LLM features", geminiLLM.Description)
}

func TestLoadAuthCredsFromTOML_EmptyFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create empty TOML file
	testFile := filepath.Join(tmpDir, "empty.toml")
	err := os.WriteFile(testFile, []byte(""), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		logger: logger,
	}

	// Load TOML file - should fail with "no sections found"
	_, err = mgr.loadAuthCredsFromTOML(testFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sections found")
}

func TestToAuthCredentials_WithSectionName(t *testing.T) {
	authFile := &AuthCredentialFile{
		APIKey:      "test-api-key-value",
		ServiceType: "google-places",
		Description: "Test description",
	}

	// Convert to AuthCredentials with section name
	creds := authFile.ToAuthCredentials("my-api-key")

	assert.Equal(t, "my-api-key", creds.Name)
	assert.Equal(t, "test-api-key-value", creds.APIKey)
	assert.Equal(t, "google-places", creds.ServiceType)
	assert.Equal(t, "api_key", creds.AuthType)
	assert.Empty(t, creds.SiteDomain)
	assert.NotEmpty(t, creds.ID)
	assert.NotZero(t, creds.CreatedAt)
	assert.NotZero(t, creds.UpdatedAt)

	// Verify description in Data map
	desc, ok := creds.Data["description"].(string)
	require.True(t, ok)
	assert.Equal(t, "Test description", desc)
}

func TestLoadAuthCredentialsFromFiles_WithSections(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with sections
	testTOML := `[test-key-1]
api_key = "key-value-1"
service_type = "service-1"
description = "Test key 1"

[test-key-2]
api_key = "key-value-2"
service_type = "service-2"
description = "Test key 2"
`

	testFile := filepath.Join(tmpDir, "test-keys.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		logger: logger,
	}

	// Load auth credentials from directory
	err = mgr.LoadAuthCredentialsFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify credentials were stored
	creds1, err := mgr.auth.GetCredentialsByName(context.Background(), "test-key-1")
	require.NoError(t, err)
	assert.Equal(t, "key-value-1", creds1.APIKey)
	assert.Equal(t, "service-1", creds1.ServiceType)
	assert.Equal(t, "api_key", creds1.AuthType)

	creds2, err := mgr.auth.GetCredentialsByName(context.Background(), "test-key-2")
	require.NoError(t, err)
	assert.Equal(t, "key-value-2", creds2.APIKey)
	assert.Equal(t, "service-2", creds2.ServiceType)
	assert.Equal(t, "api_key", creds2.AuthType)
}

func TestValidateAuthCredentialFile(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		logger: logger,
	}

	tests := []struct {
		name        string
		authFile    *AuthCredentialFile
		sectionName string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid credentials",
			authFile: &AuthCredentialFile{
				APIKey:      "test-key",
				ServiceType: "test-service",
				Description: "test description",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "missing section name",
			authFile: &AuthCredentialFile{
				APIKey:      "test-key",
				ServiceType: "test-service",
			},
			sectionName: "",
			expectError: true,
			errorMsg:    "section name is required",
		},
		{
			name: "missing api_key",
			authFile: &AuthCredentialFile{
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "api_key is required",
		},
		{
			name: "missing service_type",
			authFile: &AuthCredentialFile{
				APIKey: "test-key",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "service_type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validateAuthCredentialFile(tt.authFile, tt.sectionName)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
