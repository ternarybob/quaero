package chat

import (
	"fmt"
	"strings"

	"github.com/ternarybob/quaero/internal/models"
)

// formatDocumentPointerRAG formats a document with emphasis on cross-source relationships
// and identifiers, making it easier for the LLM to trace connections
func formatDocumentPointerRAG(doc *models.Document, index int) string {
	var parts []string

	// Document header with index
	parts = append(parts, fmt.Sprintf("=== Document %d: %s ===", index+1, doc.SourceType))

	// Title
	if doc.Title != "" {
		parts = append(parts, fmt.Sprintf("Title: %s", doc.Title))
	}

	// URL (for reference)
	if doc.URL != "" {
		parts = append(parts, fmt.Sprintf("URL: %s", doc.URL))
	}

	// Key metadata fields that indicate relationships
	metadataStr := formatMetadataForPointerRAG(doc.Metadata, doc.SourceType)
	if metadataStr != "" {
		parts = append(parts, metadataStr)
	}

	// Content (truncated aggressively to prevent context overflow)
	// With Pointer RAG's detailed formatting, we need shorter excerpts
	contentPreview := truncateContent(doc.Content, 300) // Reduced from 800
	parts = append(parts, fmt.Sprintf("Content:\n%s", contentPreview))

	parts = append(parts, "") // Empty line separator
	return strings.Join(parts, "\n")
}

