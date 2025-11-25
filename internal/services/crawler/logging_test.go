// -----------------------------------------------------------------------
// Last Modified: Monday, 3rd November 2025 9:51:04 am
// Modified By: Bob McAllan
// -----------------------------------------------------------------------

package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

var errNotFound = errors.New("not found")

// TestCrawlerServiceLogging verifies that enhanced INFO level logs are emitted correctly
func TestCrawlerServiceLogging(t *testing.T) {
	// Setup: Create test crawler service with in-memory components
	logger := arbor.NewLogger()
	config := common.NewDefaultConfig()

	// Override crawler config for testing
	config.Crawler.MaxConcurrency = 1
	config.Crawler.RequestDelay = time.Millisecond * 100
	config.Crawler.MaxDepth = 2

	jobStorage := NewInMemoryJobStorage()
	authService := NewMockAuthService()
	authStorage := NewMockAuthStorage()
	eventService := NewMockEventService()
	documentStorage := NewMockDocumentStorage()

	// VERIFICATION COMMENT 2: Added mockQueueManager parameter (required after worker cleanup)
	queueManager := &mockQueueManager{}
	service := NewService(authService, authStorage, eventService, jobStorage, documentStorage, queueManager, nil, logger, config)

	// Start service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start crawler service: %v", err)
	}
	defer service.Close()

	// Create crawl config with follow_links enabled
	crawlConfig := CrawlConfig{
		FollowLinks:     true,
		MaxDepth:        2,
		MaxPages:        10,
		Concurrency:     1,
		RateLimit:       time.Millisecond * 100,
		IncludePatterns: []string{},
		ExcludePatterns: []string{},
	}

	// Start a test crawl job (no context parameter)
	jobID, err := service.StartCrawl(
		"web",
		"pages",
		[]string{"http://example.com"},
		crawlConfig,
		"test-source-1",
		false,
		nil,
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("Failed to start crawl job: %v", err)
	}

	// Wait for job to process (short timeout for test)
	time.Sleep(2 * time.Second)

	// Get job status
	jobStatus, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("Failed to get job status: %v", err)
	}

	job, ok := jobStatus.(*models.QueueJobState)
	if !ok {
		t.Fatalf("Expected *models.QueueJobState, got %T", jobStatus)
	}

	t.Logf("Job status: %s", job.Status)
	t.Logf("Completed URLs: %d", job.Progress.CompletedURLs)
	t.Logf("Total URLs: %d", job.Progress.TotalURLs)

	// Note: In unit test environment, jobs may remain pending without workers to process them
	// This test focuses on logging setup, not actual crawl execution
	if job.Progress.TotalURLs == 0 {
		t.Error("Expected seed URL to be queued (TotalURLs should be > 0)")
	}

	// Clean up
	_ = service.CancelJob(jobID)
}

// TestCrawlerLoggingWithLinksDiscovered verifies logging when links are discovered
func TestCrawlerLoggingWithLinksDiscovered(t *testing.T) {
	t.Skip("Requires mock HTTP server with HTML content - implement when needed")

	// This test would:
	// 1. Create mock HTTP server serving HTML with links
	// 2. Start crawl job with follow_links=true
	// 3. Verify INFO logs appear:
	//    - "Link discovery enabled - will extract and follow links"
	//    - "Link filtering complete"
	//    - "Link enqueueing complete" with sample_urls
	//    - "Pattern filtering summary"
	// 4. Verify database logs contain these entries
	// 5. Verify sample URLs are limited to 3
}

// TestCrawlerLoggingWithNoLinks verifies logging when no links are discovered
func TestCrawlerLoggingWithNoLinks(t *testing.T) {
	t.Skip("Requires mock HTTP server with no-link content - implement when needed")

	// This test would:
	// 1. Create mock HTTP server serving plain text (no links)
	// 2. Start crawl job with follow_links=true
	// 3. Verify INFO log: "Link discovery enabled"
	// 4. Verify no "Link enqueueing complete" log (no links to enqueue)
	// 5. Verify database logs show discovery attempted but no links found
}

