package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/ternarybob/arbor"
	arbor_models "github.com/ternarybob/arbor/models"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/services/connectors"
	"github.com/ternarybob/quaero/internal/services/search"
	"github.com/ternarybob/quaero/internal/storage"
)

func main() {
	// Load configuration
	configPath := os.Getenv("QUAERO_CONFIG")
	if configPath == "" {
		configPath = "quaero.toml"
	}

	// Phase 1: Load config without KV replacement (storage not initialized yet)
	// Note: MCP server doesn't use KV storage, so nil is appropriate here
	config, err := common.LoadFromFile(nil, configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize minimal logger for MCP server (console only, no file output)
	logger := arbor.NewLogger().WithConsoleWriter(arbor_models.WriterConfiguration{
		Type:             arbor_models.LogWriterTypeConsole,
		TimeFormat:       "15:04:05",
		DisableTimestamp: false,
	}).WithLevelFromString("warn") // Minimal logging to avoid cluttering MCP stdio

	// Initialize storage
	storageManager, err := storage.NewStorageManager(logger, config)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer storageManager.Close()

	// Initialize search service
	searchService, err := search.NewSearchService(
		storageManager.DocumentStorage(),
		logger,
		config,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize search service")
	}

	// Initialize connector service for GitHub tools
	connectorService := connectors.NewService(storageManager.ConnectorStorage(), logger)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"quaero",
		common.GetVersion(),
		server.WithToolCapabilities(true),
	)

	// Register search tools
	mcpServer.AddTool(createSearchDocumentsTool(), handleSearchDocuments(searchService, logger))
	mcpServer.AddTool(createGetDocumentTool(), handleGetDocument(searchService, logger))
	mcpServer.AddTool(createListRecentDocumentsTool(), handleListRecent(searchService, logger))
	mcpServer.AddTool(createGetRelatedDocumentsTool(), handleGetRelated(searchService, logger))

	// Register GitHub workflow tools
	mcpServer.AddTool(createListGitHubWorkflowsTool(), handleListGitHubWorkflows(connectorService, logger))
	mcpServer.AddTool(createGetGitHubWorkflowLogsTool(), handleGetGitHubWorkflowLogs(connectorService, logger))

	// Register GitHub repository tools
	mcpServer.AddTool(createSearchGitHubRepoTool(), handleSearchGitHubRepo(searchService, logger))
	mcpServer.AddTool(createGetGitHubRepoFileTool(), handleGetGitHubRepoFile(searchService, logger))
	mcpServer.AddTool(createListGitHubRepoFilesTool(), handleListGitHubRepoFiles(searchService, logger))

	// Start server (blocks on stdio)
	if err := server.ServeStdio(mcpServer); err != nil {
		logger.Fatal().Err(err).Msg("MCP server failed")
	}
}
