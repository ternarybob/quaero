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

// Execute routes a step to the appropriate worker.
func (m *StepManager) Execute(ctx context.Context, step models.JobStep, jobDef models.JobDefinition, parentJobID string) (string, error) {
	// Determine worker type from step type
	// Note: models.JobStep uses StepType, but DefinitionWorker uses WorkerType.
	// They should be compatible or convertible.
	// Assuming direct cast for now as per existing code patterns, or we need a mapping.
	// Looking at manager_worker_architecture.md, StepType and WorkerType seem aligned.

	workerType := models.WorkerType(step.Type)

	worker, exists := m.workers[workerType]
	if !exists {
		return "", fmt.Errorf("no worker registered for step type: %s", step.Type)
	}

	// Validate config first
	if err := worker.ValidateConfig(step); err != nil {
		return "", fmt.Errorf("invalid step configuration: %w", err)
	}

	return worker.CreateJobs(ctx, step, jobDef, parentJobID)
}
