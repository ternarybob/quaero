package queue

import (
	"context"
	"fmt"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// cacheTagsKey is a context key type for cache tags
type cacheTagsKey struct{}

// cacheTagsContextKey is the context key for cache tags
var cacheTagsContextKey = cacheTagsKey{}

// GetCacheTagsFromContext extracts cache tags from context.
// Workers should call this to get cache tags to apply to created documents.
// Returns nil if no cache tags are present.
func GetCacheTagsFromContext(ctx context.Context) []string {
	if tags, ok := ctx.Value(cacheTagsContextKey).([]string); ok {
		return tags
	}
	return nil
}

// StepManager handles worker registration and routing for job definition steps.
// It replaces the worker management functionality previously in Manager.
type StepManager struct {
	workers      map[models.WorkerType]interfaces.DefinitionWorker
	cacheService interfaces.CacheService
	jobMgr       interfaces.JobStatusManager
	logger       arbor.ILogger
}

// NewStepManager creates a new StepManager
func NewStepManager(logger arbor.ILogger) *StepManager {
	return &StepManager{
		workers: make(map[models.WorkerType]interfaces.DefinitionWorker),
		logger:  logger,
	}
}

// SetCacheService sets the cache service for document caching.
// This is optional - if not set, caching is disabled.
func (m *StepManager) SetCacheService(cacheService interfaces.CacheService) {
	m.cacheService = cacheService
}

// SetJobManager sets the job manager for logging cache operations.
func (m *StepManager) SetJobManager(jobMgr interfaces.JobStatusManager) {
	m.jobMgr = jobMgr
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

// RegisterWorkerAlias registers an existing worker under an additional alias type.
// This is useful for backward compatibility when deprecating worker types.
// The worker will be invoked for both its original type and the alias type.
func (m *StepManager) RegisterWorkerAlias(worker interfaces.DefinitionWorker, aliasType models.WorkerType) {
	if worker == nil {
		return
	}
	m.workers[aliasType] = worker
	m.logger.Debug().
		Str("step_type", string(aliasType)).
		Str("target_type", string(worker.GetType())).
		Msg("Registered step worker alias (deprecated)")
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
		Str("phase", "init").
		Str("step_type", string(workerType)).
		Str("step_name", step.Name).
		Msg("Initializing step worker")

	// Call worker's Init method to assess work
	initResult, err := worker.Init(ctx, step, jobDef)
	if err != nil {
		return nil, fmt.Errorf("worker init failed: %w", err)
	}

	m.logger.Debug().
		Str("phase", "init").
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
//
// Cache check happens here:
// 1. Resolve cache config from job and step config
// 2. Generate cache tags for this step (including content hash if available)
// 3. Check if a fresh cached document exists
// 4. If cache hit, skip worker execution and return
// 5. If cache miss, proceed with worker execution
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

	// Extract content hash from initResult for cache invalidation
	var contentHash string
	if initResult != nil && initResult.ContentHash != "" {
		contentHash = initResult.ContentHash
	}

	// Generate cache tags for this step (including content hash if available)
	cacheTags := models.GenerateCacheTagsWithHash(jobDef.ID, step.Name, 1, contentHash)

	// Check cache before executing worker
	if m.cacheService != nil {
		cacheConfig := models.ResolveCacheConfig(jobDef.Config, step.Config)

		if cacheConfig.Enabled && cacheConfig.Type != models.CacheTypeNone {
			m.logger.Debug().
				Str("step_name", step.Name).
				Str("jobdef_id", jobDef.ID).
				Str("cache_type", string(cacheConfig.Type)).
				Int("cache_hours", cacheConfig.Hours).
				Strs("cache_tags", cacheTags).
				Msg("Checking step cache")

			// Check for fresh cached document
			doc, found := m.cacheService.GetFreshDocument(ctx, cacheTags, cacheConfig)
			if found {
				m.logger.Info().
					Str("step_name", step.Name).
					Str("doc_id", doc.ID).
					Str("last_synced", doc.LastSynced.Format("2006-01-02 15:04")).
					Str("cache_type", string(cacheConfig.Type)).
					Msg("Cache hit - using cached document, skipping worker execution")

				// Log to job if manager available
				if m.jobMgr != nil {
					logMsg := fmt.Sprintf("Cache hit: using cached document (last synced: %s)",
						doc.LastSynced.Format("2006-01-02 15:04"))
					m.jobMgr.AddJobLog(ctx, parentJobID, "info", logMsg)
				}

				// Return the stepID - step is considered complete with cached document
				return parentJobID, nil
			}

			// No cache hit - clean up old revisions before creating new document
			if cacheConfig.Revisions > 1 {
				if err := m.cacheService.CleanupRevisions(ctx, jobDef.ID, step.Name, cacheConfig.Revisions); err != nil {
					m.logger.Warn().
						Err(err).
						Str("step_name", step.Name).
						Msg("Failed to cleanup old revisions")
				}
			}

			m.logger.Debug().
				Str("step_name", step.Name).
				Msg("Cache miss - proceeding with worker execution")
		}
	}

	// Pass cache tags to worker via context
	ctx = context.WithValue(ctx, cacheTagsContextKey, cacheTags)

	m.logger.Debug().
		Str("step_type", string(workerType)).
		Str("step_name", step.Name).
		Bool("has_init_result", initResult != nil).
		Msg("[run] Executing step worker CreateJobs")

	return worker.CreateJobs(ctx, step, jobDef, parentJobID, initResult)
}
