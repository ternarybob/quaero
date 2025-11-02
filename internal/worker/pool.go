package worker

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/jobs"
	"github.com/ternarybob/quaero/internal/queue"
)

// Executor interface for job execution
type Executor interface {
	Execute(ctx context.Context, jobID string, payload []byte) error
}

// WorkerPool manages a pool of workers that process jobs
type WorkerPool struct {
	queueMgr   *queue.Manager
	jobMgr     *jobs.Manager
	executors  map[string]Executor
	logger     arbor.ILogger
	numWorkers int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewWorkerPool(queueMgr *queue.Manager, jobMgr *jobs.Manager, logger arbor.ILogger, numWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		queueMgr:   queueMgr,
		jobMgr:     jobMgr,
		executors:  make(map[string]Executor),
		logger:     logger,
		numWorkers: numWorkers,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterExecutor registers an executor for a job type
func (wp *WorkerPool) RegisterExecutor(jobType string, executor Executor) {
	wp.executors[jobType] = executor
	wp.logger.Info().
		Str("job_type", jobType).
		Msg("Executor registered")
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Info().
		Int("num_workers", wp.numWorkers).
		Msg("Starting worker pool")

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop() {
	wp.logger.Info().Msg("Stopping worker pool...")
	wp.cancel()
	wp.wg.Wait()
	wp.logger.Info().Msg("Worker pool stopped")
}

// worker is the main worker loop
func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()

	wp.logger.Debug().
		Int("worker_id", workerID).
		Msg("Worker started")

	for {
		select {
		case <-wp.ctx.Done():
			wp.logger.Debug().
				Int("worker_id", workerID).
				Msg("Worker stopping")
			return
		default:
			wp.processNextJob(workerID)
		}
	}
}

// processNextJob processes the next job from the queue
func (wp *WorkerPool) processNextJob(workerID int) {
	// Receive next message from queue
	msg, deleteFn, err := wp.queueMgr.Receive(wp.ctx)
	if err != nil {
		// No message available or context cancelled - just return
		return
	}

	wp.logger.Info().
		Int("worker_id", workerID).
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Msg("Processing job")

	// Update job status to running
	if err := wp.jobMgr.UpdateJobStatus(wp.ctx, msg.JobID, "running"); err != nil {
		wp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to update job status to running")
	}

	if err := wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "info", fmt.Sprintf("Job started on worker %d", workerID)); err != nil {
		wp.logger.Warn().Err(err).Msg("Failed to add job log")
	}

	// Get executor for job type
	executor, ok := wp.executors[msg.Type]
	if !ok {
		errMsg := fmt.Sprintf("No executor registered for job type: %s", msg.Type)
		wp.logger.Error().
			Str("job_type", msg.Type).
			Str("job_id", msg.JobID).
			Msg(errMsg)

		wp.jobMgr.SetJobError(wp.ctx, msg.JobID, errMsg)
		wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "error", errMsg)

		// Delete message from queue
		if err := deleteFn(); err != nil {
			wp.logger.Error().Err(err).Msg("Failed to delete message")
		}
		return
	}

	// Execute the job
	err = executor.Execute(wp.ctx, msg.JobID, msg.Payload)

	if err != nil {
		// Job failed
		wp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Job failed")

		wp.jobMgr.SetJobError(wp.ctx, msg.JobID, err.Error())
		wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "error", err.Error())
	} else {
		// Job succeeded
		wp.logger.Info().
			Str("job_id", msg.JobID).
			Msg("Job completed successfully")

		wp.jobMgr.UpdateJobStatus(wp.ctx, msg.JobID, "completed")
		wp.jobMgr.AddJobLog(wp.ctx, msg.JobID, "info", "Job completed successfully")
	}

	// Remove message from queue
	if err := deleteFn(); err != nil {
		wp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to delete message from queue")
	}
}
