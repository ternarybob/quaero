// -----------------------------------------------------------------------
// Last Modified: Wednesday, 8th October 2025 12:11:01 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// ConfluenceStorage implements the ConfluenceStorage interface for SQLite
type ConfluenceStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
}

// NewConfluenceStorage creates a new ConfluenceStorage instance
func NewConfluenceStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.ConfluenceStorage {
	return &ConfluenceStorage{
		db:     db,
		logger: logger,
	}
}

// StoreSpace stores a Confluence space
func (s *ConfluenceStorage) StoreSpace(ctx context.Context, space *models.ConfluenceSpace) error {
	dataJSON, err := json.Marshal(space)
	if err != nil {
		return fmt.Errorf("failed to marshal space data: %w", err)
	}

	now := time.Now().Unix()
	query := `
		INSERT INTO confluence_spaces (key, name, id, page_count, data, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			name = excluded.name,
			page_count = excluded.page_count,
			data = excluded.data,
			updated_at = excluded.updated_at
	`

	_, err = s.db.DB().ExecContext(ctx, query, space.Key, space.Name, space.ID, space.PageCount, string(dataJSON), now, now)
	if err != nil {
		return fmt.Errorf("failed to store space: %w", err)
	}

	return nil
}

// GetSpace retrieves a Confluence space by key
func (s *ConfluenceStorage) GetSpace(ctx context.Context, key string) (*models.ConfluenceSpace, error) {
	query := "SELECT key, name, id, page_count, data FROM confluence_spaces WHERE key = ?"
	row := s.db.DB().QueryRowContext(ctx, query, key)

	var space models.ConfluenceSpace
	var dataJSON string

	err := row.Scan(&space.Key, &space.Name, &space.ID, &space.PageCount, &dataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get space: %w", err)
	}

	if err := json.Unmarshal([]byte(dataJSON), &space); err != nil {
		return nil, fmt.Errorf("failed to unmarshal space data: %w", err)
	}

	return &space, nil
}

// GetAllSpaces retrieves all Confluence spaces
func (s *ConfluenceStorage) GetAllSpaces(ctx context.Context) ([]*models.ConfluenceSpace, error) {
	query := "SELECT key, name, id, page_count, data FROM confluence_spaces ORDER BY key"
	rows, err := s.db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query spaces: %w", err)
	}
	defer rows.Close()

	var spaces []*models.ConfluenceSpace
	for rows.Next() {
		var space models.ConfluenceSpace
		var dataJSON string

		if err := rows.Scan(&space.Key, &space.Name, &space.ID, &space.PageCount, &dataJSON); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to scan space row")
			continue
		}

		if err := json.Unmarshal([]byte(dataJSON), &space); err != nil {
			s.logger.Warn().Err(err).Str("key", space.Key).Msg("Failed to unmarshal space data")
			continue
		}

		spaces = append(spaces, &space)
	}

	s.logger.Info().Int("count", len(spaces)).Msg("Retrieved Confluence spaces from database")
	return spaces, nil
}

// DeleteSpace deletes a Confluence space
func (s *ConfluenceStorage) DeleteSpace(ctx context.Context, key string) error {
	query := "DELETE FROM confluence_spaces WHERE key = ?"
	_, err := s.db.DB().ExecContext(ctx, query, key)
	return err
}

// CountSpaces returns the number of spaces
func (s *ConfluenceStorage) CountSpaces(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM confluence_spaces"
	err := s.db.DB().QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// GetMostRecentSpace returns the most recently updated space with its timestamp
func (s *ConfluenceStorage) GetMostRecentSpace(ctx context.Context) (*models.ConfluenceSpace, int64, error) {
	query := "SELECT key, name, id, page_count, data, updated_at FROM confluence_spaces ORDER BY updated_at DESC LIMIT 1"
	row := s.db.DB().QueryRowContext(ctx, query)

	var space models.ConfluenceSpace
	var dataJSON string
	var updatedAt int64

	err := row.Scan(&space.Key, &space.Name, &space.ID, &space.PageCount, &dataJSON, &updatedAt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get most recent space: %w", err)
	}

	if err := json.Unmarshal([]byte(dataJSON), &space); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal space data: %w", err)
	}

	return &space, updatedAt, nil
}

// StorePage stores a Confluence page
func (s *ConfluenceStorage) StorePage(ctx context.Context, page *models.ConfluencePage) error {
	bodyJSON, err := json.Marshal(page.Body)
	if err != nil {
		return fmt.Errorf("failed to marshal page body: %w", err)
	}

	now := time.Now().Unix()
	query := `
		INSERT INTO confluence_pages (id, space_id, title, content, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			body = excluded.body,
			updated_at = excluded.updated_at
	`

	_, err = s.db.DB().ExecContext(ctx, query, page.ID, page.SpaceID, page.Title, "", string(bodyJSON), now, now)
	return err
}

