// -----------------------------------------------------------------------
// Job Processor - Routes jobs from queue to registered workers
// -----------------------------------------------------------------------

package workerutil

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/common"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
	"github.com/ternarybob/quaero/internal/queue"
)

// JobProcessor is a job-agnostic processor that uses Badger queue for queue management.
// It routes jobs to registered workers based on job type.
// Supports concurrent job processing via multiple worker goroutines.
// Supports event-based cancellation of running jobs via EventJobCancelled events.
type JobProcessor struct {
	queueMgr     interfaces.QueueManager
	jobMgr       *queue.Manager
	eventService interfaces.EventService
	executors    map[string]interfaces.JobWorker // Job workers keyed by job type
	logger       arbor.ILogger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
	concurrency  int // Number of concurrent worker goroutines

	// Active job tracking for cancellation
	activeJobs   map[string]context.CancelFunc // Maps job ID to its cancel function
	activeJobsMu sync.RWMutex
}

// NewJobProcessor creates a new job processor that routes jobs to registered workers.
// The concurrency parameter controls how many jobs can be processed in parallel.
func NewJobProcessor(queueMgr interfaces.QueueManager, jobMgr *queue.Manager, logger arbor.ILogger, concurrency int) *JobProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	// Ensure minimum concurrency of 1
	if concurrency < 1 {
		concurrency = 1
	}

	return &JobProcessor{
		queueMgr:    queueMgr,
		jobMgr:      jobMgr,
		executors:   make(map[string]interfaces.JobWorker), // Initialize job worker map
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		running:     false,
		concurrency: concurrency,
		activeJobs:  make(map[string]context.CancelFunc),
	}
}

// SetEventService sets the event service for job cancellation events.
// Must be called before Start() to enable event-based cancellation.
func (jp *JobProcessor) SetEventService(eventService interfaces.EventService) {
	jp.eventService = eventService
}

// RegisterExecutor registers a job worker for a job type.
// The worker must implement the JobWorker interface.
func (jp *JobProcessor) RegisterExecutor(worker interfaces.JobWorker) {
	jobType := worker.GetWorkerType()
	jp.executors[jobType] = worker
	jp.logger.Debug().
		Str("job_type", jobType).
		Msg("Job worker registered")
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
	jp.logger.Info().
		Int("concurrency", jp.concurrency).
		Msg("Starting job processor")

	// Subscribe to job cancellation events for real-time cancellation of running jobs
	if jp.eventService != nil {
		if err := jp.eventService.Subscribe(interfaces.EventJobCancelled, jp.handleJobCancelled); err != nil {
			jp.logger.Warn().Err(err).Msg("Failed to subscribe to job cancellation events")
		} else {
			jp.logger.Debug().Msg("Subscribed to job cancellation events")
		}
	}

	// Start multiple goroutines to process jobs concurrently
	for i := 0; i < jp.concurrency; i++ {
		jp.wg.Add(1)
		go jp.processJobs(i)
	}
}

// handleJobCancelled handles EventJobCancelled events to cancel running jobs.
func (jp *JobProcessor) handleJobCancelled(ctx context.Context, event interfaces.Event) error {
	// Extract job ID from event payload
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		jp.logger.Warn().Msg("Invalid job cancelled event payload")
		return nil
	}

	jobID, ok := payload["job_id"].(string)
	if !ok || jobID == "" {
		jp.logger.Warn().Msg("Job cancelled event missing job_id")
		return nil
	}

	// Look up the active job and cancel it
	jp.activeJobsMu.RLock()
	cancelFn, exists := jp.activeJobs[jobID]
	jp.activeJobsMu.RUnlock()

	if exists {
		jp.logger.Info().
			Str("job_id", jobID).
			Msg("Cancelling running job via event")
		cancelFn()
	} else {
		jp.logger.Debug().
			Str("job_id", jobID).
			Msg("Job not found in active jobs (may have already completed)")
	}

	return nil
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

// Backoff configuration for idle polling
const (
	minBackoff = 100 * time.Millisecond // Initial backoff when queue is empty
	maxBackoff = 5 * time.Second        // Maximum backoff duration
)

