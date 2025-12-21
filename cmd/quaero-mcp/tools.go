package main

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// createSearchDocumentsTool returns the search_documents tool definition
func createSearchDocumentsTool() mcp.Tool {
	return mcp.NewTool("search_documents",
		mcp.WithDescription("Search Quaero knowledge base using full-text search"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query (FTS5 syntax: quoted phrases, +required, OR, AND)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default: 10, max: 100)"),
		),
		mcp.WithArray("source_types",
			mcp.WithStringItems(),
			mcp.Description("Filter by source types: jira, confluence, github"),
		),
	)
}

// createGetDocumentTool returns the get_document tool definition
func createGetDocumentTool() mcp.Tool {
	return mcp.NewTool("get_document",
		mcp.WithDescription("Retrieve a single document by its unique ID"),
		mcp.WithString("document_id",
			mcp.Required(),
			mcp.Description("Document ID (format: doc_{uuid})"),
		),
	)
}

// createListRecentDocumentsTool returns the list_recent_documents tool definition
func createListRecentDocumentsTool() mcp.Tool {
	return mcp.NewTool("list_recent_documents",
		mcp.WithDescription("List recently updated documents, optionally filtered by source type"),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 20)"),
		),
		mcp.WithString("source_type",
			mcp.Description("Filter: jira, confluence, github"),
		),
	)
}

// createGetRelatedDocumentsTool returns the get_related_documents tool definition
func createGetRelatedDocumentsTool() mcp.Tool {
	return mcp.NewTool("get_related_documents",
		mcp.WithDescription("Find documents referencing a specific issue key or identifier"),
		mcp.WithString("reference",
			mcp.Required(),
			mcp.Description("Issue key (BUG-123) or identifier"),
		),
	)
}

// createListGitHubWorkflowsTool returns the list_github_workflows tool definition
func createListGitHubWorkflowsTool() mcp.Tool {
	return mcp.NewTool("list_github_workflows",
		mcp.WithDescription("List recent GitHub Actions workflow runs for a repository"),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner (user or org)"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		mcp.WithString("status",
			mcp.Description("Filter by status: completed, in_progress, queued, failure, success (default: all)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default: 10, max: 50)"),
		),
	)
}

// createGetGitHubWorkflowLogsTool returns the get_github_workflow_logs tool definition
func createGetGitHubWorkflowLogsTool() mcp.Tool {
	return mcp.NewTool("get_github_workflow_logs",
		mcp.WithDescription("Get logs from a specific GitHub Actions workflow run. Returns sanitized logs optimized for AI analysis with error context preserved."),
		mcp.WithString("owner",
			mcp.Required(),
			mcp.Description("Repository owner (user or org)"),
		),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name"),
		),
		mcp.WithNumber("run_id",
			mcp.Required(),
			mcp.Description("Workflow run ID (from list_github_workflows)"),
		),
	)
}

// createSearchGitHubRepoTool returns the search_github_repo tool definition
func createSearchGitHubRepoTool() mcp.Tool {
	return mcp.NewTool("search_github_repo",
		mcp.WithDescription("Search within collected GitHub repository files. Use this to find code, documentation, and configuration files that have been indexed from GitHub repositories."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query (supports FTS5 syntax: quoted phrases, +required, OR, AND)"),
		),
		mcp.WithString("repo",
			mcp.Description("Filter by repository name (e.g., 'quaero')"),
		),
		mcp.WithString("owner",
			mcp.Description("Filter by repository owner (e.g., 'ternarybob')"),
		),
		mcp.WithString("file_type",
			mcp.Description("Filter by file extension (e.g., '.go', '.ts', '.md')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default: 10, max: 50)"),
		),
	)
}

// createGetGitHubRepoFileTool returns the get_github_repo_file tool definition
func createGetGitHubRepoFileTool() mcp.Tool {
	return mcp.NewTool("get_github_repo_file",
		mcp.WithDescription("Get the full content of a specific file from a collected GitHub repository by its path."),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (e.g., 'quaero')"),
		),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("File path within the repository (e.g., 'internal/models/job.go')"),
		),
		mcp.WithString("owner",
			mcp.Description("Repository owner (optional, helps disambiguate if multiple repos have same name)"),
		),
		mcp.WithString("branch",
			mcp.Description("Branch name (optional, defaults to searching all collected branches)"),
		),
	)
}

// createListGitHubRepoFilesTool returns the list_github_repo_files tool definition
func createListGitHubRepoFilesTool() mcp.Tool {
	return mcp.NewTool("list_github_repo_files",
		mcp.WithDescription("List all collected files from a GitHub repository. Use this to browse repository structure and find files to examine."),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name (e.g., 'quaero')"),
		),
		mcp.WithString("owner",
			mcp.Description("Repository owner (optional)"),
		),
		mcp.WithString("folder",
			mcp.Description("Filter by folder path (e.g., 'internal/models')"),
		),
		mcp.WithString("file_type",
			mcp.Description("Filter by file extension (e.g., '.go')"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default: 50, max: 200)"),
		),
	)
}
