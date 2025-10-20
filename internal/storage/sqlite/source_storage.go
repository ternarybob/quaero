package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// SourceStorage implements interfaces.SourceStorage for SQLite
type SourceStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
}

// NewSourceStorage creates a new SourceStorage instance
func NewSourceStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.SourceStorage {
	return &SourceStorage{
		db:     db,
		logger: logger,
	}
}

// SaveSource creates or updates a source configuration
func (s *SourceStorage) SaveSource(ctx context.Context, source *models.SourceConfig) error {
	// Serialize CrawlConfig, Filters, and SeedURLs as JSON
	crawlConfigJSON, err := json.Marshal(source.CrawlConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal crawl config: %w", err)
	}

	filtersJSON, err := json.Marshal(source.Filters)
	if err != nil {
		return fmt.Errorf("failed to marshal filters: %w", err)
	}

	seedURLsJSON, err := json.Marshal(source.SeedURLs)
	if err != nil {
		return fmt.Errorf("failed to marshal seed URLs: %w", err)
	}

	// Convert bool to int for SQLite
	enabled := 0
	if source.Enabled {
		enabled = 1
	}

	query := `
		INSERT INTO sources (id, name, type, base_url, seed_urls, enabled, auth_id, auth_domain, crawl_config, filters, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			base_url = excluded.base_url,
			seed_urls = excluded.seed_urls,
			enabled = excluded.enabled,
			auth_id = excluded.auth_id,
			auth_domain = excluded.auth_domain,
			crawl_config = excluded.crawl_config,
			filters = excluded.filters,
			updated_at = excluded.updated_at
	`

	// Handle nullable auth_id
	var authID sql.NullString
	if source.AuthID != "" {
		authID = sql.NullString{String: source.AuthID, Valid: true}
	}

	_, err = s.db.DB().Exec(
		query,
		source.ID,
		source.Name,
		source.Type,
		source.BaseURL,
		string(seedURLsJSON),
		enabled,
		authID,
		source.AuthDomain,
		string(crawlConfigJSON),
		string(filtersJSON),
		source.CreatedAt.Unix(),
		source.UpdatedAt.Unix(),
	)

	if err != nil {
		return fmt.Errorf("failed to save source: %w", err)
	}

	s.logger.Info().
		Str("id", source.ID).
		Str("name", source.Name).
		Str("type", source.Type).
		Msg("Source saved successfully")

	return nil
}

// GetSource retrieves a source by ID
func (s *SourceStorage) GetSource(ctx context.Context, id string) (*models.SourceConfig, error) {
	query := `
		SELECT id, name, type, base_url, seed_urls, enabled, auth_id, auth_domain, crawl_config, filters, created_at, updated_at
		FROM sources
		WHERE id = ?
	`

	row := s.db.DB().QueryRow(query, id)
	source, err := s.scanSource(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("source not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// ListSources retrieves all sources ordered by created_at DESC
func (s *SourceStorage) ListSources(ctx context.Context) ([]*models.SourceConfig, error) {
	query := `
		SELECT id, name, type, base_url, seed_urls, enabled, auth_id, auth_domain, crawl_config, filters, created_at, updated_at
		FROM sources
		ORDER BY created_at DESC
	`

	rows, err := s.db.DB().Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	defer rows.Close()

	return s.scanSources(rows)
}

// DeleteSource deletes a source by ID
func (s *SourceStorage) DeleteSource(ctx context.Context, id string) error {
	query := `DELETE FROM sources WHERE id = ?`

	result, err := s.db.DB().Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("source not found: %s", id)
	}

	s.logger.Info().Str("id", id).Msg("Source deleted successfully")
	return nil
}

// GetSourcesByType retrieves sources filtered by type
func (s *SourceStorage) GetSourcesByType(ctx context.Context, sourceType string) ([]*models.SourceConfig, error) {
	query := `
		SELECT id, name, type, base_url, seed_urls, enabled, auth_id, auth_domain, crawl_config, filters, created_at, updated_at
		FROM sources
		WHERE type = ?
		ORDER BY created_at DESC
	`

	rows, err := s.db.DB().Query(query, sourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get sources by type: %w", err)
	}
	defer rows.Close()

	return s.scanSources(rows)
}

// GetEnabledSources retrieves only enabled sources
func (s *SourceStorage) GetEnabledSources(ctx context.Context) ([]*models.SourceConfig, error) {
	query := `
		SELECT id, name, type, base_url, seed_urls, enabled, auth_id, auth_domain, crawl_config, filters, created_at, updated_at
		FROM sources
		WHERE enabled = 1
		ORDER BY created_at DESC
	`

	rows, err := s.db.DB().Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled sources: %w", err)
	}
	defer rows.Close()

	return s.scanSources(rows)
}

// scanSource scans a single source from a row
func (s *SourceStorage) scanSource(row *sql.Row) (*models.SourceConfig, error) {
	var source models.SourceConfig
	var enabled int
	var crawlConfigJSON, filtersJSON, seedURLsJSON string
	var createdAt, updatedAt int64
	var authID sql.NullString

	err := row.Scan(
		&source.ID,
		&source.Name,
		&source.Type,
		&source.BaseURL,
		&seedURLsJSON,
		&enabled,
		&authID,
		&source.AuthDomain,
		&crawlConfigJSON,
		&filtersJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Convert nullable auth_id
	if authID.Valid {
		source.AuthID = authID.String
	}

	// Convert int to bool
	source.Enabled = enabled == 1

	// Deserialize JSON fields
	if err := json.Unmarshal([]byte(crawlConfigJSON), &source.CrawlConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal crawl config: %w", err)
	}

	if err := json.Unmarshal([]byte(filtersJSON), &source.Filters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal filters: %w", err)
	}

	if err := json.Unmarshal([]byte(seedURLsJSON), &source.SeedURLs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seed URLs: %w", err)
	}

	// Convert timestamps
	source.CreatedAt = timeFromUnix(createdAt)
	source.UpdatedAt = timeFromUnix(updatedAt)

	return &source, nil
}

// scanSources scans multiple sources from rows
func (s *SourceStorage) scanSources(rows *sql.Rows) ([]*models.SourceConfig, error) {
	var sources []*models.SourceConfig

	for rows.Next() {
		var source models.SourceConfig
		var enabled int
		var crawlConfigJSON, filtersJSON, seedURLsJSON string
		var createdAt, updatedAt int64
		var authID sql.NullString

		err := rows.Scan(
			&source.ID,
			&source.Name,
			&source.Type,
			&source.BaseURL,
			&seedURLsJSON,
			&enabled,
			&authID,
			&source.AuthDomain,
			&crawlConfigJSON,
			&filtersJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}

		// Convert nullable auth_id
		if authID.Valid {
			source.AuthID = authID.String
		}

		// Convert int to bool
		source.Enabled = enabled == 1

		// Deserialize JSON fields
		if err := json.Unmarshal([]byte(crawlConfigJSON), &source.CrawlConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal crawl config: %w", err)
		}

		if err := json.Unmarshal([]byte(filtersJSON), &source.Filters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal filters: %w", err)
		}

		if err := json.Unmarshal([]byte(seedURLsJSON), &source.SeedURLs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal seed URLs: %w", err)
		}

		// Convert timestamps
		source.CreatedAt = timeFromUnix(createdAt)
		source.UpdatedAt = timeFromUnix(updatedAt)

		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}

// timeFromUnix converts Unix timestamp to time.Time
func timeFromUnix(timestamp int64) time.Time {
	return time.Unix(timestamp, 0)
}
