package badger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/timshannon/badgerhold/v4"
)

// KVStorage implements the KeyValueStorage interface for Badger
type KVStorage struct {
	db     *BadgerDB
	logger arbor.ILogger
}

// NewKVStorage creates a new KVStorage instance
func NewKVStorage(db *BadgerDB, logger arbor.ILogger) interfaces.KeyValueStorage {
	return &KVStorage{
		db:     db,
		logger: logger,
	}
}

// normalizeKey converts a key to lowercase for case-insensitive storage
func (s *KVStorage) normalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

// Get retrieves a value by key (case-insensitive)
func (s *KVStorage) Get(ctx context.Context, key string) (string, error) {
	normalizedKey := s.normalizeKey(key)
	var pair interfaces.KeyValuePair
	err := s.db.Store().Get(normalizedKey, &pair)
	if err == badgerhold.ErrNotFound {
		return "", interfaces.ErrKeyNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return pair.Value, nil
}

// GetPair retrieves a full KeyValuePair by key (case-insensitive)
func (s *KVStorage) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	normalizedKey := s.normalizeKey(key)
	var pair interfaces.KeyValuePair
	err := s.db.Store().Get(normalizedKey, &pair)
	if err == badgerhold.ErrNotFound {
		return nil, interfaces.ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key/value pair: %w", err)
	}

	return &pair, nil
}

// Set inserts or updates a key/value pair (case-insensitive)
func (s *KVStorage) Set(ctx context.Context, key string, value string, description string) error {
	normalizedKey := s.normalizeKey(key)
	now := time.Now()

	pair := interfaces.KeyValuePair{
		Key:         normalizedKey,
		Value:       value,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Check if exists to preserve CreatedAt
	var existing interfaces.KeyValuePair
	err := s.db.Store().Get(normalizedKey, &existing)
	if err == nil {
		pair.CreatedAt = existing.CreatedAt
	}

	if err := s.db.Store().Upsert(normalizedKey, &pair); err != nil {
		return fmt.Errorf("failed to set key/value: %w", err)
	}

	return nil
}

// Upsert inserts or updates a key/value pair with explicit operation detection (case-insensitive)
func (s *KVStorage) Upsert(ctx context.Context, key string, value string, description string) (bool, error) {
	normalizedKey := s.normalizeKey(key)
	now := time.Now()

	pair := interfaces.KeyValuePair{
		Key:         normalizedKey,
		Value:       value,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	var existing interfaces.KeyValuePair
	err := s.db.Store().Get(normalizedKey, &existing)
	isNewKey := err == badgerhold.ErrNotFound

	if !isNewKey && err == nil {
		pair.CreatedAt = existing.CreatedAt
	} else if err != nil && err != badgerhold.ErrNotFound {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	if err := s.db.Store().Upsert(normalizedKey, &pair); err != nil {
		return false, fmt.Errorf("failed to upsert key/value: %w", err)
	}

	return isNewKey, nil
}

// Delete removes a key/value pair (case-insensitive)
func (s *KVStorage) Delete(ctx context.Context, key string) error {
	normalizedKey := s.normalizeKey(key)
	err := s.db.Store().Delete(normalizedKey, &interfaces.KeyValuePair{})
	if err == badgerhold.ErrNotFound {
		return interfaces.ErrKeyNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// DeleteAll removes all key/value pairs from storage
func (s *KVStorage) DeleteAll(ctx context.Context) error {
	// Find all pairs first
	var pairs []interfaces.KeyValuePair
	err := s.db.Store().Find(&pairs, nil)
	if err != nil {
		return fmt.Errorf("failed to list key/value pairs for deletion: %w", err)
	}

	// Delete each pair
	for _, pair := range pairs {
		if err := s.db.Store().Delete(pair.Key, &interfaces.KeyValuePair{}); err != nil {
			s.logger.Warn().Str("key", pair.Key).Err(err).Msg("Failed to delete key during DeleteAll")
		}
	}

	s.logger.Info().Int("count", len(pairs)).Msg("Deleted all key/value pairs")
	return nil
}

// List returns all key/value pairs ordered by updated_at DESC
func (s *KVStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	var pairs []interfaces.KeyValuePair
	err := s.db.Store().Find(&pairs, badgerhold.Where("Key").Ne("").SortBy("UpdatedAt").Reverse())
	if err != nil {
		return nil, fmt.Errorf("failed to list key/value pairs: %w", err)
	}
	return pairs, nil
}

// GetAll returns all key/value pairs as a map
func (s *KVStorage) GetAll(ctx context.Context) (map[string]string, error) {
	var pairs []interfaces.KeyValuePair
	err := s.db.Store().Find(&pairs, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get all key/value pairs: %w", err)
	}

	kvMap := make(map[string]string)
	for _, pair := range pairs {
		kvMap[pair.Key] = pair.Value
	}

	return kvMap, nil
}
