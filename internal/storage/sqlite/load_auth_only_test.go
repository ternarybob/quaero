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

func TestLoadAuthCredsFromTOML_WithCookieBasedAuth(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with cookie-based auth sections
	testTOML := `[atlassian-site]
name = "Bob's Atlassian"
site_domain = "bobmcallan.atlassian.net"
service_type = "atlassian"
base_url = "https://bobmcallan.atlassian.net"
user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"

[github-enterprise]
name = "GitHub Enterprise"
site_domain = "github.example.com"
service_type = "github"
base_url = "https://github.example.com"
tokens = { "access_token" = "gho_test123" }
data = { "region" = "us-east-1" }
`

	testFile := filepath.Join(tmpDir, "test-auth.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load TOML file
	sections, err := mgr.loadAuthCredsFromTOML(testFile)
	require.NoError(t, err)
	assert.Len(t, sections, 2)

	// Verify atlassian-site section
	atlassian, ok := sections["atlassian-site"]
	require.True(t, ok, "atlassian-site section should exist")
	assert.Equal(t, "Bob's Atlassian", atlassian.Name)
	assert.Equal(t, "bobmcallan.atlassian.net", atlassian.SiteDomain)
	assert.Equal(t, "atlassian", atlassian.ServiceType)
	assert.Equal(t, "https://bobmcallan.atlassian.net", atlassian.BaseURL)
	assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", atlassian.UserAgent)

	// Verify github-enterprise section
	github, ok := sections["github-enterprise"]
	require.True(t, ok, "github-enterprise section should exist")
	assert.Equal(t, "GitHub Enterprise", github.Name)
	assert.Equal(t, "github.example.com", github.SiteDomain)
	assert.Equal(t, "github", github.ServiceType)
	assert.Equal(t, "https://github.example.com", github.BaseURL)
	assert.Equal(t, "gho_test123", github.Tokens["access_token"])
	assert.Equal(t, "us-east-1", github.Data["region"])
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
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load TOML file - should fail with "no sections found"
	_, err = mgr.loadAuthCredsFromTOML(testFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sections found")
}

func TestLoadAuthCredentialsFromFiles_StoresInAuthTable(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with cookie-based auth sections
	testTOML := `[test-auth-1]
name = "Test Auth 1"
site_domain = "test1.example.com"
service_type = "test-service-1"
base_url = "https://test1.example.com"

[test-auth-2]
name = "Test Auth 2"
site_domain = "test2.example.com"
service_type = "test-service-2"
base_url = "https://test2.example.com"
tokens = { "session_token" = "abc123" }
`

	testFile := filepath.Join(tmpDir, "test-auth.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load auth credentials from directory
	err = mgr.LoadAuthCredentialsFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify credentials were stored in auth_credentials table
	credentials, err := mgr.auth.ListCredentials(context.Background())
	require.NoError(t, err)
	assert.Len(t, credentials, 2)

	// Verify test-auth-1
	var found1, found2 bool
	for _, cred := range credentials {
		if cred.Name == "Test Auth 1" {
			assert.Equal(t, "test1.example.com", cred.SiteDomain)
			assert.Equal(t, "test-service-1", cred.ServiceType)
			assert.Equal(t, "https://test1.example.com", cred.BaseURL)
			found1 = true
		}
		if cred.Name == "Test Auth 2" {
			assert.Equal(t, "test2.example.com", cred.SiteDomain)
			assert.Equal(t, "test-service-2", cred.ServiceType)
			assert.Equal(t, "https://test2.example.com", cred.BaseURL)
			assert.Equal(t, "abc123", cred.Tokens["session_token"])
			found2 = true
		}
	}
	assert.True(t, found1, "Test Auth 1 should exist in auth_credentials")
	assert.True(t, found2, "Test Auth 2 should exist in auth_credentials")
}

func TestLoadAuthCredentialsFromFiles_SkipsAPIKeySections(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with mixed cookie-based auth and API key sections
	testTOML := `[valid-cookie-auth]
name = "Valid Cookie Auth"
site_domain = "valid.example.com"
service_type = "test-service"
base_url = "https://valid.example.com"

[invalid-api-key-section]
api_key = "sk-test-1234567890"
service_type = "openai"
description = "This should be skipped"

[another-valid-auth]
name = "Another Valid Auth"
site_domain = "another.example.com"
service_type = "test-service"
`

	testFile := filepath.Join(tmpDir, "mixed-auth.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load auth credentials from directory
	err = mgr.LoadAuthCredentialsFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify only cookie-based auth sections were stored (API key section skipped)
	credentials, err := mgr.auth.ListCredentials(context.Background())
	require.NoError(t, err)
	assert.Len(t, credentials, 2, "Only 2 cookie-based auth sections should be stored")

	// Verify API key section was NOT stored in auth_credentials
	for _, cred := range credentials {
		assert.NotEqual(t, "This should be skipped", cred.Name, "API key section should have been skipped")
	}

	// Verify the valid sections were stored
	var foundValid1, foundValid2 bool
	for _, cred := range credentials {
		if cred.Name == "Valid Cookie Auth" {
			foundValid1 = true
		}
		if cred.Name == "Another Valid Auth" {
			foundValid2 = true
		}
	}
	assert.True(t, foundValid1, "Valid Cookie Auth should exist")
	assert.True(t, foundValid2, "Another Valid Auth should exist")

	// Verify API key was NOT stored in KV store either (should be in ./keys directory)
	_, err = mgr.kv.Get(context.Background(), "invalid-api-key-section")
	assert.Error(t, err, "API key should not be in KV store")
}

func TestLoadAuthCredentialsFromFiles_WithTokensAndData(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with tokens and data fields
	testTOML := `[auth-with-tokens]
name = "Auth With Tokens"
site_domain = "tokens.example.com"
service_type = "test-service"
base_url = "https://tokens.example.com"
tokens = { "access_token" = "xyz123", "refresh_token" = "abc456" }
data = { "region" = "us-west-2", "environment" = "production", "user_id" = "12345" }
`

	testFile := filepath.Join(tmpDir, "tokens-auth.toml")
	err := os.WriteFile(testFile, []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load auth credentials from directory
	err = mgr.LoadAuthCredentialsFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify credentials with tokens and data fields
	credentials, err := mgr.auth.ListCredentials(context.Background())
	require.NoError(t, err)
	require.Len(t, credentials, 1)

	cred := credentials[0]
	assert.Equal(t, "Auth With Tokens", cred.Name)
	assert.Equal(t, "tokens.example.com", cred.SiteDomain)

	// Verify tokens field
	require.NotNil(t, cred.Tokens)
	assert.Equal(t, "xyz123", cred.Tokens["access_token"])
	assert.Equal(t, "abc456", cred.Tokens["refresh_token"])

	// Verify data field
	require.NotNil(t, cred.Data)
	assert.Equal(t, "us-west-2", cred.Data["region"])
	assert.Equal(t, "production", cred.Data["environment"])
	assert.Equal(t, "12345", cred.Data["user_id"])
}

func TestLoadAuthCredentialsFromFiles_DirectoryNotFound(t *testing.T) {
	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Try to load from non-existent directory - should not error
	err := mgr.LoadAuthCredentialsFromFiles(context.Background(), "/non/existent/path")
	assert.NoError(t, err, "Missing directory should not be an error")
}

func TestLoadAuthCredentialsFromFiles_SkipsNonTOML(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create non-TOML files
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("not toml"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte("{}"), 0644)
	require.NoError(t, err)

	// Create valid TOML file
	testTOML := `[valid-auth]
name = "Valid Auth"
site_domain = "valid.example.com"
service_type = "test-service"
`
	err = os.WriteFile(filepath.Join(tmpDir, "valid.toml"), []byte(testTOML), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load auth credentials from directory
	err = mgr.LoadAuthCredentialsFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify only TOML file was processed
	credentials, err := mgr.auth.ListCredentials(context.Background())
	require.NoError(t, err)
	assert.Len(t, credentials, 1)
	assert.Equal(t, "Valid Auth", credentials[0].Name)
}

func TestValidateAuthCredentialFile(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		auth:   NewAuthStorage(db, logger),
		kv:     NewKVStorage(db, logger),
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
			name: "valid credentials with site_domain",
			authFile: &AuthCredentialFile{
				Name:        "Test Auth",
				SiteDomain:  "example.com",
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "valid credentials with base_url",
			authFile: &AuthCredentialFile{
				Name:        "Test Auth",
				BaseURL:     "https://example.com",
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "valid credentials with both site_domain and base_url",
			authFile: &AuthCredentialFile{
				Name:        "Test Auth",
				SiteDomain:  "example.com",
				BaseURL:     "https://example.com",
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "missing section name",
			authFile: &AuthCredentialFile{
				Name:       "Test Auth",
				SiteDomain: "example.com",
			},
			sectionName: "",
			expectError: true,
			errorMsg:    "section name is required",
		},
		{
			name: "missing name",
			authFile: &AuthCredentialFile{
				SiteDomain:  "example.com",
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "missing both site_domain and base_url",
			authFile: &AuthCredentialFile{
				Name:        "Test Auth",
				ServiceType: "test-service",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "either site_domain or base_url is required",
		},
		{
			name: "service_type is optional",
			authFile: &AuthCredentialFile{
				Name:       "Test Auth",
				SiteDomain: "example.com",
				// ServiceType omitted intentionally
			},
			sectionName: "test-section",
			expectError: false,
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
