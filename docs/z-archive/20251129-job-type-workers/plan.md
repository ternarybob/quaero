# Plan: Job Type Workers Architecture Refactor

## Classification
- Type: feature
- Workdir: ./docs/feature/20251129-job-type-workers/

## Analysis

### Current State
- Job type is defined at parent level (`JobDefinition.Type`)
- Step action routes to specific managers (agent, crawler, places_search, etc.)
- Multiple specialized managers exist with overlapping patterns
- Workers are type-specific but tightly coupled to managers
- Redundant TOML fields: `name` + `action` in steps, `type` at parent level

### Target State
- Job type moved INTO steps (`[step.{name}]` with `type` field)
- Generic manager that routes based on step type
- Type-defined workers implementing common interface
- Simplified TOML: `[step.{name}]` with `type` and `description`
- Support for multiple step types within single job

### Dependencies
- `internal/models/job_definition.go` - Core model changes
- `internal/jobs/service.go` - TOML parsing updates
- `internal/queue/orchestrator.go` - Routing logic
- `internal/queue/managers/*.go` - Manager consolidation
- `internal/queue/workers/*.go` - Worker interface compliance
- `internal/interfaces/job_interfaces.go` - Interface definitions

### Risks
- Breaking changes to all job definitions (acceptable per requirements)
- Complex refactor touching multiple subsystems
- Need to maintain backward compatibility during migration (not required)

## Groups

| Task | Description | Depends | Critical | Complexity | Model |
|------|-------------|---------|----------|------------|-------|
| 1 | Update JobStep model - add Type field, remove Action redundancy | none | yes:architectural-change | medium | opus |
| 2 | Create generic StepManager with type-based routing | 1 | yes:architectural-change | high | opus |
| 3 | Refactor workers to comply with unified interface | 2 | no | medium | sonnet |
| 4 | Update TOML parsing in jobs/service.go | 1 | no | medium | sonnet |
| 5 | Update test job definitions (test/config/job-definitions) | 4 | no | low | sonnet |
| 6 | Execute tests and fix failures | 5 | no | medium | sonnet |
| 7 | Update example configs (deployments/local, bin/job-definitions) | 6 | no | low | sonnet |
| 8 | Update architecture documentation | 7 | no | low | sonnet |
| 9 | Remove redundant code and TOML fields | 8 | no | low | sonnet |

## Order
Sequential: [1] -> [2] -> Concurrent: [3, 4] -> Sequential: [5] -> [6] -> Concurrent: [7, 8] -> Sequential: [9] -> Review

## Architecture Decision

### New Step Schema
```toml
[step.{name}]
type = "agent" | "crawler" | "places_search" | "web_search" | "github_repo" | "github_actions" | "transform" | "reindex"
description = "What this step does"
on_error = "continue" | "fail" | "retry"
depends = "step1,step2"  # Optional dependencies
# Type-specific config fields...
```

### Generic Manager Design
```go
type GenericStepManager struct {
    workers map[StepType]StepWorker
    storage storage.Storage
    queue   queue.Queue
}

func (m *GenericStepManager) Execute(ctx, step JobStep, parentJobID string) error {
    worker := m.workers[step.Type]
    return worker.CreateJobs(ctx, step, parentJobID)
}
```

### Worker Interface
```go
type StepWorker interface {
    GetType() StepType
    CreateJobs(ctx context.Context, step JobStep, parentJobID string) ([]QueueJob, error)
    Execute(ctx context.Context, job QueueJob) error
    Validate(step JobStep) error
}
```