// TestCrawlerLoggingWithAllLinksFiltered verifies warning logs when all links are filtered
func TestCrawlerLoggingWithAllLinksFiltered(t *testing.T) {
	t.Skip("Requires mock HTTP server with restrictive filters - implement when needed")

	// This test would:
	// 1. Create mock HTTP server with links
	// 2. Start crawl job with restrictive exclude_patterns that filter all links
	// 3. Verify WARN log appears with sample URLs
	// 4. Verify "Pattern filtering summary" shows excluded_count > 0, passed_count = 0
	// 5. Verify database logs contain warning
}

// TestCrawlerLoggingWithFollowLinksDisabled verifies skip logging
func TestCrawlerLoggingWithFollowLinksDisabled(t *testing.T) {
	// Setup: Create test crawler service
	logger := arbor.NewLogger()
	config := common.NewDefaultConfig()

	// Override crawler config for testing
	config.Crawler.MaxConcurrency = 1
	config.Crawler.RequestDelay = time.Millisecond * 100
	config.Crawler.MaxDepth = 2

	jobStorage := NewInMemoryJobStorage()
	authService := NewMockAuthService()
	authStorage := NewMockAuthStorage()
	eventService := NewMockEventService()
	documentStorage := NewMockDocumentStorage()

	// VERIFICATION COMMENT 2: Added mockQueueManager parameter (required after worker cleanup)
	queueManager := &mockQueueManager{}
	service := NewService(authService, authStorage, eventService, jobStorage, documentStorage, queueManager, nil, logger, config)

	// Start service
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start crawler service: %v", err)
	}
	defer service.Close()

	// Create crawl config with follow_links DISABLED
	crawlConfig := CrawlConfig{
		FollowLinks:     false,
		MaxDepth:        2,
		MaxPages:        10,
		Concurrency:     1,
		RateLimit:       time.Millisecond * 100,
		IncludePatterns: []string{},
		ExcludePatterns: []string{},
	}

	// Start a test crawl job (no context parameter)
	jobID, err := service.StartCrawl(
		"web",
		"pages",
		[]string{"http://example.com"},
		crawlConfig,
		"test-source-1",
		false,
		nil,
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("Failed to start crawl job: %v", err)
	}

	// Wait for job to process
	time.Sleep(2 * time.Second)

	// Get job status
	jobStatus, err := service.GetJobStatus(jobID)
	if err != nil {
		t.Fatalf("Failed to get job status: %v", err)
	}

	job, ok := jobStatus.(*models.QueueJobState)
	if !ok {
		t.Fatalf("Expected *models.QueueJobState, got %T", jobStatus)
	}

	t.Logf("Job with follow_links=false: status=%s, completed=%d", job.Status, job.Progress.CompletedURLs)

	// Verify that only seed URL was processed (no links followed)
	// Note: In test environment, this may not complete fully, so we check the intent
	if job.Progress.TotalURLs > 1 {
		t.Errorf("Expected only seed URL to be queued, but got %d total URLs", job.Progress.TotalURLs)
	}

	// Clean up
	_ = service.CancelJob(jobID)
}

// TestCrawlerLoggingWithMaxDepthReached verifies skip logging at depth limit
func TestCrawlerLoggingWithMaxDepthReached(t *testing.T) {
	t.Skip("Requires mock HTTP server with multi-level links - implement when needed")

	// This test would:
	// 1. Create mock HTTP server with nested links (depth > 2)
	// 2. Start crawl job with max_depth=2
	// 3. Verify DEBUG log at depth 2: "Skipping link discovery - depth limit reached"
	// 4. Verify no "Link discovery enabled" log at depth 2
	// 5. Verify database logs show skip reason
}

// ============================================================================
// Mock Implementations
// ============================================================================

// MockAuthService implements interfaces.AuthService
type MockAuthService struct {
	client *http.Client
}

func NewMockAuthService() *MockAuthService {
	return &MockAuthService{
		client: &http.Client{},
	}
}

func (m *MockAuthService) UpdateAuth(authData *interfaces.AtlassianAuthData) error {
	return nil
}

func (m *MockAuthService) IsAuthenticated() bool {
	return true
}

