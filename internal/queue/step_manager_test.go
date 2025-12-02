package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// Mock worker for testing
type mockDefinitionWorker struct {
	workerType       models.WorkerType
	validateErr      error
	createJobsResult string
	createJobsErr    error
	validateCalled   bool
	createJobsCalled bool
}

func (m *mockDefinitionWorker) GetType() models.WorkerType {
	return m.workerType
}

func (m *mockDefinitionWorker) ValidateConfig(step models.JobStep) error {
	m.validateCalled = true
	return m.validateErr
}

func (m *mockDefinitionWorker) CreateJobs(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string) (string, error) {
	m.createJobsCalled = true
	return m.createJobsResult, m.createJobsErr
}

func (m *mockDefinitionWorker) ReturnsChildJobs() bool {
	return true
}

func TestNewStepManager(t *testing.T) {
	logger := arbor.NewLogger()

	sm := NewStepManager(logger)

	if sm == nil {
		t.Fatal("NewStepManager returned nil")
	}

	if sm.workers == nil {
		t.Error("workers map not initialized")
	}

	if sm.logger == nil {
		t.Error("logger not set")
	}
}

func TestStepManager_RegisterWorker(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	// Test registering a worker
	worker := &mockDefinitionWorker{
		workerType: models.WorkerType("test_worker"),
	}

	sm.RegisterWorker(worker)

	if !sm.HasWorker(models.WorkerType("test_worker")) {
		t.Error("Worker not registered")
	}

	// Test registering nil worker (should be no-op)
	sm.RegisterWorker(nil)
}

func TestStepManager_HasWorker(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	worker := &mockDefinitionWorker{
		workerType: models.WorkerType("test_worker"),
	}

	// Should not have worker before registration
	if sm.HasWorker(models.WorkerType("test_worker")) {
		t.Error("HasWorker returned true for unregistered worker")
	}

	// Should have worker after registration
	sm.RegisterWorker(worker)
	if !sm.HasWorker(models.WorkerType("test_worker")) {
		t.Error("HasWorker returned false for registered worker")
	}

	// Should not have different worker
	if sm.HasWorker(models.WorkerType("other_worker")) {
		t.Error("HasWorker returned true for different worker")
	}
}

func TestStepManager_GetWorker(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	worker := &mockDefinitionWorker{
		workerType: models.WorkerType("test_worker"),
	}

	// Should return nil for unregistered worker
	if w := sm.GetWorker(models.WorkerType("test_worker")); w != nil {
		t.Error("GetWorker returned non-nil for unregistered worker")
	}

	// Should return worker after registration
	sm.RegisterWorker(worker)
	w := sm.GetWorker(models.WorkerType("test_worker"))
	if w == nil {
		t.Fatal("GetWorker returned nil for registered worker")
	}
	if w != worker {
		t.Error("GetWorker returned different worker")
	}
}

func TestStepManager_Execute_Success(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	worker := &mockDefinitionWorker{
		workerType:       models.WorkerType("test_worker"),
		createJobsResult: "job-123",
	}
	sm.RegisterWorker(worker)

	ctx := context.Background()
	step := models.JobStep{
		Type:   "test_worker",
		Name:   "Test Step",
		Config: map[string]interface{}{},
	}
	jobDef := models.JobDefinition{
		ID:   "def-1",
		Name: "Test Job Definition",
	}
	parentID := "parent-123"

	jobID, err := sm.Execute(ctx, step, jobDef, parentID)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if jobID != "job-123" {
		t.Errorf("Expected jobID 'job-123', got '%s'", jobID)
	}

	if !worker.validateCalled {
		t.Error("ValidateConfig was not called")
	}

	if !worker.createJobsCalled {
		t.Error("CreateJobs was not called")
	}
}

func TestStepManager_Execute_NoWorker(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	ctx := context.Background()
	step := models.JobStep{
		Type:   "unknown_worker",
		Name:   "Test Step",
		Config: map[string]interface{}{},
	}
	jobDef := models.JobDefinition{
		ID:   "def-1",
		Name: "Test Job Definition",
	}
	parentID := "parent-123"

	_, err := sm.Execute(ctx, step, jobDef, parentID)

	if err == nil {
		t.Fatal("Execute should have failed for unregistered worker")
	}

	if err.Error() != "no worker registered for step type: unknown_worker" {
		t.Errorf("Expected 'no worker registered' error, got: %v", err)
	}
}

func TestStepManager_Execute_ValidationError(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	validationErr := errors.New("invalid config")
	worker := &mockDefinitionWorker{
		workerType:  models.WorkerType("test_worker"),
		validateErr: validationErr,
	}
	sm.RegisterWorker(worker)

	ctx := context.Background()
	step := models.JobStep{
		Type:   "test_worker",
		Name:   "Test Step",
		Config: map[string]interface{}{},
	}
	jobDef := models.JobDefinition{
		ID:   "def-1",
		Name: "Test Job Definition",
	}
	parentID := "parent-123"

	_, err := sm.Execute(ctx, step, jobDef, parentID)

	if err == nil {
		t.Fatal("Execute should have failed with validation error")
	}

	if !worker.validateCalled {
		t.Error("ValidateConfig was not called")
	}

	if worker.createJobsCalled {
		t.Error("CreateJobs should not have been called after validation error")
	}
}

func TestStepManager_Execute_CreateJobsError(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	createErr := errors.New("failed to create jobs")
	worker := &mockDefinitionWorker{
		workerType:    models.WorkerType("test_worker"),
		createJobsErr: createErr,
	}
	sm.RegisterWorker(worker)

	ctx := context.Background()
	step := models.JobStep{
		Type:   "test_worker",
		Name:   "Test Step",
		Config: map[string]interface{}{},
	}
	jobDef := models.JobDefinition{
		ID:   "def-1",
		Name: "Test Job Definition",
	}
	parentID := "parent-123"

	_, err := sm.Execute(ctx, step, jobDef, parentID)

	if err == nil {
		t.Fatal("Execute should have failed with CreateJobs error")
	}

	if err != createErr {
		t.Errorf("Expected CreateJobs error, got: %v", err)
	}

	if !worker.validateCalled {
		t.Error("ValidateConfig was not called")
	}

	if !worker.createJobsCalled {
		t.Error("CreateJobs was not called")
	}
}

func TestStepManager_RegisterWorker_Replacement(t *testing.T) {
	logger := arbor.NewLogger()
	sm := NewStepManager(logger)

	worker1 := &mockDefinitionWorker{
		workerType:       models.WorkerType("test_worker"),
		createJobsResult: "job-1",
	}
	worker2 := &mockDefinitionWorker{
		workerType:       models.WorkerType("test_worker"),
		createJobsResult: "job-2",
	}

	// Register first worker
	sm.RegisterWorker(worker1)
	w := sm.GetWorker(models.WorkerType("test_worker"))
	if w != worker1 {
		t.Error("First worker not registered correctly")
	}

	// Register second worker with same type (should replace)
	sm.RegisterWorker(worker2)
	w = sm.GetWorker(models.WorkerType("test_worker"))
	if w != worker2 {
		t.Error("Second worker did not replace first worker")
	}
	if w == worker1 {
		t.Error("First worker still registered after replacement")
	}
}
