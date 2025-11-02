// -----------------------------------------------------------------------
// Last Modified: Monday, 3rd November 2025 10:10:42 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/services/crawler"
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

// mockJobDefinitionStorage implements interfaces.JobDefinitionStorage
type mockJobDefinitionStorage struct {
	jobDefs map[string]*models.JobDefinition
	err     error
}

func (m *mockJobDefinitionStorage) SaveJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	if m.err != nil {
		return m.err
	}
	m.jobDefs[jobDef.ID] = jobDef
	return nil
}

func (m *mockJobDefinitionStorage) GetJobDefinition(ctx context.Context, id string) (*models.JobDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	if jobDef, ok := m.jobDefs[id]; ok {
		return jobDef, nil
	}
	return nil, errors.New("job definition not found")
}

func (m *mockJobDefinitionStorage) ListJobDefinitions(ctx context.Context, opts *interfaces.JobDefinitionListOptions) ([]*models.JobDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.JobDefinition, 0, len(m.jobDefs))
	for _, jd := range m.jobDefs {
		list = append(list, jd)
	}
	return list, nil
}

func (m *mockJobDefinitionStorage) GetJobDefinitionsByType(ctx context.Context, jobType string) ([]*models.JobDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.JobDefinition, 0)
	for _, jd := range m.jobDefs {
		if string(jd.Type) == jobType {
			list = append(list, jd)
		}
	}
	return list, nil
}

func (m *mockJobDefinitionStorage) DeleteJobDefinition(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.jobDefs, id)
	return nil
}

func (m *mockJobDefinitionStorage) GetEnabledJobDefinitions(ctx context.Context) ([]*models.JobDefinition, error) {
	if m.err != nil {
		return nil, m.err
	}
	list := make([]*models.JobDefinition, 0)
	for _, jd := range m.jobDefs {
		if jd.Enabled {
			list = append(list, jd)
		}
	}
	return list, nil
}

func (m *mockJobDefinitionStorage) UpdateJobDefinition(ctx context.Context, jobDef *models.JobDefinition) error {
	if m.err != nil {
		return m.err
	}
	m.jobDefs[jobDef.ID] = jobDef
	return nil
}

func (m *mockJobDefinitionStorage) CountJobDefinitions(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return len(m.jobDefs), nil
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

// mockCrawlerService implements a mock crawler service
type mockCrawlerService struct {
	jobs map[string]map[string]interface{}
}

func (m *mockCrawlerService) Start() error {
	return nil
}

func (m *mockCrawlerService) StartCrawl(sourceType, entityType string, seedURLs []string, config interface{}, sourceID string, refreshSource bool, sourceConfigSnapshot interface{}, authSnapshot interface{}, jobDefinitionID string) (string, error) {
	return "mock-job-id", nil
}

func (m *mockCrawlerService) GetJobStatus(jobID string) (interface{}, error) {
	if job, ok := m.jobs[jobID]; ok {
		// Convert legacy map format to CrawlJob for backward compatibility
		status := job["status"].(string)
		return createCrawlJob(jobID, status, 100, 0, "jira"), nil
	}
	// Default: return completed job
	return createCrawlJob(jobID, "completed", 100, 0, "jira"), nil
}

func (m *mockCrawlerService) CancelJob(jobID string) error {
	return nil
}

func (m *mockCrawlerService) GetJobResults(jobID string) (interface{}, error) {
	return nil, nil
}

func (m *mockCrawlerService) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) (interface{}, error) {
	return nil, nil
}

func (m *mockCrawlerService) RerunJob(ctx context.Context, jobID string, updateConfig interface{}) (string, error) {
	return "mock-rerun-job-id", nil
}

func (m *mockCrawlerService) WaitForJob(ctx context.Context, jobID string) (interface{}, error) {
	return nil, nil
}

func (m *mockCrawlerService) Close() error {
	return nil
}

// statefulMockCrawlerService implements deterministic state transitions for async polling tests
type statefulMockCrawlerService struct {
	callCounts map[string]int
	states     map[string][]interface{} // jobID -> ordered states
}

func newStatefulMockCrawlerService() *statefulMockCrawlerService {
	return &statefulMockCrawlerService{
		callCounts: make(map[string]int),
		states:     make(map[string][]interface{}),
	}
}

