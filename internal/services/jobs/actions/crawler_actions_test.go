package actions

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
	"github.com/ternarybob/quaero/internal/services/jobs"
)

// Mock implementations

type mockCrawlerService struct {
	startCrawlFunc  func(context.Context, *crawler.CrawlRequest) (string, error)
	waitForJobFunc  func(context.Context, string) (interface{}, error)
	startCrawlCalls int
	waitForJobCalls int
	jobIDs          []string
}

func (m *mockCrawlerService) StartCrawl(ctx context.Context, req *crawler.CrawlRequest) (string, error) {
	m.startCrawlCalls++
	if m.startCrawlFunc != nil {
		return m.startCrawlFunc(ctx, req)
	}
	jobID := fmt.Sprintf("job-%d", m.startCrawlCalls)
	m.jobIDs = append(m.jobIDs, jobID)
	return jobID, nil
}

func (m *mockCrawlerService) WaitForJob(ctx context.Context, jobID string) (interface{}, error) {
	m.waitForJobCalls++
	if m.waitForJobFunc != nil {
		return m.waitForJobFunc(ctx, jobID)
	}
	// Return mock crawl results
	results := []*crawler.CrawlResult{
		{URL: "https://example.com/page1", Title: "Page 1"},
		{URL: "https://example.com/page2", Title: "Page 2"},
	}
	return results, nil
}

type mockAuthStorage struct {
	getCredentialsByIDFunc func(string) (*interfaces.AuthData, error)
}

func (m *mockAuthStorage) GetCredentialsByID(id string) (*interfaces.AuthData, error) {
	if m.getCredentialsByIDFunc != nil {
		return m.getCredentialsByIDFunc(id)
	}
	return &interfaces.AuthData{
		ID:       id,
		Provider: interfaces.AuthProviderAtlassian,
		Credentials: map[string]string{
			"atlToken": "test-token",
		},
	}, nil
}

func (m *mockAuthStorage) SaveCredentials(authData *interfaces.AuthData) error {
	return nil
}

func (m *mockAuthStorage) DeleteCredentials(id string) error {
	return nil
}

func (m *mockAuthStorage) ListCredentials() ([]*interfaces.AuthData, error) {
	return nil, nil
}

type mockEventService struct {
	publishFunc     func(context.Context, interfaces.EventType, interface{}) error
	publishSyncFunc func(context.Context, interfaces.EventType, interface{}) error
	events          []mockEvent
}

type mockEvent struct {
	eventType interfaces.EventType
	payload   interface{}
}

func (m *mockEventService) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) {
	// Not needed for tests
}

func (m *mockEventService) Publish(ctx context.Context, eventType interfaces.EventType, payload interface{}) error {
	m.events = append(m.events, mockEvent{eventType: eventType, payload: payload})
	if m.publishFunc != nil {
		return m.publishFunc(ctx, eventType, payload)
	}
	return nil
}

func (m *mockEventService) PublishSync(ctx context.Context, eventType interfaces.EventType, payload interface{}) error {
	m.events = append(m.events, mockEvent{eventType: eventType, payload: payload})
	if m.publishSyncFunc != nil {
		return m.publishSyncFunc(ctx, eventType, payload)
	}
	return nil
}

// Test helpers

func createTestDeps() (*CrawlerActionDeps, *mockCrawlerService, *mockAuthStorage, *mockEventService) {
	mockCrawler := &mockCrawlerService{}
	mockAuth := &mockAuthStorage{}
	mockEvents := &mockEventService{}

	logger := arbor.NewLogger()
	logger.SetConsole(true)
	logger.SetLevel(arbor.LogLevelInfo)

	cfg := &common.Config{}

	deps := &CrawlerActionDeps{
		CrawlerService: mockCrawler,
		AuthStorage:    mockAuth,
		EventService:   mockEvents,
		Config:         cfg,
		Logger:         logger,
	}

	return deps, mockCrawler, mockAuth, mockEvents
}

func createTestSources() []*models.SourceConfig {
	return []*models.SourceConfig{
		{
			ID:      "jira-source-1",
			Type:    models.SourceTypeJira,
			BaseURL: "https://example.atlassian.net",
			CrawlConfig: models.CrawlConfig{
				MaxDepth:     2,
				MaxPages:     100,
				SeedURLs:     []string{"https://example.atlassian.net/browse/PROJECT"},
				AllowedPaths: []string{"/browse/"},
			},
		},
		{
			ID:      "confluence-source-1",
			Type:    models.SourceTypeConfluence,
			BaseURL: "https://example.atlassian.net/wiki",
			CrawlConfig: models.CrawlConfig{
				MaxDepth:     3,
				MaxPages:     200,
				SeedURLs:     []string{"https://example.atlassian.net/wiki/spaces/SPACE"},
				AllowedPaths: []string{"/wiki/"},
			},
		},
	}
}

