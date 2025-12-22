package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Service manages configuration with dynamic key injection and caching
type Service struct {
	config       *common.Config
	kvStorage    interfaces.KeyValueStorage
	eventSvc     interfaces.EventService
	logger       arbor.ILogger
	mu           sync.RWMutex
	cachedConfig *common.Config
	cacheValid   bool
	configPaths  []string // Paths to config files for reload functionality
}

// NewService creates a new config service with event-driven cache invalidation
// If kvStorage is nil, key injection is skipped (backward compatibility)
// configPaths are stored for reload functionality (optional)
func NewService(
	config *common.Config,
	kvStorage interfaces.KeyValueStorage,
	eventSvc interfaces.EventService,
	logger arbor.ILogger,
	configPaths ...string,
) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	service := &Service{
		config:      config,
		kvStorage:   kvStorage,
		eventSvc:    eventSvc,
		logger:      logger,
		cacheValid:  false,
		configPaths: configPaths,
	}

	// Subscribe to key update events if event service is available
	if eventSvc != nil {
		if err := eventSvc.Subscribe(interfaces.EventKeyUpdated, service.handleKeyUpdate); err != nil {
			logger.Warn().Err(err).Msg("Failed to subscribe to key update events")
		} else {
			logger.Debug().Msg("ConfigService subscribed to key update events")
		}
	}

	return service, nil
}

// GetConfig returns a config copy with dynamically injected keys from KV storage
// Returns a deep clone to prevent mutations affecting the original config
// Returns interface{} to satisfy the ConfigService interface (actual type is *common.Config)
func (s *Service) GetConfig(ctx context.Context) (interface{}, error) {
	s.mu.RLock()
	// Check if we have a valid cache
	if s.cacheValid && s.cachedConfig != nil {
		config := s.cachedConfig
		s.mu.RUnlock()
		s.logger.Debug().Msg("Returning cached config with injected keys")
		return config, nil
	}
	s.mu.RUnlock()

	// Cache miss - rebuild with key injection
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have updated cache)
	if s.cacheValid && s.cachedConfig != nil {
		s.logger.Debug().Msg("Returning cached config after lock acquisition")
		return s.cachedConfig, nil
	}

	s.logger.Debug().Msg("Cache invalid, rebuilding config with key injection")

	// Deep clone the original config to avoid mutations
	configCopy := common.DeepCloneConfig(s.config)

	// Inject keys if KV storage is available
	if s.kvStorage != nil {
		kvMap, err := s.kvStorage.GetAll(ctx)
		if err != nil {
			s.logger.Warn().Err(err).Msg("Failed to fetch KV map for config injection")
			// Continue with uninjected config (graceful degradation)
		} else if len(kvMap) > 0 {
			if err := common.ReplaceInStruct(configCopy, kvMap, s.logger); err != nil {
				s.logger.Warn().Err(err).Msg("Failed to inject keys into config")
			} else {
				s.logger.Debug().Int("keys", len(kvMap)).Msg("Injected keys into config")
			}
		}
	}

	// Update cache
	s.cachedConfig = configCopy
	s.cacheValid = true

	return configCopy, nil
}

// InvalidateCache invalidates the cached config, forcing a rebuild on next GetConfig()
func (s *Service) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cacheValid = false
	s.cachedConfig = nil
	s.logger.Debug().Msg("Config cache invalidated")
}

// handleKeyUpdate is the event handler for EventKeyUpdated
// Invalidates cache when keys change
func (s *Service) handleKeyUpdate(ctx context.Context, event interfaces.Event) error {
	s.logger.Debug().
		Str("event_type", string(event.Type)).
		Msg("Key update event received, invalidating config cache")

	s.InvalidateCache()
	return nil
}

// Close unsubscribes from events and cleans up resources
func (s *Service) Close() error {
	if s.eventSvc != nil {
		if err := s.eventSvc.Unsubscribe(interfaces.EventKeyUpdated, s.handleKeyUpdate); err != nil {
			s.logger.Warn().Err(err).Msg("Failed to unsubscribe from key update events")
		}
	}
	s.InvalidateCache()
	s.logger.Debug().Msg("ConfigService closed")
	return nil
}

// ReloadConfig reloads configuration from files
// If clear is true, clears all KV store entries before reloading
// Uses the same code path as startup config loading
func (s *Service) ReloadConfig(ctx context.Context, clear bool) error {
	s.logger.Info().Bool("clear", clear).Strs("paths", s.configPaths).Msg("Reloading configuration")

	// Step 1: Clear KV store if requested
	if clear && s.kvStorage != nil {
		s.logger.Info().Msg("Clearing KV store before reload")
		if err := s.kvStorage.DeleteAll(ctx); err != nil {
			s.logger.Error().Err(err).Msg("Failed to clear KV store")
			return fmt.Errorf("failed to clear KV store: %w", err)
		}
		s.logger.Info().Msg("KV store cleared successfully")
	}

	// Step 2: Reload config from files using same code path as startup
	if len(s.configPaths) == 0 {
		s.logger.Warn().Msg("No config paths available for reload")
		return fmt.Errorf("no config paths available for reload")
	}

	newConfig, err := common.LoadFromFiles(s.kvStorage, s.configPaths...)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to reload config from files")
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Step 3: Update the stored config
	s.mu.Lock()
	s.config = newConfig
	s.cacheValid = false
	s.cachedConfig = nil
	s.mu.Unlock()

	s.logger.Info().Msg("Configuration reloaded successfully")
	return nil
}
