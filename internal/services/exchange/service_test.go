package exchange

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/eodhd"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// createTestLogger creates a logger for testing
func createTestLogger() arbor.ILogger {
	return arbor.NewLogger()
}

// mockKVStorage implements interfaces.KeyValueStorage for testing
type mockKVStorage struct {
	data map[string]string
}

func newMockKVStorage() *mockKVStorage {
	return &mockKVStorage{data: make(map[string]string)}
}

func (m *mockKVStorage) Get(ctx context.Context, key string) (string, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return "", interfaces.ErrKeyNotFound
}

func (m *mockKVStorage) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	if v, ok := m.data[key]; ok {
		return &interfaces.KeyValuePair{Key: key, Value: v}, nil
	}
	return nil, interfaces.ErrKeyNotFound
}

func (m *mockKVStorage) Set(ctx context.Context, key, value, description string) error {
	m.data[key] = value
	return nil
}

func (m *mockKVStorage) Upsert(ctx context.Context, key, value, description string) (bool, error) {
	_, exists := m.data[key]
	m.data[key] = value
	return !exists, nil
}

func (m *mockKVStorage) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockKVStorage) DeleteAll(ctx context.Context) error {
	m.data = make(map[string]string)
	return nil
}

func (m *mockKVStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	var pairs []interfaces.KeyValuePair
	for k, v := range m.data {
		pairs = append(pairs, interfaces.KeyValuePair{Key: k, Value: v})
	}
	return pairs, nil
}

func (m *mockKVStorage) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

func (m *mockKVStorage) ListByPrefix(ctx context.Context, prefix string) ([]interfaces.KeyValuePair, error) {
	var pairs []interfaces.KeyValuePair
	for k, v := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			pairs = append(pairs, interfaces.KeyValuePair{Key: k, Value: v})
		}
	}
	return pairs, nil
}

func TestService_GetMetadata_CacheHit(t *testing.T) {
	// Setup
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger) // nil EODHD client - should use cache

	// Pre-populate cache with fresh metadata
	metadata := &eodhd.ExchangeMetadata{
		Code:             "AU",
		Name:             "ASX",
		Timezone:         "Australia/Sydney",
		CloseTime:        "16:00",
		DataDelayMinutes: 180,
		WorkingDays:      eodhd.DefaultWorkingDays(),
		LastFetched:      time.Now().UTC(), // Fresh
	}
	data, _ := json.Marshal(metadata)
	kv.Set(context.Background(), KeyPrefix+"AU", string(data), "test")

	// Test
	result, err := svc.GetMetadata(context.Background(), "AU")

	// Verify
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if result.Code != "AU" {
		t.Errorf("Code = %s, want AU", result.Code)
	}
	if result.Timezone != "Australia/Sydney" {
		t.Errorf("Timezone = %s, want Australia/Sydney", result.Timezone)
	}
}

func TestService_GetMetadata_CacheMiss_FallbackToDefaults(t *testing.T) {
	// Setup - no cache, no EODHD client
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger) // nil EODHD client

	// Test - should fall back to defaults
	result, err := svc.GetMetadata(context.Background(), "AU")

	// Verify - should return default metadata
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if result.Code != "AU" {
		t.Errorf("Code = %s, want AU", result.Code)
	}
	if result.Timezone != "Australia/Sydney" {
		t.Errorf("Timezone = %s, want Australia/Sydney", result.Timezone)
	}
	if result.DataDelayMinutes != 180 {
		t.Errorf("DataDelayMinutes = %d, want 180", result.DataDelayMinutes)
	}
}

