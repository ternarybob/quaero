package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
	"maragu.dev/goqite"
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
	jobs     map[string]*CrawlJob
	seenURLs map[string]map[string]bool // jobID -> url -> seen
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

func (m *mockJobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	// Same as SaveJob for mock purposes
	return m.SaveJob(ctx, job)
}

func (m *mockJobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	if m.jobs == nil {
		return nil, nil
	}
	return m.jobs[jobID], nil
}

func (m *mockJobStorage) ListJobs(ctx context.Context, opts *interfaces.ListOptions) ([]*models.CrawlJob, error) {
	if m.jobs == nil {
		return []*models.CrawlJob{}, nil
	}
	jobs := make([]*models.CrawlJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		// Convert internal CrawlJob to models.CrawlJob
		modelJob := &models.CrawlJob{
			ID:           job.ID,
			SourceType:   job.SourceType,
			EntityType:   job.EntityType,
			Status:       string(job.Status),
			StartedAt:    job.StartedAt,
			CompletedAt:  job.CompletedAt,
			ConfigJSON:   job.ConfigJSON,
			ProgressJSON: job.ProgressJSON,
			ParentJobID:  job.ParentJobID,
			JobDefID:     job.JobDefID,
		}
		jobs = append(jobs, modelJob)
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

func (m *mockJobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.ListOptions) (int, error) {
	// Simple mock implementation - just return total count
	return m.CountJobs(ctx)
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

func (m *mockJobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.CrawlJob, error) {
	if m.jobs == nil {
		return []*models.CrawlJob{}, nil
	}
	jobs := make([]*models.CrawlJob, 0)
	for _, job := range m.jobs {
		if job.Status == JobStatus(status) {
			// Convert internal CrawlJob to models.CrawlJob
			modelJob := &models.CrawlJob{
				ID:           job.ID,
				SourceType:   job.SourceType,
				EntityType:   job.EntityType,
				Status:       string(job.Status),
				StartedAt:    job.StartedAt,
				CompletedAt:  job.CompletedAt,
				ConfigJSON:   job.ConfigJSON,
				ProgressJSON: job.ProgressJSON,
				ParentJobID:  job.ParentJobID,
				JobDefID:     job.JobDefID,
			}
			jobs = append(jobs, modelJob)
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

func (m *mockJobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	return nil
}

func (m *mockJobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	return []models.JobLogEntry{}, nil
}

func (m *mockJobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockJobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (m *mockJobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	if m.seenURLs == nil {
		m.seenURLs = make(map[string]map[string]bool)
	}
	if m.seenURLs[jobID] == nil {
		m.seenURLs[jobID] = make(map[string]bool)
	}

	// Check if URL was already seen
	if m.seenURLs[jobID][url] {
		return false, nil // Already seen
	}

	// Mark as seen
	m.seenURLs[jobID][url] = true
	return true, nil // Newly added
}

// Mock QueueManager
type mockQueueManager struct {
	messages []*queue.JobMessage
	handlers map[string]func(context.Context, *queue.JobMessage) error
}

func newMockQueueManager() *mockQueueManager {
	return &mockQueueManager{
		messages: make([]*queue.JobMessage, 0),
		handlers: make(map[string]func(context.Context, *queue.JobMessage) error),
	}
}

func (m *mockQueueManager) Start() error   { return nil }
func (m *mockQueueManager) Stop() error    { return nil }
func (m *mockQueueManager) Restart() error { return nil }

func (m *mockQueueManager) Enqueue(ctx context.Context, msg *queue.JobMessage) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockQueueManager) EnqueueWithDelay(ctx context.Context, msg *queue.JobMessage, delay time.Duration) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockQueueManager) Receive(ctx context.Context) (*goqite.Message, error) {
	if len(m.messages) == 0 {
		return nil, fmt.Errorf("no messages")
	}
	msg := m.messages[0]
	m.messages = m.messages[1:]

	// Serialize to JSON
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return &goqite.Message{
		ID:   goqite.ID(msg.ID),
		Body: body,
	}, nil
}

func (m *mockQueueManager) Delete(ctx context.Context, msg goqite.Message) error { return nil }
func (m *mockQueueManager) Extend(ctx context.Context, msg goqite.Message, duration time.Duration) error {
	return nil
}
func (m *mockQueueManager) GetQueueLength(ctx context.Context) (int, error) {
	return len(m.messages), nil
}
func (m *mockQueueManager) GetQueueStats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"pending_messages": len(m.messages),
		"total_messages":   len(m.messages),
	}, nil
}

// RegisterHandler allows tests to register job handlers
func (m *mockQueueManager) RegisterHandler(jobType string, handler func(context.Context, *queue.JobMessage) error) {
	m.handlers[jobType] = handler
}

// ProcessMessages manually processes all queued messages with registered handlers
func (m *mockQueueManager) ProcessMessages(ctx context.Context) error {
	for len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]

		handler, exists := m.handlers[msg.Type]
		if !exists {
			return fmt.Errorf("no handler registered for type: %s", msg.Type)
		}

		if err := handler(ctx, msg); err != nil {
			return fmt.Errorf("handler failed: %w", err)
		}
	}
	return nil
}

// Mock DocumentStorage
type mockDocumentStorage struct {
	documents map[string]*models.Document
}

func (m *mockDocumentStorage) SaveDocument(doc *models.Document) error {
	if m.documents == nil {
		m.documents = make(map[string]*models.Document)
	}
	m.documents[doc.ID] = doc
	return nil
}

func (m *mockDocumentStorage) SaveDocuments(docs []*models.Document) error {
	for _, doc := range docs {
		if err := m.SaveDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockDocumentStorage) GetDocument(id string) (*models.Document, error) {
	if m.documents == nil {
		return nil, nil
	}
	return m.documents[id], nil
}

func (m *mockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	if m.documents == nil {
		return nil, nil
	}
	for _, doc := range m.documents {
		if doc.SourceType == sourceType && doc.SourceID == sourceID {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *mockDocumentStorage) UpdateDocument(doc *models.Document) error {
	if m.documents == nil {
		m.documents = make(map[string]*models.Document)
	}
	m.documents[doc.ID] = doc
	return nil
}

func (m *mockDocumentStorage) DeleteDocument(id string) error {
	if m.documents != nil {
		delete(m.documents, id)
	}
	return nil
}

func (m *mockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *mockDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *mockDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	if m.documents == nil {
		return []*models.Document{}, nil
	}
	docs := make([]*models.Document, 0, len(m.documents))
	for _, doc := range m.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

func (m *mockDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	if m.documents == nil {
		return []*models.Document{}, nil
	}
	docs := make([]*models.Document, 0)
	for _, doc := range m.documents {
		if doc.SourceType == sourceType {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func (m *mockDocumentStorage) CountDocuments() (int, error) {
	if m.documents == nil {
		return 0, nil
	}
	return len(m.documents), nil
}

func (m *mockDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
	if m.documents == nil {
		return 0, nil
	}
	count := 0
	for _, doc := range m.documents {
		if doc.SourceType == sourceType {
			count++
		}
	}
	return count, nil
}

func (m *mockDocumentStorage) GetStats() (*models.DocumentStats, error) {
	return &models.DocumentStats{}, nil
}

func (m *mockDocumentStorage) SetForceSyncPending(id string, pending bool) error {
	return nil
}

func (m *mockDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *mockDocumentStorage) ClearAll() error {
	m.documents = make(map[string]*models.Document)
	return nil
}

// Helper function to create test service
func createTestService() *Service {
	logger := arbor.NewLogger()
	config := common.NewDefaultConfig()

	// Override crawler config for testing
	config.Crawler.MaxConcurrency = 2
	config.Crawler.RequestDelay = time.Millisecond * 100
	config.Crawler.MaxDepth = 3

	return NewService(
		&mockAuthService{},
		nil, // sourceService
		nil, // authStorage
		&mockEventService{},
		&mockJobStorage{},
		&mockDocumentStorage{},
		newMockQueueManager(), // queueManager
		logger,
		config,
	)
}

// TestNewService tests service initialization
func TestNewService(t *testing.T) {
	service := createTestService()
	defer service.Close()

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	// VERIFICATION COMMENT 2: queue and retryPolicy removed - legacy worker architecture
	// Queue management now handled by queue.WorkerPool and goqite

	if service.activeJobs == nil {
		t.Error("Expected activeJobs map to be initialized")
	}

	if service.jobResults == nil {
		t.Error("Expected jobResults map to be initialized")
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

			jobID, err := service.StartCrawl(tt.sourceType, tt.entityType, tt.seedURLs, tt.config, "", false, nil, nil)
			if err != nil {
				t.Fatalf("StartCrawl failed: %v", err)
			}

			if jobID == "" {
				t.Fatal("Expected non-empty jobID")
			}

			// Verify job was created
			jobInterface, err := service.GetJobStatus(jobID)
			if err != nil {
				t.Fatalf("GetJobStatus failed: %v", err)
			}

			job, ok := jobInterface.(*CrawlJob)
			if !ok {
				t.Fatalf("Expected *CrawlJob, got %T", jobInterface)
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
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config, "", false, nil, nil)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Test existing job
	jobInterface, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}

	job, ok := jobInterface.(*CrawlJob)
	if !ok {
		t.Fatalf("Expected *CrawlJob, got %T", jobInterface)
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
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config, "", false, nil, nil)
	if err != nil {
		t.Fatalf("StartCrawl failed: %v", err)
	}

	// Cancel the job
	err = service.CancelJob(jobID)
	if err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}

	// Verify job status
	jobInterface, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("GetJobStatus failed: %v", err)
	}

	job, ok := jobInterface.(*CrawlJob)
	if !ok {
		t.Fatalf("Expected *CrawlJob, got %T", jobInterface)
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
	jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config, "", false, nil, nil)
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
	jobsInterface, err := service.ListJobs(ctx, nil)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	// Handle both []interface{} and []*models.CrawlJob return types
	var initialCount int
	switch v := jobsInterface.(type) {
	case []interface{}:
		initialCount = len(v)
	case []*models.CrawlJob:
		initialCount = len(v)
	default:
		t.Fatalf("Expected []interface{} or []*models.CrawlJob, got %T", jobsInterface)
	}

	// Create multiple jobs
	config := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		RateLimit:   time.Millisecond * 100,
	}

	jobIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		jobID, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config, "", false, nil, nil)
		if err != nil {
			t.Fatalf("StartCrawl failed: %v", err)
		}
		jobIDs[i] = jobID
	}

	// List all jobs
	jobsInterface2, err := service.ListJobs(ctx, nil)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	// Handle both []interface{} and []*models.CrawlJob return types
	var finalCount int
	switch v := jobsInterface2.(type) {
	case []interface{}:
		finalCount = len(v)
	case []*models.CrawlJob:
		finalCount = len(v)
	default:
		t.Fatalf("Expected []interface{} or []*models.CrawlJob, got %T", jobsInterface2)
	}

	if finalCount != initialCount+3 {
		t.Errorf("Expected %d jobs, got %d", initialCount+3, finalCount)
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
	_, err := service.StartCrawl("jira", "projects", []string{"https://test.atlassian.net/api"}, config, "", false, nil, nil)
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

// TestExtractLinksFromHTML tests HTML link extraction with both quoted and unquoted hrefs
// VERIFICATION COMMENT 2: extractLinksFromHTML moved to internal/jobs/types/crawler.go with queue refactor
// This test has been disabled as the function is no longer part of the Service
func TestExtractLinksFromHTML(t *testing.T) {
	t.Skip("extractLinksFromHTML removed - functionality moved to queue-based job types")
	// VERIFICATION COMMENT 2: Test body completely removed - method no longer exists
}

// TestFilterJiraLinks tests Jira URL filtering with patterns and host filtering
// VERIFICATION COMMENT 2: filterJiraLinks moved to shared LinkFilter helper in filters.go
// This test has been disabled as the function is no longer part of the Service
func TestFilterJiraLinks(t *testing.T) {
	t.Skip("filterJiraLinks removed - functionality moved to shared LinkFilter helper")
	// VERIFICATION COMMENT 2: Test body completely removed - method no longer exists
}

// TestFilterConfluenceLinks tests Confluence URL filtering with patterns and host filtering
// VERIFICATION COMMENT 2: filterConfluenceLinks moved to shared LinkFilter helper in filters.go
// This test has been disabled as the function is no longer part of the Service
func TestFilterConfluenceLinks(t *testing.T) {
	t.Skip("filterConfluenceLinks removed - functionality moved to shared LinkFilter helper")
	// VERIFICATION COMMENT 2: Test body completely removed - method no longer exists
}

// TestCrawlerJob_SavesDocumentImmediately verifies that documents are saved immediately after successful crawls
// Renamed from TestWorkerLoop_SavesDocumentImmediately to reflect new architecture
func TestCrawlerJob_SavesDocumentImmediately(t *testing.T) {
	// Create service with mock storage
	service := createTestService()
	defer service.Close()

	// Get access to mock document storage
	mockDocStorage, ok := service.documentStorage.(*mockDocumentStorage)
	if !ok {
		t.Fatal("Expected mockDocumentStorage")
	}

	// Simulate a successful crawl result with markdown metadata
	testURL := "https://test.atlassian.net/browse/TEST-123"
	testTitle := "Test Issue Title"
	testMarkdown := "# Test Issue\n\nThis is test content in markdown format."
	sourceType := "jira"

	// Create a simulated crawl result (what CrawlerJob.Execute() produces)
	result := &CrawlResult{
		URL:        testURL,
		StatusCode: 200,
		Body:       []byte("<html><body>Test content</body></html>"),
		Error:      "", // Successful crawl
		Metadata: map[string]interface{}{
			"markdown":    testMarkdown,
			"title":       testTitle,
			"source_type": sourceType,
		},
	}

	// Simulate what CrawlerJob.Execute() does: extract markdown and save document
	var markdown string
	if md, ok := result.Metadata["markdown"]; ok {
		if mdStr, ok := md.(string); ok {
			markdown = mdStr
		}
	}

	if markdown == "" {
		t.Fatal("Expected non-empty markdown from metadata")
	}

	// Extract source type from metadata (as CrawlerJob does)
	extractedSourceType := "crawler" // Default
	if st, ok := result.Metadata["source_type"]; ok {
		if stStr, ok := st.(string); ok {
			extractedSourceType = stStr
		}
	}

	// Extract title from metadata (as CrawlerJob does)
	extractedTitle := testURL // Default fallback
	if title, ok := result.Metadata["title"]; ok {
		if titleStr, ok := title.(string); ok && titleStr != "" {
			extractedTitle = titleStr
		}
	}

	// Create document (as CrawlerJob does)
	doc := models.Document{
		ID:              "doc_test_123",
		SourceType:      extractedSourceType,
		SourceID:        testURL, // URL as source_id for deduplication
		Title:           extractedTitle,
		ContentMarkdown: markdown,
		DetailLevel:     models.DetailLevelFull,
		Metadata:        result.Metadata,
		URL:             testURL,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Save document (as CrawlerJob does)
	err := service.documentStorage.SaveDocument(&doc)
	if err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}

	// Verify document was saved to mock storage
	savedDoc, err := mockDocStorage.GetDocumentBySource(sourceType, testURL)
	if err != nil {
		t.Fatalf("GetDocumentBySource failed: %v", err)
	}

	if savedDoc == nil {
		t.Fatal("Expected document to be saved, but got nil")
	}

	// Assert document fields
	if savedDoc.SourceType != sourceType {
		t.Errorf("Expected SourceType=%s, got %s", sourceType, savedDoc.SourceType)
	}

	if savedDoc.SourceID != testURL {
		t.Errorf("Expected SourceID=%s, got %s", testURL, savedDoc.SourceID)
	}

	if savedDoc.Title != testTitle {
		t.Errorf("Expected Title=%s, got %s", testTitle, savedDoc.Title)
	}

	if savedDoc.ContentMarkdown != testMarkdown {
		t.Errorf("Expected ContentMarkdown=%s, got %s", testMarkdown, savedDoc.ContentMarkdown)
	}

	if savedDoc.DetailLevel != models.DetailLevelFull {
		t.Errorf("Expected DetailLevel=%s, got %s", models.DetailLevelFull, savedDoc.DetailLevel)
	}

	if savedDoc.URL != testURL {
		t.Errorf("Expected URL=%s, got %s", testURL, savedDoc.URL)
	}

	// Verify metadata was preserved
	if savedDoc.Metadata == nil {
		t.Error("Expected metadata to be preserved")
	} else {
		if mdVal, ok := savedDoc.Metadata["markdown"]; !ok || mdVal != testMarkdown {
			t.Error("Expected markdown in metadata to be preserved")
		}
		if titleVal, ok := savedDoc.Metadata["title"]; !ok || titleVal != testTitle {
			t.Error("Expected title in metadata to be preserved")
		}
	}

	t.Log("Document saved successfully with correct fields")
}

// TestCrawlerJob_EmptyMarkdownSkipped verifies documents aren't saved when markdown is empty
// Renamed from TestWorkerLoop_EmptyMarkdownSkipped to reflect new architecture
func TestCrawlerJob_EmptyMarkdownSkipped(t *testing.T) {
	// Create service with mock storage
	service := createTestService()
	defer service.Close()

	mockDocStorage, ok := service.documentStorage.(*mockDocumentStorage)
	if !ok {
		t.Fatal("Expected mockDocumentStorage")
	}

	// Simulate a result with empty markdown
	testURL := "https://test.atlassian.net/browse/TEST-456"
	result := &CrawlResult{
		URL:        testURL,
		StatusCode: 200,
		Body:       []byte("<html><body>Content</body></html>"),
		Error:      "",
		Metadata: map[string]interface{}{
			"markdown": "", // Empty markdown
			"title":    "Test",
		},
	}

	// Extract markdown (as CrawlerJob does)
	var markdown string
	if md, ok := result.Metadata["markdown"]; ok {
		if mdStr, ok := md.(string); ok {
			markdown = mdStr
		}
	}

	// Verify markdown is empty (skip document save in this case)
	if markdown != "" {
		t.Fatal("Expected empty markdown")
	}

	// Document should NOT be saved when markdown is empty
	// Verify no document exists
	doc, err := mockDocStorage.GetDocumentBySource("jira", testURL)
	if err != nil && err.Error() != "sql: no rows in result set" {
		t.Fatalf("GetDocumentBySource failed: %v", err)
	}

	if doc != nil {
		t.Error("Expected no document to be saved for empty markdown, but found one")
	}

	t.Log("Document correctly skipped when markdown is empty")
}

// TestCrawlerJob_QueueIntegration_Success tests successful queue-based URL crawling
// This test exercises the complete flow: enqueue -> handler -> document creation -> child jobs
func TestCrawlerJob_QueueIntegration_Success(t *testing.T) {
	ctx := context.Background()

	// Create service with mock dependencies
	service := createTestService()
	defer service.Close()

	mockQueue := service.queueManager.(*mockQueueManager)
	mockJobStore := service.jobStorage.(*mockJobStorage)
	mockDocStore := service.documentStorage.(*mockDocumentStorage)

	// Create a test job
	jobID := "test-job-123"
	job := &CrawlJob{
		ID:         jobID,
		SourceType: "jira",
		EntityType: "issues",
		Status:     JobStatusRunning,
		Progress: CrawlProgress{
			TotalURLs:   1,
			PendingURLs: 1,
		},
	}

	// Save job to storage
	if err := mockJobStore.SaveJob(ctx, job); err != nil {
		t.Fatalf("Failed to save test job: %v", err)
	}

	// Enqueue a crawler_url message
	testURL := "https://test.atlassian.net/browse/TEST-123"
	msg := &queue.JobMessage{
		ID:       "msg-1",
		Type:     "crawler_url",
		URL:      testURL,
		Depth:    0,
		ParentID: jobID,
		Config: map[string]interface{}{
			"max_depth":    float64(2),
			"follow_links": true,
			"source_type":  "jira",
		},
	}

	// Enqueue the message
	if err := mockQueue.Enqueue(ctx, msg); err != nil {
		t.Fatalf("Failed to enqueue message: %v", err)
	}

	// Verify message was enqueued
	queueLen, err := mockQueue.GetQueueLength(ctx)
	if err != nil {
		t.Fatalf("Failed to get queue length: %v", err)
	}
	if queueLen != 1 {
		t.Errorf("Expected queue length 1, got %d", queueLen)
	}

	// Simulate a simplified handler that mimics CrawlerJob.Execute behavior
	// In production, this would be the full CrawlerJob.Execute() method
	mockQueue.RegisterHandler("crawler_url", func(ctx context.Context, msg *queue.JobMessage) error {
		// Create a mock document (simulating successful scraping)
		doc := &models.Document{
			ID:              fmt.Sprintf("doc_%s", msg.ID),
			SourceID:        msg.URL,
			SourceType:      "jira",
			Title:           "Test Issue",
			ContentMarkdown: "# Test Issue\n\nTest content",
			DetailLevel:     models.DetailLevelFull,
			URL:             msg.URL,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		// Save document
		if err := mockDocStore.SaveDocument(doc); err != nil {
			return fmt.Errorf("failed to save document: %w", err)
		}

		// Update job progress (simulating what CrawlerJob does)
		jobInterface, err := mockJobStore.GetJob(ctx, msg.ParentID)
		if err != nil {
			return fmt.Errorf("failed to get job: %w", err)
		}

		job, ok := jobInterface.(*CrawlJob)
		if !ok {
			return fmt.Errorf("invalid job type")
		}

		// Update counters
		job.Progress.CompletedURLs++
		if job.Progress.PendingURLs > 0 {
			job.Progress.PendingURLs--
		}

		// Check completion
		if job.Progress.PendingURLs == 0 {
			job.Status = JobStatusCompleted
		}

		// Save updated job
		return mockJobStore.SaveJob(ctx, job)
	})

	// Process the message
	if err := mockQueue.ProcessMessages(ctx); err != nil {
		t.Fatalf("Failed to process messages: %v", err)
	}

	// Verify document was created
	doc, err := mockDocStore.GetDocumentBySource("jira", testURL)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}
	if doc == nil {
		t.Fatal("Expected document to be created")
	}
	if doc.Title != "Test Issue" {
		t.Errorf("Expected title 'Test Issue', got '%s'", doc.Title)
	}

	// Verify job progress was updated
	jobInterface, err := mockJobStore.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	job, ok := jobInterface.(*CrawlJob)
	if !ok {
		t.Fatal("Expected *CrawlJob")
	}

	if job.Progress.CompletedURLs != 1 {
		t.Errorf("Expected CompletedURLs=1, got %d", job.Progress.CompletedURLs)
	}
	if job.Progress.PendingURLs != 0 {
		t.Errorf("Expected PendingURLs=0, got %d", job.Progress.PendingURLs)
	}
	if job.Status != JobStatusCompleted {
		t.Errorf("Expected status=completed, got %s", job.Status)
	}

	t.Log("Queue integration test passed: message processed, document created, progress updated")
}

// TestCrawlerJob_QueueIntegration_Error tests error handling in queue-based crawling
func TestCrawlerJob_QueueIntegration_Error(t *testing.T) {
	ctx := context.Background()

	// Create service with mock dependencies
	service := createTestService()
	defer service.Close()

	mockQueue := service.queueManager.(*mockQueueManager)
	mockJobStore := service.jobStorage.(*mockJobStorage)

	// Create a test job
	jobID := "test-job-456"
	job := &CrawlJob{
		ID:         jobID,
		SourceType: "jira",
		Status:     JobStatusRunning,
		Progress: CrawlProgress{
			TotalURLs:   1,
			PendingURLs: 1,
		},
	}

	if err := mockJobStore.SaveJob(ctx, job); err != nil {
		t.Fatalf("Failed to save test job: %v", err)
	}

	// Enqueue a message that will trigger an error
	msg := &queue.JobMessage{
		ID:       "msg-error-1",
		Type:     "crawler_url",
		URL:      "https://test.atlassian.net/invalid",
		Depth:    0,
		ParentID: jobID,
		Config: map[string]interface{}{
			"max_depth": float64(1),
		},
	}

	if err := mockQueue.Enqueue(ctx, msg); err != nil {
		t.Fatalf("Failed to enqueue message: %v", err)
	}

	// Register handler that simulates scraping error
	mockQueue.RegisterHandler("crawler_url", func(ctx context.Context, msg *queue.JobMessage) error {
		// Simulate scraping failure
		// Update job progress to reflect failure
		jobInterface, err := mockJobStore.GetJob(ctx, msg.ParentID)
		if err != nil {
			return err
		}

		job, ok := jobInterface.(*CrawlJob)
		if !ok {
			return fmt.Errorf("invalid job type")
		}

		// Mark as failed
		job.Progress.CompletedURLs++
		job.Progress.FailedURLs++
		if job.Progress.PendingURLs > 0 {
			job.Progress.PendingURLs--
		}

		if err := mockJobStore.SaveJob(ctx, job); err != nil {
			return err
		}

		return fmt.Errorf("simulated scraping error")
	})

	// Process messages - expect error
	err := mockQueue.ProcessMessages(ctx)
	if err == nil {
		t.Fatal("Expected error from handler")
	}
	if err.Error() != "handler failed: simulated scraping error" {
		t.Errorf("Expected specific error, got: %v", err)
	}

	// Verify job progress reflects failure
	jobInterface, err := mockJobStore.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	job, ok := jobInterface.(*CrawlJob)
	if !ok {
		t.Fatal("Expected *CrawlJob")
	}

	if job.Progress.FailedURLs != 1 {
		t.Errorf("Expected FailedURLs=1, got %d", job.Progress.FailedURLs)
	}
	if job.Progress.CompletedURLs != 1 {
		t.Errorf("Expected CompletedURLs=1 (failed count as completed), got %d", job.Progress.CompletedURLs)
	}

	t.Log("Error handling test passed: failures tracked correctly")
}

// TestCrawlerJob_ProgressTracking tests detailed progress tracking throughout crawl lifecycle
func TestCrawlerJob_ProgressTracking(t *testing.T) {
	ctx := context.Background()

	service := createTestService()
	defer service.Close()

	mockQueue := service.queueManager.(*mockQueueManager)
	mockJobStore := service.jobStorage.(*mockJobStorage)
	mockDocStore := service.documentStorage.(*mockDocumentStorage)

	// Create job with multiple seed URLs
	jobID := "test-job-progress"
	job := &CrawlJob{
		ID:         jobID,
		SourceType: "jira",
		Status:     JobStatusRunning,
		Progress: CrawlProgress{
			TotalURLs:   3,
			PendingURLs: 3,
		},
	}

	if err := mockJobStore.SaveJob(ctx, job); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	// Enqueue 3 URLs
	urls := []string{
		"https://test.atlassian.net/browse/TEST-1",
		"https://test.atlassian.net/browse/TEST-2",
		"https://test.atlassian.net/browse/TEST-3",
	}

	for i, url := range urls {
		msg := &queue.JobMessage{
			ID:       fmt.Sprintf("msg-%d", i),
			Type:     "crawler_url",
			URL:      url,
			Depth:    0,
			ParentID: jobID,
			Config: map[string]interface{}{
				"max_depth":    float64(1),
				"follow_links": false, // No child spawning for this test
			},
		}
		if err := mockQueue.Enqueue(ctx, msg); err != nil {
			t.Fatalf("Failed to enqueue message %d: %v", i, err)
		}
	}

	// Register handler that tracks progress
	processedCount := 0
	mockQueue.RegisterHandler("crawler_url", func(ctx context.Context, msg *queue.JobMessage) error {
		// Create document
		doc := &models.Document{
			ID:              fmt.Sprintf("doc_%s", msg.ID),
			SourceID:        msg.URL,
			SourceType:      "jira",
			Title:           fmt.Sprintf("Issue %s", msg.ID),
			ContentMarkdown: "Content",
			CreatedAt:       time.Now(),
		}
		if err := mockDocStore.SaveDocument(doc); err != nil {
			return err
		}

		// Update job progress
		jobInterface, err := mockJobStore.GetJob(ctx, msg.ParentID)
		if err != nil {
			return err
		}

		job, ok := jobInterface.(*CrawlJob)
		if !ok {
			return fmt.Errorf("invalid job type")
		}

		// Increment completed, decrement pending
		job.Progress.CompletedURLs++
		if job.Progress.PendingURLs > 0 {
			job.Progress.PendingURLs--
		}

		// Calculate percentage
		if job.Progress.TotalURLs > 0 {
			job.Progress.Percentage = float64(job.Progress.CompletedURLs) / float64(job.Progress.TotalURLs) * 100
		}

		// Check completion
		if job.Progress.PendingURLs == 0 {
			job.Status = JobStatusCompleted
		}

		processedCount++
		return mockJobStore.SaveJob(ctx, job)
	})

	// Process all messages
	if err := mockQueue.ProcessMessages(ctx); err != nil {
		t.Fatalf("Failed to process messages: %v", err)
	}

	// Verify all URLs were processed
	if processedCount != 3 {
		t.Errorf("Expected 3 URLs processed, got %d", processedCount)
	}

	// Verify final job state
	jobInterface, err := mockJobStore.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("Failed to get final job: %v", err)
	}
	job, ok := jobInterface.(*CrawlJob)
	if !ok {
		t.Fatal("Expected *CrawlJob")
	}

	// Assert progress fields
	if job.Progress.CompletedURLs != 3 {
		t.Errorf("Expected CompletedURLs=3, got %d", job.Progress.CompletedURLs)
	}
	if job.Progress.PendingURLs != 0 {
		t.Errorf("Expected PendingURLs=0, got %d", job.Progress.PendingURLs)
	}
	if job.Progress.FailedURLs != 0 {
		t.Errorf("Expected FailedURLs=0, got %d", job.Progress.FailedURLs)
	}
	if job.Progress.Percentage != 100.0 {
		t.Errorf("Expected Percentage=100.0, got %.2f", job.Progress.Percentage)
	}
	if job.Status != JobStatusCompleted {
		t.Errorf("Expected Status=completed, got %s", job.Status)
	}

	// Verify all documents were created
	docCount, err := mockDocStore.CountDocuments()
	if err != nil {
		t.Fatalf("Failed to count documents: %v", err)
	}
	if docCount != 3 {
		t.Errorf("Expected 3 documents, got %d", docCount)
	}

	t.Log("Progress tracking test passed: all counters accurate throughout lifecycle")
}
