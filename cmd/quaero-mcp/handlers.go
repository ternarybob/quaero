package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/connectors/github"
	"github.com/ternarybob/quaero/internal/githublogs"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// handleSearchDocuments implements the search_documents tool
func handleSearchDocuments(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse query parameter (required)
		query, err := request.RequireString("query")
		if err != nil || query == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: query parameter is required"),
				},
			}, nil
		}

		// Parse limit (default: 10, max: 100)
		limit := request.GetInt("limit", 10)
		if limit > 100 {
			limit = 100
		}

		// Parse source_types filter
		sourceTypes := request.GetStringSlice("source_types", nil)

		// Execute search
		docs, err := searchService.Search(ctx, query, interfaces.SearchOptions{
			Limit:       limit,
			SourceTypes: sourceTypes,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Search failed")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Search error: %v", err)),
				},
			}, nil
		}

		// Format results as markdown
		markdown := formatSearchResults(query, docs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleGetDocument implements the get_document tool
func handleGetDocument(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse document_id parameter (required)
		docID, err := request.RequireString("document_id")
		if err != nil || docID == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: document_id parameter is required"),
				},
			}, nil
		}

		// Retrieve document
		doc, err := searchService.GetByID(ctx, docID)
		if err != nil {
			logger.Error().Err(err).Str("doc_id", docID).Msg("GetByID failed")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Document not found: %v", err)),
				},
			}, nil
		}

		// Format as markdown
		markdown := formatDocument(doc)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleListRecent implements the list_recent_documents tool
func handleListRecent(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse limit (default: 20)
		limit := request.GetInt("limit", 20)

		// Parse source_type filter
		var sourceTypes []string
		if sourceType := request.GetString("source_type", ""); sourceType != "" {
			sourceTypes = []string{sourceType}
		}

		// Execute search with empty query (lists recent docs)
		docs, err := searchService.Search(ctx, "", interfaces.SearchOptions{
			Limit:       limit,
			SourceTypes: sourceTypes,
		})
		if err != nil {
			logger.Error().Err(err).Msg("List recent failed")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("List error: %v", err)),
				},
			}, nil
		}

		// Format results as markdown
		markdown := formatRecentDocuments(docs, limit)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleGetRelated implements the get_related_documents tool
