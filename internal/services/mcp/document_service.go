package mcp

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// DocumentService implements MCP protocol for document operations
type DocumentService struct {
	storage interfaces.DocumentStorage
	logger  arbor.ILogger
}

// NewDocumentService creates a new MCP document service
func NewDocumentService(storage interfaces.DocumentStorage, logger arbor.ILogger) *DocumentService {
	return &DocumentService{
		storage: storage,
		logger:  logger,
	}
}

// ListResources returns available MCP resources
func (s *DocumentService) ListResources(ctx context.Context) (*ResourceList, error) {
	s.logger.Debug().Msg("Listing MCP resources")

	return &ResourceList{
		Resources: []Resource{
			{
				URI:         "quaero://documents/all",
				Name:        "All Documents",
				Description: "List all documents in the knowledge base",
				MimeType:    "application/json",
			},
			{
				URI:         "quaero://documents/jira",
				Name:        "Jira Documents",
				Description: "List all Jira issues",
				MimeType:    "application/json",
			},
			{
				URI:         "quaero://documents/confluence",
				Name:        "Confluence Documents",
				Description: "List all Confluence pages",
				MimeType:    "application/json",
			},
			{
				URI:         "quaero://documents/github",
				Name:        "GitHub Documents",
				Description: "List all GitHub documents",
				MimeType:    "application/json",
			},
			{
				URI:         "quaero://documents/stats",
				Name:        "Document Statistics",
				Description: "Get statistics about documents in the knowledge base",
				MimeType:    "application/json",
			},
		},
	}, nil
}

// ReadResource reads a specific MCP resource
func (s *DocumentService) ReadResource(ctx context.Context, uri string) (*ResourceContent, error) {
	s.logger.Debug().Str("uri", uri).Msg("Reading MCP resource")

	switch uri {
	case "quaero://documents/all":
		return s.getAllDocuments()
	case "quaero://documents/jira":
		return s.getDocumentsBySource("jira")
	case "quaero://documents/confluence":
		return s.getDocumentsBySource("confluence")
	case "quaero://documents/github":
		return s.getDocumentsBySource("github")
	case "quaero://documents/stats":
		return s.getStats()
	default:
		// Try to parse as document ID: quaero://documents/{id}
		if len(uri) > 21 && uri[:21] == "quaero://documents/" {
			docID := uri[21:]
			return s.getDocument(docID)
		}
		return nil, fmt.Errorf("unknown resource URI: %s", uri)
	}
}

// ListTools returns available MCP tools
func (s *DocumentService) ListTools(ctx context.Context) (*ToolList, error) {
	s.logger.Debug().Msg("Listing MCP tools")

	return &ToolList{
		Tools: []Tool{
			{
				Name:        "search_documents",
				Description: "Search documents using full-text search",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of results (default: 10)",
						},
					},
					"required": []string{"query"},
				},
			},
			{
				Name:        "get_document",
				Description: "Get a specific document by ID",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Document ID",
						},
					},
					"required": []string{"id"},
				},
			},
			{
				Name:        "list_documents",
				Description: "List documents with pagination and filtering",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source": map[string]interface{}{
							"type":        "string",
							"description": "Source type (jira, confluence, github)",
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Number of documents to return (default: 50)",
						},
						"offset": map[string]interface{}{
							"type":        "number",
							"description": "Offset for pagination (default: 0)",
						},
					},
				},
			},
		},
	}, nil
}

// CallTool executes an MCP tool
func (s *DocumentService) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	s.logger.Debug().Str("tool", name).Msg("Calling MCP tool")

	switch name {
	case "search_documents":
		return s.searchDocuments(args)
	case "get_document":
		return s.getDocumentTool(args)
	case "list_documents":
		return s.listDocuments(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// Helper methods

func (s *DocumentService) getAllDocuments() (*ResourceContent, error) {
	opts := &interfaces.ListOptions{
		Limit:    100,
		Offset:   0,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}

	docs, err := s.storage.ListDocuments(opts)
	if err != nil {
		return nil, err
	}

	return &ResourceContent{
		URI:      "quaero://documents/all",
		MimeType: "application/json",
		Text:     formatDocumentList(docs),
	}, nil
}

func (s *DocumentService) getDocumentsBySource(source string) (*ResourceContent, error) {
	docs, err := s.storage.GetDocumentsBySource(source)
	if err != nil {
		return nil, err
	}

	return &ResourceContent{
		URI:      fmt.Sprintf("quaero://documents/%s", source),
		MimeType: "application/json",
		Text:     formatDocumentList(docs),
	}, nil
}

func (s *DocumentService) getStats() (*ResourceContent, error) {
	stats, err := s.storage.GetStats()
	if err != nil {
		return nil, err
	}

	return &ResourceContent{
		URI:      "quaero://documents/stats",
		MimeType: "application/json",
		Text:     formatStats(stats),
	}, nil
}

func (s *DocumentService) getDocument(id string) (*ResourceContent, error) {
	doc, err := s.storage.GetDocument(id)
	if err != nil {
		return nil, err
	}

	return &ResourceContent{
		URI:      fmt.Sprintf("quaero://documents/%s", id),
		MimeType: "application/json",
		Text:     formatDocument(doc),
	}, nil
}

func (s *DocumentService) searchDocuments(args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'query' parameter")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	docs, err := s.storage.FullTextSearch(query, limit)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: formatDocumentList(docs)},
		},
	}, nil
}

func (s *DocumentService) getDocumentTool(args map[string]interface{}) (*ToolResult, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' parameter")
	}

	doc, err := s.storage.GetDocument(id)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: formatDocument(doc)},
		},
	}, nil
}

func (s *DocumentService) listDocuments(args map[string]interface{}) (*ToolResult, error) {
	opts := &interfaces.ListOptions{
		Limit:    50,
		Offset:   0,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}

	if source, ok := args["source"].(string); ok {
		opts.SourceType = source
	}
	if limit, ok := args["limit"].(float64); ok {
		opts.Limit = int(limit)
	}
	if offset, ok := args["offset"].(float64); ok {
		opts.Offset = int(offset)
	}

	docs, err := s.storage.ListDocuments(opts)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: formatDocumentList(docs)},
		},
	}, nil
}