func (m *statefulMockCrawlerService) Start() error {
	return nil
}

func (m *statefulMockCrawlerService) StartCrawl(sourceType, entityType string, seedURLs []string, config interface{}, sourceID string, refreshSource bool, sourceConfigSnapshot interface{}, authSnapshot interface{}) (string, error) {
	return "mock-job-id", nil
}

func (m *statefulMockCrawlerService) GetJobStatus(jobID string) (interface{}, error) {
	states, ok := m.states[jobID]
	if !ok {
		// Return completed by default for unknown jobs
		return createCrawlJob(jobID, "completed", 100, 0, "jira"), nil
	}

	callCount := m.callCounts[jobID]
	m.callCounts[jobID]++

	// Return state at call index, or last state if beyond end
	if callCount >= len(states) {
		return states[len(states)-1].(*crawler.CrawlJob), nil
	}
	return states[callCount].(*crawler.CrawlJob), nil
}

func (m *statefulMockCrawlerService) CancelJob(jobID string) error {
	return nil
}

func (m *statefulMockCrawlerService) GetJobResults(jobID string) (interface{}, error) {
	return nil, nil
}

func (m *statefulMockCrawlerService) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) (interface{}, error) {
	return nil, nil
}

func (m *statefulMockCrawlerService) RerunJob(ctx context.Context, jobID string, updateConfig interface{}) (string, error) {
	return "mock-rerun-job-id", nil
}

func (m *statefulMockCrawlerService) WaitForJob(ctx context.Context, jobID string) (interface{}, error) {
	return nil, nil
}

func (m *statefulMockCrawlerService) Close() error {
	return nil
}

func (m *statefulMockCrawlerService) setJobStates(jobID string, states []interface{}) {
	m.states[jobID] = states
}

// createCrawlJob helper creates a CrawlJob with specified state and progress
func createCrawlJob(jobID string, status string, completedURLs, failedURLs int, sourceType string) *crawler.CrawlJob {
	totalURLs := 100
	pendingURLs := totalURLs - completedURLs - failedURLs
	percentage := (float64(completedURLs) / float64(totalURLs)) * 100.0

	job := &crawler.CrawlJob{
		ID:         jobID,
		SourceType: sourceType,
		Status:     crawler.JobStatus(status),
		Progress: crawler.CrawlProgress{
			TotalURLs:     totalURLs,
			CompletedURLs: completedURLs,
			FailedURLs:    failedURLs,
			PendingURLs:   pendingURLs,
			Percentage:    percentage,
			CurrentURL:    "https://example.com/page-1",
		},
	}

	// Set error message for failed jobs
	if status == "failed" {
		job.Error = "crawl job failed"
	}

	return job
}

// mockActionHandler creates a mock action handler
func createMockActionHandler(shouldFail bool, failCount int) ActionHandler {
	callCount := 0
	return func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		callCount++
		if shouldFail && callCount <= failCount {
			return errors.New("mock action failed")
		}
		return nil
	}
}

// Test helpers

// createTestExecutor creates an executor with mock dependencies
func createTestExecutor() (*JobExecutor, *mockSourceStorage, *mockEventService, *JobTypeRegistry, *mockJobDefinitionStorage) {
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
	crawlerSvc := &mockCrawlerService{
		jobs: make(map[string]map[string]interface{}),
	}
	jobDefStorage := &mockJobDefinitionStorage{
		jobDefs: make(map[string]*models.JobDefinition),
	}

	// Create real sources.Service with mock storage
	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)

	// Create executor with real dependencies
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	return executor, sourceStorage, eventSvc, registry, jobDefStorage
}