func handleGetRelated(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse reference parameter (required)
		reference, err := request.RequireString("reference")
		if err != nil || reference == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: reference parameter is required"),
				},
			}, nil
		}

		// Execute search by reference
		docs, err := searchService.SearchByReference(ctx, reference, interfaces.SearchOptions{
			Limit: 50, // Reasonable limit for cross-references
		})
		if err != nil {
			logger.Error().Err(err).Str("reference", reference).Msg("SearchByReference failed")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Search error: %v", err)),
				},
			}, nil
		}

		// Format results as markdown
		markdown := formatRelatedDocuments(reference, docs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleListGitHubWorkflows implements the list_github_workflows tool
func handleListGitHubWorkflows(connectorService interfaces.ConnectorService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse required parameters
		owner, err := request.RequireString("owner")
		if err != nil || owner == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: owner parameter is required"),
				},
			}, nil
		}

		repo, err := request.RequireString("repo")
		if err != nil || repo == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: repo parameter is required"),
				},
			}, nil
		}

		// Parse optional parameters
		status := request.GetString("status", "")
		limit := request.GetInt("limit", 10)
		if limit > 50 {
			limit = 50
		}

		// Get GitHub connector
		ghConnector, err := getGitHubConnector(ctx, connectorService)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error: %v", err)),
				},
			}, nil
		}

		// List workflow runs
		runs, err := ghConnector.ListWorkflowRuns(ctx, owner, repo, limit, status, "")
		if err != nil {
			logger.Error().Err(err).Str("owner", owner).Str("repo", repo).Msg("Failed to list workflow runs")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error listing workflows: %v", err)),
				},
			}, nil
		}

		// Format results
		markdown := formatWorkflowRuns(owner, repo, runs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleGetGitHubWorkflowLogs implements the get_github_workflow_logs tool
func handleGetGitHubWorkflowLogs(connectorService interfaces.ConnectorService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse required parameters
		owner, err := request.RequireString("owner")
		if err != nil || owner == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: owner parameter is required"),
				},
			}, nil
		}

		repo, err := request.RequireString("repo")
		if err != nil || repo == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: repo parameter is required"),
				},
			}, nil
		}

		runID := int64(request.GetInt("run_id", 0))
		if runID == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: run_id parameter is required"),
				},
			}, nil
		}

		// Get GitHub connector
		ghConnector, err := getGitHubConnector(ctx, connectorService)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error: %v", err)),
				},
			}, nil
		}

		// Get workflow logs
		rawLogs, err := ghConnector.GetWorkflowRunLogs(ctx, owner, repo, runID)
		if err != nil {
			logger.Error().Err(err).Str("owner", owner).Str("repo", repo).Int64("run_id", runID).Msg("Failed to get workflow logs")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error getting workflow logs: %v", err)),
				},
			}, nil
		}

		// Sanitize logs for AI consumption
		sanitizedLogs := githublogs.SanitizeLogForAI(rawLogs)

		// Format results
		markdown := formatWorkflowLogs(owner, repo, runID, sanitizedLogs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// getGitHubConnector finds and returns a GitHub connector
func getGitHubConnector(ctx context.Context, connectorService interfaces.ConnectorService) (*github.Connector, error) {
	connectors, err := connectorService.ListConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}

	for _, c := range connectors {
		if c.Type == models.ConnectorTypeGitHub {
			ghConnector, err := github.NewConnector(c)
			if err != nil {
				continue // Try next connector
			}
			return ghConnector, nil
		}
	}

	return nil, fmt.Errorf("no GitHub connector found. Configure a GitHub connector in Quaero settings")
}

// formatWorkflowRuns formats workflow runs as markdown
func formatWorkflowRuns(owner, repo string, runs []github.WorkflowRun) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# GitHub Actions: %s/%s\n\n", owner, repo))

	if len(runs) == 0 {
		sb.WriteString("No workflow runs found.\n")
		return sb.String()
	}

	sb.WriteString("| Run ID | Workflow | Status | Conclusion | Branch | Started At |\n")
	sb.WriteString("|--------|----------|--------|------------|--------|------------|\n")

	for _, run := range runs {
		conclusion := run.Conclusion
		if conclusion == "" {
			conclusion = "-"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n",
			run.ID,
			run.WorkflowName,
			run.Status,
			conclusion,
			run.Branch,
			run.RunStartedAt.Format("2006-01-02 15:04"),
		))
	}

	sb.WriteString(fmt.Sprintf("\n**Total**: %d workflow runs\n", len(runs)))
	sb.WriteString("\nUse `get_github_workflow_logs` with a run_id to fetch detailed logs.\n")

	return sb.String()
}

// formatWorkflowLogs formats workflow logs as markdown
func formatWorkflowLogs(owner, repo string, runID int64, logs string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Workflow Logs: %s/%s Run #%d\n\n", owner, repo, runID))
	sb.WriteString(fmt.Sprintf("**URL**: https://github.com/%s/%s/actions/runs/%d\n\n", owner, repo, runID))
	sb.WriteString("## Sanitized Logs (Error Context Preserved)\n\n")
	sb.WriteString("```\n")
	sb.WriteString(logs)
	sb.WriteString("\n```\n")

	return sb.String()
}