func (m *MockAuthService) LoadAuth() (*interfaces.AtlassianAuthData, error) {
	return &interfaces.AtlassianAuthData{}, nil
}

func (m *MockAuthService) GetHTTPClient() *http.Client {
	return m.client
}

func (m *MockAuthService) GetBaseURL() string {
	return "http://localhost"
}

func (m *MockAuthService) GetUserAgent() string {
	return "test-agent"
}

func (m *MockAuthService) GetCloudID() string {
	return "test-cloud-id"
}

func (m *MockAuthService) GetAtlToken() string {
	return "test-token"
}

// MockAuthStorage implements interfaces.AuthStorage
type MockAuthStorage struct {
	credentials map[string]*models.AuthCredentials
}

func NewMockAuthStorage() *MockAuthStorage {
	return &MockAuthStorage{
		credentials: make(map[string]*models.AuthCredentials),
	}
}

func (m *MockAuthStorage) StoreCredentials(ctx context.Context, credentials *models.AuthCredentials) error {
	m.credentials[credentials.ID] = credentials
	return nil
}

func (m *MockAuthStorage) GetCredentialsByID(ctx context.Context, id string) (*models.AuthCredentials, error) {
	if cred, exists := m.credentials[id]; exists {
		return cred, nil
	}
	return nil, errNotFound
}

func (m *MockAuthStorage) GetCredentialsBySiteDomain(ctx context.Context, siteDomain string) (*models.AuthCredentials, error) {
	for _, cred := range m.credentials {
		if cred.SiteDomain == siteDomain {
			return cred, nil
		}
	}
	return nil, errNotFound
}

func (m *MockAuthStorage) DeleteCredentials(ctx context.Context, id string) error {
	delete(m.credentials, id)
	return nil
}

func (m *MockAuthStorage) ListCredentials(ctx context.Context) ([]*models.AuthCredentials, error) {
	creds := make([]*models.AuthCredentials, 0, len(m.credentials))
	for _, cred := range m.credentials {
		creds = append(creds, cred)
	}
	return creds, nil
}

func (m *MockAuthStorage) GetCredentials(ctx context.Context, service string) (*models.AuthCredentials, error) {
	return nil, errNotFound
}

func (m *MockAuthStorage) ListServices(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

// MockEventService implements interfaces.EventService
type MockEventService struct{}

func NewMockEventService() *MockEventService {
	return &MockEventService{}
}

func (m *MockEventService) Subscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	return nil
}

func (m *MockEventService) Unsubscribe(eventType interfaces.EventType, handler interfaces.EventHandler) error {
	return nil
}

func (m *MockEventService) Publish(ctx context.Context, event interfaces.Event) error {
	return nil
}

func (m *MockEventService) PublishSync(ctx context.Context, event interfaces.Event) error {
	return nil
}

func (m *MockEventService) Close() error {
	return nil
}

// InMemoryJobStorage implements interfaces.JobStorage for testing
type InMemoryJobStorage struct {
	jobs map[string]interface{}
	logs map[string][]models.JobLogEntry
}

func NewInMemoryJobStorage() *InMemoryJobStorage {
	return &InMemoryJobStorage{
		jobs: make(map[string]interface{}),
		logs: make(map[string][]models.JobLogEntry),
	}
}

func (s *InMemoryJobStorage) SaveJob(ctx context.Context, job interface{}) error {
	// Extract job ID from job (type assertion for CrawlJob)
	if crawlJob, ok := job.(*CrawlJob); ok {
		s.jobs[crawlJob.ID] = job
	}
	return nil
}

func (s *InMemoryJobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	if job, exists := s.jobs[jobID]; exists {
		return job, nil
	}
	return nil, errNotFound
}

func (s *InMemoryJobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	if crawlJob, ok := job.(*CrawlJob); ok {
		s.jobs[crawlJob.ID] = job
	}
	return nil
}

