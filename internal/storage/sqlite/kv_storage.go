// -----------------------------------------------------------------------
// Last Modified: Thursday, 14th November 2025 12:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// KVStorage implements the KeyValueStorage interface for SQLite
type KVStorage struct {
	db     *SQLiteDB
	logger arbor.ILogger
	mu     sync.Mutex // Prevents SQLITE_BUSY errors on concurrent writes
}

// NewKVStorage creates a new KVStorage instance
func NewKVStorage(db *SQLiteDB, logger arbor.ILogger) interfaces.KeyValueStorage {
	return &KVStorage{
		db:     db,
		logger: logger,
	}
}

// Get retrieves a value by key
func (s *KVStorage) Get(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM key_value_store WHERE key = ?`

	err := s.db.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("key '%s' not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return value, nil
}

// Set inserts or updates a key/value pair
func (s *KVStorage) Set(ctx context.Context, key string, value string, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	query := `
		INSERT INTO key_value_store (key, value, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			description = excluded.description,
			updated_at = excluded.updated_at
	`

	_, err := s.db.db.ExecContext(ctx, query, key, value, description, now, now)
	if err != nil {
		return fmt.Errorf("failed to set key/value: %w", err)
	}

	return nil
}

// Delete removes a key/value pair
func (s *KVStorage) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM key_value_store WHERE key = ?`

	result, err := s.db.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("key '%s' not found", key)
	}

	return nil
}

// List returns all key/value pairs ordered by updated_at DESC
func (s *KVStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	query := `
		SELECT key, value, description, created_at, updated_at
		FROM key_value_store
		ORDER BY updated_at DESC
	`

	rows, err := s.db.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list key/value pairs: %w", err)
	}
	defer rows.Close()

	var pairs []interfaces.KeyValuePair
	for rows.Next() {
		var pair interfaces.KeyValuePair
		var createdAt, updatedAt int64

		err := rows.Scan(&pair.Key, &pair.Value, &pair.Description, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		pair.CreatedAt = time.Unix(createdAt, 0)
		pair.UpdatedAt = time.Unix(updatedAt, 0)
		pairs = append(pairs, pair)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Return empty slice instead of nil for consistency
	if pairs == nil {
		pairs = []interfaces.KeyValuePair{}
	}

	return pairs, nil
}

// GetAll returns all key/value pairs as a map
func (s *KVStorage) GetAll(ctx context.Context) (map[string]string, error) {
	query := `SELECT key, value FROM key_value_store`

	rows, err := s.db.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all key/value pairs: %w", err)
	}
	defer rows.Close()

	kvMap := make(map[string]string)
	for rows.Next() {
		var key, value string
		err := rows.Scan(&key, &value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		kvMap[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return kvMap, nil
}
