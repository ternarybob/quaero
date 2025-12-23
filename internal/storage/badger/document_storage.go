package badger

import (
	"fmt"
	"regexp"
	"sort"
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

	s.logger.Debug().
		Str("document_id", doc.ID).
		Str("source_type", doc.SourceType).
		Msg("BadgerDB: SaveDocument starting")

	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	if err := s.db.Store().Upsert(doc.ID, doc); err != nil {
		s.logger.Error().Err(err).
			Str("document_id", doc.ID).
			Msg("BadgerDB: Failed to save document")
		return fmt.Errorf("failed to save document: %w", err)
	}

	s.logger.Debug().
		Str("document_id", doc.ID).
		Msg("BadgerDB: SaveDocument completed")
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

	hasTags := opts != nil && len(opts.Tags) > 0

	if opts != nil {
		if opts.SourceType != "" {
			query = query.And("SourceType").Eq(opts.SourceType)
		}
		// Don't apply Limit/Offset yet if we need to filter by tags in Go
		// BadgerHold doesn't support array contains, so we filter post-query
		if !hasTags {
			if opts.Limit > 0 {
				query = query.Limit(opts.Limit)
			}
			if opts.Offset > 0 {
				query = query.Skip(opts.Offset)
			}
		}
	}

	var docs []models.Document
	if err := s.db.Store().Find(&docs, query); err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	s.logger.Debug().
		Int("total_docs", len(docs)).
		Bool("has_tags_filter", hasTags).
		Msg("BadgerDB: ListDocuments query completed")

	// If tags filter is specified, filter documents that have ALL matching tags (AND logic)
	if hasTags {
		s.logger.Debug().
			Strs("filter_tags", opts.Tags).
			Msg("BadgerDB: Filtering by tags (AND logic)")

		filtered := make([]models.Document, 0)
		for _, doc := range docs {
			if hasAllDocTags(doc.Tags, opts.Tags) {
				filtered = append(filtered, doc)
			}
		}

		s.logger.Debug().
			Int("total_before_filter", len(docs)).
			Int("total_after_filter", len(filtered)).
			Msg("BadgerDB: Tag filtering completed")

		docs = filtered

		// Apply offset and limit after tag filtering
		if opts.Offset > 0 && opts.Offset < len(docs) {
			docs = docs[opts.Offset:]
		} else if opts.Offset >= len(docs) {
			docs = []models.Document{}
		}
		if opts.Limit > 0 && opts.Limit < len(docs) {
			docs = docs[:opts.Limit]
		}
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
	// Iterate all documents and collect unique tags
	var docs []models.Document
	if err := s.db.Store().Find(&docs, nil); err != nil {
		return nil, fmt.Errorf("failed to fetch documents for tags: %w", err)
	}

	// Use a map to track unique tags
	tagSet := make(map[string]struct{})
	for _, doc := range docs {
		for _, tag := range doc.Tags {
			tagSet[tag] = struct{}{}
		}
	}

	// Convert map keys to sorted slice for consistent ordering
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	return tags, nil
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

// hasAllDocTags checks if docTags contains all required tags (AND logic)
func hasAllDocTags(docTags, requiredTags []string) bool {
	tagSet := make(map[string]bool, len(docTags))
	for _, tag := range docTags {
		tagSet[tag] = true
	}

	for _, required := range requiredTags {
		if !tagSet[required] {
			return false
		}
	}
	return true
}
