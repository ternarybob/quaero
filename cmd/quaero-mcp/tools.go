package main

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// createSearchDocumentsTool returns the search_documents tool definition
func createSearchDocumentsTool() mcp.Tool {
	return mcp.NewTool("search_documents",
		mcp.WithDescription("Search Quaero knowledge base using full-text search (SQLite FTS5)"),
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