// handleSearchGitHubRepo implements the search_github_repo tool
func handleSearchGitHubRepo(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse query parameter (required)
		query, err := request.RequireString("query")
		if err != nil || query == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: query parameter is required"),
				},
			}, nil
		}

		// Parse optional filters
		repo := request.GetString("repo", "")
		owner := request.GetString("owner", "")
		fileType := request.GetString("file_type", "")
		limit := request.GetInt("limit", 10)
		if limit > 50 {
			limit = 50
		}

		// Build metadata filters
		metadataFilters := make(map[string]string)
		if repo != "" {
			metadataFilters["repo"] = repo
		}
		if owner != "" {
			metadataFilters["owner"] = owner
		}
		if fileType != "" {
			metadataFilters["file_type"] = fileType
		}

		// Search with GitHub source type filters
		docs, err := searchService.Search(ctx, query, interfaces.SearchOptions{
			Limit:           limit,
			SourceTypes:     []string{"github_git", "github_repo"},
			MetadataFilters: metadataFilters,
		})
		if err != nil {
			logger.Error().Err(err).Msg("GitHub repo search failed")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Search error: %v", err)),
				},
			}, nil
		}

		// Format results as concise markdown (file list with snippets)
		markdown := formatGitHubRepoSearchResults(query, docs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleGetGitHubRepoFile implements the get_github_repo_file tool
func handleGetGitHubRepoFile(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse required parameters
		repo, err := request.RequireString("repo")
		if err != nil || repo == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: repo parameter is required"),
				},
			}, nil
		}

		path, err := request.RequireString("path")
		if err != nil || path == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: path parameter is required"),
				},
			}, nil
		}

		// Parse optional filters
		owner := request.GetString("owner", "")
		branch := request.GetString("branch", "")

		// Build metadata filters
		metadataFilters := map[string]string{
			"repo": repo,
			"path": path,
		}
		if owner != "" {
			metadataFilters["owner"] = owner
		}
		if branch != "" {
			metadataFilters["branch"] = branch
		}

		// Search for the specific file
		docs, err := searchService.Search(ctx, "", interfaces.SearchOptions{
			Limit:           1,
			SourceTypes:     []string{"github_git", "github_repo"},
			MetadataFilters: metadataFilters,
		})
		if err != nil {
			logger.Error().Err(err).Str("repo", repo).Str("path", path).Msg("Failed to get repo file")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error getting file: %v", err)),
				},
			}, nil
		}

		if len(docs) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("File not found: %s in repo %s\n\nNote: The file may not have been collected yet. Use the GitHub collection job to import the repository.", path, repo)),
				},
			}, nil
		}

		// Format the file content
		doc := docs[0]
		markdown := formatGitHubRepoFile(doc)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// handleListGitHubRepoFiles implements the list_github_repo_files tool
func handleListGitHubRepoFiles(searchService interfaces.SearchService, logger arbor.ILogger) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse required parameters
		repo, err := request.RequireString("repo")
		if err != nil || repo == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent("Error: repo parameter is required"),
				},
			}, nil
		}

		// Parse optional filters
		owner := request.GetString("owner", "")
		folder := request.GetString("folder", "")
		fileType := request.GetString("file_type", "")
		limit := request.GetInt("limit", 50)
		if limit > 200 {
			limit = 200
		}

		// Build metadata filters
		metadataFilters := map[string]string{
			"repo": repo,
		}
		if owner != "" {
			metadataFilters["owner"] = owner
		}
		if folder != "" {
			metadataFilters["folder"] = folder
		}
		if fileType != "" {
			metadataFilters["file_type"] = fileType
		}

		// List files from the repository
		docs, err := searchService.Search(ctx, "", interfaces.SearchOptions{
			Limit:           limit,
			SourceTypes:     []string{"github_git", "github_repo"},
			MetadataFilters: metadataFilters,
			OrderBy:         "title",
			OrderDir:        "asc",
		})
		if err != nil {
			logger.Error().Err(err).Str("repo", repo).Msg("Failed to list repo files")
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.NewTextContent(fmt.Sprintf("Error listing files: %v", err)),
				},
			}, nil
		}

		// Format as file listing
		markdown := formatGitHubRepoFileList(repo, folder, docs)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(markdown),
			},
		}, nil
	}
}

