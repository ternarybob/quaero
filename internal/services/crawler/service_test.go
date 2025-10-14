package crawler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

// Mock AuthService
type mockAuthService struct {
	client *http.Client
}

func (m *mockAuthService) GetHTTPClient() *http.Client {
	if m.client != nil {
		return m.client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (m *mockAuthService) LoadAuth() (*interfaces.AtlassianAuthData, error) {
	return &interfaces.AtlassianAuthData{}, nil
}

func (m *mockAuthService) UpdateAuth(authData *interfaces.AtlassianAuthData) error {
	return nil
}

func (m *mockAuthService) IsAuthenticated() bool {
	return true
}

func (m *mockAuthService) GetBaseURL() string {
	return "https://test.atlassian.net"
}

func (m *mockAuthService) GetUserAgent() string {
	return "test-user-agent"
}

func (m *mockAuthService) GetCloudID() string {
	return "test-cloud-id"
}

func (m *mockAuthService) GetAtlToken() string {
	return "test-atl-token"
}

// Mock EventService
type mockEventService struct {
	events []interfaces.Event
}

func (m *mockEventService) Publish(ctx context.Context, event interfaces.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventService) PublishSync(ctx context.Context, event interfaces.Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockEventService) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	// No-op for tests
	return nil
}

func (m *mockEventService) Unsubscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	// No-op for tests
	return nil
}

func (m *mockEventService) Close() error {
	return nil
}

// Mock JobStorage
type mockJobStorage struct {
	jobs map[string]*CrawlJob
}

func (m *mockJobStorage) SaveJob(ctx context.Context, job interface{}) error {
	crawlJob, ok := job.(*CrawlJob)
	if !ok {
		return nil
	}
	if m.jobs == nil {
		m.jobs = make(map[string]*CrawlJob)
	}
	m.jobs[crawlJob.ID] = crawlJob
	return nil
}

func (m *mockJobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	if m.jobs == nil {
		return nil, nil
	}
	return m.jobs[jobID], nil
}