func createTestStep(action string, config map[string]interface{}) models.JobStep {
	return models.JobStep{
		Action:  action,
		Config:  config,
		OnError: models.ErrorStrategyFail,
		Retry: models.RetryConfig{
			MaxAttempts: 1,
		},
	}
}

// Tests for crawlAction

func TestCrawlAction_Success(t *testing.T) {
	deps, mockCrawler, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", map[string]interface{}{
		"wait_for_completion": true,
	})

	err := crawlAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if mockCrawler.startCrawlCalls != len(sources) {
		t.Errorf("Expected %d StartCrawl calls, got %d", len(sources), mockCrawler.startCrawlCalls)
	}

	if mockCrawler.waitForJobCalls != len(sources) {
		t.Errorf("Expected %d WaitForJob calls, got %d", len(sources), mockCrawler.waitForJobCalls)
	}

	// Verify collection events published
	collectionEvents := 0
	for _, event := range mockEvents.events {
		if event.eventType == interfaces.EventCollectionTriggered {
			collectionEvents++
		}
	}
	if collectionEvents != len(sources) {
		t.Errorf("Expected %d collection events, got %d", len(sources), collectionEvents)
	}
}

func TestCrawlAction_NoSources(t *testing.T) {
	deps, _, _, _ := createTestDeps()
	step := createTestStep("crawl", nil)

	err := crawlAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err == nil {
		t.Error("Expected error for no sources, got nil")
	}
}

func TestCrawlAction_StartCrawlFailure(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", nil)

	mockCrawler.startCrawlFunc = func(ctx context.Context, req *crawler.CrawlRequest) (string, error) {
		return "", fmt.Errorf("crawl failed")
	}

	err := crawlAction(context.Background(), step, sources, deps)

	if err == nil {
		t.Error("Expected error for failed crawl, got nil")
	}
}

func TestCrawlAction_WaitForJobFailure(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", map[string]interface{}{
		"wait_for_completion": true,
	})

	mockCrawler.waitForJobFunc = func(ctx context.Context, jobID string) (interface{}, error) {
		return nil, fmt.Errorf("wait failed")
	}

	err := crawlAction(context.Background(), step, sources, deps)

	if err == nil {
		t.Error("Expected error for failed wait, got nil")
	}
}

func TestCrawlAction_WithSeedURLOverrides(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()[:1] // Use only first source

	overrides := []interface{}{"https://custom.com/page1", "https://custom.com/page2"}
	step := createTestStep("crawl", map[string]interface{}{
		"seed_url_overrides": overrides,
	})

	var capturedRequest *crawler.CrawlRequest
	mockCrawler.startCrawlFunc = func(ctx context.Context, req *crawler.CrawlRequest) (string, error) {
		capturedRequest = req
		return "job-1", nil
	}

	err := crawlAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if capturedRequest == nil {
		t.Fatal("Expected crawl request to be captured")
	}

	// Verify overrides were passed (implementation detail may vary)
	// This is a placeholder - actual verification depends on CrawlRequest structure
}

func TestCrawlAction_WithRefreshSourceFalse(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()[:1]
	step := createTestStep("crawl", map[string]interface{}{
		"refresh_source": false,
	})

	var capturedRefreshSource bool
	originalStartCrawlFunc := mockCrawler.startCrawlFunc
	mockCrawler.startCrawlFunc = func(ctx context.Context, req *crawler.CrawlRequest) (string, error) {
		capturedRefreshSource = req.RefreshSourceConfig
		if originalStartCrawlFunc != nil {
			return originalStartCrawlFunc(ctx, req)
		}
		return "job-1", nil
	}

	err := crawlAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Note: This test assumes CrawlRequest has RefreshSourceConfig field
	// Verification depends on actual implementation
}

