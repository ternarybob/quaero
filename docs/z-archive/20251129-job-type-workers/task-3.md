# Task 3: Refactor Workers to Unified Interface

- Group: 3 | Mode: concurrent | Model: sonnet
- Skill: @golang-pro | Critical: no | Depends: 2
- Sandbox: /tmp/3agents/task-3/ | Source: ./ | Output: ./docs/feature/20251129-job-type-workers/

## Files
- `internal/queue/workers/agent_worker.go` - Update to StepWorker interface
- `internal/queue/workers/crawler_worker.go` - Update to StepWorker interface
- `internal/queue/workers/github_repo_worker.go` - Update to StepWorker interface
- `internal/queue/workers/github_log_worker.go` - Update to StepWorker interface
- `internal/queue/workers/job_processor.go` - Update worker registration

## Requirements

1. Each worker must implement `StepWorker` interface:
   - `GetType() models.StepType` - Return the step type this worker handles
   - `CreateJobs(ctx, step, jobDef, parentJobID)` - Create queue jobs for the step
   - `Execute(ctx, job)` - Execute a single queue job
   - `ReturnsChildJobs() bool` - Whether this worker creates child jobs
   - `Validate(step)` - Validate step configuration

2. Migrate manager logic into workers:
   - `AgentManager.CreateParentJob` -> `AgentWorker.CreateJobs`
   - `CrawlerManager.CreateParentJob` -> `CrawlerWorker.CreateJobs`
   - etc.

3. Update `JobProcessor` to use new worker interface

4. Ensure consistent error handling across all workers

## Acceptance
- [ ] All workers implement StepWorker interface
- [ ] Manager logic migrated to workers
- [ ] JobProcessor updated
- [ ] Consistent error handling
- [ ] Compiles: `go build ./...`
- [ ] Tests pass: `go test ./internal/queue/workers/...`
