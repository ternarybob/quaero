package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// NOTE: Phase 5 - Removed unused imports: math, unsafe (no longer needed without embedding operations)

// DocumentStorage implements interfaces.DocumentStorage
type DocumentStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
	mu     sync.Mutex // Serializes write operations to prevent SQLITE_BUSY errors
}

// NewDocumentStorage creates a new document storage instance
func NewDocumentStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.DocumentStorage {
	return &DocumentStorage{
		db:     db,
		logger: logger,
	}
}

// SaveDocument saves a single document
func (d *DocumentStorage) SaveDocument(doc *models.Document) error {
	// Serialize metadata
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// NOTE: Phase 5 - Removed embedding serialization code

	var lastSyncedUnix *int64
	if doc.LastSynced != nil {
		unix := doc.LastSynced.Unix()
		lastSyncedUnix = &unix
	}

	// Smart upsert: preserve full content when upserting metadata, always replace when upserting full
	query := `
		INSERT INTO documents (
			id, source_type, source_id, title, content, content_markdown, detail_level,
			metadata, url, created_at, updated_at,
			last_synced, source_version, force_sync_pending
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_type, source_id) DO UPDATE SET
			title = excluded.title,
			content = CASE
				WHEN excluded.detail_level = 'full' THEN excluded.content
				WHEN detail_level = 'full' THEN content
				ELSE excluded.content
			END,
			content_markdown = CASE
				WHEN excluded.detail_level = 'full' THEN excluded.content_markdown
				WHEN detail_level = 'full' THEN content_markdown
				ELSE excluded.content_markdown
			END,
			detail_level = CASE
				WHEN excluded.detail_level = 'full' THEN 'full'
				ELSE detail_level
			END,
			metadata = excluded.metadata,
			url = excluded.url,
			updated_at = excluded.updated_at,
			last_synced = excluded.last_synced,
			source_version = excluded.source_version,
			force_sync_pending = excluded.force_sync_pending
	`

	detailLevel := doc.DetailLevel
	if detailLevel == "" {
		detailLevel = models.DetailLevelFull // Backward compatibility
	}

	_, err = d.db.db.Exec(query,
		doc.ID,
		doc.SourceType,
		doc.SourceID,
		doc.Title,
		doc.Content,
		doc.ContentMarkdown,
		detailLevel,
		string(metadataJSON),
		doc.URL,
		doc.CreatedAt.Unix(),
		doc.UpdatedAt.Unix(),
		lastSyncedUnix,
		doc.SourceVersion,
		doc.ForceSyncPending,
	)

	return err
}

// SaveDocuments saves multiple documents in batch
func (d *DocumentStorage) SaveDocuments(docs []*models.Document) error {
	// Serialize write operations to prevent SQLITE_BUSY errors when multiple
	// goroutines try to write simultaneously
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO documents (
			id, source_type, source_id, title, content, content_markdown, detail_level,
			metadata, url, created_at, updated_at,
			last_synced, source_version, force_sync_pending
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_type, source_id) DO UPDATE SET
			title = excluded.title,
			content = CASE
				WHEN excluded.detail_level = 'full' THEN excluded.content
				WHEN detail_level = 'full' THEN content
				ELSE excluded.content
			END,
			content_markdown = CASE
				WHEN excluded.detail_level = 'full' THEN excluded.content_markdown
				WHEN detail_level = 'full' THEN content_markdown
				ELSE excluded.content_markdown
			END,
			detail_level = CASE
				WHEN excluded.detail_level = 'full' THEN 'full'
				ELSE detail_level
			END,
			metadata = excluded.metadata,
			url = excluded.url,
			updated_at = excluded.updated_at,
			last_synced = excluded.last_synced,
			source_version = excluded.source_version,
			force_sync_pending = excluded.force_sync_pending
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		// NOTE: Phase 5 - Removed embedding serialization code

		var lastSyncedUnix *int64
		if doc.LastSynced != nil {
			unix := doc.LastSynced.Unix()
			lastSyncedUnix = &unix
		}

		detailLevel := doc.DetailLevel
		if detailLevel == "" {
			detailLevel = models.DetailLevelFull // Backward compatibility
		}

		_, err = stmt.Exec(
			doc.ID,
			doc.SourceType,
			doc.SourceID,
			doc.Title,
			doc.Content,
			doc.ContentMarkdown,
			detailLevel,
			string(metadataJSON),
			doc.URL,
			doc.CreatedAt.Unix(),
			doc.UpdatedAt.Unix(),
			lastSyncedUnix,
			doc.SourceVersion,
			doc.ForceSyncPending,
		)
		if err != nil {
			return fmt.Errorf("failed to save document %s: %w", doc.ID, err)
		}
	}

	return tx.Commit()
}

