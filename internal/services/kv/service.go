// -----------------------------------------------------------------------
// Last Modified: Thursday, 14th November 2025 12:00:00 pm
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package kv

import (
	"context"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service provides business logic for key/value operations
type Service struct {
	storage   interfaces.KeyValueStorage
	eventSvc  interfaces.EventService
	logger    arbor.ILogger
}

// NewService creates a new key/value service
// If eventSvc is nil, event publishing is skipped (graceful degradation)
func NewService(storage interfaces.KeyValueStorage, eventSvc interfaces.EventService, logger arbor.ILogger) *Service {
	return &Service{
		storage:  storage,
		eventSvc: eventSvc,
		logger:   logger,
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

// Set stores or updates a key/value pair and publishes EventKeyUpdated
func (s *Service) Set(ctx context.Context, key string, value string, description string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Get old value for event payload (if exists)
	oldValue, _ := s.storage.Get(ctx, key)

	// Store the key/value pair
	err := s.storage.Set(ctx, key, value, description)
	if err != nil {
		s.logger.Error().Err(err).Str("key", key).Msg("Failed to store key/value pair")
		return err
	}

	s.logger.Info().Str("key", key).Msg("Stored key/value pair")

	// Publish EventKeyUpdated if event service is available
	if s.eventSvc != nil {
		event := interfaces.Event{
			Type: interfaces.EventKeyUpdated,
			Payload: map[string]interface{}{
				"key_name":  key,
				"old_value": oldValue,
				"new_value": value,
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}

		// Publish asynchronously to avoid blocking the Set operation
		if err := s.eventSvc.Publish(ctx, event); err != nil {
			s.logger.Warn().Err(err).Str("key", key).Msg("Failed to publish EventKeyUpdated")
			// Don't fail the Set operation if event publishing fails
		} else {
			s.logger.Debug().Str("key", key).Msg("Published EventKeyUpdated")
		}
	}

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
