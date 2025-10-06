package workers

import (
	"context"
	"fmt"
	"sync"

	"github.com/ternarybob/arbor"
)

// Job represents a work item to be processed
type Job func(ctx context.Context) error

// Pool manages a pool of workers for parallel processing
type Pool struct {
	jobs       chan Job
	maxWorkers int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	errors     []error
	errorsMu   sync.Mutex
	logger     arbor.ILogger
}

// NewPool creates a new worker pool
func NewPool(maxWorkers int, logger arbor.ILogger) *Pool {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		jobs:       make(chan Job, maxWorkers*2),
		maxWorkers: maxWorkers,
		ctx:        ctx,
		cancel:     cancel,
		errors:     make([]error, 0),
		logger:     logger,
	}
}

// Start begins the worker pool
func (p *Pool) Start() {
	p.logger.Info().
		Int("max_workers", p.maxWorkers).
		Msg("Starting worker pool")

	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Submit adds a job to the pool
func (p *Pool) Submit(job Job) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	}
}

// Wait waits for all jobs to complete
func (p *Pool) Wait() {
	close(p.jobs)
	p.wg.Wait()
}

// Shutdown gracefully shuts down the worker pool
func (p *Pool) Shutdown() {
	p.cancel()
	p.Wait()
	p.logger.Info().Msg("Worker pool shutdown complete")
}

// Errors returns all collected errors
func (p *Pool) Errors() []error {
	p.errorsMu.Lock()
	defer p.errorsMu.Unlock()
	return p.errors
}

// worker processes jobs from the queue
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	p.logger.Debug().
		Int("worker_id", id).
		Msg("Worker started")

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				p.logger.Debug().
					Int("worker_id", id).
					Msg("Worker stopping - job queue closed")
				return
			}

			if err := job(p.ctx); err != nil {
				p.errorsMu.Lock()
				p.errors = append(p.errors, err)
				p.errorsMu.Unlock()

				p.logger.Error().
					Err(err).
					Int("worker_id", id).
					Msg("Job failed")
			}

		case <-p.ctx.Done():
			p.logger.Debug().
				Int("worker_id", id).
				Msg("Worker stopping - context cancelled")
			return
		}
	}
}