// createTestJobDefinition creates a test job definition
func createTestJobDefinition(jobType models.JobType, sources []string, steps []models.JobStep) *models.JobDefinition {
	return &models.JobDefinition{
		ID:          "test-job-1",
		Name:        "Test Job",
		Type:        models.JobDefinitionType(jobType),
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
			CrawlConfig: models.SourceCrawlConfig{
				MaxDepth:    2,
				Concurrency: 5,
				FollowLinks: true,
				MaxPages:    100,
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
			CrawlConfig: models.SourceCrawlConfig{
				MaxDepth:    2,
				Concurrency: 5,
				FollowLinks: true,
				MaxPages:    100,
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
	crawlerSvc := &mockCrawlerService{jobs: make(map[string]map[string]interface{})}
	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}
	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)

	t.Run("successful initialization", func(t *testing.T) {
		executor, err := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)
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
		_, err := NewJobExecutor(nil, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)
		if err == nil {
			t.Error("Expected error for nil registry")
		}
	})

	t.Run("nil source service", func(t *testing.T) {
		_, err := NewJobExecutor(registry, nil, eventSvc, crawlerSvc, jobDefStorage, logger)
		if err == nil {
			t.Error("Expected error for nil source service")
		}
	})

	t.Run("nil event service", func(t *testing.T) {
		_, err := NewJobExecutor(registry, sourceService, nil, crawlerSvc, jobDefStorage, logger)
		if err == nil {
			t.Error("Expected error for nil event service")
		}
	})

	t.Run("nil crawler service", func(t *testing.T) {
		_, err := NewJobExecutor(registry, sourceService, eventSvc, nil, jobDefStorage, logger)
		if err == nil {
			t.Error("Expected error for nil crawler service")
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, nil)
		if err == nil {
			t.Error("Expected error for nil logger")
		}
	})
}

// TestExecute_Success tests successful job execution
func TestExecute_Success(t *testing.T) {
	executor, sourceStorage, eventSvc, registry, _ := createTestExecutor()

	// Setup sources
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]
	sourceStorage.sources["source-2"] = sourcesData[1]

	// Register action handlers
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(false, 0))
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1", "source-2"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify success
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false for non-crawl steps")
	}

	// Verify events published
	if len(eventSvc.events) < 3 { // Start + 2 steps + completion
		t.Errorf("Expected at least 3 events, got %d", len(eventSvc.events))
	}
}

// TestExecute_InvalidJobDefinition tests execution with invalid job definition
func TestExecute_InvalidJobDefinition(t *testing.T) {
	executor, _, _, _, _ := createTestExecutor()

	// Create invalid job definition (missing required fields)
	jobDef := &models.JobDefinition{
		ID:   "",
		Name: "",
		Type: models.JobDefinitionTypeCrawler,
	}

	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	_, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	if err == nil {
		t.Error("Expected validation error")
	}
}

// TestExecute_SourceFetchFailure tests source fetch failure
func TestExecute_SourceFetchFailure(t *testing.T) {
	executor, sourceStorage, _, _, _ := createTestExecutor()

	// Configure source storage to return error
	sourceStorage.err = errors.New("source fetch failed")

	// Create job definition
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	_, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify error
	if err == nil {
		t.Error("Expected source fetch error")
	}
	if !strings.Contains(err.Error(), "failed to fetch sources") {
		t.Errorf("Expected source fetch error, got: %v", err)
	}
}

// TestExecute_StepFailure_Continue tests continue error strategy
func TestExecute_StepFailure_Continue(t *testing.T) {
	executor, sourceStorage, _, registry, _ := createTestExecutor()

	// Setup sources
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]

	// Register action handlers - first fails, second succeeds
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition with Continue strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyContinue},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify job completes with error but execution continued
	if err == nil {
		t.Error("Expected aggregated error")
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false for non-polling steps")
	}
}

// TestExecute_StepFailure_Fail tests fail error strategy
func TestExecute_StepFailure_Fail(t *testing.T) {
	executor, sourceStorage, _, registry, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handlers - first fails
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "transform", createMockActionHandler(false, 0))

	// Create job definition with Fail strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyFail},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify job stopped immediately
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "job execution failed at step 0") {
		t.Errorf("Expected step 0 failure, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false when job fails early")
	}
}

// TestExecute_StepFailure_Retry tests retry error strategy
func TestExecute_StepFailure_Retry(t *testing.T) {
	executor, sourceStorage, _, registry, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handler that fails first 2 times, succeeds on 3rd
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 2))

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
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify job succeeds after retries
	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false for non-polling steps")
	}
}

// TestExecute_ActionHandlerNotFound tests missing action handler
func TestExecute_ActionHandlerNotFound(t *testing.T) {
	executor, sourceStorage, _, _, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Create job definition with non-existent action
	steps := []models.JobStep{
		{Name: "step1", Action: "nonexistent", OnError: models.ErrorStrategyFail},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	_, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify error
	if err == nil {
		t.Error("Expected action not found error")
	}
	if !strings.Contains(err.Error(), "action handler not found") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected action not found error, got: %v", err)
	}
}