// formatMetadataForPointerRAG extracts and formats key metadata fields that
// indicate cross-source relationships
func formatMetadataForPointerRAG(metadata map[string]interface{}, sourceType string) string {
	var parts []string

	// For Jira documents
	if sourceType == "jira" {
		if issueKey, ok := metadata["issue_key"].(string); ok && issueKey != "" {
			parts = append(parts, fmt.Sprintf("Issue Key: %s", issueKey))
		}
		if projectKey, ok := metadata["project_key"].(string); ok && projectKey != "" {
			parts = append(parts, fmt.Sprintf("Project: %s", projectKey))
		}
		if issueType, ok := metadata["issue_type"].(string); ok && issueType != "" {
			parts = append(parts, fmt.Sprintf("Type: %s", issueType))
		}
		if status, ok := metadata["status"].(string); ok && status != "" {
			parts = append(parts, fmt.Sprintf("Status: %s", status))
		}
		if priority, ok := metadata["priority"].(string); ok && priority != "" {
			parts = append(parts, fmt.Sprintf("Priority: %s", priority))
		}
	}

	// For Confluence documents
	if sourceType == "confluence" {
		if pageID, ok := metadata["page_id"].(string); ok && pageID != "" {
			parts = append(parts, fmt.Sprintf("Page ID: %s", pageID))
		}
		if spaceKey, ok := metadata["space_key"].(string); ok && spaceKey != "" {
			parts = append(parts, fmt.Sprintf("Space: %s", spaceKey))
		}
		if spaceName, ok := metadata["space_name"].(string); ok && spaceName != "" {
			parts = append(parts, fmt.Sprintf("Space Name: %s", spaceName))
		}
	}

	// For GitHub documents
	if sourceType == "github" {
		if repoName, ok := metadata["repo_name"].(string); ok && repoName != "" {
			parts = append(parts, fmt.Sprintf("Repository: %s", repoName))
		}
		if commitSHA, ok := metadata["commit_sha"].(string); ok && commitSHA != "" {
			parts = append(parts, fmt.Sprintf("Commit: %s", commitSHA))
		}
		if branch, ok := metadata["branch"].(string); ok && branch != "" {
			parts = append(parts, fmt.Sprintf("Branch: %s", branch))
		}
		if author, ok := metadata["author"].(string); ok && author != "" {
			parts = append(parts, fmt.Sprintf("Author: %s", author))
		}
	}

	// Cross-source references (CRITICAL for Pointer RAG)
	if referencedIssues, ok := metadata["referenced_issues"].([]interface{}); ok && len(referencedIssues) > 0 {
		issueStrs := make([]string, 0, len(referencedIssues))
		for _, ref := range referencedIssues {
			if refStr, ok := ref.(string); ok {
				issueStrs = append(issueStrs, refStr)
			}
		}
		if len(issueStrs) > 0 {
			parts = append(parts, fmt.Sprintf("🔗 References Issues: %s", strings.Join(issueStrs, ", ")))
		}
	}

	if referencedPRs, ok := metadata["referenced_prs"].([]interface{}); ok && len(referencedPRs) > 0 {
		prStrs := make([]string, 0, len(referencedPRs))
		for _, ref := range referencedPRs {
			if refStr, ok := ref.(string); ok {
				prStrs = append(prStrs, refStr)
			}
		}
		if len(prStrs) > 0 {
			parts = append(parts, fmt.Sprintf("🔗 References PRs: %s", strings.Join(prStrs, ", ")))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "Metadata:\n  " + strings.Join(parts, "\n  ")
}

// buildPointerRAGContextText builds the context string specifically for Pointer RAG,
// with emphasis on cross-source connections and relationship tracing
func buildPointerRAGContextText(docs []*models.Document, identifiers []string) string {
	if len(docs) == 0 {
		return ""
	}

	var parts []string

	// Header explaining the context structure
	parts = append(parts, "KNOWLEDGE BASE CONTEXT")
	parts = append(parts, "======================")
	parts = append(parts, "")

	// If identifiers were found, list them to help the LLM understand the connections
	if len(identifiers) > 0 {
		parts = append(parts, "Key Identifiers Found in Search:")
		for _, id := range identifiers {
			parts = append(parts, fmt.Sprintf("  • %s", id))
		}
		parts = append(parts, "")
		parts = append(parts, "The following documents are connected through these identifiers.")
		parts = append(parts, "Pay special attention to cross-source references.")
		parts = append(parts, "")
	}

	// Group documents by source type for better organization
	jiraDocs := []*models.Document{}
	confluenceDocs := []*models.Document{}
	githubDocs := []*models.Document{}
	otherDocs := []*models.Document{}

	for _, doc := range docs {
		switch strings.ToLower(doc.SourceType) {
		case "jira":
			jiraDocs = append(jiraDocs, doc)
		case "confluence":
			confluenceDocs = append(confluenceDocs, doc)
		case "github":
			githubDocs = append(githubDocs, doc)
		default:
			otherDocs = append(otherDocs, doc)
		}
	}

	// Format each group
	docIndex := 0
	if len(jiraDocs) > 0 {
		parts = append(parts, fmt.Sprintf("--- JIRA ISSUES (%d) ---", len(jiraDocs)))
		parts = append(parts, "")
		for _, doc := range jiraDocs {
			parts = append(parts, formatDocumentPointerRAG(doc, docIndex))
			docIndex++
		}
	}

	if len(confluenceDocs) > 0 {
		parts = append(parts, fmt.Sprintf("--- CONFLUENCE PAGES (%d) ---", len(confluenceDocs)))
		parts = append(parts, "")
		for _, doc := range confluenceDocs {
			parts = append(parts, formatDocumentPointerRAG(doc, docIndex))
			docIndex++
		}
	}

	if len(githubDocs) > 0 {
		parts = append(parts, fmt.Sprintf("--- GITHUB COMMITS (%d) ---", len(githubDocs)))
		parts = append(parts, "")
		for _, doc := range githubDocs {
			parts = append(parts, formatDocumentPointerRAG(doc, docIndex))
			docIndex++
		}
	}

	if len(otherDocs) > 0 {
		parts = append(parts, fmt.Sprintf("--- OTHER DOCUMENTS (%d) ---", len(otherDocs)))
		parts = append(parts, "")
		for _, doc := range otherDocs {
			parts = append(parts, formatDocumentPointerRAG(doc, docIndex))
			docIndex++
		}
	}

	parts = append(parts, "======================")
	parts = append(parts, "END OF CONTEXT")
	parts = append(parts, "")

	return strings.Join(parts, "\n")
}
