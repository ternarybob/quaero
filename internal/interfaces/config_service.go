package interfaces

import "context"

// ConfigService manages configuration with dynamic key injection and caching
// Note: GetConfig returns interface{} to avoid import cycle with common.Config
// Implementations should return *common.Config, but callers must type assert
type ConfigService interface {
	// GetConfig returns a config copy with dynamically injected keys from KV storage
	// Returns a deep clone to prevent mutations affecting the original config
	// Returns *common.Config (type assert required)
	GetConfig(ctx context.Context) (interface{}, error)

	// InvalidateCache invalidates the cached config, forcing a rebuild on next GetConfig()
	InvalidateCache()

	// Close unsubscribes from events and cleans up resources
	Close() error
}