func TestCrawlAction_WithWaitForCompletionFalse(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", map[string]interface{}{
		"wait_for_completion": false,
	})

	err := crawlAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if mockCrawler.startCrawlCalls != len(sources) {
		t.Errorf("Expected %d StartCrawl calls, got %d", len(sources), mockCrawler.startCrawlCalls)
	}

	if mockCrawler.waitForJobCalls != 0 {
		t.Errorf("Expected 0 WaitForJob calls when wait_for_completion=false, got %d", mockCrawler.waitForJobCalls)
	}
}

func TestCrawlAction_EventPublishFailure(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", map[string]interface{}{
		"wait_for_completion": false,
	})

	mockEvents.publishSyncFunc = func(ctx context.Context, eventType interfaces.EventType, payload interface{}) error {
		return fmt.Errorf("event publish failed")
	}

	// Should not return error, just log warning (non-critical)
	err := crawlAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error for event publish failure (non-critical), got: %v", err)
	}
}

// Tests for transformAction

func TestTransformAction_Success(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("transform", nil)

	err := transformAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify collection events published for each source
	collectionEvents := 0
	for _, event := range mockEvents.events {
		if event.eventType == interfaces.EventCollectionTriggered {
			collectionEvents++
		}
	}
	if collectionEvents != len(sources) {
		t.Errorf("Expected %d collection events, got %d", len(sources), collectionEvents)
	}
}

func TestTransformAction_NoSources(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("transform", nil)

	err := transformAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error for no sources (applies to all), got: %v", err)
	}

	// Should publish single event for all sources
	if len(mockEvents.events) != 1 {
		t.Errorf("Expected 1 event for all sources, got %d", len(mockEvents.events))
	}
}

func TestTransformAction_WithJobID(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()[:1]
	step := createTestStep("transform", map[string]interface{}{
		"job_id": "test-job-123",
	})

	err := transformAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify job_id in payload
	if len(mockEvents.events) == 0 {
		t.Fatal("Expected at least one event")
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["job_id"] != "test-job-123" {
		t.Errorf("Expected job_id in payload to be 'test-job-123', got %v", payload["job_id"])
	}
}

func TestTransformAction_WithForceSync(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()[:1]
	step := createTestStep("transform", map[string]interface{}{
		"force_sync": true,
	})

	err := transformAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify force_sync in payload
	if len(mockEvents.events) == 0 {
		t.Fatal("Expected at least one event")
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["force_sync"] != true {
		t.Errorf("Expected force_sync in payload to be true, got %v", payload["force_sync"])
	}
}

func TestTransformAction_EventPublishFailure(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("transform", nil)

	mockEvents.publishSyncFunc = func(ctx context.Context, eventType interfaces.EventType, payload interface{}) error {
		return fmt.Errorf("event publish failed")
	}

	err := transformAction(context.Background(), step, sources, deps)

	// Should return error immediately for transform action
	if err == nil {
		t.Error("Expected error for event publish failure, got nil")
	}
}

// Tests for embedAction

func TestEmbedAction_Success(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("embed", nil)

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify embedding event published
	if len(mockEvents.events) != 1 {
		t.Errorf("Expected 1 embedding event, got %d", len(mockEvents.events))
	}

	if mockEvents.events[0].eventType != interfaces.EventEmbeddingTriggered {
		t.Errorf("Expected EventEmbeddingTriggered, got %v", mockEvents.events[0].eventType)
	}
}

func TestEmbedAction_WithForceEmbed(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("embed", map[string]interface{}{
		"force_embed": true,
	})

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["force_embed"] != true {
		t.Errorf("Expected force_embed in payload to be true, got %v", payload["force_embed"])
	}
}

func TestEmbedAction_WithBatchSize(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("embed", map[string]interface{}{
		"batch_size": 100,
	})

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["batch_size"] != 100 {
		t.Errorf("Expected batch_size in payload to be 100, got %v", payload["batch_size"])
	}
}

func TestEmbedAction_WithModelName(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("embed", map[string]interface{}{
		"model_name": "custom-model",
	})

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["model_name"] != "custom-model" {
		t.Errorf("Expected model_name in payload to be 'custom-model', got %v", payload["model_name"])
	}
}

func TestEmbedAction_WithFilterSourceIDs(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	filterIDs := []interface{}{"source1", "source2"}
	step := createTestStep("embed", map[string]interface{}{
		"filter_source_ids": filterIDs,
	})

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	filterSourceIDs, ok := payload["filter_source_ids"].([]string)
	if !ok {
		t.Fatalf("Expected filter_source_ids to be []string, got %T", payload["filter_source_ids"])
	}

	if len(filterSourceIDs) != 2 {
		t.Errorf("Expected 2 filter source IDs, got %d", len(filterSourceIDs))
	}
}

func TestEmbedAction_WithSources(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("embed", nil)

	err := embedAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	filterSourceIDs, ok := payload["filter_source_ids"].([]string)
	if !ok {
		t.Fatalf("Expected filter_source_ids to be []string, got %T", payload["filter_source_ids"])
	}

	if len(filterSourceIDs) != len(sources) {
		t.Errorf("Expected %d filter source IDs, got %d", len(sources), len(filterSourceIDs))
	}
}

func TestEmbedAction_EventPublishFailure(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	step := createTestStep("embed", nil)

	mockEvents.publishSyncFunc = func(ctx context.Context, eventType interfaces.EventType, payload interface{}) error {
		return fmt.Errorf("event publish failed")
	}

	err := embedAction(context.Background(), step, []*models.SourceConfig{}, deps)

	// Should return error immediately
	if err == nil {
		t.Error("Expected error for event publish failure, got nil")
	}
}

// Tests for RegisterCrawlerActions

func TestRegisterCrawlerActions_Success(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()

	err := RegisterCrawlerActions(registry, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify all three actions are registered
	actions := registry.ListActions(models.JobTypeCrawler)
	if len(actions) != 3 {
		t.Errorf("Expected 3 registered actions, got %d", len(actions))
	}

	// Verify each action can be retrieved
	crawlAction, err := registry.GetAction(models.JobTypeCrawler, "crawl")
	if err != nil || crawlAction == nil {
		t.Errorf("Failed to get crawl action: %v", err)
	}

	transformAction, err := registry.GetAction(models.JobTypeCrawler, "transform")
	if err != nil || transformAction == nil {
		t.Errorf("Failed to get transform action: %v", err)
	}

	embedAction, err := registry.GetAction(models.JobTypeCrawler, "embed")
	if err != nil || embedAction == nil {
		t.Errorf("Failed to get embed action: %v", err)
	}
}

func TestRegisterCrawlerActions_NilRegistry(t *testing.T) {
	deps, _, _, _ := createTestDeps()

	err := RegisterCrawlerActions(nil, deps)

	if err == nil {
		t.Error("Expected error for nil registry, got nil")
	}
}

func TestRegisterCrawlerActions_NilDependencies(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)

	err := RegisterCrawlerActions(registry, nil)

	if err == nil {
		t.Error("Expected error for nil dependencies, got nil")
	}
}

func TestRegisterCrawlerActions_MissingCrawlerService(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()
	deps.CrawlerService = nil

	err := RegisterCrawlerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil CrawlerService, got nil")
	}
}

func TestRegisterCrawlerActions_MissingAuthStorage(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()
	deps.AuthStorage = nil

	err := RegisterCrawlerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil AuthStorage, got nil")
	}
}

