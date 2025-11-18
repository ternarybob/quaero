package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

func TestStoreCredentials_CookieBased(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Test storing cookie-based credentials with site domain
	creds := &models.AuthCredentials{
		Name:        "test-with-domain",
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		Data:        map[string]interface{}{"description": "Test credentials"},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Retrieve and verify BaseURL was auto-generated
	stored, err := storage.GetCredentialsByID(context.Background(), creds.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored)
	assert.Equal(t, "example.com", stored.SiteDomain)
	assert.Equal(t, "https://example.com", stored.BaseURL) // Should be auto-generated
	assert.Equal(t, "test-with-domain", stored.Name)
}

func TestStoreCredentials_WithBaseURL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Test storing credentials with explicit BaseURL
	creds := &models.AuthCredentials{
		Name:        "test-with-baseurl",
		BaseURL:     "https://mysite.example.com",
		ServiceType: "atlassian",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Retrieve and verify SiteDomain was extracted from BaseURL
	stored, err := storage.GetCredentialsByID(context.Background(), creds.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored)
	assert.Equal(t, "mysite.example.com", stored.SiteDomain) // Should be extracted
	assert.Equal(t, "https://mysite.example.com", stored.BaseURL)
}

func TestListCredentials(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create multiple cookie-based credentials
	credentials := []*models.AuthCredentials{
		{
			Name:        "cookie-1",
			SiteDomain:  "example.com",
			ServiceType: "atlassian",
			Data:        map[string]interface{}{},
		},
		{
			Name:        "cookie-2",
			SiteDomain:  "test.com",
			ServiceType: "github",
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
	assert.Len(t, all, 2)

	// Verify all are cookie-based
	for _, cred := range all {
		assert.NotEmpty(t, cred.SiteDomain, "site_domain should be set for cookie-based credentials")
		assert.NotEmpty(t, cred.Name, "name should be set")
	}
}

func TestGetCredentialsBySiteDomain(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create cookie credential
	creds := &models.AuthCredentials{
		Name:        "my-site",
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Test retrieving by site domain
	retrieved, err := storage.GetCredentialsBySiteDomain(context.Background(), "example.com")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "my-site", retrieved.Name)
	assert.Equal(t, "example.com", retrieved.SiteDomain)
}

func TestDeleteCredentials(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := arbor.NewLogger()
	storage := NewAuthStorage(db, logger)

	// Create credential
	creds := &models.AuthCredentials{
		Name:        "to-delete",
		SiteDomain:  "example.com",
		ServiceType: "atlassian",
		Data:        map[string]interface{}{},
	}

	err := storage.StoreCredentials(context.Background(), creds)
	require.NoError(t, err)

	// Delete it
	err = storage.DeleteCredentials(context.Background(), creds.ID)
	require.NoError(t, err)

	// Verify it's gone
	retrieved, err := storage.GetCredentialsByID(context.Background(), creds.ID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}
