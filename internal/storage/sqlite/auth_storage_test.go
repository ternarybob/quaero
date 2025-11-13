package sqlite

import (
	"context"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreCredentials_WithAPIKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Test storing API key credentials
	creds := &models.AuthCredentials{
		Name:        "test-api-key",
		SiteDomain:  "", // Empty site domain for API keys
		ServiceType: "google-places",
		APIKey:      "test-api-key-value",
		AuthType:    "api_key",
		Data:        map[string]interface{}{"description": "Test API key"},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Retrieve by ID
	stored, err := storage.GetCredentialsByID(context.Background(), creds.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored)
	assert.Equal(t, "test-api-key", stored.Name)
	assert.Equal(t, "api_key", stored.AuthType)
	assert.Equal(t, "test-api-key-value", stored.APIKey)
	assert.Empty(t, stored.SiteDomain) // Should be empty for API keys
	assert.Empty(t, stored.BaseURL)    // Should be empty when not provided

	// Test storing with site domain - should auto-generate BaseURL
	creds2 := &models.AuthCredentials{
		Name:        "test-with-domain",
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		AuthType:    "cookie",
		Data:        map[string]interface{}{},
	}

	err = storage.StoreCredentials(context.Background(), creds2)
	require.NoError(t, err)

	// Retrieve and verify BaseURL was auto-generated
	stored2, err := storage.GetCredentialsByID(context.Background(), creds2.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored2)
	assert.Equal(t, "example.com", stored2.SiteDomain)
	assert.Equal(t, "https://example.com", stored2.BaseURL) // Should be auto-generated
}

func TestGetCredentialsByName_WithAuthType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create cookie credential
	cookieCreds := &models.AuthCredentials{
		Name:        "my-site",
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		AuthType:    "cookie",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), cookieCreds)
	require.NoError(t, err)

	// Create API key credential with same name
	apiKeyCreds := &models.AuthCredentials{
		Name:        "my-api-key",
		SiteDomain:  "",
		ServiceType: "google-places",
		APIKey:      "api-key-value",
		AuthType:    "api_key",
		Data:        map[string]interface{}{},
	}

	err = storage.StoreCredentials(context.Background(), apiKeyCreds)
	require.NoError(t, err)

	// Test retrieving cookie credential by name
	cookieRetrieved, err := storage.GetCredentialsByName(context.Background(), "my-site")
	require.NoError(t, err)
	assert.NotNil(t, cookieRetrieved)
	assert.Equal(t, "cookie", cookieRetrieved.AuthType)
	assert.Empty(t, cookieRetrieved.APIKey)

	// Test retrieving API key credential by name
	apiKeyRetrieved, err := storage.GetCredentialsByName(context.Background(), "my-api-key")
	require.NoError(t, err)
	assert.NotNil(t, apiKeyRetrieved)
	assert.Equal(t, "api_key", apiKeyRetrieved.AuthType)
	assert.Equal(t, "api-key-value", apiKeyRetrieved.APIKey)
}

func TestGetAPIKeyByName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create API key credential
	creds := &models.AuthCredentials{
		Name:        "test-key",
		SiteDomain:  "",
		ServiceType: "google-places",
		APIKey:      "secret-api-key",
		AuthType:    "api_key",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Retrieve API key
	apiKey, err := storage.GetAPIKeyByName(context.Background(), "test-key")
	require.NoError(t, err)
	assert.Equal(t, "secret-api-key", apiKey)

	// Test with non-existent key
	_, err = storage.GetAPIKeyByName(context.Background(), "non-existent")
	assert.Error(t, err)

	// Create cookie credential with same name but different site domain
	cookieCreds := &models.AuthCredentials{
		Name:        "test-key-cookie", // Use different name to avoid ambiguity
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		AuthType:    "cookie",
		Data:        map[string]interface{}{},
	}

	err = storage.StoreCredentials(context.Background(), cookieCreds)
	require.NoError(t, err)

	// Test that we can still retrieve API key by name even after creating cookie credential with different name
	_, err = storage.GetAPIKeyByName(context.Background(), "test-key")
	assert.NoError(t, err) // Should succeed because there's an API key with this name

	// Test that we can't retrieve cookie credential as API key
	_, err = storage.GetAPIKeyByName(context.Background(), "test-key-cookie")
	assert.Error(t, err) // Should fail because it's not an API key entry
}

func TestResolveAPIKey(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create API key credential
	creds := &models.AuthCredentials{
		Name:        "google-places-key",
		SiteDomain:  "",
		ServiceType: "google-places",
		APIKey:      "resolved-api-key",
		AuthType:    "api_key",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Test ResolveAPIKey helper
	// This would be called through common.ResolveAPIKey in production
	// but we test it through the storage layer
	apiKey, err := storage.GetAPIKeyByName(context.Background(), "google-places-key")
	require.NoError(t, err)
	assert.Equal(t, "resolved-api-key", apiKey)
}

func TestListCredentials_IncludesAuthType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create multiple credentials with different auth types
	credentials := []*models.AuthCredentials{
		{
			Name:        "cookie-1",
			SiteDomain:  "example.com",
			ServiceType: "atlassian",
			AuthType:    "cookie",
			Data:        map[string]interface{}{},
		},
		{
			Name:        "api-key-1",
			SiteDomain:  "",
			ServiceType: "google-places",
			APIKey:      "key1",
			AuthType:    "api_key",
			Data:        map[string]interface{}{},
		},
		{
			Name:        "api-key-2",
			SiteDomain:  "",
			ServiceType: "gemini-llm",
			APIKey:      "key2",
			AuthType:    "api_key",
			Data:        map[string]interface{}{},
		},
	}

	for _, cred := range credentials {
		err := storage.StoreCredentials(context.Background(), cred)
		require.NoError(t, err)
	}

	// List all credentials
	all, err := storage.ListCredentials(context.Background())
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Verify all have auth_type set
	for _, cred := range all {
		assert.NotEmpty(t, cred.AuthType, "auth_type should be set for all credentials")
	}
}
