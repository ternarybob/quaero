package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ternarybob/quaero/internal/models"
)

// formatDocument formats a single document for MCP
func formatDocument(doc *models.Document) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", doc.Title))
	b.WriteString(fmt.Sprintf("**ID:** %s\n", doc.ID))
	b.WriteString(fmt.Sprintf("**Source:** %s (%s)\n", doc.SourceType, doc.SourceID))
	if doc.URL != "" {
		b.WriteString(fmt.Sprintf("**URL:** %s\n", doc.URL))
	}
	b.WriteString(fmt.Sprintf("**Created:** %s\n", doc.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("**Updated:** %s\n", doc.UpdatedAt.Format("2006-01-02 15:04:05")))

	if len(doc.Metadata) > 0 {
		b.WriteString("\n## Metadata\n\n")
		for k, v := range doc.Metadata {
			b.WriteString(fmt.Sprintf("- **%s:** %v\n", k, v))
		}
	}

	b.WriteString("\n## Content\n\n")
	if doc.ContentMarkdown != "" {
		b.WriteString(doc.ContentMarkdown)
	} else {
		b.WriteString(doc.Content)
	}

	return b.String()
}

// formatDocumentList formats a list of documents for MCP
func formatDocumentList(docs []*models.Document) string {
	if len(docs) == 0 {
		return "No documents found."
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Documents (%d)\n\n", len(docs)))

	for i, doc := range docs {
		b.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, doc.Title))
		b.WriteString(fmt.Sprintf("- **ID:** %s\n", doc.ID))
		b.WriteString(fmt.Sprintf("- **Source:** %s\n", doc.SourceType))
		if doc.URL != "" {
			b.WriteString(fmt.Sprintf("- **URL:** %s\n", doc.URL))
		}
		b.WriteString(fmt.Sprintf("- **Updated:** %s\n", doc.UpdatedAt.Format("2006-01-02")))

		// Preview of content (first 200 chars)
		content := doc.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		b.WriteString(fmt.Sprintf("- **Preview:** %s\n", strings.ReplaceAll(content, "\n", " ")))
		b.WriteString("\n")
	}

	return b.String()
}

// formatStats formats document statistics for MCP
func formatStats(stats *models.DocumentStats) string {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting stats: %v", err)
	}
	return string(data)
}

// formatDocumentJSON formats a document as JSON
func formatDocumentJSON(doc *models.Document) string {
	// Create a simplified version without embedding data
	simplified := map[string]interface{}{
		"id":          doc.ID,
		"source_type": doc.SourceType,
		"source_id":   doc.SourceID,
		"title":       doc.Title,
		"content":     doc.Content,
		"url":         doc.URL,
		"created_at":  doc.CreatedAt,
		"updated_at":  doc.UpdatedAt,
		"metadata":    doc.Metadata,
	}

	data, err := json.MarshalIndent(simplified, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting document: %v", err)
	}
	return string(data)
}
