# Task 1: Add DevOps worker type and interfaces

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Foundation layer that enables all 5 enrichment actions to be registered and executed through the job system.

## Do

- Add `WorkerTypeDevOps` to `internal/models/worker_type.go`
- Create `internal/queue/workers/devops_worker.go` implementing DefinitionWorker interface
- Define DevOps metadata schema types in `internal/models/devops.go`
- Register the worker in the worker factory

## Accept

- [ ] WorkerTypeDevOps constant exists and is valid
- [ ] DevOpsWorker implements DefinitionWorker interface
- [ ] DevOps metadata types defined (DevOpsMetadata struct with all fields)
- [ ] Worker factory can instantiate DevOpsWorker
- [ ] Code compiles without errors
