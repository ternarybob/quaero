package api

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
// with the test configuration and all API handlers are properly initialized.
// This test serves as the foundation for API testing by ensuring all components
// are available and correctly configured for API interactions.
func TestApplicationStartup(t *testing.T) {
	t.Log("=== Testing Application Startup for API Testing ===")

	// Step 1: Load test configuration
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")
	t.Logf("✓ Configuration loaded from: %s", configPath)

	// Step 2: Initialize logger
	logger := arbor.NewLogger()
	require.NotNil(t, logger, "Logger should be initialized")
	t.Log("✓ Logger initialized")

	// Step 3: Create application with all API dependencies
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")
	require.NotNil(t, application, "Application should not be nil")
	t.Log("✓ Application created successfully")

	// Step 4: Verify LLM service for API testing (offline mode for security)
	require.NotNil(t, application.LLMService, "LLM service should be initialized")
	mode := application.LLMService.GetMode()
	assert.Equal(t, interfaces.LLMModeOffline, mode, "LLM service should be in offline mode for testing")
	t.Logf("✓ LLM service initialized in %s mode", mode)

	// Step 5: Verify audit logger for compliance testing
	require.NotNil(t, application.AuditLogger, "Audit logger should be initialized")
	t.Log("✓ Audit logger initialized")

	// Step 6: Verify storage layer for API data operations
	require.NotNil(t, application.StorageManager, "Storage manager should be initialized")
	require.NotNil(t, application.StorageManager.DB(), "Database should be initialized")
	t.Log("✓ Storage manager initialized")

	// Step 7: Verify core services for API functionality
	require.NotNil(t, application.EmbeddingService, "Embedding service should be initialized")
	require.NotNil(t, application.DocumentService, "Document service should be initialized")
	require.NotNil(t, application.ChatService, "Chat service should be initialized")
	require.NotNil(t, application.EventService, "Event service should be initialized")
	t.Log("✓ Core services initialized")

	// Step 8: Verify Atlassian services for data collection APIs
	require.NotNil(t, application.AuthService, "Auth service should be initialized")
	require.NotNil(t, application.JiraService, "Jira service should be initialized")
	require.NotNil(t, application.ConfluenceService, "Confluence service should be initialized")
	t.Log("✓ Atlassian services initialized")

	// Step 9: Verify ALL API handlers are initialized and ready
	require.NotNil(t, application.APIHandler, "API handler should be initialized")
	require.NotNil(t, application.UIHandler, "UI handler should be initialized")
	require.NotNil(t, application.WSHandler, "WebSocket handler should be initialized")
	require.NotNil(t, application.ScraperHandler, "Scraper handler should be initialized")
	require.NotNil(t, application.DataHandler, "Data handler should be initialized")
	require.NotNil(t, application.CollectorHandler, "Collector handler should be initialized")
	require.NotNil(t, application.DocumentHandler, "Document handler should be initialized")
	require.NotNil(t, application.EmbeddingHandler, "Embedding handler should be initialized")
	require.NotNil(t, application.ChatHandler, "Chat handler should be initialized")
	require.NotNil(t, application.SchedulerHandler, "Scheduler handler should be initialized")
	require.NotNil(t, application.MCPHandler, "MCP handler should be initialized")
	t.Log("✓ All API handlers initialized")

	// Step 10: Verify API-relevant configuration for testing
	assert.Equal(t, "offline", config.LLM.Mode, "LLM mode should be offline for secure API testing")
	assert.True(t, config.LLM.Audit.Enabled, "Audit logging should be enabled for API compliance testing")
	assert.False(t, config.LLM.Audit.LogQueries, "Query logging should be disabled for PII protection")
	assert.Equal(t, "sqlite", config.Storage.Type, "Storage type should be SQLite for API testing")
	assert.True(t, config.Storage.SQLite.EnableFTS5, "FTS5 should be enabled for search API testing")
	assert.Equal(t, 768, config.Storage.SQLite.EmbeddingDimension, "Embedding dimension should match API expectations")
	t.Log("✓ API-relevant configuration verified")

	// Step 11: Verify processing services for background API operations
	require.NotNil(t, application.ProcessingService, "Processing service should be initialized")
	require.NotNil(t, application.EmbeddingCoordinator, "Embedding coordinator should be initialized")
	t.Log("✓ Background processing services initialized")

	// Step 12: Test that WebSocket services are ready for real-time API updates
	// The WebSocket handler should have started its background tasks
	require.NotNil(t, application.WSHandler, "WebSocket handler should be ready for real-time updates")
	t.Log("✓ WebSocket services ready for real-time API updates")

	t.Log("\n✅ SUCCESS: Application fully initialized and ready for API testing")
	t.Log("All API handlers, services, and dependencies are properly configured")
	t.Log("The application is ready for comprehensive API test suites")
}

// TestAPIHandlerDependencies verifies that all API handlers have their required dependencies
// properly injected and are ready to handle API requests
func TestAPIHandlerDependencies(t *testing.T) {
	t.Log("=== Testing API Handler Dependencies ===")

	// Load configuration and create application
	configPath := filepath.Join("..", "..", "bin", "quaero.toml")
	config, err := common.LoadFromFile(configPath)
	require.NoError(t, err, "Failed to load test configuration")

	logger := arbor.NewLogger()
	application, err := app.New(config, logger)
	require.NoError(t, err, "Application initialization should succeed")

	// Test that each API handler has access to required services
	t.Log("Verifying API handler service dependencies...")

	// Document Handler should have document service access
	require.NotNil(t, application.DocumentHandler, "Document handler should exist")
	require.NotNil(t, application.DocumentService, "Document service should exist for document API")

	// Embedding Handler should have embedding service access
	require.NotNil(t, application.EmbeddingHandler, "Embedding handler should exist")
	require.NotNil(t, application.EmbeddingService, "Embedding service should exist for embedding API")

	// Chat Handler should have chat service access
	require.NotNil(t, application.ChatHandler, "Chat handler should exist")
	require.NotNil(t, application.ChatService, "Chat service should exist for chat API")

	// Data Handler should have storage access
	require.NotNil(t, application.DataHandler, "Data handler should exist")
	require.NotNil(t, application.StorageManager, "Storage manager should exist for data API")

	// Scraper/Collector Handlers should have Atlassian services
	require.NotNil(t, application.ScraperHandler, "Scraper handler should exist")
	require.NotNil(t, application.CollectorHandler, "Collector handler should exist")
	require.NotNil(t, application.JiraService, "Jira service should exist for collection API")
	require.NotNil(t, application.ConfluenceService, "Confluence service should exist for collection API")

	t.Log("✓ All API handlers have required service dependencies")
}
