// -----------------------------------------------------------------------
// Last Modified: Monday, 21st October 2025 5:45:00 pm
// Modified By: Claude Code
// -----------------------------------------------------------------------

// Package jobs provides the job type registry for multi-step job definitions.
//
// # ARCHITECTURE: Two Parallel Job Systems
//
// Quaero has two distinct job execution systems that serve different purposes:
//
// 1. JobTypeRegistry (THIS FILE) - Multi-step job definitions with action handlers
//   - Purpose: Orchestrates complex, multi-step workflows with configurable actions
//   - Examples: Summarization jobs with scan/summarize/extract_keywords actions
//   - Storage: Job definitions stored in database (job_definitions table)
//   - Execution: Actions registered here and executed via executor service
//   - Used for: User-defined scheduled jobs, batch processing workflows
//
// 2. Queue-based job types (internal/jobs/types/) - Single-purpose message handlers
//   - Purpose: Process individual work items from goqite message queue
//   - Examples: CrawlerJob (process single URL), SummarizerJob (process action), CleanupJob
//   - Storage: Messages in goqite queue table
//   - Execution: WorkerPool pulls messages and dispatches to registered handlers
//   - Used for: Distributed crawling, background processing, async task execution
//
// # Why Two Systems?
//
//   - JobTypeRegistry: Designed for user-facing job definitions where users configure
//     multi-step workflows (e.g., "scan all docs, then summarize, then extract keywords")
//
//   - Queue-based jobs: Designed for internal task distribution where a parent job
//     creates many child tasks that workers process concurrently (e.g., parent crawler
//     job enqueues 100 URL messages that workers process independently)
//
// # Naming Confusion
//
// Some overlap exists (e.g., "summarizer" appears in both systems):
// - JobTypeRegistry "summarizer": Multi-step workflow definition
// - Queue SummarizerJob: Individual action execution message handler
//
// This is intentional: the registry action creates queue messages for async execution.
package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/models"
)

// ActionHandler is a function that executes a job step action
// It receives the execution context, a pointer to the step configuration (allowing modifications),
// and the list of sources to operate on
type ActionHandler func(ctx context.Context, step *models.JobStep, sources []*models.SourceConfig) error

// JobTypeRegistry manages the registration and retrieval of action handlers for different job types.
//
// This registry is for MULTI-STEP JOB DEFINITIONS with configurable actions.
// For queue-based single-purpose job handlers, see internal/jobs/types/ and internal/queue/worker.go.
type JobTypeRegistry struct {
	actions map[models.JobType]map[string]ActionHandler // Nested map: job type → action name → handler
	logger  arbor.ILogger
	mu      sync.RWMutex // Read-write mutex for thread-safe access
}

// NewJobTypeRegistry creates a new job type registry
func NewJobTypeRegistry(logger arbor.ILogger) *JobTypeRegistry {
	registry := &JobTypeRegistry{
		actions: make(map[models.JobType]map[string]ActionHandler),
		logger:  logger,
	}

	if logger != nil {
		logger.Info().Msg("Job type registry initialized")
	}

	return registry
}

// RegisterAction registers an action handler for a specific job type and action name
func (r *JobTypeRegistry) RegisterAction(jobType models.JobType, actionName string, handler ActionHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate inputs
	if actionName == "" {
		return fmt.Errorf("action name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Validate job type using centralized helper
	if !models.IsValidJobType(jobType) {
		return fmt.Errorf("invalid job type: %s", jobType)
	}

	// Check for duplicate registration
	if r.actions[jobType] != nil {
		if _, exists := r.actions[jobType][actionName]; exists {
			return fmt.Errorf("action %s already registered for job type %s", actionName, jobType)
		}
	}

	// Initialize inner map if needed
	if r.actions[jobType] == nil {
		r.actions[jobType] = make(map[string]ActionHandler)
	}

	// Register handler
	r.actions[jobType][actionName] = handler

	if r.logger != nil {
		r.logger.Info().
			Str("job_type", string(jobType)).
			Str("action", actionName).
			Msg("Action registered")
	}

	return nil
}

// GetAction retrieves an action handler for a specific job type and action name
func (r *JobTypeRegistry) GetAction(jobType models.JobType, actionName string) (ActionHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if job type exists
	handlers, ok := r.actions[jobType]
	if !ok {
		return nil, fmt.Errorf("no actions registered for job type %s", jobType)
	}

	// Check if action exists
	handler, ok := handlers[actionName]
	if !ok {
		return nil, fmt.Errorf("action %s not found for job type %s", actionName, jobType)
	}

	return handler, nil
}

// ListActions returns a sorted list of all registered action names for a specific job type
func (r *JobTypeRegistry) ListActions(jobType models.JobType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if job type exists
	handlers, ok := r.actions[jobType]
	if !ok {
		return []string{}
	}

	// Collect action names
	actions := make([]string, 0, len(handlers))
	for actionName := range handlers {
		actions = append(actions, actionName)
	}

	// Sort alphabetically for deterministic output
	sort.Strings(actions)

	return actions
}

// GetAllJobTypes returns a list of all job types that have registered actions
func (r *JobTypeRegistry) GetAllJobTypes() []models.JobType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect job types
	jobTypes := make([]models.JobType, 0, len(r.actions))
	for jobType := range r.actions {
		jobTypes = append(jobTypes, jobType)
	}

	return jobTypes
}
