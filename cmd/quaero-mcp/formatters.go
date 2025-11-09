package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/models"
)

// formatSearchResults formats search results as markdown
func formatSearchResults(query string, docs []*models.Document) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search Results for \"%s\" (%d results)\n\n", query, len(docs)))

	if len(docs) == 0 {
		sb.WriteString("No results found.\n")
		return sb.String()
	}

	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, doc.Title))
		sb.WriteString(fmt.Sprintf("**Source:** %s (%s)\n", doc.SourceType, doc.SourceID))
		if doc.URL != "" {
			sb.WriteString(fmt.Sprintf("**URL:** %s\n", doc.URL))
		}
		sb.WriteString(fmt.Sprintf("**Updated:** %s\n\n", doc.UpdatedAt.Format(time.RFC3339)))

		// Content preview (first 300 chars)
		content := doc.ContentMarkdown
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		sb.WriteString("#### Content:\n")
		sb.WriteString(content)
		sb.WriteString("\n\n")

		// Metadata
		if len(doc.Metadata) > 0 {
			metadataJSON, _ := json.MarshalIndent(doc.Metadata, "", "  ")
			sb.WriteString(fmt.Sprintf("**Metadata:** %s\n", string(metadataJSON)))
		}
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}

// formatDocument formats a single document as markdown
func formatDocument(doc *models.Document) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", doc.Title))
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", doc.ID))
	sb.WriteString(fmt.Sprintf("**Source:** %s (%s)\n", doc.SourceType, doc.SourceID))
	if doc.URL != "" {
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", doc.URL))
	}
	sb.WriteString(fmt.Sprintf("**Created:** %s\n", doc.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Updated:** %s\n\n", doc.UpdatedAt.Format(time.RFC3339)))

	sb.WriteString("## Content\n\n")
	sb.WriteString(doc.ContentMarkdown)
	sb.WriteString("\n\n")

	if len(doc.Metadata) > 0 {
		sb.WriteString("## Metadata\n\n```json\n")
		metadataJSON, _ := json.MarshalIndent(doc.Metadata, "", "  ")
		sb.WriteString(string(metadataJSON))
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

// formatRecentDocuments formats recent documents list as markdown
func formatRecentDocuments(docs []*models.Document, limit int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Recent Documents (%d of %d)\n\n", len(docs), limit))

	if len(docs) == 0 {
		sb.WriteString("No documents found.\n")
		return sb.String()
	}

	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("%d. **%s** (%s - %s)\n", i+1, doc.Title, doc.SourceType, doc.SourceID))
		sb.WriteString(fmt.Sprintf("   Updated: %s\n", doc.UpdatedAt.Format(time.RFC3339)))
		if doc.URL != "" {
			sb.WriteString(fmt.Sprintf("   URL: %s\n", doc.URL))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatRelatedDocuments formats related documents as markdown
func formatRelatedDocuments(reference string, docs []*models.Document) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Documents Referencing \"%s\" (%d results)\n\n", reference, len(docs)))

	if len(docs) == 0 {
		sb.WriteString("No related documents found.\n")
		return sb.String()
	}

	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, doc.Title))
		sb.WriteString(fmt.Sprintf("**Source:** %s (%s)\n", doc.SourceType, doc.SourceID))
		if doc.URL != "" {
			sb.WriteString(fmt.Sprintf("**URL:** %s\n", doc.URL))
		}
		sb.WriteString(fmt.Sprintf("**Updated:** %s\n\n", doc.UpdatedAt.Format(time.RFC3339)))

		// Content preview with reference highlighted
		content := doc.ContentMarkdown
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		sb.WriteString("#### Content:\n")
		sb.WriteString(content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}