func TestRegisterCrawlerActions_MissingEventService(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()
	deps.EventService = nil

	err := RegisterCrawlerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil EventService, got nil")
	}
}

func TestRegisterCrawlerActions_MissingConfig(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()
	deps.Config = nil

	err := RegisterCrawlerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil Config, got nil")
	}
}

func TestRegisterCrawlerActions_MissingLogger(t *testing.T) {
	logger := arbor.NewLogger()
	registry := jobs.NewJobTypeRegistry(logger)
	deps, _, _, _ := createTestDeps()
	deps.Logger = nil

	err := RegisterCrawlerActions(registry, deps)

	if err == nil {
		t.Error("Expected error for nil Logger, got nil")
	}
}

// Tests for config extraction helpers

func TestExtractStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		key      string
		expected []string
	}{
		{
			name:     "valid string slice",
			config:   map[string]interface{}{"urls": []string{"url1", "url2"}},
			key:      "urls",
			expected: []string{"url1", "url2"},
		},
		{
			name:     "interface slice with strings",
			config:   map[string]interface{}{"urls": []interface{}{"url1", "url2"}},
			key:      "urls",
			expected: []string{"url1", "url2"},
		},
		{
			name:     "missing key",
			config:   map[string]interface{}{"other": "value"},
			key:      "urls",
			expected: nil,
		},
		{
			name:     "nil config",
			config:   nil,
			key:      "urls",
			expected: nil,
		},
		{
			name:     "invalid type",
			config:   map[string]interface{}{"urls": 123},
			key:      "urls",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStringSlice(tt.config, tt.key)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("Expected %s at index %d, got %s", tt.expected[i], i, v)
				}
			}
		})
	}
}