// GetDocument retrieves a document by ID
func (d *DocumentStorage) GetDocument(id string) (*models.Document, error) {
	query := `
		SELECT id, source_type, source_id, title, content, content_markdown, detail_level,
			   metadata, url, created_at, updated_at,
			   last_synced, source_version, force_sync_pending
		FROM documents
		WHERE id = ?
	`

	row := d.db.db.QueryRow(query, id)
	return d.scanDocument(row)
}

// GetDocumentBySource retrieves a document by source reference
func (d *DocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	query := `
		SELECT id, source_type, source_id, title, content, content_markdown, detail_level,
			   metadata, url, created_at, updated_at,
			   last_synced, source_version, force_sync_pending
		FROM documents
		WHERE source_type = ? AND source_id = ?
	`

	row := d.db.db.QueryRow(query, sourceType, sourceID)
	return d.scanDocument(row)
}

// UpdateDocument updates an existing document
func (d *DocumentStorage) UpdateDocument(doc *models.Document) error {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// NOTE: Phase 5 - Removed embedding serialization code

	query := `
		UPDATE documents SET
			title = ?,
			content = ?,
			content_markdown = ?,
			metadata = ?,
			url = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err = d.db.db.Exec(query,
		doc.Title,
		doc.Content,
		doc.ContentMarkdown,
		string(metadataJSON),
		doc.URL,
		time.Now().Unix(),
		doc.ID,
	)

	return err
}

// DeleteDocument deletes a document by ID
func (d *DocumentStorage) DeleteDocument(id string) error {
	_, err := d.db.db.Exec("DELETE FROM documents WHERE id = ?", id)
	return err
}

// FullTextSearch performs full-text search using FTS5
func (d *DocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	sqlQuery := `
		SELECT d.id, d.source_type, d.source_id, d.title, d.content, d.content_markdown, d.detail_level,
			   d.metadata, d.url, d.created_at, d.updated_at,
			   d.last_synced, d.source_version, d.force_sync_pending
		FROM documents d
		INNER JOIN documents_fts fts ON d.rowid = fts.rowid
		WHERE documents_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`

	rows, err := d.db.db.Query(sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return d.scanDocuments(rows)
}

// NOTE: Phase 5 - Removed VectorSearch method (vector similarity search no longer supported)
// NOTE: Phase 5 - Removed cosineSimilarity helper function
// NOTE: Phase 5 - Removed HybridSearch method (vector search component no longer available)

// SearchByIdentifier finds documents that reference a specific identifier (e.g., BUG-123, abc123def)
// Searches in:
//  1. metadata.issue_key field
//  2. metadata.referenced_issues array
//  3. Title (case-insensitive substring match)
//  4. Content (case-insensitive substring match)
func (d *DocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	if identifier == "" {
		return []*models.Document{}, nil
	}

	// Build exclude sources clause
	excludeClause := ""
	excludeParams := []interface{}{identifier, identifier, identifier, identifier}

	if len(excludeSources) > 0 {
		excludeClause = " AND source_type NOT IN ("
		for i, src := range excludeSources {
			if i > 0 {
				excludeClause += ", "
			}
			excludeClause += "?"
			excludeParams = append(excludeParams, src)
		}
		excludeClause += ")"
	}

	// Add limit parameter
	excludeParams = append(excludeParams, limit)

	query := fmt.Sprintf(`
		SELECT id, source_type, source_id, title, content, content_markdown, detail_level,
			   metadata, url, created_at, updated_at,
			   last_synced, source_version, force_sync_pending
		FROM documents
		WHERE (
			-- Search in metadata.issue_key (case-insensitive)
			LOWER(json_extract(metadata, '$.issue_key')) = LOWER(?)
			-- Search in metadata.referenced_issues array (case-insensitive)
			OR EXISTS (
				SELECT 1 FROM json_each(metadata, '$.referenced_issues')
				WHERE LOWER(json_each.value) = LOWER(?)
			)
			-- Search in title (case-insensitive)
			OR LOWER(title) LIKE '%%' || LOWER(?) || '%%'
			-- Search in content (case-insensitive)
			OR LOWER(content) LIKE '%%' || LOWER(?) || '%%'
		)
		%s
		ORDER BY updated_at DESC
		LIMIT ?
	`, excludeClause)

	rows, err := d.db.db.Query(query, excludeParams...)
	if err != nil {
		return nil, fmt.Errorf("failed to search by identifier: %w", err)
	}
	defer rows.Close()

	return d.scanDocuments(rows)
}

// ListDocuments lists documents with pagination
func (d *DocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	if opts == nil {
		opts = &interfaces.ListOptions{
			Limit:    50,
			Offset:   0,
			OrderBy:  "updated_at",
			OrderDir: "desc",
		}
	}

	// Apply defaults if not set (prevents SQL syntax errors)
	if opts.OrderBy == "" {
		opts.OrderBy = "updated_at"
	}
	if opts.OrderDir == "" {
		opts.OrderDir = "desc"
	}
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	query := "SELECT id, source_type, source_id, title, content, content_markdown, detail_level, metadata, url, created_at, updated_at, last_synced, source_version, force_sync_pending FROM documents"

	// Add WHERE clause if filtering by source
	if opts.SourceType != "" {
		query += " WHERE source_type = '" + opts.SourceType + "'"
	}

	// Add ORDER BY
	query += fmt.Sprintf(" ORDER BY %s %s", opts.OrderBy, opts.OrderDir)

	// Add LIMIT and OFFSET
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", opts.Limit, opts.Offset)

	rows, err := d.db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return d.scanDocuments(rows)
}

// GetDocumentsBySource retrieves all documents for a source type
func (d *DocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	query := `
		SELECT id, source_type, source_id, title, content, content_markdown, detail_level,
			   metadata, url, created_at, updated_at,
			   last_synced, source_version, force_sync_pending
		FROM documents
		WHERE source_type = ?
		ORDER BY updated_at DESC
	`

	rows, err := d.db.db.Query(query, sourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return d.scanDocuments(rows)
}

// CountDocuments returns total document count
func (d *DocumentStorage) CountDocuments() (int, error) {
	var count int
	err := d.db.db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	return count, err
}

// CountDocumentsBySource returns document count for a source type
func (d *DocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
	var count int
	err := d.db.db.QueryRow("SELECT COUNT(*) FROM documents WHERE source_type = ?", sourceType).Scan(&count)
	return count, err
}

// NOTE: Phase 5 - Removed CountVectorized method (embedding counts no longer tracked)

// GetStats retrieves document statistics
func (d *DocumentStorage) GetStats() (*models.DocumentStats, error) {
	stats := &models.DocumentStats{
		DocumentsBySource: make(map[string]int),
		LastUpdated:       time.Now(),
	}

	// Total documents
	var err error
	stats.TotalDocuments, err = d.CountDocuments()
	if err != nil {
		return nil, err
	}

	// NOTE: Phase 5 - Removed vectorized count tracking (no longer using embeddings)

	// Count by source
	rows, err := d.db.db.Query("SELECT source_type, COUNT(*) FROM documents GROUP BY source_type")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sourceType string
		var count int
		if err := rows.Scan(&sourceType, &count); err != nil {
			return nil, err
		}
		stats.DocumentsBySource[sourceType] = count
	}

	// Populate individual source counts from DocumentsBySource
	stats.JiraDocuments = stats.DocumentsBySource["jira"]
	stats.ConfluenceDocuments = stats.DocumentsBySource["confluence"]

	// NOTE: Phase 5 - Removed embedding model query

	// Average content size
	var avgSize sql.NullInt64
	d.db.db.QueryRow("SELECT AVG(LENGTH(content)) FROM documents").Scan(&avgSize)
	if avgSize.Valid {
		stats.AverageContentSize = int(avgSize.Int64)
	}

	return stats, nil
}

// NOTE: Phase 5 - Removed SaveChunk method (DocumentChunk model removed)
// NOTE: Phase 5 - Removed GetChunks method (DocumentChunk model removed)
// NOTE: Phase 5 - Removed DeleteChunks method (DocumentChunk model removed)

// ClearAll deletes all documents
func (d *DocumentStorage) ClearAll() error {
	_, err := d.db.db.Exec("DELETE FROM documents")
	return err
}

// NOTE: Phase 5 - Removed ClearAllEmbeddings method (embeddings no longer exist)

// SetForceSyncPending sets the force sync pending flag for a document
func (d *DocumentStorage) SetForceSyncPending(id string, pending bool) error {
	_, err := d.db.db.Exec("UPDATE documents SET force_sync_pending = ? WHERE id = ?", pending, id)
	return err
}

// NOTE: Phase 5 - Removed SetForceEmbedPending method (force_embed_pending field removed)

// GetDocumentsForceSync gets documents with force sync pending
func (d *DocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	query := `
		SELECT id, source_type, source_id, title, content, content_markdown, detail_level,
			   metadata, url, created_at, updated_at,
			   last_synced, source_version, force_sync_pending
		FROM documents
		WHERE force_sync_pending = 1
	`

	rows, err := d.db.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return d.scanDocuments(rows)
}

// NOTE: Phase 5 - Removed GetDocumentsForceEmbed method (force_embed_pending field removed)
// NOTE: Phase 5 - Removed GetUnvectorizedDocuments method (embeddings no longer tracked)

// Helper functions

func (d *DocumentStorage) scanDocument(row *sql.Row) (*models.Document, error) {
	var doc models.Document
	var metadataJSON string
	var createdAt, updatedAt int64
	var lastSynced sql.NullInt64
	var contentMarkdown, url, sourceVersion, detailLevel sql.NullString
	var forceSyncPending sql.NullBool

	// NOTE: Phase 5 - Removed embeddingData and embeddingModel scan targets

	err := row.Scan(
		&doc.ID,
		&doc.SourceType,
		&doc.SourceID,
		&doc.Title,
		&doc.Content,
		&contentMarkdown,
		&detailLevel,
		&metadataJSON,
		&url,
		&createdAt,
		&updatedAt,
		&lastSynced,
		&sourceVersion,
		&forceSyncPending,
	)
	if err != nil {
		return nil, err
	}

	// Parse optional fields
	if contentMarkdown.Valid {
		doc.ContentMarkdown = contentMarkdown.String
	}
	if detailLevel.Valid {
		doc.DetailLevel = detailLevel.String
	} else {
		doc.DetailLevel = models.DetailLevelFull // Backward compatibility with NULL values
	}
	if url.Valid {
		doc.URL = url.String
	}
	if sourceVersion.Valid {
		doc.SourceVersion = sourceVersion.String
	}
	if forceSyncPending.Valid {
		doc.ForceSyncPending = forceSyncPending.Bool
	}
	if lastSynced.Valid {
		t := time.Unix(lastSynced.Int64, 0)
		doc.LastSynced = &t
	}

	// NOTE: Phase 5 - Removed embedding deserialization code

	// Parse metadata
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	doc.CreatedAt = time.Unix(createdAt, 0)
	doc.UpdatedAt = time.Unix(updatedAt, 0)

	return &doc, nil
}

func (d *DocumentStorage) scanDocuments(rows *sql.Rows) ([]*models.Document, error) {
	docs := make([]*models.Document, 0)

	for rows.Next() {
		var doc models.Document
		var metadataJSON string
		var createdAt, updatedAt int64
		var lastSynced sql.NullInt64
		var contentMarkdown, url, sourceVersion, detailLevel sql.NullString
		var forceSyncPending sql.NullBool

		// NOTE: Phase 5 - Removed embeddingData and embeddingModel scan targets

		err := rows.Scan(
			&doc.ID,
			&doc.SourceType,
			&doc.SourceID,
			&doc.Title,
			&doc.Content,
			&contentMarkdown,
			&detailLevel,
			&metadataJSON,
			&url,
			&createdAt,
			&updatedAt,
			&lastSynced,
			&sourceVersion,
			&forceSyncPending,
		)
		if err != nil {
			return nil, err
		}

		if contentMarkdown.Valid {
			doc.ContentMarkdown = contentMarkdown.String
		}
		if detailLevel.Valid {
			doc.DetailLevel = detailLevel.String
		} else {
			doc.DetailLevel = models.DetailLevelFull // Backward compatibility with NULL values
		}
		if url.Valid {
			doc.URL = url.String
		}
		if sourceVersion.Valid {
			doc.SourceVersion = sourceVersion.String
		}
		if forceSyncPending.Valid {
			doc.ForceSyncPending = forceSyncPending.Bool
		}
		if lastSynced.Valid {
			t := time.Unix(lastSynced.Int64, 0)
			doc.LastSynced = &t
		}

		// NOTE: Phase 5 - Removed embedding deserialization code

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &doc.Metadata); err != nil {
				d.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to unmarshal metadata")
				doc.Metadata = make(map[string]interface{})
			}
		} else {
			doc.Metadata = make(map[string]interface{})
		}

		doc.CreatedAt = time.Unix(createdAt, 0)
		doc.UpdatedAt = time.Unix(updatedAt, 0)

		docs = append(docs, &doc)
	}

	return docs, nil
}

// NOTE: Phase 5 - Removed scanChunk method (DocumentChunk model removed)
// NOTE: Phase 5 - Removed serializeEmbedding helper (embeddings no longer stored)
// NOTE: Phase 5 - Removed deserializeEmbedding helper (embeddings no longer stored)