// processJobs is the main job processing loop.
// workerID identifies which worker goroutine this is (for logging).
func (jp *JobProcessor) processJobs(workerID int) {
	defer jp.wg.Done()

	// CRITICAL: Panic recovery wrapper to capture fatal crashes
	// Without this, any panic in job processing or storage operations
	// will crash the entire application without logging
	defer func() {
		if r := recover(); r != nil {
			stackTrace := getStackTrace()

			// Log to structured logger first
			jp.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Str("stack", stackTrace).
				Int("worker_id", workerID).
				Msg("FATAL: Job processor goroutine panicked - writing crash file")

			// Write crash file for reliable persistence
			common.WriteCrashFile(r, stackTrace)

			// Exit the process - this goroutine panicking is fatal
			os.Exit(1)
		}
	}()

	jp.logger.Debug().
		Int("worker_id", workerID).
		Msg("Job processor worker started")

	// Backoff tracking for idle polling - reduces CPU when queue is empty
	currentBackoff := minBackoff

	for {
		select {
		case <-jp.ctx.Done():
			jp.logger.Debug().
				Int("worker_id", workerID).
				Msg("Job processor worker stopping")
			return
		default:
			jobProcessed := jp.processNextJob(workerID)

			if jobProcessed {
				// Reset backoff when we successfully process a job
				currentBackoff = minBackoff
			} else {
				// No job available - apply backoff to reduce CPU usage
				select {
				case <-jp.ctx.Done():
					return
				case <-time.After(currentBackoff):
				}

				// Exponential backoff: double the wait time up to max
				currentBackoff = currentBackoff * 2
				if currentBackoff > maxBackoff {
					currentBackoff = maxBackoff
				}
			}
		}
	}
}