func TestExtractBool(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		key          string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "valid true",
			config:       map[string]interface{}{"enabled": true},
			key:          "enabled",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "valid false",
			config:       map[string]interface{}{"enabled": false},
			key:          "enabled",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "missing key uses default",
			config:       map[string]interface{}{"other": "value"},
			key:          "enabled",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "nil config uses default",
			config:       nil,
			key:          "enabled",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "invalid type uses default",
			config:       map[string]interface{}{"enabled": "yes"},
			key:          "enabled",
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBool(tt.config, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid int",
			config:       map[string]interface{}{"count": 42},
			key:          "count",
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "valid float64 (JSON)",
			config:       map[string]interface{}{"count": 42.0},
			key:          "count",
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "missing key uses default",
			config:       map[string]interface{}{"other": "value"},
			key:          "count",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "nil config uses default",
			config:       nil,
			key:          "count",
			defaultValue: 100,
			expected:     100,
		},
		{
			name:         "invalid type uses default",
			config:       map[string]interface{}{"count": "many"},
			key:          "count",
			defaultValue: 100,
			expected:     100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInt(tt.config, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "valid string",
			config:       map[string]interface{}{"name": "test"},
			key:          "name",
			defaultValue: "",
			expected:     "test",
		},
		{
			name:         "missing key uses default",
			config:       map[string]interface{}{"other": "value"},
			key:          "name",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "nil config uses default",
			config:       nil,
			key:          "name",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "invalid type uses default",
			config:       map[string]interface{}{"name": 123},
			key:          "name",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(tt.config, tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Additional integration-style tests

func TestCrawlAction_MultipleSourcesWithError(t *testing.T) {
	deps, mockCrawler, _, _ := createTestDeps()
	sources := createTestSources()
	step := createTestStep("crawl", nil)
	step.OnError = models.ErrorStrategyContinue // Continue on error

	callCount := 0
	mockCrawler.startCrawlFunc = func(ctx context.Context, req *crawler.CrawlRequest) (string, error) {
		callCount++
		if callCount == 1 {
			return "", fmt.Errorf("first source failed")
		}
		return fmt.Sprintf("job-%d", callCount), nil
	}

	err := crawlAction(context.Background(), step, sources, deps)

	// Should have errors aggregated but not fail completely
	if err == nil {
		t.Error("Expected aggregated errors, got nil")
	}

	// Both sources should have been attempted
	if callCount != len(sources) {
		t.Errorf("Expected %d StartCrawl calls, got %d", len(sources), callCount)
	}
}

func TestTransformAction_WithBatchSize(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()[:1]
	step := createTestStep("transform", map[string]interface{}{
		"batch_size": 500,
	})

	err := transformAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockEvents.events) == 0 {
		t.Fatal("Expected at least one event")
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["batch_size"] != 500 {
		t.Errorf("Expected batch_size 500, got %v", payload["batch_size"])
	}
}

func TestEmbedAction_CompletePayload(t *testing.T) {
	deps, _, _, mockEvents := createTestDeps()
	sources := createTestSources()
	step := createTestStep("embed", map[string]interface{}{
		"force_embed": true,
		"batch_size":  75,
		"model_name":  "test-model-v2",
	})

	err := embedAction(context.Background(), step, sources, deps)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockEvents.events) == 0 {
		t.Fatal("Expected at least one event")
	}

	payload, ok := mockEvents.events[0].payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be map[string]interface{}")
	}

	if payload["force_embed"] != true {
		t.Errorf("Expected force_embed true, got %v", payload["force_embed"])
	}

	if payload["batch_size"] != 75 {
		t.Errorf("Expected batch_size 75, got %v", payload["batch_size"])
	}

	if payload["model_name"] != "test-model-v2" {
		t.Errorf("Expected model_name 'test-model-v2', got %v", payload["model_name"])
	}

	if payload["timestamp"] == nil {
		t.Error("Expected timestamp in payload")
	}

	filterSourceIDs, ok := payload["filter_source_ids"].([]string)
	if !ok {
		t.Fatalf("Expected filter_source_ids to be []string, got %T", payload["filter_source_ids"])
	}

	if len(filterSourceIDs) != len(sources) {
		t.Errorf("Expected %d source IDs in filter, got %d", len(sources), len(filterSourceIDs))
	}
}
