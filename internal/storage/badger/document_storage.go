package badger

import (
	"fmt"
	"regexp"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/timshannon/badgerhold/v4"
)

// DocumentStorage implements the DocumentStorage interface for Badger
type DocumentStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewDocumentStorage creates a new DocumentStorage instance
func NewDocumentStorage(db *BadgerDB, logger arbor.ILogger) interfaces.DocumentStorage {
	return &DocumentStorage{
		db:     db,
		logger: logger,
	}
}

func (s *DocumentStorage) SaveDocument(doc *models.Document) error {
	if doc.ID == "" {
		return fmt.Errorf("document ID is required")
	}

	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	if err := s.db.Store().Upsert(doc.ID, doc); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}
	return nil
}

func (s *DocumentStorage) SaveDocuments(docs []*models.Document) error {
	// BadgerHold doesn't support bulk insert in a single transaction easily exposed
	// but we can iterate. For better performance, we could use db.Badger().NewTransaction
	// but that bypasses BadgerHold's encoding.
	// For now, simple iteration.
	for _, doc := range docs {
		if err := s.SaveDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

func (s *DocumentStorage) GetDocument(id string) (*models.Document, error) {
	var doc models.Document
	if err := s.db.Store().Get(id, &doc); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return &doc, nil
}

func (s *DocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	var docs []models.Document
	err := s.db.Store().Find(&docs, badgerhold.Where("SourceType").Eq(sourceType).And("SourceID").Eq(sourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("document not found for source: %s/%s", sourceType, sourceID)
	}
	return &docs[0], nil
}

func (s *DocumentStorage) UpdateDocument(doc *models.Document) error {
	return s.SaveDocument(doc)
}

func (s *DocumentStorage) DeleteDocument(id string) error {
	if err := s.db.Store().Delete(id, &models.Document{}); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete document: %w", err)
	}
	return nil
}

func (s *DocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	// BadgerHold has limited text search capabilities (RegExp).
	// For true FTS, we'd need an external index or a more complex implementation.
	// This is a basic implementation using regex match on ContentMarkdown and Title.
	// WARNING: This is slow for large datasets.

	// Escape regex special characters in query to treat it as literal text
	escapedQuery := regexp.QuoteMeta(query)
	regex, err := regexp.Compile("(?i)" + escapedQuery) // Case insensitive
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	var docs []models.Document
	err = s.db.Store().Find(&docs, badgerhold.Where("ContentMarkdown").RegExp(regex).Or(badgerhold.Where("Title").RegExp(regex)).Limit(limit))
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	result := make([]*models.Document, len(docs))
	for i := range docs {
		result[i] = &docs[i]
	}
	return result, nil
}

func (s *DocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	// Not implemented for Badger yet - requires complex querying or secondary indexing strategy
	return nil, fmt.Errorf("SearchByIdentifier not implemented for Badger storage")
}

func (s *DocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	query := badgerhold.Where("ID").Ne("") // Select all

	if opts != nil {
		if opts.SourceType != "" {
			query = query.And("SourceType").Eq(opts.SourceType)
		}
		if opts.Limit > 0 {
			query = query.Limit(opts.Limit)
		}
		if opts.Offset > 0 {
			query = query.Skip(opts.Offset)
		}
	}

	var docs []models.Document
	if err := s.db.Store().Find(&docs, query); err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	result := make([]*models.Document, len(docs))
	for i := range docs {
		result[i] = &docs[i]
	}
	return result, nil
}

func (s *DocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	var docs []models.Document
	if err := s.db.Store().Find(&docs, badgerhold.Where("SourceType").Eq(sourceType)); err != nil {
		return nil, fmt.Errorf("failed to get documents by source: %w", err)
	}

	result := make([]*models.Document, len(docs))
	for i := range docs {
		result[i] = &docs[i]
	}
	return result, nil
}

func (s *DocumentStorage) CountDocuments() (int, error) {
	count, err := s.db.Store().Count(&models.Document{}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	return int(count), nil
}

func (s *DocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
	count, err := s.db.Store().Count(&models.Document{}, badgerhold.Where("SourceType").Eq(sourceType))
	if err != nil {
		return 0, fmt.Errorf("failed to count documents by source: %w", err)
	}
	return int(count), nil
}

func (s *DocumentStorage) GetStats() (*models.DocumentStats, error) {
	total, err := s.CountDocuments()
	if err != nil {
		return nil, err
	}

	// This is inefficient in Badger without maintaining separate counters
	// For now, we'll just do a count for known types
	jiraCount, _ := s.CountDocumentsBySource("jira")
	confluenceCount, _ := s.CountDocumentsBySource("confluence")

	return &models.DocumentStats{
		TotalDocuments:      total,
		JiraDocuments:       jiraCount,
		ConfluenceDocuments: confluenceCount,
		LastUpdated:         time.Now(),
		// AverageContentSize calculation omitted for performance
	}, nil
}

func (s *DocumentStorage) GetAllTags() ([]string, error) {
	// Requires iterating all documents or maintaining a separate tags index
	// Omitted for initial implementation
	return []string{}, nil
}

func (s *DocumentStorage) SetForceSyncPending(id string, pending bool) error {
	var doc models.Document
	if err := s.db.Store().Get(id, &doc); err != nil {
		return err
	}
	doc.ForceSyncPending = pending
	return s.SaveDocument(&doc)
}

func (s *DocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	var docs []models.Document
	if err := s.db.Store().Find(&docs, badgerhold.Where("ForceSyncPending").Eq(true)); err != nil {
		return nil, err
	}
	result := make([]*models.Document, len(docs))
	for i := range docs {
		result[i] = &docs[i]
	}
	return result, nil
}

func (s *DocumentStorage) ClearAll() error {
	return s.db.Store().DeleteMatching(&models.Document{}, nil)
}

func (s *DocumentStorage) RebuildFTS5Index() error {
	// No-op for Badger as we don't have FTS5
	return nil
}
