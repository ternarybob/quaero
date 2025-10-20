// -----------------------------------------------------------------------
// Last Modified: Monday, 21st October 2025 5:50:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/sources"
)

// Mock implementations

// mockSourceStorage implements interfaces.SourceStorage
type mockSourceStorage struct {
	sources map[string]*models.SourceConfig
	err     error
}

func (m *mockSourceStorage) SaveSource(ctx context.Context, source *models.SourceConfig) error {
	if m.err != nil {
		return m.err
	}
	m.sources[source.ID] = source
	return nil
}

func (m *mockSourceStorage) GetSource(ctx context.Context, id string) (*models.SourceConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	if source, ok := m.sources[id]; ok {
		return source, nil
	}
	return nil, errors.New("source not found")
}

func (m *mockSourceStorage) ListSources(ctx context.Context) ([]*models.SourceConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.SourceConfig, 0, len(m.sources))
	for _, s := range m.sources {
		list = append(list, s)
	}
	return list, nil
}

func (m *mockSourceStorage) DeleteSource(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.sources, id)
	return nil
}

func (m *mockSourceStorage) GetEnabledSources(ctx context.Context) ([]*models.SourceConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.SourceConfig, 0)
	for _, s := range m.sources {
		if s.Enabled {
			list = append(list, s)
		}
	}
	return list, nil
}

func (m *mockSourceStorage) GetSourcesByType(ctx context.Context, sourceType string) ([]*models.SourceConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.SourceConfig, 0)
	for _, s := range m.sources {
		if s.Type == sourceType {
			list = append(list, s)
		}
	}
	return list, nil
}

// mockAuthStorage implements interfaces.AuthStorage
type mockAuthStorage struct{}

func (m *mockAuthStorage) SaveAuthData(ctx context.Context, data *interfaces.AuthData) error {
	return nil
}

func (m *mockAuthStorage) GetAuthData(ctx context.Context) (*interfaces.AuthData, error) {
	return nil, errors.New("no auth data")
}

func (m *mockAuthStorage) DeleteAuthData(ctx context.Context) error {
	return nil
}

func (m *mockAuthStorage) DeleteCredentials(ctx context.Context, id string) error {
	return nil
}

func (m *mockAuthStorage) GetCredentials(ctx context.Context, id string) (*models.AuthCredentials, error) {
	return nil, errors.New("credentials not found")
}

func (m *mockAuthStorage) GetCredentialsByID(ctx context.Context, id string) (*models.AuthCredentials, error) {
	return nil, errors.New("credentials not found")
}

func (m *mockAuthStorage) GetCredentialsBySiteDomain(ctx context.Context, siteDomain string) (*models.AuthCredentials, error) {
	return nil, errors.New("credentials not found")
}

func (m *mockAuthStorage) StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error {
	return nil
}

func (m *mockAuthStorage) ListCredentials(ctx context.Context) ([]*models.AuthCredentials, error) {
	return []*models.AuthCredentials{}, nil
}

func (m *mockAuthStorage) ListServices(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// mockEventService implements a mock event service
type mockEventService struct {
	events  []interfaces.EventType
	data    []interface{}
	evtFull []interfaces.Event
}

func (m *mockEventService) Publish(ctx context.Context, event interfaces.Event) error {
	m.events = append(m.events, event.Type)
	m.data = append(m.data, event.Payload)
	m.evtFull = append(m.evtFull, event)
	return nil
}

func (m *mockEventService) PublishSync(ctx context.Context, event interfaces.Event) error {
	m.events = append(m.events, event.Type)
	m.data = append(m.data, event.Payload)
	m.evtFull = append(m.evtFull, event)
	return nil
}

func (m *mockEventService) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	// Not needed for executor tests
	return nil
}

func (m *mockEventService) Unsubscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	// Not needed for executor tests
	return nil
}

func (m *mockEventService) Close() error {
	// Not needed for executor tests
	return nil
}

// mockActionHandler creates a mock action handler
func createMockActionHandler(shouldFail bool, failCount int) ActionHandler {
	callCount := 0
	return func(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
		callCount++
		if shouldFail && callCount <= failCount {
			return errors.New("mock action failed")
		}
		return nil
	}
}

// Test helpers

// createTestExecutor creates an executor with mock dependencies
func createTestExecutor() (*JobExecutor, *mockSourceStorage, *mockEventService, *JobTypeRegistry) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{
		sources: make(map[string]*models.SourceConfig),
	}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create real sources.Service with mock storage
	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)

	// Create executor with real dependencies
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, logger)

	return executor, sourceStorage, eventSvc, registry
}

