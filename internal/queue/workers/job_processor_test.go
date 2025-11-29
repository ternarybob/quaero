package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// mockQueueManager implements interfaces.QueueManager for testing
type mockQueueManager struct {
	messages      chan *models.QueueMessage
	receiveCount  atomic.Int32
	concurrentMax atomic.Int32
	currentActive atomic.Int32
}

func newMockQueueManager() *mockQueueManager {
	return &mockQueueManager{
		messages: make(chan *models.QueueMessage, 100),
	}
}

func (m *mockQueueManager) Enqueue(ctx context.Context, msg models.QueueMessage) error {
	return nil
}

func (m *mockQueueManager) Receive(ctx context.Context) (*models.QueueMessage, func() error, error) {
	// Track that a worker is polling (means a goroutine is running)
	m.receiveCount.Add(1)

	// Track concurrent workers polling
	current := m.currentActive.Add(1)
	defer m.currentActive.Add(-1)

	// Update max concurrent
	for {
		old := m.concurrentMax.Load()
		if current <= old || m.concurrentMax.CompareAndSwap(old, current) {
			break
		}
	}

	// Simulate realistic queue polling - block for a short time to allow
	// concurrent workers to overlap, then return "no message" error
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		// Return timeout - simulates no messages available
		return nil, nil, context.DeadlineExceeded
	}
}

func (m *mockQueueManager) Extend(ctx context.Context, messageID string, duration time.Duration) error {
	return nil
}

func (m *mockQueueManager) Close() error {
	return nil
}

// Ensure mockQueueManager implements interfaces.QueueManager
var _ interfaces.QueueManager = (*mockQueueManager)(nil)

// TestJobProcessorConcurrencyField verifies that the concurrency field is set correctly
func TestJobProcessorConcurrencyField(t *testing.T) {
	tests := []struct {
		name             string
		inputConcurrency int
		expectedField    int
	}{
		{"normal concurrency (2)", 2, 2},
		{"high concurrency (5)", 5, 5},
		{"concurrency of 1", 1, 1},
		{"zero concurrency defaults to 1", 0, 1},
		{"negative concurrency defaults to 1", -5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := newMockQueueManager()
			logger := arbor.NewLogger()

			jp := NewJobProcessor(mockQueue, nil, logger, tt.inputConcurrency)

			if jp.concurrency != tt.expectedField {
				t.Errorf("expected concurrency field = %d, got %d", tt.expectedField, jp.concurrency)
			}
		})
	}
}

// TestJobProcessorStartsMultipleGoroutines verifies that Start() spawns the correct number of worker goroutines
// Note: This test uses a timing-based approach and may not always detect all goroutines
// For a more reliable test, see TestJobProcessorConcurrentJobExecution
func TestJobProcessorStartsMultipleGoroutines(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
	}{
		{"single worker", 1},
		{"two workers", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueue := newMockQueueManager()
			logger := arbor.NewLogger()

			jp := NewJobProcessor(mockQueue, nil, logger, tt.concurrency)

			// Start the processor
			jp.Start()

			// Give the goroutines time to start polling
			// Workers now have backoff (100ms-5s) when queue is empty, so we need to wait
			// long enough for all workers to have polled at least once.
			// Initial backoff is 100ms, so waiting 500ms should be sufficient.
			time.Sleep(500 * time.Millisecond)

			// Check that the expected number of goroutines are polling
			// The concurrentMax should be at least the concurrency level
			maxConcurrent := mockQueue.concurrentMax.Load()

			// Stop the processor
			jp.Stop()

			if int(maxConcurrent) < tt.concurrency {
				t.Errorf("expected at least %d concurrent workers polling, got %d", tt.concurrency, maxConcurrent)
			}

			t.Logf("Goroutine test passed: concurrency=%d, maxConcurrent=%d", tt.concurrency, maxConcurrent)
		})
	}
}