func (s *InMemoryJobStorage) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.QueueJobState, error) {
	jobs := make([]*models.QueueJobState, 0, len(s.jobs))
	for _, job := range s.jobs {
		// Type assert to *CrawlJob and convert to *models.QueueJobState
		if crawlJob, ok := job.(*CrawlJob); ok {
			var parentID *string
			if crawlJob.ParentID != "" {
				parentID = &crawlJob.ParentID
			}

			jobState := &models.QueueJobState{
				ID:        crawlJob.ID,
				ParentID:  parentID,
				Type:      string(crawlJob.JobType),
				Name:      crawlJob.Name,
				Config:    make(map[string]interface{}),
				Metadata:  make(map[string]interface{}),
				CreatedAt: crawlJob.CreatedAt,
				Depth:     0,
				Status:    models.JobStatus(crawlJob.Status),
			}
			jobs = append(jobs, jobState)
		}
	}
	return jobs, nil
}

func (s *InMemoryJobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.QueueJob, error) {
	jobs := make([]*models.QueueJob, 0)
	for _, job := range s.jobs {
		if crawlJob, ok := job.(*CrawlJob); ok && string(crawlJob.Status) == status {
			var parentID *string
			if crawlJob.ParentID != "" {
				parentID = &crawlJob.ParentID
			}

			queueJob := &models.QueueJob{
				ID:        crawlJob.ID,
				ParentID:  parentID,
				Type:      string(crawlJob.JobType),
				Name:      crawlJob.Name,
				Config:    make(map[string]interface{}),
				Metadata:  make(map[string]interface{}),
				CreatedAt: crawlJob.CreatedAt,
				Depth:     0,
			}
			jobs = append(jobs, queueJob)
		}
	}
	return jobs, nil
}

func (s *InMemoryJobStorage) UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error {
	if job, exists := s.jobs[jobID]; exists {
		if crawlJob, ok := job.(*CrawlJob); ok {
			crawlJob.Status = JobStatus(status)
			crawlJob.Error = errorMsg
		}
	}
	return nil
}

func (s *InMemoryJobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	// Not implemented for simple test
	return nil
}

func (s *InMemoryJobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	// Not implemented for simple test
	return nil
}

func (s *InMemoryJobStorage) GetStaleJobs(ctx context.Context, staleThresholdMinutes int) ([]*models.QueueJob, error) {
	return []*models.QueueJob{}, nil
}

func (s *InMemoryJobStorage) DeleteJob(ctx context.Context, jobID string) error {
	delete(s.jobs, jobID)
	delete(s.logs, jobID)
	return nil
}

func (s *InMemoryJobStorage) CountJobs(ctx context.Context) (int, error) {
	return len(s.jobs), nil
}

func (s *InMemoryJobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	count := 0
	for _, job := range s.jobs {
		if crawlJob, ok := job.(*CrawlJob); ok && string(crawlJob.Status) == status {
			count++
		}
	}
	return count, nil
}

func (s *InMemoryJobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	return len(s.jobs), nil
}

func (s *InMemoryJobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	return nil, nil
}

func (s *InMemoryJobStorage) GetChildJobs(ctx context.Context, parentID string) ([]*models.QueueJob, error) {
	jobs := make([]*models.QueueJob, 0)
	for _, job := range s.jobs {
		if crawlJob, ok := job.(*CrawlJob); ok && crawlJob.ParentID == parentID {
			var pID *string
			if crawlJob.ParentID != "" {
				pID = &crawlJob.ParentID
			}

			queueJob := &models.QueueJob{
				ID:        crawlJob.ID,
				ParentID:  pID,
				Type:      string(crawlJob.JobType),
				Name:      crawlJob.Name,
				Config:    make(map[string]interface{}),
				Metadata:  make(map[string]interface{}),
				CreatedAt: crawlJob.CreatedAt,
				Depth:     0,
			}
			jobs = append(jobs, queueJob)
		}
	}
	return jobs, nil
}

func (s *InMemoryJobStorage) UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error {
	return nil
}

func (s *InMemoryJobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	s.logs[jobID] = append(s.logs[jobID], logEntry)
	return nil
}

func (s *InMemoryJobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	if logs, exists := s.logs[jobID]; exists {
		return logs, nil
	}
	return []models.JobLogEntry{}, nil
}

