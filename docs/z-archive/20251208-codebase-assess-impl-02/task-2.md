# Task 2: Remove WorkerTypeExtractStructure from worker_type.go

Depends: 1 | Critical: no | Model: sonnet

## Addresses User Intent

Remove redundant code - the WorkerType constant is no longer needed after deleting the worker

## Do

- Remove `WorkerTypeExtractStructure` constant declaration in `internal/models/worker_type.go`
- Remove it from `IsValid()` switch statement
- Remove it from `AllWorkerTypes()` return slice
- Update any tests in `job_definition_test.go` that reference this type

## Accept

- [ ] WorkerTypeExtractStructure constant removed
- [ ] IsValid() no longer includes it
- [ ] AllWorkerTypes() no longer includes it
- [ ] Tests updated/removed