func (m *mockJobStorage) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]interface{}, error) {
	if m.jobs == nil {
		return []interface{}{}, nil
	}
	jobs := make([]interface{}, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (m *mockJobStorage) DeleteJob(ctx context.Context, jobID string) error {
	if m.jobs != nil {
		delete(m.jobs, jobID)
	}
	return nil
}

func (m *mockJobStorage) CountJobs(ctx context.Context) (int, error) {
	if m.jobs == nil {
		return 0, nil
	}
	return len(m.jobs), nil
}

func (m *mockJobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	if m.jobs == nil {
		return 0, nil
	}
	count := 0
	for _, job := range m.jobs {
		if job.Status == JobStatus(status) {
			count++
		}
	}
	return count, nil
}

func (m *mockJobStorage) GetJobsByStatus(ctx context.Context, status string) ([]interface{}, error) {
	if m.jobs == nil {
		return []interface{}{}, nil
	}
	jobs := make([]interface{}, 0)
	for _, job := range m.jobs {
		if job.Status == JobStatus(status) {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (m *mockJobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	return nil
}

func (m *mockJobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	return nil
}

// Helper function to create test service
func createTestService() *Service {
	logger := arbor.NewLogger()
	config := CrawlConfig{
		MaxDepth:    3,
		MaxPages:    100,
		Concurrency: 2,
		RateLimit:   time.Millisecond * 100,
	}

	return NewService(
		&mockAuthService{},
		&mockEventService{},
		&mockJobStorage{},
		logger,
		config,
	)
}

// TestNewService tests service initialization
func TestNewService(t *testing.T) {
	tests := []struct {
		name   string
		config CrawlConfig
	}{
		{
			name: "Default configuration",
			config: CrawlConfig{
				MaxDepth:    3,
				Concurrency: 2,
				RateLimit:   time.Second,
			},
		},
		{
			name: "With retry configuration",
			config: CrawlConfig{
				MaxDepth:      5,
				Concurrency:   4,
				RateLimit:     time.Millisecond * 500,
				RetryAttempts: 5,
				RetryBackoff:  time.Second * 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := arbor.NewLogger()
			service := NewService(
				&mockAuthService{},
				&mockEventService{},
				&mockJobStorage{},
				logger,
				tt.config,
			)

			if service == nil {
				t.Fatal("Expected non-nil service")
			}

			if service.queue == nil {
				t.Error("Expected queue to be initialized")
			}

			if service.rateLimiter == nil {
				t.Error("Expected rate limiter to be initialized")
			}

			if service.retryPolicy == nil {
				t.Error("Expected retry policy to be initialized")
			}

			if service.activeJobs == nil {
				t.Error("Expected activeJobs map to be initialized")
			}

			if service.jobResults == nil {
				t.Error("Expected jobResults map to be initialized")
			}

			// Verify retry policy configuration
			if tt.config.RetryAttempts > 0 {
				if service.retryPolicy.MaxAttempts != tt.config.RetryAttempts {
					t.Errorf("Expected MaxAttempts=%d, got %d", tt.config.RetryAttempts, service.retryPolicy.MaxAttempts)
				}
			}

			if tt.config.RetryBackoff > 0 {
				if service.retryPolicy.InitialBackoff != tt.config.RetryBackoff {
					t.Errorf("Expected InitialBackoff=%v, got %v", tt.config.RetryBackoff, service.retryPolicy.InitialBackoff)
				}
			}
		})
	}
}

// TestStartCrawl tests job creation and start
func TestStartCrawl(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		entityType string
		seedURLs   []string
		config     CrawlConfig
	}{
		{
			name:       "Jira projects crawl",
			sourceType: "jira",
			entityType: "projects",
			seedURLs:   []string{"https://test.atlassian.net/rest/api/3/project"},
			config: CrawlConfig{
				MaxDepth:    1,
				Concurrency: 1,
				RateLimit:   time.Millisecond * 100,
			},
		},
		{
			name:       "Confluence spaces crawl",
			sourceType: "confluence",
			entityType: "spaces",
			seedURLs:   []string{"https://test.atlassian.net/wiki/rest/api/space"},
			config: CrawlConfig{
				MaxDepth:    2,
				Concurrency: 2,
				RateLimit:   time.Millisecond * 100,
			},
		},
		{
			name:       "Multiple seed URLs",
			sourceType: "jira",
			entityType: "issues",
			seedURLs: []string{
				"https://test.atlassian.net/rest/api/3/search?jql=project=TEST",
				"https://test.atlassian.net/rest/api/3/search?jql=project=DEMO",
			},
			config: CrawlConfig{
				MaxDepth:    1,
				Concurrency: 2,
				RateLimit:   time.Millisecond * 100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := createTestService()
			defer service.Close()

			jobID, err := service.StartCrawl(tt.sourceType, tt.entityType, tt.seedURLs, tt.config)
			if err != nil {
				t.Fatalf("StartCrawl failed: %v", err)
			}

			if jobID == "" {
				t.Fatal("Expected non-empty jobID")
			}

			// Verify job was created
			job, err := service.GetJobStatus(jobID)
			if err != nil {
				t.Fatalf("GetJobStatus failed: %v", err)
			}

			if job.ID != jobID {
				t.Errorf("Expected job ID=%s, got %s", jobID, job.ID)
			}

			if job.SourceType != tt.sourceType {
				t.Errorf("Expected SourceType=%s, got %s", tt.sourceType, job.SourceType)
			}

			if job.EntityType != tt.entityType {
				t.Errorf("Expected EntityType=%s, got %s", tt.entityType, job.EntityType)
			}

			if job.Status != JobStatusRunning {
				t.Errorf("Expected Status=running, got %s", job.Status)
			}

			if job.Progress.TotalURLs != len(tt.seedURLs) {
				t.Errorf("Expected TotalURLs=%d, got %d", len(tt.seedURLs), job.Progress.TotalURLs)
			}

			// Clean up - cancel the job
			_ = service.CancelJob(jobID)
		})
	}
}

// TestGetJobStatus tests job status retrieval
func TestGetJobStatus(t *testing.T) {
	service := createTestService()
	defer service.Close()

	// Test non-existent job
	_, err := service.GetJobStatus("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent job")
	}

	// Create a job
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Test existing job
	job, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}

	if job.ID != jobID {
		t.Errorf("Expected job ID=%s, got %s", jobID, job.ID)
	}

	// Clean up
	_ = service.CancelJob(jobID)
}

// TestCancelJob tests job cancellation
func TestCancelJob(t *testing.T) {
	service := createTestService()
	defer service.Close()

	// Test cancelling non-existent job
	err := service.CancelJob("non-existent")
	if err == nil {
		t.Error("Expected error when cancelling non-existent job")
	}

	// Create a job
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Cancel the job
	err = service.CancelJob(jobID)
	if err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}

	// Verify job status
	job, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}

	if job.Status != JobStatusCancelled {
		t.Errorf("Expected Status=cancelled, got %s", job.Status)
	}

	if job.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set")
	}

	// Test cancelling already cancelled job
	err = service.CancelJob(jobID)
	if err == nil {
		t.Error("Expected error when cancelling already cancelled job")
	}
}

