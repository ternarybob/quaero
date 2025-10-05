package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"unsafe"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// DocumentStorage implements interfaces.DocumentStorage
type DocumentStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
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

	// Serialize embedding
	var embeddingData []byte
	if doc.Embedding != nil && len(doc.Embedding) > 0 {
		embeddingData, err = serializeEmbedding(doc.Embedding)
		if err != nil {
			return fmt.Errorf("failed to serialize embedding: %w", err)
		}
	}

	query := `
		INSERT INTO documents (
			id, source_type, source_id, title, content, content_markdown,
			embedding, embedding_model, metadata, url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_type, source_id) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			content_markdown = excluded.content_markdown,
			embedding = excluded.embedding,
			embedding_model = excluded.embedding_model,
			metadata = excluded.metadata,
			url = excluded.url,
			updated_at = excluded.updated_at
	`

	_, err = d.db.db.Exec(query,
		doc.ID,
		doc.SourceType,
		doc.SourceID,
		doc.Title,
		doc.Content,
		doc.ContentMarkdown,
		embeddingData,
		doc.EmbeddingModel,
		string(metadataJSON),
		doc.URL,
		doc.CreatedAt.Unix(),
		doc.UpdatedAt.Unix(),
	)

	return err
}

// SaveDocuments saves multiple documents in batch
func (d *DocumentStorage) SaveDocuments(docs []*models.Document) error {
	tx, err := d.db.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO documents (
			id, source_type, source_id, title, content, content_markdown,
			embedding, embedding_model, metadata, url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_type, source_id) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			content_markdown = excluded.content_markdown,
			embedding = excluded.embedding,
			embedding_model = excluded.embedding_model,
			metadata = excluded.metadata,
			url = excluded.url,
			updated_at = excluded.updated_at
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

		var embeddingData []byte
		if doc.Embedding != nil && len(doc.Embedding) > 0 {
			embeddingData, err = serializeEmbedding(doc.Embedding)
			if err != nil {
				return fmt.Errorf("failed to serialize embedding: %w", err)
			}
		}

		_, err = stmt.Exec(
			doc.ID,
			doc.SourceType,
			doc.SourceID,
			doc.Title,
			doc.Content,
			doc.ContentMarkdown,
			embeddingData,
			doc.EmbeddingModel,
			string(metadataJSON),
			doc.URL,
			doc.CreatedAt.Unix(),
			doc.UpdatedAt.Unix(),
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
		SELECT id, source_type, source_id, title, content, content_markdown,
			   embedding, embedding_model, metadata, url, created_at, updated_at
		FROM documents
		WHERE id = ?
	`

	row := d.db.db.QueryRow(query, id)
	return d.scanDocument(row)
}

// GetDocumentBySource retrieves a document by source reference
func (d *DocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	query := `
		SELECT id, source_type, source_id, title, content, content_markdown,
			   embedding, embedding_model, metadata, url, created_at, updated_at
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

	var embeddingData []byte
	if doc.Embedding != nil && len(doc.Embedding) > 0 {
		embeddingData, err = serializeEmbedding(doc.Embedding)
		if err != nil {
			return fmt.Errorf("failed to serialize embedding: %w", err)
		}
	}

	query := `
		UPDATE documents SET
			title = ?,
			content = ?,
			content_markdown = ?,
			embedding = ?,
			embedding_model = ?,
			metadata = ?,
			url = ?,
			updated_at = ?
		WHERE id = ?
	`

	_, err = d.db.db.Exec(query,
		doc.Title,
		doc.Content,
		doc.ContentMarkdown,
		embeddingData,
		doc.EmbeddingModel,
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
		SELECT d.id, d.source_type, d.source_id, d.title, d.content, d.content_markdown,
			   d.embedding, d.embedding_model, d.metadata, d.url, d.created_at, d.updated_at
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

// VectorSearch performs vector similarity search
func (d *DocumentStorage) VectorSearch(embedding []float32, limit int) ([]*models.Document, error) {
	// TODO: Implement with sqlite-vec when available
	// For now, return error indicating not implemented
	return nil, fmt.Errorf("vector search not yet implemented - requires sqlite-vec extension")
}

// HybridSearch combines keyword and vector search
func (d *DocumentStorage) HybridSearch(query string, embedding []float32, limit int) ([]*models.Document, error) {
	// For now, fall back to full-text search
	d.logger.Warn().Msg("Hybrid search not implemented, using full-text search only")
	return d.FullTextSearch(query, limit)
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

	query := "SELECT id, source_type, source_id, title, content, content_markdown, embedding, embedding_model, metadata, url, created_at, updated_at FROM documents"

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
		SELECT id, source_type, source_id, title, content, content_markdown,
			   embedding, embedding_model, metadata, url, created_at, updated_at
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

// CountVectorized returns count of documents with embeddings
func (d *DocumentStorage) CountVectorized() (int, error) {
	var count int
	err := d.db.db.QueryRow("SELECT COUNT(*) FROM documents WHERE embedding IS NOT NULL").Scan(&count)
	return count, err
}

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

	// Vectorized count
	stats.VectorizedCount, err = d.CountVectorized()
	if err != nil {
		return nil, err
	}

	stats.PendingVectorize = stats.TotalDocuments - stats.VectorizedCount

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

	// Get embedding model (from any document that has one)
	d.db.db.QueryRow("SELECT embedding_model FROM documents WHERE embedding_model IS NOT NULL LIMIT 1").Scan(&stats.EmbeddingModel)

	// Average content size
	var avgSize sql.NullInt64
	d.db.db.QueryRow("SELECT AVG(LENGTH(content)) FROM documents").Scan(&avgSize)
	if avgSize.Valid {
		stats.AverageContentSize = int(avgSize.Int64)
	}

	return stats, nil
}

// SaveChunk saves a document chunk
func (d *DocumentStorage) SaveChunk(chunk *models.DocumentChunk) error {
	embeddingData, err := serializeEmbedding(chunk.Embedding)
	if err != nil {
		return fmt.Errorf("failed to serialize embedding: %w", err)
	}

	query := `
		INSERT INTO document_chunks (id, document_id, chunk_index, content, embedding, token_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(document_id, chunk_index) DO UPDATE SET
			content = excluded.content,
			embedding = excluded.embedding,
			token_count = excluded.token_count
	`

	_, err = d.db.db.Exec(query,
		chunk.ID,
		chunk.DocumentID,
		chunk.ChunkIndex,
		chunk.Content,
		embeddingData,
		chunk.TokenCount,
		chunk.CreatedAt.Unix(),
	)

	return err
}

// GetChunks retrieves all chunks for a document
func (d *DocumentStorage) GetChunks(documentID string) ([]*models.DocumentChunk, error) {
	query := `
		SELECT id, document_id, chunk_index, content, embedding, token_count, created_at
		FROM document_chunks
		WHERE document_id = ?
		ORDER BY chunk_index
	`

	rows, err := d.db.db.Query(query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]*models.DocumentChunk, 0)
	for rows.Next() {
		chunk, err := d.scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// DeleteChunks deletes all chunks for a document
func (d *DocumentStorage) DeleteChunks(documentID string) error {
	_, err := d.db.db.Exec("DELETE FROM document_chunks WHERE document_id = ?", documentID)
	return err
}

// ClearAll deletes all documents
func (d *DocumentStorage) ClearAll() error {
	_, err := d.db.db.Exec("DELETE FROM documents")
	return err
}

// Helper functions

func (d *DocumentStorage) scanDocument(row *sql.Row) (*models.Document, error) {
	var doc models.Document
	var embeddingData []byte
	var metadataJSON string
	var createdAt, updatedAt int64
	var contentMarkdown, embeddingModel, url sql.NullString

	err := row.Scan(
		&doc.ID,
		&doc.SourceType,
		&doc.SourceID,
		&doc.Title,
		&doc.Content,
		&contentMarkdown,
		&embeddingData,
		&embeddingModel,
		&metadataJSON,
		&url,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse optional fields
	if contentMarkdown.Valid {
		doc.ContentMarkdown = contentMarkdown.String
	}
	if embeddingModel.Valid {
		doc.EmbeddingModel = embeddingModel.String
	}
	if url.Valid {
		doc.URL = url.String
	}

	// Deserialize embedding
	if len(embeddingData) > 0 {
		doc.Embedding, err = deserializeEmbedding(embeddingData)
		if err != nil {
			d.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to deserialize embedding")
		}
	}

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
		var embeddingData []byte
		var metadataJSON string
		var createdAt, updatedAt int64
		var contentMarkdown, embeddingModel, url sql.NullString

		err := rows.Scan(
			&doc.ID,
			&doc.SourceType,
			&doc.SourceID,
			&doc.Title,
			&doc.Content,
			&contentMarkdown,
			&embeddingData,
			&embeddingModel,
			&metadataJSON,
			&url,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if contentMarkdown.Valid {
			doc.ContentMarkdown = contentMarkdown.String
		}
		if embeddingModel.Valid {
			doc.EmbeddingModel = embeddingModel.String
		}
		if url.Valid {
			doc.URL = url.String
		}

		if len(embeddingData) > 0 {
			doc.Embedding, err = deserializeEmbedding(embeddingData)
			if err != nil {
				d.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to deserialize embedding")
			}
		}

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

func (d *DocumentStorage) scanChunk(rows *sql.Rows) (*models.DocumentChunk, error) {
	var chunk models.DocumentChunk
	var embeddingData []byte
	var createdAt int64

	err := rows.Scan(
		&chunk.ID,
		&chunk.DocumentID,
		&chunk.ChunkIndex,
		&chunk.Content,
		&embeddingData,
		&chunk.TokenCount,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if len(embeddingData) > 0 {
		chunk.Embedding, err = deserializeEmbedding(embeddingData)
		if err != nil {
			d.logger.Warn().Err(err).Str("chunk_id", chunk.ID).Msg("Failed to deserialize embedding")
		}
	}

	chunk.CreatedAt = time.Unix(createdAt, 0)

	return &chunk, nil
}

// Embedding serialization helpers
func serializeEmbedding(embedding []float32) ([]byte, error) {
	// Simple binary encoding: just write the float32 array as bytes
	data := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := uint32(0)
		// Convert float32 to uint32 bits
		*(*float32)(unsafe.Pointer(&bits)) = v
		// Write as little-endian
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}
	return data, nil
}

func deserializeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding data length: %d", len(data))
	}

	embedding := make([]float32, len(data)/4)
	for i := 0; i < len(embedding); i++ {
		bits := uint32(data[i*4]) |
			uint32(data[i*4+1])<<8 |
			uint32(data[i*4+2])<<16 |
			uint32(data[i*4+3])<<24
		embedding[i] = *(*float32)(unsafe.Pointer(&bits))
	}
	return embedding, nil
}