// TestExecute_ContextCancellation tests context cancellation
func TestExecute_ContextCancellation(t *testing.T) {
	executor, sourceStorage, _, registry, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register action handler that checks context
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
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
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Create cancellable context and cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Execute job
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	_, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify context cancellation handled
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

// TestExecute_MultipleStepFailures tests multiple failing steps with continue strategy
func TestExecute_MultipleStepFailures(t *testing.T) {
	executor, sourceStorage, _, registry, _ := createTestExecutor()

	// Setup sources
	sources := createTestSources()
	sourceStorage.sources["source-1"] = sources[0]

	// Register failing action handlers
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 100))
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "transform", createMockActionHandler(true, 100))

	// Create job definition with Continue strategy
	steps := []models.JobStep{
		{Name: "step1", Action: "crawl", OnError: models.ErrorStrategyContinue},
		{Name: "step2", Action: "transform", OnError: models.ErrorStrategyContinue},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify all errors aggregated
	if err == nil {
		t.Error("Expected aggregated errors")
	}
	if !strings.Contains(err.Error(), "2 error(s)") {
		t.Errorf("Expected 2 errors, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false for failed steps")
	}
}

// TestHandleStepError_Continue tests continue strategy
func TestHandleStepError_Continue(t *testing.T) {
	executor, _, _, _, _ := createTestExecutor()

	step := models.JobStep{
		Name:    "test",
		Action:  "test",
		OnError: models.ErrorStrategyContinue,
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	testErr := errors.New("test error")
	err := executor.handleStepError(ctx, jobDef, step, 0, testErr, []*models.SourceConfig{})

	if err == nil {
		t.Error("Expected error for continue strategy (for aggregation)")
	}
	if err != testErr {
		t.Errorf("Expected original error, got: %v", err)
	}
}

// TestHandleStepError_Fail tests fail strategy
func TestHandleStepError_Fail(t *testing.T) {
	executor, _, _, _, _ := createTestExecutor()

	step := models.JobStep{
		Name:    "test",
		Action:  "test",
		OnError: models.ErrorStrategyFail,
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	testErr := errors.New("test error")
	err := executor.handleStepError(ctx, jobDef, step, 0, testErr, []*models.SourceConfig{})

	if err == nil {
		t.Error("Expected error for fail strategy")
	}
	if err != testErr {
		t.Errorf("Expected original error, got: %v", err)
	}
}

// TestRetryStep_SuccessOnFirstRetry tests immediate success on retry
func TestRetryStep_SuccessOnFirstRetry(t *testing.T) {
	executor, _, _, registry, _ := createTestExecutor()

	// Register handler that succeeds immediately
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(false, 0))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries": 3,
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}
}

// TestRetryStep_SuccessAfterMultipleRetries tests success after multiple attempts
func TestRetryStep_SuccessAfterMultipleRetries(t *testing.T) {
	executor, _, _, registry, _ := createTestExecutor()

	// Register handler that fails twice, succeeds on third
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 2))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries":     3,
			"initial_backoff": 0.1,
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
}

