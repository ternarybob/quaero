package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
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
