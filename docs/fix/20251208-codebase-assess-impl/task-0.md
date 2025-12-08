# Task 0: Delete extract_structure files

Depends: - | Critical: no | Model: sonnet

## Addresses User Intent

Removes redundant C/C++ specific code as specified in recommendations.md

## Do

- Delete `internal/queue/workers/extract_structure_worker.go`
- Delete `internal/jobs/actions/extract_structure.go`
- Delete `internal/jobs/actions/extract_structure_test.go`

## Accept

- [ ] All 3 files deleted
- [ ] No orphaned imports remain