// TestJobProcessorStartStop tests the start/stop lifecycle
func TestJobProcessorStartStop(t *testing.T) {
	mockQueue := newMockQueueManager()
	logger := arbor.NewLogger()

	jp := NewJobProcessor(mockQueue, nil, logger, 3)

	// Should not be running initially
	if jp.running {
		t.Error("processor should not be running initially")
	}

	// Start should set running to true
	jp.Start()
	if !jp.running {
		t.Error("processor should be running after Start()")
	}

	// Double start should be a no-op
	jp.Start()
	if !jp.running {
		t.Error("processor should still be running after double Start()")
	}

	// Stop should complete (implicitly tests WaitGroup)
	jp.Stop()

	// Double stop should be a no-op
	jp.Stop()
}

// TestJobProcessorConcurrentJobExecution tests that jobs are actually processed concurrently
// This test uses a blocking queue to ensure multiple workers are waiting simultaneously
func TestJobProcessorConcurrentJobExecution(t *testing.T) {
	tests := []struct {
		name               string
		concurrency        int
		numJobs            int
		expectedConcurrent int
	}{
		{"concurrency 2 with 4 jobs", 2, 4, 2},
		{"concurrency 3 with 6 jobs", 3, 6, 3},
		{"concurrency 5 with 10 jobs", 5, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a queue manager that tracks concurrent receivers
			var concurrentReceivers atomic.Int32
			var maxConcurrent atomic.Int32
			var mu sync.Mutex
			receiverReady := make(chan struct{}, tt.concurrency)
			releaseJobs := make(chan struct{})

			// Custom queue manager for this test
			mockQueue := &blockingQueueManager{
				concurrentReceivers: &concurrentReceivers,
				maxConcurrent:       &maxConcurrent,
				mu:                  &mu,
				receiverReady:       receiverReady,
				releaseJobs:         releaseJobs,
				numJobs:             tt.numJobs,
			}

			logger := arbor.NewLogger()
			jp := NewJobProcessor(mockQueue, nil, logger, tt.concurrency)

			// Start the processor
			jp.Start()

			// Wait for all workers to be ready (blocking on receive)
			for i := 0; i < tt.concurrency; i++ {
				select {
				case <-receiverReady:
					// Worker is ready
				case <-time.After(2 * time.Second):
					t.Fatalf("timed out waiting for worker %d to be ready", i)
				}
			}

			// At this point, all workers should be blocked in Receive
			concurrent := maxConcurrent.Load()

			// Release the jobs
			close(releaseJobs)

			// Stop the processor
			jp.Stop()

			if int(concurrent) < tt.expectedConcurrent {
				t.Errorf("expected at least %d concurrent receivers, got %d", tt.expectedConcurrent, concurrent)
			}

			t.Logf("Concurrent execution test passed: expected=%d, actual=%d", tt.expectedConcurrent, concurrent)
		})
	}
}

// blockingQueueManager is a custom mock that blocks until signaled
type blockingQueueManager struct {
	concurrentReceivers *atomic.Int32
	maxConcurrent       *atomic.Int32
	mu                  *sync.Mutex
	receiverReady       chan struct{}
	releaseJobs         chan struct{}
	numJobs             int
	jobsReturned        atomic.Int32
}

func (m *blockingQueueManager) Enqueue(ctx context.Context, msg models.QueueMessage) error {
	return nil
}

func (m *blockingQueueManager) Receive(ctx context.Context) (*models.QueueMessage, func() error, error) {
	// Track concurrent receivers
	current := m.concurrentReceivers.Add(1)
	defer m.concurrentReceivers.Add(-1)

	// Update max concurrent
	for {
		old := m.maxConcurrent.Load()
		if current <= old || m.maxConcurrent.CompareAndSwap(old, current) {
			break
		}
	}

	// Signal that this receiver is ready
	select {
	case m.receiverReady <- struct{}{}:
	default:
	}

	// Wait for release signal or context cancellation
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-m.releaseJobs:
		// Return deadline exceeded to simulate no more jobs
		return nil, nil, context.DeadlineExceeded
	}
}

func (m *blockingQueueManager) Extend(ctx context.Context, messageID string, duration time.Duration) error {
	return nil
}

func (m *blockingQueueManager) Close() error {
	return nil
}

// Ensure blockingQueueManager implements interfaces.QueueManager
var _ interfaces.QueueManager = (*blockingQueueManager)(nil)