// createTestJobDefinition creates a test job definition
func createTestJobDefinition(jobType models.JobType, sources []string, steps []models.JobStep) *models.JobDefinition {
	return &models.JobDefinition{
		ID:          "test-job-1",
		Name:        "Test Job",
		Type:        jobType,
		Description: "Test job definition",
		Sources:     sources,
		Steps:       steps,
		Schedule:    "0 0 * * *",
		Enabled:     true,
		AutoStart:   false,
		Config:      make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// createTestSources creates test source configurations
func createTestSources() []*models.SourceConfig {
	return []*models.SourceConfig{
		{
			ID:      "source-1",
			Name:    "Test Jira",
			Type:    "jira",
			BaseURL: "https://test.atlassian.net",
			Enabled: true,
			Filters: make(map[string]interface{}),
			CrawlConfig: models.CrawlConfig{
				MaxDepth:    2,
				Concurrency: 5,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:      "source-2",
			Name:    "Test Confluence",
			Type:    "confluence",
			BaseURL: "https://test.atlassian.net/wiki",
			Enabled: true,
			Filters: make(map[string]interface{}),
			CrawlConfig: models.CrawlConfig{
				MaxDepth:    2,
				Concurrency: 5,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// Test cases

// TestNewJobExecutor tests executor initialization
func TestNewJobExecutor(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)
	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{}
	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)

	t.Run("successful initialization", func(t *testing.T) {
		executor, err := NewJobExecutor(registry, sourceService, eventSvc, logger)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if executor == nil {
			t.Error("Expected non-nil executor")
		}
		if executor.registry != registry {
			t.Error("Registry not set correctly")
		}
		if executor.logger != logger {
			t.Error("Logger not set correctly")
		}
	})

	t.Run("nil registry", func(t *testing.T) {
		_, err := NewJobExecutor(nil, sourceService, eventSvc, logger)
		if err == nil {
			t.Error("Expected error for nil registry")
		}
	})

	t.Run("nil source service", func(t *testing.T) {
		_, err := NewJobExecutor(registry, nil, eventSvc, logger)
		if err == nil {
			t.Error("Expected error for nil source service")
		}
	})

	t.Run("nil event service", func(t *testing.T) {
		_, err := NewJobExecutor(registry, sourceService, nil, logger)
		if err == nil {
			t.Error("Expected error for nil event service")
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := NewJobExecutor(registry, sourceService, eventSvc, nil)
		if err == nil {
			t.Error("Expected error for nil logger")
		}
	})
}

// TestExecute_Success tests successful job execution
func TestExecute_Success(t *testing.T) {
	executor, sourceStorage, eventSvc, registry := createTestExecutor()

	// Setup sources
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]
	sourceStorage.sources["source-2"] = sourcesData[1]

	// Register action handlers
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(false, 0))
	registry.RegisterAction(models.JobTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1", "source-2"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify success
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify events published
	if len(eventSvc.events) < 3 { // Start + 2 steps + completion
		t.Errorf("Expected at least 3 events, got %d", len(eventSvc.events))
	}
}

// TestExecute_InvalidJobDefinition tests execution with invalid job definition
func TestExecute_InvalidJobDefinition(t *testing.T) {
	executor, _, _, _ := createTestExecutor()

	// Create invalid job definition (missing required fields)
	jobDef := &models.JobDefinition{
		ID:   "",
		Name: "",
		Type: models.JobTypeCrawler,
	}

	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	if err == nil {
		t.Error("Expected validation error")
	}
}

// TestExecute_SourceFetchFailure tests source fetch failure
func TestExecute_SourceFetchFailure(t *testing.T) {
	executor, sourceStorage, _, _ := createTestExecutor()

	// Configure source storage to return error
	sourceStorage.err = errors.New("source fetch failed")

	// Create job definition
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify error
	if err == nil {
		t.Error("Expected source fetch error")
	}
	if !contains(err.Error(), "failed to fetch sources") {
		t.Errorf("Expected source fetch error, got: %v", err)
	}
}

// TestExecute_StepFailure_Continue tests continue error strategy
func TestExecute_StepFailure_Continue(t *testing.T) {
	executor, sourceStorage, _, registry := createTestExecutor()

	// Setup sources
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]

	// Register action handlers - first fails, second succeeds
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition with Continue strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyContinue},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify job completes with error but execution continued
	if err == nil {
		t.Error("Expected aggregated error")
	}
}

// TestExecute_StepFailure_Fail tests fail error strategy
func TestExecute_StepFailure_Fail(t *testing.T) {
	executor, sourceStorage, _, registry := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handlers - first fails
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition with Fail strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify job stopped immediately
	if err == nil {
		t.Error("Expected error")
	}
	if !contains(err.Error(), "job execution failed at step 0") {
		t.Errorf("Expected step 0 failure, got: %v", err)
	}
}

// TestExecute_StepFailure_Retry tests retry error strategy
func TestExecute_StepFailure_Retry(t *testing.T) {
	executor, sourceStorage, _, registry := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handler that fails first 2 times, succeeds on 3rd
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 2))

	// Create job definition with Retry strategy
	steps := []models.JobStep{
		{
			Name:    "step1",
			Action:  "crawl",
			OnError: models.ErrorStrategyRetry,
			Config: map[string]interface{}{
				"max_retries":        3,
				"initial_backoff":    1,
				"max_backoff":        10,
				"backoff_multiplier": 2.0,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify job succeeds after retries
	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
}

// TestExecute_ActionHandlerNotFound tests missing action handler
func TestExecute_ActionHandlerNotFound(t *testing.T) {
	executor, sourceStorage, _, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Create job definition with non-existent action
	steps := []models.JobStep{
		{Name: "step1", Action: "nonexistent", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify error
	if err == nil {
		t.Error("Expected action not found error")
	}
	if !contains(err.Error(), "action handler not found") && !contains(err.Error(), "not found") {
		t.Errorf("Expected action not found error, got: %v", err)
	}
}

// TestExecute_ContextCancellation tests context cancellation
func TestExecute_ContextCancellation(t *testing.T) {
	executor, sourceStorage, _, registry := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handler that checks context
	registry.RegisterAction(models.JobTypeCrawler, "crawl", func(ctx context.Context, step models.JobStep, sources []*models.SourceConfig) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})

	// Create job definition
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Create cancellable context and cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Execute job
	err := executor.Execute(ctx, jobDef)

	// Verify context cancellation handled
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

// TestExecute_MultipleStepFailures tests multiple failing steps with continue strategy
func TestExecute_MultipleStepFailures(t *testing.T) {
	executor, sourceStorage, _, registry := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register failing action handlers
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobTypeCrawler, "transform", createMockActionHandler(true, 100))

	// Create job definition with Continue strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyContinue},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyContinue},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	err := executor.Execute(ctx, jobDef)

	// Verify all errors aggregated
	if err == nil {
		t.Error("Expected aggregated errors")
	}
	if !contains(err.Error(), "2 error(s)") {
		t.Errorf("Expected 2 errors, got: %v", err)
	}
}

// TestHandleStepError_Continue tests continue strategy
func TestHandleStepError_Continue(t *testing.T) {
	executor, _, _, _ := createTestExecutor()

	step := models.JobStep{
		Name:    "test",
		Action:  "test",
		OnError: models.ErrorStrategyContinue,
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	testErr := errors.New("test error")
	err := executor.handleStepError(ctx, jobDef, step, 0, testErr)

	if err == nil {
		t.Error("Expected error for continue strategy (for aggregation)")
	}
	if err != testErr {
		t.Errorf("Expected original error, got: %v", err)
	}
}

// TestHandleStepError_Fail tests fail strategy
func TestHandleStepError_Fail(t *testing.T) {
	executor, _, _, _ := createTestExecutor()

	step := models.JobStep{
		Name:    "test",
		Action:  "test",
		OnError: models.ErrorStrategyFail,
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	testErr := errors.New("test error")
	err := executor.handleStepError(ctx, jobDef, step, 0, testErr)

	if err == nil {
		t.Error("Expected error for fail strategy")
	}
	if err != testErr {
		t.Errorf("Expected original error, got: %v", err)
	}
}

// TestRetryStep_SuccessOnFirstRetry tests immediate success on retry
func TestRetryStep_SuccessOnFirstRetry(t *testing.T) {
	executor, _, _, registry := createTestExecutor()

	// Register handler that succeeds immediately
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(false, 0))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries": 3,
		},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
}

// TestRetryStep_SuccessAfterMultipleRetries tests success after multiple attempts
func TestRetryStep_SuccessAfterMultipleRetries(t *testing.T) {
	executor, _, _, registry := createTestExecutor()

	// Register handler that fails twice, succeeds on third
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 2))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries":     3,
			"initial_backoff": 0.1,
		},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
}

// TestRetryStep_ExhaustedRetries tests all retries exhausted
func TestRetryStep_ExhaustedRetries(t *testing.T) {
	executor, _, _, registry := createTestExecutor()

	// Register handler that always fails
	registry.RegisterAction(models.JobTypeCrawler, "crawl", createMockActionHandler(true, 100))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries":     3,
			"initial_backoff": 0.1,
		},
	}
	jobDef := createTestJobDefinition(models.JobTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err == nil {
		t.Error("Expected error after exhausted retries")
	}
	if !contains(err.Error(), "failed after 3 retries") {
		t.Errorf("Expected retry count in error, got: %v", err)
	}
}

// TestFetchSources_Success tests successful source fetching
func TestFetchSources_Success(t *testing.T) {
	executor, sourceStorage, _, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]
	sourceStorage.sources["source-2"] = sources[1]

	ctx := context.Background()
	fetched, err := executor.fetchSources(ctx, []string{"source-1", "source-2"})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if len(fetched) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(fetched))
	}
}

