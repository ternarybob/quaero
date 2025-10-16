package mcp

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// DocumentService implements MCP protocol for document operations
type DocumentService struct {
	storage       interfaces.DocumentStorage
	searchService interfaces.SearchService
	logger        arbor.ILogger
}

// NewDocumentService creates a new MCP document service
func NewDocumentService(
	storage interfaces.DocumentStorage,
	searchService interfaces.SearchService,
	logger arbor.ILogger,
) *DocumentService {
	return &DocumentService{
		storage:       storage,
		searchService: searchService,
		logger:        logger,
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
				Description: "Search documents using full-text search. Supports keyword queries and filters.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query (can be empty for filter-only searches)",
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of results (default: 10)",
						},
						"source_types": map[string]interface{}{
							"type":        "array",
							"description": "Filter by source types (e.g., ['jira', 'confluence'])",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			{
				Name:        "search_by_reference",
				Description: "Search documents containing a specific reference (e.g., Jira issue key 'PROJ-123', user mention '@alice', PR reference 'PR #456')",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"reference": map[string]interface{}{
							"type":        "string",
							"description": "Reference to search for (e.g., 'PROJ-123', '@alice', 'PR #456')",
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of results (default: 10)",
						},
						"source_types": map[string]interface{}{
							"type":        "array",
							"description": "Filter by source types (e.g., ['jira', 'confluence'])",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"required": []string{"reference"},
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
			{
				Name:        "search_by_keywords",
				Description: "Search documents by keywords in metadata. Returns documents that have matching keywords in their metadata.keywords field.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"keywords": map[string]interface{}{
							"type":        "array",
							"description": "Array of keywords to search for",
							"items": map[string]interface{}{
								"type": "string",
							},
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of results (default: 10)",
						},
					},
					"required": []string{"keywords"},
				},
			},
			{
				Name:        "get_document_summary",
				Description: "Get summary and metadata for a specific document. Returns summary, word count, keywords, and other metadata extracted from the document.",
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
				Name:        "find_similar_sources",
				Description: "Find documents from similar sources grouped by a metadata key. Useful for finding related documents based on project, space, or other grouping criteria.",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"source_type": map[string]interface{}{
							"type":        "string",
							"description": "Source type to search within (jira, confluence, github)",
						},
						"metadata_key": map[string]interface{}{
							"type":        "string",
							"description": "Metadata key to group by (e.g., 'project_key', 'space_key')",
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of groups to return (default: 10)",
						},
					},
					"required": []string{"source_type", "metadata_key"},
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
		return s.searchDocuments(ctx, args)
	case "search_by_reference":
		return s.searchByReference(ctx, args)
	case "get_document":
		return s.getDocumentTool(ctx, args)
	case "list_documents":
		return s.listDocuments(args)
	case "search_by_keywords":
		return s.searchByKeywords(ctx, args)
	case "get_document_summary":
		return s.getDocumentSummary(ctx, args)
	case "find_similar_sources":
		return s.findSimilarSources(ctx, args)
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

func (s *DocumentService) searchDocuments(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query := ""
	if q, ok := args["query"].(string); ok {
		query = q
	}

	// Build search options
	opts := interfaces.SearchOptions{
		Limit: 10,
	}

	if l, ok := args["limit"].(float64); ok {
		opts.Limit = int(l)
	}

	// Parse source_types array if provided
	if sourceTypesRaw, ok := args["source_types"]; ok {
		if sourceTypesArray, ok := sourceTypesRaw.([]interface{}); ok {
			sourceTypes := make([]string, 0, len(sourceTypesArray))
			for _, st := range sourceTypesArray {
				if strType, ok := st.(string); ok {
					sourceTypes = append(sourceTypes, strType)
				}
			}
			opts.SourceTypes = sourceTypes
		}
	}

	// Use SearchService instead of direct storage call
	docs, err := s.searchService.Search(ctx, query, opts)
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

func (s *DocumentService) searchByReference(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	reference, ok := args["reference"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'reference' parameter")
	}

	// Build search options
	opts := interfaces.SearchOptions{
		Limit: 10,
	}

	if l, ok := args["limit"].(float64); ok {
		opts.Limit = int(l)
	}

	// Parse source_types array if provided
	if sourceTypesRaw, ok := args["source_types"]; ok {
		if sourceTypesArray, ok := sourceTypesRaw.([]interface{}); ok {
			sourceTypes := make([]string, 0, len(sourceTypesArray))
			for _, st := range sourceTypesArray {
				if strType, ok := st.(string); ok {
					sourceTypes = append(sourceTypes, strType)
				}
			}
			opts.SourceTypes = sourceTypes
		}
	}

	// Use SearchService.SearchByReference
	docs, err := s.searchService.SearchByReference(ctx, reference, opts)
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

func (s *DocumentService) getDocumentTool(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' parameter")
	}

	// Use SearchService.GetByID instead of direct storage call
	doc, err := s.searchService.GetByID(ctx, id)
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

func (s *DocumentService) searchByKeywords(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Parse keywords array
	keywordsRaw, ok := args["keywords"]
	if !ok {
		return nil, fmt.Errorf("missing 'keywords' parameter")
	}

	keywordsArray, ok := keywordsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("'keywords' must be an array")
	}

	searchKeywords := make([]string, 0, len(keywordsArray))
	for _, kw := range keywordsArray {
		if kwStr, ok := kw.(string); ok {
			searchKeywords = append(searchKeywords, kwStr)
		}
	}

	if len(searchKeywords) == 0 {
		return nil, fmt.Errorf("keywords array cannot be empty")
	}

	// Parse limit
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Get all documents and filter by keywords in metadata
	opts := &interfaces.ListOptions{
		Limit:    1000, // Get more documents to filter
		Offset:   0,
		OrderBy:  "updated_at",
		OrderDir: "desc",
	}

	allDocs, err := s.storage.ListDocuments(opts)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Filter documents by keywords in metadata
	var matchedDocs []*models.Document
	for _, doc := range allDocs {
		if doc.Metadata == nil {
			continue
		}

		// Check if document has keywords metadata field
		if keywordsRaw, ok := doc.Metadata["keywords"]; ok {
			var docKeywords []string

			// Handle different types for keywords field
			switch kw := keywordsRaw.(type) {
			case []string:
				docKeywords = kw
			case []interface{}:
				for _, k := range kw {
					if kwStr, ok := k.(string); ok {
						docKeywords = append(docKeywords, kwStr)
					}
				}
			}

			// Check if any search keyword matches document keywords
			for _, searchKw := range searchKeywords {
				for _, docKw := range docKeywords {
					if searchKw == docKw {
						matchedDocs = append(matchedDocs, doc)
						goto nextDoc // Move to next document after first match
					}
				}
			}
		}
	nextDoc:
	}

	// Apply limit
	if len(matchedDocs) > limit {
		matchedDocs = matchedDocs[:limit]
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: formatDocumentList(matchedDocs)},
		},
	}, nil
}

