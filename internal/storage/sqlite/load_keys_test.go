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

func TestLoadKeysFromTOML_WithSections(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with sections
	testTOML := `[google-api-key]
value = "AIzaSyABC123TestKey"
description = "Google API key for Gemini"

[github-token]
value = "ghp_xyz789TestToken"
description = "GitHub personal access token"

[database-password]
value = "super-secret-password"
description = "Database password"
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
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load TOML file
	sections, err := mgr.loadKeysFromTOML(testFile)
	require.NoError(t, err)
	assert.Len(t, sections, 3)

	// Verify google-api-key section
	googleAPIKey, ok := sections["google-api-key"]
	require.True(t, ok, "google-api-key section should exist")
	assert.Equal(t, "AIzaSyABC123TestKey", googleAPIKey.Value)
	assert.Equal(t, "Google API key for Gemini", googleAPIKey.Description)

	// Verify github-token section
	githubToken, ok := sections["github-token"]
	require.True(t, ok, "github-token section should exist")
	assert.Equal(t, "ghp_xyz789TestToken", githubToken.Value)
	assert.Equal(t, "GitHub personal access token", githubToken.Description)

	// Verify database-password section
	dbPassword, ok := sections["database-password"]
	require.True(t, ok, "database-password section should exist")
	assert.Equal(t, "super-secret-password", dbPassword.Value)
	assert.Equal(t, "Database password", dbPassword.Description)
}

func TestLoadKeysFromTOML_EmptyFile(t *testing.T) {
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
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load TOML file - should fail with "no sections found"
	_, err = mgr.loadKeysFromTOML(testFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sections found")
}

func TestLoadKeysFromFiles_StoresInKV(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test TOML file with key/value pairs
	testTOML := `[test-key-1]
value = "secret-value-1"
description = "Test key 1"

[test-key-2]
value = "secret-value-2"
description = "Test key 2"

[test-key-3]
value = "secret-value-3"
description = "Test key 3"
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
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load key/value pairs from directory
	err = mgr.LoadKeysFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify keys were stored in KV store
	key1, err := mgr.kv.Get(context.Background(), "test-key-1")
	require.NoError(t, err)
	assert.Equal(t, "secret-value-1", key1)

	key2, err := mgr.kv.Get(context.Background(), "test-key-2")
	require.NoError(t, err)
	assert.Equal(t, "secret-value-2", key2)

	key3, err := mgr.kv.Get(context.Background(), "test-key-3")
	require.NoError(t, err)
	assert.Equal(t, "secret-value-3", key3)

	// Verify description metadata
	entries, err := mgr.kv.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Find test-key-1 and verify description
	var found1, found2, found3 bool
	for _, entry := range entries {
		if entry.Key == "test-key-1" {
			assert.Equal(t, "Test key 1", entry.Description)
			found1 = true
		}
		if entry.Key == "test-key-2" {
			assert.Equal(t, "Test key 2", entry.Description)
			found2 = true
		}
		if entry.Key == "test-key-3" {
			assert.Equal(t, "Test key 3", entry.Description)
			found3 = true
		}
	}
	assert.True(t, found1, "test-key-1 should exist in KV store")
	assert.True(t, found2, "test-key-2 should exist in KV store")
	assert.True(t, found3, "test-key-3 should exist in KV store")
}

func TestLoadKeysFromFiles_DirectoryNotFound(t *testing.T) {
	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Call LoadKeysFromFiles with non-existent directory path
	err := mgr.LoadKeysFromFiles(context.Background(), "/nonexistent/directory/path")
	require.NoError(t, err) // Should not error (graceful degradation)

	// Verify no keys stored in KV store
	entries, err := mgr.kv.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 0, "No keys should be stored when directory doesn't exist")
}

func TestLoadKeysFromFiles_SkipsNonTOML(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create TOML file
	tomlContent := `[valid-key]
value = "valid-value"
description = "Valid key from TOML"
`
	tomlFile := filepath.Join(tmpDir, "keys.toml")
	err := os.WriteFile(tomlFile, []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Create non-TOML files
	txtFile := filepath.Join(tmpDir, "readme.txt")
	err = os.WriteFile(txtFile, []byte("This is a text file"), 0644)
	require.NoError(t, err)

	jsonFile := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(jsonFile, []byte(`{"key": "value"}`), 0644)
	require.NoError(t, err)

	mdFile := filepath.Join(tmpDir, "docs.md")
	err = os.WriteFile(mdFile, []byte("# Documentation"), 0644)
	require.NoError(t, err)

	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	// Load key/value pairs from directory
	err = mgr.LoadKeysFromFiles(context.Background(), tmpDir)
	require.NoError(t, err)

	// Verify only TOML file was processed
	entries, err := mgr.kv.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 1, "Only TOML file should be processed")

	// Verify the valid key exists
	assert.Equal(t, "valid-key", entries[0].Key)
	assert.Equal(t, "valid-value", entries[0].Value)
	assert.Equal(t, "Valid key from TOML", entries[0].Description)
}

func TestValidateKeyValueFile(t *testing.T) {
	// Create manager
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	mgr := &Manager{
		db:     db,
		kv:     NewKVStorage(db, logger),
		logger: logger,
	}

	tests := []struct {
		name        string
		kvFile      *KeyValueFile
		sectionName string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid key/value with description",
			kvFile: &KeyValueFile{
				Value:       "test-value",
				Description: "test description",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "valid key/value without description",
			kvFile: &KeyValueFile{
				Value: "test-value",
			},
			sectionName: "test-section",
			expectError: false,
		},
		{
			name: "missing section name",
			kvFile: &KeyValueFile{
				Value: "test-value",
			},
			sectionName: "",
			expectError: true,
			errorMsg:    "section name is required",
		},
		{
			name: "missing value",
			kvFile: &KeyValueFile{
				Description: "test description",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "value is required",
		},
		{
			name: "empty value",
			kvFile: &KeyValueFile{
				Value:       "",
				Description: "test description",
			},
			sectionName: "test-section",
			expectError: true,
			errorMsg:    "value is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validateKeyValueFile(tt.kvFile, tt.sectionName)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
