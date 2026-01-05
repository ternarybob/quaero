package config

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// mockKVStorage implements interfaces.KeyValueStorage for testing
type mockKVStorage struct {
	data map[string]string
	mu   sync.RWMutex
}

func newMockKVStorage() *mockKVStorage {
	return &mockKVStorage{
		data: make(map[string]string),
	}
}

func (m *mockKVStorage) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.data[key]
	if !ok {
		return "", interfaces.ErrKeyNotFound
	}
	return value, nil
}

func (m *mockKVStorage) GetPair(ctx context.Context, key string) (*interfaces.KeyValuePair, error) {
	value, err := m.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return &interfaces.KeyValuePair{
		Key:   key,
		Value: value,
	}, nil
}

func (m *mockKVStorage) Set(ctx context.Context, key, value, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockKVStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockKVStorage) List(ctx context.Context) ([]interfaces.KeyValuePair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pairs := make([]interfaces.KeyValuePair, 0, len(m.data))
	for k, v := range m.data {
		pairs = append(pairs, interfaces.KeyValuePair{
			Key:   k,
			Value: v,
		})
	}
	return pairs, nil
}

func (m *mockKVStorage) GetAll(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.data))
	for k, v := range m.data {
		result[k] = v
	}
	return result, nil
}

func (m *mockKVStorage) Upsert(ctx context.Context, key, value, description string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.data[key]
	m.data[key] = value
	return !exists, nil // Returns true if newly created, false if updated
}

func (m *mockKVStorage) DeleteAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]string)
	return nil
}

func (m *mockKVStorage) ListByPrefix(ctx context.Context, prefix string) ([]interfaces.KeyValuePair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pairs []interfaces.KeyValuePair
	for k, v := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			pairs = append(pairs, interfaces.KeyValuePair{
				Key:   k,
				Value: v,
			})
		}
	}
	return pairs, nil
}

// mockEventService implements interfaces.EventService for testing
type mockEventService struct {
	handlers map[interfaces.EventType][]interfaces.EventHandler
	mu       sync.RWMutex
}

func newMockEventService() *mockEventService {
	return &mockEventService{
		handlers: make(map[interfaces.EventType][]interfaces.EventHandler),
	}
}

func (m *mockEventService) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[eventType] = append(m.handlers[eventType], handler)
	return nil
}

func (m *mockEventService) Unsubscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Simple implementation - remove all handlers for this event type
	delete(m.handlers, eventType)
	return nil
}

func (m *mockEventService) Publish(ctx context.Context, event interfaces.Event) error {
	m.mu.RLock()
	handlers := m.handlers[event.Type]
	m.mu.RUnlock()

	for _, handler := range handlers {
		go handler(ctx, event)
	}
	return nil
}

func (m *mockEventService) PublishSync(ctx context.Context, event interfaces.Event) error {
	m.mu.RLock()
	handlers := m.handlers[event.Type]
	m.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockEventService) Close() error {
	return nil
}