// formatGitHubRepoSearchResults formats GitHub repo search results as concise markdown
func formatGitHubRepoSearchResults(query string, docs []*models.Document) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## GitHub Repo Search: \"%s\"\n\n", query))

	if len(docs) == 0 {
		sb.WriteString("No matching files found.\n\n")
		sb.WriteString("**Tip**: Make sure the repository has been collected using the GitHub collection job.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Found %d matching files:\n\n", len(docs)))

	for i, doc := range docs {
		// Extract metadata
		repo, _ := doc.Metadata["repo"].(string)
		owner, _ := doc.Metadata["owner"].(string)
		path, _ := doc.Metadata["path"].(string)
		branch, _ := doc.Metadata["branch"].(string)

		if path == "" {
			path = doc.SourceID
		}

		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, doc.Title))
		if owner != "" && repo != "" {
			sb.WriteString(fmt.Sprintf("**Repo**: %s/%s", owner, repo))
			if branch != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", branch))
			}
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("**Path**: `%s`\n", path))
		if doc.URL != "" {
			sb.WriteString(fmt.Sprintf("**URL**: %s\n", doc.URL))
		}

		// Show content preview (first 300 chars)
		content := doc.ContentMarkdown
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		sb.WriteString("\n```\n")
		sb.WriteString(content)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Use `get_github_repo_file` with the path to get the full file content.\n")

	return sb.String()
}

// formatGitHubRepoFile formats a single GitHub repo file
func formatGitHubRepoFile(doc *models.Document) string {
	var sb strings.Builder

	// Extract metadata
	repo, _ := doc.Metadata["repo"].(string)
	owner, _ := doc.Metadata["owner"].(string)
	path, _ := doc.Metadata["path"].(string)
	branch, _ := doc.Metadata["branch"].(string)

	if path == "" {
		path = doc.SourceID
	}

	sb.WriteString(fmt.Sprintf("# %s\n\n", doc.Title))

	if owner != "" && repo != "" {
		sb.WriteString(fmt.Sprintf("**Repository**: %s/%s\n", owner, repo))
	}
	if branch != "" {
		sb.WriteString(fmt.Sprintf("**Branch**: %s\n", branch))
	}
	sb.WriteString(fmt.Sprintf("**Path**: `%s`\n", path))
	if doc.URL != "" {
		sb.WriteString(fmt.Sprintf("**URL**: %s\n", doc.URL))
	}
	sb.WriteString("\n---\n\n")

	// Determine file type for syntax highlighting
	ext := ""
	if path != "" {
		parts := strings.Split(path, ".")
		if len(parts) > 1 {
			ext = parts[len(parts)-1]
		}
	}

	sb.WriteString(fmt.Sprintf("```%s\n", ext))
	sb.WriteString(doc.ContentMarkdown)
	sb.WriteString("\n```\n")

	return sb.String()
}

// formatGitHubRepoFileList formats a list of GitHub repo files
func formatGitHubRepoFileList(repo, folder string, docs []*models.Document) string {
	var sb strings.Builder

	if folder != "" {
		sb.WriteString(fmt.Sprintf("## Files in %s/%s\n\n", repo, folder))
	} else {
		sb.WriteString(fmt.Sprintf("## Files in %s\n\n", repo))
	}

	if len(docs) == 0 {
		sb.WriteString("No files found.\n\n")
		sb.WriteString("**Tip**: Make sure the repository has been collected using the GitHub collection job.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Found %d files:\n\n", len(docs)))

	// Group by folder for better organization
	folders := make(map[string][]string)
	for _, doc := range docs {
		docFolder, _ := doc.Metadata["folder"].(string)
		path, _ := doc.Metadata["path"].(string)
		if path == "" {
			path = doc.Title
		}
		folders[docFolder] = append(folders[docFolder], path)
	}

	// Output as tree-like structure
	for folderName, files := range folders {
		if folderName == "" {
			folderName = "/"
		}
		sb.WriteString(fmt.Sprintf("**%s**\n", folderName))
		for _, file := range files {
			sb.WriteString(fmt.Sprintf("  - `%s`\n", file))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nUse `get_github_repo_file` with repo and path to get file content.\n")
	sb.WriteString("Use `search_github_repo` to search within files.\n")

	return sb.String()
}
