// -----------------------------------------------------------------------
// Last Modified: Thursday, 14th November 2025 12:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package kv

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service provides business logic for key/value operations
type Service struct {
	storage interfaces.KeyValueStorage
	logger  arbor.ILogger
}

// NewService creates a new key/value service
func NewService(storage interfaces.KeyValueStorage, logger arbor.ILogger) *Service {
	return &Service{
		storage: storage,
		logger:  logger,
	}
}

// Get retrieves a value by key
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	value, err := s.storage.Get(ctx, key)
	if err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to get key/value pair")
		return "", err
	}

	s.logger.Debug().Str("key", key).Msg("Retrieved key/value pair")
	return value, nil
}

// GetPair retrieves a full KeyValuePair by key
func (s *Service) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	pair, err := s.storage.GetPair(ctx, key)
	if err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to get key/value pair")
		return nil, err
	}

	s.logger.Debug().Str("key", key).Msg("Retrieved key/value pair with metadata")
	return pair, nil
}

// Set stores or updates a key/value pair
func (s *Service) Set(ctx context.Context, key string, value string, description string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	err := s.storage.Set(ctx, key, value, description)
	if err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to store key/value pair")
		return err
	}

	s.logger.Info().Str("key", key).Msg("Stored key/value pair")
	return nil
}

// Delete removes a key/value pair
func (s *Service) Delete(ctx context.Context, key string) error {
	err := s.storage.Delete(ctx, key)
	if err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to delete key/value pair")
		return err
	}

	s.logger.Info().Str("key", key).Msg("Deleted key/value pair")
	return nil
}

// List returns all key/value pairs
func (s *Service) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	pairs, err := s.storage.List(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to list key/value pairs")
		return nil, err
	}

	s.logger.Debug().Int("count", len(pairs)).Msg("Listed key/value pairs")
	return pairs, nil
}

// GetAll returns all key/value pairs as a map
func (s *Service) GetAll(ctx context.Context) (map[string]string, error) {
	kvMap, err := s.storage.GetAll(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to retrieve all key/value pairs")
		return nil, err
	}

	s.logger.Debug().Int("count", len(kvMap)).Msg("Retrieved all key/value pairs")
	return kvMap, nil
}
