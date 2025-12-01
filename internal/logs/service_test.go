package logs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// MockJobLogStorage is a mock implementation of JobLogStorage
type MockJobLogStorage struct {
	mock.Mock
}

func (m *MockJobLogStorage) AppendLog(ctx context.Context, jobID string, entry models.JobLogEntry) error {
	args := m.Called(ctx, jobID, entry)
	return args.Error(0)
}

func (m *MockJobLogStorage) AppendLogs(ctx context.Context, jobID string, entries []models.JobLogEntry) error {
	args := m.Called(ctx, jobID, entries)
	return args.Error(0)
}

func (m *MockJobLogStorage) GetLogs(ctx context.Context, jobID string, limit int) ([]models.JobLogEntry, error) {
	args := m.Called(ctx, jobID, limit)
	if logs, ok := args.Get(0).([]models.JobLogEntry); ok {
		return logs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobLogStorage) GetLogsByLevel(ctx context.Context, jobID string, level string, limit int) ([]models.JobLogEntry, error) {
	args := m.Called(ctx, jobID, level, limit)
	if logs, ok := args.Get(0).([]models.JobLogEntry); ok {
		return logs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobLogStorage) DeleteLogs(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobLogStorage) CountLogs(ctx context.Context, jobID string) (int, error) {
	args := m.Called(ctx, jobID)
	return args.Int(0), args.Error(1)
}

func (m *MockJobLogStorage) GetLogsWithOffset(ctx context.Context, jobID string, limit int, offset int) ([]models.JobLogEntry, error) {
	args := m.Called(ctx, jobID, limit, offset)
	if logs, ok := args.Get(0).([]models.JobLogEntry); ok {
		return logs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobLogStorage) GetLogsByLevelWithOffset(ctx context.Context, jobID string, level string, limit int, offset int) ([]models.JobLogEntry, error) {
	args := m.Called(ctx, jobID, level, limit, offset)
	if logs, ok := args.Get(0).([]models.JobLogEntry); ok {
		return logs, args.Error(1)
	}
	return nil, args.Error(1)
}

// MockJobStorage is a mock implementation of JobStorage
type MockJobStorage struct {
	mock.Mock
}

func (m *MockJobStorage) CreateJob(ctx context.Context, sourceType, sourceID string, config map[string]interface{}) (string, error) {
	args := m.Called(ctx, sourceType, sourceID, config)
	return args.String(0), args.Error(1)
}

func (m *MockJobStorage) GetJob(ctx context.Context, jobID string) (interface{}, error) {
	args := m.Called(ctx, jobID)
	if job, ok := args.Get(0).(*models.QueueJobState); ok {
		return job, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) UpdateJob(ctx context.Context, job interface{}) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobStorage) DeleteJob(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobStorage) ListJobs(ctx context.Context, opts *interfaces.JobListOptions) ([]*models.QueueJobState, error) {
	args := m.Called(ctx, opts)
	if jobs, ok := args.Get(0).([]*models.QueueJobState); ok {
		return jobs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) CountJobs(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockJobStorage) CountJobsByStatus(ctx context.Context, status string) (int, error) {
	args := m.Called(ctx, status)
	return args.Int(0), args.Error(1)
}

func (m *MockJobStorage) GetJobChildStats(ctx context.Context, parentIDs []string) (map[string]*interfaces.JobChildStats, error) {
	args := m.Called(ctx, parentIDs)
	if stats, ok := args.Get(0).(map[string]*interfaces.JobChildStats); ok {
		return stats, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) GetJobsByStatus(ctx context.Context, status string) ([]*models.QueueJob, error) {
	args := m.Called(ctx, status)
	if jobs, ok := args.Get(0).([]*models.QueueJob); ok {
		return jobs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) UpdateJobStatus(ctx context.Context, jobID, status, message string) error {
	args := m.Called(ctx, jobID, status, message)
	return args.Error(0)
}

func (m *MockJobStorage) GetChildJobs(ctx context.Context, parentID string) ([]*models.QueueJob, error) {
	args := m.Called(ctx, parentID)
	if jobs, ok := args.Get(0).([]*models.QueueJob); ok {
		return jobs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) GetStaleJobs(ctx context.Context, thresholdMinutes int) ([]*models.QueueJob, error) {
	args := m.Called(ctx, thresholdMinutes)
	if jobs, ok := args.Get(0).([]*models.QueueJob); ok {
		return jobs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) MarkRunningJobsAsPending(ctx context.Context, message string) (int, error) {
	args := m.Called(ctx, message)
	return args.Int(0), args.Error(1)
}

func (m *MockJobStorage) AppendJobLog(ctx context.Context, jobID string, logEntry models.JobLogEntry) error {
	args := m.Called(ctx, jobID, logEntry)
	return args.Error(0)
}

func (m *MockJobStorage) GetJobLogs(ctx context.Context, jobID string) ([]models.JobLogEntry, error) {
	args := m.Called(ctx, jobID)
	if logs, ok := args.Get(0).([]models.JobLogEntry); ok {
		return logs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) MarkURLSeen(ctx context.Context, jobID string, url string) (bool, error) {
	args := m.Called(ctx, jobID, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockJobStorage) SaveJob(ctx context.Context, job interface{}) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockJobStorage) UpdateJobProgress(ctx context.Context, jobID string, progressJSON string) error {
	args := m.Called(ctx, jobID, progressJSON)
	return args.Error(0)
}

func (m *MockJobStorage) UpdateProgressCountersAtomic(ctx context.Context, jobID string, completedDelta, pendingDelta, totalDelta, failedDelta int) error {
	args := m.Called(ctx, jobID, completedDelta, pendingDelta, totalDelta, failedDelta)
	return args.Error(0)
}

func (m *MockJobStorage) UpdateJobHeartbeat(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockJobStorage) CountJobsWithFilters(ctx context.Context, opts *interfaces.JobListOptions) (int, error) {
	args := m.Called(ctx, opts)
	return args.Int(0), args.Error(1)
}

func (m *MockJobStorage) GetStepStats(ctx context.Context, managerID string) (*interfaces.StepStats, error) {
	args := m.Called(ctx, managerID)
	if stats, ok := args.Get(0).(*interfaces.StepStats); ok {
		return stats, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) ListStepJobs(ctx context.Context, managerID string) ([]*models.QueueJob, error) {
	args := m.Called(ctx, managerID)
	if jobs, ok := args.Get(0).([]*models.QueueJob); ok {
		return jobs, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockJobStorage) IncrementDocumentCountAtomic(ctx context.Context, jobID string) (int, error) {
	args := m.Called(ctx, jobID)
	return args.Int(0), args.Error(1)
}

func TestService_GetAggregatedLogs_ParentOnly(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Parent job with logs
	parentJob := &models.QueueJobState{
		ID:   "parent-123",
		Name: "Parent Job",
		Type: "crawler",
		Config: map[string]interface{}{
			"max_depth": 3,
			"seed_urls": []interface{}{"https://example.com"},
		},
	}

	parentLogs := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:10:00", FullTimestamp: "2025-01-01T10:10:00Z", Level: "info", Message: "Job completed"},
		{Timestamp: "10:05:00", FullTimestamp: "2025-01-01T10:05:00Z", Level: "info", Message: "Processing page 1"},
		{Timestamp: "10:00:00", FullTimestamp: "2025-01-01T10:00:00Z", Level: "info", Message: "Job started"},
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	// Budgeted batch size for 1 job with limit 1000: (1000 + 1 - 1) / 1 = 1000
	mockLogStorage.On("GetLogsWithOffset", ctx, "parent-123", 1000, 0).Return(parentLogs, nil)

	// Execute
	logs, metadata, _, err := service.GetAggregatedLogs(ctx, "parent-123", false, "all", 1000, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 3)
	assert.Equal(t, "10:00:00", logs[0].Timestamp)
	assert.Equal(t, "10:10:00", logs[2].Timestamp)

	// Check metadata
	assert.Contains(t, metadata, "parent-123")
	meta := metadata["parent-123"]
	assert.Equal(t, "Parent Job", meta.JobName)
	assert.Equal(t, "https://example.com", meta.JobURL)
	assert.Equal(t, 3, meta.JobDepth)
	assert.Equal(t, "crawler", meta.JobType)
	assert.Equal(t, "", meta.ParentID)

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_WithChildren(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Parent job with child jobs
	parentID := "parent-123"
	parentJob := &models.QueueJobState{
		ID:   parentID,
		Name: "Parent Job",
		Type: "crawler",
		Config: map[string]interface{}{
			"max_depth": 3,
			"seed_urls": []interface{}{"https://example.com"},
		},
	}

	childJobs := []*models.QueueJob{
		{
			ID:       "child-456",
			ParentID: &parentID,
			Name:     "Child Job 1",
			Type:     "crawler",
			Config: map[string]interface{}{
				"max_depth": 1,
				"seed_urls": []interface{}{"https://example.com/page1"},
			},
		},
		{
			ID:       "child-789",
			ParentID: &parentID,
			Name:     "Child Job 2",
			Type:     "crawler",
			Config: map[string]interface{}{
				"max_depth": 1,
				"seed_urls": []interface{}{"https://example.com/page2"},
			},
		},
	}

	parentLogs := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:30:00", FullTimestamp: "2025-01-01T10:30:00Z", Level: "info", Message: "Job completed"},
		{Timestamp: "10:00:00", FullTimestamp: "2025-01-01T10:00:00Z", Level: "info", Message: "Job started"},
	}

	childLogs1 := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:15:00", FullTimestamp: "2025-01-01T10:15:00Z", Level: "info", Message: "Child 1 completed"},
		{Timestamp: "10:10:00", FullTimestamp: "2025-01-01T10:10:00Z", Level: "info", Message: "Child 1 processing"},
		{Timestamp: "10:05:00", FullTimestamp: "2025-01-01T10:05:00Z", Level: "info", Message: "Child 1 started"},
	}

	childLogs2 := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:18:00", FullTimestamp: "2025-01-01T10:18:00Z", Level: "info", Message: "Child 2 completed"},
		{Timestamp: "10:12:00", FullTimestamp: "2025-01-01T10:12:00Z", Level: "info", Message: "Child 2 processing"},
		{Timestamp: "10:08:00", FullTimestamp: "2025-01-01T10:08:00Z", Level: "info", Message: "Child 2 started"},
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	mockJobStorage.On("GetChildJobs", ctx, "parent-123").Return(childJobs, nil)
	// Budgeted batch size: (limit + numJobs - 1) / numJobs = (1000 + 3 - 1) / 3 = 334
	mockLogStorage.On("GetLogsWithOffset", ctx, "parent-123", 334, 0).Return(parentLogs, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "child-456", 334, 0).Return(childLogs1, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "child-789", 334, 0).Return(childLogs2, nil)
	// Note: Service uses already-loaded child jobs from GetChildJobs for metadata (no additional GetJob calls - Comment 7)

	// Execute
	logs, metadata, _, err := service.GetAggregatedLogs(ctx, "parent-123", true, "all", 1000, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 8) // 2 parent + 3 child1 + 3 child2

	// Logs should be sorted chronologically
	assert.Equal(t, "10:00:00", logs[0].Timestamp) // Parent start
	assert.Equal(t, "10:05:00", logs[1].Timestamp) // Child1 start
	assert.Equal(t, "10:08:00", logs[2].Timestamp) // Child2 start
	assert.Equal(t, "10:30:00", logs[7].Timestamp) // Parent end

	// Check metadata for all jobs
	assert.Contains(t, metadata, "parent-123")
	assert.Contains(t, metadata, "child-456")
	assert.Contains(t, metadata, "child-789")

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_LevelFiltering(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Parent job with mixed-level logs
	parentJob := &models.QueueJobState{
		ID:   "parent-123",
		Name: "Parent Job",
		Type: "crawler",
	}

	errorLogs := []models.JobLogEntry{
		{Timestamp: "10:01:00", FullTimestamp: "2025-01-01T10:01:00Z", Level: "error", Message: "Connection failed"},
		{Timestamp: "10:04:00", FullTimestamp: "2025-01-01T10:04:00Z", Level: "error", Message: "Another error"},
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	mockLogStorage.On("GetLogsByLevelWithOffset", ctx, "parent-123", "error", 1000, 0).Return(errorLogs, nil)

	// Execute with level filter
	logs, _, _, err := service.GetAggregatedLogs(ctx, "parent-123", false, "error", 1000, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	assert.Equal(t, "error", logs[0].Level)
	assert.Equal(t, "error", logs[1].Level)

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_LimitApplied(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Parent job with many logs
	parentJob := &models.QueueJobState{
		ID:   "parent-123",
		Name: "Parent Job",
		Type: "crawler",
	}

	// Create 15 logs in DESC order (newest first) to match storage behavior
	// Storage returns DESC order: newest (14) down to oldest (0)
	// After reversal for ASC: 0, 1, 2, ..., 14
	parentLogs := make([]models.JobLogEntry, 15)
	for i := 0; i < 15; i++ {
		// Index 0 gets newest (14), index 14 gets oldest (0)
		logIndex := 14 - i
		parentLogs[i] = models.JobLogEntry{
			Timestamp:     fmt.Sprintf("10:%02d:00", logIndex),
			FullTimestamp: fmt.Sprintf("2025-01-01T10:%02d:00Z", logIndex),
			Level:         "info",
			Message:       fmt.Sprintf("Log entry %d", logIndex),
		}
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "parent-123", 10, 0).Return(parentLogs, nil)

	// Execute with limit of 10
	logs, _, _, err := service.GetAggregatedLogs(ctx, "parent-123", false, "all", 10, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 10) // Limited to 10
	assert.Equal(t, "10:00:00", logs[0].Timestamp)
	assert.Equal(t, "10:09:00", logs[9].Timestamp)

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_JobNotFound(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Mock expectations - GetJob will fail when checking if parent exists
	// No need to mock GetLogsWithOffset since GetJob fails first
	mockJobStorage.On("GetJob", ctx, "nonexistent").Return(nil, assert.AnError)

	// Execute
	_, _, _, err := service.GetAggregatedLogs(ctx, "nonexistent", false, "all", 1000, "", "asc")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job not found")

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_ChildJobErrorContinues(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Parent job with child jobs (one will fail)
	parentID := "parent-123"
	parentJob := &models.QueueJobState{
		ID:   parentID,
		Name: "Parent Job",
		Type: "crawler",
	}

	childJobs := []*models.QueueJob{
		{
			ID:       "child-456",
			ParentID: &parentID,
			Name:     "Child Job 1",
			Type:     "crawler",
		},
		{
			ID:       "child-789",
			ParentID: &parentID,
			Name:     "Child Job 2",
			Type:     "crawler",
		},
	}

	parentLogs := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:00:00", FullTimestamp: "2025-01-01T10:00:00Z", Level: "info", Message: "Job started"},
	}

	childLogs1 := []models.JobLogEntry{
		// Storage returns DESC order (newest first) - these will be reversed for ASC order
		{Timestamp: "10:05:00", FullTimestamp: "2025-01-01T10:05:00Z", Level: "info", Message: "Child 1 started"},
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	mockJobStorage.On("GetChildJobs", ctx, "parent-123").Return(childJobs, nil)
	// Budgeted batch size: (limit + numJobs - 1) / numJobs = (1000 + 3 - 1) / 3 = 334
	mockLogStorage.On("GetLogsWithOffset", ctx, "parent-123", 334, 0).Return(parentLogs, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "child-456", 334, 0).Return(childLogs1, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "child-789", 334, 0).Return(nil, assert.AnError)
	// Note: Service uses already-loaded child jobs from GetChildJobs for metadata (no additional GetJob calls - Comment 7)

	// Execute (should still get parent and child-456 logs despite child-789 error)
	logs, metadata, _, err := service.GetAggregatedLogs(ctx, "parent-123", true, "all", 1000, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 2) // Parent + child-456
	assert.Contains(t, metadata, "parent-123")
	assert.Contains(t, metadata, "child-456")

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_GetAggregatedLogs_EmptyLogs(t *testing.T) {
	ctx := context.Background()
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Setup: Job with no logs
	parentJob := &models.QueueJobState{
		ID:   "parent-123",
		Name: "Parent Job",
		Type: "crawler",
		Config: map[string]interface{}{
			"max_depth": 3,
			"seed_urls": []interface{}{"https://example.com"},
		},
	}

	// Mock expectations
	mockJobStorage.On("GetJob", ctx, "parent-123").Return(parentJob, nil)
	mockLogStorage.On("GetLogsWithOffset", ctx, "parent-123", 1000, 0).Return([]models.JobLogEntry{}, nil)

	// Execute
	logs, metadata, _, err := service.GetAggregatedLogs(ctx, "parent-123", false, "all", 1000, "", "asc")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, logs, 0)
	assert.Contains(t, metadata, "parent-123")

	mockJobStorage.AssertExpectations(t)
	mockLogStorage.AssertExpectations(t)
}

func TestService_extractJobMetadata(t *testing.T) {
	logger := arbor.NewLogger()

	// Create mock storage
	mockLogStorage := new(MockJobLogStorage)
	mockJobStorage := new(MockJobStorage)

	// Create service
	service := NewService(mockLogStorage, mockJobStorage, logger)

	// Use unexported function through a type assertion
	// This is a limitation of testing private functions directly
	// In a real scenario, this would be tested through public methods that use it
	t.Log("extractJobMetadata is tested indirectly through GetAggregatedLogs")
	assert.NotNil(t, service)
}
