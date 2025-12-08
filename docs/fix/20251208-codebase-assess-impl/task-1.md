# Task 1: Remove WorkerTypeExtractStructure from worker_type.go

Depends: 0 | Critical: no | Model: sonnet

## Addresses User Intent

Removes redundant worker type constant after worker deletion

## Do

- Remove `WorkerTypeExtractStructure` constant from `internal/models/worker_type.go`
- Remove from `IsValid()` switch statement
- Remove from `AllWorkerTypes()` slice

## Accept

- [ ] WorkerTypeExtractStructure removed from all 3 locations
- [ ] File compiles without errors
