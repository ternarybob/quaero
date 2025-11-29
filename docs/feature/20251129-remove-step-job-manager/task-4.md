# Task 4: Update all worker implementations

- Group: 4 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-4/ | Source: ./ | Output: ./docs/feature/20251129-remove-step-job-manager/

## Files
- `internal/queue/workers/agent_worker.go` - Update to DefinitionWorker
- `internal/queue/workers/crawler_worker.go` - Update to DefinitionWorker
- `internal/queue/workers/places_worker.go` - Update to DefinitionWorker
- `internal/queue/workers/web_search_worker.go` - Update to DefinitionWorker
- `internal/queue/workers/github_repo_worker.go` - Update to DefinitionWorker
- `internal/queue/workers/github_log_worker.go` - Update to DefinitionWorker

## Requirements
1. Update compile-time assertions from `interfaces.StepWorker` to `interfaces.DefinitionWorker`
2. Update `GetType()` return type from `models.StepType` to `models.WorkerType`
3. Update returned constants from `StepType*` to `WorkerType*`
4. Rename `ValidateStep` method to `ValidateConfig`

## Acceptance
- [ ] All workers implement DefinitionWorker interface
- [ ] All GetType() methods return WorkerType
- [ ] ValidateStep renamed to ValidateConfig
- [ ] Code compiles without errors