// getStackTrace returns a formatted stack trace for panic debugging
func getStackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// processNextJob processes the next job from the queue, routing it to the appropriate worker based on job type.
// workerID identifies which worker goroutine is processing (for logging).
// Returns true if a job was processed, false if no job was available.
func (jp *JobProcessor) processNextJob(workerID int) bool {
	// Create a timeout context for receiving messages
	ctx, cancel := context.WithTimeout(jp.ctx, 1*time.Second)
	defer cancel()

	var msg *queue.Message
	var deleteFn func() error
	var err error
	jobProcessed := false

	// Panic recovery for individual job processing
	defer func() {
		if r := recover(); r != nil {
			jp.logger.Error().
				Str("panic", fmt.Sprintf("%v", r)).
				Str("stack", getStackTrace()).
				Int("worker_id", workerID).
				Msg("Recovered from panic in job processing")

			if msg != nil {
				// Update job status to failed
				jp.jobMgr.SetJobError(jp.ctx, msg.JobID, fmt.Sprintf("Job panicked: %v", r))
				jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")

				// Ensure message is deleted so it doesn't loop
				if deleteFn != nil {
					if err := deleteFn(); err != nil {
						jp.logger.Error().Err(err).Msg("Failed to delete message after panic")
					}
				}
			}
		}
	}()

	// Receive next message from queue
	msg, deleteFn, err = jp.queueMgr.Receive(ctx)
	if err != nil {
		// No message available or timeout - return false to trigger backoff
		return false
	}

	// Mark that we received a job (for backoff reset)
	jobProcessed = true

	// Track job start time for duration calculation
	jobStartTime := time.Now()

	// Log job start at Info level (significant event)
	// Include context in message for Service Logs visibility (structured fields not always displayed)
	jp.logger.Info().
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Int("worker_id", workerID).
		Msgf("Job started: %s (type=%s, worker=%d)", msg.JobID[:8], msg.Type, workerID)

	jp.logger.Trace().
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Int("worker_id", workerID).
		Msg("Processing job from queue")

	// Deserialize queue job from payload
	queueJob, err := models.QueueJobFromJSON(msg.Payload)
	if err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to deserialize queue job")

		// Delete malformed message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete malformed message")
		}
		return jobProcessed
	}

	// Validate queue job
	if err := queueJob.Validate(); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Invalid queue job")

		// Mark job as failed in database (not just delete from queue)
		// This ensures parent jobs don't wait indefinitely for failed children
		if updateErr := jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, string(models.JobStatusFailed)); updateErr != nil {
			jp.logger.Error().Err(updateErr).Str("job_id", msg.JobID).Msg("Failed to update invalid job status to failed")
		}
		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, fmt.Sprintf("Job validation failed: %v", err))
		jp.jobMgr.AddJobLog(jp.ctx, msg.JobID, "error", fmt.Sprintf("Job validation failed: %v", err))

		// Delete invalid message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete invalid message")
		}
		return jobProcessed
	}

	// Check if job has been cancelled before executing
	// This prevents processing of jobs that were cancelled while pending in the queue
	jobInterface, err := jp.jobMgr.GetJob(jp.ctx, msg.JobID)
	if err == nil {
		if jobState, ok := jobInterface.(*models.QueueJobState); ok {
			if jobState.Status == models.JobStatusCancelled {
				jp.logger.Info().
					Str("job_id", msg.JobID).
					Str("job_type", msg.Type).
					Int("worker_id", workerID).
					Msg("Job was cancelled, skipping execution")

				// Delete message from queue without executing
				if err := deleteFn(); err != nil {
					jp.logger.Error().Err(err).Msg("Failed to delete cancelled job message")
				}
				return jobProcessed
			}
		}
	}

	// Get worker for job type
	worker, ok := jp.executors[msg.Type]
	if !ok {
		errMsg := fmt.Sprintf("No worker registered for job type: %s", msg.Type)
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
		return jobProcessed
	}

	// Validate queue job with worker
	if err := worker.Validate(queueJob); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Msg("Queue job validation failed")

		// Update job status to failed
		jp.jobMgr.SetJobError(jp.ctx, msg.JobID, err.Error())
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")

		// Delete message from queue
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete message")
		}
		return jobProcessed
	}

	// Create a per-job cancellable context for event-based cancellation
	// This allows the job to be cancelled via EventJobCancelled events
	jobCtx, jobCancel := context.WithCancel(jp.ctx)

	// Register the job in activeJobs map for cancellation support
	jp.activeJobsMu.Lock()
	jp.activeJobs[msg.JobID] = jobCancel
	jp.activeJobsMu.Unlock()

	// Ensure we clean up the active job entry when done
	defer func() {
		jp.activeJobsMu.Lock()
		delete(jp.activeJobs, msg.JobID)
		jp.activeJobsMu.Unlock()
		jobCancel() // Clean up context resources
	}()

	// Execute the job using the worker with the per-job context
	jp.logger.Debug().
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Msg("Calling worker.Execute")
	err = worker.Execute(jobCtx, queueJob)
	jp.logger.Debug().
		Str("job_id", msg.JobID).
		Str("job_type", msg.Type).
		Bool("has_error", err != nil).
		Msg("Worker.Execute returned")

	// Check if job was cancelled via context
	if jobCtx.Err() == context.Canceled {
		// Include context in message for Service Logs visibility
		cancelDuration := time.Since(jobStartTime)
		jp.logger.Info().
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Int("worker_id", workerID).
			Dur("duration", cancelDuration).
			Msgf("Job cancelled: %s (type=%s, worker=%d, duration=%s)", msg.JobID[:8], msg.Type, workerID, cancelDuration.Round(time.Millisecond))

		// Job was cancelled - update status (already done by cancel handler)
		// Just delete the message and return
		if err := deleteFn(); err != nil {
			jp.logger.Error().Err(err).Msg("Failed to delete cancelled job message")
		}
		return jobProcessed
	}

	if err != nil {
		// Job failed - log at Error level with duration
		// Include context in message for Service Logs visibility
		failDuration := time.Since(jobStartTime)
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Int("worker_id", workerID).
			Dur("duration", failDuration).
			Msgf("Job failed: %s (type=%s, worker=%d, duration=%s) error=%v", msg.JobID[:8], msg.Type, workerID, failDuration.Round(time.Millisecond), err)

		// Error is already set by worker, just ensure status is updated
		jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "failed")

		// Log failure to parent step job for UI visibility (ERR level)
		// This ensures failed child jobs appear in the step's log stream
		parentID := queueJob.GetParentID()
		if parentID != "" {
			errMsg := fmt.Sprintf("Job failed: %s (type=%s, duration=%s) error=%v",
				msg.JobID[:8], msg.Type, failDuration.Round(time.Millisecond), err)
			if logErr := jp.jobMgr.AddJobLog(jp.ctx, parentID, "error", errMsg); logErr != nil {
				jp.logger.Warn().Err(logErr).
					Str("parent_id", parentID).
					Str("job_id", msg.JobID).
					Msg("Failed to log job failure to parent")
			}
		}

		// Set finished_at timestamp for failed jobs
		if finishErr := jp.jobMgr.SetJobFinished(jp.ctx, msg.JobID); finishErr != nil {
			jp.logger.Warn().Err(finishErr).Str("job_id", msg.JobID).Msg("Failed to set finished_at timestamp")
		}
	} else {
		// Job succeeded - log at Info level with duration
		// Include context in message for Service Logs visibility
		duration := time.Since(jobStartTime)
		jp.logger.Info().
			Str("job_id", msg.JobID).
			Str("job_type", msg.Type).
			Int("worker_id", workerID).
			Dur("duration", duration).
			Msgf("Job completed: %s (type=%s, worker=%d, duration=%s)", msg.JobID[:8], msg.Type, workerID, duration.Round(time.Millisecond))

		// For parent jobs, do NOT mark as completed here - JobMonitor will handle completion
		// when all children are done. For other job types, mark as completed immediately.
		if msg.Type == "parent" {
			jp.logger.Trace().
				Str("job_id", msg.JobID).
				Msg("Parent job execution completed - leaving in running state for child monitoring")
			// Parent job remains in "running" state and will be monitored by JobMonitor
		} else {
			// Non-parent jobs are marked as completed immediately
			jp.jobMgr.UpdateJobStatus(jp.ctx, msg.JobID, "completed")

			// Set finished_at timestamp for completed jobs
			if finishErr := jp.jobMgr.SetJobFinished(jp.ctx, msg.JobID); finishErr != nil {
				jp.logger.Warn().Err(finishErr).Str("job_id", msg.JobID).Msg("Failed to set finished_at timestamp")
			}
		}
	}

	// Remove message from queue
	if err := deleteFn(); err != nil {
		jp.logger.Error().
			Err(err).
			Str("job_id", msg.JobID).
			Msg("Failed to delete message from queue")
	}

	return jobProcessed
}
