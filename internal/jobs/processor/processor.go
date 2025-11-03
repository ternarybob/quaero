package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/queue"
)

// Executor interface for job execution
type Executor interface {
	Execute(ctx context.Context, jobID string, payload []byte) error
}

// JobProcessor is a simplified job processor that uses goqite directly.
// It replaces the complex WorkerPool with a simpler polling-based approach.
type JobProcessor struct {
	queueMgr  *queue.Manager
	jobMgr    *jobs.Manager
	executors map[string]Executor
	logger    arbor.ILogger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	mu        sync.Mutex
}

// NewJobProcessor creates a new job processor.
func NewJobProcessor(queueMgr *queue.Manager, jobMgr *jobs.Manager, logger arbor.ILogger) *JobProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &JobProcessor{
		queueMgr:  queueMgr,
		jobMgr:    jobMgr,
		executors: make(map[string]Executor),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		running:   false,
	}
}

// RegisterExecutor registers an executor for a job type.
func (jp *JobProcessor) RegisterExecutor(jobType string, executor Executor) {
	jp.executors[jobType] = executor
	jp.logger.Info().
		Str("job_type", jobType).
		Msg("Executor registered")
}

// Start starts the job processor.
// This should be called AFTER all services are fully initialized to avoid deadlocks.
func (jp *JobProcessor) Start() {
	jp.mu.Lock()
	defer jp.mu.Unlock()

	if jp.running {
		jp.logger.Warn().Msg("Job processor already running")
		return
	}

	jp.running = true
	jp.logger.Info().Msg("Starting job processor")

	// Start a single goroutine to process jobs
	jp.wg.Add(1)
	go jp.processJobs()
}

// Stop stops the job processor gracefully.
func (jp *JobProcessor) Stop() {
	jp.mu.Lock()
	if !jp.running {
		jp.mu.Unlock()
		return
	}
	jp.mu.Unlock()

	jp.logger.Info().Msg("Stopping job processor...")
	jp.cancel()
	jp.wg.Wait()
	jp.logger.Info().Msg("Job processor stopped")
}

// processJobs is the main job processing loop.
func (jp *JobProcessor) processJobs() {
	defer jp.wg.Done()

	jp.logger.Info().Msg("Job processor goroutine started")

	for {
		select {
		case <-jp.ctx.Done():
			jp.logger.Info().Msg("Job processor goroutine stopping")
			return
		default:
			jp.processNextJob()
		}
	}
}

// processNextJob processes the next job from the queue.
func (jp *JobProcessor) processNextJob() {
	// Create a timeout context for receiving messages
	ctx, cancel := context.WithTimeout(jp.ctx, 1*time.Second)
	defer cancel()

	// Receive next message from queue
	msg, deleteFn, err := jp.queueMgr.Receive(ctx)
	if err != nil {
		// No message available or timeout - just return
		return
	}

	jp.logger.Info().
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Msg("Processing job")

	// Update job status to running
	if err := jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "running"); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to update job status to running")
	}

	if err := jp.jobMgr.AddJobLog(jp.ctx, msg.JobID, "info", "Job started"); err != nil {
		jp.logger.Warn().Err(err).Msg("Failed to add job log")
	}

	// Get executor for job type
	executor, ok := jp.executors[msg.Type]
	if !ok {
		errMsg := fmt.Sprintf("No executor registered for job type: %s", msg.Type)
		jp.logger.Error().
			Str("job_type", msg.Type).
			Str("job_id", msg.JobID).
			Msg(errMsg)

		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, errMsg)
		jp.jobMgr.AddJobLog(jp.ctx, msg.JobID, "error", errMsg)

		// Delete message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete message")
		}
		return
	}

	// Execute the job
	err = executor.Execute(jp.ctx, msg.JobID, msg.Payload)

	if err != nil {
		// Job failed
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Job failed")

		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, err.Error())
		jp.jobMgr.AddJobLog(jp.ctx, msg.JobID, "error", err.Error())
	} else {
		// Job succeeded
		jp.logger.Info().
			Str("job_id", msg.JobID).
			Msg("Job completed successfully")

		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "completed")
		jp.jobMgr.AddJobLog(jp.ctx, msg.JobID, "info", "Job completed successfully")
	}

	// Remove message from queue
	if err := deleteFn(); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to delete message from queue")
	}
}