func (s *DocumentService) getDocumentSummary(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'id' parameter")
	}

	// Get document
	doc, err := s.searchService.GetByID(ctx, id)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Extract metadata fields
	var summary, lastSummarized string
	var wordCount int
	var keywords []string

	if doc.Metadata != nil {
		if s, ok := doc.Metadata["summary"].(string); ok {
			summary = s
		}
		if wc, ok := doc.Metadata["word_count"].(float64); ok {
			wordCount = int(wc)
		} else if wc, ok := doc.Metadata["word_count"].(int); ok {
			wordCount = wc
		}
		if ls, ok := doc.Metadata["last_summarized"].(string); ok {
			lastSummarized = ls
		}

		// Extract keywords
		if kwRaw, ok := doc.Metadata["keywords"]; ok {
			switch kw := kwRaw.(type) {
			case []string:
				keywords = kw
			case []interface{}:
				for _, k := range kw {
					if kwStr, ok := k.(string); ok {
						keywords = append(keywords, kwStr)
					}
				}
			}
		}
	}

	// Format summary output
	output := fmt.Sprintf("# Document Summary: %s\n\n", doc.Title)
	output += fmt.Sprintf("**ID:** %s\n", doc.ID)
	output += fmt.Sprintf("**Source:** %s (%s)\n", doc.SourceType, doc.SourceID)
	if doc.URL != "" {
		output += fmt.Sprintf("**URL:** %s\n", doc.URL)
	}
	output += "\n## Metadata\n\n"

	if summary != "" {
		output += fmt.Sprintf("**Summary:** %s\n\n", summary)
	} else {
		output += "**Summary:** Not available\n\n"
	}

	if wordCount > 0 {
		output += fmt.Sprintf("**Word Count:** %d\n\n", wordCount)
	}

	if len(keywords) > 0 {
		output += "**Keywords:** "
		for i, kw := range keywords {
			if i > 0 {
				output += ", "
			}
			output += kw
		}
		output += "\n\n"
	}

	if lastSummarized != "" {
		output += fmt.Sprintf("**Last Summarized:** %s\n\n", lastSummarized)
	}

	// Add other metadata fields
	if doc.Metadata != nil && len(doc.Metadata) > 0 {
		output += "## Additional Metadata\n\n"
		for k, v := range doc.Metadata {
			// Skip already displayed fields
			if k == "summary" || k == "word_count" || k == "keywords" || k == "last_summarized" {
				continue
			}
			output += fmt.Sprintf("- **%s:** %v\n", k, v)
		}
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: output},
		},
	}, nil
}

func (s *DocumentService) findSimilarSources(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	sourceType, ok := args["source_type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'source_type' parameter")
	}

	metadataKey, ok := args["metadata_key"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'metadata_key' parameter")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Get all documents for the source type
	docs, err := s.storage.GetDocumentsBySource(sourceType)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, nil
	}

	// Group documents by metadata key
	groups := make(map[string][]*models.Document)
	for _, doc := range docs {
		if doc.Metadata == nil {
			continue
		}

		// Get the metadata value for the grouping key
		if value, ok := doc.Metadata[metadataKey]; ok {
			valueStr := fmt.Sprintf("%v", value)
			groups[valueStr] = append(groups[valueStr], doc)
		}
	}

	// Format output
	output := fmt.Sprintf("# Similar Sources Grouped by '%s'\n\n", metadataKey)
	output += fmt.Sprintf("**Source Type:** %s\n", sourceType)
	output += fmt.Sprintf("**Total Groups:** %d\n", len(groups))
	output += fmt.Sprintf("**Total Documents:** %d\n\n", len(docs))

	// Sort groups by size and output
	groupCount := 0
	for groupKey, groupDocs := range groups {
		if groupCount >= limit {
			break
		}
		groupCount++

		output += fmt.Sprintf("## Group: %s (%d documents)\n\n", groupKey, len(groupDocs))

		// Show up to 5 documents per group
		displayCount := len(groupDocs)
		if displayCount > 5 {
			displayCount = 5
		}

		for i := 0; i < displayCount; i++ {
			doc := groupDocs[i]
			output += fmt.Sprintf("- **%s** (ID: %s)\n", doc.Title, doc.ID)
			if doc.URL != "" {
				output += fmt.Sprintf("  URL: %s\n", doc.URL)
			}
		}

		if len(groupDocs) > 5 {
			output += fmt.Sprintf("  ... and %d more documents\n", len(groupDocs)-5)
		}
		output += "\n"
	}

	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: output},
		},
	}, nil
}
