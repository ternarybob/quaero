# Task 2: Remove extract_structure registration from app.go

Depends: 0,1 | Critical: no | Model: sonnet

## Addresses User Intent

Removes worker registration after worker and type deletion

## Do

- Remove `extractStructureWorker` creation and registration from `internal/app/app.go` (lines ~698-706)
- Remove associated debug log line

## Accept

- [ ] No reference to ExtractStructureWorker in app.go
- [ ] File compiles without errors
