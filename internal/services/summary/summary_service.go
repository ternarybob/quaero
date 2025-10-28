// -----------------------------------------------------------------------
// Last Modified: Sunday, 13th October 2025 8:00:00 am
// Modified By: Claude Code
// -----------------------------------------------------------------------

package summary

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// Service generates and maintains summary documents about the corpus
// These summary documents are embedded and searchable via RAG,
// allowing queries like "how many documents are in the system"
type Service struct {
	docStorage interfaces.DocumentStorage
	docService interfaces.DocumentService
	logger     arbor.ILogger
}

// NewService creates a new summary document service
func NewService(
	docStorage interfaces.DocumentStorage,
	docService interfaces.DocumentService,
	logger arbor.ILogger,
) *Service {
	s := &Service{
		docStorage: docStorage,
		docService: docService,
		logger:     logger,
	}

	return s
}

// GenerateSummaryDocument creates/updates a special summary document
// containing metadata about the document corpus
func (s *Service) GenerateSummaryDocument(ctx context.Context) error {
	s.logger.Info().Msg("Generating corpus summary document")

	// Get document counts by source type
	totalDocs, err := s.docStorage.CountDocuments()
	if err != nil {
		return fmt.Errorf("failed to count total documents: %w", err)
	}

	jiraDocs, err := s.docStorage.CountDocumentsBySource("jira")
	if err != nil {
		return fmt.Errorf("failed to count jira documents: %w", err)
	}

	confluenceDocs, err := s.docStorage.CountDocumentsBySource("confluence")
	if err != nil {
		return fmt.Errorf("failed to count confluence documents: %w", err)
	}

	// NOTE: Phase 5 - Removed embedded document count (no longer using embeddings)

	// Generate summary content
	now := time.Now().UTC()
	content := fmt.Sprintf(`QUAERO DOCUMENT CORPUS SUMMARY

This document contains metadata about the Quaero document corpus.
It is automatically generated and updated to provide queryable information
about the system's knowledge base.

Last Updated: %s

CORPUS STATISTICS:
- Total Documents: %d
- Jira Issues: %d
- Confluence Pages: %d

SOURCE BREAKDOWN:
The corpus contains documents from multiple sources:
1. Jira: Project management and issue tracking documents (%d total)
2. Confluence: Wiki pages and documentation (%d total)

This summary is updated automatically every 5 minutes via the scheduler
and at application startup.

Questions you can ask about this data:
- "How many documents are in the system?"
- "How many Jira issues are indexed?"
- "How many Confluence pages are available?"
- "What is the total document count?"
`,
		now.Format(time.RFC3339),
		totalDocs,
		jiraDocs,
		confluenceDocs,
		jiraDocs,
		confluenceDocs,
	)

	// Create document with well-known ID
	lastSynced := now
	summaryDoc := &models.Document{
		ID:              "corpus-summary-metadata",
		SourceID:        "system",
		SourceType:      "system",
		Title:           "Quaero Corpus Summary - Document Statistics and Metadata",
		ContentMarkdown: content,
		LastSynced:      &lastSynced,
		CreatedAt:       now,
		UpdatedAt:       now,
		// NOTE: Phase 5 - Embedding removal: no longer using embeddings
	}

	// Save document (upsert by ID)
	if err := s.docService.SaveDocuments(ctx, []*models.Document{summaryDoc}); err != nil {
		return fmt.Errorf("failed to save summary document: %w", err)
	}

	s.logger.Info().
		Int("total_docs", totalDocs).
		Int("jira_docs", jiraDocs).
		Int("confluence_docs", confluenceDocs).
		Msg("Corpus summary document generated successfully")

	return nil
}