// TestFetchSources_EmptySourceList tests empty source list
func TestFetchSources_EmptySourceList(t *testing.T) {
	executor, _, _, _ := createTestExecutor()

	ctx := context.Background()
	fetched, err := executor.fetchSources(ctx, []string{})

	if err != nil {
		t.Errorf("Expected no error for empty list, got: %v", err)
	}
	if len(fetched) != 0 {
		t.Errorf("Expected empty slice, got %d sources", len(fetched))
	}
}

// TestFetchSources_SourceNotFound tests source not found error
func TestFetchSources_SourceNotFound(t *testing.T) {
	executor, _, _, _ := createTestExecutor()

	ctx := context.Background()
	_, err := executor.fetchSources(ctx, []string{"nonexistent"})

	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
	if !contains(err.Error(), "failed to fetch source") {
		t.Errorf("Expected fetch error, got: %v", err)
	}
}

// TestExtractRetryConfig tests retry configuration extraction
func TestExtractRetryConfig(t *testing.T) {
	t.Run("all values present", func(t *testing.T) {
		config := map[string]interface{}{
			"max_retries":        5,
			"initial_backoff":    3,
			"max_backoff":        120,
			"backoff_multiplier": 3.0,
		}

		maxRetries, initialBackoff, maxBackoff, multiplier := extractRetryConfig(config)

		if maxRetries != 5 {
			t.Errorf("Expected max_retries=5, got %d", maxRetries)
		}
		if initialBackoff != 3*time.Second {
			t.Errorf("Expected initial_backoff=3s, got %v", initialBackoff)
		}
		if maxBackoff != 120*time.Second {
			t.Errorf("Expected max_backoff=120s, got %v", maxBackoff)
		}
		if multiplier != 3.0 {
			t.Errorf("Expected multiplier=3.0, got %f", multiplier)
		}
	})

	t.Run("missing values use defaults", func(t *testing.T) {
		config := map[string]interface{}{}

		maxRetries, initialBackoff, maxBackoff, multiplier := extractRetryConfig(config)

		if maxRetries != 3 {
			t.Errorf("Expected default max_retries=3, got %d", maxRetries)
		}
		if initialBackoff != 2*time.Second {
			t.Errorf("Expected default initial_backoff=2s, got %v", initialBackoff)
		}
		if maxBackoff != 60*time.Second {
			t.Errorf("Expected default max_backoff=60s, got %v", maxBackoff)
		}
		if multiplier != 2.0 {
			t.Errorf("Expected default multiplier=2.0, got %f", multiplier)
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		maxRetries, initialBackoff, maxBackoff, multiplier := extractRetryConfig(nil)

		if maxRetries != 3 {
			t.Errorf("Expected default max_retries=3, got %d", maxRetries)
		}
		if initialBackoff != 2*time.Second {
			t.Errorf("Expected default initial_backoff=2s, got %v", initialBackoff)
		}
		if maxBackoff != 60*time.Second {
			t.Errorf("Expected default max_backoff=60s, got %v", maxBackoff)
		}
		if multiplier != 2.0 {
			t.Errorf("Expected default multiplier=2.0, got %f", multiplier)
		}
	})

	t.Run("string duration format", func(t *testing.T) {
		config := map[string]interface{}{
			"initial_backoff": "5s",
			"max_backoff":     "2m",
		}

		_, initialBackoff, maxBackoff, _ := extractRetryConfig(config)

		if initialBackoff != 5*time.Second {
			t.Errorf("Expected initial_backoff=5s, got %v", initialBackoff)
		}
		if maxBackoff != 2*time.Minute {
			t.Errorf("Expected max_backoff=2m, got %v", maxBackoff)
		}
	})
}
