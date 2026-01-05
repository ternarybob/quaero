// Package exchange provides exchange metadata management with caching.
// It fetches exchange details from EODHD and caches them locally.
package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
)

const (
	// DefaultCacheTTL is the default time-to-live for cached exchange metadata.
	DefaultCacheTTL = 24 * time.Hour

	// KeyPrefix is the prefix for exchange metadata keys in KV storage.
	KeyPrefix = "exchange:metadata:"
)

// Service provides exchange metadata management with caching.
type Service struct {
	eodhd    *eodhd.Client
	kvSvc    interfaces.KeyValueStorage
	logger   arbor.ILogger
	cacheTTL time.Duration
}

// NewService creates a new exchange metadata service.
func NewService(eodhdClient *eodhd.Client, kvStorage interfaces.KeyValueStorage, logger arbor.ILogger) *Service {
	return &Service{
		eodhd:    eodhdClient,
		kvSvc:    kvStorage,
		logger:   logger,
		cacheTTL: DefaultCacheTTL,
	}
}

// WithCacheTTL sets a custom cache TTL.
func (s *Service) WithCacheTTL(ttl time.Duration) *Service {
	s.cacheTTL = ttl
	return s
}

// GetMetadata retrieves exchange metadata, using cache if available and fresh.
// Falls back to defaults if both cache and API fail.
func (s *Service) GetMetadata(ctx context.Context, exchangeCode string) (*eodhd.ExchangeMetadata, error) {
	// Try cache first
	cached, err := s.getFromCache(ctx, exchangeCode)
	if err == nil && cached != nil && s.isCacheFresh(cached) {
		s.logger.Debug().
			Str("exchange", exchangeCode).
			Str("last_fetched", cached.LastFetched.Format(time.RFC3339)).
			Msg("Using cached exchange metadata")
		return cached, nil
	}

	// Cache miss or stale - fetch from API
	metadata, err := s.fetchFromAPI(ctx, exchangeCode)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("exchange", exchangeCode).
			Msg("Failed to fetch exchange metadata from API, using defaults")

		// Return defaults as fallback
		return common.DefaultExchangeMetadata(exchangeCode), nil
	}

	// Store in cache
	if err := s.storeInCache(ctx, exchangeCode, metadata); err != nil {
		s.logger.Warn().
			Err(err).
			Str("exchange", exchangeCode).
			Msg("Failed to cache exchange metadata")
	}

	return metadata, nil
}

// IsTickerStale checks if cached data for a ticker is stale.
// It parses the ticker, fetches exchange metadata, and performs the staleness check.
func (s *Service) IsTickerStale(ctx context.Context, ticker string, docDate time.Time) (*common.StalenessResult, error) {
	// Parse ticker to get exchange code
	parsed := common.ParseEODHDTicker(ticker)
	if parsed.Code == "" {
		return &common.StalenessResult{
			IsStale: true,
			Reason:  fmt.Sprintf("invalid ticker format: %s", ticker),
		}, nil
	}

	exchangeCode := parsed.DetailsExchangeCode()

	// Get exchange metadata
	metadata, err := s.GetMetadata(ctx, exchangeCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange metadata: %w", err)
	}

	// Check staleness
	result := common.CheckTickerStaleness(docDate, time.Now().UTC(), metadata)
	return &result, nil
}

// RefreshMetadata forces a refresh of exchange metadata from the API.
func (s *Service) RefreshMetadata(ctx context.Context, exchangeCode string) (*eodhd.ExchangeMetadata, error) {
	metadata, err := s.fetchFromAPI(ctx, exchangeCode)
	if err != nil {
		return nil, err
	}

	if err := s.storeInCache(ctx, exchangeCode, metadata); err != nil {
		s.logger.Warn().
			Err(err).
			Str("exchange", exchangeCode).
			Msg("Failed to cache refreshed exchange metadata")
	}

	return metadata, nil
}

// getFromCache retrieves exchange metadata from KV cache.
func (s *Service) getFromCache(ctx context.Context, exchangeCode string) (*eodhd.ExchangeMetadata, error) {
	key := KeyPrefix + exchangeCode
	value, err := s.kvSvc.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var metadata eodhd.ExchangeMetadata
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached metadata: %w", err)
	}

	return &metadata, nil
}

// storeInCache stores exchange metadata in KV cache.
func (s *Service) storeInCache(ctx context.Context, exchangeCode string, metadata *eodhd.ExchangeMetadata) error {
	key := KeyPrefix + exchangeCode

	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	description := fmt.Sprintf("Exchange metadata for %s, fetched at %s",
		exchangeCode, metadata.LastFetched.Format(time.RFC3339))

	return s.kvSvc.Set(ctx, key, string(data), description)
}

// isCacheFresh checks if cached metadata is still within TTL.
func (s *Service) isCacheFresh(metadata *eodhd.ExchangeMetadata) bool {
	if metadata == nil {
		return false
	}
	return time.Since(metadata.LastFetched) < s.cacheTTL
}

// fetchFromAPI fetches exchange metadata from EODHD API.
func (s *Service) fetchFromAPI(ctx context.Context, exchangeCode string) (*eodhd.ExchangeMetadata, error) {
	if s.eodhd == nil {
		return nil, fmt.Errorf("EODHD client not configured")
	}

	metadata, err := s.eodhd.GetExchangeMetadata(ctx, exchangeCode)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange details: %w", err)
	}

	return metadata, nil
}

// ListCachedExchanges returns all exchange codes that have cached metadata.
func (s *Service) ListCachedExchanges(ctx context.Context) ([]string, error) {
	pairs, err := s.kvSvc.ListByPrefix(ctx, KeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list cached exchanges: %w", err)
	}

	exchanges := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		// Extract exchange code from key (format: "exchange:metadata:AU")
		if len(pair.Key) > len(KeyPrefix) {
			exchanges = append(exchanges, pair.Key[len(KeyPrefix):])
		}
	}

	return exchanges, nil
}
