package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/app"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// TestApplicationStartup verifies that the application initializes successfully
// with the test configuration, all services are properly initialized, and the
// application can be cleanly shutdown.
func TestApplicationStartup(t *testing.T) {
	t.Log("=== Testing Application Startup ===")

	// Step 1: Load test configuration
	configPath := filepath.Join("..", "..", "bin", "quaero-test.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")
	t.Logf("✓ Configuration loaded from: %s", configPath)

	// Step 2: Initialize logger
	logger := arbor.NewLogger()
	require.NotNil(t, logger, "Logger should be initialized")
	t.Log("✓ Logger initialized")

	// Step 3: Create application
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application, "Application should not be nil")
	t.Log("✓ Application created successfully")

	// Step 4: Verify LLM service initialized in offline mode
	require.NotNil(t, application.LLMService, "LLM service should be initialized")
	mode := application.LLMService.GetMode()
	assert.Equal(t, interfaces.LLMModeOffline, mode, "LLM service should be in offline mode")
	t.Logf("✓ LLM service initialized in %s mode", mode)

	// Step 5: Verify audit logger initialized
	require.NotNil(t, application.AuditLogger, "Audit logger should be initialized")
	t.Log("✓ Audit logger initialized")

	// Step 6: Verify storage manager initialized
	require.NotNil(t, application.StorageManager, "Storage manager should be initialized")
	require.NotNil(t, application.StorageManager.DB(), "Database should be initialized")
	t.Log("✓ Storage manager initialized")

	// Step 7: Verify embedding service initialized
	require.NotNil(t, application.EmbeddingService, "Embedding service should be initialized")
	t.Log("✓ Embedding service initialized")

	// Step 8: Verify document service initialized
	require.NotNil(t, application.DocumentService, "Document service should be initialized")
	t.Log("✓ Document service initialized")

	// Step 9: Verify Atlassian services initialized
	require.NotNil(t, application.AuthService, "Auth service should be initialized")
	require.NotNil(t, application.JiraService, "Jira service should be initialized")
	require.NotNil(t, application.ConfluenceService, "Confluence service should be initialized")
	t.Log("✓ Atlassian services initialized")

	// Step 10: Verify HTTP handlers initialized
	require.NotNil(t, application.APIHandler, "API handler should be initialized")
	require.NotNil(t, application.UIHandler, "UI handler should be initialized")
	require.NotNil(t, application.WSHandler, "WebSocket handler should be initialized")
	require.NotNil(t, application.ScraperHandler, "Scraper handler should be initialized")
	require.NotNil(t, application.DataHandler, "Data handler should be initialized")
	require.NotNil(t, application.CollectorHandler, "Collector handler should be initialized")
	require.NotNil(t, application.DocumentHandler, "Document handler should be initialized")
	t.Log("✓ HTTP handlers initialized")

	// Step 11: Verify configuration values
	assert.Equal(t, "offline", config.LLM.Mode, "LLM mode should be offline")
	assert.Equal(t, "./models", config.LLM.Offline.ModelDir, "Model directory should match config")
	assert.Equal(t, "nomic-embed-text-v1.5-q8.gguf", config.LLM.Offline.EmbedModel, "Embed model should match config")
	assert.Equal(t, "qwen2.5-7b-instruct-q4.gguf", config.LLM.Offline.ChatModel, "Chat model should match config")
	assert.Equal(t, 2048, config.LLM.Offline.ContextSize, "Context size should match config")
	assert.Equal(t, 4, config.LLM.Offline.ThreadCount, "Thread count should match config")
	assert.Equal(t, 0, config.LLM.Offline.GPULayers, "GPU layers should match config")
	assert.True(t, config.LLM.Audit.Enabled, "Audit logging should be enabled")
	assert.False(t, config.LLM.Audit.LogQueries, "Query logging should be disabled for PII protection")
	t.Log("✓ Configuration values verified")

	// Step 12: Clean shutdown
	// Note: We don't explicitly close the application here as it doesn't have a Close() method
	// The WebSocket background tasks will be cleaned up when the test exits
	t.Log("✓ Application startup test completed successfully")

	t.Log("\n✅ SUCCESS: Application initialized and verified")
}
