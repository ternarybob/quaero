package queue

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// StepManager handles worker registration and routing for job definition steps.
// It replaces the worker management functionality previously in Manager.
type StepManager struct {
	workers map[models.WorkerType]interfaces.DefinitionWorker
	logger  arbor.ILogger
}

// NewStepManager creates a new StepManager
func NewStepManager(logger arbor.ILogger) *StepManager {
	return &StepManager{
		workers: make(map[models.WorkerType]interfaces.DefinitionWorker),
		logger:  logger,
	}
}

// RegisterWorker registers a DefinitionWorker for its declared WorkerType.
// If a worker for the same type is already registered, it will be replaced.
func (m *StepManager) RegisterWorker(worker interfaces.DefinitionWorker) {
	if worker == nil {
		return
	}
	stepType := worker.GetType()
	m.workers[stepType] = worker
	m.logger.Debug().Str("step_type", string(stepType)).Msg("Registered step worker")
}

// HasWorker checks if a worker is registered for the given WorkerType.
func (m *StepManager) HasWorker(workerType models.WorkerType) bool {
	_, exists := m.workers[workerType]
	return exists
}

// GetWorker returns the worker registered for the given WorkerType, or nil if not found.
func (m *StepManager) GetWorker(workerType models.WorkerType) interfaces.DefinitionWorker {
	return m.workers[workerType]
}

// Init initializes a step by calling the worker's Init method.
// This is the assessment phase where workers determine what work needs to be done.
// Returns the WorkerInitResult for inspection before job creation.
func (m *StepManager) Init(ctx context.Context, step models.JobStep, jobDef models.JobDefinition) (*interfaces.WorkerInitResult, error) {
	workerType := models.WorkerType(step.Type)

	worker, exists := m.workers[workerType]
	if !exists {
		return nil, fmt.Errorf("no worker registered for step type: %s", step.Type)
	}

	// Validate config first
	if err := worker.ValidateConfig(step); err != nil {
		return nil, fmt.Errorf("invalid step configuration: %w", err)
	}

	m.logger.Debug().
		Str("step_type", string(workerType)).
		Str("step_name", step.Name).
		Msg("Initializing step worker")

	// Call worker's Init method to assess work
	initResult, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		return nil, fmt.Errorf("worker init failed: %w", err)
	}

	m.logger.Debug().
		Str("step_type", string(workerType)).
		Str("step_name", step.Name).
		Int("work_items", len(initResult.WorkItems)).
		Int("total_count", initResult.TotalCount).
		Str("strategy", string(initResult.Strategy)).
		Msg("Step worker initialized")

	return initResult, nil
}

// Execute routes a step to the appropriate worker for job creation.
// If initResult is provided, it will be passed to CreateJobs.
// If nil, the worker will call Init internally.
func (m *StepManager) Execute(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string, initResult *interfaces.WorkerInitResult) (string, error) {
	workerType := models.WorkerType(step.Type)

	worker, exists := m.workers[workerType]
	if !exists {
		return "", fmt.Errorf("no worker registered for step type: %s", step.Type)
	}

	// If initResult is nil, validate config (Init wasn't called separately)
	if initResult == nil {
		if err := worker.ValidateConfig(step); err != nil {
			return "", fmt.Errorf("invalid step configuration: %w", err)
		}
	}

	m.logger.Debug().
		Str("step_type", string(workerType)).
		Str("step_name", step.Name).
		Bool("has_init_result", initResult != nil).
		Msg("Executing step worker CreateJobs")

	return worker.CreateJobs(ctx, step, jobDef, parentJobID, initResult)
}
