package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// JobProcessor is a job-agnostic processor that uses goqite for queue management.
// It routes jobs to registered executors based on job type.
type JobProcessor struct {
	queueMgr  *queue.Manager
	jobMgr    *jobs.Manager
	executors map[string]interfaces.JobExecutor
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
		executors: make(map[string]interfaces.JobExecutor),
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		running:   false,
	}
}

// RegisterExecutor registers a job executor for a job type.
// The executor must implement the JobExecutor interface.
func (jp *JobProcessor) RegisterExecutor(executor interfaces.JobExecutor) {
	jobType := executor.GetJobType()
	jp.executors[jobType] = executor
	jp.logger.Info().
		Str("job_type", jobType).
		Msg("Job executor registered")
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
		Msg("Processing job from queue")

	// Deserialize job model from payload
	jobModel, err := models.FromJSON(msg.Payload)
	if err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to deserialize job model")

		// Delete malformed message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete malformed message")
		}
		return
	}

	// Validate job model
	if err := jobModel.Validate(); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Invalid job model")

		// Delete invalid message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete invalid message")
		}
		return
	}

	// Get executor for job type
	executor, ok := jp.executors[msg.Type]
	if !ok {
		errMsg := fmt.Sprintf("No executor registered for job type: %s", msg.Type)
		jp.logger.Error().
			Str("job_type", msg.Type).
			Str("job_id", msg.JobID).
			Msg(errMsg)

		// Update job status to failed
		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, errMsg)
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")

		// Delete message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete message")
		}
		return
	}

	// Validate job model with executor
	if err := executor.Validate(jobModel); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Msg("Job model validation failed")

		// Update job status to failed
		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, err.Error())
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")

		// Delete message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete message")
		}
		return
	}

	// Execute the job using the executor
	err = executor.Execute(jp.ctx, jobModel)

	if err != nil {
		// Job failed
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Msg("Job execution failed")

		// Error is already set by executor, just ensure status is updated
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")
	} else {
		// Job succeeded
		jp.logger.Info().
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Msg("Job completed successfully")

		// Status is already set by executor, but ensure it's completed
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "completed")
	}

	// Remove message from queue
	if err := deleteFn(); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to delete message from queue")
	}
}