// StorePages stores multiple Confluence pages
func (s *ConfluenceStorage) StorePages(ctx context.Context, pages []*models.ConfluencePage) error {
	if len(pages) == 0 {
		return nil
	}

	tx, err := s.db.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	query := `
		INSERT INTO confluence_pages (id, space_id, title, content, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			content = excluded.content,
			body = excluded.body,
			updated_at = excluded.updated_at
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	storedCount := 0
	for _, page := range pages {
		bodyJSON, err := json.Marshal(page.Body)
		if err != nil {
			s.logger.Warn().Str("id", page.ID).Err(err).Msg("Failed to marshal page body")
			continue
		}

		_, err = stmt.ExecContext(ctx, page.ID, page.SpaceID, page.Title, "", string(bodyJSON), now, now)
		if err != nil {
			s.logger.Error().Str("id", page.ID).Err(err).Msg("Failed to execute insert for page")
			return fmt.Errorf("failed to store page %s: %w", page.ID, err)
		}

		storedCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info().Int("stored", storedCount).Int("total", len(pages)).Msg("Stored Confluence pages batch")
	return nil
}

// GetPage retrieves a Confluence page by ID
func (s *ConfluenceStorage) GetPage(ctx context.Context, id string) (*models.ConfluencePage, error) {
	query := "SELECT id, space_id, title, body FROM confluence_pages WHERE id = ?"
	row := s.db.DB().QueryRowContext(ctx, query, id)

	var page models.ConfluencePage
	var bodyJSON string

	err := row.Scan(&page.ID, &page.SpaceID, &page.Title, &bodyJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	if err := json.Unmarshal([]byte(bodyJSON), &page.Body); err != nil {
		return nil, fmt.Errorf("failed to unmarshal page body: %w", err)
	}

	return &page, nil
}

// GetPagesBySpace retrieves all pages for a space
func (s *ConfluenceStorage) GetPagesBySpace(ctx context.Context, spaceID string) ([]*models.ConfluencePage, error) {
	query := "SELECT id, space_id, title, body FROM confluence_pages WHERE space_id = ? ORDER BY title"
	rows, err := s.db.DB().QueryContext(ctx, query, spaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pages: %w", err)
	}
	defer rows.Close()

	var pages []*models.ConfluencePage
	for rows.Next() {
		var page models.ConfluencePage
		var bodyJSON string

		if err := rows.Scan(&page.ID, &page.SpaceID, &page.Title, &bodyJSON); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to scan page row")
			continue
		}

		if err := json.Unmarshal([]byte(bodyJSON), &page.Body); err != nil {
			s.logger.Warn().Err(err).Str("id", page.ID).Msg("Failed to unmarshal page body")
			continue
		}

		pages = append(pages, &page)
	}

	return pages, nil
}

// DeletePage deletes a Confluence page
func (s *ConfluenceStorage) DeletePage(ctx context.Context, id string) error {
	query := "DELETE FROM confluence_pages WHERE id = ?"
	_, err := s.db.DB().ExecContext(ctx, query, id)
	return err
}

// DeletePagesBySpace deletes all pages for a space
func (s *ConfluenceStorage) DeletePagesBySpace(ctx context.Context, spaceID string) error {
	query := "DELETE FROM confluence_pages WHERE space_id = ?"
	result, err := s.db.DB().ExecContext(ctx, query, spaceID)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	s.logger.Info().Str("space", spaceID).Int64("deleted", rows).Msg("Deleted pages for space")
	return nil
}

// CountPages returns the total number of pages
func (s *ConfluenceStorage) CountPages(ctx context.Context) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM confluence_pages"
	err := s.db.DB().QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// GetMostRecentPage returns the most recently updated page with its timestamp
func (s *ConfluenceStorage) GetMostRecentPage(ctx context.Context) (*models.ConfluencePage, int64, error) {
	query := "SELECT id, space_id, title, body, updated_at FROM confluence_pages ORDER BY updated_at DESC LIMIT 1"
	row := s.db.DB().QueryRowContext(ctx, query)

	var page models.ConfluencePage
	var bodyJSON string
	var updatedAt int64

	err := row.Scan(&page.ID, &page.SpaceID, &page.Title, &bodyJSON, &updatedAt)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get most recent page: %w", err)
	}

	if err := json.Unmarshal([]byte(bodyJSON), &page.Body); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal page body: %w", err)
	}

	return &page, updatedAt, nil
}

// CountPagesBySpace returns the number of pages for a space
func (s *ConfluenceStorage) CountPagesBySpace(ctx context.Context, spaceID string) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM confluence_pages WHERE space_id = ?"
	err := s.db.DB().QueryRowContext(ctx, query, spaceID).Scan(&count)
	return count, err
}

// SearchPages performs full-text search on pages
func (s *ConfluenceStorage) SearchPages(ctx context.Context, query string) ([]*models.ConfluencePage, error) {
	// TODO: Implement FTS5 search
	return nil, nil
}

// ClearAll deletes all Confluence data
func (s *ConfluenceStorage) ClearAll(ctx context.Context) error {
	// Delete pages
	if _, err := s.db.DB().ExecContext(ctx, "DELETE FROM confluence_pages"); err != nil {
		return fmt.Errorf("failed to clear pages: %w", err)
	}

	// Delete spaces
	if _, err := s.db.DB().ExecContext(ctx, "DELETE FROM confluence_spaces"); err != nil {
		return fmt.Errorf("failed to clear spaces: %w", err)
	}

	s.logger.Info().Msg("Cleared all Confluence data from database")
	return nil
}
