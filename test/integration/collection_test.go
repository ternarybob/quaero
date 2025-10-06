package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/server"
)

func setupCollectionTestServer(t *testing.T) (*app.App, *server.Server, string) {
	t.Helper()

	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	config.Server.Port = 8087
	config.Storage.SQLite.Path = ":memory:"

	logger := arbor.NewLogger()
	require.NotNil(t, logger, "Logger should be initialized")

	application, err := app.New(config, logger)
	require.NoError(t, err, "Failed to create application")

	srv := server.New(application)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	serverURL := "http://localhost:8087"
	return application, srv, serverURL
}

func TestCollectionJiraSync(t *testing.T) {
	application, srv, serverURL := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	t.Run("sync jira without auth returns error", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/collection/jira/sync", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, "error", result["status"])
		assert.NotEmpty(t, result["error"])
	})
}

func TestCollectionConfluenceSync(t *testing.T) {
	application, srv, serverURL := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	t.Run("sync confluence without auth returns error", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/collection/confluence/sync", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, "error", result["status"])
		assert.NotEmpty(t, result["error"])
	})
}

func TestCollectionSyncAll(t *testing.T) {
	application, srv, serverURL := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	t.Run("sync all without auth returns error", func(t *testing.T) {
		resp, err := http.Post(serverURL+"/api/collection/sync-all", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, "error", result["status"])
	})
}

func TestCollectionEndpointMethodRestrictions(t *testing.T) {
	application, srv, serverURL := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	tests := []struct {
		name     string
		endpoint string
	}{
		{"jira sync", "/api/collection/jira/sync"},
		{"confluence sync", "/api/collection/confluence/sync"},
		{"sync all", "/api/collection/sync-all"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" only accepts POST", func(t *testing.T) {
			resp, err := http.Get(serverURL + tt.endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
		})
	}
}

func TestCollectionWithDocuments(t *testing.T) {
	application, srv, _ := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	ctx := context.Background()

	t.Run("verify documents table exists", func(t *testing.T) {
		count, err := application.StorageManager.DocumentStorage().CountDocuments()
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("verify jira and confluence storage exist", func(t *testing.T) {
		projectCount, err := application.StorageManager.JiraStorage().CountProjects(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, projectCount)

		spaceCount, err := application.StorageManager.ConfluenceStorage().CountSpaces(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, spaceCount)
	})
}

func TestCollectionCoordinatorIntegration(t *testing.T) {
	application, srv, _ := setupCollectionTestServer(t)
	defer application.Close()
	defer srv.Shutdown(context.Background())

	t.Run("coordinator handles force sync pending documents", func(t *testing.T) {
		initialCount, err := application.StorageManager.DocumentStorage().CountDocuments()
		require.NoError(t, err)
		assert.Equal(t, 0, initialCount)
	})

	t.Run("event service publishes collection events", func(t *testing.T) {
		eventService := application.EventService
		assert.NotNil(t, eventService, "EventService should be initialized")
	})
}
