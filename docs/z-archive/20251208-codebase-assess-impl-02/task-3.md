# Task 3: Remove extract_structure worker registration from app.go

Depends: 1,2 | Critical: no | Model: sonnet

## Addresses User Intent

Complete removal of extract_structure infrastructure from application bootstrap

## Do

- Remove the block in `internal/app/app.go` that creates and registers `extractStructureWorker`
- Lines ~699-706 should be deleted

## Accept

- [ ] No reference to NewExtractStructureWorker in app.go
- [ ] No reference to extractStructureWorker variable in app.go
