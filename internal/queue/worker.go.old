package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ternarybob/arbor"
	"maragu.dev/goqite"
)

// JobHandler is a function that handles a specific job type
type JobHandler func(ctx context.Context, msg *JobMessage) error

// JobStorage is a minimal interface for job status management in worker pool
// This matches the required methods from interfaces.JobStorage
type JobStorage interface {
	UpdateJobStatus(ctx context.Context, jobID string, status string, errorMsg string) error
	// MarkRunningJobsAsPending marks all running jobs as pending (for graceful shutdown)
	MarkRunningJobsAsPending(ctx context.Context, reason string) (int, error)
}

// WorkerPool manages a pool of workers that process queue messages
type WorkerPool struct {
	queueMgr   *Manager
	handlers   map[string]JobHandler
	jobStorage JobStorage
	logger     arbor.ILogger
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queueMgr *Manager, jobStorage JobStorage, logger arbor.ILogger) *WorkerPool {
	// Create child context from manager's context to isolate worker pool lifecycle
	ctx, cancel := context.WithCancel(queueMgr.ctx)

	return &WorkerPool{
		queueMgr:   queueMgr,
		handlers:   make(map[string]JobHandler),
		jobStorage: jobStorage,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterHandler registers a job type handler
func (wp *WorkerPool) RegisterHandler(jobType string, handler JobHandler) {
	wp.handlers[jobType] = handler
	wp.logger.Debug().
		Str("job_type", jobType).
		Msg("Job handler registered")
}

// Start starts the worker pool
func (wp *WorkerPool) Start() error {
	wp.logger.Info().
		Int("concurrency", wp.queueMgr.config.Concurrency).
		Msg("Starting worker pool")

	// Start worker goroutines
	for i := 0; i < wp.queueMgr.config.Concurrency; i++ {
		go wp.worker(i)
	}

	return nil
}

// Stop gracefully stops the worker pool and marks running jobs as pending for resume
func (wp *WorkerPool) Stop() error {
	wp.logger.Info().Msg("Stopping worker pool - marking running jobs as pending")

	// Mark all running jobs as pending so they can be resumed after restart
	// This allows graceful shutdown without losing job progress
	ctx := context.Background() // Use background context since worker context is being cancelled

	count, err := wp.jobStorage.MarkRunningJobsAsPending(ctx, "Service shutdown - job will resume on restart")
	if err != nil {
		wp.logger.Warn().Err(err).Msg("Failed to mark running jobs as pending during shutdown")
	} else if count > 0 {
		wp.logger.Info().Int("count", count).Msg("Marked running jobs as pending for graceful shutdown")
	}

	// Cancel worker pool context to stop all workers
	wp.cancel()

	// Give workers a brief moment to finish current processing
	time.Sleep(500 * time.Millisecond)

	wp.logger.Info().Msg("Worker pool stopped")
	return nil
}

// worker is the main worker loop that processes messages
func (wp *WorkerPool) worker(workerID int) {
	// Stagger worker starts to reduce database lock contention
	// Spread workers evenly across the poll interval
	staggerDelay := (wp.queueMgr.config.PollInterval / time.Duration(wp.queueMgr.config.Concurrency)) * time.Duration(workerID)
	if staggerDelay > 0 {
		time.Sleep(staggerDelay)
	}

	wp.logger.Debug().
		Int("worker_id", workerID).
		Dur("stagger_delay", staggerDelay).
		Msg("Worker started")

	ticker := time.NewTicker(wp.queueMgr.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			wp.logger.Debug().
				Int("worker_id", workerID).
				Msg("Worker stopped")
			return

		case <-ticker.C:
			// Try to receive a message
			if err := wp.processMessage(workerID); err != nil {
				errMsg := err.Error()
				// Log all errors except "no message" and SQLITE_BUSY errors
				// SQLITE_BUSY errors are expected with high concurrency and will retry on next poll
				if errMsg != "no message" && !strings.Contains(errMsg, "database is locked") && !strings.Contains(errMsg, "SQLITE_BUSY") {
					wp.logger.Warn().
						Err(err).
						Int("worker_id", workerID).
						Msg("Error processing message")
				}
			}
		}
	}
}

// processMessage receives and processes a single message
func (wp *WorkerPool) processMessage(workerID int) error {
	// Receive message with visibility timeout
	msg, err := wp.queueMgr.Receive(wp.ctx)
	if err != nil {
		// Check if no message is available (queue empty)
		if err.Error() == "no message" {
			return fmt.Errorf("no message")
		}
		return fmt.Errorf("failed to receive message: %w", err)
	}

	// Decode message body
	jobMsg, err := FromJSON(msg.Body)
	if err != nil {
		wp.logger.Error().
			Err(err).
			Str("message_id", string(msg.ID)).
			Int("worker_id", workerID).
			Msg("Failed to decode message body")
		// Delete invalid message with retry logic
		if err := wp.retryDelete(*msg, "invalid message"); err != nil {
			wp.logger.Warn().
				Err(err).
				Str("message_id", string(msg.ID)).
				Msg("Failed to delete invalid message after retries")
		}
		return fmt.Errorf("invalid message body: %w", err)
	}

	wp.logger.Debug().
		Str("message_id", jobMsg.ID).
		Str("type", jobMsg.Type).
		Int("worker_id", workerID).
		Msg("Processing message")

	// Update message status
	jobMsg.Status = "running"
	jobMsg.StartedAt = time.Now()

	// Find handler for job type
	handler, exists := wp.handlers[jobMsg.Type]
	if !exists {
		wp.logger.Error().
			Str("type", jobMsg.Type).
			Str("message_id", jobMsg.ID).
			Msg("No handler registered for job type")
		// Delete message with unknown type with retry logic
		if err := wp.retryDelete(*msg, "unknown job type"); err != nil {
			wp.logger.Warn().
				Err(err).
				Str("message_id", jobMsg.ID).
				Msg("Failed to delete unknown job type message after retries")
		}
		return fmt.Errorf("no handler for job type: %s", jobMsg.Type)
	}

	// Execute handler
	startTime := time.Now()
	handlerErr := handler(wp.ctx, jobMsg)
	duration := time.Since(startTime)

	if handlerErr != nil {
		jobMsg.Status = "failed"
		jobMsg.CompletedAt = time.Now()

		wp.logger.Error().
			Err(handlerErr).
			Str("message_id", jobMsg.ID).
			Str("type", jobMsg.Type).
			Dur("duration", duration).
			Int("worker_id", workerID).
			Msg("Job handler failed")

		// Delete failed message from queue with retry logic
		// Note: goqite v0.3.1 handles max receives internally via MaxReceive config
		// Messages that exceed max receives are automatically moved to dead-letter
		if err := wp.retryDelete(*msg, "handler failed"); err != nil {
			wp.logger.Error().
				Err(err).
				Str("message_id", jobMsg.ID).
				Msg("Failed to delete message after failure - will be redelivered by queue")
		}

		return handlerErr
	}

	// Handler succeeded
	jobMsg.Status = "completed"
	jobMsg.CompletedAt = time.Now()

	wp.logger.Info().
		Str("message_id", jobMsg.ID).
		Str("type", jobMsg.Type).
		Dur("duration", duration).
		Int("worker_id", workerID).
		Msg("Job completed successfully")

	// Delete message from queue (success) with retry logic
	if err := wp.retryDelete(*msg, "job completed"); err != nil {
		wp.logger.Error().
			Err(err).
			Str("message_id", jobMsg.ID).
			Msg("Failed to delete message after successful processing - will be redelivered by queue")
		return err
	}

	return nil
}

// retryDelete retries queue message deletion with exponential backoff for SQLITE_BUSY errors
func (wp *WorkerPool) retryDelete(msg goqite.Message, context string) error {
	var lastErr error
	delay := 200 * time.Millisecond // Initial delay

	for attempt := 1; attempt <= 3; attempt++ {
		lastErr = wp.queueMgr.Delete(wp.ctx, msg)
		if lastErr == nil {
			return nil
		}

		// Check if error is SQLITE_BUSY
		errMsg := lastErr.Error()
		isBusyError := strings.Contains(errMsg, "database is locked") || strings.Contains(errMsg, "SQLITE_BUSY")

		if !isBusyError {
			// Non-transient error, don't retry
			return lastErr
		}

		if attempt < 3 {
			// Log retry attempt
			wp.logger.Warn().
				Int("attempt", attempt).
				Int("max_attempts", 3).
				Str("delay", delay.String()).
				Str("context", context).
				Str("error", errMsg).
				Str("message_id", string(msg.ID)).
				Msg("Queue delete failed, retrying")

			// Wait before retry
			select {
			case <-wp.ctx.Done():
				return wp.ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff: double the delay
			delay *= 2
		}
	}

	// All attempts exhausted
	wp.logger.Error().
		Int("max_attempts", 3).
		Str("context", context).
		Err(lastErr).
		Str("message_id", string(msg.ID)).
		Msg("All retry attempts exhausted for queue delete")
	return lastErr
}