// TestConfigService_Caching verifies cache hit/miss behavior
func TestConfigService_Caching(t *testing.T) {
	logger := arbor.NewLogger()
	defer common.Stop()

	// Create test config
	testConfig := &common.Config{
		Server: common.ServerConfig{
			Port: 8080,
			Host: "localhost",
		},
		PlacesAPI: common.PlacesAPIConfig{
			APIKey: "{test-key}",
		},
	}

	kvStorage := newMockKVStorage()
	kvStorage.Set(context.Background(), "test-key", "replaced-value", "test")

	eventSvc := newMockEventService()

	// Create ConfigService
	service, err := NewService(testConfig, kvStorage, eventSvc, logger)
	if err != nil {
		t.Fatalf("Failed to create ConfigService: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// First call - should be cache miss
	config1, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	// Verify key injection worked
	cfg1, ok := config1.(*common.Config)
	if !ok {
		t.Fatal("GetConfig returned wrong type")
	}
	if cfg1.PlacesAPI.APIKey != "replaced-value" {
		t.Errorf("Expected APIKey to be 'replaced-value', got '%s'", cfg1.PlacesAPI.APIKey)
	}

	// Second call - should be cache hit (same pointer)
	config2, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config on second call: %v", err)
	}

	cfg2, ok := config2.(*common.Config)
	if !ok {
		t.Fatal("GetConfig returned wrong type on second call")
	}

	// Verify we got the cached version
	if cfg1 != cfg2 {
		t.Error("Expected cached config to be returned (same pointer)")
	}

	t.Log("✓ Caching works correctly (cache hit after first call)")
}

// TestConfigService_EventInvalidation verifies EventKeyUpdated invalidates cache
func TestConfigService_EventInvalidation(t *testing.T) {
	logger := arbor.NewLogger()
	defer common.Stop()

	testConfig := &common.Config{
		Server: common.ServerConfig{
			Port: 8080,
		},
		PlacesAPI: common.PlacesAPIConfig{
			APIKey: "{test-key}",
		},
	}

	kvStorage := newMockKVStorage()
	kvStorage.Set(context.Background(), "test-key", "original-value", "test")

	eventSvc := newMockEventService()

	service, err := NewService(testConfig, kvStorage, eventSvc, logger)
	if err != nil {
		t.Fatalf("Failed to create ConfigService: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Get config (cache it)
	config1, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	cfg1 := config1.(*common.Config)
	if cfg1.PlacesAPI.APIKey != "original-value" {
		t.Errorf("Expected APIKey to be 'original-value', got '%s'", cfg1.PlacesAPI.APIKey)
	}

	// Update KV storage
	kvStorage.Set(ctx, "test-key", "new-value", "test")

	// Publish EventKeyUpdated
	event := interfaces.Event{
		Type: interfaces.EventKeyUpdated,
		Payload: map[string]interface{}{
			"key_name":  "test-key",
			"old_value": "original-value",
			"new_value": "new-value",
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}

	// Use PublishSync to ensure handler runs immediately
	if err := eventSvc.PublishSync(ctx, event); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Give the event handler time to process (it may be async)
	time.Sleep(50 * time.Millisecond)

	// Get config again - should rebuild cache with new value
	config2, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config after event: %v", err)
	}

	cfg2 := config2.(*common.Config)
	if cfg2.PlacesAPI.APIKey != "new-value" {
		t.Errorf("Expected APIKey to be 'new-value' after event, got '%s'", cfg2.PlacesAPI.APIKey)
	}

	// Verify it's a different instance (cache was rebuilt)
	if cfg1 == cfg2 {
		t.Error("Expected new config instance after cache invalidation")
	}

	t.Log("✓ Event-driven cache invalidation works correctly")
}

// TestConfigService_KeyInjection verifies {key-name} replacement works
func TestConfigService_KeyInjection(t *testing.T) {
	logger := arbor.NewLogger()
	defer common.Stop()

	testConfig := &common.Config{
		Server: common.ServerConfig{
			Port: 8080,
		},
		PlacesAPI: common.PlacesAPIConfig{
			APIKey: "{google-places-key}",
		},
	}

	kvStorage := newMockKVStorage()
	kvStorage.Set(context.Background(), "google-places-key", "AIzaSyTest123", "test")

	eventSvc := newMockEventService()

	service, err := NewService(testConfig, kvStorage, eventSvc, logger)
	if err != nil {
		t.Fatalf("Failed to create ConfigService: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	config, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	cfg := config.(*common.Config)

	// Verify key injection replaced placeholder
	if cfg.PlacesAPI.APIKey != "AIzaSyTest123" {
		t.Errorf("Expected APIKey to be 'AIzaSyTest123', got '%s'", cfg.PlacesAPI.APIKey)
	}

	// Verify original config was not mutated
	if testConfig.PlacesAPI.APIKey != "{google-places-key}" {
		t.Error("Original config should not be mutated")
	}

	t.Log("✓ Key injection works correctly without mutating original config")
}

// TestConfigService_NilKVStorage verifies graceful degradation when kvStorage is nil
func TestConfigService_NilKVStorage(t *testing.T) {
	logger := arbor.NewLogger()
	defer common.Stop()

	testConfig := &common.Config{
		Server: common.ServerConfig{
			Port: 8080,
		},
		PlacesAPI: common.PlacesAPIConfig{
			APIKey: "{test-key}",
		},
	}

	eventSvc := newMockEventService()

	// Create service with nil kvStorage
	service, err := NewService(testConfig, nil, eventSvc, logger)
	if err != nil {
		t.Fatalf("Failed to create ConfigService with nil kvStorage: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	config, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config with nil kvStorage: %v", err)
	}

	cfg := config.(*common.Config)

	// Verify placeholder is preserved (no injection)
	if cfg.PlacesAPI.APIKey != "{test-key}" {
		t.Errorf("Expected APIKey to remain as '{test-key}', got '%s'", cfg.PlacesAPI.APIKey)
	}

	t.Log("✓ Graceful degradation works with nil kvStorage")
}

// TestConfigService_ConcurrentAccess verifies thread-safety
func TestConfigService_ConcurrentAccess(t *testing.T) {
	logger := arbor.NewLogger()
	defer common.Stop()

	testConfig := &common.Config{
		Server: common.ServerConfig{
			Port: 8080,
		},
		PlacesAPI: common.PlacesAPIConfig{
			APIKey: "{test-key}",
		},
	}

	kvStorage := newMockKVStorage()
	kvStorage.Set(context.Background(), "test-key", "concurrent-value", "test")

	eventSvc := newMockEventService()

	service, err := NewService(testConfig, kvStorage, eventSvc, logger)
	if err != nil {
		t.Fatalf("Failed to create ConfigService: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Concurrent reads
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.GetConfig(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent read error: %v", err)
	}

	// Concurrent invalidations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.InvalidateCache()
		}()
	}

	wg.Wait()

	// Verify we can still get config
	config, err := service.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config after concurrent operations: %v", err)
	}

	cfg := config.(*common.Config)
	if cfg.PlacesAPI.APIKey != "concurrent-value" {
		t.Errorf("Expected APIKey to be 'concurrent-value', got '%s'", cfg.PlacesAPI.APIKey)
	}

	t.Log("✓ Thread-safe concurrent access works correctly")
}