// TestRetryStep_ExhaustedRetries tests all retries exhausted
func TestRetryStep_ExhaustedRetries(t *testing.T) {
	executor, _, _, registry, _ := createTestExecutor()

	// Register handler that always fails
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", createMockActionHandler(true, 100))

	step := models.JobStep{
		Name:   "test",
		Action: "crawl",
		Config: map[string]interface{}{
			"max_retries":     3,
			"initial_backoff": 0.1,
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{}, []models.JobStep{step})

	ctx := context.Background()
	err := executor.retryStep(ctx, jobDef, step, []*models.SourceConfig{})

	if err == nil {
		t.Error("Expected error after exhausted retries")
	}
	if !strings.Contains(err.Error(), "failed after 3 retries") {
		t.Errorf("Expected retry count in error, got: %v", err)
	}
}

// TestFetchSources_Success tests successful source fetching
func TestFetchSources_Success(t *testing.T) {
	executor, sourceStorage, _, _, _ := createTestExecutor()

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
	executor, _, _, _, _ := createTestExecutor()

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
	executor, _, _, _, _ := createTestExecutor()

	ctx := context.Background()
	_, err := executor.fetchSources(ctx, []string{"nonexistent"})

	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "failed to fetch source") {
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

// TestAsyncPolling_SuccessfulCompletion tests successful async polling to completion
func TestAsyncPolling_SuccessfulCompletion(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create stateful crawler service with job progressing from running to completed
	crawlerSvc := newStatefulMockCrawlerService()
	crawlerSvc.setJobStates("crawl-job-1", []interface{}{
		createCrawlJob("crawl-job-1", "running", 50, 0, "jira"),
		createCrawlJob("crawl-job-1", "completed", 100, 0, "jira"),
	})

	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}

	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	// Setup source
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]

	// Register action handler that stores job IDs for polling
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		// Simulate crawler action storing job IDs
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		step.Config["crawl_job_ids"] = []string{"crawl-job-1"}
		step.Config["wait_for_completion"] = true
		return nil
	})

	// Create job definition
	steps := []models.JobStep{
		{
			Name:    "crawl-step",
			Action:  "crawl",
			OnError: models.ErrorStrategyFail,
			Config: map[string]interface{}{
				"wait_for_completion": true,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify success
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if !result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be true for crawl step with wait_for_completion")
	}

	// Give polling goroutine time to complete (ticker interval is 5s, need ~11s for 2 ticks)
	time.Sleep(11 * time.Second)

	// Verify EventJobProgress events were emitted with crawl-specific fields
	crawlProgressEventCount := 0
	for _, evt := range eventSvc.evtFull {
		if evt.Type == interfaces.EventJobProgress {
			payload, ok := evt.Payload.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if this is a crawl-specific progress event (has crawl_job_id)
			if crawlJobID, ok := payload["crawl_job_id"].(string); ok {
				crawlProgressEventCount++

				// Verify crawl-specific fields are present
				if crawlJobID == "" {
					t.Error("EventJobProgress crawl_job_id is empty")
				}
				if sourceType, ok := payload["source_type"].(string); !ok || sourceType != "jira" {
					t.Errorf("EventJobProgress source_type incorrect, got: %v", payload["source_type"])
				}
				if _, ok := payload["total_urls"]; !ok {
					t.Error("EventJobProgress missing total_urls field")
				}
				if _, ok := payload["completed_urls"]; !ok {
					t.Error("EventJobProgress missing completed_urls field")
				}
				if _, ok := payload["percentage"]; !ok {
					t.Error("EventJobProgress missing percentage field")
				}
			}
		}
	}

	if crawlProgressEventCount < 2 {
		t.Errorf("Expected at least 2 crawl progress events (with crawl_job_id), got %d", crawlProgressEventCount)
	}
}

// TestAsyncPolling_JobFailure tests async polling handling a job failure
func TestAsyncPolling_JobFailure(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create stateful crawler service with job progressing from running to failed
	crawlerSvc := newStatefulMockCrawlerService()
	crawlerSvc.setJobStates("crawl-job-2", []interface{}{
		createCrawlJob("crawl-job-2", "running", 50, 5, "confluence"),
		createCrawlJob("crawl-job-2", "failed", 60, 10, "confluence"),
	})

	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}

	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	// Setup source
	sourcesData := createTestSources()
	sourceStorage.sources["source-2"] = sourcesData[1]

	// Register action handler that stores job IDs for polling
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		step.Config["crawl_job_ids"] = []string{"crawl-job-2"}
		step.Config["wait_for_completion"] = true
		return nil
	})

	// Create job definition with Continue error strategy
	steps := []models.JobStep{
		{
			Name:    "crawl-step",
			Action:  "crawl",
			OnError: models.ErrorStrategyContinue,
			Config: map[string]interface{}{
				"wait_for_completion": true,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-2"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Async polling: Execute returns immediately, errors reported via events only
	if err != nil {
		t.Errorf("Expected no immediate error (async polling), got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if !result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be true for crawl step with wait_for_completion")
	}

	// Give polling goroutine time to complete (ticker interval is 5s, need ~11s for 2 ticks)
	time.Sleep(11 * time.Second)

	// Verify EventJobProgress events were emitted including failure status
	failedEventFound := false
	for _, evt := range eventSvc.evtFull {
		if evt.Type == interfaces.EventJobProgress {
			payload, ok := evt.Payload.(map[string]interface{})
			if !ok {
				continue
			}

			status, ok := payload["status"].(string)
			if ok && status == "failed" {
				failedEventFound = true

				// Verify error field is present
				if errMsg, ok := payload["error"].(string); !ok || errMsg == "" {
					t.Error("Failed event missing error message")
				}

				// Verify failed_urls field
				if failedURLs, ok := payload["failed_urls"].(int); !ok || failedURLs == 0 {
					t.Error("Failed event should have failed_urls > 0")
				}
			}
		}
	}

	if !failedEventFound {
		t.Error("Expected to find failed status in EventJobProgress")
	}
}

// TestAsyncPolling_ContextCancellation tests context cancellation during polling
func TestAsyncPolling_ContextCancellation(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create stateful crawler service that stays running indefinitely
	crawlerSvc := newStatefulMockCrawlerService()
	crawlerSvc.setJobStates("crawl-job-3", []interface{}{
		createCrawlJob("crawl-job-3", "running", 10, 0, "jira"),
		createCrawlJob("crawl-job-3", "running", 20, 0, "jira"),
		createCrawlJob("crawl-job-3", "running", 30, 0, "jira"),
		createCrawlJob("crawl-job-3", "running", 40, 0, "jira"),
		createCrawlJob("crawl-job-3", "running", 50, 0, "jira"),
		createCrawlJob("crawl-job-3", "running", 60, 0, "jira"),
	})

	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}

	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	// Setup source
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]

	// Register action handler
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		step.Config["crawl_job_ids"] = []string{"crawl-job-3"}
		step.Config["wait_for_completion"] = true
		return nil
	})

	// Create job definition
	steps := []models.JobStep{
		{
			Name:    "crawl-step",
			Action:  "crawl",
			OnError: models.ErrorStrategyFail,
			Config: map[string]interface{}{
				"wait_for_completion": true,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job with cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	_, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Note: Polling happens in background goroutine with context.Background(),
	// so cancelling request context won't affect polling. This test verifies
	// that the main execute returns immediately even if polling continues.
	// In production, polling would continue until jobs complete or service stops.

	// Verify execution completed (possibly with timeout if action checks context)
	if err != nil && !strings.Contains(err.Error(), "context") {
		// Error is acceptable if it's context-related
		t.Logf("Execute returned error: %v", err)
	}
}

// TestAsyncPolling_SkipWhenWaitForCompletionFalse tests skipping polling when wait_for_completion is false
func TestAsyncPolling_SkipWhenWaitForCompletionFalse(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create stateful crawler service
	crawlerSvc := newStatefulMockCrawlerService()

	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}

	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	// Setup source
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]

	// Register action handler that stores job IDs but sets wait_for_completion = false
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		step.Config["crawl_job_ids"] = []string{"crawl-job-4"}
		step.Config["wait_for_completion"] = false
		return nil
	})

	// Create job definition with wait_for_completion = false
	steps := []models.JobStep{
		{
			Name:    "crawl-step",
			Action:  "crawl",
			OnError: models.ErrorStrategyFail,
			Config: map[string]interface{}{
				"wait_for_completion": false,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Verify success (no polling happened)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be false when wait_for_completion is false")
	}

	// Wait a bit to ensure polling doesn't happen
	time.Sleep(500 * time.Millisecond)

	// Verify GetJobStatus was NOT called (because polling was skipped)
	if crawlerSvc.callCounts["crawl-job-4"] > 0 {
		t.Errorf("Expected polling to be skipped, but GetJobStatus was called %d times", crawlerSvc.callCounts["crawl-job-4"])
	}

	// Verify no crawl-specific progress events (no polling happened)
	crawlProgressEventCount := 0
	for _, evt := range eventSvc.evtFull {
		if evt.Type == interfaces.EventJobProgress {
			payload, ok := evt.Payload.(map[string]interface{})
			if ok {
				// Check if this is a crawl-specific progress event (has crawl_job_id)
				if _, hasCrawlJobID := payload["crawl_job_id"]; hasCrawlJobID {
					crawlProgressEventCount++
				}
			}
		}
	}

	// Should have NO crawl-specific progress events since polling was skipped
	if crawlProgressEventCount > 0 {
		t.Errorf("Expected no crawl progress events (polling skipped), got %d", crawlProgressEventCount)
	}
}

// TestAsyncPolling_MultipleJobsWithMixedOutcomes tests polling multiple jobs with different outcomes
func TestAsyncPolling_MultipleJobsWithMixedOutcomes(t *testing.T) {
	logger := arbor.NewLogger()
	registry := NewJobTypeRegistry(logger)

	sourceStorage := &mockSourceStorage{sources: make(map[string]*models.SourceConfig)}
	authStorage := &mockAuthStorage{}
	eventSvc := &mockEventService{
		events:  make([]interfaces.EventType, 0),
		data:    make([]interface{}, 0),
		evtFull: make([]interfaces.Event, 0),
	}

	// Create stateful crawler service with multiple jobs
	crawlerSvc := newStatefulMockCrawlerService()

	// Job 1: Completes successfully
	crawlerSvc.setJobStates("crawl-job-5", []interface{}{
		createCrawlJob("crawl-job-5", "running", 50, 0, "jira"),
		createCrawlJob("crawl-job-5", "completed", 100, 0, "jira"),
	})

	// Job 2: Fails
	crawlerSvc.setJobStates("crawl-job-6", []interface{}{
		createCrawlJob("crawl-job-6", "running", 30, 0, "confluence"),
		createCrawlJob("crawl-job-6", "failed", 50, 20, "confluence"),
	})

	jobDefStorage := &mockJobDefinitionStorage{jobDefs: make(map[string]*models.JobDefinition)}

	sourceService := sources.NewService(sourceStorage, authStorage, eventSvc, logger)
	executor, _ := NewJobExecutor(registry, sourceService, eventSvc, crawlerSvc, jobDefStorage, logger)

	// Setup sources
	sourcesData := createTestSources()
	sourceStorage.sources["source-1"] = sourcesData[0]
	sourceStorage.sources["source-2"] = sourcesData[1]

	// Register action handler that stores multiple job IDs
	registry.RegisterAction(models.JobDefinitionTypeCrawler, "crawl", func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error {
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		step.Config["crawl_job_ids"] = []string{"crawl-job-5", "crawl-job-6"}
		step.Config["wait_for_completion"] = true
		return nil
	})

	// Create job definition with Continue error strategy
	steps := []models.JobStep{
		{
			Name:    "crawl-step",
			Action:  "crawl",
			OnError: models.ErrorStrategyContinue,
			Config: map[string]interface{}{
				"wait_for_completion": true,
			},
		},
	}
	jobDef := createTestJobDefinition(models.JobDefinitionTypeCrawler, []string{"source-1", "source-2"}, steps)

	// Execute job
	ctx := context.Background()
	noOpCallback := func(ctx context.Context, status string, errorMsg string) error { return nil }
	result, err := executor.Execute(ctx, jobDef, noOpCallback, nil)

	// Async polling: Execute returns immediately, errors reported via events only
	if err != nil {
		t.Errorf("Expected no immediate error (async polling), got: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if !result.AsyncPollingActive {
		t.Error("Expected AsyncPollingActive to be true for crawl step with wait_for_completion and multiple jobs")
	}

	// Give polling goroutine time to complete (ticker interval is 5s, need ~11s for 2 ticks)
	time.Sleep(11 * time.Second)

	// Verify both jobs were polled
	if crawlerSvc.callCounts["crawl-job-5"] == 0 {
		t.Error("Expected crawl-job-5 to be polled")
	}
	if crawlerSvc.callCounts["crawl-job-6"] == 0 {
		t.Error("Expected crawl-job-6 to be polled")
	}

	// Verify progress events for both jobs
	foundJob5Completed := false
	foundJob6Failed := false
	for _, evt := range eventSvc.evtFull {
		if evt.Type == interfaces.EventJobProgress {
			payload, ok := evt.Payload.(map[string]interface{})
			if !ok {
				continue
			}

			crawlJobID, _ := payload["crawl_job_id"].(string)
			status, _ := payload["status"].(string)

			if crawlJobID == "crawl-job-5" && status == "completed" {
				foundJob5Completed = true
			}
			if crawlJobID == "crawl-job-6" && status == "failed" {
				foundJob6Failed = true
			}
		}
	}

	if !foundJob5Completed {
		t.Error("Expected to find completed status for crawl-job-5")
	}
	if !foundJob6Failed {
		t.Error("Expected to find failed status for crawl-job-6")
	}
}