func TestService_GetMetadata_CacheStale_FallbackToDefaults(t *testing.T) {
	// Setup
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger).WithCacheTTL(1 * time.Hour) // nil EODHD client

	// Pre-populate cache with stale metadata
	metadata := &eodhd.ExchangeMetadata{
		Code:        "AU",
		LastFetched: time.Now().UTC().Add(-2 * time.Hour), // Older than TTL
	}
	data, _ := json.Marshal(metadata)
	kv.Set(context.Background(), KeyPrefix+"AU", string(data), "test")

	// Test - should detect stale cache and fall back to defaults
	result, err := svc.GetMetadata(context.Background(), "AU")

	// Verify - should return default metadata (since EODHD client is nil)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if result.Code != "AU" {
		t.Errorf("Code = %s, want AU", result.Code)
	}
}

func TestService_IsTickerStale_ValidTicker(t *testing.T) {
	// Setup
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger)

	// Pre-populate cache with fresh metadata
	metadata := &eodhd.ExchangeMetadata{
		Code:             "AU",
		Timezone:         "Australia/Sydney",
		CloseTime:        "16:00",
		DataDelayMinutes: 180,
		WorkingDays:      eodhd.DefaultWorkingDays(),
		LastFetched:      time.Now().UTC(),
	}
	data, _ := json.Marshal(metadata)
	kv.Set(context.Background(), KeyPrefix+"AU", string(data), "test")

	// Test - check if data from today is stale
	today := time.Now().UTC().Truncate(24 * time.Hour)
	result, err := svc.IsTickerStale(context.Background(), "CBA.AU", today)

	// Verify
	if err != nil {
		t.Fatalf("IsTickerStale() error = %v", err)
	}
	if result == nil {
		t.Fatal("IsTickerStale() result is nil")
	}
	// Today's data should generally not be stale (unless it's very late)
	// The exact result depends on current time, but we should get a valid result
	if result.Reason == "" {
		t.Error("IsTickerStale() should have a reason")
	}
}

func TestService_IsTickerStale_InvalidTicker(t *testing.T) {
	// Setup
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger)

	// Test with invalid ticker format
	result, err := svc.IsTickerStale(context.Background(), "INVALID", time.Now())

	// Verify - should return stale=true with error reason
	if err != nil {
		t.Fatalf("IsTickerStale() error = %v", err)
	}
	if !result.IsStale {
		t.Error("IsTickerStale() should return stale for invalid ticker")
	}
	if result.Reason == "" {
		t.Error("IsTickerStale() should have error reason for invalid ticker")
	}
}

func TestService_ListCachedExchanges(t *testing.T) {
	// Setup
	kv := newMockKVStorage()
	logger := createTestLogger()
	svc := NewService(nil, kv, logger)

	// Pre-populate cache
	for _, code := range []string{"AU", "US", "LSE"} {
		metadata := &eodhd.ExchangeMetadata{Code: code, LastFetched: time.Now().UTC()}
		data, _ := json.Marshal(metadata)
		kv.Set(context.Background(), KeyPrefix+code, string(data), "test")
	}

	// Test
	exchanges, err := svc.ListCachedExchanges(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("ListCachedExchanges() error = %v", err)
	}
	if len(exchanges) != 3 {
		t.Errorf("ListCachedExchanges() returned %d exchanges, want 3", len(exchanges))
	}
}

func TestService_isCacheFresh(t *testing.T) {
	svc := &Service{cacheTTL: 24 * time.Hour}

	tests := []struct {
		name        string
		lastFetched time.Time
		want        bool
	}{
		{"nil metadata", time.Time{}, false},
		{"fresh (1 hour ago)", time.Now().UTC().Add(-1 * time.Hour), true},
		{"stale (25 hours ago)", time.Now().UTC().Add(-25 * time.Hour), false},
		{"exactly at TTL", time.Now().UTC().Add(-24 * time.Hour), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var meta *eodhd.ExchangeMetadata
			if !tt.lastFetched.IsZero() {
				meta = &eodhd.ExchangeMetadata{LastFetched: tt.lastFetched}
			}

			got := svc.isCacheFresh(meta)
			if got != tt.want {
				t.Errorf("isCacheFresh() = %v, want %v", got, tt.want)
			}
		})
	}
}
