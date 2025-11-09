// -----------------------------------------------------------------------
// Document Persister - Integration with document storage system
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// DocumentPersister handles persistence of crawled documents to the document storage system
type DocumentPersister struct {
	documentStorage interfaces.DocumentStorage
	eventService    interfaces.EventService
	logger          arbor.ILogger
}

// NewDocumentPersister creates a new document persister
func NewDocumentPersister(documentStorage interfaces.DocumentStorage, eventService interfaces.EventService, logger arbor.ILogger) *DocumentPersister {
	return &DocumentPersister{
		documentStorage: documentStorage,
		eventService:    eventService,
		logger:          logger,
	}
}

// SaveCrawledDocument saves a crawled document to the document storage system
func (dp *DocumentPersister) SaveCrawledDocument(crawledDoc *CrawledDocument) error {
	// Convert to standard document model
	doc := crawledDoc.ToDocument()

	// Track whether this is a new document (for event publishing)
	isNewDocument := false

	// Check if document already exists (by source URL)
	existingDoc, err := dp.documentStorage.GetDocumentBySource("crawler", crawledDoc.SourceURL)
	if err == nil && existingDoc != nil {
		// Document exists, update it
		doc.ID = existingDoc.ID               // Keep the same ID
		doc.CreatedAt = existingDoc.CreatedAt // Preserve creation time

		if err := dp.documentStorage.UpdateDocument(doc); err != nil {
			dp.logger.Error().
				Err(err).
				Str("document_id", doc.ID).
				Str("source_url", crawledDoc.SourceURL).
				Msg("Failed to update existing crawled document")
			return fmt.Errorf("failed to update crawled document: %w", err)
		}

		dp.logger.Debug().
			Str("document_id", doc.ID).
			Str("source_url", crawledDoc.SourceURL).
			Str("job_id", crawledDoc.JobID).
			Int("content_size", crawledDoc.ContentSize).
			Msg("Updated existing crawled document")
	} else {
		// Document doesn't exist, create new one
		isNewDocument = true

		if err := dp.documentStorage.SaveDocument(doc); err != nil {
			dp.logger.Error().
				Err(err).
				Str("document_id", doc.ID).
				Str("source_url", crawledDoc.SourceURL).
				Msg("Failed to save new crawled document")
			return fmt.Errorf("failed to save crawled document: %w", err)
		}

		dp.logger.Debug().
			Str("document_id", doc.ID).
			Str("source_url", crawledDoc.SourceURL).
			Str("job_id", crawledDoc.JobID).
			Int("content_size", crawledDoc.ContentSize).
			Msg("Saved new crawled document")
	}

	// Publish document_saved event ONLY for NEW documents (not updates)
	// This prevents double-counting when the same URL is re-crawled
	if isNewDocument && dp.eventService != nil && crawledDoc.ParentJobID != "" {
		payload := map[string]interface{}{
			"job_id":        crawledDoc.JobID,
			"parent_job_id": crawledDoc.ParentJobID,
			"document_id":   doc.ID,
			"source_url":    crawledDoc.SourceURL,
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		event := interfaces.Event{
			Type:    interfaces.EventDocumentSaved,
			Payload: payload,
		}
		// Publish asynchronously to not block document save
		go func() {
			if err := dp.eventService.Publish(context.Background(), event); err != nil {
				dp.logger.Warn().
					Err(err).
					Str("document_id", doc.ID).
					Str("parent_job_id", crawledDoc.ParentJobID).
					Msg("Failed to publish document_saved event")
			}
		}()

		dp.logger.Debug().
			Str("document_id", doc.ID).
			Str("job_id", crawledDoc.JobID).
			Str("parent_job_id", crawledDoc.ParentJobID).
			Msg("Published document_saved event for new document")
	} else if !isNewDocument && crawledDoc.ParentJobID != "" {
		dp.logger.Debug().
			Str("document_id", doc.ID).
			Str("job_id", crawledDoc.JobID).
			Str("parent_job_id", crawledDoc.ParentJobID).
			Msg("Skipped document_saved event for updated document (prevents double-counting)")
	}

	return nil
}

// SaveCrawledDocuments saves multiple crawled documents in a batch operation
func (dp *DocumentPersister) SaveCrawledDocuments(crawledDocs []*CrawledDocument) error {
	if len(crawledDocs) == 0 {
		return nil
	}

	// Convert all crawled documents to standard documents
	docs := make([]*models.Document, len(crawledDocs))
	for i, crawledDoc := range crawledDocs {
		docs[i] = crawledDoc.ToDocument()
	}

	// Save all documents in batch
	if err := dp.documentStorage.SaveDocuments(docs); err != nil {
		dp.logger.Error().
			Err(err).
			Int("document_count", len(docs)).
			Msg("Failed to save crawled documents batch")
		return fmt.Errorf("failed to save crawled documents batch: %w", err)
	}

	dp.logger.Info().
		Int("document_count", len(docs)).
		Msg("Saved crawled documents batch")

	return nil
}

// GetCrawledDocument retrieves a crawled document by ID
func (dp *DocumentPersister) GetCrawledDocument(documentID string) (*CrawledDocument, error) {
	doc, err := dp.documentStorage.GetDocument(documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	crawledDoc, err := FromDocument(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document to crawled document: %w", err)
	}

	return crawledDoc, nil
}

// GetCrawledDocumentByURL retrieves a crawled document by source URL
func (dp *DocumentPersister) GetCrawledDocumentByURL(sourceURL string) (*CrawledDocument, error) {
	doc, err := dp.documentStorage.GetDocumentBySource("crawler", sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get document by URL: %w", err)
	}

	crawledDoc, err := FromDocument(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document to crawled document: %w", err)
	}

	return crawledDoc, nil
}

// GetCrawledDocumentsByJob retrieves all crawled documents for a specific job
func (dp *DocumentPersister) GetCrawledDocumentsByJob(jobID string) ([]*CrawledDocument, error) {
	// Use the document storage to search for documents with the job_id in metadata
	// This is a workaround since the interface doesn't have a direct method for this
	docs, err := dp.documentStorage.GetDocumentsBySource("crawler")
	if err != nil {
		return nil, fmt.Errorf("failed to get crawler documents: %w", err)
	}

	var crawledDocs []*CrawledDocument
	for _, doc := range docs {
		// Check if this document belongs to the specified job
		if doc.Metadata != nil {
			if docJobID, ok := doc.Metadata["job_id"].(string); ok && docJobID == jobID {
				crawledDoc, err := FromDocument(doc)
				if err != nil {
					dp.logger.Warn().
						Err(err).
						Str("document_id", doc.ID).
						Msg("Failed to convert document to crawled document")
					continue
				}
				crawledDocs = append(crawledDocs, crawledDoc)
			}
		}
	}

	return crawledDocs, nil
}

// GetCrawledDocumentsByParentJob retrieves all crawled documents for a parent job
func (dp *DocumentPersister) GetCrawledDocumentsByParentJob(parentJobID string) ([]*CrawledDocument, error) {
	// Use the document storage to search for documents with the parent_job_id in metadata
	docs, err := dp.documentStorage.GetDocumentsBySource("crawler")
	if err != nil {
		return nil, fmt.Errorf("failed to get crawler documents: %w", err)
	}

	var crawledDocs []*CrawledDocument
	for _, doc := range docs {
		// Check if this document belongs to the specified parent job
		if doc.Metadata != nil {
			if docParentJobID, ok := doc.Metadata["parent_job_id"].(string); ok && docParentJobID == parentJobID {
				crawledDoc, err := FromDocument(doc)
				if err != nil {
					dp.logger.Warn().
						Err(err).
						Str("document_id", doc.ID).
						Msg("Failed to convert document to crawled document")
					continue
				}
				crawledDocs = append(crawledDocs, crawledDoc)
			}
		}
	}

	return crawledDocs, nil
}

// DeleteCrawledDocument deletes a crawled document by ID
func (dp *DocumentPersister) DeleteCrawledDocument(documentID string) error {
	if err := dp.documentStorage.DeleteDocument(documentID); err != nil {
		dp.logger.Error().
			Err(err).
			Str("document_id", documentID).
			Msg("Failed to delete crawled document")
		return fmt.Errorf("failed to delete crawled document: %w", err)
	}

	dp.logger.Debug().
		Str("document_id", documentID).
		Msg("Deleted crawled document")

	return nil
}

// CountCrawledDocuments returns the total number of crawled documents
func (dp *DocumentPersister) CountCrawledDocuments() (int, error) {
	count, err := dp.documentStorage.CountDocumentsBySource("crawler")
	if err != nil {
		return 0, fmt.Errorf("failed to count crawled documents: %w", err)
	}

	return count, nil
}

// SearchCrawledDocuments performs full-text search on crawled documents
func (dp *DocumentPersister) SearchCrawledDocuments(query string, limit int) ([]*CrawledDocument, error) {
	docs, err := dp.documentStorage.FullTextSearch(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search crawled documents: %w", err)
	}

	var crawledDocs []*CrawledDocument
	for _, doc := range docs {
		// Only include crawler documents
		if doc.SourceType == "crawler" {
			crawledDoc, err := FromDocument(doc)
			if err != nil {
				dp.logger.Warn().
					Err(err).
					Str("document_id", doc.ID).
					Msg("Failed to convert document to crawled document")
				continue
			}
			crawledDocs = append(crawledDocs, crawledDoc)
		}
	}

	return crawledDocs, nil
}

// GetCrawledDocumentStats returns statistics about crawled documents
func (dp *DocumentPersister) GetCrawledDocumentStats() (*CrawledDocumentStats, error) {
	// Get overall document stats
	stats, err := dp.documentStorage.GetStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get document stats: %w", err)
	}

	// Count crawler documents specifically
	crawlerCount, err := dp.documentStorage.CountDocumentsBySource("crawler")
	if err != nil {
		return nil, fmt.Errorf("failed to count crawler documents: %w", err)
	}

	crawledStats := &CrawledDocumentStats{
		TotalDocuments:     crawlerCount,
		LastUpdated:        stats.LastUpdated,
		AverageContentSize: stats.AverageContentSize,
	}

	return crawledStats, nil
}

// CrawledDocumentStats represents statistics about crawled documents
type CrawledDocumentStats struct {
	TotalDocuments     int       `json:"total_documents"`
	LastUpdated        time.Time `json:"last_updated"`
	AverageContentSize int       `json:"average_content_size"`
}

// DocumentExists checks if a document already exists for the given URL
func (dp *DocumentPersister) DocumentExists(sourceURL string) (bool, error) {
	_, err := dp.documentStorage.GetDocumentBySource("crawler", sourceURL)
	if err != nil {
		// If error is "not found", document doesn't exist
		if err.Error() == "document not found" || err.Error() == "not found" {
			return false, nil
		}
		// Other errors are actual failures
		return false, fmt.Errorf("failed to check document existence: %w", err)
	}

	// Document exists
	return true, nil
}