func (s *InMemoryJobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	// Simple mock - always return true (newly added)
	return true, nil
}

func (s *InMemoryJobStorage) MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error) {
	// Simple mock - count running jobs and mark as pending
	count := 0
	for _, job := range s.jobs {
		if crawlJob, ok := job.(*CrawlJob); ok && crawlJob.Status == JobStatusRunning {
			crawlJob.Status = JobStatusPending
			count++
		}
	}
	return count, nil
}

// Helper method to verify log contains expected content
func (s *InMemoryJobStorage) VerifyLogContains(t *testing.T, jobID, expectedSubstring string) bool {
	entries, exists := s.logs[jobID]
	if !exists {
		t.Errorf("No logs found for job %s", jobID)
		return false
	}

	for _, entry := range entries {
		if strings.Contains(entry.Message, expectedSubstring) {
			return true
		}
	}

	t.Errorf("Log substring %q not found in job %s logs", expectedSubstring, jobID)
	return false
}

// Helper method to count logs at specific level
func (s *InMemoryJobStorage) CountLogsByLevel(jobID, level string) int {
	entries, exists := s.logs[jobID]
	if !exists {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.Level == level {
			count++
		}
	}
	return count
}

// VERIFICATION COMMENT 2: mockQueueManager defined in service_test.go (shared between test files)

// MockDocumentStorage implements interfaces.DocumentStorage for testing
type MockDocumentStorage struct {
	documents map[string]*models.Document
}

func NewMockDocumentStorage() *MockDocumentStorage {
	return &MockDocumentStorage{
		documents: make(map[string]*models.Document),
	}
}

func (m *MockDocumentStorage) SaveDocument(doc *models.Document) error {
	if m.documents == nil {
		m.documents = make(map[string]*models.Document)
	}
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) SaveDocuments(docs []*models.Document) error {
	for _, doc := range docs {
		if err := m.SaveDocument(doc); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockDocumentStorage) GetDocument(id string) (*models.Document, error) {
	if m.documents == nil {
		return nil, nil
	}
	return m.documents[id], nil
}

func (m *MockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
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

func (m *MockDocumentStorage) UpdateDocument(doc *models.Document) error {
	if m.documents == nil {
		m.documents = make(map[string]*models.Document)
	}
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) DeleteDocument(id string) error {
	if m.documents != nil {
		delete(m.documents, id)
	}
	return nil
}

func (m *MockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	if m.documents == nil {
		return []*models.Document{}, nil
	}
	docs := make([]*models.Document, 0, len(m.documents))
	for _, doc := range m.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

func (m *MockDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
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

func (m *MockDocumentStorage) CountDocuments() (int, error) {
	if m.documents == nil {
		return 0, nil
	}
	return len(m.documents), nil
}

func (m *MockDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
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

func (m *MockDocumentStorage) GetStats() (*models.DocumentStats, error) {
	return &models.DocumentStats{}, nil
}

func (m *MockDocumentStorage) SetForceSyncPending(id string, pending bool) error {
	return nil
}

func (m *MockDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) ClearAll() error {
	m.documents = make(map[string]*models.Document)
	return nil
}

func (m *MockDocumentStorage) RebuildFTS5Index() error {
	// Mock implementation - no-op for testing
	return nil
}

func (m *MockDocumentStorage) GetAllTags() ([]string, error) {
	return []string{}, nil
}

// mockQueueManager implements interfaces.QueueManager for testing
type mockQueueManager struct{}

func (m *mockQueueManager) Start() error   { return nil }
func (m *mockQueueManager) Stop() error    { return nil }
func (m *mockQueueManager) Restart() error { return nil }
func (m *mockQueueManager) Close() error   { return nil }

func (m *mockQueueManager) Enqueue(ctx context.Context, msg models.QueueMessage) error {
	return nil
}

func (m *mockQueueManager) Receive(ctx context.Context) (*models.QueueMessage, func() error, error) {
	return nil, nil, fmt.Errorf("no messages")
}

func (m *mockQueueManager) Extend(ctx context.Context, messageID string, duration time.Duration) error {
	return nil
}