// TestGetJobResults tests result retrieval
func TestGetJobResults(t *testing.T) {
	service := createTestService()
	defer service.Close()

	// Test getting results for non-existent job
	_, err := service.GetJobResults("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent job results")
	}

	// Create a job
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Get results (may be empty if job just started)
	results, err := service.GetJobResults(jobID)
	if err != nil {
		t.Fatalf("GetJobResults failed: %v", err)
	}

	if results == nil {
		t.Error("Expected non-nil results slice")
	}

	// Clean up
	_ = service.CancelJob(jobID)
}

// TestListJobs tests job listing
func TestListJobs(t *testing.T) {
	service := createTestService()
	defer service.Close()

	ctx := context.Background()

	// List jobs when none exist
	jobs, err := service.ListJobs(ctx, nil)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	initialCount := len(jobs)

	// Create multiple jobs
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}

	jobIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config)
		if err != nil {
			t.Fatalf("StartCrawl failed: %v", err)
		}
		jobIDs[i] = jobID
	}

	// List all jobs
	jobs, err = service.ListJobs(ctx, nil)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != initialCount+3 {
		t.Errorf("Expected %d jobs, got %d", initialCount+3, len(jobs))
	}

	// Clean up
	for _, jobID := range jobIDs {
		_ = service.CancelJob(jobID)
	}
}

// TestCrawlConfigToJSON tests configuration serialization
func TestCrawlConfigToJSON(t *testing.T) {
	config := CrawlConfig{
		MaxDepth:        3,
		MaxPages:        100,
		Concurrency:     4,
		RateLimit:       time.Second,
		RetryAttempts:   3,
		RetryBackoff:    time.Second * 2,
		IncludePatterns: []string{".*\\.com"},
		ExcludePatterns: []string{".*\\.pdf"},
		FollowLinks:     true,
		DetailLevel:     "full",
	}

	jsonStr, err := config.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if jsonStr == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Deserialize and verify
	parsed, err := FromJSONCrawlConfig(jsonStr)
	if err != nil {
		t.Fatalf("FromJSONCrawlConfig failed: %v", err)
	}

	if parsed.MaxDepth != config.MaxDepth {
		t.Errorf("Expected MaxDepth=%d, got %d", config.MaxDepth, parsed.MaxDepth)
	}

	if parsed.MaxPages != config.MaxPages {
		t.Errorf("Expected MaxPages=%d, got %d", config.MaxPages, parsed.MaxPages)
	}

	if parsed.Concurrency != config.Concurrency {
		t.Errorf("Expected Concurrency=%d, got %d", config.Concurrency, parsed.Concurrency)
	}

	if parsed.DetailLevel != config.DetailLevel {
		t.Errorf("Expected DetailLevel=%s, got %s", config.DetailLevel, parsed.DetailLevel)
	}
}

// TestCrawlProgressToJSON tests progress serialization
func TestCrawlProgressToJSON(t *testing.T) {
	progress := CrawlProgress{
		TotalURLs:           100,
		CompletedURLs:       50,
		FailedURLs:          5,
		PendingURLs:         45,
		CurrentURL:          "https://example.com",
		Percentage:          50.0,
		StartTime:           time.Now(),
		EstimatedCompletion: time.Now().Add(time.Hour),
	}

	jsonStr, err := progress.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if jsonStr == "" {
		t.Error("Expected non-empty JSON string")
	}

	// Deserialize and verify
	parsed, err := FromJSONCrawlProgress(jsonStr)
	if err != nil {
		t.Fatalf("FromJSONCrawlProgress failed: %v", err)
	}

	if parsed.TotalURLs != progress.TotalURLs {
		t.Errorf("Expected TotalURLs=%d, got %d", progress.TotalURLs, parsed.TotalURLs)
	}

	if parsed.CompletedURLs != progress.CompletedURLs {
		t.Errorf("Expected CompletedURLs=%d, got %d", progress.CompletedURLs, parsed.CompletedURLs)
	}

	if parsed.FailedURLs != progress.FailedURLs {
		t.Errorf("Expected FailedURLs=%d, got %d", progress.FailedURLs, parsed.FailedURLs)
	}

	if parsed.CurrentURL != progress.CurrentURL {
		t.Errorf("Expected CurrentURL=%s, got %s", progress.CurrentURL, parsed.CurrentURL)
	}
}

// TestServiceShutdown tests graceful shutdown
func TestServiceShutdown(t *testing.T) {
	service := createTestService()

	// Start a job
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}
	_, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Shutdown should complete without blocking
	done := make(chan error, 1)
	go func() {
		done <- service.Shutdown()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Shutdown timed out")
	}
}
